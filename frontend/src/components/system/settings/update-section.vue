<script setup lang="ts">
import {CircleAlert, CircleCheck, CloudDownload, ExternalLink, LoaderCircle, RefreshCw} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onMounted, ref} from 'vue'
import {BrowserOpenURL, Quit} from '../../../../wailsjs/runtime/runtime'
import {useAppUpdateState} from '../../../composables/system/useAppUpdateState'
import {useAppVersion} from '../../../composables/useAppVersion'
import {formatErrorMessage} from '../../../utils/format'
import SettingsSection from './section.vue'

type UpdateStatus = 'idle' | 'checking' | 'latest' | 'available' | 'error' | 'updated'

const message = useMessage()
const {appVersion, loadAppVersion} = useAppVersion()
const {
  appUpdateInfo,
  appUpdateCheckedAt,
  isCheckingAppUpdate,
  isUpdatingApp,
  checkAppUpdate,
  updateAppSelf,
} = useAppUpdateState()
const checkError = ref('')
const updatedTargetVersion = ref('')

// =============== 更新状态归纳 ===============
// 说明：把检查结果、检查中、检查失败和已写入更新归纳为单一状态，模板只按状态渲染。
// 边界：已写入更新优先级最高，直到用户重启前都保持“重启生效”提示。
const status = computed<UpdateStatus>(() => {
  if (updatedTargetVersion.value) return 'updated'
  if (isCheckingAppUpdate.value) return 'checking'
  if (checkError.value) return 'error'
  if (!appUpdateInfo.value) return 'idle'
  return appUpdateInfo.value.hasUpdate && appUpdateInfo.value.assetName ? 'available' : 'latest'
})
const latestVersionText = computed(() => appUpdateInfo.value?.latestVersion || '—')
const releaseTitle = computed(() => appUpdateInfo.value?.releaseName || appUpdateInfo.value?.latestVersion || '')
const releaseNotes = computed(() => appUpdateInfo.value?.releaseNotes?.trim() || '')
const platformText = computed(() => {
  const info = appUpdateInfo.value
  return info?.os ? `${info.os} / ${info.arch}` : '—'
})

const lastCheckedText = computed(() => (
  appUpdateCheckedAt.value ? appUpdateCheckedAt.value.toLocaleString('zh-CN', {hour12: false}) : '—'
))

// =============== 更新检查 ===============
// 说明：进入页面时复用本次程序周期内的检查结果；点击“检查更新”强制刷新远端状态。
// 边界：检查失败只在本区域展示错误，不影响已有的检查结果缓存；此前失败没有结果时，再次进入页面会重新检查。
const runCheck = async (force: boolean): Promise<void> => {
  checkError.value = ''
  try {
    await checkAppUpdate({force: force || !appUpdateInfo.value})
  } catch (error) {
    checkError.value = formatErrorMessage(error)
  }
}

onMounted(() => {
  void loadAppVersion()
  void runCheck(false)
})

// =============== 执行更新 ===============
// 说明：用户确认后由后端下载、校验并替换当前程序，成功后提示重启生效。
// 边界：失败时保留当前程序并提示错误；后端判断无需更新时按“已是最新”处理。
const handleUpdate = async (): Promise<void> => {
  try {
    const result = await updateAppSelf()
    if (result.updated) {
      updatedTargetVersion.value = result.targetVersion || result.check.latestVersion
      message.success(result.message || '更新已完成，重启应用后生效。')
    } else {
      message.info(result.message || '当前已是最新版本。')
    }
  } catch (error) {
    message.error(`应用更新失败：${formatErrorMessage(error)}`)
  }
}

const openReleasePage = (): void => {
  if (appUpdateInfo.value?.releaseUrl) {
    BrowserOpenURL(appUpdateInfo.value.releaseUrl)
  }
}
</script>

