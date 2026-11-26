<script setup lang="ts">
import {Bell, MessageSquarePlus, Star, X} from '@lucide/vue'
import {useMessage, type DropdownOption} from 'naive-ui'
import {computed, h, nextTick, ref, shallowRef, watch} from 'vue'

import EditorForm from '../../components/feedback/editor-form.vue'
import GalaxyMap from '../../components/feedback/galaxy-map.vue'
import MapStatus from '../../components/feedback/map-status.vue'
import PersonalList from '../../components/feedback/personal-list.vue'
import StarComposer, {type CardComposerMode} from '../../components/feedback/star-composer.vue'
import StarDetailCard from '../../components/feedback/star-detail-card.vue'
import StarDetail from '../../components/feedback/star-detail.vue'
import StarQuote from '../../components/feedback/star-quote.vue'
import VisibleStars from '../../components/feedback/visible-stars.vue'
import type {ProjectedStar} from '../../composables/feedback/galaxy-camera'
import {isInView, revalidateCandidates} from '../../composables/feedback/galaxy-camera'
import {galaxyStatus, isCoveredByAggregate, isPublicFeedback, nextSnapshotOrder} from '../../composables/feedback/galaxy-status'
import {useFeedbackGalaxy} from '../../composables/feedback/useFeedbackGalaxy'
import type {StarDetail as StarDetailData} from '../../types/feedback'

definePage({meta: {layout: 'main', keepAlive: true, title: '反馈星系'}})

// =============== 页面展示 ===============
// 说明：星图占满页面，按需显示预览、星点列表与详情。
const {
  foreground, map, preview, personal, editor, detail, panel, panelHeading, browse, points, visibleStars,
  confirmWithdraw, writing, actionError, status, currentCollection, closePanel,
  actionMenu, openActions, dismissActions, openPanel,
  changePersonalPage, openDetail, openMine, hover, changeStar, resetCamera, zoom, locate,
} = useFeedbackGalaxy(focusAfterPanel)
const {camera, arrivals, ready, error: mapError, loading: mapLoading, snapshotRegion, viewRegion, snapshotBounds, snapshotSyncedOrder, updateCamera, retry: retryRequest} = map
const {hover: hoveredStar, hovered, quote} = preview
const {detail: selectedStar, highlightedEntry, target, busy: detailBusy, unavailable, error: detailError} = detail
const {editor: openedEditor, draft, error: editorError, saving, length, limit, valid} = editor
const {mine, notices} = personal
const mapView = ref<InstanceType<typeof GalaxyMap>>()
const mapStatus = ref<InstanceType<typeof MapStatus>>()

// =============== 面板关闭后的焦点落点 ===============
// 说明：打开面板会卸载空态卡片里的「提意见」按钮，Escape 关闭后原节点已不存在。
// 使用：useFeedbackGalaxy 在原焦点节点失效时调用；当前仍有「提意见」入口就聚焦它，否则（例如已有星）回到星图。
// 边界：只在面板关闭后的下一帧执行，不抢走用户已经移到别处的焦点。
function focusAfterPanel(): void {
  void nextTick(() => {
    const active = document.activeElement
    if (active && active !== document.body && !active.closest('.galaxy-panel, .star-card-backdrop')) return
    if (mapStatus.value?.focusPublish()) return
    mapView.value?.focus()
  })
}

// =============== 星图状态提示 ===============
// 说明：星图状态由 galaxyStatus 统一判定。重试期间按钮显示加载，避免重复点击。
// 使用：错误重试走地图的 retry；所读星移出视野时「回到这颗星」按该星坐标拉回镜头。
const retrying = ref(false)
// 只有审核通过且未撤回的公开反馈才进入公开星图体系；
// 待审、转人工等私有反馈只在当前星图确有对应私有星点时参与移出视野判断，绝不触发未单独显示提示。
const selectedPublic = computed(() => panel.value === 'detail' && !unavailable.value &&
  !!selectedStar.value && isPublicFeedback(selectedStar.value.original.reviewState, selectedStar.value.withdrawn))
