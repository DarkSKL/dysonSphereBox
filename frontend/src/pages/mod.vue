<script setup lang="ts">
import {AlertCircle, RefreshCw} from '@lucide/vue'
import {createDiscreteApi, useMessage} from 'naive-ui'
import {computed, nextTick, onActivated, onBeforeUnmount, onMounted, ref} from 'vue'
import {useRouter} from 'vue-router'
import {SearchAndLoadBepInEx} from '../../wailsjs/go/modDependence/BepInExService'
import {GetPackageList} from '../../wailsjs/go/modDependence/MarketplaceService'
import {response} from '../../wailsjs/go/models'
import {GetConfig} from '../../wailsjs/go/system/ConfigurationService'
import {CheckDysonSphereExe} from '../../wailsjs/go/system/EnvironmentService'
import {useWorkshopState} from '../composables/useWorkshopState'

definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: 'Mod',
  },
})

const pageSize = 10

const router = useRouter()
const {
  gamePath,
} = useWorkshopState()

const selectedPackId = ref<number | null>(null)
const dialogVisible = ref(false)
const isCheckingGamePath = ref(true)
const isCheckingBepInEx = ref(false)
const canOpenModPage = ref(false)
const isBepInExInstalled = ref(false)
const bepInExVersion = ref('')
const baseURL = ref('')
const packageList = ref<response.ModDependencyPackageDTO[]>([])
const currentPage = ref(0)
const totalPackages = ref(0)
const reachedPackageEnd = ref(false)
const isLoadingPackages = ref(false)
const packageLoadError = ref('')
const loadMoreTarget = ref<HTMLElement | null>(null)
const installedPackageIds = ref<Record<number, boolean>>({})
const installingPackageIds = ref<Record<number, boolean>>({})
const packageProgress = ref<Record<number, number>>({})
const installTimers: Partial<Record<number, number>> = {}
const {dialog} = createDiscreteApi(['dialog'])
const message = useMessage()

let loadMoreObserver: IntersectionObserver | null = null

const selectedPack = computed<response.ModDependencyPackageDTO | undefined>(() => {
  return packageList.value.find((pack) => pack.id === selectedPackId.value)
})
const hasLoadedAllPackages = computed(() => {
  return reachedPackageEnd.value || (
    totalPackages.value > 0 &&
    packageList.value.length >= totalPackages.value
  )
})
const canLoadMorePackages = computed(() => {
  return canOpenModPage.value && !isLoadingPackages.value && !hasLoadedAllPackages.value
})

// =============== 整合包列表 ===============
// 说明：维护远端整合包分页数据，滚动到底部时追加下一页。
// 边界：接口失败时保留已加载数据，首屏失败展示重试入口。
const loadPackagePage = async (page: number, replace = false, showFeedback = false): Promise<void> => {
  if (isLoadingPackages.value) {
    return
  }

  isLoadingPackages.value = true
  packageLoadError.value = ''
  try {
    const result = await GetPackageList(page, pageSize)
    const nextList = result.list ?? []

    if (replace) {
      packageList.value = nextList
    } else {
      const existingIds = new Set(packageList.value.map((pack) => pack.id))
      packageList.value = [
        ...packageList.value,
        ...nextList.filter((pack) => !existingIds.has(pack.id)),
      ]
    }

    currentPage.value = result.page || page
    totalPackages.value = result.total || 0
    reachedPackageEnd.value = (
      (result.total > 0 && packageList.value.length >= result.total) ||
      nextList.length < pageSize
    )
    if (showFeedback) {
      message.success(replace ? '整合包列表已刷新' : '已加载下一页整合包')
    }
  } catch (error) {
    packageLoadError.value = formatErrorMessage(error)
    if (showFeedback) {
      message.error(`整合包加载失败：${packageLoadError.value}`)
    }
  } finally {
    isLoadingPackages.value = false
  }
}

const refreshPackages = async (showFeedback = true): Promise<void> => {
  await loadPackagePage(1, true, showFeedback)
  await nextTick()
  observeLoadMoreTarget()
}

