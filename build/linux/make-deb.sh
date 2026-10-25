#!/usr/bin/env bash
# 出包：Linux 只出 deb〔已定：2026-10-21 你拍 A9乙；rpm／AppImage 不进这条脚本〕。
#
# 干什么用：把 frontend/ 那一份界面构建出来、把 cmd/gui 与 cmd/service 两个可执行文件
#   用 -tags gtk3 编出来（A2乙 那条构建线），按壳找文件的规矩摆进 deb，最后打成一个 .deb。
# 怎么跑（仓库根）：make package            → 版本号自动取（见下面 VERSION 那段）
#                   make package VERSION=x  → 用你指定的号
#                   bash build/linux/make-deb.sh 0.0.0-m5 → 产物 build/out/github-accel_<版本>_amd64.deb
#
# 会错在哪、出错时人看得见什么：
#   ① 漏了 -tags gtk3 的构建，在这台装了 webkitgtk-6.0 的机器上照样成功，换台机器才炸——
#      所以编完当场 ldd 核一遍：必须链 webkit2gtk-4.1，出现 webkitgtk-6.0 就直接停（不是警告）。
#   ② 界面文件摆错地方：壳按三条顺序找（-dist 指的、可执行文件同级的 web/dist、仓库里的 frontend/dist），
#      deb 里走的是第二条，所以二进制放 /opt/github-accel/、产物放 /opt/github-accel/web/dist；
#      摆歪了症状是窗口开起来一片空白，人看得见的是壳退出时那一句"找过这几处"。
#   ③ 版本没注进去：service 里那句 version 默认是 0.0.0-m1，界面「关于」那一格会一直显示 M1 的号，
#      所以这里用 -ldflags -X 把真号打进去；buildDate 不许在代码里编死，只在这一步注入。
#   ④ 依赖手抄：Depends 由 dpkg-shlibdeps 现场算，抄一份迟早跟 ldd 不一致。
#   ⑤ 装/卸脚本：本包**不带** postinst／prerm／postrm，也**不带** systemd 单元——服务装卸那一半还没定
#      （理由见文末那句交底）。所以 dpkg 装它只是摆文件，不会动 /etc/hosts、不会启任何常驻进程。
#
# 怎么回来：改版本号就重跑一次；产物不对先 `dpkg-deb --contents` 看摆放，再看上面①②哪一条。

set -euo pipefail

BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
PKG=github-accel
MAINTAINER="${MAINTAINER:-hlep <hlep@localhost>}"

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
OUT="$ROOT/build/out"
STAGE="$OUT/root"
# GAC_TAGS 只为"故意编一条错构建线、看第 3 步那把闸拦不拦得住"这一次负向测留的口（见文末⑤）。
# 默认值就是唯一那条构建线；这个口子不会放过坏包——第 3 步的 ldd 核对跟这里填什么无关。
TAGS="${GAC_TAGS:-gtk3}"

# 版本号只有一个家：仓库根的 VERSION 文件，存的是「下一个要打的号」。
# 取号顺序：命令行参数 > 环境变量 > VERSION 文件（文件没有才落到最后那个保底字面量）。
# 包**打成**之后（set -e，走到那里＝前六步全过）把文件往后推一格——不管这个包用的是自动号
# 还是指定号，推的都是"那个号+1"，这样文件永远指向一个没用过的号，自动号不会跟旧包撞车。
VERSION_FILE="$ROOT/VERSION"
next_version() {
	# 0.0.0-m5 → 0.0.0-m6；1.0.0 → 1.0.1。没有数字尾巴的号接 .1。
	if [[ "$1" =~ ^(.*[^0-9])([0-9]+)$ ]]; then
		printf '%s%d\n' "${BASH_REMATCH[1]}" "$(( ${BASH_REMATCH[2]} + 1 ))"
	else
		printf '%s.1\n' "$1"
	fi
}
VERSION="${1:-${VERSION:-$(cat "$VERSION_FILE" 2>/dev/null || echo 0.0.0-m5)}}"

say() { printf '\033[1m== %s\033[0m\n' "$*"; }
die() { printf '出包停在这一步：%s\n' "$*" >&2; exit 1; }

say "1/6 构建界面（正式界面只有 frontend/ 那一棵树，产物只有 dist/ 那一份）"
cd "$ROOT/frontend"
npm run build >/dev/null
[ -f dist/index.html ] || die "frontend/dist/index.html 没出来，npm run build 那一步就红了"

say "2/6 编两个可执行文件（-tags $TAGS，A2乙 那条唯一的构建线）"
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

