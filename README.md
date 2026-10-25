# GitHub 加速助手 / GitHub Accelerator

把 GitHub 相关域名经 hosts 指到更可用的 IP——检测、应用、恢复，一键完成。
Repoint GitHub-related domains to faster IPs via the hosts file — detect, apply, restore, one click.

[![CI](https://github.com/xiaokentrl/github-help/actions/workflows/ci.yml/badge.svg)](https://github.com/xiaokentrl/github-help/actions/workflows/ci.yml)
[![Release](https://github.com/xiaokentrl/github-help/actions/workflows/release.yml/badge.svg)](https://github.com/xiaokentrl/github-help/actions/workflows/release.yml)
[![License: ISC](https://img.shields.io/badge/License-ISC-blue.svg)](LICENSE)

---

## 它是什么 / What it does

**中文**：一个给开发者的桌面工具。它用多种方式（GitHub520、DoH、系统解析等）为 GitHub 相关域名找出当前可用的 IP，择优写入系统 hosts 文件，让 `github.com`、`api.github.com`、`gist.github.com` 等恢复访问。界面树里还备好了 JetBrains、AI 开发工具、Steam、公共 CDN、微软/Xbox、Coursera 等 11 类共 54 条域名，勾选即可批量处理。

**English**: A desktop tool for developers. It resolves GitHub-related domains through multiple sources (GitHub520, DoH, system resolution, …), picks the best IP, and writes it into the system hosts file so `github.com`, `api.github.com`, `gist.github.com` etc. become reachable again. The sidebar also ships 11 categories / 54 domains — JetBrains, AI tools, Steam, public CDNs, Microsoft/Xbox, Coursera — ready for batch processing.

## 下载安装 / Install

从 [Releases](https://github.com/xiaokentrl/github-help/releases) 下载对应平台：

| 平台 | 包 | 说明 |
| --- | --- | --- |
| Linux (Debian/Ubuntu) | `*.deb` | `sudo dpkg -i *.deb`，装完自动注册并启动系统服务 |
| Linux (RPM 系) | `*.rpm` | `sudo rpm -Uvh *.rpm`，同上 |
| Linux (免安装) | `*.AppImage` | 双击即用（宿主需已装 webkit2gtk-4.1） |
| Windows | `*_windows_amd64.zip` | 解压后运行 `github-accel-gui.exe` |

装完在应用菜单里点「GitHub 加速助手」。改 hosts 的动作由一个以 root 运行的后台服务（`github-accel.service`）完成，界面本身不带任何权限；两者通过本机 Unix socket 通信。

## 使用 / Usage

1. 左侧类目树勾选要加速的域名（首次有引导条）；
2. 点「检测全部」——每一项测出当前最快可用的 IP；
3. 点「应用队列」——写入 hosts（每行带 `gac:` 标记，可随时一键还原）；
4. 「恢复默认 hosts」随时撤销全部改动；「同步状态」重新读取 hosts 实况。

其它：工具栏「打开Hosts」可放开 hosts 编辑权限（0777，下一次「应用」自动收回）用顺手的编辑器手改；设置中心里可换主题（六套）、语言、开机行为；「检测升级」走本仓库的 GitHub Release 检查更新。

## 从源码构建 / Build from source

依赖：Go 1.27、Node ≥ 20、GTK3 与 webkit2gtk-4.1 开发头（`libgtk-3-dev libwebkit2gtk-4.1-dev`）。

```bash
make frontend          # 构建前端
make package           # 一条龙出 deb + rpm + AppImage + Windows zip（产物在 build/out/）
make check             # CI 同款本地检查：go test/vet + gofmt + Windows 交叉编译
```

测试：前端 `node --test frontend/tests/*.test.mjs`（191 条），Go `go test -tags gtk3 ./...`（13 包）。

## 安全模型 / Security notes

- 后台服务以 **root** 常驻（写 `/etc/hosts` 本来就需要）；界面以普通权限运行，二者经本机 socket 通信，socket 权限 0666（本机任何用户可连）——这是开发者工具的既定取舍；
- 写入 hosts 的每一行都带 `gac:` 标记并记录应用前的地址，恢复靠标记逐行撤销，你自己的行永远原样保留；
- hosts 备份保留最近 10 份，写入失败自动回滚；
- 「打开Hosts」临时放开 0777 编辑权限，下一次「应用」自动收回；
- 更新检查只读本仓库的 GitHub Release；更新包下载后**不会自动安装**。

## 目录速览 / Layout

```
cmd/gui        界面壳（Wails v3 + webkit2gtk）
cmd/service    后台服务（唯一能写 hosts 的进程）
internal/      解析、探测、hosts 写入、IPC、队列、设置……
frontend/      原生 JS + Vite 的界面（无框架）
build/         四种格式的打包脚本（deb/rpm/AppImage/Windows）
scripts/       发版说明起草
tools/         文档锚点校验
```

规划与设计文档见仓库根目录的六份中文 `.md`（技术方案、前后端目录规划、组件库规划、需求盘点）。

## 许可 / License

[ISC](LICENSE)。本工具修改系统 hosts 文件，使用后果由使用者自担；不对 GitHub 的可用性作任何保证。
/ This tool modifies your system hosts file — use at your own risk. No guarantee of GitHub availability.
