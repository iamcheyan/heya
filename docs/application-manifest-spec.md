# Application Manifest Specification

状态：v1 协议草案，供 heya 0.x 实现；JSON 文件 `app.json`，不依赖数据库、网络、中心仓库或安装服务。

## 目标与非目标

heya 的定位是 Kindle + iPod + Linux Launcher + Pocket OS。Music、Video、Books、Terminal、Bilibili、RetroArch、Games 都是 Application。游戏只是应用类型之一；Launcher 不内置 ROM 概念、不枚举游戏、不对游戏提供一等特殊地位。未来游戏目录、ROM 集或模拟器组合由普通应用或可选 Provider 提供，返回的仍是应用入口。

本协议描述发现、展示和启动，不定义二进制包格式、安装器、沙箱、商店、ROM 元数据、游戏库、权限强制、在线服务或 OTA 实现。

## 标准目录

根目录固定为 `/storage/heya`。所有应用可写数据与系统程序分离；升级系统时保留用户内容，缓存可删除。

| 路径 | 职责与生命周期 | 备份 | OTA |
|---|---|---|---|
| `apps/<id>/` | 应用文件与 `app.json`；由用户/发行包安装和更新 | 建议备份清单和用户取得的应用 | 不覆盖用户应用；升级器可按包策略迁移 |
| `data/<id>/` | 持久用户数据、数据库、阅读进度、下载索引；按应用生命周期保留 | 是，优先备份 | 不覆盖；应用升级可执行显式迁移 |
| `cache/<id>/` | 可重建缓存、临时缩略图；可随时清空 | 否 | 可清空，不参与用户数据迁移 |
| `logs/<id>/` | 有界运行日志和故障记录；滚动保留 | 可选、短期 | 可轮换或清除 |
| `themes/` | heya 与应用主题/外观配置 | 是 | 保留用户覆盖；默认值由系统包更新 |
| `downloads/<id>/` | 用户下载、离线媒体和未完成内容；用户资产 | 是（空间允许） | 不覆盖、不自动删除 |

推荐把可执行代码和 manifest 放 `apps`，而不是将可变状态混在应用包内。旧式 `apps/<目录>/app.json` 原样扫描；v1 `id` 是稳定逻辑身份，目录名可变但正常安装器应使用 `<id>`。目录改名不改变 ID；更名应用时保留 ID。不同应用不得重用 ID；同目录扫描发现重复 ID 时拒绝后续项并报告错误。旧清单缺少 ID 时，兼容读取将目录名转为小写并把空格替换成 `-`；其他不符合 ID 格式的目录名需由维护者显式设置 `id`。

## Manifest v1

最小形式：

```json
{
  "manifestVersion": 1,
  "id": "music",
  "name": "Music",
  "entrypoint": "/usr/bin/music"
}
```

### 字段

| 字段 | 必需 | 规则 |
|---|---:|---|
| `manifestVersion` | v1 必需；历史清单可省略 | 当前只接受整数 `1`；缺失视为旧版 v1 |
| `id` | v1 必需；历史清单可省略 | 1–64 字符，小写 ASCII 字母、数字、`.`、`_`、`-`，首字符为字母或数字；唯一且不随显示名称变化 |
| `name` | 是 | 人类可读非空名称 |
| `entrypoint` | 是 | 绝对可执行文件路径；以 argv 直接执行，不经过 shell |
| `description` | 否 | 简短描述 |
| `version` | 否 | 应用自身版本字符串，不强制 SemVer |
| `author`、`license`、`homepage` | 否 | 署名、许可证标识/文本、项目 URL |
| `exec` | 仅兼容 | `entrypoint` 的旧字段名；新清单应改用 `entrypoint`。两者同时出现必须相同 |
| `args` | 否 | 字符串数组，每项是一个参数；不拆词、不展开变量、不执行 shell |
| `environment` | 否 | 字符串数组 `KEY=value`；按 key 覆盖父环境同名项；不接受空 key |
| `workingDirectory` | 否 | 绝对路径。缺省继承 heya 当前目录；不支持变量展开 |
| `icon` | 否 | 优先 Nerd Font 名称（如 `nf-md-music`）；缺省用通用应用图标 |
| `iconColor`、`badge`、`accent` | 否/预留 | 纯展示元数据；v1 可忽略，颜色建议 `#RRGGBB`，不作为协议行为要求 |
| `category` | 否 | 自由字符串；建议使用 `media`、`reader`、`network`、`system`、`games`、`tools` 等惯例；消费者必须容忍未知值 |
| `permissions` | 否 | 能力声明字符串数组，如 `network`、`audio`、`storage`；只用于说明/UI/未来协商，**不实施隔离或授权** |
| `features` | 否 | 可选能力声明，如 `background-download`、`resume-support`、`gamepad-input`；消费者忽略未知值 |
| `hidden` | 否 | `true` 时不列入默认启动列表 |

