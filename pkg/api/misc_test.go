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

func TestEmptyListsReturnArray(t *testing.T) {
	s := newTestServer(t)
	for _, ep := range []string{
		"/api/1.4/patches", "/api/1.4/covers", "/api/1.4/series",
		"/api/1.4/projects", "/api/1.4/people", "/api/1.4/events",
		"/api/1.4/bundles",
	} {
		t.Run(ep, func(t *testing.T) {
			resp := s.get(t, ep)
			assert.Equal(t, 200, resp.StatusCode)
			var items []map[string]any
			decodeJSON(t, resp, &items)
			assert.NotNil(t, items, "expected [], got null")
		})
	}
}

func TestIndexEndpoint(t *testing.T) {
	s := newTestServer(t)
	index := getOne(t, s, "/api/1.4")
	for _, key := range []string{"patches", "covers", "series", "projects", "people", "events", "bundles"} {
		assertField(t, index, key)
	}
}

func TestIndexVersionPrefix(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4")
	require.Equal(t, 200, resp.StatusCode)
}

func TestPagination(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	for i := 0; i < 5; i++ {
		s.insertPatch(t, projID,
			fmt.Sprintf("<page-%d@test>", i),
			fmt.Sprintf("patch %d", i))
	}

	resp := s.get(t, "/api/1.4/patches/?per_page=2&page=1")
	require.Equal(t, 200, resp.StatusCode)

	link := resp.Header.Get("Link")
	assert.NotEmpty(t, link, "missing Link header")

	var patches []map[string]any
	decodeJSON(t, resp, &patches)
	assert.Len(t, patches, 2)

	// page 2
	resp = s.get(t, "/api/1.4/patches/?per_page=2&page=2")
	decodeJSON(t, resp, &patches)
	assert.Len(t, patches, 2)

	// page 3 (last)
	resp = s.get(t, "/api/1.4/patches/?per_page=2&page=3")
	decodeJSON(t, resp, &patches)
	assert.Len(t, patches, 1)
}

func TestReadWithoutAuth(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<noauth@test>", "public patch")

	for _, path := range []string{
		"/api/1.4/projects",
		"/api/1.4/patches",
		"/api/1.4/covers",
		"/api/1.4/series",
		"/api/1.4/events",
	} {
		resp := s.get(t, path)
		assert.Equal(t, 200, resp.StatusCode, path)
		resp.Body.Close()
	}
}
