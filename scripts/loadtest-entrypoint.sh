#!/bin/sh
set -eu
if [ "$#" -eq 1 ]; then
    case "$1" in
        "/bin/sh /usr/local/bin/loadtest-entrypoint.sh") ;;
        *) exec /bin/sh -c "$1" ;;
    esac
elif [ "$#" -gt 1 ]; then
    exec "$@"
fi
/usr/local/bin/test-fixtures seed
/usr/local/bin/test-fixtures &
exec /usr/local/bin/metatube
