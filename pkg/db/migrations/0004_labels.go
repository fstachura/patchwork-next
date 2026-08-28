// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

type label0004 struct {
	bun.BaseModel `bun:"table:label" unique:"project_id,name"`

	ID          int    `bun:"id,pk,autoincrement"`
	ProjectID   *int   `bun:"project_id" fk:"project.id,cascade"`
	Name        string `bun:"name,notnull"`
	Description string `bun:"description,notnull"`
	Color       int    `bun:"color,notnull"`
}

type patchLabel0004 struct {
	bun.BaseModel `bun:"table:patch_label" unique:"patch_id,label_id"`

	ID      int `bun:"id,pk,autoincrement"`
	PatchID int `bun:"patch_id,notnull" fk:"patch.id,cascade"`
	LabelID int `bun:"label_id,notnull" fk:"label.id,cascade"`
}

func init() {
	Register(up0004, down0004)
}

func up0004(ctx context.Context, tx bun.Tx) error {
	if err := db.CreateSchemaFrom(ctx, tx, []any{
		(*label0004)(nil),
		(*patchLabel0004)(nil),
	}); err != nil {
		return err
	}
	_, err := tx.NewInsert().Model(&label0004{
		Name:        "RFC",
		Description: "Request for comments",
		Color:       0x0097a7,
	}).On("CONFLICT DO NOTHING").Exec(ctx)
	return err
}

func down0004(ctx context.Context, tx bun.Tx) error {
	for _, table := range []string{"patch_label", "label"} {
		if _, err := tx.NewDropTable().Table(table).IfExists().Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}
