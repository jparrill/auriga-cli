#!/usr/bin/env bash
# update-vllm.sh — keep the native vLLM install on auriga in sync with the repo.
#
# Runs ON THE SERVER (auriga). Pulls main and decides by itself whether a fast
# precompiled overlay is enough or a deep compile is required. Called by
# `update()` through ~/.customized/scripts/update-vllm.sh.
#
#   (no args)     pull + auto-select overlay or full compile, then relink.
#                 A deep compile is skipped unless VLLM_ALLOW_FULL=1.
#   --pull-only   git pull only (used by update(); no build).
#   --overlay     force the precompiled overlay path.
#   --full        force a deep compile of C++/HIP kernels + Rust frontend.
#   --recover     reinstall the wheel of the last built commit (no git, ~1 min).
#   --force       rebuild even if this commit is already deployed.
#   --status      show source/built/wheel state, symlink, disk.
#   --variant V   ROCm wheel variant (default rocm100; see wheels.vllm.ai/rocm).
#
# Decision rules (auto mode):
#   1. dirty csrc/cmake/setup.py/rust  -> full (local kernels must be compiled)
#   2. wheel published for HEAD         -> overlay (libs match HEAD exactly)
#   3. kernel/CMake/Rust files changed  -> full (no wheel to borrow libs from)
#   4. only Python/docs changed         -> overlay on nearest wheel
#
# Layout:
#   ~/Projects/vllm              source + .venv + build artifacts
#   ~/Projects/vllm/.venv/bin/vllm
#   ~/infra/bin/vllm  ->  ~/Projects/vllm/.venv/bin/vllm
set -euo pipefail

SRC="${VLLM_SRC:-$HOME/Projects/vllm}"
VENV="$SRC/.venv"
LINK="${VLLM_LINK:-$HOME/infra/bin/vllm}"
STATE="$SRC/.built-commit"
VSTATE="$SRC/.built-version"
WSTATE="$SRC/.built-wheel"
MSTATE="$SRC/.built-mode"
LOG_DIR="$HOME/infra/services/vllm"
WHEEL_INDEX="https://wheels.vllm.ai/rocm"
VARIANT="${VLLM_ROCM_VARIANT:-rocm100}"
GPU_ARCH="${VLLM_GPU_ARCH:-gfx1151}"
MIN_FREE_GIB=25

# Files whose changes cannot be carried by a Python-only build.
KERNEL_PATTERN='^(csrc/|cmake/|CMakeLists\.txt|setup\.py|rust/|tools/build_rust|requirements/|pyproject\.toml)'

MODE=auto
FORCE=0

die() { echo "error: $*" >&2; exit 1; }
log() { echo "==> $*"; }

usage() { sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

while (( $# )); do
    case "$1" in
        --pull-only) MODE=pull ;;
        --overlay)   MODE=overlay ;;
        --full)      MODE=full ;;
        --recover)   MODE=recover ;;
        --force)     FORCE=1 ;;
        --status)    MODE=status ;;
        --variant)   VARIANT="${2:?--variant needs a value}"; shift ;;
        -h|--help)   usage ;;
        *)           die "unknown flag: $1 (try --help)" ;;
    esac
    shift
done

export PATH="$HOME/.local/bin:$PATH"
[ -d "$SRC/.git" ] || die "$SRC is not a git checkout"

# ---------------------------------------------------------------- state helpers
head_commit() { git -C "$SRC" rev-parse --short HEAD; }
head_sha() { git -C "$SRC" rev-parse HEAD; }
head_subject() { git -C "$SRC" log -1 --format=%s; }
built_commit() { [ -f "$STATE" ] && cat "$STATE" || echo "-"; }
built_version() { [ -f "$VSTATE" ] && cat "$VSTATE" || echo "-"; }
built_wheel() { [ -f "$WSTATE" ] && cat "$WSTATE" || echo "-"; }
built_mode() { [ -f "$MSTATE" ] && cat "$MSTATE" || echo "-"; }
link_target() { [ -L "$LINK" ] && readlink -f "$LINK" || echo "(missing)"; }
free_gib() { df -Pk "$SRC" | awk 'NR==2 {printf "%d", $4/1048576}'; }

