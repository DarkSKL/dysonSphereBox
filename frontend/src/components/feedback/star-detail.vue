<script setup lang="ts">
import {CornerDownRight, ShieldCheck} from '@lucide/vue'
import {computed, nextTick, watch} from 'vue'

import {formatFeedbackTime, reviewLabels, type Entry, type EntryKind, type StarDetail} from '../../types/feedback'

const props = defineProps<{
  detail: StarDetail
  busy: boolean
  highlighted: string
}>()

const emit = defineEmits<{
  page: [value: number]
}>()

// =============== 讨论分组与去重 ===============
// 说明：置顶分组有数量上限，翻页时把超出上限的同类条目合并到对应分组并按 ID 去重。
const allReplies = computed(() => props.detail.replies.list ?? [])
function groupedReplies(kind: EntryKind, pinned: Entry[] | null | undefined): Entry[] {
  const group = new Map((pinned ?? []).map((entry) => [entry.id, entry]))
  for (const entry of allReplies.value) if (entry.kind === kind) group.set(entry.id, entry)
  return [...group.values()]
}

const official = computed(() => groupedReplies('official', props.detail.officialReplies))
const supplements = computed(() => groupedReplies('supplement', props.detail.supplements).sort((left, right) =>
  left.createdAt.localeCompare(right.createdAt) || left.id.length - right.id.length || left.id.localeCompare(right.id)))
const comments = computed(() => groupedReplies('comment', []))
const lastPage = computed(() => Math.max(1, Math.ceil(props.detail.replies.total / props.detail.replies.pageSize)))

function initialOf(name: string | undefined, fallback = '匿'): string {
  const trimmed = name?.trim() ?? ''
  return trimmed ? (Array.from(trimmed)[0] || fallback) : fallback
}

watch(() => props.highlighted, async (id) => {
  if (!id) return
  await nextTick()
  document.getElementById('feedback-entry-' + id)?.scrollIntoView({block: 'nearest'})
}, {immediate: true})
</script>

<template>
  <!-- ~ 纯净阅读正文与作者补充 -->
  <!-- * 说明：所有互动与管理图标已归位至顶栏工具区与底栏评论条，中部完全留给正文、官方回应与简洁评论，避免多层分隔线切割空间。 -->
  <!-- * 交互：消息提醒深链自动滚动并高亮对应条目；多页留言仅在超出单页时展示轻量分页器。 -->
  <!-- * 依赖：StarDetail 正文、官方回应、作者补充与分页留言。 -->
  <!-- * 边界：无留言时不占用多余空占位行，底部输入条自身即构成自然引导。 -->
  <article
    class="feedback-post"
    :id="'feedback-entry-' + detail.original.id"
    :class="{'entry-highlight': highlighted === detail.original.id}"
  >
    <p class="post-body">{{ detail.original.body }}</p>
    <p v-if="detail.reviewReason" class="entry-reason">审核说明 · {{ detail.reviewReason }}</p>

    <div v-if="supplements.length" class="supplement-list">
      <div
        v-for="entry of supplements"
        :key="entry.id"
        :id="'feedback-entry-' + entry.id"
        class="supplement-item"
        :class="{'entry-highlight': highlighted === entry.id}"
      >
        <div class="supplement-meta">
          <CornerDownRight :size="12" class="supplement-icon" aria-hidden="true" />
          <span class="supplement-label">作者补充</span>
          <span class="meta-time">{{ formatFeedbackTime(entry.createdAt) }}</span>
          <span v-if="entry.reviewState !== 'approved'" class="state-chip">{{ reviewLabels[entry.reviewState] }}</span>
        </div>
        <p class="supplement-body">{{ entry.body }}</p>
        <p v-if="entry.reviewReason" class="entry-reason">审核说明 · {{ entry.reviewReason }}</p>
      </div>
    </div>
  </article>

  <!-- ~ 开发组官方回应（仅存在时呈现） -->
  <section v-if="official.length" class="official-section" aria-label="开发组回应">
    <article
      v-for="entry of official"
      :key="entry.id"
      :id="'feedback-entry-' + entry.id"
      class="official-item"
      :class="{'entry-highlight': highlighted === entry.id}"
    >
      <div class="official-meta">
        <span class="official-who">
          <ShieldCheck :size="13" class="official-icon" aria-hidden="true" />
          <strong>{{ entry.displayName || '开发组' }}</strong>
          <span class="official-tag">官方回应</span>
        </span>
        <span class="meta-time">{{ formatFeedbackTime(entry.createdAt) }}</span>
      </div>
      <p class="official-body">{{ entry.body }}</p>
      <p v-if="entry.reviewReason" class="entry-reason">审核说明 · {{ entry.reviewReason }}</p>
    </article>
  </section>

  <!-- ~ 简洁评论流（仅存在留言时呈现，去掉冗余标题行与空状态占位行） -->
  <section v-if="comments.length" class="comment-section" aria-label="玩家留言">
    <ul class="comment-list">
      <li
        v-for="entry of comments"
        :key="entry.id"
        :id="'feedback-entry-' + entry.id"
        class="comment-item"
        :class="{'entry-highlight': highlighted === entry.id}"
      >
        <span class="comment-avatar" aria-hidden="true">{{ initialOf(entry.displayName, '玩') }}</span>
        <div class="comment-content">
          <div class="comment-meta">
            <span class="comment-author-wrap">
              <strong class="comment-author">{{ entry.displayName || '匿名玩家' }}</strong>
              <span v-if="entry.reviewState !== 'approved'" class="state-chip">{{ reviewLabels[entry.reviewState] }}</span>
            </span>
            <span class="meta-time">{{ formatFeedbackTime(entry.createdAt) }}</span>
          </div>
          <p class="comment-body">{{ entry.body }}</p>
          <p v-if="entry.reviewReason" class="entry-reason">审核说明 · {{ entry.reviewReason }}</p>
        </div>
      </li>
    </ul>

    <n-pagination
      v-if="lastPage > 1"
      size="small"
      class="comment-pages"
      :page="detail.replies.page"
      :page-count="lastPage"
      :page-slot="5"
      :disabled="busy"
      @update:page="emit('page', $event)"
    />
  </section>
