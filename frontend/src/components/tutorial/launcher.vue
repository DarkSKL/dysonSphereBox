<script setup lang="ts">
import { CircleHelp } from '@lucide/vue';
import { usePageTutorial } from '../../composables/usePageTutorial';

// =============== 全局教程入口 ===============
// 说明：页面内容处于占位阶段时，只保留应用框架教程，避免教程指向已经移除的业务控件。
const { restartTutorial } = usePageTutorial('global');

// =============== 手动重看 ===============
// 说明：忽略数据库状态，直接重新启动应用框架教程。
const restartCurrentTutorial = async (): Promise<void> => {
  await restartTutorial();
};
</script>

<template>
  <button
    type="button"
    class="tutorial-launcher-button"
    title="查看应用教程"
    aria-label="查看应用教程"
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
