<script setup lang="ts">
import {AlertCircle, CheckCircle2, Download, PackageOpen, Trash2, Wrench, X} from '@lucide/vue'

defineProps<{
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
}>()

defineEmits<{
  (event: 'open', key: string): void
  (event: 'uninstall', key: string): void
  (event: 'repair', key: string): void
  (event: 'cancel', key: string): void
}>()
</script>

<template>
  <button
    class="mod-pack-card group"
    :class="{ 'is-installed': installed, 'is-installing': installing, 'is-failed': failed, 'is-conflicted': conflicted }"
    type="button"
    @click="!installing && $emit('open', packageKey)"
  >
    <div class="flex shrink-0 items-start justify-between gap-3 p-2.5">
      <div class="min-w-0">
        <h3 class="truncate text-sm font-semibold text-zinc-950">{{ title }}</h3>
        <p class="mt-1 line-clamp-1 text-xs leading-5 text-zinc-500">{{ description }}</p>
        <p class="mt-1 truncate text-[11px] text-zinc-400">{{ author || '未知作者' }}</p>
      </div>
      <button
        v-if="installing"
        class="grid h-7 w-7 shrink-0 place-items-center rounded border border-zinc-300 text-zinc-600 opacity-100 transition hover:bg-zinc-50"
        type="button"
        title="取消安装"
        @click.stop="$emit('cancel', packageKey)"
      >
        <X :size="14" :stroke-width="1.8" />
      </button>
      <button
        v-else-if="conflicted"
        class="grid h-7 w-7 shrink-0 place-items-center rounded border border-amber-200 text-amber-700 opacity-100 transition hover:bg-amber-50"
        type="button"
        title="修复安装冲突"
        @click.stop="$emit('repair', packageKey)"
      >
        <Wrench :size="14" :stroke-width="1.8" />
      </button>
      <button
        v-else-if="installed && !installing"
        class="grid h-7 w-7 shrink-0 place-items-center rounded border border-red-200 text-red-600 opacity-0 transition hover:bg-red-50 group-hover:opacity-100"
        type="button"
        title="卸载"
        @click.stop="$emit('uninstall', packageKey)"
      >
        <Trash2 :size="14" :stroke-width="1.8" />
      </button>
    </div>

    <div v-if="installing" class="h-1 bg-zinc-100">
      <div class="h-full bg-zinc-950 transition-all duration-300" :style="{width: `${progress}%`}"></div>
    </div>
    <div v-else-if="failed || conflicted" class="h-1 bg-red-100">
      <div class="h-full w-full bg-red-500"></div>
    </div>

    <div class="relative grid flex-1 place-items-center border-t border-zinc-200 bg-zinc-50">
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
        <div class="rounded border border-zinc-200 bg-white/90 px-2 py-0.5 text-xs text-zinc-600">
          <PackageOpen :size="12" :stroke-width="1.8" class="mr-1 inline-block align-[-2px]" />
          <span>{{ fileCount }} 个组件</span>
        </div>
        <div class="hidden rounded border border-zinc-200 bg-white/90 px-2 py-0.5 text-xs text-zinc-600 sm:block">
          <Download :size="12" :stroke-width="1.8" class="mr-1 inline-block align-[-2px]" />
          <span>{{ downloadCount }}</span>
        </div>
      </div>
      <div
        v-if="installed && !failed && !installing"
        class="absolute right-2 top-2 flex h-7 w-7 items-center justify-center rounded border border-emerald-200 bg-emerald-50 text-emerald-700 shadow-sm"
        title="已安装"
      >
        <CheckCircle2 :size="12" :stroke-width="1.9" />
      </div>
      <div
        v-if="(failed || conflicted) && !installing"
        class="absolute right-2 top-2 flex h-7 w-7 items-center justify-center rounded border border-amber-200 bg-amber-50 text-amber-700 shadow-sm"
        :title="conflicted ? '待处理冲突' : '安装失败'"
      >
        <Wrench v-if="conflicted" :size="12" :stroke-width="1.9" />
        <AlertCircle v-else :size="12" :stroke-width="1.9" />
      </div>
      <div v-if="installing" class="absolute inset-0 grid place-items-center bg-white/75 backdrop-blur-sm">
        <div class="text-center">
          <p class="max-w-[9rem] truncate text-sm font-semibold text-zinc-950">{{ progressStatus || '安装中' }}</p>
          <p class="mt-1 text-xs text-zinc-500">{{ Math.floor(progress) }}%<span v-if="downloadSpeed"> · {{ downloadSpeed }}</span></p>
          <button class="mt-2 border border-zinc-300 bg-white px-2 py-1 text-xs text-zinc-700 hover:border-zinc-500" type="button" @click.stop="$emit('cancel', packageKey)">
            取消
          </button>
        </div>
      </div>
      <div v-else-if="failed || conflicted" class="absolute inset-x-2 bottom-2 border border-amber-200 bg-amber-50/95 px-2 py-1 text-amber-800 shadow-sm">
        <p class="truncate text-xs font-semibold">{{ progressStatus || (conflicted ? '待处理冲突' : '安装失败') }}</p>
        <p class="mt-0.5 line-clamp-2 text-[11px] leading-4">{{ failureMessage || progressStatus || '请稍后重试。' }}</p>
      </div>
    </div>
  </button>
</template>

<style scoped lang="scss">
.mod-pack-card {
  @apply flex aspect-square w-full flex-col overflow-hidden rounded border border-zinc-200 bg-white text-left transition;

  &:hover {
    @apply border-zinc-950;
  }

  &.is-failed {
    @apply border-red-300;
  }

  &.is-conflicted {
    @apply border-amber-300;
  }
}

.mod-pack-emblem {
  @apply grid h-16 w-16 place-items-center rounded border border-zinc-300 bg-white text-3xl font-semibold text-zinc-900;
  box-shadow: inset 0 0 0 6px #f4f4f5;
}
</style>
