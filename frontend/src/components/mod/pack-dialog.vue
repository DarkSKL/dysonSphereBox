<script setup lang="ts">
import {CheckCircle2, ExternalLink, X} from '@lucide/vue'
import {computed} from 'vue'
import {response} from '../../../wailsjs/go/models'

const props = defineProps<{
  show: boolean
  pack?: response.ModDependencyPackageDTO
  installed: boolean
}>()

const emit = defineEmits<{
  (event: 'update:show', value: boolean): void
  (event: 'install', id: number): void
}>()

const title = computed(() => props.pack?.packageName ?? '')
const description = computed(() => props.pack?.packageDescription || '暂无整合包说明。')
const dependencies = computed(() => props.pack?.dependencies ?? [])
const packageSizeText = computed(() => formatPackageSize(props.pack?.packageSize ?? 0))

// =============== 整合包大小格式化 ===============
// 说明：接口返回 byte 单位，详情面板统一换算为 MB 展示。
// 边界：空值或异常值按 0 MB 处理，避免显示“未知”。
const formatPackageSize = (sizeInBytes: number): string => {
  if (!Number.isFinite(sizeInBytes) || sizeInBytes <= 0) {
    return '0 MB'
  }

  return `${(sizeInBytes / 1024 / 1024).toFixed(2)} MB`
}

const handleInstall = (): void => {
  if (!props.pack) {
    return
  }

  emit('install', props.pack.id)
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
        <p class="mb-5 text-sm leading-6 text-zinc-500">{{ description }}</p>

        <div class="mb-5 grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
          <div class="border border-zinc-200 bg-zinc-50 px-3 py-2">
            <p class="text-zinc-400">版本</p>
            <p class="mt-1 truncate font-medium text-zinc-950">{{ pack.packageVersion || '未知' }}</p>
          </div>
          <div class="border border-zinc-200 bg-zinc-50 px-3 py-2">
            <p class="text-zinc-400">作者</p>
            <p class="mt-1 truncate font-medium text-zinc-950">{{ pack.author || '未知' }}</p>
          </div>
          <div class="border border-zinc-200 bg-zinc-50 px-3 py-2">
            <p class="text-zinc-400">下载</p>
            <p class="mt-1 truncate font-medium text-zinc-950">{{ pack.downloadCount || 0 }}</p>
          </div>
          <div class="border border-zinc-200 bg-zinc-50 px-3 py-2">
            <p class="text-zinc-400">大小</p>
            <p class="mt-1 truncate font-medium text-zinc-950">{{ packageSizeText }}</p>
          </div>
        </div>

        <div class="mb-3 text-sm text-zinc-500">包含 {{ dependencies.length }} 个 Mod 组件</div>
        <div class="max-h-[42vh] overflow-auto border-y border-zinc-200">
          <div
            v-for="mod in dependencies"
            :key="mod.id || mod.modName"
            class="grid grid-cols-[minmax(10rem,14rem)_1fr] gap-4 border-b border-zinc-100 px-1 py-3 last:border-b-0"
          >
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-zinc-950">{{ mod.modName || '未知 Mod' }}</p>
              <p class="mt-0.5 text-xs text-zinc-400">{{ mod.modVersion || '未知版本' }}</p>
            </div>
            <p class="text-sm leading-6 text-zinc-600">{{ mod.modDescription || '暂无说明。' }}</p>
          </div>
          <div v-if="dependencies.length === 0" class="px-1 py-5 text-sm text-zinc-500">
            该整合包暂未返回组件明细。
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex items-center justify-between gap-4">
          <a
            v-if="pack?.url"
            class="inline-flex min-w-0 items-center gap-1 text-xs text-zinc-500 hover:text-zinc-950"
            :href="pack.url"
            target="_blank"
          >
            <ExternalLink :size="13" :stroke-width="1.8" class="shrink-0" />
            <span class="truncate">{{ pack.source || '查看来源' }}</span>
          </a>
          <p v-else class="text-xs text-zinc-500">{{ pack?.source || '暂无来源链接' }}</p>
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
