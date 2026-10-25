/* 「微软 / Xbox」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1117 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'ms',
  name: '微软 / Xbox',
  order: 8,
  items: [
    { id: 'ms-store', name: 'Microsoft Store', domains: ['storeedgefd.dsx.mp.microsoft.com'], init: 'unset' },
    { id: 'ms-xbox', name: 'Xbox Live', domains: ['xboxlive.com'], init: 'unset' },
    { id: 'ms-winup', name: 'Windows Update', domains: ['update.microsoft.com'], init: 'unset' },
    { id: 'ms-office', name: 'Office', domains: ['officeapps.live.com'], init: 'unset' }
  ]
};
