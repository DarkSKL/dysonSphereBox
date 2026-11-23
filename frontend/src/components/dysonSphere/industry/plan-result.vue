<script setup lang="ts">
import {computed} from 'vue'
import type {industryworkspace,industryplan} from '../../../../wailsjs/go/models'
import PlanetConditions from './planet-conditions.vue'
// =============== 结果摘要与按需施工清单 ===============
// 说明：所有产量与设施由后台核验；默认折叠长清单，预览只加载选中星球。
const props=defineProps<{report:industryworkspace.Report;itemName:(id:number)=>string;siteName:(id:number)=>string;exporting:boolean}>()
defineEmits<{(e:'preview',id:number):void;(e:'export',destination:'file'|'library'):void}>()
const statuses:Record<string,string>={ready:'规划已通过核验',conditional:'规划已通过核验，需满足所列条件',missing_data:'资源资料不足',infeasible:'当前条件无法达标',layout_not_found:'尚未找到完整布局',budget_exhausted:'当前分工未找到可用布局'}
const roles:Record<string,string>={mining:'采集',import:'星际输入',export:'星际输出',delivery:'净交付',storage:'余料入库',external:'外部供给','external-export':'外部接收','power-fuel':'能源配套'}
const number=(value:number)=>value.toLocaleString('zh-CN',{maximumFractionDigits:3})
const conditions=computed(()=>{
  const groups=new Map<number,industryplan.External[]>()
  for(const e of props.report.external??[]) {
    const entries=groups.get(e.planetId)??[]
    entries.push(e); groups.set(e.planetId,entries)
  }
  return groups
})
const otherSites=computed(()=>[...conditions.value.keys()].filter(id=>!props.report.planets?.some(p=>p.id===id)))
const routes=computed(()=>{
  const pairs=new Map<string,industryplan.Route[]>()
  for(const row of props.report.routes??[]) {
    const key=row.fromPlanetId+'-'+row.toPlanetId
    const items=pairs.get(key)??[]
    items.push(row); pairs.set(key,items)
  }
  return [...pairs.values()]
})
const warpers=computed(()=>(props.report.routes??[]).reduce((sum,r)=>sum+r.warperRatePerMinute,0))
const stageTimes=computed(()=>Object.entries(props.report.layoutMetrics?.stageMillis??{}).sort((a,b)=>b[1]-a[1]))
const adjustments=computed(()=>Object.entries(props.report.layoutMetrics?.adjustments??{}).sort((a,b)=>b[1]-a[1]))
const routeVolume=computed(()=>(props.report.routes??[]).reduce((sum,r)=>sum+r.ratePerMinute,0))
const routeSettings=computed(()=>{
  const pairs=new Set<string>(),goods=new Set<string>()
  for(const route of props.report.routes??[]){
    const a=route.fromPlanetId,b=route.toPlanetId
    const pair=[Math.min(a,b),Math.max(a,b)].join('-')
    pairs.add(pair); goods.add(pair+'-'+route.itemId)
  }
  return {pairs:pairs.size,goods:goods.size}
})
const interstellarRoutes=computed(()=>(props.report.routes??[]).filter(r=>r.interstellar).length)
const constructionRoles=computed(()=>{
  const sites=props.report.planets??[]
  const factories=sites.filter(site=>site.modules>0)
  return {manufacturing:factories.length,mining:sites.filter(site=>site.mines?.length).length,both:factories.filter(site=>site.mines?.length).length}
})
const mainWarperCargo=computed(()=>{
  const totals=new Map<number,number>()
  for(const row of props.report.routes??[]){
    if(row.warperRatePerMinute>0) totals.set(row.itemId,(totals.get(row.itemId)??0)+row.warperRatePerMinute)
  }
  return [...totals].sort((a,b)=>b[1]-a[1]||a[0]-b[0]).slice(0,3)
})
const deliverySites=computed(()=>{
  const deliveries=(props.report.planets??[]).flatMap(p=>(p.ports??[])
    .filter(port=>port.kind==='delivery').map(port=>({planet:p.id,item:port.itemId,rate:port.ratePerMinute})))
  return deliveries.some(row=>row.planet!==props.report.centerPlanetId)?deliveries:[]
})
const generation=computed(()=>{
  const counts=new Map<number,number>()
  for(const p of props.report.planets??[]){
    const power=p.power
    if(!power) continue
    for(const source of power.sources?.length?power.sources:[power]){
      if(source.facilityId>0&&source.powerMw>0&&(!('mode' in source)||source.mode!=='charge')){
        counts.set(source.facilityId,(counts.get(source.facilityId)??0)+source.count)
      }
    }
  }
  return [...counts.entries()].sort((a,b)=>a[0]-b[0])
})
const photonSites=computed(()=>(props.report.planets??[]).flatMap(p=>p.photons?[{id:p.id,name:p.name,rate:p.photons.ratePerMinute}]:[]))
const externalPhotons=computed(()=>(props.report.external??[]).filter(e=>e.itemId===1208&&e.kind==='declared-input').reduce((sum,e)=>sum+e.ratePerMinute,0))
const surplusFlows=computed(()=>(props.report.planets??[]).flatMap(site=>(site.ports??[])
  .filter(port=>(port.kind==='storage'||(port.kind==='external-export'&&!port.peerPlanetId))&&port.ratePerMinute>0)
  .map(port=>({site:site.id,item:port.itemId,rate:port.ratePerMinute,minutes:(port.storageItems??0)/port.ratePerMinute}))))
