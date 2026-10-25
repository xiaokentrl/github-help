#!/usr/bin/env python3
"""核锚脚本：查这六份文档里每一条 `[…](file:///绝对路径#L行号)` 指的那一行真在不在。
干什么用：六份文档大量用「原型第几行」当证据，原型一改行号就漂，漂了没人查就成了假锚点。
          跑法（仓库根，一条命令）：
          python3 tools/verify-anchors.py project-todo.md 前端目录结构规划.md 组件库规划.md \
                    后端目录结构规划.md 技术方案规划.md 项目需求和缺口盘点.md
会错在哪：① 链接指向的文件不存在或行号超出文件行数 → bad；
          ② 同一行里出现两条完全相同的锚点 → dup（多半是复制粘贴，读的人会以为有两处证据）；
          ③ 正文里的裸 `[` 会被当成链接文字起点，所以散文里不许留裸方括号。
          ④ 它只判「存不存在」，不判「内容对不对」——行号还在但那一行已换成别的代码，它看不见。
怎么回来：报 bad 就逐条打开那一行看内容对不对。ROOT 用环境变量 ANCHOR_ROOT 覆盖
          （负向实测要在 /tmp 副本上跑：脚本默认读真仓库，指错地方就变成假绿）。
          2026-10-23 起这份脚本住在仓库里（原先只在 /tmp，重启一次就没了，
          下一个人连"锚点合计多少"都没法复现）；搬动它没改一条判据，搬完在 /tmp 副本注入过一次坏锚点，照样报 bad。"""
import os
import re
import sys

ROOT = os.environ.get("ANCHOR_ROOT", "/home/kentrl/github-hlep")
LINK = re.compile(r"\[([^\]\[]*)\]\(file://([^)#\s]+)(#L(\d+))?[^)]*\)")

def line_count(path):
    with open(path, encoding="utf-8", errors="replace") as fh:
        return sum(1 for _ in fh)

total = 0
bad = []
dups = []
cache = {}
for arg in sys.argv[1:]:
    doc = arg if os.path.isabs(arg) else os.path.join(ROOT, arg)
    name = os.path.basename(doc)
    with open(doc, encoding="utf-8") as fh:
        lines = fh.read().split("\n")
    per_doc = 0
    for idx, text in enumerate(lines, 1):
        seen = {}
        for m in LINK.finditer(text):
            target, line = m.group(2), m.group(4)
            anchor = (target, line)
            per_doc += 1
            seen[anchor] = seen.get(anchor, 0) + 1
            if not os.path.isabs(target):
                bad.append(f"{name}:{idx} 相对路径 {target}")
                continue
            if not os.path.exists(target):
                bad.append(f"{name}:{idx} 文件不存在 {target}")
                continue
            if line is None:
                bad.append(f"{name}:{idx} 锚点没有行号 {target}")
                continue
            if target not in cache:
                cache[target] = line_count(target)
            n = int(line)
            if n < 1 or n > cache[target]:
                bad.append(f"{name}:{idx} 行号 {n} 超出 {os.path.basename(target)}（共 {cache[target]} 行）")
        for (target, line), cnt in seen.items():
            if cnt > 1:
                dups.append(f"{name}:{idx} 同一行重复锚点 {os.path.basename(target)}#{line} ×{cnt}")
    total += per_doc
    print(f"{name}: 锚点 {per_doc}")
print(f"合计 {total} 处、bad={len(bad)}、dups={len(dups)}")
for item in bad[:20]:
    print("BAD", item)
for item in dups[:20]:
    print("DUP", item)
sys.exit(1 if (bad or dups) else 0)
