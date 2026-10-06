#!/usr/bin/env bash
# Strata inference engine — build & test on auriga (gfx1151)
# Run ON AURIGA: bash strata-setup.sh {deps|rocm|build|download|serve|bench|clean}
set -euo pipefail

# --- Paths (follow auriga server layout) ---
STRATA_SRC="$HOME/infra/bin/strata-src"
ROCM_DIR="$HOME/rocm"
SDK="$ROCM_DIR/install"
STRATA_DATA="$HOME/infra/services/strata"
STRATA_PORT=8082

ROCM_TARBALL="therock-dist-linux-gfx1151-7.14.1.tar.gz"
ROCM_URL="https://repo.amd.com/rocm/tarball-multi-arch/$ROCM_TARBALL"
ROCM_SHA256="c40e8f2bd6630a7d11557c762b99c6fa8afb04c9bd0e51ed1675ee1ca24afb00"

# ROCm env (only if SDK exists)
_setup_rocm_env() {
    [ -d "$SDK/bin" ] || return 0
    export ROCM_PATH="$SDK" HIP_PATH="$SDK" HIP_PLATFORM=amd
    export PATH="$SDK/bin:$SDK/lib/llvm/bin:$PATH"
    export LD_LIBRARY_PATH="$SDK/lib:$SDK/lib/rocm_sysdeps/lib:$SDK/lib/llvm/lib:${LD_LIBRARY_PATH:-}"
}
_setup_rocm_env

cmd_deps() {
    echo "==> Installing build dependencies..."
    sudo dnf install -y gcc gcc-c++ cmake ninja-build git python3 python3-pip
    if ! groups | grep -q render; then
        echo "!! Add user to render+video: sudo usermod -aG render,video $USER && newgrp render"
    fi
}

cmd_rocm() {
    echo "==> ROCm toolchain for gfx1151..."
    mkdir -p "$ROCM_DIR" && cd "$ROCM_DIR"
    if [ ! -f "$ROCM_TARBALL" ]; then
        curl -LO "$ROCM_URL"
    fi
    echo "$ROCM_SHA256  $ROCM_TARBALL" | sha256sum -c -
    if [ ! -d "$SDK/bin" ]; then
        mkdir -p install
        tar -xzf "$ROCM_TARBALL" -C install
    fi
    _setup_rocm_env
    echo "ROCm ready: $SDK"
}

cmd_build() {
    _setup_rocm_env
    if [ ! -d "$STRATA_SRC" ]; then
        git clone https://github.com/Niko1221/Strata.git "$STRATA_SRC"
    else
        cd "$STRATA_SRC" && git pull
    fi
    cd "$STRATA_SRC"
    cmake -S . -B build-halo -G Ninja -DCMAKE_BUILD_TYPE=Release \
        -DSTRATA_ENABLE_HIP=ON -DSTRATA_ENABLE_CUDA=OFF -DSTRATA_PREFILL_MMQ=ON \
        -DCMAKE_HIP_ARCHITECTURES=gfx1151 \
        -DCMAKE_HIP_COMPILER="$SDK/lib/llvm/bin/clang++" \
        -DCMAKE_HIP_COMPILER_ROCM_ROOT="$SDK" \
        "-DCMAKE_PREFIX_PATH=$SDK;$SDK/lib/rocm_sysdeps;$SDK/lib/llvm" \
        "-DCMAKE_HIP_FLAGS=--rocm-path=$SDK --rocm-device-lib-path=$SDK/lib/llvm/amdgcn/bitcode"
    cmake --build build-halo --target strata -j "$(nproc)"
    echo "==> Binary: $STRATA_SRC/build-halo/strata"
}

cmd_download() {
    cd "$STRATA_SRC"
    mkdir -p "$STRATA_DATA"
    # Coder variant: half experts, code-optimized, ~40GB
    ./setup.sh --family coder --context 131072 --data-dir "$STRATA_DATA" --no-browser --yes
}

