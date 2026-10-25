#!/usr/bin/env bash
# 出包（Windows 线）：交叉编两个 exe + 界面产物 + 说明，打成一个 zip。
# Windows 一路用 WebView2（系统自带），不碰 gtk——所以不带 -tags gtk3（那是 Linux cgo 的线），
# 交叉编译时 cgo 自动关闭，GOOS=windows go build 的可行性与 CI/检查号同一条已验过。
# 产物：build/out/github-accel_<版本>_windows_amd64.zip
#
# 交底（如实）：打的是"绿色 zip"，不是 NSIS 安装器——本机没装 makensis，装它要动本机环境；
#   想要双击安装式再说一声。WebView2 运行时 Win10/11 一般自带，没有时壳起不来（那是系统组件）。
#   service 在 Windows 上写 hosts（C:\Windows\System32\drivers\etc\hosts）需要管理员权限，
#   这份 zip 不替用户提权——以管理员运行 service 是用户自己的动作。

set -euo pipefail

VERSION="${1:-${VERSION:-0.0.0-m5}}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

say() { printf '\033[1m== %s\033[0m\n' "$*"; }
die() { printf '出包停在这一步：%s\n' "$*" >&2; exit 1; }

PKG=github-accel
HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
OUT="$ROOT/build/out"
WINDIR="$OUT/win/$PKG"

say "win 1/3 构建界面（同一份 frontend/，Windows 上加载的就是它）"
cd "$ROOT/frontend"
npm run build >/dev/null
[ -f dist/index.html ] || die "frontend/dist/index.html 没出来，npm run build 那一步就红了"

say "win 2/3 交叉编两个 exe（无 gtk tag；service 注入版本与构建时间）"
cd "$ROOT"
rm -rf "$OUT/win"
mkdir -p "$WINDIR/web"
GOOS=windows GOARCH=amd64 go build -o "$WINDIR/$PKG-gui.exe" ./cmd/gui
GOOS=windows GOARCH=amd64 go build \
  -ldflags "-X main.version=$VERSION -X main.buildDate=$BUILD_DATE" \
  -o "$WINDIR/$PKG-service.exe" ./cmd/service
# 壳找界面的第二条：可执行文件同级的 web/dist。zip 解开就是发布态目录。
cp -a "$ROOT/frontend/dist" "$WINDIR/web/dist"
file "$WINDIR/$PKG-gui.exe" | grep -q 'PE32+.*x86-64' \
  || die "编出来的不是 Windows x64 的 PE——交叉编译线不对"

cat >"$WINDIR/说明.txt" <<EOF
GitHub 加速助手 $VERSION（Windows 绿色包）

里面两个程序：
  github-accel-service.exe  后台服务——唯一被允许写 hosts 的那一个。改 hosts 需要管理员，
                            所以要用"以管理员身份运行"启动它。
  github-accel-gui.exe      界面——普通权限双击即可，通过本机管道跟服务说话。

hosts 文件位置：C:\\Windows\\System32\\drivers\\etc\\hosts
界面文件在 web\\dist\\ 里，两个 exe 要跟 web 文件夹待在一起，别拆散。
EOF

say "win 3/3 打 zip 包"
ZIP="$OUT/${PKG}_${VERSION}_windows_amd64.zip"
rm -f "$ZIP"
( cd "$OUT/win" && zip -qr "$ZIP" "$PKG" )
unzip -l "$ZIP" | sed -n '1,12p'
echo
echo "产物：$ZIP"
echo "版本 $VERSION／构建时间 $BUILD_DATE 已注进 service；NSIS 安装器没做（见脚本头那条交底）"
