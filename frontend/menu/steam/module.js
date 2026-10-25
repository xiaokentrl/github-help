/* 「Steam」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1087 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'steam',
  name: 'Steam',
  order: 4,
  items: [
    { id: 'st-store', name: '商店', domains: ['store.steampowered.com'], init: 'applied' },
    { id: 'st-comm', name: '社区', domains: ['steamcommunity.com'], init: 'error' },
    { id: 'st-cdn', name: 'CDN', domains: ['cdn.steampowered.com'], init: 'applied' },
    { id: 'st-dl', name: '下载节点', domains: ['ext1-hkg1.steamserver.net'], init: 'stale' },
    { id: 'st-img', name: '图片与头像', domains: ['avatars.steamstatic.com'], init: 'unset' }
  ]
};
