#!/usr/bin/env bash
# Patchwork - automated patch tracking system
# Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
#
# SPDX-License-Identifier: GPL-2.0-or-later

# Exercise every "pw admin" subcommand plus "pw config" and "pw db" against a
# throwaway SQLite database.

. "$(dirname "$0")/_init.sh"

last_id() { sed -n 's/.*id=\([0-9]*\).*/\1/p' <<<"$1" | tail -1; }

db_init

section "config"
info "config print"
pw config print >"$WORKDIR/config.out"
assert_contains "$(cat "$WORKDIR/config.out")" "[database]" "config print output"
info "config url"
out=$(printf 'sqlite\nsmoke.db\n' | pw config url 2>/dev/null)
assert_contains "$out" 'sqlite://smoke.db' "config url output"

section "db"
info "db sync is idempotent"
pw db sync
info "db export refuses a non-Django database"
if pw db export >"$WORKDIR/export.out" 2>&1; then
	fail "pw db export unexpectedly succeeded on a native database"
fi
assert_contains "$(cat "$WORKDIR/export.out")" "Django" "db export error"

section "projects"
pw admin project create -n "Smoke" -l smoke -i "$LISTID" -e list@example.com --subject-match '\[PATCH'
pw admin project create -n "Other" -l other -i other.lists.example.com -e other@example.com
out=$(pw admin project list)
assert_contains "$out" "smoke" "project list"
assert_contains "$out" "other" "project list"
pw admin project show smoke >/dev/null
pw admin project update smoke --web-url https://smoke.example.com
pw admin project create -n "Throwaway" -l throwaway -i tw.lists.example.com -e tw@example.com
pw admin project delete -f throwaway

section "users"
pw admin user create -u admin -e admin@example.com --admin < <(set_password)
pw admin user create -u maint -e maint@example.com < <(set_password)
pw admin user create -u dev -e dev@example.com < <(set_password)
out=$(pw admin user list)
assert_contains "$out" "admin" "user list"
assert_contains "$out" "maint" "user list"
pw admin user passwd dev < <(set_password)
pw admin user create -u throwaway -e throwaway@example.com < <(set_password)
pw admin user delete -f throwaway

info "db import loads SQL from stdin"
printf "INSERT INTO auth_token (key, created, user_id) SELECT 'admintoken', datetime('now'), id FROM auth_user WHERE username='admin';\n" |
	pw db import
assert_eq 1 "$(db_count auth_token)" "auth_token rows"

section "tags"
pw admin tag create -n "Smoke-by" -p '^Smoke-by:' -a SM
assert_contains "$(pw admin tag list)" "Smoke-by" "tag list"
pw admin tag delete -f "Smoke-by"

section "states"
pw admin state create -n "Needs ACK" -s needs-ack -o 42 --action-required
assert_contains "$(pw admin state list)" "needs-ack" "state list"
pw admin state delete -f needs-ack

section "maintainers"
pw admin maintainer add smoke maint
assert_contains "$(pw admin maintainer list smoke)" "maint" "maintainer list"
pw admin maintainer add smoke dev
pw admin maintainer remove smoke dev

section "delegate rules"
out=$(pw admin delegate-rule create --project smoke --user maint --path 'drivers/*' --priority 5)
rule_id=$(last_id "$out")
assert_contains "$(pw admin delegate-rule list smoke)" "drivers/*" "delegate-rule list"
pw admin delegate-rule delete -f "$rule_id"

section "webhooks"
out=$(pw admin webhook create --project smoke --user admin \
	--url "http://$SINK_ADDR/hook" --secret "$SECRET" --events '*')
hook_id=$(last_id "$out")
assert_contains "$(pw admin webhook list smoke)" "$SINK_ADDR" "webhook list"
pw admin webhook update "$hook_id" --events patch-created,check-created
out=$(pw admin webhook create --project smoke --user admin --url "http://$SINK_ADDR/tw" --no-active)
tw_id=$(last_id "$out")
pw admin webhook delete -f "$tw_id"

section "gc"
pw admin gc