# The overlay borrows compiled libs from the wheel published for a commit.
# Return that index when it exists, fail otherwise.
wheel_index_for() {
    local sha="$1"
    [ -n "$sha" ] || return 1
    curl -sf --max-time 10 -o /dev/null "${WHEEL_INDEX}/${sha}/${VARIANT}/vllm/" || return 1
    echo "${WHEEL_INDEX}/${sha}/${VARIANT}"
}

# The overlay refuses a variant that does not match the detected ROCm runtime,
# so hand it the exact wheel URL instead of a variant name.
wheel_url_for() {
    local sha="$1" page href
    page="${WHEEL_INDEX}/${sha}/${VARIANT}/vllm/"
    href=$(curl -sf --max-time 20 "$page" | grep -oE 'href="[^"]*\.whl"' | head -1 \
        | sed 's/^href="//; s/"$//')
    [ -n "$href" ] || return 1
    python3 -c 'import sys, urllib.parse; print(urllib.parse.urljoin(sys.argv[1], sys.argv[2]))' \
        "$page" "$href"
}

print_status() {
    local commit built
    commit=$(head_commit); built=$(built_commit)
    echo "source        $SRC"
    echo "commit        $commit  ($(head_subject))"
    echo "built commit  $built"
    echo "built mode    $(built_mode)"
    echo "built wheel   $(built_wheel)"
    echo "built version $(built_version)"
    echo "symlink       $LINK -> $(link_target)"
    if [ -x "$LINK" ]; then
        echo "reported      $("$LINK" --version 2>/dev/null | head -1)"
    else
        echo "reported      (not runnable)"
    fi
    echo "venv python   $("$VENV/bin/python" --version 2>&1 || echo missing)"
    echo "disk free     $(free_gib) GiB"
    [ "$commit" = "$built" ] && echo "state         up to date" || echo "state         new commit not deployed"
}

# ---------------------------------------------------------------------- actions
pull_only() {
    log "pulling $SRC"
    if ! git -C "$SRC" diff --quiet || ! git -C "$SRC" diff --cached --quiet; then
        git -C "$SRC" status --short
        die "working tree is dirty; commit or stash it before pulling"
    fi
    git -C "$SRC" pull --ff-only
    echo "    commit $(head_commit)"
}

check_disk() {
    local avail
    avail=$(free_gib)
    (( avail >= MIN_FREE_GIB )) || die "only ${avail} GiB free (need ${MIN_FREE_GIB})"
}

dirty_kernel_sources() {
    git -C "$SRC" status --porcelain -- csrc cmake CMakeLists.txt setup.py rust tools/build_rust \
        requirements pyproject.toml | grep -q . && return 0 || return 1
}

kernel_delta() {
    local from="$1"
    [ "$from" = "-" ] && return 0
    git -C "$SRC" diff --name-only "$from"..HEAD 2>/dev/null | grep -qE "$KERNEL_PATTERN"
}

# Wheels lag a few hours behind main, so HEAD often has none. Walk back to the
# newest commit that does and report "<commit> <url>".
nearest_wheel() {
    local sha="$1" c url n=0
    for c in $(git -C "$SRC" rev-list --max-count=20 "$sha"); do
        if url=$(wheel_url_for "$c"); then
            echo "$c $url"
            return 0
        fi
        n=$((n + 1))
    done
    return 1
}

# True when files between two commits cannot be carried by a Python-only build.
kernel_delta_between() {
    git -C "$SRC" diff --name-only "$1".."$2" 2>/dev/null | grep -qE "$KERNEL_PATTERN"
}

# Decide the build path. Echo: none | overlay | full.
decide_mode() {
    local commit from kernel_changed=no
    commit=$(head_commit)

    if [ "$commit" = "$(built_commit)" ] && [ -x "$LINK" ] && [ "$FORCE" -eq 0 ]; then
        echo "none"; return
    fi
    if dirty_kernel_sources; then
        echo "full"; return
    fi
    from=$(built_commit)
    if kernel_delta "$from"; then
        kernel_changed=yes
    fi
    local sha wheel_commit _url
    sha=$(head_sha)
    if wheel_index_for "$sha" >/dev/null; then
        echo "overlay"; return
    fi
    if read -r wheel_commit _url <<< "$(nearest_wheel "$sha")"; then
        if kernel_delta_between "$wheel_commit" "$sha"; then
            echo "full"; return
        fi
        echo "overlay"; return
    fi
    if [ "$kernel_changed" = yes ]; then
        echo "full"; return
    fi
    echo "none"
}

