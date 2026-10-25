/* 一次性的收数口：浏览器里的量具（tests/components/state-matrix.probe.js）把读数 POST 到这里，
   落成 /tmp/gac-matrix-raw.json，再由 pin.mjs 补上源码指纹、写进 tests/baseline/。
   为什么要这一趟：读数是浏览器量的，指纹是 node 算的，两边凑不到一个进程里；
                   让浏览器直接改仓库里的文件既不体面也不安全。
   会错在哪：它只听 127.0.0.1、只活这一次，用完不删也无妨——但它不校验来源，
             同机任何进程都能往这个文件里写，所以收到的东西要先看 kind 再落盘。
   怎么回来：`node frontend/tests/lib/sink.mjs &` → 在浏览器里 POST → 用完 kill。 */
import { createServer } from 'node:http';
import { writeFile } from 'node:fs/promises';

const out = process.argv[2] || '/tmp/gac-matrix-raw.json';
const srv = createServer((req, res) => {
  if (req.method !== 'POST') { res.writeHead(404).end(); return; }
  const chunks = [];
  req.on('data', c => chunks.push(c));
  req.on('end', async () => {
    const text = Buffer.concat(chunks).toString('utf8');
    let kind = null;
    try { kind = JSON.parse(text)?.meta?.kind || null; } catch { /* 下面按坏数据报 */ }
    res.writeHead(kind ? 200 : 400, { 'Access-Control-Allow-Origin': '*' });
    res.end(kind || 'bad');
    if (!kind) { console.log('SINK 收到的不是读数（meta.kind 缺失），已丢弃'); return; }
    await writeFile(out, text);
    console.log(`SINK 落了 ${kind}：${text.length} 字节 → ${out}`);
    srv.close();
  });
});
srv.listen(9223, '127.0.0.1', () => console.log('SINK 听着 9223，收一次就收工'));
