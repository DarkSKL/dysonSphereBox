import type {StationFacts, StationLogic, StationParameters} from '../types/building-panel'

// =============== 物流塔蓝图参数 ===============
// 说明：解析蓝图记录中物流运输站的 2048 字参数，供建筑面板只读展示。
// 布局：物品槽从 0 起每 6 字为 [物品, 本地逻辑, 星际逻辑, 上限, 保持模式]；320 起依次为充能功率（每 tick）、运输机路程（余弦 × 1e8）、
//       运输船路程（米 / 100）、轨道采集器、曲速启用路程（米）、翘曲器必备、运输机与运输船起送量、集装数量。
//       与生成器 helper/blueprint/build/algorithm/station.go 写入的布局一致，换算规则与内置蓝图编辑器相同。
// 边界：只读解析，不校验也不修改参数；参数长度不足时返回 null。

/** 数值核对 helper/transport/data/infrastructure-25955035.json（游戏 0.10.35 PrefabDesc 事实）；充能上限取图鉴工作功率 */
export const STATION_FACTS: Readonly<Record<number, StationFacts>> = {
  2103: {itemId: 2103, interstellar: false, slots: 4, capacity: 5000, drones: 50, vessels: 0, energy: 180e6, maxCharge: 60e6},
  2104: {itemId: 2104, interstellar: true, slots: 5, capacity: 10000, drones: 100, vessels: 10, energy: 12e9, maxCharge: 300e6},
}

const METERS_PER_LY = 2.4e6
const METERS_PER_AU = 4e4

const logic = (value: number): StationLogic => value === 1 ? 'supply' : value === 2 ? 'demand' : 'none'

/** 解析物流塔参数；物品为 0 的槽位记为空槽 */
export const parseStationParameters = (facts: StationFacts, parameters: readonly number[] | undefined): StationParameters | null => {
  if (!parameters || parameters.length < 329) return null
  const storage = Array.from({length: facts.slots}, (_, slot) => {
    const base = slot * 6
    const itemId = parameters[base]
    return itemId > 0 ? {itemId, local: logic(parameters[base + 1]), remote: logic(parameters[base + 2]), max: parameters[base + 3]} : null
  })
  const cosine = Math.min(1, Math.max(-1, parameters[321] / 1e8))
  const shipRange = parameters[322] * 100 / METERS_PER_LY
  return {
    storage,
    settings: {
      chargePower: parameters[320] * 60,
      droneRange: Math.acos(cosine) * 180 / Math.PI,
      shipRange: shipRange > 9999 ? null : shipRange,
      warpDistance: parameters[324] / METERS_PER_AU,
      includeOrbitCollector: parameters[323] > 0,
      warperNecessary: parameters[325] > 0,
      droneDelivery: parameters[326],
      shipDelivery: parameters[327],
      pilerCount: parameters[328],
    },
  }
}
