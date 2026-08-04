// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeopleList(t *testing.T) {
	s := newTestServer(t)
	s.insertPerson(t, "person@test.com", "Test Person")
	items := getList(t, s, "/api/1.4/people")
	require.Len(t, items, 1)
	p := items[0]
	assertField(t, p, "id")
	assertField(t, p, "name")
	assertField(t, p, "email")
	assert.Equal(t, "person@test.com", p["email"])
}

func TestPeopleListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/people")
	assert.Len(t, items, 0)
}

func TestPeopleSearch(t *testing.T) {
	s := newTestServer(t)
	s.insertPerson(t, "alice@test", "Alice")
	s.insertPerson(t, "bob@test", "Bob")

	items := getList(t, s, "/api/1.4/people/?q=Alice")
	assert.Len(t, items, 1)
}

func TestPersonCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/people", "", map[string]string{"name": "x"})
	assert.Equal(t, 405, resp.StatusCode)
}

func TestPersonDetail(t *testing.T) {
	s := newTestServer(t)
	id := s.insertPerson(t, "detail@test", "Detail Person")
	p := getOne(t, s, fmt.Sprintf("/api/1.4/people/%d", id))
	assert.Equal(t, "Detail Person", p["name"])
}

func TestPersonDetailAnonymous(t *testing.T) {
	s := newTestServer(t)
	s.insertPerson(t, "anon@test", "Anon")
	resp := s.get(t, "/api/1.4/people")
	assert.Equal(t, 200, resp.StatusCode)
}

func TestPersonDetailInvalid(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/people/invalid")
	assert.Equal(t, 422, resp.StatusCode)
}

func TestPersonDetailLinked(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "linked", "linked@test")
	s.exec(t, `INSERT INTO person (email, name, user_id)
		VALUES ('linked@test', 'Linked Person', ?)`, userID)
	var personID int
	s.db.NewRaw(`SELECT id FROM person WHERE email = 'linked@test'`).
		Scan(context.Background(), &personID)

	p := getOne(t, s, fmt.Sprintf("/api/1.4/people/%d", personID))
	assert.Equal(t, "linked@test", p["email"])
}

func TestPersonNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/people/99999")
	require.Equal(t, 404, resp.StatusCode)
}

func TestPersonUserLinked(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "linked2", "linked2@test")
	s.exec(t, `INSERT INTO person (email, name, user_id)
		VALUES ('linked2@test', 'Linked', ?)`, userID)
	var personID int
	s.db.NewRaw(`SELECT id FROM person WHERE email = 'linked2@test'`).
		Scan(context.Background(), &personID)

	p := getOne(t, s, fmt.Sprintf("/api/1.4/people/%d", personID))
	assertNested(t, p, "user", "id")
	assertNested(t, p, "user", "username")
}
