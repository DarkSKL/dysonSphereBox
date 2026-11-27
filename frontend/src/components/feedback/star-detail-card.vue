<script setup lang="ts">
import {AlertCircle, Compass, FilePlus2, RotateCw, Trash2, X} from '@lucide/vue'
import {computed, nextTick, ref, shallowRef, watch} from 'vue'

import {useGalaxyTheme} from '../../composables/feedback/useGalaxyTheme'
import {accountUser} from '../../pages/services/account'
import {
  formatFeedbackTime,
  progressLabels,
  reviewLabels,
  type DisplayStar,
  type ReviewState,
  type StarDetail,
  type StarProgress,
} from '../../types/feedback'
import GalaxyMinimap from './galaxy-minimap.vue'
import StarAvatar from './star-avatar.vue'
import type {CardComposerMode} from './star-composer.vue'

interface HeaderSnapshot {
  starId: string
  authorName: string
  authorAnonymous: boolean
  authorAvatar: string
  createdAt: string
  progress: StarProgress
  isPrivate: boolean
  privateLabel: string
  releaseVersion: string
  owned: boolean
  canEdit: boolean
  published: boolean
  reviewState?: ReviewState
  unavailable: boolean
  hasDetail: boolean
}

const props = withDefaults(defineProps<{
  open: boolean
  starId: string
  detail: StarDetail | null
  mapPoint: DisplayStar | null
  busy: boolean
  writing: boolean
  unavailable: boolean
  detailError: string
  actionError: string
  confirmWithdraw: boolean
  activeMode?: CardComposerMode | null
  mapHint?: 'offscreen' | 'grouped' | ''
  minimapStars?: readonly DisplayStar[]
}>(), {
  activeMode: null,
  mapHint: '',
  minimapStars: () => [],
})

const emit = defineEmits<{
  close: []
  retry: []
  locate: []
  supplement: []
  withdraw: []
  cancelWithdraw: []
  confirmWithdraw: []
  afterLeave: []
}>()

// =============== 主题与离场快照 ===============
// 说明：复用星图双主题色板；卡片退场期间保留最后一帧星体信息，避免关闭过渡时文字突然变空。
// 边界：不发起业务请求，离场结束后清空快照。
const theme = useGalaxyTheme()
const heading = ref<HTMLElement>()
const snapshot = shallowRef<HeaderSnapshot | null>(null)

function isAnonymousDisplay(name: string | undefined): boolean {
  const trimmed = name?.trim() ?? ''
  return !trimmed || trimmed === '匿名' || trimmed === '匿名玩家'
}

function buildSnapshot(): HeaderSnapshot {
  const detail = props.detail
  const point = props.mapPoint
  const reviewState = detail?.original.reviewState
  const published = reviewState === 'approved'
  const isPrivate = props.unavailable || (detail ? !published : Boolean(point?.privateLabel))
  const rawName = detail?.original.displayName?.trim() ?? ''
  const authorAnonymous = !detail || isAnonymousDisplay(rawName)
  const entryAvatar = detail?.original.avatar || detail?.original.headerImg || ''
  // 匿名反馈强制使用默认头像；本人实名反馈在服务端未单独回传头像时复用当前账号头像，其余由默认头像兜底。
  const authorAvatar = authorAnonymous
    ? ''
    : (entryAvatar || (detail?.owned ? (accountUser.value?.headerImg ?? '') : ''))
  return {
    starId: detail?.starId || props.starId || point?.starId || '',
    authorName: detail ? (rawName || '匿名玩家') : '',
    authorAnonymous,
    authorAvatar,
    createdAt: detail?.original.createdAt ?? '',
    progress: detail?.progress ?? point?.progress ?? 'feedback',
    isPrivate,
    privateLabel: point?.privateLabel ?? '',
    releaseVersion: detail?.releaseVersion?.trim() ?? '',
    owned: Boolean(detail?.owned),
    canEdit: Boolean(detail?.canEdit),
    published,
    reviewState,
    unavailable: props.unavailable,
    hasDetail: Boolean(detail),
  }
}

watch(
  [() => props.open, () => props.starId, () => props.detail, () => props.mapPoint, () => props.unavailable, accountUser],
  () => {
    if (props.open) snapshot.value = buildSnapshot()
  },
  {immediate: true},
)

