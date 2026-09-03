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

const sessionMaxAge = 14 * 24 * time.Hour // 2 weeks

func (q *Queries) CreateSession(userID int) (string, error) {
	key := make([]byte, 20)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	sessionKey := hex.EncodeToString(key)

	session := Session{
		SessionKey: sessionKey,
		UserID:     userID,
		ExpireDate: time.Now().Add(sessionMaxAge),
	}
	err := q.Insert(&session)
	if err != nil {
		return "", err
	}
	return sessionKey, nil
}

func (q *Queries) GetSessionUser(sessionKey string) (*User, error) {
	var session Session
	err := q.Select(&session).
		Where("session_key = ?", sessionKey).
		Where("expire_date > ?", time.Now()).
		Scan(q.Ctx)
	if err != nil {
		return nil, err
	}

	var user User
	err = q.Select(&user).
		Where("id = ?", session.UserID).
		Where("is_active = ?", true).
		Scan(q.Ctx)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (q *Queries) DeleteSession(sessionKey string) error {
	_, err := q.Delete((*Session)(nil)).
		Where("session_key = ?", sessionKey).
		Exec(q.Ctx)
	return err
}

func (q *Queries) DeleteUserSessions(userID int) error {
	_, err := q.Delete((*Session)(nil)).
		Where("user_id = ?", userID).
		Exec(q.Ctx)
	return err
}

func (q *Queries) CleanExpiredSessions() (int64, error) {
	res, err := q.Delete((*Session)(nil)).
		Where("expire_date <= ?", time.Now()).
		Exec(q.Ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
