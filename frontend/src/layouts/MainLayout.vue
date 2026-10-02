<script setup lang="ts">
import {
  List,
  Minus,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Square,
  Sun,
  UserRound,
  X,
} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {useRoute} from 'vue-router'
import {Quit, WindowMinimise, WindowToggleMaximise} from '../../wailsjs/runtime/runtime'
import {useAppVersion} from '../composables/useAppVersion'
import {usePageTutorial} from '../composables/usePageTutorial'
import {useThemeMode} from '../composables/useThemeMode'
import AccountModal from '../components/layout/account-modal.vue'
import {gameNavigationRegistry, globalNavigationEntries} from '../navigation/registry'
import {accountIsAuthenticated, accountUser, restoreAccountSession} from '../services/account'

const route = useRoute()
const message = useMessage()
const isGlobalMenuOpen = ref(false)
const isAccountModalOpen = ref(false)
const isAccountAuthenticated = accountIsAuthenticated
const accountDisplayName = computed(() => accountUser.value?.nickName || accountUser.value?.userName || '已登录')
const accountEntryTitle = computed(() => isAccountAuthenticated.value ? accountDisplayName.value : '登录或注册账号')
const globalMenuTrigger = ref<HTMLButtonElement | null>(null)
const globalMenuRoot = ref<HTMLDivElement | null>(null)
const {appVersion, loadAppVersion} = useAppVersion()
const {isDarkMode, isSavingThemeMode, loadThemeMode, toggleThemeMode} = useThemeMode()
const {tryAutoStartTutorial: tryAutoStartGlobalTutorial} = usePageTutorial('global')

// =============== 当前游戏上下文 ===============
// 说明：根据当前路由解析游戏，第二层菜单只读取该游戏注册的数据；全局页面不挂载游戏菜单。
const currentGame = computed(() => (
  gameNavigationRegistry.find((game) => route.path === game.to || route.path.startsWith(`${game.to}/`))
))
const currentGameNavigation = computed(() => currentGame.value?.navigation ?? [])
const activeNavIndex = computed(() => currentGameNavigation.value.findIndex((item) => (
  route.path === item.to || (item.to !== currentGame.value?.to && route.path.startsWith(`${item.to}/`))
)))

// =============== 游戏选项收起状态 ===============
// 说明：收起后只保留图标，状态保存在本地存储中，重启应用后沿用上次选择。
const sidebarCollapsedStorageKey = 'dsbox:sidebar-collapsed'
const readSidebarCollapsed = (): boolean => {
  try {
    return localStorage.getItem(sidebarCollapsedStorageKey) === '1'
  } catch {
    return false
  }
}
const isSidebarCollapsed = ref(readSidebarCollapsed())
const sidebarToggleTitle = computed(() => (isSidebarCollapsed.value ? '展开侧边栏' : '收起侧边栏'))

const toggleSidebarCollapsed = (): void => {
  isSidebarCollapsed.value = !isSidebarCollapsed.value
  try {
    localStorage.setItem(sidebarCollapsedStorageKey, isSidebarCollapsed.value ? '1' : '0')
  } catch {
    // 本地存储不可用时仅保留本次会话状态。
  }
}

// =============== 全局菜单交互 ===============
// 说明：全局入口由底部列表按钮向上展开为一列快捷按钮；点击外部、切换路由或按 Escape 时收起，Escape 将焦点还给触发按钮。
const isGlobalRouteActive = computed(() => globalNavigationEntries.some((item) => route.path === item.to))

const toggleGlobalMenu = (): void => {
  isGlobalMenuOpen.value = !isGlobalMenuOpen.value
}

const closeGlobalMenu = (restoreFocus = false): void => {
  isGlobalMenuOpen.value = false
  if (restoreFocus) {
    globalMenuTrigger.value?.focus()
  }
}

const handleDocumentPointerDown = (event: PointerEvent): void => {
  if (isGlobalMenuOpen.value && !globalMenuRoot.value?.contains(event.target as Node)) {
    closeGlobalMenu()
  }
}