const view = computed<HeaderSnapshot>(() => (props.open ? buildSnapshot() : (snapshot.value ?? buildSnapshot())))
const headingTitle = computed(() => view.value.authorName || '星星详情')
const showProgress = computed(() => !view.value.unavailable && (view.value.hasDetail || Boolean(props.mapPoint)))
// 仅在待审核/转人工/未通过等非公开状态时呈现审核徽章，已过审公开星不再堆叠冗余标签。
const reviewBadge = computed(() => {
  if (view.value.unavailable) return ''
  if (view.value.reviewState && view.value.reviewState !== 'approved') return reviewLabels[view.value.reviewState]
  return view.value.privateLabel
})
const reviewTone = computed(() => {
  if (view.value.reviewState === 'rejected') return 'is-error'
  if (view.value.reviewState === 'pending' || view.value.reviewState === 'manual' || view.value.privateLabel) return 'is-pending'
  return ''
})
const starAccent = computed(() =>
  view.value.isPrivate
    ? theme.value.privateColor
    : (theme.value.starColors[view.value.progress] ?? theme.value.starColors.feedback),
)
const hasOwnerTools = computed(() => view.value.hasDetail && !view.value.unavailable && view.value.owned)
// 缩略图高亮当前读的星：优先用详情坐标，详情未到时退回星图点位。
const minimapSelected = computed(() => {
  const source = props.detail ?? props.mapPoint
  return source ? {x: source.x, y: source.y, z: source.z} : null
})

// =============== 焦点接管与遮罩关闭守卫 ===============
// 说明：打开或切星时聚焦卡片标题以便 Escape 立即生效；仅当按下与抬起均落在遮罩自身才关闭，防止划选正文拖出边界误关。
let pointerStartedOnBackdrop = false

watch([() => props.open, () => props.starId], ([opened]) => {
  pointerStartedOnBackdrop = false
  if (!opened) return
  void nextTick(() => heading.value?.focus({preventScroll: true}))
}, {immediate: true})

function onBackdropPointerDown(): void {
  pointerStartedOnBackdrop = true
}

function onCardPointerDown(): void {
  pointerStartedOnBackdrop = false
}

function onBackdropClick(): void {
  const shouldClose = pointerStartedOnBackdrop
  pointerStartedOnBackdrop = false
  if (shouldClose) requestClose()
}

function requestClose(): void {
  if (props.writing) return
  if (props.confirmWithdraw) {
    emit('cancelWithdraw')
    return
  }
  emit('close')
}

function onAfterLeave(): void {
  if (!props.open) snapshot.value = null
  emit('afterLeave')
}

defineExpose({
  focusHeading: () => heading.value?.focus({preventScroll: true}),
})
</script>

