<script setup lang="ts">
import { computed, ref } from 'vue'
import { Archive, ArrowLeft, Check, ChevronDown, ChevronRight, CircleAlert, CloudUpload, Code2, Download, FolderOpen, GitBranch, HardDrive, Info, Layers3, LogIn, PackageOpen, Plus, RefreshCw, Search, ShieldCheck, Sparkles, Upload, X } from 'lucide-vue-next'
import { formatBytes, useMarketPrototype } from './model'

const p = useMarketPrototype()
const inputRef = ref<HTMLInputElement>()
const folderRef = ref<HTMLInputElement>()
const expanded = ref(true)
const filteredFiles = computed(() => p.visibleFiles.value)
const currentTitle = computed(() => p.view.value === 'publish' ? '发布到盒子市场' : p.view.value === 'mine' ? '我的发布' : '模组市场')

function chooseFile(): void { inputRef.value?.click() }
function chooseFolder(): void { folderRef.value?.click() }
function openDetail(item: typeof p.catalog.value[number]): void { p.detail.value = item; p.installState.value = 'idle' }
function submitLogin(): void { p.continueAfterLogin() }
</script>

<template>
  <div class="app-shell">
    <aside class="rail">
      <div class="brand-mark">DS</div>
      <button class="rail-button is-active" title="模组市场" @click="p.view = 'market'"><Layers3 :size="18" /></button>
      <button class="rail-button" title="我的发布" @click="p.view = 'mine'"><PackageOpen :size="18" /></button>
      <div class="rail-spacer"></div>
      <button class="rail-button" title="设置"><HardDrive :size="18" /></button>
    </aside>
    <main class="main-shell">
      <header class="topbar">
        <div><span class="eyebrow">DYSON SPHERE BOX</span><h1>{{ currentTitle }}</h1></div>
        <div class="top-actions"><span class="source-chip"><span class="source-dot"></span>{{ p.source === 'box' ? '盒子市场' : 'Thunderstore' }}</span><button class="quiet-button" @click="p.view = 'publish'"><Upload :size="15" />发布模组</button></div>
      </header>

      <section v-if="p.view === 'market'" class="page-content">
        <div class="market-toolbar">
          <div class="search-box"><Search :size="16" /><input v-model="p.search" placeholder="搜索名称、作者或说明" /></div>
          <div class="segmented"><button :class="{selected: p.source === 'box'}" @click="p.source = 'box'">盒子市场</button><button :class="{selected: p.source === 'thunderstore'}" @click="p.source = 'thunderstore'">Thunderstore</button></div>
          <div class="segmented compact"><button :class="{selected: p.kind === 'all'}" @click="p.kind = 'all'">全部</button><button :class="{selected: p.kind === 'mod'}" @click="p.kind = 'mod'">模组</button><button :class="{selected: p.kind === 'pack'}" @click="p.kind = 'pack'">整合包</button></div>
          <button class="icon-button" title="刷新"><RefreshCw :size="16" /></button>
        </div>
        <div v-if="p.serverOffline && p.source === 'box'" class="notice warning"><CircleAlert :size="16" /><span>盒子服务暂时不可用。Thunderstore 仍可独立浏览和安装。</span><button @click="p.source = 'thunderstore'">切换到 Thunderstore</button></div>
        <div class="catalog-grid">
          <article v-for="item in p.catalog" :key="item.id" class="package-card" @click="openDetail(item)">
            <div class="package-art" :class="item.kind"><img v-if="item.icon" :src="item.icon" alt="" /><span v-else>{{ item.kind === 'pack' ? 'PACK' : 'MOD' }}</span><span class="package-kind">{{ item.kind === 'pack' ? '整合包' : '模组' }}</span></div>
            <div class="package-body"><div class="package-title-row"><h2>{{ item.name }}</h2><span class="version">v{{ item.version }}</span></div><p>{{ item.description }}</p><div class="package-meta"><span>{{ item.author }}</span><span>{{ item.source === 'box' ? '盒子' : 'TS' }}</span></div></div>
            <button class="card-install" @click.stop="openDetail(item)"><Download :size="15" />查看详情</button>
          </article>
        </div>
      </section>

      <section v-else-if="p.view === 'publish'" class="page-content publish-page">
        <div class="back-row"><button class="text-button" @click="p.view = 'market'"><ArrowLeft :size="15" />返回市场</button><span class="draft-label"><span class="draft-dot"></span>{{ p.state === 'editing' ? '草稿自动保存中' : p.state === 'uploading' ? '正在上传' : p.state === 'review' ? '等待审核' : p.state === 'interrupted' ? '上传已暂停' : p.state === 'published' ? '已发布' : '编辑草稿' }}</span></div>
        <div class="publish-layout">
          <div class="publish-main">
            <div class="section-heading"><div><span class="eyebrow">STEP 1 · CONTENT</span><h2>添加要发布的内容</h2><p>可以添加 DLL、压缩包，或直接选择已有的 Mod 文件夹。目录结构会被保留。</p></div><Sparkles :size="22" class="heading-icon" /></div>
            <div class="drop-zone" @click="chooseFile"><input ref="inputRef" type="file" multiple hidden @change="p.addLocalFiles(($event.target as HTMLInputElement).files)" /><div class="drop-icon"><Plus :size="21" /></div><strong>添加模组文件或压缩包</strong><span>支持 DLL、ZIP、7Z 等格式，可一次选择多个文件</span><button class="outline-button" @click.stop="chooseFile"><Code2 :size="15" />选择文件</button></div>
            <div class="drop-zone folder-zone" @click="chooseFolder"><input ref="folderRef" type="file" webkitdirectory directory multiple hidden @change="p.addLocalFiles(($event.target as HTMLInputElement).files)" /><div class="drop-icon folder"><FolderOpen :size="20" /></div><strong>添加已有的 Mod 文件夹</strong><span>递归收集文件，自动保留相对路径和依赖关系</span><button class="outline-button" @click.stop="chooseFolder"><FolderOpen :size="15" />选择文件夹</button></div>
            <div class="scenario-strip"><span>快速查看输入识别：</span><button :class="{active: p.scenario === 'dll'}" @click="p.setScenario('dll')">单个 DLL</button><button :class="{active: p.scenario === 'zip'}" @click="p.setScenario('zip')">标准 ZIP</button><button :class="{active: p.scenario === 'folder'}" @click="p.setScenario('folder')">Mod 文件夹</button><button :class="{active: p.scenario === 'dependency'}" @click="p.setScenario('dependency')">未知依赖</button><button :class="{active: p.scenario === 'conflict'}" @click="p.setScenario('conflict')">路径冲突</button></div>

            <div class="intake-panel">
              <div class="intake-summary"><div><span class="eyebrow">识别结果</span><h3>{{ p.included.length }} 个待发布文件 <span>· {{ formatBytes(p.totalBytes) }}</span></h3></div><div class="summary-stats"><span><strong>{{ p.references.length }}</strong> 个依赖引用</span><span><strong>{{ p.excluded.length }}</strong> 个已排除</span></div></div>
              <div class="file-tools"><div class="file-search"><Search :size="14" /><input v-model="p.fileSearch" placeholder="筛选相对路径" /></div><button class="text-button" @click="p.showExcluded = !p.showExcluded">{{ p.showExcluded ? '隐藏已排除' : '查看已排除' }} <ChevronDown :size="14" /></button></div>
              <div class="tree-list"><div v-for="file in filteredFiles" :key="file.path" class="tree-row" :class="file.category"><ChevronRight v-if="file.path.includes('/')" :size="14" class="tree-chevron" /><span v-else class="tree-placeholder"></span><span class="file-icon">{{ file.path.endsWith('.dll') ? 'DLL' : file.path.endsWith('.json') ? '{}' : file.category === 'reference' ? 'REF' : 'FILE' }}</span><span class="file-path">{{ file.path }}</span><span v-if="file.category === 'excluded'" class="file-reason">{{ file.reason }}</span><span v-else-if="file.category === 'reference'" class="file-reason reference">{{ file.reason }}</span><span v-else class="file-size">{{ file.bytes ? formatBytes(file.bytes) : '自动处理' }}</span></div></div>
              <div v-if="!filteredFiles.length" class="empty-intake"><Upload :size="18" /><span>还没有文件。添加 DLL、压缩包或 Mod 文件夹开始。</span></div>
            </div>

            <div class="section-heading metadata-heading"><div><span class="eyebrow">STEP 2 · DETAILS</span><h2>填写基本信息</h2><p>发布前只需要补齐这些信息，manifest、README 和默认图标会自动生成。</p></div></div>
            <div class="metadata-form"><label>名称<input v-model="p.name" placeholder="例如：星际生产面板" /></label><label>版本<input v-model="p.version" placeholder="1.0.0" /></label><label class="full">说明<textarea v-model="p.description" rows="3" placeholder="告诉玩家这个模组解决什么问题。" /></label></div>
            <div class="dependency-inline"><div><span class="field-label">依赖识别</span><strong>{{ p.dependencyStatus === 'unknown' ? '需要补充依赖' : p.dependencyStatus === 'known' ? '已从清单识别' : '无依赖' }}</strong><p>{{ p.dependencyStatus === 'unknown' ? '裸 DLL 通常无法可靠推断依赖，不会默认当作“无依赖”。' : '识别结果会在后台校验，发布时保留精确版本。' }}</p></div><div v-if="p.dependencyStatus === 'unknown'" class="dependency-action"><input v-model="p.dependency" placeholder="可选：Owner-Name-Version" /><button class="outline-button" @click="p.dependencyStatus = p.dependency.trim() ? 'known' : 'unknown'">补充</button></div><span v-else class="check-mark"><Check :size="15" /></span></div>
            <div v-if="p.issues.length" class="inline-issues"><div v-for="issue in p.issues" :key="issue"><CircleAlert :size="15" />{{ issue }}<button v-if="issue.includes('路径冲突')" @click="p.resolvedConflict = true">保留当前文件</button></div></div>
            <div class="publish-footer"><div><ShieldCheck :size="16" /><span>上传前会自动排除凭证、日志、绝对路径和未知个人配置。</span></div><button class="primary-button" :disabled="p.state === 'uploading'" @click="p.publish"><CloudUpload :size="16" />{{ p.state === 'uploading' ? `上传中 ${p.progress}%` : p.state === 'review' ? '已提交审核' : '发布到盒子市场' }}</button></div>
            <div v-if="p.state === 'uploading' || p.state === 'interrupted' || p.state === 'review'" class="upload-status"><div class="status-line"><span>{{ p.state === 'interrupted' ? '网络中断，草稿和已上传内容已保留' : p.state === 'review' ? '文件已上传，正在进行自动检查' : '正在上传文件' }}</span><strong>{{ p.state === 'review' ? '100%' : `${p.progress}%` }}</strong></div><div class="progress-track"><span :style="{width: `${p.state === 'review' ? 100 : p.progress}%`}"></span></div><button v-if="p.state === 'interrupted'" class="text-button" @click="p.startUpload">继续上传</button></div>
          </div>
          <aside class="publish-aside"><div class="aside-block"><span class="eyebrow">发布前检查</span><div class="check-row"><Check :size="15" />路径结构已保留</div><div class="check-row"><Check :size="15" />个人内容默认排除</div><div class="check-row" :class="{warn: p.dependencyStatus === 'unknown'}"><CircleAlert v-if="p.dependencyStatus === 'unknown'" :size="15" /><Check v-else :size="15" />依赖关系{{ p.dependencyStatus === 'unknown' ? '待补充' : '已识别' }}</div></div><div class="aside-block subdued"><span class="eyebrow">格式兼容</span><p>盒子市场会生成兼容 Thunderstore 的标准 manifest。外部依赖保留为精确引用，不会重新上传第三方 DLL。</p><a href="#" @click.prevent>查看发布规则 <ChevronRight :size="14" /></a></div><div class="aside-block subdued"><span class="eyebrow">需要帮助？</span><p>大多数发布只需要添加文件、填写信息、点击发布。只有路径冲突或权利问题会阻止提交。</p></div></aside>
        </div>
      </section>

      <section v-else class="page-content empty-page"><div class="empty-icon"><PackageOpen :size="24" /></div><h2>还没有发布记录</h2><p>发布你的第一个模组或整合包，状态会显示在这里。</p><button class="primary-button" @click="p.view = 'publish'"><Upload :size="16" />开始发布</button></section>
    </main>

    <div v-if="p.detail" class="modal-backdrop" @click.self="p.detail = null"><section class="detail-modal"><button class="modal-close" title="关闭" @click="p.detail = null"><X :size="17" /></button><div class="detail-art" :class="p.detail.kind"><img v-if="p.detail.icon" :src="p.detail.icon" alt="" /><span v-else>{{ p.detail.kind === 'pack' ? 'PACK' : 'MOD' }}</span></div><div class="detail-content"><span class="eyebrow">{{ p.detail.source === 'box' ? '盒子市场' : 'THUNDERSTORE' }} · {{ p.detail.kind === 'pack' ? '整合包' : '模组' }}</span><h2>{{ p.detail.name }}</h2><p>{{ p.detail.description }}</p><div class="detail-meta"><span>作者 {{ p.detail.author }}</span><span>版本 v{{ p.detail.version }}</span></div><div v-if="p.detail.kind === 'pack'" class="profile-choice"><label><input v-model="p.isolatedProfiles" type="checkbox" /> 我已启用独立环境 profile</label><span v-if="!p.isolatedProfiles"><Info :size="14" /> 未启用前不会修改当前环境</span></div><div v-if="p.installState === 'blocked'" class="inline-issues"><div><CircleAlert :size="15" />整合包需要独立环境 profile，当前不会合并到现有环境。<button @click="p.isolatedProfiles = true">启用 profile</button></div></div><div v-if="p.installState === 'failed'" class="inline-issues"><div><CircleAlert :size="15" />下载或校验失败，原有环境未修改。<button @click="p.installState = 'idle'">重试</button></div></div><div v-if="p.installState === 'installed'" class="installed-banner"><Check :size="15" />已安装到{{ p.detail.kind === 'pack' ? '独立 profile' : '当前环境' }}</div><div class="modal-actions"><button class="quiet-button" @click="p.detail = null">稍后</button><button v-if="p.installState === 'installing'" class="primary-button" disabled><RefreshCw :size="16" class="spin" />正在安装</button><button v-else class="primary-button" @click="p.install">{{ p.installState === 'installed' ? '重新安装' : '一键安装' }} <Download :size="15" /></button></div></div></section></div>
    <div v-if="p.showLogin" class="modal-backdrop"><section class="login-modal"><div class="detail-art small"><LogIn :size="24" /></div><h2>登录后继续发布</h2><p>文件和表单已保留。登录只用于确认发布者身份，不影响匿名浏览和安装。</p><input v-model="p.loginName" autofocus placeholder="邮箱或用户名" @keyup.enter="submitLogin" /><div class="modal-actions"><button class="quiet-button" @click="p.showLogin = false">稍后</button><button class="primary-button" :disabled="!p.loginName.trim()" @click="submitLogin">登录并继续 <ArrowLeft :size="15" class="rotate-180" /></button></div></section></div>
  </div>
</template>
