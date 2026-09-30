<script setup lang="ts">
import { CheckCircle2, ExternalLink, X } from '@lucide/vue'
import { computed } from 'vue'
import { modDependence } from '../../../../wailsjs/go/models'

const props = defineProps<{
  show: boolean
  pack?: modDependence.RemoteModPackageDTO
  installed: boolean
  installing: boolean
  canInstall: boolean
  installDisabledText?: string
}>()

const emit = defineEmits<{
  (event: 'update:show', value: boolean): void
  (event: 'install', key: string): void
  (event: 'uninstall', key: string): void
}>()

const title = computed(() => props.pack?.displayName ?? '')
const description = computed(() => props.pack?.description || '暂无整合包说明。')
const dependencies = computed(() => props.pack?.dependencies ?? [])
const displayComponents = computed(() => {
  if (!props.pack) {
    return []
  }

  return [
    {
      key: props.pack.key,
      displayName: props.pack.displayName || props.pack.name,
      name: props.pack.name,
      version: props.pack.version,
      description: props.pack.description,
    },
    ...dependencies.value,
  ]
})
const packageSizeText = computed(() => formatPackageSize(props.pack?.packageSize ?? 0))
const installButtonDisabled = computed(() => props.installing || !props.canInstall)
const installButtonText = computed(() => {
  if (props.installing) {
    return props.installed ? '卸载中' : '安装中'
  }
  if (!props.canInstall) {
    return '需先安装 BepInEx'
  }

  return props.installed ? '重新安装' : '一键安装'
})

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
  if (!props.pack || props.installing || !props.canInstall) {
    return
  }

  emit('install', props.pack.key)
  emit('update:show', false)
}

const handleUninstall = (): void => {
  if (!props.pack || props.installing) {
    return
  }

  emit('uninstall', props.pack.key)
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
            <p class="mt-1 truncate font-medium text-zinc-950">{{ pack.version || '未知' }}</p>
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

        <div class="mb-3 text-sm text-zinc-500">包含 {{ displayComponents.length }} 个模组组件</div>
        <div class="max-h-[42vh] overflow-auto border-y border-zinc-200">
          <div
            v-for="mod in displayComponents"
            :key="mod.key || mod.name"
            class="grid grid-cols-[minmax(10rem,14rem)_1fr] gap-4 border-b border-zinc-100 px-1 py-3 last:border-b-0"
          >
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-zinc-950">{{ mod.displayName || mod.name || '未知模组' }}</p>
              <p class="mt-0.5 text-xs text-zinc-400">{{ mod.version || '未知版本' }}</p>
            </div>
            <p class="text-sm leading-6 text-zinc-600">{{ mod.description || '暂无说明。' }}</p>
          </div>
          <div v-if="displayComponents.length === 0" class="px-1 py-5 text-sm text-zinc-500">
            该整合包暂未返回组件明细。
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex items-center justify-between gap-4">
          <a
            v-if="pack?.packageUrl || pack?.downloadUrl"
            class="inline-flex min-w-0 items-center gap-1 text-xs text-zinc-500 hover:text-zinc-950"
            :href="pack.packageUrl || pack.downloadUrl"
            target="_blank"
          >
            <ExternalLink :size="13" :stroke-width="1.8" class="shrink-0" />
            <span class="truncate">{{ pack.source || '查看来源' }}</span>
          </a>
          <p v-else class="text-xs text-zinc-500">{{ pack?.source || '暂无来源链接' }}</p>
          <div class="flex items-center gap-3">
            <p
              v-if="!canInstall"
              class="max-w-[15rem] text-right text-xs leading-5 text-amber-600"
            >
              {{ installDisabledText || '需要优先安装 BepInEx。' }}
            </p>
            <n-button v-if="installed" type="error" secondary :loading="installing" @click="handleUninstall">
              卸载
            </n-button>
            <n-button type="primary" :loading="installing" :disabled="installButtonDisabled" @click="handleInstall">
            <template v-if="installed" #icon>
              <CheckCircle2 :size="16" :stroke-width="1.8" />
            </template>
              {{ installButtonText }}
            </n-button>
          </div>
        </div>
      </template>
    </n-card>
  </n-modal>
</template>

