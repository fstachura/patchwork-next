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

func (q *Queries) ListLabels() ([]Label, error) {
	var labels []Label
	err := q.DB.NewSelect().Model(&labels).
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
	q *bun.SelectQuery, include, exclude []int,
) *bun.SelectQuery {
	for _, id := range exclude {
		q = q.Where("NOT ('?'::jsonb <@ label_ids)", id)
	}
	for _, id := range include {
		q = q.Where("'?'::jsonb <@ label_ids", id)
	}
	return q
}

func (q *Queries) LoadPatchLabels(patches []Patch, projectLabels []Label) error {
	if len(patches) == 0 {
		return nil
	}

	for pi, _ := range patches {
		for _, labelId := range patches[pi].LabelIDs {
			for _, labelModel := range projectLabels {
				if labelModel.ID == labelId {
					patches[pi].Labels = append(patches[pi].Labels, labelModel)
					break
				}
			}
		}
	}

	return nil
}
