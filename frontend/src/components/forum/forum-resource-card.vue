<script setup lang="ts">
import {
  AlertCircle,
  Copy,
  Download,
  ExternalLink,
  FileArchive,
  FileCode2,
  Film,
  Flag,
  Package,
  Store,
  ZoomIn,
} from '@lucide/vue'
import {computed, ref, watch} from 'vue'
import {BrowserOpenURL} from '../../../wailsjs/runtime/runtime'
import {buildForumMediaUrl} from '../../services/forum'
import type {ForumBlockState, ForumResource} from '../../types/forum'
import {formatFileSize} from '../../utils/format'

const props = defineProps<{
  resourceId: number
  state: ForumBlockState
  resource?: ForumResource
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

const mediaLoadFailed = ref(false)
const lightboxOpen = ref(false)

const isTakenDown = computed(() => props.state !== 'visible' || !props.resource)
const takenDownText = computed(() => (props.state === 'unavailable' ? '资源暂不可用' : '资源已下架'))
const mediaUrl = computed(() => (props.resource ? buildForumMediaUrl(props.resource.id) : ''))
watch(mediaUrl, () => {
  mediaLoadFailed.value = false
})
const isBusy = computed(() => Boolean(props.busyAction && props.resource && props.busyAction.startsWith(`${props.resource.id}:`)))

const modSubtitle = computed(() => {
  const res = props.resource
  if (!res) return ''
  if (res.source === 'thunderstore') {
    const coord = [res.namespace, res.packageName].filter(Boolean).join(' / ')
    const version = res.version ? `v${res.version}` : '最新版 / 包主页'
    return [coord || 'Thunderstore 模组', version].join(' · ')
  }
  return `托管模组压缩包 (.zip) · ${formatFileSize(res.size ?? 0)}`
})

const blueprintSubtitle = computed(() => {
  const res = props.resource
  if (!res) return ''
  if (res.source === 'inline') {
    return '内联蓝图编码 · 可直接复制或导入蓝图库'
  }
  return `托管蓝图文件 (.txt) · ${formatFileSize(res.size ?? 0)}`
})

function openExternalUrl(url?: string): void {
  if (url && /^https?:\/\//i.test(url)) {
    BrowserOpenURL(url)
  }
}
</script>

<template>
  <!-- ~ 论坛阅读资源块 -->
  <!-- * 说明：按接口确认的位置渲染图片、MP4 播放器、蓝图/模组/存档卡或原位下架占位 -->
  <!-- * 交互：蓝图支持复制编码/导入库，存档支持校验导入，模组支持打开市场或导入本地库，所有可见资源均可举报 -->
  <!-- * 边界：下架或不可用块保留原位置并阻断下载与预览 -->
  <div v-if="isTakenDown" class="forum-resource-takedown" role="status">
    <AlertCircle :size="15" />
    <span>{{ takenDownText }}</span>
  </div>

  <figure v-else-if="resource && resource.type === 'image'" class="forum-media-block">
    <div v-if="mediaLoadFailed" class="forum-resource-takedown">
      <AlertCircle :size="15" />
      <span>图片加载失败或已下架</span>
    </div>
    <template v-else>
      <div class="forum-image-stage" @click="lightboxOpen = true">
        <img :src="mediaUrl" :alt="resource.name" loading="lazy" @error="mediaLoadFailed = true" />
        <span class="image-zoom-hint"><ZoomIn :size="14" />查看大图</span>
      </div>
      <figcaption class="forum-media-caption">
        <span class="caption-name" :title="resource.name">{{ resource.name }}</span>
        <span v-if="resource.size" class="caption-size">{{ formatFileSize(resource.size) }}</span>
        <button type="button" class="caption-report" @click="emit('report-resource', resource)">
          <Flag :size="12" />举报资源
        </button>
      </figcaption>
    </template>

    <n-modal v-model:show="lightboxOpen" preset="card" :title="resource.name" class="forum-lightbox-modal" style="width: min(92vw, 960px);">
      <div class="forum-lightbox-body">
        <img :src="mediaUrl" :alt="resource.name" />
      </div>
    </n-modal>
  </figure>

  <figure v-else-if="resource && resource.type === 'video'" class="forum-media-block">
    <div v-if="mediaLoadFailed" class="forum-resource-takedown">
      <AlertCircle :size="15" />
      <span>视频加载失败或已下架</span>
    </div>
    <template v-else>
      <div class="forum-video-stage">
        <video :src="mediaUrl" controls preload="metadata" @error="mediaLoadFailed = true" />
      </div>
      <figcaption class="forum-media-caption">
        <span class="caption-name" :title="resource.name"><Film :size="13" />{{ resource.name }}</span>
        <span v-if="resource.size" class="caption-size">{{ formatFileSize(resource.size) }}</span>
        <button type="button" class="caption-report" @click="emit('report-resource', resource)">
          <Flag :size="12" />举报资源
        </button>
      </figcaption>
    </template>
  </figure>

  <div v-else-if="resource" class="forum-game-card ds-bracket-hover">
    <div class="card-icon" :class="`is-${resource.type}`">
      <FileCode2 v-if="resource.type === 'blueprint'" :size="18" />
      <FileArchive v-else-if="resource.type === 'save'" :size="18" />
      <Package v-else :size="18" />
    </div>

    <div class="card-info">
      <div class="card-title" :title="resource.name">{{ resource.name }}</div>
      <div class="card-sub">
        <template v-if="resource.type === 'blueprint'">{{ blueprintSubtitle }}</template>
        <template v-else-if="resource.type === 'save'">戴森球主存档 (.dsv) · {{ formatFileSize(resource.size ?? 0) }}</template>
        <template v-else>{{ modSubtitle }}</template>
      </div>
    </div>

    <div class="card-actions">
      <template v-if="resource.type === 'blueprint'">
        <n-button size="small" secondary :loading="busyAction === `${resource.id}:copy-blueprint`" :disabled="isBusy" @click="emit('copy-blueprint', resource)">
          <template #icon><Copy :size="14" /></template>
          复制编码
        </n-button>
        <n-button size="small" type="primary" :loading="busyAction === `${resource.id}:import-blueprint`" :disabled="isBusy" @click="emit('import-blueprint', resource)">
          <template #icon><Download :size="14" /></template>
          导入蓝图库
        </n-button>
      </template>

      <template v-else-if="resource.type === 'save'">
        <n-button size="small" type="primary" :loading="busyAction === `${resource.id}:import-save`" :disabled="isBusy" @click="emit('import-save', resource)">
          <template #icon><Download :size="14" /></template>
          导入到存档
        </n-button>
      </template>

      <template v-else-if="resource.type === 'mod' && resource.source === 'thunderstore'">
        <n-button size="small" type="primary" :loading="busyAction === `${resource.id}:open-thunderstore`" :disabled="isBusy" @click="emit('open-thunderstore', resource)">
          <template #icon><Store :size="14" /></template>
          {{ resource.version ? `打开 v${resource.version}` : '模组市场查看' }}
        </n-button>
        <n-tooltip v-if="resource.url">
          <template #trigger>
            <n-button size="small" quaternary @click="openExternalUrl(resource.url)">
              <template #icon><ExternalLink :size="14" /></template>
            </n-button>
          </template>
          在浏览器打开 Thunderstore 页面
        </n-tooltip>
      </template>

      <template v-else-if="resource.type === 'mod'">
        <n-button size="small" type="primary" :loading="busyAction === `${resource.id}:import-mod`" :disabled="isBusy" @click="emit('import-mod', resource)">
          <template #icon><Download :size="14" /></template>
          导入模组库
        </n-button>
      </template>

      <n-tooltip>
        <template #trigger>
          <n-button size="small" quaternary :disabled="isBusy" aria-label="举报资源" @click="emit('report-resource', resource)">
            <template #icon><Flag :size="14" /></template>
          </n-button>
        </template>
        举报此资源
      </n-tooltip>
    </div>
  </div>
</template>

<style scoped lang="scss">
.forum-resource-takedown {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 12px 0;
  padding: 10px 14px;
  border-radius: 4px;
  border: 1px dashed var(--ds-border-default);
  background: var(--ds-hover-bg);
  color: var(--ds-text-muted);
  font-size: 13px;
}

.forum-media-block {
  margin: 14px 0;
  border-radius: 4px;
  border: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel);
  overflow: hidden;
}

