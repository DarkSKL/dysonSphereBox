<script setup lang="ts">
import {AlertCircle, CheckCircle2, Download, LoaderCircle, PackageOpen, Trash2, Wrench, X} from '@lucide/vue'
import {onActivated, onBeforeUnmount, onDeactivated, onMounted, ref} from 'vue'

const props = defineProps<{
  packageKey: string
  title: string
  description: string
  imageUrl: string
  fileCount: number
  downloadCount: number
  author: string
  installed: boolean
  installing: boolean
  failed: boolean
  conflicted: boolean
  progress: number
  progressStatus: string
  downloadSpeed: string
  failureMessage: string
  translationStatus?: string
}>()

const emit = defineEmits<{
  (event: 'open', key: string): void
  (event: 'uninstall', key: string): void
  (event: 'repair', key: string): void
  (event: 'cancel', key: string): void
  (event: 'description-visible', key: string, visible: boolean): void
  (event: 'retry-translation'): void
}>()

// 说明：只观察描述所在元素，不预取视口之外的卡片；缓存页停用和卸载时清理观察器。
const descriptionElement = ref<HTMLElement | null>(null)
let descriptionObserver: IntersectionObserver | undefined
function observeDescription(): void {
  descriptionObserver?.disconnect()
  if (!descriptionElement.value) return
  descriptionObserver = new IntersectionObserver((entries) => {
    emit('description-visible', props.packageKey, entries.some((entry) => entry.isIntersecting))
  })
  descriptionObserver.observe(descriptionElement.value)
}
function stopObservingDescription(): void {
  descriptionObserver?.disconnect()
  emit('description-visible', props.packageKey, false)
}
onMounted(observeDescription)
onActivated(observeDescription)
onDeactivated(stopObservingDescription)
onBeforeUnmount(stopObservingDescription)
</script>

