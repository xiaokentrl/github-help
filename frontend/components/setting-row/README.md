# SettingRow（`components/setting-row/`）

- **这个件管什么**：设置面板里"一句标题 + 一句说明 + 右边那颗控件"这一行。是 17 个目录里唯一的**分子件**（第 5 节：它由 Switch／Checkbox／Select／TextField 加文字拼成），状态：默认／整行禁用（标题与说明一起灰 `.55`，跟原型 :374 那个值一致）／点标题即聚焦控件。
- **有哪些状态靠谁给**：`title`／`desc` 两句由调用方给（件里不写中文）；`control` 传 `Switch()`／`Checkbox()`／`Select()`／`TextField()` 造出来的对象，件去找里面那个真能聚焦的 `input`／`select` 并挂上 `<label for>`（没有 id 就地补一个）；挂不上就退化成"点整行转交焦点"，不硬造一个不做事的假关联；控件的外部 id 已由别人管时传 `for` 指定。禁用走 `setDisabled(true)`，一次管三处：行灰、说明灰、控件 `disabled`。**"为什么灰"那句走 `disabledReason`**：件把它同时挂到行自己的 `title` 和里面那颗控件的 `title` 上——只挂行不挂控件的话，鼠标停到开关上什么也读不到（人性化清单第 4 条；状态矩阵 `setting-row/disabled` 那行量的就是这两个 `title` 是否都在）。
- **谁在用**：七个设置面板共 18 处这种行（使用点，数法见组件库规划.md 第 4 节那条）。**其他五个分子件不在这儿**——TreeRow／CategoryHeader／MetricRow／QueueRow／ThemeCard 认识业务，归 `views/*/parts/`。
