<script setup lang="ts">
import {computed, ref} from 'vue'
import {useRouter} from 'vue-router'
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
  isGamePathValid,
  isModEnvironmentInstalled,
  installPackage,
  uninstallPackage,
} = useWorkshopState()

const selectedPackKey = ref<ModPackageKey | null>(null)
const dialogVisible = ref(false)

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
</script>

<template>
  <section class="mx-auto max-w-6xl px-8 py-8">
    <div v-if="!isGamePathValid" class="grid min-h-[26rem] place-items-center">
      <div class="max-w-md text-center">
        <h1 class="text-xl font-semibold text-zinc-950">请先设置游戏路径</h1>
        <p class="mt-3 text-sm leading-6 text-zinc-500">未检测到《戴森球计划》游戏路径，请前往设置页面配置。</p>
        <n-button class="mt-6" type="primary" @click="router.push('/settings')">前往设置</n-button>
      </div>
    </div>

    <template v-else>
      <div class="mb-8">
        <div class="mb-2 flex items-center justify-between gap-4">
          <div class="flex items-center gap-2">
            <span class="text-sm text-zinc-500">游戏路径</span>
            <span
              class="rounded border px-2 py-0.5 text-xs"
              :class="isModEnvironmentInstalled ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'border-zinc-200 bg-zinc-50 text-zinc-500'"
            >
              {{ isModEnvironmentInstalled ? 'Mod 环境已安装' : '未安装 BepInEx' }}
            </span>
          </div>
          <n-button text type="primary" @click="router.push('/settings')">更改</n-button>
        </div>
        <div class="truncate rounded border border-zinc-200 bg-zinc-50 px-3 py-2 text-sm text-zinc-700">{{ gamePath }}</div>
      </div>

      <div class="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
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

      <p class="text-xs text-zinc-500">所有 Mod 文件均从国内服务器下载，无需加速器，不限速。</p>
    </template>

    <ModPackDialog
      v-model:show="dialogVisible"
      :pack="selectedPack"
      :installed="selectedPack ? installedPackages[selectedPack.key] : false"
      @install="installPackage"
    />
  </section>
</template>
