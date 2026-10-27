# CPR72 阶段5：国际化

状态：源码与配置调查完成；浏览器/native 实测待执行。

## 字体与回退机制

- `apps/heya-pocket/fonts.json`：
  ```json
  {
    "fallback": ["../../assets/fonts/NotoSansCJK-Demo.otf"],
    "characters": "ステト世中你好文日本界繁語體국스어테트한"
  }
  ```
- 编译器侧 `framework/compiler/font-config.ts` 读取 `fallback`（字体文件路径数组）与 `characters`（需烘焙的字形码点），也支持 `characterFiles`、`ranges`。
- 构建日志中可见 fallback 生效：`fonts: 1 fallback face(s), 20 declared characters`，并生成多个 font slot（12/14/16/24px regular 与 14/18/20px bold 等，各 113 glyphs）。
- CJK 渲染依赖构建期烘焙的 fallback atlas；未包含在 `characters` 中的字形运行时不可得（这是 baked 字体模型的固有限制）。

## 已验证（阶段1/2 截图附带）

- 阶段1 Registry 扫描 `examples/font-samples` 得到六个应用，名称包含中文（`你好 Hello 日本語`、`你好世界 Hello`、`繁體中文 Hello`）、日文（`日本語テスト Hello`）、韩文（`한국어 테스트 Hello`）、拉丁（`Hello World`）与混排。
- 这些名称在 PocketJS 浏览器 host 的 480×272 canvas 上渲染（截图 `registry-ui-*.png` 为 alpha/beta 临时应用；CJK 名称渲染证据见 CPR69 阶段0 的 `pocketjs-cjk2.png` 等历史截图与本报告阶段1记录）。

## 待验证项（阶段5）

按任务书要求，需覆盖并记录：

- ASCII
- UTF-8
- 中文
- 日文
- 韩文
- Emoji
- 混排
- Noto Sans CJK fallback 生效
- 字体加载失败场景（例如 fallback 字体缺失/路径错误时的行为）

需输出：渲染正确性、缺字情况、截图证据。

## 已知风险（待实测确认）

- baked 字体模型：`characters` 未声明的码点在运行时无字形，可能显示空白或 tofu；Emoji 若未声明码点可能缺字。
- 字体加载失败：编译器 `font-config.ts` 对不存在文件返回空 `fallbackTtfs`（`if (!existsSync(path)) return { fallbackTtfs: [], codepoints: [] }`），因此缺失 fallback 可能静默降级而非报错——需在阶段5实测确认并记录。
