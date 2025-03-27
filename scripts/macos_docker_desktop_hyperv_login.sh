#!/usr/bin/env sh
# echo "Hello from the hyperv bootstrap script!"

# check if were on macos
MODE=$1

[ -z "$MODE" ] && MODE="default"

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
        -v $(pwd)/$(basename "$0"):/$INIT_NAME alpine:latest /bin/sh /$INIT_NAME super
}

super() {
    echo "super"
    cp "$0" /host/
    chmod u+x /host/$(basename "$0")
    # chroot to /host and run the script
    chroot /host "/$(basename "$0")" hyper
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
