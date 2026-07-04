<script setup lang="ts">
import { CircleHelp } from '@lucide/vue';
import { computed } from 'vue';
import { useRoute } from 'vue-router';

import { usePageTutorial } from '../../composables/usePageTutorial';
import type { TutorialPageKey } from '../../tutorials/types';

const route = useRoute();

type RoutedTutorialPageKey = Exclude<TutorialPageKey, 'global'>;

// =============== 路由映射 ===============
// 说明：把当前主导航路由映射到教程 pageKey，供顶部重看入口使用。
const routeTutorialMap: Record<string, RoutedTutorialPageKey> = {
  '/': 'home',
  '/headless': 'headless',
  '/mod': 'mod',
  '/blueprint-generator': 'blueprint',
  '/environment': 'environment',
  '/log': 'log',
  '/settings': 'settings',
  '/about': 'about',
};

const tutorialControllers = {
  home: usePageTutorial('home'),
  headless: usePageTutorial('headless'),
  mod: usePageTutorial('mod'),
  blueprint: usePageTutorial('blueprint'),
  environment: usePageTutorial('environment'),
  log: usePageTutorial('log'),
  settings: usePageTutorial('settings'),
  about: usePageTutorial('about'),
} satisfies Record<RoutedTutorialPageKey, ReturnType<typeof usePageTutorial>>;

const currentPageKey = computed<RoutedTutorialPageKey | null>(() => routeTutorialMap[route.path] ?? null);
const currentTutorial = computed(() => (
  currentPageKey.value ? tutorialControllers[currentPageKey.value] ?? null : null
));
const canLaunchTutorial = computed(() => currentTutorial.value !== null);

// =============== 手动重看 ===============
// 说明：忽略数据库状态，直接启动当前页面教程；完成或关闭后仍会覆盖当前版本状态。
const restartCurrentTutorial = async (): Promise<void> => {
  if (!currentTutorial.value) {
    return;
  }

  await currentTutorial.value.restartTutorial();
};
</script>

<template>
  <button
    v-if="canLaunchTutorial"
    type="button"
    class="tutorial-launcher-button"
    title="查看本页教程"
    aria-label="查看本页教程"
    data-tour="main-tutorial-launcher"
    @click="restartCurrentTutorial"
  >
    <CircleHelp :size="16" :stroke-width="1.9" />
  </button>
</template>

<style scoped lang="scss">
.tutorial-launcher-button {
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-muted);
  transition:
    border-color 160ms ease,
    background 160ms ease,
    color 160ms ease,
    transform 160ms ease;
  --wails-draggable: no-drag;
}

.tutorial-launcher-button:hover {
  border-color: var(--ds-border-strong);
  background: var(--ds-hover-bg);
  color: var(--ds-brand);
}

.tutorial-launcher-button:active {
  transform: translateY(1px);
}
</style>
