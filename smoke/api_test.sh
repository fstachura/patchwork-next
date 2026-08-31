#!/usr/bin/env bash
# Patchwork - automated patch tracking system
# Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
#
# SPDX-License-Identifier: GPL-2.0-or-later

# Exercise the HTTP REST API: unauthenticated reads, authenticated writes and
# the webhooks they trigger.

. "$(dirname "$0")/_init.sh"

MAIL="$REPO/pkg/mail/testdata/mail/0009-git-rename-with-diff.mbox"
SERIES="$REPO/pkg/mail/testdata/series/base-cover-letter.mbox"

jq_len() { printf '%s' "$HTTP_BODY" | jq 'length'; }

assert_nonempty() {
	if [ "$(jq_len)" -lt 1 ]; then
		fail "$1: expected a non-empty list"
	fi
}

db_init
pw admin project create -n "Smoke" -l smoke -i "$LISTID" -e list@example.com
pw admin user create -u admin -e admin@example.com --admin < <(set_password)
# Tokens cannot be created through the admin CLI, so inject one directly.
pw db import <<EOF
INSERT INTO auth_token (key, created, user_id)
SELECT '$TOKEN', datetime('now'), id
FROM auth_user WHERE username = 'admin';
EOF
pw admin webhook create --project smoke --user admin \
	--url "http://$SINK_ADDR/hook" --secret "$SECRET" --events '*'
sink_start

section "seed data via ingress"
pw ingress -l "$LISTID" -i < "$MAIL"
pw ingress -l "$LISTID" -m < "$SERIES"

http_start

section "API reads (no auth)"
for path in projects patches series events people; do
	info "GET /$path"
	api_get "$path"
	assert_http 200
	assert_nonempty "$path"
done

section "API reads (bearer token)"
info "GET /users requires authentication"
api_get users
assert_http 401
api_get users "$TOKEN"
assert_http 200
assert_nonempty users

section "API writes (bearer token)"
api_get patches
pid=$(printf '%s' "$HTTP_BODY" | jq -r '.[0].id')
info "patch id=$pid"

info "unauthenticated write is rejected"
api_post "patches/$pid/checks" "" '{"state":"success","context":"smoke/noauth"}'
assert_http 401

info "create a check"
api_post "patches/$pid/checks" "$TOKEN" \
	'{"state":"success","context":"smoke/ci","target_url":"http://ci.example.com","description":"ok"}'
assert_http 201

info "change the patch state"
api_patch "patches/$pid" "$TOKEN" '{"state":"under review"}'
assert_http 200
api_get "patches/$pid"
assert_http 200
assert_eq "Under Review" "$(printf '%s' "$HTTP_BODY" | jq -r '.state')" "patch state"

section "webhooks from API writes"
wait_webhook check-created 1 15
wait_webhook patch-state-changed 1 15
assert_webhook_signed
