# AI 开发工具（`ai`）

这块菜单管 6 条二级条目，排第 3；条目的名字与域名见 `menu.json`。
数据来源：`frontend/menu/ai/module.js`（唯一真源），本目录是它的生成物——手改这里会被下一次生成盖掉。
谁能用：`internal/menu` 读取包（服务启动时校验），编排层 `internal/service` 只读它，界面不直接读。
父域归属（本块 10 个）见 `common/common.json`，口径见 `common/README.md`。
