# heya — Project Plan

轻量、JSON 驱动、可扩展的 Linux 掌机 Launcher Shell。

**定位：Kindle + iPod + Linux Launcher + Pocket OS 的统一外壳，不是游戏前端。**
游戏只是「应用」的一种。第一阶段优先在 Linux PC 上开发运行，最终跑在 ROCKNIX / Knulli / muOS / ARM64 Linux 掌机（RG34XX、RG35XX、RG40XX、H700 系列）。

本文件是第一阶段交付物：**先定架构，再写代码**。

---

## 1. 目标与非目标

### 目标

1. 从 ROCKNIX 现有的迷你启动器（`handheld-launcher`，C + SDL2）中把**启动器职责**拆分出来，成为独立项目。
2. Go 实现，**单二进制**部署，ARM64 交叉编译。
3. **JSON 驱动 UI**：新增应用不改代码，扫描 `/storage/heya/apps` 自动生成菜单。
4. **图标即文本**：Nerd Font 字形，不引入 PNG 图标资源。
5. 主题由 JSON 描述，用户改 JSON 即可换主题，无需编译。
6. 720×480 横屏目标分辨率（RG34XX 原生 3:2），8/16/32 整数缩放，不依赖浏览器 / HTML / Electron。
7. 输入语义统一（A=Enter、B=Esc、DPad=WASD、L1=Q、R1=E、Start=Space、Select=Tab），支持手柄热插拔。

### 非目标（明确排除）

- 不做游戏库前端：不做刮削（scraper）、不做 EmulationStation 的 gamelist 元数据体系、不做主题引擎兼容、不做收藏夹/游戏集合 UI。
- 不复制 ES 的 UI 框架、C++ 代码或主题格式。
- 不接管模拟器本身的配置管理（RetroArch core 选项、per-game 覆盖等）。
- 不在 Milestone 1–5 内替换系统前端；替换 ES 是 M6，且必须保持游戏启动契约不变。

---

## 2. 现状分析摘要

完整分析见 [`docs/rocknix-architecture.md`](docs/rocknix-architecture.md)。要点：

- 启动链：`default.target → rocknix.target → graphical.target → sway.service → essway.service`，`essway.service` 执行 `/usr/bin/start_es.sh`。
- `start_es.sh` 先 source `es_settings`（时区、locale、环境导入），再做旋转/扫描输出复位，然后：
  ```sh
  if [ -x /usr/bin/handheld-launcher ]; then exec /usr/bin/handheld-launcher; fi
  emulationstation --log-path /var/log --no-splash
  ```
  也就是说，**现有迷你启动器已经通过「优先 exec」的方式顶替了 ES**，这正是要拆出去的入口。
- 现有迷你启动器：单个 `main.c`（约 1200 行，本轮另加 `system_tools.c`），C + SDL2，720×480 固定布局，自己画 8×8 点阵字体，菜单硬编码在代码里。
- 它已经有：控制中心（Wi-Fi/蓝牙/USB/远程访问/亮度/音量/熄屏）、Wi-Fi 扫描、蓝牙扫描、实时系统信息、系统工具（ZRAM/诊断/备份/恢复/音频重启）、电源菜单、XMPlayer 启动。
- 它缺的：可扩展性（菜单写死）、配置化（无 JSON）、主题化、应用发现、可移植性（绑定 H700 的 720×480 与 ROCKNIX 脚本）。

---

## 3. 总体架构

```
                    ┌──────────────────────────────┐
   systemd ────────▶│ cmd/heya (main)              │
                    │  flags / config / bootstrap  │
                    └──────────────┬───────────────┘
                                   │
        ┌──────────────────────────┼──────────────────────────┐
        ▼                          ▼                          ▼
┌───────────────┐         ┌────────────────┐         ┌───────────────┐
│ internal/     │         │ internal/      │         │ internal/     │
│ registry      │  model  │ launcher       │  state  │ ui            │
│ 扫描 apps/ROM │────────▶│ 导航/页面栈/动作 │────────▶│ 渲染/布局/主题  │
└───────────────┘         └───────┬────────┘         └───────┬───────┘
                                  │ action                   │ frame
                                  ▼                          ▼
                          ┌────────────────┐         ┌───────────────┐
                          │ internal/system│         │ internal/input│
                          │ 电量/WiFi/BT/音量│         │ SDL 事件/手柄   │
                          └────────────────┘         └───────────────┘
```

