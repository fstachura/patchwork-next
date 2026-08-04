// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Register(up0003, down0003)
}

func up0003(ctx context.Context, tx bun.Tx) error {
	_, err := tx.NewDropIndex().
		Index("idx_patch_project_id_archived_date_desc").
		IfExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = tx.NewCreateIndex().
		Table("patch").
		Index("idx_patch_project_id_archived_state_id_date_desc").
		ColumnExpr("project_id, archived, state_id, date DESC").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = tx.NewCreateIndex().
		Table("ci_check").
		Index("idx_ci_check_patch_id_state").
		ColumnExpr("patch_id, state").
		IfNotExists().
		Exec(ctx)
	return err
}

func down0003(ctx context.Context, tx bun.Tx) error {
	_, err := tx.NewDropIndex().
		Index("idx_ci_check_patch_id_state").
		IfExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = tx.NewDropIndex().
		Index("idx_patch_project_id_archived_state_id_date_desc").
		IfExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = tx.NewCreateIndex().
		Table("patch").
		Index("idx_patch_project_id_archived_date_desc").
		ColumnExpr("project_id, archived, date DESC").
		IfNotExists().
		Exec(ctx)
	return err
}