<template>
  <!-- ~ 反馈星点详情弹出卡片 -->
  <!-- * 说明：高信息密度三段式布局——单排融合顶栏（作者左置，状态与管理图标右置）、纯净正文评论区、融合点赞与输入的底部栏。 -->
  <!-- * 交互：点击遮罩、右上关闭图标或按 Escape 收起卡片；作者管理按钮就地收在顶栏右侧，不额外占行。 -->
  <!-- * 依赖：当前选中星点、详情数据、正文插槽与底部互动评论插槽。 -->
  <!-- * 边界：写入期间禁止关闭；退场动画期间冻结最后内容并禁用指针事件，不闪烁、不拦截底层星图点击。 -->
  <Transition name="star-card-modal" @after-leave="onAfterLeave">
    <div
      v-if="open"
      class="star-card-backdrop"
      :class="'is-' + theme.mode"
      @pointerdown.self="onBackdropPointerDown"
      @click.self="onBackdropClick"
      @wheel.stop
      @keydown.esc.stop.prevent="requestClose"
    >
      <!-- 右上角星图缩略图：纯展示，不拦截指针 -->
      <GalaxyMinimap v-if="minimapStars.length" :stars="minimapStars" :selected="minimapSelected" />

      <section
        class="star-detail-card ds-bracket"
        role="dialog"
        aria-modal="true"
        aria-labelledby="feedback-star-card-title"
        :style="{'--star-accent': starAccent}"
        @pointerdown="onCardPointerDown"
      >
        <!-- ~ 单排融合顶栏 -->
        <!-- * 说明：左侧呈现作者与发布时间，右侧紧凑收拢星号、状态标签、作者管理工具图标与关闭按钮，彻底消除多层空行堆叠。 -->
        <header class="star-card-header">
          <div class="header-author">
            <span class="author-avatar" aria-hidden="true">
              <StarAvatar
                :identity="view.starId || 'star-author'"
                :anonymous="view.authorAnonymous"
                :source="view.authorAvatar"
              />
            </span>
            <div class="author-info">
              <div class="author-name-line">
                <h2
                  ref="heading"
                  id="feedback-star-card-title"
                  class="star-card-title"
                  :title="headingTitle"
                  tabindex="-1"
                >
                  {{ headingTitle }}
                </h2>
                <span v-if="view.owned && view.hasDetail" class="owner-chip">我的</span>
              </div>
              <p v-if="view.createdAt" class="star-time">{{ formatFeedbackTime(view.createdAt) }}</p>
            </div>
          </div>

          <div class="header-right">
            <div class="header-meta">
              <span v-if="view.starId" class="star-id type-display">#{{ view.starId }}</span>
              <span v-if="showProgress" class="status-pill">
                <i class="status-dot" :class="{'is-private': view.isPrivate}" aria-hidden="true" />
                <span>{{ progressLabels[view.progress] }}</span>
              </span>
              <span v-if="view.releaseVersion" class="meta-chip type-mono">v{{ view.releaseVersion }}</span>
              <span v-if="reviewBadge" class="meta-chip" :class="reviewTone">{{ reviewBadge }}</span>
            </div>

            <span class="header-sep" aria-hidden="true" />

            <div class="header-tools" role="toolbar" aria-label="反馈管理与窗口操作">
              <n-tooltip v-if="hasOwnerTools && !view.canEdit && view.published" :delay="220" placement="bottom">
                <template #trigger>
                  <button
                    type="button"
                    class="tool-icon-btn"
                    :class="{'is-active': activeMode === 'supplement'}"
                    aria-label="追加作者补充"
                    :aria-pressed="activeMode === 'supplement'"
                    :disabled="writing || busy"
                    @click="emit('supplement')"
                  >
                    <FilePlus2 :size="15" :stroke-width="1.85" aria-hidden="true" />
                  </button>
                </template>
                追加补充
              </n-tooltip>

              <n-tooltip v-if="hasOwnerTools" :delay="220" placement="bottom">
                <template #trigger>
                  <button
                    type="button"
                    class="tool-icon-btn is-danger"
                    :class="{'is-active': confirmWithdraw}"
                    aria-label="撤回这条反馈"
                    :disabled="writing"
                    @click="emit('withdraw')"
                  >
                    <Trash2 :size="15" :stroke-width="1.85" aria-hidden="true" />
                  </button>
                </template>
                撤回反馈
              </n-tooltip>

              <n-tooltip :delay="240" placement="bottom-end">
                <template #trigger>
                  <button
                    type="button"
                    class="tool-icon-btn"
                    aria-label="关闭星星详情"
                    :disabled="writing"
                    @click="requestClose"
                  >
                    <X :size="16" :stroke-width="1.9" aria-hidden="true" />
                  </button>
                </template>
                关闭 (Esc)
              </n-tooltip>
            </div>
          </div>
        </header>

        <!-- ~ 就地撤回确认条（紧贴顶栏撤回图标下方展开，零视线跳跃） -->
        <div v-if="confirmWithdraw && detail" class="withdraw-strip" role="alertdialog" aria-label="确认撤回反馈">
          <AlertCircle :size="14" class="withdraw-icon" aria-hidden="true" />
          <span class="withdraw-copy">撤回后，这条反馈与所有讨论将停止对外展示。</span>
          <div class="withdraw-btns">
            <n-button size="tiny" quaternary :disabled="writing" @click="emit('cancelWithdraw')">取消</n-button>
            <n-button size="tiny" type="error" :loading="writing" @click="emit('confirmWithdraw')">确认撤回</n-button>
          </div>
        </div>

        <!-- ~ 星图位置提示带 -->
        <!-- * 说明：所读星移出镜头或未在当前快照单独显示时提供极简事实提示与回位入口。 -->
        <div v-if="mapHint" class="star-map-strip" role="status">
          <Compass :size="13" class="strip-icon" aria-hidden="true" />
          <span class="strip-text">{{ mapHint === 'offscreen' ? '这颗星不在当前画面内' : '当前星图未单独显示这条反馈' }}</span>
          <button v-if="mapHint === 'offscreen'" type="button" class="strip-action" @click="emit('locate')">回到这颗星</button>
        </div>

        <!-- ~ 纯净正文与简洁评论滚动区 -->
        <div class="star-card-body">
          <div v-if="busy && !detail && !unavailable && !detailError" role="status" class="card-loading">
            <span class="loading-bar is-wide" />
            <span class="loading-bar is-mid" />
            <span class="loading-bar is-short" />
          </div>

          <div v-if="detailError" class="card-alert" role="status">
            <AlertCircle :size="14" class="alert-icon" aria-hidden="true" />
            <span class="alert-text">{{ detailError }}</span>
            <n-button size="tiny" quaternary @click="emit('retry')">
              <template #icon><RotateCw :size="12" aria-hidden="true" /></template>
              重试
            </n-button>
          </div>

          <div v-if="actionError && !unavailable" class="card-alert" role="alert">
            <AlertCircle :size="14" class="alert-icon" aria-hidden="true" />
            <span class="alert-text">{{ actionError }}</span>
          </div>

          <slot />
        </div>

        <!-- ~ 底部融合互动与评论插槽 -->
        <div v-if="$slots.footer" class="star-card-footer">
          <slot name="footer" />
        </div>
      </section>
    </div>
  </Transition>
