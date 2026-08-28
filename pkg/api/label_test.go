// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s *testServer) insertLabel(t *testing.T, name string, projectID *int, color int) int {
	t.Helper()
	var id int
	if projectID != nil {
		s.db.NewRaw(`
			INSERT INTO label (name, description, color, project_id)
			VALUES (?, '', ?, ?)
			RETURNING id
		`, name, color, *projectID).Scan(t.Context(), &id)
	} else {
		s.db.NewRaw(`
			INSERT INTO label (name, description, color)
			VALUES (?, '', ?)
			RETURNING id
		`, name, color).Scan(t.Context(), &id)
	}
	return id
}

func (s *testServer) addPatchLabel(t *testing.T, patchID, labelID int) {
	t.Helper()
	s.exec(t, `
		INSERT INTO patch_label (patch_id, label_id)
		VALUES (?, ?)
	`, patchID, labelID)
}

func TestPatchLabelsInList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<lab-list@test>", "labeled patch")
	labelID := s.insertLabel(t, "RFC", nil, 0x0097a7)
	s.addPatchLabel(t, patchID, labelID)

	items := getList(t, s, "/api/1.4/patches")
	require.Len(t, items, 1)
	labels, ok := items[0]["labels"].([]any)
	require.True(t, ok, "labels field should be an array")
	require.Len(t, labels, 1)
	assert.Equal(t, "RFC", labels[0])
}

func TestPatchLabelsInDetail(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<lab-det@test>", "labeled detail")
	labelID := s.insertLabel(t, "RFC", nil, 0x0097a7)
	s.addPatchLabel(t, patchID, labelID)

	p := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", patchID))
	labels, ok := p["labels"].([]any)
	require.True(t, ok, "labels field should be an array")
	require.Len(t, labels, 1)
	assert.Equal(t, "RFC", labels[0])
}

func TestPatchLabelsEmpty(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<no-lab@test>", "no labels")

	items := getList(t, s, "/api/1.4/patches")
	require.Len(t, items, 1)
	labels, ok := items[0]["labels"].([]any)
	require.True(t, ok, "labels field should be an array")
	assert.Empty(t, labels)
}

func TestPatchLabelsMultiple(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<multi-lab@test>", "multi labels")
	l1 := s.insertLabel(t, "RFC", nil, 0x0097a7)
	l2 := s.insertLabel(t, "WIP", &projID, 0xff9800)
	s.addPatchLabel(t, patchID, l1)
	s.addPatchLabel(t, patchID, l2)

	p := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", patchID))
	labels, ok := p["labels"].([]any)
	require.True(t, ok)
	require.Len(t, labels, 2)
	assert.Equal(t, "RFC", labels[0])
	assert.Equal(t, "WIP", labels[1])
}

func TestPatchFilterByLabel(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<fl1@test>", "rfc patch")
	p2 := s.insertPatch(t, projID, "<fl2@test>", "normal patch")
	labelID := s.insertLabel(t, "RFC", nil, 0x0097a7)
	s.addPatchLabel(t, p1, labelID)
	_ = p2

	items := getList(t, s, "/api/1.4/patches?labels=RFC")
	require.Len(t, items, 1)
	assert.Equal(t, "rfc patch", items[0]["name"])
}

func TestPatchFilterByLabelExclude(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<fle1@test>", "rfc patch")
	s.insertPatch(t, projID, "<fle2@test>", "normal patch")
	labelID := s.insertLabel(t, "RFC", nil, 0x0097a7)
	s.addPatchLabel(t, p1, labelID)

	items := getList(t, s, "/api/1.4/patches?labels=-RFC")
	require.Len(t, items, 1)
	assert.Equal(t, "normal patch", items[0]["name"])
}

func TestPatchFilterByMultipleLabels(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<fml1@test>", "both labels")
	p2 := s.insertPatch(t, projID, "<fml2@test>", "one label")
	l1 := s.insertLabel(t, "RFC", nil, 0x0097a7)
	l2 := s.insertLabel(t, "WIP", nil, 0xff9800)
	s.addPatchLabel(t, p1, l1)
	s.addPatchLabel(t, p1, l2)
	s.addPatchLabel(t, p2, l1)

	items := getList(t, s, "/api/1.4/patches?labels=RFC,WIP")
	require.Len(t, items, 1)
	assert.Equal(t, "both labels", items[0]["name"])
}

func TestPatchLabelsNotInOlderAPI(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<old-api@test>", "old api")
	labelID := s.insertLabel(t, "RFC", nil, 0x0097a7)
	s.addPatchLabel(t, patchID, labelID)

	p := getOne(t, s, fmt.Sprintf("/api/1.3/patches/%d", patchID))
	_, hasLabels := p["labels"]
	assert.False(t, hasLabels, "labels should not appear in API 1.3")
}
