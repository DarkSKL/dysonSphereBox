<script setup lang="ts">
import { AlertCircle, CheckCircle2, Database, RefreshCw, Search } from '@lucide/vue'
import { useModPageState } from '../composables/useModPageState'

definePage({
  meta: {
    layout: 'main',
    keepAlive: true,
    title: '模组',
  },
})

const {
  router,
  sourceOptions,
  selectedSource,
  searchKeyword,
  dialogVisible,
  isCheckingGamePath,
  isCheckingBepInEx,
  isInstallingBepInEx,
  canOpenModPage,
  isBepInExInstalled,
  packageList,
  loadMoreTarget,
  installedPackageKeys,
  installingPackageKeys,
  failedPackageKeys,
  packageProgress,
  packageProgressStatus,
  packageDownloadSpeeds,
  packageFailureMessages,
  packageConflicts,
  selectedPack,
  hasLoadedAllPackages,
  isManualSource,
  canInstallPackages,
  selectedSourceLabel,
  bepinExStatusTitle,
  bepinExStatusDescription,
  packageLoadError,
  isLoadingPackages,
  openPack,
  handleUninstall,
  openConflictDialog,
  cancelInstall,
  handleInstall,
  refreshPackages,
  handleSourceChange,
  installBepInExFromModPage,
  resolveCoverUrl,
} = useModPageState()
</script>

