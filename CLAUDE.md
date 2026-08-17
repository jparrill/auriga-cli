# auriga-cli

Go CLI for managing LLM models, profiles, benchmarks on a local AMD AI server.

## Build & Test

```bash
make build          # Build for current platform
make deploy-remote  # Cross-compile to Linux and deploy to auriga
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

### Context Size Convention

Context size defaults maximize usable context while ensuring dual-instance (dense + MoE) fits in 108GB GTT.

Resolution chain:
```
--ctx-size flag > profiles.X.ctx_size > llama_server.ctx_size > 131072
```

Guidelines:
- **Dense models**: 65536 (65K). Dense models are larger per-param, conserve memory for MoE on the other port.
- **MoE models**: 131072 (131K) minimum. Use 262144 (262K) when the model supports it AND dual-instance fits.
- Qwen3.6 MoE (35B-A3B): supports 262K, fits in dual with any dense model.
- Qwen3-Coder-Next (46GB Q4): 131K only — 262K too tight for dual with large dense models.
- gemma4/ornith MoE: 131K (architecture max).

Memory estimation for dual-instance:
- Model size + KV cache (Q8: ~64KB/token for MoE with GQA, ~128KB for dense)
- Both must fit within 108GB GTT total

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
- `ctx_size` — context window size override (optional, see Context Size below)
- `flags` — extra llama-server flags
- `mtp_drafter` — external MTP drafter GGUF filename (optional, stored in gguf_dir)
- `mtp_drafter_repo` — HuggingFace repo for the drafter (optional, for sync)
- `dflash` — DFlash drafter GGUF filename (optional, stored in gguf_dir)
- `dflash_repo` — HuggingFace repo for the DFlash drafter (optional, for sync; falls back to `repo`)

### Profile Lifecycle

- `auriga profile sync` — downloads missing model/mmproj files from HuggingFace
- `auriga profile prune` — detects and deletes orphaned .gguf files not referenced by any profile (checks model, mmproj, mtp_drafter, dflash fields)
- `auriga profile validate` — checks ctx_size vs model max (from GGUF metadata), estimates memory (model + KV cache + drafters), validates dual-instance fit against GTT, detects missing files (model, mmproj, drafters) and mtp_drafter/dflash without --model-draft
