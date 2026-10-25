#!/usr/bin/env bash
# 出包（AppImage 线）。二进制由 stage-linux.sh 编好，本脚本做三件私有的事：
#   ① 拼 AppDir：可执行文件 + 界面产物（壳按"可执行文件同级的 web/dist"找，所以 dist 摆二进制旁边）；
#   ② 按 ldd 闭包把动态库捆绑进去（glibc 核心与加载器除外——捆绑 libc 是 AppImage 的经典大坑，不吃）；
#      连带 gio 模块、gdk-pixbuf loaders、gtk3 模块、glib schemas、webkit 辅助进程与资源目录；
#   ③ 下载官方 appimagetool（缓存进 build/out/tools/，那是 gitignore 的一次性目录）打成一个 .AppImage。
#
# 交底（如实）：webkit 的辅助进程（WebKitWebProcess 等）在 Debian/Ubuntu 上按编译期绝对路径找，
#   AppImage 没有官方的相对化机制——所以本包在"宿主已装 libwebkit2gtk-4.1"的机器上窗口一定起得来，
#   在没装的机器上主窗口能不能起、网页进程会不会折，**没有验证过**（真宿主视觉走查本来就欠着，归 P2）。
#   已做的努力：辅助进程与资源目录照样捆绑 + AppRun 里把能指的路径全指进去。

set -euo pipefail

VERSION="${1:-${VERSION:-0.0.0-m5}}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
OUT="$ROOT/build/out"
STAGE="$OUT/root"

# shellcheck source=../common/stage-linux.sh
source "$ROOT/build/common/stage-linux.sh"

say "appimage ① 拼 AppDir（可执行文件 + 界面产物）"
APPDIR="$OUT/appimage/AppDir"
rm -rf "$OUT/appimage"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/applications" \
         "$APPDIR/usr/share/icons/hicolor/256x256/apps"
cp -a "$STAGE/opt/$PKG/$PKG-gui" "$STAGE/opt/$PKG/$PKG-service" "$APPDIR/usr/bin/"
chmod 0755 "$APPDIR/usr/bin/$PKG-gui" "$APPDIR/usr/bin/$PKG-service"
cp -a "$STAGE/opt/$PKG/web/dist" "$APPDIR/usr/bin/web"
# 桌面入口补一行 Icon=（deb 那份不带 Icon 是因为在系统主题里回退；AppImage 的图标就是自己的）
sed '/^Type=Application/a Icon='"$PKG" "$STAGE/usr/share/applications/$PKG.desktop" \
  > "$APPDIR/usr/share/applications/$PKG.desktop"
cp "$APPDIR/usr/share/applications/$PKG.desktop" "$APPDIR/"

say "appimage ② 生成图标（程序自带的一枚实心圆，不引外部素材）"
python3 - "$APPDIR" "$PKG" <<'PYEOF'
import struct, sys, zlib, os
appdir, pkg = sys.argv[1], sys.argv[2]
W = H = 256
rows = bytearray()
for y in range(H):
    rows.append(0)                       # 每行行首一个 filter 字节
    for x in range(W):
        dx, dy = x - W/2, y - H/2
        inside = dx*dx + dy*dy <= (W*0.38)**2
        # 深青底白圆点：仓库名 github-accel，取"加速=圆点飞出"的极简形
        dot = (x - W*0.42)**2 + (y - H*0.42)**2 <= (W*0.16)**2
        rows += bytes((13, 71, 110, 255)) if inside and not dot else \
                bytes((250, 250, 250, 255)) if inside and dot else \
                bytes((0, 0, 0, 0))
def chunk(tag, data):
    c = tag + data
    return struct.pack('>I', len(data)) + c + struct.pack('>I', zlib.crc32(c))
png = (b'\x89PNG\r\n\x1a\n'
       + chunk(b'IHDR', struct.pack('>IIBBBBB', W, H, 8, 6, 0, 0, 0))
       + chunk(b'IDAT', zlib.compress(bytes(rows), 9))
       + chunk(b'IEND', b''))
icon_dir = f'{appdir}/usr/share/icons/hicolor/256x256/apps'
for p in (f'{icon_dir}/{pkg}.png', f'{appdir}/{pkg}.png'):
    open(p, 'wb').write(png)
os.symlink(f'{pkg}.png', f'{appdir}/.DirIcon')
PYEOF

