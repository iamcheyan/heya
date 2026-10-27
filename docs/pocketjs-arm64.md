# CPR72 阶段7：ARM64

状态：**部分验证**。aarch64 交叉编译成功、QEMU 下二进制可执行；目标设备（RG34XX）实测未完成（无实机、hx90 不可达、QEMU 缺 aarch64 X11 sysroot）。

## 环境调查

| 途径 | 结果 |
|---|---|
| RG34XX 实机 | 无 |
| hx90（内网主机，任务书提供） | **不可达**（`No route to host`） |
| QEMU user（qemu-aarch64） | 本机可用（`/usr/bin/qemu-aarch64`、`qemu-aarch64-static`） |
| aarch64 交叉工具链 | 初始无；已安装 `gcc-aarch64-linux-gnu`（4:14.2.0-1），Rust 已加 `aarch64-unknown-linux-gnu` target |

## 交叉编译（成功）

```sh
rustup target add aarch64-unknown-linux-gnu
export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-linux-gnu-gcc
cargo build --release --target aarch64-unknown-linux-gnu --manifest-path programs/desktop/Cargo.toml
```

- 首次编译：4m41s，`EXIT=0`。
- 产物：`target/aarch64-unknown-linux-gnu/release/pocket-desktop-host`（2,259,488 bytes）。
- `file` 验证：`ELF 64-bit LSB pie executable, ARM aarch64, version 1 (SYSV), dynamically linked, interpreter /lib/ld-linux-aarch64.so.1` → **确为 aarch64**。

### 带 profile 重编译

裸 `cargo build` 不含导出 profile，运行会报 `pocket-desktop-host was built without an exported profile`。设置 `POCKETJS_PROFILE_RS=<dist/linux-heya/pocket_profile.rs>` 后重编译（9.79s）成功，产出含 linux-app profile 的 aarch64 二进制。

## QEMU 运行（部分成功）

```sh
qemu-aarch64 -L /usr/aarch64-linux-gnu ./pocket-desktop-host --app heya-pocket-main \
  --js heya-pocket-main.js --pak heya-pocket-main.pak --viewport 720x480 --density 1 \
  --title heyaARM --fixed --announce-ready --quit-after 120
```

- **第一次（无 profile）**：`Error: pocket-desktop-host was built without an exported profile; build it through pocket build` → 证明二进制**实际在 QEMU 下执行**，完成参数解析并运行到 profile 校验逻辑。
- **第二次（有 profile，DISPLAY=:20）**：`Error: os error at winit-0.30.13/src/platform_impl/linux/mod.rs:788: Failed to load one of xlib's shared libraries` → 运行到 X11/winit 初始化，因 **QEMU 用户态环境缺 aarch64 X11/xlib 库**而失败。

结论：aarch64 二进制**可执行**（CPU 指令与动态链接基本路径正常）；完整 GUI 渲染需 aarch64 X11/wgpu sysroot（当前不可用）。

## 未完成 / 阻塞

- **无目标设备性能数据**：启动时间、内存、CPU、帧率、输入延迟、发热、长稳（15/30/60 分钟）均未测。
- **hx90 不可达**：`ssh <用户>@<内网 hx90 主机>` → `No route to host`；无法用其 ROCKNIX 构建容器。
- **QEMU sysroot 不全**：`/usr/aarch64-linux-gnu` 仅提供基础 libc（来自 gcc-aarch64-linux-gnu），缺 X11/libxcb/wgpu 依赖，无法完成 GUI 运行。
- **未提供缩短采样长稳替代**：因无可用 ARM64 运行环境，无法执行。

## 对任务书问题的回答

**"ARM64？"**：**部分验证**。PocketJS 可交叉编译为 aarch64 并在 QEMU 下启动执行（架构支持成立）；但 **RG34XX/ARM64 设备的实际性能、资源占用、稳定性完全未验证**，不足以支撑设备上量决策。
