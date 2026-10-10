import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './style.scss'
import { installDiagnostics } from './services/diagnostics'

// =============== 应用引导 ===============
// 说明：只负责创建 Vue 实例、挂载路由并加载全局样式。
const app = createApp(App)
const disposeDiagnostics = installDiagnostics(app, router)
app.use(router).mount('#app')
if (import.meta.hot) import.meta.hot.dispose(disposeDiagnostics)
