// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import "github.com/uptrace/bun"

func (p *Patch) CombinedCheckState() string {
	if p.CheckCounts == [4]int{} {
		return "pending"
	}
	if p.CheckCounts[CheckFail] > 0 {
		return "fail"
	}
	if p.CheckCounts[CheckWarning] > 0 {
		return "warning"
	}
	if p.CheckCounts[CheckPending] > 0 {
		return "pending"
	}
	return "success"
}

func (q *Queries) LoadPatchCheckCounts(patches []Patch) error {
	if len(patches) == 0 {
		return nil
	}

	ids := make([]int, len(patches))
	byID := make(map[int]*Patch, len(patches))
	for i := range patches {
		ids[i] = patches[i].ID
		byID[patches[i].ID] = &patches[i]
	}

	type checkRow struct {
		PatchID int `bun:"patch_id"`
		State   int `bun:"state"`
		Count   int `bun:"count"`
	}
	var rows []checkRow
	if err := q.DB.NewSelect().Model((*Check)(nil)).
		Column("patch_id", "state").
		ColumnExpr("count(*) AS count").
		Where("patch_id IN ?", bun.Tuple(ids)).
		GroupExpr("patch_id, state").
		Scan(q.Ctx, &rows); err != nil {
		return err
	}

	for _, r := range rows {
		if p, ok := byID[r.PatchID]; ok {
			if r.State >= 0 && r.State < 4 {
				p.CheckCounts[r.State] = r.Count
			}
		}
	}
	return nil
}
