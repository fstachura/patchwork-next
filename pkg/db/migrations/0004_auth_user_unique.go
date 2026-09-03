// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

func init() {
	Register(up0004, down0004)
}

// up0004 enforces uniqueness of auth_user.username. Django never
// constrained the column, so existing databases may hold duplicates.
// Rather than silently corrupt data or fail with an opaque driver error,
// the migration checks first and reports the offending values so an
// operator can resolve them before retrying.
func up0004(ctx context.Context, tx bun.Tx) error {
	if err := checkAuthUserDuplicates(ctx, tx, "username"); err != nil {
		return err
	}

	_, err := tx.NewCreateIndex().
		Table("auth_user").
		Index("uidx_auth_user_username").
		Unique().
		Column("username").
		IfNotExists().
		Exec(ctx)
	return err
}

func checkAuthUserDuplicates(ctx context.Context, tx bun.Tx, column string) error {
	var dups []string
	err := tx.NewSelect().
		TableExpr("auth_user").
		Column(column).
		Group(column).
		Having("COUNT(*) > 1").
		Order(column).
		Scan(ctx, &dups)
	if err != nil {
		return fmt.Errorf("check duplicate %s: %w", column, err)
	}
	if len(dups) > 0 {
		return fmt.Errorf(
			"cannot add unique constraint on auth_user.%s: %d duplicate "+
				"value(s) must be resolved first: %s",
			column, len(dups), strings.Join(dups, ", "))
	}
	return nil
}

func down0004(ctx context.Context, tx bun.Tx) error {
	_, err := tx.NewDropIndex().
		Index("uidx_auth_user_username").
		IfExists().
		Exec(ctx)
	return err
}
