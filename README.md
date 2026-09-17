# lintop

`lintop` is a Linux port of [mactop](https://github.com/metaspartan/mactop) — a
terminal-based `top` for real-time system monitoring. It keeps mactop's terminal
UI, layouts, themes, process management and headless/Prometheus output, but the
metrics backends are rewritten for Linux with first-class **NVIDIA GPU** support.

This port was built and verified on Ubuntu with an **NVIDIA GeForce RTX 3090**,
but works on any x86_64/ARM64 Linux host with or without an NVIDIA GPU.

## Features

- **NVIDIA GPU monitoring** via `nvidia-smi`: utilization, VRAM, temperature,
  power draw, SM clock, fan speed, per-process GPU attribution and estimated
  FP32/FP16 TFLOPs
- CPU utilization per core from `/proc/stat`
- CPU package temperature and per-chip sensors from `/sys/class/hwmon`
- CPU package power from RAPL when readable (root-only on some systems)
- Memory and swap from `/proc/meminfo`
- Per-process CPU, memory, command, state and time from `/proc`
- Network throughput from `/proc/net/dev`
- Disk I/O from `/proc/diskstats`, filesystem usage from `/proc/mounts`
- Battery status on laptops from `/sys/class/power_supply`
- Terminal UI with 20 layouts, themes, process search/sort/kill, freeze
- Headless output: JSON, YAML, XML, CSV, TOON
- Optional Prometheus metrics server (`-p <port>`)

macOS-only features (Apple ANE/DRAM bandwidth, SMC fan control, menu bar,
overlay HUD, Thunderbolt/RDMA, display FPS) are gracefully disabled on Linux.

## Build

Requires Go 1.25+.

```sh
git clone <this-repo> lintop
cd lintop
make build          # produces ./lintop
```

or directly:

```sh
go build -o lintop main.go
```

## Install

```sh
sudo make install   # installs to /usr/local/bin/lintop
```

## Usage

```sh
./lintop
```

Common flags:

```sh
lintop --interval 1000            # update interval (ms)
lintop --foreground green         # theme color (named or hex)
lintop --bg "#22212C"             # background color
lintop --headless --count 1       # one JSON sample and exit
lintop --headless --pretty        # pretty JSON stream
lintop --headless --format csv    # json, yaml, xml, csv, toon
lintop -p 2112                    # enable Prometheus on :2112/metrics
lintop --lang en                  # UI language
```

### NVIDIA notes

- `nvidia-smi` must be installed and on `PATH` (comes with the NVIDIA driver).
- If no NVIDIA GPU is present, GPU fields are simply zero/omitted.
- Per-process GPU usage is approximated from each process's share of active
  compute VRAM, weighted by the system-wide GPU utilization.

### CPU power

Package power is read from the RAPL powercap interface. On many distributions
`/sys/class/powercap/*/energy_uj` is root-only; run `sudo lintop` if you want
CPU wattage. GPU power always works via `nvidia-smi`.

## Config and logs

| File | Path |
| --- | --- |
| Config | `$XDG_CONFIG_HOME/lintop/config.json` (or `~/.lintop/config.json`) |
| Theme | `$XDG_CONFIG_HOME/lintop/theme.json` (or `~/.lintop/theme.json`) |
| Log | `$XDG_STATE_HOME/lintop/lintop.log` (or `~/.lintop/lintop.log`) |

## Keyboard shortcuts

- `q` quit, `r` refresh, `c` cycle color, `b` cycle background, `p` party mode
- `l` cycle layouts, `i` info layout, `F` fan/thermal layout, `a` SoC history
- `+`/`-` adjust update interval
- `/` search processes, `F9` kill selected process, `f` freeze list
- arrows or `h`/`j`/`k`/`l` navigate, `Enter`/`Space` sort, `?` help

## License

MIT. Upstream mactop by Carsen Klock. See `LICENSE`.