.forum-image-stage {
  position: relative;
  display: flex;
  justify-content: center;
  padding: 10px;
  cursor: zoom-in;

  img {
    max-width: 100%;
    max-height: 460px;
    border-radius: 4px;
    object-fit: contain;
  }

  .image-zoom-hint {
    position: absolute;
    right: 16px;
    bottom: 16px;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 4px 8px;
    border-radius: 4px;
    background: rgb(0 0 0 / 0.58);
    color: #fff;
    font-size: 11px;
    opacity: 0;
    transition: opacity 0.15s ease;
  }

  &:hover .image-zoom-hint {
    opacity: 1;
  }
}

.forum-video-stage {
  background: #000;
  display: flex;
  justify-content: center;

  video {
    width: 100%;
    max-height: 460px;
    outline: none;
  }
}

.forum-media-caption {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  border-top: 1px solid var(--ds-border-subtle);
  font-size: 12px;
  color: var(--ds-text-muted);
}

.caption-name {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.caption-size {
  flex-shrink: 0;
  font-family: var(--ds-font-mono);
}

.caption-report {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: none;
  background: transparent;
  color: var(--ds-text-muted);
  font-size: 12px;
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 4px;

  &:hover {
    color: var(--ds-error);
    background: var(--ds-error-bg);
  }
}

.forum-game-card {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 14px 0;
  padding: 12px 14px;
  border-radius: 4px;
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel);
  transition: background-color 150ms ease, border-color 150ms ease;

  &:hover {
    border-color: transparent;
    background: var(--ds-bg-panel-elevated);
  }
}

.card-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 4px;
  background: var(--ds-active-bg);
  color: var(--ds-text-primary);
  flex-shrink: 0;

  &.is-blueprint {
    color: var(--ds-res-on);
    background: var(--ds-res-on-soft);
  }

  &.is-save {
    color: var(--ds-save-accent);
    background: rgb(40 120 196 / 0.1);
  }
}

.card-info {
  flex: 1;
  min-width: 0;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ds-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-sub {
  margin-top: 3px;
  font-size: 12px;
  color: var(--ds-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.forum-lightbox-body {
  display: flex;
  justify-content: center;
  max-height: 76vh;
  overflow: auto;

  img {
    max-width: 100%;
    max-height: 74vh;
    object-fit: contain;
  }
}
</style>
