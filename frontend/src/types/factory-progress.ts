import type {blueprint} from '../../wailsjs/go/models'

/** 职责：描述真实生成阶段；使用：桥接回调与工作区状态；边界：序号表示当前处理项，不是整体完成百分比。 */
export interface FactoryGenerationProgress {
  readonly taskId?: string
  readonly stage: string
  readonly message: string
  readonly current: number
  readonly total: number
  readonly protocolVersion?: number
  readonly kind?: 'phase' | 'building-batch' | 'preview-delta' | 'preview-snapshot' | 'preview-complete' | 'search-start' | 'search-failed' | 'candidate-plan' | 'candidate-start' | 'candidate-finish'
  readonly candidateSequence?: number
  readonly snapshotRevision?: number
  readonly chunkIndex?: number
  readonly chunkCount?: number
  readonly candidates?: readonly {readonly id: string; readonly ordinal: number}[]
  readonly candidateId?: string
  readonly candidateOrdinal?: number
  readonly candidateStatus?: 'valid' | 'rejected' | 'pruned'
  readonly winnerCandidateId?: string
  readonly sequence?: number
  readonly coordinateSystem?: 'blueprint-area-local-v1' | 'layout-grid-v1'
  readonly previewBatch?: FactoryBuildingBatch
  readonly buildingCount?: number
  readonly batchCount?: number
  readonly operations?: readonly FactoryPreviewOperation[]
  readonly areas?: readonly FactoryPreviewArea[]
  readonly primaryAreaIndex?: number
}

/** 职责：提供动画所需的建筑姿态；使用：临时和最终场景共用；边界：不含蓝图串、参数大数组或算法失败原因。 */
export interface FactoryPreviewBuilding extends Readonly<Pick<blueprint.Building,
  'index' | 'itemId' | 'modelIndex' | 'areaIndex' | 'localOffset' | 'localOffset2'
  | 'yaw' | 'yaw2' | 'tilt' | 'tilt2' | 'pitch' | 'pitch2' | 'inputFromSlot' | 'outputToSlot'>> {
  readonly inputObjectId?: string
  readonly outputObjectId?: string
  readonly productionIndex: number
  readonly isTemporary: boolean
}

/** 职责：描述实时预览使用的嵌套区域元数据；使用：解释 Area 局部坐标；边界：不改变蓝图编解码 DTO。 */
export interface FactoryPreviewArea {
  readonly index: number
  readonly parentIndex: number
  readonly tropicAnchor: number
  readonly areaSegments: number
  readonly anchorLocalOffset: Readonly<{x: number; y: number}>
  readonly size: Readonly<{x: number; y: number}>
}

/** 职责：描述稳定对象的新增、替换或撤销；使用：渲染器按 type 应用；边界：remove 不重复发送建筑数据。 */
export type FactoryPreviewOperation = {
  readonly type: 'add' | 'replace'
  readonly objectId: string
  readonly building: FactoryPreviewBuilding
} | {
  readonly type: 'remove'
  readonly objectId: string
}

/** 职责：描述被增量渲染的生产模块；使用：预览动画按模块归类建筑；边界：id 只在单次请求内稳定。 */
export interface FactoryPreviewModule {
  readonly id: string
  readonly name: string
  readonly productionIndex: number
  readonly mainItemId?: number
  readonly recipeId?: number
  readonly machineId?: number
  readonly machineCount?: number
}

/** 职责：描述最终蓝图中的一批建筑记录；使用：动画预览按 sequence 增量应用；边界：坐标属于 Area 局部坐标，不是行星世界坐标。 */
export interface FactoryBuildingBatch {
  readonly module: FactoryPreviewModule
  readonly batchIndex: number
  readonly batchCount: number
  readonly buildings: readonly FactoryPreviewBuildingRecord[]
  readonly areas: readonly FactoryPreviewArea[]
}

/** 职责：描述最终蓝图建筑记录及其预览生命周期；使用：最终建筑分块；边界：isTemporary 固定为 false，不修改蓝图记录。 */
export type FactoryPreviewBuildingRecord = Readonly<blueprint.Building & {isTemporary: false}>

/** 职责：关联运行时事件与请求；使用：过滤迟到消息；边界：同一方案每次生成也必须使用不同标识。 */
export interface FactoryGenerationProgressEvent extends FactoryGenerationProgress {
  readonly taskId: string
}
