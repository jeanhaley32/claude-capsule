#!/bin/bash
# Bring up the rodin stack inside the container: tailscaled, vessel-writer, relayd.
# Everything mutable (configs, tokens, allowlists, tailscale identity, documents,
# logs) lives under /claude-env/rodin on the encrypted volume, so nothing is
# lost when the container or image is rebuilt.
set -euo pipefail

STATE=/claude-env/rodin
RELAY_DIR="$STATE/agent-relay"
WRITER_ROOT="$STATE/writing"
MATRIX_DIR="$STATE/matrix"
SOCK="${RELAY_SOCKET:-/tmp/agent-relay.sock}"
WRITER_PORT="${WRITER_PORT:-8791}"
MATRIX_PORT=6167
MATRIX_PUBLIC_PORT=8448
mkdir -p "$RELAY_DIR" "$WRITER_ROOT" "$MATRIX_DIR" "$STATE/logs"
chmod 700 "$STATE" "$RELAY_DIR" "$MATRIX_DIR"

# `capsule rodin` passes /dev/net/tun + NET_ADMIN, so tailscaled runs in kernel
# mode and the container itself owns the tailnet address relayd binds to.
if ! pgrep -x tailscaled >/dev/null; then
    sudo mkdir -p /var/run/tailscale
    sudo nohup tailscaled --state="$STATE/tailscaled.state" \
        >"$STATE/logs/tailscaled.log" 2>&1 &
    sleep 2
fi
if ! tailscale ip -4 >/dev/null 2>&1; then
    echo "Tailscale is not connected yet. Run:  sudo tailscale up" >&2
    echo "then re-run rodin-up." >&2
    exit 1
fi

# Matrix homeserver: server_name is this container's MagicDNS name, fixed on
# first run because Matrix user ids embed it.
if [ ! -f "$MATRIX_DIR/server_name" ]; then
    tailscale status --json | jq -r '.Self.DNSName | rtrimstr(".")' > "$MATRIX_DIR/server_name"
fi
SERVER_NAME="$(cat "$MATRIX_DIR/server_name")"
if [ ! -f "$MATRIX_DIR/registration_token" ]; then
    head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n' > "$MATRIX_DIR/registration_token"
    chmod 600 "$MATRIX_DIR/registration_token"
fi
sed "s/__SERVER_NAME__/$SERVER_NAME/g" /opt/rodin/matrix/continuwuity.toml.tmpl > "$MATRIX_DIR/continuwuity.toml"

# TLS for Matrix clients on the tailnet: https://<server_name>:8448 -> homeserver.
if ! sudo tailscale serve status 2>/dev/null | grep -q ":$MATRIX_PUBLIC_PORT"; then
    sudo tailscale serve --bg --https="$MATRIX_PUBLIC_PORT" "http://127.0.0.1:$MATRIX_PORT" >/dev/null \
        || echo "warning: tailscale serve failed; enable HTTPS certificates in your Tailscale admin console (DNS > HTTPS Certificates)" >&2
fi

first_run=0
if [ ! -f "$RELAY_DIR/config.json" ]; then
    # state_dir is pinned so relayd never falls back to a cwd outside the volume.
    jq --arg d "$RELAY_DIR" '.state_dir = $d' /opt/rodin/agent-relay/config.example.json \
        > "$RELAY_DIR/config.json"
    first_run=1
fi
if [ ! -f "$RELAY_DIR/security.yaml" ]; then
    cp /opt/rodin/agent-relay/security.example.yaml "$RELAY_DIR/security.yaml"
fi
if [ ! -f "$RELAY_DIR/.env" ]; then
    printf '# Bot tokens, one per line, e.g.\n# TELEGRAM_BOT_TOKEN=123456:abc\n# DISCORD_BOT_TOKEN=...\n' > "$RELAY_DIR/.env"
    chmod 600 "$RELAY_DIR/.env"
fi
if [ "$first_run" = 1 ]; then
    echo "First run. Wrote:" >&2
    echo "  $RELAY_DIR/config.json    <- set admins (your Telegram/Discord user id)" >&2
    echo "  $RELAY_DIR/.env           <- bot token(s)" >&2
    echo "  $RELAY_DIR/security.yaml  <- what the relayed Claude session may do" >&2
    echo "Edit them, then re-run rodin-up. See /etc/claude-code/CLAUDE.md for the walkthrough." >&2
    exit 1
fi

cat > "$STATE/mcp.json" << JSON
{
  "mcpServers": {
    "relay":  { "command": "/usr/local/bin/relay-shim", "args": ["--socket", "$SOCK"] },
    "writer": { "command": "/usr/local/bin/writer-mcp", "args": ["-base", "http://127.0.0.1:$WRITER_PORT"] }
  }
}
JSON

# Claude's permission settings for the relayed session, derived from security.yaml.
mkdir -p "$STATE/claude-workspace/.claude"
SEC_FLAGS="$(cd "$RELAY_DIR" && apply-security --config security.yaml --settings "$STATE/claude-workspace/.claude/settings.json")"
echo "$SEC_FLAGS" > "$STATE/claude-flags"

if ! tmux has-session -t rodin 2>/dev/null; then
    tmux new-session -d -s rodin -n writer -c /opt/rodin/vessel-writer \
        "WRITER_ROOT=$WRITER_ROOT WRITER_PORT=$WRITER_PORT WRITER_CLIENTLOG=$STATE/logs/writer-client.log ./run.sh 2>&1 | tee -a $STATE/logs/writer.log"
fi
if ! tmux list-windows -t rodin -F '#W' | grep -qx matrix; then
    tmux new-window -t rodin -n matrix -c "$MATRIX_DIR" \
        "CONDUWUIT_CONFIG=$MATRIX_DIR/continuwuity.toml continuwuity 2>&1 | tee -a $STATE/logs/matrix.log"
fi
if ! tmux list-windows -t rodin -F '#W' | grep -qx relayd; then
    rm -f "$SOCK"
    tmux new-window -t rodin -n relayd -c "$RELAY_DIR" \
        "set -a; . ./.env; set +a; relayd --config config.json 2>&1 | tee -a $STATE/logs/relayd.log"
fi

cat << EOT
rodin is up
  writer   http://127.0.0.1:$WRITER_PORT      (tmux rodin:writer)
  matrix   https://$SERVER_NAME:$MATRIX_PUBLIC_PORT   (tmux rodin:matrix; tailnet only)
  relayd   $SOCK   (tmux rodin:relayd, loopback API on 127.0.0.1:9210)
  state    $STATE  (encrypted volume)

No Matrix accounts yet?  rodin-matrix-setup <your-username>

Start the relayed Claude session (in tmux so it survives your shell):
  tmux new-window -t rodin -n claude -c $STATE/claude-workspace \\
    "claude --mcp-config $STATE/mcp.json $SEC_FLAGS --dangerously-load-development-channels server:relay"
  tmux attach -t rodin:claude      # approve the one-time dev-channels prompt, then Ctrl-b d
EOT
