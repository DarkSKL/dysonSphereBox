<script setup lang="ts">
import {AlertCircle, CheckCircle2, Database, LoaderCircle, RefreshCw, Search} from '@lucide/vue'
import ModPackCard from '../../components/dysonSphere/mod/pack-card.vue'
import ModPackDialog from '../../components/dysonSphere/mod/pack-dialog.vue'
import {useModPageState} from '../../composables/dysonSphere/useModPageState'

definePage({ meta: { layout: 'main', title: '模组', keepAlive: true } })

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
  <!-- ~ 模组页面 -->
  <!-- * 说明：顶部工具栏展示 BepInEx 状态、搜索、来源与刷新；下方为 Thunderstore 模组卡片网格 -->
  <!-- * 交互：点击卡片打开详情安装；滚动到底部自动加载下一页 -->
  <!-- * 依赖：依赖 useModPageState 收敛的 Marketplace / Installer / BepInEx 后端调用 -->
  <!-- * 边界：游戏路径无效时整体虚化并提示前往游戏设置，不允许任何安装操作 -->
  <section class="mod-page" aria-label="模组">
    <div class="mod-page-body" :class="{'is-gated': isCheckingGamePath || !canOpenModPage}">
      <!-- ~ 工具栏 -->
      <!-- * 说明：BepInEx 状态条占满剩余宽度，右侧为搜索、来源切换与刷新 -->
      <!-- * 交互：未安装 BepInEx 时状态条内提供安装按钮；搜索输入 300ms 后刷新第一页 -->
      <!-- * 依赖：依赖 bepinExStatusTitle / Description、searchKeyword、selectedSource -->
      <!-- * 边界：所有控件统一 34px 高度；窄屏时搜索与按钮整体换到下一行 -->
      <div class="mod-toolbar">
        <div class="mod-bepinex" :class="isBepInExInstalled ? 'is-ready' : 'is-missing'" role="status">
          <LoaderCircle v-if="isCheckingBepInEx" :size="15" :stroke-width="1.9" class="mod-bepinex-icon is-spinning" />
          <CheckCircle2 v-else-if="isBepInExInstalled" :size="15" :stroke-width="1.9" class="mod-bepinex-icon" />
          <AlertCircle v-else :size="15" :stroke-width="1.9" class="mod-bepinex-icon" />
          <span class="mod-bepinex-title">{{ bepinExStatusTitle }}</span>
          <span class="mod-bepinex-desc" :title="bepinExStatusDescription">{{ bepinExStatusDescription }}</span>
          <n-button
            v-if="!isBepInExInstalled && !isCheckingBepInEx"
            class="mod-bepinex-action"
            size="tiny"
            type="primary"
            :loading="isInstallingBepInEx"
            @click="installBepInExFromModPage"
          >
            安装 BepInEx
          </n-button>
        </div>

        <div class="mod-toolbar-actions">
          <n-input
            v-model:value="searchKeyword"
            class="mod-search"
            clearable
            placeholder="搜索模组"
            :disabled="isManualSource"
            :input-props="{spellcheck: false, autocomplete: 'off', 'aria-label': '搜索模组'}"
          >
            <template #prefix>
              <Search :size="15" :stroke-width="1.8" />
            </template>
          </n-input>
          <n-popselect
            v-model:value="selectedSource"
            :options="sourceOptions"
            trigger="click"
            @update:value="handleSourceChange"
          >
            <n-button secondary>
              <template #icon>
                <Database :size="15" :stroke-width="1.8" />
              </template>
              {{ selectedSourceLabel }}
            </n-button>
          </n-popselect>
          <n-button
            secondary
            :loading="isLoadingPackages"
            :disabled="isManualSource"
            @click="() => refreshPackages()"
          >
            <template #icon>
              <RefreshCw :size="15" :stroke-width="1.8" />
            </template>
            刷新
          </n-button>
        </div>
      </div>

      <!-- ~ 模组网格 -->
      <!-- * 说明：卡片展示名称、简介、作者、封面、组件数与下载量，并叠加安装进度/失败/冲突状态 -->
      <!-- * 交互：点击卡片打开详情；卡片上可取消安装、卸载或处理冲突 -->
      <!-- * 依赖：依赖 packageList 与按 packageKey 维护的安装状态映射 -->
      <!-- * 边界：首屏加载时展示骨架卡片，列数随内容区宽度自适应 -->
      <div class="mod-grid">
        <ModPackCard
          v-for="pack in packageList"
          :key="pack.key"
          :package-key="pack.key"
          :title="pack.displayName || pack.name || '未命名模组'"
          :description="pack.description || '暂无模组说明。'"
          :image-url="resolveCoverUrl(pack.coverUrl)"
          :file-count="pack.dependencyCount || pack.dependencies?.length || 0"
          :download-count="pack.downloadCount || 0"
          :author="pack.author || pack.namespace || ''"
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
          <div v-for="index in 8" :key="`skeleton-${index}`" class="mod-skeleton" aria-hidden="true">
            <div class="mod-skeleton-head">
              <span class="mod-skeleton-line is-title"></span>
              <span class="mod-skeleton-line"></span>
              <span class="mod-skeleton-line is-short"></span>
            </div>
            <div class="mod-skeleton-media"></div>
          </div>
        </template>
      </div>

      <!-- ~ 列表状态 -->
      <!-- * 说明：首屏失败、空列表、加载更多与到底提示，底部哨兵触发下一页 -->
      <!-- * 交互：首屏失败时可重试 -->
      <!-- * 依赖：依赖 packageLoadError、isLoadingPackages、hasLoadedAllPackages -->
      <!-- * 边界：已有数据时加载失败只在底部提示，不清空现有列表 -->
      <div class="mod-list-status">
        <div v-if="packageLoadError && packageList.length === 0" class="mod-list-error" role="alert">
          <AlertCircle :size="16" :stroke-width="1.8" class="shrink-0" />
          <span class="mod-list-error-text">{{ packageLoadError }}</span>
          <n-button size="small" type="error" secondary @click="() => refreshPackages()">重试</n-button>
        </div>
        <div v-else-if="!isLoadingPackages && packageList.length === 0" class="mod-list-empty">
          {{ isManualSource ? '暂时不支持手动导入' : (searchKeyword.trim() ? '没有找到匹配的模组。' : '暂无可用模组。') }}
        </div>
        <p v-else class="mod-list-tip" aria-live="polite">
          <span v-if="isLoadingPackages">正在加载模组…</span>
          <span v-else-if="packageLoadError">{{ packageLoadError }}</span>
          <span v-else-if="hasLoadedAllPackages">已经到底了。</span>
          <span v-else>继续向下滚动加载更多。</span>
        </p>
        <div ref="loadMoreTarget" class="mod-load-sentinel"></div>
      </div>
    </div>

    <!-- ~ 游戏路径遮罩 -->
    <!-- * 说明：游戏路径未就绪时覆盖列表，阻止模组操作 -->
    <!-- * 交互：路径无效时提供前往游戏设置入口 -->
    <!-- * 依赖：依赖 services/dysonSphere/game 的游戏目录检测 -->
    <!-- * 边界：检测失败按不可用处理 -->
    <div v-if="isCheckingGamePath || !canOpenModPage" class="mod-gate">
      <div class="mod-gate-content">
        <h1 class="mod-gate-title">{{ isCheckingGamePath ? '正在检测游戏路径' : '无法开启模组' }}</h1>
        <p class="mod-gate-desc">
          {{ isCheckingGamePath ? '正在确认 DSPGAME.exe 是否存在。' : '未检测到 DSPGAME.exe，请先在游戏设置中配置《戴森球计划》游戏路径。' }}
        </p>
        <n-button v-if="!isCheckingGamePath" type="primary" @click="router.push('/dysonSphere/settings')">
          前往游戏设置
        </n-button>
      </div>
    </div>

    <!-- ~ 模组详情弹窗 -->
    <!-- * 说明：展示选中模组的版本、作者、大小与依赖组件 -->
    <!-- * 交互：安装由后端自动处理依赖下载、解压与本地记录 -->
    <!-- * 依赖：依赖 selectedPack 与安装状态映射 -->
    <!-- * 边界：未安装 BepInEx 时禁用安装 -->
    <ModPackDialog
      v-model:show="dialogVisible"
      :pack="selectedPack"
      :installed="selectedPack ? installedPackageKeys[selectedPack.key] || false : false"
      :installing="selectedPack ? installingPackageKeys[selectedPack.key] || false : false"
      :can-install="canInstallPackages"
      install-disabled-text="需要优先安装 BepInEx 后才能安装模组。"
      @install="handleInstall"
      @uninstall="handleUninstall"
    />
  </section>
