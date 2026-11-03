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
const isOptionalIdentity = (value: unknown): boolean => value === undefined || typeof value === 'string'
const isPreviewBuilding = (value: unknown): boolean => {
  if (!isRecord(value)) return false
  if (!isIntegerAtLeast(value.index, -1) || !isIntegerAtLeast(value.itemId, 1)
    || !isIntegerAtLeast(value.modelIndex, 1) || !isIntegerAtLeast(value.areaIndex, 0)
    || !isIntegerAtLeast(value.productionIndex, -1) || !isPoint(value.localOffset)
    || !isPoint(value.localOffset2)) return false
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

const hasAreaFrame = (event: Record<string, unknown>): boolean => {
  if (event.coordinateSystem === 'layout-grid-v1') return event.areas === undefined
    || (Array.isArray(event.areas) && event.areas.length === 0)
  if (event.coordinateSystem !== 'blueprint-area-local-v1' || !Array.isArray(event.areas)
    || event.areas.length < 1 || event.areas.length > 64) return false
  const primary = event.primaryAreaIndex ?? 0
  return isIntegerAtLeast(primary, 0) && event.areas.every(area => (
    isRecord(area) && isIntegerAtLeast(area.index, 0) && isIntegerAtLeast(area.parentIndex, -1)
    && ['tropicAnchor', 'areaSegments', 'anchorLocalOffsetX', 'anchorLocalOffsetY', 'width', 'height']
      .every(key => typeof area[key] === 'number' && Number.isSafeInteger(area[key]))
  )) && event.areas.some(area => isRecord(area) && area.index === primary)
}

const hasCompletionCounts = (event: Record<string, unknown>): boolean => (
  isIntegerAtLeast(event.buildingCount, 0) && isIntegerAtLeast(event.batchCount, 0)
)

/** 职责：校验任务级阶段、旧分块与新差量事件；使用：Wails 事件入口；边界：只接受已知版本，不理解或转发算法失败原因。 */
export const isFactoryGenerationProgressEvent = (value: unknown): value is FactoryGenerationProgressEvent => {
  if (!isRecord(value) || typeof value.taskId !== 'string' || !value.taskId
    || typeof value.stage !== 'string' || typeof value.message !== 'string'
    || !isIntegerAtLeast(value.current, 0) || !isIntegerAtLeast(value.total, 0)
    || value.total < value.current) return false
  if (value.protocolVersion === undefined && value.kind === undefined) return true
  if (!isIntegerAtLeast(value.sequence, 1)) return false
  if (value.protocolVersion === 2) {
    if (value.kind === 'preview-delta') return hasAreaFrame(value)
      && Array.isArray(value.operations) && value.operations.length > 0
      && value.operations.length <= 128 && value.operations.every(isFactoryPreviewOperation)
    return value.kind === 'preview-complete'
      && value.coordinateSystem === 'blueprint-area-local-v1' && hasCompletionCounts(value)
  }
  if (value.protocolVersion !== 1) return false
  if (value.kind === 'phase') return true
  if (value.coordinateSystem !== 'blueprint-area-local-v1') return false
  if (value.kind === 'preview-complete') return hasCompletionCounts(value)
  if (value.kind !== 'building-batch' || !isRecord(value.previewBatch)) return false
  const batch = value.previewBatch
  return isRecord(batch.module) && typeof batch.module.id === 'string'
    && typeof batch.module.name === 'string' && isIntegerAtLeast(batch.module.productionIndex, -1)
    && isIntegerAtLeast(batch.batchIndex, 0) && isIntegerAtLeast(batch.batchCount, 1)
    && batch.batchCount > batch.batchIndex
    && Array.isArray(batch.buildings) && batch.buildings.length > 0 && Array.isArray(batch.areas)
}
