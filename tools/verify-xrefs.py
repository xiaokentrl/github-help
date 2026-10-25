#!/usr/bin/env python3
"""核门牌脚本：查「`某某.md` 第 N 节」这类引用，目标文件里真有没有那级编号标题。
干什么用：六份文档互相指门牌（"账只写在后端那份第 15 节"），那份文件改写或重排编号后引用就指空。
会错在哪：① 目标文件名不存在 → bad；② 有这文件但没那级编号标题 → bad，并列出它实有哪些号；
          ③ 只认两种写法——**反引号包住的 .md 文件名**紧跟「第 N 节」，以及「本文第 N 节」这一类自指；
             光秃秃一个「第 N 节」不猜归属（宁可不报，也不报一条假的把人引去改本来对的地方）；
          ④ 它只核编号在不在，不核那一节内容还对不对——内容对不对要人读。
怎么回来：报 bad 就去目标文件看那节改叫什么、改引用那一方；不许顺手改目标文件的编号迁就引用。
ROOT 用环境变量 XREF_ROOT 覆盖（负向实测要在 /tmp 副本上跑，否则读的还是真仓库、假绿）。
2026-10-23 起这份脚本与 verify-anchors.py 一起住进 tools/（原先只在 /tmp，重启一次就没了，
下一个人连"门牌多少条"都没法复现）；搬动它没改一条判据，搬完在 /tmp 副本注入过一次坏门牌，照样报 bad。"""
import os
import re
import sys

ROOT = os.environ.get("XREF_ROOT", "/home/kentrl/github-hlep")
DOCS = sys.argv[1:] or [
    "project-todo.md", "前端目录结构规划.md", "组件库规划.md",
    "技术方案规划.md", "项目需求和缺口盘点.md", "后端目录结构规划.md",
]
# `X.md` 与「第 N 节」之间最多隔 6 个非汉字字符（够放一个空格或一个"里"字前的标点）
CROSS = re.compile(r"`([^`\s]+\.md)`[\s，、的]{0,6}第\s*(\d+(?:\.\d+)*)\s*节")
SELF = re.compile(r"(?:本文|本篇|本节所在文件|本文那?)\s*第\s*(\d+(?:\.\d+)*)\s*节")
HEAD = re.compile(r"^#{1,6}\s+(\d+(?:\.\d+)*)[\.\s、]")

def headings(path):
    out = set()
    with open(path, encoding="utf-8", errors="replace") as fh:
        for line in fh:
            m = HEAD.match(line)
            if m:
                out.add(m.group(1))
    return out

table = {}
for doc in DOCS:
    path = doc if os.path.isabs(doc) else os.path.join(ROOT, doc)
    if os.path.exists(path):
        table[os.path.basename(path)] = headings(path)

total = 0
bad = []
for doc in DOCS:
    path = doc if os.path.isabs(doc) else os.path.join(ROOT, doc)
    name = os.path.basename(path)
    with open(path, encoding="utf-8") as fh:
        lines = fh.read().split("\n")
    hits = []
    for lineno, line in enumerate(lines, 1):
        for m in CROSS.finditer(line):
            hits.append((lineno, m.group(1), m.group(2)))
        for m in SELF.finditer(line):
            hits.append((lineno, name, m.group(1)))
    for lineno, owner, num in hits:
        total += 1
        if owner not in table:
            bad.append(f"{name}:{lineno} 指向不存在的文件 {owner}（第 {num} 节）")
            continue
        if num not in table[owner]:
            root_num = num.split(".")[0]
            have = "、".join(sorted(x for x in table[owner] if x.split(".")[0] == root_num)) or "无此大节"
            bad.append(f"{name}:{lineno} {owner} 没有「第 {num} 节」（它实有：{have}）")
print(f"跨文档门牌 {total} 条、bad={len(bad)}")
for item in bad[:40]:
    print("BAD", item)
sys.exit(1 if bad else 0)
