import {shallowRef} from 'vue'
import manifestData from '../../data/encyclopedia/manifest.json'
import {encyclopediaGuides} from '../../data/encyclopedia/guides'
import type {EncyclopediaEntry, EncyclopediaKey, EncyclopediaKind} from '../../types/encyclopedia'
import {EncyclopediaDataError, validateManifest, validateRelations, validateShard} from './catalog-core'
import {encyclopediaCategories} from './categories'

// =============== 静态分片加载 ===============
// 说明：页面唯一的数据访问入口；动态 import 只使用构建白名单，失败不访问外网兜底。
export const encyclopediaManifest = validateManifest(manifestData)
export {encyclopediaGuides}
const loaders: Record<EncyclopediaKind, () => Promise<{default: unknown}>> = {
  building: () => import('../../data/encyclopedia/building.json'),
  item: () => import('../../data/encyclopedia/item.json'),
  recipe: () => import('../../data/encyclopedia/recipe.json'),
  technology: () => import('../../data/encyclopedia/technology.json'),
  resource: () => import('../../data/encyclopedia/resource.json'),
}
const shards = new Map<EncyclopediaKind, EncyclopediaEntry[]>()
const pending = new Map<EncyclopediaKind, Promise<EncyclopediaEntry[]>>()
const entryMap = new Map<EncyclopediaKey, EncyclopediaEntry>()

export function cachedCatalog(): EncyclopediaEntry[] {
  return [...shards.values()].flat()
}
export async function loadCatalogShard(kind: EncyclopediaKind): Promise<EncyclopediaEntry[]> {
  const cached = shards.get(kind)
  if (cached) return cached
  const current = pending.get(kind)
  if (current) return current
  const task = (async () => {
    try {
      const module = await loaders[kind]()
      const entries = validateShard(module.default, kind, encyclopediaManifest)
      shards.set(kind, entries)
      for (const entry of entries) entryMap.set(`${kind}:${entry.id}`, entry)
      return entries
    } catch (error) {
      if (error instanceof EncyclopediaDataError) throw error
      throw new EncyclopediaDataError('shardUnavailable', '图鉴数据加载失败，请重试')
    } finally {
      pending.delete(kind)
    }
  })()
  pending.set(kind, task)
  return task
}
export async function loadAllCatalog(): Promise<EncyclopediaEntry[]> {
  await Promise.all((Object.keys(loaders) as EncyclopediaKind[]).map(loadCatalogShard))
  const entries = cachedCatalog()
  validateRelations(entries)
  return entries
}
export function resolveCatalogEntry(key: EncyclopediaKey): EncyclopediaEntry | undefined {
  return entryMap.get(key)
}

// =============== 可替换图标资源包 ===============
// 说明：只探测同源本地图标包，缺失时所有条目使用本地 Game Icons；不引入远程图片依赖。
export const encyclopediaSpriteReady = shallowRef(false)
export const encyclopediaCategoryIconsReady = shallowRef<ReadonlySet<string>>(new Set())
let spriteProbe: Promise<void> | undefined
export function loadLocalIconPack(): Promise<void> {
  if (!spriteProbe) spriteProbe = new Promise(resolve => {
    const image = new Image()
    image.onload = () => {
      encyclopediaSpriteReady.value = true
      void Promise.all(encyclopediaCategories.map(category => new Promise<string | null>(done => {
        const icon = new Image()
        icon.onload = () => done(category.key)
        icon.onerror = () => done(null)
        icon.src = `/encyclopedia-icons/categories/${category.key}.png`
      }))).then(keys => { encyclopediaCategoryIconsReady.value = new Set(keys.filter((key): key is string => key !== null)); resolve() })
    }
    image.onerror = () => { encyclopediaSpriteReady.value = false; resolve() }
    image.src = encyclopediaManifest.sprite.path
  })
  return spriteProbe
}
