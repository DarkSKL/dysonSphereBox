// =============== 图鉴只读数据契约 ===============
// 说明：统一离线分片、实体属性与跨类别引用；数据不包含可执行 HTML 或游戏文件路径。
export const encyclopediaKinds = ['building', 'item', 'recipe', 'technology', 'resource'] as const
export type EncyclopediaKind = typeof encyclopediaKinds[number]
export type EncyclopediaKey = `${EncyclopediaKind}:${string}`
export type EncyclopediaCategory = 'building' | 'item' | 'technology' | 'combat' | 'achievement'
export type EncyclopediaRelationType = 'ingredient' | 'product' | 'producer' | 'prerequisite' | 'unlocks' | 'recipe' | 'usedBy' | 'researchedBy' | 'obtainedFrom'

export interface EncyclopediaRelation {
  type: EncyclopediaRelationType
  target: EncyclopediaKey
  quantity?: number
  unit?: string
}

export interface EncyclopediaProperty {
  label: string
  value: string | number
  unit?: string
}

export interface EncyclopediaEntry {
  id: string
  kind: EncyclopediaKind
  name: {zh: string; en?: string}
  aliases?: string[]
  description?: string
  tags: string[]
  group: string
  order: number
  iconKey?: string
  icon?: {x: number; y: number}
  properties: EncyclopediaProperty[]
  gameVersion: string
  source: string
  relations: EncyclopediaRelation[]
}

export interface EncyclopediaManifest {
  schemaVersion: 1
  datasetVersion: string
  gameVersion: string
  generatedAt: string
  sources: {name: string; url: string; license: string; commit: string; usedFor: string[]}[]
  shards: Record<EncyclopediaKind, string>
  counts: Record<EncyclopediaKind, number>
  sprite: {width: number; height: number; cell: number; path: string}
}

export interface EncyclopediaGuide {
  id: string
  category: EncyclopediaCategory
  title: string
  summary: string
  paragraphs: readonly string[]
  related: readonly EncyclopediaKey[]
  to?: string
  action?: string
}

export type EncyclopediaErrorCode = 'manifestUnavailable' | 'shardUnavailable' | 'invalidJson' | 'schemaMismatch' | 'duplicateId' | 'brokenRelation' | 'versionMismatch'

// =============== 成就索引契约 ===============
// 说明：成就为只读阅读数据，不对应 catalog 分片；关联条目仍引用已收录实体。
export type EncyclopediaAchievementGroup = 'gathering' | 'construction' | 'icarus' | 'exploration' | 'production' | 'power' | 'challenge' | 'combat'
export interface EncyclopediaAchievement {
  id: number
  name: string
  group: EncyclopediaAchievementGroup
  condition: string
  tip?: string
  combat?: boolean
  noMetadata?: boolean
  related?: readonly EncyclopediaKey[]
}