</template>

<style scoped lang="scss">
// =============== 遮罩与商业卡片外壳 ===============
// 说明：沿用账号弹窗（account-modal）的 8px 外框圆角、1px 细边框与实底面板体系；
//       通过单排融合顶栏与底部互动输入条消除多余横条，让正文与评论成为视觉中心。
.star-card-backdrop {
  position: absolute;
  z-index: 4;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgb(1 4 12 / 0.56);
  backdrop-filter: blur(8px);
  // 命名容器：供右上角缩略图按遮罩宽度决定是否与居中卡片并排。
  container: star-backdrop / inline-size;

  &.is-light {
    background: rgb(235 235 235 / 0.56);
    backdrop-filter: blur(6px);
  }
}

.star-detail-card {
  --bracket-size: 16px;
  --bracket-color: color-mix(in srgb, var(--star-accent, var(--ds-border-strong)) 58%, transparent);
  position: relative;
  display: flex;
  flex-direction: column;
  width: min(560px, calc(100% - 12px));
  max-height: min(680px, calc(100% - 12px));
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-radius: 8px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-primary);
  box-shadow: 0 26px 76px rgb(0 0 0 / 0.44);
  isolation: isolate;

  &.ds-bracket::after {
    z-index: 3;
  }
}

.is-light .star-detail-card {
  --bracket-color: var(--ds-border-strong);
  box-shadow: 0 22px 58px rgb(0 0 0 / 0.14);
}

// =============== 单排融合顶栏 ===============
.star-card-header {
  position: relative;
  z-index: 1;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 12px 12px 18px;
  border-bottom: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel);
}

.header-author {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1 1 140px;
  min-width: 0;
}

.author-avatar {
  flex: none;
  width: 32px;
  height: 32px;
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-radius: 50%;
  background: var(--ds-bg-panel-elevated);
}

.author-info {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.author-name-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.star-card-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  line-height: 1.35;
  color: var(--ds-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  &:focus {
    outline: none;
  }
  &:focus-visible {
    outline: none;
    box-shadow: inset 0 -1.5px 0 color-mix(in srgb, var(--ds-brand) 48%, transparent);
  }
}

.owner-chip {
  flex: none;
  padding: 0 5px;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  color: var(--ds-text-secondary);
  font-size: 10.5px;
  font-weight: 600;
  line-height: 1.5;
}

.star-time {
  margin: 0;
  color: var(--ds-text-muted);
  font-size: 11.5px;
  line-height: 1.3;
  white-space: nowrap;
}

.header-right {
  flex: 0 1 auto;
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.header-meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 6px;
  font-size: 11.5px;
  line-height: 1.25;
}

.star-id {
  color: var(--star-accent, var(--ds-text-secondary));
  font-size: 12px;
  font-weight: 600;
}

.is-light .star-id {
  color: var(--ds-text-muted);
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 7px;
  border: 1px solid color-mix(in srgb, var(--star-accent, var(--ds-border-default)) 26%, transparent);
  border-radius: 4px;
  background: color-mix(in srgb, var(--star-accent, var(--ds-brand)) 11%, transparent);
  color: var(--ds-text-primary);
  font-weight: 600;
  white-space: nowrap;
}

.is-light .status-pill {
  border-color: var(--ds-border-default);
  background: var(--ds-active-bg);
}

@keyframes status-star-twinkle {
  0%, 100% {
    opacity: 0.4;
    transform: scale(0.86);
  }
  50% {
    opacity: 1;
    transform: scale(1.08);
  }
}

.status-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--star-accent, var(--ds-brand));
  &.is-private {
    box-shadow: 0 0 6px var(--star-accent, var(--ds-brand));
    animation: status-star-twinkle 2.4s ease-in-out infinite;
  }
}

.is-light .status-dot.is-private {
  box-shadow: none;
}

@media (prefers-reduced-motion: reduce) {
  .status-dot.is-private {
    animation: none;
  }
}

