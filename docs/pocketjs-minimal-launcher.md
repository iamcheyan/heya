# CPR72 阶段8：最小可用 Launcher 闭环

状态：**部分验证**。浏览器 host 端完成完整闭环（启动→运行→退出→返回首页）；native X11 端渲染与输入已验证，但按键注入 launch 未成功（窗口定位问题，见下）。

## 功能范围审计（任务书约束）

任务书要求：只允许 **应用列表 / 启动 / 返回 / 刷新 / 状态栏**；禁止主题系统、商店、联网、下载器、账户系统。

| 功能 | 状态 | 说明 |
|---|---|---|
| 应用列表 | ✅ 有 | Registry 扫描结果渲染为 carousel（FocusGrid + Focusable） |
| 启动 | ✅ 有 | IPC `launch(app_id)`，仅启动 Registry 已发现应用 |
| 返回 | ✅ 有 | `app_exited` 事件驱动，状态栏回 `HEYA READY`，消息提示 returned to heya |
| 刷新 | ✅ 有 | IPC `rescan()`（启动时 + Quick Menu） |
| 状态栏 | ✅ 有 | 显示 `RUNNING <app_id>` 或 `HEYA READY`，另有 Wi-Fi/Battery/clock 占位 |
| 主题系统 | ✅ 未引入 | Guest 用固定 Tailwind class，无运行时主题切换 |
| 商店 | ✅ 未引入 | 无 |
| 联网 | ⚠️ 部分 | Guest 用 `net.fetch` 访问**本机 loopback** bridge（127.0.0.1），非外部网络；无互联网访问 |
| 下载器 | ✅ 未引入 | 无 |
| 账户系统 | ✅ 未引入 | 无 |
| 收藏（favorite） | ⚠️ 额外 | 不在任务书五项之列，但也不在禁止列表。属原型遗留功能，若严格遵循"只允许五项"应移除 |

## 已验证：浏览器 host 完整闭环（阶段2 证据）

脚本 `pocketjs/tools/dev-bridge/ui-smoke.sh`，输出：
```
UI launch -> app_started/app_exited -> returned; get_status; rescan alpha -> alpha,beta -> beta; no page errors
```

流程与证据：

1. **启动**：Playwright 按 `KeyX`（CROSS）触发 Guest `launch()`；bridge `/cmd` 返回 `ok`，`status.running=true`、`app_id=alpha`、`pid` 已设。
2. **运行**：Guest 收到 `app_started` + `status_changed` 事件，状态栏切到 `RUNNING alpha`。
3. **退出**：应用（`/usr/bin/true`）退出，bridge 发 `app_exited` + `status_changed`；Guest 轮询获取，状态栏回 `HEYA READY`，消息 `alpha exited · returned to heya`。
4. **返回首页**：截图 `pocketjs/screenshots/bridge-launch-return.png` 显示卡片列表与 HEYA READY，`get_status` 返回 `running=false`。
5. **刷新**：Registry 增删后 rescan，`alpha → alpha,beta → beta`，canvas 状态变化。

## 已验证：native 渲染与输入（阶段3 证据）

- native host（`pocket-desktop-host`）在 X11 `:20` 启动，输出 `READY`，`--quit-after` 正常退出。
- `xdotool` 向窗口发送 `Right Right Down space` 与 `ctrl+q`，进程在 `ctrl+q` 后退出 → native host 接收键盘输入。
- native 按键映射完整（`programs/desktop/src/buttons.rs`）：`z/enter`→CROSS、`x/backspace`→CIRCLE、`a`→SQUARE、`s`→TRIANGLE、`tab`→SELECT、`space`→START、`q/l`→L、`w/r`→R。**框架/host 侧支持任务书全部按键**；阶段4 记录的 START/SELECT/L/R 无响应是 Guest 未绑定 `onButtonPress` 所致。

## 未完成 / 阻塞

- **native 端 launch 按键注入未成功**：尝试用 `xdotool` 向 native 窗口发送 `z`（CROSS）触发 launch，未观测到 bridge `app_started` 事件。原因：后台启动时窗口未稳定映射（`xdotool search --name` 返回空），属测试工具/时序问题，非架构阻塞。阶段2 已在浏览器 host 验证同一 `launch()` 代码路径（Guest 侧逻辑一致）。
- Guest 的 START/SELECT/L/R 未绑定（阶段4 问题），若补齐可在 native 端验证更多按键。

## 结论

最小闭环的**功能与协议**已在浏览器 host 实测通过（启动→运行→退出→返回首页 + 刷新 + 状态栏），功能范围符合任务书约束（无主题/商店/下载器/账户；联网仅限本机 loopback bridge）。native 端已验证渲染与键盘输入，但 launch 的按键注入闭环待补（工具时序问题）。
