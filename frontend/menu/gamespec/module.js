/* 「特定游戏专项」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1102 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'gamespec',
  name: '特定游戏专项',
  order: 6,
  items: [
    { id: 'gs-pubg', name: '绝地求生', domains: ['pubg.com'], init: 'unset' },
    { id: 'gs-bf', name: '战地系列', domains: ['battlefield.com', 'ea.com'], init: 'unset' },
    { id: 'gs-r6', name: '彩虹六号', domains: ['ubisoft.com'], init: 'unset' },
    { id: 'gs-ff14', name: 'FF14 国际版', domains: ['finalfantasyxiv.com'], init: 'unset' },
    { id: 'gs-tark', name: '逃离塔科夫', domains: ['escapefromtarkov.com'], init: 'unset' },
    { id: 'gs-gta', name: 'GTA V', domains: ['rockstargames.com'], init: 'unset' }
  ]
};
