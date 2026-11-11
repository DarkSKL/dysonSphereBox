import {onBeforeUnmount, reactive, ref, watch} from 'vue'
import type {blueprint} from '../../../wailsjs/go/models'
import type {AlgorithmVizController, AlgoVizArea, AlgoVizBatch, AlgoVizBuilding, AlgoVizCoordinates, AlgoVizOperation} from '../../packages/algorithm-viz/src'
import {decodeFactoryBlueprint} from '../../pages/services/dysonSphere/factory'
import {observeFactoryGenerationEvents} from '../../pages/services/dysonSphere/factory-progress'
import type {FactoryPreviewArea, FactoryPreviewOperation} from '../../types/factory-progress'
import {isFactoryGenerationProgressEvent} from '../../utils/factory-preview-protocol'
import {createFrameCoalescer} from '../../utils/frame-coalescer'
import {ALGORITHM_REPLAY_CHUNK as CHUNK, prepareAlgorithmReplay, yieldAlgorithmReplay, type AlgorithmMirrorEntry} from '../../utils/algorithm-replay'
import type {QuantWorkspace} from './useQuantWorkspace'

// =============== 算法可视化数据源 ===============
// 说明：把生成过程与结果翻译为可视化包的场景操作（新增 / 替换 / 撤销），并维护一份场景镜像用于差量比较。
// 使用：算法可视化组件在 setup 中调用，场景创建后通过 getViz 取得控制器；组件卸载时自动退订事件。
// 数据来源：
//   1. 实时跟随：算法可视化可见时，工作区为生成请求开启 preview-delta，边计算边播放算法真实的放置、平移与回退，
//      画面最多落后算法约 1 秒；
//   2. 结果回放：打开前已生成的结果、导入的蓝图，或无法提供完整过程快照的模式，解码蓝图后与当前场景比较得到
//      撤销 / 替换 / 新增，首次显示按记录顺序（即算法装配顺序）逐个落位。
// 边界：只读旁路，不改变生成成败、任务过滤或工作区状态；解码失败只提示，不影响蓝图预览与导出。

// =============== 实时跟随开关 ===============
// 说明：模块级共享。算法可视化可见时 active 为 true，useQuantWorkspace 据此在生成请求中开启 emitPreviewDeltas；
//       不进入生成签名，切换不会触发重新生成。
// 边界：某个「模式 | 密度」组合以 GEN_E_PROGRESS_PREVIEW 失败时记入 unsupported，工作区改为普通生成重试一次，
//       之后该组合不再请求过程差量，避免可视化影响生成结果。
export const algorithmFollow = reactive({active: false, unsupported: new Set<string>()})

/** 实时跟随不可用组合的键 */
export const algorithmFollowKey = (mode: number, density: string): string => `${mode}|${density}`

/** 算法当前阶段：来自生成过程的阶段事件，实时显示，与播放进度无关 */
export interface AlgorithmPhase {
  readonly message: string
  readonly current: number
  readonly total: number
  readonly detail?: string
}

export type AlgorithmFeedSource = 'idle' | 'live' | 'replay'

const LIVE_STAGE_LABELS: Record<string, string> = {layout: '布局装配', assembly: '装配机器行', routing: '连接物料路线'}

const liveAreas = (areas?: readonly FactoryPreviewArea[]): AlgoVizArea[] | undefined => areas?.length
  ? areas.map(area => ({index: area.index, parentIndex: area.parentIndex, anchorX: area.anchorLocalOffset.x, anchorY: area.anchorLocalOffset.y, width: area.size.x, height: area.size.y}))
  : undefined

const documentAreas = (areas: blueprint.Area[] | undefined): AlgoVizArea[] => (areas ?? []).map(area => ({
  index: area.index, parentIndex: area.parentIndex, anchorX: area.anchorLocalOffsetX, anchorY: area.anchorLocalOffsetY, width: area.width, height: area.height,
}))

