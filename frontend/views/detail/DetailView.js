/* 详情面板（views 层第三块）。干什么用：把原型 :1894-1991 那一整段 renderDetail() 变成"结构建一次、内容每次补"的渲染器——
   选中哪条域名，右边那两张卡加底下三颗钮就跟着换。
   会错在哪：① 原型这里是整块 innerHTML 重画。检测/应用跑起来时 renderDetail 会被频繁叫醒（flows 里那几处 ui.renderDetail()），
                每叫一次整块重画，#detail 的滚动位置就被甩回顶部——用户正看候选表，表跳走了。
                这里把七行指标、三颗钮、两张卡的壳留住，只 update 里面的字（8.4 那笔"单行更新"的账）。
             ② 那七行在忙和闲必须是同一批行，否则一进检测面板多一行少一行，抖一下（C4 修过的病，见 parts/MetricRow.js）。
             ③ "这一条不该出现"必须是真的不挂在 DOM 上，不是 hidden：样式表里没有 [hidden] 那条兜底规则，
                而 .btn 自己写着 display:inline-flex，用 hidden 只会留下一个空格子。
                原型这里也是干脆不生成那段模板（:1943、:1936）。
             ④ 底下那条灰字提示（applyHint）跟"点不动时那句挡路话"不是同一批句子：
                原型这里写的是「正在批量检测…」，挡路那句是「批量检测进行中，请稍候」（:2276）。
                合一套句子＝改了用户看到的原话，所以 detail.hint.* 那四句单独留着，判序也照原型 :1940（批量→队列→启动）。
   怎么回来：本文件不自己改 store，四个动作全走 main.js 注入的 handlers；
             每一句中文都从 i18n 取（tests/i18n.test.mjs 会比对键表与调用点）；
             「☰」那一格换成了 Icon('list')——"字符全换内联 SVG"是已定的那笔，
             逐屏比对时这一格会比原型那个 22px 字符小 4px（18 那一档），这是已定口径带来的差异，不是漏改。 */

import {
  state, findItem, findCategoryOfItem, canApply, canRestoreHosts
} from '../../core/store.js';
import { ItemMeta } from '../../core/meta.js';
import { busySingle } from '../../core/task/gate.js';
import { EmptyState } from '../../components/empty-state/EmptyState.js';
import { ErrorBox } from '../../components/error-box/ErrorBox.js';
import { Button } from '../../components/button/Button.js';
import { Tag } from '../../components/tag/Tag.js';
import { ProgressBar } from '../../components/progress-bar/ProgressBar.js';
import { MetricRow } from './parts/MetricRow.js';
import { el, replace } from '../../utils/dom.js';

/** 那七行指标：名字、值、类名各一处（逐字照原型 :1894-1902）。值由调用方算好，这里不拼第二种单位 */
const METRICS = [
  { key: 'detail.m.currentIp', cls: it => (it.appliedIp ? 'mono' : 'dim'), text: (it, t) => it.appliedIp || t('common.dash') },
  { key: 'detail.m.bestIp', cls: it => (it.bestIp ? 'mono' : 'dim'), text: (it, t) => it.bestIp || t('common.dash') },
  { key: 'detail.m.latency', cls: it => (it.latency ? '' : 'dim'), text: (it, t) => (it.latency ? it.latency + ' ms' : t('common.dash')) },
  { key: 'detail.m.speed', cls: it => (it.speed ? '' : 'dim'), text: (it, t) => it.speed || t('common.dash') },
  { key: 'detail.m.score', cls: it => (it.score ? '' : 'dim'), text: (it, t) => it.score || t('common.dash') },
  { key: 'detail.m.source', cls: it => (it.source ? 'm-src' : 'dim m-src'), text: (it, t) => it.source || t('common.dash') },
  { key: 'detail.m.lastCheck', cls: () => 'dim m-time', text: (it, t) => it.lastCheck || t('detail.neverChecked') }
];

/** 候选表那一行不是 4.15 那五个行级件之一（原型 :1922-1930 就是三行模板），所以留在这个文件里不另建件 */
function candRow(c, i, item, t) {
  const isCurrent = item.appliedIp === c.ip;
  const isBest = item.bestIp === c.ip;
  const score = el('td');
  score.appendChild(ProgressBar({ kind: 'score', value: c.score }).el);
  score.appendChild(document.createTextNode(String(c.score)));
  return el('tr', { class: isCurrent ? 'current' : (i === 0 ? 'best' : '') }, [
    el('td', { text: String(i + 1) }),
    el('td', { class: 'mono', text: c.ip }),
    el('td', { class: 'tags-cell' }, isBest ? Tag({ text: t('detail.best'), tone: 'green', size: 'legacy' }).el : []),
    el('td', { text: `${c.latency} ms` }),
    el('td', { text: `${c.speed} MB/s` }),
    score
  ]);
}

/**
 * @param {object} d
 * @param {HTMLElement} d.root                     #detail
 * @param {(key:string,params?:object)=>string} d.t
 * @param {object} d.handlers  { onDetectOne, onApplyOne, onViewLog, onRestoreHosts }（都收当前条目对象）
 */
