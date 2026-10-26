# ROCKNIX 启动流程与前端架构分析

分析对象：本仓库对应的 ROCKNIX fork（`next`，commit `742bd52562`）以及
`ROCKNIX/emulationstation-next`（`f6c542872e`，ES 上游）。

目的是界定「启动器职责」的边界：哪些属于系统启动链，哪些属于前端，哪些必须保留给游戏启动契约。

---

## 1. 启动链（systemd）

```
default.target
  └── rocknix.target            Alias=default.target
        Requires=multi-user.target graphical.target
        After=graphical.target
        Conflicts=rescue.target
        │
        ├── sway.service                     合成器（Wayland）
        │     └── essway.service             Requires=sway.service, After=sway.service
        │           ExecStart=/usr/bin/start_es.sh
        │           Environment=HOME=/storage
        │           EnvironmentFile=/etc/profile
        │           Restart=always, RestartSec=2
        │           WantedBy=rocknix.target
        │
        ├── handheld-launcher.service        Requires=sway.service, Conflicts=essway.service
        │     ExecStart=/usr/bin/handheld-launcher
        │     Environment=HOME=/storage
        │                 XDG_RUNTIME_DIR=/var/run/0-runtime-dir
        │                 WAYLAND_DISPLAY=wayland-1
        │                 SDL_VIDEODRIVER=wayland
        │                 SDL_AUDIODRIVER=pulseaudio
        │     Restart=always, RestartSec=2
        │
        └── emustation.service               ConditionPathExists=/run/rocknix/start.games
              ExecStartPre=/usr/bin/es_settings
              ExecStart=/usr/bin/emulationstation --log-path /var/log --no-splash
              KillMode=process, TimeoutStopSec=3
```

要点：

- `essway.service` 与 `handheld-launcher.service` **互斥**（`Conflicts=essway.service`），同一时刻只有一个前端。
- 现有迷你启动器不是通过改 `start_es.sh` 接管的，而是**两个 unit 二选一 + start_es.sh 内的 exec 优先**双保险。
- 开机自启动脚本序列在 `projects/ROCKNIX/packages/rocknix/autostart/`：`001-setup`、`003-upgrade`、`006-display`、`007-rootpw`、`008-perfmode`、`009-sleepmode`、`010-uimode`、`050-audio`、`055-hdmi-check`、`080-network`、`081-usbgadget`、`099-networkservices`。这些是**系统准备**，不属于前端职责。

---

## 2. `start_es.sh`：前端入口

`projects/ROCKNIX/packages/ui/emulationstation/sources/start_es.sh`

```sh
. $(dirname $0)/es_settings                 # 时区 / locale / 环境导入
ES_MODE=$(cat /sys/class/drm/card*-"${WLR_CON}"/modes | head -n1)
prerotate_env "${WLR_CON_TRANSFORM}" "${ES_MODE%%x*}"   # 面板旋转
scanout_reset_stale                          # 扫描输出复位

if [ -x /usr/bin/handheld-launcher ]; then
  exec /usr/bin/handheld-launcher            # ← 现有迷你启动器在这里顶替 ES
fi

emulationstation --log-path /var/log --no-splash
```

结论：**「谁被 exec」就是前端选择点**。heya 的 M6 只需要接管这一处（或替换 unit），不需要改 ES 二进制。

---

## 3. `es_settings`：前端运行环境

`projects/ROCKNIX/packages/ui/emulationstation/sources/es_settings`

| 职责 | 具体行为 |
|---|---|
| 时区 | 读 `system.timezone`，写 `/storage/.cache/timezone` 与 `system_timezone`，`systemctl restart tz-data.service` |
| 语言 | 读 `system.language`，用 `localedef` 在 `/storage/.config/emulationstation/locale` 生成 locale，导出 `LANG/LANGUAGE/LOCPATH`，`systemctl import-environment` |
| 设备 quirk | 从 `/etc/profile.d/999-export` 导入 `*_` 变量到 systemd 环境 |
| 互斥 | `set_kill set "emulationstation"`（保证同一时刻只有一个前端实例） |

heya 若接管前端，需要复现：时区、locale 导出、quirk 环境导入。**这三件事不做，语言和时区会在前端里表现异常。**

---

## 4. 配置与数据路径

| 路径 | 属主 | 内容 |
|---|---|---|
| `/storage/.emulationstation/` | 用户 | `es_settings.cfg`、`es_input.cfg`、`themes/`、`collections/`、各系统 gamelist |
| `/etc/emulationstation/es_systems.cfg` | 系统 | 系统定义（名称、ROM 路径、扩展名、启动命令模板） |
| `/usr/config/emulationstation/` | 只读模板 | 默认 `es_systems.cfg`、默认主题、默认输入配置 |
| `/storage/roms/<system>/` | 用户 | ROM 与 `gamelist.xml`、`images/`、`videos/` |
| `/storage/.config/system/configs/system.cfg` | 用户 | 全局设置键值（`rocknix-settings` 读写） |
| `/var/log/exec.log` | 系统 | `runemu.sh` 的启动日志 |
| `/storage/roms/backup/ROCKNIX_BACKUP.zip` | 用户 | `backuptool` 备份产物 |

heya 的约定：**自己的数据只写 `/storage/heya/`**，系统目录一律只读。

---

## 5. 游戏扫描逻辑（ES 侧）

