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

func TestPatchFilterProject(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<fp@test>", "filtered")

	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/?project=%d", projID))
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/patches/?project=99999")
	assert.Len(t, items, 0)
}

func TestProjectCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/projects", "", map[string]string{"name": "x"})
	assert.Equal(t, 405, resp.StatusCode)
}

func TestProjectDelete405(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "pdel", "pdel@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "DELETE",
		fmt.Sprintf("/api/1.4/projects/%d", projID), token, nil)
	assert.Equal(t, 405, resp.StatusCode)
}

func TestProjectDetailByID(t *testing.T) {
	s := newTestServer(t)
	id := s.insertProject(t)
	p := getOne(t, s, fmt.Sprintf("/api/1.4/projects/%d", id))
	assert.Equal(t, "Test Project", p["name"])
}

func TestProjectDetailByLinkname(t *testing.T) {
	s := newTestServer(t)
	s.insertProject(t)
	p := getOne(t, s, "/api/1.4/projects/test-project")
	assert.Equal(t, "test-project", p["link_name"])
}

func TestProjectDetailByNumericLinkname(t *testing.T) {
	s := newTestServer(t)
	s.exec(t, `
		INSERT INTO project (
			linkname, name, listid, listemail, subject_match,
			web_url, scm_url, webscm_url,
			list_archive_url, list_archive_url_format, commit_url_format,
			send_notifications, use_tags, show_dependencies, auto_supersede
		) VALUES ('12345', 'Numeric Project', 'num.example.com',
			'num@example.com', '', '', '', '', '', '', '',
			false, true, false, false)
	`)

	p := getOne(t, s, "/api/1.4/projects/12345")
	assert.Equal(t, "12345", p["link_name"])
}

func TestProjectList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	items := getList(t, s, "/api/1.4/projects")
	require.Len(t, items, 1)
	p := items[0]
	assertValue(t, p, "id", float64(projID))
	assertValue(t, p, "name", "Test Project")
	assertValue(t, p, "link_name", "test-project")
	assertValue(t, p, "list_id", "test.example.com")
	assertValue(t, p, "list_email", "test@test.example.com")
	assertContains(t, p, "url", fmt.Sprintf("/projects/%d", projID))
	assertField(t, p, "web_url")
	assertField(t, p, "maintainers")

	maintainers := p["maintainers"].([]any)
	assert.Len(t, maintainers, 0)
}

func TestProjectListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/projects")
	assert.Len(t, items, 0)
}

func TestProjectMaintainers(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "maintainer", "maint@test")
	s.makeMaintainer(t, userID, projID)

	p := getOne(t, s, fmt.Sprintf("/api/1.4/projects/%d", projID))
	maintainers, ok := p["maintainers"].([]any)
	require.True(t, ok, "maintainers should be an array")
	assert.Len(t, maintainers, 1)
}

func TestProjectNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/projects/nonexistent")
	require.Equal(t, 404, resp.StatusCode)
}

func TestProjectUpdateAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/projects/%d", projID), "",
		map[string]string{"web_url": "https://hack.com"})
	assert.Equal(t, 401, resp.StatusCode)
}

func TestProjectUpdateMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "pmaint", "pm@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/projects/%d", projID), token,
		map[string]string{"web_url": "https://updated.example.com"})
	require.Equal(t, 200, resp.StatusCode)
	var proj map[string]any
	decodeJSON(t, resp, &proj)
	assert.Equal(t, "https://updated.example.com", proj["web_url"])
}

func TestProjectUpdateReadonlyField(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "pro", "pro@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	// huma rejects unknown fields with 422
	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/projects/%d", projID), token,
		map[string]string{"link_name": "hacked"})
	require.Equal(t, 422, resp.StatusCode)
}
