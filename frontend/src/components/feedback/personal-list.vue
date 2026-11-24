<script setup lang="ts">
import {computed} from 'vue'

import {formatFeedbackTime, noticeLabels, progressLabels, reviewLabels, type FeedbackNotice, type MyStar, type Page} from '../../types/feedback'

const props = defineProps<{
  mode: 'mine' | 'notices'
  mine: Page<MyStar>
  notices: Page<FeedbackNotice>
  busy: boolean
  error: string
}>()
const emit = defineEmits<{
  mine: [star: MyStar]
  notice: [notice: FeedbackNotice]
  page: [page: number]
  retry: []
}>()
const current = computed(() => props.mode === 'mine' ? props.mine : props.notices)
const pageCount = computed(() => Math.max(1, Math.ceil(current.value.total / current.value.pageSize)))
</script>

<template>
  <!-- ~ 我的星星与作者提醒 -->
  <!-- * 说明：在个人范围内显示审核进展、退回说明和未读更新。 -->
  <!-- * 交互：选择星星定位并读取详情，选择提醒读取当前版本。 -->
  <!-- * 依赖：Mine、Notices 的分页以及当前会话。 -->
  <!-- * 边界：已撤回条目保留状态，点击后仅显示内容不可用。 -->
  <div class="personal-list">
    <div v-if="error" class="personal-error" role="alert">
      {{ error }} <n-button size="tiny" @click="emit('retry')">重试</n-button>
    </div>
    <p v-if="busy" role="status">读取中…</p>
    <p v-else-if="!current.list?.length && !error">{{ mode === 'mine' ? '你还没有发布星星。' : '暂无提醒。' }}</p>
    <ul v-if="mode === 'mine'">
      <li v-for="star in mine.list" :key="star.starId">
        <button class="personal-row" type="button" @click="emit('mine', star)">
          <strong class="personal-preview">{{ star.preview || '反馈星' }}</strong>
          <span class="personal-meta">
            <span>{{ star.withdrawn ? '已撤回' : reviewLabels[star.reviewState] }}</span>
            <span v-if="!star.withdrawn">{{ progressLabels[star.progress] }}</span>
            <span v-if="star.newComment && !star.withdrawn" class="personal-new">有新留言</span>
            <span>{{ formatFeedbackTime(star.createdAt) }}</span>
          </span>
          <span v-if="star.reviewReason && !star.withdrawn" class="personal-error">{{ star.reviewReason }}</span>
        </button>
      </li>
    </ul>
    <ul v-else>
      <li v-for="notice in notices.list" :key="notice.id">
        <button class="personal-row" type="button" @click="emit('notice', notice)">
          <strong>{{ noticeLabels[notice.kind] }}</strong>
          <span class="personal-meta">
            <span v-if="!notice.readAt" class="personal-new">未读</span>
            <span>{{ formatFeedbackTime(notice.createdAt) }}</span>
          </span>
        </button>
      </li>
    </ul>
    <n-pagination v-if="pageCount > 1" :page="current.page" :page-count="pageCount" :page-slot="4"
      :disabled="busy" @update:page="emit('page', $event)" />
  </div>
</template>

<style scoped lang="scss">
.personal-list ul { margin: 0 0 14px; padding: 0; list-style: none; }
.personal-list li + li { border-top: 1px solid var(--ds-border-subtle); }
.personal-list p { color: var(--ds-text-secondary); font-size: 13px; }
.personal-row {
  display: grid; gap: 5px; width: 100%; min-width: 0; padding: 11px 5px; text-align: left;
  border: 0; border-radius: 4px; color: var(--ds-text-primary); background: transparent; cursor: pointer;
  &:hover { background: var(--ds-bg-panel); }
  &:focus-visible { outline: 2px solid var(--ds-brand); outline-offset: -2px; }
}
.personal-row strong { font-size: 14px; font-weight: 600; }
.personal-preview { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.personal-meta { display: flex; flex-wrap: wrap; gap: 3px 9px; color: var(--ds-text-secondary); font-size: 12px; }
.personal-new { color: var(--ds-info); font-weight: 600; }
.personal-error { color: var(--ds-error); font-size: 13px; overflow-wrap: anywhere; }
</style>
