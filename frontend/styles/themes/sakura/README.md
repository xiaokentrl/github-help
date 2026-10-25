# 樱花粉（`styles/themes/sakura/`）

- **这套主题管什么**：只改一组颜色变量的值——背景、面板、边框、正文、主色、强调色与影子，外加这套的 `color-scheme` 与四支契约变量（`--on-primary`/`--on-green`/`--focus-ring`/`--focus-glow`）。
- **值从哪来**：本文件夹 `theme.css` 里那批变量 逐字取自原型 :504 与 :524 两段；`theme.js` 里的名称、简介、预览四色取自 :3261-3266 的 THEMES 那行。
- **谁能用它**：`themes/index.js` 扫描登记，托盘胶囊那六个圆点和主题弹窗那六张卡读扫描结果；组件层和视图层不许 import 本文件夹里的 `theme.css`，它由入口按顺序加载（第 7 节那条顺序）。
