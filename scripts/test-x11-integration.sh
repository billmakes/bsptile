#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

for command in Xvfb xvfb-run xfwm4 dbus-run-session jq wmctrl xdotool xprop xterm; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "missing integration dependency: $command" >&2
        exit 1
    fi
done

tmp=$(mktemp -d)
cleanup() {
    rm -rf "$tmp"
}
trap cleanup EXIT

config="$tmp/config.toml"
socket="$tmp/bsptile.sock"
lock="$tmp/bsptile.lock"
log="$tmp/bsptile.log"
cache="$tmp/cache"

cat >"$config" <<'EOF'
tiling_enabled = true
tiling_gui = 0
tiling_icon = []
keybindings_enabled = false
window_ignore = []
window_gap_size = 4
window_focus_delay = 0
window_pointer_warp = false
window_floating_above = true
window_decoration = true
proportion_step = 0.05
proportion_min = 0.2
edge_margin = [0, 0, 0, 0]
edge_margin_primary = [0, 0, 0, 0]
drop_target_width = 2

[keys]
[mouse]
[modes]
[systray]

[[window_rules]]
class = "^StickyTest$"
sticky = true
monitor = 1
EOF

export BSPTILE_TEST_ROOT="$root"
export BSPTILE_TEST_CONFIG="$config"
export BSPTILE_TEST_SOCKET="$socket"
export BSPTILE_TEST_LOCK="$lock"
export BSPTILE_TEST_LOG="$log"
export BSPTILE_TEST_CACHE="$cache"
export BSPTILE_TEST_WM_LOG="$tmp/xfwm4.log"
export BSPTILE_TEST_CLIENT_LOG="$tmp/xterm.log"

