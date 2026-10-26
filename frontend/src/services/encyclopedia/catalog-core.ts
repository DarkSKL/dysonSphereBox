import type {EncyclopediaEntry, EncyclopediaErrorCode, EncyclopediaKind, EncyclopediaManifest, EncyclopediaProperty, EncyclopediaRelation, EncyclopediaRelationType} from '../../types/encyclopedia'

// =============== 数据校验与错误边界 ===============
// 说明：仅解析静态数据的已知字段；调用方捕获稳定错误码，未知扩展字段和关系类型不进入 UI。
const kinds: readonly EncyclopediaKind[] = ['building', 'item', 'recipe', 'technology', 'resource']
const relationTypes: readonly EncyclopediaRelationType[] = ['ingredient', 'product', 'producer', 'prerequisite', 'unlocks', 'recipe', 'usedBy', 'researchedBy', 'obtainedFrom']
export class EncyclopediaDataError extends Error {
  constructor(readonly code: EncyclopediaErrorCode, message: string) {
    super(message)
    this.name = 'EncyclopediaDataError'
  }
}

function fail(code: EncyclopediaErrorCode, message: string): never {
  throw new EncyclopediaDataError(code, message)
}
function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
function text(value: unknown): value is string {
  return typeof value === 'string' && Boolean(value.trim()) && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/.test(value)
}

/** 校验 manifest 的版本、溯源和构建白名单；不将任意路径交给动态 import。 */
export function validateManifest(value: unknown): EncyclopediaManifest {
  if (!record(value) || value.schemaVersion !== 1 || !text(value.datasetVersion) || !text(value.gameVersion)
    || !text(value.generatedAt) || !Number.isFinite(Date.parse(value.generatedAt))) fail('schemaMismatch', '图鉴数据版本或清单格式不兼容')
  if (!Array.isArray(value.sources) || !value.sources.length || value.sources.some(source => !record(source) || !text(source.name) || !text(source.license) || !text(source.url) || !/^[a-f0-9]{40}$/.test(String(source.commit)))) fail('schemaMismatch', '图鉴缺少来源记录')
  if (!record(value.shards) || !record(value.counts)) fail('schemaMismatch', '图鉴分片清单缺失')
  for (const kind of kinds) {
    if (value.shards[kind] !== `${kind}.json` || !Number.isSafeInteger(value.counts[kind]) || Number(value.counts[kind]) < 0) fail('schemaMismatch', '图鉴分片路径或数量无效')
  }
  if (!record(value.sprite) || value.sprite.width !== 1472 || value.sprite.height !== 1472 || value.sprite.cell !== 64 || value.sprite.path !== '/encyclopedia-icons/icons.webp') fail('schemaMismatch', '图鉴图标清单无效')
  return value as unknown as EncyclopediaManifest
}

/** 解析单个类别；关系的跨分片存在性由整包校验与 loadAllCatalog 共同验证。 */
export function validateShard(value: unknown, kind: EncyclopediaKind, manifest: EncyclopediaManifest): EncyclopediaEntry[] {
  if (!Array.isArray(value)) fail('invalidJson', '图鉴分片不是有效列表')
  const seen = new Set<string>()
  const entries: EncyclopediaEntry[] = []
  for (const raw of value) {
    if (record(raw) && typeof raw.kind === 'string' && !kinds.includes(raw.kind as EncyclopediaKind)) continue
    if (!record(raw) || raw.kind !== kind || !text(raw.id) || !record(raw.name) || !text(raw.name.zh) || !text(raw.source) || !text(raw.group) || !Array.isArray(raw.relations)) fail('schemaMismatch', '图鉴条目缺少必要字段')
    if (raw.gameVersion !== manifest.gameVersion) fail('versionMismatch', '图鉴分片包含不同的游戏版本')
    if (seen.has(raw.id)) fail('duplicateId', `重复条目：${raw.id}`)
    seen.add(raw.id)
    const relations: EncyclopediaRelation[] = []
    for (const relation of raw.relations) {
      if (!record(relation) || !relationTypes.includes(relation.type as EncyclopediaRelationType)) continue
      if (!text(relation.target) || !/^(building|item|recipe|technology|resource):[^\s:]+$/.test(relation.target)) fail('brokenRelation', '图鉴包含无效关联')
      if (relation.quantity !== undefined && (typeof relation.quantity !== 'number' || !Number.isFinite(relation.quantity) || relation.quantity <= 0 || !text(relation.unit))) fail('schemaMismatch', '图鉴关联数量无效')
      relations.push({type: relation.type as EncyclopediaRelationType, target: relation.target as EncyclopediaRelation['target'], ...(relation.quantity !== undefined ? {quantity: relation.quantity as number, unit: relation.unit as string} : {})})
    }
    const properties: EncyclopediaProperty[] = []
    if (Array.isArray(raw.properties)) for (const property of raw.properties) {
      if (!record(property) || !text(property.label) || !(text(property.value) || (typeof property.value === 'number' && Number.isFinite(property.value)))) fail('schemaMismatch', '图鉴属性格式无效')
      properties.push({label: property.label, value: property.value as string | number, ...(text(property.unit) ? {unit: property.unit} : {})})
    }
    const icon = record(raw.icon) && Number.isInteger(raw.icon.x) && Number.isInteger(raw.icon.y)
      && Number(raw.icon.x) >= 0 && Number(raw.icon.x) <= 1408 && Number(raw.icon.y) >= 0 && Number(raw.icon.y) <= 1408
      ? {x: Number(raw.icon.x), y: Number(raw.icon.y)} : undefined
    entries.push({id: raw.id, kind, name: {zh: raw.name.zh, ...(text(raw.name.en) ? {en: raw.name.en} : {})},
      ...(text(raw.description) ? {description: raw.description} : {}), aliases: Array.isArray(raw.aliases) ? raw.aliases.filter(text) : [],
      tags: Array.isArray(raw.tags) ? raw.tags.filter(text) : [], group: raw.group,
      order: typeof raw.order === 'number' && Number.isFinite(raw.order) ? raw.order : entries.length,
      iconKey: text(raw.iconKey) ? raw.iconKey : undefined, icon, properties, gameVersion: manifest.gameVersion, source: raw.source, relations})
  }
  return entries
}