`entrypoint` 比 `exec` 更适合作为协议名：名称准确表达单一入口，不暗示 shell 命令行文本。v1 为现有清单提供 `exec` 读取兼容。Go、Rust、C 程序直接执行；Bash/Lua 程序可将解释器作为 entrypoint，脚本路径作为 `args`，例如 `entrypoint=/bin/sh`、`args=["/storage/heya/apps/tool/main.sh"]`。禁止把 `exec` 当作 shell 命令字符串拼接成命令行。

不设独立 `launchOptions`：v1 的 `args`、`environment`、`workingDirectory` 已覆盖基础启动契约。需要复杂启动图时交由应用自身或未来明确版本的扩展协议解决。

### Schema 草案

JSON Schema 2020-12；它描述新发布的 v1 清单。为旧清单保留兼容读取的实现必须允许缺失 `manifestVersion`、`id`，并将目录名用作缺省 ID。

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://heya.invalid/schemas/application-manifest-v1.json",
  "title": "heya Application Manifest v1",
  "type": "object",
  "additionalProperties": false,
  "required": ["manifestVersion", "id", "name", "entrypoint"],
  "properties": {
    "manifestVersion": {"const": 1},
    "id": {"type": "string", "minLength": 1, "maxLength": 64, "pattern": "^[a-z0-9][a-z0-9._-]{0,63}$"},
    "name": {"type": "string", "minLength": 1},
    "description": {"type": "string"},
    "version": {"type": "string"},
    "author": {"type": "string"},
    "license": {"type": "string"},
    "homepage": {"type": "string", "format": "uri"},
    "entrypoint": {"type": "string", "minLength": 1, "pattern": "^/"},
    "exec": {"type": "string", "minLength": 1, "pattern": "^/"},
    "args": {"type": "array", "items": {"type": "string"}},
    "environment": {"type": "array", "items": {"type": "string", "pattern": "^[A-Za-z_][A-Za-z0-9_]*="}},
    "workingDirectory": {"type": "string", "pattern": "^/"},
    "icon": {"type": "string"},
    "iconColor": {"type": "string", "pattern": "^#[0-9A-Fa-f]{6}$"},
    "badge": {"type": "string"},
    "accent": {"type": "string", "pattern": "^#[0-9A-Fa-f]{6}$"},
    "category": {"type": "string"},
    "permissions": {"type": "array", "uniqueItems": true, "items": {"type": "string"}},
    "features": {"type": "array", "uniqueItems": true, "items": {"type": "string"}},
    "hidden": {"type": "boolean"}
  }
}
```

实现额外校验：`exec` 与 `entrypoint` 同时存在时值相同；身份全局唯一；工作目录绝对路径。`additionalProperties:false` 促使发布者使用已定义协议；未来协议版本增加字段时按版本升级 Schema。若需要厂商扩展，先设计命名空间字段，不要静默放开顶层拼写错误。

### 示例

音乐应用：

```json
{
  "manifestVersion": 1,
  "id": "music",
  "name": "Music",
  "description": "本地音乐播放器",
  "version": "1.2.0",
  "author": "Community",
  "license": "GPL-3.0-or-later",
  "entrypoint": "/storage/heya/apps/music/bin/player",
  "args": ["--library", "/storage/heya/data/music/library.db"],
  "environment": ["MUSIC_HOME=/storage/heya/data/music"],
  "workingDirectory": "/storage/heya/data/music",
  "icon": "nf-md-music",
  "category": "media",
  "permissions": ["audio", "storage"],
  "features": ["resume-support"]
}
```

阅读器：

```json
{
  "manifestVersion": 1,
  "id": "books",
  "name": "Books",
  "description": "离线电子书阅读器",
  "version": "0.8.0",
  "entrypoint": "/usr/bin/books",
  "args": ["--library", "/storage/heya/downloads/books"],
  "icon": "nf-md-book-open-page-variant",
  "category": "reader",
  "permissions": ["storage"],
  "features": ["resume-support"]
}
```

游戏类应用（仍是普通应用）：

```json
{
  "manifestVersion": 1,
  "id": "retroarch",
  "name": "RetroArch",
  "description": "多核心游戏应用；ROM 由用户管理",
  "version": "1.19.0",
  "entrypoint": "/usr/bin/retroarch",
  "args": ["-L", "/usr/lib/libretro/core.so"],
  "icon": "nf-md-controller-classic",
  "category": "games",
  "permissions": ["gamepad-input", "storage", "audio", "video"],
  "features": ["gamepad-input"]
}
```

### 版本兼容与降级

1. 新 manifest 发布者写 `manifestVersion: 1`，并提供稳定 ID。
2. 向后兼容的可选字段在 v1 增加只允许有语义默认值，旧消费者忽略字段或直接忽略该应用；不得改变既有字段语义、类型或必填约定。
3. 旧文件缺 `manifestVersion` 时视为 v1 兼容输入；缺 `id` 时以目录名小写化（空格转 `-`）后补入内存。读取时不自动改写文件。
4. 未知版本（例如 2）不得猜测或按 v1 启动：跳过该应用，保留其他应用，报告包含目录和版本的错误。未知字段也按 manifest 严格解码失败处理。
5. 需要不兼容语义时发布新版本；可在升级器提供迁移期读取旧格式，再原子写入新格式。不要只提高 schema 版本却让旧 Launcher 误执行。
6. Launcher 是故障隔离边界：一个无效/未来版本应用不得导致整页扫描失败；冲突 ID 只拒绝冲突项并报告。

### 扩展评估与风险

- `permissions`、`features` 值采用开放字符串，形成可演进的声明词汇；当前展示声明，不意味着容器、seccomp 或文件访问控制。UI 不得显示成已授予/已隔离。
- Nerd Font 图标是主方案：小体积、适合手持低分辨率、与现有字体资产一致。`iconColor`/`badge`/`accent` 可由未来主题支持，现阶段没有布局与对比度规范，不应成为必填。PNG 可作为未来可选 fallback，不作为主协议要求。
- `dependency`/`plugin` 会引入解析顺序、循环依赖、ABI 与故障隔离；本期不预留承诺字段。`repository`、`updateURL`、`autoUpdate`、OTA 牵涉信任、签名、回滚与供应链，不能通过单个 URL 字段安全解决。`multiLanguage` 需要明确 locale fallback 和复数规则；可暂用系统语言/单一 name。未来需求成熟后用有版本的扩展块。
- 生命周期钩子 `preLaunch`/`postLaunch` 会引入命令执行、失败次序、超时、重复启动及关机路径等风险；当前收益有限，应由应用自管理，不加入 manifest。
- 启动环境是当前进程用户权限；权限字段不限制进程。环境值按数组传递，entrypoint 和 args 不经 shell，从协议入口层避免 shell 注入。未来若引入脚本化安装钩子，必须单独评审信任边界。

## 长期稳定性承诺

最难修改且必须尽早稳定：Application ID、manifest 版本/兼容策略、entrypoint/args 启动语义、`/storage/heya` 中用户数据与缓存边界、Provider 输出的统一应用语义。可重写：Launcher 状态机、SDL UI、渲染、扫描实现、系统读取实现。目录布局与 manifest 已发布即视为外部协议，变更须保留迁移读取路径和明确版本策略。