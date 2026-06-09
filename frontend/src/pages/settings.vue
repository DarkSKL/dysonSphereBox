<script setup lang="ts">
import {Download, FolderOpen, RefreshCw, Save, Search, Trash2} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onMounted, ref} from 'vue'
import {SelectDysonSphereDirectory} from '../../wailsjs/go/main/App'
import {SearchAndLoadBepInEx, Uninstall} from '../../wailsjs/go/modDependence/BepInExService'
import {modDependence, updater} from '../../wailsjs/go/models'
import {GetConfig, SetConfig} from '../../wailsjs/go/system/ConfigurationService'
import {LoadOrInitDysonSphereExe} from '../../wailsjs/go/system/EnvironmentService'
import {CheckAppUpdate, UpdateAppSelf} from '../../wailsjs/go/system/UpdateService'
import {useWorkshopState} from '../composables/useWorkshopState'

definePage({
  meta: {
    layout: 'main',
    title: '设置',
  },
})

const {
  gamePath,
  isGamePathValid,
} = useWorkshopState()

const isSelectingGamePath = ref(false)
const isLoadingGamePath = ref(false)
const isSavingGamePath = ref(false)
const isSavingBaseURL = ref(false)
const isLoadingBepInExStatus = ref(false)
const isCheckingBepInEx = ref(false)
const isUninstallingBepInEx = ref(false)
const isCheckingAppUpdate = ref(false)
const isUpdatingApp = ref(false)
const updateAppVisible = ref(false)
const uninstallBepInExVisible = ref(false)
const savedGamePath = ref(gamePath.value)
const baseURL = ref('')
const savedBaseURL = ref('')
const message = useMessage()
const bepinExStatus = ref<modDependence.BepInExStatus | null>(null)
const appUpdateInfo = ref<updater.CheckResult | null>(null)
const hasPendingGamePathChange = computed(() => gamePath.value.trim() !== savedGamePath.value.trim())
const baseURLValidationMessage = computed(() => {
  const nextBaseURL = baseURL.value.trim()
  if (!nextBaseURL) {
    return ''
  }

  try {
    const parsedURL = new URL(nextBaseURL)
    if (!['http:', 'https:'].includes(parsedURL.protocol) || !parsedURL.host) {
      return '请输入完整的 HTTP/HTTPS 地址。'
    }
  } catch {
    return '请输入完整的 HTTP/HTTPS 地址。'
  }

  return ''
})
const hasPendingBaseURLChange = computed(() => baseURL.value.trim() !== savedBaseURL.value.trim())
const canSaveBaseURL = computed(() => hasPendingBaseURLChange.value && !baseURLValidationMessage.value)
const bepinExSummary = computed(() => {
  if (!bepinExStatus.value) {
    return '尚未检查 BepInEx'
  }
  if (!bepinExStatus.value.installed) {
    return '未安装 BepInEx'
  }

  return `已安装 BepInEx ${bepinExStatus.value.installedVersion || '未知版本'}`
})
const appUpdateSummary = computed(() => {
  if (!appUpdateInfo.value) {
    return '尚未检查应用更新'
  }
  if (!appUpdateInfo.value.latestVersion) {
    return `当前版本 ${appUpdateInfo.value.currentVersion}，未发现可用发布版本`
  }
  if (appUpdateInfo.value.hasUpdate) {
    return `当前版本 ${appUpdateInfo.value.currentVersion}，发现新版本 ${appUpdateInfo.value.latestVersion}`
  }

  return `当前已是最新版本 ${appUpdateInfo.value.currentVersion}`
})
const canUpdateApp = computed(() => Boolean(appUpdateInfo.value?.hasUpdate) && !isUpdatingApp.value)