watch(() => route.fullPath, () => closeGlobalMenu())

onMounted(() => {
  document.addEventListener('pointerdown', handleDocumentPointerDown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handleDocumentPointerDown)
})

// =============== 用户入口 ===============
// 说明：打开账号弹窗；账号状态与提交逻辑由独立组件管理。
const handleUserEntry = (): void => {
  isAccountModalOpen.value = true
}

const title = computed(() => route.meta.title || '戴森球盒子')
const themeToggleTitle = computed(() => (isDarkMode.value ? '切换到日间模式' : '切换到夜间模式'))

onMounted(() => {
  void loadAppVersion()
  void restoreAccountSession()
  void loadThemeMode()
  void tryAutoStartGlobalTutorial()
})

// =============== 主题切换交互 ===============
// 说明：从主题按钮中心触发圆形扩散切换，主题值由 useThemeMode 负责持久化。
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
    message.error('主题保存失败：' + (error instanceof Error ? error.message : String(error || '未知错误')))
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
  <div class="app-shell" :class="{'is-global-context': !currentGame, 'is-sidebar-collapsed': isSidebarCollapsed}">
    <!-- ~ 动态宇宙背景 -->
    <!-- * 说明：两层漂移星云 + 平移星流 + 闪烁星点，配色与首页戴森球场景背景对齐 -->
    <!-- * 交互：纯装饰层，不响应指针事件，不进入可访问性树 -->
    <!-- * 依赖：依赖 style.scss 中的 --ds-cosmos-* 与 --ds-bg-twinkle Token -->
    <!-- * 边界：只使用 CSS 合成动画；首页场景为不透明画布，会覆盖内容区内的这一层 -->
    <div class="app-cosmos" aria-hidden="true">
      <div class="app-cosmos-nebula is-a" />
      <div class="app-cosmos-nebula is-b" />
      <div class="app-cosmos-stars" />
      <div class="app-cosmos-twinkle" />
    </div>

    <!-- ~ 快捷入口栏 -->
    <!-- * 说明：快捷栏承载用户、游戏切换、全局页面和全局设置入口 -->
    <!-- * 交互：点击游戏标签切换到对应首页；全局页面不占用游戏第二层菜单 -->
    <!-- * 依赖：依赖游戏注册表、全局导航入口、Vue Router 与 Lucide 图标 -->
    <!-- * 边界：会话信息不可用时头像仍可打开登录弹窗；当前仅支持戴森球计划 -->
    <aside class="app-quick-rail" aria-label="快捷入口">
      <button
        type="button"
        class="app-rail-avatar"
        :title="accountEntryTitle"
        :aria-label="accountEntryTitle"
        @click="handleUserEntry"
      >
        <UserRound :size="20" :stroke-width="1.7" />
      </button>

      <div class="app-rail-divider" />

      <nav class="app-rail-games" role="tablist" aria-label="游戏切换" data-tour="main-brand">
        <RouterLink
          v-for="game in gameNavigationRegistry"
          :key="game.key"
          :to="game.to"
          class="app-rail-button app-rail-game"
          role="tab"
          :class="{'is-selected': game.key === currentGame?.key}"
          :title="game.label"
          :aria-label="game.label"
          :aria-selected="game.key === currentGame?.key"
        >
          <component :is="game.icon" :size="20" :stroke-width="1.7" />
          <span class="app-rail-game-label">{{ game.shortLabel }}</span>
        </RouterLink>
      </nav>

      <div class="app-rail-spacer" />
      <!-- ~ 全局入口 -->
      <!-- * 说明：底部列表按钮向上展开一列与其同尺寸的快捷按钮，承载日志、关于和全局设置 -->
      <!-- * 交互：点击切换展开；点击外部、选择入口或按 Escape 收起；悬停或聚焦时在右侧显示名称 -->
      <!-- * 依赖：依赖 globalNavigationEntries、Vue Router 与 Lucide 图标 -->
      <!-- * 边界：收起时列表不渲染，避免隐藏按钮进入 Tab 顺序；当前处于全局页面时触发按钮保持选中态 -->
      <div ref="globalMenuRoot" class="app-rail-global" @keydown.esc.prevent.stop="closeGlobalMenu(true)">
        <button
          ref="globalMenuTrigger"
          type="button"
          class="app-rail-button app-rail-menu-button"
          :class="{'is-selected': isGlobalRouteActive}"
          :title="isGlobalMenuOpen ? '收起全局菜单' : '打开全局菜单'"
          :aria-label="isGlobalMenuOpen ? '收起全局菜单' : '打开全局菜单'"
          aria-controls="app-global-menu"
          :aria-expanded="isGlobalMenuOpen"
          :data-tour="currentGame ? undefined : 'main-nav'"
          @click="toggleGlobalMenu"
        >
          <List :size="19" :stroke-width="1.8" class="app-rail-menu-icon" />
        </button>

        <!-- 动画作用在子按钮上，Transition 无法从根节点推断时长，需显式声明（含最长错峰延迟）。 -->
        <Transition name="app-rail-global-menu" :duration="{enter: 320, leave: 150}">
          <nav
            v-if="isGlobalMenuOpen"
            id="app-global-menu"
            class="app-rail-global-menu"
            aria-label="全局入口"
          >
            <RouterLink
              v-for="(item, index) in globalNavigationEntries"
              :key="item.key"
              :to="item.to"
              class="app-rail-button app-rail-global-link"
              :style="{'--item-order': globalNavigationEntries.length - 1 - index}"
              :aria-label="item.label"
              :aria-current="route.path === item.to ? 'page' : undefined"
              @click="closeGlobalMenu()"
            >
              <component :is="item.icon" :size="19" :stroke-width="1.8" />
              <span class="app-rail-tip" aria-hidden="true">{{ item.label }}</span>
            </RouterLink>
          </nav>
        </Transition>
      </div>
    </aside>

    <AccountModal :open="isAccountModalOpen" @close="isAccountModalOpen = false" />

    <section class="app-workspace">
      <!-- ~ 无边框窗口标题栏 -->
      <!-- * 说明：跨越游戏选项与页面内容，统一承载当前标题、版本、主题和窗口控制 -->
      <!-- * 交互：标题空白区可拖动窗口；右侧按钮调用主题与 Wails 窗口 API -->
      <!-- * 依赖：依赖 route.meta.title、useThemeMode 与 runtime 控制方法 -->
      <!-- * 边界：所有可点击控件均禁止窗口拖动，保留既有教程选择器 -->
      <header class="app-header">
        <div class="app-drag-region">
          <span class="app-header-root">DSBOX</span>
          <span class="app-header-divider" aria-hidden="true">/</span>
          <span class="app-header-title" data-tour="main-header-title">{{ title }}</span>
        </div>
        <div class="app-window-controls">
          <span class="app-version-pill">{{ appVersion }}</span>
          <!-- ~ 日夜模式切换 -->
          <!-- * 说明：在窗口工具栏中提供主题切换入口 -->
          <!-- * 交互：点击后从按钮中心揭示新主题，并将选择保存到系统配置 -->
          <!-- * 依赖：依赖 useThemeMode 与 View Transitions API -->
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
          <!-- * 说明：替代系统标题栏按钮控制无边框桌面窗口 -->
          <!-- * 交互：调用 Wails runtime 最小化、最大化和退出方法 -->
          <!-- * 依赖：依赖 WindowMinimise、WindowToggleMaximise 与 Quit -->
          <!-- * 边界：按钮区域禁止窗口拖拽，避免点击时触发移动窗口 -->
          <div class="window-control-group" data-tour="main-window-controls">
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

      <div class="app-workspace-body">
        <aside v-if="currentGame" class="app-sidebar">
          <!-- ~ 游戏选项 -->
          <!-- * 说明：只渲染当前游戏注册的数据，避免不同游戏的路由选项耦合在布局中 -->
          <!-- * 交互：选择游戏选项后跳转页面，高亮容器移动至当前路由入口 -->
          <!-- * 依赖：依赖 currentGameNavigation、activeNavIndex、Vue Router 与图标组件 -->
          <!-- * 边界：全局页面隐藏此层；收起或窄窗口下只保留图标和可访问名称 -->
          <nav id="app-game-nav" class="app-nav" data-tour="main-nav" aria-label="游戏选项">
            <span
              v-if="activeNavIndex >= 0"
              class="app-nav-active-highlight"
              :style="{transform: `translateY(${activeNavIndex * 45}px)`}"
              aria-hidden="true"
            />
            <RouterLink
              v-for="item in currentGameNavigation"
              :key="item.key"
              :to="item.to"
              class="app-nav-link"
              exact-active-class="app-nav-link-active"
              :aria-label="item.label"
              :title="isSidebarCollapsed ? item.label : undefined"
            >
              <component :is="item.icon" :size="17" :stroke-width="1.8" class="app-nav-icon" />
              <span class="app-nav-label">{{ item.label }}</span>
            </RouterLink>
          </nav>

          <!-- ~ 收起/展开切换 -->
          <!-- * 说明：在侧边栏底部切换图标模式与图标加文本模式 -->
          <!-- * 交互：点击切换宽度，状态由 toggleSidebarCollapsed 持久化 -->
          <!-- * 边界：窄窗口下侧边栏固定为图标模式，隐藏此按钮 -->
          <button
            type="button"
            class="app-sidebar-toggle"
            :title="sidebarToggleTitle"
            :aria-label="sidebarToggleTitle"
            aria-controls="app-game-nav"
            :aria-expanded="!isSidebarCollapsed"
            @click="toggleSidebarCollapsed"
          >
            <PanelLeftOpen v-if="isSidebarCollapsed" :size="17" :stroke-width="1.8" class="app-nav-icon" />
            <PanelLeftClose v-else :size="17" :stroke-width="1.8" class="app-nav-icon" />
            <span class="app-nav-label">收起</span>
          </button>
        </aside>

        <section class="app-main-column">
          <main class="app-page-host">
            <slot />
          </main>
        </section>
      </div>
    </section>
  </div>
