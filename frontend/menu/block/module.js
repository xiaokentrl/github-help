/* 「去广告与屏蔽」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1127 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'block',
  name: '去广告与屏蔽',
  order: 10,
  items: [
    { id: 'bl-antiad', name: 'anti-AD', domains: ['anti-ad.net'], init: 'unset' },
    { id: 'bl-adrules', name: 'AdRules', domains: ['adrules.top'], init: 'unset' },
    { id: 'bl-steven', name: 'StevenBlack', domains: ['raw.githubusercontent.com/StevenBlack/hosts'], init: 'unset' }
  ]
};
