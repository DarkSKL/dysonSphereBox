<script setup lang="ts">
import {Check, Package, Search} from '@lucide/vue'
import {computed, ref, watch} from 'vue'
import {GetRemotePackageList} from '../../../wailsjs/go/modDependence/MarketplaceService'
import type {modDependence} from '../../../wailsjs/go/models'
import type {ForumModDraft} from '../../types/forum'
import {buildThunderstorePackageUrl, isValidThunderstoreVersion, parseThunderstoreUrl} from '../../utils/forum-markdown'

const props = defineProps<{
  show: boolean
  initial?: ForumModDraft | null
}>()

const emit = defineEmits<{
  (event: 'update:show', value: boolean): void
  (event: 'confirm', draft: ForumModDraft): void
}>()

const activeTab = ref<'market' | 'url'>('market')
const keyword = ref('')
const loading = ref(false)
const loadError = ref('')
const packages = ref<modDependence.RemoteModPackageDTO[]>([])
const selectedPack = ref<modDependence.RemoteModPackageDTO | null>(null)
const lockVersion = ref(true)

const urlInput = ref('')
const urlVersionOverride = ref('')

const parsedUrl = computed(() => parseThunderstoreUrl(urlInput.value))

async function loadMarketPackages(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    const result = await GetRemotePackageList('thunderstore', 1, 20, keyword.value.trim())
    packages.value = result.list ?? []
  } catch {
    loadError.value = '读取 Thunderstore 模组市场失败，请稍后重试或改用粘贴链接。'
  } finally {
    loading.value = false
  }
}

watch(() => props.show, (open) => {
  if (!open) return
  if (props.initial?.mode === 'thunderstore' && props.initial.url && !props.initial.namespace) {
    activeTab.value = 'url'
    urlInput.value = props.initial.url
    urlVersionOverride.value = props.initial.version || ''
  } else {
    activeTab.value = 'market'
    urlInput.value = props.initial?.url || ''
    urlVersionOverride.value = props.initial?.version || ''
  }
  if (!packages.value.length) {
    void loadMarketPackages()
  }
})

const overrideVersionTrimmed = computed(() => urlVersionOverride.value.trim())
const versionConflict = computed(() => {
  if (!overrideVersionTrimmed.value) return false
  if (!isValidThunderstoreVersion(overrideVersionTrimmed.value)) return true
  if (parsedUrl.value.version && parsedUrl.value.version !== overrideVersionTrimmed.value) return true
  return false
})

const canConfirm = computed(() => {
  if (activeTab.value === 'market') {
    return Boolean(selectedPack.value?.namespace && selectedPack.value?.name)
  }
  return parsedUrl.value.valid && !versionConflict.value
})

function confirmSelection(): void {
  if (activeTab.value === 'market') {
    const pack = selectedPack.value
    if (!pack) return
    const version = lockVersion.value && isValidThunderstoreVersion(pack.version || '') ? pack.version : undefined
    // 服务端契约：有版本号时可直接传 {namespace, packageName, version}；未锁定版本（普通包页）则传规范化包页 url 且不带 namespace/packageName。
    emit('confirm', {
      mode: 'thunderstore',
      label: pack.displayName || `${pack.namespace}/${pack.name}`,
      namespace: pack.namespace,
      packageName: pack.name,
      version,
      url: version ? undefined : buildThunderstorePackageUrl(pack.namespace, pack.name),
    })
    emit('update:show', false)
    return
  }

  const parsed = parsedUrl.value
  if (!parsed.valid || versionConflict.value) return
  const version = overrideVersionTrimmed.value || parsed.version || undefined
  emit('confirm', {
    mode: 'thunderstore',
    label: parsed.namespace && parsed.packageName
      ? `${parsed.namespace}/${parsed.packageName}`
      : parsed.url,
    namespace: parsed.namespace,
    packageName: parsed.packageName,
    version,
    url: parsed.url,
  })
  emit('update:show', false)
}
</script>

