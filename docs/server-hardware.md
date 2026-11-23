# Auriga Server — Hardware Inventory

Measured on the live machine, not inferred from other docs. Every value below lists the
command that produced it. If a number here disagrees with `CLAUDE.md`, `README.md` or
auriga-lab, **this file wins** — re-run the commands to confirm before trusting either side.

Last audit: 2026-11-21, host `auriga`, kernel 7.1.13-200.fc44.x86_64.

## Summary

| Item | Value | Source |
|------|-------|--------|
| Machine | **Framework Desktop**, SKU `FRAMDACP06`, version A6 | `hostnamectl`, `dmidecode -t system` |
| Mainboard | Framework `FRANMFCP06` | `dmidecode -t baseboard` |
| BIOS | 03.06, dated 2026-07-13 | `hostnamectl` |
| OS | Fedora 44 (Server Edition), kernel 7.1.13-200.fc44 | `/etc/fedora-release`, `uname -r` |
| CPU | AMD Ryzen AI Max+ 395, Zen 5, **16C/32T**, 625–5187 MHz | `lscpu` |
| CPU cache | L2 16 MiB (16 instances), **L3 64 MiB (2 instances = 2 CCDs)** | `lscpu` |
| CPU features | AVX-512 full set, `avx512_bf16`, `avx_vnni` | `lscpu` flags |
| RAM | **8 × 16 GiB** Micron LPDDR5-8000 (`MT62F4G32D8DV-023`), 128 GiB physical | `dmidecode -t memory` |
| RAM usable | **125.07 GiB** (131 150 220 kB) | `/proc/meminfo` |
| Swap | 8 GiB on zram | `free -h`, `lsblk` |
| iGPU | **Radeon 8060S**, PCI `1002:1586`, **gfx1151**, **40 CU**, 2900 MHz | `lspci -nnk`, `rocminfo` |
| iGPU memory | UMA APU: **512 MiB** visible VRAM + **105.5 GiB GTT** (113 246 208 000 B) | `/sys/class/drm/card1/device/mem_info_*` |
| NPU | **XDNA2**, PCI `1022:17f0`, driver `amdxdna`, `/dev/accel/accel0` — **unused** | `lsmod`, `/dev/accel` |
| ROCm | **7.1.1** from Fedora RPMs; hipcc 7.1.52802, clang 20.0.0.rocm | `rpm -qa \| grep rocm`, `hipcc --version` |
| Disk 0 | WD_BLACK SN7100 **1 TB** → `/` (xfs), 920 GB, 241 GB free | `lsblk`, `df -h` |
| Disk 1 | WD_BLACK SN7100 **4 TB** → `/data` (xfs), 3.7 TB, **2.0 TB free** | `lsblk`, `df -h` |
| `~/infra` | symlink → `/data/infra` (i.e. on the 4 TB disk) | `readlink -f ~/infra` |

## GPU compute reality check
`rocminfo` reporting gfx1151 does not prove the GPU computes. This was validated by
running PyTorch inside the image `localhost/qwen-image-21_server:latest`:

```
python 3.12.13
torch 2.12.0+git6bbd260
hip 7.2.53211
torch.cuda.is_available() = True
torch.cuda.get_device_capability(0) = (11, 5)
binary matmul 4096x4096 fp16, 10 iterations: 5.2 TFLOPs
```

The empty `get_device_name()` string is cosmetic: gfx1151 is not in torch's name table.

5.2 TFLOPs is memory-bandwidth bound, not compute bound. LPDDR5x-8000 quad-channel gives
roughly 256 GB/s shared with the CPU, so any kernel that streams data will sit near that
ceiling no matter how many CUs it has. **Plan workloads around bandwidth, not TFLOPs.**

## ROCm user space version matters more than the torch wheel

Measured on this box (kernel driver 7.1.13, gfx1151). Same benchmark every time: allocate a
tensor on `cuda` and run one op. "Segfault" means exit 139 on `torch.ones(1024, device="cuda")`,
i.e. HIP reports the device but cannot launch a single kernel.

| torch wheel | HIP (userspace) | Base image | Result |
|-------------|-----------------|------------|--------|
| 2.10.0+rocm7.0 | 7.0.51831 | Debian 13 (py3.10-slim) | segfault on first kernel |
| 2.11.0+rocm7.1 | 7.1.52802 | Debian 13 | segfault on first kernel |
| 2.11.0+rocm7.1 | 7.1.52802 | Ubuntu 22.04 | segfault on first kernel |
| **2.11.0+rocm7.2** | **7.2.26015** | **Ubuntu 22.04** | **works** |
| 2.12.0+git (vllm image) | 7.2.53211 | Ubuntu (ROCm 7.2 stack) | works (5.2 TFLOPs) |

