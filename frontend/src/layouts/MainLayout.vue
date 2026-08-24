<script setup lang="ts">
import {
  Activity,
  Boxes,
  Home,
  Info,
  Minus,
  Moon,
  Package,
  PencilRuler,
  ScrollText,
  Server,
  Settings,
  User,
  Square,
  Sun,
  X,
} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onMounted} from 'vue'
import {useRoute} from 'vue-router'
import {Quit, WindowMinimise, WindowToggleMaximise} from '../../wailsjs/runtime/runtime'
import dysonSphereProgramLogo from '../assets/images/dyson-sphere-program-logo.png'
import TutorialLauncher from '../components/tutorial/launcher.vue'
import {useAppVersion} from '../composables/useAppVersion'
import {usePageTutorial} from '../composables/usePageTutorial'
import {useThemeMode} from '../composables/useThemeMode'

const route = useRoute()
const message = useMessage()
const {appVersion, loadAppVersion} = useAppVersion()
const {isDarkMode, isSavingThemeMode, loadThemeMode, toggleThemeMode} = useThemeMode()
const {tryAutoStartTutorial: tryAutoStartGlobalTutorial} = usePageTutorial('global')

const navItems = [
  {label: '首页', to: '/', icon: Home},
  {label: '开服', to: '/headless', icon: Server},
  {label: '模组', to: '/mod', icon: Package},
  {label: '蓝图', to: '/blueprint-generator', icon: PencilRuler},
  {label: '资源', to: '/environment', icon: Boxes},
  {label: '个人', to: '/profile', icon: User},
  {label: '日志', to: '/log', icon: ScrollText},
  {label: '设置', to: '/settings', icon: Settings},
  {label: '关于我们', to: '/about', icon: Info},
]

const title = computed(() => route.meta.title || '戴森球盒子')
const themeToggleTitle = computed(() => (isDarkMode.value ? '切换到日间模式' : '切换到夜间模式'))

onMounted(() => {
  void loadAppVersion()
  void loadThemeMode()
  void tryAutoStartGlobalTutorial()
})

// =============== 主题切换交互 ===============
// 说明：从右上角图标按钮中心触发圆形扩散切换，主题值由 useThemeMode 负责持久化。
const handleThemeToggle = async (event: MouseEvent): Promise<void> => {
  if (isSavingThemeMode.value) {
    return
  }

  const target = event.currentTarget as HTMLElement | null
  const rect = target?.getBoundingClientRect()
  try {
    await toggleThemeMode(
      rect
        ? {
            x: rect.left + rect.width / 2,
            y: rect.top + rect.height / 2,
          }
        : undefined,
    )
  } catch (error) {
    message.error(`主题保存失败：${error instanceof Error ? error.message : String(error || '未知错误')}`)
  }
}

// =============== 窗口控制交互 ===============
// 说明：无边框窗口隐藏系统标题栏后，由右上角按钮代理最小化、最大化和关闭操作。
const minimizeWindow = (): void => {
  WindowMinimise()
}

const toggleMaximizeWindow = (): void => {
  WindowToggleMaximise()
}

const closeWindow = (): void => {
  Quit()
}
</script>

