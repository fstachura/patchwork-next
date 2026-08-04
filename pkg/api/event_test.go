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

func TestEventActor(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "actor", "actor@test")
	patchID := s.insertPatch(t, projID, "<evact@test>", "p")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id, actor_id)
		VALUES (?, 'patch-state-changed', datetime('now'), ?, ?)`,
		projID, patchID, userID)

	items := getList(t, s, "/api/1.4/events")
	require.Len(t, items, 1)
	assertNested(t, items[0], "actor", "id")
}

func TestEventActorNull(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<evan@test>", "p")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)`, projID, patchID)

	items := getList(t, s, "/api/1.4/events")
	assert.Nil(t, items[0]["actor"], "actor should be null")
}

func TestEventCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/events", "", map[string]string{"category": "x"})
	assert.Equal(t, 405, resp.StatusCode)
}

func TestEventPayload(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<evp@test>", "event patch")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)`, projID, patchID)

	items := getList(t, s, "/api/1.4/events")
	require.Len(t, items, 1)
	assertField(t, items[0], "payload")
	payload, ok := items[0]["payload"].(map[string]any)
	require.True(t, ok, "payload is not an object")
	patch, ok := payload["patch"].(map[string]any)
	require.True(t, ok, "payload.patch is not an object")
	assert.Equal(t, "event patch", patch["name"])
}

func TestEventsFilterActor(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "efa", "efa@test")
	patchID := s.insertPatch(t, projID, "<efa@test>", "p")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id, actor_id)
		VALUES (?, 'patch-state-changed', datetime('now'), ?, ?)`,
		projID, patchID, userID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/events/?actor=%d", userID))
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/events/?actor=99999")
	assert.Len(t, items, 0)
}

func TestEventsFilterCategory(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<efc@test>", "p")
	s.exec(t, `
		INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)
	`, projID, patchID)
	s.exec(t, `
		INSERT INTO event (project_id, category, date, series_id)
		VALUES (?, 'series-created', datetime('now'), NULL)
	`, projID)

	items := getList(t, s, "/api/1.4/events/?category=patch-created")
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/events/?category=nonexistent")
	assert.Len(t, items, 0)
}

func TestEventsFilterPatch(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<efpa1@test>", "p1")
	s.insertPatch(t, projID, "<efpa2@test>", "p2")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)`, projID, p1)

	items := getList(t, s, fmt.Sprintf("/api/1.4/events/?patch=%d", p1))
	assert.Len(t, items, 1)
}

func TestEventsFilterProject(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<efp@test>", "p")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)`, projID, patchID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/events/?project=%d", projID))
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/events/?project=99999")
	assert.Len(t, items, 0)
}

func TestEventsFilterSeries(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "efs@test", "EFS")
	seriesID := s.insertSeries(t, projID, personID, "s")
	s.exec(t, `INSERT INTO event (project_id, category, date, series_id)
		VALUES (?, 'series-created', datetime('now'), ?)`, projID, seriesID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/events/?series=%d", seriesID))
	assert.Len(t, items, 1)
}

func TestEventsListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/events")
	assert.Len(t, items, 0)
}

func TestEventsOrderAscending(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.exec(t, `INSERT INTO event (project_id, category, date)
		VALUES (?, 'series-created', datetime('now', '-1 hour'))`, projID)
	s.exec(t, `INSERT INTO event (project_id, category, date)
		VALUES (?, 'patch-created', datetime('now'))`, projID)

	items := getList(t, s, "/api/1.4/events/?order=date")
	require.Len(t, items, 2)
	assert.Equal(t, "series-created", items[0]["category"], "ascending order should show oldest first")
}

func TestEventsOrderByDate(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.exec(t, `INSERT INTO event (project_id, category, date)
		VALUES (?, 'series-created', datetime('now', '-1 hour'))`, projID)
	s.exec(t, `INSERT INTO event (project_id, category, date)
		VALUES (?, 'patch-created', datetime('now'))`, projID)

	items := getList(t, s, "/api/1.4/events")
	require.Len(t, items, 2)
	assert.Equal(t, "patch-created", items[0]["category"], "default order should be newest first")
}

func TestEventsWithData(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<ev@test>", "event patch")
	s.exec(t, `
		INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)
	`, projID, patchID)

	items := getList(t, s, "/api/1.4/events")
	require.Len(t, items, 1)
	ev := items[0]
	assertField(t, ev, "id")
	assertField(t, ev, "category")
	assertField(t, ev, "date")
	assertNested(t, ev, "project", "id")
	assert.Equal(t, "patch-created", ev["category"])
}
