// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import "github.com/uptrace/bun"

func (q *Queries) LoadBundlePatches(bundles []Bundle) error {
	if len(bundles) == 0 {
		return nil
	}
	ids := make([]int, len(bundles))
	byID := make(map[int]*Bundle, len(bundles))
	for i := range bundles {
		ids[i] = bundles[i].ID
		byID[bundles[i].ID] = &bundles[i]
	}

	type row struct {
		BundleID int `bun:"bundle_id"`
		Order    int `bun:"order"`
		Patch
	}
	var rows []row
	if err := q.DB.NewSelect().
		Model((*BundlePatch)(nil)).
		ColumnExpr(`bundle_patch.bundle_id, bundle_patch.?, patch.*`, bun.Ident("order")).
		Join("JOIN patch ON patch.id = bundle_patch.patch_id").
		Where("bundle_patch.bundle_id IN ?", bun.Tuple(ids)).
		OrderExpr("bundle_patch.bundle_id, bundle_patch.\"order\" ASC").
		Scan(q.Ctx, &rows); err != nil {
		return err
	}

	for _, r := range rows {
		if b, ok := byID[r.BundleID]; ok {
			b.BundlePatches = append(b.BundlePatches, r.Patch)
		}
	}
	for i := range bundles {
		if bundles[i].BundlePatches == nil {
			bundles[i].BundlePatches = []Patch{}
		}
	}
	return nil
}

func (q *Queries) ListUserBundles(userID int) ([]Bundle, error) {
	var bundles []Bundle
	err := q.DB.NewSelect().Model(&bundles).
		Where("owner_id = ?", userID).
		OrderExpr("name ASC").
		Scan(q.Ctx)
	return bundles, err
}
