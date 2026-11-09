import {defineConfig} from 'vite'
import {fileURLToPath, URL} from 'node:url'
import VueRouter from 'unplugin-vue-router/vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import {NaiveUiResolver} from 'unplugin-vue-components/resolvers'
import fs from 'node:fs'
import path from 'node:path'

// 默认头像位于 frontend 外的 client/resources，开发服务器只额外允许读取该目录；
// 显式设置 allow 会覆盖默认值，因此同时保留 frontend 根目录。
const frontendDir = fileURLToPath(new URL('.', import.meta.url))
const resourcesDir = fileURLToPath(new URL('../resources', import.meta.url))

/** 一个简单的 Vite 插件，在构建时复制蓝图预览库的外部资产 */
function copyBlueprintEditorAssets() {
  return {
    name: 'copy-dsp-blueprint-editor',
    apply: 'build', // 仅在构建时生效
    closeBundle() {
      // closeBundle 钩子在打包完成后触发。
      // 预览库 ESM 产物会随宿主打包进 dist/assets，并按 import.meta.url 定位同目录资产，
      // 因此把 basis/、img/ 复制到 dist/assets 下。
      const src = path.resolve(frontendDir, 'src/packages/dsp-blueprint-editor')
      const dest = path.resolve(frontendDir, 'dist/assets')
      fs.mkdirSync(dest, { recursive: true })
      for (const dir of ['basis', 'img']) {
        const from = path.join(src, dir)
        if (!fs.existsSync(from)) {
          console.warn(`⚠️ ${dir} 目录不存在，跳过复制`)
          continue
        }
        fs.cpSync(from, path.join(dest, dir), { recursive: true })
      }
      console.log('✅ 蓝图预览资产已复制到 dist/assets 目录')
    }
  }
}

// https://vitejs.dev/config/
export default defineConfig({
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
    copyBlueprintEditorAssets(),
  ],
  server: {
    fs: {
      allow: [frontendDir, resourcesDir],
    },
  },
  resolve: {
    alias: {
      // 开发环境下，将 models/、img/、basis/ 映射到 dsp-blueprint-editor 目录下
      '/models': path.resolve(__dirname, 'src/packages/dsp-blueprint-editor/models')
    }
  }
})