const selectedOnMap = computed(() => panel.value === 'detail' && !unavailable.value &&
  !!selectedStar.value && (selectedPublic.value || points.value.some(star => star.starId === target.value)))
// 按详情自己的坐标判断是否在画面内：聚合时快照只含代表星，不能用「快照里有没有这个 ID」代替。
const selectedVisible = computed(() => selectedOnMap.value && !!selectedStar.value &&
  isInView(selectedStar.value, camera.value, map.size.value))
const selectedClickable = computed(() => visibleStars.value.some(star => star.point.starId === target.value))
// 因果快照证据：记录当前详情首次被观测为已过审公开态的逻辑序号；
// 只有在此之后才发起并结算的地图快照，才可证实刚过审的星已被服务端纳入本轮快照计算。
const selectedApprovedOrder = ref(0)
let selectedApprovedKey = ''
let confirmedSyncOrder = 0
watch([panel, unavailable, selectedStar], () => {
  const star = panel.value === 'detail' && !unavailable.value ? selectedStar.value : null
  if (!star || !isPublicFeedback(star.original.reviewState, star.withdrawn)) {
    selectedApprovedKey = ''
    selectedApprovedOrder.value = 0
    confirmedSyncOrder = 0
    return
  }
  const key = `${star.starId}:${star.original.reviewVersion}`
  if (key !== selectedApprovedKey) {
    selectedApprovedKey = key
    selectedApprovedOrder.value = nextSnapshotOrder()
    confirmedSyncOrder = 0
  }
}, {flush: 'sync', immediate: true})
const selectedSnapshotConfirmed = computed(() =>
  selectedApprovedOrder.value > 0 && snapshotSyncedOrder.value >= selectedApprovedOrder.value,
)
// 公开星在画面内但未出现在旧快照时，主动触发一次条件快照同步：
// 刚过审的独立星会立刻现身可点；若新快照仍未单独显示该星且同 32×32 桶存在聚合星，再展示中性提示。
watch([
  foreground, selectedPublic, selectedVisible, selectedClickable,
  mapError, snapshotRegion, viewRegion, snapshotSyncedOrder, selectedApprovedOrder,
], () => {
  const order = selectedApprovedOrder.value
  if (!foreground.value || !selectedPublic.value || !selectedVisible.value || selectedClickable.value) return
  if (mapError.value || !snapshotRegion.value || snapshotRegion.value !== viewRegion.value) return
  if (!order || snapshotSyncedOrder.value >= order || confirmedSyncOrder === order) return
  confirmedSyncOrder = order
  map.refreshNow()
}, {flush: 'post', immediate: true})
const galaxyState = computed(() => galaxyStatus({
  foreground: foreground.value, ready: ready.value, error: mapError.value,
  total: points.value.length, visible: visibleStars.value.length,
  snapshotRegion: snapshotRegion.value, viewRegion: viewRegion.value, loading: mapLoading.value,
  selectedId: selectedOnMap.value ? target.value : '',
  selectedPublic: selectedPublic.value,
  selectedVisible: selectedVisible.value,
  selectedClickable: selectedClickable.value,
  selectedGrouped: selectedPublic.value && !!selectedStar.value &&
    isCoveredByAggregate(selectedStar.value, points.value, snapshotBounds.value, {
      snapshotConfirmed: selectedSnapshotConfirmed.value,
    }),
}))
// =============== 星云卡片与右侧栏分流 ===============
// 说明：点击星点弹出的详情改为星云卡片；发布、我的星星与作者提醒继续使用右侧工作面板。
const isDetailOpen = computed(() => panel.value === 'detail')
const isSidePanelOpen = computed(() => panel.value === 'mine' || panel.value === 'notices' || panel.value === 'publish')
const selectedMapPoint = computed(() => target.value ? points.value.find(star => star.starId === target.value) ?? null : null)
const frozenDetail = shallowRef<StarDetailData | null>(null)

