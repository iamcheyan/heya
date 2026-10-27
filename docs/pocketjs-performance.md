# CPR72 阶段6：性能

状态：**已实测**（本机 x86_64，Linux X11，软件渲染 llvmpipe）。

## 测量环境与方法

- 机器：本机 x86_64，Linux；显示 `:20`（X.Org 21.1.16）。
- PocketJS：`pocket-desktop-host`（release，`dist/linux-heya/program`），Vulkan/llvmpipe 软件渲染，720×480，`--fixed`。
- SDL2 heya：本机构建 `go build -o /tmp/heya-sdl2 ./cmd/heya`（go1.25.5，CGO，go-sdl2 v0.4.40，SDL2 2.32.4），`--apps <dir> --width 720 --height 480`，`SDL_VIDEODRIVER=x11`。
- RSS/CPU：采样 `/proc/<pid>/statm`（RSS，页×4KB）与 `/proc/<pid>/stat`（utime+stime ticks），每 0.5s 一次，稳定 1s 后采样 3s（6 个样本），`SC_CLK_TCK=100`。
- 脚本：`pocketjs/tools/measure/compare.py`（基线对比）、`scale.py`（规模）。
- **重要限制**：本机为 llvmpipe 软件渲染，无物理 GPU；绝对值不代表 RG34XX 目标设备。仅作同机相对对比。

## 基线对比（默认示例应用，3 次运行）

数据：`pocketjs/logs/perf-compare.json`

| 指标 | PocketJS native | SDL2 heya |
|---|---|---|
| RSS 平均 | 102.4 – 102.5 MB | 125.1 – 125.6 MB |
| RSS 峰值 | ~122.9 MB | ~125.8 MB |
| CPU（3s 采样） | 0.15 – 0.18 s | 0.28 – 0.32 s |

## 规模测试（6 / 50 / 100 卡片，3 秒采样）

数据：`pocketjs/logs/perf-scale.json`

| 卡片数 | PocketJS RSS avg | PocketJS CPU | SDL2 RSS avg | SDL2 CPU |
|---|---|---|---|---|
| 6 | 123.4 MB | 0.04 s | 125.7 MB | 0.35 s |
| 50 | 123.3 MB | 0.03 s | 128.4 MB | 0.35 s |
| 100 | 123.3 MB | 0.03 s | 129.0 MB | 0.40 s |

观察：

- **PocketJS RSS 不随卡片数增长**（123.3–123.4 MB 恒定）：因为 Guest 只渲染可视卡片（carousel），列表数据虽全量但渲染节点有限。
- **SDL2 heya RSS 随卡片数增长**（125.7 → 129.0 MB）：SDL2 UI 为每页行构建文本/纹理资源。
- **空闲 CPU：PocketJS 显著低于 SDL2**（0.03–0.04s vs 0.35–0.40s，约 1/10）。PocketJS 在空闲时通过 idle hash / pending frame API 推迟呈现，不增加 Guest turn。

## 帧追踪（PocketJS）

`--trace-frames` 输出 `FRAME_TRACE,<stage>,<tick>,<wall_us>,<elapsed_us>`，300 ticks 产生 602 条。

样本：

```
FRAME_TRACE,tick,1,...,474
FRAME_TRACE,render-submit,1,...,28204     # 首帧初始化
FRAME_TRACE,work,1,...,28778
FRAME_TRACE,tick,2,...,169
FRAME_TRACE,work,2,...,210
```

- 稳定态 `tick` 阶段 169–474 μs（CPU 提交）。
- 源码注明这些是 **CPU 提交耗时**，非 GPU 完成或显示延迟；llvmpipe 软件渲染下 GPU 耗时未单独测量。
- 60 Hz 目标下，帧预算 16.7 ms；稳定 tick 提交 <0.5 ms，但完整帧含 render-submit 与软件光栅化，需目标设备实测。

## 交互性能（来自阶段4实测）

- 连按/重复触发：SQUARE 快速按 5 次，`repeatToggles=5`，每次边沿都触发，无丢失。
- 长按：CROSS 按住 1200ms，按下与释放均引起画面变化。
- 菜单切换：SQUARE 开、CIRCLE 关均有效（阶段4记录：菜单内焦点移动无效，为功能问题而非性能问题）。

## 未完成项（诚实标注）

- **未做**：空界面单独测量（Guest 无空态）；连续滚动长测（Guest 为 carousel 非滚动列表）；GC 测量（QuickJS 无暴露 GC 指标，未测）。
- **未做**：菜单切换/连续焦点切换的定量帧率（仅有定性画面变化证据）。
- **未做**：SDL2 heya 的 `--dump` 无显示渲染对比（该模式用于出图，不代表运行时）。
- **限制**：软件渲染 llvmpipe；无目标设备（RG34XX）数据。阶段7另测 ARM64。

## 结论（基于本机软件渲染）

PocketJS 在本机 x86_64 软件渲染下的空闲 CPU 明显低于 SDL2 heya，RSS 略低且随列表规模稳定（不随卡片数增长）。这**不构成**目标设备（RG34XX，ARM64 + 有限 GPU）的性能结论——阶段7需实测。但就"渲染架构开销"而言，PocketJS 的 retained-mode + 损伤驱动呈现模型在规模增长时表现更稳。
