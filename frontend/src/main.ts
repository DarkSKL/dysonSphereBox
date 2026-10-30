import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './style.scss'
import { installDiagnostics } from './services/diagnostics'
import { bootstrapAccountSession } from './services/account'

// =============== 应用引导 ===============
// 说明：创建 Vue 实例、挂载路由并立即启动账号会话引导；引导内部会等待 Go 后端 startup 完成。
// 边界：账号同步失败不阻塞页面渲染，账号入口仍可手动重试。
const app = createApp(App)
const disposeDiagnostics = installDiagnostics(app)
app.use(router).mount('#app')
void bootstrapAccountSession()
if (import.meta.hot) import.meta.hot.dispose(disposeDiagnostics)
