// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const confirmationValidityDays = 7

func (c *EmailConfirmation) IsValid() bool {
	return c.Active && time.Since(c.Date) < confirmationValidityDays*24*time.Hour
}

func (q *Queries) CreateEmailConfirmation(confType, email string, userID *int) (*EmailConfirmation, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	key := hex.EncodeToString(buf)

	conf := &EmailConfirmation{
		Type:   confType,
		Email:  email,
		UserID: userID,
		Key:    key,
		Date:   time.Now(),
		Active: true,
	}
	err := q.Insert(conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func (q *Queries) CleanExpiredConfirmations() (int64, error) {
	cutoff := time.Now().Add(-confirmationValidityDays * 24 * time.Hour)
	res, err := q.Delete((*EmailConfirmation)(nil)).
		Where("date < ? OR active = ?", cutoff, false).
		Exec(q.Ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (q *Queries) CleanInactiveUsers() (int64, error) {
	res, err := q.Delete((*User)(nil)).
		Where("is_active = ?", false).
		Where(
			"id NOT IN (?)",
			q.Select((*EmailConfirmation)(nil)).
				Column("user_id").
				Where("user_id IS NOT NULL"),
		).
		Exec(q.Ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
