# 设置面板 · 托盘（`views/settings/panes/tray/`）

- **这一格管什么**：两个开关——关闭窗口时最小化到托盘、显示托盘图标。两个都是点「保存」才生效。
- **值从哪来**：键名与默认值在 `core/settings.js` 的 `DEFAULTS`（`trayMinimize`／`showTrayIcon`）；句子在 `core/i18n/locales/zh-CN.js` 的 `settings.tray.*`。
- **谁能用它**：只有 `views/settings/SettingsView.js` 通过 `panes/index.js` 扫到它。真正关窗口的行为归 GTK 壳那边，本轮是 mock，点了只有"保存了"这句回执。
