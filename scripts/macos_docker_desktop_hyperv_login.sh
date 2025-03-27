#!/usr/bin/env sh
# echo "Hello from the hyperv bootstrap script!"

# https://flokoe.github.io/bash-hackers-wiki/howto/getopts_tutorial/

OPTIND=1

VERBOSE=0
KEEP=0
INTERACTIVE=1
MODE="base"
FILE=""

help() {
    echo "Usage: ${0##*/} [-h] [-v] [-k] [-n] [-m MODE] [-f FILE_HERE:FILE_THERE]"
    echo "Bootstrap into macos hyperv"
    echo "  -h  show this help text"
    echo "  -v  verbose mode"
    echo "  -k  if set, will keep the file on the host after running. default is to delete it"
    echo "  -n  non-interactive mode. will not launch a shell. default is to be interactive"
    echo "  -m  mode to run in (base, super, hyper). used internally. just leave it alone"
    echo "  -f  optionally a file to copy from the host to the container"
}

while getopts "hvknf:m:" opt; do
    case "$opt" in
    h)
        help
        exit 0
        ;;
    v) VERBOSE=1 ;;
    k) KEEP=1 ;;
    n) INTERACTIVE=0 ;;
    f) FILE=$OPTARG ;;
    m) MODE=$OPTARG ;;
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

[ "$VERBOSE" -eq 1 ] && V="-v"
[ "$KEEP" -eq 1 ] && K="-k"
[ "$INTERACTIVE" -eq 0 ] && N="-n"

INIT_NAME="init.sh"

base() {
    if [[ "$OSTYPE" != "darwin"* ]]; then
        echo "This script is only for macOS"
        exit 1
    fi
    # We're on macos. Launch a cont
    # Launch a container attached to the host vm
    [ "$VERBOSE" -eq 1 ] && echo "[base] launching donor container"

    MOUNTS=""
    MOUNTS="$MOUNTS -v /:/host"
    MOUNTS="$MOUNTS -v $(realpath "$0"):/$INIT_NAME"
    if [ -n "$FILE" ]; then
        SRC=$(echo "$FILE" | cut -d: -f1)
        SRC=$(realpath "$SRC")
        DEST=$(basename "$SRC")
        [ "$VERBOSE" -eq 1 ] && echo "[base] mounting $SRC to /$DEST in the donor container"
        MOUNTS="$MOUNTS -v $SRC:/$DEST"
    fi

    FLAGS="--net=host --ipc=host --uts=host --pid=host --privileged --security-opt=seccomp=unconfined"
    SUPER_ARGS="$V $K $N -m super"
    if [ -n "$FILE" ]; then
        SUPER_ARGS="$SUPER_ARGS -f $FILE"
    fi
    docker run $FLAGS -it --rm \
        $MOUNTS \
        alpine:latest \
        /bin/sh /$INIT_NAME $SUPER_ARGS
}

super() {
    # copy self to the host
    [ "$VERBOSE" -eq 1 ] && echo "[super] copying self to host"
    cp "$0" /host/
    chmod u+x /host/$(basename "$0")

    # if a file was specified, copy it to the host
    if [ -n "$FILE" ]; then
        # split the file argument into source and destination
        SRC=$(echo "$FILE" | cut -d: -f1)
        SRC=/$(basename "$SRC")
        DEST=$(echo "$FILE" | cut -d: -f2)
        # make sure the destination directory exists
        [ "$VERBOSE" -eq 1 ] && echo "[super] copying $SRC to /host/$DEST"
        mkdir -p /host"$(dirname "$DEST")"
        # copy the file to the host
        # echo "Copying $SRC to /host/$DEST"
        cp "$SRC" /host/"$DEST"
    fi
    # chroot to /host and run the script
    [ "$VERBOSE" -eq 1 ] && echo "[super] chrooting to host"
    HYPER_ARGS="$V $K $N -m hyper"
    if [ -n "$FILE" ]; then
        HYPER_ARGS="$HYPER_ARGS -f $FILE"
    fi
    chroot /host "/$(basename "$0")" $HYPER_ARGS
}

hyper() {
    # install a couple of utility things and then launch into a shell
    if [ $INTERACTIVE -eq 1 ]; then
        [ "$VERBOSE" -eq 1 ] && echo "[hyper] installing utilities"
        apt-get install -y fish htop jq vim 1>/dev/null 2>&1
        SHELL=$(which fish) EDITOR=$(which vim.basic) fish
    else
        # non-interactive mode.
        [ "$VERBOSE" -eq 1 ] && echo "[hyper] non-interactive mode"
    fi
    if [ -n "$FILE" ] && [ "$KEEP" -eq 0 ]; then
        DEST=$(echo "$FILE" | cut -d: -f2)
        [ "$VERBOSE" -eq 1 ] && echo "[hyper] deleting /$DEST"
        # delete the file from the host.
        rm /"$DEST"
    fi
}

# switch on the mode
case $MODE in
base) base ;;
super) super ;;
hyper) hyper ;;
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
