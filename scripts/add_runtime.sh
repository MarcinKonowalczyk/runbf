#!/usr/bin/env sh
# }
#     "runtimes": {
#         "foo": {
#             "path": "/foo/bar/runc"
#     }
# }

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

RUNTIME_NAME="foo"
RUNTIME_PATH="/foo/bar/shim"

# RUNTIME_NAME="$1"
# RUNTIME_PATH="$2"
# if [ -z "$RUNTIME_NAME" ] || [ -z "$RUNTIME_PATH" ]; then
#     echo "Usage: $0 <runtime_name> <runtime_path>"
#     exit 1
# fi

# Add the runtime to the daemon config

JQ_SCRIPT=".runtimes += {\"$RUNTIME_NAME\": {\"path\": \"$RUNTIME_PATH\"}}"
jq "$JQ_SCRIPT" ~/.docker/daemon.json >/tmp/daemon.json
cat /tmp/daemon.json >~/.docker/daemon.json
