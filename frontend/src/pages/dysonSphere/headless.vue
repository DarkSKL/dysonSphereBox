<script setup lang="ts">
import {Activity, AlertTriangle, Check, ChevronRight, Download, LoaderCircle, Play, RefreshCw, Save, Search, Server, Settings2, Square, Terminal, Users, Wrench} from '@lucide/vue'
import {computed, h, nextTick, onDeactivated, ref, watch} from 'vue'
import {RouterLink} from 'vue-router'
import {useDialog, type AutoCompleteOption} from 'naive-ui'
import {useHeadlessConsole} from '../../composables/dysonSphere/useHeadlessConsole'
import NetworkTrendChart from '../../components/dysonSphere/network-trend-chart.vue'
import {formatMbps, mibpsToMbps} from '../../utils/format'

definePage({meta: {layout: 'main', title: '开服', keepAlive: true}})

const {
  status, logSnapshot, loading, busyAction, hostPortDraft, archiveDraft, archiveSuggestions, archiveLoading, archiveError,
  logSource, logLevel, logSearch, autoScroll, errorText, logError, settingsDirty, filteredLogLines,
  refreshStatus, refreshLog, prepare, start, stop, saveSettings, loadArchiveOptions, exportLog,
} = useHeadlessConsole()
const dialog = useDialog()
const terminalElement = ref<HTMLElement | null>(null)
const settingsOpen = ref(false)

// 说明：缓存页面离开时收起配置面板，保留未保存的配置草稿。
onDeactivated(() => { settingsOpen.value = false })
// 说明：每次展开配置面板时重新扫描存档目录，保证联想列表与磁盘一致。
watch(settingsOpen, (open) => { if (open) void loadArchiveOptions() })

