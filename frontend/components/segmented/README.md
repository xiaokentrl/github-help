# Segmented（`components/segmented/`）

- **这个件管什么**：二选一／三选一的那一排段；状态有选中、未选、悬停、焦点、禁用。选中态的字色走 `--on-primary`（不写死白）。
- **有哪些状态靠谁给**：只暴露 `value` + `onChange`，同步交给状态层——件自己不许去 toggle 别人的 class（原型靠 `syncLayoutSegButtons()` 手工同步，"看着选了其实没存"就是这么来的）。禁用走 `disabled` 或 `setDisabled(true)`，一传就是**整排每一段各自都灰**（判据读的是"所有控件都 off"，不是"外层容器标了 disabled"——所以别只灰外层）；灰了要说清在等什么，那句由调用方传 `title`（人性化清单第 4 条）。无障碍名由调用方给 `ariaLabel`。
- **谁在用**：设置中心的布局面板（宽度模式、侧栏对齐…）。键盘：左右方向键换段，Home/End 到两端。
