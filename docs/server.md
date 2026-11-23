# Auriga Server

## What is Auriga

Auriga is a local AI inference server built on a **Framework Desktop** (AMD Ryzen AI Max+ 395, Radeon 8060S iGPU, gfx1151, 40 CU) with **8 × 16 GiB LPDDR5-8000**, of which **125.07 GiB are usable** and **105.5 GiB are GTT** available to the GPU. It runs Fedora 44 and hosts llama-server instances for LLM inference, accessible over the local network and Tailscale.

Storage: 1 TB NVMe for `/` and a 4 TB NVMe mounted at `/data`, where `~/infra` is a symlink. Models and service data live there, not on the root disk.

Measured values, capacity limits and the commands to reproduce them are in [server-hardware.md](server-hardware.md). Trust that file over any other document when they disagree.

The primary use case is running local LLMs for coding agents (OpenCode, Claude Code) and chat, with llama-server as the main inference backend.

## Access

| Method | Address |
|--------|---------|
| SSH | `ssh auriga` |
| LAN | 192.168.1.143 |
| Tailscale | 100.77.65.108 |
| Dense llama-server (slot 1) | port 8090 |
| MoE llama-server (slot 2) | port 8091 |
| Ollama (backup) | port 11434 |

## Directory Layout

```
~/
├── bin/
│   └── auriga                              # CLI binary (symlinked from infra/bin/)
├── infra/
│   ├── bin/
│   │   ├── auriga                          # CLI binary (main copy)
│   │   ├── llama-server -> llama.cpp-src/build/bin/llama-server  # symlink
│   │   ├── llama-server-stock              # upstream llama.cpp build
│   │   ├── llama-server-strix-halo         # patched build for Strix Halo
│   │   ├── llama-perplexity                # perplexity measurement tool
│   │   └── llama.cpp-src/                  # llama.cpp source (for building)
│   ├── ai/
│   │   ├── models/
│   │   │   ├── gguf/                       # GGUF model files
│   │   │   │   ├── MTP/                    # MTP drafter files
│   │   │   │   ├── UD-IQ3_XXS/            # Split quantization files
│   │   │   │   └── *.gguf                  # Model files
│   │   │   ├── mmproj/                     # Multimodal projectors (per-profile subdirs)
│   │   │   ├── datasets/                   # Perplexity/benchmark datasets
│   │   │   │   └── wikitext-2-raw/
│   │   │   ├── jinja-templates/            # Chat templates (Qwen fix, etc.)
│   │   │   ├── modelfiles/                 # Ollama modelfiles
│   │   │   └── ollama/                     # Ollama model storage
│   │   ├── benchmarks/                     # Benchmark results
│   │   └── config/                         # AI-related configs
│   ├── services/                           # Service configs
│   └── logs/                               # Server logs
└── .config/auriga/
    ├── config.yaml                         # Main auriga config
    ├── prompts/                            # Benchmark prompt templates
    ├── suites/                             # Benchmark suite definitions
    └── sensitive-patterns.yaml             # Patterns to filter from output
```

## llama-server Binaries

**Rule: every binary in `~/infra/bin` is a symlink to an artifact inside a source
checkout in `~/Projects/<repo>`.** The source tree, its `build/` directory and its
shared libraries stay in `~/Projects`; `~/infra/bin` only holds the name you call.

```
~/Projects/strix-halo-llamacpp/vulkan/llama-server-strix-halo
        └── ~/infra/bin/llama-server-strix-halo          (symlink)
~/Projects/llama.cpp/build/bin/llama-server
        └── ~/infra/bin/llama-server-mainline            (symlink)
~/Projects/vllm/.venv/bin/vllm
        └── ~/infra/bin/vllm                             (symlink)
```

Why: the upstream binaries are ~12 KB launchers with a `RUNPATH` pointing into
their build tree. Copying the file alone breaks (the `.so`s live next to it) and
a copied binary goes stale as soon as you rebuild. `~/.customized/scripts/build-llama-server.sh`
implements this pattern for llama.cpp.

