# bsptile improvement plan

A prioritized, behavior-preserving punch list compiled from a read-through of
the codebase. Each item names the file, the concrete change, why it's worth
doing, and an honest risk assessment. Items are grouped by tier; within a tier
they're ordered roughly by "most value per line of change."

The guiding constraint is **no behavior changes for users**. Anything that
would alter the UX (new config keys, removed features, etc.) is explicitly
called out.

---

## P0 — Real bugs / resource leaks

These are concrete defects in the code today. Each is small and isolated.

### 1. `common/info.go` — HTTP response body leak in `FetchReleases` / `FetchIssues`
- **Where:** `common/info.go:119–135` and `common/info.go:205–222`.
- **What:** Both functions call `http.Get(...)` and `io.ReadAll(response.Body)`
  but never `defer response.Body.Close()`. Every daemon startup (and any future
  call site) leaks a TCP connection plus the underlying file descriptor until
  Go's GC runs the body finalizer.
- **Fix:** Add `defer response.Body.Close()` immediately after the error
  check, matching the pattern already used in `ui/updater.go:109`.
- **Risk:** Trivial. Pure cleanup — no behavior change.

### 2. `common/info.go` — Unchecked type assertions on GitHub JSON
- **Where:** `common/info.go:142–146` (`FetchReleases`) and
  `common/info.go:228–233` (`FetchIssues`).
- **What:** Code reads
  `data["id"].(float64)`, `data["html_url"].(string)`, `data["tag_name"].(string)[1:]`.
  `IsInMap` only checks for the key's presence — it does **not** verify
  the value's type or that `tag_name` is non-empty. If GitHub ever returns
  a different shape (null tag, rate-limit JSON, error body), the daemon
  panics at startup before X is even attached.
- **Fix:** Use comma-ok assertions, guard the `[1:]` slice with a length check,
  and bail out gracefully (return the empty `releases` / `issues` slice).
- **Risk:** Low. The "happy path" stays identical; only failure modes change.

### 3. Control socket file is never cleaned up on `exit`/`restart`
- **Where:** `input/action.go:1004–1024` (`Exit`) and `input/action.go:982–1002`
  (`Restart`); `socket/server.go:246–258` (`Server.Close`).
- **What:** `Server.Close()` exists and even has correct ownership checks
  (`removeOwnedSocket`), but it is never wired up. `Exit()` calls
  `Disconnect()` (D-Bus) and `os.Exit(0)`. `Restart()` ends with
  `syscall.Exec(...)`. In both cases the Unix socket file is left on disk.
- **Symptom:** On next start, `removeStaleSocket` cleans it up, so users
  don't usually notice. But the daemon's shutdown is unhygienic and may
  leave a stale socket if the cleanup path ever changes.
- **Fix:** Have `socket.Init` register the server somewhere (e.g. a package-level
  pointer, or returned to `main.go` and stored), then call `srv.Close()` in
  `Exit` and `Restart` *before* exec/exit. Alternative: return the server from
  `socket.Init` (it already does), keep it on the tracker, and let
  `input.Disconnect()` invoke `Close()`.
- **Risk:** Low if implemented as a single `*Server` field on the tracker.
  Make sure `Close()` runs *before* `syscall.Exec` so the new process can
  bind cleanly. Already covered by the ownership check.

### 4. Dead `Client.Locked` flag — pure noise that survives every refactor
- **Where:** `store/client.go:28, 66, 70–76, 206`.
- **What:** `Client.Lock()` / `UnLock()` set `Locked`, and `MoveWindow` checks
  it; but `Lock()` is never called from anywhere in the codebase (confirmed
  by `grep`). `unlockClients` in `desktop/tracker.go` calls `c.UnLock()` but
  there is nothing to unlock. The lock-rejection branch in `MoveWindow`
  cannot fire.
- **Fix:** Delete `Lock`, `UnLock`, the `Locked` field, the `if c.Locked`
  branch in `MoveWindow`, and `unlockClients` in `tracker.go`. Adjust the
  two callers of `unlockClients` (`onStateUpdate`, `onPointerUpdate`)
  accordingly.
- **Risk:** Low if grep confirms no external test/depender; this is pure
  dead code. Worth verifying tests still pass.