- `SystemData` 解析 `es_systems.cfg`：每个 `<system>` 含 `name`、`path`、`extension`、`command`（含 `%ROM%`、`%BASENAME%`、`%CONTROLLERSCONFIG%` 占位符）、`platform`、`theme`。
- `FileData` 递归枚举 `path` 下的文件，按 `extension` 过滤；同名 `gamelist.xml` 提供元数据（`name`、`desc`、`image`、`thumbnail`、`rating`、`releasedate`、`developer`、`publisher`、`genre`、`players`、`favorite`、`playcount`、`lastplayed`）。
- `Gamelist` 负责 `gamelist.xml` 的读写；媒体缓存目录由 `Paths` 管理。
- 排序/过滤：`FileSorts`、`FileFilterIndex`。

**heya 不复用这套模型**：游戏库是 ES 的领域。heya 只把「游戏」当作可执行应用的一种，通过外部 app 条目或后续可选模块接入。

---

## 6. 收藏夹与集合

- `CollectionSystemManager` 维护两类集合：
  - 自动集合：`favorites`（来自 `gamelist.xml` 的 `favorite=true`）、最近游玩等；
  - 自定义集合：`/storage/.emulationstation/collections/*.cfg`。
- 集合是 ES 的 UI 概念，依赖 gamelist 元数据。

heya 不实现该机制（非目标）。若将来需要「收藏」，用 `/storage/heya/favorites.json` 自己的模型，不读 ES 的集合文件。

---

## 7. 主题机制（ES 侧）

- `ThemeData` 从 `Paths::getUserEmulationStationPath() + "/themes"` 与 `/etc/emulationstation/themes` 加载主题集（set）。
- 每个 set 有 `themes.xml` 描述视图元素（image、text、ninepatch、video、color、font、变量替换），系统可覆盖。
- 主题是 XML + 位图/字体资源，支持九宫格缩放与变量。

heya 用**极简 JSON 主题**（背景/文本/强调/选中色 + 字体 + 缩放），不兼容 ES 主题格式。这是有意的取舍：ES 主题引擎复杂度与「Pocket OS 外壳」目标不匹配。

---

## 8. 输入映射（ES 侧）

- `InputManager` 读 `/storage/.emulationstation/es_input.cfg`，按 `deviceName` 匹配手柄配置；系统默认在 `/etc/emulationstation/es_input.cfg`。
- 输入来自 SDL 事件；手柄热插拔由 SDL 的 device added/removed 事件驱动。
- 游戏内输入不走 ES：`gptokeyb` 把手柄映射成键盘事件给模拟器，配置文件随系统/游戏下发（如 `projects/ROCKNIX/packages/rocknix/sources/.../controls.ini`）。

**heya 的映射**：语义动作（Confirm/Cancel/Up/Down/…）+ 可覆盖的 `input.json`。H700 上物理 A 映射到 SDL A（确认），物理 B 映射到 SDL B（返回）；方向键、肩键、Start 和 Select 由 ROCKNIX GameControllerDB 映射。

---

## 9. 游戏启动契约（M6 的关键）

1. ES 用 `es_systems.cfg` 的 `command` 模板拼出命令，把 `%ROM%` 替换成实际路径，通常形如：
   `/usr/bin/run "<rom>" -P <platform> --core=<core> --emulator=<emu>`
2. `/usr/bin/run` → `runemu.sh`：
   - 解析 `$1`（ROM）、`-P`（平台）、`--core=`、`--emulator=`；
   - 切到性能模式（`performance`）；
   - 读取 `system.cfg` 里的 per-platform/per-game 设置（`get_setting`）；
   - 设置显示/音频/输入环境，启动对应模拟器；
   - 记录 `/var/log/exec.log`，退出后恢复省电模式与前端。
3. 因此 **前端只负责「选哪个游戏」，启动细节全在 `runemu.sh`**。

heya 替换 ES 时：保留 `/usr/bin/run` 调用方式即可，不重写模拟器参数逻辑。

---

## 10. 现有迷你启动器在链路中的位置

- 单元：`handheld-launcher.service`（`Requires=sway.service`、`Conflicts=essway.service`）。
- 入口：`start_es.sh` 中 `if [ -x /usr/bin/handheld-launcher ]; then exec ...; fi`。
- 能力（当前实现）：控制中心（Wi-Fi/蓝牙/USB/远程访问/亮度/音量/熄屏）、Wi-Fi 与蓝牙扫描连接、实时系统信息、系统工具（ZRAM 状态、诊断文件、配置备份/恢复、音频重启）、电源菜单、XMPlayer 启动。
- 不涉及：游戏库、刮削、主题、收藏夹。
- 形态：单文件 C + SDL2，720×480 横屏，菜单与文案硬编码，8×8 点阵字体自绘。

---

## 11. 对 heya 的约束（从以上事实推出）

1. **数据根**：`/storage/heya/`（apps、themes、state），不写 `/storage/.emulationstation`。
2. **前端入口**：M6 时替换 `essway.service` 的 `ExecStart` 或 `start_es.sh` 的 exec 目标；两者保留其一的回退能力。
3. **环境**：必须自己完成时区、locale、quirk 环境导入（第 3 节），否则中文/日文与时间会异常。
4. **游戏**：只通过 `/usr/bin/run` 启动，参数格式与 ES 一致；不碰 `runemu.sh`。
5. **输入**：语义动作 + 可覆盖映射；H700 上 A 确认、B 返回。
6. **显示**：720×480 横屏；`SDL_VIDEODRIVER` 由环境决定（ROCKNIX 为 `wayland`），不硬编码。
7. **权限**：系统目录只读；`exec` 目标做绝对路径与权限校验。
