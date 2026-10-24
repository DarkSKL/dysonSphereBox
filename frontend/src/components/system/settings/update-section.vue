<script setup lang="ts">
import {CloudDownload, ExternalLink, LogOut, RefreshCw} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onMounted, ref} from 'vue'
import {BrowserOpenURL, Quit} from '../../../../wailsjs/runtime/runtime'
import {useAppUpdateState} from '../../../composables/system/useAppUpdateState'
import {useAppVersion} from '../../../composables/useAppVersion'
import {formatErrorMessage} from '../../../utils/format'
import {describeUpdateError} from '../../../utils/update-error'
import SettingsSection from './section.vue'
import SettingsValueBox, {type SettingsValueTone} from './value-box.vue'

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
const lastCheckAt = ref<Date | null>(null)
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
const checkErrorInfo = computed(() => describeUpdateError(checkError.value))
const platformText = computed(() => {
  const info = appUpdateInfo.value
  return info?.os ? `${info.os}/${info.arch}` : '—'
})
// 说明：优先展示本页最近一次检查（含失败）的时间；复用启动时缓存结果时回退到成功缓存时间。
const lastCheckedText = computed(() => {
  const checkedAt = lastCheckAt.value ?? appUpdateCheckedAt.value
  return checkedAt ? checkedAt.toLocaleString('zh-CN', {hour12: false}) : '—'
})

// =============== 值框文案 ===============
// 说明：值框用一句话说明当前版本与更新结论，颜色随状态变化；检查失败时展示可读原因，完整错误放在下方详情中。
const valueText = computed(() => {
  switch (status.value) {
    case 'checking':
      return '正在检查更新…'
    case 'latest':
      return `当前已是最新版本 ${appVersion.value}`
    case 'available':
      return `发现新版本 ${latestVersionText.value}（当前 ${appVersion.value}）`
    case 'error':
      return `检查更新失败：${checkErrorInfo.value.summary}`
    case 'updated':
      return `已更新到 ${updatedTargetVersion.value}，重启应用后生效`
    default:
      return `当前版本 ${appVersion.value}`
  }
})
const valueTone = computed<SettingsValueTone>(() => {
  if (status.value === 'available') return 'warning'
  if (status.value === 'error') return 'error'
  if (status.value === 'updated') return 'success'
  if (status.value === 'checking') return 'muted'
  return 'default'
})

