# displayctl

> A unified display-mode, DPI and **multi-monitor layout** manager for Linux X11 sessions.
>
> 统一的 Linux X11 显示模式、DPI 与**多屏布局**管理工具。

It combines two things that are usually separate:

1. **Multi-monitor layout management** — auto-discover monitors, remember a
   setup per hardware combination (like `autorandr`), mirror/extend/single
   modes, per-output position/resolution, persisted profiles.
2. **Virtual-session DPI management** — the original use case: a shared
   `Virtual-1` output used by XRDP / Sunshine / a local monitor, where EDID
   never changes so `autorandr` is useless, and XRDP dynamically resizes the
   screen requiring automatic DPI refresh.

`displayctl` handles both through **fingerprint-driven profile selection**, an
**event-driven design**, and a **RandR daemon**.

---

## Quick start

```sh
displayctl apply auto          # auto-select the right profile for current hardware
displayctl layout extend       # quick: extend all monitors (preferred modes, auto position)
displayctl layout mirror       # quick: mirror all monitors on a common mode
displayctl layout single DP-1  # quick: only DP-1, everything else off
displayctl save dock-extend    # persist the CURRENT state as a named profile
displayctl list                # show profiles (★ = matches current hardware)
displayctl current             # show live layout + DPI
displayctl daemon              # watch hotplug → re-apply auto; refresh DPI on resize
```

All commands accept `--json` where useful (`list --json`, `current --json`) for
scripting / rofi integration.

---

## How `apply auto` selects a profile

```
discover connected outputs (xrandr --verbose, incl. EDID)
├─ any output has no EDID?  (Virtual-1 / XRDP / Sunshine)
│     → apply the default=true profile          (legacy virtual behavior)
└─ all outputs have EDID    (physical monitors)
      ├─ fingerprint matches a saved profile?  → apply it (full state)
      └─ no match (unknown combination)?       → auto-arrange extend layout
                                                 + hint: `displayctl save <name>`
```

- **Fingerprint** = sha256 over the sorted set of connected outputs'
  `name:edidhash` labels. Order-independent; different monitor on the same port
  → different fingerprint.
- **Full state** (`manage = "full"`, the default): applying a profile turns
  **off** any connected output the profile does not list. Use `manage =
  "partial"` for virtual profiles that must not disable anything.
- **Auto-arrange** (unknown combo): internal output (`eDP-`/`LVDS-`) becomes
  primary at `0x0`, other outputs chained to the right at preferred modes. The
  generated layout is temporary — run `save` to persist it.

The **daemon** recomputes the fingerprint on every (debounced) RandR event:
if it changed (plug/unplug) it re-runs `apply auto`; if unchanged (an XRDP
resize) it only refreshes the global DPI. Because applying a layout never
changes the connected set, this gate prevents re-apply loops.

---

## Configuration

### Paths

| Path | Default | Override |
|------|---------|----------|
| Config dir | `~/.config/display` | `DISPLAYCTL_DIR` env var |
| Profiles | `<dir>/profiles/*.toml` | — |
| Post-switch hooks | `<dir>/post-switch.d/` | — |

### v2 profile format (multi-output)

```toml
# profiles/dock-extend.toml
default = false
layout = "extend"            # informational: extend|mirror|single|custom
manage = "full"              # full = turn off unlisted connected outputs

[match]                      # binds this profile to a hardware combination
id = "9f2a7c1bdeadbeef"      # fingerprint (written by `save`, don't hand-edit)
outputs = ["DP-1:ab12cd34", "eDP-1:ef56ab78"]   # human-readable, for debugging

[outputs.eDP-1]              # one table per output, keyed by xrandr name
mode = "2560x1600"           # "WxH" | "current" | "preferred" | "off"
rate = 120                   # optional refresh rate
pos = "0x0"                  # geometry (mouse crosses screens via this)
primary = true               # primary output = DPI reference + i3 main screen
# rotate = "normal"          # optional: normal|left|right|inverted
# scale = 1.0                # optional xrandr --scale (bitmap transform; can blur)

[outputs.DP-1]
mode = "3840x2160"
pos = "2560x0"

[dpi]                        # X11 has ONE global DPI per screen
tiers = true                 # auto from reference width, or: value = 144
reference = "eDP-1"          # output the DPI keys off; default = primary
```

#### mode vocabulary