</template>

<style scoped lang="scss">
// =============== 正文与作者补充 ===============
.feedback-post {
  padding: 0;
}

.post-body {
  margin: 0;
  color: var(--ds-text-primary);
  font-size: 14.5px;
  line-height: 1.76;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.entry-reason {
  margin: 8px 0 0;
  padding-left: 10px;
  border-left: 2px solid var(--ds-error);
  color: var(--ds-error);
  font-size: 12.5px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.supplement-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 12px;
}

.supplement-item {
  padding: 2px 0 2px 10px;
  border-left: 2px solid var(--ds-border-default);
}

.supplement-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 2px;
  font-size: 11.5px;
}

.supplement-icon {
  color: var(--ds-text-muted);
}

.supplement-label {
  color: var(--ds-text-secondary);
  font-weight: 600;
}

.supplement-body {
  margin: 0;
  color: var(--ds-text-secondary);
  font-size: 13.5px;
  line-height: 1.64;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

// =============== 开发组官方回应 ===============
.official-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 14px;
}

.official-item {
  padding: 10px 12px;
  border-left: 2px solid var(--ds-warning);
  border-radius: 0 4px 4px 0;
  background: var(--ds-warning-bg);
}

.official-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 4px;
  font-size: 12px;
}

.official-who {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--ds-text-primary);
  strong {
    font-weight: 600;
  }
}

.official-icon {
  color: var(--ds-warning);
}

.official-tag {
  padding: 0 5px;
  border-radius: 3px;
  background: color-mix(in srgb, var(--ds-warning) 18%, transparent);
  color: var(--ds-warning);
  font-size: 10.5px;
  font-weight: 600;
  line-height: 1.55;
}

.official-body {
  margin: 0;
  color: var(--ds-text-primary);
  font-size: 13.5px;
  line-height: 1.65;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

// =============== 简洁评论流 ===============
.comment-section {
  margin-top: 14px;
  padding-top: 6px;
  border-top: 1px solid var(--ds-border-subtle);
}

.comment-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.comment-item {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 8px 0;

  & + .comment-item {
    border-top: 1px solid var(--ds-border-subtle);
  }
}

.comment-avatar {
  flex: none;
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  margin-top: 1px;
  border: 1px solid var(--ds-border-subtle);
  border-radius: 50%;
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-muted);
  font-size: 10.5px;
  font-weight: 600;
}

.comment-content {
  flex: 1;
  min-width: 0;
}

.comment-meta {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 2px;
}

.comment-author-wrap {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.comment-author {
  color: var(--ds-text-primary);
  font-size: 12.5px;
  font-weight: 600;
}

.meta-time {
  flex: none;
  color: var(--ds-text-muted);
  font-size: 11px;
}

.state-chip {
  padding: 0 5px;
  border-radius: 3px;
  background: var(--ds-hover-bg);
  color: var(--ds-text-muted);
  font-size: 10.5px;
  line-height: 1.5;
}

.comment-body {
  margin: 0;
  color: color-mix(in srgb, var(--ds-text-primary) 88%, transparent);
  font-size: 13px;
  line-height: 1.58;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.entry-highlight {
  padding-left: 10px;
  border-left: 2px solid var(--ds-warning);
  border-radius: 0 4px 4px 0;
  background: var(--ds-warning-bg);
}

.comment-pages {
  justify-content: center;
  margin-top: 8px;
}
</style>
