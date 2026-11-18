import {BaseNode, ExtensionCategory, register} from '@antv/g6'
import type {AnalysisChartNode, AnalysisNodeStyle} from '../../../../types/analysis-g6'
import {factoryIconCanvas} from '../factory-icons'

// =============== G6 分析图形 ===============
// 职责：把分析图表的矩形、路径、文字和图标绘制为真正的 G6 节点；使用：统一画布；边界：不提供关系图布局，不嵌套其他图表引擎。
class AnalysisShape extends BaseNode<AnalysisNodeStyle> {
  protected drawKeyShape(a: Required<AnalysisNodeStyle>) {
    const paint = {fill: a.fill, stroke: a.stroke, lineWidth: a.lineWidth ?? 0, opacity: a.opacity ?? 1, cursor: a.cursor, pointerEvents: a.pointerEvents}
    if (a.chartShape === 'path') return this.upsert('key', 'path', {...paint, d: a.chartPath, lineDash: a.chartDashed ? [3, 4] : undefined}, this)
    if (a.chartShape === 'text') return this.upsert('key', 'text', {...paint, text: a.chartText, fontFamily: a.chartFont, fontSize: a.chartFontSize ?? 12, fontWeight: a.chartFontWeight ?? 400, textAlign: a.chartAlign ?? 'left', textBaseline: 'middle'}, this)
    if (a.chartShape === 'image') return this.upsert('key', 'image', {x: -(a.chartWidth ?? 20) / 2, y: -(a.chartHeight ?? 20) / 2, width: a.chartWidth, height: a.chartHeight, src: a.chartImage, pointerEvents: 'none'}, this)
    return this.upsert('key', 'rect', {...paint, x: 0, y: -(a.chartHeight ?? 0) / 2, width: a.chartWidth ?? 0, height: a.chartHeight ?? 0, radius: a.chartRadius ?? 0}, this)
  }
}
register(ExtensionCategory.NODE, 'dsbox-analysis-shape', AnalysisShape)

// =============== 图形数据构造 ===============
// 说明：统一样式命名，普通装饰不参与命中；传入浮层内容的节点才负责鼠标交互。内容由调用方转义。
export const chartNode = (id: string, x: number, y: number, style: Partial<AnalysisNodeStyle> & Pick<AnalysisNodeStyle, 'chartShape'>, tooltip?: string): AnalysisChartNode => ({
  id, type: 'dsbox-analysis-shape',
  style: {x, y, label: false, icon: false, halo: false, port: false, badge: false, fill: 'transparent', stroke: 'transparent', lineWidth: 0, pointerEvents: tooltip ? 'auto' : 'none', ...style},
  data: tooltip ? {tooltip} : undefined,
})
export const chartRect = (id: string, x: number, y: number, width: number, height: number, fill: string, tooltip?: string): AnalysisChartNode => chartNode(id, x, y, {chartShape: 'rect', chartWidth: Math.max(0, width), chartHeight: height, chartRadius: Math.min(4, height / 2), fill}, tooltip)
export const chartPath = (id: string, x: number, y: number, path: string, stroke: string, dashed = false): AnalysisChartNode => chartNode(id, x, y, {chartShape: 'path', chartPath: path, stroke, lineWidth: 1, chartDashed: dashed})
export const chartText = (id: string, x: number, y: number, text: string, fill: string, font: string, align: 'left' | 'center' | 'right' = 'left', fontSize = 12): AnalysisChartNode => chartNode(id, x, y, {chartShape: 'text', chartText: text, fill, chartFont: font, chartFontSize: fontSize, chartAlign: align})
const iconUrls = new WeakMap<HTMLCanvasElement, string>()
/** 图鉴图标复用已有裁切缓存；精灵就绪后由画布重新构图，缺失图标不影响数据绘制。 */
export const chartIcon = (id: string, x: number, y: number, name: string, size = 20): AnalysisChartNode | null => {
  const canvas = factoryIconCanvas(name, size)
  if (!canvas) return null
  let src = iconUrls.get(canvas)
  if (!src) {src = canvas.toDataURL(); iconUrls.set(canvas, src)}
  return chartNode(id, x, y, {chartShape: 'image', chartWidth: size, chartHeight: size, chartImage: src})
}