### 5. `store/client.go::MoveToScreenDirect` — unchecked screen-index access
- **Where:** `store/client.go:180–189`.
- **What:** `geom := Workplace.Displays.Screens[screen].Geometry` — no bounds
  check. Compare with `CenterOnScreen()` 10 lines down which does check
  `int(screen) >= len(Workplace.Displays.Screens)`. Callers in
  `desktop/tracker.go:483` and `input/action.go:841` already check bounds
  themselves, but `input/dbusbinding.go::WindowToScreen` only checks
  `< ScreenCount` — fine — and applyWindowRulePlacement also checks. Still,
  defensive consistency matters because the panic path bypasses recovery.
- **Fix:** Mirror `CenterOnScreen`'s guard and return `false` on out-of-range.
- **Risk:** Trivial. Behavior on the happy path is unchanged.

### 6. `common/info.go` — Synchronous HTTP at startup blocks daemon launch
- **Where:** `common/info.go:102–104` invoked from `InitInfo` in
  `main.go:56`.
- **What:** `FetchReleases` and `FetchIssues` are called inline during
  `InitInfo`. `http.Get` uses `http.DefaultClient`, which has **no timeout**.
  If GitHub or its DNS is slow/blocked, daemon startup hangs.
- **Fix:** Either (a) wrap calls in a `context.WithTimeout(5s)` /
  custom `http.Client{Timeout: 5 * time.Second}`, or (b) move release/issue
  fetching to a goroutine after `runMain` has bound the X event loop and
  publish results via a callback that refreshes the systray menu when ready.
  Option (a) is the minimum fix and preserves current ordering.
- **Risk:** Option (a) low; option (b) medium because the systray menu
  currently reads `Source.Releases` at build time of `items()`. Recommend (a)
  for now.

---

## P1 — Robustness and correctness, low blast radius

### 7. `store/client.go::IsIgnored` recompiles user regex on every call
- **Where:** `store/client.go:392–436`.
- **What:** For every `IsIgnored(info)` call, the function compiles each
  `window_ignore` entry's class and (optional) name regex from scratch. This
  is in the hot path: `Update()` walks every stacked window, and every call
  to `GetInfo` indirectly drives float decisions. `WindowRule` already
  compiles its pattern once into `classPattern` (see `common/config.go:194–204`);
  `window_ignore` is the same data shape but untreated.
- **Fix:** Promote `window_ignore` to a typed struct (`WindowIgnoreRule`) with
  pre-compiled patterns and validate/compile in `validateConfig` alongside
  `WindowRule`. Or, less invasively, build a small `sync.Map`-cached compile
  helper keyed by pattern string.
- **Risk:** Medium. The validation step would catch malformed entries earlier,
  which is a small UX win, but the on-disk TOML schema (`["class", "name"]`
  tuples) should stay backward-compatible. Decode into `[][]string` first,
  then walk into typed cache after validation, leaving the user-facing
  format untouched.

### 8. `store/client.go::GetInfo` is hot and unmemoized
- **Where:** `store/client.go:522–622` and most of `desktop/tracker.go`.
- **What:** `GetInfo` issues ~7 X round-trips per call (WmClassGet, WmNameGet,
  DecorGeometry, WmDesktopGet, WmWindowTypeGet, WmStateGet, WmNormalHintsGet,
  WmHintsGet, two `xprop.GetProperty`). The tracker calls `GetInfo` once per
  stacked window inside `Update()`, plus several times per property change
  (`IsMaximized(store.GetInfo(c.Window.Id))` etc.).
- **Fix:** Plumb a single `*Info` through the property handlers rather than
  re-fetching, and consider a per-client cache in tracker that the X
  `PropertyNotify` handler keeps current. Smaller win: collapse repeated
  `store.GetInfo(c.Window.Id)` in `handleResizeClient` /
  `handleMoveClient` to use `c.Latest`, since `attachHandlers` updates
  `c.Latest` from the same handler before they fire.