const loadNextPackagePage = async (): Promise<void> => {
  if (!canLoadMorePackages.value) {
    return
  }

  await loadPackagePage(currentPage.value + 1, false, Boolean(packageLoadError.value))
}

// =============== 滚动加载 ===============
// 说明：用 IntersectionObserver 监听列表底部哨兵，进入可视区后请求下一页。
// 边界：页面被 KeepAlive 复用时会重复挂载观察器，因此每次绑定前先断开旧实例。
const observeLoadMoreTarget = (): void => {
  loadMoreObserver?.disconnect()
  if (!loadMoreTarget.value) {
    return
  }

  loadMoreObserver = new IntersectionObserver((entries) => {
    if (entries.some((entry) => entry.isIntersecting)) {
      loadNextPackagePage()
    }
  }, {
    root: null,
    rootMargin: '220px 0px',
    threshold: 0.01,
  })
  loadMoreObserver.observe(loadMoreTarget.value)
}

// =============== 整合包交互 ===============
// 说明：当前安装逻辑仍是本地占位状态，真实安装接口接入后可替换这里。
const openPack = (id: number): void => {
  selectedPackId.value = id
  dialogVisible.value = true
}

const handleUninstall = (id: number): void => {
  installedPackageIds.value[id] = false
}

const handleInstall = async (id: number): Promise<void> => {
  if (!(await ensureBepInExReady(true)) || installingPackageIds.value[id]) {
    return
  }

  installingPackageIds.value[id] = true
  packageProgress.value[id] = 0
  window.clearInterval(installTimers[id])
  installTimers[id] = window.setInterval(() => {
    packageProgress.value[id] = Math.min((packageProgress.value[id] || 0) + 4, 100)
    if (packageProgress.value[id] < 100) {
      return
    }

    window.clearInterval(installTimers[id])
    window.setTimeout(() => {
      installedPackageIds.value[id] = true
      installingPackageIds.value[id] = false
      packageProgress.value[id] = 0
      message.success('整合包安装状态已更新')
    }, 350)
  }, 120)
}

// =============== 封面地址 ===============
// 说明：根据 System/BaseURL 与接口 coverUrl 拼出可直接访问的封面图片地址。
// 边界：任一端为空时返回空字符串；绝对地址直接使用接口值。
const resolveCoverUrl = (coverUrl?: string): string => {
  const trimmedCoverUrl = (coverUrl || '').trim()
  if (!trimmedCoverUrl) {
    return ''
  }
  if (/^https?:\/\//i.test(trimmedCoverUrl)) {
    return trimmedCoverUrl
  }

  const trimmedBaseURL = baseURL.value.trim()
  if (!trimmedBaseURL) {
    return ''
  }

  return `${trimmedBaseURL.replace(/\/+$/, '')}/${trimmedCoverUrl.replace(/^\/+/, '')}`
}

// =============== 游戏与前置框架检查 ===============
// 说明：沿用设置页保存的游戏路径，并在可打开 Mod 页时同步刷新 BepInEx 状态。
const refreshGamePathAvailability = async (): Promise<void> => {
  isCheckingGamePath.value = true
  try {
    baseURL.value = await GetConfig('System', 'BaseURL')
    const configuredPath = await GetConfig('Game', 'StartupFilePath')
    const nextGamePath = configuredPath || gamePath.value
    if (configuredPath) {
      gamePath.value = configuredPath
    }

    canOpenModPage.value = await CheckDysonSphereExe(nextGamePath)
    if (canOpenModPage.value) {
      await refreshBepInExAvailability(nextGamePath)
      if (packageList.value.length === 0 && !isLoadingPackages.value) {
        await refreshPackages(false)
      }
    } else {
      isBepInExInstalled.value = false
      bepInExVersion.value = ''
    }
  } catch {
    canOpenModPage.value = false
    isBepInExInstalled.value = false
    bepInExVersion.value = ''
  } finally {
    isCheckingGamePath.value = false
  }
}

