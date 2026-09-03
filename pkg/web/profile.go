// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/events"
	"github.com/getpatchwork/patchwork/pkg/log"
)

type profileData struct {
	PC                  pageContext
	User                *db.User
	LinkedEmails        []linkedEmail
	Bundles             []db.Bundle
	Token               string
	MaintainerProjects  []db.Project
	ContributorProjects []db.Project
}

type linkedEmail struct {
	PersonID int
	Email    string
}

func (h *webHandler) ProfilePage(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)
	pc := h.pageCtx(r)

	// load linked emails
	var people []db.Person
	q.Select(&people).
		Where("user_id = ?", user.ID).
		OrderExpr("email ASC").
		Scan(ctx)

	var emails []linkedEmail
	for _, p := range people {
		emails = append(emails, linkedEmail{
			PersonID: p.ID,
			Email:    p.Email,
		})
	}

	// load bundles
	var bundles []db.Bundle
	q.Select(&bundles).
		Where("owner_id = ?", user.ID).
		OrderExpr("name ASC").
		Scan(ctx)
	for i := range bundles {
		bundles[i].Owner = user
	}

	// load API token
	var token string
	q.Select((*db.AuthToken)(nil)).Column("key").
		Where("user_id = ?", user.ID).
		Scan(ctx, &token)

	// load maintainer projects
	var maintainerProjects []db.Project
	q.Select(&maintainerProjects).
		Join("JOIN project_maintainer AS mp ON mp.project_id = project.id").
		Where("mp.user_id = ?", user.ID).
		OrderExpr("project.name ASC").
		Scan(ctx)

	// load contributor projects (projects where user submitted patches)
	var contributorProjects []db.Project
	q.Select(&contributorProjects).
		Distinct().
		Join("JOIN patch AS pa ON pa.project_id = project.id").
		Join("JOIN person AS pe ON pe.id = pa.submitter_id").
		Where("pe.user_id = ?", user.ID).
		OrderExpr("project.name ASC").
		Scan(ctx)

	data := profileData{
		PC:                  pc,
		User:                user,
		LinkedEmails:        emails,
		Bundles:             bundles,
		Token:               token,
		MaintainerProjects:  maintainerProjects,
		ContributorProjects: contributorProjects,
	}
	profilePage(data).Render(ctx, w)
}

func (h *webHandler) ProfileUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)

	if !h.validateCSRF(r) {
		http.Redirect(w, r, "/user", http.StatusFound)
		return
	}

	r.ParseForm()
	itemsPerPage, _ := strconv.Atoi(r.FormValue("items_per_page"))
	if itemsPerPage < 1 {
		itemsPerPage = 100
	}
	showIds := r.FormValue("show_ids") == "on"

	_, _ = q.Update((*db.User)(nil)).
		Where("id = ?", user.ID).
		Set("items_per_page = ?", itemsPerPage).
		Set("show_ids = ?", showIds).
		Exec(ctx)

	http.Redirect(w, r, "/user", http.StatusFound)
}

func (h *webHandler) LinkEmail(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)
	pc := h.pageCtx(r)

	if r.Method == "GET" {
		linkEmailPage(pc, "").Render(ctx, w)
		return
	}

	if !h.validateCSRF(r) {
		linkEmailPage(pc, "Invalid request.").Render(ctx, w)
		return
	}

	r.ParseForm()
	email := r.FormValue("email")
	if email == "" {
		linkEmailPage(pc, "Email is required.").Render(ctx, w)
		return
	}

	conf, err := q.CreateEmailConfirmation("userperson", email, &user.ID)
	if err != nil {
		log.Errorf("link email: %s", err)
		linkEmailPage(pc, "Failed to create confirmation.").Render(ctx, w)
		return
	}

	baseURL := h.cfg.Http.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}
	link := fmt.Sprintf("%s/confirm/%s", baseURL, conf.Key)
	body := fmt.Sprintf(
		"Please click the following link to link this email to your Patchwork account:\n\n%s\n",
		link,
	)
	go func() {
		err = events.SendEmail(&h.cfg.SMTP, email, "Patchwork email confirmation", body, nil)
		if err != nil {
			log.Errorf("link email: send: %s", err)
		}
	}()

	confirmResultPage(pc, fmt.Sprintf("A confirmation email has been sent to %s.", email)).
		Render(ctx, w)
}

func (h *webHandler) UnlinkEmail(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)

	if !h.validateCSRF(r) {
		http.Redirect(w, r, "/user", http.StatusFound)
		return
	}

	personID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		http.Redirect(w, r, "/user", http.StatusFound)
		return
	}

	var person db.Person
	err = q.Select(&person).
		Where("id = ?", personID).
		Where("user_id = ?", user.ID).
		Scan(ctx)
	if err != nil {
		http.Redirect(w, r, "/user", http.StatusFound)
		return
	}

	if person.Email == user.Email {
		http.Redirect(w, r, "/user", http.StatusFound)
		return
	}

	_, _ = q.Update(&person).
		Where("id = ?", person.ID).
		Set("user_id = NULL").
		Exec(ctx)

	http.Redirect(w, r, "/user", http.StatusFound)
}

