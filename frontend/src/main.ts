import { createApp } from 'vue'
import {createPinia} from 'pinia'
import App from './App.vue'
import { router } from './router'
import {useAccountStore} from './stores/account'
import './style.scss'

// =============== 应用引导 ===============
// 说明：只负责创建 Vue 实例、挂载路由并加载全局样式。
const app = createApp(App)
const pinia = createPinia()

app.use(pinia).use(router)

// 在首屏渲染前从 SQLite 恢复可展示会话，避免个人页先渲染空登录态再切换资料页。
void useAccountStore(pinia).restoreSession().finally(() => app.mount('#app'))