// 卡片退场期间冻结最后一份可用详情，防止详情置空导致淡出帧闪烁。
watch([isDetailOpen, selectedStar, unavailable], ([opened, current, isUnavailable]) => {
  if (opened && current && !isUnavailable) frozenDetail.value = current
  else if (opened && isUnavailable) frozenDetail.value = null
}, {immediate: true})

const cardDetail = computed(() => (isDetailOpen.value ? (!unavailable.value ? selectedStar.value : null) : frozenDetail.value))
const cardComposerMode = computed<CardComposerMode | null>(() =>
  isDetailOpen.value && openedEditor.value && openedEditor.value.mode !== 'publish' ? openedEditor.value.mode : null,
)
const cardDiscussionCount = computed(() => {
  const current = cardDetail.value
  if (!current) return 0
  const listCount = current.replies.list?.length ?? 0
  const pinnedCount = (current.officialReplies?.length ?? 0) + (current.supplements?.length ?? 0)
  return Math.max(current.replies.total, listCount, pinnedCount)
})
const detailMapHint = computed<'offscreen' | 'grouped' | ''>(() =>
  galaxyState.value === 'selected-offscreen' ? 'offscreen' : galaxyState.value === 'selected-grouped' ? 'grouped' : '',
)

// 列表打开时不再提示「画面内无星」；发布面板已打开时，空状态卡片的「提意见」入口是多余的；
// 详情卡片已弹出时，所读星位置提示由卡片顶栏承接，避免在半透明遮罩后重复叠影。
const statusKind = computed(() => {
  const kind = galaxyState.value
  if ((browse.value || crowdIds.value.length) && kind === 'offscreen') return ''
  if (panel.value === 'publish' && kind === 'empty') return ''
  if (isDetailOpen.value && (kind === 'selected-offscreen' || kind === 'selected-grouped')) return ''
  return kind
})
// 重试按钮的转圈与本次请求同生命周期：retry 在请求结算（成功、失败或被废弃）时才返回。
// 失败时错误提示保留、按钮恢复可点；成功时错误提示随快照到达消失。
async function retryMap(): Promise<void> {
  if (retrying.value) return
  retrying.value = true
  try { await retryRequest() } finally { retrying.value = false }
}
function publishFromEmpty(): void {
  openPanel('publish')
}

// 回到这颗星：详情自带坐标；拉远到恰好容下它，保留当前朝向。
function locateSelected(): void {
  const star = selectedStar.value
  if (!star) return
  locate(star)
}

