<script setup lang="ts">
interface SettingsSectionProps {
  readonly id: string
  readonly title: string
  readonly description: string
}

defineProps<SettingsSectionProps>()
</script>

<template>
  <!-- ~ 设置分区 -->
  <!-- * 说明：左侧为分区标题与说明，右侧承载该分区的具体控件，分区之间以细线分隔 -->
  <!-- * 交互：本组件只负责排版，不处理任何设置读写 -->
  <!-- * 依赖：依赖 id/title/description 属性和默认插槽 -->
  <!-- * 边界：内容区较窄时改为上下排布，避免控件被压缩 -->
  <section class="settings-section" :aria-labelledby="`${id}-title`">
    <header class="settings-section-head">
      <h2 :id="`${id}-title`" class="settings-section-title">{{ title }}</h2>
      <p class="settings-section-description">{{ description }}</p>
    </header>
    <div class="settings-section-body">
      <slot />
    </div>
  </section>
</template>

<style scoped lang="scss">
.settings-section {
  display: grid;
  grid-template-columns: minmax(180px, 240px) minmax(0, 1fr);
  gap: 20px 48px;
  padding: 28px 0;
  border-top: 1px solid var(--ds-border-default);
}

// 首个分区紧贴页面顶部留白，不再额外画分隔线
.settings-section:first-child {
  padding-top: 0;
  border-top: 0;
}

.settings-section-title {
  margin: 0;
  color: var(--ds-text-primary);
  font-size: 15px;
  font-weight: 700;
}

.settings-section-description {
  margin: 8px 0 0;
  color: var(--ds-text-muted);
  font-size: 12px;
  line-height: 1.7;
}

.settings-section-body {
  min-width: 0;
}

@container settings-page (max-width: 720px) {
  .settings-section {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