# ---------------------------------------------------------------- build phases
ensure_build_tools() {
    # vLLM pins setuptools<81; newer versions break its setup.py metadata checks.
    uv pip install --python "$VENV/bin/python" \
        "setuptools>=78.1.1,<81" wheel setuptools-rust setuptools-scm \
        packaging ninja cmake pybind11 >/dev/null
}

# setup.py develop refuses to install vLLM's own requirements (vllm#34073), so
# they must be in place beforehand. Idempotent and fast once satisfied.
install_requirements() {
    log "installing vLLM requirements (common + rocm)"
    uv pip install --python "$VENV/bin/python" --index-strategy unsafe-best-match \
        -r "$SRC/requirements/common.txt" -r "$SRC/requirements/rocm.txt"
}

# The ROCm stack must come from the wheel index: plain PyPI torch is CUDA and
# fails device detection. Device extras install the gfx1151 kernels (hipBLASLt).
install_rocm_stack() {
    local index="$1"
    log "installing ROCm torch stack for $GPU_ARCH"
    uv pip install --python "$VENV/bin/python" \
        --extra-index-url "$index" --index-strategy unsafe-best-match \
        "torch[device-$GPU_ARCH]" "torchvision[device-$GPU_ARCH]" \
        "amd-torch-device-$GPU_ARCH" "amd-torchvision-device-$GPU_ARCH"
    # vLLM detects ROCm through amdsmi; without it the platform plugin returns
    # None and every command dies with "Failed to infer device type".
    uv pip install --python "$VENV/bin/python" \
        --extra-index-url "$index" --index-strategy unsafe-best-match amdsmi
    "$VENV/bin/rocm-sdk" init || log "rocm-sdk init skipped (no devel package)"
}

bootstrap() {
    local index sha
    have_uv || die "uv not found (pipx install uv)"
    check_disk
    sha=$(head_sha)
    index=$(wheel_index_for "$sha") \
        || die "no wheel index for $sha yet; cannot bootstrap (retry later or use a release tag)"
    log "creating venv ($VENV)"
    uv venv --python 3.14 "$VENV" >/dev/null
    install_rocm_stack "$index"
    log "installing vLLM wheel for $(head_commit)"
    wheel_url_for "$sha" >/dev/null || die "cannot resolve the vLLM wheel URL for $sha"
    uv pip install --python "$VENV/bin/python" --no-deps "$(wheel_url_for "$sha")"
    install_requirements
    ensure_build_tools
}

overlay() {
    local sha index
    have_uv || die "uv not found (pipx install uv)"
    check_disk
    ensure_build_tools
    sha=$(head_sha)
    cd "$SRC"
    unset VLLM_PRECOMPILED_WHEEL_COMMIT VLLM_PRECOMPILED_WHEEL_VARIANT VLLM_PRECOMPILED_WHEEL_LOCATION
    local libs_commit
    if url=$(wheel_url_for "$sha"); then
        libs_commit="$sha"
        log "overlay: Python from $(head_commit) + libs from the same commit"
    elif read -r libs_commit url <<< "$(nearest_wheel "$sha")"; then
        if kernel_delta_between "$libs_commit" "$sha"; then
            die "no wheel for ${sha:0:12} and it changes C++/HIP/Rust since ${libs_commit:0:12}; run --full"
        fi
        log "overlay: Python from $(head_commit) + libs from ${libs_commit:0:12} (newest wheel, Python-only drift)"
    else
        die "no wheel found in the last 20 commits of main; cannot overlay (use --full)"
    fi
    echo "$libs_commit" > "$WSTATE"
    # Must be exported before any pip/uv call: reinstalling the editable package
    # (e.g. while installing requirements) re-runs setup.py, which needs to know
    # where the precompiled libs live.
    export VLLM_USE_PRECOMPILED=1
    export VLLM_PRECOMPILED_WHEEL_LOCATION="$url"
    install_requirements
    ensure_build_tools
    "$VENV/bin/python" setup.py develop
}

