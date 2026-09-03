// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"

	"github.com/getpatchwork/patchwork/pkg/db"
)

type contextKey int

const webUserKey contextKey = iota

// dummyPasswordHash is compared against when the submitted username has
// no account, so the login handler spends roughly the same time as it
// would verifying a real password. This keeps failed logins from
// leaking whether a username exists. It is computed lazily on the first
// login so the expensive hash does not slow down unrelated commands that
// merely link in this package.
var dummyPasswordHash = sync.OnceValue(func() string {
	return db.HashPassword("patchwork-login-timing-guard")
})

func getWebUser(r *http.Request) *db.User {
	u, _ := r.Context().Value(webUserKey).(*db.User)
	return u
}

// secureCookies reports whether session cookies must carry the Secure
// attribute. It is derived from the configured base URL so that HTTPS
// deployments (including those terminating TLS at a reverse proxy) get
// Secure cookies while plain-HTTP setups keep working.
func (h *webHandler) secureCookies() bool {
	return strings.HasPrefix(h.cfg.Http.BaseURL, "https://")
}

// safeRedirectPath sanitizes a user-supplied "next" destination so it
// can only point back to this site. Anything that is not a plain local
// path (absolute URLs, protocol-relative "//host" or "/\host" forms) is
// rejected in favor of the site root, preventing open redirects.
func safeRedirectPath(next string) string {
	if next == "" || next[0] != '/' {
		return "/"
	}
	if len(next) > 1 && (next[1] == '/' || next[1] == '\\') {
		return "/"
	}
	return next
}

func (h *webHandler) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("sessionid")
		if err == nil && cookie.Value != "" {
			user, err := db.New(r.Context(), h.db).GetSessionUser(cookie.Value)
			if err == nil && user != nil {
				ctx := r.Context()
				ctx = context.WithValue(ctx, webUserKey, user)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *webHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	next := safeRedirectPath(r.URL.Query().Get("next"))
	pc := h.pageCtx(r)
	loginPage(pc, "", next).Render(r.Context(), w)
}

func (h *webHandler) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.FormValue("username")
	password := r.FormValue("password")
	next := safeRedirectPath(r.FormValue("next"))
	pc := h.pageCtx(r)

	if !h.validateCSRF(r) {
		loginPage(pc, "Invalid request. Please try again.", next).Render(r.Context(), w)
		return
	}

	q := db.GetQueries(r.Context())
	user, err := q.GetUserByUsername(username)
	var hash string
	if user != nil {
		hash = user.Password
	} else {
		// Always run a password check, even for an unknown user, so
		// that a missing account takes about as long as a wrong
		// password and cannot be distinguished by timing (username
		// enumeration).
		hash = dummyPasswordHash()
	}
	if !db.CheckPassword(password, hash) || err != nil {
		w.WriteHeader(http.StatusForbidden)
		loginPage(pc, "Invalid username or password.", next).Render(r.Context(), w)
		return
	}

	sessionKey, err := db.New(r.Context(), h.db).CreateSession(user.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		loginPage(pc, "Internal error. Please try again.", next).Render(r.Context(), w)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "sessionid",
		Value:    sessionKey,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   14 * 24 * 60 * 60,
	})

	http.Redirect(w, r, next, http.StatusFound)
}

func (h *webHandler) LogoutSubmit(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("sessionid")
	if err == nil && cookie.Value != "" {
		_ = db.New(r.Context(), h.db).DeleteSession(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "sessionid",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookies(),
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *webHandler) csrfToken(r *http.Request) string {
	cookie, err := r.Cookie("sessionid")
	seed := "anonymous"
	if err == nil && cookie.Value != "" {
		seed = cookie.Value
	}
	mac := hmac.New(sha256.New, h.csrfKey)
	mac.Write([]byte(seed))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *webHandler) validateCSRF(r *http.Request) bool {
	token := r.FormValue("csrftoken")
	expected := h.csrfToken(r)
	return hmac.Equal([]byte(token), []byte(expected))
}

func requireLogin(w http.ResponseWriter, r *http.Request) bool {
	if getWebUser(r) == nil {
		http.Redirect(w, r, "/user/login?next="+r.URL.Path, http.StatusFound)
		return false
	}
	return true
}