数据流是单向的：`input → launcher(state) → ui(render)`，`registry` 只在启动/刷新时提供只读模型，`system` 通过受控动作（action）读写设备状态。

### 模块边界（对应 issue 的 A–E）

| 模块 | Go 包 | 职责 | 明确不做 |
|---|---|---|---|
| A Input | `internal/input` | SDL2 事件循环、键盘/手柄统一映射、热插拔、按键重复抑制 | 不做业务判断，不认识菜单 |
| B UI | `internal/ui` | 帧循环、布局、整数缩放、字体与 Nerd Font 字形、主题色 | 不做导航状态、不读设备状态 |
| C Registry | `internal/registry` | 扫描 `/storage/heya/apps/*/app.json`、构建菜单模型、（后续）扫描 ROM 与配置 | 不执行程序、不渲染 |
| D Launcher | `internal/launcher` | 页面栈、选中项、滚动、动作分发（exec / system action / 返回） | 不直接画像素、不直接读 sysfs |
| E System | `internal/system` | 电量、Wi-Fi、蓝牙、音量、亮度、USB 模式；能力探测与降级 | 不决定 UI 文案与布局 |

**关键约束**：`ui` 与 `system` 之间没有直接依赖；`launcher` 是唯一的状态持有者；`input` 只产出语义动作（`Up/Down/Left/Right/Confirm/Cancel/PageUp/PageDown/Quit`），不产出具体按键。

---

## 4. 目录结构

```
heya/
├── cmd/heya/            # main：参数、配置加载、依赖装配
├── internal/
│   ├── input/           # A: SDL 事件、手柄映射、热插拔
│   ├── ui/              # B: 渲染、布局、字体、主题
│   ├── registry/        # C: 应用/ROM/配置扫描
│   ├── launcher/        # D: 导航、页面栈、动作
│   └── system/          # E: 电量/WiFi/BT/音量/亮度
├── assets/              # 内置字体（Nerd Font 子集）、默认配置
├── themes/              # 内置主题 JSON
├── examples/            # 示例 apps/ 与 app.json
├── docs/                # 架构与协议文档
└── PROJECT_PLAN.md
```

约定：`internal/` 不对外暴露 API；`cmd/heya` 只做装配。任何跨模块数据结构定义在 `internal/model`（小体量，纯数据 + 校验）。

---

## 5. JSON 协议设计

### 5.1 应用清单：`/storage/heya/apps/<app>/app.json`

```json
{
  "name": "Music",
  "icon": "nf-md-music",
  "exec": "/storage/heya/apps/music/music",
  "args": ["--fullscreen"],
  "category": "media",
  "hidden": false
}
```

规则：

- 目录名即 app id；`app.json` 缺失或非法 → 跳过并记录一条 warning，不阻塞启动。
- `exec` 必须是绝对路径，且启动前校验：存在、可执行、属主为 root 或当前用户、不是 world-writable。
- 不使用 shell 解析：`exec` + `args` 直接 `execve`，杜绝注入。
- `icon` 是 Nerd Font 字形名（见 5.3），未知字形回退到通用方块图标。

### 5.2 菜单页：`menu.json`（可选，用于静态页/自定义页）

```json
{
  "title": "Applications",
  "items": [
    { "name": "Music", "icon": "nf-md-music", "exec": "/apps/music/music" },
    { "name": "Settings", "icon": "nf-md-cog", "page": "settings.json" }
  ]
}
```

### 5.3 图标：Nerd Font 字形映射

```json
{ "nf-md-music": "F0759", "nf-md-cog": "F0493" }
```

