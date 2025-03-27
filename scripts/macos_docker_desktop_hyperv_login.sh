#!/usr/bin/env sh
# echo "Hello from the hyperv bootstrap script!"

# https://flokoe.github.io/bash-hackers-wiki/howto/getopts_tutorial/

OPTIND=1

VERBOSE=0
MODE="default"
FILE=""

help() {
    echo "Usage: ${0##*/} [-h] [-v] [-m MODE] [-f FILE_HERE:FILE_THERE]"
    echo "Bootstrap into macos hyperv"
    echo "  -h  show this help text"
    echo "  -v  verbose mode"
    echo "  -m  mode to run in (default, super, hyper). used internally. just leave it alone"
    echo "  -f  optionally a file to copy from the host to the container"
}

while getopts "hvf:m:" opt; do
    case "$opt" in
    h)
        help
        exit 0
        ;;
    v)
        VERBOSE=1
        ;;
    m)
        MODE=$OPTARG
        ;;
    f)
        FILE=$OPTARG
        ;;
    \?)
        echo "Invalid option: -$OPTARG" >&2
        help
        exit 1
        ;;
    :)
        echo "Option -$OPTARG requires an argument." >&2
        help
        exit 1
        ;;
    esac
done

shift $((OPTIND - 1))

[ "${1:-}" = "--" ] && shift

if [ "$VERBOSE" -eq 1 ]; then
    echo "Verbose mode enabled"
    echo "Mode: $mode"
    echo "File: $file"
fi

INIT_NAME="init.sh"

default() {
    echo "default"
    if [[ "$OSTYPE" != "darwin"* ]]; then
        echo "This script is only for macOS"
        exit 1
    fi
    # We're on macos. Launch a cont
    # Launch a container attached to the host vm
    docker run --net=host --ipc=host --uts=host --pid=host --privileged \
        --security-opt=seccomp=unconfined -it --rm \
        -v /:/host \
        -v $(realpath "$0"):/$INIT_NAME alpine:latest /bin/sh /$INIT_NAME -m super
}

super() {
    echo "super"
    cp "$0" /host/
    chmod u+x /host/$(basename "$0")
    # chroot to /host and run the script
    chroot /host "/$(basename "$0")" -m hyper
}

hyper() {
    echo "hyper"
    apt-get install -y fish htop jq 1>/dev/null 2>&1
    fish
}

# switch on the mode
case $MODE in
default)
    default
    ;;
super)
    super
    ;;
hyper)
    hyper
    ;;
*)
    if [ "$MODE" == "--help" ] || [ "$MODE" == "-h" ]; then
        echo "script for bootstrapping into macos hyperv"
        echo "run with no arguments from macos"
        echo "run with 'super' from the a container mapped to the host"
        echo "run with 'hyper' from the chrooted host"
    else
        echo "invalid mode $MODE"
    fi
    exit 1
    ;;
esac
