#!/usr/bin/env python3
"""生成应用图标（256×256 PNG），deb／rpm／AppImage 三条打包线共用这一份。

干什么用：程序自身不带美术素材，图标由脚本现场生成——深青底白圆点，
取"加速＝圆点飞出"的极简形。为什么要公用一个文件：三条线各画一份，
迟早画成三张不一样的图，菜单里 deb 一个样、AppImage 又一个样。

用法：python3 build/common/make-icon.py <输出路径> [<输出路径> ...]
"""
import struct
import sys
import zlib

W = H = 256


def render() -> bytes:
    rows = bytearray()
    for y in range(H):
        rows.append(0)  # 每行行首一个 filter 字节
        for x in range(W):
            dx, dy = x - W / 2, y - H / 2
            inside = dx * dx + dy * dy <= (W * 0.38) ** 2
            dot = (x - W * 0.42) ** 2 + (y - H * 0.42) ** 2 <= (W * 0.16) ** 2
            rows += (
                bytes((13, 71, 110, 255)) if inside and not dot
                else bytes((250, 250, 250, 255)) if inside and dot
                else bytes((0, 0, 0, 0))
            )
    return bytes(rows)


def chunk(tag: bytes, data: bytes) -> bytes:
    c = tag + data
    return struct.pack(">I", len(data)) + c + struct.pack(">I", zlib.crc32(c))


def main() -> None:
    if len(sys.argv) < 2:
        sys.exit("用法: make-icon.py <输出路径> [...]")
    png = (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", W, H, 8, 6, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(render(), 9))
        + chunk(b"IEND", b"")
    )
    for path in sys.argv[1:]:
        with open(path, "wb") as f:
            f.write(png)
        print(f"图标已写 {path}")


if __name__ == "__main__":
    main()
