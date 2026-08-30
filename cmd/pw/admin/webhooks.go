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
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alecthomas/kong"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type WebhookCmd struct {
	List   WebhookListCmd   `cmd:"" help:"List webhooks for a project."`
	Create WebhookCreateCmd `cmd:"" help:"Create a webhook."`
	Update WebhookUpdateCmd `cmd:"" help:"Update a webhook."`
	Delete WebhookDeleteCmd `cmd:"" help:"Delete a webhook."`
}

type WebhookListCmd struct {
	Project string `arg:"" default:"" help:"Project linkname (omit to list all)."`
}

func (c *WebhookListCmd) Run(ctx context.Context) error {
	queries := pw.NewQueries(ctx)

	var hooks []db.Webhook

	q := queries.Select(&hooks).OrderExpr("id ASC")
	if c.Project != "" {
		project, err := queries.GetProjectByLinkname(c.Project)
		if err != nil {
			return fmt.Errorf("project %q not found", c.Project)
		}
		q = q.Where("project_id = ?", project.ID)
	}

	if err := q.Scan(ctx); err != nil {
		return err
	}

	// resolve project names
	projIDs := make(map[int]bool)
	for _, h := range hooks {
		projIDs[h.ProjectID] = true
	}
	projMap := make(map[int]string)
	if len(projIDs) > 0 {
		var projects []db.Project
		ids := make([]int, 0, len(projIDs))
		for id := range projIDs {
			ids = append(ids, id)
		}
		queries.Select(&projects).
			Where("id IN ?", bun.Tuple(ids)).
			Scan(ctx)
		for _, p := range projects {
			projMap[p.ID] = p.Linkname
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tPROJECT\tURL\tEVENTS\tACTIVE\n")
	for _, h := range hooks {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%v\n",
			h.ID, projMap[h.ProjectID], h.URL, h.Events, h.Active)
	}
	return w.Flush()
}

func EventCategories() []string {
	events := make([]string, 0, len(db.ValidEventCategories))
	for e := range db.ValidEventCategories {
		events = append(events, e)
	}
	slices.Sort(events)
	return events
}

type EventsFlag string

func (v *EventsFlag) Decode(ctx *kong.DecodeContext) error {
	tok, err := ctx.Scan.PopValue("events")
	if err != nil {
		return err
	}
	val := fmt.Sprintf("%v", tok.Value)
	if val == "?" {
		fmt.Println("Valid event categories:")
		for _, c := range EventCategories() {
			fmt.Printf("  %s\n", c)
		}
		os.Exit(0)
	}
	*v = EventsFlag(val)
	return nil
}

type WebhookCreateCmd struct {
	Project string     `required:"" help:"Project linkname."`
	User    string     `required:"" help:"Creator username."`
	URL     string     `required:"" name:"url" help:"Webhook endpoint URL."`
	Secret  string     `name:"secret" help:"Secret for HMAC signatures."`
	Events  EventsFlag `name:"events" default:"*" completion:"csv:events" help:"Comma-separated event categories, or '*' for all (use '?' to list all supported events)."`
	Active  bool       `name:"active" default:"true" negatable:"" help:"Whether the webhook is active."`
}

func (c *WebhookCreateCmd) Run(ctx context.Context) error {
	if err := db.ValidateEvents(string(c.Events)); err != nil {
		return err
	}

	q, err := pw.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer q.Rollback()

	project, err := q.GetProjectByLinkname(c.Project)
	if err != nil {
		return fmt.Errorf("project %q not found", c.Project)
	}

	user, err := q.GetUserByUsername(c.User)
	if err != nil {
		return fmt.Errorf("user %q not found", c.User)
	}

	hook := db.Webhook{
		ProjectID: project.ID,
		URL:       c.URL,
		Secret:    c.Secret,
		Events:    string(c.Events),
		Active:    c.Active,
		CreatorID: user.ID,
		Created:   time.Now(),
	}
	err = q.Insert(&hook)
	if err != nil {
		return err
	}

	err = q.Commit()
	if err != nil {
		return err
	}

	fmt.Printf("Created webhook (id=%d) %s\n", hook.ID, hook.URL)
	return nil
}

type WebhookUpdateCmd struct {
	ID     int    `arg:"" help:"Webhook ID."`
	URL    string `name:"url" help:"Webhook endpoint URL."`
	Secret string `name:"secret" help:"Secret for HMAC signatures."`
	Events string `name:"events" completion:"csv:events" help:"Comma-separated event categories, or '*' for all (use '?' to list all supported events)."`
	Active *bool  `name:"active" negatable:"" help:"Whether the webhook is active."`
}

func (c *WebhookUpdateCmd) Run(ctx context.Context) error {
	queries, err := pw.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer queries.Rollback()

	hook, err := queries.GetWebhookByID(c.ID)
	if err != nil {
		return fmt.Errorf("webhook %d not found", c.ID)
	}

	q := queries.Update(hook).WherePK()
	updated := false
	if c.URL != "" {
		q = q.Set("url = ?", c.URL)
		updated = true
	}
	if c.Secret != "" {
		q = q.Set("secret = ?", c.Secret)
		updated = true
	}
	if c.Events != "" {
		if err := db.ValidateEvents(c.Events); err != nil {
			return err
		}
		q = q.Set("events = ?", c.Events)
		updated = true
	}
	if c.Active != nil {
		q = q.Set("active = ?", *c.Active)
		updated = true
	}

	if !updated {
		return fmt.Errorf("no fields to update")
	}

	_, err = q.Exec(ctx)
	if err != nil {
		return err
	}

	err = queries.Commit()
	if err != nil {
		return err
	}

	fmt.Printf("Updated webhook %d\n", c.ID)
	return nil
}

type WebhookDeleteCmd struct {
	Force bool `short:"f" help:"Skip confirmation."`
	ID    int  `arg:"" help:"Webhook ID to delete."`
}

func (c *WebhookDeleteCmd) Run(ctx context.Context) error {
	q, err := pw.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer q.Rollback()

	hook, err := q.GetWebhookByID(c.ID)
	if err != nil {
		return fmt.Errorf("webhook %d not found", c.ID)
	}

	if !c.Force {
		fmt.Printf("Delete webhook %d (url=%q)? [y/N] ",
			hook.ID, hook.URL)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	_, err = q.Delete(hook).WherePK().Exec(ctx)
	if err != nil {
		return err
	}

	err = q.Commit()
	if err != nil {
		return err
	}

	fmt.Printf("Deleted webhook %d\n", c.ID)
	return nil
}
