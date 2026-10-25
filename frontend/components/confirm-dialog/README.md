# ConfirmDialog（`components/confirm-dialog/`）

- **这个件管什么**：危险动作前那一句问话。层级 `--z-dialog`(60)、遮罩、`role="dialog"`＋`aria-modal` 都照原型 :338-348。状态：开（淡入 + 缩放入场）／Esc 关／点遮罩＝取消／Tab 与 Shift+Tab 关在框内／**关掉后焦点回到叫它出来的那颗钮**。
- **有哪些状态靠谁给**：文案三件必须给齐，件里不写一个字中文——`title`＝要做什么、`lines`＝影响哪几条（一行一条）、`warn`＝能不能撤（单独一行红字，就是 `.warn-line`）；两颗钮的字走 `confirmText`／`cancelText`；`danger:true` 时确认钮走红底那一档。返回值还是一个 Promise：`true`＝确认、`false`＝取消或 Esc。
- **谁在用**：原型 `confirmDialog(` 有 7 个调用点（还原、清空队列、覆盖写入那几类）。**这三件文案缺一条就不要调用它**——只写"确认操作"的框，用户点完才知道后果，问一句等于没问（第 4 节 4.13 那条就是这么写的）。
