# Patchwork - automated patch tracking system
# Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
#
# SPDX-License-Identifier: GPL-2.0-or-later

# Shared library sourced by every smoke test. It re-executes the test inside a
# rootless network namespace, exposes reusable helpers (logging, assertions,
# service lifecycle, webhook verification) and installs a cleanup trap. It is
# not meant to be executed on its own.

# Re-exec the test into a private user+network namespace so that the ingress,
# http and webhook-sink daemons bind fixed loopback ports without clashing with
# the host or a concurrent run. Fall back to a non-isolated run when
# unprivileged user namespaces are unavailable.
if [ -z "${SMOKE_UNSHARED:-}" ]; then
	if unshare --user --map-root-user --net true 2>/dev/null; then
		export SMOKE_UNSHARED=1
		exec unshare --user --map-root-user --net -- "$0" "$@"
	else
		export SMOKE_UNSHARED=0
		echo "warning: unprivileged user namespaces unavailable," \
			"running without network isolation" >&2
	fi
fi

set -e -o pipefail
trap '' PIPE

if [ "${SMOKE_UNSHARED:-0}" = 1 ]; then
	ip link set lo up
fi

REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
export PATH="$REPO:$PATH"

NAME=$(basename "$0" .sh)
: "${WORKDIR:=$REPO/.testdata/$NAME}"
: "${DB:=$WORKDIR/smoke.db}"
: "${INGRESS_ADDR:=127.0.0.1:2525}"
: "${HTTP_ADDR:=127.0.0.1:8080}"
: "${SINK_ADDR:=127.0.0.1:9099}"
: "${API:=http://$HTTP_ADDR/api/1.4}"
: "${LISTID:=smoke.lists.example.com}"
: "${SECRET:=smoke-secret}"
: "${TOKEN:=0123456789abcdef0123456789abcdef01234567}"
: "${PASSWORD:=smoke-pass}"

PIDS=()
HTTP_CODE=""
HTTP_BODY=""

if [ -t 1 ]; then
	C_SECTION='\033[1;35m'
	C_INFO='\033[32m'
	C_LOG='\033[34m'
	C_TRACE='\033[36m'
	C_OK='\033[1;32m'
	C_ERR='\033[1;31m'
	C_OFF='\033[0m'
else
	C_SECTION=""
	C_INFO=""
	C_LOG=""
	C_TRACE=""
	C_OK=""
	C_ERR=""
	C_OFF=""
fi

section() { printf '%b==> %s%b\n' "$C_SECTION" "$*" "$C_OFF"; }
info() { printf '%b--- %s%b\n' "$C_INFO" "$*" "$C_OFF"; }
ok() { printf '%bok: %s%b\n' "$C_OK" "$*" "$C_OFF"; }
fail() { printf '%bfail: %s%b\n' "$C_ERR" "$*" "$C_OFF" >&2; return 1; }

xtrace() {
	printf '%b+ %s%b\n' "$C_TRACE" "$*" "$C_OFF" >&2
	command "$@"
}

assert_eq() {
	if [ "$1" != "$2" ]; then
		fail "${3:-assertion}: expected '$1', got '$2'"
	fi
}

assert_contains() {
	case "$1" in
	*"$2"*) ;;
	*) fail "${3:-assertion}: '$2' not found" ;;
	esac
}

assert_http() {
	assert_eq "$1" "$HTTP_CODE" "HTTP status"
}

# Kill a process and all of its descendants, children before parents, so that
# "go run" wrappers take their compiled child down with them.
kill_tree() {
	local pid="$1" sig="${2:-TERM}" child
	for child in $(pgrep -P "$pid" 2>/dev/null); do
		kill_tree "$child" "$sig"
	done
	kill "-$sig" "$pid" 2>/dev/null || true
}

# Terminate a process tree, escalating to SIGKILL after a grace period, and
# reap the leader.
kill_wait() {
	local pid="$1" secs="${2:-10}" i=0
	kill -0 "$pid" 2>/dev/null || return 0
	kill_tree "$pid" TERM
	while kill -0 "$pid" 2>/dev/null; do
		if [ "$i" -ge "$((secs * 10))" ]; then
			kill_tree "$pid" KILL
			break
		fi
		sleep 0.1
		i=$((i + 1))
	done
	wait "$pid" 2>/dev/null || true
}

# Wait until something is listening on the given TCP port.
wait_port() {
	local port="$1" timeout="${2:-10}"
	SECONDS=0
	while ! ss -tlnH sport = ":$port" | grep -q .; do
		if [ "$SECONDS" -gt "$timeout" ]; then
			fail "nothing listening on port $port after ${timeout}s"
		fi
		sleep 0.1
	done
}

