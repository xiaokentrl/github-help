/* 「JetBrains」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1071 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'jetbrains',
  name: 'JetBrains',
  order: 2,
  items: [
    { id: 'jb-plug', name: '插件市场', domains: ['plugins.jetbrains.com', 'downloads.marketplace.jetbrains.com'], init: 'unset' },
    { id: 'jb-ide', name: 'IDE 下载', domains: ['download.jetbrains.com', 'download-cdn.jetbrains.com'], init: 'unset' },
    { id: 'jb-acc', name: '账号验证', domains: ['account.jetbrains.com'], init: 'unset' },
    { id: 'jb-tool', name: 'Toolbox', domains: ['toolbox.jetbrains.com'], init: 'unset' },
    { id: 'jb-res', name: '文档与资源', domains: ['resources.jetbrains.com', 'www.jetbrains.com'], init: 'unset' },
    { id: 'jb-upd', name: '更新检查', domains: ['updates.jetbrains.com'], init: 'unset' }
  ]
};