</template>

<style scoped lang="scss">
.app-shell {
  --app-rail-width: 88px;
  --app-sidebar-width: 232px;
  position: relative;
  display: flex;
  min-width: 720px;
  width: 100%;
  height: 100vh;
  overflow: hidden;
  background: var(--ds-bg-shell);
  color: var(--ds-text-primary);
}

// =============== 动态宇宙背景 ===============
// 说明：铺满整个窗口，快捷栏、侧边栏、标题栏以半透明底色叠在其上，整窗连成一片流动的星域。
// 边界：各层只做 transform / opacity 动画，由合成器处理，不触发重绘；减少动态时全部静止。
.app-cosmos {
  position: absolute;
  z-index: 0;
  inset: 0;
  overflow: hidden;
  pointer-events: none;
}

.app-cosmos > * {
  position: absolute;
  will-change: transform, opacity;
}

// 星云层：比窗口四周各大 10%，缓慢漂移与呼吸，漂移幅度不超过预留边距，不会露边。
.app-cosmos-nebula {
  inset: -10%;
}

.app-cosmos-nebula.is-a {
  background: var(--ds-cosmos-nebula-a);
  animation: app-nebula-drift-a 46s ease-in-out infinite alternate;
}

.app-cosmos-nebula.is-b {
  background: var(--ds-cosmos-nebula-b);
  animation: app-nebula-drift-b 62s ease-in-out infinite alternate;
}

