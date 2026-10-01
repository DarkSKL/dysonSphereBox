<script setup lang="ts">
export type SettingsHintTone = 'muted' | 'success' | 'warning' | 'error'

interface SettingsSectionProps {
  readonly id: string
  readonly title: string
  /** 默认提示文案；提供 hint 插槽时被覆盖 */
  readonly description?: string
  readonly hintTone?: SettingsHintTone
}

withDefaults(defineProps<SettingsSectionProps>(), {
  description: '',
  hintTone: 'muted',
})
</script>

<template>
  <!-- ~ 设置项 -->
  <!-- * 说明：标题一行；下方为“值 + 操作按钮”同一行；最下方一行提示说明当前状态 -->
  <!-- * 交互：本组件只负责排版，不处理任何设置读写 -->
  <!-- * 依赖：默认插槽放值（首个元素占满剩余宽度）与按钮；extra 插槽放标题旁徽标；hint 插槽覆盖提示；after 插槽放补充内容 -->
  <!-- * 边界：提示保持一行；内容区较窄时按钮整体换到值的下一行，避免输入框被压缩 -->
  <section class="settings-section" :aria-labelledby="`${id}-title`">
    <header class="settings-section-head">
      <h2 :id="`${id}-title`" class="settings-section-title">{{ title }}</h2>
      <slot name="extra" />
    </header>
    <div class="settings-section-row">
      <slot />
    </div>
    <p :id="`${id}-hint`" class="settings-section-hint" :class="`is-${hintTone}`" aria-live="polite">
      <slot name="hint">{{ description }}</slot>
    </p>
    <slot name="after" />
  </section>
</template>

<style scoped lang="scss">
.settings-section + .settings-section {
  margin-top: 28px;
}

.settings-section-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.settings-section-title {
  margin: 0;
  color: var(--ds-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.settings-section-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.settings-section-hint {
  min-height: 18px;
  margin: 6px 0 0;
  overflow: hidden;
  color: var(--ds-text-muted);
  font-size: 12px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.settings-section-hint.is-success {
  color: var(--ds-success);
}

.settings-section-hint.is-warning {
  color: var(--ds-warning);
}

.settings-section-hint.is-error {
  color: var(--ds-error);
}
</style>

<style lang="scss">
// 插槽内容来自调用方组件（含 Naive UI 组件根节点），scoped 选择器无法稳定命中，这里用非 scoped 规则。
// 值占满剩余宽度，按钮保持统一宽度；剩余宽度不足 260px 时按钮整体换到下一行。
// 行内所有控件（输入框、只读值框、按钮）统一使用 --ds-settings-control-height，保证各设置项高度一致。
.settings-section-row {
  --ds-settings-control-height: 34px;
}

.settings-section-row > :first-child {
  flex: 1 1 260px;
  min-width: 0;
}

.settings-section-row > :not(:first-child) {
  flex: 0 0 auto;
}

.settings-section-row > .n-input,
.settings-section-row > .n-button {
  height: var(--ds-settings-control-height);
}

.settings-section-row > .n-input {
  --n-height: var(--ds-settings-control-height) !important;
}

.settings-section-row > .n-button {
  --n-height: var(--ds-settings-control-height) !important;
  min-width: 84px;
}
</style>
