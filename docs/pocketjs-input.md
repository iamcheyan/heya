# CPR72 阶段4：输入系统

状态：**已实测**（浏览器 host，heya Guest，3 个临时应用 alpha/beta/gamma）。

## 输入映射表（浏览器 host KEYMAP → BTN → Guest 行为）

官方常量（`contracts/spec/spec.ts`）：

| 常量 | 值 | PSP 语义 | RG34XX 语义 |
|---|---|---|---|
| SELECT | 0x0001 | Select | Select |
| START | 0x0008 | Start | Start |
| UP | 0x0010 | D-pad 上 | D-pad 上 |
| RIGHT | 0x0020 | D-pad 右 | D-pad 右 |
| DOWN | 0x0040 | D-pad 下 | D-pad 下 |
| LEFT | 0x0080 | D-pad 左 | D-pad 左 |
| LTRIGGER | 0x0100 | L | L |
| RTRIGGER | 0x0200 | R | R |
| TRIANGLE | 0x1000 | Y | X |
| CIRCLE | 0x2000 | B | A |
| CROSS | 0x4000 | A | B |
| SQUARE | 0x8000 | X | Y |

浏览器 host 键位（`programs/web/dev/engine.js`）：Arrow→Dpad；KeyX→CROSS；Enter/KeyZ→CIRCLE；KeyA→SQUARE；KeyS→TRIANGLE；Shift→SELECT；Space→START；KeyL/KeyQ→LTRIGGER；KeyR/KeyE→RTRIGGER。

Native desktop host（`programs/desktop/src/main.rs`）：winit `KeyboardInput` → `key_name` → `button_for` → BTN；Ctrl+Q 退出。

## 实测结果（`pocketjs/tools/dev-bridge/input-smoke.sh`）

环境：PocketJS 0.15.0 浏览器 host + heya Guest（3 个临时 Registry 应用），Playwright headless，逐键按下 160ms 后释放，比较 canvas framebuffer hash。

| 按键 | 语义 | 画面是否改变 | 结论 |
|---|---|---|---|
| ArrowRight | RIGHT | true | 响应（焦点移动） |
| ArrowLeft | LEFT | true | 响应 |
| ArrowUp | UP | true | 响应 |
| ArrowDown | DOWN | true | 响应 |
| KeyX | CROSS (A) | true | 响应（launch） |
| KeyZ | CIRCLE (B) | true | 响应（关菜单） |
| KeyA | SQUARE (X) | true | 响应（开/关菜单） |
| KeyS | TRIANGLE (Y) | true | 响应（收藏） |
| Space | START | **false** | **未绑定，无响应** |
| Shift | SELECT | **false** | **未绑定，无响应** |
| KeyQ | L | **false** | **未绑定，无响应** |
| KeyE | R | **false** | **未绑定，无响应** |

其他行为：

- **长按**（CROSS 按住 1200ms）：`changedDuring=true`、`changedAfter=true` → 按下与释放均引起画面变化（active 变体 + launch）。
- **连按/重复触发**（SQUARE 快速按 5 次）：`repeatToggles=5` → 每次边沿都触发（`onButtonPress` 为边沿检测 `pressed = buttons & ~prevButtons`）。
- **菜单切换**：SQUARE 开菜单（`opened=true`）、CIRCLE 关菜单（`closed=true`）均有效；但**菜单内 ArrowDown 移动焦点 `moved=false`**（画面未变）。
- **无 Guest 运行时错误**（`errors: []`）。

## 问题列表

1. **START / SELECT / L / R 未绑定**：heya Guest 只绑定了 LEFT/RIGHT/CROSS/TRIANGLE/SQUARE/CIRCLE。任务书要求的 Start、Select、L、R 在当前原型中无响应。这是覆盖缺口，不是框架缺陷（BTN 常量与 host 映射均支持）。
2. **菜单内焦点移动无效**：Quick Menu 打开后 ArrowDown 未改变画面。原因分析：Guest 用 `pushFocusScope(menuRoot, { autoFocus: true, restoreFocus: false })` 推入 scope，但菜单内 `Focusable` 项焦点移动未生效——可能是 `FocusGrid`/scope 内 focusable 收集问题，或菜单项未注册为 focusable。需进一步定位（阶段4未完成项）。
3. **UP/DOWN 在卡片列表中的作用**：Guest 的 `move()` 只绑定 LEFT/RIGHT；UP/DOWN 虽改变画面（可能是 spring 动画或焦点样式），但未绑定为列表前后移动。功能语义不完整。

## 证据截图

`pocketjs/screenshots/input-*.png`（逐键）与 `input-menu-*.png`、`input-hold-cross.png`、`input-repeat-square.png`。

## 焦点语义（`framework/src/input.ts`，源码事实）

- 默认遍历为文档顺序（DFS）；grid 外 DOWN/RIGHT→next，UP/LEFT→previous。
- `FocusGrid` 用 columns/wrap 覆盖子树内遍历。
- CIRCLE 触发焦点节点 `onPress` 并向上冒泡到最近有 handler 的祖先。
- 焦点丢失修复顺序：下一兄弟子树 → 上一兄弟子树 → 最近可聚焦祖先 → none（unlink 前计算）。
- 每次焦点变更调用 `ops.setFocus`，native 应用 `focus:` 变体；按住 CIRCLE 时 `ops.setActive` 应用 `active:` 变体，释放/焦点变更即清除。
