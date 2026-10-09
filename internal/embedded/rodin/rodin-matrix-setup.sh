#!/bin/bash
# One-time Matrix provisioning: create the owner account and the relay bot
# account on the local homeserver, store the bot's access token in the relay's
# .env, and enable the matrix frontend in config.json. Re-running is safe; it
# skips accounts that already exist.
set -euo pipefail

STATE=/claude-env/rodin
MATRIX_DIR="$STATE/matrix"
RELAY_DIR="$STATE/agent-relay"
HS_LOCAL="http://127.0.0.1:6167"
OWNER="${1:-}"
BOT="${2:-rodin}"

[ -f "$MATRIX_DIR/server_name" ] || { echo "homeserver not initialised; run rodin-up first" >&2; exit 1; }
SERVER_NAME="$(cat "$MATRIX_DIR/server_name")"
TOKEN="$(cat "$MATRIX_DIR/registration_token")"
HS_PUBLIC="https://$SERVER_NAME:8448"

if [ -z "$OWNER" ]; then
    read -rp "Your Matrix username (letters/digits, e.g. jean): " OWNER
fi

curl -fsS "$HS_LOCAL/_matrix/client/versions" >/dev/null \
    || { echo "homeserver not answering on $HS_LOCAL; is tmux rodin:matrix running?" >&2; exit 1; }

# Matrix registration is a two-step "user-interactive auth" flow: the first
# POST returns a session id, the second completes it with the token.
register_with() {
    local user="$1" pass="$2" token="$3" sess
    sess="$(curl -sS -X POST "$HS_LOCAL/_matrix/client/v3/register" -d '{}' | jq -r .session)"
    jq -n --arg u "$user" --arg p "$pass" --arg t "$token" --arg s "$sess" \
        '{username:$u, password:$p, inhibit_login:false,
          auth:{type:"m.login.registration_token", token:$t, session:$s}}' \
    | curl -sS -X POST "$HS_LOCAL/_matrix/client/v3/register" -d @-
}

# Continuwuity ignores the configured token until the first account exists; it
# prints a one-time bootstrap token in its log instead. Fall back to that.
register() {
    local out boot
    out="$(register_with "$1" "$2" "$TOKEN")"
    if [ "$(echo "$out" | jq -r '.errcode // empty')" = "M_FORBIDDEN" ]; then
        boot="$(sed 's/\x1b\[[0-9;]*m//g' "$STATE/logs/matrix.log" 2>/dev/null \
                | sed -n 's/.*using the registration token \([A-Za-z0-9]*\).*/\1/p' | tail -1)"
        [ -n "$boot" ] && out="$(register_with "$1" "$2" "$boot")"
    fi
    echo "$out"
}

user_exists() {
    [ "$(curl -sS "$HS_LOCAL/_matrix/client/v3/register/available?username=$1" | jq -r '.available // false')" != "true" ]
}

if user_exists "$OWNER"; then
    echo "owner @$OWNER:$SERVER_NAME already exists, skipping"
else
    while :; do
        read -rsp "Choose a password for @$OWNER:$SERVER_NAME: " p1; echo
        read -rsp "Repeat: " p2; echo
        [ "$p1" = "$p2" ] && [ -n "$p1" ] && break
        echo "passwords differ or empty, try again"
    done
    out="$(register "$OWNER" "$p1")"; unset p1 p2
    echo "$out" | jq -e .user_id >/dev/null || { echo "owner registration failed: $out" >&2; exit 1; }
    echo "created @$OWNER:$SERVER_NAME"
fi

ENV_FILE="$RELAY_DIR/.env"
if grep -q '^MATRIX_ACCESS_TOKEN=' "$ENV_FILE" 2>/dev/null && user_exists "$BOT"; then
    echo "bot @$BOT:$SERVER_NAME already provisioned, skipping"
else
    botpass="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    out="$(register "$BOT" "$botpass")"; unset botpass
    access="$(echo "$out" | jq -r '.access_token // empty')"
    [ -n "$access" ] || { echo "bot registration failed: $out" >&2; exit 1; }
    sed -i '/^MATRIX_ACCESS_TOKEN=/d' "$ENV_FILE"
    echo "MATRIX_ACCESS_TOKEN=$access" >> "$ENV_FILE"
    chmod 600 "$ENV_FILE"
    unset access
    echo "created bot @$BOT:$SERVER_NAME; token stored in $ENV_FILE"
fi

tmp="$(mktemp)"
jq --arg hs "$HS_PUBLIC" --arg admin "@$OWNER:$SERVER_NAME" --arg spool "$MATRIX_DIR/media-spool" '
    .matrix.enabled = true
  | .matrix.homeserver_url = $hs
  | .matrix.token_env = "MATRIX_ACCESS_TOKEN"
  | .matrix.admins = [$admin]
  | .matrix.media_spool = $spool' "$RELAY_DIR/config.json" > "$tmp" && mv "$tmp" "$RELAY_DIR/config.json"
mkdir -p "$MATRIX_DIR/media-spool"

cat << EOT

Matrix is provisioned.
  homeserver   $HS_PUBLIC
  you          @$OWNER:$SERVER_NAME
  relay bot    @$BOT:$SERVER_NAME

Next:
  1. Restart relayd so it picks up the matrix frontend:
       tmux kill-window -t rodin:relayd; rodin-up
  2. On your laptop (on the tailnet), open Element and sign in with
     homeserver $HS_PUBLIC as @$OWNER. Start a DM with @$BOT:$SERVER_NAME.
     The bot auto-joins and replies through the relay.
EOT
