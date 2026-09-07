#!/bin/bash
# System stats aggregator for Home Assistant command_line sensor

# CPU
cpu_usage=$(awk 'NR==1{idle=$5; total=0; for(i=2;i<=NF;i++) total+=$i; printf "%.1f", (1-idle/total)*100}' /proc/stat)
cpu_temp=$(cat /sys/class/hwmon/hwmon1/temp1_input 2>/dev/null)
cpu_temp_c=$(echo "scale=1; ${cpu_temp:-0}/1000" | bc)

# RAM
eval $(awk '/MemTotal/{t=$2} /MemAvailable/{a=$2} END{printf "ram_total=%d ram_avail=%d ram_used=%d ram_pct=%.1f", t/1024, a/1024, (t-a)/1024, (1-a/t)*100}' /proc/meminfo)

# GPU via rocm-smi
gpu_json=$(rocm-smi --showuse --showtemp --showpower --showmeminfo vram --showmeminfo gtt --json 2>/dev/null)

gpu_use=$(echo "$gpu_json" | python3 -c "
import sys, json
d = json.load(sys.stdin).get('card0', {})
print(d.get('GPU use (%)', '0'))
" 2>/dev/null || echo 0)

gpu_temp=$(echo "$gpu_json" | python3 -c "
import sys, json
d = json.load(sys.stdin).get('card0', {})
print(d.get('Temperature (Sensor edge) (C)', '0'))
" 2>/dev/null || echo 0)

gpu_power=$(echo "$gpu_json" | python3 -c "
import sys, json
d = json.load(sys.stdin).get('card0', {})
print(d.get('Current Socket Graphics Package Power (W)', '0'))
" 2>/dev/null || echo 0)

gtt_total=$(echo "$gpu_json" | python3 -c "
import sys, json
d = json.load(sys.stdin).get('card0', {})
print(round(int(d.get('GTT Total Memory (B)', '0')) / 1073741824, 1))
" 2>/dev/null || echo 0)

gtt_used=$(echo "$gpu_json" | python3 -c "
import sys, json
d = json.load(sys.stdin).get('card0', {})
print(round(int(d.get('GTT Total Used Memory (B)', '0')) / 1073741824, 1))
" 2>/dev/null || echo 0)

vram_json=$(rocm-smi --showmeminfo vram --json 2>/dev/null)

vram_total=$(echo "$vram_json" | python3 -c "
import sys, json
d = json.load(sys.stdin).get('card0', {})
print(round(int(d.get('VRAM Total Memory (B)', '0')) / 1073741824, 1))
" 2>/dev/null || echo 0)

vram_used=$(echo "$vram_json" | python3 -c "
import sys, json
d = json.load(sys.stdin).get('card0', {})
print(round(int(d.get('VRAM Total Used Memory (B)', '0')) / 1073741824, 1))
" 2>/dev/null || echo 0)

# Disk
eval $(df / --output=size,used,avail,pcent 2>/dev/null | awk 'NR==2{gsub(/%/,"",$4); printf "disk_total=%d disk_used=%d disk_avail=%d disk_pct=%d", $1/1048576, $2/1048576, $3/1048576, $4}')

# Uptime and load
load=$(awk '{print $1}' /proc/loadavg)
uptime_s=$(awk '{print int($1)}' /proc/uptime)

cat <<EOF
{
  "cpu_usage": ${cpu_usage:-0},
  "cpu_temp": ${cpu_temp_c:-0},
  "ram_total_mb": ${ram_total:-0},
  "ram_used_mb": ${ram_used:-0},
  "ram_pct": ${ram_pct:-0},
  "gpu_use": ${gpu_use:-0},
  "gpu_temp": ${gpu_temp:-0},
  "gpu_power": ${gpu_power:-0},
  "gtt_total_gb": ${gtt_total:-0},
  "gtt_used_gb": ${gtt_used:-0},
  "vram_total_gb": ${vram_total:-0},
  "vram_used_gb": ${vram_used:-0},
  "disk_total_gb": ${disk_total:-0},
  "disk_used_gb": ${disk_used:-0},
  "disk_pct": ${disk_pct:-0},
  "load_1m": ${load:-0},
  "uptime_s": ${uptime_s:-0}
}
EOF
