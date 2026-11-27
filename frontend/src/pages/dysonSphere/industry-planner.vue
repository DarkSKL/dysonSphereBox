<script setup lang="ts">
import {computed, reactive} from 'vue'
import {FolderOpen, Info, MapPin, Orbit, Plus, X} from '@lucide/vue'
import {industryplan} from '../../../wailsjs/go/models'
import {useIndustryPlanner} from '../../composables/dysonSphere/useIndustryPlanner'
import PlanResult from '../../components/dysonSphere/industry/plan-result.vue'
import StarMap from '../../components/dysonSphere/industry/star-map.vue'
import EnergyChoice from '../../components/dysonSphere/industry/energy-choice.vue'
import ViewerPanel from '../../components/dysonSphere/blueprint/viewer-panel.vue'
import ItemIcon from '../../components/dysonSphere/factory/item-icon.vue'
import IconSelect from '../../components/dysonSphere/factory/icon-select.vue'
import {ensureFactoryIcons} from '../services/dysonSphere/factory-icons'

definePage({meta:{layout:'main',title:'特殊蓝图',keepAlive:true}})
const w=reactive(useIndustryPlanner())
ensureFactoryIcons() // 交付目标卡片的物品图标依赖图鉴精灵索引
// =============== 工作区状态 ===============
// 说明：选星只改变检查面板；生成与预览由用户触发。
const selectedPlan=computed(()=>w.report?.planets.find(p=>p.id===w.resourceSite))
const selectedEnergy=computed(()=>w.energy.overrides.findIndex(p=>p.planetId===w.resourceSite))
// 星图标记：结果中的中心星球优先，未生成时显示手动指定的中心；交付点取自各目标。
const mapCenter=computed(()=>w.report?.centerPlanetId||w.form.center||null)
const mapDeliveries=computed(()=>w.targets.map(t=>t.delivery).filter((id):id is number=>id!=null))
const starOptions=computed(()=>(w.selection?.starSystems??[]).map(s=>({label:s.name,value:s.starId})))
const receiverOptions=(id:number)=>(w.selection?.sites??[]).filter(p=>!p.gas&&p.starId===id).map(p=>({label:p.name,value:p.id}))
const addSphere=()=>w.dyson.supplies.push(new industryplan.DysonSupply({starId:starOptions.value[0]?.value??0,planetIds:[],availableMw:1000,efficiency:.8,uptime:1}))
const preferences=[{label:'建造简单',value:'construction'},{label:'减少运输',value:'logistics'},{label:'减少能耗',value:'energy'},{label:'减少游戏负载',value:'game-load'}]
const foundations=[{label:'不铺地基',value:'none'},{label:'仅施工区域',value:'construction'},{label:'整星陆地区域',value:'planet'}]
const levels=[1,2,3,4].map(value=>({value,label:'Mk.'+['I','II','III','IV'][value-1]}))
const phases:Record<string,string>={quant:'计算物料平衡',resources:'调查矿源',assign:'安排星球分工',blueprint:'生成并核验蓝图',repair:'调整局部布局',verified:'核验完成',expand:'开发选中星球的剩余空间'}
const sources:Record<string,string>={saved:'存档记录',generated:'已核对资源',candidate:'待精算矿点',unknown:'资料不足'}
const number=(v:number)=>v.toLocaleString('zh-CN',{maximumFractionDigits:2})
// =============== 存档选项 ===============
// 说明：游戏自动生成的 _lastexit_ / _autosave_* 存档名对玩家不直观，单独分组并换成可读名称、附修改时间；手动存档保持原名在前。
const autoSaveLabel=(name:string)=>{
  if(name==='_lastexit_')return '退出游戏时自动保存'
  if(name==='_autosave_errored')return '出错时自动保存'
  const slot=/^_autosave_(\d+)$/.exec(name)
  return slot?`自动保存 ${Number(slot[1])+1}`:null
}
const savedAt=(value:string)=>{const d=new Date(value);return Number.isNaN(d.getTime())?'':d.toLocaleString('zh-CN',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'})}
const saveOptions=computed(()=>{
  const manual:{label:string,value:string}[]=[],auto:{label:string,value:string}[]=[]
  for(const s of w.saves){const label=autoSaveLabel(s.name);if(label){const time=savedAt(s.lastModifiedAt);auto.push({label:time?`${label} · ${time}`:label,value:s.id})}else manual.push({label:s.name,value:s.id})}
  if(!auto.length)return manual
  return [...(manual.length?[{type:'group',label:'我的存档',key:'manual',children:manual}]:[]),{type:'group',label:'游戏自动存档',key:'auto',children:auto}]
})

// =============== 生成进度 ===============
// 说明：显示后台排除暂停后的用时。充分开发按当前扩产轮次显示预计范围。
function duration(seconds:number, approximate = false) {
  if (seconds < 60) return `${seconds} 秒`
  const minutes = approximate ? Math.ceil(seconds / 60) : Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} 分钟`
  const hours = Math.floor(minutes / 60), rest = minutes % 60
  return `${hours} 小时${rest ? ` ${rest} 分钟` : ''}`
}
const estimateLabel = computed(() => {
  const task = w.task
  if (!task?.estimate) return '正在按目标估算时间…'
  const low = duration(task.estimate.minSeconds, true), high = duration(task.estimate.maxSeconds, true)
  const scope = task.expanding ? '本轮扩产' : w.form.developFully ? '目标产线' : ''
  return `${scope}预计还需 ${low === high ? low : `${low} 至 ${high}`}`
})
const phaseLabel = computed(() => {
  if (w.task?.state === 'cancelling') return '正在取消规划'
  if (w.paused) return '计算已暂停'
  if (w.running) return phases[w.task?.progress.phase ?? ''] ?? '正在准备规划'
  return w.report ? '规划结果已更新' : '准备规划'
})
</script>

<template>
  <!-- ~ 特殊蓝图工作台：紧凑输入、真实星图与按需检查 -->
  <main class="industry-page">
    <header class="toolbar">
      <div class="page-title"><Orbit :size="19"/><h1>特殊蓝图</h1><span class="active-tool">跨星球产业线</span></div>
      <div class="save-toolbar">
        <span class="directory-mode" :title="w.root||w.suggested">{{w.temporaryRoot?'临时目录':'跟随游戏设置'}}</span>
        <n-select class="save-select" size="small" filterable :value="w.saveId" :loading="w.busy" :disabled="w.busy" :options="saveOptions" placeholder="选择存档" @update:value="w.chooseSave"/>
        <n-button size="small" quaternary :disabled="w.busy" title="临时选择其他存档目录" @click="w.openDirectory(true)"><FolderOpen :size="17"/></n-button>
        <n-button v-if="w.temporaryRoot" size="tiny" @click="w.restoreDirectory">恢复跟随</n-button>
      </div>
    </header>
    <div class="workbench">
      <aside class="controls">
        <!-- ~ 主要目标保持可见；高级选项按需展开 -->
        <!-- ~ 交付目标：与「量化与蓝图」目标卡片同一形态——图标 + 物品 + 速率一行，交付星球作为次要行 -->
        <section class="control-section">
          <h2 class="section-head">交付目标<span class="count">{{w.targets.length}}</span></h2>
          <div class="target-list">
            <div v-for="(target,index) in w.targets" :key="index" class="target-card ds-bracket">
              <ItemIcon :name="target.item?w.itemName(target.item):''" :size="30"/>
              <div class="target-main">
                <IconSelect v-model:value="target.item" filterable :options="w.items" :icon-size="0" placeholder="选择物品" class="target-item" aria-label="交付物品"/>
                <n-input-number v-model:value="target.rate" size="tiny" :min=".001" :show-button="false" class="target-rate" aria-label="净产量（件/分钟）"><template #suffix>/min</template></n-input-number>
                <n-select v-model:value="target.delivery" size="tiny" filterable clearable :options="w.sites" class="target-delivery" placeholder="交付至中心星球" aria-label="交付星球"><template #arrow><MapPin :size="12"/></template></n-select>
              </div>
              <button type="button" class="icon-button" :disabled="w.targets.length<=1" aria-label="移除目标" title="移除目标" @click="w.targets.splice(index,1)"><X :size="13"/></button>
            </div>
            <button type="button" class="target-add" :disabled="w.targets.length>=100" @click="w.targets.push({item:null,rate:60,delivery:null})"><Plus :size="13"/>添加目标</button>
          </div>
          <label>中心星球<n-select size="small" filterable clearable v-model:value="w.form.center" :options="w.sites" placeholder="自动选择"/></label>
          <label v-if="w.selection?.technology">
            <span class="label-line">规划科技<n-tooltip><template #trigger><Info :size="12" class="label-info" tabindex="0" aria-label="存档科技详情"/></template>存档采矿 ×{{number(w.selection.technology.miningSpeed)}} · 运输船 {{w.selection.technology.shipCargo}} 件/艘 · 运输机 {{w.selection.technology.droneCargo}} 件/架<br/>单图最多 {{number(w.selection.technology.blueprintLimit)}} 座建筑、{{number(w.selection.technology.foundationLimit)}} 格地基</n-tooltip></span>
            <n-select size="small" :value="w.form.technologyMode" :options="[{label:'使用存档科技',value:'save'},{label:'手动模拟科技',value:'custom'}]" @update:value="w.setTechnology"/>
          </label>
          <n-tooltip placement="right"><template #trigger><n-checkbox v-model:checked="w.form.developFully">充分开发规划星球</n-checkbox></template>先满足目标，再利用这些星球的剩余空间按比例扩产，额外产量单独列出</n-tooltip>
        </section>
        <section class="control-section">
          <div class="section-title"><h2>能源</h2><n-checkbox v-model:checked="w.form.power">配套供电</n-checkbox></div>
          <EnergyChoice v-if="w.form.power" v-model="w.energy.default" :sites="w.sites" :item-name="w.itemName"/>
          <p v-else class="hint">结果会列出需要接入的电网功率。</p>
          <n-checkbox v-model:checked="w.form.fuel">制造发电燃料与透镜</n-checkbox>
        </section>
        <section class="control-section">
          <h2>临界光子</h2>
          <n-select size="small" v-model:value="w.dyson.mode" :options="[{label:'尚未配置戴森球',value:'unconfigured'},{label:'已有戴森球，自行接收',value:'sphere'},{label:'由玩家持续外供光子',value:'external'}]"/>
          <template v-if="w.dyson.mode==='sphere'">
            <div v-for="(sphere,index) in w.dyson.supplies" :key="index" class="sphere-source">
              <div class="target-row"><n-select size="small" :value="sphere.starId" filterable :options="starOptions" placeholder="戴森球所在恒星" @update:value="(id:number)=>{sphere.starId=id;sphere.planetIds=[]}"/><n-button size="tiny" quaternary title="移除戴森球" @click="w.dyson.supplies.splice(index,1)"><X :size="14"/></n-button></div>
              <n-select size="small" v-model:value="sphere.planetIds" multiple filterable :options="receiverOptions(sphere.starId)" placeholder="选择接收星球"/>
              <label>扣除其他用途后的可用功率 · MW<n-input-number size="small" v-model:value="sphere.availableMw" :min="1"/></label>
              <div class="two-fields"><label>接收效率<n-input-number size="small" v-model:value="sphere.efficiency" :min=".01" :max="1" :step=".05"/></label><label>可保证连续率<n-input-number size="small" v-model:value="sphere.uptime" :min=".01" :max="1" :step=".05"/></label></div>
            </div>
            <n-button size="tiny" :disabled="!w.selection" @click="addSphere">添加戴森球</n-button>
            <n-checkbox v-model:checked="w.dyson.lenses">使用普通引力透镜</n-checkbox>
            <p class="hint">光子和射线发电共用功率额度。接收效率、连续率按所选位置确认。</p>
          </template>
        </section>
        <details class="control-section"><summary>建造与量化设置</summary>
          <div class="check-list"><n-checkbox v-model:checked="w.form.mining">新建采矿设施</n-checkbox><n-checkbox v-model:checked="w.form.logistics">规划星际运输</n-checkbox><n-checkbox v-model:checked="w.form.storage">储存剩余副产物</n-checkbox><n-checkbox v-model:checked="w.form.spray">增产剂喷涂</n-checkbox><n-checkbox v-model:checked="w.form.buffer">中转箱与储液罐</n-checkbox></div>
          <label>优化偏好<n-select size="small" v-model:value="w.form.preference" :options="preferences"/></label>
          <label>计算档位<n-select size="small" v-model:value="w.form.profile" :options="[{label:'标准',value:'standard'},{label:'低算力',value:'low'}]"/></label>
          <n-checkbox v-model:checked="w.form.pauseBetweenWindows">分段计算，每 3 分钟暂停</n-checkbox>
          <label>固体采矿设备<n-select size="small" v-model:value="w.form.facility" :options="[{label:'小型采矿机',value:2301},{label:'大型采矿机',value:2316}]"/></label>
          <label>采矿科技倍率<n-input-number size="small" v-model:value="w.form.miningSpeed" :disabled="w.form.technologyMode==='save'" :min=".1" :step=".1"/></label>
          <label v-if="w.form.facility===2316">大矿机工作倍率<n-input-number size="small" v-model:value="w.form.minerSpeed" :min="1" :max="3" :step=".1"/></label>
          <label>地基<n-select size="small" v-model:value="w.form.foundation" :options="foundations"/></label>
          <template v-if="w.form.storage"><label>余料仓储星<n-select size="small" v-model:value="w.form.warehouse" :options="w.sites" filterable clearable placeholder="自动选择"/></label><label>空仓缓冲 · 分钟<n-input-number size="small" v-model:value="w.form.minutes" :min="1"/></label></template>
          <template v-if="w.form.spray"><label>喷涂方式<n-select size="small" v-model:value="w.form.sprayMode" :options="[{label:'额外产出',value:2},{label:'生产加速',value:1}]"/></label><label>增产等级<n-select size="small" v-model:value="w.form.sprayPoints" :options="[1,2,4].map(value=>({label:value+' 点',value}))"/></label></template>
          <div class="two-fields"><label>传送带<n-select size="small" v-model:value="w.form.belt" :options="levels.slice(0,3)"/></label><label>分拣器<n-select size="small" v-model:value="w.form.sorter" :options="levels"/></label></div>
        </details>
        <details class="control-section"><summary>配方与玩家外供</summary>
          <label>持续外供物品<n-select size="small" multiple filterable clearable v-model:value="w.form.externalItems" :options="w.items" placeholder="原料或半成品"/></label>
          <div class="target-row"><n-select size="small" filterable v-model:value="w.ruleItem" :options="w.items" placeholder="指定配方的物品"/><n-button size="tiny" :disabled="!w.ruleItem" @click="w.addRule">添加</n-button></div>
          <div v-for="(rule,index) in w.rules" :key="rule.item" class="recipe"><div class="section-title"><b>{{w.itemName(rule.item)}}</b><n-button size="tiny" quaternary @click="w.rules.splice(index,1)">移除</n-button></div><n-select size="small" :value="rule.choice" :options="w.recipeChoices(rule)" @update:value="(value:number)=>{rule.choice=value;rule.building=0}"/><n-select v-if="rule.choice>0" size="small" v-model:value="rule.building" :options="w.buildings(rule)"/></div>
        </details>
        <details class="control-section"><summary>运输船与航程</summary>
          <label>单船载货 · 件<n-input-number size="small" v-model:value="w.form.cargo" :disabled="w.form.technologyMode==='save'" :min="1"/></label>
          <label>巡航速度 · 米/秒<n-input-number size="small" v-model:value="w.form.cruise" :disabled="w.form.technologyMode==='save'" :min="1"/></label>
          <n-checkbox v-model:checked="w.form.warpEnabled" :disabled="w.form.technologyMode==='save'">启用曲速航行</n-checkbox>
          <label v-if="w.form.warpEnabled">曲速 · 米/秒<n-input-number size="small" v-model:value="w.form.warp" :disabled="w.form.technologyMode==='save'" :min="1"/></label>
          <label v-if="w.form.warpEnabled">启用曲速距离 · 米<n-input-number size="small" v-model:value="w.form.warpDistance" :min="20000" :max="2147483647" :precision="0"/></label><label>额外装卸等候 · 秒<n-input-number size="small" v-model:value="w.form.turnaround" :min="1"/></label><label>每塔船数<n-input-number size="small" v-model:value="w.form.ships" :min="1" :max="10"/></label>
        </details>
      </aside>
      <section class="visual-workspace" :class="{'has-report':!!w.report}">
        <div class="map-wrap">
          <StarMap v-if="w.selection" :selection="w.selection" :report="w.report" :selected="w.resourceSite" :center="mapCenter" :deliveries="mapDeliveries" :loading="w.busy" @select="(id:number)=>w.resourceSite=id"/>
          <div v-else class="empty-map"><Orbit :size="56" :stroke-width=".8"/><h2>{{w.busy?'正在读取星区':'从一份存档开始'}}</h2><p>选择顶部存档，查看资源与可建设星球。</p><n-button v-if="!w.saves.length&&!w.busy" size="small" @click="w.openDirectory(true)">选择存档目录</n-button></div>
          <aside v-if="w.selection" class="inspector">
            <n-select size="small" filterable clearable v-model:value="w.resourceSite" :options="w.allSites" placeholder="搜索或点击星球"/>
            <template v-if="w.currentResources">
              <div class="section-title"><h2>{{w.currentResources.name}}</h2><span>{{w.currentResources.gas?'气态巨星':'固态星球'}}</span></div>
              <p class="hint">{{w.currentResources.star}} · {{sources[w.currentResources.source]??'资料待确认'}}</p>
              <div v-if="selectedPlan" class="planet-summary"><strong>{{selectedPlan.modules}} 个制造模块</strong><span>{{number(selectedPlan.buildings)}} 座建筑</span><span>{{number(selectedPlan.power?.powerMw??selectedPlan.powerMw)}} MW</span><n-button v-for="(s,i) in selectedPlan.sheets" :key="s.id" size="small" @click="w.showPreview(s.id)">预览施工图 {{i+1}}</n-button></div>
              <details><summary>星球资源 · {{w.currentResources.resources.length}} 种</summary><ul><li v-for="res in w.currentResources.resources" :key="res.itemId"><span>{{w.itemName(res.itemId)}}</span><span v-if="res.availableCount!=null">{{number(res.availableCount)}} 处可用</span><span v-else-if="res.count!=null">{{number(res.count)}} 处</span><span v-else>待调查</span></li></ul><n-button v-if="w.currentResources.source==='candidate'" size="tiny" :disabled="w.running" :loading="w.inspecting" @click="w.inspectResources">调查这颗星球</n-button></details>
              <details v-if="!w.currentResources.gas"><summary>这颗星球的电源</summary><EnergyChoice v-if="selectedEnergy>=0" v-model="w.energy.overrides[selectedEnergy]!.choice" :sites="w.sites" :item-name="w.itemName"/><n-button v-else size="small" @click="w.overrideEnergy(w.currentResources.id)">单独指定</n-button><n-button v-if="selectedEnergy>=0" size="tiny" quaternary @click="w.energy.overrides.splice(selectedEnergy,1)">恢复默认策略</n-button></details>
            </template>
          </aside>
        </div>
        <div class="run-bar">
          <div class="run-status">
            <strong role="status">{{phaseLabel}}</strong>
            <div v-if="w.task && (w.task.state==='running' || w.paused)" class="run-time" role="timer" aria-live="off" title="按本次分工和本机速度估算，重排布局时会更新。暂停时间不计入用时。">
              <b>{{estimateLabel}}</b><span>已计算 {{duration(w.task.elapsedSeconds)}}</span>
            </div>
            <span v-else-if="w.report && w.task">计算用时 {{duration(w.task.elapsedSeconds)}}</span>
            <span v-else-if="!w.running">按存档资源、已有建筑与所选条件安排新产线。</span>
          </div>
          <n-button v-if="w.running||w.paused" size="small" :disabled="w.task?.state==='cancelling'" @click="w.cancel">取消</n-button>
          <n-button v-if="w.paused" type="primary" @click="w.resume">继续计算</n-button>
          <n-button v-else type="primary" :loading="w.running" :disabled="!w.ready||w.busy||w.running" @click="w.start">生成规划</n-button>
        </div>
        <p v-if="w.directResourceError||w.error" class="problem" role="alert">{{w.directResourceError||w.error}}</p>
        <p v-if="w.task?.state==='cancelled'" class="hint result-message">规划已取消，可以调整条件后重新生成。</p>
        <p v-if="w.exported" class="hint result-message">已导出至 {{w.exported}}</p>
        <PlanResult v-if="w.report" :report="w.report" :item-name="w.itemName" :site-name="w.siteName" :exporting="w.exporting" @preview="w.showPreview" @export="w.exportPack"/>
      </section>
    </div>
    <n-modal :show="w.previewId!=null" @update:show="(open:boolean)=>{if(!open)w.closePreview()}"><div class="planet-preview"><div class="section-title"><h2>{{w.previewName}}</h2><n-button size="small" @click="w.closePreview">关闭</n-button></div><ViewerPanel v-if="w.previewCode" :code="w.previewCode" interactive/><p v-else-if="w.previewError">{{w.previewError}}</p><n-spin v-else size="small"/></div></n-modal>
  </main>
</template>

<style scoped lang="scss">
.run-time{display:flex;align-items:baseline;flex-wrap:wrap;gap:4px 12px;font-variant-numeric:tabular-nums}.run-time b{font-weight:500}
.industry-page{height:100%;display:flex;flex-direction:column;overflow:hidden;color:var(--ds-text-primary)}
.toolbar,.page-title,.save-toolbar,.section-title,.target-row,.run-bar{display:flex;align-items:center;gap:10px}.toolbar{padding:12px 18px;border-bottom:1px solid var(--ds-border-default);justify-content:space-between;flex-wrap:wrap}.page-title{white-space:nowrap}.active-tool{font-size:12px;border-left:1px solid var(--ds-border-default);padding-left:12px;color:var(--ds-text-secondary)}h1,h2,p{margin:0}h1{font-size:18px;font-weight:600}h2{font-size:14px;font-weight:600}.save-select{width:240px}.directory-mode{font-size:11px;color:var(--ds-text-secondary)}
.workbench{display:grid;grid-template-columns:310px minmax(0,1fr);flex:1;min-height:0}.controls{overflow:auto;border-right:1px solid var(--ds-border-default);background:var(--ds-bg-sidebar)}.control-section{padding:16px;border-bottom:1px solid var(--ds-border-subtle);display:grid;gap:10px}.section-title{justify-content:space-between}.target,.sphere-source,.recipe{display:grid;gap:8px;padding:10px 0;border-bottom:1px solid var(--ds-border-subtle)}.target:first-of-type{padding-top:0}.target-row>:first-child{flex:1;min-width:0}label{display:grid;gap:5px;font-size:12px;color:var(--ds-text-secondary)}.two-fields{display:grid;grid-template-columns:1fr 1fr;gap:8px;min-width:0}.check-list{display:grid;gap:8px}.hint{font-size:12px;line-height:1.65;color:var(--ds-text-secondary)}details.control-section{display:block}summary{display:flex;gap:7px;align-items:center;cursor:pointer;list-style:none;font-size:13px;font-weight:600}summary::before{content:'›';display:inline-block;font-size:17px;line-height:1;transition:transform .15s}details[open]>summary::before{transform:rotate(90deg)}details[open]>summary{margin-bottom:12px}details.control-section>label,details.control-section>.two-fields,details.control-section>.target-row,details.control-section>.check-list{margin-bottom:10px}
.visual-workspace{min-width:0;overflow:auto;display:flex;flex-direction:column}.visual-workspace>*{flex:none}.map-wrap{flex:1 0 0;height:auto;min-height:clamp(420px,66vh,800px);position:relative;display:flex;flex-direction:column;border-bottom:1px solid var(--ds-border-default)}.map-wrap>.star-map,.map-wrap>.empty-map{flex:1;min-height:0}.visual-workspace.has-report>.map-wrap{flex-grow:0}.empty-map{height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px;color:var(--ds-text-secondary)}.empty-map h2{font-size:19px;color:var(--ds-text-primary)}.empty-map p{font-size:13px}.map-caption{position:absolute;left:16px;bottom:12px;display:grid;gap:3px;pointer-events:none;font-size:12px}.map-caption span{color:var(--ds-text-secondary);font-size:11px}.inspector{position:absolute;right:14px;top:14px;width:240px;max-height:calc(100% - 65px);overflow:auto;box-sizing:border-box;background:var(--ds-bg-panel);border:1px solid var(--ds-border-default);border-radius:4px;padding:12px;display:grid;gap:12px}.inspector .section-title span{font-size:11px;color:var(--ds-text-secondary)}.inspector details{padding-top:10px;border-top:1px solid var(--ds-border-subtle)}.inspector ul{list-style:none;margin:0 0 10px;padding:0;display:grid;gap:7px}.inspector li{display:flex;justify-content:space-between;font-size:12px;gap:6px}.inspector li span:last-child{color:var(--ds-text-secondary)}.planet-summary{display:grid;grid-template-columns:1fr 1fr;gap:6px;font-size:12px}.planet-summary strong,.planet-summary button{grid-column:1/-1}.run-bar{padding:14px 18px;border-bottom:1px solid var(--ds-border-subtle)}.run-status{flex:1;display:grid;gap:3px;font-size:13px}.run-status span{font-size:12px;color:var(--ds-text-secondary)}.problem{margin:14px 18px;border-left:3px solid var(--ds-border-strong);padding:8px 12px;font-size:13px}.result-message{padding:10px 18px}.planet-preview{display:grid;grid-template-rows:auto 1fr;height:min(80vh,850px);width:min(90vw,1200px);padding:14px;gap:12px;box-sizing:border-box;background:var(--ds-bg-panel);border:1px solid var(--ds-border-default);border-radius:4px}
// ~ 交付目标卡片：沿用「量化与蓝图」目标卡片（括角、图标、无框输入、/min 速率），交付星球作为第三行次要信息。
.section-head{display:flex;align-items:baseline;gap:6px}.count{margin-left:auto;color:var(--ds-text-muted);font-family:var(--ds-font-display);font-weight:400;font-size:12px;letter-spacing:0}
.target-list{display:grid;gap:4px}.target-card{--bracket-color:var(--ds-border-strong);display:flex;align-items:center;gap:8px;padding:6px 4px 6px 10px;background:var(--ds-bg-panel-elevated)}.target-card>*{position:relative;z-index:1}.target-card:hover,.target-card:focus-within{--bracket-color:var(--ds-brand)}
.target-main{display:flex;min-width:0;flex:1;flex-direction:column;gap:2px}.target-item{min-width:0}
.target-item :deep(.n-base-selection){--n-border:1px solid transparent!important;--n-color:transparent!important;--n-padding-single-left:2px!important;--n-font-size:13px!important;font-weight:600}
.target-rate,.target-delivery{width:100%}.target-rate :deep(.n-input){--n-border:1px solid transparent!important;--n-color:var(--ds-hover-bg)!important;--n-padding-left:4px!important;--n-font-size:12px!important;font-family:var(--ds-font-display)}
.target-delivery :deep(.n-base-selection){--n-border:1px solid transparent!important;--n-color:transparent!important;--n-padding-single-left:4px!important;--n-font-size:11px!important;--n-placeholder-color:var(--ds-text-muted)!important}
.target-add{border:1px dashed var(--ds-border-default);border-radius:2px;padding:5px 0;display:inline-flex;align-items:center;justify-content:center;gap:4px;background:transparent;color:var(--ds-text-secondary);font-size:12px;cursor:pointer;transition:border-color 150ms,color 150ms}.target-add:hover:not(:disabled){border-color:var(--ds-brand);color:var(--ds-brand)}.target-add:disabled{cursor:not-allowed;opacity:.4}
.icon-button{display:inline-flex;width:22px;height:22px;flex:0 0 auto;align-items:center;justify-content:center;border:0;border-radius:2px;background:transparent;color:var(--ds-text-muted);cursor:pointer}.icon-button:hover:not(:disabled){background:var(--ds-hover-bg);color:var(--ds-text-primary)}.icon-button:disabled{cursor:not-allowed;opacity:.4}
.label-line{display:inline-flex;align-items:center;gap:4px}.label-info{color:var(--ds-text-muted);cursor:help}.label-info:hover,.label-info:focus-visible{color:var(--ds-brand);outline:none}
@media(prefers-reduced-motion:reduce){.target-add{transition:none}}
@media(max-width:1150px){.workbench{grid-template-columns:280px minmax(0,1fr)}.directory-mode{display:none}.inspector{width:210px}.map-wrap{min-height:max(420px,62vh)}}
@media(max-width:800px){.industry-page{overflow:auto}.workbench{display:flex;flex-direction:column;flex:none}.controls{max-height:50vh;border-right:0}.map-wrap{min-height:0}.map-wrap>.star-map,.map-wrap>.empty-map{flex:none;height:65vh}.inspector{position:static;width:auto;max-height:none;border-width:1px 0 0;border-radius:0;padding:14px 16px}.save-toolbar{width:100%}.save-select{flex:1}.map-caption{max-width:calc(100% - 230px)}.toolbar{padding:12px}}
</style>
