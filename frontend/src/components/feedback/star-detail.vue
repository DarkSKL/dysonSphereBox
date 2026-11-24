<script setup lang="ts">
import {computed, nextTick, watch} from 'vue'
import {Heart} from '@lucide/vue'

import {formatFeedbackTime, progressLabels, reviewLabels, type Entry, type EntryKind, type StarDetail} from '../../types/feedback'

const props = defineProps<{
  detail: StarDetail
  busy: boolean
  writing: boolean
  highlighted: string
}>()
const emit = defineEmits<{
  like: []
  edit: []
  supplement: []
  comment: []
  withdraw: []
  page: [value: number]
}>()

const allReplies = computed(() => props.detail.replies.list ?? [])
// 置顶分组有数量上限。翻页时把超出上限的同类条目合并到对应分组。
function groupedReplies(kind: EntryKind, pinned: Entry[] | null | undefined): Entry[] {
  const group = new Map((pinned ?? []).map((entry) => [entry.id, entry]))
  for (const entry of allReplies.value) if (entry.kind === kind) group.set(entry.id, entry)
  return [...group.values()]
}
const official = computed(() => groupedReplies('official', props.detail.officialReplies))
const supplements = computed(() => groupedReplies('supplement', props.detail.supplements).sort((left, right) =>
  left.createdAt.localeCompare(right.createdAt) || left.id.length - right.id.length || left.id.localeCompare(right.id)))
const otherReplies = computed(() => groupedReplies('comment', []))
const groups = computed(() => [
  {label: '开发组回应', entries: official.value},
  {label: '作者补充', entries: supplements.value},
  {label: '玩家留言', entries: otherReplies.value},
])
const published = computed(() => props.detail.original.reviewState === 'approved')
const lastPage = computed(() => Math.max(1, Math.ceil(props.detail.replies.total / props.detail.replies.pageSize)))

watch(() => props.highlighted, async (id) => {
  if (!id) return
  await nextTick()
  document.getElementById('feedback-entry-' + id)?.scrollIntoView({block: 'nearest'})
}, {immediate: true})
</script>

<template>
  <!-- ~ 正文与作者操作 -->
  <!-- * 说明：原文保留署名和审核进度，处理进展单独标注。 -->
  <!-- * 交互：使用当前正文版本互动；编辑、补充和撤回由作者权限决定。 -->
  <!-- * 依赖：StarDetail 的可见文本、进展与操作许可。 -->
  <!-- * 边界：待审核的正文可以阅读自己的稿件，公开互动仅对已通过的星开放。 -->
  <article class="feedback-original" :id="'feedback-entry-' + detail.original.id"
    :class="{'entry-highlight': highlighted === detail.original.id}">
    <div class="entry-meta">
      <strong>{{ detail.original.displayName || '匿名玩家' }}</strong>
      <span>{{ formatFeedbackTime(detail.original.createdAt) }}</span>
    </div>
    <div class="entry-meta">
      <span class="entry-progress" :data-progress="detail.progress">{{ progressLabels[detail.progress] }}</span>
      <span v-if="detail.releaseVersion">版本 {{ detail.releaseVersion }}</span>
      <span v-if="detail.owned">{{ reviewLabels[detail.original.reviewState] }}</span>
    </div>
    <p class="entry-body">{{ detail.original.body }}</p>
    <p v-if="detail.reviewReason" class="entry-reason">审核说明 · {{ detail.reviewReason }}</p>
    <div class="entry-actions">
      <n-button v-if="published" size="small" :type="detail.liked ? 'primary' : 'default'"
        :aria-pressed="detail.liked" :disabled="writing || busy" @click="emit('like')">
        <template #icon><Heart :size="15" aria-hidden="true" /></template>
        {{ detail.liked ? '已点赞' : '点赞' }} · {{ detail.likeCount }}
      </n-button>
      <n-button v-if="published" size="small" :disabled="writing || busy" @click="emit('comment')">写留言</n-button>
      <n-button v-if="detail.owned && detail.canEdit" size="small" :disabled="writing || busy" @click="emit('edit')">编辑正文</n-button>
      <n-button v-if="detail.owned && !detail.canEdit && published" size="small" :disabled="writing || busy"
        @click="emit('supplement')">追加补充</n-button>
      <n-button v-if="detail.owned" size="small" quaternary type="error" :disabled="writing"
        @click="emit('withdraw')">撤回</n-button>
    </div>
    <slot />
  </article>

  <!-- ~ 回应、补充和留言 -->
  <!-- * 说明：按职责呈现同一星下的发言并按 ID 去重。 -->
  <!-- * 交互：页码改变只更新讨论，原文和详情目标保留。 -->
  <!-- * 依赖：详情的置顶分组和 replies 分页。 -->
  <!-- * 边界：对作者可读的待审或退回发言保留文字状态。 -->
  <section v-for="group of groups" :key="group.label" class="entry-group" v-show="group.entries.length || (group.label === '玩家留言' && published)">
    <h3>{{ group.label }}</h3>
    <p v-if="!group.entries.length" class="empty-discussion">暂无可显示的留言</p>
    <article v-for="entry of group.entries" :key="entry.id" :id="'feedback-entry-' + entry.id"
      class="entry-reply" :class="{'entry-highlight': highlighted === entry.id}">
      <div class="entry-meta">
        <strong>{{ entry.displayName || (entry.kind === 'official' ? '开发组' : '匿名玩家') }}</strong>
        <span>{{ formatFeedbackTime(entry.createdAt) }}</span>
        <span v-if="entry.reviewState !== 'approved'">{{ reviewLabels[entry.reviewState] }}</span>
      </div>
      <p class="entry-body">{{ entry.body }}</p>
      <p v-if="entry.reviewReason" class="entry-reason">审核说明 · {{ entry.reviewReason }}</p>
    </article>
  </section>
  <n-pagination v-if="lastPage > 1" class="discussion-pages" :page="detail.replies.page" :page-count="lastPage"
    :page-slot="4" :disabled="busy" @update:page="emit('page', $event)" />
</template>

<style scoped lang="scss">
.feedback-original, .entry-group { padding: 0 0 16px; margin-bottom: 16px; border-bottom: 1px solid var(--ds-border-subtle); }
.entry-meta { display: flex; align-items: center; flex-wrap: wrap; gap: 4px 9px; color: var(--ds-text-secondary); font-size: 12px; }
.entry-meta strong { color: var(--ds-text-primary); font-weight: 600; }
.entry-meta + .entry-meta { margin-top: 7px; }
.entry-progress { font-weight: 600; }
.entry-progress[data-progress='implemented'] { color: var(--ds-warning); }
.entry-body { white-space: pre-wrap; overflow-wrap: anywhere; margin: 12px 0; line-height: 1.65; }
.entry-reason { margin: 10px 0; color: var(--ds-error); white-space: pre-wrap; overflow-wrap: anywhere; font-size: 13px; }
.entry-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; margin: 12px 0; }
.entry-group h3 { font-size: 14px; font-weight: 600; margin: 0 0 8px; }
.entry-reply { padding: 9px 0 2px; }
.entry-reply + .entry-reply { border-top: 1px solid var(--ds-border-subtle); }
.entry-reply .entry-body { margin: 6px 0 9px; }
.entry-highlight { outline: 1px solid var(--ds-warning); outline-offset: 4px; }
.empty-discussion { margin: 0; color: var(--ds-text-secondary); font-size: 13px; }
.discussion-pages { justify-content: center; margin-bottom: 8px; }
</style>