// =============== 点击拥挤处的附近星列表 ===============
// 说明：密集时点击落在多颗难分辨的星之间，不猜目标，复用「可浏览星点」列表按距离列出。
// 边界：列表只在点击处出现，Escape 或选择后收起；选择后沿用 openFromList 打开详情。
// 只保存候选 ID；显示的列表每次由当前快照重新校验：轮询替换代表星或移除星点后，失效的选项自动消失。
const crowdIds = ref<string[]>([])
const crowd = computed<ProjectedStar[]>(() => crowdIds.value.length ? revalidateCandidates(crowdIds.value, visibleStars.value) : [])
function showCrowd(stars: ProjectedStar[]): void {
  dismissActions()
  preview.clear()
  browse.value = false
  crowdIds.value = stars.map(star => star.point.starId)
}
// 镜头一动，点击处的星位置即已过期，立即收起列表。
watch(camera, () => { if (crowdIds.value.length) crowdIds.value = [] })
// 候选随快照更新重新校验：失效项从候选 ID 中移除；全部失效时收起列表，仅原焦点在列表内时交还星图。
watch(crowd, (list) => {
  if (!crowdIds.value.length) return
  if (!list.length) {
    const inList = !!document.activeElement?.closest('.visible-stars')
    crowdIds.value = []
    if (inList) void nextTick(() => mapView.value?.focus())
  } else if (list.length !== crowdIds.value.length) {
    crowdIds.value = list.map(star => star.point.starId)
  }
})
function closeCrowd(): void {
  crowdIds.value = []
  void nextTick(() => mapView.value?.focus())
}
const menuOptions: DropdownOption[] = [
  {key: 'publish', label: '提意见', icon: () => h(MessageSquarePlus, {size: 16, 'aria-hidden': true})},
  {key: 'mine', label: '我的星星', icon: () => h(Star, {size: 16, 'aria-hidden': true})},
  {key: 'notices', label: '作者提醒', icon: () => h(Bell, {size: 16, 'aria-hidden': true})},
]
const menuProps = () => ({id: 'feedback-map-actions', role: 'menu', 'aria-label': '星图操作', style: 'min-width: 144px'})
const menuItemProps = () => ({role: 'menuitem'})
const message = useMessage()
watch(status, text => {
  if (!text) return
  if (foreground.value) message.success(text)
  status.value = ''
})
const panelTitle = computed(() => ({mine: '我的星星', notices: '作者提醒', publish: '发布反馈', detail: '', '': ''})[panel.value])
const anchor = computed(() => {
  if (!hoveredStar.value?.anchor) return undefined
  return visibleStars.value.find(star => star.point.starId === hovered.value)
})
const showQuote = computed(() => !!anchor.value && !!quote.value?.preview.trim() && !browse.value && !actionMenu.value && !isDetailOpen.value)
const canTransfer = computed(() => !!selectedStar.value?.owned && !selectedStar.value.canEdit &&
  selectedStar.value.original.reviewState === 'approved' && openedEditor.value?.mode === 'edit')
const editorDisabled = computed(() => {
  if (openedEditor.value?.mode === 'publish') return false
  if (!selectedStar.value || detailBusy.value) return true
  if (openedEditor.value?.mode === 'edit') return !selectedStar.value.canEdit
  return selectedStar.value.original.reviewState !== 'approved'
})

function toggleBrowse(): void {
  dismissActions()
  browse.value = !browse.value
  if (!browse.value) {
    preview.clear()
    void nextTick(() => mapView.value?.focus())
  }
}

function selectMenuAction(key: string | number): void {
  if (!actionMenu.value || (key !== 'publish' && key !== 'mine' && key !== 'notices')) return
  mapView.value?.focus()
  openPanel(key)
}

function openFromList(starId: string): void {
  browse.value = false
  crowdIds.value = []
  void nextTick(() => {
    mapView.value?.focus()
    openDetail(starId)
  })
}
</script>

