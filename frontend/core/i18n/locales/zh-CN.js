/* 简体中文文案，唯一一份。干什么用：core 的挡路话、流程提示、确认框里的每一句话都只写在这里，
   代码里只留键名（gate.busy.queue 这种），界面拿到键交给 t() 换成这一份里的字。
   会错在哪：① 值一改，tests/i18n.test.mjs 会把"代码在用但这里没有"和"这里写着但没人用"两种情况都报红——
                左边那份名单会过期，比写错字更难发现；
             ② 这里的每一句都是照着原型逐字抄的（后面注释的那行号就是抄来的地方）。
                改了字＝改了界面上用户看到的原话，属于"私自改变其他功能"，除非有人点名要改。
   怎么回来：新增一种语言＝在这个目录里新增一个同键名的文件，不改这里，也不改 t.js；
             键名按「谁说的」分组：core 那边三组——gate.*（点不动的那句解释）、flow.*（一条流程跑完/跑挂时说的话，
             组名后面再跟那条流程的名字）、err.*（为什么没成那一句，键名是 err.＋错误码，
             这一族由 tests/errors.test.mjs 对账、上面那条"没人用"扫描放过它）；
             phase.* 那一组随 mock 一起退役了：正在干哪一步那八句在 core/meta.js 的表里，服务回的阶段名只认那四档。
             界面那边按块分——tree.* / detail.* / queue.* / toolbar.* / shortcut.*（快捷键的回执，跟工具栏那颗钮的句子不同）/
             statusbar.* / topbar.* / banner.*（首次使用那条横幅）/ tray.*（顶部那颗托盘胶囊与它的下拉）/
             layout.* / sidebar.*，
             再加一组所有块共用的 common.*（破折号、取消、确认）；
             设置中心与主题弹窗这一轮（views/settings、views/theme-picker）再补两组：
             settings.*（外壳、导航、七个面板各一句）与 theme.*（「选择主题」那六张卡的弹窗）。
             名单与 tests/i18n.test.mjs 的 KEY_RE 前缀是同一份：新增一组＝两处一起加，漏一处那组键会被判成"没人用"。 */

