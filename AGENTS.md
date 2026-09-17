# auriga-cli

Go CLI for managing LLM models, profiles, benchmarks, and parameter sweeps on a local AMD AI server.

For server-side documentation (directory layout, service management, adding models, updating binaries), see [docs/server.md](docs/server.md).

## Build & Test

```bash
make build          # Build for current platform
make install        # Build + install binary, config, prompts, suites
make test           # Run tests (short)
make test-all       # Run all tests including integration
make lint           # Run golangci-lint
make vet            # Run go vet
make fmt            # Format code (gofmt + goimports)
make all            # fmt + vet + lint + test + build
make deploy-remote  # Cross-compile to Linux and rsync to auriga via SSH
```

**Deploy convention**: never cross-compile and copy manually. Commit + push, then build on auriga with `ggpull && make install`. Use `make deploy-remote` only for quick iterations.

## Architecture

- `cmd/auriga/` — entry point
- `internal/cli/` — Cobra commands
  - `profile/` — create, delete, list, serve, stop, switch, setup, sync, prune, validate
  - `model/` — list, create, ensure, prune
  - `benchmark/` — run, list, show, compare, download, suites
  - `serve/` — start, stop, list (direct llama-server management)
  - `show/` — config, perf
  - `sweep/` — init, validate, run (parameter tuning)
  - `ps/` — running instance status
- `internal/benchmark/` — benchmark execution engine
  - `formats/` — pluggable format runners (humaneval, GSM8K, IFEval, etc.)
  - `prompts/` — prompt templates for benchmark runs
- `internal/perf/` — performance measurement (TTFT, tok/s, perplexity)
- `internal/llamaserver/` — llama-server process management
- `internal/ollama/` — Ollama API client
- `internal/systemd/` — systemd user service generation
- `internal/huggingface/` — HuggingFace API (model resolution, download, dflash discovery)
- `internal/exec/` — command execution, file downloads, container sandbox (podman/docker)
- `internal/ui/` — terminal UI (lipgloss, tables, confirmations)
- `internal/config/` — configuration defaults and helpers
- `contrib/` — external integrations (Home Assistant dashboards, stats server)
- `suites/` — benchmark suite definitions (YAML registry, shipped to `~/.config/auriga/suites/`)

## CLI Commands

### `auriga profile`

| Command | Purpose |
|---------|---------|
| `profile create` | Create a new profile from HuggingFace repo |
| `profile setup` | Interactive setup (downloads model + mmproj + drafter) |
| `profile serve` | Start llama-server with a profile |
| `profile switch` | Stop current and start new profile on same port |
| `profile stop` | Stop a running profile |
| `profile list` | List profiles with SPEC column (mtp/dflash/-) |
| `profile sync` | Download missing model/mmproj/drafter files |
| `profile prune` | Delete orphaned .gguf files not referenced by any profile |
| `profile validate` | Check ctx_size, memory fit, missing files, drafter repos |
| `profile delete` | Remove a profile from config |

### `auriga benchmark`

| Command | Purpose |
|---------|---------|
| `benchmark run --slot N` | Run a suite against a running llama-server slot |
| `benchmark list` | List benchmark results (reads summary.json) |
| `benchmark show <run>` | Show detailed per-problem results |
| `benchmark compare` | Compare results across runs |
| `benchmark download --slot N` | Download benchmark datasets |
| `benchmark suites` | List available/installed suites from registry |

Benchmark execution is slot-based (`--slot 1` or `--slot 2`). Supports `--resume` and `--retry-failed` for interrupted runs. Results stored per profile in `benchmark.results_dir`.

### `auriga sweep`

| Command | Purpose |
|---------|---------|
| `sweep init <profile>` | Generate sweep config YAML |
| `sweep validate --config` | Validate sweep config before running |
| `sweep run --config` | Execute parameter sweep, produce JSON/CSV report |

### `auriga show`

| Command | Purpose |
|---------|---------|
| `show config` | Display resolved configuration |
| `show perf [profile]` | Quick perf test: TTFT, prompt tok/s, gen tok/s, perplexity |

`show perf` runs warmup before measuring, uses median of 5 runs with min-max range. Perplexity auto-measured on first run and cached.

### `auriga ps`

Shows running llama-server instances with SPEC column (detected from process args).

### `auriga model`