<template>
  <section class="mx-auto max-w-6xl px-8 py-8">
    <div class="relative min-h-[30rem]">
      <!-- ~ 整合包工具栏 -->
      <!-- * 说明：同层展示 BepInEx 前置状态、整合包搜索、来源切换与刷新入口 -->
      <!-- * 交互：未安装 BepInEx 时可直接安装；搜索框按关键词刷新第一页，滚动到底部自动请求下一页 -->
      <!-- * 依赖：依赖 BepInExService.Install、MarketplaceService.GetPackageList 与分页状态 -->
      <!-- * 边界：游戏路径无效时整体虚化并阻止列表交互 -->
      <div
        class="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <div
          v-if="canOpenModPage"
          class="flex h-[34px] min-w-0 flex-1 items-center justify-between gap-4 border px-3"
          :class="isBepInExInstalled ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : 'border-amber-200 bg-amber-50 text-amber-800'"
        >
          <div class="flex h-full min-w-0 flex-1 items-center gap-2">
            <CheckCircle2
              v-if="isBepInExInstalled"
              :size="16"
              :stroke-width="1.9"
              class="shrink-0 text-emerald-700"
            />
            <AlertCircle
              v-else
              :size="16"
              :stroke-width="1.9"
              class="shrink-0 text-amber-700"
            />
            <p class="shrink-0 text-sm font-semibold leading-none">{{ bepinExStatusTitle }}</p>
            <p class="ml-auto min-w-0 truncate text-right text-xs leading-none opacity-80">{{ bepinExStatusDescription }}</p>
          </div>
          <n-button
            v-if="!isBepInExInstalled"
            class="shrink-0"
            type="primary"
            :loading="isInstallingBepInEx"
            :disabled="isCheckingBepInEx"
            @click="installBepInExFromModPage"
          >
            安装 BepInEx
          </n-button>
        </div>
        <!-- ~ 整合包搜索 -->
        <!-- * 说明：在 BepInEx 状态和渠道来源之间提供关键词搜索 -->
        <!-- * 交互：输入后自动刷新列表，清空后恢复当前来源的默认分页 -->
        <!-- * 依赖：依赖 searchKeyword 与 MarketplaceService 的 keyword 参数 -->
        <!-- * 边界：桌面宽度固定占工具栏 30%，不参与左右组件的空间分配 -->
        <div v-if="canOpenModPage" class="w-full md:basis-[30%] md:max-w-[30%] md:shrink-0">
          <n-input
            v-model:value="searchKeyword"
            class="mod-search-input"
            size="small"
            clearable
            placeholder="搜索模组"
            :disabled="isManualSource"
          >
            <template #prefix>
              <Search :size="15" :stroke-width="1.8" class="text-zinc-400" />
            </template>
          </n-input>
        </div>
        <div class="flex shrink-0 items-center gap-2 self-end md:self-auto">
          <n-popselect
            v-model:value="selectedSource"
            :options="sourceOptions"
            trigger="click"
            @update:value="handleSourceChange"
          >
            <n-button class="h-[34px] shrink-0" secondary>
              <template #icon>
                <Database :size="16" :stroke-width="1.8" />
              </template>
              {{ selectedSourceLabel }}
            </n-button>
          </n-popselect>
          <n-button
            class="h-[34px] shrink-0"
            secondary
            :loading="isLoadingPackages"
            :disabled="isManualSource"
            @click="() => refreshPackages()"
          >
            <template #icon>
              <RefreshCw :size="16" :stroke-width="1.8" />
            </template>
            刷新
          </n-button>
        </div>
      </div>

      <!-- ~ 整合包列表 -->
      <!-- * 说明：以卡片网格展示后端分页返回的模组整合包 -->
      <!-- * 交互：点击卡片打开详情，底部哨兵进入视口后加载下一页 -->
      <!-- * 依赖：依赖 packageList、installedPackageKeys、installingPackageKeys 与后端安装器状态 -->
      <!-- * 边界：首屏加载、空列表和请求失败都有独立反馈，不遮挡游戏路径检查层 -->
      <div
        class="mb-8 grid grid-cols-2 gap-4 md:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6"
        :class="(isCheckingGamePath || !canOpenModPage) ? 'pointer-events-none select-none blur-sm' : ''"
      >
        <ModPackCard
          v-for="pack in packageList"
          :key="pack.key"
          :package-key="pack.key"
          :title="pack.displayName || '未命名整合包'"
          :description="pack.description || '暂无整合包说明。'"
          :image-url="resolveCoverUrl(pack.coverUrl)"
          :file-count="pack.dependencyCount || pack.dependencies?.length || 0"
          :download-count="pack.downloadCount || 0"
          :author="pack.author || ''"
          :installed="installedPackageKeys[pack.key] || false"
          :installing="installingPackageKeys[pack.key] || false"
          :failed="failedPackageKeys[pack.key] || false"
          :conflicted="Boolean(packageConflicts[pack.key])"
          :progress="packageProgress[pack.key] || 0"
          :progress-status="packageProgressStatus[pack.key] || ''"
          :download-speed="packageDownloadSpeeds[pack.key] || ''"
          :failure-message="packageFailureMessages[pack.key] || ''"
          @open="openPack"
          @uninstall="handleUninstall"
          @repair="(key) => openConflictDialog(key, true)"
          @cancel="cancelInstall"
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
          {{ isManualSource ? '暂时不支持手动导入' : '暂无可用整合包。' }}
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
      <!-- * 说明：游戏路径未准备好时阻止模组市场操作 -->
      <!-- * 交互：路径缺失时提供跳转设置页入口 -->
      <!-- * 依赖：依赖 CheckDysonSphereExe 与 settings 页保存的 Game/StartupFilePath -->
      <!-- * 边界：检查失败按不可用处理，避免在错误目录下继续安装 -->
      <div v-if="isCheckingGamePath || !canOpenModPage" class="absolute inset-0 z-10 grid place-items-center bg-white/65 backdrop-blur-sm">
        <div class="max-w-md px-6 text-center">
          <h1 class="text-xl font-semibold text-zinc-950">{{ isCheckingGamePath ? '正在检测游戏路径' : '无法开启模组' }}</h1>
          <p class="mt-3 text-sm leading-6 text-zinc-500">
            {{ isCheckingGamePath ? '正在确认 DSPGAME.exe 是否存在。' : '未检测到 DSPGAME.exe，请正确选择《戴森球计划》游戏路径。' }}
          </p>
          <n-button v-if="!isCheckingGamePath" class="mt-6" type="primary" @click="router.push('/settings')">前往设置</n-button>
        </div>
      </div>
    </div>

    <!-- ~ 整合包详情弹窗 -->
    <!-- * 说明：展示当前选中整合包的版本、作者、来源与依赖明细 -->
    <!-- * 交互：点击安装后由后端安装器自动处理 BepInEx、下载、解压与本地记录 -->
    <!-- * 依赖：依赖 selectedPack、installedPackageKeys 与 installingPackageKeys -->
    <!-- * 边界：未选中整合包时弹窗内容为空，不触发安装事件 -->
    <ModPackDialog
      v-model:show="dialogVisible"
      :pack="selectedPack"
      :installed="selectedPack ? installedPackageKeys[selectedPack.key] || false : false"
      :installing="selectedPack ? installingPackageKeys[selectedPack.key] || false : false"
      :can-install="canInstallPackages"
      install-disabled-text="需要优先安装 BepInEx 后才能安装整合包。"
      @install="handleInstall"
      @uninstall="handleUninstall"
    />
  </section>
</template>

<style scoped lang="scss">
.mod-search-input {
  --n-height: 34px;

  :deep(.n-input-wrapper) {
    align-items: center;
    height: 100%;
  }

  :deep(.n-input__input),
  :deep(.n-input__input-el) {
    height: 100%;
    line-height: 34px;
  }
}
</style>
