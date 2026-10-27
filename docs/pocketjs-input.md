# CPR72 阶段4：输入系统

状态：源码调查完成；浏览器 host 实测待执行（依赖阶段3 native 构建完成后一并验证）。

## 官方按键常量（`contracts/spec/spec.ts` BTN）

| 常量 | 值 | 语义 |
|---|---|---|
| SELECT | 0x0001 | Select |
| START | 0x0008 | Start |
| UP | 0x0010 | D-pad 上 |
| RIGHT | 0x0020 | D-pad 右 |
| DOWN | 0x0040 | D-pad 下 |
| LEFT | 0x0080 | D-pad 左 |
| LTRIGGER | 0x0100 | L |
| RTRIGGER | 0x0200 | R |
| ZL | 0x0400 | ZL |
| ZR | 0x0800 | ZR |
| TRIANGLE | 0x1000 | Y（三角） |
| CIRCLE | 0x2000 | B（圆） |
| CROSS | 0x4000 | A（叉） |
| SQUARE | 0x8000 | X（方） |

注：PocketJS 的 A/B/X/Y 语义按 PSP 命名（CROSS=A、CIRCLE=B、SQUARE=X、TRIANGLE=Y），与 heya/RG34XX 的按键语义需映射。

### 浏览器 host 键位映射（`programs/web/dev/engine.js` KEYMAP）

| 键盘 | BTN |
|---|---|
| ArrowUp / ArrowDown / ArrowLeft / ArrowRight | UP / DOWN / LEFT / RIGHT |
| KeyX | CROSS |
| Enter / KeyZ | CIRCLE |
| KeyA | SQUARE |
| KeyS | TRIANGLE |
| ShiftLeft / ShiftRight | SELECT |
| Space | START |
| KeyL / KeyQ | LTRIGGER |
| KeyR / KeyE | RTRIGGER |

### Native desktop host 键位映射（`programs/desktop/src/main.rs`）

winit `KeyboardInput` → `key_name`（arrows→up/down/left/right、enter、escape、space 等）→ `button_for(&name)` → BTN bit。Ctrl+Q 退出。

## 焦点语义（`framework/src/input.ts`）

- 默认遍历顺序为文档顺序（DFS，每次导航按键重算，对 UI 规模树开销可忽略）。
- `FocusGrid` 可在子树内用 row/column 语义覆盖遍历；grid 外 DOWN/RIGHT→next，UP/LEFT→previous。
- CIRCLE 触发当前焦点节点的 `onPress`，向上冒泡到最近有 handler 的祖先。
- 焦点丢失（节点移除）修复顺序：下一个兄弟子树 → 上一个兄弟子树 → 最近可聚焦祖先 → none；在 unlink 前计算。
- 每次焦点变更调用 `ops.setFocus`，native core 直接应用 `focus:` 变体，无每帧 JS。
- 按住 CIRCLE 时焦点节点带 `active:` 变体（`ops.setActive`，spec op 26），释放即清除；焦点变更/scope 变更/移除修复时也清除，避免粘滞。
- `FocusGrid` wrap：`right` 到行尾跳到行首、`left` 到行首跳到行尾、`down` 到底部跳到列首、`up` 到顶部跳到列底（见 `moveGridFocus`）。

## 待验证项（阶段4）

- Dpad（UP/DOWN/LEFT/RIGHT）焦点切换
- A/B/X/Y（CROSS/CIRCLE/SQUARE/TRIANGLE）动作
- Start / Select
- L / R
- 长按（held）行为
- 连按 / 重复触发（`onButtonPress` 为边沿触发：`pressed = buttons & ~prevButtons`）
- 菜单切换（`pushFocusScope` / `FocusScope`）
- `FocusGrid` / `Focusable` / `FocusScope` 行为

## heya Guest 当前按键绑定

```
onButtonPress(BTN.LEFT,  move(-1))
onButtonPress(BTN.RIGHT, move(1))
onButtonPress(BTN.CROSS, launch())
onButtonPress(BTN.TRIANGLE, toggleFavorite)
onButtonPress(BTN.SQUARE, toggle menu)
onButtonPress(BTN.CIRCLE, close menu)
```

未绑定：UP / DOWN / START / SELECT / LTRIGGER / RTRIGGER。这属于当前原型的覆盖缺口，阶段4会记录为问题列表。