onMounted(async () => {
  void checkAppUpdate({silentWhenNoUpdate: true})

  try {
    const configuredBaseURL = await GetConfig('System', 'BaseURL')
    baseURL.value = configuredBaseURL
    savedBaseURL.value = configuredBaseURL

    const configuredPath = await GetConfig('Game', 'StartupFilePath')
    if (configuredPath) {
      gamePath.value = configuredPath
      savedGamePath.value = configuredPath
      await searchAndLoadBepInEx(configuredPath, {showDialog: false, showLoading: false})
      return
    }

    savedGamePath.value = gamePath.value
  } catch (error) {
    message.error(`读取设置失败：${formatErrorMessage(error)}`)
  }
})

const selectGamePath = async (): Promise<void> => {
  if (isSelectingGamePath.value) {
    return
  }

  isSelectingGamePath.value = true
  try {
    const selectedPath = await SelectDysonSphereDirectory()
    if (selectedPath) {
      gamePath.value = selectedPath
      await searchAndLoadBepInEx(selectedPath, {showDialog: false, showLoading: false})
      message.success('已选择游戏路径，并完成 BepInEx 检查')
      return
    }

    message.warning('未选择游戏路径，当前配置未改变')
  } catch (error) {
    message.error(`选择游戏路径失败：${formatErrorMessage(error)}`)
  } finally {
    isSelectingGamePath.value = false
  }
}

const loadGamePathFromEnvironment = async (): Promise<void> => {
  if (isLoadingGamePath.value) {
    return
  }

  isLoadingGamePath.value = true
  try {
    const detectedPath = await LoadOrInitDysonSphereExe()
    if (detectedPath) {
      gamePath.value = detectedPath
      await searchAndLoadBepInEx(detectedPath, {showDialog: false, showLoading: false})
      message.success('已找到游戏路径，并完成 BepInEx 检查')
      return
    }

    message.warning('未找到 DSPGAME.exe，请手动选择游戏目录')
  } catch (error) {
    message.error(`自动获取失败：${formatErrorMessage(error)}`)
  } finally {
    isLoadingGamePath.value = false
  }
}

const confirmGamePath = async (): Promise<void> => {
  if (isSavingGamePath.value || !hasPendingGamePathChange.value) {
    return
  }

  isSavingGamePath.value = true
  try {
    const nextGamePath = gamePath.value.trim()
    await SetConfig('Game', 'StartupFilePath', nextGamePath)
    gamePath.value = nextGamePath
    savedGamePath.value = nextGamePath
    await searchAndLoadBepInEx(nextGamePath, {showDialog: false, showLoading: false})
    message.success('游戏路径已保存')
  } catch (error) {
    message.error(`保存游戏路径失败：${formatErrorMessage(error)}`)
  } finally {
    isSavingGamePath.value = false
  }
}

// =============== 基础地址保存 ===============
// 说明：保存后端 System/BaseURL 配置，并触发后端全局 HTTP 客户端刷新。
// 边界：允许留空；非空时必须是包含协议和主机名的 HTTP/HTTPS 地址。
const confirmBaseURL = async (): Promise<void> => {
  if (isSavingBaseURL.value || !canSaveBaseURL.value) {
    return
  }

  isSavingBaseURL.value = true
  try {
    const nextBaseURL = baseURL.value.trim()
    await SetConfig('System', 'BaseURL', nextBaseURL)
    baseURL.value = nextBaseURL
    savedBaseURL.value = nextBaseURL
    message.success(nextBaseURL ? '连接地址已保存' : '连接地址已清空')
  } catch (error) {
    message.error(`保存连接地址失败：${formatErrorMessage(error)}`)
  } finally {
    isSavingBaseURL.value = false
  }
}

const openAppUpdate = (): void => {
  updateAppVisible.value = true
}

const openBepInExUninstall = (): void => {
  uninstallBepInExVisible.value = true
}

interface AppUpdateCheckOptions {
  readonly silentWhenNoUpdate?: boolean
}

