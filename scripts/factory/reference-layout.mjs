// 职责：运行本地参考机器行生成器，输出几何与端口 golden；使用：DSBOX_LAYOUT_GOLDEN；边界：仅临时产物，不复制参考源码入库。
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import {createRequire} from 'node:module'
import {pathToFileURL} from 'node:url'
import {build} from '../../frontend/node_modules/esbuild/lib/main.js'
const source = process.env.DSBOX_REFERENCE_SOURCE
const dependencies = process.env.DSBOX_REFERENCE_NODE_MODULES
const output = process.env.DSBOX_LAYOUT_GOLDEN
if (!source || !dependencies || !output) throw new Error('缺少参考目录、依赖目录或输出路径')
const require = createRequire(import.meta.url)
const temp = await fs.mkdtemp(path.join(os.tmpdir(), 'dsbox-layout-golden-'))
const entry = path.join(temp, 'entry.mjs')
await fs.writeFile(entry, `export {ProductLineBuildInfo} from ${JSON.stringify(path.join(source,'src/types/ProductLineBuildInfo.ts'))}; export {Setting} from ${JSON.stringify(path.join(source,'src/types/Setting.ts'))}; export {toStr} from ${JSON.stringify(path.join(source,'src/utils/parser.js'))}; export {BuleprintMaker} from ${JSON.stringify(path.join(source,'src/utils/BuleprintMaker.ts'))}; export {LocalStation} from ${JSON.stringify(path.join(source,'src/types/LocalStation.ts'))}; export {InterstellarStation} from ${JSON.stringify(path.join(source,'src/types/InterstellarStation.ts'))}; export {MixDistributeHelper} from ${JSON.stringify(path.join(source,'src/utils/MixDistributeHelper.ts'))};`)
const bundled = path.join(temp,'reference.mjs')
await build({entryPoints:[entry],outfile:bundled,bundle:true,platform:'node',format:'esm',target:'es2020',logLevel:'silent',
  banner:{js:"import {createRequire} from 'node:module'; const require = createRequire(import.meta.url);"},
  plugins:[{name:'local-reference',setup(api){
    api.onResolve({filter:/^@\//},async args=>{
      const base = path.join(source,'src',args.path.slice(2))
      for (const extension of ['', '.ts', '.js', '.jsx']) { try { await fs.access(base+extension); return {path:base+extension} } catch {} }
      throw new Error(`无法解析 ${args.path}`)
    })
    api.onResolve({filter:/^pako$/},()=>({path:require.resolve('pako',{paths:[dependencies]})}))
    api.onResolve({filter:/^antd$/},()=>({path:'antd',namespace:'mock'}))
    api.onLoad({filter:/.*/,namespace:'mock'},()=>({contents:'export const message = {warning: console.warn, error: console.error, success: console.log}',loader:'js'}))
  }}]})
const reference = await import(pathToFileURL(bundled))
const data = JSON.parse((await fs.readFile(path.join(source,'data/Vanilla.json'),'utf8')).replace(/^\uFEFF/,''))
const groups = [[2302,2315,2319],[2303,2304,2305,2318],[2308],[2309,2317],[2310],[2901,2902]]
const cases = []
for (let typ=0;typ<groups.length;typ++) {
  const recipe = data.recipes.find(r=>r.Factories?.includes(groups[typ][0]) && r.Items.length>0 && r.Items.length<=5)
  for (const mode of [1,2,5]) for (const count of [1,5]) {
    const production = {recipeId:recipe.ID,machineType:typ+1,machineLv:1,machineId:groups[typ][0],machineCount:count,productionMode:-1,increaseLv:0,increasePoints:0,mainItemId:recipe.Results[0]}
    const setting = new reference.Setting(); setting.modeType=mode
    const line = new reference.ProductLineBuildInfo(recipe.ID,count,production,1,setting,0,true)
    const document = reference.BuleprintMaker.createEmptyBlueprintData(); document.buildings=line.allbuilds
    cases.push({production,mode,blueprint:reference.toStr(document)})
  }
}
await fs.writeFile(output,JSON.stringify(cases))
const stations=[]
for (const mode of [3,4]) {
  const station=mode===3?new reference.LocalStation():new reference.InterstellarStation()
  station.setStorageInfo(0,1101,true);station.setStorageInfo(1,1001,false)
  const document=reference.BuleprintMaker.createEmptyBlueprintData();document.buildings=[station]
  stations.push({mode,blueprint:reference.toStr(document)})
}
await fs.writeFile(output+'.stations.json',JSON.stringify(stations))
const allocations=data.recipes.filter(r=>r.Items.length>0&&r.Items.length<=6).map(r=>({recipeId:r.ID,plans:new reference.MixDistributeHelper(r.ID).plans.map(p=>({itemId:p.itemId,expected:p.except,real:p.real}))}))
await fs.writeFile(output+'.mix.json',JSON.stringify(allocations))
console.log(JSON.stringify({output,cases:cases.length,stations:stations.length,allocations:allocations.length}))
