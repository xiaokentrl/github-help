/* 「游戏平台」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1094 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'gameplat',
  name: '游戏平台',
  order: 5,
  items: [
    { id: 'gp-epic', name: 'Epic Games', domains: ['download.epicgames.com'], init: 'unset' },
    { id: 'gp-ea', name: 'EA / Origin', domains: ['origin.com', 'akamai.cdn.ea.com'], init: 'unset' },
    { id: 'gp-bnet', name: 'Battle.net', domains: ['battle.net'], init: 'unset' },
    { id: 'gp-ubi', name: 'Ubisoft Connect', domains: ['ubisoft.com', 'uplay.com'], init: 'unset' },
    { id: 'gp-riot', name: 'Riot Games', domains: ['riotgames.com'], init: 'unset' },
    { id: 'gp-gog', name: 'GOG', domains: ['gog.com'], init: 'unset' }
  ]
};