// =============== 应用更新检查 ===============
// 说明：调用后端 Gitee Release 更新源，读取当前程序版本、最新版本与匹配的更新包。
// 边界：只检查和展示远端状态，不会下载或替换当前程序。
const checkAppUpdate = async (options: AppUpdateCheckOptions = {}): Promise<void> => {
  if (isCheckingAppUpdate.value) {
    return
  }

  isCheckingAppUpdate.value = true
  try {
    const result = await CheckAppUpdate()
    appUpdateInfo.value = result
    if (result.hasUpdate) {
      message.success(`发现新版本 ${result.latestVersion}`)
      return
    }

    if (!options.silentWhenNoUpdate) {
      message.success('当前已经是最新版本')
    }
  } catch (error) {
    message.error(`检查应用更新失败：${formatErrorMessage(error)}`)
  } finally {
    isCheckingAppUpdate.value = false
  }
}

// =============== 应用自更新 ===============
// 说明：用户确认后下载 Gitee Release 附件，校验 checksums.txt，并替换当前可执行文件。
// 边界：更新完成后不会强制退出应用，需要用户手动重启以运行新版本。
const confirmAppUpdate = async (): Promise<void> => {
  if (isUpdatingApp.value) {
    return
  }

  isUpdatingApp.value = true
  try {
    const result = await UpdateAppSelf()
    appUpdateInfo.value = result.check
    updateAppVisible.value = false
    message.success(result.message || '更新完成，请重启应用')
  } catch (error) {
    message.error(`应用更新失败：${formatErrorMessage(error)}`)
  } finally {
    isUpdatingApp.value = false
  }
}

// =============== BepInEx 卸载 ===============
// 说明：调用后端真实卸载逻辑，移除 BepInEx、Doorstop 注入文件和所有本地 Mod 依赖记录。
// 边界：该操作破坏性较强，只能从二次确认弹窗触发；失败时保留当前状态并提示错误。
const confirmBepInExUninstall = async (): Promise<void> => {
  if (isUninstallingBepInEx.value) {
    return
  }

  isUninstallingBepInEx.value = true
  try {
    const result = await Uninstall()
    uninstallBepInExVisible.value = false
    bepinExStatus.value = result.status
    message.success(result.message || 'BepInEx 已卸载')
  } catch (error) {
    message.error(`BepInEx 卸载失败：${formatErrorMessage(error)}`)
  } finally {
    isUninstallingBepInEx.value = false
  }
}

interface BepInExSearchOptions {
  readonly showDialog?: boolean
  readonly showLoading?: boolean
}

const searchAndLoadBepInEx = async (
  selectedPath: string = gamePath.value,
  options: BepInExSearchOptions = {},
): Promise<void> => {
  const {
    showDialog = true,
    showLoading = true,
  } = options

  if (isLoadingBepInExStatus.value) {
    return
  }

  isLoadingBepInExStatus.value = true
  if (showLoading) {
    isCheckingBepInEx.value = true
  }
  try {
    const status = await SearchAndLoadBepInEx(selectedPath.trim())
    bepinExStatus.value = status
    if (showDialog) {
      message.success(
        status.installed
          ? `已安装 BepInEx ${status.installedVersion || '未知版本'}`
          : '当前游戏目录下未安装 BepInEx',
      )
    }
  } catch (error) {
    bepinExStatus.value = null
    if (showDialog) {
      message.error(`BepInEx 检查失败：${formatErrorMessage(error)}`)
    }
  } finally {
    isLoadingBepInExStatus.value = false
    if (showLoading) {
      isCheckingBepInEx.value = false
    }
  }
}

const formatErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }

  return String(error || '未知错误')
}
</script>

