// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type UserCmd struct {
	List   UserListCmd   `cmd:"" help:"List users."`
	Create UserCreateCmd `cmd:"" help:"Create a user."`
	Delete UserDeleteCmd `cmd:"" help:"Delete a user."`
	Passwd UserPasswdCmd `cmd:"" help:"Change a user password."`
}

type UserListCmd struct{}

func (c *UserListCmd) Run(ctx context.Context) error {
	var users []db.User
	err := pw.NewQueries(ctx).Select(&users).
		OrderExpr("username ASC").
		Scan(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tUSERNAME\tEMAIL\tACTIVE\tADMIN\n")
	for _, u := range users {
		fmt.Fprintf(w, "%d\t%s\t%s\t%v\t%v\n",
			u.ID, u.Username, u.Email,
			u.IsActive, u.IsAdmin)
	}
	return w.Flush()
}

type UserCreateCmd struct {
	Username string `short:"u" help:"Username."`
	Email    string `short:"e" help:"Email address."`
	Admin    bool   `help:"Grant admin privileges."`
}

func (c *UserCreateCmd) Run(ctx context.Context) error {
	if c.Username == "" {
		var err error
		c.Username, err = readLine("Username: ")
		if err != nil {
			return err
		}
		if c.Username == "" {
			return fmt.Errorf("username cannot be empty")
		}
	}
	if c.Email == "" {
		var err error
		c.Email, err = readLine("Email: ")
		if err != nil {
			return err
		}
	}

	password, err := readPassword("Password: ")
	if err != nil {
		return err
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	confirm, err := readPassword("Confirm password: ")
	if err != nil {
		return err
	}
	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}

	q, err := pw.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer q.Rollback()

	user := db.User{
		Username:   c.Username,
		Email:      c.Email,
		Password:   db.HashPassword(password),
		IsAdmin:    c.Admin,
		IsActive:   true,
		DateJoined: time.Now(),
	}
	err = q.Insert(&user)
	if err != nil {
		return err
	}

	err = q.Commit()
	if err != nil {
		return err
	}

	fmt.Printf("Created user %q (id=%d)\n", user.Username, user.ID)
	return nil
}

type UserDeleteCmd struct {
	Force    bool   `short:"f" help:"Skip confirmation."`
	Username string `arg:"" help:"Username to delete."`
}

func (c *UserDeleteCmd) Run(ctx context.Context) error {
	q, err := pw.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer q.Rollback()

	user, err := q.GetUserByUsername(c.Username)
	if err != nil {
		return fmt.Errorf("user %q not found", c.Username)
	}

	if !c.Force {
		fmt.Printf("Delete user %q (id=%d)? [y/N] ", user.Username, user.ID)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	_, err = q.Delete(user).WherePK().Exec(ctx)
	if err != nil {
		return err
	}

	err = q.Commit()
	if err != nil {
		return err
	}

	fmt.Printf("Deleted user %q\n", c.Username)
	return nil
}

type UserPasswdCmd struct {
	Username string `arg:"" help:"Username to change password for."`
}

func (c *UserPasswdCmd) Run(ctx context.Context) error {
	q, err := pw.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer q.Rollback()

	user, err := q.GetUserByUsername(c.Username)
	if err != nil {
		return fmt.Errorf("user %q not found", c.Username)
	}

	password, err := readPassword("New password: ")
	if err != nil {
		return err
	}
	confirm, err := readPassword("Confirm password: ")
	if err != nil {
		return err
	}
	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	_, err = q.Update(user).
		WherePK().
		Set("password = ?", db.HashPassword(password)).
		Exec(ctx)
	if err != nil {
		return err
	}

	err = q.Commit()
	if err != nil {
		return err
	}

	fmt.Printf("Password updated for %q\n", c.Username)
	return nil
}

func readLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	return strings.TrimSpace(line), err
}

// stdinReader is shared across readPassword calls so that a single reader's
// buffered read-ahead is not discarded between the password and confirmation
// prompts. A fresh bufio.Reader per call would swallow the second line.
var stdinReader = bufio.NewReader(os.Stdin)

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(pw), err
	}
	line, err := stdinReader.ReadString('\n')
	return strings.TrimSpace(line), err
}