require_tools() {
	local tool missing=0
	for tool in pw go git jq curl ss sqlite3; do
		if ! command -v "$tool" >/dev/null 2>&1; then
			echo "error: required tool not found: $tool" >&2
			missing=1
		fi
	done
	if ! git send-email --help >/dev/null 2>&1; then
		echo "error: git send-email is not available" >&2
		missing=1
	fi
	if [ "$missing" -ne 0 ]; then
		fail "missing required tools"
	fi
}

# Wipe the per-test work directory, write its config and create the schema.
db_init() {
	require_tools
	rm -rf "$WORKDIR"
	mkdir -p "$WORKDIR"
	cat >"$WORKDIR/patchwork.toml" <<-EOF
		[database]
		url = "sqlite://$DB"
		auto-sync = true

		[ingress]
		listen = "$INGRESS_ADDR"

		[http]
		listen = "$HTTP_ADDR"

		[webhook]
		blocked-cidrs = ["169.254.169.254/32"]
	EOF
	export PATCHWORK_TOML="$WORKDIR/patchwork.toml"
	pw db sync
}

db_count() {
	sqlite3 -init /dev/null "$DB" "SELECT count(*) FROM $1;"
}

pw() {
	xtrace pw "$@"
}

set_password() {
	printf '%s\n' "$PASSWORD"
	sleep 0.1
	printf '%s\n' "$PASSWORD"
}

ingress_start() {
	pw ingress -l "$LISTID" &
	INGRESS_PID=$!
	PIDS+=("$INGRESS_PID")
	wait_port "${INGRESS_ADDR##*:}"
}

http_start() {
	pw http &
	HTTP_PID=$!
	PIDS+=("$HTTP_PID")
	wait_port "${HTTP_ADDR##*:}"
}

# --- webhook sink ---------------------------------------------------------

sink_start() {
	xtrace go run "$REPO/smoke/webhook-sink.go" \
		-addr "$SINK_ADDR" -secret "$SECRET" -log "$WORKDIR/webhook.log" \
		> >(awk "{print \"$C_LOG\" \$0 \"$C_OFF\"}") &
	SINK_PID=$!
	PIDS+=("$SINK_PID")
	wait_port "${SINK_ADDR##*:}"
}

webhook_count() {
	local category="${1:-}"
	if [ ! -f "$WORKDIR/webhook.log" ]; then
		echo 0
		return
	fi
	if [ -n "$category" ]; then
		grep -c "\"event\":\"$category\"" "$WORKDIR/webhook.log" || true
	else
		grep -c . "$WORKDIR/webhook.log" || true
	fi
}

wait_webhook() {
	local category="$1" count="${2:-1}" timeout="${3:-10}"
	SECONDS=0
	while [ "$(webhook_count "$category")" -lt "$count" ]; do
		if [ "$SECONDS" -gt "$timeout" ]; then
			fail "expected >=$count '$category' webhooks, got $(webhook_count "$category")"
		fi
		sleep 0.2
	done
}

assert_webhook_signed() {
	if grep -q '"sig_ok":false' "$WORKDIR/webhook.log" 2>/dev/null; then
		fail "some webhook deliveries had an invalid signature"
	fi
}

# --- HTTP API -------------------------------------------------------------

# http_json <method> <path> [token] [json body]. Sets HTTP_CODE and HTTP_BODY.
http_json() {
	local method="$1" path="$2" token="${3:-}" body="${4:-}"
	local args=(-sS -X "$method" -H 'Content-Type: application/json' -w $'\n%{http_code}')
	if [ -n "$token" ]; then
		args+=(-H "Authorization: Bearer $token")
	fi
	if [ -n "$body" ]; then
		args+=(-d "$body")
	fi
	local out
	out=$(curl "${args[@]}" "$API/$path")
	HTTP_CODE=${out##*$'\n'}
	HTTP_BODY=${out%$'\n'*}
}

api_get() { http_json GET "$1" "${2:-}"; }
api_post() { http_json POST "$1" "$2" "$3"; }
api_patch() { http_json PATCH "$1" "$2" "$3"; }

# --- cleanup --------------------------------------------------------------

cleanup() {
	local status=$? pid
	set +e
	for pid in "${PIDS[@]}"; do
		kill_wait "$pid" 15
	done
	if [ "$status" -eq 0 ]; then
		ok "$NAME passed"
	else
		printf '%bfail: %s failed (status=%s)%b\n' \
			"$C_ERR" "$NAME" "$status" "$C_OFF" >&2
	fi
	exit "$status"
}
trap cleanup EXIT

section "$NAME"