内置 `assets/icons.json` 提供常用字形；用户可在 `themes/icons.json` 覆盖。**不使用 PNG/SVG**。

### 5.4 主题：`themes/<name>/theme.json`

```json
{
  "background": "#000000",
  "text": "#ffffff",
  "muted": "#8899aa",
  "accent": "#00ff00",
  "selection": "#0055ff",
  "font": "assets/SymbolsNerdFont-Regular.ttf",
  "scale": 16
}
```

`scale` 只接受 8 / 16 / 32（整数缩放，见 6）。非法值回退到 16。

---

## 6. 渲染与输入规范

- 目标画布 720×480（横屏，3:2，与 RG34XX 原生面板一致）；布局按横屏设计：顶部状态栏 + 左侧列表 + 右侧详情，实际窗口按整数倍缩放（`scale` 8/16/32 对应字体与行高，窗口可再整数放大）。
- 每帧只做一次纹理上传；文本用 `SDL_ttf` 渲染到缓存纹理（按 `文本+颜色+字号` 做 LRU 缓存），避免每帧重排。
- 目标帧率 60fps，低端机（H700 / Cortex-A53 / 1GB RAM）保证 30fps 下限。
- 输入语义映射（默认，可被 `input.json` 覆盖）：

| 手柄 | 键盘 | 语义 |
|---|---|---|
| A（东侧，SDL `CONTROLLER_BUTTON_B`） | Enter | Confirm |
| B（南侧，SDL `CONTROLLER_BUTTON_A`） | Esc | Cancel |
| D-Pad / 左摇杆 | WASD / 方向键 | Up/Down/Left/Right |
| L1 / R1 | Q / E | PageUp / PageDown |
| Start | Space | Start |
| Select | Tab | Select |

  Anbernic 的 A/B 物理位置与 SDL 标准相反，映射表必须**按设备可覆盖**，而不是硬编码在代码里。

---

## 7. 里程碑

| # | 里程碑 | 交付 | 验收 |
|---|---|---|---|
| M1 | 显示菜单 | `cmd/heya` + `ui` + `input`，读 `menu.json` 渲染、上下移动、退出 | PC 上 720×480 窗口显示菜单，键鼠/手柄可导航 |
| M2 | 启动程序 | `launcher` 动作 + `execve`，返回后重绘 | 从菜单启动 `/bin/true`、`htop` 等并正确返回 |
| M3 | 主题系统 | `themes/*/theme.json`、颜色/字体/缩放热加载 | 改 JSON 重启即换主题，无需编译 |
| M4 | 应用扫描 | `registry` 扫描 `/storage/heya/apps`，生成主菜单 | 放入新 app 目录后无需改代码即出现在菜单 |
| M5 | 系统信息 | `system`：电量、Wi-Fi、蓝牙、音量、亮度、USB 模式（能力探测 + 降级） | 在 ROCKNIX H700 上读数正确；缺失能力显示 Unavailable |
| M6 | 替换 ROCKNIX ES | 打包成 `heya` 包、systemd unit 接管 `essway.service`，保留游戏启动契约 | H700 实机开机进 heya；游戏经 `/usr/bin/run` 正常启动并返回 |

M6 前置条件：M1–M5 在实机验证通过，且 `docs/rocknix-architecture.md` 中的游戏启动契约有实机回归记录。

---

## 8. 风险分析