/** 整体校验跨分片引用；缺失目标时停止展示关系，避免混用不完整的数据集。 */
export function validateRelations(entries: readonly EncyclopediaEntry[]): void {
  const keys = new Set(entries.map(entry => `${entry.kind}:${entry.id}`))
  for (const entry of entries) for (const relation of entry.relations) {
    if (!keys.has(relation.target)) fail('brokenRelation', `关联目标不存在：${relation.target}`)
  }
}

// =============== 搜索与展示格式 ===============
// 说明：搜索索引仅构建一次；全角归一、名称权重、AND 筛选与稳定排序都与页面解耦。
export function normalizeKeyword(value: string): string {
  return value.normalize('NFKC').toLocaleLowerCase().trim().replace(/\s+/g, ' ')
}
export interface EncyclopediaSearchRow {
  entry: EncyclopediaEntry
  id: string
  names: string[]
  aliases: string[]
  tags: string[]
}
export function buildSearchIndex(entries: readonly EncyclopediaEntry[]): EncyclopediaSearchRow[] {
  return entries.map(entry => ({entry, id: normalizeKeyword(entry.id), names: [entry.name.zh, entry.name.en ?? ''].map(normalizeKeyword).filter(Boolean), aliases: (entry.aliases ?? []).map(normalizeKeyword), tags: entry.tags.map(normalizeKeyword)}))
}
export function searchCatalog(index: readonly EncyclopediaSearchRow[], keyword: string, filters: {kinds?: readonly EncyclopediaKind[]; tag?: string; combat?: boolean} = {}): EncyclopediaEntry[] {
  const query = normalizeKeyword(keyword)
  const collator = new Intl.Collator('zh-CN', {numeric: true})
  return index.flatMap(row => {
    if (filters.kinds && !filters.kinds.includes(row.entry.kind)) return []
    if (filters.tag && !row.entry.tags.includes(filters.tag)) return []
    if (filters.combat && !row.entry.tags.includes('黑雾')) return []
    let score = query ? 0 : 1
    if (query) {
      if (row.id === query || row.names.includes(query)) score = 1000
      else if (row.names.some(name => name.startsWith(query))) score = 800
      else if (row.aliases.includes(query)) score = 600
      else if ([row.id, ...row.names, ...row.aliases].some(field => field.includes(query))) score = 400
      else if (row.tags.some(tag => tag.includes(query))) score = 100
    }
    return score ? [{entry: row.entry, score}] : []
  }).sort((a, b) => query ? b.score - a.score || collator.compare(a.entry.name.zh, b.entry.name.zh) || a.entry.id.localeCompare(b.entry.id) : a.entry.order - b.entry.order)
    .map(row => row.entry)
}
export function formatProperty(property: EncyclopediaProperty): string {
  if (typeof property.value !== 'number') return `${property.value}${property.unit ? ` ${property.unit}` : ''}`
  const value = property.value
  if (property.unit === 'W' || property.unit === 'J') {
    const prefix = value >= 1e9 ? {factor: 1e9, unit: 'G'} : value >= 1e6 ? {factor: 1e6, unit: 'M'} : value >= 1e3 ? {factor: 1e3, unit: 'k'} : {factor: 1, unit: ''}
    return `${Number((value / prefix.factor).toFixed(3))} ${prefix.unit}${property.unit}`
  }
  return `${Number(value.toFixed(3)).toLocaleString('zh-CN')}${property.unit ? ` ${property.unit}` : ''}`
}
