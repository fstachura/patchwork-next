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

func TestSeriesCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/series", "", map[string]string{"name": "x"})
	assert.Equal(t, 405, resp.StatusCode)
}

func TestSeriesDelete405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "DELETE", "/api/1.4/series/1", "", nil)
	assert.Equal(t, 405, resp.StatusCode)
}

func TestSeriesDependencies(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "dep@test", "Dep")
	s1 := s.insertSeries(t, projID, personID, "base")
	s2 := s.insertSeries(t, projID, personID, "dependent")
	s.exec(t, `INSERT INTO series_dependencies (from_series_id, to_series_id)
		VALUES (?, ?)`, s2, s1)

	sr := getOne(t, s, fmt.Sprintf("/api/1.4/series/%d", s2))
	deps, ok := sr["dependencies"].([]any)
	require.True(t, ok, "dependencies should be an array")
	assert.Len(t, deps, 1)

	// check dependents on the base series
	base := getOne(t, s, fmt.Sprintf("/api/1.4/series/%d", s1))
	dependents, ok := base["dependents"].([]any)
	require.True(t, ok, "dependents should be an array")
	assert.Len(t, dependents, 1)
}

func TestSeriesDetail(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "sd@test", "SD")
	seriesID := s.insertSeries(t, projID, personID, "detail series")
	sr := getOne(t, s, fmt.Sprintf("/api/1.4/series/%d", seriesID))
	assert.Equal(t, "detail series", sr["name"])
	assertField(t, sr, "version")
	assertField(t, sr, "total")
	assertNested(t, sr, "submitter", "id")
	assertNested(t, sr, "project", "id")
}

func TestSeriesDetailInvalid(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/series/invalid")
	assert.Equal(t, 422, resp.StatusCode)
}

func TestSeriesEmptyMetadata(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "sem@test", "SEM")
	seriesID := s.insertSeries(t, projID, personID, "no meta")

	sr := getOne(t, s, fmt.Sprintf("/api/1.4/series/%d", seriesID))
	meta, ok := sr["metadata"].(map[string]any)
	require.True(t, ok, "metadata should be an object even when empty")
	assert.Len(t, meta, 0)
}

func TestSeriesFilterMetadata(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "mf@test", "MF")
	seriesID := s.insertSeries(t, projID, personID, "meta")
	s.exec(t, `INSERT INTO series_metadata (series_id, key, value)
		VALUES (?, 'github', 'owner/repo')`, seriesID)

	items := getList(t, s, "/api/1.4/series/?metadata_key=github")
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/series/?metadata_value=owner/repo")
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/series/?metadata_key=nonexistent")
	assert.Len(t, items, 0)
}

func TestSeriesFilterProject(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "sf@test", "SF")
	s.insertSeries(t, projID, personID, "filtered")

	items := getList(t, s, fmt.Sprintf("/api/1.4/series/?project=%d", projID))
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/series/?project=99999")
	assert.Len(t, items, 0)
}

func TestSeriesFilterSubmitter(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "ssf@test", "SSF")
	s.insertSeries(t, projID, personID, "s")

	items := getList(t, s, fmt.Sprintf("/api/1.4/series/?submitter=%d", personID))
	assert.Len(t, items, 1)
	items = getList(t, s, "/api/1.4/series/?submitter=99999")
	assert.Len(t, items, 0)
}

func TestSeriesList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "s@test", "Submitter")
	seriesID := s.insertSeries(t, projID, personID, "test series")
	items := getList(t, s, "/api/1.4/series")
	require.Len(t, items, 1)
	sr := items[0]
	assertValue(t, sr, "id", float64(seriesID))
	assertValue(t, sr, "name", "test series")
	assertValue(t, sr, "version", float64(1))
	assertValue(t, sr, "total", float64(2))
	assertValue(t, sr, "received_total", float64(0))
	assertValue(t, sr, "received_all", false)
	assertContains(t, sr, "url", fmt.Sprintf("/series/%d", seriesID))
	assertContains(t, sr, "mbox", fmt.Sprintf("/series/%d/mbox", seriesID))
	assertField(t, sr, "date")
	assertField(t, sr, "patches")
	assertField(t, sr, "metadata")
	assertField(t, sr, "dependencies")
	assertField(t, sr, "dependents")
	assertNested(t, sr, "submitter", "id")
	assertNested(t, sr, "project", "id")
}

func TestSeriesListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/series")
	assert.Len(t, items, 0)
}

func TestSeriesMetadata(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "sm@test", "SM")
	seriesID := s.insertSeries(t, projID, personID, "meta series")

	s.exec(t, `INSERT INTO series_metadata (series_id, key, value)
		VALUES (?, 'github', 'owner/repo#42')`, seriesID)
	s.exec(t, `INSERT INTO series_metadata (series_id, key, value)
		VALUES (?, 'ci', 'passed')`, seriesID)

	sr := getOne(t, s, fmt.Sprintf("/api/1.4/series/%d", seriesID))
	assertField(t, sr, "metadata")

	meta, ok := sr["metadata"].(map[string]any)
	require.True(t, ok, "metadata should be an object")
	assert.Equal(t, "owner/repo#42", meta["github"])
	assert.Equal(t, "passed", meta["ci"])
}

func TestSeriesNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/series/99999")
	assert.Equal(t, 404, resp.StatusCode)
}

func TestSeriesReceivedTotal(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "rt@test", "RT")
	seriesID := s.insertSeries(t, projID, personID, "recv") // total=2
	patchID := s.insertPatch(t, projID, "<rt1@test>", "p1")
	s.exec(t, `UPDATE patch SET series_id = ?, number = 1 WHERE id = ?`,
		seriesID, patchID)

	sr := getOne(t, s, fmt.Sprintf("/api/1.4/series/%d", seriesID))
	assert.Equal(t, float64(1), sr["received_total"])
	assert.Equal(t, false, sr["received_all"])
}

func TestSeriesUpdateMetadata(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "su@test", "SU")
	seriesID := s.insertSeries(t, projID, personID, "updatable")
	userID := s.insertUser(t, "smaint", "sm@test")
	s.makeMaintainer(t, userID, projID)
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/series/%d", seriesID), token,
		map[string]any{"metadata": map[string]string{"key": "value"}})
	require.Equal(t, 200, resp.StatusCode)
	var sr map[string]any
	decodeJSON(t, resp, &sr)
	meta, ok := sr["metadata"].(map[string]any)
	require.True(t, ok, "metadata not an object")
	assert.Equal(t, "value", meta["key"])
}
