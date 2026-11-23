# Backlog

Open tasks from the 2026-11-23 audit of the live server (`auriga`,
Framework Desktop) and its `~/.config/auriga/config.yaml`.

Each task lists the symptom as observed on the server, where the code is
responsible, and how to verify the fix. Priorities: **P1** = wrong behavior
users will hit, **P2** = incorrect reporting / cleanup, **P3** = hygiene.

Live baseline at audit time:

- slot 1 / 8090: `qwen3.8-27b-dflash2-q4` (dense, ctx 262144, KV q8_0, dflash)
- slot 2 / 8091: `gemma4-26b-q4` (moe, ctx 131072, KV q4_0, MTP)
- GTT 51.9 / 105.5 GiB, `/` 83% used, gguf 510.5 GB

---

## P1 - `profile validate` mislabels a dflash profile as MTP

**Observed**: `auriga profile validate` reports `SPEC=mtp` for
`qwen3.8-27b-dflash2-q4`, while `profile list`, `ps` and `show perf` all
report `dflash`. Memory estimate also differs (44.5 vs 43.4 GiB) because the
drafter is counted in one path and not the other.

**Where**: `internal/cli/profile/validate.go:408-429`. The `dflash` block sets
`v.SpecType = "dflash"` (line 409) and the following block overwrites it with
`"mtp"` whenever the GGUF metadata declares MTP heads (line 416) — which is
true for Qwen3.8-27B.

**Fix**: make an explicit `dflash` field authoritative. Keep the MTP check as
the fallback, and only warn when `--spec-type draft-mtp` is requested without
MTP heads / external drafter.

**Verify**: `auriga profile validate` shows `dflash` for
`qwen3.8-27b-dflash2-q4` and `muse-glimmer-30b-q4`, and `mtp` everywhere else
in the current config. Add a unit test for "When profile sets dflash and the
model has MTP heads, it should report dflash".

---

## P1 - `auriga ps` attributes the running server to the wrong profile

**Observed**: `auriga ps` lists port 8090 as profile `qwen3.8-27b-q4`, but the
systemd unit that started it says `profile: qwen3.8-27b-dflash2-q4` and the
command line carries `--spec-type draft-dflash --model-draft
Qwen3.8-27B-DFlash2-Q4_K_M.gguf`.

**Where**: two compounding causes.

1. `internal/cli/ps/ps.go:341` (`resolveProfile`) matches only by model
   filename, and `qwen3.8-27b-q4` and `qwen3.8-27b-dflash2-q4` share
   `Qwen3.8-27B-UD-Q4_K_XL.gguf` — so which profile wins depends on map
   iteration order.
2. The authoritative override comes from `ReadActiveProfile(port)`, but that
   state file lives in `/tmp/auriga-llama-server-<port>.profile`
   (`internal/llamaserver/server.go:224`, `profileFileForPort`). `/tmp` is cleaned
   on reboot, so after the last reboot the hint is gone for 8090 while 8091
   still has its file from Oct 30.

**Fix**: persist the active profile under `~/.local/state/auriga/` (create it
on demand) and use `--model-draft` / `--slot-save-path` / `--spec-type` as
tie-breakers when several profiles resolve to the same model file.

**Verify**: `auriga ps` shows `qwen3.8-27b-dflash2-q4` on 8090 across a reboot,
with no `/tmp` state present.

---

## P1 - systemd units silently ignore the restart rate limit

**Observed**: journal shows `/home/jparrill/.config/systemd/user/auriga-llama-server-8090.service:11: Unknown key 'StartLimitIntervalSec' in section [Service], ignoring.`
for both slots. A profile that cannot start therefore restarts every 30 s
forever.

**Where**: `internal/systemd/service.go:37-38` writes `StartLimitIntervalSec`
and `StartLimitBurst` into `[Service]`; they belong to `[Unit]`.
`internal/systemd/service_test.go:32-33` asserts the wrong placement too.

**Fix**: emit both keys inside `[Unit]`, update the tests to locate the keys in
the `[Unit]` section (not anywhere in the file).

**Verify**: `systemctl --user daemon-reload && systemctl --user show
auriga-llama-server-8090 -p StartLimitIntervalUSec -p StartLimitBurst` returns
600s / 3, and the journal no longer complains after `systemctl --user restart`.

---

## P2 - `profile prune` never scans subdirectories

**Observed**: `auriga profile prune --dry-run` reports "No orphaned files
found", yet `MTP/mtp-Qwen3.8-Flash-Next-shared-Q4_K_M.gguf` (1.9 GB) is not
referenced by any profile, and the multi-part quant directories
(`UD-IQ3_XXS` 77 GB, `UD-IQ4_XS` 88 GB) are not scanned at all.

