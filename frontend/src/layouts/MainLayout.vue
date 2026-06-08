<script setup lang="ts">
import {Activity, Home, Info, Package, ScrollText, Server, Settings} from '@lucide/vue'
import {computed, onMounted} from 'vue'
import {useRoute} from 'vue-router'
import {useAppVersion} from '../composables/useAppVersion'

const route = useRoute()
const {appVersion, loadAppVersion} = useAppVersion()

const navItems = [
  {label: '首页', to: '/', icon: Home},
  {label: '开服', to: '/headless', icon: Server},
  {label: 'Mod', to: '/mod', icon: Package},
  {label: '日志', to: '/log', icon: ScrollText},
  {label: '设置', to: '/settings', icon: Settings},
  {label: '关于我们', to: '/about', icon: Info},
]

const title = computed(() => route.meta.title || 'DysonSphereBox')

onMounted(() => {
  void loadAppVersion()
})
</script>

<template>
  <div class="h-screen overflow-hidden bg-[#fbfbfc] text-zinc-950">
    <aside class="fixed inset-y-0 left-0 z-20 flex w-48 flex-col border-r border-zinc-200 bg-[#f7f7f8]">
      <div class="flex h-16 items-center justify-between border-b border-zinc-200 px-4">
        <RouterLink to="/" class="min-w-0">
          <p class="truncate text-lg font-semibold text-zinc-950">戴森工坊</p>
          <p class="text-[11px] text-zinc-400">戴森球计划 Mod 工具箱</p>
        </RouterLink>
      </div>

      <!-- ~ 主导航 -->
      <!-- * 说明：展示应用的主要功能入口 -->
      <!-- * 交互：点击后切换到对应页面，并高亮当前路由 -->
      <!-- * 依赖：依赖 navItems 与 Vue Router 自动路由 -->
      <!-- * 边界：较长导航名称会截断，避免挤压图标和侧边栏宽度 -->
      <nav class="grid gap-1 p-2">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="flex h-10 items-center gap-3 rounded border border-transparent px-3 text-sm font-medium text-zinc-700 transition hover:border-zinc-200 hover:bg-white hover:text-zinc-950"
          active-class="border-blue-200 bg-white text-blue-700 shadow-sm hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700"
        >
          <component :is="item.icon" :size="16" :stroke-width="1.8" class="shrink-0" />
          <span class="truncate">{{ item.label }}</span>
        </RouterLink>
      </nav>

      <!-- ~ 应用心跳状态 -->
      <!-- * 说明：用绿色心跳点提示桌面 UI 当前已正常渲染 -->
      <!-- * 交互：该区域只做状态展示，不承载操作入口 -->
      <!-- * 依赖：依赖前端运行时渲染，不连接后端健康检查接口 -->
      <!-- * 边界：这不是服务器探活结果，只表示本地界面正在运行 -->
      <div class="mt-auto border-t border-zinc-200 p-3">
        <div class="rounded border border-zinc-200 bg-white px-3 py-2">
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs text-zinc-500">运行状态</span>
            <Activity :size="15" :stroke-width="2.2" class="heartbeat-icon text-emerald-500" />
          </div>
          <p class="mt-1 text-xs text-zinc-800">应用运行中</p>
        </div>
      </div>
    </aside>

    <section class="flex h-screen flex-col pl-48">
      <!-- ~ 顶部标题栏 -->
      <!-- * 说明：展示当前页面标题与应用说明 -->
      <!-- * 交互：右侧展示当前版本号，不承担页面操作 -->
      <!-- * 依赖：依赖路由 meta.title 生成标题 -->
      <!-- * 边界：未配置标题时回退到应用名称 -->
      <header class="flex h-16 shrink-0 items-center justify-between border-b border-zinc-200 bg-white px-8">
        <div>
          <p class="text-sm font-semibold text-zinc-950">{{ title }}</p>
          <p class="text-xs text-zinc-500">戴森球计划 Mod 管理工具</p>
        </div>
        <div class="flex items-center gap-2">
          <span class="rounded border border-zinc-200 bg-zinc-50 px-2 py-1 text-xs text-zinc-500">{{ appVersion }}</span>
        </div>
      </header>

      <main class="min-h-0 flex-1 overflow-auto">
        <slot />
      </main>
    </section>
  </div>
</template>

<style scoped lang="scss">
.heartbeat-icon {
  filter: drop-shadow(0 0 5px rgb(16 185 129 / 0.24));
}

.heartbeat-icon :deep(path) {
  animation: heartbeat-trace 1.35s linear infinite;
  stroke-dasharray: 12 18;
}

@keyframes heartbeat-trace {
  from {
    stroke-dashoffset: 0;
  }

  to {
    stroke-dashoffset: 60;
  }
}
</style>
