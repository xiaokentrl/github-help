# 设置面板 · 关于我们（`views/settings/panes/about/`）

- **这一格管什么**：应用名、一句话简介、三个入口钮（项目主页／反馈问题／许可协议，点了各回执一句），底下那张"键 值"四行。
- **值从哪来**：当前版本与构建日期读 `state.update`（跟「检测升级」那一格同一个源）；数据目录读 `api.status()` 回来的那一个，不在这里猜本机路径（已定：C5 那条）。这一格 `hasSave:false`。
- **谁能用它**：只有 `views/settings/SettingsView.js` 通过 `panes/index.js` 扫到它。
