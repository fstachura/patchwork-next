#!/usr/bin/env bash
# Patchwork - automated patch tracking system
# Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
#
# SPDX-License-Identifier: GPL-2.0-or-later

# Exercise the three ingress modes (single message, mbox stream and the SMTP
# daemon driven by git send-email) and verify that webhooks are delivered.

. "$(dirname "$0")/_init.sh"

MAIL="$REPO/pkg/mail/testdata/mail/0009-git-rename-with-diff.mbox"
SERIES="$REPO/pkg/mail/testdata/series/base-cover-letter.mbox"

db_init
pw admin project create -n "Smoke" -l smoke -i "$LISTID" -e list@example.com
pw admin user create -u dev -e dev@example.com < <(set_password)
pw admin webhook create --project smoke --user dev \
	--url "http://$SINK_ADDR/hook" --secret "$SECRET" --events '*'
sink_start

section "ingress -i (single message)"
pw ingress -l "$LISTID" -i < "$MAIL"
assert_eq 1 "$(db_count patch)" "patches after single message"

section "ingress -m (mbox series)"
before=$(db_count patch)
pw ingress -l "$LISTID" -m < "$SERIES"
if [ "$(db_count patch)" -le "$before" ]; then
	fail "patch count did not grow after mbox ingest"
fi
if [ "$(db_count series)" -lt 1 ]; then
	fail "no series created"
fi
if [ "$(db_count cover)" -lt 1 ]; then
	fail "no cover letter created"
fi

section "webhooks from ingest"
wait_webhook patch-created 1 15
wait_webhook series-created 1 15
assert_webhook_signed

section "ingress SMTP daemon (git send-email)"
ingress_start
before=$(webhook_count patch-created)
patches=$(db_count patch)
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME="Random Developer"
export GIT_AUTHOR_EMAIL="dev@example.com"
export GIT_COMMITTER_NAME="$GIT_AUTHOR_NAME"
export GIT_COMMITTER_EMAIL="$GIT_AUTHOR_EMAIL"
xtrace git send-email \
	--smtp-server=127.0.0.1 \
	--smtp-server-port="${INGRESS_ADDR##*:}" \
	--confirm=never --no-annotate --suppress-cc=all \
	--to=list@example.com --from=dev@example.com \
	--8bit-encoding=UTF-8 --subject-prefix="PATCH pw-next" -1

wait_webhook patch-created "$((before + 1))" 20
if [ "$(db_count patch)" -le "$patches" ]; then
	fail "SMTP ingest did not create a new patch"
fi
assert_webhook_signed