| 风险 | 影响 | 应对 |
|---|---|---|
| **CGO + SDL2 交叉编译**：Go 调 SDL2 需 CGO，aarch64 交叉编译要 sysroot 与静态库 | 高（可能卡住单二进制目标） | 在 ROCKNIX 构建容器内编译（已有 aarch64 工具链）；提供 `make arm64` 目标；CI 用容器镜像固定工具链；必要时静态链接 libSDL2/SDL2_ttf |
| **显示后端差异**：ROCKNIX 用 sway/wayland，Knulli 走 Batocera，muOS 可能 KMSDRM | 中 | SDL 视频驱动由环境变量与运行时探测决定（`wayland` → `kmsdrm` → `x11`），不写死；启动日志打印实际驱动 |
| **字体与图标覆盖**：Nerd Font 字形随版本变动，体积大 | 中 | 只内置所需子集（Symbols Nerd Font，OFL 许可），`icons.json` 与字体版本一起锁定并做覆盖自检 |
| **游戏启动契约**：ES 通过 `es_systems.cfg` 的 `%ROM%` 调 `/usr/bin/run`，`runemu.sh` 负责性能模式、环境、模拟器参数 | 高（M6 破坏即无法玩游戏） | M6 前不改契约：heya 只负责「选择」，启动仍走 `/usr/bin/run`；保留 ES 作为回退项 |
| **输入映射设备差异**（Anbernic A/B 互换、无 L2/R2） | 中 | 映射表配置化 + 启动时探测设备名；实机回归清单 |
| **性能与内存**（Cortex-A53、1GB RAM） | 中 | 纹理/文本缓存、避免每帧分配；pprof + 实机帧率测量作为 M1 验收项 |
| **配置破坏**：误写 `/storage/.emulationstation` 或用户 ROM 目录 | 高 | heya 只读写 `/storage/heya`；对系统目录只读；所有写操作走白名单路径 |
| **安全性**：JSON 里的 `exec` 可指向任意二进制 | 中 | 绝对路径 + 属主/权限校验 + 无 shell；可选白名单目录 |
| **多机型维护成本** | 中 | 设备差异收敛到 `internal/system` 的能力探测与 `themes/<device>/`，不散落到 UI |

---

## 9. 与现有迷你启动器的关系

- 现有 `handheld-launcher`（C + SDL2）**继续作为 ROCKNIX 镜像里的可用前端**，不删除。
- heya 按里程碑逐步覆盖它的能力；M6 才在镜像里切换 `start_es.sh` 的 exec 目标。
- 迁移顺序：JSON 菜单（M1–M2）→ 主题（M3）→ 应用扫描（M4）→ 系统面板（M5，复用现有脚本契约）→ 替换（M6）。
- 现有 C 实现作为**行为参考**：菜单分组、确认对话框、错误提示、模拟器守卫（宿主机不执行 reboot/poweroff）都应在新实现中保留。

---

## 10. 第一阶段的验收（本文件）

1. 架构与模块边界：第 3 节。
2. 目录结构：第 4 节。
3. JSON 协议：第 5 节。
4. 里程碑：第 7 节。
5. 风险分析：第 8 节。
6. Rocknix 启动流程分析：`docs/rocknix-architecture.md`。

架构确认后再进入 M1 编码。

---

## 11. 实施状态（2026-10-26）

| 里程碑 | 状态 | 证据 |
|---|---|---|
| M1 显示菜单 | 完成 | `internal/ui`、`internal/input`；720×480 渲染由 `make dump` 产出 PNG 验证 |
| M2 启动程序 | 完成 | `launcher.CmdExec` + `cmd/heya` 重新打开 UI；实测启动 `/usr/bin/printf` 后返回菜单 |
| M3 主题系统 | 完成 | `themes/default`、`themes/light`；同页面背景像素 `#0b0e14` → `#f5f2ea` 验证 |
| M4 应用扫描 | 完成 | `internal/registry` 扫描 `apps/*/app.json`；`SELECT` 键运行时重扫 |
| M5 系统信息 | 完成 | `internal/system` 读取 battery/temp/mem/storage/net/volume/brightness/ZRAM，缺失显示 Unavailable |
| M6 替换 ES | 集成文件就绪 | `integrations/rocknix/`：package.mk、heya.service、start_es.sh.patch；待镜像构建与实机验收 |

测试：`go test ./...` 覆盖 model/registry/input/launcher/system/ui（含图标像素、主题色、探测降级）。

尚未完成（需实机/镜像）：ARM64 交叉链接（需 SDL2/SDL2_ttf aarch64 sysroot）、镜像内打包、H700 实机启动与游戏回归。