// =============== 存档联想 ===============
// 说明：把后端存档候选映射为自动补全选项，右侧附带最近修改时间辅助辨认。
const formatArchiveTime = (value: string): string => {
  const time = new Date(value)
  return Number.isNaN(time.getTime()) ? '' : time.toLocaleString('zh-CN', {hour12: false, month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'})
}
const archiveTimeText = computed(() => new Map(archiveSuggestions.value.map((item) => [item.name, formatArchiveTime(item.lastModifiedAt)])))
const archiveAutoOptions = computed<AutoCompleteOption[]>(() => archiveSuggestions.value.map((item) => ({label: item.name, value: item.name})))
const renderArchiveLabel = (option: AutoCompleteOption) => h('span', {class: 'archive-option'}, [
  h('span', {class: 'archive-option-name'}, String(option.label)),
  h('span', {class: 'archive-option-time'}, archiveTimeText.value.get(String(option.value)) ?? ''),
])

// =============== 终端与停止确认 ===============
// 说明：日志自动滚动只影响视图；停止会强制终止进程，须明确说明存档风险。
watch([filteredLogLines, autoScroll], async () => {
  if (!autoScroll.value) return
  await nextTick()
  terminalElement.value?.scrollTo({top: terminalElement.value.scrollHeight})
})
const confirmStop = (): void => {
  dialog.warning({
    title: '停止服务器？',
    content: '将强制终止服务器进程并断开所有玩家连接。尚未保存的游戏进度可能丢失，请先确认存档已保存。',
    positiveText: '停止服务器',
    negativeText: '取消',
    onPositiveClick: () => { void stop() },
  })
}

// =============== 展示格式 ===============
// 说明：把进程秒数、网络速率和样本转换为工作台可读格式。
const uptimeText = computed(() => {
  const seconds = status.value?.uptimeSeconds ?? 0
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainder = seconds % 60
  return [hours, minutes, remainder].map((part) => String(part).padStart(2, '0')).join(':')
})
const formatSpeed = (value = 0): string => formatMbps(mibpsToMbps(value))
const dependencyStatus = (installed: boolean): string => installed ? '完整' : '缺失'
</script>

<template>
  <main class="headless-page" aria-labelledby="headless-title">
    <!-- ~ 工作台页头 -->
    <!-- * 说明：展示服务器状态、进程元信息和主要启停操作 -->
    <!-- * 交互：四个图标入口支持刷新、配置面板、环境准备和启停确认 -->
    <!-- * 依赖：真实 HeadlessService 状态快照与操作结果 -->
    <!-- * 边界：未配置游戏路径或环境未就绪时展示真实阻塞原因 -->
    <header class="console-header">
      <div class="title-row">
        <div class="title-lockup"><Server :size="23" /><h1 id="headless-title">服务器控制台</h1>
          <span class="state-label" :class="status?.processRunning ? 'state-running' : status?.ready ? 'state-ready' : 'state-stopped'">
            <i />{{ status?.processRunning ? '运行中' : status?.ready ? '可启动' : '未就绪' }}
          </span>
        </div>
        <div class="actions" role="group" aria-label="服务器操作" @keydown.esc="settingsOpen = false">
          <n-tooltip><template #trigger><span class="action-trigger"><n-button class="icon-action" quaternary :loading="loading" aria-label="刷新状态" @click="refreshStatus(); refreshLog()"><template #icon><RefreshCw :size="18" /></template></n-button></span></template>刷新状态</n-tooltip>
          <!-- ~ 服务器配置面板 -->
          <!-- * 说明：将端口和存档配置收进页头操作入口 -->
          <!-- * 交互：点击展开；草稿修改后显式保存，关闭面板保留草稿 -->
          <!-- * 依赖：HostPort、ServerArchive 与 settingsDirty -->
          <!-- * 边界：运行中只读，未配置游戏目录时禁止修改 -->
          <n-popover v-model:show="settingsOpen" trigger="click" placement="bottom-end" :show-arrow="false" :style="{width: '360px', maxWidth: 'calc(100vw - 32px)'}">
            <template #trigger>
              <n-tooltip><template #trigger><span class="action-trigger"><n-button class="icon-action" :class="{'has-pending-settings': settingsDirty}" :quaternary="!settingsOpen" :secondary="settingsOpen" aria-label="服务器配置" :aria-expanded="settingsOpen" aria-controls="server-config-panel"><template #icon><Settings2 :size="18" /></template></n-button></span></template>{{ settingsDirty ? '服务器配置 · 有未保存的修改' : '服务器配置' }}</n-tooltip>
            </template>
            <section id="server-config-panel" class="config-panel" aria-labelledby="config-title" @keydown.esc="settingsOpen = false">
              <div class="section-heading">
                <div class="section-title"><h2 id="config-title">服务器配置</h2></div>
                <span class="section-hint">{{ status?.processRunning ? '停止后可修改' : '下次启动时生效' }}</span>
              </div>
              <div class="config-fields">
                <label class="field"><span>主机端口</span><n-input v-model:value="hostPortDraft" class="type-mono" type="text" :input-props="{inputmode:'numeric'}" placeholder="8469" :disabled="!status?.gameDir || status?.processRunning || !!busyAction"><template #suffix><span class="field-suffix">TCP / UDP</span></template></n-input></label>
                <!-- * 交互：聚焦即列出存档目录中的 .dsv，输入时按名称包含匹配；留空沿用最近存档 -->
                <!-- * 边界：菜单渲染在字段内，外层不能用 label，否则点击选项会转发焦点到输入框导致菜单重新展开 -->
                <div class="field"><span id="archive-field-label">启动存档</span>
                  <n-auto-complete v-model:value="archiveDraft" class="archive-control type-mono" :options="archiveAutoOptions" :render-label="renderArchiveLabel" :get-show="() => true" :input-props="{'aria-labelledby': 'archive-field-label'}" show-empty blur-after-select clearable :to="false" :loading="archiveLoading" placeholder="输入匹配存档，留空加载最近存档" :disabled="!status?.gameDir || status?.processRunning || !!busyAction">
                    <template #empty><span class="archive-empty">{{ archiveLoading ? '正在读取存档目录' : archiveError || '没有匹配的存档' }}</span></template>
                  </n-auto-complete>
                </div>
              </div>
              <div class="config-panel-footer">
                <span class="section-hint">{{ settingsDirty ? '有未保存的配置' : '无待保存修改' }}</span>
                <n-button size="small" :type="settingsDirty ? 'primary' : 'default'" :loading="busyAction === 'save'" :disabled="!settingsDirty || !!busyAction || status?.processRunning" @click="saveSettings"><template #icon><Save :size="15" /></template>保存配置</n-button>
              </div>
            </section>
          </n-popover>
          <n-tooltip><template #trigger><span class="action-trigger"><n-button class="icon-action" quaternary :loading="busyAction === 'prepare'" :disabled="!status?.gameDir || status?.processRunning || !!busyAction" aria-label="准备环境" @click="prepare"><template #icon><Wrench :size="18" /></template></n-button></span></template>{{ status?.processRunning ? '准备环境 · 请先停止服务器' : '准备环境' }}</n-tooltip>
          <n-tooltip><template #trigger><span class="action-trigger"><n-button v-if="!status?.processRunning" class="icon-action" type="primary" :loading="busyAction === 'start'" :disabled="!status?.ready || settingsDirty || !!busyAction" aria-label="启动服务器" @click="start"><template #icon><Play :size="18" /></template></n-button><n-button v-else class="icon-action" type="error" secondary :loading="busyAction === 'stop'" :disabled="!!busyAction" aria-label="停止服务器" @click="confirmStop"><template #icon><Square :size="18" /></template></n-button></span></template>{{ status?.processRunning ? '停止服务器' : settingsDirty ? '启动服务器 · 请先保存配置' : '启动服务器' }}</n-tooltip>
        </div>
      </div>
      <div class="runtime-meta">
        <span class="server-name">Nebula · 无头服务器</span>
        <span><b>端口</b><code>{{ status?.hostPort ?? '--' }}</code></span>
        <span v-if="status?.processRunning"><b>PID</b><code>{{ status.processId || '--' }}</code></span>
        <span v-if="status?.processRunning"><b>运行时长</b><code>{{ uptimeText }}</code></span>
        <span v-if="status?.portListening" class="port-live"><i />端口监听中</span>
      </div>
    </header>

    <div v-if="errorText" class="error-strip" role="alert"><AlertTriangle :size="16" />{{ errorText }}</div>

    <section v-if="status?.issues?.length" class="issues-section" aria-label="环境诊断">
      <div class="section-heading"><div class="section-title"><AlertTriangle :size="16" /><h2>启动检查</h2></div><RouterLink :to="status.gameDir ? '/dysonSphere/mod' : '/dysonSphere/settings'" class="issue-link">{{ status.gameDir ? '管理模组' : '配置游戏目录' }}<ChevronRight :size="13" /></RouterLink></div>
      <ul class="issue-list"><li v-for="issue in status.issues" :key="issue">{{ issue }}</li></ul>
    </section>

    <div class="workspace-grid">
      <!-- ~ 网络趋势 -->
      <!-- * 说明：展示本机网卡的真实上下行采样值 -->
      <!-- * 交互：随状态轮询滚动更新 -->
      <!-- * 依赖：network.samples 与当前速率 -->
      <!-- * 边界：指标属于整机网络流量，不代表服务器进程专属流量 -->
      <section class="network-section" aria-labelledby="network-title">
        <div class="section-heading"><div class="section-title"><Activity :size="16" /><h2 id="network-title">网络趋势</h2></div><span class="section-hint">本机网卡 · {{ status?.network.sampleWindowSeconds ?? 0 }} 秒窗口</span></div>
        <div class="network-stats">
          <div><span class="stat-label"><i class="upload-dot" />上行</span><strong>{{ formatSpeed(status?.network.uploadMBps) }}</strong></div>
          <div><span class="stat-label"><i class="download-dot" />下行</span><strong>{{ formatSpeed(status?.network.downloadMBps) }}</strong></div>
          <div class="peak-stat"><span class="stat-label"><i class="upload-dot" />上行峰值</span><strong>{{ formatSpeed(status?.network.uploadPeakMBps) }}</strong></div>
          <div class="peak-stat"><span class="stat-label"><i class="download-dot" />下行峰值</span><strong>{{ formatSpeed(status?.network.downloadPeakMBps) }}</strong></div>
          <span v-if="!status?.network.valid" class="network-warning">{{ status?.network.error || '等待有效采样' }}</span>
        </div>
        <div class="chart-wrap">
          <NetworkTrendChart :samples="status?.network.samples ?? []" />
        </div>
      </section>

      <!-- ~ 依赖诊断 -->
      <!-- * 说明：展示启动所需 Mod 的本地文件完整性 -->
      <!-- * 交互：可从列表查看缺失文件提示与版本 -->
      <!-- * 依赖：V2 安装记录、旧版目录兼容扫描和磁盘实存校验 -->
      <!-- * 边界：只报告状态，不自动下载或安装模组 -->
      <section class="dependency-section" aria-labelledby="dependency-title">
        <div class="section-heading"><div class="section-title"><Settings2 :size="16" /><h2 id="dependency-title">开服依赖</h2></div><span class="section-hint">{{ status?.dependencies?.filter((item) => item.installed).length ?? 0 }} / {{ status?.dependencies?.length ?? 0 }} 已就绪</span></div>
        <div class="dependency-list">
          <div v-for="item in status?.dependencies ?? []" :key="item.id" class="dependency-row">
            <span class="dependency-indicator" :class="item.installed ? 'dependency-ok' : 'dependency-missing'"><Check v-if="item.installed" :size="13" /><AlertTriangle v-else :size="13" /></span>
            <div class="dependency-copy" :title="item.installed ? item.function : item.missingFiles?.join('、')"><strong>{{ item.name }}</strong><small>{{ item.installed ? (item.installedVersion || '本地文件完整') : (item.missingFiles?.join('、') || '缺少受管文件') }}</small></div>
            <span class="dependency-result" :class="item.installed ? 'text-success' : 'text-warning'">{{ dependencyStatus(item.installed) }}</span>
          </div>
          <div v-if="!status?.dependencies?.length" class="section-empty"><LoaderCircle v-if="loading" :size="16" class="spin" /><span v-else>等待环境检测</span></div>
        </div>
        <p class="dependency-footnote">Nebula 核心依赖缺失时无法启动服务器。</p>
      </section>

      <!-- ~ 日志控制台 -->
      <!-- * 说明：查看 BepInEx 或 Unity 本地日志尾部 -->
      <!-- * 交互：来源切换、等级筛选、文本搜索、自动滚动和完整日志导出 -->
      <!-- * 依赖：HeadlessService.ReadServerLog 与 ExportServerLog -->
      <!-- * 边界：日志最多展示最近 300 行；原始文本不解析为玩家或延迟数据 -->
      <section class="logs-section" aria-labelledby="logs-title">
        <div class="section-heading log-heading"><div class="section-title"><Terminal :size="16" /><h2 id="logs-title">服务器日志</h2></div><n-button quaternary size="small" title="导出完整日志" aria-label="导出完整日志" :disabled="!logSnapshot?.exists" @click="exportLog"><Download :size="15" /></n-button></div>
        <div class="log-tools">
            <n-select v-model:value="logSource" size="small" :options="[{label:'BepInEx',value:'bepinex'},{label:'Unity',value:'unity'}]" class="source-select" />
            <n-select v-model:value="logLevel" size="small" :options="[{label:'全部等级',value:'all'},{label:'错误',value:'error'},{label:'警告',value:'warning'},{label:'信息',value:'info'},{label:'调试',value:'debug'}]" class="level-select" />
            <n-input v-model:value="logSearch" size="small" placeholder="搜索日志" class="search-input"><template #prefix><Search :size="14" /></template></n-input>
        </div>
        <div ref="terminalElement" class="terminal" tabindex="0" aria-label="服务器日志内容">
          <div v-if="logError" class="log-error" role="alert">{{ logError }}</div>
          <div v-if="logSnapshot?.truncated" class="truncated-note">已截取日志尾部；导出可获取完整文件。</div>
          <div v-for="(line, index) in filteredLogLines" :key="`${index}-${line.content}`" class="log-line">
            <span class="log-level" :class="`level-${line.level}`">{{ line.level.toUpperCase() }}</span><span class="log-content">{{ line.content }}</span>
          </div>
            <div v-if="!filteredLogLines.length && !logError" class="log-empty">{{ logSnapshot?.exists ? '没有匹配的日志行' : '日志文件尚未生成' }}</div>
        </div>
        <div class="log-footer"><label class="auto-scroll"><input v-model="autoScroll" type="checkbox" />自动滚动</label><span>{{ logSnapshot?.updatedAt ? `日志更新于 ${new Date(logSnapshot.updatedAt).toLocaleTimeString('zh-CN', {hour12:false})}` : '等待日志文件' }}</span></div>
      </section>

      <!-- ~ 在线连接 -->
      <!-- * 说明：保留设计稿中的连接管理分区并显示当前能力状态 -->
      <!-- * 交互：后端接口接入前不提供踢人或清空连接等操作 -->
      <!-- * 依赖：当前版本没有可读取玩家、延迟或连接时长的 Nebula 管理接口 -->
      <!-- * 边界：不显示样例玩家，不把日志文本推断为在线状态 -->
      <section class="connections-section" aria-labelledby="connections-title">
        <div class="section-heading"><div class="section-title"><Users :size="16" /><h2 id="connections-title">在线连接</h2></div><span class="unavailable-label">未接入</span></div>
        <div class="connection-empty"><Users :size="22" /><strong>玩家连接数据不可用</strong><p>当前 Nebula 后端没有玩家列表、延迟和连接控制接口。</p></div>
      </section>
    </div>

    <footer class="console-footer">
      <span><i :class="{'is-live': status?.processRunning}" />{{ status?.processRunning ? '服务器进程运行中' : '服务器进程未运行' }}</span>
      <span v-if="status?.gameDir">游戏目录 <code>{{ status.gameDir }}</code></span>
      <span v-else>请先在游戏设置中配置有效的游戏目录</span>
    </footer>

  </main>
</template>

<style scoped lang="scss">
// 说明：版心与模组页一致（1680px + --ds-page-* 边距），大窗口下仍铺满主要工作区。
.headless-page {
  --ds-accent: var(--ds-brand);
  // 说明：图表与终端高度随视口高度伸缩，大屏展示更多日志与采样。
  --headless-chart-min-height: clamp(210px, 26vh, 360px);
  --headless-terminal-height: clamp(220px, 34vh, 560px);
  width: 100%;
  max-width: 1680px;
  min-height: 100%;
  margin-inline: auto;
  padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom);
  color: var(--ds-text-primary);
  container-type: inline-size;
}

.console-header { padding-bottom: 17px; border-bottom: 1px solid var(--ds-border-subtle); }
.title-row, .title-lockup, .actions, .runtime-meta, .section-heading, .section-title, .network-stats, .log-tools, .log-footer, .console-footer { display: flex; align-items: center; }
.console-footer i, .port-live i, .state-label i { width: 6px; height: 6px; border-radius: 50%; background: var(--ds-text-muted); }
.console-footer i.is-live, .port-live i { background: var(--ds-success); box-shadow: 0 0 0 3px var(--ds-success-bg); }
.title-row { gap: 14px; }
.title-lockup { gap: 10px; min-width: 0; }
.title-lockup > svg { color: var(--ds-accent); flex: none; }
h1 { margin: 0; font-size: 24px; font-weight: 650; line-height: 1.3; }
.state-label { display: inline-flex; align-items: center; gap: 7px; padding: 4px 8px; border: 1px solid transparent; border-radius: 4px; font-size: 11px; white-space: nowrap; }
.state-running, .state-ready { color: var(--ds-success); background: var(--ds-success-bg); border-color: var(--ds-success-border); }
.state-stopped { color: var(--ds-warning); background: var(--ds-warning-bg); border-color: var(--ds-warning-border); }
.state-running i { background: var(--ds-success); }
.state-ready i { background: var(--ds-success); }
.state-stopped i { background: var(--ds-warning); }
.actions { gap: 6px; margin-left: auto; flex: none; }
.action-trigger { display: inline-flex; }
.actions .icon-action { width: 36px; height: 36px; padding: 0; }
.has-pending-settings::after { position: absolute; top: 5px; right: 5px; width: 5px; height: 5px; border-radius: 50%; background: var(--ds-warning); content: ''; }
.actions :deep(.n-button) { border-radius: 4px; }
.runtime-meta { flex-wrap: wrap; gap: 8px 20px; margin-top: 10px; color: var(--ds-text-secondary); font-size: 12px; }
.runtime-meta span { display: inline-flex; align-items: center; gap: 7px; }
.runtime-meta b { color: var(--ds-text-muted); font-weight: 400; }
code { color: var(--ds-text-secondary); font-family: var(--ds-font-mono); font-size: 11px; font-variant-numeric: tabular-nums; }
.server-name { color: var(--ds-text-primary) !important; }
.port-live { color: var(--ds-success); }
.error-strip { display: flex; gap: 9px; align-items: center; margin-top: 12px; padding: 10px 12px; border: 1px solid var(--ds-error-border); background: var(--ds-error-bg); color: var(--ds-error); font-size: 12px; }
.issues-section, .workspace-grid { margin-top: 19px; }
.section-heading { justify-content: space-between; gap: 12px; min-height: 24px; }
.section-title { gap: 8px; min-width: 0; }
.section-title > svg { color: var(--ds-text-secondary); flex: none; }
h2 { margin: 0; font-size: 14px; line-height: 1.4; font-weight: 600; }
.section-hint { color: var(--ds-text-muted); font-size: 11px; }
.config-panel { padding: 4px; }
.config-fields { display: flex; flex-direction: column; gap: 14px; margin-top: 18px; }
.config-panel-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 18px; padding-top: 12px; border-top: 1px solid var(--ds-border-subtle); }
.field { display: flex; flex-direction: column; gap: 6px; color: var(--ds-text-secondary); font-size: 11px; }
.field :deep(.n-input), .archive-control :deep(.n-input) { --n-border-radius: 4px !important; }
.field-suffix { color: var(--ds-text-muted); font-family: var(--ds-font-mono); font-size: 10px; }
.archive-control { width: 100%; }
// 说明：菜单外层背景随内容伸缩，只限制内部滚动区高度，避免选项溢出背景。
.archive-control :deep(.n-base-select-menu .n-scrollbar), .archive-control :deep(.n-base-select-menu .n-virtual-list) { max-height: 240px; }
.archive-control :deep(.archive-option) { display: flex; align-items: center; justify-content: space-between; gap: 12px; width: 100%; min-width: 0; }
.archive-control :deep(.archive-option-name) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.archive-control :deep(.archive-option-time) { flex: none; color: var(--ds-text-muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.archive-empty { color: var(--ds-text-muted); font-size: 12px; }
.saved-state { display: inline-flex; align-items: center; gap: 5px; height: 34px; padding: 0 9px; color: var(--ds-success); font-size: 11px; white-space: nowrap; }
.issues-section { padding: 11px 13px; border: 1px solid var(--ds-warning-border); background: var(--ds-warning-bg); }
.issue-count { color: var(--ds-warning); font-size: 11px; }
.issue-list { display: flex; flex-wrap: wrap; gap: 7px 22px; margin: 9px 0 0; padding-left: 17px; color: var(--ds-text-secondary); font-size: 11px; }
.workspace-grid { display: grid; grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr); gap: 28px 32px; align-items: start; }
.network-section, .dependency-section, .logs-section, .connections-section { min-width: 0; }
// 说明：网络趋势与右侧开服依赖同行拉伸，图表吃满剩余高度以对齐两列底边。
.network-section { display: flex; flex-direction: column; align-self: stretch; }
.network-stats { gap: 19px; margin-top: 13px; }
.network-stats > div { display: flex; flex-direction: column; gap: 5px; }
.stat-label { display: inline-flex; align-items: center; gap: 6px; color: var(--ds-text-muted); font-size: 11px; }
.stat-label i { width: 6px; height: 6px; border-radius: 50%; }
.upload-dot { background: var(--ds-accent); }.download-dot { background: var(--ds-success); }
.network-stats strong { font-family: var(--ds-font-display); font-size: 16px; font-weight: 600; font-variant-numeric: tabular-nums; }
.network-stats .peak-stat strong { color: var(--ds-text-secondary); font-weight: 500; }
.network-stats .peak-stat:nth-child(3) { padding-left: 19px; border-left: 1px solid var(--ds-border-subtle); }
.network-warning { margin-left: auto; color: var(--ds-text-muted); font-size: 10px; }
.chart-wrap { position: relative; flex: 1; min-height: var(--headless-chart-min-height); margin-top: 14px; background: var(--ds-bg-panel); border-radius: 4px; }
.chart-wrap > .network-chart { position: absolute; inset: 0; }
.dependency-list { margin-top: 6px; }
.dependency-row { display: flex; align-items: center; gap: 9px; min-height: 43px; border-bottom: 1px solid var(--ds-border-subtle); }
.dependency-row:last-child { border-bottom: 0; }
.dependency-indicator { display: grid; place-items: center; width: 19px; height: 19px; flex: none; border-radius: 50%; }
.dependency-ok { color: var(--ds-success); background: var(--ds-success-bg); }.dependency-missing { color: var(--ds-warning); background: var(--ds-warning-bg); }
.dependency-copy { display: flex; flex: 1; flex-direction: column; gap: 3px; min-width: 0; }
.dependency-copy strong { overflow: hidden; color: var(--ds-text-primary); font-size: 11px; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
.dependency-copy small { overflow: hidden; color: var(--ds-text-muted); font-family: var(--ds-font-mono); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.dependency-result { font-size: 10px; }.text-success { color: var(--ds-success); }.text-warning { color: var(--ds-warning); }
.dependency-footnote { margin: 8px 0 0; color: var(--ds-text-muted); font-size: 10px; }
.section-empty { display: flex; align-items: center; gap: 8px; padding: 15px 0; color: var(--ds-text-muted); font-size: 11px; }
.logs-section { grid-column: 1; }
.log-heading { align-items: flex-start; }
.log-tools { gap: 8px; margin-top: 10px; }
.log-tools :deep(.n-select) { flex: none; }
.source-select { width: 104px; }.level-select { width: 104px; }.search-input { flex: 1; min-width: 110px; }
.terminal { height: var(--headless-terminal-height); overflow: auto; margin-top: 10px; padding: 10px 11px; border: 1px solid var(--ds-border-default); border-radius: 4px; background: var(--ds-bg-panel-elevated); font-family: var(--ds-font-mono); font-variant-ligatures: none; font-size: 11px; }
.log-error { color: var(--ds-error); overflow-wrap: anywhere; }
.log-line { display: grid; grid-template-columns: 52px minmax(0, 1fr); gap: 9px; min-height: 23px; align-items: start; line-height: 1.55; }
.log-level { font-size: 9px; }.level-error { color: var(--ds-error); }.level-warning { color: var(--ds-warning); }.level-info { color: var(--ds-success); }.level-debug { color: var(--ds-text-muted); }
.log-content { overflow-wrap: anywhere; color: var(--ds-text-secondary); white-space: pre-wrap; }
.truncated-note { margin-bottom: 7px; color: var(--ds-warning); font-family: inherit; }
.log-empty { display: grid; height: calc(var(--headless-terminal-height) - 50px); place-items: center; color: var(--ds-text-muted); font-family: var(--ds-font-ui); }
.log-footer { justify-content: space-between; gap: 12px; margin-top: 7px; color: var(--ds-text-muted); font-size: 10px; }
.auto-scroll { display: inline-flex; gap: 6px; align-items: center; cursor: pointer; }.auto-scroll input { accent-color: var(--ds-accent); }
// 说明：在线连接与日志同行拉伸，空态区域吃满剩余高度以对齐日志底边。
.connections-section { display: flex; flex-direction: column; align-self: stretch; grid-column: 2; grid-row: 2; }
.unavailable-label { padding: 3px 7px; border: 1px solid var(--ds-border-subtle); color: var(--ds-text-muted); font-size: 10px; }
.connection-empty { display: flex; flex: 1; min-height: 260px; flex-direction: column; align-items: center; justify-content: center; gap: 12px; padding: 16px; margin-top: 10px; color: var(--ds-text-muted); text-align: center; }
.connection-empty > svg { opacity: .7; }.connection-empty strong { color: var(--ds-text-secondary); font-size: 12px; font-weight: 500; }.connection-empty p { max-width: 245px; margin: 0; font-size: 10px; line-height: 1.6; }
.connection-note { display: flex; align-items: center; gap: 6px; margin-top: 8px; color: var(--ds-text-muted); font-size: 10px; }
.console-footer { justify-content: space-between; flex-wrap: wrap; gap: 7px 18px; margin-top: 24px; color: var(--ds-text-muted); font-size: 10px; }
.console-footer > span:first-child { display: inline-flex; align-items: center; gap: 7px; }
.console-footer code { max-width: min(56vw, 880px); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.issue-link { display: inline-flex; align-items: center; gap: 3px; color: var(--ds-warning); font-size: 12px; }
.issue-link:hover { text-decoration: underline; }
.config-panel .section-hint, .field, .dependency-copy strong, .runtime-meta, .connection-empty strong { font-size: 12px; }
.issue-list, .dependency-copy small, .dependency-result, .connection-empty p, .dependency-footnote, .log-footer, .console-footer { font-size: 11px; }
// 说明：宽版心下放宽两列间距，并让左列占比更大，给图表和日志更多横向空间。
@container (min-width: 1360px) {
  .workspace-grid { grid-template-columns: minmax(0, 2fr) minmax(360px, 1fr); gap: 32px 40px; }
}
@container (max-width: 620px) {
  .headless-page { padding-top: 15px; }
  .network-stats { flex-wrap: wrap; row-gap: 10px; }
  .title-row { align-items: flex-start; flex-direction: column; }.actions { margin-left: 0; }
  h1 { font-size: 20px; }
  .workspace-grid { grid-template-columns: minmax(0, 1fr); gap: 16px; }.logs-section, .connections-section { grid-column: auto; grid-row: auto; }
  .log-heading { flex-direction: column; }.log-tools { width: 100%; flex-wrap: wrap; }.search-input { flex: 1; width: auto; }
  .console-footer { align-items: flex-start; flex-direction: column; }
}
</style>