say "appimage ③ 按 ldd 闭包捆绑动态库（glibc 核心除外）"
LIBDIR="$APPDIR/usr/lib"
mkdir -p "$LIBDIR"
EXCL='(ld-linux|libc\.so|libm\.so|libdl\.|libpthread|librt\.)'
for BIN in "$APPDIR/usr/bin/$PKG-gui" "$APPDIR/usr/bin/$PKG-service"; do
  ldd "$BIN" | grep -oE '/[^ ]+\.so[^ ]*' | sort -u | while read -r lib; do
    case "$(basename "$lib")" in $EXCL) continue ;; esac
    cp -Ln "$lib" "$LIBDIR/" 2>/dev/null || true
  done
done
# 库带了资源目录的几大家，一并搬（搬的路径保持与系统一致，AppRun 里再指过去）：
TRIPLE=$(gcc -dumpmachine 2>/dev/null || echo x86_64-linux-gnu)
SRC=/usr/lib/$TRIPLE
for d in gio/modules gdk-pixbuf-2.0 gtk-3.0 webkit2gtk-4.1; do
  [ -d "$SRC/$d" ] && cp -a "$SRC/$d" "$LIBDIR/$TRIPLE/" 2>/dev/null || true
done
mkdir -p "$APPDIR/usr/share/glib-2.0" "$APPDIR/usr/share/webkitgtk-4.1" 2>/dev/null || true
cp -a /usr/share/glib-2.0/schemas "$APPDIR/usr/share/glib-2.0/" 2>/dev/null || true
[ -f /usr/share/glib-2.0/schemas/gschemas.compiled ] || \
  glib-compile-schemas "$APPDIR/usr/share/glib-2.0/schemas" 2>/dev/null || true
[ -d /usr/share/webkitgtk-4.1 ] && cp -a /usr/share/webkitgtk-4.1 "$APPDIR/usr/share/" || true
echo "捆绑了 $(find "$LIBDIR" -name '*.so*' | wc -l) 个 .so"

cat >"$APPDIR/AppRun" <<APPEOF
#!/bin/sh
# AppRun：把捆绑库与资源目录指给 gtk/webkit。pixbuf 的 loaders.cache 里是绝对路径，
# squashfs 只读改不了——运行时改写到临时文件再喂给 GDK_PIXBUF_MODULE_FILE。
APPDIR=\$(cd "\$(dirname "\$0")" && pwd)
TRIPLE=$TRIPLE
export LD_LIBRARY_PATH="\$APPDIR/usr/lib\${LD_LIBRARY_PATH:+:\$LD_LIBRARY_PATH}"
export GIO_MODULE_DIR="\$APPDIR/usr/lib/\$TRIPLE/gio/modules"
export GTK_PATH="\$APPDIR/usr/lib/\$TRIPLE"
export GSETTINGS_SCHEMA_DIR="\$APPDIR/usr/share/glib-2.0/schemas"
CACHE="\$APPDIR/usr/lib/\$TRIPLE/gdk-pixbuf-2.0/2.10.0/loaders.cache"
if [ -f "\$CACHE" ]; then
  TMPD=\$(mktemp -d)
  sed "s|/usr/lib/\$TRIPLE|\$APPDIR/usr/lib/\$TRIPLE|g" "\$CACHE" > "\$TMPD/loaders.cache"
  export GDK_PIXBUF_MODULE_FILE="\$TMPD/loaders.cache"
fi
exec "\$APPDIR/usr/bin/$PKG-gui" "\$@"
APPEOF
chmod 0755 "$APPDIR/AppRun"

say "appimage ④ 下载 appimagetool 并打包"
TOOL="$OUT/tools/appimagetool-x86_64.AppImage"
mkdir -p "$OUT/tools"
if [ ! -f "$TOOL" ]; then
  curl -fsSL -o "$TOOL" \
    https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage
  chmod 0755 "$TOOL"
fi
APPIMAGE="$OUT/${PKG}_${VERSION}_x86_64.AppImage"
rm -f "$APPIMAGE"
"$TOOL" --appimage-extract-and-run "$APPDIR" "$APPIMAGE" 2>&1 | grep -v '^zsyncmake' || true

say "appimage ⑤ 产物自检（运行时能不能起、结构对不对）"
ls -la "$APPIMAGE"
"$APPIMAGE" --appimage-help >/dev/null 2>&1 && echo "AppImage 运行时自检：OK" \
  || die "打出来的 AppImage 连 --appimage-help 都不应——运行时坏了，别出这个包"
echo
echo "产物：$APPIMAGE"
echo "版本 $VERSION／构建时间 $BUILD_DATE 已注进 service；窗口在真宿主上的表现见脚本头那条交底"
