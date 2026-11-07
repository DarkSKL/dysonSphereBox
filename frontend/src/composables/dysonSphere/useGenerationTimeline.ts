import {computed, onBeforeUnmount, onMounted, reactive, ref, shallowRef} from 'vue'
import {observeRawFactoryGenerationEvents} from '../../pages/services/dysonSphere/factory-progress'
import {isFactoryGenerationProgressEvent} from '../../utils/factory-preview-protocol'

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
// 边界：开启差量后后端要求获选候选带完整预览快照，缺失时生成会以 GEN_E_PROGRESS_PREVIEW 失败，且事件量大；
//       调试开关保留该失败以便定位，可视化跟随则由工作区改为普通生成重试。
export const generationDebugOptions = reactive({debugMode: false, emitPreviewDeltas: false})

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
  const entries = shallowRef<GenerationTimelineEntry[]>([])
  const previewEntries = shallowRef<GenerationTimelineEntry[]>([])
  const paused = ref(false)
  const selected = ref<GenerationTimelineEntry | null>(null)
  const dropped = ref(0)
  const deltaCount = ref(0)
  const completeCount = ref(0)
  let nextId = 0
  let stop: (() => void) | undefined

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
      if (entry.kind === 'preview-delta') deltaCount.value++
      else completeCount.value++
      previewEntries.value = [entry, ...previewEntries.value].slice(0, TIMELINE_LIMIT)
    }
    const next = [entry, ...entries.value]
    if (next.length > TIMELINE_LIMIT) dropped.value += next.length - TIMELINE_LIMIT
    entries.value = next.slice(0, TIMELINE_LIMIT)
  }
  const clear = (): void => { entries.value = []; previewEntries.value = []; dropped.value = 0; deltaCount.value = 0; completeCount.value = 0; selected.value = null }
  const taskCount = computed(() => new Set(entries.value.map(entry => entry.taskId)).size)

  onMounted(() => { stop = observeRawFactoryGenerationEvents(record) })
  onBeforeUnmount(() => { stop?.(); stop = undefined })
  return {entries, previewEntries, paused, selected, dropped, deltaCount, completeCount, taskCount, clear}
}
