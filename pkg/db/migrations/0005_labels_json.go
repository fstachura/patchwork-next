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

func init() {
	Register(up0005, down0005)
}

func up0005(ctx context.Context, tx bun.Tx) error {
	_, err := tx.NewAddColumn().Model(new(db.Patch)).ColumnExpr("label_ids jsonb").Exec(ctx)
	if err != nil {
		return err
	}

	patchLabels := tx.NewSelect().
		Model((*db.Patch)(nil)).
		ColumnExpr("patch.id, jsonb_agg(pl.label_id) as labels_json").
		Join("INNER JOIN patch_label AS pl on pl.patch_id=patch.id").
		Group("patch.id")

	_, err = tx.NewUpdate().
		With("patch_labels", patchLabels).
		Model(new(db.Patch)).
		TableExpr("patch_labels").
		Set("label_ids = patch_labels.labels_json").
		Where("patch.id = patch_labels.id").
		Exec(ctx)

	return err
}

func down0005(ctx context.Context, tx bun.Tx) error {
	_, err := tx.NewDropColumn().Model(new(db.Patch)).Column("label_ids").Exec(ctx)
	return err
}
