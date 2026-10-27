#!/usr/bin/env python3
"""阶段6性能对比：PocketJS native host 与 SDL2 heya。

测量：启动时间（到 READY 或首帧）、稳定 RSS、CPU 占用（/proc 采样）。
用法：python3 compare.py --pocketjs <dir> --sdl2 <bin> --display :20 --runs 3
"""
import argparse, json, os, subprocess, time, signal, sys


def sample(pid, seconds, interval=0.5):
    """采样 RSS(KB) 与 CPU ticks。"""
    out = []
    end = time.time() + seconds
    while time.time() < end:
        try:
            with open(f"/proc/{pid}/statm") as f:
                rss_pages = int(f.read().split()[1])
            with open(f"/proc/{pid}/stat") as f:
                parts = f.read().split()
                utime, stime = int(parts[13]), int(parts[14])
            out.append({"rss_kb": rss_pages * 4, "cpu_ticks": utime + stime})
        except (FileNotFoundError, IndexError, ValueError):
            break
        time.sleep(interval)
    return out


def run_pocketjs(program_dir, display, runs, sample_seconds=3):
    results = []
    for i in range(runs):
        env = dict(os.environ, DISPLAY=display)
        t0 = time.time()
        proc = subprocess.Popen(
            ["./pocket-desktop-host", "--app", "heya-pocket-main", "--js", "heya-pocket-main.js",
             "--pak", "heya-pocket-main.pak", "--viewport", "720x480", "--density", "1",
             "--title", "heya", "--fixed", "--announce-ready"],
            cwd=program_dir, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        ready_ms = None
        samples = []
        # 读 READY 行并同时采样
        import threading
        def reader():
            nonlocal ready_ms
            for line in proc.stdout:
                if line.startswith("READY"):
                    try:
                        ready_ms = int(line.split()[1])
                    except (IndexError, ValueError):
                        pass
                    break
        th = threading.Thread(target=reader, daemon=True)
        th.start()
        # 采样
        samples = sample(proc.pid, sample_seconds, 0.5)
        boot_wall = None
        if ready_ms is not None:
            boot_wall = int(time.time() * 1000) - ready_ms  # 近似
        proc.send_signal(signal.SIGTERM)
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
        rss = [s["rss_kb"] for s in samples] or [0]
        ticks = [s["cpu_ticks"] for s in samples] or [0]
        cpu_delta = (max(ticks) - min(ticks)) if len(ticks) > 1 else 0
        results.append({
            "run": i + 1,
            "ready_epoch_ms": ready_ms,
            "rss_kb_max": max(rss),
            "rss_kb_avg": int(sum(rss) / len(rss)),
            "cpu_ticks_delta": cpu_delta,
            "samples": len(samples),
        })
        time.sleep(1)
    return results


def run_sdl2(binary, repo, display, runs, sample_seconds=3):
    results = []
    for i in range(runs):
        env = dict(os.environ, DISPLAY=display, SDL_VIDEODRIVER="x11")
        t0 = time.time()
        proc = subprocess.Popen(
            [binary, "--apps", "examples/apps", "--width", "720", "--height", "480"],
            cwd=repo, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(1.0)  # 让启动稳定
        samples = sample(proc.pid, sample_seconds, 0.5)
        proc.send_signal(signal.SIGTERM)
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
        rss = [s["rss_kb"] for s in samples] or [0]
        ticks = [s["cpu_ticks"] for s in samples] or [0]
        cpu_delta = (max(ticks) - min(ticks)) if len(ticks) > 1 else 0
        results.append({
            "run": i + 1,
            "rss_kb_max": max(rss),
            "rss_kb_avg": int(sum(rss) / len(rss)),
            "cpu_ticks_delta": cpu_delta,
            "samples": len(samples),
        })
        time.sleep(1)
    return results


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--pocketjs", required=True)
    ap.add_argument("--sdl2", required=True)
    ap.add_argument("--repo", required=True)
    ap.add_argument("--display", default=":20")
    ap.add_argument("--runs", type=int, default=3)
    ap.add_argument("--out", default="perf-compare.json")
    a = ap.parse_args()
    # 时钟频率用于把 ticks 转秒（通常 100 Hz）
    clk = os.sysconf("SC_CLK_TCK")
    pj = run_pocketjs(a.pocketjs, a.display, a.runs)
    sd = run_sdl2(a.sdl2, a.repo, a.display, a.runs)
    for row in pj + sd:
        row["cpu_seconds"] = round(row["cpu_ticks_delta"] / clk, 3)
        row["rss_mb_avg"] = round(row["rss_kb_avg"] / 1024, 1)
        row["rss_mb_max"] = round(row["rss_kb_max"] / 1024, 1)
    report = {"clock_ticks_per_sec": clk, "pocketjs_native": pj, "sdl2_heya": sd}
    with open(a.out, "w") as f:
        json.dump(report, f, indent=1)
    print(json.dumps(report, indent=1))


if __name__ == "__main__":
    main()