export function createDetailView({ root, t, handlers }) {
  /* ---- 壳：建一次，之后只挪动和改字 ---- */
  const empty = EmptyState({
    variant: 'panel', icon: 'list',
    title: t('detail.emptyTitle'), hint: t('detail.emptyHint'), action: null
  });

  const crumbName = document.createTextNode('');
  const crumb = el('span', { class: 'crumb' });
  const badge = el('span', { class: 'badge unset' });
  const restoreBtn = Button({
    label: t('detail.restoreHosts'), size: 'sm', kind: 'restore',
    title: t('detail.restoreHostsTitle'), onClick: () => handlers.onRestoreHosts(current)
  });
  const head1 = el('div', { class: 'card-head' });
  const metricRows = METRICS.map(() => MetricRow());
  const metricsBox = el('div', { class: 'metrics' }, metricRows.map(r => r.el));
  const domainsLine = el('div', { class: 'domains-line' });
  const errBox = ErrorBox({ text: '', kind: 'error', retry: null });
  const card1 = el('div', { class: 'card' });

  const candCountBadge = el('span', { class: 'badge unset' });
  const tbody = el('tbody');
  const card2 = el('div', { class: 'card' }, [
    el('div', { class: 'card-head' }, [
      el('h3', { text: t('detail.cands') }), el('div', { class: 'spacer' }), candCountBadge
    ]),
    el('table', { class: 'cands' }, [
      el('thead', {}, el('tr', {}, [
        el('th', { text: '#' }),
        el('th', { text: t('detail.colIp') }),
        el('th', { text: t('detail.colTag') }),
        el('th', { text: t('detail.colLatency') }),
        el('th', { text: t('detail.colSpeed') }),
        el('th', { text: t('detail.colScore') })
      ])),
      tbody
    ])
  ]);

  const detectBtn = Button({ label: t('detail.detectOne'), onClick: () => handlers.onDetectOne(current) });
  const applyBtn = Button({ label: t('detail.applyOne'), kind: 'primary', onClick: () => handlers.onApplyOne(current) });
  const logBtn = Button({ label: t('detail.viewLog'), kind: 'ghost', onClick: () => handlers.onViewLog(current) });
  const hintEl = el('span', { class: 'hint' });
  const actions = el('div', { class: 'actions' });

  let shown = null;     // root 上现在挂着哪一套：'empty' 还是 'detail'
  let current = null;   // 三颗钮建时就绑在这个变量上，所以按下去拿到的永远是画着的那条

  function showShell(which) {
    if (shown === which) return;
    shown = which;
    replace(root, which === 'empty' ? empty.el : [card1, card2, actions]);
  }

  /** 底下那条灰字（原型 :1938-1941）。判序与那三句挡路话不同，所以不借 gate 的句子，只借 gate 的判据 */
  function hintText(item, canDetect) {
    if (!canApply(item)) return t('detail.hint.needDetect');
    if (canDetect) return '';
    if (state.batchDetecting) return t('detail.hint.batch');
    if (state.queueRunning) return t('detail.hint.queue');
    if (state.startupAutomationRunning) return t('detail.hint.startup');
    return '';
  }

  function render() {
    const item = state.selectedId ? findItem(state.selectedId) : null;
    current = item;

    if (!item) {
      showShell('empty');
      empty.update({ title: t('detail.emptyTitle'), hint: t('detail.emptyHint') });
      return;
    }
    showShell('detail');

    const cat = findCategoryOfItem(item.id);
    const meta = ItemMeta[item.status];
    if (!meta) throw new Error(`条目 ${item.id} 的状态「${item.status}」在 core/meta.js 的 ItemMeta 里没有——详情那块徽标不知道画什么`);
    const isBusy = item.status === 'detecting' || item.status === 'applying';

    crumbName.textContent = cat ? cat.name : '';
    crumb.textContent = `/ ${item.name}`;
    badge.className = 'badge ' + meta.badge;
    badge.textContent = meta.text;
    replace(head1, [
      el('h3', {}, [crumbName, crumb]),
      el('div', { class: 'spacer' }),
      badge,
      canRestoreHosts(item) && item.appliedIp !== null ? restoreBtn.el : null
    ]);

    metricRows.forEach((row, i) => {
      const m = METRICS[i];
      row.update({ label: t(m.key), value: m.text(item, t), cls: m.cls(item), busy: isBusy });
    });

    replace(domainsLine, [
      el('span', { class: 'lbl', text: t('detail.domains') }),
      ...item.domains.map(d => el('span', { class: 'chip', text: d }))
    ]);

    errBox.update({ text: item.errorMsg || '', kind: item.bestIp ? 'warn' : 'error' });
    replace(card1, [head1, metricsBox, domainsLine, item.errorMsg ? errBox.el : null]);

    candCountBadge.textContent = t('detail.candCount', { n: item.candidates.length });
    replace(tbody, item.candidates.length
      ? item.candidates.map((c, i) => candRow(c, i, item, t))
      : el('tr', {}, el('td', { class: 'cand-empty', attrs: { colspan: '6' }, text: t('detail.candEmpty') })));

    // "三件事有没有在跑"只问 gate 一处（C2 说的就是别在这儿再抄一遍条件）；条目自己忙不在 gate 里
    const canDetect = !isBusy && busySingle(state).ok;
    detectBtn.setDisabled(!canDetect);
    applyBtn.setDisabled(!(canDetect && canApply(item)));
    const hint = hintText(item, canDetect);
    hintEl.textContent = hint;
    replace(actions, [detectBtn.el, applyBtn.el, logBtn.el, hint ? hintEl : null]);
  }

  render();
  return { root, render };
}