<template>
  <div class="app-shell h-screen overflow-hidden">
    <aside class="app-sidebar fixed inset-y-0 left-0 z-20 flex w-48 flex-col">
      <div class="app-sidebar-header flex h-16 items-center justify-center px-4">
        <!-- ~ 应用品牌入口 -->
        <!-- * 说明：展示戴森球计划官方字标，替代普通文字品牌名 -->
        <!-- * 交互：点击后返回首页，保留侧栏固定宽度内的可点击区域 -->
        <!-- * 依赖：依赖本地化的 Steam 官方静态 Logo 资源 -->
        <!-- * 边界：图片加载失败时通过 aria-label 保留可访问名称，不阻塞导航渲染 -->
        <RouterLink to="/" class="app-brand-link min-w-0" aria-label="戴森球计划" data-tour="main-brand">
          <img :src="dysonSphereProgramLogo" alt="" class="app-brand-logo" />
        </RouterLink>
      </div>

      <!-- ~ 主导航 -->
      <!-- * 说明：展示应用的主要功能入口 -->
      <!-- * 交互：点击后切换到对应页面，并高亮当前路由 -->
      <!-- * 依赖：依赖 navItems 与 Vue Router 自动路由 -->
      <!-- * 边界：较长导航名称会截断，避免挤压图标和侧边栏宽度 -->
      <nav class="grid gap-1 p-2" data-tour="main-nav">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="app-nav-link flex h-10 items-center gap-3 rounded border px-3 text-sm font-medium transition"
          active-class="app-nav-link-active"
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
      <div class="app-sidebar-status mt-auto p-3">
        <div class="app-status-panel rounded px-3 py-2">
          <div class="flex items-center justify-between gap-2">
            <span class="app-muted-text text-xs">运行状态</span>
            <Activity :size="15" :stroke-width="2.2" class="heartbeat-icon" />
          </div>
          <p class="app-secondary-text mt-1 text-xs">应用运行中</p>
        </div>
      </div>
    </aside>

    <section class="flex h-screen flex-col pl-48">
      <!-- ~ 顶部标题栏 -->
      <!-- * 说明：展示当前页面标题与应用说明 -->
      <!-- * 交互：右侧展示当前版本号，不承担页面操作 -->
      <!-- * 依赖：依赖路由 meta.title 生成标题 -->
      <!-- * 边界：未配置标题时回退到应用名称 -->
      <header class="app-header flex h-16 shrink-0 items-center justify-between pl-8 pr-4">
        <div class="app-drag-region min-w-0 flex-1">
          <p class="app-header-title text-sm font-semibold" data-tour="main-header-title">{{ title }}</p>
          <p class="app-muted-text text-xs">戴森球盒子</p>
        </div>
        <div class="app-window-controls flex items-center gap-2">
          <span class="app-version-pill inline-flex h-10 items-center rounded px-3 text-xs" data-tour="main-app-version">{{ appVersion }}</span>
          <TutorialLauncher />
          <!-- ~ 日夜模式切换 -->
          <!-- * 说明：在版本信息旁提供无文字图标按钮，用于切换日间和夜间模式 -->
          <!-- * 交互：点击后从按钮中心触发圆形扩散动画，并将结果写入 System/AppTheme 配置 -->
          <!-- * 依赖：依赖 useThemeMode、ConfigurationService.GetConfig/SetConfig 与 View Transitions API -->
          <!-- * 边界：动画不可用或减少动态时直接切换；保存中禁用按钮防止重复写入 -->
          <button
            type="button"
            class="theme-toggle-button"
            :title="themeToggleTitle"
            :aria-label="themeToggleTitle"
            :disabled="isSavingThemeMode"
            data-tour="main-theme-toggle"
            @click="handleThemeToggle"
          >
            <Sun v-if="isDarkMode" :size="16" :stroke-width="1.9" />
            <Moon v-else :size="16" :stroke-width="1.9" />
          </button>
          <!-- ~ 自定义窗口控制 -->
          <!-- * 说明：在无边框窗口下提供最小化、最大化和关闭按钮，替代系统标题栏按钮 -->
          <!-- * 交互：点击后调用 Wails runtime 窗口 API -->
          <!-- * 依赖：依赖 WindowMinimise、WindowToggleMaximise 与 Quit -->
          <!-- * 边界：按钮区域禁止窗口拖拽，避免点击时触发移动窗口 -->
          <div class="window-control-group flex items-center gap-1" data-tour="main-window-controls">
            <button class="window-control-button" type="button" title="最小化" aria-label="最小化" @click="minimizeWindow">
              <Minus :size="15" :stroke-width="1.9" />
            </button>
            <button class="window-control-button" type="button" title="最大化" aria-label="最大化" @click="toggleMaximizeWindow">
              <Square :size="13" :stroke-width="1.9" />
            </button>
            <button class="window-control-button is-close" type="button" title="关闭" aria-label="关闭" @click="closeWindow">
              <X :size="15" :stroke-width="1.9" />
            </button>
          </div>
        </div>
      </header>

      <main class="min-h-0 flex-1 overflow-auto">
        <slot />
      </main>
    </section>
  </div>