</template>

<style scoped lang="scss">
// 模组网格需要更宽的版心（足够容纳 7 列），其余边距沿用 --ds-page-* Token
.mod-page {
  position: relative;
  width: 100%;
  max-width: 1680px;
  min-height: 100%;
  margin-inline: auto;
  padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom);
  --mod-control-height: 34px;
}

.mod-page-body.is-gated {
  filter: blur(4px);
  pointer-events: none;
  user-select: none;
}

// =============== 工具栏 ===============
.mod-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.mod-bepinex {
  display: flex;
  box-sizing: border-box;
  height: var(--mod-control-height);
  min-width: 0;
  flex: 1 1 320px;
  align-items: center;
  gap: 8px;
  padding: 0 6px 0 12px;
  border: 1px solid;
  border-radius: 4px;
  font-size: 13px;

  &.is-ready {
    border-color: var(--ds-success-border);
    background: var(--ds-success-bg);
    color: var(--ds-success);
  }

  &.is-missing {
    border-color: var(--ds-warning-border);
    background: var(--ds-warning-bg);
    color: var(--ds-warning);
  }
}

.mod-bepinex-icon {
  flex-shrink: 0;

  &.is-spinning {
    animation: mod-spin 1s linear infinite;
  }
}

.mod-bepinex-title {
  flex-shrink: 0;
  font-weight: 700;
}

