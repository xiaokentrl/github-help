# 设置面板 · 布局（`views/settings/panes/layout/`）

- **这一格管什么**：主菜单宽度、队列高度两档（点一下立刻生效并落盘），检测／应用时焦点跟随两个开关（点「保存」才生效）。
- **值从哪来**：档位数字的家在 `core/settings.js`（`SIDEBAR_PRESETS`／`QUEUE_PRESETS`／上下限），句子在 `core/i18n/locales/zh-CN.js` 的 `settings.layout.*`；本文件一处都不写死。
- **谁能用它**：只有 `views/settings/SettingsView.js` 通过 `panes/index.js` 扫到它。加一格＝加一个 `<面板id>/pane.js`（＋这份三行说明），别的文件一个字不改。