- **Risk:** Medium — risks introducing staleness bugs if the cache isn't
  invalidated correctly. Start with the smaller win (use `c.Latest` where
  it's already authoritative) before considering deeper caching.

### 9. `ui/updater.go::extractFile` returns `*tar.Reader` that reads through a
   `gzip.Reader` whose `Close` already deferred to fire
- **Where:** `ui/updater.go:168–194`.
- **What:** `extractFile` does `defer gzipReader.Close()` and then returns
  `tarReader`. After return, the deferred `Close()` runs and `applyUpdate`
  reads from `tarReader`, which transitively reads from the closed
  `gzipReader`. In practice `gzip.Reader.Close` doesn't actually close the
  underlying reader (the body is in a `bytes.Buffer`), so it usually works
  — but this is brittle and relies on stdlib implementation details.
- **Fix:** Move `gzipReader.Close()` to `applyUpdate`'s caller path:
  return both the `*tar.Reader` and the `*gzip.Reader` (or an `io.Closer`),
  and close in the outer `UpdateBinary` after `applyUpdate` returns.
- **Risk:** Low. The self-updater is rarely exercised; tests should cover the
  happy path of `applyUpdate` reading the full archive bytes.

### 10. `common/cache.go::InitCache` swallows `MkdirAll` errors
- **Where:** `common/cache.go:27–31`.
- **What:** On `MkdirAll` failure the code logs but does not return — later
  `os.WriteFile`/`os.Stat` calls in `info.go` silently fail. Several
  feature paths (release cache, issue cache) become no-ops with confusing
  warnings.
- **Fix:** Either flip the cache to "disabled" when the folder can't be
  created (so the rest of the code reliably no-ops via `CacheDisabled()`),
  or `log.Fatal`. Disabling is the safer choice.
- **Risk:** Trivial; failure semantics get clearer.

### 11. `MoveWindow`/`MoveResizeXWindow` failure mode: silent no-op
- **Where:** `store/client.go:230–234`, `store/ewmh.go:45–55`.
- **What:** `MoveResizeXWindow` returns `false` on `w <= 0 || h <= 0` and on
  EWMH errors, but `MoveWindow` (the *Client* method) does not propagate that
  result. The tile loop in `store/manager.go:Apply` assumes the move
  succeeded and updates `c.Latest.Dimensions.Geometry` regardless. If the
  move failed (window destroyed mid-tile, X disconnected), bsptile's
  geometry record diverges from reality.
- **Fix:** Have `Client.MoveWindow` return `bool` and propagate so
  `applyNode` can skip the `Latest.Dimensions.Geometry` write on failure.
  Alternative: re-fetch geometry after a failure marker.
- **Risk:** Low — narrows a correctness gap. Hard to write a regression test
  without an X harness, but the change is local.

### 12. `ui/overlay.go::drawText` infinite-loop guard
- **Where:** `ui/overlay.go:154–170`.
- **What:** `drawText` recursively decrements `size` if the rendered width
  exceeds the available area. No floor — a tiny canvas plus a long string
  will keep recursing until `size <= 0`, which `Extents` may also fail to
  bound. Probably never hits in practice because the UI canvas is sized
  generously, but it's an unbounded recursion.
- **Fix:** Stop recursing when `size <= 6` (or pick a sensible minimum) and
  truncate the text instead.
- **Risk:** Trivial — only the degenerate case changes.

### 13. `desktop/tracker.go::onPointerUpdate` timer math
- **Where:** `desktop/tracker.go:836–846`.
- **What:** `var t time.Duration = 0` is multiplied by `time.Millisecond`
  later (`t * time.Millisecond`). When `t = 0`, the multiplication is `0`,
  and `time.AfterFunc(0, ...)` fires immediately, which is intended. But the
  unit-mixing pattern (a `time.Duration` named `t` carrying a plain integer
  count of milliseconds) is fragile and easy to break in a refactor. The
  same idiom appears in several other `time.AfterFunc` callers.
- **Fix:** Audit `time.Duration` usage. Use `time.Duration * time.Millisecond`
  only when the source value is genuinely a count of milliseconds; otherwise
  store the full `time.Duration` from the start.
- **Risk:** Behavior unchanged if done carefully. Mostly readability /
  defensiveness.

---

## P2 — Quality of life, dev experience, hygiene

### 14. Build artifacts checked into the working tree
- **Where:** `bsptile` (11 MB) and `bsptilectl` (2.5 MB) in repo root.
- **What:** Compiled binaries sit in the repo root. They aren't in
  `.gitignore` (only `.gitignore` content was 34 bytes — likely just an
  editor pattern). Anyone building locally either commits the new binaries
  by accident or has to remember to skip them.
- **Fix:** Add `/bsptile` and `/bsptilectl` to `.gitignore`. Consider also
  ignoring `/dist/` if goreleaser writes there.
- **Risk:** None.

### 15. `Makefile install` doesn't create `~/.local/bin` if missing
- **Where:** `Makefile:27–29`.
- **What:** `install -Dm755` *does* create the directory tree, so this is
  fine — no action. (Listed only to record the audit.)

### 16. Magic numbers in BSP layout / drag detection
- **Where:** `desktop/tracker.go:259` (`muteHandlers(200ms)`),
  `desktop/tracker.go:607` (`pt.Dragging(500)`),
  `input/action.go:91` (`100ms`), `input/mousebinding.go:24` (`poll(100)`),
  `ui/overlay.go:45` (`150ms`), `desktop/tracker.go:843` (`50ms`).
- **What:** A handful of inter-related timers govern how the daemon
  arbitrates between user drags and bsptile's own MoveResize echoes. They
  are all magic numbers spread across files; tuning one without breaking the
  others is risky.
- **Fix:** Group them in a `desktop/timings.go` (or a small typed const
  block in `tracker.go`) with comments describing what each one mitigates,
  so the next person to touch drag handling can see the picture in one
  place. Do *not* expose them as config — they're internal coordination.
- **Risk:** None — pure relocation. Behavior must not change.

### 17. `cmd/bsptilectl/main.go` exit-code branching is redundant
- **Where:** `cmd/bsptilectl/main.go:67–76`.
- **What:** Both branches (`!stream` and `stream`) end in `os.Exit(1)`. The
  comment explains the intent but the code can be flattened to a single
  exit.
- **Fix:** `if jerr != nil || !resp.OK { os.Exit(1) }`.
- **Risk:** None.

### 18. README & `bsptilectl --help` drift on query targets
- **Where:** `cmd/bsptilectl/main.go:158`, `README.md:206`.
- **What:** `bsptilectl --help` lists `query [workspaces|windows|clients|workplace|config]`
  while the daemon also supports `query actions` (used by the bsptilectl
  `actions` shortcut). The README is consistent, the inline help isn't.
- **Fix:** Add `actions` to the help text.
- **Risk:** None.

### 19. `socket/server.go::handleSubscribe` blocks on `conn.Read` for liveness
- **Where:** `socket/server.go:215–226`.
- **What:** A 64-byte buffer just reads until error. This works, but it's
  worth pairing with a `SetReadDeadline` heartbeat so a half-open TCP-style
  socket (e.g. systemd-suspended sockets, weird mounts) doesn't pin a
  goroutine forever. Unix sockets rarely hit this in practice, so this is
  P2 hygiene rather than a real bug.
- **Fix:** Optional. If kept, document the assumption.
- **Risk:** Low if implemented; do nothing if uncertain.

### 20. Test coverage gaps on hot paths
- **Where:** `input/action.go` (1056 lines, 166 lines of tests),
  `desktop/tracker.go` (994 lines, 101 lines of tests), `ui/updater.go`
  (no tests).
- **What:** The action dispatcher and `tryNumberedAction` are easy to
  unit-test (string in, behavior out). Tracker logic depends on X, but the
  pure helpers (`dropEdge`, `dropZoneRect`, `arrivalEdge`,
  `edgeClientForArrival`) are testable in isolation. The
  `validateChecksum`/`extractFile` helpers in `ui/updater.go` are pure and
  exercising them with a synthetic tarball would harden the update path.
- **Fix:** Add targeted unit tests for the pure helpers above. Aim for
  ~30 lines per helper.
- **Risk:** None — tests don't change runtime behavior.

### 21. `HasFlag` runtime check on `os.Args[1:]` is effectively dead
- **Where:** `common/info.go:259–261`.
- **What:** `HasFlag` looks for `name` (without leading dash) in both
  `Build.Flags` (set at link time) **and** `os.Args[1:]`. But the daemon
  uses `flag.Parse` first, which would error on an unknown `-name` flag and
  exit before `HasFlag` ever runs. So the `os.Args` branch only matches
  positional args that don't begin with `-`, which is not how anyone would
  actually pass these flags.
- **Fix:** Either register the flags properly via the `flag` package (which
  changes behavior — out of scope) or document that `disable-*-interface`
  flags are link-time only and remove the misleading `os.Args` check. The
  documentation route is safer.
- **Risk:** If the runtime path is actually depended on by anyone, removing
  it would silently break their workflow. Keep, but add a comment noting it
  only works pre-`flag.Parse` (which is never).

### 22. Inconsistent error logging idiom
- **Where:** Everywhere; e.g. `log.Warn("Error retrieving...", err)`.
- **What:** Some sites use `log.Warn("msg: ", err)`, others use
  `log.Warn("msg ", err, ...)`, others use `log.Errorf` style with `%w`
  inside `fmt.Errorf`. logrus is used throughout but format usage varies.
- **Fix:** Pick a house style — recommend `log.WithError(err).Warn("msg")`
  for structured fields when adopting later. For now, lower priority than
  the substantive items above.
- **Risk:** None if done carefully; aesthetic only.

---

## P3 — Optional / longer-horizon

### 23. `BindPointer` polls the X pointer at 10 Hz
- **Where:** `input/mousebinding.go:23–41`.
- **What:** A 100 ms ticker drives `PointerUpdate`, which issues a
  `QueryPointer` round-trip every tick. That's ~10 X round-trips per second
  even when the user isn't moving. Some of this is fundamental to the
  hover-focus design, but X has event masks that can deliver the data
  passively (e.g. `xevent.MotionNotify` on the root window).
- **Fix:** Out of scope for a "no behavior change" cleanup pass; flag as
  a future investigation. If the daemon's idle CPU footprint matters,
  this is the largest single contributor.
- **Risk:** High if attempted — hover focus, drag detection, button-state
  tracking all derive from this poll. Don't touch without a test plan.

### 24. `input/dbusbinding.go` global state vs. multiple instances
- **Where:** `input/dbusbinding.go:29–34` (package-level vars
  `iface`, `opath`, `props`, `methods`).
- **What:** All D-Bus state is package-global. Fine in practice (one daemon
  per session), but harder to test and to compose with a future "second
  bus" scenario.
- **Fix:** Wrap in a struct and pass it around. Not worth doing on its own;
  fold into a larger D-Bus refactor if and when one is needed.
- **Risk:** Medium if attempted; behavior must remain bit-identical.

### 25. `goreleaser` config not validated in CI
- **Where:** `.goreleaser.yml`, `.github/`.
- **What:** Couldn't tell whether CI runs `goreleaser check` before tag
  builds. If it doesn't, broken release configs only surface at release
  time.
- **Fix:** Add `goreleaser check` to CI if not already there.
- **Risk:** None.

---

## Suggested execution order

If picking these up incrementally, the cheapest valuable batch is **1, 2, 4,
5, 6, 10, 14, 17, 18**: all small, all behavior-preserving, and together
they remove the visible foot-guns (HTTP body leaks, GitHub-API panic,
unchecked screen index, dead code, build-artifact noise, CLI help drift,
and the startup hang).

A second batch — **3, 9, 11** — is small in lines but each touches a
critical path (socket shutdown, self-update, geometry consistency). Worth
doing in their own PRs so each can be reverted cleanly if a regression
shows up.

Items **7, 8** (regex caching, `GetInfo` reduction) are the highest-impact
performance wins but require care to avoid staleness regressions. Do them
after the P0/P1 tier is green.

**16, 20** (timing knobs, test coverage) are pure quality-of-life work.
Slot them in when shipping nearby changes anyway.

The P3 items are flagged as ideas, not commitments — each is its own
project.

---

## Out of scope (deliberately)

- New features, new config keys, new actions, new layout modes.
- Wayland support, sxhkd replacement, alternative socket protocols.
- Anything that would change the user-visible behavior of an existing
  action, config field, or rule.
- Reformatting / mass rename / linter sweep.
