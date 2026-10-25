# 自定义 / 订阅源（`custom`）

这块菜单管 3 条二级条目，排第 11；条目的名字与域名见 `menu.json`。
数据来源：`frontend/menu/custom/module.js`（唯一真源），本目录是它的生成物——手改这里会被下一次生成盖掉。
谁能用：`internal/menu` 读取包（服务启动时校验），编排层 `internal/service` 只读它，界面不直接读。
本块有 2 条不是合法域名形状（自定义域名、github.com/zhouzhehong/china-hosts-accelerator），写 hosts 那一刻会被拒绝并如实标出来〔已定：D4 甲，数据不擅自改〕。
父域归属（本块 1 个）见 `common/common.json`，口径见 `common/README.md`。