const refreshBepInExAvailability = async (path: string = gamePath.value): Promise<void> => {
  isCheckingBepInEx.value = true
  try {
    const status = await SearchAndLoadBepInEx(path)
    isBepInExInstalled.value = status.installed
    bepInExVersion.value = status.installedVersion
  } catch {
    isBepInExInstalled.value = false
    bepInExVersion.value = ''
  } finally {
    isCheckingBepInEx.value = false
  }
}

const ensureBepInExReady = async (showDialog: boolean): Promise<boolean> => {
  await refreshBepInExAvailability()
  if (isBepInExInstalled.value) {
    return true
  }

  if (showDialog) {
    dialog.warning({
      title: '缺少 BepInEx 前置框架',
      content: '安装整合包前需要先安装 BepInEx。请先到设置页安装或检查 BepInEx 前置框架。',
      positiveText: '前往设置',
      negativeText: '取消',
      onPositiveClick: () => {
        router.push('/settings')
      },
    })
  }

  return false
}

const formatErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message
  }

  return String(error || '未知错误')
}

onMounted(async () => {
  await refreshGamePathAvailability()
  await nextTick()
  observeLoadMoreTarget()
})
onActivated(async () => {
  await refreshGamePathAvailability()
  await nextTick()
  observeLoadMoreTarget()
})
onBeforeUnmount(() => {
  loadMoreObserver?.disconnect()
  Object.values(installTimers).forEach((timer) => {
    window.clearInterval(timer)
  })
})
</script>

<template>
  <section class="mx-auto max-w-6xl px-8 py-8">
    <div class="relative min-h-[30rem]">
      <!-- ~ 整合包工具栏 -->
      <!-- * 说明：提供远端整合包列表刷新入口 -->
      <!-- * 交互：点击刷新重新拉取第一页，滚动到底部自动请求下一页 -->
      <!-- * 依赖：依赖 MarketplaceService.GetPackageList 与分页状态 -->
      <!-- * 边界：游戏路径无效时整体虚化并阻止列表交互 -->
      <div
        class="mb-4 flex justify-end"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <n-button secondary :loading="isLoadingPackages" @click="() => refreshPackages()">
          <template #icon>
            <RefreshCw :size="16" :stroke-width="1.8" />
          </template>
          刷新
        </n-button>
      </div>

      <!-- ~ 整合包列表 -->
      <!-- * 说明：以卡片网格展示后端分页返回的 Mod 整合包 -->
      <!-- * 交互：点击卡片打开详情，底部哨兵进入视口后加载下一页 -->
      <!-- * 依赖：依赖 packageList、installedPackageIds、installingPackageIds 与 packageProgress -->
      <!-- * 边界：首屏加载、空列表和请求失败都有独立反馈，不遮挡游戏路径检查层 -->
      <div
        class="mb-8 grid grid-cols-2 gap-4 md:grid-cols-4"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <ModPackCard
          v-for="pack in packageList"
          :key="pack.id"
          :package-id="pack.id"
          :title="pack.packageName || '未命名整合包'"
          :description="pack.packageDescription || '暂无整合包说明。'"
          :image-url="resolveCoverUrl(pack.coverUrl)"
          :file-count="pack.dependencyCount || pack.dependencies?.length || 0"
          :download-count="pack.downloadCount || 0"
          :author="pack.author || ''"
          :installed="installedPackageIds[pack.id] || false"
          :installing="installingPackageIds[pack.id] || false"
          :progress="packageProgress[pack.id] || 0"
          @open="openPack"
          @uninstall="handleUninstall"
        />

        <template v-if="isLoadingPackages && packageList.length === 0">
          <div
            v-for="index in 8"
            :key="`skeleton-${index}`"
            class="aspect-square animate-pulse border border-zinc-200 bg-white"
          >
            <div class="h-24 border-b border-zinc-100 p-3">
              <div class="h-4 w-2/3 bg-zinc-100"></div>
              <div class="mt-3 h-3 w-full bg-zinc-100"></div>
              <div class="mt-2 h-3 w-4/5 bg-zinc-100"></div>
            </div>
            <div class="grid h-[calc(100%-6rem)] place-items-center bg-zinc-50">
              <div class="h-20 w-20 border border-zinc-200 bg-white"></div>
            </div>
          </div>
        </template>
      </div>

      <!-- ~ 列表状态 -->
      <!-- * 说明：承载空态、错误、加载更多与底部触发哨兵 -->
      <!-- * 交互：失败时允许重试；哨兵被观察到时自动加载下一页 -->
      <!-- * 依赖：依赖 packageLoadError、isLoadingPackages、hasLoadedAllPackages -->
      <!-- * 边界：已加载数据失败时只提示底部错误，不清空现有列表 -->
      <div
        class="pb-8"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <div
          v-if="packageLoadError && packageList.length === 0"
          class="flex items-center justify-between gap-4 border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"
        >
          <div class="flex min-w-0 items-center gap-2">
            <AlertCircle :size="16" :stroke-width="1.8" class="shrink-0" />
            <span class="truncate">{{ packageLoadError }}</span>
          </div>
          <n-button size="small" type="error" secondary @click="() => refreshPackages()">重试</n-button>
        </div>
        <div
          v-else-if="!isLoadingPackages && packageList.length === 0"
          class="border border-zinc-200 bg-white px-4 py-8 text-center text-sm text-zinc-500"
        >
          暂无可用整合包。
        </div>
        <div v-else class="text-center text-xs text-zinc-500">
          <span v-if="isLoadingPackages">正在加载整合包...</span>
          <span v-else-if="packageLoadError">{{ packageLoadError }}</span>
          <span v-else-if="hasLoadedAllPackages">已经到底了。</span>
          <span v-else>继续向下滚动加载更多。</span>
        </div>
        <div ref="loadMoreTarget" class="h-8"></div>
      </div>

