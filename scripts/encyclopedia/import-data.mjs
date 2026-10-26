// 职责：将固定提交的 FactorioLab 元数据转换为离线图鉴；使用：维护者手动更新数据，--icons 刷新项目内置图片；边界：不读取游戏文件，日常构建和运行不依赖此下载流程。
import {readFile, writeFile, mkdir} from 'node:fs/promises'
import {createHash} from 'node:crypto'
import {fileURLToPath} from 'node:url'
import path from 'node:path'

const root = fileURLToPath(new URL('../../', import.meta.url))
const lock = JSON.parse(await readFile(new URL('./source-lock.json', import.meta.url), 'utf8'))
const commit = lock.commit
const base = `https://raw.githubusercontent.com/factoriolab/factoriolab/${commit}/`
const output = path.join(root, 'frontend/src/data/encyclopedia')
const sourceIndex = process.argv.indexOf('--source-dir')
const sourceDir = sourceIndex >= 0 ? process.argv[sourceIndex + 1] : null
const withIcons = process.argv.includes('--icons')
const files = {'data.json': 'public/data/dsp/data.json', 'zh.json': 'public/data/dsp/i18n/zh.json', LICENSE: 'LICENSE'}
const inputs = {}
for (const [name, remote] of Object.entries(files)) {
  if (sourceDir) inputs[name] = await readFile(path.join(sourceDir, name))
  else {
    const response = await fetch(base + remote)
    if (!response.ok) throw new Error(`下载失败 ${remote}: ${response.status}`)
    inputs[name] = Buffer.from(await response.arrayBuffer())
  }
  if (createHash('sha256').update(inputs[name]).digest('hex') !== lock.checksums[name]) throw new Error(`固定源文件校验失败：${name}`)
}
const data = JSON.parse(inputs['data.json'])
const zh = JSON.parse(inputs['zh.json'])
const version = data.version.DSP
const sourceName = 'FactorioLab DSP'
const itemById = new Map(data.items.map(item => [item.id, item]))
const icons = new Map(data.icons.map(icon => [icon.id, {x: icon.x, y: icon.y}]))
const rawRecipes = data.recipes.filter(recipe => recipe.flags?.includes('mining') && Object.keys(recipe.in ?? {}).length === 0)
const rawItems = new Set(rawRecipes.flatMap(recipe => Object.keys(recipe.out)))
const overrides = {
  'sorter-4': '集装分拣器', 'holo-beacon': '全息投影器', 'holo-beacon-tech': '全息投影',
  'df-plasma-turret-sr': '近程等离子体塔', 'ray-receiver-dark-fog': '射线接收站（黑雾透镜）',
  'df-dark-fog-lens': '黑雾透镜', 'df-jamming-capsule': '干扰胶囊', 'df-suppressing-capsule': '压制胶囊',
  'df-suppressing-capsule-tech': '压制胶囊', 'df-jammer-tower-tech': '干扰塔',
  'sorter-cargo-integration': '分拣器货物集装', 'microcrystalline-component': '微晶元件',
  'critical-photon-dark-fog': '临界光子（黑雾透镜）', 'df-dark-fog-lens-tech': '黑雾透镜科技',
}
// 说明：补齐上游尚未提供的中文名称；仅翻译实体名称，不推测游戏数值。
function translate(id, fallback, recipe = false) {
  if (overrides[id]) return overrides[id]
  const recipeName = recipe ? zh.recipes[id] : undefined
  const translated = recipeName && /[\u3400-\u9fff]/.test(recipeName) ? recipeName : zh.items[id] ?? recipeName
  if (translated) return translated
  for (const [prefix, label] of [['df-em-weapon-strength-', '电磁武器伤害'], ['df-enhanced-structure-', '结构强化'], ['pile-sorter-', '集装分拣器货物叠加'], ['inventory-capacity-', '背包容量']]) {
    if (id.startsWith(prefix)) return `${label} Lv.${id.slice(prefix.length)}`
  }
  return fallback
}
function itemKind(item) {
  if (item.technology) return 'technology'
  return item.category.startsWith('buildings') ? 'building' : 'item'
}
function reference(id) {
  const item = itemById.get(id)
  if (!item) throw new Error(`未知实体 ${id}`)
  return `${itemKind(item)}:${id}`
}
function groupFor(item) {
  const id = item.id
  if (item.technology) return item.category === 'upgrades' ? '升级研究' : '科技研究'
  if (itemKind(item) === 'building') {
    if (id.startsWith('df-') && !/smelter|assembler|lab/.test(id)) return '战斗防御'
    if (/tower|turbine|power|substation|accumulator|solar-panel|artificial-star/.test(id)) return '电力设施'
    if (/conveyor|sorter|storage|tank|logistics|piler|distributor/.test(id)) return '运输仓储'
    if (/lab|ejector|silo|receiver|sphere/.test(id)) return '科研与戴森球'
    return '生产设施'
  }
  if (id.startsWith('df-')) return '战斗物资'
  if (rawItems.has(id)) return '矿物与流体'
  if (/matrix/.test(id)) return '矩阵与特殊物品'
  if (item.fuel || item.module) return '燃料与增产剂'
  return '材料与组件'
}
function createEntry(kind, value, index, group, iconId = value.icon ?? value.id) {
  return {id: value.id, kind, name: {zh: translate(value.id, value.name, kind === 'recipe' || kind === 'resource'), en: value.name},
    tags: [group, ...(value.id.startsWith('df-') ? ['黑雾'] : [])], group, order: index,
    iconKey: iconId, ...(icons.has(iconId) ? {icon: icons.get(iconId)} : {}), properties: [], gameVersion: version, source: sourceName, relations: []}
}
const shards = {building: [], item: [], recipe: [], technology: [], resource: []}
const entries = new Map()
for (const [index, item] of data.items.entries()) {
  if (item.category === 'effects') continue
  const kind = itemKind(item)
  const entry = createEntry(kind, item, index, groupFor(item))
  if (item.stack) entry.properties.push({label: '堆叠上限', value: item.stack, unit: '个'})
  if (item.machine?.usage !== undefined) entry.properties.push({label: '工作功率', value: item.machine.usage * 1000, unit: 'W'})
  if (item.machine?.drain !== undefined) entry.properties.push({label: '待机功率', value: item.machine.drain * 1000, unit: 'W'})
  if (item.machine?.speed !== undefined) entry.properties.push({label: '生产速度', value: item.machine.speed, unit: '倍'})
  if (item.belt) entry.properties.push({label: '运输速度', value: item.belt.speed ?? item.belt, unit: '个/秒'})
  if (item.fuel?.value) entry.properties.push({label: '燃料热值', value: item.fuel.value * 1000000, unit: 'J'})
  shards[kind].push(entry)
  entries.set(`${kind}:${entry.id}`, entry)
}
for (const [index, recipe] of data.recipes.entries()) {
  const outputId = Object.keys(recipe.out)[0]
  const outputItem = itemById.get(outputId)
  if (!outputItem || outputItem.category === 'effects') continue
  const group = recipe.flags?.includes('technology') ? '研究配方' : recipe.flags?.includes('mining') ? '采集配方' : groupFor(outputItem)
  const iconId = icons.has(recipe.icon ?? recipe.id) ? (recipe.icon ?? recipe.id) : (outputItem.icon ?? outputId)
  const entry = createEntry('recipe', recipe, index, group, iconId)
  if (Number.isFinite(recipe.time)) entry.properties.push({label: '基础耗时', value: recipe.time, unit: 's'})
  for (const [id, quantity] of Object.entries(recipe.in ?? {})) {
    if (!entries.has(reference(id))) continue
    entry.relations.push({type: 'ingredient', target: reference(id), quantity, unit: '个'})
  }
  for (const [id, quantity] of Object.entries(recipe.out)) {
    if (!entries.has(reference(id))) continue
    entry.relations.push({type: 'product', target: reference(id), quantity, unit: itemById.get(id).technology ? '哈希' : '个'})
  }
  for (const id of recipe.producers ?? []) entry.relations.push({type: 'producer', target: reference(id)})
  shards.recipe.push(entry)
  entries.set(`recipe:${entry.id}`, entry)
}
// 说明：建立正反关系；科技研究的哈希产出保留为科技属性，不误当作物品数量。
for (const entry of shards.recipe) {
  for (const relation of entry.relations) {
    const target = entries.get(relation.target)
    if (!target) throw new Error(`坏引用 ${relation.target}`)
    if (relation.type === 'product') {
      target.relations.push({type: target.kind === 'technology' ? 'researchedBy' : 'recipe', target: `recipe:${entry.id}`})
      if (target.kind === 'technology') {
        target.properties.push({label: '研究哈希', value: relation.quantity, unit: '哈希'})
        target.relations.push(...entry.relations.filter(item => item.type === 'ingredient'))
      }
    }
    if (relation.type === 'ingredient') target.relations.push({type: 'usedBy', target: `recipe:${entry.id}`})
  }
}
for (const item of data.items.filter(item => item.technology)) {
  const entry = entries.get(reference(item.id))
  for (const id of item.technology.prerequisites ?? []) entry.relations.push({type: 'prerequisite', target: reference(id)})
  for (const id of item.technology.recipeUnlock ?? []) {
    if (!entries.has(`recipe:${id}`)) continue
    entry.relations.push({type: 'unlocks', target: `recipe:${id}`})
    entries.get(`recipe:${id}`).relations.push({type: 'prerequisite', target: reference(item.id)})
  }
}
for (const [index, recipe] of rawRecipes.entries()) {
  const outputId = Object.keys(recipe.out)[0]
  const entry = createEntry('resource', recipe, index, /giant/.test(recipe.id) ? '气态行星' : /ocean|acid/.test(recipe.id) ? '海洋资源' : '矿脉资源', outputId)
  entry.relations.push({type: 'recipe', target: `recipe:${recipe.id}`}, ...entries.get(`recipe:${recipe.id}`).relations.filter(item => item.type !== 'ingredient'))
  for (const relation of entry.relations.filter(item => item.type === 'product')) entries.get(relation.target).relations.push({type: 'obtainedFrom', target: `resource:${entry.id}`})
  shards.resource.push(entry)
  entries.set(`resource:${entry.id}`, entry)
}
for (const entry of entries.values()) {
  const seen = new Set()
  entry.relations = entry.relations.filter(relation => {
    if (!entries.has(relation.target)) throw new Error(`坏引用 ${relation.target}`)
    if (relation.quantity !== undefined && (!Number.isFinite(relation.quantity) || relation.quantity <= 0)) throw new Error(`无效数量 ${entry.id}`)
    const key = `${relation.type}:${relation.target}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}
await mkdir(output, {recursive: true})
for (const [kind, values] of Object.entries(shards)) await writeFile(path.join(output, `${kind}.json`), JSON.stringify(values))
const manifest = {
  schemaVersion: 1, datasetVersion: `dsp-${version}-${commit.slice(0, 7)}`, gameVersion: version,
  generatedAt: new Date().toISOString(),
  sources: [{name: sourceName, url: `https://github.com/factoriolab/factoriolab/tree/${commit}/public/data/dsp`, license: 'MIT', commit, usedFor: ['metadata', 'i18n']}],
  shards: Object.fromEntries(Object.keys(shards).map(kind => [kind, `${kind}.json`])),
  counts: Object.fromEntries(Object.entries(shards).map(([kind, values]) => [kind, values.length])),
  sprite: {width: 1472, height: 1472, cell: 64, path: '/encyclopedia-icons/icons.webp'},
  checksums: Object.fromEntries(Object.entries(inputs).map(([name, value]) => [name, createHash('sha256').update(value).digest('hex')])),
}
await writeFile(path.join(output, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n')
await writeFile(path.join(output, 'LICENSE-FactorioLab.txt'), inputs.LICENSE)
if (withIcons) {
  const destination = path.join(root, 'frontend/public/encyclopedia-icons')
  await mkdir(destination, {recursive: true})
  let sprite
  if (sourceDir) sprite = await readFile(path.join(sourceDir, 'icons.webp'))
  else {
    const response = await fetch(base + 'public/data/dsp/icons.webp')
    if (!response.ok) throw new Error(`图标下载失败: ${response.status}`)
    sprite = Buffer.from(await response.arrayBuffer())
  }
  if (createHash('sha256').update(sprite).digest('hex') !== lock.checksums['icons.webp']) throw new Error('固定图标包校验失败')
  await writeFile(path.join(destination, 'icons.webp'), sprite)
  await writeFile(path.join(destination, 'LICENSE-FactorioLab.txt'), inputs.LICENSE)
  await mkdir(path.join(destination, 'categories'), {recursive: true})
  for (const asset of lock.categoryIcons) {
    let bytes
    if (sourceDir) bytes = await readFile(path.join(sourceDir, 'categories', `${asset.key}.png`))
    else {
      const response = await fetch(asset.url)
      if (!response.ok) throw new Error(`分类图标下载失败：${asset.key}`)
      bytes = Buffer.from(await response.arrayBuffer())
    }
    if (createHash('sha256').update(bytes).digest('hex') !== asset.sha256) throw new Error(`分类图标校验失败：${asset.key}`)
    await writeFile(path.join(destination, 'categories', `${asset.key}.png`), bytes)
  }
  await writeFile(path.join(destination, 'NOTICE.txt'), `戴森球计划图鉴图片素材\n\n实体精灵图来源：FactorioLab，固定提交 ${commit}\nhttps://github.com/factoriolab/factoriolab/tree/${commit}/public/data/dsp\n分类图标来源：戴森球计划 BWIKI\nhttps://wiki.biligame.com/dsp/首页\n原始图片地址与 SHA-256：项目 scripts/encyclopedia/source-lock.json\nFactorioLab 仓库许可证文本：同目录 LICENSE-FactorioLab.txt\n\n本目录随项目源码提交，并由 Vite/Wails 随程序打包；运行时不依赖图片源站。\n游戏素材、名称及商标的权利归 Youthcat Studio、Gamera Games 等原权利人所有；来源记录和随附仓库许可证不代表取得额外授权。\n`)
}
console.log(JSON.stringify({version, commit, counts: manifest.counts, localIcons: withIcons}, null, 2))
