// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundleList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "bundler", "b@test")
	s.exec(t, `INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'my bundle', true)`, userID, projID)

	items := getList(t, s, "/api/1.4/bundles")
	require.Len(t, items, 1)
	b := items[0]
	assertField(t, b, "id")
	assertField(t, b, "name")
	assertField(t, b, "public")
	assertNested(t, b, "owner", "id")
	assertNested(t, b, "project", "id")
}

func TestBundleListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/bundles")
	assert.Len(t, items, 0)
}

func TestBundleListPublicOnly(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "bauth", "bauth@test")
	s.exec(t, `INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'public', true)`, userID, projID)
	s.exec(t, `INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'private', false)`, userID, projID)

	items := getList(t, s, "/api/1.4/bundles")
	assert.Len(t, items, 1)
	assert.Equal(t, "public", items[0]["name"])
}

func TestBundleListAuthSeesOwnPrivate(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "me", "me@test")
	token := s.insertToken(t, userID)
	otherID := s.insertUser(t, "other", "other@test")

	s.exec(t, `INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'my-private', false)`, userID, projID)
	s.exec(t, `INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'other-private', false)`, otherID, projID)
	s.exec(t, `INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'public-one', true)`, otherID, projID)

	resp := s.authRequest(t, "GET", "/api/1.4/bundles", token, nil)
	require.Equal(t, 200, resp.StatusCode)
	var items []map[string]any
	json.NewDecoder(resp.Body).Decode(&items)
	resp.Body.Close()

	require.Len(t, items, 2)
}

func TestBundleDetail(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "bd", "bd@test")
	var bundleID int
	s.db.NewRaw(`INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'detail bundle', true) RETURNING id`,
		userID, projID).Scan(context.Background(), &bundleID)

	b := getOne(t, s, fmt.Sprintf("/api/1.4/bundles/%d", bundleID))
	assert.Equal(t, "detail bundle", b["name"])
}

func TestBundleDetailInvalid(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/bundles/invalid")
	assert.Equal(t, 422, resp.StatusCode)
}

func TestBundleDetailPatches(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "bp", "bp@test")
	patchID := s.insertPatch(t, projID, "<bp@test>", "bundle patch")
	var bundleID int
	s.db.NewRaw(`INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'with patches', true) RETURNING id`,
		userID, projID).Scan(context.Background(), &bundleID)
	s.exec(t, `INSERT INTO bundle_patch (bundle_id, patch_id, "order")
		VALUES (?, ?, 0)`, bundleID, patchID)

	b := getOne(t, s, fmt.Sprintf("/api/1.4/bundles/%d", bundleID))
	assertValue(t, b, "name", "with patches")
	assertValue(t, b, "public", true)
	assertContains(t, b, "url", fmt.Sprintf("/bundles/%d", bundleID))
	assertContains(t, b, "mbox", fmt.Sprintf("/bundles/%d/mbox", bundleID))
	assertNested(t, b, "owner", "id")
	assertNested(t, b, "project", "id")

	patches := b["patches"].([]any)
	require.Len(t, patches, 1)
	patchObj := patches[0].(map[string]any)
	assert.Equal(t, float64(patchID), patchObj["id"])
	assert.Equal(t, "bundle patch", patchObj["name"])
}

func TestBundleNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/bundles/99999")
	assert.Equal(t, 404, resp.StatusCode)
}

func TestBundleFilterOwner(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "bfo", "bfo@test")
	s.exec(t, `INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'b', true)`, userID, projID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/bundles/?owner=%d", userID))
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/bundles/?owner=99999")
	assert.Len(t, items, 0)
}

func TestBundleFilterProject(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "bf", "bf@test")
	s.exec(t, `INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'b', true)`, userID, projID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/bundles/?project=%d", projID))
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/bundles/?project=99999")
	assert.Len(t, items, 0)
}

// --- Create ---

func TestBundleCreateAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<ca@test>", "p")

	resp := s.authRequest(t, "POST", "/api/1.4/bundles", "",
		map[string]any{"name": "b", "patches": []int{patchID}})
	assert.Equal(t, 401, resp.StatusCode)
}

func TestBundleCreateValid(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "creator", "creator@test")
	token := s.insertToken(t, userID)
	p1 := s.insertPatch(t, projID, "<cv1@test>", "patch 1")
	p2 := s.insertPatch(t, projID, "<cv2@test>", "patch 2")

	resp := s.authRequest(t, "POST", "/api/1.4/bundles", token,
		map[string]any{
			"name":    "my-bundle",
			"public":  true,
			"patches": []int{p1, p2},
		})
	require.Equal(t, 201, resp.StatusCode)

	var b map[string]any
	json.NewDecoder(resp.Body).Decode(&b)
	resp.Body.Close()

	assertValue(t, b, "name", "my-bundle")
	assertValue(t, b, "public", true)
	assertNested(t, b, "owner", "id")
	assertNested(t, b, "project", "id")

	patches := b["patches"].([]any)
	require.Len(t, patches, 2)
}

func TestBundleCreateEmptyPatches(t *testing.T) {
	s := newTestServer(t)
	s.insertProject(t)
	userID := s.insertUser(t, "empty", "empty@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "POST", "/api/1.4/bundles", token,
		map[string]any{
			"name":    "empty-bundle",
			"patches": []int{},
		})
	assert.Equal(t, 400, resp.StatusCode)
}