cmd_serve() {
    cd "$STRATA_SRC"
    _setup_rocm_env

    export STRATA_HIPBLASLT_TUNING="$STRATA_SRC/tools/hip/gfx1151-hipblaslt-100401.txt"

    # gfx1151 fast flags (change rounding, not answers)
    export STRATA_PF_FUSED=1
    export STRATA_PF_GEMM=1
    export STRATA_HC_UPMIX=1
    export STRATA_PA_FAST=1
    export STRATA_HIP_WMMA=1
    export STRATA_SELECT_WMMA=1
    export STRATA_HC_Q8=1
    export STRATA_PF_SWITCH_MIN_T=4096

    echo "==> Starting Strata on port $STRATA_PORT..."
    echo "    API: http://0.0.0.0:$STRATA_PORT/v1"
    echo "    UI:  http://0.0.0.0:$STRATA_PORT"
    ./setup.sh --port "$STRATA_PORT" --data-dir "$STRATA_DATA" --no-browser --host 0.0.0.0
}

cmd_bench() {
    local port="${1:-$STRATA_PORT}"
    local api="http://127.0.0.1:$port/v1"
    echo "==> Benchmarking Strata at $api"

    # Warmup
    echo "--- warmup ---"
    curl -sf "$api/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"strata","messages":[{"role":"user","content":"hi"}],"max_tokens":5}' > /dev/null

    # Short gen
    echo "--- 256 tokens ---"
    local start end
    start=$(date +%s%N)
    local resp
    resp=$(curl -sf "$api/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"strata","messages":[{"role":"user","content":"Write a Go function that implements a concurrent worker pool with error handling. Include comments."}],"max_tokens":256}')
    end=$(date +%s%N)
    local ms=$(( (end - start) / 1000000 ))
    local comp_tok
    comp_tok=$(echo "$resp" | python3 -c "import json,sys; print(json.load(sys.stdin).get('usage',{}).get('completion_tokens','?'))" 2>/dev/null || echo "?")
    echo "Completion tokens: $comp_tok  Time: ${ms}ms"
    if [ "$comp_tok" != "?" ] && [ "$ms" -gt 0 ]; then
        echo "Approx: $(python3 -c "print(f'{$comp_tok / ($ms / 1000):.1f} tok/s')")"
    fi

    # Long gen
    echo "--- 1024 tokens ---"
    start=$(date +%s%N)
    resp=$(curl -sf "$api/chat/completions" \
        -H "Content-Type: application/json" \
        -d '{"model":"strata","messages":[{"role":"user","content":"Write a comprehensive guide to building a REST API in Go with error handling, middleware, database integration, and testing."}],"max_tokens":1024}')
    end=$(date +%s%N)
    ms=$(( (end - start) / 1000000 ))
    comp_tok=$(echo "$resp" | python3 -c "import json,sys; print(json.load(sys.stdin).get('usage',{}).get('completion_tokens','?'))" 2>/dev/null || echo "?")
    echo "Completion tokens: $comp_tok  Time: ${ms}ms"
    if [ "$comp_tok" != "?" ] && [ "$ms" -gt 0 ]; then
        echo "Approx: $(python3 -c "print(f'{$comp_tok / ($ms / 1000):.1f} tok/s')")"
    fi

    # Status
    echo ""
    echo "--- server status ---"
    curl -sf "http://127.0.0.1:$port/v1/status" 2>/dev/null | python3 -m json.tool 2>/dev/null || echo "(no /v1/status)"
    echo ""
    echo "Web UI: http://127.0.0.1:$port"
}

cmd_clean() {
    echo "Removing Strata source and data..."
    rm -rf "$STRATA_SRC" "$STRATA_DATA"
    echo "ROCm kept at $ROCM_DIR. Remove manually if wanted."
}

case "${1:-help}" in
    deps)     cmd_deps ;;
    rocm)     cmd_rocm ;;
    build)    cmd_deps; cmd_rocm; cmd_build ;;
    download) cmd_download ;;
    serve)    cmd_serve ;;
    bench)    cmd_bench "${2:-}" ;;
    clean)    cmd_clean ;;
    all)      cmd_deps; cmd_rocm; cmd_build; cmd_download ;;
    help)
        cat <<'EOF'
Usage: strata-setup.sh {deps|rocm|build|download|serve|bench [port]|clean|all}

  deps      Install Fedora build deps
  rocm      Download ROCm 7.14.1 gfx1151 toolchain (~10GB)
  build     deps + rocm + compile Strata (~25 min)
  download  Download Coder model (~40GB)
  serve     Start on port 8082 with gfx1151 fast flags
  bench     Quick tok/s benchmark
  clean     Remove source + data (keep ROCm)
  all       build + download
EOF
        ;;
    *) echo "Unknown: $1. Run with 'help'."; exit 1 ;;
esac
