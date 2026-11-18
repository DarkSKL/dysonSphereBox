<script setup lang="ts">
import {computed} from 'vue'
import {BrowserOpenURL} from '../../../wailsjs/runtime/runtime'
import {renderReleaseNotes} from '../../utils/release-notes'

const props = defineProps<{source?: string; emptyText: string}>()
const html = computed(() => renderReleaseNotes(props.source ?? ''))

// =============== 发布说明链接 ===============
// 说明：正文链接在渲染时已经移除 href，只由鼠标主键、中键和 Enter 触发，并交给系统浏览器打开。
// 边界：忽略右键和没有地址的目标，避免把应用 WebView 导航到远端页面。
const openReleaseNoteLink = (event: MouseEvent | KeyboardEvent): void => {
  if (!(event.target instanceof Element)) return
  const target = event.target.closest<HTMLAnchorElement>('a[data-external-href]')
  const href = target?.dataset.externalHref
  if (!href || (event instanceof MouseEvent && event.button > 1)) return
  event.preventDefault()
  event.stopPropagation()
  void BrowserOpenURL(href)
}
</script>

<template>
  <!-- 设置页和标题栏弹窗共用清洗后的正文、外链交互和排版，各入口自行控制滚动区尺寸。 -->
  <div class="release-notes">
    <div
      v-if="html"
      class="release-notes-markdown"
      v-html="html"
      @click="openReleaseNoteLink"
      @auxclick.prevent="openReleaseNoteLink"
      @keydown.enter="openReleaseNoteLink"
    />
    <p v-else>{{ emptyText }}</p>
  </div>
</template>

<style scoped lang="scss">
.release-notes {
  overflow-wrap: anywhere;
}

.release-notes-markdown {
  :deep(> :first-child) { margin-top: 0; }
  :deep(> :last-child) { margin-bottom: 0; }
  :deep(p) { margin: 8px 0; }
  :deep(h1), :deep(h2), :deep(h3), :deep(h4), :deep(h5), :deep(h6) {
    margin: 14px 0 6px;
    color: var(--ds-text-primary);
    font-weight: 700;
    line-height: 1.35;
  }
  :deep(h1) { font-size: 17px; }
  :deep(h2) { font-size: 16px; }
  :deep(h3) { font-size: 14px; }
  :deep(h4), :deep(h5), :deep(h6) { font-size: 13px; }
  :deep(strong) { color: var(--ds-text-primary); font-weight: 600; }
  :deep(ul), :deep(ol) { margin: 6px 0; padding-left: 1.6em; }
  :deep(ul) { list-style: disc; }
  :deep(ol) { list-style: decimal; }
  :deep(li) { margin: 2px 0; }
  :deep(li > p) { margin: 2px 0; }
  :deep(blockquote) {
    margin: 8px 0;
    padding-left: 10px;
    border-left: 3px solid var(--ds-border-subtle);
    color: var(--ds-text-muted);
  }
  :deep(hr) { margin: 12px 0; border: 0; border-top: 1px solid var(--ds-border-subtle); }
  :deep(a[data-external-href]) {
    color: var(--ds-brand);
    text-decoration: underline;
    cursor: pointer;
  }
  :deep(a[data-external-href]:focus-visible) { outline: 2px solid var(--ds-brand); outline-offset: 2px; }
  :deep(code) {
    padding: 1px 4px;
    border-radius: 3px;
    background: var(--ds-bg-panel);
    font-family: var(--ds-font-mono);
    font-size: 12px;
  }
  :deep(pre) {
    margin: 8px 0;
    padding: 10px 12px;
    overflow: auto;
    border-radius: 4px;
    background: var(--ds-bg-panel);
  }
  :deep(pre code) { padding: 0; background: transparent; }
  :deep(table) { width: 100%; margin: 10px 0; border-collapse: collapse; font-size: 12px; }
  :deep(th), :deep(td) { padding: 5px 8px; border: 1px solid var(--ds-border-subtle); text-align: left; }
  :deep(th) { background: var(--ds-bg-panel); color: var(--ds-text-primary); font-weight: 600; }
  :deep(img) { max-width: 100%; height: auto; border-radius: 4px; }
}
</style>
