import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './style.scss'

// =============== 应用引导 ===============
// 说明：只负责创建 Vue 实例、挂载路由并加载全局样式。
createApp(App).use(router).mount('#app')