**Where**: `internal/cli/profile/prune.go:138` skips every directory entry
(`if e.IsDir() || !strings.HasSuffix(e.Name(), ".gguf")`), so `filepath.WalkDir`
never descends.

**Fix**: walk recursively, treat multi-part GGUF sets (`-0000N-of-0000M.gguf`)
as a single unit referenced by its first shard (the name used in `model`),
and report sizes per shard set. Keep `--dry-run` as the default reporting
mode and never delete a shard of a referenced set.

**Verify**: `--dry-run` lists the unreferenced shared MTP drafter, lists
nothing that a profile references (including `UD-IQ3_XXS/...-00001-of-00003.gguf`),
and the sum of reported orphans matches `du` on those paths.

---

## P2 - Ollama model list drifted from what is on disk

**Observed**: `ollama.models` in `config.yaml` declares 6 models; the daemon
serves 14 (267.5 GB). Undeclared: `gpt-oss:120b` (65 GB),
`hf.co/unsloth/Qwen3-Coder-Next-GGUF:Q4_K_M` (48.5 GB),
`hf.co/Jiunsong/supergemma4-26b-uncensored-gguf-v2:Q4_K_M`,
`hf.co/HauhauCS/Qwen3.6-35B-A3B-Uncensored-HauhauCS-Aggressive:Q4_K_M`,
`mikemikeok/Qwythos-9B-Uncensored:latest`, `devstral:24b`, `qwen3:30b-a3b`,
`nomic-embed-text:latest`.

**Fix**: decide which of the 14 are still wanted, then reconcile
`ollama.models` and remove the rest (about 150 GB not accounted for). If model
list drift is expected to recur, add a `model ensure`-style report that diffs
config against `GET /api/tags`.

**Verify**: `ollama list` and `ollama.models` agree, and `auriga ps` disk usage
for the Ollama component drops accordingly.

---

## P2 - Benchmark run `humaneval-go-qwen3-coder` scored 11%

**Observed**: `humaneval-go-qwen3-coder-2026-09-11_2335` passes 18/164 in 1h36m
while every other model in the same suite sits at 74-99%. 18/164 in the
shortest runtime of the whole set points at the sandbox/eval path, not the
model.

**Fix**: re-run with `--retry-failed` and inspect per-problem failures before
trusting the number; if the container sandbox failed to start, the runner
should mark the run as errored instead of recording a pass rate.

**Verify**: the re-run either lands in a plausible band or is flagged as
infrastructure failure in `summary.json`.

---

## P3 - Global `quant` and `reasoning_budget` are dead config

**Observed**: `llama_server.quant: Q4_K_M` and
`llama_server.reasoning_budget: 4096` are not inherited by any profile (every
profile spells out its GGUF filename) and `--reasoning-budget` does not appear
in any running command line. They read as knobs that do something.

**Fix**: either implement the inheritance (quant selecting a model variant,
budget appended to flags) or remove both keys and document that per-profile
values are the only source of truth.

**Verify**: `auriga show config` output and `config.yaml.example` agree with
the documented behavior.

---

## P3 - Deployed binary is behind HEAD

**Observed**: `~/bin/auriga` on the server is from Oct 1 19:57; HEAD is
`46745d2` (Oct 6, docs only), so behavior is current but the
binary is not from a tagged commit.

**Fix**: `ssh auriga 'cd ~/Projects/auriga-cli && ggpull && make install'`
after the fixes above land, so the deployed binary always corresponds to a
known commit. Consider `auriga --version` (currently unknown flag) reporting
the build commit.

**Verify**: `auriga --version` prints a commit that exists in `git log main` on
the server checkout.

---

## P3 - `qwen3.8-flash-next-q3/q4` deviate from the serving pattern

**Observed**: both profiles run `llama-server-stock` with
`--load-mode mmap --no-host --no-repack --fit off --n-cpu-moe 0`, while every
other profile uses `--load-mode mlock` on the strix-halo fork. They have not
been started since Sep 7.

**Fix**: `auriga profile switch` to each one and confirm they start and serve on
the current stock build (10861); drop flags that were only needed while
debugging if they are no longer required.

**Verify**: both profiles pass `profile validate`, start cleanly, and answer on
their slot port.

---

## P3 - Dual-instance fit is reported but not enforced

