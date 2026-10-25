## English

### github-accel 0.0.0-m13 What's Changed: first release

**Version type: Milestone**

**Added**

- Initial release: GitHub acceleration tool that repoints GitHub-related domains to more usable IPs via hosts (Go + Wails v3 backend, vanilla JS + Vite frontend; the backend service is the only component allowed to write hosts)
- Packaging in four formats: deb, rpm, AppImage and a portable Windows zip; CI builds and tests on every push, and the release workflow packages all four on a `v*` tag
- Auto-incrementing versioning from the VERSION file; bilingual (EN/中文) release notes drafted from git history

## 中文

### 0.0.0-m13 首个发布

**版本类型：里程碑版**

**新增**

- 首个发布：GitHub 加速助手——把 GitHub 相关域名经 hosts 指到更可用的 IP（Go + Wails v3 后端、原生 JS + Vite 前端；后台服务是唯一被允许写 hosts 的组件）
- 四种格式的出包线：deb、rpm、AppImage、Windows 绿色 zip；每次推送 CI 自动构建并测试，推 `v*` 标签自动出四件包并发 Release
- 版本号从 VERSION 文件自动取号推进；发版说明由 git 提交起草、中英双语定稿
