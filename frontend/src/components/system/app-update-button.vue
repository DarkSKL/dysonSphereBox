<script lang="ts">
// 说明：事件和禁用属性直接交给 Naive UI 按钮，外围描边只负责展示下载进度。
export default {inheritAttrs: false}
</script>

<script setup lang="ts">
import {useAppUpdateState} from '../../composables/system/useAppUpdateState'

const props = defineProps<{readonly secondary?: boolean; readonly disabled?: boolean}>()

// =============== 按钮下载边框 ===============
// 说明：SVG 使用真实周长归一化，沿四边展示实际字节比例；未知大小时保留空轨道并展示已下载量。
// 边界：100% 仅表示下载完成，校验安装期间保持满框并禁用操作；失败后随共享状态复位。
const {isUpdatingApp, appUpdatePercentage, appUpdateLabel, appUpdateDetail} = useAppUpdateState()
</script>

<template>
  <!-- ~ 应用更新按钮 -->
  <!-- * 说明：边框沿周长显示真实下载进度，文字补充当前阶段与百分比 -->
  <!-- * 交互：更新期间禁止重复点击；依赖：共享应用更新状态与 Naive UI 按钮 -->
  <!-- * 边界：下载大小未知时不展示虚假比例，完成和失败均由共享状态解除进度描边 -->
  <span class="app-update-button" :class="{'is-updating': isUpdatingApp}" :title="isUpdatingApp ? appUpdateDetail : undefined">
    <n-button
      v-bind="$attrs"
      type="primary"
      :secondary="props.secondary || isUpdatingApp"
      :disabled="props.disabled || isUpdatingApp"
      :aria-busy="isUpdatingApp"
    >
      <template #icon><slot name="icon" /></template>
      <span v-if="isUpdatingApp" class="app-update-button-label">{{ appUpdateLabel }}</span>
      <slot v-else />
    </n-button>
    <svg
      v-if="isUpdatingApp"
      class="app-update-button-border"
      role="progressbar"
      :aria-label="appUpdateLabel"
      :aria-valuenow="appUpdatePercentage"
      :aria-valuetext="`${appUpdateLabel}，${appUpdateDetail}`"
      aria-valuemin="0"
      aria-valuemax="100"
    >
      <rect class="app-update-button-track" x="1" y="1" rx="3" />
      <rect
        class="app-update-button-fill" x="1" y="1" rx="3" pathLength="100"
        :stroke-dasharray="`${appUpdatePercentage ?? 0} 100`"
      />
    </svg>
  </span>
</template>

<style scoped lang="scss">
.app-update-button {
  position: relative;
  display: inline-flex;
  flex-shrink: 0;
  vertical-align: middle;

  :deep(.n-button) { min-width: 128px; width: 100%; }

  &.is-updating :deep(.n-button) {
    opacity: 1;
    color: var(--ds-text-primary);
    background: var(--ds-bg-panel-elevated);
  }
}

.app-update-button-label {
  font-family: var(--ds-font-display);
  font-variant-numeric: tabular-nums;
}

.app-update-button-border {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: none;

  rect { width: calc(100% - 2px); height: calc(100% - 2px); fill: none; stroke-width: 2px; }
}

.app-update-button-track { stroke: var(--ds-border-default); }
.app-update-button-fill { stroke: var(--ds-brand); }
</style>
