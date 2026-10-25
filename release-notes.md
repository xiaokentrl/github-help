## English

### github-accel 0.0.0-m25 What's Changed from 0.0.0-m14 to 0.0.0-m25

**Version type: Milestone**

**Added**

- systemd auto-start: the deb/rpm packages now ship a `github-accel.service` (root daemon, endpoint at `/run/github-accel`); install scripts enable and start it, uninstall stops it and cleans `/run` residue while keeping your config
- Real update channel: "Check update" queries this repo's GitHub latest release and shows the changelog; "Download update" fetches the matching package (deb for Linux, zip for Windows) into the config's `updates/` folder — installation itself stays in your hands
- Offline detection: the service probes public DNS every 5 seconds; when the network drops, pending detection is held and the status bar shows "network offline", when it returns detection resumes automatically
- "Open Hosts" button on the toolbar: releases edit permission on the hosts file (0777, reverted at the next apply) and opens it in your editor
- Real links in "About": project page / feedback / license buttons open in your browser (new OpenURL contract method)
- Application icon in menus, and a LICENSE file (ISC)

**Changed**

- Permission model: the background service runs as root by design (developer tool); the GUI stays unprivileged and connects via a local socket
- Config folder renamed to `~/.config/github-hlep` (was `github-accel`)
- First-run defaults: auto-detect and auto-apply on startup now default to OFF; existing configs keep their values

## 中文

### 0.0.0-m25 相比 0.0.0-m14 更新内容

**版本类型：里程碑版**

**新增**

- 开机常驻：deb/rpm 包自带 `github-accel.service`（root 后台服务，端点在 /run/github-accel）；装完自动启用并启动，卸载停服务、清 /run 残留，配置保留
- 更新通道接真：「检测升级」真问本仓库 GitHub 最新版并显示更新内容；「下载更新」按平台把安装包拉到配置目录 updates/ 下——装不装、什么时候装归你
- 断网判定：服务每 5 秒探测一次网络；断网时检测任务自动扣住、状态栏显示"网络已断开"，恢复联网后自动继续
- 工具栏新增「打开Hosts」：放开 hosts 编辑权限（0777，下一次「应用」自动收回）并用系统编辑器打开
- 「关于我们」三颗入口钮接真：项目主页／反馈／许可在浏览器打开（新契约方法 OpenURL）
- 应用图标进菜单；补 LICENSE 文件（ISC）

**修改**

- 权限模型：后台服务按设计以 root 常驻（开发者工具）；界面保持普通权限，走本机 socket 连接
- 配置目录改名：`~/.config/github-hlep`（原 github-accel）
- 首次安装默认值：「自动检测已选」「自动应用队列」默认改为关；已装用户的配置不受影响
