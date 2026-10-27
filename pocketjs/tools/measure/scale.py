#!/usr/bin/env python3
"""阶段6：规模性能测试（6/50/100 卡片）。
生成含 N 个应用的 Registry 目录，测 PocketJS native 与 SDL2 heya 的 RSS/CPU。
用法：python3 scale.py --pocketjs <dir> --sdl2 <bin> --repo <repo> --display :20
"""
import argparse, json, os, subprocess, time, signal, tempfile, shutil


def sample(pid, seconds, interval=0.5):
    out = []
    end = time.time() + seconds
    while time.time() < end:
        try:
            with open(f"/proc/{pid}/statm") as f:
                rss = int(f.read().split()[1]) * 4
            with open(f"/proc/{pid}/stat") as f:
                p = f.read().split()
                ticks = int(p[13]) + int(p[14])
            out.append((rss, ticks))
        except Exception:
            break
        time.sleep(interval)
    return out


def make_apps(directory, count):
    os.makedirs(directory, exist_ok=True)
    for i in range(count):
        d = os.path.join(directory, f"app{i:03d}")
        os.makedirs(d, exist_ok=True)
        with open(os.path.join(d, "app.json"), "w") as f:
            json.dump({"id": f"app{i:03d}", "name": f"App {i:03d}",
                       "icon": "nf-md-application", "exec": "/usr/bin/true",
                       "category": "bench"}, f)


def measure(cmd, cwd, env, sample_seconds=3, settle=1.0):
    t0 = time.time()
    proc = subprocess.Popen(cmd, cwd=cwd, env=env,
                            stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(settle)
    s = sample(proc.pid, sample_seconds)
    proc.send_signal(signal.SIGTERM)
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
    if not s:
        return None
    rss = [x[0] for x in s]
    ticks = [x[1] for x in s]
    return {
        "rss_mb_avg": round(sum(rss) / len(rss) / 1024, 1),
        "rss_mb_max": round(max(rss) / 1024, 1),
        "cpu_ticks_delta": max(ticks) - min(ticks),
        "samples": len(s),
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--pocketjs", required=True, help="dist/linux-heya/program")
    ap.add_argument("--sdl2", required=True)
    ap.add_argument("--repo", required=True)
    ap.add_argument("--display", default=":20")
    ap.add_argument("--counts", default="6,50,100")
    ap.add_argument("--out", default="perf-scale.json")
    a = ap.parse_args()
    clk = os.sysconf("SC_CLK_TCK")
    tmp = tempfile.mkdtemp(prefix="heya-bench-")
    report = {"clock_ticks_per_sec": clk, "scales": {}}
    try:
        for count in [int(c) for c in a.counts.split(",")]:
            apps = os.path.join(tmp, f"apps{count}")
            make_apps(apps, count)
            # PocketJS: bridge 指向该目录，native host 渲染
            bridge_host = "127.0.0.1:8132"
            bridge = subprocess.Popen(["/tmp/heya-bridge", "--apps", apps, "--listen", bridge_host],
                                      stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            time.sleep(1.0)
            pj = measure(
                ["./pocket-desktop-host", "--app", "heya-pocket-main", "--js", "heya-pocket-main.js",
                 "--pak", "heya-pocket-main.pak", "--viewport", "720x480", "--density", "1",
                 "--title", "heya", "--fixed"],
                cwd=a.pocketjs, env=dict(os.environ, DISPLAY=a.display))
            bridge.terminate()
            try:
                bridge.wait(timeout=5)
            except subprocess.TimeoutExpired:
                bridge.kill()
            time.sleep(0.5)
            # SDL2 heya
            sd = measure([a.sdl2, "--apps", apps, "--width", "720", "--height", "480"],
                         cwd=a.repo, env=dict(os.environ, DISPLAY=a.display, SDL_VIDEODRIVER="x11"))
            if pj:
                pj["cpu_seconds"] = round(pj["cpu_ticks_delta"] / clk, 3)
            if sd:
                sd["cpu_seconds"] = round(sd["cpu_ticks_delta"] / clk, 3)
            report["scales"][str(count)] = {"pocketjs_native": pj, "sdl2_heya": sd}
            print(f"count={count} pocketjs={pj} sdl2={sd}")
        with open(a.out, "w") as f:
            json.dump(report, f, indent=1)
        print(json.dumps(report, indent=1))
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    main()