func TestBundleCreateCrossProject(t *testing.T) {
	s := newTestServer(t)
	proj1 := s.insertProject(t)
	userID := s.insertUser(t, "cross", "cross@test")
	token := s.insertToken(t, userID)
	p1 := s.insertPatch(t, proj1, "<cp1@test>", "patch 1")

	var proj2 int
	s.db.NewRaw(`INSERT INTO project
		(linkname, name, listid, listemail, subject_match,
		 web_url, scm_url, webscm_url,
		 list_archive_url, list_archive_url_format, commit_url_format,
		 send_notifications, use_tags, show_dependencies, auto_supersede)
		VALUES ('proj2', 'Project 2', 'proj2.lists.test', 'proj2@lists.test', '',
			'', '', '', '', '', '', false, false, false, false)
		RETURNING id`).Scan(context.Background(), &proj2)
	p2 := s.insertPatch(t, proj2, "<cp2@test>", "patch 2")

	resp := s.authRequest(t, "POST", "/api/1.4/bundles", token,
		map[string]any{
			"name":    "cross-bundle",
			"patches": []int{p1, p2},
		})
	assert.Equal(t, 400, resp.StatusCode)
}

// --- Update ---

func TestBundleUpdateOwner(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "updater", "updater@test")
	token := s.insertToken(t, userID)
	patchID := s.insertPatch(t, projID, "<u1@test>", "p1")

	// create the bundle first
	resp := s.authRequest(t, "POST", "/api/1.4/bundles", token,
		map[string]any{
			"name":    "orig-name",
			"public":  false,
			"patches": []int{patchID},
		})
	require.Equal(t, 201, resp.StatusCode)
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	bundleID := int(created["id"].(float64))

	// update name and public
	resp = s.authRequest(t, "PATCH", fmt.Sprintf("/api/1.4/bundles/%d", bundleID), token,
		map[string]any{
			"name":   "new-name",
			"public": true,
		})
	require.Equal(t, 200, resp.StatusCode)
	var updated map[string]any
	json.NewDecoder(resp.Body).Decode(&updated)
	resp.Body.Close()

	assertValue(t, updated, "name", "new-name")
	assertValue(t, updated, "public", true)
}

func TestBundleUpdatePatches(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "upatcher", "upatcher@test")
	token := s.insertToken(t, userID)
	p1 := s.insertPatch(t, projID, "<up1@test>", "p1")
	p2 := s.insertPatch(t, projID, "<up2@test>", "p2")
	p3 := s.insertPatch(t, projID, "<up3@test>", "p3")

	resp := s.authRequest(t, "POST", "/api/1.4/bundles", token,
		map[string]any{
			"name":    "patch-update",
			"patches": []int{p1},
		})
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	bundleID := int(created["id"].(float64))

	// replace patches
	resp = s.authRequest(t, "PATCH", fmt.Sprintf("/api/1.4/bundles/%d", bundleID), token,
		map[string]any{
			"patches": []int{p2, p3},
		})
	require.Equal(t, 200, resp.StatusCode)
	var updated map[string]any
	json.NewDecoder(resp.Body).Decode(&updated)
	resp.Body.Close()

	patches := updated["patches"].([]any)
	require.Len(t, patches, 2)
}

func TestBundleUpdateNonOwner(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	ownerID := s.insertUser(t, "bowner", "bowner@test")
	otherID := s.insertUser(t, "bother", "bother@test")
	otherToken := s.insertToken(t, otherID)
	patchID := s.insertPatch(t, projID, "<no1@test>", "p1")

	var bundleID int
	s.db.NewRaw(`INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'owned', true) RETURNING id`,
		ownerID, projID).Scan(context.Background(), &bundleID)
	s.exec(t, `INSERT INTO bundle_patch (bundle_id, patch_id, "order")
		VALUES (?, ?, 0)`, bundleID, patchID)

	resp := s.authRequest(t, "PATCH", fmt.Sprintf("/api/1.4/bundles/%d", bundleID), otherToken,
		map[string]any{"name": "hijacked"})
	assert.Equal(t, 403, resp.StatusCode)
}

// --- Delete ---

func TestBundleDeleteOwner(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "deleter", "deleter@test")
	token := s.insertToken(t, userID)
	patchID := s.insertPatch(t, projID, "<d1@test>", "p1")

	resp := s.authRequest(t, "POST", "/api/1.4/bundles", token,
		map[string]any{
			"name":    "to-delete",
			"patches": []int{patchID},
		})
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	bundleID := int(created["id"].(float64))

	resp = s.authRequest(t, "DELETE", fmt.Sprintf("/api/1.4/bundles/%d", bundleID), token, nil)
	assert.Equal(t, 204, resp.StatusCode)

	// verify it's gone
	resp = s.get(t, fmt.Sprintf("/api/1.4/bundles/%d", bundleID))
	assert.Equal(t, 404, resp.StatusCode)
}

func TestBundleDeleteNonOwner(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	ownerID := s.insertUser(t, "downer", "downer@test")
	otherID := s.insertUser(t, "dother", "dother@test")
	otherToken := s.insertToken(t, otherID)

	var bundleID int
	s.db.NewRaw(`INSERT INTO bundle (owner_id, project_id, name, public)
		VALUES (?, ?, 'nodelete', true) RETURNING id`,
		ownerID, projID).Scan(context.Background(), &bundleID)

	resp := s.authRequest(t, "DELETE", fmt.Sprintf("/api/1.4/bundles/%d", bundleID), otherToken, nil)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestBundleDeleteAnonymous(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "DELETE", "/api/1.4/bundles/1", "", nil)
	assert.Equal(t, 401, resp.StatusCode)
}
