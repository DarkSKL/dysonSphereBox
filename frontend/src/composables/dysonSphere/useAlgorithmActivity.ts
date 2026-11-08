import {computed, shallowRef, type ShallowRef} from 'vue'
import type {AlgoVizProgress} from '../../packages/algorithm-viz/src'

// =============== 算法活动记录 ===============
// 说明：把可视化舞台的进度回调按时间片折成一列列「活动快照」，供蓝图预览栏的热力图渲染；
//       行为建筑、新增、替换、撤销、队列，列为时间片，越新越靠右。
// 边界：模块级共享状态（舞台与预览栏是兄弟组件）；只在有动作或队列时才产生新列，空闲时保留上一轮的形状；
//       累计计数回落（舞台重置、重新回放）视为新一轮，自动清空。

export type ActivityKey = 'objects' | 'added' | 'replaced' | 'removed' | 'pending'
export type ActivityColumn = Record<ActivityKey, number>
export interface ActivityRow {key: ActivityKey; label: string; tone: string; /** 累计量取每片增量，水平量取片内峰值 */ cumulative: boolean}

export const ACTIVITY_ROWS: readonly ActivityRow[] = [
  {key: 'objects', label: '建筑', tone: 'var(--ds-brand)', cumulative: false},
  {key: 'added', label: '新增', tone: 'var(--ds-success)', cumulative: true},
  {key: 'replaced', label: '替换', tone: 'var(--ds-info)', cumulative: true},
  {key: 'removed', label: '撤销', tone: 'var(--ds-warning)', cumulative: true},
  {key: 'pending', label: '队列', tone: 'var(--ds-text-secondary)', cumulative: false},
]

/** 单个时间片的毫秒数 */
const SLICE_MS = 250
/** 最多保留的列数（超出丢弃最早的） */
const MAX_COLUMNS = 240

const empty = (): ActivityColumn => ({objects: 0, added: 0, replaced: 0, removed: 0, pending: 0})

// 状态挂在 globalThis 上：开发期热更新会让本模块被加载出多个副本，舞台写入与热力图读取必须落在同一份状态上
interface ActivityStore {
  history: ShallowRef<ActivityColumn[]>
  current: ShallowRef<ActivityColumn | null>
  latest: ShallowRef<AlgoVizProgress | null>
  slice: number
}
const holder = globalThis as typeof globalThis & {__dsboxAlgorithmActivity?: ActivityStore}
const store: ActivityStore = holder.__dsboxAlgorithmActivity ??= {history: shallowRef([]), current: shallowRef(null), latest: shallowRef(null), slice: -1}
const {history, current, latest} = store

/** 记录一次舞台进度；舞台的进度回调调用 */
export const recordAlgorithmActivity = (progress: AlgoVizProgress): void => {
  const before = latest.value
  if (before && (progress.added < before.added || progress.replaced < before.replaced || progress.removed < before.removed)) {
    history.value = []
    current.value = null
    store.slice = -1
  }
  const base = before && progress.added >= before.added ? before : null
  latest.value = progress
  const delta = {
    added: progress.added - (base?.added ?? 0),
    replaced: progress.replaced - (base?.replaced ?? 0),
    removed: progress.removed - (base?.removed ?? 0),
  }
  const active = delta.added + delta.replaced + delta.removed > 0 || progress.pending > 0 || progress.animating > 0 || progress.falling > 0
  if (!active) return
  const index = Math.floor(performance.now() / SLICE_MS)
  let column = current.value
  if (!column || index !== store.slice) {
    if (column) history.value = [...history.value, column].slice(-MAX_COLUMNS)
    column = empty()
    store.slice = index
  }
  current.value = {
    objects: Math.max(column.objects, progress.objects),
    added: column.added + delta.added,
    replaced: column.replaced + delta.replaced,
    removed: column.removed + delta.removed,
    pending: Math.max(column.pending, progress.pending),
  }
}

/** 清空记录（舞台卸载时调用） */
export const resetAlgorithmActivity = (): void => {
  history.value = []
  current.value = null
  latest.value = null
  store.slice = -1
}

export const useAlgorithmActivity = () => ({
  columns: computed(() => (current.value ? [...history.value, current.value] : history.value)),
  latest,
})
