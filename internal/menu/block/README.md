# 去广告与屏蔽（`block`）

这块菜单管 3 条二级条目，排第 10；条目的名字与域名见 `menu.json`。
数据来源：`frontend/menu/block/module.js`（唯一真源），本目录是它的生成物——手改这里会被下一次生成盖掉。
谁能用：`internal/menu` 读取包（服务启动时校验），编排层 `internal/service` 只读它，界面不直接读。
本块有 1 条不是合法域名形状（raw.githubusercontent.com/StevenBlack/hosts），写 hosts 那一刻会被拒绝并如实标出来〔已定：D4 甲，数据不擅自改〕。
父域归属（本块 2 个）见 `common/common.json`，口径见 `common/README.md`。
