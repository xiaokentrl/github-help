/* 「GitHub」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1061 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'github',
  name: 'GitHub',
  order: 1,
  items: [
    { id: 'gh-main', name: '主站与 API', domains: ['github.com', 'api.github.com', 'gist.github.com'], init: 'applied' },
    { id: 'gh-raw', name: 'Raw 文件', domains: ['raw.githubusercontent.com'], init: 'applied' },
    { id: 'gh-rel', name: 'Release 资产', domains: ['objects.githubusercontent.com'], init: 'stale' },
    { id: 'gh-img', name: '图片与头像', domains: ['avatars.githubusercontent.com', 'user-images.githubusercontent.com'], init: 'applied' },
    { id: 'gh-cdn', name: '静态资源 CDN', domains: ['github.githubassets.com', 'github.global.ssl.fastly.net'], init: 'applied' },
    { id: 'gh-pages', name: 'GitHub Pages', domains: ['pages.github.com'], init: 'unset' },
    { id: 'gh-gist', name: 'Gist', domains: ['gist.githubusercontent.com'], init: 'unset' },
    { id: 'gh-act', name: 'Actions', domains: ['pipelines.actions.githubusercontent.com'], init: 'unset' }
  ]
};