// 星点层：宽高各多出一个平铺单元（360×320），平移恰好一个单元后无缝循环，形成星流。
.app-cosmos-stars {
  top: 0;
  left: 0;
  width: calc(100% + 360px);
  height: calc(100% + 320px);
  background: var(--ds-cosmos-stars);
  animation: app-stars-flow 240s linear infinite;
}

// 闪烁星点层：只做透明度呼吸。
.app-cosmos-twinkle {
  inset: 0;
  background: var(--ds-bg-twinkle);
  animation: app-star-twinkle 5.2s ease-in-out infinite alternate;
}

@keyframes app-nebula-drift-a {
  0% {
    transform: translate3d(-3%, -2%, 0) scale(1);
  }

  50% {
    transform: translate3d(1.5%, 1.5%, 0) scale(1.05);
  }

  100% {
    transform: translate3d(3%, -1%, 0) scale(1.02);
  }
}

@keyframes app-nebula-drift-b {
  0% {
    opacity: 0.7;
    transform: translate3d(2.5%, 2%, 0) scale(1.04);
  }

  50% {
    opacity: 1;
    transform: translate3d(-1.5%, -1%, 0) scale(1);
  }

  100% {
    opacity: 0.8;
    transform: translate3d(-3%, 1.5%, 0) scale(1.03);
  }
}