<template>
  <SettingsSection
    id="settings-update"
    title="应用更新"
    description="从 Gitee Release 获取戴森球盒子的新版本。更新包会先校验完整性，写入后需重启应用生效。"
  >
    <!-- ~ 版本信息 -->
    <!-- * 说明：并列展示当前版本、最新版本、运行平台与最近检查时间 -->
    <!-- * 交互：只读信息，随检查结果刷新 -->
    <!-- * 依赖：依赖 useAppVersion 与 useAppUpdateState 的检查结果 -->
    <!-- * 边界：尚未取得检查结果时最新版本和平台显示占位符 -->
    <dl class="update-facts">
      <div class="update-fact">
        <dt>当前版本</dt>
        <dd>{{ appVersion }}</dd>
      </div>
      <div class="update-fact">
        <dt>最新版本</dt>
        <dd>{{ latestVersionText }}</dd>
      </div>
      <div class="update-fact">
        <dt>运行平台</dt>
        <dd>{{ platformText }}</dd>
      </div>
      <div class="update-fact">
        <dt>上次检查</dt>
        <dd>{{ lastCheckedText }}</dd>
      </div>
    </dl>

    <!-- ~ 更新状态 -->
    <!-- * 说明：用一行状态说明检查中、已是最新、发现新版本、检查失败或等待重启 -->
    <!-- * 交互：状态变化通过 aria-live 通知读屏器 -->
    <!-- * 依赖：依赖归纳后的 status 与检查错误文本 -->
    <!-- * 边界：idle 状态仅在首次检查尚未开始的瞬间出现 -->
    <p class="update-status" :class="`is-${status}`" aria-live="polite">
      <LoaderCircle v-if="status === 'checking'" class="update-spin" :size="15" :stroke-width="1.8" aria-hidden="true" />
      <CircleAlert v-else-if="status === 'error' || status === 'available'" :size="15" :stroke-width="1.8" aria-hidden="true" />
      <CircleCheck v-else-if="status !== 'idle'" :size="15" :stroke-width="1.8" aria-hidden="true" />
      <template v-if="status === 'checking'">正在检查更新…</template>
      <template v-else-if="status === 'latest'">当前已是最新版本。</template>
      <template v-else-if="status === 'available'">发现新版本 {{ latestVersionText }}，可立即更新。</template>
      <template v-else-if="status === 'error'">检查更新失败：{{ checkError }}</template>
      <template v-else-if="status === 'updated'">已更新到 {{ updatedTargetVersion }}，重启应用后生效。</template>
      <template v-else>尚未检查更新。</template>
    </p>

    <!-- ~ 更新操作 -->
    <!-- * 说明：检查更新、确认后执行更新，更新完成后提供退出应用入口 -->
    <!-- * 交互：立即更新需二次确认；更新执行中禁用检查，避免状态交叉 -->
    <!-- * 依赖：依赖 CheckAppUpdate / UpdateAppSelf 绑定与 Wails Quit 运行时 -->
    <!-- * 边界：只有检测到匹配当前平台的更新包时才显示立即更新 -->
    <div class="update-actions">
      <n-popconfirm
        v-if="status === 'available'"
        positive-text="立即更新"
        negative-text="取消"
        @positive-click="handleUpdate"
      >
        <template #trigger>
          <n-button type="primary" :loading="isUpdatingApp">
            <template #icon>
              <CloudDownload :size="15" :stroke-width="1.8" />
            </template>
            立即更新
          </n-button>
        </template>
        将下载 {{ appUpdateInfo?.assetName }} 并替换当前程序，完成后需重启应用。
      </n-popconfirm>
      <n-button v-if="status === 'updated'" type="primary" @click="Quit">退出应用</n-button>
      <n-button
        :loading="isCheckingAppUpdate"
        :disabled="isUpdatingApp || status === 'updated'"
        @click="runCheck(true)"
      >
        <template #icon>
          <RefreshCw :size="15" :stroke-width="1.8" />
        </template>
        检查更新
      </n-button>
      <n-button v-if="appUpdateInfo?.releaseUrl" quaternary @click="openReleasePage">
        <template #icon>
          <ExternalLink :size="15" :stroke-width="1.8" />
        </template>
        发布页面
      </n-button>
    </div>

    <!-- ~ 发布说明 -->
    <!-- * 说明：展示目标 Release 的标题、发布时间与原文说明 -->
    <!-- * 交互：说明较长时在固定高度内滚动 -->
    <!-- * 依赖：依赖检查结果中的 releaseName、publishedAt 与 releaseNotes -->
    <!-- * 边界：只在有新版本或等待重启时展示；说明按纯文本保留换行，不渲染 Markdown -->
    <section
      v-if="(status === 'available' || status === 'updated') && (releaseTitle || releaseNotes)"
      class="update-notes"
      aria-label="发布说明"
    >
      <header class="update-notes-head">
        <span class="update-notes-title">{{ releaseTitle }}</span>
        <span v-if="appUpdateInfo?.publishedAt" class="update-notes-date">{{ appUpdateInfo.publishedAt }}</span>
      </header>
      <p class="update-notes-body">{{ releaseNotes || '该版本没有附带发布说明。' }}</p>
    </section>
  </SettingsSection>
</template>

<style scoped lang="scss">
.update-facts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 16px 24px;
  margin: 0;
}

.update-fact dt {
  color: var(--ds-text-muted);
  font-size: 12px;
}

.update-fact dd {
  margin: 6px 0 0;
  color: var(--ds-text-primary);
  font-size: 14px;
  font-variant-numeric: tabular-nums;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.update-status {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 22px 0 0;
  color: var(--ds-text-secondary);
  font-size: 13px;
}

.update-status.is-latest,
.update-status.is-updated {
  color: var(--ds-success);
}

.update-status.is-available {
  color: var(--ds-warning);
}

.update-status.is-error {
  color: var(--ds-error);
}

.update-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
}

.update-notes {
  margin-top: 22px;
  border: 1px solid var(--ds-border-subtle);
  border-radius: 4px;
  background: var(--ds-bg-panel);
}

.update-notes-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--ds-border-subtle);
}

.update-notes-title {
  color: var(--ds-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.update-notes-date {
  color: var(--ds-text-muted);
  font-size: 12px;
}

.update-notes-body {
  max-height: 240px;
  margin: 0;
  padding: 12px 14px;
  overflow: auto;
  color: var(--ds-text-secondary);
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.update-spin {
  animation: update-spin 0.9s linear infinite;
}

@keyframes update-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .update-spin {
    animation: none;
  }
}
</style>
