# CPR72 阶段5：国际化

状态：**已实测**。8 类字符集（ASCII/UTF-8/中/日/韩/Emoji/混排/繁体）在 PocketJS 浏览器 host 上均渲染出字形，无缺字 tofu；字体加载失败场景为构建期硬失败（已实测）。

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

## 实测结果（`pocketjs/tools/dev-bridge/i18n-single.sh`）

方法：Registry 目录只放一个应用（确保它是选中卡片），逐个替换应用名并 reload，探测 480×272 canvas 名称区（y 42%–60%，x 28%–72%）的白色文字像素（whitish）与彩色像素（colored）。

| 字符集 | 样本 | whitish（字形像素） | 结论 |
|---|---|---|---|
| ASCII | `Hello World` | 366 | 渲染正常 |
| UTF-8 | `café naïve Ünïcödé` | 244 | 渲染正常（含 é ï Ü ö 等） |
| 中文 | `你好世界 中文测试` | 283 | 渲染正常 |
| 日文 | `日本語テスト こんにちは` | 292 | 渲染正常（汉字+假名） |
| 韩文 | `한국어 테스트 안녕하세요` | 312 | 渲染正常 |
| Emoji | `😀🎉🚀✅❤️` | 267 | 渲染为**单色字形**（非彩色，见下方限制） |
| 混排 | `Hello 你好 日本語 한국어 😀` | 373 | 渲染正常 |
| 繁体 | `繁體中文測試` | 184 | 渲染正常 |

- 所有样本 whitish 均 >180，说明名称区确有字形像素，**无缺字/tofu**。
- 彩色像素恒为 98（来自卡片边框等 UI 元素，非文字），说明文本本身以单色（白）渲染。
- Guest 运行时错误：`errors: []`。
- 证据截图：`pocketjs/screenshots/i18n-single-{ascii,utf8,chinese,japanese,korean,emoji,mixed,traditional}.png`；多卡片版 `i18n-card-*.png`。

## 字体加载失败场景（实测）

- 操作：将 `apps/heya-pocket/fonts.json` 的 `fallback` 指向不存在的 `assets/fonts/DoesNotExist.otf`，重新构建。
- 结果：**构建期硬失败**，非静默降级：
  ```
  error: PocketJS fonts.json: file not found: .../assets/fonts/DoesNotExist.otf
      at fail (framework/compiler/font-config.ts:15:56)
  EXIT=1
  ```
- 更正：先前推测"缺失 fallback 可能静默降级"是错的；实际 `font-config.ts` 对 fallback 文件执行 `existsSync` 校验并 `fail()` 报错，构建中止。这有利于提前发现字体缺失，但也意味着部署时必须保证 fallback 字体存在。

## 限制

- **Emoji 为单色**：baked 字体模型下 emoji 被烘焙为单色字形（实测彩色像素无增加），不呈现彩色 emoji。这是 PocketJS baked text 的固有限制。
- **baked 字形覆盖**：`fonts.json` 的 `characters` 未声明的码点运行时无字形。本次通过 `--extra-chars` 覆盖了测试字符；生产需保证字符集完整声明，否则新增字符会缺字。
- 本阶段在浏览器 host（480×272）验证；native host（720×480）的 CJK 渲染依赖同一 baked pak，未单独截图验证。
- Noto Sans CJK fallback 生效：构建日志 `fonts: 1 fallback face(s)`，且中日韩样本均渲染出字形（whitish 283–312），证明 fallback 参与烘焙并生效。
