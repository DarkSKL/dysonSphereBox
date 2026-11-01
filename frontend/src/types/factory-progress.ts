import type {blueprint} from '../../wailsjs/go/models'

/** 职责：描述真实生成阶段；使用：桥接回调与工作区状态；边界：序号表示当前处理项，不是整体完成百分比。 */
export interface FactoryGenerationProgress {
  readonly stage: string
  readonly message: string
  readonly current: number
  readonly total: number
  readonly protocolVersion?: number
  readonly kind?: 'phase' | 'building-batch' | 'preview-complete'
  readonly sequence?: number
  readonly coordinateSystem?: 'blueprint-area-local-v1'
  readonly previewBatch?: FactoryBuildingBatch
  readonly buildingCount?: number
  readonly batchCount?: number
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
  readonly buildings: readonly blueprint.Building[]
  readonly areas: readonly blueprint.Area[]
}

/** 职责：关联运行时事件与请求；使用：过滤迟到消息；边界：同一方案每次生成也必须使用不同标识。 */
export interface FactoryGenerationProgressEvent extends FactoryGenerationProgress {
  readonly requestId: string
}
