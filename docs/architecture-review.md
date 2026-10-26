# heya 架构评审

## 结论摘要

heya 是面向掌机的 Pocket Linux Shell，不是 ES 替代品、ROM 管理器、游戏前端或游戏库。长期协议边界是 Application Manifest、Provider 输出和 `/storage/heya` 目录布局；Launcher、UI、扫描实现和系统探测代码都可以替换。Phase 1 必须先固定协议身份和兼容规则，并提供轻量 Provider 与能力接口；不应先实现游戏专用路径、插件装载器、在线仓库或完整系统服务框架。

## Part 2：Registry 与 Provider

### 现状

`internal/registry/registry.go` 由 `Registry.Scan()` 单体扫描 `apps/*/app.json`，校验后排序，`Page()` 转为 UI 页面。它只处理应用，但 Registry 名称与职责已被描述为未来还要处理 Apps/ROMs/Configs。Launcher 接收普通 `model.Item`，并不认识 ROM，这符合 Games are Applications 的方向。

### 问题

- 扫描与应用提供者耦合，未来如果把 ROM/游戏资料直接放进 `Scan()`，Launcher 将被迫区分游戏类型，违反统一应用模型。
- 配置、音乐库、书目等来源不是可执行 Application；若都塞入相同 Registry 返回类型，会模糊职责。
- 当前没有稳定 provider contract，替换文件系统扫描或聚合来源会牵动启动代码。

### 建议与成本

Phase 1 建立最小 `Provider` 接口（输出 `[]model.App` 与逐项错误）、`ApplicationsProvider`，Registry 作为聚合器保留历史 `New(dir).Scan()` 接口。应用 provider 负责读取文件系统清单；任何 games/books/music provider 要么产生标准 App，要么由一个容器 Application 承载浏览/选择流程。Launcher 只接触 Application，不查询 ROM、不按游戏类别分支。

收益是来源可替换，插件实现可逐步加入，单个 provider 错误可以隔离。代价是多一层接口、测试替身和错误汇总；目前不值得做动态插件加载、依赖注入框架或多层 provider registry。Provider 接口是内部 Go API，不是跨进程/磁盘协议，待实现需求驱动可重写。

### 迁移步骤

1. 保留 `Registry.Scan()` 和 `Page()`，改为聚合 Provider，调用方不必一次迁移。
2. 将原扫描逻辑移入 `ApplicationsProvider`，在读取边界补全旧 manifest 的 v1 与目录 ID。
3. 维护重复 ID、未来版本、未知字段为逐项问题；其他应用继续显示。
4. 将来要加入书籍/音乐来源时，先判定产物是否真是可启动 Application；配置仍由专属模块消费，不混进应用注册表。
5. 如引入第三方 provider，另行定义版本化 IPC/ABI、信任与崩溃边界；当前 Go interface 不能承诺插件 ABI。

Phase 1 不实现 ROM Provider、游戏管理器、插件 ABI。游戏应用的 `category: games` 只是普通分类字符串，不触发核心特殊行为。

## Part 3：System 模块边界

### 现状

`internal/system` 一个包读取 battery、temperature、memory、storage、network/Wi-Fi/Bluetooth、volume、brightness、ZRAM，借助 sysfs/procfs 和命令行探测，缺失时标 unknown。`Launcher` 直接持有 `*system.Probe`，并负责把系统快照格式化成状态行。

### 问题

增长到 WiFi、Bluetooth、Battery、Audio、Storage、Power、OTA 后，单文件会混合只读探测、控制副作用、命令适配和数据结构。特别是 OTA 属于更新/恢复流程，不能简单加成一个探测方法；Power 控制也要明确权限与失败语义。

### 建议与迁移

Phase 1 保持单包、按领域函数隔离当前 probe，添加能力查询接口，避免为尚未存在的功能过度拆包。达到具体维护压力时再拆为 `internal/system/{battery,wifi,bluetooth,audio,power,storage,display}`：每个领域拥有自己的 Probe/Controller 和测试；上层组合成兼容的系统 Snapshot。OTA 独立为更新子系统，带签名、回滚和恢复设计，不属于普通 status probe。

提前拆包的收益是依赖和权限边界清楚、团队并行容易；代价是包间接口、更多 wiring、共享状态协调与当前小代码库的跳转成本。分拆应由领域确实需要独立控制、模拟或生命周期时触发，而非仅因目录看起来整齐。当前 `system` 仍可作为 facade，避免 Launcher 一次迁移多个模块。

