// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"strings"

	"github.com/uptrace/bun"
)

func (q *Queries) ListProjectLabels(projectID int) ([]Label, error) {
	var labels []Label
	err := q.DB.NewSelect().Model(&labels).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("project_id = ?", projectID).
				WhereOr("project_id IS NULL")
		}).
		OrderExpr("name ASC").
		Scan(q.Ctx)
	return labels, err
}

func (q *Queries) FindLabelsByName(projectID int, names []string) ([]Label, error) {
	var labels []Label
	lower := make([]string, len(names))
	for i, n := range names {
		lower[i] = strings.ToLower(n)
	}
	err := q.DB.NewSelect().Model(&labels).
		Where("LOWER(name) IN ?", bun.Tuple(lower)).
		WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("project_id = ?", projectID).
				WhereOr("project_id IS NULL")
		}).
		Scan(q.Ctx)
	return labels, err
}

func (q *Queries) SetPatchLabels(patchID int, labelIDs []int) error {
	for _, labelID := range labelIDs {
		pl := PatchLabel{PatchID: patchID, LabelID: labelID}
		_, err := q.DB.NewInsert().Model(&pl).
			On("CONFLICT (patch_id, label_id) DO NOTHING").
			Exec(q.Ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func FilterPatchLabels(
	q *bun.SelectQuery, include, exclude []string,
) *bun.SelectQuery {
	if len(exclude) > 0 {
		sub := q.NewSelect().
			Model((*PatchLabel)(nil)).
			Column("patch_id").
			Join("JOIN label ON label.id = patch_label.label_id").
			Where("label.name IN ?", bun.Tuple(exclude))
		q = q.Where("patch.id NOT IN (?)", sub)
	}
	if len(include) > 0 {
		sub := q.NewSelect().
			Model((*PatchLabel)(nil)).
			Column("patch_id").
			Join("JOIN label ON label.id = patch_label.label_id").
			Where("label.name IN ?", bun.Tuple(include)).
			GroupExpr("patch_id").
			Having("COUNT(DISTINCT label.name) >= ?", len(include))
		q = q.Where("patch.id IN (?)", sub)
	}
	return q
}

func (q *Queries) LoadPatchLabels(patches []Patch) error {
	if len(patches) == 0 {
		return nil
	}

	ids := make([]int, len(patches))
	byId := make(map[int]*Patch, len(patches))
	for i := range patches {
		p := &patches[i]
		ids[i] = p.ID
		byId[p.ID] = p
	}

	type labelRow struct {
		PatchID int    `bun:"patch_id"`
		Name    string `bun:"name"`
		Color   int    `bun:"color"`
	}
	var labelRows []labelRow
	if err := q.DB.NewSelect().Model((*PatchLabel)(nil)).
		ColumnExpr("patch_id, label.name, label.color").
		Join("JOIN label ON label_id = label.id").
		Where("patch_id IN ?", bun.Tuple(ids)).
		OrderExpr("label.name ASC").
		Scan(q.Ctx, &labelRows); err != nil {
		return err
	}

	for _, r := range labelRows {
		if p, ok := byId[r.PatchID]; ok {
			p.Labels = append(p.Labels, Label{
				Name:  r.Name,
				Color: r.Color,
			})
		}
	}
	return nil
}
