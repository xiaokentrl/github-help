# 设置面板 · 外观（`views/settings/panes/appearance/`）

- **这一格管什么**：语言下拉（点「保存」才生效，切完提示下次启动）与主题那一行（点「选择主题…」立刻换，不重启）。
- **值从哪来**：可选语言＝`core/i18n/locales/` 扫出来的那几门，每门自己的写法＝`core/i18n/languages.js`；六个主题名＝`styles/themes/` 扫出来的那一份。本文件两处都不列名单。
- **谁能用它**：只有 `views/settings/SettingsView.js` 通过 `panes/index.js` 扫到它；那颗「选择主题…」把弹窗交给 `views/theme-picker/`。
