import type {blueprint} from '../../wailsjs/go/models'
import type {AlgoVizBuilding, AlgoVizOperation} from '../packages/algorithm-viz/src'

// =============== 完整蓝图的协作式回放准备 ===============
// 说明：保留全部记录与跨批连接，分批建立身份索引和新旧差量；期间让出主线程供绘制和交互。
// 边界：只准备数据，不操作场景或镜像；任务变化时丢弃尚未提交的差量，不截断建筑数量。
export const ALGORITHM_REPLAY_CHUNK = 512

export interface AlgorithmMirrorEntry {
  readonly building: AlgoVizBuilding
  readonly index?: number
}

export interface AlgorithmReplayDiff {
  readonly removes: AlgoVizOperation[]
  readonly replaces: AlgoVizOperation[]
  readonly adds: AlgoVizOperation[]
}

/** 让浏览器处理输入与绘制；隐藏页面也能完成，不依赖会暂停的动画帧。 */
export const yieldAlgorithmReplay = (): Promise<void> => new Promise(resolve => { window.setTimeout(resolve, 0) })

const sameBuilding = (a: AlgoVizBuilding, b: AlgoVizBuilding): boolean => {
  const near = (p?: number, q?: number): boolean => Math.abs((p ?? 0) - (q ?? 0)) < 1e-3
  return a.itemId === b.itemId && a.modelIndex === b.modelIndex && a.areaIndex === b.areaIndex && a.group === b.group
    && Boolean(a.temporary) === Boolean(b.temporary) && a.outputId === b.outputId
    && near(a.x, b.x) && near(a.y, b.y) && near(a.z, b.z) && near(a.x2, b.x2) && near(a.y2, b.y2) && near(a.z2, b.z2)
    && Math.abs((a.yaw ?? 0) - (b.yaw ?? 0)) < 0.01
}

/** 准备完整差量；身份索引先于建筑转换，避免跨批或超出旧阈值的带连接丢失。取消返回null，原镜像保持原样。 */
export const prepareAlgorithmReplay = async (
  buildings: readonly blueprint.Building[],
  mirror: ReadonlyMap<string, AlgorithmMirrorEntry>,
  groups: ReadonlyMap<number, number>,
  isCurrent: () => boolean,
  yieldControl: () => Promise<void> = yieldAlgorithmReplay,
): Promise<AlgorithmReplayDiff | null> => {
  if (!isCurrent()) return null
  const idOfIndex = new Map<number, string>()
  const seen = new Set<string>()
  const removes: AlgoVizOperation[] = [], replaces: AlgoVizOperation[] = [], adds: AlgoVizOperation[] = []
  let work = 0
  for (const [id, entry] of mirror) {
    if (entry.index !== undefined) idOfIndex.set(entry.index, id)
    if (++work % ALGORITHM_REPLAY_CHUNK === 0) { await yieldControl(); if (!isCurrent()) return null }
  }
  for (const building of buildings) {
    if (!idOfIndex.has(building.index)) idOfIndex.set(building.index, `bp:${building.index}`)
    if (++work % ALGORITHM_REPLAY_CHUNK === 0) { await yieldControl(); if (!isCurrent()) return null }
  }
  for (const record of buildings) {
    const id = idOfIndex.get(record.index) as string
    seen.add(id)
    const building: AlgoVizBuilding = {
      itemId: record.itemId, modelIndex: record.modelIndex, x: record.localOffset.x, y: record.localOffset.y, z: record.localOffset.z,
      x2: record.localOffset2.x, y2: record.localOffset2.y, z2: record.localOffset2.z, yaw: record.yaw, areaIndex: record.areaIndex,
      group: groups.get(record.index) ?? -1, temporary: false,
      outputId: record.outputObjIdx >= 0 ? idOfIndex.get(record.outputObjIdx) : undefined, index: record.index,
    }
    const previous = mirror.get(id)
    if (!previous) adds.push({type: 'add', id, building})
    else if (!sameBuilding(previous.building, building)) replaces.push({type: 'replace', id, building})
    if (++work % ALGORITHM_REPLAY_CHUNK === 0) { await yieldControl(); if (!isCurrent()) return null }
  }
  for (const id of mirror.keys()) {
    if (!seen.has(id)) removes.push({type: 'remove', id})
    if (++work % ALGORITHM_REPLAY_CHUNK === 0) { await yieldControl(); if (!isCurrent()) return null }
  }
  return isCurrent() ? {removes, replaces, adds} : null
}
