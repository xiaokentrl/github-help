/* 设置项的唯一登记处：默认值、上下限、读写。干什么用：界面里那 13 个设置键只在这里登记一遍，
   面板读它、main.js 应用它、重启后还是它——面板自己不许留副本。
   两份存法是有分工的：真家在服务那一份文件里（契约 GetSettings／SaveSettings，main.js 开机取一次、改动之后推一次），
   localStorage 这一份只是"服务还没答话之前先画对"的缓存——「开机自动检测」这类开关只有进了服务那份文件才作数。
   会错在哪：默认值在面板里写一份、这里写一份，改一处忘另一处，就会出现"说明写 240、打开是 340"。
             存档里翻到已经不认的值（旧主题的"深色""跟随系统"就是这种），不能崩也不能瞎用。
   怎么回来：不在 DEFAULTS 里的键一律不认（load 时逐项过滤）；主题 id 交给 styles/themes 扫出来的名单校，
             认不出就回落 light，这一条写在 settings.test.mjs 里。存储用 localStorage，键名跟原型一致。 */

export const SETTINGS_KEY = 'gh-accel-settings-v1';

export const DEFAULTS = {
  sidebarWidth: 240,
  queueHeight: 252,
  zoom: 1,
  language: 'zh-CN',
  theme: 'light',
  trayMinimize: true,
  showTrayIcon: true,
  autoStart: true,
  startMinimized: false,
  detectFocusFollow: false,
  applyFocusFollow: false,
  // 开机自动化默认关〔已定：2026-10-25 用户拍板〕——装完第一次开窗就自动写 hosts
  // 属"程序自作主张动系统文件"，得用户显式开。已装用户的那份 settings.json 不受影响
  // （normalize 只信文件里真写了的键，这里只管"第一次"）。
  autoDetectOnStartup: false,
  autoApplyOnStartup: false
};

export const SIDEBAR_MIN_W = 200;
export const SIDEBAR_MAX_W = 600;
export const QUEUE_MIN_H = 120;

export const SIDEBAR_PRESETS = [240, 340, 480];
export const QUEUE_PRESETS = [180, 252, 360];
export const ZOOM_PRESETS = [0.9, 1, 1.1, 1.25];

const BOOL_KEYS = ['trayMinimize', 'showTrayIcon', 'autoStart', 'startMinimized',
  'detectFocusFollow', 'applyFocusFollow', 'autoDetectOnStartup', 'autoApplyOnStartup'];

/** 这四个键今天只有"记下来"这一半：真要动的是窗口与开机项，那归界面壳（cmd/gui），它还没接。
    干什么用：告诉保存那一句"这话不能说满"——勾了「关闭窗口时最小化到托盘」不等于窗口会藏起来。
    会错在哪：这一份名单与界面壳之间必须同轮改。壳接上那天把对应的那一条从这里删掉，
             「外观／托盘／开机」那几格的说辞就自动不再打折扣；留着就是界面在说已经做不到的话。
    怎么回来：tests/settings.test.mjs 两头都查——这四个键除了面板与这份名单之外别处不许有人读（有人读了＝壳接上了却没删），
             名单本身也必须一直是这四个。 */
export const SHELL_KEYS = ['trayMinimize', 'showTrayIcon', 'autoStart', 'startMinimized'];

function clampInt(v, lo, hi, fallback) {
  const n = Math.round(Number(v));
  if (!Number.isFinite(n)) return fallback;
  return Math.min(hi, Math.max(lo, n));
}

/** 把任意来源（存档、面板控件）的一组值整理成合法设置；纯函数，能在 node 里直接测 */
export function normalize(raw, themeIds) {
  const out = { ...DEFAULTS };
  if (!raw || typeof raw !== 'object') return out;

  out.sidebarWidth = clampInt(raw.sidebarWidth, SIDEBAR_MIN_W, SIDEBAR_MAX_W, DEFAULTS.sidebarWidth);
  out.queueHeight = clampInt(raw.queueHeight, QUEUE_MIN_H, 2000, DEFAULTS.queueHeight);

  const z = Number(raw.zoom);
  out.zoom = [0.9, 1, 1.1, 1.25].includes(z) ? z : DEFAULTS.zoom;

  if (typeof raw.language === 'string' && raw.language) out.language = raw.language;
  if (themeIds && themeIds.length) {
    if (themeIds.includes(raw.theme)) out.theme = raw.theme;
  } else if (typeof raw.theme === 'string' && raw.theme) {
    out.theme = raw.theme;
  }
  for (const k of BOOL_KEYS) {
    if (typeof raw[k] === 'boolean') out[k] = raw[k];
  }
  // 「自动应用队列」依赖「自动检测已选」：检测没开，应用这个勾必须落回关闭（原型 :1522 那段联动）
  if (!out.autoDetectOnStartup) out.autoApplyOnStartup = false;
  return out;
}

export function loadSettings(themeIds, storage) {
  const store = storage || globalThis.localStorage;
  try {
    const raw = store && store.getItem(SETTINGS_KEY);
    return normalize(raw ? JSON.parse(raw) : null, themeIds);
  } catch {
    return normalize(null, themeIds);
  }
}

export function saveSettings(settings, storage) {
  const store = storage || globalThis.localStorage;
  try {
    if (store) store.setItem(SETTINGS_KEY, JSON.stringify(settings));
  } catch {
    // 写不进去（隐私模式、配额满）只影响下次启动的记忆，不该打断当前这次操作
  }
}

export function queueMaxHeight(innerHeight) {
  return Math.min(innerHeight * 0.7, innerHeight - 200);
}
