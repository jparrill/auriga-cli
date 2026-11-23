#!/usr/bin/env bash
# build-llama-server.sh — build llama.cpp in ~/Projects and expose it via ~/infra/bin.
#
# Runs ON THE SERVER (auriga). Enforces the binary layout used on this box:
#
#   source checkout  ~/Projects/<dir>            (git clone, kept up to date)
#   build artifacts  ~/Projects/<dir>/build/bin/ (launcher + shared libraries)
#   public name      ~/infra/bin/<link-name>     (symlink, never a copy)
#
# Upstream llama-server is a ~12 KB launcher whose RUNPATH points at its build
# tree, so copying the file out of the tree produces a broken or stale binary.
#
# Usage:
#   build-llama-server.sh --link-name llama-server-mainline --ref b12653
#   build-llama-server.sh --link-name llama-server-stock --dir llama.cpp-b10861 --no-build
#   build-llama-server.sh --link-name llama-server-mainline --pull          # update checkout
set -euo pipefail

REPO_URL="${LLAMA_REPO_URL:-https://github.com/ggml-org/llama.cpp}"
PROJECTS="${PROJECTS_DIR:-$HOME/Projects}"
INFRA_BIN="${INFRA_BIN:-$HOME/infra/bin}"
DIR_NAME=llama.cpp
LINK_NAME=""
REF=""
PULL=0
BUILD=1
JOBS="$(nproc)"
TARGETS=(llama-server llama-mtmd-cli llama-cli llama-perplexity)

die() { echo "error: $*" >&2; exit 1; }
log() { echo "==> $*"; }

usage() { sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0; }

while (( $# )); do
    case "$1" in
        --dir)       DIR_NAME="${2:?--dir needs a value}"; shift ;;
        --link-name) LINK_NAME="${2:?--link-name needs a value}"; shift ;;
        --ref)       REF="${2:?--ref needs a value}"; shift ;;
        --pull)      PULL=1 ;;
        --no-build)  BUILD=0 ;;
        --jobs)      JOBS="${2:?--jobs needs a value}"; shift ;;
        -h|--help)   usage ;;
        *)           die "unknown flag: $1 (try --help)" ;;
    esac
    shift
done

[ -n "$LINK_NAME" ] || die "--link-name is required (e.g. llama-server-mainline)"

SRC="$PROJECTS/$DIR_NAME"
BIN_DIR="$SRC/build/bin"

log "source: $SRC"
if [ -d "$SRC/.git" ]; then
    if [ -n "$REF" ]; then
        git -C "$SRC" fetch --depth 1 origin "$REF" || die "cannot fetch $REF"
        git -C "$SRC" checkout -q FETCH_HEAD
        log "checked out $REF ($(git -C "$SRC" rev-parse --short HEAD))"
    elif [ "$PULL" -eq 1 ]; then
        git -C "$SRC" pull --ff-only || die "pull failed"
        log "updated to $(git -C "$SRC" rev-parse --short HEAD)"
    fi
else
    [ -d "$SRC" ] && die "$SRC exists but is not a git checkout"
    mkdir -p "$PROJECTS"
    if [ -n "$REF" ]; then
        git clone --depth 1 --branch "$REF" "$REPO_URL" "$SRC"
    else
        git clone --depth 1 "$REPO_URL" "$SRC"
    fi
    log "cloned $(git -C "$SRC" rev-parse --short HEAD)"
fi

if [ "$BUILD" -eq 1 ]; then
    log "configuring (Vulkan, Release)"
    cmake -S "$SRC" -B "$SRC/build" -DGGML_VULKAN=ON -DCMAKE_BUILD_TYPE=Release >/dev/null
    log "building ${TARGETS[*]} with $JOBS jobs"
    cmake --build "$SRC/build" -j"$JOBS" --target "${TARGETS[@]}"
fi

[ -x "$BIN_DIR/llama-server" ] || die "$BIN_DIR/llama-server not found (run without --no-build)"

# The launcher must resolve its libs from this checkout, never from another tree.
runpath=$(readelf -d "$BIN_DIR/llama-server" | grep -o "$BIN_DIR" || true)
[ -n "$runpath" ] || die "RUNPATH of $BIN_DIR/llama-server does not point at $BIN_DIR"

mkdir -p "$INFRA_BIN"
suffix="${LINK_NAME#llama-server}"
for t in "${TARGETS[@]}"; do
    [ -x "$BIN_DIR/$t" ] || continue
    if [ "$t" = "llama-server" ]; then
        name="$LINK_NAME"
    else
        name="$t$suffix"
    fi
    ln -sfn "$BIN_DIR/$t" "$INFRA_BIN/$name"
    log "linked $INFRA_BIN/$name -> $BIN_DIR/$t"
done

log "version: $("$INFRA_BIN/$LINK_NAME" --version 2>&1 | head -1)"
log "commit:  $(git -C "$SRC" rev-parse --short HEAD)"
