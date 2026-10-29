/** 职责：描述真实生成阶段；使用：桥接回调与工作区状态；边界：序号表示当前处理项，不是整体完成百分比。 */
export interface FactoryGenerationProgress {
  readonly stage: string
  readonly message: string
  readonly current: number
  readonly total: number
}

/** 职责：关联运行时事件与请求；使用：过滤迟到消息；边界：同一方案每次生成也必须使用不同标识。 */
export interface FactoryGenerationProgressEvent extends FactoryGenerationProgress {
  readonly requestId: string
}