export const useAlgorithmFeed = (w: QuantWorkspace, getViz: () => AlgorithmVizController | null) => {
  const source = ref<AlgorithmFeedSource>('idle')
  const notice = ref('')
  const decoding = ref(false)
  const phase = ref<AlgorithmPhase | null>(null)
  // 阶段事件高频到达：每帧只写一次响应式状态，清空时先取消挂起的写入，避免旧阶段回写
  const phaseWriter = createFrameCoalescer<AlgorithmPhase>(value => { phase.value = value })
  const mirror = new Map<string, AlgorithmMirrorEntry>()
  let liveTask = ''
  let liveComplete = false
  let liveWinner = ''
  let liveV4 = false
  let streamNotice = ''
  const candidateSequences = new Map<string, number>()
  const snapshots = new Map<string, {revision: number; count: number; chunks: Map<number, readonly FactoryPreviewOperation[]>; areas?: readonly AlgoVizArea[]}>()
  let phaseStage = ''
  let phaseTask = ''
  let lastSourceKey = ''
  let decodeVersion = 0

  // =============== 镜像与发送 ===============
  // 说明：每批操作先写入镜像再入队，保证镜像始终等于场景最终状态。
  const commit = (operations: AlgoVizOperation[], options: Omit<AlgoVizBatch, 'operations'>): void => {
    if (!operations.length) return
    for (const op of operations) {
      if (op.type === 'remove') mirror.delete(op.id)
      else mirror.set(op.id, {building: op.building, index: op.building.index})
    }
    getViz()?.enqueue({...options, operations})
  }
  const clearScene = (animate: boolean): void => {
    // 大场景切换来源时直接复用实例池，避免给旧建筑再安排大量退场动画；当前蓝图的记录仍全部保留。
    const transition = animate && mirror.size <= CHUNK
    mirror.clear()
    getViz()?.reset({animate: transition})
  }

  // =============== 实时差量 ===============
  // 说明：新任务的第一批差量到达时清空舞台；对象 ID 带任务前缀，避免与其它任务冲突。
  const liveKey = (taskId: string, objectId: string): string => `t:${taskId}:${objectId}`
  const liveOperation = (taskId: string, op: FactoryPreviewOperation): AlgoVizOperation => {
    if (op.type === 'remove') return {type: 'remove', id: liveKey(taskId, op.objectId)}
    const b = op.building
    return {type: op.type, id: liveKey(taskId, op.objectId), building: {
      itemId: b.itemId, modelIndex: b.modelIndex, x: b.localOffset.x, y: b.localOffset.y, z: b.localOffset.z,
      x2: b.localOffset2.x, y2: b.localOffset2.y, z2: b.localOffset2.z, yaw: b.yaw, areaIndex: b.areaIndex,
      group: b.productionIndex, temporary: b.isTemporary,
      outputId: b.outputObjectId ? liveKey(taskId, b.outputObjectId) : undefined,
      index: b.index >= 0 ? b.index : undefined,
    }}
  }
  const liveStage = (stage: string, coordinates: AlgoVizCoordinates, operations: AlgoVizOperation[]): string => {
    if (coordinates === 'area') return '提交正式结果'
    if (stage === 'preview') return operations.every(op => op.type === 'remove') ? '回退候选方案' : '调整候选方案'
    return LIVE_STAGE_LABELS[stage] ?? '布局装配'
  }

  // =============== 生成事件 ===============
  // 说明：差量事件按跟随节奏转为场景操作；阶段事件实时更新 phase，未收到差量时同时显示为阶段标题。
  const onEvent = (payload: unknown): void => {
    if (!isFactoryGenerationProgressEvent(payload)) return
    const event = payload
    if (event.stage === 'preview-stream-end' && event.taskId === liveTask && liveV4 && !liveComplete) {
      liveComplete = true
      getViz()?.failSearch()
      notice.value = '本次计算未成功，继续演示候选退场过程'
      return
    }
    if (event.stage === 'preview-stream-incomplete') {
      liveTask = ''
      liveV4 = false
      liveComplete = false
      clearScene(false)
      notice.value = event.message
      streamNotice = event.message
      if (!w.generating) void replay()
      return
    }
    if (event.protocolVersion === 4) {
      if (event.kind === 'search-start') {
        clearScene(false)
        liveTask = event.taskId
        liveComplete = false
        liveWinner = ''
        liveV4 = true
        streamNotice = ''
        notice.value = ''
        candidateSequences.clear()
        snapshots.clear()
        decodeVersion++
        decoding.value = false
        source.value = 'live'
        return
      }
      if (event.taskId !== liveTask) return
      const viz = getViz()
      if (!viz) return
      const failStream = (message: string): void => {
        liveTask = ''
        liveV4 = false
        liveComplete = false
        clearScene(false)
        snapshots.clear()
        notice.value = message
        streamNotice = message
        if (!w.generating) void replay()
      }
      if (event.candidateId) {
        const previous = candidateSequences.get(event.candidateId) ?? 0
        if (event.candidateSequence !== previous + 1) { failStream('候选过程序号不连续，等待正式结果回放'); return }
        candidateSequences.set(event.candidateId, event.candidateSequence)
      }
      if (event.kind === 'candidate-plan' && event.candidates) {
        for (const candidate of event.candidates) viz.beginCandidate(candidate.id, candidate.ordinal)
      } else if (event.kind === 'candidate-start' && event.candidateId) {
        viz.beginCandidate(event.candidateId, event.candidateOrdinal ?? 1)
      } else if (event.kind === 'candidate-finish' && event.candidateId && event.candidateStatus) {
        viz.finishCandidate(event.candidateId, event.candidateStatus)
        if (event.candidateStatus !== 'valid') {
          const prefix = `t:${event.taskId}:${event.candidateId}:`
          for (const id of mirror.keys()) if (id.startsWith(prefix)) mirror.delete(id)
        }
      } else if (event.kind === 'preview-snapshot' && event.candidateId && event.operations) {
        const revision = event.snapshotRevision ?? 0
        const count = event.chunkCount ?? 0
        const snapshot = snapshots.get(event.candidateId) ?? {revision, count, chunks: new Map<number, readonly FactoryPreviewOperation[]>(), areas: liveAreas(event.areas)}
        const index = event.chunkIndex ?? 0
        if (snapshot.revision !== revision || snapshot.count !== count || snapshot.chunks.has(index)) { failStream('候选最终快照分块冲突，等待正式结果回放'); return }
        snapshot.chunks.set(index, event.operations)
        snapshots.set(event.candidateId, snapshot)
      } else if (event.kind === 'preview-delta' && event.candidateId && event.operations) {
        const coordinates: AlgoVizCoordinates = event.coordinateSystem === 'blueprint-area-local-v1' ? 'area' : 'layout'
        const operations = event.operations.map(op => liveOperation(`${event.taskId}:${event.candidateId}`, op))
        const changes = new Map<string, boolean>()
        for (const operation of operations) {
          const exists = changes.get(operation.id) ?? mirror.has(operation.id)
          if (operation.type === 'add' ? exists : !exists) { failStream('候选对象变化不连续，等待正式结果回放'); return }
          changes.set(operation.id, operation.type !== 'remove')
        }
        commit(operations, {candidateId: event.candidateId, coordinates, areas: liveAreas(event.areas), stage: LIVE_STAGE_LABELS[event.stage] ?? event.stage, pace: 'cinematic'})
      } else if (event.kind === 'preview-complete') {
        const candidateId = event.candidateId ?? ''
        const snapshot = snapshots.get(candidateId)
        if (!snapshot || snapshot.revision !== event.snapshotRevision || snapshot.chunks.size !== snapshot.count || snapshot.count !== event.chunkCount) {
          failStream('候选最终快照不完整，等待正式结果回放')
          return
        }
        const records: FactoryPreviewOperation[] = []
        for (let index = 0; index < snapshot.count; index++) records.push(...(snapshot.chunks.get(index) ?? []))
        if (records.length !== event.buildingCount || new Set(records.map(operation => operation.objectId)).size !== records.length) {
          failStream('候选最终快照建筑数量不一致，等待正式结果回放')
          return
        }
        const operations = records.map(operation => liveOperation(`${event.taskId}:${candidateId}`, operation))
        mirror.clear()
        commit(operations, {candidateId, coordinates: 'area', areas: snapshot.areas, pace: 'cinematic', snapshot: true})
        snapshots.clear()
        liveComplete = true
        liveWinner = event.winnerCandidateId ?? ''
        if (w.generated?.winnerCandidateId === liveWinner) viz.finalizeSearch(liveWinner)
      } else if (event.kind === 'search-failed') {
        liveComplete = true
        viz.failSearch()
        notice.value = '本次搜索未找到可用方案，继续演示各候选的失败过程'
      }
      return
    }
    if (event.kind === 'preview-delta' && event.operations?.length) {
      if (!getViz()) return
      if (liveTask !== event.taskId) {
        liveTask = event.taskId
        liveComplete = false
        decodeVersion++
        decoding.value = false
        lastSourceKey = 'gen'
        clearScene(true)
      }
      source.value = 'live'
      const coordinates: AlgoVizCoordinates = event.coordinateSystem === 'blueprint-area-local-v1' ? 'area' : 'layout'
      const operations = event.operations.map(op => liveOperation(event.taskId, op))
      commit(operations, {coordinates, areas: liveAreas(event.areas), stage: liveStage(event.stage, coordinates, operations), pace: 'follow'})
      return
    }
    if (event.kind === 'preview-complete') {
      if (event.taskId === liveTask) liveComplete = true
      return
    }
    if (event.kind !== undefined && event.kind !== 'phase') return
    if (!event.message || !w.generating) return
    phaseWriter.push({message: event.message, current: event.current, total: event.total, detail: event.detail})
    // 已收到本任务差量时，阶段标题由播放进度给出，避免与落后约 1 秒的画面不一致
    if (event.taskId === liveTask) return
    if (event.taskId !== phaseTask) {
      phaseTask = event.taskId
      phaseStage = ''
    }
    if (event.stage === phaseStage) return
    phaseStage = event.stage
    getViz()?.caption(event.message.replace(/^正在/, ''), event.total > 0 ? `${event.current} / ${event.total}` : '算法运行中')
  }

  // =============== 结果回放 ===============
  // 说明：解码当前蓝图，与镜像按记录下标比较；撤销先于替换，替换先于新增，新增按记录顺序（算法装配顺序）。
  // 边界：导入蓝图与生成结果互不比较，切换来源时先清空舞台；全部建筑分批准备和提交，旧任务不能覆盖新结果。
  const groupsOf = (document: blueprint.Document): Map<number, number> => {
    const groups = new Map<number, number>()
    // 生成结果的产线归属：bindings 记录的是建筑数组下标，生成器保证其等于记录下标
    const bindings = w.replay ? undefined : w.generated?.bindings
    if (bindings?.length) {
      for (const binding of bindings) for (const index of binding.buildingIndices ?? []) groups.set(index, binding.productionIndex)
      return groups
    }
    // 导入蓝图没有产线归属：按配方区分颜色，同一配方的机器同色
    const recipes = new Map<number, number>()
    for (const building of document.buildings ?? []) {
      if (building.recipeId <= 0) continue
      if (!recipes.has(building.recipeId)) recipes.set(building.recipeId, recipes.size)
      groups.set(building.index, recipes.get(building.recipeId) as number)
    }
    return groups
  }

  const replay = async (): Promise<void> => {
    const version = ++decodeVersion
    const code = w.code
    if (!getViz()) return
    if (!code) {
      decoding.value = false
      if (liveV4 && liveTask && !w.replay) return
      if (mirror.size) clearScene(true)
      source.value = 'idle'
      lastSourceKey = ''
      return
    }
    if (liveComplete && liveWinner && !w.replay) {
      if (w.generated?.winnerCandidateId === liveWinner) getViz()?.finalizeSearch(liveWinner)
      lastSourceKey = 'gen'
      decoding.value = false
      notice.value = ''
      return
    }
    if (liveV4 && liveTask && !w.replay) return
    // 实时跟随已完整提交本次结果：场景已是最终状态，无需再解码比较
    if (liveTask && liveComplete && !w.replay && mirror.size === (w.generated?.buildingCount ?? -1)) {
      lastSourceKey = 'gen'
      decoding.value = false
      notice.value = ''
      return
    }
    decoding.value = true
    notice.value = streamNotice
    let document: blueprint.Document
    try {
      document = await decodeFactoryBlueprint(code, false)
    } catch (caught) {
      if (version === decodeVersion) notice.value = `蓝图解码失败，无法回放：${caught instanceof Error ? caught.message : String(caught)}`
      if (version === decodeVersion) decoding.value = false
      return
    }
    const isCurrent = (): boolean => version === decodeVersion && code === w.code && getViz() !== null
    if (!isCurrent()) return
    const sourceKey = w.replay ? `import:${w.previewRevision}` : 'gen'
    if (sourceKey !== lastSourceKey) {
      if (mirror.size) clearScene(true)
      lastSourceKey = sourceKey
    }
    try {
      const buildings = document.buildings ?? []
      const groups = groupsOf(document)
      const diff = await prepareAlgorithmReplay(buildings, mirror, groups, isCurrent)
      if (!diff) return
      const {removes, replaces, adds} = diff
      const viz = getViz()
      if (!viz || removes.length + replaces.length + adds.length === 0) return
      if (source.value !== 'live' || !liveComplete) source.value = 'replay'
      const fresh = mirror.size === 0
      viz.caption(fresh ? (w.replay ? '回放导入的蓝图' : '按装配顺序回放生成结果') : '比较新旧方案', fresh ? `${adds.length} 个建筑` : `撤下 ${removes.length} · 替换 ${replaces.length} · 新增 ${adds.length}`)
      const areas = documentAreas(document.areas)
      const operations = [...removes, ...replaces, ...adds]
      const stage = fresh ? '逐个落位' : '应用方案差异'
      for (let i = 0; i < operations.length; i += CHUNK) {
        if (!isCurrent()) return
        commit(operations.slice(i, i + CHUNK), {coordinates: 'area', areas, stage: i === 0 ? stage : undefined, pace: 'replay'})
        await yieldAlgorithmReplay()
      }
    } finally { if (version === decodeVersion) decoding.value = false }
  }

  // =============== 生命周期 ===============
  // 说明：新结果或导入（previewRevision 变化）触发回放；每次生成开始时重置跟随状态；
  //       生成结束但实时差量未完成（失败、取消或改为普通生成重试）时，按结果恢复场景。
  const stop = observeFactoryGenerationEvents(onEvent, () => (getViz()?.getProgress().pending ?? 0) < 20000, () => w.generationTaskId)
  watch(() => w.generationTaskId, () => {
    if (getViz()?.getProgress().candidates) {
      clearScene(false)
      lastSourceKey = ''
    }
    liveTask = ''
    liveComplete = false
    liveV4 = false
    liveWinner = ''
    candidateSequences.clear()
    snapshots.clear()
    decodeVersion++
    decoding.value = false
  }, {flush: 'sync'})
  watch(() => [w.previewRevision, w.code] as const, () => { void replay() })
  watch(() => w.generating, generating => {
    phaseWriter.cancel()
    if (generating) {
      liveTask = ''
      liveComplete = false
      liveV4 = false
      phase.value = null
      return
    }
    phase.value = null
    if (liveV4 && liveTask) return
    if (liveTask && !liveComplete) {
      liveTask = ''
      lastSourceKey = ''
      void replay()
    }
  })
  onBeforeUnmount(() => { stop(); phaseWriter.cancel(); decodeVersion++ })

  /** 重新从当前结果完整回放：清空舞台后按装配顺序重建 */
  const rebuild = (): void => {
    clearScene(false)
    lastSourceKey = ''
    liveTask = ''
    liveComplete = false
    liveWinner = ''
    liveV4 = false
    streamNotice = ''
    void replay()
  }

  return {source, notice, decoding, phase, sync: replay, rebuild}
}
