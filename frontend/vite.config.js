/* 前端的构建入口。干什么用：把 frontend/ 这一份打进 dist/，Wails 那侧（cmd/gui）嵌入的就是这个目录；
   `npm run dev` 起开发服务器，`npm test` 那条完全不走这里（测试用 node --test 直接跑源码）。
   会错在哪：① 端口不固定会让 Wails 的开发模式连不上——它按这个端口去取热更新的地址；
             ② outDir 指到 frontend 外面就会跟 cmd/gui 的 embed 路径错位，表现是"改了代码界面不动"，
                而不是报错。
   怎么回来：这两处各有一个人能一眼看住的数：port 与 outDir。改它们要同时看 cmd/gui 那边读的是哪个目录。 */
import { defineConfig } from 'vite';

export default defineConfig({
  // 根目录就是 frontend/：index.html 在这儿，源码里的相对路径也都以它为基准
  clearScreen: false,
  server: {
    port: 9222,          // Wails 开发模式固定来取这个端口，换端口要同时换 cmd/gui 里的地址
    strictPort: true,    // 被占用就直接失败，不许悄悄退到 9223（那样"改了没反应"最难查）
    watch: { ignored: ['**/dist/**', '**/tests/**', '**/*.test.mjs'] }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: true      // 出问题时能在 webview 控制台看到是哪个源文件；M1 之前这份只有前端人在读
  }
});
