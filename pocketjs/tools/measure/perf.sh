#!/bin/sh
# 阶段6性能测量：PocketJS 浏览器 host 与 SDL2 heya 的 RSS/CPU 采样。
# 用法：perf.sh <pid> <seconds> <label>
set -eu
pid=${1:?usage: perf.sh <pid> <seconds> <label>}
seconds=${2:?usage: perf.sh <pid> <seconds> <label>}
label=${3:?usage: perf.sh <pid> <seconds> <label>}
out="perf-$label.csv"
printf 'label,elapsed_s,rss_kb,cpu_ticks\n' > "$out"
i=0
while [ "$i" -lt "$seconds" ]; do
  if [ -r "/proc/$pid/stat" ] && [ -r "/proc/$pid/statm" ]; then
    rss=$(awk '{print $2}' "/proc/$pid/statm" 2>/dev/null || echo 0)
    # statm 单位是页（通常 4KB）
    rss_kb=$((rss * 4))
    ticks=$(awk '{print $14 + $15}' "/proc/$pid/stat" 2>/dev/null || echo 0)
    printf '%s,%d,%d,%d\n' "$label" "$i" "$rss_kb" "$ticks" >> "$out"
  else
    printf '%s,%d,dead,dead\n' "$label" "$i" >> "$out"
    break
  fi
  i=$((i + 1))
  sleep 1
done
echo "wrote $out"
