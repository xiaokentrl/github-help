/* 「Coursera」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1123 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'coursera',
  name: 'Coursera',
  order: 9,
  items: [
    { id: 'co-video', name: '视频 CDN', domains: ['d3c33hcgiwev3.cloudfront.net'], init: 'unset' },
    { id: 'co-main', name: '主站资源', domains: ['d1a3f4spazzrp4.cloudfront.net'], init: 'unset' }
  ]
};
