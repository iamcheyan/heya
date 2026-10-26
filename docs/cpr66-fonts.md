# CPR66: 支持中日韩字体的 launcher UI

heya 在 SDL_ttf 的主字体外支持 CJK fallback font；字体由主题 JSON 配置。代码独立识别 CJK/非拉丁文字段并选择字体，所以 JetBrainsMono Nerd Font 的英文、图标仍保持原有形态。

## 主题字段

```json
{
  "font": "assets/JetBrainsMonoNerdFont-Regular.ttf",
  "font_cjk": "assets/NotoSansCJK-Regular.ttc",
  "scale": 16
}
```

`font_cjk` 可省略；应用会尝试系统 Noto CJK、WenQuanYi 字体。主题内的相对字体路径按主题 JSON 所在目录解析。默认主题随 ROCKNIX 包包含 Noto Sans CJK TTC 字体和字体许可文件。

## 验收

- 中英文示例 app 标签可正常显示，CJK 无 tofu 方框或截断。
- `SDL_VIDEODRIVER=dummy go test ./...` 通过。
- `SDL_VIDEODRIVER=dummy make dump` 输出三张页面图；中文标签的实际渲染已目视验收。

更新默认主题或字体时，不覆盖 `/storage/heya/themes/theme.json` 中用户自己的配置。
