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

func TestWebhookCRUD(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whmaint", "wh@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	// create
	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks", projID), token,
		map[string]string{
			"url":    "http://hook.example.com",
			"secret": "s3cret",
			"events": "*",
		})
	require.Equal(t, 201, resp.StatusCode)
	var hook map[string]any
	decodeJSON(t, resp, &hook)
	assert.Equal(t, "http://hook.example.com", hook["url"])
	assert.NotContains(t, hook, "secret", "secret should not be in response")
	hookID := int(hook["id"].(float64))

	// list
	resp = s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks", projID), token, nil)
	require.Equal(t, 200, resp.StatusCode)
	var hooks []map[string]any
	decodeJSON(t, resp, &hooks)
	assert.Len(t, hooks, 1)

	// detail
	resp = s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/%d", projID, hookID), token, nil)
	require.Equal(t, 200, resp.StatusCode)

	// update
	resp = s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/%d", projID, hookID), token,
		map[string]any{"active": false})
	require.Equal(t, 200, resp.StatusCode)
	decodeJSON(t, resp, &hook)
	assert.Equal(t, false, hook["active"])

	// delete
	resp = s.authRequest(t, "DELETE",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/%d", projID, hookID), token, nil)
	require.Equal(t, 204, resp.StatusCode)

	// list should be empty
	resp = s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks", projID), token, nil)
	decodeJSON(t, resp, &hooks)
	assert.Len(t, hooks, 0)
}

func TestWebhookCreateInvalidEvents(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whie", "whie@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks", projID), token,
		map[string]string{"url": "http://x", "events": "invalid-event"})
	assert.Equal(t, 422, resp.StatusCode)
}

func TestWebhookCreateSpecificEvents(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whse", "whse@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks", projID), token,
		map[string]string{
			"url":    "http://x",
			"secret": "",
			"events": "patch-created,series-completed",
		})
	require.Equal(t, 201, resp.StatusCode)
	var hook map[string]any
	decodeJSON(t, resp, &hook)
	assert.Equal(t, "patch-created,series-completed", hook["events"])
}

func TestWebhookListAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)

	resp := s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks", projID), "", nil)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestWebhookListNonMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whnm", "whnm@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks", projID), token, nil)
	assert.Equal(t, 403, resp.StatusCode)
}

func TestWebhookSecretWriteOnly(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whsec", "whsec@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks", projID), token,
		map[string]string{"url": "http://x", "secret": "topsecret", "events": "*"})
	require.Equal(t, 201, resp.StatusCode)
	var hook map[string]any
	decodeJSON(t, resp, &hook)
	assert.NotContains(t, hook, "secret", "secret should not appear in response")

	hookID := int(hook["id"].(float64))
	resp = s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/%d", projID, hookID), token, nil)
	decodeJSON(t, resp, &hook)
	assert.NotContains(t, hook, "secret", "secret should not appear in GET response")
}