Only hand-written adapters (`vllm-serve`) and the `auriga` CLI live in `~/infra/bin`
as real files.

| Binary | Source tree | When to use |
|--------|-------------|-------------|
| `llama-server-strix-halo` | `~/Projects/strix-halo-llamacpp` (fork, b10639) | Default. Patched for Strix Halo iGPU. Dense models + dflash/MTP. |
| `llama-server-mainline` | `~/Projects/llama.cpp` (upstream b12653) | Everything upstream: new architectures (Inkling 2 Mini needs ≥ b12611), split quant files, `--no-repack`, `--fit off`. |
| `llama-server-unsloth` | `~/Projects/llama.cpp-unsloth` (Unsloth prebuilt b12649, self-contained) | Only for Inkling 2 Mini: this prebuilt pins the DSpark speculator (upstream PR #32218 is still open, and b12653 rejects the drafter with a `joint_proj` shape mismatch). |
| `llama-server-halobox` | `~/Projects/halo-box-llamacpp` | Experimental upstream variant, not used by any profile. |
| `llama-cli` / `llama-perplexity` | `~/Projects/llama.cpp/build-ppl/bin` | Auxiliary tools from the mainline tree (separate build dir so rebuilding them never relinks the libs of a running server). |

The old `llama-server-stock` (b10861) was **retired on 2026-11-23**: its source tree
cannot be rebuilt offline any more (it downloads the web UI `dist.tar.gz` for the tag
and that asset is gone), and `llama-server-mainline` accepts every flag those profiles
used (`--fit`, `--no-repack`, `--n-cpu-moe`, `--load-mode`, `--no-host`). Profiles that
used it now point at `llama-server-mainline`.

**Never use bare `llama-server`** — it no longer exists. Always reference one of the
builds above in profile configs via the `bin` field.

## Other native binaries

| Name | Source | Notes |
|------|--------|-------|
| `~/infra/bin/vllm` | `~/Projects/vllm/.venv/bin/vllm` | Python venv entry point, see the vLLM section. |
| `~/infra/bin/auriga` | `~/Projects/auriga-cli/bin/auriga` | `make install` links it (no copy). |

`~/infra/bin/orca/orca-linux.AppImage` is the only binary in `~/infra/bin` that is
not built here: it is a downloaded third-party AppImage used by
`orca-server.service`. It is left in place on purpose (moving it would break the
unit); treat it as an exception to the rule.


### Speculative decoding for Inkling 2 Mini

Inkling ships its drafter as a separate DSpark GGUF (converted from `mtp.safetensors`,
block size 8): `dspark-Inkling-2-Mini-Q8_0.unsloth.gguf` (0.5 GB). The quant files
themselves carry no MTP heads, so `--spec-type draft-mtp` fails with
`failed to create MTP context`.

Measured on this box (Inkling UD-Q4_K_XL):

| Engine | Spec | Generation |
|---|---|---|
| mainline b12653 | none (drafter rejected) | 28.4 tok/s |
| Unsloth prebuilt b12649 | `--spec-type draft-dspark --spec-draft-n-max 3` | **49.2 tok/s** |

Draft acceptance sits at 0.91-0.96 with a mean accepted length of ~3.8 tokens. The
profile sets `bin: ~/infra/bin/llama-server-unsloth`, the spec flags and the drafter
path explicitly (it is a `Mainline_Support`-style file, so `profile sync` does not
know how to fetch it).

## Service Management

Auriga manages llama-server instances as systemd user services.

```bash
# Start a profile (creates/updates systemd service)
auriga profile serve qwen3.8-27b-q4 --slot 1 --daemon

# Switch profile on same port (stops old, starts new)
auriga profile switch qwen3.6-mtp-q4 --slot 1

# Stop a profile
auriga profile stop qwen3.8-27b-q4

# Stop all instances
auriga profile stop

# Check running instances
auriga ps
```

### Systemd services

Services are named `auriga-llama-server-{port}.service` and run as user services (`systemctl --user`).

```bash
# Check status
systemctl --user status auriga-llama-server-8090.service

# View logs
journalctl --user -u auriga-llama-server-8090.service -f

# The stats server
systemctl --user status auriga-stats-server.service
```

### PID files

PID files at `/tmp/auriga-llama-server-{port}.pid`. Used by `auriga ps` and `profile stop`.

## Adding a New Model

### 1. Create a profile

```bash
# From HuggingFace repo (interactive)
auriga profile setup new-model --repo unsloth/Model-Name-GGUF

# Or create config entry only (download later with sync)
auriga profile create new-model --repo unsloth/Model-Name-GGUF
```

### 2. Download model files

```bash
# Download missing GGUF, mmproj, and drafter files
auriga profile sync --name new-model
```

### 3. Configure the profile

Edit `~/.config/auriga/config.yaml` to set flags, ctx_size, type, etc. Key fields:

- `type`: `dense` or `moe` (determines port: dense=8090, moe=8091)
- `bin`: which llama-server binary to use
- `ctx_size`: context window size
- `flags`: extra llama-server flags (cache type, batch size, MTP, etc.)
- `mmproj`: multimodal projector filename (for vision models)
- `mtp_drafter` / `dflash`: speculative decoding drafter files

### 4. Validate

```bash
# Check memory fit, ctx_size vs model max, missing files
auriga profile validate
```

### 5. Serve

```bash
auriga profile serve new-model --slot 1 --daemon
```

## Dual-Instance Setup

The typical setup runs two llama-server instances simultaneously:

- **Slot 1 (port 8090)**: Dense model for primary chat/coding (e.g., qwen3.8-27b-q4)
- **Slot 2 (port 8091)**: MoE model for secondary tasks, title gen, or alternative model (e.g., qwen3.6-mtp-q4)

Both must fit in available GTT. Use `auriga profile validate` to check memory estimates.

```bash
# Start both
auriga profile serve qwen3.8-27b-q4 --slot 1 --daemon
auriga profile serve qwen3.6-mtp-q4 --slot 2 --daemon

# Verify
auriga ps
```

## Cleaning Up

```bash
# List orphaned GGUF files (not referenced by any profile)
auriga profile prune --dry-run

# Delete orphaned files
auriga profile prune

# Clean up Ollama models
auriga model prune
```

## Updating llama-server

```bash
ssh auriga
cd ~/infra/bin/llama.cpp-src
git pull
mkdir -p build && cd build
cmake .. -DGGML_VULKAN=ON -DCMAKE_BUILD_TYPE=Release
cmake --build . --config Release -j$(nproc)
# Binary is at build/bin/llama-server, symlinked from ~/infra/bin/llama-server
```

For the Strix Halo patched build, follow the same process in a separate source tree.

## Updating auriga CLI

From the development machine (macOS):

```bash
cd ~/Projects/auriga-cli
# Option A: deploy-remote (cross-compile + rsync)
make deploy-remote

# Option B: commit + build on server (preferred)
git push
ssh auriga "cd ~/Projects/auriga-cli && git pull && make install"
```

## Ollama

Ollama runs as a system service and is used as a backup chat backend. It's not the primary inference engine.

```bash
# Ollama is managed by systemd (system-level, not user)
sudo systemctl status ollama

# Models stored in ~/infra/ai/models/ollama/
# Config in ~/infra/ai/models/modelfiles/
```

## AI Services

Auriga hosts additional AI services beyond llama-server. Each follows a standard deployment pattern depending on whether the service is Python-based (container) or native C++ (build on server).

### Directory Convention

```
~/infra/services/<service-name>/    # Service data (models, output, config)
~/infra/bin/<service-name>-src/     # Native builds only: source + binary
~/Projects/<project-name>/          # macOS: project repo with Makefile, Dockerfile, etc.
```

Models downloaded from HuggingFace go under `~/infra/ai/models/hf/<model-name>/`.

### Port Allocation

| Port | Service |
|------|---------|
| 8080 | SearXNG |
| 8082 | Open Knowledge |
| 8083 | Available (used by service UIs) |
| 8090 | llama-server slot 1 (dense) |
| 8091 | llama-server slot 2 (MoE) |
| 8092 | AI services (minimax-h3, qwen-image) |
| 11434 | Ollama |

### Pattern A: Container-based (Python services)

For Python inference services (Diffusers, Transformers, FastAPI). Examples: minimax-h3, qwen-image-2.1.

**Project structure** (on macOS, deployed to auriga):

```
<project>/
├── Makefile              # build, up, down, logs, status, test
├── Dockerfile            # Base image + pip deps
├── Dockerfile.server     # Server-specific (optional split)
├── Dockerfile.ui         # UI container (optional)
├── serve.py              # FastAPI server
├── ui.py                 # Gradio UI (optional)
├── run.sh                # Standalone run script (no Make)
└── compose.yaml          # Optional docker-compose
```

**Makefile targets** (standard across all container services):

```makefile
MODEL_DIR    ?= /data/infra/ai/models/hf/<model>
OUTPUT_DIR   ?= /data/infra/services/<service>/output
SERVER_IMAGE ?= localhost/<service>-server:latest
PORT         ?= 8092

build        # Build container image(s)
up           # Start container(s) with GPU passthrough
down         # Stop and remove container(s)
restart      # down + up
logs         # Follow server logs
status       # Show container status + health check
clean        # Remove output files
test         # Smoke test against running server
```

**Container run flags** (required for GPU on gfx1151):

```bash
podman run -d --name <service> \
  --device /dev/kfd --device /dev/dri \
  --ipc=host --security-opt seccomp=unconfined --network=host \
  -e HSA_OVERRIDE_GFX_VERSION=11.5.1 \
  -e HSA_ENABLE_SDMA=0 \
  -e HIP_FORCE_DEV_KERNARG=1 \
  -e PYTORCH_ROCM_ARCH=gfx1151 \
  -e PYTORCH_CUDA_ALLOC_CONF=expandable_segments:True \
  -v <model_dir>:/model:ro \
  -v <output_dir>:/output \
  <image>
```

**Deployment workflow**:

```bash
# On macOS: build, push to auriga, build image, start
scp -r <project>/ auriga:~/Projects/<project>/
ssh auriga "cd ~/Projects/<project> && make build && make up"
```

### Pattern B: Native build (C++/HIP services)

For compiled inference engines that need direct ROCm/HIP access. Examples: HIP-based engines built from a source checkout in ~/Projects.

**Layout on auriga**:

```
~/infra/bin/<engine>-src/       # Git clone + build artifacts
~/infra/services/<engine>/      # Model packs, data, output
```

**Build workflow** (on auriga):

```bash
git clone <repo> ~/infra/bin/<engine>-src
cd ~/infra/bin/<engine>-src
# Build with ROCm for gfx1151
cmake -S . -B build -G Ninja -DCMAKE_BUILD_TYPE=Release \
  -D<ENGINE>_ENABLE_HIP=ON -DCMAKE_HIP_ARCHITECTURES=gfx1151 ...
cmake --build build -j$(nproc)
```

**Runtime env vars** (required for gfx1151):

```bash
export ROCM_PATH=$HOME/rocm/install
export HIP_PATH=$ROCM_PATH HIP_PLATFORM=amd
export LD_LIBRARY_PATH=$ROCM_PATH/lib:$LD_LIBRARY_PATH
```

**Setup scripts**: kept in `auriga-cli/scripts/` and deployed via `git pull` on auriga.

### Deployed Services

| Service | Type | Project | Port | Model | Status |
|---------|------|---------|------|-------|--------|
| minimax-h3 | Container | `video-minimax-h3/` | 8092 | MiniMax-H3 (INT8) | On-demand |
| qwen-image-2.1 | Container | `image-qwen-image-2.1/` | 8092 | Qwen-Image-2.1 | On-demand |

On-demand services share port 8092 — only one container service runs at a time. Start with `make up`, stop with `make down`.


## vLLM (native build)

vLLM runs from a source checkout plus a Python 3.14 venv, not from a container.
The container adapter (`~/infra/bin/vllm-serve`) is kept as a fallback.

```
~/Projects/vllm/            # git clone of vllm-project/vllm (main)
~/Projects/vllm/.venv/      # venv: torch+ROCm wheels, torch kernels, vLLM
~/infra/bin/vllm  ->  ~/Projects/vllm/.venv/bin/vllm
```

`auriga` only needs `vllm.bin: ~/infra/bin/vllm`; the arguments it injects
(`serve <model_dir> --host --port --max-model-len`) are the native CLI.

### Updating

`~/.customized/scripts/update-vllm.sh` decides the build path itself (it ships
itself to auriga with `--remote-local`, like `opencode-update --remote`):

| Mode | What it does | Cost |
|------|--------------|------|
| `--pull-only` | `git pull --ff-only` | seconds |
| default (auto) | pull, then overlay or full compile, then relink | 1-3 min (overlay) |
| `--overlay` | Python from the checkout + compiled libs from the wheel published for that commit | 1-3 min |
| `--full` | compile `csrc/` for gfx1151 + Rust frontend | 20-45 min |
| `--recover` | reinstall the last built wheel and relink | ~1 min |
| `--status` | show commit, built commit/wheel/mode, symlink, disk | instant |

Decision rules: dirty `csrc/`/`cmake/`/`setup.py`/`rust/` -> full; wheel published
for HEAD -> overlay; no wheel for HEAD but only Python changed since the newest
wheel -> overlay with that wheel's libs; kernel files changed and no wheel -> full.
A deep compile is never triggered unattended (`VLLM_ALLOW_FULL=1` to allow it).

`update()` calls that same script as the last step before the Oh-my-ZSH update.

### Bootstrap notes

Host prerequisites (once): `python3.14-devel` — Triton builds a small C shim at
runtime and needs `Python.h`; without it the engine core dies at startup.

The ROCm stack must come from the wheel index (`wheels.vllm.ai/rocm/<commit>/rocm100`),
never from PyPI: plain PyPI `torch` is a CUDA build and vLLM then fails with
`Failed to infer device type`. Required pieces: `torch[device-gfx1151]`,
`torchvision[device-gfx1151]`, `amd-torch-device-gfx1151` (hipBLASLt kernels;
without it `HIPBLAS_STATUS_INVALID_VALUE`), and `amdsmi` (vLLM's ROCm probe).

State files in the checkout: `.built-commit`, `.built-version`, `.built-wheel`,
`.built-mode`. Build logs: `~/infra/services/vllm/`.

### Known limitations

- **No video decoding**: `torchcodec` has no ROCm wheel; it is uninstalled, so
  video inputs are unavailable (images and text work). `requirements/rocm.txt`
  may try to reinstall it — remove it if `libtorchcodec_image.so` fails to load.
- **`--full` cannot compile yet**: `rocm[devel]` (the SDK headers/toolchain) is
  not published for the pip ROCm 10 index, so `rocm-sdk init` fails and the
  compile path is blocked. Overlay is the working path until that changes.
- Wheels lag main by a few hours, so HEAD often has none; the overlay borrows
  the newest wheel's libs and refuses when kernel sources changed in between.

## Monitoring

The `auriga-stats-server.service` exposes system metrics via HTTP. Home Assistant dashboards in `contrib/homeassistant/` consume these metrics for monitoring GPU utilization, VRAM, temperatures, and inference stats.

```bash
# Quick perf test
auriga show perf

# Perf for specific profile
auriga show perf qwen3.8-27b-q4
```