<template>
  <section class="mx-auto max-w-3xl px-8 py-8">
    <h1 class="mb-6 text-xl font-semibold text-zinc-950">设置</h1>

    <div class="space-y-8">
      <section>
        <label class="mb-2 block text-sm font-semibold text-zinc-950">游戏路径</label>
        <div class="flex gap-2">
          <n-input v-model:value="gamePath" placeholder="请选择游戏安装目录" />
          <n-button secondary :loading="isSelectingGamePath" @click="selectGamePath">
            <template #icon>
              <FolderOpen :size="16" :stroke-width="1.8" />
            </template>
            选择
          </n-button>
          <n-button secondary :loading="isLoadingGamePath" @click="loadGamePathFromEnvironment">
            <template #icon>
              <Search :size="16" :stroke-width="1.8" />
            </template>
            自动获取
          </n-button>
        </div>
        <div v-if="hasPendingGamePathChange" class="mt-3 flex items-center gap-3">
          <n-button type="primary" :loading="isSavingGamePath" @click="confirmGamePath">确认修改游戏路径</n-button>
          <span class="text-xs text-zinc-500">确认后会同步保存到 GameStartFilePath。</span>
        </div>
        <p v-if="!isGamePathValid" class="mt-2 text-sm text-red-600">未在该目录下找到 DSPGAME.exe</p>
        <p v-else class="mt-2 text-xs text-zinc-500">已自动检测到游戏路径</p>
      </section>

      <!-- ~ 连接地址配置 -->
      <!-- * 说明：维护后端 HTTP 客户端使用的 System/BaseURL 配置 -->
      <!-- * 交互：地址变化后显示保存按钮，保存成功后同步刷新后端全局客户端 -->
      <!-- * 依赖：依赖 ConfigurationService.GetConfig 与 ConfigurationService.SetConfig -->
      <!-- * 边界：允许清空地址；非空地址必须包含 http/https 协议与主机名 -->
      <section>
        <label class="mb-2 block text-sm font-semibold text-zinc-950">连接地址</label>
        <div class="flex gap-2">
          <n-input
            v-model:value="baseURL"
            placeholder="例如：https://api.example.com"
            :status="baseURLValidationMessage ? 'error' : undefined"
          />
          <n-button
            type="primary"
            :disabled="!canSaveBaseURL"
            :loading="isSavingBaseURL"
            @click="confirmBaseURL"
          >
            <template #icon>
              <Save :size="16" :stroke-width="1.8" />
            </template>
            保存
          </n-button>
        </div>
        <p v-if="baseURLValidationMessage" class="mt-2 text-sm text-red-600">{{ baseURLValidationMessage }}</p>
        <p v-else-if="hasPendingBaseURLChange" class="mt-2 text-xs text-amber-600">保存后会同步更新全局 HTTP 客户端。</p>
        <p v-else-if="savedBaseURL" class="mt-2 text-xs text-zinc-500">当前全局 HTTP 客户端会以该地址作为 BaseURL。</p>
        <p v-else class="mt-2 text-xs text-zinc-500">未配置时不会主动请求远程服务。</p>
      </section>

      <section>
        <label class="mb-2 block text-sm font-semibold text-zinc-950">BepInEx</label>
        <div class="flex gap-2">
          <n-input :value="bepinExSummary" readonly />
          <n-button secondary :loading="isCheckingBepInEx" @click="searchAndLoadBepInEx()">
            <template #icon>
              <RefreshCw :size="16" :stroke-width="1.8" />
            </template>
            检查
          </n-button>
          <n-button :disabled="!bepinExStatus?.installed" secondary type="error" :loading="isUninstallingBepInEx" @click="openBepInExUninstall">
            <template #icon>
              <Trash2 :size="16" :stroke-width="1.8" />
            </template>
            卸载
          </n-button>
        </div>
        <p v-if="!bepinExStatus" class="mt-2 text-xs text-zinc-500">选择游戏目录后会自动检查 BepInEx。</p>
        <p v-else-if="!bepinExStatus.installed" class="mt-2 text-sm text-red-600">未在当前游戏目录下找到 BepInEx。</p>
        <p v-else class="mt-2 text-xs text-zinc-500">检测到 BepInEx 文件存在，版本仅展示本地读取结果。</p>
      </section>

      <!-- ~ 应用更新 -->
      <!-- * 说明：展示戴森球盒子当前版本与 Gitee Release 最新版本 -->
      <!-- * 交互：用户先检查更新，发现新版本后可打开确认弹窗并执行自更新 -->
      <!-- * 依赖：依赖 UpdateService.CheckAppUpdate 与 UpdateService.UpdateAppSelf -->
      <!-- * 边界：更新完成后需要用户重启应用，开发版构建会被正式 Release 视为可更新 -->
      <section>
        <label class="mb-2 block text-sm font-semibold text-zinc-950">应用更新</label>
        <div class="flex gap-2">
          <n-input :value="appUpdateSummary" readonly />
          <n-button secondary :loading="isCheckingAppUpdate" @click="() => checkAppUpdate()">
            <template #icon>
              <RefreshCw :size="16" :stroke-width="1.8" />
            </template>
            检查
          </n-button>
          <n-button type="primary" :disabled="!canUpdateApp" :loading="isUpdatingApp" @click="openAppUpdate">
            <template #icon>
              <Download :size="16" :stroke-width="1.8" />
            </template>
            更新
          </n-button>
        </div>
        <p v-if="appUpdateInfo?.hasUpdate" class="mt-2 text-sm text-amber-600">
          将下载 {{ appUpdateInfo.assetName }}，校验通过后替换当前程序。
        </p>
        <p v-else-if="appUpdateInfo" class="mt-2 text-xs text-zinc-500">当前运行环境：{{ appUpdateInfo.os }}/{{ appUpdateInfo.arch }}</p>
        <p v-else class="mt-2 text-xs text-zinc-500">更新包来自 Gitee Release，并使用 checksums.txt 校验完整性。</p>
      </section>
    </div>

    <n-modal
      v-model:show="updateAppVisible"
      preset="dialog"
      title="更新戴森球盒子"
      positive-text="开始更新"
      negative-text="取消"
      :positive-button-props="{ type: 'primary', loading: isUpdatingApp, disabled: isUpdatingApp }"
      :negative-button-props="{ disabled: isUpdatingApp }"
      :mask-closable="!isUpdatingApp"
      @positive-click="confirmAppUpdate"
    >
      <div class="space-y-4 text-sm text-zinc-600">
        <p>将下载并校验最新版本，更新完成后需要重启应用。</p>
        <div class="grid grid-cols-[6rem_1fr] gap-y-2 rounded border border-zinc-200 bg-zinc-50 px-3 py-3 text-xs">
          <span class="text-zinc-500">当前版本</span>
          <span class="font-medium text-zinc-950">{{ appUpdateInfo?.currentVersion || '未知版本' }}</span>
          <span class="text-zinc-500">最新版本</span>
          <span class="font-medium text-zinc-950">{{ appUpdateInfo?.latestVersion || '未知版本' }}</span>
          <span class="text-zinc-500">更新包</span>
          <span class="font-medium text-zinc-950">{{ appUpdateInfo?.assetName || '未匹配' }}</span>
        </div>
        <p v-if="appUpdateInfo?.releaseNotes" class="text-xs leading-5 text-zinc-500">{{ appUpdateInfo.releaseNotes }}</p>
      </div>
    </n-modal>

    <n-modal
      v-model:show="uninstallBepInExVisible"
      preset="dialog"
      title="卸载 BepInEx"
      positive-text="确认卸载"
      negative-text="取消"
      :positive-button-props="{ type: 'error', loading: isUninstallingBepInEx, disabled: isUninstallingBepInEx }"
      :negative-button-props="{ disabled: isUninstallingBepInEx }"
      :mask-closable="!isUninstallingBepInEx"
      @positive-click="confirmBepInExUninstall"
    >
      <div class="space-y-4 text-sm text-zinc-600">
        <p>将从当前游戏目录移除 BepInEx、Doorstop 注入文件和所有已安装的依赖 Mod。</p>
        <div class="rounded border border-red-200 bg-red-50 px-3 py-3 text-xs leading-5 text-red-700">
          该操作不会删除《戴森球计划》本体文件，但会删除 BepInEx/plugins 下的所有 Mod，并清空本地 Mod 安装记录。
        </div>
        <p class="text-xs text-zinc-500">确认后需要重新安装 BepInEx 与整合包才能继续使用 Mod。</p>
      </div>
    </n-modal>
  </section>
</template>