export default {
  /* ---- 点不动时那句解释（原型 :2275-2282、:2321-2327、:2344、:2774-2805、:2846-2851、:2874、:2891、:3178-3180） ---- */
  'gate.busy.startup': '启动自动化进行中，请稍候',              // :2275
  'gate.busy.detecting': '批量检测进行中，请稍候',              // :2276
  'gate.busy.queue': '队列正在执行中，请稍候',                  // :2277
  'gate.busy.queueDetect': '队列正在执行中，请稍候再检测',       // :2323 「检测全部」专用
  'gate.busy.queueApply': '队列正在执行中',                     // :2847 「应用队列」专用，原型这里没写「请稍候」
  'gate.busy.detectWait': '正在批量检测，请稍候',               // :2779 还原／:2848 应用队列撞见批量检测
  'gate.busy.restoreTask': '队列正在执行中，请稍后再恢复',       // :2891 队列行里那颗「↺ 恢复到应用前」
  'gate.busy.restoreAllStartup': '启动自动化进行中，无法恢复',   // :3184 工具栏「恢复」，三条结尾都是「无法恢复」
  'gate.busy.restoreAllQueue': '队列正在执行中，无法恢复',       // :3185
  'gate.busy.restoreAllDetecting': '正在批量检测，无法恢复',     // :3186
  'gate.detect.pick': '请先勾选要检测的项',                     // :2327
  'gate.detect.nothing': '没有可检测的项（可能已在队列中）',     // :2344 筛完全没了
  'gate.detect.alreadyQueued': '该项已在检测队列中',             // :2280
  'gate.apply.runningItem': '该项正在队列中执行',                // :2282
  'gate.apply.needDetect': '请先检测生成候选 IP',                // :2874 也是详情面板那条灰字
  'gate.apply.pick': '请先勾选要替换的项',                      // :2851
  'gate.restore.nothing': '没有可还原的状态',                    // :2774
  'gate.restore.noneInCat': '此类目下没有可还原项',              // :2805

  /* ---- 检测（原型 :2313-2314、:2368、:2405、:2425-2431、:2443） ---- */
  'flow.detect.start': '开始检测 {n} 项…',
  'flow.detect.summary': '检测完成：{parts}',
  'flow.detect.noChange': '无变化',
  'flow.detect.partDetected': '待应用 {n}',
  'flow.detect.partApplied': '已是最优 {n}',
  'flow.detect.partStale': '待更新 {n}',
  'flow.detect.partError': '失败 {n}',
  /* 「这一条检测失败了」那两句不在这里：真服务回的是错误码，句子在下面的 err.* 那一族（已定：D6 乙）。
     上一版 mock 自己拼的 flow.detect.failed／kept／crash 与 flow.item.gone 四条随 mock 一起退役。 */

  /* ---- 应用（原型 :2855-2868、:2879-2880） ----
     原型 :2671、:2704、:2709 那三句"没有候选／写不进去／已回滚"同样随 mock 退役：真服务把这三件事
     回成 GAC-DETECT-001／GAC-HOSTS-003／GAC-HOSTS-004 三个码，界面上说的是下面 err.* 那三句。 */
  'flow.apply.askTitle': '有 {n} 项尚未检测',
  'flow.apply.askBody': '检测是应用的必经步骤。',                 // 原型 :2857 用 <br> 断成两行，这里就两行
  'flow.apply.askBody2': '是否先自动检测这 {n} 项，再统一应用？',
  'flow.apply.askConfirm': '先检测再应用',
  'flow.apply.noneApplicable': '没有可应用的项（均未检测）',
  'flow.apply.alreadyQueued': '所选项目已在队列中',               // :2867 「应用队列」那一条
  'flow.apply.alreadyQueuedOne': '该项已在队列中',                // :2880 「仅应用此项」那一条，两句不同
  'flow.enqueued': '已加入队列 {n} 项',
  'flow.enqueuedOne': '已加入队列：{name}',

  /* ---- 队列操作（原型 :3110、:3115、:3123-3126、:3141、:3149、:3153、:3157、:3165） ---- */
  'flow.queue.pauseWillHold': '将在当前任务完成后暂停',
  'flow.queue.resumed': '队列已继续',
  'flow.queue.cancelTitle': '取消等待中的任务',
  'flow.queue.cancelBody': '将取消 {n} 个等待中的任务，',          // 原型 :3124 <br> 后面那句单独一行
  'flow.queue.cancelBody2': '正在执行的任务不受影响。',
  'flow.queue.cancelConfirm': '取消任务',
  'flow.queue.cancelled': '已取消 {n} 个等待任务',
  'flow.queue.cleared': '已清理 {n} 条已完成任务',
  'flow.queue.noRetry': '没有需要重试的任务',
  'flow.queue.requeued': '已重新入队 {n} 项',
  'flow.queue.requeuedOne': '已重新入队：{name}',

  /* ---- 还原 / 恢复（原型 :2746、:2783-2799、:2812-2842、:2892、:2902-2952） ---- */
  'flow.noRecord': '（无记录）',                                  // 应用前／当前 IP 空着时填的那格
  'flow.restore.itemConfirmTitle': '还原 Hosts',
  'flow.restore.itemConfirmBody': '将把 {name} 的 hosts 记录还原到本次应用前的状态。',
  'flow.restore.prevIp': '应用前 IP {ip}',
  'flow.restore.curIp': '当前写入 IP {ip}',
  'flow.restore.wasAbsent': '应用前 hosts 无记录，还原后相当于删除该项。',
  'flow.restore.willDisappear': '还原后该项回到应用前的状态，还原按钮消失。',
  'flow.restore.confirm': '确认还原',
  /* 单项还原跑完那三句（itemDone／itemRetry／itemFailed）随 mock 退役：真服务把结果写成那一行任务的状态
     （已恢复／失败＋错误码），界面上说的是 err.* 那一句与队列那一行，不再由流程自己复述一遍。 */
  'flow.restore.catTitle': '还原「{name}」下的 Hosts',
  'flow.restore.catCount': '将把 {n} 项还原到应用前状态。',
  'flow.restore.catDisappear': '还原后这些项的「还原Hosts」按钮会消失。',   // :2819 类目自己的一句，跟单项那句不同
  'flow.restore.catListLabel': '可还原项：',
  'flow.restore.catLine': '{name}：{prev} ← {cur}',
  'flow.restore.catAll': '「{name}」下 {n} 项已全部还原',
  'flow.restore.catNone': '「{name}」下全部 {n} 项还原失败',
  'flow.restore.catMixed': '「{name}」下 {ok} 项成功，{fail} 项失败',
  'flow.restore.alreadyDone': '该项已还原，无需重复操作',
  'flow.restore.taskTitle': '恢复到应用前状态',
  'flow.restore.taskBody': '将把 {name} 在 hosts 中的记录恢复到本次应用前的状态。',
  'flow.restore.phases': '恢复流程：备份 → 写回 → 刷新 DNS → 验证。恢复成功后该任务标记为「已恢复」。',
  'flow.restore.taskConfirm': '确认恢复',
  /* 原型 :2902-2952 那两条"失败：hosts 写入失败…"与那一句"已恢复到应用前状态"是 mock 演出来的过程话；
     真服务回的是错误码与那一行的状态，所以这里只留点完之后的那两句：跑挂了提示去看队列，跑完了报个名字。 */
  'flow.restore.taskFailedToast': '恢复失败，请查看队列错误信息',
  'flow.restore.taskDone': '{name} 已恢复',

  /* ---- 工具栏「恢复」＝整体回到没写过 hosts 的样子（原型 :3184-3209） ----
     那三条前面的「• 」是原型 bodyHtml 里就写着的，照抄；确认框只把一行行字排出来，不再自己加符号。 */
  'flow.restoreAll.title': '恢复默认 hosts',
  'flow.restoreAll.lead': '将执行以下操作：',
  'flow.restoreAll.drop': '• 移除所有由本工具写入的域名映射',
  'flow.restoreAll.backup': '• 恢复修改前的 hosts 备份',
  'flow.restoreAll.clear': '• 清空当前队列',
  'flow.restoreAll.warn': '此操作不可撤销。',
  'flow.restoreAll.confirm': '确认恢复',
  'flow.restoreAll.done': '已恢复默认 hosts，所有项回到「未替换」',

  /* ---- 启动自动化（原型 :2964-3001，每句都带「启动自动化：」前缀，为的是跟手点的那几条分得开） ---- */
  'flow.startup.noChecked': '启动自动化：没有勾选项，已跳过',
  'flow.startup.allBusy': '启动自动化：勾选项都在执行中，已跳过',
  'flow.startup.detect': '启动自动化：开始检测 {n} 项…',
  'flow.startup.nothingToApply': '启动自动化：检测后没有可应用的项，已跳过',
  'flow.startup.alreadyQueued': '启动自动化：所有项已在队列中，跳过入队',
  'flow.startup.apply': '启动自动化：入队 {n} 项，开始应用…',
  'flow.startup.crash': '启动自动化异常：{msg}',

  /* ---- 「同步状态」那一句（原型 :1743） ---- */
  'flow.sync.changed': '同步完成，{n} 项状态有更新',
  'flow.sync.clean': '同步完成，界面已与真实状态一致',

  /* ==== 为什么没成：错误码那一句（服务只回「码＋参数」，句子全在这里。已定：D6 乙） ====
     键名＝`err.` ＋ internal/errs/codes.go 里那个码，两边集合等值、句子里的占位符有没有依据，
     都由 frontend/tests/errors.test.mjs（第十三条门禁 G13）现读 Go 那一份对账，这一族不进上面那条"没人用"扫描。
     写句子的一条规矩：{占位符}只准用「这个码每一个出错现场都保证给得到」的那几个数——
     少给一个，屏幕上就原地留着 {path} 这种字样（t.js 的行为，见 core/i18n/t.js）。这一条由 G13 现读 Go 那侧的出错现场逐码查。
     所以下面有九条例子故意不带数（GAC-DETECT-001／GAC-HOSTS-003、004、008／GAC-SETTINGS-003、004／
     GAC-TASK-001、002、003）：这些码在各个出错现场给的东西不齐，带上去会原地露出占位符。
     具体是哪个域名、哪个路径，队列那一行的错误与日志里的原文都能看到。
     只有 GAC-IPC-040 那句带的 {endpoint} 例外：这条线连的是哪个文件由界面这一侧补（core/api/wire.js），
     真拿不到时 errors.js 给一句兜底的话，界面上不会露出 {endpoint}。 */
  'err.GAC-IPC-001': '后台服务回的那一行读不懂，界面没法照它画',
  'err.GAC-IPC-002': '界面发出的那一句话没带上要办的事，这动作没送出去',
  'err.GAC-IPC-003': '后台服务答不回来了：回执送不回界面（对面可能已经关掉）',
  'err.GAC-IPC-010': '连上的不是本项目的后台服务。为安全起见，这句话没说出去',
  'err.GAC-IPC-011': '后台服务里没有这个功能，多半是界面与服务不是同一轮构建',
  'err.GAC-IPC-012': '后台服务办这件事的时候出错了，具体原因在服务日志里',
  'err.GAC-IPC-020': '后台服务起不来：建不出放运行时文件的目录',
  'err.GAC-IPC-021': '后台服务起不来：本机那条监听线开不了',
  'err.GAC-IPC-022': '后台服务起不来：通信文件的权限没能收成只有它自己能读写',
  'err.GAC-IPC-023': '后台服务起不来：上一次留下的通信文件删不掉（可能还有一个服务在跑）',
  'err.GAC-IPC-030': '这个平台的通信线还没接上（Windows 那条要到 M1 那一站才做）',
  'err.GAC-IPC-040': '连不上后台服务（{endpoint}）。它可能没在跑：启动它，界面会自己接上',
  'err.GAC-IPC-041': '后台服务 12 秒没答话，这一句先当没成功，屏幕上还是上一次的读数',
  'err.GAC-IPC-042': '后台服务答回来的样子不对，界面认不出它是哪一句',
  'err.GAC-IPC-043': '界面和后台服务的接口版本对不上，请把两个一起升级成同一版',

  'err.GAC-HOSTS-001': '读不到 hosts 文件（{path}）：确认它还在不在、当前身份能不能读',
  'err.GAC-HOSTS-002': 'hosts 里有一行读不懂。这一行我们不会偷偷丢掉，也不会往里写',
  'err.GAC-HOSTS-003': '写不进 hosts：权限不够，或文件被别的程序占着',
  'err.GAC-HOSTS-004': '写进去以后重读对不上，已经用备份还原回去了',
  'err.GAC-HOSTS-005': '还原也失败了——这是最坏那一档。备份在 {backup}，要还原的是 {path}，请人工照这两份核对',
  'err.GAC-HOSTS-006': '已经有一个写入在跑了，等它结束再点这一下',
  'err.GAC-HOSTS-007': '「{domain}」被两条条目一起管着，这一步拒绝写 hosts（先只留一条）',
  'err.GAC-HOSTS-008': '这条数据不是域名该有的样子，拒绝写入',
  'err.GAC-HOSTS-009': '「{domain}」跟别的域名挤在同一行（{line}，同一行还有 {otherNames}）：整行停用会连累它们，拆开又等于替你重排，所以这一行没动',
  'err.GAC-HOSTS-010': 'hosts 改成了，但新文件的属主没补回原来那一份（当前身份不是 root，或这一半要到 M5）',
  'err.GAC-HOSTS-011': '改动前那份备份没存下来。这一次写入照做了，但出问题时要你自己找依据',

  'err.GAC-SETTINGS-001': '找不到放设置的目录，界面不知道要把设置存到哪儿',
  'err.GAC-SETTINGS-002': '读不到设置文件（{path}）',
  'err.GAC-SETTINGS-003': '设置文件看不懂，界面上的设置不会跟着改',
  'err.GAC-SETTINGS-004': '设置写不出去，改完会退回原来的值',

  'err.GAC-DETECT-001': '这一条没检出可用的地址，详情面板那一列还是空的',
  'err.GAC-DETECT-002': '「{mode}」这一路取数在 {limit} 之内没交回结果，本轮换下一路',
  'err.GAC-DETECT-003': '交回的地址全都服务不了这个域名，换下一批候选还有意义',
  'err.GAC-DETECT-004': '这个域名疑似被针对性阻断，换 IP 也不顶用，先别反复重试',
  'err.GAC-DETECT-005': '这一轮超出取数与探测的预算：只做了 {fetches}／{fetchLimit} 次取数、{probes}／{probeLimit} 次探测，剩下的下一轮再排',
  'err.GAC-DETECT-006': '这一种取数方式要联网，而现在连不通',

  'err.GAC-PERM-001': '当前身份写不动 hosts（{path}）：{cause}',
  'err.GAC-PERM-002': '刷 DNS 缓存没成功，浏览器可能还在用旧地址（Windows 上那项服务要权限）',

  'err.GAC-TASK-001': '队列里找不到这个任务号，那一下没改成',
  'err.GAC-TASK-002': '这一行已经跑到头了，撤不动它',
  'err.GAC-TASK-003': '队列暂停中，这一步在动手前被叫停；点「继续队列」再跑',
  'err.GAC-TASK-004': '清单里没有这条条目（{itemId}）——它不是检测失败，也不是加速失败，是根本没这一条',

  'err.GAC-UPDATE-501': '{method} 这一档要到 {milestone} 那一站才有，本期不提供',

  'err.GAC-UNKNOWN-000': '撞上一条没登记的错误（{code}），后台服务那句原文是：{message}。请把这两样一起反馈',

  /* ---- 左侧树（原型 :1806-1808、:1822-1824、:1831） ----
     两句「还原…」的悬停说明逐字照原型；tree.itemCheckAria／tree.catCheckAria 原型没有——
     那里是 <span class="cb">（:1799、:1816），既不能 Tab 聚焦也没有名字，这两句是补读屏名用的，不是改功能。 */
  'tree.restoreItemTitle': '还原到本次应用前的 Hosts 状态',
  'tree.restoreCatTitle': '还原「{name}」下所有可还原项到本次应用前的 Hosts 状态',
  'tree.itemCheckAria': '勾选 {name}',
  'tree.catCheckAria': '勾选类目 {name}',
  'tree.empty': '没有匹配的类目或域名',

  /* ---- 三处「没数可显示」时填的那个破折号（原型 :1897、:2073、:2207 用的是同一个字符） ---- */
  'common.dash': '—',

  /* ---- 详情面板（原型 :1894-1991） ----
     七行指标的名字只在 DETAIL_METRICS 那张表里写一遍（:1894），这里跟着它一行一句；
     「需先检测生成候选 IP」跟挡路那句「请先检测生成候选 IP」（gate.apply.needDetect）是原型里就有的两种说法，别合。 */
  'detail.emptyTitle': '请选择左侧任意项查看详情',
  'detail.emptyHint': '勾选后可点击「检测全部」批量检测',
  'detail.m.currentIp': '当前写入 IP',
  'detail.m.bestIp': '检测最优 IP',
  'detail.m.latency': '延迟',
  'detail.m.speed': '下载速度',
  'detail.m.score': '综合评分',
  'detail.m.source': 'IP 来源',
  'detail.m.lastCheck': '上次检测',
  'detail.neverChecked': '尚未检测',
  'detail.cands': '候选 IP',
  'detail.candCount': '{n} 个候选',
  'detail.colIp': 'IP 地址',
  'detail.colTag': '标记',
  'detail.colLatency': '延迟',
  'detail.colSpeed': '速度',
  'detail.colScore': '评分',
  'detail.candEmpty': '尚未检测，点击下方「单独检测」获取候选 IP',
  'detail.best': '最优',
  'detail.domains': '域名',
  'detail.restoreHosts': '还原Hosts',
  'detail.restoreHostsTitle': '还原到本次应用前的 Hosts 状态',
  'detail.detectOne': '单独检测',
  'detail.applyOne': '仅应用此项',
  'detail.viewLog': '查看日志',
  'detail.logToast': '已打开 {name} 的操作日志（模拟）',
  'detail.hint.needDetect': '需先检测生成候选 IP',
  'detail.hint.batch': '正在批量检测…',
  'detail.hint.queue': '队列执行中…',
  'detail.hint.startup': '启动自动化中…',

  /* ---- 底部队列（原型 :2031-2128、:2136-2198） ----
     汇总那两句是「按任务类型分列」改动后的产物（原型 :2016-2029 那段注释说清了为什么要分列），
     拼起来用「 · 」；队列只有检测任务时只出现前一句。
     「暂停队列／继续队列」是同一颗钮的两种文字（原型 :2083），按 paused 取哪一句，不是两颗钮。 */
  'queue.title': '队列',
  'queue.summaryDetect': '检测完成 {done}/{total}',
  'queue.summaryApply': '写入完成 {done}/{total}',
  'queue.cur.detect': '检测中',
  'queue.cur.apply': '执行中',
  'queue.cur.restore': '恢复中',
  'queue.cur.line': '{prefix}：{name}',
  'queue.pause': '暂停队列',
  'queue.resume': '继续队列',
  'queue.cancel': '取消等待',
  'queue.clearDone': '清空已完成',
  'queue.retryFailed': '重试失败',
  'queue.collapse': '收起队列',
  'queue.expand': '展开队列',
  'queue.emptyTitle': '队列为空',
  'queue.emptyHint': '勾选类目后点击「检测全部」或「应用队列」',
  'queue.colTask': '任务',
  'queue.colStatus': '状态',
  'queue.colDuration': '耗时',
  'queue.colAction': '操作',
  'queue.scrollTop': '回到顶部',
  'queue.scrollBottom': '滚到底部',
  'queue.kindDetect': '检测',
  'queue.kindApply': '应用',
  'queue.kindRestore': '恢复',
  'queue.actRestore': '恢复',
  'queue.actRestoreTitle': '恢复到应用前的 hosts 状态',
  'queue.actRetry': '重试',
  'queue.actRetryTitle': '重新入队执行',

  /* ---- 确认框那两颗钮的兜底字（原型 :2245 的 confirmText='确认' 与 :762 那颗「取消」） ----
     流程里每一句 confirm 都自己带了确认文字，这两句只在没人给的时候用。 */
  'common.confirm': '确认',
  'common.cancel': '取消',
  'common.appName': 'GitHub 加速助手',              // :657、:669 那两处（顶栏标题与托盘胶囊那行字）是同一个名字

  /* ---- 工具栏那两颗互斥钮与四句点击回执（原型 :1336-1349、:3067-3091） ----
     「全选 ↔ 全不选」「折叠 ↔ 展开」各是同一颗钮的两种文字（:1341、:1346），不是两颗钮；
     悬停说明跟着文字一起换（:1342、:1347），原型写的是「全选所有项（Ctrl+A）」这种带快捷键提示的整句。 */
  'toolbar.selectAll': '全选',
  'toolbar.selectNone': '全不选',
  'toolbar.selectAllTitle': '全选所有项（Ctrl+A）',
  'toolbar.selectNoneTitle': '取消所有勾选（Ctrl+A）',
  'toolbar.fold': '折叠',
  'toolbar.unfold': '展开',
  'toolbar.foldTitle': '折叠所有类目',
  'toolbar.unfoldTitle': '展开所有类目',
  'toolbar.checkedAll': '已全选所有类目',           // :3071 那颗钮按下去
  'toolbar.checkedNone': '已取消所有勾选',           // :3071
  'toolbar.inverted': '已反选',                     // :3077
  'toolbar.expanded': '已展开所有类目',              // :3084
  'toolbar.folded': '已折叠所有类目',                // :3087
  'shortcut.checkedAll': '已全选',                  // :3232 Ctrl+A 那一句，跟 :3071 那句不同，照原型留着两种说法

  /* ---- 工具栏另外五样（原型 :690-703） ----
     三颗大钮的字没有"两种状态"（不像全选／折叠那两颗），所以一句就够；
     模式下拉那三项在原型 :696-698 就是「模式：Hosts」＋两项「未启用」的占位，后两项禁用、点了不动，
     因为换解析方式属后端那一侧（技术方案第 12 节），界面上今天没有这条路；
     清空那颗小钮是 components/text-field 自己补的（原型只有一只光秃的搜索框），所以它那句说明在原型里找不到行号。 */
  'toolbar.invert': '反选',                         // :691
  'toolbar.detectAll': '检测全部',                   // :701
  'toolbar.applyQueue': '应用队列',                  // :702
  'toolbar.restore': '恢复',                        // :703
  'toolbar.searchPlaceholder': '搜索类目、域名或平台',   // :694
  'toolbar.searchClearTitle': '清空搜索',
  'toolbar.modeLabel': '解析方式',
  'toolbar.modeHosts': '模式：Hosts',                // :696
  'toolbar.modeProxy': '系统代理（未启用）',           // :697
  'toolbar.modePac': 'PAC（未启用）',                // :698

  /* ---- 横幅那一整条（原型 :682-687） ----
     中间那句被 <b> 断成三段：pre 与 rest 各自贴着 <b>，所以一句结尾不带标点、另一句开头带那个句号，
     三段合起来才是原型 :683 那一整句。少一段或者把句号挪个位置，界面就与原型差一个字符。 */
  'banner.pre': '首次使用建议：',
  'banner.steps': '勾选类目 → 检测全部 → 应用队列',
  'banner.rest': '。检测/应用时会在树上高亮当前项。',
  'banner.close': '知道了',                          // :686

  /* ---- 状态栏与顶栏（原型 :2207-2229、:3217-3221） ----
     已选那一格在原型是「已选 <b>0</b> / 0 项」：中间那个数单独加粗，所以句子拆成前后两段、数由代码填。
     两段各自带一个空格（前段结尾一个、后段开头一个），那是 </b> 前后那两个空格——去掉任何一个，这行就比原型挤。
     「DNS 」「模式 」「上次检测 」这三句结尾也各带一个空格，那是原型 :749-751 里 <b> 前面那个空格。
     「模式 Hosts」今天还是死值（原型 :749 也是死的：本期只做 Hosts 这一档〔已定〕）。
     第一格原先那句「网络正常」（:745）2026-10-23 换成这两句：它画的是"这条线与壳通不通"，
     真来源是 gac:state 那一句（state.connected）。断网判定（GAC-DETECT-006）接上那天再改回"网络"那一档，
     事件名 network:change 契约里已经占好了。 */
  'statusbar.selectionPre': '已选 ',
  'statusbar.selectionRest': ' / {m} 项',
  'statusbar.linkUp': '后台服务已连上',
  'statusbar.linkDown': '后台服务未连上',
  'statusbar.startup': '启动自动化中',                // :747
  'statusbar.modeLabel': '模式 ',
  'statusbar.modeHosts': 'Hosts',                   // :749 那个 <b> 里的字
  'statusbar.dns': 'DNS ',
  'statusbar.lastDetect': '上次检测 ',
  'topbar.permTitle': '修改 hosts 需要管理员权限',      // :671
  'topbar.checkUpdate': '检查更新',                  // :674 那颗钮的字与悬停说明是同一句
  'topbar.permGranted': '管理员权限',               // :2217
  'topbar.permDenied': '需要管理员权限',             // :2220
  'topbar.permToastGranted': '已获取管理员权限，可正常读写 hosts',           // :3219
  'topbar.permToastDenied': '需要管理员权限才能写入 hosts，请重新以管理员身份运行',  // :3220

  /* ---- 顶部中间那颗托盘胶囊与它的下拉（原型 :655-663、:3289-3320） ----
     两条菜单项那句回执，原型 :3308 写的是「退出：演示占位，这个原型里不会真关窗口。」——
     "这个原型"四个字在真程序里是假话，所以换成说清"哪一条通道还没接"；意思（今天不会真关／真最小化）一字未改。
     最小化与退出真正生效属壳那一侧（技术方案 M0／F7），界面上今天没有那条通道。 */
  'tray.pillTitle': '托盘图标',
  'tray.minimize': '最小化到托盘',                   // :662
  'tray.quit': '退出',                             // :663
  'tray.toastMinimize': '最小化到托盘：还没接上系统托盘那一条通道，界面还开着。',
  'tray.toastQuit': '退出：还没接上系统托盘那一条通道，这个窗口不会真关。',

  /* ---- 两道拖动把手的悬停说明（原型 :89、:94 的 title） ----
     把手归 components/drag-handle，件里不写字，这两句由 main.js 递进去。 */
  'layout.sidebarTitle': '拖动调整主菜单宽度',
  'layout.queueTitle': '拖动调整队列高度',

  /* ---- 侧栏底下那两颗钮（原型 :1722-1744、:711、:714） ----
     「同步中…」是点下去那一刻换上的字，换完还要换回「同步状态」（原型 :1727 与 :1741 那两下）。 */
  'sidebar.sync': '同步状态',
  'sidebar.syncBusy': '同步中…',
  'sidebar.syncTitle': '从后端重新拉取状态，覆盖界面现有数据',
  'sidebar.settings': '设置中心',
  'sidebar.settingsTitle': '打开设置中心',

  /* ==== 设置中心（views/settings，#29/#44 这一轮）====
     每一句都是照着原型 :768-1035 那段逐字抄的，行号注在后面。
     「200–600」那两个界限量用 {min}／{max} 从 core/settings.js 现取（原型写死在 :796，
     而这两个数的家是 SIDEBAR_MIN_W／SIDEBAR_MAX_W，抄进句子就有两份源，改一处句子先说假话）。
     「6 套配色」那六个名字同理，由 themes/ 扫出来的那份清单填 {n} 与 {names}。 */
  'settings.title': '设置中心',                                       // :771
  'settings.close': '关闭',                                          // :774 与 :1046 那两颗 × 的 title
  'settings.save': '保存',                                           // :1034
  'settings.saved': '设置已保存',                                     // :1629
  'settings.langSwitched': '语言已切换，下次启动生效',                 // :1615
  // 那四把只有程序外壳能动的开关（SHELL_KEYS：托盘最小化／显示托盘图标／开机自启／启动时最小化）。
  // 干什么用：保存那一下如果动了一类这样的项，就在「设置已保存」之外再说清一半——存下来是真的，生效还不是。
  // 会错在哪：不说＝界面在承诺一件今天做不到的事（勾了「关闭窗口时最小化到托盘」，窗口照样关）。
  // 怎么回来：壳接上那四件（技术方案 M0／F7）那一轮，把 core/settings.js 的 SHELL_KEYS 里对应那条删掉，
  // 这一句就不再出现（用例 tests/views-settings.test.mjs 钉的是"动了名单里的项才有这一句"）。
  'settings.shellPending': '这 {n} 项已经存下来了，但它们要动的是窗口与开机启动项——那一条通道归程序外壳，今天还没接上，本次运行看不出效果。',
  'settings.group.prefs': '偏好',                                     // :779
  'settings.group.info': '信息',                                      // :785
  'settings.nav.layout': '布局',                                      // :780
  'settings.nav.zoom': '界面缩放',                                     // :781
  'settings.nav.appearance': '外观',                                   // :782
  'settings.nav.tray': '托盘',                                        // :783
  'settings.nav.startup': '开机自启动',                                // :784
  'settings.nav.update': '检测升级',                                   // :786
  'settings.nav.about': '关于我们',                                    // :787
  'settings.sub.layout': '调整主界面元素尺寸',                          // :1540
  'settings.sub.zoom': '调整整体缩放比例',                              // :1540
  'settings.sub.appearance': '语言与主题',                              // :1541
  'settings.sub.tray': '系统托盘与窗口关闭行为',                         // :1541
  'settings.sub.startup': '登录时自动启动后台服务',                      // :1542
  'settings.sub.update': '检查新版本与更新日志',                         // :1542
  'settings.sub.about': '应用信息与反馈',                               // :1543

  'settings.layout.title': '布局',
  'settings.layout.desc': '调整主界面主要区域的尺寸。拖动主界面边缘也可以直接调整，两者会同步。',   // :793
  'settings.layout.sidebar': '主菜单宽度',                             // :795
  'settings.layout.sidebarDesc': '左侧类目树的宽度，范围 {min}–{max} px。拖动侧栏右边缘也可以调整。',  // :796
  'settings.layout.queue': '队列高度',                                 // :807
  'settings.layout.queueDesc': '底部任务队列面板的高度。拖动队列顶部的横条也可以调整。',              // :808
  'settings.layout.detectHead': '检测与应用',                           // :819
  'settings.layout.detectDesc': '控制执行检测或应用时，主菜单是否自动展开当前项所在类目。',            // :820
  'settings.layout.detectFollow': '检测时焦点跟随',                     // :823
  'settings.layout.detectFollowDesc': '开启后，点击「检测全部」时会自动收起其他类目，只展开当前正在检测的项所在类目；检测完成后恢复检测前的折叠状态。关闭时保持用户当前的折叠状态不变，仅通过金色高亮提示当前检测项。',  // :824-828
  'settings.layout.applyFollow': '应用时焦点跟随',                       // :838
  'settings.layout.applyFollowDesc': '开启后，点击「应用队列」时会自动收起其他类目，只展开当前正在应用的项所在类目；应用完成后恢复应用前的折叠状态。关闭时保持用户当前的折叠状态不变，仅通过蓝色高亮提示当前应用项。',  // :839-843
  'settings.size.narrow': '窄',                                        // :799
  'settings.size.standard': '标准',                                    // :800
  'settings.size.wide': '宽',                                          // :801
  'settings.size.small': '小',                                         // :811
  'settings.size.middle': '中',                                        // :812
  'settings.size.large': '大',                                         // :813

  'settings.zoom.title': '界面缩放',
  'settings.zoom.desc': '整体缩放比例，影响所有文字与控件。高 DPI 屏幕建议 110%–125%。',              // :855
  'settings.zoom.ratio': '缩放比例',                                   // :857
  'settings.zoom.ratioDesc': '通过 CSS zoom 实现，仅本机生效。如需更精细的缩放，可在操作系统层面调整显示比例。',  // :858

  'settings.appearance.title': '外观',
  'settings.appearance.desc': '语言切换后要重启才生效；主题是点一下就换，不用重启。',                  // :873
  'settings.appearance.language': '语言',                              // :875
  'settings.appearance.languageDesc': '界面显示语言。切换后会提示重启，重启后生效。',                  // :876
  'settings.appearance.theme': '主题',                                 // :887
  'settings.appearance.themeDesc': '一共 {n} 套配色：{names}。点「选择主题…」立刻换，不用重启。',      // :888（名字由 themes/ 扫出来的那份填）
  'settings.appearance.themeCurrent': '当前：{name}',                   // :890
  'settings.appearance.pickTheme': '选择主题…',                         // :891

  'settings.tray.title': '托盘',
  'settings.tray.desc': '系统托盘区的显示与窗口关闭行为。',              // :898
  'settings.tray.minimize': '关闭窗口时最小化到托盘',                    // :900
  'settings.tray.minimizeDesc': '点击窗口关闭按钮时隐藏到系统托盘，不退出程序。只有通过托盘菜单「退出」才真正退出。',  // :901
  'settings.tray.icon': '显示托盘图标',                                 // :907
  'settings.tray.iconDesc': '在系统托盘区显示图标，右键可快速访问常用操作。',                          // :908

  'settings.startup.title': '开机自启动',
  'settings.startup.desc': '系统登录后自动拉起后台 daemon，让 GitHub 加速始终生效。',                  // :917
  'settings.startup.autoStart': '登录时自动启动',                        // :920
  'settings.startup.autoStartDesc': '在用户登录后通过计划任务（Windows）或 systemd（Linux）自动启动后台服务。',  // :921
  'settings.startup.minimized': '启动后最小化',                          // :928
  'settings.startup.minimizedDesc': '自启动时隐藏主窗口，只在托盘显示，避免每次登录都弹窗。',            // :929
  'settings.startup.autoDetect': '自动检测已选',                         // :936
  'settings.startup.autoDetectDesc': '应用启动后，自动对所有已勾选的项执行一次检测。没有勾选项时跳过本次自动化。检测过程中的焦点跟随由「检测时焦点跟随」开关决定。',  // :937-941
  'settings.startup.autoApply': '自动应用队列',                          // :951
  'settings.startup.autoApplyDesc': '检测完成后，自动把检测成功的项加入应用队列并执行。依赖「自动检测已选」，未开启检测时此项不可用。检测失败或没有可用 IP 的项会自动跳过。',  // :952-955（原型这三行里那句「依赖」加了 <b>，这里按纯文本走：强调靠字面，不靠标记）

  'settings.update.title': '检测升级',
  'settings.update.desc': '检查是否有新版本可用。检查过程不会影响正在运行的加速任务。',                  // :968
  'settings.update.current': '当前版本',                                // :971
  'settings.update.buildDate': '构建日期',                              // :972
  'settings.update.latest': '最新版本',                                 // :973
  'settings.update.check': '检查更新',                                  // :976
  'settings.update.checking': '检查中…',                                // :1657、:1671 那颗钮与那一格同字
  'settings.update.idle': '尚未检查',                                   // :977、:1668
  'settings.update.checkingNow': '正在检查…',                            // :1663
  'settings.update.downloading': '正在下载更新包…',                      // :1664、:1680
  'settings.update.ready': '更新包已就绪，重启后安装',                    // :1665
  'settings.update.found': '发现新版本 {v}',                             // :1666
  'settings.update.isLatest': '已是最新版本',                             // :1667
  'settings.update.changelog': '本次更新内容',                            // :981
  'settings.update.download': '立即更新',                                // :986
  'settings.update.downloadingBtn': '下载中…',                            // :1680
  'settings.update.relaunch': '重启安装',                                 // :1681
  'settings.update.history': '版本历史',                                  // :995
  // 版本历史那两条：原型 :997-999 摆的是 v1.0.0／v0.9.0／v0.5.0 三条编出来的历史（演示用）。
  // 真接上服务之后这一格只有一个读数是可证的——GetVersion 回的那一个；更早的历史要更新通道（M5）才有，
  // 所以这里只留两句实话。更新通道接上那天把 historyNone 换成"服务回的那份清单"。
  'settings.update.hCurrent': '本机正在运行的这一份，版本与构建日期都向后台服务问过。',
  'settings.update.historyNone': '更早的版本历史由更新通道提供，本期不提供。',
  'settings.update.toastFound': '发现新版本 {v}',                         // :1692
  'settings.update.toastLatest': '已是最新版本 {v}',                      // :1693
  'settings.update.toastDownloaded': '更新包下载完成，重启后安装',          // :1710
  'settings.update.toastRelaunch': '重启后自动安装（模拟）',                // :1697

  'settings.about.title': '关于我们',
  'settings.about.desc': '应用信息与反馈入口。',                          // :1006
  'settings.about.name': 'GitHub 加速助手',                              // :1011
  'settings.about.tagline': '让 GitHub 访问更稳定、更快速。开源、跨平台、无侵入。',   // :1012
  'settings.about.repo': '项目主页',                                     // :1014
  'settings.about.feedback': '反馈问题',                                  // :1015
  'settings.about.licenseBtn': '许可协议',                                // :1016
  'settings.about.license': 'CC BY-SA 4.0',                              // :1025
  'settings.about.versionInfo': '版本信息',                                // :1022
  'settings.about.rowCurrent': '当前版本',                                 // :1023
  'settings.about.rowBuild': '构建日期',                                   // :1024
  'settings.about.rowDataDir': '数据目录',                                  // :1026
  'settings.about.toastRepo': '打开项目主页（模拟）',                        // :1717
  'settings.about.toastFeedback': '打开反馈页面（模拟）',                    // :1718
  'settings.about.toastLicense': '打开许可协议（模拟）',                     // :1719

  /* ==== 「选择主题」那六张卡的弹窗（views/theme-picker，原型 :1039-1051） ====
     卡上的名字与简介不在这里——它们住在 styles/themes/<id>/theme.js 那一份里，卡片直接读它。 */
  'theme.title': '选择主题',                                             // :1043
  'theme.desc': '{n} 套配色，点一张立刻预览；选中的会自动记住，下次打开还是它。',       // :1044
  'theme.done': '完成'                                                    // :1049
};
