/* 「公共 CDN」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1110 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'cdn',
  name: '公共 CDN',
  order: 7,
  items: [
    { id: 'cdn-jsdelivr', name: 'jsDelivr', domains: ['cdn.jsdelivr.net'], init: 'unset' },
    { id: 'cdn-unpkg', name: 'unpkg', domains: ['unpkg.com'], init: 'unset' },
    { id: 'cdn-cdnjs', name: 'cdnjs', domains: ['cdnjs.cloudflare.com'], init: 'unset' },
    { id: 'cdn-gfonts', name: 'Google Fonts', domains: ['fonts.googleapis.com', 'fonts.gstatic.com'], init: 'unset' },
    { id: 'cdn-bootstrap', name: 'Bootstrap', domains: ['stackpath.bootstrapcdn.com'], init: 'unset' }
  ]
};