.mod-bepinex-desc {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  color: var(--ds-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mod-bepinex-action {
  flex-shrink: 0;
}

.mod-toolbar-actions {
  display: flex;
  flex: 0 1 auto;
  align-items: center;
  gap: 8px;
}

.mod-search {
  width: 240px;
}

.mod-toolbar-actions :deep(.n-input),
.mod-toolbar-actions :deep(.n-button) {
  --n-height: var(--mod-control-height) !important;
  height: var(--mod-control-height);
}

// =============== 模组网格 ===============
// 说明：列数按内容区宽度在 4～7 之间取值，卡片目标最小宽度约 180px（含 14px 间距）。
// 边界：窄于 4 列所需宽度时仍保持 4 列，卡片等比缩小；宽于 7 列所需宽度时卡片等比放大。
.mod-page-body {
  container: mod-market / inline-size;
}

.mod-grid {
  --mod-grid-columns: 4;
  display: grid;
  grid-template-columns: repeat(var(--mod-grid-columns), minmax(0, 1fr));
  gap: 14px;
}

@container mod-market (min-width: 956px) {
  .mod-grid {
    --mod-grid-columns: 5;
  }
}

@container mod-market (min-width: 1150px) {
  .mod-grid {
    --mod-grid-columns: 6;
  }
}

@container mod-market (min-width: 1344px) {
  .mod-grid {
    --mod-grid-columns: 7;
  }
}

.mod-skeleton {
  display: flex;
  aspect-ratio: 1;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  animation: mod-pulse 1.4s ease-in-out infinite;
}

.mod-skeleton-head {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
}

.mod-skeleton-line {
  width: 100%;
  height: 10px;
  border-radius: 2px;
  background: var(--ds-hover-bg);

  &.is-title {
    width: 60%;
    height: 14px;
  }

  &.is-short {
    width: 40%;
  }
}

.mod-skeleton-media {
  flex: 1;
  border-top: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-shell);
}

// =============== 列表状态 ===============
.mod-list-status {
  padding-top: 20px;
}

.mod-list-error {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border: 1px solid var(--ds-error-border);
  border-radius: 4px;
  background: var(--ds-error-bg);
  color: var(--ds-error);
  font-size: 13px;
}

.mod-list-error-text {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mod-list-empty {
  padding: 32px 16px;
  border: 1px solid var(--ds-border-subtle);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-muted);
  font-size: 13px;
  text-align: center;
}

.mod-list-tip {
  margin: 0;
  color: var(--ds-text-muted);
  font-size: 12px;
  text-align: center;
}

.mod-load-sentinel {
  height: 32px;
}

// =============== 游戏路径遮罩 ===============
.mod-gate {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: grid;
  place-items: center;
  background: var(--ds-overlay-bg);
}

.mod-gate-content {
  max-width: 420px;
  padding: 0 24px;
  text-align: center;
}

.mod-gate-title {
  margin: 0;
  color: var(--ds-text-primary);
  font-size: 18px;
  font-weight: 700;
}

.mod-gate-desc {
  margin: 10px 0 20px;
  color: var(--ds-text-muted);
  font-size: 13px;
  line-height: 1.7;
}

@keyframes mod-spin {
  to {
    transform: rotate(360deg);
  }
}

@keyframes mod-pulse {
  50% {
    opacity: 0.55;
  }
}

// 内容区较窄时搜索框随工具栏换行后占满整行
@media (max-width: 900px) {
  .mod-toolbar-actions {
    flex: 1 1 100%;
  }

  .mod-search {
    width: auto;
    flex: 1 1 auto;
  }
}
</style>

<style lang="scss">
// 冲突弹窗由 useDialog 渲染到页面外部，scoped 样式无法命中
.mod-conflict-label {
  color: var(--ds-text-primary);
}

.mod-conflict-help,
.mod-conflict-path {
  color: var(--ds-text-muted);
}
</style>
