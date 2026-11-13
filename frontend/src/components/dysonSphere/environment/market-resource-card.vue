<script setup lang="ts">
import {ArrowUp, RotateCw, Store, Wrench, X} from '@lucide/vue'
import {computed} from 'vue'
import ResourceCard from './resource-card.vue'

// =============== 市场资源展示契约 ===============
// 说明：接收已安装版本与更新快照，只向父级发送操作意图；安装器及全局串行状态由父级维护。
const props = withDefaults(defineProps<{
  packageKey: string
  title: string
  version: string
  description: string
  updateVersion?: string
  updateDisabled?: boolean
  updating?: boolean
  uninstalling?: boolean
  progress?: number
  progressStatus?: string
  downloadSpeed?: string
  failureMessage?: string
  conflicted?: boolean
}>(), {
  updateVersion: '',
  updateDisabled: false,
  updating: false,
  uninstalling: false,
  progress: 0,
  progressStatus: '',
  downloadSpeed: '',
  failureMessage: '',
  conflicted: false,
})

const emit = defineEmits<{
  (event: 'open', key: string): void
  (event: 'update', key: string): void
  (event: 'cancel', key: string): void
  (event: 'repair', key: string): void
  (event: 'contextmenu', mouseEvent: MouseEvent): void
}>()

// =============== 卡片状态与提示 ===============
// 说明：全局禁用仅限制升级入口；失败时仍显示传入的已安装版本，错误详情收进悬停提示。
const busy = computed(() => props.updating || props.uninstalling)
const upgradeDisabled = computed(() => busy.value || props.updateDisabled)
const hasFailure = computed(() => Boolean(props.failureMessage.trim()))
const upgradeLabel = computed(() => `升级至 ${props.updateVersion}`)
const retryLabel = computed(() => props.updateVersion ? `重试升级至 ${props.updateVersion}` : '重试更新')
const percent = computed(() => Number.isFinite(props.progress)
  ? Math.floor(Math.min(100, Math.max(0, props.progress)))
  : 0)
const stage = computed(() => props.progressStatus || '更新中')
const progressLabel = computed(() => [stage.value, `${percent.value}%`, props.downloadSpeed].filter(Boolean).join(' · '))
const status = computed(() => {
  if (props.uninstalling) return '卸载中'
  if (props.updating) return '更新中'
  if (props.conflicted) return '待修复'
  if (hasFailure.value) return '更新失败'
  return '已启用'
})
const cardTitle = computed(() => [
  props.title,
  `已安装版本：${props.version}`,
  props.description,
  props.updating ? progressLabel.value : '',
  props.failureMessage,
].filter(Boolean).join('\n'))

// =============== 资源操作转发 ===============
// 说明：主按钮忙碌时不打开详情；徽标独立于主按钮，使用原生键盘激活并阻止事件冒泡。
function openResource(): void {
  if (!busy.value) emit('open', props.packageKey)
}

// 说明：升级与失败重试共用串行限制，不将其他卡片的操作显示为本卡片正在更新。
function updateResource(): void {
  if (!upgradeDisabled.value) emit('update', props.packageKey)
}

// 说明：取消只作用于当前更新，修复只作用于当前冲突，不受全局升级禁用影响。
function cancelUpdate(): void {
  if (props.updating && !props.uninstalling) emit('cancel', props.packageKey)
}

function repairConflict(): void {
  if (props.conflicted && !busy.value) emit('repair', props.packageKey)
}
</script>

