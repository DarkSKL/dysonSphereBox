<script setup lang="ts">
import {createDiscreteApi} from 'naive-ui'
import {computed, onActivated, onMounted, ref} from 'vue'
import {useRouter} from 'vue-router'
import {SearchAndLoadBepInEx} from '../../wailsjs/go/modDependence/BepInExService'
import {GetConfig} from '../../wailsjs/go/system/ConfigurationService'
import {CheckDysonSphereExe} from '../../wailsjs/go/system/EnvironmentService'
import {modPackages, type ModPackage, type ModPackageKey, useWorkshopState} from '../composables/useWorkshopState'

definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: 'Mod',
  },
})

const router = useRouter()
const {
  gamePath,
  installedPackages,
  installingPackages,
  packageProgress,
  installPackage,
  uninstallPackage,
} = useWorkshopState()

const selectedPackKey = ref<ModPackageKey | null>(null)
const dialogVisible = ref(false)
const isCheckingGamePath = ref(true)
const isCheckingBepInEx = ref(false)
const canOpenModPage = ref(false)
const isBepInExInstalled = ref(false)
const bepInExVersion = ref('')
const {dialog} = createDiscreteApi(['dialog'])

const selectedPack = computed<ModPackage | undefined>(() => {
  return modPackages.find((pack) => pack.key === selectedPackKey.value)
})

const openPack = (key: ModPackageKey): void => {
  selectedPackKey.value = key
  dialogVisible.value = true
}

const handleUninstall = (key: ModPackageKey): void => {
  uninstallPackage(key)
}

const handleInstall = async (key: ModPackageKey): Promise<void> => {
  if (!(await ensureBepInExReady(true))) {
    return
  }

  installPackage(key)
}

const refreshGamePathAvailability = async (): Promise<void> => {
  isCheckingGamePath.value = true
  try {
    const configuredPath = await GetConfig('Game', 'StartupFilePath')
    const nextGamePath = configuredPath || gamePath.value
    if (configuredPath) {
      gamePath.value = configuredPath
    }

    canOpenModPage.value = await CheckDysonSphereExe(nextGamePath)
    if (canOpenModPage.value) {
      await refreshBepInExAvailability(nextGamePath)
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

onMounted(refreshGamePathAvailability)
onActivated(refreshGamePathAvailability)
</script>

<template>
  <section class="mx-auto max-w-6xl px-8 py-8">
    <div class="relative min-h-[30rem]">
      <div
        class="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <ModPackCard
          v-for="pack in modPackages"
          :key="pack.key"
          :package-key="pack.key"
          :title="pack.title"
          :description="pack.description"
          :file-count="pack.fileCount"
          :installed="installedPackages[pack.key]"
          :installing="installingPackages[pack.key]"
          :progress="packageProgress[pack.key]"
          @open="openPack"
          @uninstall="handleUninstall"
        />
      </div>

      <p
        class="text-xs text-zinc-500"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <span v-if="isCheckingBepInEx">正在检查 BepInEx 前置框架。</span>
        <span v-else-if="isBepInExInstalled">已检测到 BepInEx {{ bepInExVersion || '未知版本' }}，可安装整合包。</span>
        <span v-else>安装整合包前需要先安装 BepInEx 前置框架。</span>
      </p>

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

    <ModPackDialog
      v-model:show="dialogVisible"
      :pack="selectedPack"
      :installed="selectedPack ? installedPackages[selectedPack.key] : false"
      @install="handleInstall"
    />
  </section>
</template>
