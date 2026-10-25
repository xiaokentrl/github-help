#!/usr/bin/env bash
# 共享的 Linux 摆件步骤：deb／rpm／AppImage 三条打包线吃的是同一棵 staged 树。
# 用法：调用方先设 VERSION、BUILD_DATE、ROOT、OUT、STAGE（通常 $OUT/root），再 source 本文件。
# 干三件事：
#   ① 构建前端（frontend/ 是唯一界面真源，产物只有 dist/ 一份）；
#   ② 编 gui／service 两个可执行文件（-tags gtk3 + ldd 当场核构建线，A2乙）；
#   ③ 摆 /opt 布局、/usr/bin 软链、桌面入口。
# 各格式私有的东西（DEBIAN control、rpm spec、AppRun 等）不在这儿，各自脚本自己加。
# 版本号：调用方给；直接跑本文件不构成完整出包，号默认 0.0.0-m5，自动取号的唯一入口是 make。

set -euo pipefail

say() { printf '\033[1m== %s\033[0m\n' "$*"; }
die() { printf '出包停在这一步：%s\n' "$*" >&2; exit 1; }

PKG=github-accel
TAGS="${GAC_TAGS:-gtk3}"

say "stage 1/3 构建界面（正式界面只有 frontend/ 那一棵树，产物只有 dist/ 那一份）"
cd "$ROOT/frontend"
npm run build >/dev/null
[ -f dist/index.html ] || die "frontend/dist/index.html 没出来，npm run build 那一步就红了"

say "stage 2/3 编两个可执行文件（-tags $TAGS，A2乙 那条唯一的构建线）"
cd "$ROOT"
rm -rf "$STAGE"
BIN_GUI="$STAGE/opt/$PKG/$PKG-gui"
BIN_SVC="$STAGE/opt/$PKG/$PKG-service"
mkdir -p "$(dirname "$BIN_GUI")" "$STAGE/opt/$PKG/web/dist"
# service 里有 version/buildDate 这两个变量；gui 没有，所以 gui 不带 -X。
go build -tags "$TAGS" -o "$BIN_GUI" ./cmd/gui
go build -tags "$TAGS" \
  -ldflags "-X main.version=$VERSION -X main.buildDate=$BUILD_DATE" \
  -o "$BIN_SVC" ./cmd/service
cp -a "$ROOT/frontend/dist/." "$STAGE/opt/$PKG/web/dist/"
chmod 0755 "$BIN_GUI" "$BIN_SVC"

say "stage 3/3 当场核构建线：必须链 webkit2gtk-4.1，不许出现 webkitgtk-6.0"
LINKED=$(ldd "$BIN_GUI")
echo "$LINKED" | grep -q 'webkit2gtk-4\.1' || die "$BIN_GUI 没链 libwebkit2gtk-4.1——这条构建线不是 A2乙 那条"
echo "$LINKED" | grep -q 'webkitgtk-6\.0' && die "$BIN_GUI 链到了 webkitgtk-6.0（＝漏了 -tags $TAGS），这台机器上能跑、换机器就炸" || true
echo "$LINKED" | grep -Eo 'libgtk-3\.so[^ ]*|libwebkit2gtk[^ ]*' | sort -u

say "stage 摆 /usr/bin 软链与桌面入口（三种格式共用）"
mkdir -p "$STAGE/usr/bin" "$STAGE/usr/share/applications"
ln -sf "/opt/$PKG/$PKG-gui" "$STAGE/usr/bin/$PKG-gui"
ln -sf "/opt/$PKG/$PKG-service" "$STAGE/usr/bin/$PKG-service"
cat >"$STAGE/usr/share/applications/$PKG.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=GitHub 加速助手
Comment=把 GitHub 相关域名指到更可用的 IP（改 hosts 的动作全在后台服务那一侧）
Exec=$PKG-gui
Terminal=false
Categories=Utility;Network;
EOF
chmod 0644 "$STAGE/usr/share/applications/$PKG.desktop"