</script>

<template>
  <!-- ~ 摘要不展示候选预算、内部错误码或建筑实体编号 -->
  <section class="industry-result">
    <div class="result-head"><h2 role="status">{{statuses[report.status]??'规划已结束'}}</h2><div v-if="report.exportable" class="export-actions"><n-button size="small" type="primary" :loading="exporting" @click="$emit('export',report.blueprintCount===1?'file':'library')">{{report.blueprintCount===1?'保存蓝图 TXT':'写入游戏蓝图库'}}</n-button><n-button v-if="report.blueprintCount>1" size="small" :disabled="exporting" @click="$emit('export','file')">导出分享 ZIP</n-button></div></div>
    <p>{{report.saveName}}<template v-if="report.centerPlanetId"> · 规划中心 <strong>{{siteName(report.centerPlanetId)}}</strong></template></p>
    <p>{{report.planets?.length??0}} 颗{{report.exportable?'建设':'候选'}}星球 · {{report.blueprintCount}} 张施工图 · {{routes.length}} 组星球间运输<span v-if="routes.length">（{{routeSettings.pairs}} 对星球，按方向分组） · {{report.routes.length}} 项货运 · 预计 {{number(warpers)}} 翘曲器/分</span></p>
    <p v-if="report.exportable&&report.planets.length>1">{{constructionRoles.manufacturing}} 颗星承担制造，{{constructionRoles.mining}} 颗星承担采集<span v-if="constructionRoles.both">，其中 {{constructionRoles.both}} 颗两者兼有</span>。</p>
    <p v-for="change in report.technologyChanges" :key="change">模拟条件 · {{change}}</p>
    <details v-if="report.layoutMetrics?.rebuilds">
      <summary>生成耗时明细</summary>
      <p>逐星布局与核验 {{number(report.layoutMetrics.rebuildMillis/1000)}} 秒，共重建 {{report.layoutMetrics.rebuilds}} 次，复用已有布局 {{report.layoutMetrics.reuses}} 次。分工修正 {{number(report.layoutMetrics.repairMillis/1000)}} 秒。</p>
      <ul v-if="stageTimes.length"><li v-for="[label,ms] in stageTimes" :key="label">{{label}} · {{number(ms/1000)}} 秒</li></ul>
      <p v-if="adjustments.length">分工调整原因 · {{adjustments.map(([reason,count])=>reason+' '+count+' 次').join('、')}}</p>
    </details>
    <div v-for="(issue,index) in report.issues" :key="index" class="issue"><strong>{{issue.cause}}</strong><p>{{issue.action}}</p></div>
    <div class="target-totals"><span v-for="(target,index) in report.targets" :key="index">{{itemName(target.itemId)}}目标 <strong>{{number(target.ratePerMinute)}}</strong> 件/分</span></div>
    <p v-if="report.exportable">满足所列条件后的规划净产量 · <span v-for="(target,index) in report.achievedTargets" :key="index">{{itemName(target.itemId)}} {{number(target.ratePerMinute)}} 件/分　</span></p>
    <p v-if="report.exportable&&deliverySites.length">实际交付地点 · <span v-for="row in deliverySites" :key="row.planet+'-'+row.item">{{siteName(row.planet)}} 收取{{itemName(row.item)}} {{number(row.rate)}} 件/分　</span></p>
    <p v-if="report.exportable&&generation.length">配套供电 · {{generation.map(([id,count])=>itemName(id)+' × '+number(count)).join('、')}}</p>
    <p v-if="report.exportable&&(photonSites.length||externalPhotons)">临界光子来源 · <span v-for="site in photonSites" :key="site.id">{{site.name}} 接收 {{number(site.rate)}} 个/分　</span><span v-if="externalPhotons">玩家外供 {{number(externalPhotons)}} 个/分</span></p>
    <div v-if="report.exportable&&surplusFlows.length" class="issue"><strong>稳产需要持续处理副产物</strong><p v-for="flow in surplusFlows" :key="flow.site+'-'+flow.item">{{siteName(flow.site)}}每分钟需消纳 {{number(flow.rate)}} 个{{itemName(flow.item)}}。<span v-if="flow.minutes>0">按规划容量，空仓约 {{number(flow.minutes)}} 分钟装满，之后相关产线会堵塞。</span></p></div>
    <template v-if="report.extraTargets?.length"><h3>充分开发带来的额外产量</h3><div class="target-totals"><span v-for="(target,index) in report.extraTargets" :key="index">{{itemName(target.itemId)}} +{{number(target.ratePerMinute)}} 件/分</span></div><p>{{report.expansionNote}}</p></template>
    <section v-if="report.resourceGaps?.length"><h3>采集供给缺口</h3><div class="table-scroll"><table><thead><tr><th>原料</th><th>需求 / 分</th><th>已查矿源的速率上限 / 分</th><th>调查情况</th></tr></thead><tbody><tr v-for="gap in report.resourceGaps" :key="gap.itemId"><td>{{itemName(gap.itemId)}}</td><td>{{number(gap.requiredPerMinute)}}</td><td>{{number(gap.availablePerMinute)}}</td><td>{{gap.pendingPlanets?'尚有待查星球':'候选已查完'}}<span v-if="gap.unknownPlanets">，{{gap.unknownPlanets}} 颗资料不足</span></td></tr></tbody></table></div><p>实际采速还取决于能放下的设备和接通的传送带。</p></section>
    <details v-if="report.warnings?.length"><summary>施工与运行说明 · {{report.warnings.length}} 项</summary><ul><li v-for="warning in report.warnings" :key="warning">{{warning}}</li></ul></details>
    <details v-for="id in otherSites" :key="id"><summary>{{siteName(id)}} · 外部配套</summary><PlanetConditions :entries="conditions.get(id)??[]" :item-name="itemName" :site-name="siteName"/></details>
    <details v-if="routes.length">
      <summary>{{report.exportable?'运输线路':'候选运输线路'}} · {{routes.length}} 组</summary>
      <p>合计 {{number(routeVolume)}} 件/分，{{report.routes.length-interstellarRoutes}} 项在同一恒星系内，{{interstellarRoutes}} 项跨恒星系。翘曲器按满载往返估算，同系运输达到设定的曲速距离也会计入。发货塔从空仓启动时须等第一船装满，可预存一船启动物资。</p>
      <p v-if="mainWarperCargo.length">翘曲器消耗最多的货物 · {{mainWarperCargo.map(([id,cost])=>itemName(id)+' '+number(cost)+' 个/分').join('、')}}</p>
      <p v-if="report.remoteGroup">新塔使用空闲站组 {{report.remoteGroup}}。原版天体航线按星球对和货物设置，同一货物往返两地共用一项。本表涉及 {{routeSettings.goods}} 个这样的组合，请核对已有航线。同一货物发往多地时，逐个检查收货星和到货量。</p>
      <details v-for="pair in routes" :key="pair[0].fromPlanetId+'-'+pair[0].toPlanetId">
        <summary>{{siteName(pair[0].fromPlanetId)}} → {{siteName(pair[0].toPlanetId)}} · {{pair.length}} 项货运</summary>
        <div class="table-scroll"><table>
          <thead><tr><th>货物</th><th>件/分</th><th>往返</th><th>翘曲器/分</th><th>发货塔用电</th></tr></thead>
          <tbody><tr v-for="(route,index) in pair" :key="index"><td>{{itemName(route.itemId)}}</td><td>{{number(route.ratePerMinute)}}</td><td>{{number(route.roundTripSeconds)}} 秒</td><td>{{number(route.warperRatePerMinute)}}</td><td>{{number(route.vesselPowerMw??0)}} MW</td></tr></tbody>
        </table></div>
      </details>
    </details>
    <details v-if="report.planets?.length"><summary>{{report.exportable?'逐星施工':'候选分工'}} · {{report.planets.length}} 颗星球</summary>
      <details v-for="planet in report.planets" :key="planet.id" class="planet-detail"><summary>{{planet.star}} / {{planet.name}}<span>{{planet.modules}} 个模块 · {{planet.buildings}} 座建筑</span></summary>
        <p>{{planet.stations}} 座物流塔 · {{planet.vessels}} 艘运输船 · {{planet.foundationCells}} 格地基<span v-if="planet.mines?.length"> · 采集{{planet.mines.map(m=>itemName(m.itemId)).join('、')}}</span><span v-if="planet.id===report.centerPlanetId"> · 规划中心</span></p>
        <div v-for="(sheet,i) in planet.sheets" :key="sheet.id" class="result-head"><p>{{i+1}}. {{sheet.name}} · {{number(sheet.buildings)}} 座建筑、{{number(sheet.foundationCells)}} 格地基</p><n-button size="tiny" @click="$emit('preview',sheet.id)">预览施工图</n-button></div>
        <PlanetConditions :entries="conditions.get(planet.id)??[]" :item-name="itemName" :site-name="siteName"/>
        <details v-if="planet.fleet?.length">
          <summary>发货塔配船 · {{planet.fleet.length}} 座塔，共 {{planet.vessels}} 艘</summary>
          <p>新塔已关闭自动补充运输船。只在下列发货塔装入所列数量，其余新塔留空，再按运输线路核对去向。同塔各货物共用这些船。</p>
          <div class="table-scroll"><table>
            <thead><tr><th>塔的位置</th><th>装入运输船</th><th>远程供应货物</th></tr></thead>
            <tbody><tr v-for="station in planet.fleet" :key="station.buildingIndex">
              <td>经度 {{number(station.longitude)}}°，纬度 {{number(station.latitude)}}°</td>
              <td>{{station.vessels}} 艘</td>
              <td>{{station.cargo.map(c=>itemName(c.itemId)).join('、')}}</td>
            </tr></tbody>
          </table></div>
        </details>
        <p v-if="planet.file">经度 {{number(planet.longitude)}}° · 纬度 {{number(planet.latitude)}}° · 朝向 {{number(planet.direction)}}°</p>
        <p v-if="planet.foundationCells">沙土需求 <span v-if="planet.soilEstimateKnown">{{number(planet.soilLowerBound)}} 至 {{number(planet.soilUpperBound)}}</span><span v-else>待核实地形</span></p>
        <template v-if="planet.power?.sources?.length"><p v-for="(source,index) in planet.power.sources" :key="index">{{itemName(source.facilityId)}} × {{source.count}}<span v-if="source.powerMw"> · {{number(source.powerMw)}} MW</span><span v-if="source.fuelItemId"> · {{itemName(source.fuelItemId)}} {{number(source.fuelRatePerMinute)}} 件/分</span><span v-if="source.storedMj"> · 储能 {{number(source.storedMj)}} MJ</span><span v-if="source.mode==='charge'"> · 充电</span><span v-if="source.mode==='discharge'"> · 放电</span></p></template>
        <p v-else-if="planet.power?.facilityId">{{itemName(planet.power.facilityId)}} × {{planet.power.count}} · {{number(planet.power.powerMw)}} MW</p>
        <p v-if="planet.power">实装持续负荷 {{number(planet.power.requiredMw)}} MW<span v-if="planet.power.externalGridMw"> · 已有电网可用余量 {{number(planet.power.externalGridMw)}} MW</span></p>
        <p v-if="planet.outboundVesselMw">其中运输船满载发货约用 {{number(planet.outboundVesselMw)}} MW，按本星出发、回程空载计算。</p>
        <p v-if="planet.photons">光子接收站 × {{planet.photons.count}} · {{number(planet.photons.ratePerMinute)}} 光子/分 · 占用戴森球 {{number(planet.photons.spherePowerMw)}} MW</p>
        <p v-for="warning in planet.warnings" :key="warning">{{warning}}</p>
        <details v-if="planet.mines?.length"><summary>采集与可用储量</summary><ul><li v-for="mine in planet.mines" :key="mine.itemId">{{itemName(mine.itemId)}} · {{itemName(mine.facilityId)}} × {{mine.count}} · {{number(mine.ratePerMinute)}} 件/分<span v-if="mine.coveredItems!=null&&mine.availableMinutes!=null"> · 覆盖余量 {{number(mine.coveredItems)}} 件，按 100% 耗矿折合 {{number(mine.availableMinutes)}} 分钟</span></li></ul></details>
        <details v-if="planet.ports?.length"><summary>物料收支</summary><ul><li v-for="(port,index) in planet.ports" :key="index">{{roles[port.kind]??'物料配套'}} · {{itemName(port.itemId)}} {{number(port.ratePerMinute)}} 件/分<span v-if="port.storageItems&&port.ratePerMinute"> · 空仓约 {{number(port.storageItems/port.ratePerMinute)}} 分钟装满</span></li></ul></details>
        <details v-if="planet.existingSlots?.length"><summary>已有物流货槽需核对 · {{planet.existingSlots.length}} 处</summary><p>已有本地货槽可能与新塔自动配对，开工前请检查相应物品的供需设置。</p><ul><li v-for="(slot,index) in planet.existingSlots" :key="index">{{itemName(slot.itemId)}} · 第 {{slot.slot}} 槽 · {{slot.remote?'已有天体航线':'本地'}}{{slot.logic===1?'供应':'需求'}}</li></ul></details>
        <details v-if="planet.materials?.length"><summary>施工材料与采集位置</summary><ul class="materials"><li v-for="item in planet.materials" :key="item.itemId">{{itemName(item.itemId)}} × {{item.count}}</li></ul><ul><li v-for="(mine,index) in planet.minePlacements" :key="index">{{itemName(mine.facilityId)}} · 经度 {{number(mine.longitude)}}° · 纬度 {{number(mine.latitude)}}° · 覆盖 {{mine.veinIds?.length??0}} 处矿点</li></ul></details>
      </details>
    </details>
  </section>
