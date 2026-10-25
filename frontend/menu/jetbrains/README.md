# JetBrains（`menu/jetbrains/`）

- **这块菜单管什么**：6 条二级条目——插件市场、IDE 下载、账号验证、Toolbox、文档与资源、更新检查。
- **条目从哪来**：写在本文件夹 `module.js` 的 `items` 里，逐字搬自原型 :1071 那一段；加一条只改这里，别的文件夹一个字不动。
- **谁能用它**：只有 `views/menu/` 那一份渲染器读它（那是 F5，还没建）。本文件夹不许出现 `document.`、`innerHTML`，也不许 import `views/` 里的东西——第 4 节那两条 grep 管着。
