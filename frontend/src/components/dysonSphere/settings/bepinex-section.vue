<script setup lang="ts">
import {CloudDownload, RefreshCw, Trash2} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, ref, watch} from 'vue'
import {
  gameDirectory,
  gameDirectoryLoaded,
  getBepInExStatus,
  getLatestBepInExPackage,
  installBepInEx,
  uninstallBepInEx,
  type BepInExPackage,
  type BepInExStatus,
} from '../../../services/dysonSphere/game'
import {formatErrorMessage} from '../../../utils/format'
import SettingsSection, {type SettingsHintTone} from '../../system/settings/section.vue'
import SettingsValueBox, {type SettingsValueTone} from '../../system/settings/value-box.vue'

type BepInExState = 'no-game' | 'checking' | 'installed' | 'broken' | 'missing' | 'error'

const message = useMessage()
const status = ref<BepInExStatus | null>(null)
const latestPackage = ref<BepInExPackage | null>(null)
const statusError = ref('')
const isChecking = ref(false)
const isCheckingLatest = ref(false)
const isInstalling = ref(false)
const isUninstalling = ref(false)
const isOperating = computed(() => isInstalling.value || isUninstalling.value)

// =============== 状态归纳 ===============
// 说明：未安装与“部分文件存在但不完整”分开展示，后者需要重新安装修复。
// 边界：没有有效游戏目录时不检测，直接提示先配置游戏路径。
const state = computed<BepInExState>(() => {
  if (!gameDirectory.value) return 'no-game'
  if (isChecking.value && !status.value) return 'checking'
  if (statusError.value) return 'error'
  if (!status.value) return 'checking'
  if (status.value.installed) return 'installed'
  const hasAnyFile = status.value.coreExists || status.value.preloaderExists
    || status.value.winHTTPExists || status.value.doorstopExists
  return hasAnyFile ? 'broken' : 'missing'
})

// =============== 值框与提示行 ===============
// 说明：值框一句话说明安装状态与版本；提示行补充文件检测结论与 Thunderstore 最新版本。
const valueText = computed(() => {
  switch (state.value) {
    case 'no-game':
      return '未配置游戏路径'
    case 'checking':
      return '正在检测 BepInEx…'
    case 'installed':
      return `已安装 BepInEx ${status.value?.installedVersion || ''}`.trim()
    case 'broken':
      return 'BepInEx 文件不完整'
    case 'missing':
      return '未安装 BepInEx'
    default:
      return '检测失败'
  }
})
const valueTone = computed<SettingsValueTone>(() => {
  if (state.value === 'broken' || state.value === 'missing') return 'warning'
  if (state.value === 'error') return 'error'
  if (state.value === 'no-game' || state.value === 'checking') return 'muted'
  return 'default'
})
const latestText = computed(() => {
  if (isCheckingLatest.value) return '最新版本查询中'
  return latestPackage.value?.version ? `最新版本 ${latestPackage.value.version}` : '最新版本查询失败'
})
const hint = computed<{text: string, tone: SettingsHintTone}>(() => {
  switch (state.value) {
    case 'no-game':
      return {text: 'Mod 前置加载框架，请先在上方配置游戏路径。', tone: 'muted'}
    case 'checking':
      return {text: '正在检查游戏目录中的 BepInEx 文件…', tone: 'muted'}
    case 'installed':
      return {text: `检测到 BepInEx 文件与 Doorstop 配置正常，${latestText.value}。`, tone: 'success'}
    case 'broken':
      return {text: `缺少：${(status.value?.missingFiles ?? []).join('、')}，建议重新安装。`, tone: 'warning'}
    case 'missing':
      return {text: `安装 Mod 前需先安装 BepInEx，将从 Thunderstore 下载，${latestText.value}。`, tone: 'warning'}
    default:
      return {text: statusError.value, tone: 'error'}
  }
})

// =============== 状态检测 ===============
// 说明：游戏目录就绪或变化后检测本地状态，并查询 Thunderstore 上的最新版本。
// 边界：最新版本查询失败只影响提示文案，不影响本地状态与安装操作。
const refreshStatus = async (): Promise<void> => {
  if (!gameDirectory.value || isChecking.value) return
  isChecking.value = true
  statusError.value = ''
  try {
    status.value = await getBepInExStatus()
  } catch (error) {
    status.value = null
    statusError.value = formatErrorMessage(error)
  } finally {
    isChecking.value = false
  }
}