</template>

<style scoped lang="scss">
.export-actions{display:flex;gap:8px;flex-wrap:wrap}
.industry-result{padding:18px}.result-head,.target-totals{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}h2,h3,p{margin:0}h2{font-size:16px}h3{font-size:14px;margin-top:12px}p,li,td{font-size:12px}p{color:var(--ds-text-secondary);line-height:1.7;margin:6px 0}.target-totals{justify-content:flex-start;margin:12px 0;font-size:13px;font-variant-numeric:tabular-nums}.issue{border-left:2px solid var(--ds-border-strong);padding:8px 12px;margin:12px 0;font-size:13px}.issue p{margin-bottom:0}details{border-top:1px solid var(--ds-border-subtle);margin-top:12px;padding-top:10px}summary{cursor:pointer;font-size:13px;font-weight:600;line-height:1.8}summary span{font-weight:400;color:var(--ds-text-secondary);margin-left:10px}.planet-detail{margin-left:10px}ul{margin:8px 0;padding-left:20px;line-height:1.8}.materials{columns:2}.table-scroll{overflow:auto}table{border-collapse:collapse;width:100%;font-variant-numeric:tabular-nums}td,th{text-align:left;padding:7px 10px 7px 0;border-bottom:1px solid var(--ds-border-subtle);font-size:12px}th{font-weight:500;color:var(--ds-text-secondary)}
</style>