say "3/6 当场核构建线：必须链 webkit2gtk-4.1，不许出现 webkitgtk-6.0"
LINKED=$(ldd "$BIN_GUI")
echo "$LINKED" | grep -q 'webkit2gtk-4\.1' || die "$BIN_GUI 没链 libwebkit2gtk-4.1——这条构建线不是 A2乙 那条"
echo "$LINKED" | grep -q 'webkitgtk-6\.0' && die "$BIN_GUI 链到了 webkitgtk-6.0（＝漏了 -tags $TAGS），这台机器上能跑、换机器就炸" || true
echo "$LINKED" | grep -Eo 'libgtk-3\.so[^ ]*|libwebkit2gtk[^ ]*' | sort -u

say "4/6 依赖现场算（dpkg-shlibdeps，不手抄）"
DEPS_FILE="$OUT/substvars"
: >"$DEPS_FILE"
# dpkg-shlibdeps 只肯在"有 debian/control 的那棵树"里跑（不然报「无法读取 debian/control」），
# 而它拿那份文件只为知道包名。产物目录 build/out 本来就是一次性的，就地摆一份最小的给它。
mkdir -p "$OUT/debian"
printf 'Source: %s\n\nPackage: %s\nArchitecture: any\n\n' "$PKG" "$PKG" >"$OUT/debian/control"
( cd "$OUT" && dpkg-shlibdeps -T"$DEPS_FILE" \
    "root/opt/$PKG/$PKG-gui" "root/opt/$PKG/$PKG-service" ) >"$OUT/deps.txt" 2>"$OUT/shlibdeps.err" \
  || die "dpkg-shlibdeps 失败了，看 $OUT/shlibdeps.err"
grep -q . "$OUT/shlibdeps.err" && sed -n '1,3p' "$OUT/shlibdeps.err" || true
DEPS=$(sed -n 's/^shlibs:Depends=//p' "$DEPS_FILE")
[ -n "$DEPS" ] || die "算出来的依赖是空的——那说明 ldd 没量到东西，别出这个包"
echo "Depends: $DEPS"

say "5/6 摆 control／桌面入口／命令链接／md5sums"
ARCH=$(dpkg --print-architecture)
mkdir -p "$STAGE/DEBIAN" "$STAGE/usr/bin" "$STAGE/usr/share/applications"
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
SIZE=$(du -sk "$STAGE" --exclude=DEBIAN | cut -f1)
cat >"$STAGE/DEBIAN/control" <<EOF
Package: $PKG
Version: $VERSION
Architecture: $ARCH
Maintainer: $MAINTAINER
Installed-Size: $SIZE
Depends: $DEPS
Section: utils
Priority: optional
Description: GitHub 加速助手（改 hosts 指向更可用的 IP）
  两个可执行文件：界面壳 $PKG-gui 不带权限，常驻后台 $PKG-service 是唯一被允许写 hosts 的那一个，
  两者之间走本机 AF_UNIX 端点（权限当场 0600，并用 SO_PEERCRED 核对端 UID）。
  构建线：-tags gtk3（GTK3 + webkit2gtk-4.1）。不签名〔已定：A7乙〕。
  本包只摆文件：不带 systemd 单元、不带 postinst／prerm／postrm，装它不会启动常驻进程、不会改 hosts；
  服务装卸那一半按规划还等规格书（技术方案第 13 节 M5 那一格）。
EOF
( cd "$STAGE" && find . -type f ! -path './DEBIAN/*' -exec md5sum {} + | sed 's|\./||' ) >"$STAGE/DEBIAN/md5sums"
chmod 0644 "$STAGE/DEBIAN/control" "$STAGE/DEBIAN/md5sums" "$STAGE/usr/share/applications/$PKG.desktop"
chmod 0755 "$STAGE/DEBIAN"

say "6/6 打包"
DEB="$OUT/${PKG}_${VERSION}_${ARCH}.deb"
rm -f "$DEB"
dpkg-deb --build --root-owner-group "$STAGE" "$DEB"
ls -la "$DEB"
dpkg-deb --info "$DEB" | sed -n '1,30p'
NEXT=$(next_version "$VERSION")
printf '%s\n' "$NEXT" >"$VERSION_FILE"
echo
echo "产物：$DEB"
echo "版本 $VERSION／构建时间 $BUILD_DATE 已注进 service（界面「关于」那一格读的就是它）"
echo "下一个版本号已写进 $VERSION_FILE：$NEXT（不指定版本时自动用它）"
echo
cat <<'EOF'

交底：本包不带服务装卸。原因写死在脚本开头⑤——代码里 ipc.CheckPeer 只放「同 UID 或 root 来连」，
      root 跑的服务会把普通权限的界面拒在门外，而「装/卸载服务进步规格书」那条随 D 一起挂起，
      所以单元文件要等你一句再写，不在出包这一步替你定权限模型。
EOF
