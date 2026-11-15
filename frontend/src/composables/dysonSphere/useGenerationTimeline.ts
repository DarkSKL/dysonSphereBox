import {useMessage} from 'naive-ui'
import {computed, onBeforeUnmount, onMounted, reactive, ref, shallowRef} from 'vue'
import {observeRawFactoryGenerationEvents} from '../../pages/services/dysonSphere/factory-progress'
import {downloadJson} from '../../utils/download-json'
import {isFactoryGenerationProgressEvent} from '../../utils/factory-preview-protocol'
import {createFrameCoalescer} from '../../utils/frame-coalescer'

// =============== 时间流条目 ===============
// 说明：每条 Wails 生成事件归档为一张只读卡片；原始载荷原样保留，供调试弹窗查看。
// 边界：只记录，不参与生成任务过滤、预览渲染或错误处理。
export interface GenerationTimelineEntry {
  readonly id: number
  readonly receivedAt: number
  readonly taskId: string
  readonly kind: string
  readonly stage: string
  readonly message: string
  readonly current: number
  readonly total: number
  readonly sequence: number | null
  /** 是否通过前端协议校验；未通过的事件不会被生成流程消费，调试时需要显眼标记。 */
  readonly valid: boolean
  readonly payload: unknown
}

// =============== 生成调试选项 ===============
// 说明：模块级共享状态，时间流侧栏写入，工作区在每次提交生成请求时读取。
// 使用：debugMode 由检查面板切换，决定是否显示生成事件时间流；emitPreviewDeltas 对应后端同名请求字段，
//       调试模式下按此开关随请求发送（算法可视化跟随时另由 useAlgorithmFeed.algorithmFollow 开启）；
//       两者默认关闭，只影响下一次生成，不进入生成签名，切换不会触发重新生成。
// 边界：开启差量后请求 v4，蓝图转发层只渲染候选1；无法提供完整预览快照的组合会记录并自动降级为普通生成，
//       不会因预览导致生成失败，但事件量大，调试面板会丢弃超过上限的旧条目。
export const generationDebugOptions = reactive({debugMode: true, emitPreviewDeltas: true})

/** 时间流保留上限；预览差量事件频率高，超过后丢弃最旧条目，避免调试面板拖慢页面。 */
const TIMELINE_LIMIT = 500

const field = (payload: unknown, key: string): unknown => (
  payload !== null && typeof payload === 'object' ? (payload as Record<string, unknown>)[key] : undefined
)
const text = (value: unknown): string => typeof value === 'string' ? value : ''
const integer = (value: unknown): number => typeof value === 'number' && Number.isFinite(value) ? value : 0

// =============== 生成事件时间流 ===============
// 说明：挂载时旁路订阅 factory:generationProgress，按到达顺序记录全部任务的事件，最新在前。
// 使用：调试侧栏调用；暂停只停止追加，不取消订阅；KeepAlive 停用期间仍继续记录。
// 边界：无 Wails 运行时（纯 Vite 预览）时静默为空列表。
export const useGenerationTimeline = () => {
  const message = useMessage()
  const entries = shallowRef<GenerationTimelineEntry[]>([])
  const previewEntries = shallowRef<GenerationTimelineEntry[]>([])
  const paused = ref(false)
  const selected = ref<GenerationTimelineEntry | null>(null)
  const dropped = ref(0)
  const deltaCount = ref(0)
  const completeCount = ref(0)
  let nextId = 0
  let stop: (() => void) | undefined
  /** 按任务暂存差量原文；预览完成时合并成一份样例并自动下载。 */
  const pendingDeltas = new Map<string, unknown[]>()

  const saveTaskDeltas = (taskId: string, items: unknown[]): void => {
    try {
      downloadJson(items, `preview-delta-${taskId.slice(0, 8) || 'task'}-${Date.now()}.json`)
      message.success(`已自动保存 ${items.length} 条预览差量`)
    } catch { message.warning('预览差量自动保存失败，可在事件详情中手动保存') }
  }
  // 高频原始事件先写有界缓冲；响应式列表每帧发布一次，仍保留最新500项和完整计数。
  let pending: GenerationTimelineEntry[] = []
  let pendingPreviews: GenerationTimelineEntry[] = []
  let received = 0
  let deltas = 0
  let completes = 0
  const writer = createFrameCoalescer<boolean>(() => {
    dropped.value += Math.max(0, entries.value.length + received - TIMELINE_LIMIT)
    entries.value = [...pending.slice(-TIMELINE_LIMIT).reverse(), ...entries.value].slice(0, TIMELINE_LIMIT)
    if (pendingPreviews.length) previewEntries.value = [...pendingPreviews.slice(-TIMELINE_LIMIT).reverse(), ...previewEntries.value].slice(0, TIMELINE_LIMIT)
    deltaCount.value += deltas
    completeCount.value += completes
    pending = []
    pendingPreviews = []
    received = deltas = completes = 0
  })

  const record = (payload: unknown): void => {
    if (paused.value) return
    const sequence = field(payload, 'sequence')
    const entry: GenerationTimelineEntry = {
      id: ++nextId,
      receivedAt: Date.now(),
      taskId: text(field(payload, 'taskId')),
      kind: text(field(payload, 'kind')) || 'phase',
      stage: text(field(payload, 'stage')),
      message: text(field(payload, 'message')),
      current: integer(field(payload, 'current')),
      total: integer(field(payload, 'total')),
      sequence: typeof sequence === 'number' ? sequence : null,
      valid: isFactoryGenerationProgressEvent(payload),
      payload,
    }
    if (entry.kind === 'preview-delta' || entry.kind === 'preview-complete') {
      const taskKey = entry.taskId || 'unknown'
      if (entry.kind === 'preview-delta') {
        deltas++
        const list = pendingDeltas.get(taskKey) ?? []
        list.push(payload)
        if (list.length > TIMELINE_LIMIT) list.shift()
        pendingDeltas.set(taskKey, list)
      } else {
        completes++
        // 预览结束即自动落盘；同一任务只保存一次，不逐条触发下载。
        const list = pendingDeltas.get(taskKey)
        pendingDeltas.delete(taskKey)
        if (list?.length) saveTaskDeltas(taskKey, list)
      }
      pendingPreviews.push(entry)
      if (pendingPreviews.length > TIMELINE_LIMIT * 2) pendingPreviews = pendingPreviews.slice(-TIMELINE_LIMIT)
    }
    pending.push(entry)
    received++
    if (pending.length > TIMELINE_LIMIT * 2) pending = pending.slice(-TIMELINE_LIMIT)
    writer.push(true)
  }
  const clear = (): void => {
    writer.cancel()
    pending = []; pendingPreviews = []; received = deltas = completes = 0
    entries.value = []; previewEntries.value = []; dropped.value = 0; deltaCount.value = 0; completeCount.value = 0; selected.value = null
    pendingDeltas.clear()
  }
  const taskCount = computed(() => new Set(entries.value.map(entry => entry.taskId)).size)

  onMounted(() => { stop = observeRawFactoryGenerationEvents(record) })
  onBeforeUnmount(() => { stop?.(); stop = undefined; writer.cancel(); pending = []; pendingPreviews = [] })
  return {entries, previewEntries, paused, selected, dropped, deltaCount, completeCount, taskCount, clear}
}
