#!/usr/bin/env sh

JQ=$(command -v jq)
if [ -z "$JQ" ]; then
    echo "jq is not installed. Please install jq to use this script."
    exit 1
fi

DAEMON_CONFIG="$HOME/.docker/daemon.json"
if [ ! -f "$DAEMON_CONFIG" ]; then
    echo "Daemon config file not found at $DAEMON_CONFIG. Please create it first."
    exit 1
fi

RUNTIME_NAME="brainfuck"
RUNTIME_PATH="/usr/bin/containerd-shim-brainfuck-v1"

JQ_SCRIPT=".runtimes += {\"$RUNTIME_NAME\": {\"runtimeType\": \"$RUNTIME_PATH\"}}"
jq "$JQ_SCRIPT" ~/.docker/daemon.json >/tmp/daemon.json
cat /tmp/daemon.json >~/.docker/daemon.json