<template>
  <!-- ~ 选择 Thunderstore 模组弹窗 -->
  <!-- * 说明：支持直接从盒子模组市场搜索选择，或粘贴 Thunderstore 包页 / 指定版本页链接 -->
  <!-- * 交互：可选择是否锁定版本号；确认后绑定到 mod 槽位并插入 [[resource:mod]] -->
  <n-modal :show="show" preset="card" title="插入模组市场引用" style="width: min(92vw, 620px);" @update:show="emit('update:show', $event)">
    <n-tabs v-model:value="activeTab" type="line" size="small">
      <n-tab-pane name="market" tab="从盒子模组市场选择">
        <div class="market-picker">
          <form class="market-search" @submit.prevent="loadMarketPackages">
            <n-input v-model:value="keyword" placeholder="输入模组名称、作者或命名空间搜索" clearable @clear="loadMarketPackages">
              <template #prefix><Search :size="15" /></template>
            </n-input>
            <n-button secondary :loading="loading" attr-type="submit">搜索</n-button>
          </form>

          <div v-if="loadError" class="picker-hint is-error">{{ loadError }}</div>
          <div v-else-if="loading && !packages.length" class="picker-hint">正在读取 Thunderstore 模组列表…</div>
          <div v-else-if="!packages.length" class="picker-hint">没有找到匹配的 Thunderstore 模组。</div>

          <div v-else class="market-list">
            <button
              v-for="pack in packages"
              :key="pack.key"
              type="button"
              class="market-item"
              :class="{'is-selected': selectedPack?.key === pack.key}"
              @click="selectedPack = pack"
            >
              <div class="item-icon"><Package :size="16" /></div>
              <div class="item-main">
                <div class="item-title">
                  <span>{{ pack.displayName || pack.name }}</span>
                  <span class="item-version">v{{ pack.version }}</span>
                </div>
                <div class="item-coord">{{ pack.namespace }} / {{ pack.name }}</div>
              </div>
              <Check v-if="selectedPack?.key === pack.key" :size="16" class="item-check" />
            </button>
          </div>

          <div v-if="selectedPack" class="version-option">
            <n-checkbox v-model:checked="lockVersion">
              锁定当前版本（v{{ selectedPack.version }}）；取消勾选则指向普通包主页
            </n-checkbox>
          </div>
        </div>
      </n-tab-pane>

      <n-tab-pane name="url" tab="粘贴 Thunderstore 链接">
        <div class="url-picker">
          <div class="form-field">
            <label class="field-label">Thunderstore 戴森球计划链接</label>
            <n-input
              v-model:value="urlInput"
              placeholder="https://thunderstore.io/c/dyson-sphere-program/p/命名空间/包名/"
              clearable
            />
          </div>
          <div class="form-field">
            <label class="field-label">可选指定版本号</label>
            <n-input
              v-model:value="urlVersionOverride"
              placeholder="留空则跟随链接；若链接不含版本则指向普通包页"
              clearable
            />
          </div>
          <div v-if="urlInput.trim()" class="url-preview" :class="{'is-valid': parsedUrl.valid && !versionConflict}">
            <template v-if="parsedUrl.valid && !versionConflict">
              已识别包坐标：<strong>{{ parsedUrl.namespace }} / {{ parsedUrl.packageName }}</strong>
              <span v-if="overrideVersionTrimmed || parsedUrl.version"> · 版本 v{{ overrideVersionTrimmed || parsedUrl.version }}</span>
              <span v-else> · 普通包页（不锁定版本）</span>
            </template>
            <template v-else-if="parsedUrl.valid && versionConflict">
              版本号格式需为 x.y.z，且不能与链接中已包含的版本冲突。
            </template>
            <template v-else>
              请输入 https://thunderstore.io 的戴森球包页（/c/dyson-sphere-program/p/命名空间/包名/）、版本页（.../v/版本号/）或官方下载链接。
            </template>
          </div>
        </div>
      </n-tab-pane>
    </n-tabs>

    <template #footer>
      <div class="modal-footer">
        <n-button secondary @click="emit('update:show', false)">取消</n-button>
        <n-button type="primary" :disabled="!canConfirm" @click="confirmSelection">插入到正文</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped lang="scss">
.market-picker,
.url-picker {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 6px;
}

.market-search {
  display: flex;
  gap: 8px;
}

.market-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 280px;
  overflow-y: auto;
  padding-right: 4px;
}

.market-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, background-color 0.15s ease;

  &:hover {
    background: var(--ds-hover-bg);
    border-color: var(--ds-border-default);
  }

  &.is-selected {
    border-color: var(--ds-brand);
    background: var(--ds-active-bg);
  }
}

.item-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 6px;
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-secondary);
  flex-shrink: 0;
}

.item-main {
  flex: 1;
  min-width: 0;
}

.item-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--ds-text-primary);
}

.item-version {
  font-family: var(--ds-font-mono);
  font-size: 11px;
  font-weight: 400;
  color: var(--ds-text-muted);
}

.item-coord {
  margin-top: 2px;
  font-size: 12px;
  color: var(--ds-text-muted);
  font-family: var(--ds-font-mono);
}

.item-check {
  color: var(--ds-brand);
  flex-shrink: 0;
}

.version-option {
  padding-top: 2px;
  font-size: 12px;
}

.picker-hint {
  padding: 20px 12px;
  text-align: center;
  font-size: 13px;
  color: var(--ds-text-muted);

  &.is-error {
    color: var(--ds-error);
  }
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ds-text-primary);
}

.url-preview {
  padding: 8px 10px;
  border-radius: 6px;
  border: 1px solid var(--ds-warning-border);
  background: var(--ds-warning-bg);
  font-size: 12px;
  color: var(--ds-warning);

  &.is-valid {
    border-color: var(--ds-success-border);
    background: var(--ds-success-bg);
    color: var(--ds-success);
  }
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
