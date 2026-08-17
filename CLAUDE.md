# auriga-cli

Go CLI for managing LLM models, profiles, benchmarks on a local AMD AI server.

## Build & Test

```bash
make build          # Build for current platform
make deploy         # Cross-compile to Linux and deploy to auriga
go test ./... -short  # Run tests
go vet ./...        # Lint
```

## Architecture

- `cmd/auriga/` — entry point
- `internal/cli/` — Cobra commands (profile, model, benchmark, ps)
- `internal/llamaserver/` — llama-server process management
- `internal/ollama/` — Ollama API client
- `internal/systemd/` — systemd user service generation
- `internal/huggingface/` — HuggingFace API (model resolution, download)
- `internal/exec/` — command execution and file downloads
- `internal/ui/` — terminal UI (lipgloss, tables, confirmations)
- `internal/config/` — configuration defaults and helpers

## Standards

### Model Quantization Priority

Prefer Q8 with MTP (Multi-Token Prediction) for all profiles. MTP provides ~2x speedup on Strix Halo.

- Always use Q8_0 or Q8_K_XL quants when available
- Q4 only when Q8 is too large for parallel setup (e.g., 80B MoE models)
- Prefer MTP-enabled GGUFs (built-in speculative decoding)
- Trusted GGUF sources: unsloth, bartowski

### Speculative Decoding

Three types supported:
- **MTP (Multi-Token Prediction)** — built-in draft heads, flags: `--spec-type draft-mtp --spec-draft-n-max 2`
- **External MTP drafter** — separate drafter GGUF via `mtp_drafter` field
- **DFlash** — proprietary drafter (Muse Glimmer), via `dflash` field

MTP + mmproj works on llama-server b9601+: MTP accelerates text turns, auto-fallback for vision turns.

### MoE vs Dense Port Convention

MoE models MUST NOT run on the same port as dense models on llama-server.

- Dense models (e.g., Qwen3.6-27B, DeepSeek-R1-Distill) run on `llama_server.dense_port` (default 8090)
- MoE models (e.g., Qwen3.6-35B-A3B, gemma-4-26B-A4B) run on `llama_server.moe_port` (default 8091)
- MoE detection heuristic: model name containing `-A\d+B` pattern (e.g., `-A3B`, `-A4B`)

Port resolution chain:
```
profile.port (explicit override) > type-derived port (dense/moe) > dense_port > 8090
```

### Naming Conventions

- PID files: `/tmp/auriga-llama-server-{port}.pid`
- Systemd services: `auriga-llama-server-{port}.service`

### Test Style

Table-driven with Gherkin naming: `"When X, it should Y"`. No repetitive individual subtests.

## Config

Config file: `~/.config/auriga/config.yaml`

Profile fields:
- `repo` — HuggingFace repo
- `model` — GGUF filename
- `mmproj` — multimodal projector (optional)
- `type` — `dense` or `moe` (auto-detected from model name if omitted)
- `port` — explicit port override (optional)
- `flags` — extra llama-server flags
- `mtp_drafter` — external MTP drafter GGUF filename (optional, stored in gguf_dir)
- `mtp_drafter_repo` — HuggingFace repo for the drafter (optional, for sync)
- `dflash` — DFlash drafter GGUF filename (optional, stored in gguf_dir)

### Profile Lifecycle

- `auriga profile sync` — downloads missing model/mmproj files from HuggingFace
- `auriga profile prune` — detects and deletes orphaned .gguf files not referenced by any profile (checks model, mmproj, mtp_drafter, dflash fields)