<!--      <p-->
<!--        class="text-xs text-zinc-500"-->
<!--        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"-->
<!--      >-->
<!--        <span v-if="isCheckingBepInEx">正在检查 BepInEx 前置框架。</span>-->
<!--        <span v-else-if="isBepInExInstalled">已检测到 BepInEx {{ bepInExVersion || '未知版本' }}，可安装整合包。</span>-->
<!--        <span v-else>安装整合包前需要先安装 BepInEx 前置框架。</span>-->
<!--      </p>-->

      <!-- ~ 游戏路径遮罩 -->
      <!-- * 说明：游戏路径未准备好时阻止 Mod 市场操作 -->
      <!-- * 交互：路径缺失时提供跳转设置页入口 -->
      <!-- * 依赖：依赖 CheckDysonSphereExe 与 settings 页保存的 Game/StartupFilePath -->
      <!-- * 边界：检查失败按不可用处理，避免在错误目录下继续安装 -->
      <div v-if="isCheckingGamePath || !canOpenModPage" class="absolute inset-0 z-10 grid place-items-center bg-white/65 backdrop-blur-sm">
        <div class="max-w-md px-6 text-center">
          <h1 class="text-xl font-semibold text-zinc-950">{{ isCheckingGamePath ? '正在检测游戏路径' : '无法开启 Mod' }}</h1>
          <p class="mt-3 text-sm leading-6 text-zinc-500">
            {{ isCheckingGamePath ? '正在确认 DSPGAME.exe 是否存在。' : '未检测到 DSPGAME.exe，请正确选择《戴森球计划》游戏路径。' }}
          </p>
          <n-button v-if="!isCheckingGamePath" class="mt-6" type="primary" @click="router.push('/settings')">前往设置</n-button>
        </div>
      </div>
    </div>

    <!-- ~ 整合包详情弹窗 -->
    <!-- * 说明：展示当前选中整合包的版本、作者、来源与依赖明细 -->
    <!-- * 交互：点击安装前会检查 BepInEx 状态 -->
    <!-- * 依赖：依赖 selectedPack 与 installedPackageIds -->
    <!-- * 边界：未选中整合包时弹窗内容为空，不触发安装事件 -->
    <ModPackDialog
      v-model:show="dialogVisible"
      :pack="selectedPack"
      :installed="selectedPack ? installedPackageIds[selectedPack.id] || false : false"
      @install="handleInstall"
    />
  </section>
</template>
