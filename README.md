<div align="center">

# lintop

### The Linux system monitor for your terminal — with first-class NVIDIA GPU support

`htop` + `nvtop` + `powermetrics`, in one fast TUI. Real-time CPU, memory, disk, network,
temperatures, processes **and** NVIDIA GPU metrics (utilization, VRAM, power, clocks) — written in Go.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Platform: Linux](https://img.shields.io/badge/platform-linux-blue.svg)](#supported-platforms)
[![NVIDIA GPU](https://img.shields.io/badge/NVIDIA-CUDA-76B900.svg?logo=nvidia&logoColor=white)](#nvidia-gpu-support)
[![Go](https://img.shields.io/badge/go-1.25%2B-00ADD8.svg?logo=go&logoColor=white)](https://go.dev)
[![GitHub stars](https://img.shields.io/github/stars/iamhsouna/lintop?style=social)](https://github.com/iamhsouna/lintop/stargazers)
[![GitHub issues](https://img.shields.io/github/issues/iamhsouna/lintop)](https://github.com/iamhsouna/lintop/issues)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

</div>

---

**lintop** is an open-source, real-time **Linux monitoring tool** for the terminal.
It is a Linux port of [mactop](https://github.com/metaspartan/mactop) rebuilt around Linux
interfaces (`/proc`, `/sys`, RAPL, hwmon) and [NVIDIA `nvidia-smi`](https://developer.nvidia.com/nvidia-system-management-interface).
If you searched for a **terminal GPU monitor**, an **htop with GPU**, an **nvtop alternative**,
or a **lightweight Linux `top` in Go**, lintop is for you.

It runs entirely in your terminal, needs no daemon, and can also emit **JSON / YAML / XML / CSV / TOON**
and expose **Prometheus metrics** for dashboards and alerting.

## Table of contents

- [Why lintop?](#why-lintop)
- [Features](#features)
- [Screenshots & demo](#screenshots--demo)
- [One-line install](#one-line-install)
- [One-line update](#one-line-update)
- [Quick start](#quick-start)
- [NVIDIA GPU support](#nvidia-gpu-support)
- [lintop vs htop vs nvtop](#lintop-vs-htop-vs-nvtop)
- [Headless & Prometheus](#headless--prometheus)
- [CLI flags](#cli-flags)
- [Keyboard shortcuts](#keyboard-shortcuts)
- [Configuration](#configuration)
- [Build from source](#build-from-source)
- [Supported platforms](#supported-platforms)
- [Troubleshooting / FAQ](#troubleshooting--faq)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)
- [Developer](#developer)
- [Donate](#donate)

## Why lintop?

- **One tool instead of three.** CPU, memory, network, disk, temperatures and NVIDIA GPU
  in a single, fast terminal UI — no juggling `htop`, `nvtop`, `nvidia-smi`, and `sensors`.
- **Built for GPU workstations and AI/ML boxes.** See GPU utilization, VRAM, power draw, SM clock,
  temperature and per-process GPU attribution next to the rest of the system.
- **Scriptable.** `--headless` outputs machine-readable data in JSON, YAML, XML, CSV or TOON.
- **Observable.** Built-in Prometheus exporter for Grafana dashboards.
- **Zero dependencies at runtime.** A single static binary. No Python, no Node, no daemon.
- **Free and open source (MIT).**

## Features

| Area | What you get |
| --- | --- |
| **NVIDIA GPU** | Utilization %, VRAM used/total, temperature, power (W), SM clock, fan %, per-process GPU share, estimated FP32/FP16 TFLOPS |
| **CPU** | Total and per-core usage, core topology, load, CPU package/DIMM temperatures |
| **Power** | GPU power via `nvidia-smi`; CPU package power via RAPL where readable |
| **Memory** | RAM used/available/total, swap used/total |
| **Processes** | PID, user, virtual/resident memory, CPU %, GPU %, memory %, runtime, command; search, sort and kill (F9) |
| **Disk** | Read/write throughput and IOPS, per-filesystem usage |
| **Network** | Per-interface RX/TX throughput and link speed (Wi-Fi and Ethernet) |
| **Temperatures** | Grouped hwmon sensors: CPU, GPU, SSD/NVMe, board, ambient |
| **Battery** | Charge level and charging state on laptops |
| **UI** | 20 layouts, 14+ color themes, custom hex themes, light/dark detection, party mode |
| **Output** | TUI, headless JSON/YAML/XML/CSV/TOON, Prometheus `/metrics` |
| **Operations** | `lintop --update` self-update, one-line installer, persistent config |

## Screenshots & demo

> Run `lintop` in any terminal and press `?` for help, `l` to cycle 20 layouts, `c` to cycle themes.

```text
 lintop  •  AMD Ryzen 9 5950X 16-Core Processor  •  32C (32P)  •  NVIDIA GeForce RTX 3090  •  121 GB

┌─ CPU 32 cores (32P)  12.4% @ 3.4 GHz  54°C ───┐ ┌─ GPU Usage  18% @ 210MHz (51°C) ─────────────┐
│ P0  [❚❚❚❚❚        ] 24.1%                      │ │ P8  [❚❚❚          ] 11.2%                   │
│ P1  [❚❚❚          ] 13.7%                      │ │ P9  [❚            ]  4.8%                   │
│ ...                                            │ │ ...                                          │
└────────────────────────────────────────────────┘ └──────────────────────────────────────────────┘

┌─ Power ────────────────────────────────────────┐ ┌─ Memory ─────────────────────────────────────┐
│ CPU: 0.00 W | GPU: 35.06 W                     │ │ 7.6 GB / 121.4 GB  Swap: 0.0 / 8.0 GB       │
│ DRAM: 0.00 W                                   │ │                                              │
│ Total: 35.06 W                                 │ │                                              │
└────────────────────────────────────────────────┘ └──────────────────────────────────────────────┘

┌─ Process List ─────────────────────────────────────────────────────────────────────────────────────┐
│   PID USER      VIRT    RES    CPU    GPU    MEM        TIME CMD                                  │
│  1234 hsouna   12.3G  512M  42.1%   9.4%   0.4%       1m20s python3                              │
└────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## One-line install

**Linux (x86_64 / arm64):**

```bash
curl -fsSL https://raw.githubusercontent.com/iamhsouna/lintop/main/install.sh | bash
```

The installer downloads the latest release, verifies the archive, and installs `lintop`
to `/usr/local/bin` (or `~/.local/bin` when it needs no root). If no release is available yet,
it automatically falls back to building from source (requires Go).

Custom location or a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/iamhsouna/lintop/main/install.sh | bash -s -- --prefix "$HOME/.local/bin"
curl -fsSL https://raw.githubusercontent.com/iamhsouna/lintop/main/install.sh | bash -s -- --version v2.1.5
```

## One-line update

Keep lintop up to date with either command:

```bash
lintop --update
```

```bash
curl -fsSL https://raw.githubusercontent.com/iamhsouna/lintop/main/install.sh | bash -s -- --update
```

`lintop --update` checks the latest GitHub release, downloads the correct binary for your
architecture, and atomically replaces the running executable. Use `sudo lintop --update` if
lintop is installed in a root-owned directory.

## Quick start

```bash
lintop                       # launch the TUI
lintop --interval 500        # refresh twice per second
lintop --foreground green    # pick a theme color
lintop --headless --count 1  # print one JSON sample and exit
lintop -p 2112               # expose Prometheus on :2112/metrics
```

## NVIDIA GPU support

lintop talks to the NVIDIA driver through **`nvidia-smi`**, which ships with the standard driver.

**Requirements**

- An NVIDIA GPU with a recent proprietary driver (`nvidia-smi` on `PATH`).
- CUDA/TensorRT are **not** required — only the driver utilities.

**What is collected**

| Metric | Source |
| --- | --- |
| GPU utilization % | `nvidia-smi --query-gpu=utilization.gpu` |
| VRAM used / total | `nvidia-smi --query-gpu=memory.used,memory.total` |
| GPU temperature | `nvidia-smi --query-gpu=temperature.gpu` |
| Power draw (W) | `nvidia-smi --query-gpu=power.draw` |
| SM clock / max SM clock | `nvidia-smi --query-gpu=clocks.sm,clocks.max.sm` |
| Fan speed % | `nvidia-smi --query-gpu=fan.speed` |
| Per-process GPU share | `nvidia-smi --query-compute-apps=pid,used_memory` |
| FP32/FP16 TFLOPS estimate | CUDA-core table × max SM clock |

> Verified on an **NVIDIA GeForce RTX 3090** (Ampere) running Ubuntu. Multi-GPU hosts report the
> primary GPU in the TUI while aggregate VRAM/TFLOPs include every detected GPU.

If `nvidia-smi` is missing or no GPU is present, GPU fields are simply zero and the rest of
lintop keeps working.

## lintop vs htop vs nvtop

| Capability | lintop | htop | nvtop |
| --- | :---: | :---: | :---: |
| CPU / memory / process list | ✅ | ✅ | — |
| NVIDIA GPU utilization & VRAM | ✅ | — | ✅ |
| GPU power, clock, temperature | ✅ | — | ✅ |
| Per-process GPU attribution | ✅ | — | ✅ |
| Disk & network throughput | ✅ | partial | — |
| Temperature sensors (hwmon) | ✅ | — | partial |
| Battery status | ✅ | — | — |
| Prometheus metrics | ✅ | — | — |
| Headless JSON/YAML/XML/CSV | ✅ | — | — |
| Single static binary, no daemon | ✅ | ✅ | ✅ |

Use lintop when you want the whole machine — CPU *and* NVIDIA GPU — in one screen.

## Headless & Prometheus

```bash
lintop --headless --count 1 --pretty          # one pretty JSON object
lintop --headless --format yaml               # streaming YAML
lintop --headless --format csv --count 10     # CSV with a header row
lintop -p 2112                                 # Prometheus exporter
```

Scrape config:

```yaml
scrape_configs:
  - job_name: lintop
    static_configs:
      - targets: ["localhost:2112"]
```

## CLI flags

| Flag | Description |
| --- | --- |
| `-u`, `--update` | Check for updates and self-update from GitHub |
| `-v`, `--version` | Print the version and exit |
| `-h`, `--help` | Show help |
| `-i`, `--interval <ms>` | Refresh interval in milliseconds (default `1000`) |
| `--headless` | No TUI; print samples to stdout |
| `--format <fmt>` | `json` (default), `yaml`, `xml`, `csv`, `toon` |
| `--count <n>` | Number of samples in headless mode (`0` = infinite) |
| `--pretty` | Pretty-print headless output |
| `-p`, `--prometheus <port>` | Enable the Prometheus metrics server |
| `--foreground <color>` | Theme color (named or `#hex`) |
| `--bg <color>` | Background color (named or `#hex`) |
| `--unit-network <unit>` | `auto`, `byte`, `kb`, `mb`, `gb` |
| `--unit-disk <unit>` | `auto`, `byte`, `kb`, `mb`, `gb` |
| `--unit-temp <unit>` | `celsius` (default), `fahrenheit` |
| `--pid <pid>` | Monitor a single process |
| `--lang <code>` | UI language (20 languages, auto-detected) |

## Keyboard shortcuts

| Key | Action |
| --- | --- |
| `q` | Quit |
| `?` / `h` | Toggle help |
| `l` / `L` | Cycle layouts forward / backward |
| `c` / `C` | Cycle foreground colors |
| `b` / `B` | Cycle background colors |
| `p` | Party mode 🌈 |
| `i` | Info layout |
| `F` | Fan & thermals layout |
| `a` | SoC/history layout |
| `+` / `-` | Slower / faster refresh |
| `/` | Search processes |
| `F9` | Kill the selected process |
| `f` | Freeze the process list |
| `↑ ↓ ← →`, `j` `k` | Navigate |
| `Enter` / `Space` | Sort by column |

## Configuration

lintop persists your layout, theme, interval and sort preferences.

| File | Path |
| --- | --- |
| Config | `$XDG_CONFIG_HOME/lintop/config.json` (or `~/.lintop/config.json`) |
| Theme | `$XDG_CONFIG_HOME/lintop/theme.json` (or `~/.lintop/theme.json`) |
| Log | `$XDG_STATE_HOME/lintop/lintop.log` (or `~/.lintop/lintop.log`) |

Custom theme example (`theme.json`):

```json
{
  "foreground": "#9580FF",
  "background": "#22212C",
  "cpu": "#FF5252",
  "gpu": "#448AFF",
  "memory": "#69F0AE",
  "network": "#FFAB40",
  "power": "#FF6E40"
}
```

## Build from source

```bash
git clone https://github.com/iamhsouna/lintop.git
cd lintop
make build          # produces ./lintop
make test           # run the test suite
sudo make install   # install to /usr/local/bin
```

Or directly:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o lintop .
```

The Linux build is pure Go — no CGO, no C toolchain required.

## Supported platforms

| OS | Arch | Status |
| --- | --- | --- |
| Linux | x86_64 / amd64 | ✅ Fully supported |
| Linux | arm64 / aarch64 | ✅ Fully supported |
| Linux on Apple Silicon (Asahi) | arm64 | ✅ Runs; ANE panels shown |
| macOS (Apple Silicon) | arm64 | Use [mactop](https://github.com/metaspartan/mactop) |

Only NVIDIA GPUs are supported for GPU metrics today. AMD/Intel GPU support is on the roadmap.

## Troubleshooting / FAQ

**GPU shows 0% / no GPU data.**
Check that `nvidia-smi` works on its own: `nvidia-smi`. If it is not on `PATH`, install the
NVIDIA driver (not just the CUDA toolkit).

**CPU power is always 0.**
Package power comes from the RAPL powercap interface, which is often root-only
(`/sys/class/powercap/*/energy_uj`). Run `sudo lintop` to read CPU wattage. GPU power
always works without root.

**No temperature sensors shown.**
Install `lm-sensors` and run `sudo sensors-detect`, then reload. lintop reads
`/sys/class/hwmon`.

**Fan control doesn't work.**
Fan control is a macOS/SMC-only feature; on Linux the fan panel is read-only by design.

**`lintop --update` says permission denied.**
lintop is installed in a root-owned directory. Run `sudo lintop --update`, or reinstall
with `--prefix "$HOME/.local/bin"`.

**Why is the ANE gauge missing?**
The Apple Neural Engine exists only on Apple Silicon. lintop hides ANE panels on other
hardware; they appear automatically on macOS or on Apple Silicon running Linux (Asahi).

## Roadmap

- [ ] AMD (ROCm/`rocm-smi`) and Intel (`intel_gpu_top`) GPU support
- [ ] Per-GPU selection and multi-GPU TUI panes
- [ ] Prebuilt release binaries and install script automation (in progress)
- [ ] Config profiles and saved layouts
- [ ] More languages and themes

## Contributing

Contributions, bug reports and feature requests are welcome!

1. Fork the repo and create a branch: `git checkout -b feature/amazing`
2. Keep the build green: `make build && make test`
3. Open a pull request with a clear description.

If lintop is useful to you, please ⭐ **star the repository** — it helps other Linux and
NVIDIA users find it.

## License

Distributed under the **MIT License**. See [`LICENSE`](LICENSE) for details.

lintop is a Linux port of [mactop](https://github.com/metaspartan/mactop) by Carsen Klock.

---

<h2 align="center">Developer</h2>

<p align="center">
  <strong>Hsouna Zinoubi</strong><br/>
  GitHub: <a href="https://github.com/iamhsouna">@iamhsouna</a>
</p>

<p align="center">
  Built and maintained with ❤️ for the Linux and NVIDIA community.
</p>

<h2 align="center">Donate</h2>

<p align="center">
  If lintop saves you time, consider supporting development.<br/>
  <strong>USDT (TRC20 / Tron)</strong>
</p>

<p align="center">
  <code>TP7Ma1rpi16GjT4GhHAFGb6cK7TBTXqJHH</code>
</p>

<p align="center">
  ⚠️ Send only <strong>USDT on the TRON (TRC20)</strong> network to this address.
</p>
