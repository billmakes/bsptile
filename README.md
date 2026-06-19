# bsptile
![os](https://img.shields.io/badge/os-%20linux%20|%20freebsd%20-blue?style=flat-square)
![platform](https://img.shields.io/badge/platform-%20amd64%20|%20arm64%20|%20armv6%20|%20386%20-teal?style=flat-square)

A binary-space-partitioning auto-tiling manager for Openbox, Fluxbox, IceWM,
Xfwm, KWin, Marco, Muffin, Mutter and other
[EWMH](https://en.wikipedia.org/wiki/Extended_Window_Manager_Hints#List_of_window_managers_that_support_Extended_Window_Manager_Hints)
compliant window managers on [X11](https://en.wikipedia.org/wiki/X_Window_System).
Designed to drop in on top of XFCE, LXDE, LXQt, KDE and GNOME-flavoured
(Mate, Deepin, Cinnamon, Budgie) desktops without replacing the underlying
window manager.

Keep your current WM running and let **bsptile sit on top of it**. Once
enabled, bsptile owns the _placement_ and _sizing_ of tracked windows; the
WM keeps decorations, focus stealing prevention, and everything else.

## About
bsptile started as a fork of [cortile](https://github.com/leukipp/cortile)
to add a few quality-of-life things on top of its solid foundation, and over
time grew into its own design: a strict binary space partitioning tree, a
control socket modeled on `bspc`, window/workspace rule definitions, and
first-class [sxhkd](https://github.com/baskerville/sxhkd) integration. The
design lineage is firmly [bspwm](https://github.com/baskerville/bspwm)-shaped —
stateless client, action-driven daemon, scriptable rules — adapted to ride
on top of an existing EWMH window manager instead of being one.

## Features
- Binary space partitioning, maximized, and fullscreen layouts.
- Per-node split ratios with directional grow/shrink as a single `resize_<dir>` action.
- `balance` to equalize every leaf's area across an unbalanced tree.
- Window rules: per-`WM_CLASS` floating/tile overrides, target monitor, target desktop, sticky.
- Workspace rules: per-(desktop × screen) initial tiling state, layout, decoration.
- Control socket + `bsptilectl` CLI (`action`, `query`, `subscribe`, `reload`, `wm`) with JSON IPC.
- Native sxhkd integration — sxhkd owns global key/mouse grabs while bsptile exposes actions through `bsptilectl`.
- D-Bus interface for legacy and Python integrations.
- Drag-and-drop window swap, drop-zone indicator, cross-monitor direction-aware moves.
- Multi-monitor support with per-screen BSP trees.
- Toggle window decorations per workspace.
- Systray icon + menu.

## Build and install
You need [Go ≥ 1.22](https://go.dev/dl/).

```bash
git clone https://github.com/billmakes/bsptile.git
cd bsptile
make build
make install
```

`make build` creates `./bsptile` and `./bsptilectl`. `make install` installs
both to `~/.local/bin`.

Run the daemon manually:

```bash
bsptile
```

### Service
To enable auto tiling on startup, run bsptile as a systemd user service.
A template is provided in the [services](https://github.com/billmakes/bsptile/tree/main/assets/services) folder:
```bash
cp assets/services/bsptile.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now bsptile.service
```

## Usage
Tiling uses a binary space partitioning tree. Every window is a leaf, and
opening a new window splits the focused leaf along its longest side.
Maximized and fullscreen are explicit modes; `layout_bsp` returns the
workspace to its tree. Window swaps exchange tree leaves without changing
the tree structure. Resizing a tiled window adjusts the nearest matching
split ratio.

## Configuration
The configuration file is at `~/.config/bsptile/config.toml` (or
`$XDG_CONFIG_HOME`) and is created with default values on first startup.
See [config.toml](https://github.com/billmakes/bsptile/blob/main/config.toml) for all available options.

A few selected knobs:

- `window_focus_delay` — milliseconds before hover-focus fires (`0` disables; `100`–`150` is a comfortable range). Disable the WM's focus-follows-mouse first.
- `window_pointer_warp` — move the pointer with directional focus, window swaps, and screen moves.
- `window_floating_above` — keep dialogs and unmanaged windows above tiled ones.

### Bindings
bsptile no longer grabs global keys or mouse buttons from `config.toml`.
Bind keys/buttons in sxhkd or another hotkey daemon and call `bsptilectl`.

```sxhkd
super + o
    bsptilectl action toggle

super + shift + {1-9}
    bsptilectl action window_to_desktop_{1-9}

super + {1-9}
    bsptilectl action desktop_{1-9}

super + q
    bsptilectl action close

button12
    bsptilectl action layout_maximized
```

The example [`assets/sxhkdrc.example`](https://github.com/billmakes/bsptile/blob/main/assets/sxhkdrc.example)
is a complete sxhkd profile with regular desktop hotkeys, media keys, bsptile
actions, workspace sends, resize chords, and commented high mouse-button
examples.

Typical setup:

```bash
mkdir -p ~/.config/sxhkd
cp assets/sxhkdrc.example ~/.config/sxhkd/sxhkdrc.bsptile
sxhkd -c ~/.config/sxhkd/sxhkdrc.bsptile &
```

After edits:

```bash
pkill -USR1 -x sxhkd
```

List available bsptile actions from the running daemon:

```bash
bsptilectl actions
```

### Window rules
Per-class overrides applied when a window is first tracked. Indexing for
`monitor` / `desktop` is **1-based** to match what xfwm shows in its UI.
First match wins.

```toml
[[window_rules]]
class = "(?i)vesktop"
monitor = 2
desktop = 2

[[window_rules]]
class = "(?i)pavucontrol"
floating = true

[[window_rules]]
class = "Steam"
name = "^Friends List$"
sticky = true
```

Fields:
- `class` (required) — RE2 regex against `WM_CLASS`.
- `name` (optional) — RE2 regex against `WM_NAME`. Narrows the match.
- `floating` — leave the window unmanaged (same effect as `window_ignore`).
- `sticky` — float above and visible on every desktop.
- `tile` — force-tile even when bsptile would otherwise float (e.g. dialogs).
- `monitor` — 1-indexed monitor to send the window to on first track.
- `desktop` — 1-indexed desktop to send the window to on first track.

### Workspace rules
Per-`(desktop, screen)` initial state. Runtime toggles still win until the
daemon restarts or reloads.

```toml
[[workspace_rules]]
desktop = 2
tiling = false

[[workspace_rules]]
desktop = 3
screen = 1
layout = "maximized"
```

Fields:
- `desktop` (required, 1-indexed).
- `screen` (optional, 1-indexed; omit = every screen of that desktop).
- `tiling` — override `tiling_enabled`.
- `layout` — `"bsp"`, `"maximized"`, or `"fullscreen"`.
- `decoration` — override `window_decoration`.

### Common pointer shortcuts
- Move window: <kbd>Alt</kbd>+<kbd>Left-Click</kbd>
- Resize window: <kbd>Alt</kbd>+<kbd>Right-Click</kbd> (or, on xfwm, <kbd>Super</kbd>+<kbd>Right-Click</kbd>)
- Maximize window: <kbd>Alt</kbd>+<kbd>Double-Click</kbd>

## IPC
### Control socket (`bsptilectl`)
The daemon listens on a Unix-domain socket at `$BSPTILE_SOCKET`,
`$XDG_RUNTIME_DIR/bsptile-<display>.sock`, or `/tmp/bsptile-<display>-<uid>.sock`
(in that order of precedence). `bsptilectl` speaks JSON over that socket:

```bash
bsptilectl action toggle
bsptilectl action balance --mod workspaces
bsptilectl actions
bsptilectl query workspaces | jq .
bsptilectl subscribe action workplace
bsptilectl reload
bsptilectl wm exit
```

Commands:
- `action <name> [--mod current|screens|workspaces]` — invoke a bsptile action.
- `actions` or `action --list` — list available actions with descriptions.
- `query [workspaces|windows|clients|workplace|config|actions]` — read live state as JSON.
- `subscribe [topic…]` — long-lived connection that streams newline-delimited JSON events. Topics: `action`, `workplace`, `windows`, `clients`, `workspaces`, `*`.
- `reload` — reread `config.toml`.
- `wm exit|restart` — stop or restart the daemon.

Desktop-related actions are split intentionally:

- `window_to_desktop_<n>`, `window_to_desktop_next`, `window_to_desktop_previous`
  move the active window to another desktop without changing the visible view.
- `desktop_<n>`, `desktop_next`, `desktop_previous` switch the visible desktop
  without moving the active window.

A sample sxhkd file that drives bsptile entirely through `bsptilectl` is at
[`assets/sxhkdrc.example`](https://github.com/billmakes/bsptile/blob/main/assets/sxhkdrc.example).

### D-Bus
The daemon also exports its properties and methods via D-Bus for legacy and
Python integrations. `bsptile dbus -help` lists what's available. Disable with
`-disable-dbus-interface`.

### Python
Python bindings wrap the D-Bus interface in easy-to-use methods.
See [bsptile-addons](https://github.com/billmakes/bsptile-addons) for example scripts.

## Development
Run in verbose mode:
```bash
./bsptile -v
```

## Debugging
- `bsptile -vv` enables debug output.
- Log file defaults to `/tmp/bsptile.log`.

## Security
- The D-Bus API exposes internal window properties — disable with `-disable-dbus-interface`.
- The control socket also exposes them — disable with `-disable-socket-interface`.
- Scripts in `~/.config/bsptile/addons/` are executed on startup — disable with `-disable-addons-folder`.
- Do not run bsptile as root.

## Credits & thanks
- [cortile](https://github.com/leukipp/cortile) by **leukipp** — bsptile started as a fork of cortile, and its EWMH plumbing, tracker scaffolding, and overall daemon shape still owe an enormous amount to that project.
- [bspwm](https://github.com/baskerville/bspwm) by **baskerville** — the spiritual model for the BSP semantics, the `bspc`/socket control-plane design, and the window-rule grammar that this project carries.

## License
[MIT](https://github.com/billmakes/bsptile/blob/main/LICENSE)
