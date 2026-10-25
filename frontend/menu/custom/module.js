/* 「自定义 / 订阅源」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1132 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'custom',
  name: '自定义 / 订阅源',
  order: 11,
  items: [
    { id: 'cu-manual', name: '手动映射', domains: ['自定义域名'], init: 'unset' },
    { id: 'cu-gh520', name: 'GitHub520', domains: ['raw.hellogithub.com'], init: 'unset' },
    { id: 'cu-china', name: 'china-hosts', domains: ['github.com/zhouzhehong/china-hosts-accelerator'], init: 'unset' }
  ]
};