<template>
  <section class="feedback-page" aria-label="反馈星系" :aria-busy="foreground && !ready && !mapError" @keydown.tab.capture="dismissActions">
    <!-- ~ 地图与辅助导航 -->
    <!-- * 说明：以全景为初始视野，悬停星点可预览反馈。 -->
    <!-- * 交互：拖动旋转、滚轮缩放，回车打开可浏览星点列表。 -->
    <!-- * 依赖：当前镜头与最近一次成功的视区快照。 -->
    <!-- * 边界：网络恢复保留镜头，星点列表提供画布外的文本入口。 -->
    <div class="galaxy-scene" :class="{'has-panel': isSidePanelOpen}">
      <GalaxyMap ref="mapView" :sources="points" :stars="visibleStars" :camera="camera" :arrivals="arrivals" :hovered="hovered"
        :menu-open="!!actionMenu" @menu="openActions" @dismiss-menu="dismissActions"
        :tooltip-id="showQuote ? 'feedback-star-quote' : undefined" @approach="preview.approach"
        :active="foreground" :selected="!unavailable ? target : ''" @change="updateCamera"
        @zoom="zoom" @home="resetCamera" @hover="hover" @open="openDetail" @browse="toggleBrowse" @crowd="showCrowd" />
      <MapStatus ref="mapStatus" :kind="statusKind" :message="mapError" :retrying="retrying" @retry="retryMap" @publish="publishFromEmpty"
        @locate="locateSelected" @home="resetCamera" />
      <StarQuote v-if="showQuote && quote && anchor" :quote="quote" :anchor="anchor"
        @hold="preview.hold" @leave="preview.leave" @contextmenu.prevent="openActions({x: $event.clientX, y: $event.clientY})" />
      <VisibleStars class="scene-stars" :opened="browse || crowdIds.length > 0" :stars="crowdIds.length ? crowd : visibleStars"
        :heading="crowdIds.length ? '附近的星' : '当前视野'" :quote="quote"
        @close="crowdIds.length ? closeCrowd() : toggleBrowse()" @open="openFromList" @preview="hover" />

      <!-- ~ 星图右键菜单 -->
      <!-- * 说明：在点击位置打开反馈操作，选择后进入对应面板。 -->
      <!-- * 边界：菜单仅在星图前台显示，平时不占用画布空间。 -->
      <n-dropdown v-if="actionMenu" trigger="manual" placement="bottom-start" :show="true" :animated="false"
        :x="actionMenu.x" :y="actionMenu.y" :options="menuOptions" :keyboard="true"
        :menu-props="menuProps" :node-props="menuItemProps"
        @clickoutside="dismissActions" @update:show="(shown: boolean) => !shown && dismissActions()" @select="selectMenuAction" />

      <!-- ~ 反馈星点详情弹出卡片 -->
      <!-- * 说明：点击星点后在星图中央以星云观测卡弹出详情与讨论，不再从右侧展开侧栏。 -->
      <!-- * 交互：点击遮罩、关闭按钮或 Escape 收起卡片，焦点自动返回原入口。 -->
      <!-- * 依赖：当前目标星点、星体详情、作者权限与内嵌编辑器状态。 -->
      <!-- * 边界：撤回或失权后仅保留不可用提示；离场动画结束时再释放冻结的展示详情。 -->
      <StarDetailCard
        :open="isDetailOpen"
        :star-id="target"
        :detail="selectedStar"
        :map-point="selectedMapPoint"
        :busy="detailBusy"
        :writing="writing || saving"
        :unavailable="unavailable"
        :detail-error="detailError"
        :action-error="actionError"
        :confirm-withdraw="confirmWithdraw"
        :active-mode="cardComposerMode"
        :map-hint="detailMapHint"
        @close="closePanel"
        @retry="detail.refresh()"
        @locate="locateSelected"
        @edit="cardComposerMode === 'edit' ? editor.close() : editor.open('edit', selectedStar ?? undefined)"
        @supplement="cardComposerMode === 'supplement' ? editor.close() : editor.open('supplement', selectedStar ?? undefined)"
        @withdraw="confirmWithdraw = !confirmWithdraw"
        @cancel-withdraw="confirmWithdraw = false"
        @confirm-withdraw="changeStar('withdraw')"
        @after-leave="frozenDetail = null"
      >
        <StarDetail
          v-if="cardDetail"
          :detail="cardDetail"
          :busy="detailBusy"
          :highlighted="highlightedEntry"
          @page="detail.changePage"
        />

        <template v-if="cardDetail && !unavailable" #footer>
          <StarComposer
            :published="cardDetail.original.reviewState === 'approved'"
            :liked="cardDetail.liked"
            :like-count="cardDetail.likeCount"
            :discussion-count="cardDiscussionCount"
            :busy="detailBusy"
            :writing="writing || saving"
            :mode="cardComposerMode"
            :body="draft.body"
            :length="length"
            :limit="limit"
            :valid="valid"
            :saving="saving"
            :error="editorError"
            :disabled="editorDisabled"
            :can-transfer="canTransfer"
            @like="changeStar('like')"
            @open-comment="editor.open('comment', selectedStar ?? undefined)"
            @text="editor.update({...draft, body: $event})"
            @submit="editor.submit(selectedStar ?? undefined)"
            @close="editor.close()"
            @transfer="selectedStar && editor.transferToSupplement(selectedStar)"
          />
        </template>
      </StarDetailCard>

      <!-- ~ 右侧功能面板（发布、我的星星与作者提醒） -->
      <!-- * 说明：承载发布反馈、个人列表和作者提醒，与星点详情弹出卡分离。 -->
      <!-- * 交互：Escape 收起，关闭后将焦点交回原入口。 -->
      <!-- * 依赖：panel 模式与个人分页状态。 -->
      <!-- * 边界：从个人列表点选星星后自动切换为星云详情弹出卡。 -->
      <aside v-if="isSidePanelOpen" class="galaxy-panel" aria-labelledby="feedback-panel-title" @keydown.esc.stop.prevent="closePanel">
        <header class="panel-header">
          <h2 ref="panelHeading" id="feedback-panel-title" tabindex="-1">{{ panelTitle }}</h2>
          <n-button quaternary size="small" aria-label="关闭星系面板" title="关闭" :disabled="writing || saving" @click="closePanel">
            <X :size="17" aria-hidden="true" />
          </n-button>
        </header>
        <div class="panel-body">
          <PersonalList v-if="panel === 'mine' || panel === 'notices'" :mode="panel"
            :mine="mine.page" :notices="notices.page" :busy="currentCollection.busy" :error="currentCollection.error"
            @mine="openMine" @notice="personal.openNotice" @page="changePersonalPage" @retry="changePersonalPage()" />

          <EditorForm v-if="panel === 'publish' && openedEditor?.mode === 'publish'" mode="publish" :body="draft.body"
            :show-name="draft.showName" :limit="limit" :length="length" :valid="valid" :saving="saving" :error="editorError"
            :disabled="false" :can-transfer="false" @text="editor.update({...draft, body: $event})"
            @name="editor.update({...draft, showName: $event})" @submit="editor.submit()" @close="closePanel" />
        </div>
      </aside>
    </div>
  </section>