dbus-run-session -- xvfb-run -a -s "-screen 0 1280x800x24 -nolisten tcp" \
    bash -euo pipefail -c '
        root=$BSPTILE_TEST_ROOT
        xfwm4 --replace --compositor=off >"$BSPTILE_TEST_WM_LOG" 2>&1 &
        wm_pid=$!
        tiled_pid=
        second_pid=
        sticky_pid=
        daemon_pid=

        cleanup_inner() {
            if [[ -n ${daemon_pid:-} ]]; then
                kill "$daemon_pid" 2>/dev/null || true
            fi
            kill "${tiled_pid:-}" "${second_pid:-}" "${sticky_pid:-}" 2>/dev/null || true
            kill "$wm_pid" 2>/dev/null || true
            wait "${tiled_pid:-}" "${second_pid:-}" "${sticky_pid:-}" 2>/dev/null || true
            wait "$wm_pid" 2>/dev/null || true
        }
        trap cleanup_inner EXIT

        wait_for() {
            local description=$1
            shift
            for _ in $(seq 1 100); do
                if "$@"; then
                    return 0
                fi
                sleep 0.05
            done
            echo "timed out waiting for $description" >&2
            cat "$BSPTILE_TEST_LOG" >&2 2>/dev/null || true
            return 1
        }

        client_count_is() {
            local expected=$1
            [[ $("$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" query clients 2>/dev/null |
                jq ".data | length") == "$expected" ]]
        }

        state_contains() {
            local window=$1
            local state=$2
            xprop -id "$window" _NET_WM_STATE 2>/dev/null | grep -q "$state"
        }

        state_absent() {
            local window=$1
            local state=$2
            ! state_contains "$window" "$state"
        }

        client_x_is() {
            local expected=$1
            [[ $("$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" query clients |
                jq ".data[0].Latest.Dimensions.Geometry.X") == "$expected" ]]
        }

        layout_is() {
            local expected=$1
            [[ $("$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" query workspaces |
                jq ".data[0].Layout") == "$expected" ]]
        }

        for _ in $(seq 1 100); do
            if xprop -root _NET_SUPPORTING_WM_CHECK 2>/dev/null | grep -q WINDOW; then
                break
            fi
            sleep 0.05
        done
        xprop -root _NET_SUPPORTING_WM_CHECK | grep -q WINDOW

        xterm -class TiledTest -name tiled-test -geometry 80x24+20+20 >"$BSPTILE_TEST_CLIENT_LOG" 2>&1 &
        tiled_pid=$!
        for _ in $(seq 1 100); do
            if xprop -root _NET_CLIENT_LIST 2>/dev/null | grep -q WINDOW; then
                break
            fi
            sleep 0.05
        done
        xprop -root _NET_CLIENT_LIST | grep -q WINDOW

        "$root/bsptile" \
            -config "$BSPTILE_TEST_CONFIG" \
            -socket "$BSPTILE_TEST_SOCKET" \
            -lock "$BSPTILE_TEST_LOCK" \
            -log "$BSPTILE_TEST_LOG" \
            -cache "$BSPTILE_TEST_CACHE" \
            -vv \
            disable-dbus-interface disable-addons-folder disable-release-info &
        daemon_pid=$!

        for _ in $(seq 1 100); do
            if [[ -S $BSPTILE_TEST_SOCKET ]]; then
                break
            fi
            if ! kill -0 "$daemon_pid" 2>/dev/null; then
                cat "$BSPTILE_TEST_LOG" >&2 || true
                exit 1
            fi
            sleep 0.05
        done

        [[ -S $BSPTILE_TEST_SOCKET ]]
        [[ $(stat -c %a "$BSPTILE_TEST_SOCKET") == 600 ]]

        "$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" query workplace |
            grep -q "\"ok\":true"
        "$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" query config |
            grep -q "\"KeybindingsEnabled\":false"
        "$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" reload |
            grep -q "\"ok\":true"

        tiled_window=$(xdotool search --sync --class TiledTest | head -1)
        xdotool windowactivate --sync "$tiled_window"
        wait_for "initial tracked window" client_count_is 1

        # A single BSP client uses half-gap placement (x=2); maximized uses
        # a full gap on each edge (x=4), then toggles back to BSP.
        wait_for "initial BSP geometry" client_x_is 2
        "$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" action layout_maximized |
            grep -q "\"ok\":true"
        wait_for "maximized geometry" client_x_is 4
        "$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" action layout_maximized |
            grep -q "\"ok\":true"
        wait_for "restored BSP geometry" client_x_is 2

        # Native EWMH maximize requests (the same path as XFWM title-bar
        # maximize) must toggle into and back out of bsptile maximized mode.
        # The tracker intentionally ignores maximize requests during the first
        # second of a window lifetime while applications establish initial
        # geometry and state.
        sleep 1.1
        wmctrl -i -r "$tiled_window" -b toggle,maximized_vert,maximized_horz
        wait_for "native maximized layout" layout_is 1
        wmctrl -i -r "$tiled_window" -b toggle,maximized_vert,maximized_horz
        wait_for "native maximize toggle back to BSP" layout_is 0

        "$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" action layout_fullscreen |
            grep -q "\"ok\":true"
        wait_for "fullscreen state" state_contains "$tiled_window" _NET_WM_STATE_FULLSCREEN
        "$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" action layout_fullscreen |
            grep -q "\"ok\":true"
        wait_for "fullscreen state removal" state_absent "$tiled_window" _NET_WM_STATE_FULLSCREEN

        # Window lifecycle: a second normal window is tracked, then removed.
        xterm -class SecondTest -name second-test -geometry 80x24+100+100 >/dev/null 2>&1 &
        second_pid=$!
        wait_for "second tracked window" client_count_is 2
        kill "$second_pid"
        wait "$second_pid" 2>/dev/null || true
        second_pid=
        wait_for "second window removal" client_count_is 1

        # Sticky rules are unmanaged, always above, and assigned to all
        # desktops. They must not enter the BSP client list.
        xterm -class StickyTest -name sticky-test -geometry 40x10+200+200 >/dev/null 2>&1 &
        sticky_pid=$!
        sticky_window=$(xdotool search --sync --class StickyTest | head -1)
        wait_for "sticky state" state_contains "$sticky_window" _NET_WM_STATE_STICKY
        wait_for "above state" state_contains "$sticky_window" _NET_WM_STATE_ABOVE
        wait_for "all-desktops assignment" bash -c \
            "xprop -id $sticky_window _NET_WM_DESKTOP | grep -Eqi \"0xffffffff|4294967295\""
        client_count_is 1

        "$root/bsptilectl" --socket "$BSPTILE_TEST_SOCKET" wm exit |
            grep -q "\"ok\":true"

        for _ in $(seq 1 100); do
            if ! kill -0 "$daemon_pid" 2>/dev/null; then
                daemon_pid=
                break
            fi
            sleep 0.05
        done
        [[ -z $daemon_pid ]]
    '

echo "X11 integration smoke test passed"
