# bsptile
![build](https://img.shields.io/github/actions/workflow/status/billmakes/bsptile/release.yaml?style=flat-square)
![date](https://img.shields.io/github/release-date/billmakes/bsptile?style=flat-square)
![downloads](https://img.shields.io/github/downloads/billmakes/bsptile/total?style=flat-square)
![os](https://img.shields.io/badge/os-%20linux%20|%20freebsd%20-blue?style=flat-square)
![platform](https://img.shields.io/badge/platform-%20amd64%20|%20arm64%20|%20armv6%20|%20386%20-teal?style=flat-square)

Linux auto tiling manager with hot corner support for Openbox, Fluxbox, IceWM, Xfwm, KWin, Marco, Muffin, Mutter and other [EWMH](https://en.wikipedia.org/wiki/Extended_Window_Manager_Hints#List_of_window_managers_that_support_Extended_Window_Manager_Hints) compliant window managers using the [X11](https://en.wikipedia.org/wiki/X_Window_System) window system.
Provides dynamic tiling for XFCE, LXDE, LXQt, KDE and GNOME (Mate, Deepin, Cinnamon, Budgie) based desktop environments.

Keep your current window manager and run **bsptile on top** of it.
Once enabled, the tiling manager handles _resizing_ and _positioning_ of _existing_ and _new_ windows.

## Features
- [x] Workspace based tiling.
- [x] Auto detection of panels.
- [x] Toggle window decorations.
- [x] User interface for tiling mode.
- [x] Systray icon indicator and menu.
- [x] Custom addons via python bindings.
- [x] Keyboard, hot corner and systray bindings.
- [x] Binary space partitioning, maximized and fullscreen modes.
- [x] Per-node BSP split ratios.
- [x] Floating and sticky windows.
- [x] Drag & drop window swap.
- [x] Workspace-aware BSP trees.
- [x] Multi monitor support.

## Installation
Manually [download](https://github.com/billmakes/bsptile/releases/latest) the latest binary from [releases](https://github.com/billmakes/bsptile/releases/latest) or use wget:
```bash
wget -qO- $(wget -qO- https://api.github.com/repos/billmakes/bsptile/releases/latest | \
jq -r '.assets[] | select(.name | contains ("linux_amd64.tar.gz")) | .browser_download_url') | \
tar -xvz
```

Run the binary to start tiling:
```bash
./bsptile
```

### Service
To enable auto tiling on startup, run bsptile as a systemd user service.
A template is provided in the [services](https://github.com/billmakes/bsptile/tree/main/assets/services) folder:
```bash
cp bsptile.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable bsptile.service
systemctl --user start bsptile.service
```

### Usage
The tiling mode uses a binary space partitioning tree. Every window is a leaf,
and opening a window splits the focused leaf along its longest side. Maximized
and fullscreen are explicit modes; `layout_bsp` returns the workspace to its
tree.

Window swaps exchange tree leaves without changing the tree structure. Resizing a tiled
window adjusts the nearest matching split ratio.

## Configuration
The configuration file is located at `~/.config/bsptile/config.toml` (or `XDG_CONFIG_HOME`) and is created with default values on first startup.
See [config.toml](https://github.com/billmakes/bsptile/blob/main/config.toml) for all available options.

### Shortcuts
Default keyboard shortcuts:
| Keys                                                    | Description                                     |
| ------------------------------------------------------- | ----------------------------------------------- |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>Home</kbd>        | Enable tiling on the current screen             |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>End</kbd>         | Disable tiling on the current screen            |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>T</kbd>           | Toggle between enable and disable               |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>D</kbd>           | Toggle window decoration on and off             |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>R</kbd>           | Disable tiling and restore windows              |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>BackSpace</kbd>   | Reset BSP split ratios                          |
| <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>C</kbd>           | Reload configuration and keyboard shortcuts     |
| <kbd>Mod4</kbd>+<kbd>Shift</kbd>+<kbd>R</kbd>           | Enter resize mode                               |
| <kbd>Mod4</kbd>+<kbd>E</kbd>                            | Return to BSP tiling mode                       |
| <kbd>Mod4</kbd>+<kbd>Alt</kbd>+<kbd>Space</kbd>         | Activate maximized layout                       |
| <kbd>Mod4</kbd>+<kbd>F</kbd>                            | Activate fullscreen layout                      |
| <kbd>Mod4</kbd>+<kbd>H/J/K/L</kbd>                     | Directional window focus                        |
| <kbd>Mod4</kbd>+<kbd>Shift</kbd>+<kbd>H/J/K/L</kbd>    | Move window directionally                       |
| <kbd>Mod4</kbd>+<kbd>Tab</kbd>                          | Focus next window                               |
| <kbd>Mod4</kbd>+<kbd>Shift</kbd>+<kbd>Tab</kbd>         | Focus previous window                           |
| <kbd>Mod4</kbd>+<kbd>+</kbd>                            | Increase the focused BSP split ratio            |
| <kbd>Mod4</kbd>+<kbd>-</kbd>                            | Decrease the focused BSP split ratio            |

### Key modes

Actions named `mode_<name>` switch to the matching `[modes.<name>]` shortcut
layer. Mode shortcuts are exact bindings and do not inherit `mod_screens` or
`mod_workspaces`. Every mode must define `mode_default` so there is always a
binding that returns to the normal `[keys]` layer.

```toml
[keys]
mode_resize = "Mod4-Shift-r"

[modes.resize]
mode_default = ["Escape", "Return"]
proportion_left = ["Mod4-h", "Mod4-Left"]
proportion_down = ["Mod4-j", "Mod4-Down"]
proportion_up = ["Mod4-k", "Mod4-Up"]
proportion_right = ["Mod4-l", "Mod4-Right"]
```

Hot corner events (configured under `[corners]`):
| Corner                             | Description                          |
| ---------------------------------- | ------------------------------------ |
| <kbd>Top</kbd>-<kbd>Left</kbd>     | Focus previous window                |
| <kbd>Top</kbd>-<kbd>Right</kbd>    | Focus next window                    |
| <kbd>Bottom</kbd>-<kbd>Right</kbd> | Increase the focused BSP split ratio |
| <kbd>Bottom</kbd>-<kbd>Left</kbd>  | Decrease the focused BSP split ratio |

Systray events (configured under `[systray]`):
| Pointer                            | Description                          |
| ---------------------------------- | ------------------------------------ |
| <kbd>Middle</kbd>-<kbd>Click</kbd> | Toggle between enable and disable    |
| <kbd>Scroll</kbd>-<kbd>Up</kbd>    | Focus previous window                |
| <kbd>Scroll</kbd>-<kbd>Down</kbd>  | Focus next window                    |
| <kbd>Scroll</kbd>-<kbd>Right</kbd> | Increase the focused BSP split ratio |
| <kbd>Scroll</kbd>-<kbd>Left</kbd>  | Decrease the focused BSP split ratio |

Common pointer shortcuts:
- Move window: <kbd>Alt</kbd>+<kbd>Left-Click</kbd>
- Resize window: <kbd>Alt</kbd>+<kbd>Right-Click</kbd>
- Maximize window: <kbd>Alt</kbd>+<kbd>Double-Click</kbd>

## Addons
External processes can communicate with bsptile via [dbus](https://en.wikipedia.org/wiki/D-Bus).

### D-Bus
Running `bsptile` starts a dbus server exposing internal properties and method calls.
A built-in dbus client can be started as a secondary process via `bsptile dbus -...` to listen for events and execute remote procedure calls.

See `bsptile dbus -help` for available properties and methods.

### Python
Python bindings wrap the dbus interface in easy-to-use methods.
See the [bsptile-addons](https://github.com/billmakes/bsptile-addons) repository for example scripts.

## Development
You need [go >= 1.22](https://go.dev/dl/) to build bsptile.

Clone and build:
```bash
git clone https://github.com/billmakes/bsptile.git
cd bsptile
make build   # produces ./bsptile
```

Install to \`~/.local/bin\`:
```bash
make install
```

Or install directly from the develop branch without cloning:
```bash
go install github.com/billmakes/bsptile/v2@develop
```

Run in verbose mode:
```bash
./bsptile -v
# or after install:
bsptile -v
```

## Debugging
- Run with `bsptile -vv` for additional debug output.
- Log file is created at `/tmp/bsptile.log` by default.

## Security
- The dbus API exposes internal window properties. Disable with `bsptile disable-dbus-interface`.
- Scripts in `~/.config/bsptile/addons/` are executed on startup. Disable with `bsptile disable-addons-folder`.
- Do not run bsptile as root.

## License
[MIT](https://github.com/billmakes/bsptile/blob/main/LICENSE)
