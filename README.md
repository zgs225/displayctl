# displayctl

> A unified display mode and DPI manager for Linux X11 sessions — built to solve the "same `Virtual-1`, different clients" problem that autorandr can't handle.

> 统一的 Linux X11 显示模式与 DPI 管理工具 — 专为解决 autorandr 无法应对的"同一 `Virtual-1`、不同客户端"场景而设计。

---

## The Problem / 问题

A single X11 `:0` session on `Virtual-1` is shared across multiple remote clients — XRDP, Sunshine, a local physical monitor. The same output needs different modes depending on the client, but **EDID never changes**, so autorandr's EDID-matching profiles are useless. XRDP also dynamically resizes `Virtual-1` to match each connecting client's resolution, requiring automatic DPI refresh on every resize.

单一 X11 `:0` 会话在 `Virtual-1` 上被多个远程客户端共享 — XRDP、Sunshine、本地物理显示器。同一输出需要根据客户端使用不同的模式，但 **EDID 始终不变**，因此 autorandr 的 EDID 匹配机制完全失效。XRDP 还会动态调整 `Virtual-1` 的分辨率以匹配每个连接的客户端，这就需要在每次 resize 时自动刷新 DPI。

`displayctl` solves this with **event-driven profile selection** and a **RandR daemon** that auto-refreshes DPI.

`displayctl` 通过 **事件驱动的 profile 选择** 和 **RandR 守护进程** 自动刷新 DPI 来解决此问题。

---

## Features / 特性

| Feature | 特性 |
|---------|------|
| TOML-based profiles for xrdp, sunshine, local monitor | 基于 TOML 的 profile 配置文件 |
| One-shot `apply` command for profile / WxH / auto modes | 一次性 `apply` 命令，支持 profile / WxH / auto 三种模式 |
| Long-lived `daemon` that watches RandR screen-change events | 长驻 `daemon`，监听 RandR 屏幕变化事件 |
| Automatic DPI recalculation when XRDP resizes | XRDP resize 时自动重新计算 DPI |
| Post-switch hooks for downstream refresh (polybar, etc.) | post-switch hooks 用于刷新下游组件（如 polybar） |
| Single Go binary, no runtime dependencies beyond X11 tools | 单个 Go 二进制，运行时仅依赖 X11 工具 |

---

## Installation / 安装

### Build from source / 从源码构建

```bash
git clone https://github.com/yuez/displayctl.git ~/Workspace/Golang/displayctl
cd ~/Workspace/Golang/displayctl
go build -o displayctl .
cp displayctl ~/.bin/displayctl
```

### Dependencies / 运行时依赖

The binary shells out to these X11 tools (already present on any X11 system):

二进制调用以下 X11 工具（任何 X11 系统都已具备）：

- `xrandr`
- `xrdb`

---

## CLI

```
displayctl apply <profile|WxH|auto>    # Apply a named profile, a temporary WxH mode, or the default profile
displayctl list                        # List available profiles with summary
displayctl current                     # Show current xrandr mode + DPI
displayctl daemon                      # Long-lived: watch RandR events, auto-refresh DPI
```

### Apply argument modes / apply 参数模式

| Argument | Behavior | 行为 |
|----------|----------|------|
| `auto` | Find and apply the profile with `default = true` | 加载 `default = true` 的 profile |
| `<profile-name>` | Load `profiles/<name>.toml` and apply | 加载 `profiles/<name>.toml` |
| `<WxH>` (e.g. `2560x1440`) | Apply mode to active output with `dpi.tiers = true` | 对当前活动输出应用该分辨率，使用 tiers DPI |

### Exit codes / 退出码

| Code | Meaning | 含义 |
|------|---------|------|
| 0 | Success | 成功 |
| 1 | General error | 一般错误 |
| 2 | Profile not found | 未找到 profile |
| 3 | xrandr execution failed | xrandr 执行失败 |
| 4 | No default profile found (only for `auto`) | 未找到默认 profile（仅 `auto`） |

---

## Configuration / 配置

### Paths / 路径

| Path | Default | Override |
|------|---------|----------|
| Config dir | `~/.config/display` | `DISPLAYCTL_DIR` env var |
| Profiles | `<dir>/profiles/*.toml` | — |
| Post-switch hooks | `<dir>/post-switch.d/` | — |

### Profile TOML format / Profile TOML 格式

```toml
# xrdp.toml — XRDP sets the mode itself, don't override
default = false

[output]
name = "Virtual-1"
mode = "current"     # "current" = skip xrandr --mode, only set DPI

[dpi]
tiers = true         # Auto-calculate DPI from current resolution width
```

```toml
# sunshine.toml — fixed 4K streaming
default = false

[output]
name = "Virtual-1"
mode = "3840x2160"
rate = 60

[dpi]
value = 192          # Fixed DPI value; takes priority over tiers
```