full_build() {
    have_uv || die "uv not found (pipx install uv)"
    check_disk
    [ -x "$VENV/bin/rocm-sdk" ] || die "rocm-sdk missing in venv; run --overlay first"
    ensure_build_tools
    cd "$SRC"
    export VLLM_TARGET_DEVICE=rocm
    export ROCM_PATH="$("$VENV/bin/rocm-sdk" path --root)"
    export HIP_PATH="$ROCM_PATH"
    export PATH="$("$VENV/bin/rocm-sdk" path --bin):$PATH"
    export VLLM_GPU_ARCHES="$GPU_ARCH"
    rm -f "$WSTATE"
    log "full compile for $GPU_ARCH (20-45 min), ROCM_PATH=$ROCM_PATH"
    uv pip install --python "$VENV/bin/python" -e . --no-deps --no-build-isolation
}

verify_install() {
    local hip
    hip=$("$VENV/bin/python" -c 'import torch; print(torch.version.hip or "cpu")' 2>/dev/null) \
        || die "torch import failed"
    [ "$hip" != "cpu" ] || die "torch is not ROCm (got a CUDA/CPU build)"
    log "torch reports hip $hip"
}

link_binary() {
    local commit="$1" version
    [ -x "$VENV/bin/vllm" ] || die "$VENV/bin/vllm missing after install"
    version=$("$VENV/bin/vllm" --version 2>/dev/null | grep -m1 -E '^[0-9]+\.[0-9]+' || true)
    [ -n "$version" ] || die "vllm --version produced no output"
    mkdir -p "$(dirname "$LINK")"
    ln -sfn "$VENV/bin/vllm" "$LINK.new"
    mv -Tf "$LINK.new" "$LINK"
    echo "$commit" > "$STATE"
    printf '%s' "$version" > "$VSTATE"
    log "linked $LINK -> $(readlink "$LINK")"
    echo "    version: $version"
}

# ------------------------------------------------------------------- execution
have_uv() { command -v uv >/dev/null 2>&1; }

run_deploy() {
    local requested="$1" chosen
    mkdir -p "$LOG_DIR"
    exec 9>"$SRC/.build.lock"
    flock -n 9 || die "another vllm build is already running"

    if [ "$requested" = auto ]; then
        chosen=$(decide_mode)
        if [ "$chosen" = none ]; then
            log "$(head_commit) already deployed — nothing to do (use --force)"
            return 0
        fi
        if [ "$chosen" = full ] && [ "${VLLM_ALLOW_FULL:-0}" != 1 ]; then
            log "deep compile required (kernel/C++/Rust change or no wheel for HEAD)"
            log "skipped in unattended mode; run: update-vllm.sh --full"
            return 0
        fi
    else
        chosen="$requested"
    fi

    pull_only

    local logfile="$LOG_DIR/build-$(date +%Y%m%d-%H%M)-$chosen.log"
    log "mode: $chosen   log: $logfile"
    {
        if [ ! -x "$VENV/bin/vllm" ]; then
            bootstrap
        fi
        case "$chosen" in
            overlay) overlay ;;
            full)    overlay && full_build ;;
        esac
        verify_install
        link_binary "$(head_commit)"
        echo "$chosen" > "$MSTATE"
    } 2>&1 | tee -a "$logfile"
    return "${PIPESTATUS[0]}"
}

recover() {
    local sha index
    have_uv || die "uv not found (pipx install uv)"
    sha=$(built_wheel); [ "$sha" = "-" ] && sha=$(head_sha)
    index=$(wheel_index_for "$sha") || die "no wheel published for $sha"
    log "reinstalling the wheel for ${sha:0:12}"
    uv pip install --python "$VENV/bin/python" --no-deps --force-reinstall \
        --extra-index-url "$index" --index-strategy unsafe-best-match vllm
    verify_install
    link_binary "${sha:0:7}"
}

case "$MODE" in
    status)  print_status ;;
    pull)    pull_only ;;
    auto)    run_deploy auto ;;
    overlay) run_deploy overlay ;;
    full)    run_deploy full ;;
    recover)
        mkdir -p "$LOG_DIR"
        recover 2>&1 | tee -a "$LOG_DIR/recover-$(date +%Y%m%d-%H%M).log"
        exit "${PIPESTATUS[0]}" ;;
esac
