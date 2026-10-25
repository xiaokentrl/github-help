# 设置面板 · 检测升级（`views/settings/panes/update/`）

- **这一格管什么**：三个版本读数＋「检查更新」、发现新版本才露出来的那张（更新内容清单＋下载进度与「立即更新」）、底下那张版本历史。
- **值从哪来**：全部读 `core/store.js` 里的 `state.update`（当前版本、构建日期、最新、checking／downloading／downloaded、log、进度）。发请求归 `core/api/client.js` 那两个动词（`checkUpdate`／`downloadUpdate`），改状态归 `main.js`，本面板一个数也不改。
- **谁能用它**：只有 `views/settings/SettingsView.js` 通过 `panes/index.js` 扫到它。这一格 `hasSave:false`（原型 :1551：更新与关于两格藏起「保存」），`badge:true` 让导航那条右边挂上红点。
