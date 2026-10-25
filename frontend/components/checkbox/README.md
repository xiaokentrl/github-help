# Checkbox（`components/checkbox/`）

- **这个件管什么**：一条域名／一块类目"要不要纳入本次操作"，含半选（`indeterminate`）。状态：未选、选中、半选、悬停、焦点可见、禁用。
- **有哪些状态靠谁给**：半选点下去之后往哪走（全选还是清空）由上层定，件只把 `checked`／`unchecked` 报出去；无障碍名由调用方给 `ariaLabel`。禁用走 `disabled` 或 `setDisabled(true)`（真 input 与外面那颗方块一起灰），灰了要说清在等什么——那句由调用方传 `title`，件挂到包装层上（人性化清单第 4 条）。
- **谁在用**：`views/menu/parts/TreeRow.js` 与 `CategoryHeader.js`、队列表的行。原型那两处是 `<span class="cb">`（:1799、:1816），没有 tabindex 也没有 role，键盘选不中任何东西——这一件把真 input 叠回同一个位置。