</template>

<style scoped lang="scss">
.app-shell {
  background:
    radial-gradient(circle at 72% 18%, var(--ds-glow-gold), transparent 26%),
    radial-gradient(circle at 24% 78%, var(--ds-glow-teal), transparent 32%),
    var(--ds-bg-shell);
  color: var(--ds-text-primary);
}

.app-sidebar {
  border-right: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-sidebar);
}

.app-sidebar-header,
.app-sidebar-status,
.app-header {
  border-bottom: 1px solid var(--ds-border-subtle);
}

.app-sidebar-status {
  border-bottom: 0;
}

.app-header {
  background: var(--ds-bg-header);
  --wails-draggable: drag;
}

.app-drag-region {
  --wails-draggable: drag;
}

.app-window-controls,
.theme-toggle-button,
.app-version-pill,
.window-control-button {
  --wails-draggable: no-drag;
}

.app-header-title {
  color: var(--ds-text-primary);
}

.app-muted-text {
  color: var(--ds-text-muted);
}

.app-brand-link {
  display: flex;
  width: 100%;
  height: 48px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.app-brand-logo {
  width: min(168px, 100%);
  height: 58px;
  object-fit: contain;
  object-position: center;
  opacity: 0.9;
  filter:
    brightness(0)
    saturate(100%)
    invert(10%)
    sepia(10%)
    saturate(426%)
    hue-rotate(201deg)
    brightness(92%)
    contrast(94%)
    drop-shadow(0 1px 1px rgb(255 255 255 / 0.42));
  transition:
    filter 180ms ease,
    opacity 180ms ease;
}

:root[data-theme='dark'] .app-brand-logo {
  opacity: 1;
  filter:
    drop-shadow(0 1px 2px rgb(0 0 0 / 0.5))
    drop-shadow(0 0 8px rgb(255 255 255 / 0.12));
}

.app-secondary-text {
  color: var(--ds-text-secondary);
}

.app-status-panel {
  border: 0;
  background: transparent;
}

.app-nav-link {
  border-color: transparent;
  color: var(--ds-nav-text);
}

.app-nav-link:hover {
  border-color: var(--ds-border-subtle);
  background: var(--ds-hover-bg);
  color: var(--ds-text-primary);
}

.app-nav-link-active {
  border-color: var(--ds-border-strong) !important;
  background: var(--ds-active-bg) !important;
  color: var(--ds-text-primary) !important;
}

.app-version-pill,
.theme-toggle-button,
.window-control-button {
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel);
  color: var(--ds-text-muted);
}

.theme-toggle-button,
.window-control-button {
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: 4px;
  transition:
    border-color 160ms ease,
    background 160ms ease,
    color 160ms ease,
    transform 160ms ease;
}

.theme-toggle-button:hover,
.window-control-button:hover {
  border-color: var(--ds-border-strong);
  background: var(--ds-hover-bg);
  color: var(--ds-brand);
}

.theme-toggle-button:active,
.window-control-button:active {
  transform: translateY(1px);
}

.theme-toggle-button:disabled {
  cursor: not-allowed;
  color: var(--ds-text-disabled);
}

.window-control-button.is-close:hover {
  border-color: rgb(232 125 99 / 0.42);
  background: rgb(232 125 99 / 0.14);
  color: var(--ds-error);
}

.heartbeat-icon {
  color: var(--ds-success);
  filter: drop-shadow(0 0 5px rgb(124 212 131 / 0.24));
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