## Part 4：Capability Layer

### 现状与风险

当前 `system.Status` 通过 `Known` 字段表达部分硬件/探测结果，Launcher 直接显示值；没有显式 Supports API。新增菜单控制时若以设备型号判断（如 `if RG34XX`），型号、板卡 revision、内核驱动和发行版将绑进 UI，条件会随设备扩增。

### 建议

Phase 1 增加极小的 `internal/capabilities.Device` 能力接口：`SupportsWiFi()`、`SupportsBluetooth()`、`SupportsBrightness()`、`SupportsSuspend()`、`SupportsBattery()`。当前 `internal/system.Probe` 实现该接口，事实来源是驱动/节点/服务探测；未来 UI 依赖抽象，不依赖型号名称。能力表示可用性，不等同于当前状态或操作成功；操作仍由系统控制器执行，必须报告运行时错误。

这层现在成本很低且边界明确，值得建立；不需要全局 DI 容器，也不需要将每个属性都抽象。未来硬件差异增加后可以拆成按领域接口。`SupportsWiFi` 此类便捷方法不得掩盖需要异步发现、策略、权限或操作结果的复杂服务。

## Part 5：长期生态与协议稳定性

### Pocket Linux Shell 演进判断

定位天然支持 Pocket Linux Shell：它可从统一启动壳逐步成为应用目录、输入/显示前端和系统状态入口。但这不要求 heya 管理软件包、ROM 或系统升级；应把 OS 平台能力与应用商店职责分开。

### 最难修改的协议

| 边界 | 修改成本 | 必须早期稳定的决定 |
|---|---|---|
| Manifest 与 ID | 高，用户清单、应用安装和数据目录都引用 | 版本号、ID 规则、入口/参数、安全启动语义、向后兼容和未知版本处理 |
| Provider 语义 | 中高，来源与 Launcher/应用模型耦合 | Provider 产物仍是普通 Application；错误隔离；无 ROM 专用核心对象 |
| Directory Layout | 很高，备份、用户数据、OTA、卸载均依赖 | 持久 data 与可删 cache 分开；用户 downloads/themes 不被覆盖；应用 ID 命名策略 |
| Go 内部模块边界 | 中低 | 不承诺 Go interface 作为外部插件 ABI；按用例演进 |

Launcher 可重写，UI/SDL 可重写，Registry 的具体扫描实现可重写，`internal/system` 的读取/命令后端可重写。应用 manifest、目录含义、provider 的统一对象语义不能轻易重写；若必须更改应有 schema/协议迁移工具、旧版兼容窗口和回滚策略。

## Phase 1 决策表

| 决策 | 阶段 | 可执行工作 |
|---|---|---|
| 发布 v1 manifest，定义缺省旧格式映射及未来版本降级 | Phase 1 | `manifestVersion`、稳定 `id`、`entrypoint`/旧 `exec` 规则与 schema |
| 固定 `/storage/heya` 子目录职责 | Phase 1 | layout 常量；创建 data/cache/logs/downloads，不动现有 apps/themes |
| Provider 接口与 applications provider | Phase 1 | 保留旧 Registry facade，应用统一为 `model.App` |
| capability 设备接口 | Phase 1 | 用系统节点/工具可用性探测，不写型号分支 |
| permissions/features 声明展示 | Phase 1 | 明示无隔离语义；不据此拒绝系统访问 |
| Wi-Fi/蓝牙/音量/亮度完整控制器拆分 | 延后 | 具体控制操作出现时为每个领域设计失败、权限和测试边界 |
| ROM Provider、插件 ABI、软件仓库、依赖解析 | 延后 | 有真实应用与信任模型后评估；不改变 Launcher 的统一 Application 输入 |
| OTA/自动更新 | 延后 | 必须先有签名、回滚、失败恢复和用户数据保护设计 |
| 多语言 manifest 与生命周期钩子 | 延后 | 明确 locale fallback、钩子超时和失败语义之后再版本化 |

## 验收约束

所有路径均应从 `layout` 常量出发；扫描坏项不破坏其他应用；启动始终是 argv 直接执行；UI 不按设备型号或游戏类别分支；用户数据目录与系统 OTA 更新解耦。每次协议变化必须增加旧清单回归测试与至少一个新协议示例。