```toml
# monitor.toml — local physical monitor fallback
default = true

[output]
name = "Virtual-1"
mode = "3840x2160"

[dpi]
value = 192
```

#### Field reference / 字段说明

| Field | Required | Description |
|-------|----------|-------------|
| `default` | No | `true` marks this as the profile for `apply auto`. Default: `false` |
| `output.name` | Yes | xrandr output name (e.g. `Virtual-1`) |
| `output.mode` | Yes | `WxH` string or `"current"` (skip mode change) |
| `output.rate` | No | Refresh rate. If omitted, xrandr picks default |
| `dpi.value` | No* | Fixed DPI value. Priority over `tiers` |
| `dpi.tiers` | No* | `true` = calculate DPI from resolution width |

\* At least one of `dpi.value` or `dpi.tiers` should be set. If neither, DPI is not changed.

### DPI tiers logic / DPI 分档逻辑

| Screen width | DPI | Tier |
|-------------|-----|------|
| >= 3000 | 192 | retina |
| >= 2700 | 168 | — |
| >= 2000 | 144 | hidpi |
| < 2000 | 96 | normal |

---

## Event-Driven Design / 事件驱动设计

`displayctl` **does not** auto-detect environment by sniffing env vars or processes (XRDP env vars are unreliable; Sunshine is always running regardless of client connection). Instead, the **event source explicitly calls** the profile:

`displayctl` **不**通过嗅探环境变量或进程来自动检测环境（XRDP 环境变量不可靠；Sunshine 始终在后台运行，与是否有客户端连接无关）。取而代之的是 **事件源显式调用** profile：

| Scenario | Trigger | Command |
|----------|---------|---------|
| i3 startup | i3 `exec_always` autostart | `displayctl apply auto` |
| XRDP login | XRDP `startwm.sh` | `displayctl apply xrdp` |
| XRDP client resize | `displayctl daemon` (RandR event) | Auto DPI refresh |
| Sunshine connect | Sunshine `on_connect` hook | `displayctl apply sunshine` |
| Manual switch | User terminal | `displayctl apply 2560x1440` |

---

## Post-Switch Hooks / 切换后钩子

Place executable files in `~/.config/display/post-switch.d/` (sorted by filename). Each hook receives:

将可执行脚本放入 `~/.config/display/post-switch.d/`（按文件名排序执行）。每个 hook 接收以下环境变量：

- `DISPLAYCTL_OUTPUT` — output name (e.g. `Virtual-1`)
- `DISPLAYCTL_MODE` — new mode (e.g. `3840x2160`)
- `DISPLAYCTL_DPI` — new DPI value (e.g. `192`)

Example / 示例:

```sh
# post-switch.d/10-polybar
#!/bin/sh
~/.config/polybar/launch.sh &
```

There are **no pre-switch hooks** — xrandr mode changes don't require rollback.

没有 **pre-switch 钩子** — xrandr 模式切换不需要回滚。

---

## Dotfiles Integration / 集成到 dotfiles

In a chezmoi-based dotfiles repo:

在基于 chezmoi 的 dotfiles 仓库中：

```
dot_config/display/
├── profiles/
│   ├── xrdp.toml
│   ├── sunshine.toml
│   └── monitor.toml
└── post-switch.d/
    └── (your hooks)
```

i3 autostart (`dot_config/i3/config.tmpl`):

```
exec_always --no-startup-id displayctl apply auto
exec --no-startup-id displayctl daemon
```

xinitrc (`dot_xinitrc`):

```
command -v displayctl >/dev/null 2>&1 && displayctl apply auto
exec i3
```

---

## Architecture / 架构

```
displayctl/
├── main.go                  # Entrypoint
├── cmd/
│   ├── root.go              # Cobra root
│   ├── apply.go             # apply subcommand
│   ├── list.go              # list subcommand
│   ├── current.go           # current subcommand
│   └── daemon.go            # daemon subcommand
├── config/
│   └── defaults.go          # Default paths
└── internal/
    ├── xrandr/              # xrandr CLI wrapper
    ├── dpi/                 # DPI calculation + xrdb/rofi
    ├── profile/             # TOML profile parsing
    ├── hook/                # post-switch.d execution
    └── randr/               # xgb RandR event listener
```

### Dependencies / 依赖

| Library | Purpose |
|---------|---------|
| [`github.com/spf13/cobra`](https://github.com/spf13/cobra) | CLI framework |
| [`github.com/BurntSushi/xgb`](https://github.com/BurntSushi/xgb) | X11 protocol |
| [`github.com/pelletier/go-toml/v2`](https://github.com/pelletier/go-toml) | TOML parsing |

---

## License / 许可证

MIT

---

## See Also / 相关项目

- [`autorandr`](https://github.com/phillipberndt/autorandr) — EDID-based profile switching (works well for laptops with physically different monitors; useless when EDID is fixed)
- [`xrandr`](https://wiki.archlinux.org/title/Xrandr) — the underlying CLI tool