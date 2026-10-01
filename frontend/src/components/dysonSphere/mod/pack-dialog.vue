<script setup lang="ts">
import { CheckCircle2, ExternalLink, X } from '@lucide/vue'
import { computed } from 'vue'
import { modDependence } from '../../../../wailsjs/go/models'
import { BrowserOpenURL } from '../../../../wailsjs/runtime/runtime'

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
const description = computed(() => props.pack?.description || '暂无模组说明。')
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

// =============== 模组大小格式化 ===============
// 说明：接口返回 byte 单位，详情面板统一换算为 MB 展示。
// 边界：空值或异常值按 0 MB 处理，避免显示“未知”。
const formatPackageSize = (sizeInBytes: number): string => {
  if (!Number.isFinite(sizeInBytes) || sizeInBytes <= 0) {
    return '0 MB'
  }

  return `${(sizeInBytes / 1024 / 1024).toFixed(2)} MB`
}

// 外部链接交给系统浏览器打开，WebView 内不跳转
const sourceUrl = computed(() => props.pack?.packageUrl || props.pack?.downloadUrl || '')
const openSource = (): void => {
  if (sourceUrl.value) {
    BrowserOpenURL(sourceUrl.value)
  }
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
        <p class="pack-dialog-desc mb-5 text-sm leading-6">{{ description }}</p>

        <div class="mb-5 grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
          <div class="pack-dialog-stat px-3 py-2">
            <p class="pack-dialog-muted">版本</p>
            <p class="mt-1 truncate font-medium pack-dialog-strong">{{ pack.version || '未知' }}</p>
          </div>
          <div class="pack-dialog-stat px-3 py-2">
            <p class="pack-dialog-muted">作者</p>
            <p class="mt-1 truncate font-medium pack-dialog-strong">{{ pack.author || '未知' }}</p>
          </div>
          <div class="pack-dialog-stat px-3 py-2">
            <p class="pack-dialog-muted">下载</p>
            <p class="mt-1 truncate font-medium pack-dialog-strong">{{ pack.downloadCount || 0 }}</p>
          </div>
          <div class="pack-dialog-stat px-3 py-2">
            <p class="pack-dialog-muted">大小</p>
            <p class="mt-1 truncate font-medium pack-dialog-strong">{{ packageSizeText }}</p>
          </div>
        </div>

        <div class="pack-dialog-muted mb-3 text-sm">包含 {{ displayComponents.length }} 个模组组件</div>
        <div class="pack-dialog-list max-h-[42vh] overflow-auto">
          <div
            v-for="mod in displayComponents"
            :key="mod.key || mod.name"
            class="pack-dialog-row grid grid-cols-[minmax(10rem,14rem)_1fr] gap-4 px-1 py-3"
          >
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold pack-dialog-strong">{{ mod.displayName || mod.name || '未知模组' }}</p>
              <p class="pack-dialog-muted mt-0.5 text-xs">{{ mod.version || '未知版本' }}</p>
            </div>
            <p class="pack-dialog-desc text-sm leading-6">{{ mod.description || '暂无说明。' }}</p>
          </div>
          <div v-if="displayComponents.length === 0" class="pack-dialog-muted px-1 py-5 text-sm">
            该模组暂未返回组件明细。
          </div>
        </div>
      </div>

      <template #footer>
        <div class="flex items-center justify-between gap-4">
          <n-button v-if="sourceUrl" text size="tiny" class="min-w-0" @click="openSource">
            <template #icon>
              <ExternalLink :size="13" :stroke-width="1.8" />
            </template>
            {{ pack?.source || '查看来源' }}
          </n-button>
          <p v-else class="pack-dialog-muted text-xs">{{ pack?.source || '暂无来源链接' }}</p>
          <div class="flex items-center gap-3">
            <p
              v-if="!canInstall"
              class="pack-dialog-warning max-w-[15rem] text-right text-xs leading-5"
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

<style scoped lang="scss">
.pack-dialog-desc {
  color: var(--ds-text-secondary);
}

.pack-dialog-muted {
  color: var(--ds-text-muted);
}

.pack-dialog-strong {
  color: var(--ds-text-primary);
}

.pack-dialog-warning {
  color: var(--ds-warning);
}

.pack-dialog-stat {
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel);
}

.pack-dialog-list {
  border-block: 1px solid var(--ds-border-default);
}

.pack-dialog-row + .pack-dialog-row {
  border-top: 1px solid var(--ds-border-subtle);
}
</style>
