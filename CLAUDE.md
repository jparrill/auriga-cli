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
