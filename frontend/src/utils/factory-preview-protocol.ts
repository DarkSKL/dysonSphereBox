import type {FactoryGenerationProgressEvent, FactoryPreviewOperation} from '../types/factory-progress'

// =============== 生成事件输入边界 ===============
// 说明：只校验传输与渲染字段，不判断寻路、回退或模式规则；未知协议不会进入场景。
const isRecord = (value: unknown): value is Record<string, unknown> => (
  value !== null && typeof value === 'object' && !Array.isArray(value)
)
const isIntegerAtLeast = (value: unknown, minimum: number): value is number => (
  typeof value === 'number' && Number.isSafeInteger(value) && value >= minimum
)
const isPoint = (value: unknown): boolean => (
  isRecord(value) && ['x', 'y', 'z'].every(key => (
    typeof value[key] === 'number' && Number.isFinite(value[key])
  ))
)
const isIntegerPair = (value: unknown): boolean => (
  isRecord(value) && ['x', 'y'].every(axis => (
    typeof value[axis] === 'number' && Number.isSafeInteger(value[axis])
  ))
)
const isOptionalIdentity = (value: unknown): boolean => value === undefined || typeof value === 'string'
const isPreviewBuilding = (value: unknown): boolean => {
  if (!isRecord(value)) return false
  if (!isIntegerAtLeast(value.index, -1) || !isIntegerAtLeast(value.itemId, 1)
    || !isIntegerAtLeast(value.modelIndex, 1) || !isIntegerAtLeast(value.areaIndex, 0)
    || !isIntegerAtLeast(value.productionIndex, -1) || typeof value.isTemporary !== 'boolean'
    || !isPoint(value.localOffset)
    || !isPoint(value.localOffset2)) return false
  if (value.isTemporary ? value.index !== -1 || value.areaIndex !== 0 : value.index < 0) return false
  return ['yaw', 'yaw2', 'tilt', 'tilt2', 'pitch', 'pitch2'].every(key => (
    typeof value[key] === 'number' && Number.isFinite(value[key])
  )) && isIntegerAtLeast(value.inputFromSlot, -1) && isIntegerAtLeast(value.outputToSlot, -1)
    && isOptionalIdentity(value.inputObjectId) && isOptionalIdentity(value.outputObjectId)
}

/** 职责：识别单个渲染操作；使用：事件校验与协议测试；边界：对象身份不能为空，撤销不允许夹带建筑。 */
export const isFactoryPreviewOperation = (value: unknown): value is FactoryPreviewOperation => {
  if (!isRecord(value) || typeof value.objectId !== 'string' || !value.objectId) return false
  if (value.type === 'remove') return value.building === undefined
  return (value.type === 'add' || value.type === 'replace') && isPreviewBuilding(value.building)
}

/** 职责：识别嵌套区域元数据；使用：差量与最终分块共用校验；边界：不接受旧的扁平区域字段作为替代。 */
const isPreviewArea = (value: unknown): boolean => (
  isRecord(value) && isIntegerAtLeast(value.index, 0) && isIntegerAtLeast(value.parentIndex, -1)
  && ['tropicAnchor', 'areaSegments'].every(key => (
    typeof value[key] === 'number' && Number.isSafeInteger(value[key])
  )) && isIntegerPair(value.anchorLocalOffset) && isIntegerPair(value.size)
)

const hasAreaFrame = (event: Record<string, unknown>): boolean => {
  if (event.coordinateSystem === 'layout-grid-v1') return event.areas === undefined
    || (Array.isArray(event.areas) && event.areas.length === 0)
  if (event.coordinateSystem !== 'blueprint-area-local-v1' || !Array.isArray(event.areas)
    || event.areas.length < 1 || event.areas.length > 64) return false
  const primary = event.primaryAreaIndex ?? 0
  return isIntegerAtLeast(primary, 0) && event.areas.every(isPreviewArea)
    && event.areas.some(area => isRecord(area) && area.index === primary)
}

const hasCompletionCounts = (event: Record<string, unknown>): boolean => (
  isIntegerAtLeast(event.buildingCount, 0) && isIntegerAtLeast(event.batchCount, 0)
)

