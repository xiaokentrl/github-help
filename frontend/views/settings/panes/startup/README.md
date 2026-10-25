# 设置面板 · 开机自启动（`views/settings/panes/startup/`）

- **这一格管什么**：四个开关——登录时自动启动、启动后最小化、自动检测已选、自动应用队列；全部点「保存」才生效。
- **值从哪来**：键名与默认值在 `core/settings.js` 的 `DEFAULTS`（`autoStart`／`startMinimized`／`autoDetectOnStartup`／`autoApplyOnStartup`）；句子在 `core/i18n/locales/zh-CN.js` 的 `settings.startup.*`。本文件一处都不写死。
- **这条联动谁定**：「自动应用」依赖「自动检测」。规矩的家在 `core/settings.js` 的 `normalize()`（检测没开就把应用打回关闭），这里画的是它的另一半：检测一关，应用那颗既按不动、整张卡片一起灰（原型 :1516-1530）。
- **谁能用它**：只有 `views/settings/SettingsView.js` 通过 `panes/index.js` 扫到它；启动自动化真跑不跑归 `core/flows.js` 读这两个键。
