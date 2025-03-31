#!/bin/sh
# https://stackoverflow.com/a/5773251
kill -STOP $$ # suspend myself
# ... until I receive SIGCONT
exec $@