func (h *webHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)
	pc := h.pageCtx(r)

	if r.Method == "GET" {
		changePasswordPage(pc, "").Render(ctx, w)
		return
	}

	if !h.validateCSRF(r) {
		changePasswordPage(pc, "Invalid request.").Render(ctx, w)
		return
	}

	r.ParseForm()
	oldPassword := r.FormValue("old_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if !db.CheckPassword(oldPassword, user.Password) {
		changePasswordPage(pc, "Current password is incorrect.").Render(ctx, w)
		return
	}
	if newPassword == "" {
		changePasswordPage(pc, "New password is required.").Render(ctx, w)
		return
	}
	if newPassword != confirmPassword {
		changePasswordPage(pc, "New passwords do not match.").Render(ctx, w)
		return
	}

	_, _ = q.Update((*db.User)(nil)).
		Where("id = ?", user.ID).
		Set("password = ?", db.HashPassword(newPassword)).
		Exec(ctx)

	// Invalidate every existing session so that a stolen cookie cannot
	// outlive a password change, then issue a fresh session for the
	// device performing the change.
	_ = q.DeleteUserSessions(user.ID)
	if sessionKey, err := q.CreateSession(user.ID); err == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     "sessionid",
			Value:    sessionKey,
			Path:     "/",
			HttpOnly: true,
			Secure:   h.secureCookies(),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   14 * 24 * 60 * 60,
		})
	}

	http.Redirect(w, r, "/user", http.StatusFound)
}

func (h *webHandler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)

	if !h.validateCSRF(r) {
		http.Redirect(w, r, "/user", http.StatusFound)
		return
	}

	if _, err := q.Delete((*db.AuthToken)(nil)).
		Where("user_id = ?", user.ID).Exec(q.Ctx); err != nil {
		serverErrorPage(w, "delete old token", err)
		return
	}

	key := make([]byte, 20)
	if _, err := rand.Read(key); err != nil {
		serverErrorPage(w, "generate token", err)
		return
	}
	token := hex.EncodeToString(key)

	if err := q.Insert(&db.AuthToken{
		Key: token, Created: time.Now(), UserID: user.ID,
	}); err != nil {
		serverErrorPage(w, "create token", err)
		return
	}

	http.Redirect(w, r, "/user", http.StatusFound)
}

func (h *webHandler) PasswordReset(w http.ResponseWriter, r *http.Request) {
	pc := h.pageCtx(r)

	if r.Method == "GET" {
		passwordResetPage(pc, "").Render(r.Context(), w)
		return
	}

	if !h.validateCSRF(r) {
		passwordResetPage(pc, "Invalid request.").Render(r.Context(), w)
		return
	}

	ctx := r.Context()
	q := db.GetQueries(ctx)
	r.ParseForm()
	email := r.FormValue("email")

	var user db.User
	err := q.Select(&user).
		Where("LOWER(email) = LOWER(?)", email).
		Where("is_active = ?", true).
		Scan(ctx)
	if err != nil {
		// don't reveal whether user exists
		confirmResultPage(pc, "If an account exists with that email, a reset link has been sent.").
			Render(ctx, w)
		return
	}

	conf, err := q.CreateEmailConfirmation("password_reset", email, &user.ID)
	if err != nil {
		log.Errorf("password reset: %s", err)
		confirmResultPage(pc, "If an account exists with that email, a reset link has been sent.").
			Render(ctx, w)
		return
	}

	baseURL := h.cfg.Http.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}
	link := fmt.Sprintf("%s/password-reset/%s", baseURL, conf.Key)
	body := fmt.Sprintf(
		"Please click the following link to reset your password:\n\n%s\n\nThis link will expire in 7 days.\n",
		link,
	)

	go func() {
		err = events.SendEmail(&h.cfg.SMTP, email, "Patchwork password reset", body, nil)
		if err != nil {
			log.Errorf("reset email: send: %s", err)
		}
	}()

	confirmResultPage(pc, "If an account exists with that email, a reset link has been sent.").
		Render(ctx, w)
}

func (h *webHandler) PasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	pc := h.pageCtx(r)
	key := urlParam(r, "key")

	var conf db.EmailConfirmation
	err := q.Select(&conf).
		Where("key = ?", key).
		Where("type = ?", "password_reset").
		Scan(ctx)
	if err != nil || !conf.IsValid() {
		confirmResultPage(pc, "Invalid or expired reset link.").Render(ctx, w)
		return
	}

	if r.Method == "GET" {
		passwordResetConfirmPage(pc, key, "").Render(ctx, w)
		return
	}

	if !h.validateCSRF(r) {
		passwordResetConfirmPage(pc, key, "Invalid request.").Render(ctx, w)
		return
	}

	r.ParseForm()
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	if newPassword == "" {
		passwordResetConfirmPage(pc, key, "Password is required.").Render(ctx, w)
		return
	}
	if newPassword != confirmPassword {
		passwordResetConfirmPage(pc, key, "Passwords do not match.").Render(ctx, w)
		return
	}

	_, _ = q.Update((*db.User)(nil)).
		Where("id = ?", *conf.UserID).
		Set("password = ?", db.HashPassword(newPassword)).
		Exec(ctx)

	// Drop any existing sessions so a reset also locks out whoever may
	// currently hold a session for this account.
	_ = q.DeleteUserSessions(*conf.UserID)

	_, _ = q.Update(&conf).
		Where("id = ?", conf.ID).
		Set("active = ?", false).
		Exec(ctx)

	confirmResultPage(pc, "Your password has been reset. You can now log in.").Render(ctx, w)
}