@keyframes app-stars-flow {
  from {
    transform: translate3d(0, 0, 0);
  }

  to {
    transform: translate3d(-360px, -320px, 0);
  }
}

@keyframes app-star-twinkle {
  from {
    opacity: 0.2;
  }

  to {
    opacity: 1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-cosmos > * {
    animation: none;
  }
}

.app-shell.is-global-context {
  --app-sidebar-width: 0px;
}

.app-workspace {
  position: absolute;
  inset: 0 0 0 var(--app-rail-width);
  display: flex;
  min-width: 0;
  flex-direction: column;
}

.app-workspace-body {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex: 1 1 auto;
}

.app-quick-rail {
  position: fixed;
  z-index: 30;
  inset: 0 auto 0 0;
  display: flex;
  width: var(--app-rail-width);
  flex-direction: column;
  align-items: center;
  padding: 20px 0 16px;
  border-right: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-rail);
}

.app-rail-button {
  display: grid;
  width: 46px;
  height: 46px;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 4px;
  color: var(--ds-nav-text);
  text-decoration: none;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    color 160ms ease;
}

.app-rail-button:hover,
.app-rail-button.router-link-active,
.app-rail-button.is-selected {
  border-color: var(--ds-border-strong);
  background: var(--ds-active-bg);
  color: var(--ds-brand);
}

// 用户头像：圆形入口与方形游戏标签区分，未登录时使用中性色。
.app-rail-avatar {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 auto;
  place-items: center;
  padding: 0;
  border: 1px solid var(--ds-border-default);
  border-radius: 50%;
  background: var(--ds-bg-panel);
  color: var(--ds-text-muted);
  cursor: pointer;
  transition:
    border-color 160ms ease,
    color 160ms ease;
}

.app-rail-avatar:hover {
  border-color: var(--ds-border-strong);
  color: var(--ds-brand);
}

.app-rail-divider {
  width: 40px;
  height: 1px;
  flex: 0 0 auto;
  margin: 18px 0;
  background: var(--ds-border-subtle);
}

.app-rail-games {
  display: grid;
  justify-items: center;
  gap: 8px;
}

.app-rail-global {
  position: relative;
  display: grid;
  flex: 0 0 auto;
  justify-items: center;
}

// 游戏切换标签：图标加短名称，选中项左侧显示品牌色指示条。
.app-rail-game {
  position: relative;
  width: 60px;
  height: 58px;
  align-content: center;
  gap: 4px;
}

.app-rail-game.is-selected::before {
  position: absolute;
  inset: 12px auto 12px -1px;
  width: 2px;
  border-radius: 0 2px 2px 0;
  background: var(--ds-brand-gradient);
  box-shadow: 0 0 8px var(--ds-brand-glow);
  content: '';
}

.app-rail-game-label {
  font-size: 10px;
  font-weight: 650;
  line-height: 1;
  white-space: nowrap;
}

