import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  root: 'design/mod-market',
  plugins: [vue()],
  server: { host: '127.0.0.1', port: 5184, strictPort: true },
  build: { outDir: '../../dist-design/mod-market', emptyOutDir: true },
})
