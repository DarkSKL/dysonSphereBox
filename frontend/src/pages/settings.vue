<script setup lang="ts">
import {Download, FolderOpen, RefreshCw, Save, Search, Trash2} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, onMounted, ref} from 'vue'
import {SelectDysonSphereDirectory} from '../../wailsjs/go/main/App'
import {GetLatestVersion, SearchAndLoadBepInEx} from '../../wailsjs/go/modDependence/BepInExService'
import {modDependence} from '../../wailsjs/go/models'
import {GetConfig, SetConfig} from '../../wailsjs/go/system/ConfigurationService'
import {LoadOrInitDysonSphereExe} from '../../wailsjs/go/system/EnvironmentService'
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
const updateBepInExVisible = ref(false)
const uninstallBepInExVisible = ref(false)
const savedGamePath = ref(gamePath.value)
const baseURL = ref('')
const savedBaseURL = ref('')
const message = useMessage()
const bepinExStatus = ref<modDependence.BepInExStatus | null>(null)
const latestBepInExVersion = ref('')
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
const hasBepInExUpdate = computed(() => {
  const installedVersion = normalizeVersionForComparison(bepinExStatus.value?.installedVersion || '')
  const latestVersion = normalizeVersionForComparison(latestBepInExVersion.value)

  return Boolean(
    bepinExStatus.value?.installed &&
      installedVersion &&
      latestVersion &&
      installedVersion !== latestVersion,
  )
})
const bepinExSummary = computed(() => {
  if (!bepinExStatus.value) {
    return '尚未检查 BepInEx'
  }
  if (!bepinExStatus.value.installed) {
    return '未安装 BepInEx'
  }

  return `已安装 BepInEx ${bepinExStatus.value.installedVersion || '未知版本'}`
})

onMounted(async () => {
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

const openBepInExUpdate = (): void => {
  updateBepInExVisible.value = true
}

const openBepInExUninstall = (): void => {
  uninstallBepInExVisible.value = true
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
    try {
      const latestPackage = await GetLatestVersion()
      latestBepInExVersion.value = latestPackage.version
    } catch {
      latestBepInExVersion.value = ''
    }
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

const normalizeVersionForComparison = (version: string): string => {
  const parts = version.trim().replace(/^v/i, '').split('.')
  while (parts.length > 0 && parts[parts.length - 1] === '0') {
    parts.pop()
  }

  return parts.join('.')
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
          <n-button :disabled="!hasBepInExUpdate" type="primary" @click="openBepInExUpdate">
            <template #icon>
              <Download :size="16" :stroke-width="1.8" />
            </template>
            更新
          </n-button>
          <n-button :disabled="!bepinExStatus?.installed" secondary type="error" @click="openBepInExUninstall">
            <template #icon>
              <Trash2 :size="16" :stroke-width="1.8" />
            </template>
            卸载
          </n-button>
        </div>
        <p v-if="!bepinExStatus" class="mt-2 text-xs text-zinc-500">选择游戏目录后会自动检查 BepInEx。</p>
        <p v-else-if="!bepinExStatus.installed" class="mt-2 text-sm text-red-600">未在当前游戏目录下找到 BepInEx。</p>
        <p v-else-if="hasBepInExUpdate" class="mt-2 text-sm text-amber-600">发现新版本：{{ latestBepInExVersion }}</p>
        <p v-else class="mt-2 text-xs text-zinc-500">当前已是最新版本。</p>
      </section>
    </div>

    <n-modal v-model:show="updateBepInExVisible" preset="dialog" title="更新 BepInEx" positive-text="开始更新" negative-text="取消">
      <div class="space-y-4 text-sm text-zinc-600">
        <p>将把 BepInEx 从当前版本更新到最新版本。</p>
        <div class="grid grid-cols-[6rem_1fr] gap-y-2 rounded border border-zinc-200 bg-zinc-50 px-3 py-3 text-xs">
          <span class="text-zinc-500">当前版本</span>
          <span class="font-medium text-zinc-950">{{ bepinExStatus?.installedVersion || '未知版本' }}</span>
          <span class="text-zinc-500">最新版本</span>
          <span class="font-medium text-zinc-950">{{ latestBepInExVersion || '未知版本' }}</span>
        </div>
        <p class="text-xs text-zinc-500">更新前会保留已有配置文件，实际更新逻辑稍后接入。</p>
      </div>
    </n-modal>

    <n-modal v-model:show="uninstallBepInExVisible" preset="dialog" title="卸载 BepInEx" positive-text="确认卸载" negative-text="取消">
      <div class="space-y-4 text-sm text-zinc-600">
        <p>将从当前游戏目录移除 BepInEx 相关文件。</p>
        <div class="rounded border border-red-200 bg-red-50 px-3 py-3 text-xs leading-5 text-red-700">
          该操作不会删除《戴森球计划》本体文件，但可能导致依赖 BepInEx 的 Mod 无法继续运行。
        </div>
        <p class="text-xs text-zinc-500">实际卸载逻辑稍后接入。</p>
      </div>
    </n-modal>
  </section>
</template>