**Observed**: `profile validate` correctly warns on combos above ~86% of GTT
(`qwen3-coder` + `qwen3.8-27b-q8` = 100%, `qwen3.8-27b-dflash2-q4` +
`qwen3.8-27b-q8` = 96%, `qwen3.8-27b-q4` + `qwen3.8-27b-q8` = 95%,
`qwen3.6-mtp-q8` + `qwen3.8-27b-q8` = 93%), but `profile serve` / `profile
switch` start anyway. The current pair uses 60.7 GiB of 105.5.

**Fix**: reuse the validate estimate in `serve`/`switch` and require `--yes` (or
an explicit override flag) when the resulting combination exceeds a
configurable GTT threshold.

**Verify**: switching to a 100% combo prompts before starting; the current
combination starts without prompting.

---

# vLLM native backend (added 2026-11-23)

vLLM now runs from `~/Projects/vllm` + venv with `~/infra/bin/vllm` as a symlink,
deployed by `scripts/update-vllm.sh`. These gaps remain.

## P2 - `profile validate` skips vLLM profiles

**Observed**: `auriga profile validate` prints `CTX - SPEC - MEMORY -` for
`gemma4-26b-ciru-vllm` and leaves it out of the dual-instance fit table.

**Where**: `internal/cli/profile/validate.go:337-345` returns right after checking
`model_dir`, before context, memory and spec resolution.

**Fix**: read `config.json` (`model_max_length`, `num_hidden_layers`,
`num_key_value_heads`, `head_dim`) from `model_dir` and feed the same estimate
used for GGUF profiles; include the profile in the fit table using
`gpu_memory_utilization * GTT` when set.

**Verify**: the vLLM profile shows a CTX row and appears in the fit matrix.

## P2 - HF `model_dir` trees are unmanaged

**Observed**: `gemma4-26b-ciru-strix` is 16 GB of safetensors that neither
`profile prune` nor `model ensure` knows about; `profile sync` skips vLLM
profiles by design.

**Fix**: list `model_dir` sizes in `ps` disk usage (already partially there) and
report unreferenced HF directories under `mmproj`-style handling.

## P2 - `--full` cannot compile (no ROCm SDK headers)

**Observed**: `rocm-sdk-devel` is not published in the pip ROCm 10 index
(`wheels.vllm.ai/rocm/<commit>/rocm100`, `stable.repo.amd.com/rocm/whl-next`),
so `rocm[devel]==10.0.0` is unsatisfiable and `rocm-sdk init` fails. The overlay
path works; the deep-compile path is blocked.

**Fix**: either wait for AMD to publish `rocm-sdk-devel`, or add a fallback that
compiles against the system `/opt/rocm` with a torch built for it. Until then,
`update-vllm.sh --full` fails fast with a clear message.

## P3 - No video inputs (torchcodec)

**Observed**: `torchcodec` has no ROCm wheel; the PyPI build fails to load
(`libtorchcodec_image.so`) against ROCm torch, so it was uninstalled. Text and
image inputs work; video does not.

**Fix**: track AMD/ROCm torchcodec builds; re-install when one exists for the
torch version in the venv.

---

# Infra leftovers after the 2026-11-23 binary audit

Every native binary in `~/infra/bin` now resolves into `~/Projects` (see
`docs/server.md`). What is left are artifacts and experiments that no longer
have a consumer. Nothing here is required by a running service.

| Path | Size | What it is | Recommendation |
|------|------|-----------|----------------|
| `~/infra/vllm-venv` | 25 GB | venv from the container-era attempt | delete; `~/Projects/vllm/.venv` replaced it |
| `~/infra/vllm-patches/gemma4.py` | 68 KB | patched Gemma4 model class for old vLLM | delete; vLLM 0.34 ships `gemma4_mm` |
| `~/infra/vllm-build.log` | 40 KB | old build log | delete |
| `~/infra/pulid-backup/pip-before.txt` | 4 KB | pip snapshot from a July experiment | delete |
| `~/infra/services/tupaccheck` | 0 B | aborted voice experiment (empty dirs) | delete |
| `~/infra/services/.backup-*`, `.staging`, `generate-landing.py.bak*` | 68 KB | housekeeping backups | delete |
| `~/infra/bin/orca/orca-linux.AppImage.backup-1.4.210` | 200 MB | previous Orca version | delete once the current AppImage is confirmed |
| `~/infra/bin/llama-server-halobox` + `~/Projects/halo-box-llamacpp` | 1.8 GB | experimental fork, no profile references it | decide whether to keep |
| `comfyui`, `miko` container descriptors | — | units not running / `miko` unit not found | decide whether to keep |

Strata was removed on 2026-11-23 (engine, source checkout and 151 GB of packs);
port 8083 is free.
