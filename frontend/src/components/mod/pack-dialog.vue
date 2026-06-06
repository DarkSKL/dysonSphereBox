<script setup lang="ts">
import {CheckCircle2, X} from '@lucide/vue'
import {computed} from 'vue'
import type {ModPackage, ModPackageKey} from '../../composables/useWorkshopState'

const props = defineProps<{
  show: boolean
  pack?: ModPackage
  installed: boolean
}>()

const emit = defineEmits<{
  (event: 'update:show', value: boolean): void
  (event: 'install', key: ModPackageKey): void
}>()

const title = computed(() => props.pack?.title ?? '')

const handleInstall = (): void => {
  if (!props.pack) {
    return
  }

  emit('install', props.pack.key)
  emit('update:show', false)
}
</script>

<template>
  <n-modal :show="show" @update:show="$emit('update:show', $event)">
    <n-card class="w-[42rem] max-w-[calc(100vw-2rem)]" :title="title" :bordered="false" role="dialog" aria-modal="true">
      <template #header-extra>
        <n-button quaternary circle size="small" title="关闭" @click="$emit('update:show', false)">
          <template #icon>
            <X :size="16" :stroke-width="1.8" />
          </template>
        </n-button>
      </template>

      <div v-if="pack">
        <p class="mb-5 text-sm leading-6 text-zinc-500">{{ pack.description }}</p>

        <div class="mb-3 text-sm text-zinc-500">包含 {{ pack.mods.length }} 个 Mod 组件</div>
        <div class="max-h-[42vh] overflow-auto border-y border-zinc-200">
          <div
            v-for="mod in pack.mods"
            :key="mod.name"
            class="grid grid-cols-[minmax(10rem,14rem)_1fr] gap-4 border-b border-zinc-100 px-1 py-3 last:border-b-0"
          >
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-zinc-950">{{ mod.name }}</p>
              <p class="mt-0.5 text-xs text-zinc-400">{{ mod.version }}</p>
            </div>
            <p class="text-sm leading-6 text-zinc-600">{{ mod.description }}</p>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex items-center justify-between gap-4">
          <p class="text-xs text-zinc-500">所有 Mod 文件均从国内服务器下载，无需加速器。</p>
          <n-button type="primary" @click="handleInstall">
            <template v-if="installed" #icon>
              <CheckCircle2 :size="16" :stroke-width="1.8" />
            </template>
            {{ installed ? '重新安装' : '一键安装' }}
          </n-button>
        </div>
      </template>
    </n-card>
  </n-modal>
</template>