.meta-chip {
  padding: 2px 6px;
  border-radius: 4px;
  background: var(--ds-hover-bg);
  color: var(--ds-text-muted);
  font-size: 11px;
  white-space: nowrap;

  &.is-pending {
    background: var(--ds-warning-bg);
    color: var(--ds-warning);
  }
  &.is-error {
    background: var(--ds-error-bg);
    color: var(--ds-error);
  }
}

.header-sep {
  flex: none;
  width: 1px;
  height: 16px;
  background: var(--ds-border-default);
}

.header-tools {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  flex: none;
}

.tool-icon-btn {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-secondary);
  cursor: pointer;
  transition: background-color 140ms ease, color 140ms ease, border-color 140ms ease;

  &:hover:not(:disabled) {
    background: var(--ds-hover-bg);
    color: var(--ds-text-primary);
  }

  &:focus-visible {
    outline: 2px solid var(--ds-brand);
    outline-offset: 1px;
  }

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  &.is-active {
    border-color: var(--ds-border-default);
    background: var(--ds-active-bg);
    color: var(--ds-text-primary);
  }

  &.is-danger {
    color: var(--ds-text-muted);
    &:hover:not(:disabled),
    &.is-active {
      border-color: var(--ds-error-border);
      background: var(--ds-error-bg);
      color: var(--ds-error);
    }
  }
}

// =============== 撤回确认、位置提示与正文滚动区 ===============
.withdraw-strip {
  position: relative;
  z-index: 1;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px 18px;
  border-bottom: 1px solid var(--ds-error-border);
  background: var(--ds-error-bg);
  color: var(--ds-text-primary);
  font-size: 12.5px;
}

.withdraw-icon {
  flex: none;
  color: var(--ds-error);
}

.withdraw-copy {
  flex: 1;
  min-width: 160px;
  line-height: 1.4;
}

.withdraw-btns {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}

.star-map-strip {
  position: relative;
  z-index: 1;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 18px;
  border-bottom: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel-elevated);
  color: var(--ds-text-secondary);
  font-size: 12px;
  line-height: 1.4;
}

.strip-icon {
  flex: none;
  color: var(--ds-text-muted);
}

.strip-text {
  flex: 1;
  min-width: 0;
}

.strip-action {
  flex: none;
  padding: 2px 8px;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-primary);
  font: inherit;
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 140ms ease, background-color 140ms ease;

  &:hover {
    border-color: var(--ds-border-strong);
    background: var(--ds-hover-bg);
  }
}

.star-card-body {
  position: relative;
  z-index: 1;
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
  padding: 16px 18px 16px;
}

.star-card-footer {
  position: relative;
  z-index: 2;
  flex: 0 0 auto;
}

.card-loading {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 4px 0 8px;
}

.loading-bar {
  display: block;
  height: 12px;
  border-radius: 3px;
  background: var(--ds-hover-bg);
  &.is-wide { width: 92%; }
  &.is-mid { width: 74%; }
  &.is-short { width: 46%; }
}

.card-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding: 8px 10px;
  border: 1px solid var(--ds-error-border);
  border-radius: 4px;
  background: var(--ds-error-bg);
  color: var(--ds-error);
  font-size: 12.5px;
  line-height: 1.45;
}

.alert-icon {
  flex: none;
}

.alert-text {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}

// =============== 弹窗进出场过渡 ===============
.star-card-modal-enter-active {
  transition: opacity 200ms cubic-bezier(0.22, 1, 0.36, 1);
  .star-detail-card {
    transition:
      opacity 220ms cubic-bezier(0.22, 1, 0.36, 1),
      transform 240ms cubic-bezier(0.22, 1, 0.36, 1);
  }
}

.star-card-modal-leave-active {
  pointer-events: none;
  transition: opacity 160ms ease-in;
  .star-detail-card {
    transition:
      opacity 150ms ease-in,
      transform 160ms ease-in;
  }
}

.star-card-modal-enter-from,
.star-card-modal-leave-to {
  opacity: 0;
  .star-detail-card {
    opacity: 0;
    transform: translateY(8px) scale(0.98);
  }
}

@media (max-width: 540px) {
  .star-card-backdrop {
    padding: 10px;
  }
  .star-card-header {
    flex-wrap: wrap;
    padding: 11px 12px 11px 14px;
  }
  .header-right {
    width: 100%;
    justify-content: space-between;
  }
  .header-sep {
    display: none;
  }
  .withdraw-strip,
  .star-map-strip {
    padding: 7px 14px;
  }
  .star-card-body {
    padding: 14px;
  }
}
</style>
