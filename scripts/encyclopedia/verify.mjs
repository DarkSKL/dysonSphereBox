// 职责：验证离线图鉴数据、搜索契约与坏数据边界；使用：在应用根目录执行 node scripts/encyclopedia/verify.mjs；边界：不写源码、不连接游戏或互联网。
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import {createRequire} from 'node:module'
import {fileURLToPath} from 'node:url'
import path from 'node:path'

const root = fileURLToPath(new URL('../../', import.meta.url))
const require = createRequire(path.join(root, 'frontend/package.json'))
const ts = require('typescript')
const source = await readFile(path.join(root, 'frontend/src/services/encyclopedia/catalog-core.ts'), 'utf8')
const code = ts.transpileModule(source, {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext}}).outputText
const core = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)
const dataDir = path.join(root, 'frontend/src/data/encyclopedia')
const json = async name => JSON.parse(await readFile(path.join(dataDir, name), 'utf8'))
const manifest = core.validateManifest(await json('manifest.json'))
const all = []
for (const kind of Object.keys(manifest.shards)) {
  const raw = await json(manifest.shards[kind])
  const entries = core.validateShard(raw, kind, manifest)
  assert.equal(entries.length, manifest.counts[kind])
  assert.ok(Buffer.byteLength(JSON.stringify(raw)) <= 2 * 1024 * 1024)
  all.push(...entries)
}
core.validateRelations(all)
assert.equal(all.length, 1001)
assert.ok(Buffer.byteLength(JSON.stringify(all)) < 8 * 1024 * 1024)
const index = core.buildSearchIndex(all)
const find = (kind, id) => all.find(entry => entry.kind === kind && entry.id === id)
assert.ok(core.searchCatalog(index, '铁块')[0].name.zh === '铁块')
assert.equal(core.searchCatalog(index, 'ＩＲＯＮ－ＯＲＥ', {kinds: ['item']})[0].id, 'iron-ore')
assert.deepEqual(core.searchCatalog(index, '铁', {kinds: ['item'], tag: '矿物与流体'}).map(entry => entry.id), ['iron-ore'])
assert.equal(core.searchCatalog(index, 'not-a-real-entry-87654').length, 0)
assert.equal(core.searchCatalog(index, ' ', {kinds: ['building']}).length, 64)
assert.ok(core.searchCatalog(index, '', {combat: true}).every(entry => entry.tags.includes('黑雾')))
const aliased = structuredClone(find('item', 'iron-ingot'))
aliased.aliases = ['铁锭']
assert.equal(core.searchCatalog(core.buildSearchIndex([aliased]), '铁锭')[0].id, 'iron-ingot')
assert.equal(core.formatProperty(find('item', 'coal').properties.find(property => property.label === '燃料热值')), '2.7 MJ')
assert.equal(core.formatProperty(find('building', 'assembling-machine-1').properties.find(property => property.label === '工作功率')), '270 kW')
assert.equal(core.formatProperty({label: '工作功率', value: 0, unit: 'W'}), '0 W')
const building = structuredClone(find('building', 'tesla-tower'))
function rejects(code, fn) { assert.throws(fn, error => error.code === code) }
rejects('duplicateId', () => core.validateShard([building, building], 'building', manifest))
rejects('versionMismatch', () => core.validateShard([{...building, gameVersion: 'different'}], 'building', manifest))
rejects('schemaMismatch', () => core.validateManifest({...manifest, schemaVersion: 2}))
rejects('schemaMismatch', () => core.validateManifest({...manifest, shards: {...manifest.shards, building: 'https://remote/other.json'}}))
rejects('brokenRelation', () => core.validateRelations([{...building, relations: [{type: 'recipe', target: 'recipe:missing'}]}]))
rejects('schemaMismatch', () => core.validateShard([{...building, relations: [{type: 'ingredient', target: 'item:iron-ore', quantity: 0, unit: '个'}]}], 'building', manifest))
const extended = core.validateShard([{...building, unknown: 'ignore', relations: [...building.relations, {type: 'futureRelation', target: 'future:unknown'}]}], 'building', manifest)[0]
assert.equal(extended.unknown, undefined)
assert.equal(extended.relations.length, building.relations.length)
const guideSource = await readFile(path.join(dataDir, 'guides.ts'), 'utf8')
const guidesCode = ts.transpileModule(guideSource, {compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext}}).outputText
const {encyclopediaGuides} = await import(`data:text/javascript;base64,${Buffer.from(guidesCode).toString('base64')}`)
const keys = new Set(all.map(entry => `${entry.kind}:${entry.id}`))
for (const guide of encyclopediaGuides) for (const key of guide.related) assert.ok(keys.has(key), `离线阅读含无效引用：${guide.id} -> ${key}`)
console.log(`图鉴校验通过：${all.length} 条记录，5 个分片，搜索/筛选/数量单位/版本/错误边界/阅读关联均通过。`)