<template>
  <!-- ~ 已安装市场资源 -->
  <!-- * 说明：复用资源卡片的图标、版本与启用态；包装层和主按钮共同保持 118px 方卡尺寸 -->
  <!-- * 交互：点击空白或内容打开详情，右键透传原始鼠标事件 -->
  <!-- * 依赖：版本来自 props.version；更新进度完全由父级快照驱动 -->
  <!-- * 边界：忙碌时阻止打开；失败说明仅在悬停提示中显示，不增加错误面板 -->
  <div
    class="market-resource-card relative aspect-square w-full min-w-0"
    :class="{'is-updating': updating}"
    @contextmenu.prevent="emit('contextmenu', $event)"
  >
    <ResourceCard
      class="market-resource-card__main w-full"
      :name="title"
      :meta="version"
      :status="status"
      :tone="conflicted && !updating ? 'conflict' : 'enabled'"
      :operating="busy"
      :aria-disabled="busy"
      :title="cardTitle"
      @click="openResource"
    >
      <Store :size="22" :stroke-width="1.7" aria-hidden="true" />
    </ResourceCard>

    <!-- ~ 卡片操作徽标 -->
    <!-- * 说明：操作按钮是主按钮的兄弟节点，升级入口常驻；更新、冲突和失败分别提供取消、修复和重试 -->
    <!-- * 交互：点击及键盘事件停止冒泡，保留原生 Enter / Space 激活 -->
    <!-- * 依赖：updateDisabled 仅禁用升级及重试；取消和修复只检查当前任务 -->
    <!-- * 边界：不嵌套 button，不触发详情，不在组件内部调用安装器 -->
    <div
      class="market-resource-card__actions absolute right-2 top-2 flex items-center gap-1"
      @click.stop
      @keydown.stop
      @keyup.stop
    >
      <button
        v-if="updating"
        type="button"
        class="market-resource-card__action"
        title="取消更新"
        aria-label="取消更新"
        :disabled="uninstalling"
        @click.stop="cancelUpdate"
      >
        <X :size="13" :stroke-width="1.8" aria-hidden="true" />
      </button>
      <button
        v-else-if="conflicted"
        type="button"
        class="market-resource-card__action is-warning"
        :title="['修复更新冲突', failureMessage].filter(Boolean).join('\n')"
        aria-label="修复更新冲突"
        :disabled="busy"
        @click.stop="repairConflict"
      >
        <Wrench :size="13" :stroke-width="1.8" aria-hidden="true" />
      </button>
      <button
        v-else-if="hasFailure && updateVersion"
        type="button"
        class="market-resource-card__action is-error"
        :title="[retryLabel, failureMessage].filter(Boolean).join('\n')"
        :aria-label="retryLabel"
        :disabled="upgradeDisabled"
        @click.stop="updateResource"
      >
        <RotateCw :size="13" :stroke-width="1.8" aria-hidden="true" />
      </button>
      <button
        v-if="updateVersion && !hasFailure && !conflicted && !updating"
        type="button"
        class="market-resource-card__action is-upgrade"
        :title="upgradeLabel"
        :aria-label="upgradeLabel"
        :disabled="upgradeDisabled"
        @click.stop="updateResource"
      >
        <ArrowUp :size="13" :stroke-width="1.8" aria-hidden="true" />
      </button>
    </div>

    <!-- ~ 当前更新进度 -->
    <!-- * 说明：窄进度条和阶段 / 百分比占用预留底部空间，完整阶段与速率由主卡片提示展示 -->
    <!-- * 交互：进度层不拦截主按钮点击；百分比固定宽度，长阶段省略 -->
    <!-- * 依赖：只在当前卡片 updating 时显示，数值归一到 0 至 100 -->
    <!-- * 边界：不添加动画，不随全局 updateDisabled 显示虚假进度 -->
    <div v-if="updating" class="market-resource-card__progress pointer-events-none absolute bottom-2.5 left-2.5 right-2.5">
      <div class="mb-1 flex min-w-0 items-center gap-1 text-[10px] leading-[14px]">
        <span class="min-w-0 flex-1 truncate">{{ stage }}</span>
        <span class="market-resource-card__percent shrink-0">{{ percent }}%</span>
      </div>
      <div
        class="market-resource-card__track h-[3px] overflow-hidden rounded-sm"
        role="progressbar"
        :aria-label="`${title} 更新进度`"
        :aria-valuenow="percent"
        :aria-valuetext="progressLabel"
        :aria-valuemin="0"
        :aria-valuemax="100"
      >
        <div class="market-resource-card__fill h-full" :style="{width: `${percent}%`}"></div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
// 说明：仅在本包装组件内压缩状态标签并为底部进度预留空间，不改变共享资源卡片。
.market-resource-card {
  :deep(.resource-card-meta) {
    gap: 4px;
  }

  :deep(.resource-card-status) {
    gap: 3px;
    padding: 1px 4px;
    font-size: 10px;
  }

  &.is-updating .market-resource-card__main {
    padding-bottom: 40px;
  }
}

.market-resource-card__action {
  display: grid;
  width: 22px;
  height: 22px;
  place-items: center;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-secondary);

  &.is-upgrade {
    color: var(--ds-brand);
  }

  &.is-warning {
    border-color: var(--ds-warning-border);
    color: var(--ds-warning);
  }

  &.is-error {
    border-color: var(--ds-error-border);
    color: var(--ds-error);
  }

  &:hover:not(:disabled) {
    border-color: var(--ds-border-strong);
    background: var(--ds-hover-bg);
  }

  &:focus-visible {
    outline: 2px solid var(--ds-focus-ring);
    outline-offset: 2px;
  }

  &:disabled {
    cursor: not-allowed;
    color: var(--ds-text-disabled);
  }
}

.market-resource-card__progress {
  color: var(--ds-text-secondary);
}

.market-resource-card__percent {
  min-width: 4ch;
  font-family: var(--ds-font-display);
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.market-resource-card__track {
  background: var(--ds-pressed-bg);
}

.market-resource-card__fill {
  background: var(--ds-brand);
}
</style>
