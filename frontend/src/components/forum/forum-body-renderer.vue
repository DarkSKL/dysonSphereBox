<script setup lang="ts">
import {computed} from 'vue'
import {BrowserOpenURL} from '../../../wailsjs/runtime/runtime'
import type {ForumBodyBlock, ForumResource} from '../../types/forum'
import {buildForumReadingSegments} from '../../utils/forum-markdown'
import ForumResourceCard from './forum-resource-card.vue'

const props = defineProps<{
  body: string
  bodyFormat?: string
  bodyBlocks?: ForumBodyBlock[]
  resources?: ForumResource[]
  busyAction?: string
}>()

const emit = defineEmits<{
  (event: 'copy-blueprint', resource: ForumResource): void
  (event: 'import-blueprint', resource: ForumResource): void
  (event: 'import-save', resource: ForumResource): void
  (event: 'import-mod', resource: ForumResource): void
  (event: 'open-thunderstore', resource: ForumResource): void
  (event: 'report-resource', resource: ForumResource): void
}>()

const isMarkdown = computed(() => props.bodyFormat === 'markdown')
const segments = computed(() => (
  isMarkdown.value
    ? buildForumReadingSegments(props.body, props.bodyBlocks, props.resources)
    : []
))

// 拦截正文中的安全外链点击，改由桌面系统浏览器打开，避免 WebView 跳离应用。
function handleMarkdownClick(event: MouseEvent): void {
  const target = event.target as HTMLElement | null
  const anchor = target?.closest('a[data-forum-link]') as HTMLAnchorElement | null
  if (!anchor) return
  event.preventDefault()
  const href = anchor.getAttribute('data-forum-link') || anchor.getAttribute('href') || ''
  if (/^https?:\/\//i.test(href)) {
    BrowserOpenURL(href)
  }
}
</script>

<template>
  <!-- ~ 论坛正文阅读渲染器 -->
  <!-- * 说明：Markdown 帖全文解析并按 bodyBlocks 确认位置插入资源卡；旧版纯文本帖按原文+底部附件列表渲染 -->
  <!-- * 交互：点击外链由系统浏览器打开，点击资源卡触发复制/导入/市场/举报 -->
  <!-- * 边界：HTML 片段已经 DOMPurify 净化，禁用原始 HTML 与外部图片直连 -->
  <div class="forum-body-renderer" @click="handleMarkdownClick">
    <template v-if="isMarkdown">
      <template v-for="(segment, index) in segments" :key="`${segment.kind}-${index}`">
        <div v-if="segment.kind === 'html'" class="forum-markdown-prose" v-html="segment.html" />
        <ForumResourceCard
          v-else
          :resource-id="segment.resourceId"
          :state="segment.state"
          :resource="segment.resource"
          :busy-action="busyAction"
          @copy-blueprint="emit('copy-blueprint', $event)"
          @import-blueprint="emit('import-blueprint', $event)"
          @import-save="emit('import-save', $event)"
          @import-mod="emit('import-mod', $event)"
          @open-thunderstore="emit('open-thunderstore', $event)"
          @report-resource="emit('report-resource', $event)"
        />
      </template>
    </template>

    <template v-else>
      <div class="forum-plain-body">{{ body }}</div>
      <div v-if="resources && resources.length > 0" class="forum-legacy-attachments">
        <div class="legacy-title">附带资源</div>
        <ForumResourceCard
          v-for="item in resources"
          :key="item.id"
          :resource-id="item.id"
          state="visible"
          :resource="item"
          :busy-action="busyAction"
          @copy-blueprint="emit('copy-blueprint', $event)"
          @import-blueprint="emit('import-blueprint', $event)"
          @import-save="emit('import-save', $event)"
          @import-mod="emit('import-mod', $event)"
          @open-thunderstore="emit('open-thunderstore', $event)"
          @report-resource="emit('report-resource', $event)"
        />
      </div>
    </template>
  </div>
</template>

<style scoped lang="scss">
.forum-plain-body {
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.75;
  font-size: 14px;
  color: var(--ds-text-primary);
}

.forum-legacy-attachments {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--ds-border-subtle);
}

.legacy-title {
  margin-bottom: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--ds-text-muted);
}

.forum-markdown-prose {
  font-size: 14px;
  line-height: 1.75;
  color: var(--ds-text-primary);
  word-break: break-word;

  :deep(h1),
  :deep(h2),
  :deep(h3),
  :deep(h4) {
    margin: 16px 0 8px;
    font-weight: 700;
    line-height: 1.4;
    color: var(--ds-text-primary);
  }

  :deep(h1) { font-size: 20px; }
  :deep(h2) { font-size: 17px; }
  :deep(h3) { font-size: 15px; }

  :deep(p) {
    margin: 0 0 10px;
  }

  :deep(p:last-child) {
    margin-bottom: 0;
  }

  :deep(ul),
  :deep(ol) {
    margin: 6px 0 10px;
    padding-left: 22px;
  }

  :deep(ul) { list-style: disc; }
  :deep(ol) { list-style: decimal; }

  :deep(blockquote) {
    margin: 10px 0;
    padding: 6px 12px;
    border-left: 3px solid var(--ds-border-strong);
    background: var(--ds-hover-bg);
    color: var(--ds-text-secondary);
  }

  :deep(code) {
    padding: 1px 5px;
    border-radius: 4px;
    background: var(--ds-active-bg);
    font-family: var(--ds-font-mono);
    font-size: 12.5px;
  }

  :deep(pre) {
    margin: 10px 0;
    padding: 12px 14px;
    border-radius: 6px;
    border: 1px solid var(--ds-border-subtle);
    background: var(--ds-bg-panel);
    overflow-x: auto;

    code {
      padding: 0;
      background: transparent;
      font-size: 12.5px;
      line-height: 1.6;
    }
  }

  :deep(hr) {
    margin: 14px 0;
    border: none;
    border-top: 1px solid var(--ds-border-subtle);
  }

  :deep(a) {
    color: var(--ds-brand);
    text-decoration: underline;
    text-underline-offset: 2px;
    cursor: pointer;

    &:hover {
      opacity: 0.82;
    }
  }

  :deep(table) {
    width: 100%;
    border-collapse: collapse;
    margin: 10px 0;
    font-size: 13px;

    th, td {
      padding: 6px 10px;
      border: 1px solid var(--ds-border-default);
      text-align: left;
    }

    th {
      background: var(--ds-bg-panel);
      font-weight: 600;
    }
  }
}
</style>
