#!/usr/bin/env bash
# 出包（deb 线）：Linux 的 deb／rpm／AppImage 与 Windows 包都从 build/ 出〔原 A9乙"只出 deb"
#   已由用户 2026-10-25 改口径：四种包都要〕，共享的"编＋摆"步骤在 build/common/stage-linux.sh，
#   本脚本只做 deb 私有的三件事：依赖现场算、DEBIAN 元数据、dpkg-deb 打包。
# 怎么跑（仓库根）：make package / make deb（自动取号）或 make deb VERSION=x；
#   直接跑 bash build/linux/make-deb.sh <版本> 也行，但号不会自动推进——自动取号的唯一入口是 make。
# 产物：build/out/github-accel_<版本>_amd64.deb
#
# 会错在哪、出错时人看得见什么（沿袭原先 deb 脚本那四条，构建线与摆放的判据在 stage-linux.sh）：
#   ① 依赖手抄：Depends 由 dpkg-shlibdeps 现场算，抄一份迟早跟 ldd 不一致。
#   ② 装/卸脚本：本包**不带** postinst／prerm／postrm，也**不带** systemd 单元——服务装卸那一半
#      还没定（服务用谁的权限跑等你拍板）。所以 dpkg 装它只是摆文件，不会动 /etc/hosts、
#      不会启任何常驻进程。
#   ③ 版本没注进去：service 里那句 version 默认是 0.0.0-m1，「关于」那一格会一直显示 M1 的号；
#      stage-linux.sh 用 -ldflags -X 把真号打进去，buildDate 不许在代码里编死，只在这一步注入。
#
# 怎么回来：改版本号就重跑一次；产物不对先 `dpkg-deb --contents` 看摆放，再看 stage 那三步。

set -euo pipefail

VERSION="${1:-${VERSION:-0.0.0-m5}}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
MAINTAINER="${MAINTAINER:-hlep <hlep@localhost>}"

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
OUT="$ROOT/build/out"
STAGE="$OUT/root"

# shellcheck source=../common/stage-linux.sh
source "$ROOT/build/common/stage-linux.sh"

say "deb ① 依赖现场算（dpkg-shlibdeps，不手抄）"
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

say "deb ② 摆 control／装卸脚本／md5sums"
ARCH=$(dpkg --print-architecture)
mkdir -p "$STAGE/DEBIAN"
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
  两个可执行文件：界面壳 $PKG-gui 不带权限，常驻后台 $PKG-service 以 root 跑、是唯一被允许写 hosts 的那一个，
  两者之间走本机 AF_UNIX 端点〔权限模型已定 2026-10-25：服务 root 常驻在 /run/github-accel，对端只核"本机来的"〕。
  构建线：-tags gtk3（GTK3 + webkit2gtk-4.1）。不签名〔已定：A7乙〕。
  postinst 启用并启动 github-accel.service（开机常驻，装完只提权 dpkg 这一次）；
  卸载停服务并清 /run/github-accel 残留；配置（/root/.config/github-hlep）卸载时保留。
EOF
# 装卸脚本〔2026-10-25 起带上——权限模型已定,服务装卸不再是"等规格书"的挂账〕。
# 装完只提权一次＝dpkg -i 那一次 sudo,postinst 以 root 跑 enable --now,不再要任何口令;
# 卸载不残留＝prerm 停服务,postrm 摘自启、清 /run/github-accel 那个 socket 目录。
cat >"$STAGE/DEBIAN/postinst" <<'EOS'
#!/bin/sh
set -e
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload
  systemctl enable github-accel.service >/dev/null || true
  # restart 而不是 start:升级路径上旧进程要换血,不 restart 则升级完还跑着旧二进制
  systemctl restart github-accel.service || true
fi
EOS
cat >"$STAGE/DEBIAN/prerm" <<'EOS'
#!/bin/sh
set -e
if command -v systemctl >/dev/null 2>&1; then
  systemctl stop github-accel.service >/dev/null 2>&1 || true
fi
EOS
cat >"$STAGE/DEBIAN/postrm" <<'EOS'
#!/bin/sh
set -e
if [ "$1" = remove ] || [ "$1" = purge ]; then
  if command -v systemctl >/dev/null 2>&1; then
    systemctl disable github-accel.service >/dev/null 2>&1 || true
    systemctl daemon-reload || true
    systemctl reset-failed >/dev/null 2>&1 || true
  fi
  rm -rf /run/github-accel
fi
EOS
chmod 0755 "$STAGE/DEBIAN/postinst" "$STAGE/DEBIAN/prerm" "$STAGE/DEBIAN/postrm"
( cd "$STAGE" && find . -type f ! -path './DEBIAN/*' -exec md5sum {} + | sed 's|\./||' ) >"$STAGE/DEBIAN/md5sums"
chmod 0644 "$STAGE/DEBIAN/control" "$STAGE/DEBIAN/md5sums" "$STAGE/usr/share/applications/$PKG.desktop"
chmod 0755 "$STAGE/DEBIAN"

say "deb ③ 打包"
DEB="$OUT/${PKG}_${VERSION}_${ARCH}.deb"
rm -f "$DEB"
dpkg-deb --build --root-owner-group "$STAGE" "$DEB"
ls -la "$DEB"
dpkg-deb --info "$DEB" | sed -n '1,30p'
echo
echo "产物：$DEB"
echo "版本 $VERSION／构建时间 $BUILD_DATE 已注进 service（界面「关于」那一格读的就是它）"
echo
cat <<'EOF'

交底：本包带 systemd 常驻（github-accel.service，root，端点 /run/github-accel）。
      postinst 装+起，prerm 停，postrm 摘自启并清 /run 残留；root 的配置目录卸载时保留。
      deb／rpm／AppImage／Windows 四条线的取号入口都在 make。
EOF
