<script setup lang="ts">
import { Construction } from '@lucide/vue'

interface PagePlaceholderProps {
  readonly eyebrow: string
  readonly title: string
  readonly description: string
  readonly note?: string
}

withDefaults(defineProps<PagePlaceholderProps>(), {
  note: '页面结构已预留，功能和交互将在下一轮设计中接入。',
})
</script>

<template>
  <!-- ~ 功能页面占位 -->
  <!-- * 说明：为尚未重新设计的功能页提供统一的空状态和页面边界。 -->
  <!-- * 交互：当前不触发后端请求、不创建文件操作，也不展示旧版业务控件。 -->
  <!-- * 依赖：仅依赖页面传入的标题文案和 Lucide 图标。 -->
  <!-- * 边界：后续页面设计应替换内容区域，保留页面级路由与布局契约。 -->
  <section class="page-placeholder" :aria-labelledby="`placeholder-title-${title}`">
    <div class="page-placeholder-copy">
      <p class="page-placeholder-eyebrow">{{ eyebrow }}</p>
      <h1 :id="`placeholder-title-${title}`" class="page-placeholder-title">{{ title }}</h1>
      <p class="page-placeholder-description">{{ description }}</p>
    </div>

    <section class="page-placeholder-panel" aria-label="页面设计状态">
      <div class="page-placeholder-icon" aria-hidden="true">
        <Construction :size="22" :stroke-width="1.6" />
      </div>
      <div>
        <p class="page-placeholder-status">设计整理中</p>
        <p class="page-placeholder-note">{{ note }}</p>
      </div>
    </section>
  </section>
</template>

<style scoped lang="scss">
.page-placeholder {
  display: grid;
  align-content: start;
  min-height: 100%;
  padding: clamp(32px, 6vw, 92px) clamp(24px, 7vw, 120px);
}

.page-placeholder-copy { max-width: 620px; }

.page-placeholder-eyebrow {
  margin: 0 0 18px;
  color: var(--ds-brand);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.page-placeholder-title {
  margin: 0;
  color: var(--ds-text-primary);
  font-size: clamp(32px, 4vw, 58px);
  font-weight: 700;
  letter-spacing: -0.04em;
  line-height: 1.05;
}

.page-placeholder-description {
  max-width: 48ch;
  margin: 22px 0 0;
  color: var(--ds-text-secondary);
  font-size: 15px;
  line-height: 1.75;
}

.page-placeholder-panel {
  display: flex;
  max-width: 620px;
  align-items: center;
  gap: 14px;
  margin-top: clamp(54px, 9vw, 120px);
  padding: 18px 0;
  border-top: 1px solid var(--ds-border-default);
  border-bottom: 1px solid var(--ds-border-default);
}

.page-placeholder-icon {
  display: grid;
  width: 44px;
  height: 44px;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  color: var(--ds-brand);
}

.page-placeholder-status,
.page-placeholder-note { margin: 0; }

.page-placeholder-status {
  color: var(--ds-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.page-placeholder-note {
  margin-top: 4px;
  color: var(--ds-text-muted);
  font-size: 12px;
  line-height: 1.6;
}
</style>