| value | xrandr action |
|-------|---------------|
| `"WxH"` | `--mode WxH` |
| `"preferred"` / `"auto"` | `--auto` (display's native mode) |
| `"current"` | keep current mode (only adjust DPI/position) |
| `"off"` | `--off` (disable this output) |

#### DPI tiers

| reference width | DPI |
|-----------------|-----|
| ≥ 3000 | 192 |
| ≥ 2700 | 168 |
| ≥ 2000 | 144 |
| < 2000 | 96 |

> **Mixed-DPI note:** X11 cannot do true per-monitor DPI — there is one global
> `Xft.dpi`. `reference` picks which screen the global DPI favours (usually the
> primary/internal). Per-output `scale` is an optional bitmap-transform escape
> hatch but introduces blur, so it is off by default.

### Legacy v1 format (still supported, read-only)

The original single-output `[output]` profiles (xrdp / sunshine / monitor) keep
working. A profile is legacy when it has `[output]` and no `[outputs]`.

```toml
default = true
[output]
name = "Virtual-1"
mode = "current"     # "current" = skip mode change, only set DPI
[dpi]
tiers = true
```

---

## CLI

```
displayctl apply <profile|WxH|auto>      # apply profile / temp WxH / auto-select
displayctl layout <extend|mirror|single <output>>   # quick layout, not saved
displayctl save <name>                   # persist current state as a profile
displayctl list [--json]                 # list profiles (★ = current match)
displayctl current [--json]              # show live layout + DPI
displayctl daemon                        # hotplug re-apply + DPI refresh
```

### `apply` argument modes

| Argument | Behavior |
|----------|----------|
| `auto` | fingerprint-match → default (virtual) → auto-arrange (unknown) |
| `<profile-name>` | load `profiles/<name>.toml` and apply |
| `<WxH>` | apply mode to the active output with tiers DPI (legacy temp mode) |

---

## Event-driven design

`displayctl` does **not** sniff environment variables or processes to guess the
context. The event source explicitly calls the right thing:

| Scenario | Trigger | Command |
|----------|---------|---------|
| i3 startup | i3 `exec_always` | `displayctl apply auto` |
| Monitor plug/unplug | `displayctl daemon` (RandR) | auto re-apply (fingerprint gate) |
| XRDP login | XRDP `startwm.sh` | `displayctl apply xrdp` |
| XRDP client resize | `displayctl daemon` (RandR) | DPI refresh |
| Sunshine connect | Sunshine `on_connect` | `displayctl apply sunshine` |
| Manual switch | terminal / rofi | `displayctl apply <name>` / `layout …` |

## Post-switch hooks

Executable files in `~/.config/display/post-switch.d/` (sorted by name) run
after every apply, receiving:

- `DISPLAYCTL_OUTPUT` — reference output name
- `DISPLAYCTL_MODE` — resolved mode
- `DISPLAYCTL_DPI` — new DPI value
- `DISPLAYCTL_XCURSOR_SIZE` — computed Xcursor size

---

## Installation

### Via mise (recommended)

```toml
# ~/.config/mise/conf.d/displayctl.toml
[tools]
"go:github.com/zgs225/displayctl" = "v2.0.0"
```

> For non-registry Go tools the `go:` prefix must be the **key**, not the value.
> Pin a specific tag (not `latest`) to avoid Go proxy pseudo-version mismatches.

### Build from source

```sh
git clone https://github.com/zgs225/displayctl.git
cd displayctl
go build -o displayctl .
```

### Runtime dependencies

Shells out to `xrandr` and `xrdb` (present on any X11 system).

---

## Architecture

```
displayctl/
├── main.go
├── cmd/
│   ├── root.go            # Cobra root
│   ├── apply.go           # apply + apply-auto (resolveAuto shared with daemon)
│   ├── apply_v2.go        # v2 multi-output apply (Discover+Build+Apply+DPI+hooks)
│   ├── layout.go          # extend/mirror/single quick layouts
│   ├── save.go            # persist current state
│   ├── list.go            # list [--json]
│   ├── current.go         # current [--json]
│   └── daemon.go          # RandR watch, fingerprint-gated re-apply, DPI refresh
├── config/defaults.go     # paths
└── internal/
    ├── xrandr/            # xrandr CLI wrapper + pure Parse/ScreenSize/Output
    ├── fingerprint/       # hardware-combination id (sha256 over output labels)
    ├── profile/           # v2 TOML schema, legacy compat, Save/Load/Find
    ├── layout/            # pure planning: Build, AutoArrange, Mirror, Single,
    │                      #   Capture, ResolveAuto
    ├── dpi/               # DPI calculation + xrdb/xrandr
    ├── hook/              # post-switch.d execution
    └── randr/             # xgb RandR event listener
```

Core logic (parsing, fingerprinting, profile schema, layout planning,
auto-selection) is **pure and unit-tested**; the effectful layer (xrandr/xrdb
execution, xgb events) is kept thin.

### Dependencies

| Library | Purpose |
|---------|---------|
| [`github.com/spf13/cobra`](https://github.com/spf13/cobra) | CLI framework |
| [`github.com/BurntSushi/xgb`](https://github.com/BurntSushi/xgb) | X11/RandR protocol |
| [`github.com/pelletier/go-toml/v2`](https://github.com/pelletier/go-toml) | TOML parsing |

---

## Testing

```sh
go test ./...
```

Parser tests use real `xrandr` fixtures captured from a hybrid-GPU laptop
(internal BOE panel + Dell U2720QM 4K).

---

## License

MIT

## See also

- [`autorandr`](https://github.com/phillipberndt/autorandr) — EDID-based
  profile switching. Works for physically distinct monitors but fails when EDID
  is fixed (the `Virtual-1` case). `displayctl`'s fingerprint matching covers
  the physical case while its explicit-call model covers the virtual case.
