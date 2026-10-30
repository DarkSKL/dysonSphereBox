import {defineConfig} from 'vite'
import {fileURLToPath, URL} from 'node:url'
import VueRouter from 'unplugin-vue-router/vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import {NaiveUiResolver} from 'unplugin-vue-components/resolvers'

// 默认头像位于 frontend 外的 client/resources，开发服务器只额外允许读取该目录；
// 显式设置 allow 会覆盖默认值，因此同时保留 frontend 根目录。
const frontendDir = fileURLToPath(new URL('.', import.meta.url))
const resourcesDir = fileURLToPath(new URL('../resources', import.meta.url))

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    fs: {
      allow: [frontendDir, resourcesDir],
    },
  },
  plugins: [
    VueRouter({
      routesFolder: 'src/pages',
      dts: 'src/typed-router.d.ts',
    }),
    vue(),
    Components({
      dts: 'src/components.d.ts',
      directoryAsNamespace: true,
      resolvers: [NaiveUiResolver()],
    }),
  ]
})
