<script setup lang="ts">
import {CircleAlert, Inbox, RotateCw} from '@lucide/vue'
import {ref, watch} from 'vue'

defineProps<{
  readonly loading: boolean
  readonly error: string
  readonly empty: boolean
  readonly emptyLabel: string
  readonly hasMore: boolean
  readonly moreError: string
}>()
const emit = defineEmits<{
  (event: 'retry'): void
  (event: 'more'): void
}>()

// =============== 列表滚动 ===============
// 底部标记在可见时触发续载，每批完成后重新观察以填满较高的窗口。
const listEnd = ref<HTMLElement | null>(null)
watch(listEnd, (element, _, onCleanup) => {
  if (!element) return
  const observer = new IntersectionObserver(([entry]) => {
    if (entry.isIntersecting && listEnd.value === element) emit('more')
  })
  observer.observe(element)
  onCleanup(() => observer.disconnect())
}, {flush: 'post'})
</script>

<template>
  <section class="notification-feed">
    <!-- ~ 消息列表内容 -->
    <!-- * 说明：站内信和公告共用列表密度与状态反馈 -->
    <!-- * 交互：滚动列表，加载失败时可重试 -->
    <!-- * 依赖：上层列表数据和查询状态 -->
    <!-- * 边界：首次加载展示骨架；追加失败后保留条目，允许原地重试 -->
    <span v-if="loading && empty" class="sr-only" role="status">正在加载消息…</span>
    <div class="notification-feed-viewport" :aria-busy="loading">
      <div v-if="loading && empty" class="notification-feed-skeleton" aria-hidden="true">
        <div v-for="row in 5" :key="row" class="notification-feed-skeleton-row">
          <span />
          <span />
        </div>
      </div>
      <div v-else-if="error" class="notification-feed-state" role="alert">
        <CircleAlert :size="24" :stroke-width="1.7" class="notification-feed-error" aria-hidden="true" />
        <p>{{ error }}</p>
        <n-button secondary size="small" @click="emit('retry')">
          <template #icon><RotateCw :size="14" aria-hidden="true" /></template>
          重试
        </n-button>
      </div>
      <div v-else-if="empty" class="notification-feed-state" role="status">
        <Inbox :size="28" :stroke-width="1.6" aria-hidden="true" />
        <p>{{ emptyLabel }}</p>
      </div>
      <ul v-else class="notification-feed-list">
        <slot />
      </ul>
      <div v-if="moreError" class="notification-feed-load-status" role="alert">
        {{ moreError }}
        <n-button size="small" @click="emit('more')">重试</n-button>
      </div>
      <p v-else-if="loading && !empty" class="notification-feed-load-status" role="status">正在加载消息…</p>
      <div v-else-if="hasMore && !error && !loading" ref="listEnd" class="notification-feed-list-end" aria-hidden="true" />
    </div>
  </section>
</template>

<style scoped lang="scss">
.notification-feed {
  --notification-row-padding: 12px;
  --notification-row-gap: 12px;
  --notification-icon-size: 38px;
  display: flex;
  min-height: 0;
  flex: 1 1 auto;
  flex-direction: column;
}

.notification-feed-viewport {
  min-height: 0;
  flex: 1 1 auto;
  overflow: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}

.notification-feed-list,
.notification-feed-skeleton {
  margin: 0;
  padding: 4px 8px 8px;
  list-style: none;
}

.notification-feed-list > :deep(li + li),
.notification-feed-skeleton-row + .notification-feed-skeleton-row {
  border-top: 1px solid var(--ds-border-subtle);
}

.notification-feed :deep(.notification-row),
.notification-feed-skeleton-row {
  display: grid;
  width: 100%;
  min-height: 66px;
  grid-template-columns: var(--notification-icon-size) minmax(0, 1fr);
  align-items: center;
  gap: var(--notification-row-gap);
  padding: 10px var(--notification-row-padding);
}

.notification-feed :deep(.notification-row) {
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-primary);
  text-align: left;
  cursor: pointer;
}

.notification-feed :deep(.notification-row:disabled) {
  color: var(--ds-text-secondary);
  cursor: default;
}

.notification-feed :deep(.notification-row:hover:not(:disabled)),
.notification-feed :deep(.notification-row[aria-expanded='true']) {
  background: var(--ds-hover-bg);
}

.notification-feed :deep(.notification-row:active:not(:disabled)) {
  background: var(--ds-active-bg);
}

.notification-feed :deep(.notification-row:focus-visible) {
  outline: 2px solid var(--ds-brand);
  outline-offset: -2px;
}

.notification-feed :deep(.notification-row-icon),
.notification-feed-skeleton-row > :first-child {
  position: relative;
  display: grid;
  width: var(--notification-icon-size);
  height: var(--notification-icon-size);
  place-items: center;
  border: 1px solid var(--ds-border-subtle);
  border-radius: 50%;
  background: var(--ds-bg-panel);
  color: var(--ds-text-secondary);
}

.notification-feed :deep(.is-unread .notification-row-icon) {
  background: var(--ds-active-bg);
  color: var(--ds-brand);
}

.notification-feed :deep(.notification-row-dot) {
  position: absolute;
  top: 0;
  right: 0;
  width: 8px;
  height: 8px;
  border: 1px solid var(--ds-bg-panel-elevated);
  border-radius: 50%;
  background: var(--ds-brand);
}

.notification-feed :deep(.notification-row-content) {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.notification-feed :deep(.notification-row-heading) {
  display: flex;
  min-width: 0;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.notification-feed :deep(.notification-row-title),
.notification-feed :deep(.notification-row-meta) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notification-feed :deep(.notification-row-title) {
  font-size: 14px;
  line-height: 1.45;
}

.notification-feed :deep(.is-unread .notification-row-title) {
  font-weight: 700;
}

.notification-feed :deep(.notification-row-meta) {
  color: var(--ds-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.notification-feed :deep(.notification-row-time) {
  flex: 0 0 auto;
  color: var(--ds-text-secondary);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.notification-feed-state {
  display: grid;
  min-height: 176px;
  justify-items: center;
  align-content: center;
  gap: 10px;
  padding: 24px;
  color: var(--ds-text-secondary);
  font-size: 13px;
  overflow-wrap: anywhere;
  text-align: center;
}

.notification-feed-state p {
  margin: 0;
}

.notification-feed-error {
  color: var(--ds-error);
}

.notification-feed-skeleton-row > :last-child {
  display: grid;
  gap: 8px;
}

.notification-feed-skeleton-row > :last-child::before,
.notification-feed-skeleton-row > :last-child::after {
  width: 45%;
  height: 9px;
  border-radius: 3px;
  background: var(--ds-active-bg);
  content: '';
}

.notification-feed-skeleton-row > :last-child::after {
  width: 68%;
  background: var(--ds-hover-bg);
}

.notification-feed-list-end {
  height: 1px;
}

.notification-feed-load-status {
  margin: 0;
  padding: 8px 20px;
  color: var(--ds-text-secondary);
  font-size: 12px;
  text-align: center;
}

</style>
