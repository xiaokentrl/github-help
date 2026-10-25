# Icon（`components/icon/`）

- **这个件管什么**：界面里的小符号，一律内联 SVG，尺寸只有 14 / 16 / 18 三档，颜色跟所在行的字色走（`currentColor`）。
- **有哪些状态靠谁给**：图标自己不带状态；纯装饰时 `aria-hidden`，自己承担意思时由调用方给 `label`。要加一个图标只在 `Icon.js` 的 `PATHS` 里加一条。
- **谁在用**：Button、TextField、EmptyState、行级件与各个视图。原型用的是字符（⟳ ↺ ↓ × ✓ ▾），已定全换内联 SVG——不同字体里字符宽度不一，同一行会在 Windows 与 Linux 之间左右漂。
