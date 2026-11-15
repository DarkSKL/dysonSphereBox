import type {build, quant, statistics} from '../../wailsjs/go/models'

// 职责：补足Wails指针字段的JSON null语义，提供纯数据契约；使用：后续统计页；边界：不重算后端数值，不把未知转成零。
type Data<T> = {
  [K in keyof T as T[K] extends (...args: never[]) => unknown ? never : K]: T[K]
}

export type FactoryPowerStatistics = Omit<Data<statistics.Power>, 'expectedMw'> & {
  expectedMw?: number | null
}
export type FactoryThermalStatistics = Omit<Data<statistics.Thermal>, 'dissipationMw'> & {
  dissipationMw?: number | null
}
export type FactoryBuildingStatistics = Omit<Data<statistics.BuildingGroup>, 'power' | 'thermal' | 'utilization'> & {
  power: FactoryPowerStatistics
  thermal: FactoryThermalStatistics
  utilization?: number | null
}
export type FactoryLineStatistics = Omit<Data<statistics.Line>, 'power' | 'thermal'> & {
  power: FactoryPowerStatistics
  thermal: FactoryThermalStatistics
}
export type FactoryMaterialStatistics = Omit<Data<statistics.Material>, 'heatValueMj'> & {
  heatValueMj?: number | null
}
export type FactoryHeatmapEntry = Omit<Data<statistics.HeatmapEntry>, 'thermal' | 'utilization' | 'expectedMw'> & {
  thermal: FactoryThermalStatistics
  utilization?: number | null
  expectedMw?: number | null
}
export type FactoryStatistics = Omit<Data<statistics.Report>, 'power' | 'thermal' | 'buildings' | 'lines' | 'materials' | 'heatmap' | 'space'> & {
  power: FactoryPowerStatistics
  thermal: FactoryThermalStatistics
  buildings: FactoryBuildingStatistics[]
  lines: FactoryLineStatistics[]
  materials: FactoryMaterialStatistics[]
  heatmap: FactoryHeatmapEntry[]
  space?: Data<statistics.Space> | null
}

// 职责：读取两阶段统一统计；使用：现有Solve/Generate结果；边界：旧快照没有统计时返回null，页面应重新求解或生成。
export const factoryStatisticsOf = (result: quant.Result | build.Result | null | undefined): FactoryStatistics | null =>
  result?.statistics ?? null