<template>
  <div
    class="mod-pack-card group"
    :class="{ 'is-installed': installed, 'is-installing': installing, 'is-failed': failed, 'is-conflicted': conflicted }"
    role="button"
    tabindex="0"
    @click="!installing && $emit('open', packageKey)"
    @keydown.enter.prevent="!installing && $emit('open', packageKey)"
    @keydown.space.prevent="!installing && $emit('open', packageKey)"
  >
    <div class="mod-pack-card__header flex shrink-0 items-start justify-between gap-3 p-2.5">
      <div class="min-w-0">
        <h3 class="mod-pack-card__title truncate text-sm font-semibold" :title="title">{{ title }}</h3>
        <p ref="descriptionElement" class="mod-pack-card__description mt-1 line-clamp-1 text-xs leading-5" :title="description">{{ description }}</p>
        <n-button
          v-if="translationStatus === 'error'"
          text size="tiny"
          title="翻译暂不可用，已显示原文。请检查登录状态后重试。"
          @click.stop="$emit('retry-translation')"
          @keydown.enter.stop
          @keydown.space.stop
        >重试翻译</n-button>
        <p class="mod-pack-card__author mt-1 truncate text-[11px]">{{ author || '未知作者' }}</p>
      </div>
      <!-- * 说明：右上角依次为翻译加载圈与安装相关操作；加载圈仅表示状态，不可点击 -->
      <div class="flex shrink-0 items-center gap-1.5">
        <span
          v-if="translationStatus === 'loading'"
          class="mod-pack-card__translating grid h-7 w-5 place-items-center"
          role="status"
          aria-label="正在翻译"
          title="正在翻译"
        >
          <LoaderCircle :size="14" :stroke-width="1.9" class="mod-pack-card__spinner" />
        </span>
        <button
          v-if="installing"
          class="mod-pack-card__icon-action grid h-7 w-7 shrink-0 place-items-center rounded opacity-100 transition"
          type="button"
          title="取消安装"
          @click.stop="$emit('cancel', packageKey)"
        >
          <X :size="14" :stroke-width="1.8" />
        </button>
        <button
          v-else-if="conflicted"
          class="mod-pack-card__icon-action is-warning grid h-7 w-7 shrink-0 place-items-center rounded opacity-100 transition"
          type="button"
          title="修复安装冲突"
          @click.stop="$emit('repair', packageKey)"
        >
          <Wrench :size="14" :stroke-width="1.8" />
        </button>
        <button
          v-else-if="installed && !installing"
          class="mod-pack-card__icon-action is-danger grid h-7 w-7 shrink-0 place-items-center rounded opacity-0 transition group-hover:opacity-100"
          type="button"
          title="卸载"
          @click.stop="$emit('uninstall', packageKey)"
        >
          <Trash2 :size="14" :stroke-width="1.8" />
        </button>
      </div>
    </div>

    <div v-if="installing" class="mod-pack-card__progress-track h-1">
      <div class="mod-pack-card__progress h-full transition-all duration-300" :style="{width: `${progress}%`}"></div>
    </div>
    <div v-else-if="failed || conflicted" class="mod-pack-card__progress-track is-danger h-1">
      <div class="mod-pack-card__progress is-danger h-full w-full"></div>
    </div>

    <div class="mod-pack-card__media relative grid flex-1 place-items-center border-t">
      <img
        v-if="imageUrl"
        class="absolute inset-0 h-full w-full object-cover"
        :src="imageUrl"
        :alt="title"
        loading="lazy"
      />
      <div v-else class="mod-pack-emblem">
        <Wrench :size="34" :stroke-width="1.7" />
      </div>
      <div v-if="imageUrl" class="absolute inset-x-0 bottom-0 h-16 bg-gradient-to-t from-black/45 to-transparent"></div>
      <div class="absolute bottom-2 right-2 flex max-w-[calc(100%-1rem)] gap-1">
        <div class="mod-pack-card__stat rounded px-2 py-0.5 text-xs">
          <PackageOpen :size="12" :stroke-width="1.8" class="mr-1 inline-block align-[-2px]" />
          <span><span class="type-display">{{ fileCount }}</span> 个组件</span>
        </div>
        <div class="mod-pack-card__stat hidden rounded px-2 py-0.5 text-xs sm:block">
          <Download :size="12" :stroke-width="1.8" class="mr-1 inline-block align-[-2px]" />
          <span class="type-display">{{ downloadCount }}</span>
        </div>
      </div>
      <div
        v-if="installed && !failed && !installing"
        class="mod-pack-card__status-badge is-success absolute right-2 top-2 flex h-7 w-7 items-center justify-center rounded"
        title="已安装"
      >
        <CheckCircle2 :size="12" :stroke-width="1.9" />
      </div>
      <button
        v-if="conflicted && !installing"
        class="mod-pack-card__status-badge is-warning absolute right-2 top-2 flex h-7 w-7 items-center justify-center rounded transition"
        type="button"
        title="修复安装冲突"
        @click.stop="$emit('repair', packageKey)"
      >
        <Wrench :size="12" :stroke-width="1.9" />
      </button>
      <div
        v-else-if="failed && !installing"
        class="mod-pack-card__status-badge is-warning absolute right-2 top-2 flex h-7 w-7 items-center justify-center rounded"
        :title="conflicted ? '待处理冲突' : '安装失败'"
      >
        <AlertCircle :size="12" :stroke-width="1.9" />
      </div>
      <div v-if="installing" class="mod-pack-card__installing absolute inset-0 grid place-items-center backdrop-blur-sm">
        <div class="text-center">
          <p class="mod-pack-card__title max-w-[9rem] truncate text-sm font-semibold">{{ progressStatus || '安装中' }}</p>
          <p class="mod-pack-card__description mt-1 text-xs">{{ Math.floor(progress) }}%<span v-if="downloadSpeed"> · {{ downloadSpeed }}</span></p>
          <button class="mod-pack-card__cancel mt-2 px-2 py-1 text-xs" type="button" @click.stop="$emit('cancel', packageKey)">
            取消
          </button>
        </div>
      </div>
      <div v-else-if="failed || conflicted" class="mod-pack-card__failure absolute inset-x-2 bottom-2 px-2 py-1">
        <p class="truncate text-xs font-semibold">{{ progressStatus || (conflicted ? '待处理冲突' : '安装失败') }}</p>
        <p class="mt-0.5 line-clamp-2 text-[11px] leading-4">{{ failureMessage || progressStatus || '请稍后重试。' }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.mod-pack-card {
  @apply flex aspect-square w-full flex-col overflow-hidden rounded text-left transition;
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel);
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 0.02);

  &:hover {
    border-color: var(--ds-border-strong);
  }

  &.is-failed {
    border-color: var(--ds-error-border);
  }

  &.is-conflicted {
    border-color: var(--ds-border-strong);
  }
}

