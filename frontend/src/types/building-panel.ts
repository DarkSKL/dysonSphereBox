// =============== 算法可视化建筑面板 ===============
// 说明：选中建筑后显示的游戏风格面板数据。普通建筑使用悬停浮窗样式，矩阵研究站与物流运输站使用游戏内的建筑窗口样式。
// 边界：只描述展示数据，不携带蓝图参数原文；数值来自量化数据集、量化方案与蓝图记录，不是游戏运行时的库存或电量。

/** 面板中的物品：product 为配方产物，input 为配方原料，marker 为传送带标记物品；count 为每次制作的件数 */
export interface BuildingPanelItem {
  readonly itemId: number
  readonly name: string
  readonly role: 'product' | 'input' | 'marker'
  readonly count?: number
}

/** 建筑功率（瓦）：work 为满载工作功率（已计入增产剂耗电），idle 为待机功率 */
export interface BuildingPanelPower {
  readonly work: number
  readonly idle: number
}

/** 各类面板共有的建筑身份 */
export interface BuildingPanelHead {
  readonly id: string
  readonly itemId: number
  readonly name: string
  /** 产线归属文案，如「铁块 产线」「共享设施」 */
  readonly owner: string
  /** 记录状态文案，如「正式记录 #4」 */
  readonly record: string
  readonly temporary: boolean
  /** 蓝图坐标文案 */
  readonly coords: string
}

/** 普通建筑：游戏中光标悬停片刻后出现的浮窗 */
export interface BriefBuildingPanel extends BuildingPanelHead {
  readonly kind: 'brief'
  readonly items: readonly BuildingPanelItem[]
  readonly power: BuildingPanelPower | null
}

/** 增产效果：mode 为 extra（额外产出）或 speed（生产加速）；output 与 power 为增量比例（0.25 即 +25%） */
export interface LabProliferatorEffect {
  readonly mode: 'extra' | 'speed'
  readonly output: number
  readonly power: number
}

/** 矩阵研究站：生产模式显示配方原料、产物与速率；没有配方时为科研模式 */
export interface LabBuildingPanel extends BuildingPanelHead {
  readonly kind: 'lab'
  readonly inputs: readonly BuildingPanelItem[]
  readonly output: BuildingPanelItem | null
  /** 单次制作秒数（已计入设备速度与生产加速），用于产物进度环 */
  readonly cycle: number | null
  /** 单机主产物每分钟产量 */
  readonly rate: number | null
  /** 单机满载工作功率（瓦） */
  readonly power: number | null
  /** 量化方案中的产线负荷（0 到 1）；导入蓝图没有方案时为 null */
  readonly utilization: number | null
  readonly proliferator: LabProliferatorEffect
}

/** 物流塔原型的基础能力 */
export interface StationFacts {
  readonly itemId: number
  readonly interstellar: boolean
  /** 物品槽位数 */
  readonly slots: number
  /** 每槽基础容量 */
  readonly capacity: number
  readonly drones: number
  readonly vessels: number
  /** 储能上限（焦） */
  readonly energy: number
  /** 最大充能功率滑块上限（瓦） */
  readonly maxCharge: number
}

/** 物流逻辑：supply 供应、demand 需求、none 仓储 */
export type StationLogic = 'none' | 'supply' | 'demand'

export interface StationStorage {
  readonly itemId: number
  readonly local: StationLogic
  readonly remote: StationLogic
  readonly max: number
}

/** 物流站设置；距离与比例已换算为游戏界面单位 */
export interface StationSettings {
  /** 最大充能功率（瓦） */
  readonly chargePower: number
  /** 运输机最远路程（度） */
  readonly droneRange: number
  /** 运输船最远路程（光年）；null 表示无限 */
  readonly shipRange: number | null
  /** 曲速启用路程（AU） */
  readonly warpDistance: number
  readonly includeOrbitCollector: boolean
  readonly warperNecessary: boolean
  /** 运输机起送量（%） */
  readonly droneDelivery: number
  /** 运输船起送量（%） */
  readonly shipDelivery: number
  /** 输出货物集装数量；0 表示使用科技上限 */
  readonly pilerCount: number
}

export interface StationParameters {
  /** 按槽位排列；空槽为 null */
  readonly storage: readonly (StationStorage | null)[]
  readonly settings: StationSettings
}

/** 物流运输站：槽位与设置读取自蓝图记录；读取中或不可用时槽位全空、settings 为 null，并由 notice 说明原因 */
export interface StationBuildingPanel extends BuildingPanelHead {
  readonly kind: 'station'
  readonly facts: StationFacts
  /** 长度等于原型槽位数；空槽为 null */
  readonly slots: readonly (StationSlot | null)[]
  readonly settings: StationSettings | null
  readonly loading: boolean
  readonly notice: string
}

/** 面板中的物流塔槽位：在蓝图参数之外附带物品名称 */
export interface StationSlot extends StationStorage {
  readonly name: string
}

export type BuildingPanel = BriefBuildingPanel | LabBuildingPanel | StationBuildingPanel
