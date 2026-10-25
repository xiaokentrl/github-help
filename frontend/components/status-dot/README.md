# StatusDot（`components/status-dot/`）

- **这个件管什么**：一条项／一个任务"现在处于哪一步"的那颗点，外加紧跟其后的状态字。六档：`applied`／`stale`／`unset`／`error`／`pending`／`detected`。状态：呼吸（`pending` 自动）、其余静态；深浅六套下 `unset` 那颗靠 `--rail` 各自可见。
- **有哪些状态靠谁给**：状态名从 `core/meta.js` 的 `ItemMeta`/`TaskMeta` 取（那里已把 `status → dot` 对好），状态字由调用方传 `label`——件里不持有文案。**点 + 字两条通道必须一起给**：只丢一个彩点，色盲用户和黑白打印下就是全废；传了 `label` 时点是纯装饰（`aria-hidden`），没传时点自己带名——那一个名字由调用方传 `ariaLabel`，件据此补 `role="img"` + `aria-label`，绝不出现"只有颜色能看出来"（状态矩阵 `status-dot/noLabel` 那行查的就是这一对属性在不在）。
- **谁在用**：类目树的每一行（原型 :1803-1804，`.dot` + `.stext` 那一对）、详情卡头部的状态徽标、队列行的状态格（:2116，`size:'inline'`）。队列那些 `waiting/running/done` 的**字色**属于 `views/queue/parts/QueueRow.js`，不在这个件里。