.mod-pack-card__header {
  background: linear-gradient(180deg, var(--ds-bg-panel-elevated), var(--ds-bg-panel));
}

.mod-pack-card__title {
  color: var(--ds-text-primary);
}

.mod-pack-card__description {
  color: var(--ds-text-secondary);
}

.mod-pack-card__author {
  color: var(--ds-text-muted);
}

.mod-pack-card__translating {
  color: var(--ds-text-muted);
}

.mod-pack-card__spinner {
  animation: mod-pack-card-spin 0.9s linear infinite;
}

@media (prefers-reduced-motion: reduce) {
  .mod-pack-card__spinner {
    animation-duration: 2.4s;
  }
}

@keyframes mod-pack-card-spin {
  to {
    transform: rotate(360deg);
  }
}

.mod-pack-card__icon-action,
.mod-pack-card__cancel {
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel);
  color: var(--ds-text-secondary);

  &:hover {
    border-color: var(--ds-border-strong);
    background: var(--ds-hover-bg);
    color: var(--ds-brand);
  }
}

.mod-pack-card__icon-action.is-warning {
  color: var(--ds-warning);
}

.mod-pack-card__icon-action.is-danger {
  color: var(--ds-error);
}

.mod-pack-card__progress-track {
  background: var(--ds-pressed-bg);
}

.mod-pack-card__progress {
  background: var(--ds-brand);
}

.mod-pack-card__progress-track.is-danger {
  background: var(--ds-error-bg);
}

.mod-pack-card__progress.is-danger {
  background: var(--ds-error);
}

.mod-pack-card__media {
  border-color: var(--ds-border-subtle);
  background: var(--ds-bg-shell);
}

.mod-pack-card__stat {
  border: 1px solid rgb(255 255 255 / 0.4);
  background: rgb(6 6 12 / 0.72);
  color: #eef1ff;
  box-shadow: 0 6px 16px rgb(0 0 0 / 0.18);
  text-shadow: 0 1px 2px rgb(0 0 0 / 0.38);
}

.mod-pack-card__status-badge {
  border: 1px solid var(--ds-border-default);
  box-shadow: 0 6px 16px rgb(0 0 0 / 0.18);
}

.mod-pack-card__status-badge.is-success {
  border-color: var(--ds-success-border);
  background: var(--ds-success-bg);
  color: var(--ds-success);
}

.mod-pack-card__status-badge.is-warning {
  border-color: var(--ds-warning-border);
  background: var(--ds-warning-bg);
  color: var(--ds-warning);

  &:hover {
    background: var(--ds-pressed-bg);
  }
}

.mod-pack-card__installing {
  background: var(--ds-overlay-bg);
}

.mod-pack-card__failure {
  border: 1px solid var(--ds-error-border);
  background: var(--ds-overlay-bg);
  backdrop-filter: blur(6px);
  color: var(--ds-text-primary);
  box-shadow: 0 8px 20px rgb(0 0 0 / 0.2);
}

.mod-pack-emblem {
  @apply grid h-16 w-16 place-items-center rounded text-3xl font-semibold;
  border: 1px solid var(--ds-border-default);
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-primary);
  box-shadow: inset 0 0 0 6px var(--ds-hover-bg);
}
</style>

