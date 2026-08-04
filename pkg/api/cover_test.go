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

func TestCoverCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/covers", "", map[string]string{"name": "x"})
	assert.Equal(t, 405, resp.StatusCode)
}

func TestCoverDelete405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "DELETE", "/api/1.4/covers/1", "", nil)
	assert.Equal(t, 405, resp.StatusCode)
}

func TestCoverDetail(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	coverID := s.insertCover(t, projID, "<cd@test>", "cover detail")
	c := getOne(t, s, fmt.Sprintf("/api/1.4/covers/%d", coverID))
	assertField(t, c, "content")
	assertField(t, c, "headers")
}

func TestCoverFilterMsgid(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertCover(t, projID, "<cm-filter@test>", "c")

	items := getList(t, s, "/api/1.4/covers/?msgid=cm-filter@test")
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/covers/?msgid=nope@test")
	assert.Len(t, items, 0)
}

func TestCoverFilterProject(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertCover(t, projID, "<cf@test>", "c")

	items := getList(t, s, fmt.Sprintf("/api/1.4/covers/?project=%d", projID))
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/covers/?project=99999")
	assert.Len(t, items, 0)
}

func TestCoverFilterSubmitter(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertCover(t, projID, "<cs@test>", "c")
	// insertCover creates person "test@example.com"
	personID := s.insertPerson(t, "test@example.com", "Test Author")

	items := getList(t, s, fmt.Sprintf("/api/1.4/covers/?submitter=%d", personID))
	assert.Len(t, items, 1)
}

func TestCoverList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	coverID := s.insertCover(t, projID, "<c1@test>", "test cover")
	items := getList(t, s, "/api/1.4/covers")
	require.Len(t, items, 1)
	c := items[0]
	assertValue(t, c, "id", float64(coverID))
	assertValue(t, c, "name", "test cover")
	assertValue(t, c, "msgid", "<c1@test>")
	assertContains(t, c, "url", fmt.Sprintf("/covers/%d", coverID))
	assertContains(t, c, "mbox", fmt.Sprintf("/covers/%d/mbox", coverID))
	assertContains(t, c, "comments", fmt.Sprintf("/covers/%d/comments", coverID))
	assertField(t, c, "date")
	assertField(t, c, "series")
	assertNested(t, c, "submitter", "id")
	assertNested(t, c, "submitter", "email")
	assertNested(t, c, "project", "id")
}

func TestCoverListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/covers")
	assert.Len(t, items, 0)
}

func TestCoverNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/covers/99999")
	assert.Equal(t, 404, resp.StatusCode)
}

func TestEventsFilterCover(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	coverID := s.insertCover(t, projID, "<efc@test>", "c")
	s.exec(t, `INSERT INTO event (project_id, category, date, cover_id)
		VALUES (?, 'cover-created', datetime('now'), ?)`, projID, coverID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/events/?cover=%d", coverID))
	assert.Len(t, items, 1)
}
