<script setup lang="ts">
export type SettingsValueTone = 'default' | 'success' | 'warning' | 'error' | 'muted'

interface SettingsValueBoxProps {
  readonly tone?: SettingsValueTone
}

withDefaults(defineProps<SettingsValueBoxProps>(), {
  tone: 'default',
})
</script>

<template>
  <!-- ~ 只读值框 -->
  <!-- * 说明：与输入框同尺寸的只读状态框，用于展示安装状态、版本等不可直接编辑的值 -->
  <!-- * 交互：不可聚焦、不可编辑，内容过长时省略 -->
  <!-- * 依赖：默认插槽为展示文本 -->
  <!-- * 边界：只负责展示，状态变化由调用方驱动 -->
  <div class="settings-value" :class="`is-${tone}`">
    <slot />
  </div>
</template>

<style scoped lang="scss">
.settings-value {
  display: flex;
  box-sizing: border-box;
  height: var(--ds-settings-control-height, 34px);
  align-items: center;
  padding: 0 12px;
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-primary);
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.settings-value.is-success {
  color: var(--ds-success);
}

.settings-value.is-warning {
  color: var(--ds-warning);
}

.settings-value.is-error {
  color: var(--ds-error);
}

.settings-value.is-muted {
  color: var(--ds-text-muted);
}
</style>
