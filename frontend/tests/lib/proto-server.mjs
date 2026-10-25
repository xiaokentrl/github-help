/* 只读地把仓库根那一份原型 HTML 端起来，给界面量具当"并排的对照物"。
   干什么用：界面逐屏要跟原型对，量具得能在同一个页面里读到原型那棵树。
             这份服务不复制原型、不落第二份文件——浏览器只用 fetch 把文本取过来，
             再塞进 srcdoc  iframe（srcdoc 的文档跟父页面同源，量具才读得到里面的节点）。
   会错在哪：① 它把仓库根整个目录都端出来了，同机任何进程都能翻你的别的文件——所以只认那一个 URL 路径，
                其余一律 404，且只听 127.0.0.1；② 它带 CORS（不然父页面 fetch 不到），
                意思是"本机任何人都能读这一份原型"，这是本机的假定为可接受的那一半；
             ③ 用完不 kill 就一直在，下一个读的人以为原型长在 9226 上。
   怎么回来：量那一趟临时起、量完 kill：
             `node frontend/tests/lib/proto-server.mjs &` → 浏览器里 fetch http://127.0.0.1:9226/前端原型设计稿唯一真实来源.html →
             用完 kill。真身路径与端口都只在这里写一次。 */
import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..', '..');
const NAME = '前端原型设计稿唯一真实来源.html';
const PORT = 9226;

const srv = createServer(async (req, res) => {
  const p = decodeURIComponent((req.url || '/').split('?')[0]);
  if (req.method === 'OPTIONS') {
    res.writeHead(204, { 'Access-Control-Allow-Origin': '*', 'Access-Control-Allow-Methods': 'GET' }).end();
    return;
  }
  if (p !== `/${NAME}`) { res.writeHead(404).end('only the prototype'); return; }
  const body = await readFile(path.join(ROOT, NAME));
  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8', 'Access-Control-Allow-Origin': '*' }).end(body);
});

srv.listen(PORT, '127.0.0.1', () => console.log(`原型只读端口 ${PORT}：只端 /${NAME}`));
