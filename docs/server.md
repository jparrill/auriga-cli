# Auriga Server

## What is Auriga

Auriga is a local AI inference server built on an AMD Ryzen AI Max+ 395 workstation with 128GB LPDDR5x unified RAM. Available GTT comes from configuration or system detection and is displayed in GiB. It runs Fedora 44 and hosts llama-server instances for LLM inference, accessible over the local network and Tailscale.

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

There are two builds of llama-server:

| Binary | When to use |
|--------|-------------|
| `llama-server-strix-halo` | Default. Patched for Strix Halo iGPU. Most models. |
| `llama-server-stock` | For models that need upstream features not yet in the patched build (e.g., split quantization files, `--no-repack`, `--fit off`). |

**Never use bare `llama-server`** — it's a symlink that may point to either build. Always reference `llama-server-strix-halo` or `llama-server-stock` explicitly in profile configs via the `bin` field.

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

For compiled inference engines that need direct ROCm/HIP access. Examples: strata (Qwen3.8-Flash-Next).

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
| strata | Native | `auriga-cli/scripts/` | 8083 | Qwen3.8-Flash-Next (UD-IQ4_XS) | On-demand |

On-demand services share port 8092 — only one container service runs at a time. Start with `make up`, stop with `make down`.

Strata runs on port 8083 alongside llama-server. Use `scripts/strata-setup.sh serve` to start, kill the process to stop.

## Monitoring

The `auriga-stats-server.service` exposes system metrics via HTTP. Home Assistant dashboards in `contrib/homeassistant/` consume these metrics for monitoring GPU utilization, VRAM, temperatures, and inference stats.

```bash
# Quick perf test
auriga show perf

# Perf for specific profile
auriga show perf qwen3.8-27b-q4
```
