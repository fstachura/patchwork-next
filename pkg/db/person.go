// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import "strings"

func (q *Queries) GetOrCreatePerson(email, name string) (*Person, error) {
	p := &Person{Email: strings.ToLower(email)}
	if name != "" {
		p.Name = &name
	}
	// Keep an existing non-empty name rather than letting a later
	// message (possibly spoofing this address) overwrite it. The name
	// is only filled in when the stored one is missing or empty.
	err := q.DB.NewInsert().Model(p).
		On("CONFLICT (email) DO UPDATE").
		Set("name = COALESCE(NULLIF(person.name, ''), EXCLUDED.name)").
		Returning("*").
		Scan(q.Ctx)
	return p, err
}