Conclusion: **HIP 7.2 is the first user space that runs kernels on gfx1151 with this kernel
driver.** 7.0 and 7.1 detect the card and report 105.5 GiB of memory, then crash. The host
`hipcc` being 7.1.52802 does not help; the wheel carries its own ROCm.

Practical rules:
- Pin ROCm 7.2 wheels (`--index-url https://download.pytorch.org/whl/rocm7.2`).
- Verify with a real kernel at build/run time, never with `torch.cuda.is_available()` alone.
  A tiny `torch.ones(1024, device="cuda").sum()` is enough to catch this class of failure.
- `torch.cuda.get_arch_list()` at *build* time is useless: there is no GPU inside the build
  container, so it prints `[]` and `is_available()` prints `False`.


## FAQ / common mistakes

- **"128 GB unified RAM"** — 128 GiB is the *physical* DRAM. Usable is 125.07 GiB; the
  difference is firmware carve-out. The GTT window is 105.5 GiB, not 108.
- **"241 GB free"** — that is the root disk only. Models, service data and outputs live on
  `/data` (4 TB, 2.0 TB free) because `~/infra` is a symlink there.
- **"85 GB used by llama-server"** — `free -h` "used" is not per-process RSS (8.9 GB + 7.5 GB
  at audit time). What is actually consumed on the GPU side is **GTT** (50.8 GiB at audit
  time), which is what limits concurrent inference services.
- **"Strix Halo is a discrete GPU"** — it is not. There is no separate VRAM pool, no
  `rocm-smi` memory partition, and `mem_info_vram_total` only reports the 512 MiB carve-out.
- **The NPU is not usable from ROCm.** XDNA2 requires IRON / MLIR-AIE tooling. It is present
  and idle; do not plan any inference on it.

## Reproduce this audit

```bash
ssh auriga 'set +e
hostnamectl | grep -Ei "hostname|chassis|hardware|firmware|kernel"
sudo dmidecode -t system -t baseboard | grep -Ei "manufacturer|product|version"
sudo dmidecode -t memory | grep -E "^\s+(Size|Locator|Speed|Part Number):" | head -32
lscpu | grep -Ei "model name|^cpu\(s\)|core|socket|max mhz|L1d|L2|L3"
free -h; awk "/MemTotal/{printf \"%.2f GiB\n\", \$2/1048576}" /proc/meminfo
lspci -nnk | grep -A3 -Ei "vga|display|3d controller"
rocminfo | grep -Ei "Agent [0-9]|Marketing|gfx|Compute Unit|Max Clock"
cat /sys/class/drm/card1/device/mem_info_gtt_total
for f in /sys/module/amdgpu/parameters/*; do echo "$(basename $f)=$(cat $f)"; done
ls -l /dev/accel* /dev/dri/*; lsmod | grep -E "amdgpu|amdxdna"
rpm -qa | grep -E "^rocm-core|^rocm-runtime"; /opt/rocm/bin/hipcc --version | head -3
lsblk -o NAME,SIZE,TYPE,MODEL; df -h -x tmpfs -x devtmpfs | grep -vE "overlay|containers"
readlink -f ~/infra'
```

GPU compute check (any image with ROCm torch, e.g. the qwen-image one):

```bash
ssh auriga 'podman run --rm --device /dev/kfd --device /dev/dri --ipc=host \
  --security-opt seccomp=unconfined -e PYTORCH_ROCM_ARCH=gfx1151 \
  localhost/qwen-image-21_server:latest python -c "
import torch,time
print(torch.__version__, torch.version.hip, torch.cuda.is_available(), torch.cuda.get_device_capability(0))
a=torch.randn(4096,4096,device=\"cuda\",dtype=torch.float16); b=torch.randn_like(a)
torch.cuda.synchronize(); t=time.time()
for _ in range(10): c=a@b
torch.cuda.synchronize(); dt=(time.time()-t)/10
print(round(2*4096**3/dt/1e12,1), \"TFLOPs fp16\")"'
```

## Capacity planning cheatsheet

| Constraint | Value | Consequence |
|-----------|-------|-------------|
| GTT total | 105.5 GiB | Hard ceiling for everything GPU |
| GTT used at audit | 50.8 GiB | Two llama-server instances (dense 8090 + MoE 8091) |
| GTT free | ~54 GiB | Enough for one extra 2–4 GiB inference service |
| Disk for models/data | `/data`, 2.0 TB free | Never use `/` for models |
| Unified bandwidth | ~256 GB/s | Batch/prefill cost dominates, not raw FLOPs |