const refreshLatest = async (): Promise<void> => {
  if (!gameDirectory.value || isCheckingLatest.value) return
  isCheckingLatest.value = true
  try {
    latestPackage.value = await getLatestBepInExPackage()
  } catch {
    latestPackage.value = null
  } finally {
    isCheckingLatest.value = false
  }
}

const refreshAll = (): void => {
  void refreshStatus()
  void refreshLatest()
}

watch(
  [gameDirectoryLoaded, gameDirectory],
  ([loaded, dir], previous) => {
    if (!loaded) return
    if (!dir) {
      status.value = null
      statusError.value = ''
      return
    }
    // 目录变化后旧状态不再代表新目录
    if (previous && previous[1] !== dir) {
      status.value = null
    }
    refreshAll()
  },
  {immediate: true},
)

// =============== 安装与卸载 ===============
// 说明：安装会从 Thunderstore 下载最新 BepInEx；文件不完整时重新安装修复。
// 边界：卸载会删除 BepInEx 目录（含其中的 Mod）与 Doorstop 注入文件，因此需二次确认。
const handleInstall = async (): Promise<void> => {
  if (isOperating.value || !gameDirectory.value) return
  isInstalling.value = true
  try {
    const result = await installBepInEx()
    status.value = result.status
    message.success(result.message || 'BepInEx 安装完成。')
  } catch (error) {
    message.error(`BepInEx 安装失败：${formatErrorMessage(error)}`)
    void refreshStatus()
  } finally {
    isInstalling.value = false
  }
}

const handleUninstall = async (): Promise<void> => {
  if (isOperating.value || !gameDirectory.value) return
  isUninstalling.value = true
  try {
    const result = await uninstallBepInEx()
    status.value = result.status
    if (result.skipped) {
      message.info(result.message || '未检测到可卸载的 BepInEx 文件。')
    } else {
      message.success(result.message || 'BepInEx 已卸载。')
    }
  } catch (error) {
    message.error(`BepInEx 卸载失败：${formatErrorMessage(error)}`)
    void refreshStatus()
  } finally {
    isUninstalling.value = false
  }
}
</script>

<template>
  <!-- ~ BepInEx -->
  <!-- * 说明：值框展示安装状态与版本，右侧为检查与安装/卸载；提示行展示文件检测结论与最新版本 -->
  <!-- * 交互：已安装时第二个按钮为卸载（需二次确认），未安装或文件不完整时为安装 -->
  <!-- * 依赖：依赖 services/dysonSphere/game 的 BepInEx 状态、安装与卸载方法 -->
  <!-- * 边界：未配置游戏路径时禁用全部操作；安装或卸载进行中禁用其它操作 -->
  <SettingsSection id="game-settings-bepinex" title="BepInEx" :hint-tone="hint.tone">
    <SettingsValueBox :tone="valueTone" :title="valueText">{{ valueText }}</SettingsValueBox>
    <n-button
      :loading="isChecking"
      :disabled="state === 'no-game' || isOperating"
      @click="refreshAll"
    >
      <template #icon>
        <RefreshCw :size="15" :stroke-width="1.8" />
      </template>
      检查
    </n-button>
    <n-popconfirm
      v-if="state === 'installed'"
      positive-text="卸载"
      negative-text="取消"
      @positive-click="handleUninstall"
    >
      <template #trigger>
        <n-button type="error" secondary :loading="isUninstalling" :disabled="isOperating && !isUninstalling">
          <template #icon>
            <Trash2 :size="15" :stroke-width="1.8" />
          </template>
          卸载
        </n-button>
      </template>
      将删除游戏目录下的 BepInEx 文件夹（含其中的 Mod）与 Doorstop 注入文件，确认卸载？
    </n-popconfirm>
    <n-button
      v-else
      type="primary"
      secondary
      :loading="isInstalling"
      :disabled="state === 'no-game' || state === 'checking' || (isOperating && !isInstalling)"
      @click="handleInstall"
    >
      <template #icon>
        <CloudDownload :size="15" :stroke-width="1.8" />
      </template>
      {{ state === 'broken' ? '修复' : '安装' }}
    </n-button>
    <template #hint>{{ hint.text }}</template>
  </SettingsSection>
</template>
