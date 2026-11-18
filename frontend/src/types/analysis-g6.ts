import type {BaseNodeStyleProps, NodeData} from '@antv/g6'

// 职责：约束分析图表交给 G6 的图形与数据；使用：渲染适配器和图表组件；边界：不承载后端统计或改变统计口径。
export interface AnalysisNodeStyle extends BaseNodeStyleProps, Record<string, unknown> {
  chartShape: 'rect' | 'path' | 'text' | 'image'
  chartWidth?: number
  chartHeight?: number
  chartPath?: string
  chartText?: string
  chartImage?: string
  chartFont?: string
  chartFontSize?: number
  chartFontWeight?: number
  chartAlign?: 'left' | 'center' | 'right'
  chartRadius?: number
  chartDashed?: boolean
}
export interface AnalysisChartNode extends NodeData {
  style: AnalysisNodeStyle
  data?: {tooltip?: string}
}
export type AnalysisSceneBuilder = (width: number, height: number) => AnalysisChartNode[]

// 职责：描述同一数轴下的条段、标记与行标签；使用：电力、产线和物料图；边界：所有位置均保留原始数值，由渲染层映射为像素。
export interface AnalysisBarSegment {from: number; to: number; color: string}
export interface AnalysisBarRow {
  id: string
  name: string
  icon: string
  segments: readonly AnalysisBarSegment[]
  marks?: readonly number[]
  label: string
  labelAt: number
  labelSide?: 'left' | 'right'
  labelIcon?: string
  secondaryLabel?: {text: string; at: number; side: 'left' | 'right'}
  tooltip: string
}