.app-rail-spacer {
  flex: 1;
}

.app-rail-menu-button {
  flex: 0 0 auto;
}

.app-rail-menu-button:focus-visible,
.app-rail-global-link:focus-visible {
  outline: 2px solid var(--ds-brand);
  outline-offset: 2px;
}

.app-rail-menu-button[aria-expanded='true'] {
  border-color: var(--ds-border-strong);
  background: var(--ds-active-bg);
  color: var(--ds-brand);
}

.app-sidebar {
  position: relative;
  z-index: 5;
  display: flex;
  width: var(--app-sidebar-width);
  height: 100%;
  flex: 0 0 var(--app-sidebar-width);
  flex-direction: column;
  overflow: hidden;
  padding: 16px 14px 18px;
  border-right: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-sidebar);
  transition:
    width 240ms cubic-bezier(0.22, 1, 0.36, 1),
    flex-basis 240ms cubic-bezier(0.22, 1, 0.36, 1);
}

// 收起状态：宽度让图标中心与展开时保持一致，切换过程中图标不发生位移，只裁切并淡出文本。
.app-shell.is-sidebar-collapsed {
  --app-sidebar-width: 68px;
}

.app-sidebar .app-nav-label {
  transition: opacity 160ms ease;
}

.app-shell.is-sidebar-collapsed .app-sidebar .app-nav-label {
  opacity: 0;
}

.app-sidebar-toggle {
  display: flex;
  min-width: 0;
  height: 40px;
  flex: 0 0 auto;
  align-items: center;
  gap: 12px;
  margin-top: auto;
  padding: 0 10px;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-muted);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    color 160ms ease;
}

.app-sidebar-toggle:hover {
  border-color: var(--ds-border-subtle);
  background: var(--ds-hover-bg);
  color: var(--ds-text-primary);
}

.app-sidebar-toggle:active {
  background: var(--ds-pressed-bg);
}

.app-sidebar-toggle:focus-visible,
.app-nav-link:focus-visible {
  outline: 2px solid var(--ds-brand);
  outline-offset: -2px;
}

.app-nav {
  position: relative;
  display: grid;
  align-content: start;
  gap: 5px;
}

.app-nav-active-highlight {
  position: absolute;
  z-index: 0;
  top: 0;
  left: 0;
  width: 100%;
  height: 40px;
  border: 1px solid var(--ds-border-strong);
  border-radius: 4px;
  background: var(--ds-active-bg);
  box-shadow: var(--ds-active-glow);
  pointer-events: none;
  transition: transform 260ms cubic-bezier(0.22, 1, 0.36, 1);
}

// 选中高亮左侧的品牌渐变指示条，与快捷栏游戏标签的指示条呼应。
.app-nav-active-highlight::before {
  position: absolute;
  inset: 10px auto 10px -1px;
  width: 2px;
  border-radius: 0 2px 2px 0;
  background: var(--ds-brand-gradient);
  box-shadow: 0 0 8px var(--ds-brand-glow);
  content: '';
}

.app-nav-link {
  position: relative;
  z-index: 1;
  display: flex;
  min-width: 0;
  height: 40px;
  align-items: center;
  gap: 12px;
  padding: 0 10px;
  border: 1px solid transparent;
  border-radius: 4px;
  color: var(--ds-nav-text);
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
  transition: color 140ms ease;
}

.app-nav-icon {
  flex: 0 0 auto;
}

.app-nav-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-nav-link:hover:not(.app-nav-link-active) {
  border-color: var(--ds-border-subtle);
  background: var(--ds-hover-bg);
  color: var(--ds-text-primary);
}

.app-nav-link-active {
  border-color: transparent;
  background: transparent;
  color: var(--ds-brand);
}

// 全局入口列表：悬浮在触发按钮正上方，按钮尺寸与样式和快捷栏按钮一致，不使用抽屉面板。
.app-rail-global-menu {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 50%;
  display: grid;
  justify-items: center;
  gap: 8px;
  transform: translateX(-50%);
}

