<script setup lang="ts">
import {Activity, Copy, PanelRightClose, PanelRightOpen, Pause, Play, Trash2} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed, ref} from 'vue'
import {ClipboardSetText} from '../../../../wailsjs/runtime/runtime'
import {generationDebugOptions, useGenerationTimeline, type GenerationTimelineEntry} from '../../../composables/dysonSphere/useGenerationTimeline'

const message = useMessage()
const {entries, previewEntries, paused, selected, dropped, deltaCount, completeCount, clear} = useGenerationTimeline()
const collapsed = ref(false)
const previewOnly = ref(false)
const testActive = ref(false)
const props = defineProps<{canGenerate: boolean; generating: boolean}>()
const emit = defineEmits<{generate: []}>()

// =============== 差量采集入口 ===============
// 说明：为当前方案发起一次明确启用 preview-delta 的生成，避免仅打开开关却没有新请求。
// 边界：没有可用量化方案或正在生成时禁用；暂停的记录先恢复，确保测试事件可见。
const generateWithDeltas = (): void => {
  if (!props.canGenerate || props.generating) return
  generationDebugOptions.emitPreviewDeltas = true
  paused.value = false
  previewOnly.value = true
  testActive.value = true
  clear()
  emit('generate')
}

// =============== 卡片展示 ===============
// 说明：事件类型映射为中文标签与色调；同一任务的连续卡片只在首张展示任务分隔头。
const kindLabels: Record<string, string> = {'phase': '阶段', 'building-batch': '建筑分块', 'preview-delta': '预览差量', 'preview-complete': '预览完成'}
const kindLabel = (kind: string): string => kindLabels[kind] ?? kind
const shortTask = (taskId: string): string => taskId ? taskId.slice(0, 8) : '无任务 ID'
const clock = (time: number): string => {
  const date = new Date(time)
  const pad = (value: number, size = 2): string => String(value).padStart(size, '0')
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(), 3)}`
}
const operationCount = (entry: GenerationTimelineEntry): number => {
  const operations = (entry.payload as {operations?: unknown} | null)?.operations
  return Array.isArray(operations) ? operations.length : 0
}
const visibleEntries = computed(() => previewOnly.value ? previewEntries.value : entries.value)
const startsTask = (index: number): boolean => index === 0 || visibleEntries.value[index - 1].taskId !== visibleEntries.value[index].taskId

// =============== 事件详情弹窗 ===============
// 说明：点击卡片后展示原始载荷的格式化 JSON，可一键复制用于对照后端日志。
// 边界：载荷无法序列化时退回字符串显示，不影响时间流。
const detailVisible = computed({
  get: () => selected.value !== null,
  set: (value: boolean) => { if (!value) selected.value = null },
})
const detailJson = computed(() => {
  if (!selected.value) return ''
  try { return JSON.stringify(selected.value.payload, null, 2) } catch { return String(selected.value.payload) }
})
const copyDetail = async (): Promise<void> => {
  try {
    if (!await ClipboardSetText(detailJson.value)) throw new Error('clipboard')
    message.success('事件内容已复制')
  } catch { message.error('复制失败') }
}
</script>

<template>
  <!-- ~ 生成事件时间流 -->
  <!-- * 说明：工作台最右侧调试侧栏，按到达时间倒序列出 factory:generationProgress 全部事件卡片 -->
  <!-- * 交互：点击卡片弹窗查看原始载荷；可暂停追加、清空记录、折叠为窄条 -->
  <!-- * 依赖：useGenerationTimeline 旁路订阅 Wails 事件，不经过生成任务的任务 ID 过滤 -->
  <!-- * 边界：最多保留 500 条，超出丢弃最旧；未通过协议校验的事件以警示色标出 -->
  <aside class="timeline" :class="{'is-collapsed': collapsed}" aria-label="生成事件时间流">
    <header class="timeline-head">
      <n-tooltip placement="left"><template #trigger><n-button size="tiny" quaternary :aria-label="collapsed ? '展开事件时间流' : '折叠事件时间流'" @click="collapsed = !collapsed"><template #icon><PanelRightOpen v-if="collapsed" :size="14" /><PanelRightClose v-else :size="14" /></template></n-button></template>{{ collapsed ? '展开事件时间流' : '折叠事件时间流' }}</n-tooltip>
      <template v-if="!collapsed">
        <div class="head-title"><span>生成事件</span><small>差量 {{ deltaCount }} · 完成 {{ completeCount }} · 共 {{ entries.length }} 条<template v-if="dropped"> · 已丢弃 {{ dropped }}</template></small></div>
        <n-tooltip><template #trigger><n-button size="tiny" quaternary :type="paused ? 'warning' : 'default'" :aria-label="paused ? '继续记录' : '暂停记录'" @click="paused = !paused"><template #icon><Play v-if="paused" :size="13" /><Pause v-else :size="13" /></template></n-button></template>{{ paused ? '已暂停，点击继续记录' : '暂停记录新事件' }}</n-tooltip>
        <n-tooltip><template #trigger><n-button size="tiny" quaternary :disabled="!entries.length" aria-label="清空事件" @click="clear"><template #icon><Trash2 :size="13" /></template></n-button></template>清空事件</n-tooltip>
      </template>
    </header>

    <!-- 采集开关：控制下一次生成是否请求 preview-delta；默认关闭，避免大蓝图事件量与预览快照缺失导致的生成失败 -->
    <label v-if="!collapsed" class="timeline-option">
      <n-switch v-model:value="generationDebugOptions.emitPreviewDeltas" size="small" />
      <span class="option-text"><span>采集预览差量</span><small>下次生成生效 · 事件量大，可能使生成失败</small></span>
    </label>
    <div v-if="!collapsed" class="timeline-test">
      <n-button size="small" secondary block :loading="generating" :disabled="!canGenerate || generating" @click="generateWithDeltas">重新生成并采集差量</n-button>
      <n-button size="tiny" quaternary block :type="previewOnly ? 'primary' : 'default'" @click="previewOnly = !previewOnly">{{ previewOnly ? '只看预览事件 · 显示全部事件' : '只看预览事件' }}</n-button>
      <small v-if="!canGenerate">请先完成量化计算</small>
      <small v-else-if="testActive && !deltaCount && entries.length && !generating">本次未收到 preview-delta，可切换查看全部事件定位阶段</small>
    </div>

    <!-- 折叠态：只保留计数，点击展开 -->
    <button v-if="collapsed" type="button" class="rail" :title="`${entries.length} 条生成事件`" @click="collapsed = false"><Activity :size="14" /><span>{{ entries.length }}</span></button>

    <div v-else class="timeline-body">
      <p v-if="!visibleEntries.length" class="timeline-empty">{{ paused ? '记录已暂停' : previewOnly ? '等待 preview-delta 事件…' : '等待蓝图生成事件…' }}</p>
      <template v-for="(entry, index) in visibleEntries" :key="entry.id">
        <div v-if="startsTask(index)" class="task-head type-mono" :title="entry.taskId">任务 {{ shortTask(entry.taskId) }}</div>
        <button type="button" class="event-card" :class="[`kind-${entry.kind}`, {'is-invalid': !entry.valid}]" @click="selected = entry">
          <i class="event-dot" aria-hidden="true" />
          <span class="event-meta"><span class="event-kind">{{ kindLabel(entry.kind) }}</span><span class="type-mono">{{ entry.stage }}</span><span class="event-time type-mono">{{ clock(entry.receivedAt) }}</span></span>
          <span class="event-message">{{ entry.message || '（无消息）' }}</span>
          <span class="event-extra type-mono">
            <span v-if="entry.sequence !== null">#{{ entry.sequence }}</span>
            <span v-if="entry.total > 0">{{ entry.current }}/{{ entry.total }}</span>
            <span v-if="operationCount(entry)">{{ operationCount(entry) }} ops</span>
            <span v-if="!entry.valid" class="event-invalid">协议校验未通过</span>
          </span>
        </button>
      </template>
    </div>

    <n-modal v-model:show="detailVisible" preset="card" :title="selected ? `${kindLabel(selected.kind)} · ${selected.stage || '未知阶段'}` : '事件详情'" style="width: min(720px, 92vw)">
      <div v-if="selected" class="detail">
        <dl class="detail-meta">
          <dt>任务</dt><dd class="type-mono">{{ selected.taskId || '—' }}</dd>
          <dt>接收时间</dt><dd class="type-mono">{{ clock(selected.receivedAt) }}</dd>
          <dt>序号</dt><dd class="type-mono">{{ selected.sequence ?? '—' }}</dd>
          <dt>协议校验</dt><dd :class="selected.valid ? 'is-ok' : 'is-bad'">{{ selected.valid ? '通过' : '未通过（生成流程会忽略该事件）' }}</dd>
          <dt>消息</dt><dd>{{ selected.message || '—' }}</dd>
        </dl>
        <div class="detail-bar"><span>原始载荷</span><n-button size="tiny" quaternary @click="copyDetail"><template #icon><Copy :size="13" /></template>复制 JSON</n-button></div>
        <pre class="detail-json type-mono">{{ detailJson }}</pre>
      </div>
    </n-modal>
  </aside>
</template>

<style scoped lang="scss">
// 外框由宿主页面提供，组件只负责内部布局。
.timeline { display: flex; width: clamp(240px, 18vw, 300px); height: 100%; min-height: 0; flex-direction: column; color: var(--ds-text-primary); }
.timeline.is-collapsed { width: 36px; }
.timeline-head { display: flex; min-height: 40px; flex-shrink: 0; align-items: center; gap: 4px; border-bottom: 1px solid var(--ds-border-subtle); padding: 6px 6px; font-size: 13px; font-weight: 600; }
.is-collapsed .timeline-head { justify-content: center; padding: 6px 0; }
.head-title { display: flex; min-width: 0; flex: 1; flex-direction: column; overflow: hidden; white-space: nowrap; }
.head-title small { overflow: hidden; color: var(--ds-text-muted); font-size: 10px; font-weight: 400; text-overflow: ellipsis; }
.timeline-option { display: flex; flex-shrink: 0; align-items: center; gap: 8px; border-bottom: 1px solid var(--ds-border-subtle); padding: 6px 10px; font-size: 12px; cursor: pointer; }
.option-text { display: flex; min-width: 0; flex-direction: column; }
.option-text small { color: var(--ds-text-muted); font-size: 10px; }
.timeline-test { display: flex; flex-shrink: 0; flex-direction: column; gap: 4px; border-bottom: 1px solid var(--ds-border-subtle); padding: 7px 10px; }
.timeline-test small { color: var(--ds-text-muted); font-size: 10px; }
.rail { display: flex; flex-direction: column; align-items: center; gap: 4px; border: 0; padding: 10px 0; background: transparent; color: var(--ds-text-secondary); font-size: 11px; cursor: pointer; }
.rail:hover { color: var(--ds-text-primary); }
.timeline-body { position: relative; display: flex; min-height: 0; flex: 1; flex-direction: column; gap: 4px; overflow-y: auto; padding: 8px 10px 16px; }
.timeline-empty { margin: 24px 0 0; color: var(--ds-text-muted); font-size: 12px; text-align: center; }
.task-head { flex: 0 0 auto; margin: 8px 0 2px; color: var(--ds-text-muted); font-size: 10px; letter-spacing: .04em; }
.task-head:first-child { margin-top: 0; }
// 时间线：左侧竖线串联同一任务的卡片，圆点颜色区分事件类型。
.event-card { --tone: var(--ds-text-muted); position: relative; display: flex; flex: 0 0 auto; width: 100%; flex-direction: column; gap: 2px; border: 1px solid var(--ds-border-subtle); border-radius: 2px; margin-left: 10px; max-width: calc(100% - 10px); padding: 6px 8px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 11px; text-align: left; cursor: pointer; transition: border-color 150ms, background-color 150ms; }
.event-card::before { content: ''; position: absolute; top: -5px; bottom: -5px; left: -8px; width: 1px; background: var(--ds-border-default); }
.event-card:hover, .event-card:focus-visible { border-color: var(--tone); background: var(--ds-hover-bg); outline: none; }
.event-dot { position: absolute; top: 10px; left: -11px; width: 7px; height: 7px; border-radius: 50%; background: var(--tone); }
.event-card.kind-phase { --tone: var(--ds-brand); }
.event-card.kind-building-batch { --tone: var(--ds-info); }
.event-card.kind-preview-delta { --tone: var(--ds-text-muted); }
.event-card.kind-preview-complete { --tone: var(--ds-success); }
.event-card.is-invalid { --tone: var(--ds-warning); border-color: var(--ds-warning-border); }
.event-meta { display: flex; min-width: 0; align-items: center; gap: 6px; white-space: nowrap; }
.event-kind { color: var(--tone); font-weight: 600; }
.event-time { margin-left: auto; color: var(--ds-text-muted); font-size: 10px; }
.event-message { color: var(--ds-text-primary); font-size: 12px; line-height: 1.45; overflow-wrap: anywhere; white-space: normal; }
.event-extra { display: flex; flex-wrap: wrap; gap: 8px; color: var(--ds-text-muted); font-size: 10px; }
.event-extra:empty { display: none; }
.event-invalid { color: var(--ds-warning); }
.detail-meta { display: grid; grid-template-columns: 72px 1fr; gap: 4px 12px; margin: 0 0 12px; font-size: 12px; }
.detail-meta dt { color: var(--ds-text-muted); }
.detail-meta dd { margin: 0; overflow-wrap: anywhere; }
.detail-meta .is-ok { color: var(--ds-success); }
.detail-meta .is-bad { color: var(--ds-warning); }
.detail-bar { display: flex; align-items: center; justify-content: space-between; color: var(--ds-text-muted); font-size: 12px; }
.detail-json { max-height: 56vh; overflow: auto; margin: 6px 0 0; border: 1px solid var(--ds-border-subtle); border-radius: 2px; padding: 10px 12px; background: var(--ds-hover-bg); font-size: 11px; line-height: 1.55; white-space: pre; }
@media (prefers-reduced-motion: reduce) { .event-card { transition: none; } }
</style>