/** 职责：校验 v3 最终建筑分块；使用：预览事件入口；边界：正式记录必须显式携带 false，区域采用嵌套结构。 */
const hasBuildingBatch = (value: unknown): boolean => {
  if (!isRecord(value) || !isRecord(value.module)) return false
  if (typeof value.module.id !== 'string' || typeof value.module.name !== 'string'
    || !isIntegerAtLeast(value.module.productionIndex, -1)
    || !isIntegerAtLeast(value.batchIndex, 0) || !isIntegerAtLeast(value.batchCount, 1)
    || value.batchCount <= value.batchIndex || !Array.isArray(value.buildings)
    || value.buildings.length === 0 || !Array.isArray(value.areas)
    || value.areas.length < 1 || value.areas.length > 64) return false
  return value.buildings.every(building => isRecord(building) && building.isTemporary === false
    && isIntegerAtLeast(building.index, 0)) && value.areas.every(isPreviewArea)
}

/** 职责：校验任务级阶段、旧分块与新差量事件；使用：Wails 事件入口；边界：只接受已知版本，不理解或转发算法失败原因。 */
export const isFactoryGenerationProgressEvent = (value: unknown): value is FactoryGenerationProgressEvent => {
  if (!isRecord(value) || typeof value.taskId !== 'string' || !value.taskId
    || typeof value.stage !== 'string' || typeof value.message !== 'string'
    || !isIntegerAtLeast(value.current, 0) || !isIntegerAtLeast(value.total, 0)
    || value.total < value.current) return false
  if (value.protocolVersion === undefined && value.kind === undefined) return true
  if (!isIntegerAtLeast(value.sequence, 1)) return false
  if (value.protocolVersion === 4) {
    if (value.kind === 'candidate-plan') return Array.isArray(value.candidates) && value.candidates.every(candidate => isRecord(candidate) && typeof candidate.id === 'string' && candidate.id.length > 0 && isIntegerAtLeast(candidate.ordinal, 1))
    if (value.kind === 'search-start' || value.kind === 'search-failed') return true
    if (typeof value.candidateId !== 'string' || !value.candidateId) return false
    if (!isIntegerAtLeast(value.candidateSequence, 1)) return false
    if (value.kind === 'candidate-start') return isIntegerAtLeast(value.candidateOrdinal, 1)
    if (value.kind === 'candidate-finish') return ['valid', 'rejected', 'pruned'].includes(String(value.candidateStatus))
    if (value.kind === 'preview-snapshot') return value.coordinateSystem === 'blueprint-area-local-v1'
      && hasAreaFrame(value) && isIntegerAtLeast(value.snapshotRevision, 1)
      && isIntegerAtLeast(value.chunkCount, 1) && isIntegerAtLeast(value.chunkIndex ?? 0, 0)
      && Number(value.chunkIndex ?? 0) < value.chunkCount
      && Array.isArray(value.operations) && value.operations.length > 0 && value.operations.length <= 128
      && value.operations.every(operation => isFactoryPreviewOperation(operation) && operation.type === 'add' && !operation.building.isTemporary)
    if (value.kind === 'preview-complete' && (typeof value.winnerCandidateId !== 'string' || !value.winnerCandidateId)) return false
  }
  if (value.protocolVersion === 3 || value.protocolVersion === 4) {
    if (value.kind === 'preview-delta') return hasAreaFrame(value)
      && Array.isArray(value.operations) && value.operations.length > 0
      && value.operations.length <= 128 && value.operations.every(operation => (
        isFactoryPreviewOperation(operation) && (operation.type === 'remove'
          || operation.building.isTemporary === (value.coordinateSystem === 'layout-grid-v1'))
      ))
    if (value.kind === 'building-batch') return value.coordinateSystem === 'blueprint-area-local-v1'
      && hasBuildingBatch(value.previewBatch)
    return value.kind === 'preview-complete'
      && value.coordinateSystem === 'blueprint-area-local-v1' && hasCompletionCounts(value)
  }
  if (value.protocolVersion !== 1) return false
  if (value.kind === 'phase') return true
  return value.kind === 'preview-complete' && value.coordinateSystem === 'blueprint-area-local-v1'
    && hasCompletionCounts(value)
}
