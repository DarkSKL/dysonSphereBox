// =============== 资源页类型 ===============
// 说明：资源卡片色调，决定边框与状态点颜色；状态值与后端 EnvItem.State 保持一致，另含卡片类型色调。
export type ResourceCardTone =
  | 'default'
  | 'enabled'
  | 'disabled'
  | 'missing'
  | 'conflict'
  | 'unsupported'
  | 'folder'
  | 'save'
  | 'market'