// 快捷栏底色为半透明，悬浮在其上的全局入口使用实色面板，避免与下方按钮叠色。
.app-rail-global-link {
  position: relative;
  background: var(--ds-bg-panel);
}

.app-rail-global-link:hover:not(.router-link-active) {
  border-color: var(--ds-border-default);
  background: var(--ds-hover-bg);
  color: var(--ds-text-primary);
}

// 名称提示：悬停或键盘聚焦时在按钮右侧出现，不占用快捷栏宽度。
.app-rail-tip {
  position: absolute;
  top: 50%;
  left: calc(100% + 10px);
  padding: 5px 8px;
  border: 1px solid var(--ds-border-subtle);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  box-shadow: 0 4px 12px rgb(5 20 50 / 0.14);
  color: var(--ds-text-primary);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.2;
  white-space: nowrap;
  opacity: 0;
  pointer-events: none;
  transform: translate(-4px, -50%);
  transition:
    opacity 140ms ease,
    transform 160ms cubic-bezier(0.22, 1, 0.36, 1);
}

.app-rail-global-link:hover .app-rail-tip,
.app-rail-global-link:focus-visible .app-rail-tip {
  opacity: 1;
  transform: translate(0, -50%);
}

// 展开动画：按钮自下而上依次升起，离触发按钮最近的最先出现；收起时统一快速淡出。
.app-rail-global-menu-enter-active .app-rail-global-link {
  transition:
    opacity 180ms cubic-bezier(0.23, 1, 0.32, 1),
    transform 240ms cubic-bezier(0.22, 1, 0.36, 1);
  transition-delay: calc(var(--item-order) * 35ms);
}

.app-rail-global-menu-leave-active .app-rail-global-link {
  transition:
    opacity 140ms ease-in,
    transform 140ms ease-in;
}

.app-rail-global-menu-enter-from .app-rail-global-link,
.app-rail-global-menu-leave-to .app-rail-global-link {
  opacity: 0;
  transform: translateY(10px) scale(0.92);
}

.app-rail-menu-icon {
  transition: transform 220ms cubic-bezier(0.22, 1, 0.36, 1);
}

.app-rail-menu-button[aria-expanded='true'] .app-rail-menu-icon {
  transform: rotate(-90deg);
}

@media (prefers-reduced-motion: reduce) {
  .app-rail-global-menu-enter-from .app-rail-global-link,
  .app-rail-global-menu-leave-to .app-rail-global-link {
    transform: none;
  }

  .app-rail-global-menu-enter-active .app-rail-global-link,
  .app-rail-global-menu-leave-active .app-rail-global-link {
    transition: opacity 140ms ease-out;
    transition-delay: 0ms;
  }

  .app-rail-menu-icon,
  .app-rail-tip {
    transition: none;
  }

  .app-rail-menu-button[aria-expanded='true'] .app-rail-menu-icon {
    transform: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-nav-active-highlight,
  .app-sidebar,
  .app-sidebar .app-nav-label {
    transition: none;
  }
}

.app-main-column {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 0;
  flex: 1 1 auto;
  flex-direction: column;
  overflow: hidden;
}

.app-header {
  position: relative;
  z-index: 10;
  display: flex;
  height: 54px;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 18px 0 34px;
  border-bottom: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-header);
}

// 标题栏底部左侧的品牌渐变光线，自左向右淡出，营造舰载终端的扫描线感。
.app-header::after {
  position: absolute;
  bottom: -1px;
  left: 0;
  width: min(42%, 420px);
  height: 1px;
  background: linear-gradient(90deg, var(--ds-brand), transparent);
  opacity: 0.7;
  pointer-events: none;
  content: '';
}

.app-drag-region {
  display: flex;
  min-width: 0;
  height: 100%;
  flex: 1 1 auto;
  align-items: center;
  gap: 10px;
  color: var(--ds-text-muted);
  font-size: 11px;
  --wails-draggable: drag;
}

