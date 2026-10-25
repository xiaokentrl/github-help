# 设置面板 · 界面缩放（`views/settings/panes/zoom/`）

- **这一格管什么**：四档缩放（90%／100%／110%／125%），点一下立刻改 `document.body.style.zoom` 并落盘。
- **值从哪来**：档位数字的家在 `core/settings.js` 的 `ZOOM_PRESETS`（`normalize()` 认的就是这四个值）；百分比的字由数值现算，不另写四份句子。
- **谁能用它**：只有 `views/settings/SettingsView.js` 通过 `panes/index.js` 扫到它。