</template>

<style scoped lang="scss">
.feedback-page {
  position: absolute;
  inset: 0;
  min-width: 0;
  color: var(--ds-text-primary);
  background: var(--ds-bg-shell);
}
.galaxy-scene {
  --galaxy-panel-width: 0px;
  position: absolute; inset: 0; overflow: hidden;
}
.galaxy-scene.has-panel { --galaxy-panel-width: min(428px, 62%); }
.galaxy-scene .scene-stars {
  width: min(300px, calc(100% - var(--galaxy-panel-width) - 24px));
  max-height: calc(100% - 24px);
  overflow-wrap: anywhere;
}
.galaxy-panel {
  position: absolute; z-index: 3; inset: 0 0 0 auto; display: flex; flex-direction: column;
  width: var(--galaxy-panel-width); border-left: 1px solid var(--ds-border-subtle);
  color: var(--ds-text-primary); background: var(--ds-bg-panel-elevated);
}
.panel-header {
  flex: 0 0 auto; display: flex; align-items: center; justify-content: space-between;
  padding: 11px 16px; border-bottom: 1px solid var(--ds-border-subtle);
}
.panel-header h2 {
  margin: 0; font-size: 16px; font-weight: 650; padding-bottom: 2px;
  // 抽屉标题仅作为程序化焦点落点（供无障碍与 Esc/Tab 键盘流接管），去掉默认生硬矩形外框；
  // 键盘聚焦时仅保留一条贴合产品细边框语言的克制底部微线。
  &:focus { outline: none; }
  &:focus-visible { outline: none; box-shadow: inset 0 -1.5px 0 color-mix(in srgb, var(--ds-brand) 42%, transparent); }
}
.panel-body { padding: 14px 16px; flex: 1; min-height: 0; overflow: auto; scrollbar-gutter: stable; }
</style>
