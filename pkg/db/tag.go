// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"regexp"

	"github.com/uptrace/bun"
)

func (q *Queries) LoadPatchTags(patches []Patch) error {
	if len(patches) == 0 {
		return nil
	}

	ids := make([]int, len(patches))
	byID := make(map[int]*Patch, len(patches))
	for i := range patches {
		ids[i] = patches[i].ID
		byID[patches[i].ID] = &patches[i]
	}

	type tagRow struct {
		PatchID int    `bun:"patch_id"`
		Abbrev  string `bun:"abbrev"`
		Count   int    `bun:"count"`
	}
	var rows []tagRow
	if err := q.DB.NewSelect().Model((*PatchTag)(nil)).
		ColumnExpr("patch_tag.patch_id, tag.abbrev, patch_tag.count").
		Join("JOIN tag ON tag.id = patch_tag.tag_id").
		Where("patch_tag.patch_id IN ?", bun.Tuple(ids)).
		Scan(q.Ctx, &rows); err != nil {
		return err
	}

	for _, r := range rows {
		if p, ok := byID[r.PatchID]; ok {
			if p.Tags == nil {
				p.Tags = make(map[string]int)
			}
			p.Tags[r.Abbrev] = r.Count
		}
	}
	for i := range patches {
		if patches[i].Tags == nil {
			patches[i].Tags = map[string]int{}
		}
	}
	return nil
}

// RefreshTagCounts recalculates tag counts for a patch by scanning
// the patch content and all its comments for tag patterns.
func (q *Queries) RefreshTagCounts(patch *Patch) error {
	// load tags
	var tags []Tag
	err := q.DB.NewSelect().Model(&tags).Scan(q.Ctx)
	if err != nil {
		return err
	}

	// collect all text to scan: patch content + all comment contents
	var contents []string
	if patch.Content != nil && *patch.Content != "" {
		contents = append(contents, *patch.Content)
	}

	var comments []PatchComment
	q.DB.NewSelect().Model(&comments).
		Where("patch_id = ?", patch.ID).
		Scan(q.Ctx)
	for _, c := range comments {
		if c.Content != nil && *c.Content != "" {
			contents = append(contents, *c.Content)
		}
	}

	// count matches for each tag across all content
	for _, tag := range tags {
		re, err := regexp.Compile("(?mi)" + tag.Pattern)
		if err != nil {
			continue
		}
		count := 0
		for _, content := range contents {
			count += len(re.FindAllString(content, -1))
		}

		if count == 0 {
			if _, err := q.DB.NewDelete().Model((*PatchTag)(nil)).
				Where("patch_id = ?", patch.ID).
				Where("tag_id = ?", tag.ID).
				Exec(q.Ctx); err != nil {
				return err
			}
		} else {
			pt := &PatchTag{
				PatchID: patch.ID,
				TagID:   tag.ID,
				Count:   int(count),
			}
			_, err := q.DB.NewInsert().Model(pt).
				On("CONFLICT (patch_id, tag_id) DO UPDATE").
				Set("count = EXCLUDED.count").
				Exec(q.Ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
