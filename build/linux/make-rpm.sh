#!/usr/bin/env bash
# 出包（rpm 线）。二进制由 stage-linux.sh 编好，rpm 只装文件——所以 rpmbuild 环境里连 webkit 都不用装。
# rpmbuild 本机有就用本机的；没有就走 rockylinux:9 容器（一次性的，不动本机环境，装完即弃）。
# 产物：build/out/github-accel_<版本>.x86_64.rpm（RPMVER 里 0.0.0-m8 → 0.0.0~m8：
#   rpm 的 Version 不许带连字符；~ 是 rpm 官方的"预发布"记号，排序在无波浪号的正式号之前）。
#
# 依赖：%files 里的 ELF 让 rpmbuild 的 find-requires 现场算出 soname 依赖（同 deb 的 dpkg-shlibdeps
#   现场算那条口径），不手抄。spec 里没有任何版本号第二份——变量是脚本灌进去的。

set -euo pipefail

VERSION="${1:-${VERSION:-0.0.0-m5}}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
OUT="$ROOT/build/out"
STAGE="$OUT/root"

# shellcheck source=../common/stage-linux.sh
source "$ROOT/build/common/stage-linux.sh"

say "rpm ① 摆 rpmbuild 树并灌 spec"
RPMVER="${VERSION//-/\~}"
RPMDIR="$OUT/rpmbuild"
rm -rf "$RPMDIR"
mkdir -p "$RPMDIR/SPECS" "$RPMDIR/RPMS" "$RPMDIR/SRPMS" "$RPMDIR/BUILD" "$RPMDIR/BUILDROOT"
SPEC="$RPMDIR/SPECS/$PKG.spec"
cat >"$SPEC" <<EOF
Name:           $PKG
Version:        $RPMVER
Release:        1
Summary:        GitHub 加速助手（改 hosts 指向更可用的 IP）
License:        ISC
%define debug_package %{nil}

%description
GitHub 加速助手：界面壳 $PKG-gui 不带权限，常驻后台 $PKG-service 是唯一被允许写 hosts 的那一个，
两者之间走本机 AF_UNIX 端点。构建线 -tags gtk3（GTK3 + webkit2gtk-4.1）。不签名。
本包只摆文件：不带 systemd 单元与装卸脚本，装它不会启动常驻进程、不会改 hosts。

%install
mkdir -p %{buildroot}/opt/$PKG %{buildroot}/usr/bin %{buildroot}/usr/share/applications %{buildroot}/lib/systemd/system
cp -a %{gac_stage}/opt/$PKG/.            %{buildroot}/opt/$PKG/
cp -a %{gac_stage}/usr/bin/.              %{buildroot}/usr/bin/
cp -a %{gac_stage}/usr/share/applications/. %{buildroot}/usr/share/applications/
cp -a %{gac_stage}/usr/share/icons/.      %{buildroot}/usr/share/icons/
cp -a %{gac_stage}/lib/systemd/.          %{buildroot}/lib/systemd/

%files
/opt/$PKG
/usr/bin/$PKG-gui
/usr/bin/$PKG-service
/usr/share/applications/$PKG.desktop
/usr/share/icons/hicolor/256x256/apps/$PKG.png
/lib/systemd/system/$PKG.service

# 与 deb 同一套口径:装+起、停、摘+清(装完只提权 rpm 这一次;卸载不残留 /run/github-accel)。
# 不用 %systemd_post 那组宏——构建容器没装 systemd-rpm-macros 时宏原样落进包里,
# 装机 scriptlet 当场挂(容器真装实测抓到的);直写 systemctl 命令,谁都展开得了。
%post
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl enable github-accel.service >/dev/null || true
  systemctl start github-accel.service || true
fi

%preun
if command -v systemctl >/dev/null 2>&1; then
  systemctl stop github-accel.service >/dev/null 2>&1 || true
fi

%postun
if command -v systemctl >/dev/null 2>&1; then
  systemctl disable github-accel.service >/dev/null 2>&1 || true
  systemctl daemon-reload || true
  systemctl reset-failed >/dev/null 2>&1 || true
fi
if [ "\$1" = 0 ]; then rm -rf /run/github-accel; fi
EOF

# stage 树挂进 rpmbuild：本机跑给真路径，容器跑给 /stage。rpm 会自己对 buildroot 里的
# ELF 跑 find-requires，所以容器里只装 rpm-build，别的什么都不用装。
if command -v rpmbuild >/dev/null 2>&1; then
  say "rpm ② 本机 rpmbuild 打包"
  rpmbuild -ba --define "_topdir $RPMDIR" --define "gac_stage $STAGE" "$SPEC"
else
  say "rpm ② 走 rockylinux:9 容器打包（本机没装 rpmbuild，容器用完即弃）"
  # 四个代理变量清空是必须的：本机 /etc/containers/containers.conf.d/proxy.conf 给所有容器
  # 塞了 HTTP_PROXY=127.0.0.1:10808，而那个代理只听主机的回环——容器里的 127.0.0.1 是它自己，
  # 不清空则 dnf 永远连不出去（实测：不清＝Curl error (7)，清空＝rpm-build 正常装上）。
  docker run --rm -v "$RPMDIR":/build:rw -v "$STAGE":/stage:ro \
    -e HTTP_PROXY= -e HTTPS_PROXY= -e http_proxy= -e https_proxy= \
    rockylinux:9 \
    bash -c 'dnf -qy install rpm-build >/dev/null && rpmbuild -ba --define "_topdir /build" --define "gac_stage /stage" /build/SPECS/'"$PKG"'.spec'
fi

say "rpm ③ 产物"
find "$RPMDIR/RPMS" -name '*.rpm' -print -exec ls -la {} \;
# 拷到 build/out 顶层，与 deb／AppImage／zip 同层——Release 工作流与 checksums.txt
# 的收集口径都是"build/out 顶层的四类后缀"，rpm 留在 rpmbuild 子树里会被静默漏发（v0.0.0-m13 实测）。
cp -a "$RPMDIR"/RPMS/*/*.rpm "$OUT/"
echo
echo "版本 $VERSION（rpm 里记作 $RPMVER）／构建时间 $BUILD_DATE 已注进 service"