// =============== 更新检查 ===============
// 说明：进入页面时复用本次程序周期内的检查结果；点击“检查”强制刷新远端状态。
// 边界：检查失败只在本区域展示错误，不影响已有的检查结果缓存；此前失败没有结果时，再次进入页面会重新检查。
const runCheck = async (force: boolean): Promise<void> => {
  checkError.value = ''
  try {
    const {skipped} = await checkAppUpdate({force: force || !appUpdateInfo.value})
    // 说明：复用程序周期缓存时不改写时间，保留产生该结果的检查时刻。
    if (!skipped) lastCheckAt.value = new Date()
  } catch (error) {
    checkError.value = formatErrorMessage(error)
    lastCheckAt.value = new Date()
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
  <!-- ~ 应用更新 -->
  <!-- * 说明：值框展示当前版本与检查结论，右侧为检查与更新；提示行展示运行平台与上次检查时间 -->
  <!-- * 交互：更新需二次确认；更新执行中禁用检查；写入完成后“更新”替换为“退出应用” -->
  <!-- * 依赖：依赖 useAppVersion、useAppUpdateState 与 Wails BrowserOpenURL / Quit 运行时 -->
  <!-- * 边界：只有检测到匹配当前平台的更新包时才允许更新；发布说明只在有新版本或等待重启时展示，检查失败时改为展示处理建议与原始错误 -->
  <SettingsSection id="settings-update" title="应用更新">
    <SettingsValueBox :tone="valueTone" :title="valueText">{{ valueText }}</SettingsValueBox>
    <n-button
      :loading="isCheckingAppUpdate"
      :disabled="isUpdatingApp || status === 'updated'"
      @click="runCheck(true)"
    >
      <template #icon>
        <RefreshCw :size="15" :stroke-width="1.8" />
      </template>
      检查
    </n-button>
    <n-button v-if="status === 'updated'" type="primary" secondary @click="Quit">
      <template #icon>
        <LogOut :size="15" :stroke-width="1.8" />
      </template>
      退出应用
    </n-button>
    <n-popconfirm v-else positive-text="立即更新" negative-text="取消" @positive-click="handleUpdate">
      <template #trigger>
        <n-button type="primary" secondary :disabled="status !== 'available'" :loading="isUpdatingApp">
          <template #icon>
            <CloudDownload :size="15" :stroke-width="1.8" />
          </template>
          更新
        </n-button>
      </template>
      将下载 {{ appUpdateInfo?.assetName }} 并替换当前程序，完成后需重启应用。
    </n-popconfirm>
    <template #hint>当前运行环境：{{ platformText }}　上次检查：{{ lastCheckedText }}</template>

    <!-- ~ 发布说明 -->
    <!-- * 说明：展示目标 Release 的标题、发布时间、原文说明与发布页链接 -->
    <!-- * 交互：说明较长时在固定高度内滚动 -->
    <!-- * 依赖：依赖检查结果中的 releaseName、publishedAt、releaseNotes 与 releaseUrl -->
    <!-- * 边界：说明按纯文本保留换行，不渲染 Markdown -->
    <template #after>
      <!-- ~ 检查失败说明 -->
      <!-- * 说明：值框给出可读原因，这里补全处理建议与后端原始错误，避免单行值框截断 -->
      <!-- * 交互：原始错误默认折叠，需要排障时展开；重新检查仍使用行内“检查”按钮 -->
      <!-- * 依赖：checkErrorInfo 的可读原因/建议与 checkError 原始错误链 -->
      <!-- * 边界：仅在检查失败时出现，与发布说明互斥 -->
      <section v-if="status === 'error'" class="update-error" role="alert" aria-label="更新检查错误说明">
        <header class="update-error-head">
          <span class="update-error-title">处理建议</span>
        </header>
        <p class="update-error-body">{{ checkErrorInfo.hint }}</p>
        <details class="update-error-detail">
          <summary>技术详情</summary>
          <code>{{ checkError }}</code>
        </details>
      </section>

      <section
        v-if="(status === 'available' || status === 'updated') && (releaseTitle || releaseNotes)"
        class="update-notes"
        aria-label="发布说明"
      >
        <header class="update-notes-head">
          <span class="update-notes-title">{{ releaseTitle }}</span>
          <span v-if="appUpdateInfo?.publishedAt" class="update-notes-date">{{ appUpdateInfo.publishedAt }}</span>
          <n-button v-if="appUpdateInfo?.releaseUrl" class="update-notes-link" text size="tiny" @click="openReleasePage">
            <template #icon>
              <ExternalLink :size="13" :stroke-width="1.8" />
            </template>
            发布页面
          </n-button>
        </header>
        <p class="update-notes-body">{{ releaseNotes || '该版本没有附带发布说明。' }}</p>
      </section>
    </template>
  </SettingsSection>
</template>

<style scoped lang="scss">
.update-error {
  margin-top: 12px;
  border: 1px solid var(--ds-error-border);
  border-radius: 4px;
  background: var(--ds-bg-panel);
}

.update-error-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--ds-error-border);
}

.update-error-title {
  color: var(--ds-error);
  font-size: 13px;
  font-weight: 700;
}

.update-error-body {
  margin: 0;
  padding: 12px 14px 0;
  color: var(--ds-text-secondary);
  font-size: 12px;
  line-height: 1.7;
}

.update-error-detail {
  padding: 8px 14px 12px;
  color: var(--ds-text-muted);
  font-size: 11px;
}

.update-error-detail summary {
  cursor: pointer;
}

.update-error-detail code {
  display: block;
  margin-top: 8px;
  padding: 10px;
  border-radius: 4px;
  background: var(--ds-bg-panel-elevated);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.update-notes {
  margin-top: 12px;
  border: 1px solid var(--ds-border-subtle);
  border-radius: 4px;
  background: var(--ds-bg-panel);
}

.update-notes-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
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

.update-notes-link {
  margin-left: auto;
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
</style>