| Command | Purpose |
|---------|---------|
| `model list` | List available models |
| `model create` | Create a new model entry |
| `model ensure` | Ensure model files exist |
| `model prune` | Clean up unused model files |

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

- Dense models (e.g., Qwen3.8-27B, gemma-4-31B) run on `llama_server.slot_1_port` (default 8090)
- MoE models (e.g., Qwen3.6-35B-A3B, gemma-4-26B-A4B) run on `llama_server.slot_2_port` (default 8091)
- MoE detection heuristic: model name containing `-A\d+B` pattern (e.g., `-A3B`, `-A4B`)

Port resolution chain:
```
profile.port (explicit override) > type-derived port (dense/moe) > slot_1_port > 8090
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

### Top-level sections

| Section | Purpose |
|---------|---------|
| `ollama` | Ollama host, models_dir, model list (backup backend) |
| `llama_server` | Binary path, gguf/mmproj dirs, ports (`slot_1_port`/`slot_2_port`), perplexity config |
| `profiles` | Named model configurations (see below) |
| `benchmark` | max_tokens, max_retries, gen_timeout, results_dir |

### Profile fields

- `repo` — HuggingFace repo
- `model` — GGUF filename
- `bin` — llama-server binary path override (use `llama-server-strix-halo` or `llama-server-stock`, never bare `llama-server`)
- `mmproj` — multimodal projector (optional)
- `type` — `dense` or `moe` (auto-detected from model name if omitted)
- `port` — explicit port override (optional)
- `ctx_size` — context window size override (optional)
- `flags` — extra llama-server flags
- `mtp_drafter` — external MTP drafter GGUF filename (optional, stored in gguf_dir)
- `mtp_drafter_repo` — HuggingFace repo for the drafter (optional, for sync)
- `dflash` — DFlash drafter GGUF filename (optional, stored in gguf_dir)
- `dflash_repo` — HuggingFace repo for the DFlash drafter (optional, for sync; falls back to `repo`)

### Drafter Auto-Injection

`profile serve` and `profile switch` auto-inject `--model-draft <path>` when:
- `mtp_drafter` or `dflash` is set in the profile config
- The drafter file exists on disk in `gguf_dir`
- `--model-draft` is not already in the profile's flags

Both commands verify drafter file existence before starting and show DFlash/MTP Drafter in the confirmation panel.

### Drafter Auto-Discovery

`profile create --dflash` and `profile setup --dflash` auto-discover dflash drafter GGUFs:
- Calls `huggingface.ResolveDFlash(repo)` to find files with "dflash" in name + `.gguf` extension
- Downloads the drafter to `gguf_dir` (setup) or suggests `profile sync` (create)
- Writes the `dflash` field to the profile config

### Profile Lifecycle

- `auriga profile sync` — downloads missing model/mmproj/drafter files from HuggingFace (uses `dflash_repo` > `repo` fallback for dflash, `mtp_drafter_repo` > `repo` for mtp_drafter)
- `auriga profile prune` — detects and deletes orphaned .gguf files not referenced by any profile (checks model, mmproj, mtp_drafter, dflash fields)
- `auriga profile validate` — checks ctx_size vs model max (from GGUF metadata), estimates memory (model + KV cache + drafters), validates dual-instance fit against GTT, detects missing files (model, mmproj, drafters), warns on mtp_drafter/dflash without repo for sync
- `auriga profile list` — shows SPEC column (mtp/dflash/- based on config flags and drafter fields)
- `auriga ps` — shows SPEC column for running llama-server instances (detected from process args)

## Benchmark System

Benchmarks use a pluggable format runner system (`internal/benchmark/formats/`). Each format implements `BuildPrompt`, `ValidateResponse`, and `BuildRetryPrompt`.

Suite definitions live in `suites/` (YAML registry) and are installed to `~/.config/auriga/suites/`. Use `auriga benchmark suites` to list available and installed suites.

Execution is container-sandboxed (podman with docker fallback) for code evaluation. SELinux labeling disabled for compatibility.

Results are stored per-profile in `benchmark.results_dir` with per-problem metrics and summary.json.

## Contrib

- `contrib/auriga-stats-server.py` — HTTP stats server for llama-server metrics
- `contrib/homeassistant/` — Home Assistant dashboard, sensors, and templates for monitoring
