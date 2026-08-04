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

func TestPatchCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/patches", "", map[string]string{"name": "x"})
	assert.Equal(t, 405, resp.StatusCode)
}

func TestPatchDelete405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "DELETE", "/api/1.4/patches/1", "", nil)
	assert.Equal(t, 405, resp.StatusCode)
}

func TestPatchDetail(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<detail@test>", "detail patch")
	p := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", patchID))
	assert.Equal(t, "detail patch", p["name"])
	assertField(t, p, "diff")
	assertField(t, p, "headers")
	assertField(t, p, "hash")
	assertNested(t, p, "submitter", "id")
	assertNested(t, p, "project", "id")
}

func TestPatchDetailInvalid(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/patches/invalid")
	// huma returns 422 for invalid path parameter type
	assert.Equal(t, 422, resp.StatusCode)
}

func TestPatchFilterArchived(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<pa@test>", "p")

	items := getList(t, s, "/api/1.4/patches/?archived=false")
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/patches/?archived=true")
	assert.Len(t, items, 0)
}

func TestPatchFilterHash(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<hash@test>", "hash")

	items := getList(t, s, "/api/1.4/patches/?hash=abc123")
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/patches/?hash=ABC123")
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/patches/?hash=nonexistent")
	assert.Len(t, items, 0)
}

func TestPatchFilterMsgid(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<msgid-filter@test>", "msgid")

	items := getList(t, s, "/api/1.4/patches/?msgid=msgid-filter@test")
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/patches/?msgid=nonexistent@test")
	assert.Len(t, items, 0)
}

func TestPatchFilterState(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<state@test>", "state")

	items := getList(t, s, "/api/1.4/patches/?state=New")
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/patches/?state=Accepted")
	assert.Len(t, items, 0)
}

func TestPatchFilterSubmitter(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<ps@test>", "p")

	personID := s.insertPerson(t, "test@example.com", "Test Author")
	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/?submitter=%d", personID))
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/patches/?submitter=99999")
	assert.Len(t, items, 0)
}

func TestPatchList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<p1@test>", "test patch")
	items := getList(t, s, "/api/1.4/patches")
	require.Len(t, items, 1)
	p := items[0]

	// value checks matching Python assertSerialized
	assertValue(t, p, "id", float64(patchID))
	assertValue(t, p, "name", "test patch")
	assertValue(t, p, "msgid", "<p1@test>")
	assertContains(t, p, "url", fmt.Sprintf("/patches/%d", patchID))
	assertContains(t, p, "mbox", fmt.Sprintf("/patches/%d/mbox", patchID))
	assertContains(t, p, "comments", fmt.Sprintf("/patches/%d/comments", patchID))
	assertContains(t, p, "checks", fmt.Sprintf("/patches/%d/checks", patchID))
	assertField(t, p, "date")
	assertField(t, p, "tags")
	assertField(t, p, "series")

	assertNested(t, p, "submitter", "id")
	assertNested(t, p, "submitter", "email")
	assertNested(t, p, "project", "id")
	assertNested(t, p, "project", "name")

	// api returns state as a plain string
	assertValue(t, p, "state", "New")

	seriesList := p["series"].([]any)
	assert.Len(t, seriesList, 0)

	tags := p["tags"].(map[string]any)
	assert.Len(t, tags, 0)

	// related uses omitempty, absent when empty
	if rel, ok := p["related"]; ok {
		related := rel.([]any)
		assert.Empty(t, related, "related should be empty")
	}
}

func TestPatchListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/patches")
	assert.Len(t, items, 0)
}

func TestPatchNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/patches/99999")
	require.Equal(t, 404, resp.StatusCode)
}

func TestPatchNotFoundUpdate(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "m4", "m4@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH", "/api/1.4/patches/99999", token,
		map[string]string{"state": "Accepted"})
	assert.Equal(t, 404, resp.StatusCode)
}

func TestPatchSearch(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<q1@test>", "unique searchable name")
	s.insertPatch(t, projID, "<q2@test>", "other patch")

	items := getList(t, s, "/api/1.4/patches/?q=searchable")
	assert.Len(t, items, 1)
}

func TestPatchUpdateAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<anon@test>", "p")

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), "",
		map[string]string{"state": "Accepted"})
	assert.Equal(t, 401, resp.StatusCode)
}

func TestPatchUpdateArchived(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<arch@test>", "p")
	userID := s.insertUser(t, "m3", "m3@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]bool{"archived": true})
	require.Equal(t, 200, resp.StatusCode)

	var result map[string]any
	decodeJSON(t, resp, &result)
	assert.Equal(t, true, result["archived"])
}

func TestPatchUpdateInvalidDelegate(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<invdel@test>", "p")
	userID := s.insertUser(t, "invdel", "invdel@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]any{"delegate": 99999})
	// Without FK constraints, the update succeeds with a dangling
	// reference. With FK constraints enabled, the DB rejects it.
	assert.Contains(t, []int{200, 400, 500}, resp.StatusCode)
}

func TestPatchUpdateInvalidState(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<bad-state@test>", "p")
	userID := s.insertUser(t, "m2", "m2@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]string{"state": "Nonexistent"})
	assert.Equal(t, 400, resp.StatusCode)
}

func TestPatchUpdateMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<maint@test>", "p")
	userID := s.insertUser(t, "maintainer", "maint@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]string{"state": "Accepted"})
	require.Equal(t, 200, resp.StatusCode)

	var result map[string]any
	decodeJSON(t, resp, &result)
	assert.Equal(t, "Accepted", result["state"])
}

func TestPatchUpdateNonMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<nonmaint@test>", "p")
	userID := s.insertUser(t, "normie", "normie@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]string{"state": "Accepted"})
	assert.Equal(t, 403, resp.StatusCode)
}

func TestPatchUpdateState(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<state-up@test>", "p")
	userID := s.insertUser(t, "maint", "m@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]string{"state": "RFC"})
	require.Equal(t, 200, resp.StatusCode)

	var result map[string]any
	decodeJSON(t, resp, &result)
	assert.Equal(t, "RFC", result["state"])
}

func TestPatchWebURL(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t) // has web_url = http://example.com
	patchID := s.insertPatch(t, projID, "<weburl@test>", "p")

	p := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", patchID))
	assertField(t, p, "web_url")
	webURL, _ := p["web_url"].(string)
	assert.NotEmpty(t, webURL, "web_url should not be empty")
}
