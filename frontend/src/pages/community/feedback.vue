<script setup lang="ts">
import {Bell, MessageSquarePlus, Star, X} from '@lucide/vue'
import {useMessage, type DropdownOption} from 'naive-ui'
import {computed, h, nextTick, ref, watch} from 'vue'

import EditorForm from '../../components/feedback/editor-form.vue'
import GalaxyMap from '../../components/feedback/galaxy-map.vue'
import PersonalList from '../../components/feedback/personal-list.vue'
import StarDetail from '../../components/feedback/star-detail.vue'
import StarQuote from '../../components/feedback/star-quote.vue'
import VisibleStars from '../../components/feedback/visible-stars.vue'
import {useFeedbackGalaxy} from '../../composables/feedback/useFeedbackGalaxy'

definePage({meta: {layout: 'main', keepAlive: true, title: '反馈星系'}})

// =============== 页面展示 ===============
// 说明：星图占满页面，按需显示预览、星点列表与详情。
const {
  foreground, map, preview, personal, editor, detail, panel, panelHeading, browse, points, visibleStars,
  confirmWithdraw, writing, actionError, status, currentCollection, closePanel,
  actionMenu, openActions, dismissActions, openPanel,
  changePersonalPage, openDetail, openMine, hover, changeStar, resetCamera, zoom,
} = useFeedbackGalaxy()
const {camera, arrivals, ready, error: mapError, updateCamera, refreshNow} = map
const {hover: hoveredStar, hovered, quote} = preview
const {detail: selectedStar, highlightedEntry, target, busy: detailBusy, unavailable, error: detailError} = detail
const {editor: openedEditor, draft, error: editorError, saving, length, limit, valid} = editor
const {mine, notices} = personal
const mapView = ref<InstanceType<typeof GalaxyMap>>()
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
const panelTitle = computed(() => ({detail: '星星详情', mine: '我的星星', notices: '作者提醒', publish: '发布反馈', '': ''})[panel.value])
const anchor = computed(() => {
  if (!hoveredStar.value?.anchor) return undefined
  return visibleStars.value.find(star => star.point.starId === hovered.value)
})
const showQuote = computed(() => !!anchor.value && !!quote.value?.preview.trim() && !browse.value && !actionMenu.value)
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
    <div class="galaxy-scene" :class="{'has-panel': panel}">
      <GalaxyMap ref="mapView" :sources="points" :stars="visibleStars" :camera="camera" :arrivals="arrivals" :hovered="hovered"
        :menu-open="!!actionMenu" @menu="openActions" @dismiss-menu="dismissActions"
        :tooltip-id="showQuote ? 'feedback-star-quote' : undefined" @approach="preview.approach"
        :active="foreground" :selected="!unavailable ? target : ''" @change="updateCamera"
        @zoom="zoom" @home="resetCamera" @hover="hover" @open="openDetail" @browse="toggleBrowse" />
      <div v-if="mapError" class="map-error" role="alert">
        {{ mapError }} <n-button size="tiny" @click="refreshNow">重试</n-button>
      </div>
      <StarQuote v-if="showQuote && quote && anchor" :quote="quote" :anchor="anchor"
        @hold="preview.hold" @leave="preview.leave" @contextmenu.prevent="openActions({x: $event.clientX, y: $event.clientY})" />
      <VisibleStars class="scene-stars" :opened="browse" :stars="visibleStars" :quote="quote"
        @close="toggleBrowse" @open="openFromList" @preview="hover" />

      <!-- ~ 星图右键菜单 -->
      <!-- * 说明：在点击位置打开反馈操作，选择后进入对应面板。 -->
      <!-- * 边界：菜单仅在星图前台显示，平时不占用画布空间。 -->
      <n-dropdown v-if="actionMenu" trigger="manual" placement="bottom-start" :show="true" :animated="false"
        :x="actionMenu.x" :y="actionMenu.y" :options="menuOptions" :keyboard="true"
        :menu-props="menuProps" :node-props="menuItemProps"
        @clickoutside="dismissActions" @update:show="(shown: boolean) => !shown && dismissActions()" @select="selectMenuAction" />

      <!-- ~ 独立阅读与写入面板 -->
      <!-- * 说明：切换详情、发布、我的星星和提醒。 -->
      <!-- * 交互：Escape 收起，关闭后将焦点交回原入口。 -->
      <!-- * 依赖：当前目标、作者权限和同会话写入状态。 -->
      <!-- * 边界：撤回或失权后只保留不可用状态。 -->
      <aside v-if="panel" class="galaxy-panel" aria-labelledby="feedback-panel-title" @keydown.esc.stop.prevent="closePanel">
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

          <template v-if="panel === 'detail'">
            <p v-if="detailBusy && !selectedStar" role="status" class="panel-hint">正在读取反馈…</p>
            <div v-if="detailError" class="panel-error" role="status">
              {{ detailError }} <n-button size="tiny" @click="detail.refresh()">重试</n-button>
            </div>
            <p v-if="actionError && !unavailable" class="panel-error" role="alert">{{ actionError }}</p>
            <div v-if="confirmWithdraw && selectedStar" class="withdraw-confirm">
              <p>撤回后，星星及其讨论将停止对所有玩家展示。确定撤回？</p>
              <n-button size="small" :disabled="writing" @click="confirmWithdraw = false">取消</n-button>
              <n-button size="small" type="error" :loading="writing" @click="changeStar('withdraw')">确认撤回</n-button>
            </div>
            <StarDetail v-if="selectedStar && !unavailable" :detail="selectedStar" :busy="detailBusy" :writing="writing || saving"
              :highlighted="highlightedEntry"
              @like="changeStar('like')" @edit="editor.open('edit', selectedStar ?? undefined)"
              @comment="editor.open('comment', selectedStar ?? undefined)" @supplement="editor.open('supplement', selectedStar ?? undefined)"
              @withdraw="confirmWithdraw = true" @page="detail.changePage">
              <EditorForm v-if="openedEditor && openedEditor.mode !== 'publish'" :mode="openedEditor.mode" :body="draft.body"
                :show-name="draft.showName" :limit="limit" :length="length" :valid="valid" :saving="saving" :error="editorError"
                :disabled="editorDisabled" :can-transfer="canTransfer" @text="editor.update({...draft, body: $event})"
                @submit="editor.submit(selectedStar ?? undefined)" @close="editor.close()"
                @transfer="selectedStar && editor.transferToSupplement(selectedStar)" />
            </StarDetail>
          </template>

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
.map-error {
  position: absolute; bottom: 12px; left: 12px; max-width: calc(100% - 24px);
  display: flex; align-items: center; gap: 8px; padding: 6px 10px; border-radius: 4px;
  color: var(--ds-text-primary); background: var(--ds-bg-panel-elevated); font-size: 13px;
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
.panel-header h2 { margin: 0; font-size: 16px; font-weight: 650; }
.panel-body { padding: 14px 16px; flex: 1; min-height: 0; overflow: auto; scrollbar-gutter: stable; }
.panel-hint, .panel-error, .withdraw-confirm { font-size: 13px; }
.panel-error { color: var(--ds-error); margin-bottom: 12px; overflow-wrap: anywhere; }
.withdraw-confirm { padding: 8px 0 12px; margin-bottom: 12px; border-bottom: 1px solid var(--ds-border-subtle); }
.withdraw-confirm p { margin: 0 0 8px; }
.withdraw-confirm button + button { margin-left: 6px; }
</style>