.app-header-root,
.app-header-divider,
.app-header-title {
  --wails-draggable: drag;
}

.app-header-root {
  color: var(--ds-text-muted);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.app-header-divider {
  opacity: 0.6;
}

.app-header-title {
  overflow: hidden;
  color: var(--ds-text-primary);
  font-size: 12px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-window-controls,
.window-control-group {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
}

.app-window-controls {
  gap: 8px;
  --wails-draggable: no-drag;
}

.window-control-group {
  gap: 4px;
  margin-left: 5px;
  padding-left: 10px;
  border-left: 1px solid var(--ds-border-subtle);
  --wails-draggable: no-drag;
}

// 页面宿主：保持透明，露出外壳的宇宙背景；首页场景以绝对定位铺满此区域。
.app-page-host {
  position: relative;
  min-width: 0;
  min-height: 0;
  flex: 1 1 auto;
  overflow: auto;
}

.app-version-pill,
.theme-toggle-button,
.window-control-button {
  --wails-draggable: no-drag;
}

.app-version-pill {
  display: inline-flex;
  height: 28px;
  align-items: center;
  padding: 0 9px;
  border: 1px solid var(--ds-border-subtle);
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-muted);
  font-family: var(--ds-font-mono);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
}

.theme-toggle-button,
.window-control-button {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-muted);
  cursor: pointer;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    color 160ms ease;
}

.theme-toggle-button:hover,
.window-control-button:hover {
  border-color: var(--ds-border-default);
  background: var(--ds-hover-bg);
  color: var(--ds-brand);
}

.theme-toggle-button:active,
.window-control-button:active {
  background: var(--ds-pressed-bg);
}

.theme-toggle-button:disabled {
  cursor: not-allowed;
  color: var(--ds-text-disabled);
}

.window-control-button.is-close:hover {
  border-color: var(--ds-error-border);
  background: var(--ds-error-bg);
  color: var(--ds-error);
}

@media (max-width: 1200px) {
  .app-shell {
    --app-rail-width: 72px;
    --app-sidebar-width: 204px;
  }

  .app-shell.is-sidebar-collapsed {
    --app-sidebar-width: 64px;
  }

  .app-sidebar {
    padding-right: 12px;
    padding-left: 12px;
  }

  .app-header {
    padding-left: 26px;
  }
}

@media (max-width: 900px) {
  .app-shell {
    --app-rail-width: 64px;
    --app-sidebar-width: 72px;
  }

  // 窄窗口固定为图标模式，收起开关不再生效。
  .app-shell.is-sidebar-collapsed {
    --app-sidebar-width: 72px;
  }

  .app-sidebar {
    align-items: center;
    padding: 16px 8px 14px;
  }

  .app-sidebar-toggle {
    display: none;
  }

  .app-nav-label {
    display: none;
  }

  .app-nav {
    width: 100%;
    gap: 5px;
  }

  .app-nav-link {
    justify-content: center;
    padding: 0;
  }

  .app-header {
    padding-left: 18px;
  }
}

@media (max-width: 640px) {
  .app-shell {
    min-width: 0;
    --app-rail-width: 52px;
    --app-sidebar-width: 56px;
  }

  .app-shell.is-sidebar-collapsed {
    --app-sidebar-width: 56px;
  }

  .app-quick-rail {
    padding-top: 12px;
  }

  .app-rail-button,
  .app-rail-avatar {
    width: 38px;
    height: 38px;
  }

  .app-rail-divider {
    width: 30px;
    margin: 12px 0;
  }

  .app-rail-game-label {
    display: none;
  }

  .app-header {
    height: 48px;
    gap: 8px;
    padding: 0 8px;
  }

  .app-header-root,
  .app-header-divider,
  .app-version-pill {
    display: none;
  }

  .app-window-controls {
    gap: 2px;
  }

  .window-control-group {
    gap: 0;
    margin-left: 1px;
    padding-left: 3px;
  }
}
</style>
