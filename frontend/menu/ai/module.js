/* 「AI 开发工具」这块菜单的条目清单：菜单叫什么、排第几、里面哪几条二级条目（域名和初态逐字照原型）。
   改这块菜单只动这个文件夹。怕的是手抄错一个字母、界面就跟原型对不上，
   所以 frontend/tests/structure.test.mjs 每次现读原型 :1079 那一段逐字比，抄漏一条当场红。 */
export default {
  id: 'ai',
  name: 'AI 开发工具',
  order: 3,
  items: [
    { id: 'ai-cursor', name: 'Cursor', domains: ['cursor.com', 'cursorapi.com', 'cursor-cdn.com'], init: 'unset' },
    { id: 'ai-windsurf', name: 'Windsurf', domains: ['windsurf.com', 'codeium.com'], init: 'unset' },
    { id: 'ai-copilot', name: 'GitHub Copilot', domains: ['api.individual.githubcopilot.com'], init: 'unset' },
    { id: 'ai-trae', name: 'Trae', domains: ['trae.ai', 'trae-api.com'], init: 'unset' },
    { id: 'ai-gemini', name: 'Gemini', domains: ['daily-cloudcode-pa.googleapis.com'], init: 'unset' },
    { id: 'ai-claude', name: 'Claude / MCP', domains: ['api.anthropic.com'], init: 'unset' }
  ]
};
