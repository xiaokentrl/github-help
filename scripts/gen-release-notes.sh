#!/usr/bin/env bash
# 起草 release-notes.md（phpo 同款双语模板：英文在前、中文在后）。
# 素材：上一颗 v* 标签到 HEAD 的 git 提交主题（这个项目没有 CHANGELOG，git 记录就是账）。
# 分界线（新增／修改／删除）与中英措辞由人定稿——脚本只把素材和模板摆好，不猜分类、不机翻。
# 用法：bash scripts/gen-release-notes.sh [版本]（缺省取 VERSION 文件——发版打的正是那个号）。
# 流程：make notes 起草 → 人工定稿 → 随版本一起提交 → 推 v<版本> 标签触发 Release 工作流。
#   工作流会核对"release-notes.md 里有这个版本号"，没写版本号的定稿推上去会被当场拒。

set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/.." && pwd)
cd "$ROOT"

VERSION="${1:-$(cat "$ROOT/VERSION" 2>/dev/null || echo 0.0.0-m5)}"
[ -n "$VERSION" ] || { echo "取不到版本号" >&2; exit 1; }

PREV=$(git tag -l 'v*' --sort=-v:refname | head -1 || true)
if [ -n "$PREV" ]; then
  RANGE="$PREV..HEAD"
  PREVNUM="${PREV#v}"
  HEAD_EN="What's Changed from $PREVNUM to $VERSION"
  HEAD_ZH="相比 $PREVNUM 更新内容"
else
  RANGE="HEAD"
  HEAD_EN="first release"
  HEAD_ZH="首个发布"
fi

if [[ "$VERSION" == *-* ]]; then
  TYPE_EN="Milestone"; TYPE_ZH="里程碑版"
else
  TYPE_EN="Stable"; TYPE_ZH="稳定版"
fi

LOG=$(git log --reverse --pretty='- %s' "$RANGE" 2>/dev/null || true)
[ -n "$LOG" ] || LOG='- (no commits since last tag)'

{
cat <<EOF
## English

### github-accel $VERSION $HEAD_EN

**Version type: $TYPE_EN**

**Changed**

EOF
printf '%s\n\n' "$LOG"
cat <<'EOF'
(Draft: split into Added / Changed / Removed and polish wording by hand; the bullets above are verbatim commit subjects. Delete this line before release.)

## 中文

EOF
cat <<EOF
### $VERSION $HEAD_ZH

**版本类型：$TYPE_ZH**

**修改**

EOF
printf '%s\n\n' "$LOG"
cat <<'EOF'
（草稿：请人工把条目分进「新增／修改／删除」并改写措辞；上面是提交主题原文照录。发版前删掉这一行。）
EOF
} > "$ROOT/release-notes.md"

echo "已起草 $ROOT/release-notes.md（版本 $VERSION，对比基准 ${PREV:-无，首个}）"
echo "下一步：人工定稿 → git commit → git tag v$VERSION && git push origin v$VERSION"
