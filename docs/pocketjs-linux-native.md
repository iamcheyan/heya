# CPR72 阶段3：PocketJS Linux Native Host 调查

状态：部分验证。PocketJS 0.15.0 的 `linux-app` 目标与 desktop host 在本机可编译；heya Guest 已从浏览器全局 API 迁移到原生 API 并通过编译。native 运行时渲染/输入实测仍在进行。

## 源码证据

- `profiles/linux-app.ts`：定义 Linux window/native profile，1280×800 physical viewport，800×600 logical，`presentations: ["native"]`，capabilities 含 `input.buttons`、`display.viewport.live`、`text.glyphs.baked`、`io.offload`、`text.layout.offload`，60 Hz。
- `programs/desktop/linux/pocket.runtime-config.ts`：指向 `tools/build/programs/desktop.ts` 的 `linuxBuild`。
- `tools/build/programs/desktop.ts`：Linux desktop recipe 打包 `pocket-desktop-host`，复制 `.js/.pak/app.pocket/app.plan.json`，生成 `run` 脚本，执行 `./pocket-desktop-host --app ... --js ... --pak ... --viewport WxH --density N --title ...`。
- `programs/desktop/src/main.rs`：winit window + wgpu surface + 输入事件映射（键盘/鼠标/IME/按钮）；`--quit-after-ticks N` 可在无人值守下退出。
- `platforms/linux`：`drm`（DRM/KMS dumb buffer scanout）、`fbdev`（framebuffer packed true-color）、`evdev`（非阻塞键盘/游戏手柄/触摸）、`alsa`（libasound PCM）。这些是 component crates，不拥有 Ui/客体。
- 本机 Rust 1.96.0，已安装 `x86_64-unknown-linux-gnu` target；`Xvfb`、`weston` 可用；`pkg-config` 已有 `wayland-client`、`xkbcommon`、`libudev`。

## 实测

- `cargo check --manifest-path programs/desktop/Cargo.toml` 在本机通过（Rust 1.96.0，`x86_64-unknown-linux-gnu`，约 2m47s），stock Linux desktop host crate 可编译。
- 官方 demo `apps/hero` 编译通过：`bun tools/pocket.ts compile --manifest apps/hero/pocket.json --project-root . --target linux-app --output dist/hero-linux.pocket` → pak 22 entries / 1,034,592 bytes，js 212,070 bytes。证明 `linux-app` 目标工具链可用。
- heya Guest 首次编译 **失败**（真实阻塞）：`main.tsx:82 TS2304 Cannot find name 'requestAnimationFrame'`、`main.tsx:113/121 TS2304 Cannot find name 'window'`。根因：QuickJS native guest 无 `window`/`requestAnimationFrame`/真实 `setTimeout`；官方文档 `docs/DETERMINISM.md` 与 `framework/src/clock.ts` 指明 `after()` 是 `setTimeout` 的确定性替代（deadline 为帧索引）。
- 修复后编译通过：Guest 的 `requestAnimationFrame` → `after(0, ...)`，`window.setInterval` → `onFrame(() => { if (++tick % 60 === 0) ... })`。产出 `dist/heya-pocket-linux.pocket`：pak 8 entries / 306,176 bytes，js 202,872 bytes，`ResolvedBuildPlan sha256:6dba049a9ecaf588c418aac59c1d1e9b62ac0eb1fa3e087cbb1ee40a0957894f`。
- 结论：heya Guest 原先依赖浏览器全局 API，是其在 native 路径下的真实阻塞点；改用 PocketJS 原生 API 后 linux-app 编译路径打通。仍在验证 native 运行时（`pocket build` + `pocket-desktop-host`）能否在无浏览器下渲染并接收输入。

## 限制

- 本机无 aarch64 交叉工具链；阶段7 需要时再想办法（QEMU user / hx90 容器）。
- 本机无物理 DRM/GPU 设备；验证必须在 headless Xvfb 下进行，不能证明真实 GPU 性能。