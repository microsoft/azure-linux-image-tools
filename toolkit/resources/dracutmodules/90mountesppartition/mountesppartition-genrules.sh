#!/bin/sh

echo "Running mountesppartition-genrules.sh" > /dev/kmsg

# this gets called after all devices have settled.
/sbin/initqueue --finished --onetime --unique /sbin/mountesppartition > /dev/kmsg
