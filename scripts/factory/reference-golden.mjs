// 职责：只在本地读取用户提供的参考实现并生成独立 golden；使用：见验证文档；边界：参考源码、语料和运行产物不入 Git，不调用游戏或数据库。
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import { createRequire } from 'node:module'
import { pathToFileURL } from 'node:url'
import { build } from '../../frontend/node_modules/esbuild/lib/main.js'

const source = process.env.DSBOX_REFERENCE_SOURCE
const dependencies = process.env.DSBOX_REFERENCE_NODE_MODULES
const output = process.env.DSBOX_QUANT_GOLDEN
if (!source || !dependencies || !output) throw new Error('需要 DSBOX_REFERENCE_SOURCE、DSBOX_REFERENCE_NODE_MODULES 和 DSBOX_QUANT_GOLDEN')
const require = createRequire(import.meta.url)
const temp = await fs.mkdtemp(path.join(os.tmpdir(), 'dsbox-golden-'))
const datasets = {}
for (const file of (await fs.readdir(path.join(source, 'data'))).filter(file => file.endsWith('.json'))) {
  datasets[`../data/${file}`] = JSON.parse((await fs.readFile(path.join(source, 'data', file), 'utf8')).replace(/^\uFEFF/, ''))
}
const contexts = await fs.readFile(path.join(source, 'src/contexts.jsx'), 'utf8')
const defaultLiteral = contexts.match(/const DEFAULT_SETTINGS = (\{[\s\S]*?\n\});/)[1]
const entry = path.join(temp, 'entry.mjs')
await fs.writeFile(entry, `import { get_game_data } from ${JSON.stringify(path.join(source, 'src/GameData.jsx'))};
import { GlobalState } from ${JSON.stringify(path.join(source, 'src/global_state.jsx'))};
import { init_scheme_data } from ${JSON.stringify(path.join(source, 'src/scheme_data.jsx'))};
export { get_game_data, GlobalState, init_scheme_data };
export const settings = ${defaultLiteral};`)
const bundled = path.join(temp, 'reference.mjs')
await build({
  entryPoints: [entry], outfile: bundled, bundle: true, platform: 'node', format: 'esm', logLevel: 'silent',
  banner: { js: "import { createRequire as nodeCreateRequire } from 'node:module'; const require = nodeCreateRequire(import.meta.url);" },
  plugins: [{ name: 'reference-adapters', setup(api) {
    api.onResolve({ filter: /^javascript-lp-solver$/ }, () => ({ path: require.resolve('javascript-lp-solver', { paths: [dependencies] }) }))
    api.onLoad({ filter: /GameData\.jsx$/ }, async args => {
      const original = await fs.readFile(args.path, 'utf8')
      return { contents: original.replace(/const data_index_modules = import\.meta\.glob\([\s\S]*?\n\}\);/, `const data_index_modules = ${JSON.stringify(datasets)};`), loader: 'js' }
    })
    api.onLoad({ filter: /scheme_data\.jsx$/ }, async args => {
      const original = await fs.readFile(args.path, 'utf8')
      return { contents: original.slice(original.indexOf('const DEFAULT_SCHEME_DATA'), original.indexOf('export function SchemeStorage')), loader: 'js' }
    })
  } }],
})
const reference = await import(pathToFileURL(bundled))
globalThis.alert = message => { throw new Error(message) }
const modGuids = { MoreMegaStructure: 'Gnimaerd.DSP.plugin.MoreMegaStructure', TheyComeFromVoid: 'com.ckcz123.DSP_Battle', GenesisBook: 'org.LoShin.GenesisBook', FractionateEverything: 'com.menglei.dsp.FractionateEverything' }
const cases = [
  { name: 'blue-matrix-60', request: { targets: [{ itemId: 6001, ratePerMinute: 60 }], dataset: 'Vanilla' } },
  { name: 'sulfuric-acid-60', request: { targets: [{ itemId: 1116, ratePerMinute: 60 }], dataset: 'Vanilla' } },
  { name: 'multi-target-oil-hydrogen', request: { targets: [{ itemId: 1116, ratePerMinute: 120 }, { itemId: 1120, ratePerMinute: 90 }], dataset: 'Vanilla' } },
  { name: 'blue-spray-4', request: { targets: [{ itemId: 6001, ratePerMinute: 60 }], dataset: 'Vanilla', spray: 4 } },
  { name: 'blue-accelerator-2', request: { targets: [{ itemId: 6001, ratePerMinute: 60 }], dataset: 'Vanilla', spray: 2, mode: 1 } },
  { name: 'white-matrix-1', request: { targets: [{ itemId: 6006, ratePerMinute: 1 }], dataset: 'Vanilla' } },
  { name: 'blue-spray-1', request: { targets: [{ itemId: 6001, ratePerMinute: 60 }], dataset: 'Vanilla', spray: 1 } },
  { name: 'blue-spray-2', request: { targets: [{ itemId: 6001, ratePerMinute: 60 }], dataset: 'Vanilla', spray: 2 } },
  { name: 'blue-fastest-machines', request: { targets: [{ itemId: 6001, ratePerMinute: 60 }], dataset: 'Vanilla', fastest: true } },
  { name: 'blue-rounding-boundary', request: { targets: [{ itemId: 6001, ratePerMinute: 20.1 }], dataset: 'Vanilla' } },
  { name: 'fractionate-spray-10', request: { targets: [{ itemId: 6001, ratePerMinute: 60 }], dataset: 'FractionateEverything', spray: 10 } },
]
for (const key of Object.keys(datasets).map(key => path.basename(key, '.json'))) {
  if (key !== 'Vanilla') cases.push({ name: `dataset-${key}`, request: { targets: [{ itemId: 6001, ratePerMinute: 60 }], dataset: key } })
}
const results = []
for (const test of cases) {
  const req = test.request
  const mods = req.dataset === 'Vanilla' ? ['Vanilla'] : req.dataset.split('_').map(name => modGuids[name])
  const data = reference.get_game_data(mods)
  const item_data = {}
  for (let i = 0; i < data.recipe_data.length; i++) for (const name of Object.keys(data.recipe_data[i].产物)) {
    item_data[name] ??= [Object.keys(item_data).length]
    item_data[name].push(i)
  }
  const scheme = reference.init_scheme_data(data)
  if (req.fastest) for (let i = 0; i < data.recipe_data.length; i++) scheme.scheme_for_recipe[i].建筑 = data.factory_data[data.recipe_data[i].设施].length - 1
  if (req.spray) {
    for (let i = 0; i < data.recipe_data.length; i++) {
      const recipe = data.recipe_data[i]
      if (recipe.增产 & (req.mode === 1 ? 1 : 2)) scheme.scheme_for_recipe[i] = { 建筑: 0, 增产点数: req.spray, 增产模式: req.mode || 2 }
    }
  }
  const settings = structuredClone(reference.settings)
  const state = new reference.GlobalState({ game_data: data, item_data }, scheme, settings)
  const needs = Object.fromEntries(req.targets.map(target => [data.id_name_dict[target.itemId], target.ratePerMinute]))
  const [amounts, leftovers] = state.calculate(needs)
  const request = { targets: req.targets, dataset: req.dataset, scheme: { itemRecipeChoices: scheme.item_recipe_choices, schemeForRecipe: scheme.scheme_for_recipe.map(choice => ({ building: choice.建筑, increasePoints: choice.增产点数, productionMode: choice.增产模式 })) } }
  const machineIds = new Set([2302, 2315, 2319, 2303, 2304, 2305, 2318, 2308, 2309, 2317, 2310, 2901, 2902])
  const productions = []
  let powerMw = 0
  for (const [name, amount] of Object.entries(amounts)) {
    if (amount <= 1e-8) continue
    const recipeIndex = item_data[name][scheme.item_recipe_choices[name]]
    const recipe = data.recipe_data[recipeIndex]
    const selection = scheme.scheme_for_recipe[recipeIndex]
    const factory = data.factory_data[recipe.设施][selection.建筑]
    const machineId = Number(data.name_id_dict[factory.名称])
    if (!machineIds.has(machineId)) continue
    const exactMachineCount = amount / 60 / state.item_graph[name].产出倍率 / factory.倍率
    const roundedWithOffset = exactMachineCount + 0.49994 * 0.1 ** settings.fixed_num
    productions.push({mainItemId: Number(data.name_id_dict[name]), recipeId: recipe.oid, machineId, exactMachineCount, machineCount: Math.ceil(roundedWithOffset.toFixed(2))})
    const power = selection.增产模式 && selection.增产点数 ? data.proliferator_effect[selection.增产点数].耗电倍率 : 1
    powerMw += exactMachineCount * factory.耗能 * power
  }
  results.push({ name: test.name, request, amounts, leftovers, keys: state.key_item_list, productions, powerMw })
}
await fs.writeFile(output, JSON.stringify(results, null, 2))
console.log(JSON.stringify({ output, cases: results.length, referenceSource: source }))
