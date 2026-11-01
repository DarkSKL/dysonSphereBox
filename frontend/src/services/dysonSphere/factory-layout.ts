type FactoryLayoutControls = {
  maxMachineInALine: number
  blockWidthLimit: number
  mergeType: number
  useMixed: boolean
  mix2: boolean
  needCoaterNet: boolean
}

/**
 * buildFactoryLayoutRequest 将工作台布局状态整理为生成请求。
 * 职责：防止自动球面模式携带平面专属参数；使用：生成和工作区导出共用；边界：不修改工作区状态，兼容开关始终关闭。
 */
export function buildFactoryLayoutRequest<T extends FactoryLayoutControls>(layout: T, automaticSurface: boolean) {
  const {maxMachineInALine, blockWidthLimit, mergeType, ...shared} = layout

  return {
    ...shared,
    ...(automaticSurface ? {} : {maxMachineInALine, blockWidthLimit, mergeType}),
    useMixed: false,
    mix2: false,
    needCoaterNet: false,
  }
}
