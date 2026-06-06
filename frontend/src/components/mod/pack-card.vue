<script setup lang="ts">
import {CheckCircle2, PackageOpen, Trash2, UsersRound, Wrench} from '@lucide/vue'
import type {ModPackageKey} from '../../composables/useWorkshopState'

defineProps<{
  packageKey: ModPackageKey
  title: string
  description: string
  fileCount: number
  installed: boolean
  installing: boolean
  progress: number
}>()

defineEmits<{
  (event: 'open', key: ModPackageKey): void
  (event: 'uninstall', key: ModPackageKey): void
}>()
</script>

<template>
  <button
    class="mod-pack-card group"
    :class="{ 'is-installed': installed, 'is-installing': installing }"
    type="button"
    @click="!installing && $emit('open', packageKey)"
  >
    <div class="flex shrink-0 items-start justify-between gap-3 p-3">
      <div class="min-w-0">
        <h3 class="truncate text-sm font-semibold text-zinc-950">{{ title }}</h3>
        <p class="mt-1 line-clamp-2 text-xs leading-5 text-zinc-500">{{ description }}</p>
      </div>
      <button
        v-if="installed && !installing"
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

    <div class="relative grid flex-1 place-items-center border-t border-zinc-200 bg-zinc-50">
      <div class="mod-pack-emblem">
        <UsersRound v-if="packageKey === 'nebula'" :size="34" :stroke-width="1.7" />
        <Wrench v-else :size="34" :stroke-width="1.7" />
      </div>
      <div class="absolute bottom-2 right-2 rounded border border-zinc-200 bg-white/90 px-2 py-0.5 text-xs text-zinc-600">
        <PackageOpen :size="12" :stroke-width="1.8" class="mr-1 inline-block align-[-2px]" />
        <span>{{ fileCount }} 个文件</span>
      </div>
      <div
        v-if="installed && !installing"
        class="absolute left-2 top-2 flex items-center gap-1 rounded border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-xs text-emerald-700"
      >
        <CheckCircle2 :size="12" :stroke-width="1.9" />
        <span>已安装</span>
      </div>
      <div v-if="installing" class="absolute inset-0 grid place-items-center bg-white/75 backdrop-blur-sm">
        <div class="text-center">
          <p class="text-sm font-semibold text-zinc-950">安装中</p>
          <p class="mt-1 text-xs text-zinc-500">{{ Math.floor(progress) }}%</p>
        </div>
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
}

.mod-pack-emblem {
  @apply grid h-20 w-20 place-items-center rounded border border-zinc-300 bg-white text-3xl font-semibold text-zinc-900;
  box-shadow: inset 0 0 0 6px #f4f4f5;
}
</style>
