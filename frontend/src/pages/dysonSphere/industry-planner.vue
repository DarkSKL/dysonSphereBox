<script setup lang="ts">
import {computed, h, reactive, ref, useId} from 'vue'
import type {SelectOption} from 'naive-ui'
import {ChevronDown, Crosshair, FolderOpen, Info, Orbit, Plus, X} from '@lucide/vue'
import {industryplan} from '../../../wailsjs/go/models'
import {useIndustryPlanner} from '../../composables/dysonSphere/useIndustryPlanner'
import PlanResult from '../../components/dysonSphere/industry/plan-result.vue'
import StarMap from '../../components/dysonSphere/industry/star-map.vue'
import EnergyChoice from '../../components/dysonSphere/industry/energy-choice.vue'
import ViewerPanel from '../../components/dysonSphere/blueprint/viewer-panel.vue'
import ItemIcon from '../../components/dysonSphere/factory/item-icon.vue'
import {ensureFactoryIcons} from '../services/dysonSphere/factory-icons'

definePage({meta:{layout:'main',title:'特殊蓝图',keepAlive:true}})
const w=reactive(useIndustryPlanner())
ensureFactoryIcons()
// =============== 交付目标显示 ===============
// 说明：卡片单独显示大图标，选项菜单保留小图标帮助辨认产物。
const renderTargetItem=(option:SelectOption, selected:boolean)=>selected ? String(option.label) : h('span',{class:'inline-flex min-w-0 items-center gap-2'},[
  h(ItemIcon,{name:String(option.label),size:16}),
  h('span',{class:'truncate'},String(option.label)),
])
// =============== 工作区状态 ===============
// 说明：检查窗由用户开合；重新选星只更新窗口内容。生成与预览仍由用户触发。
const inspectorOpen=ref(false)
const inspectorId=useId()
const inspector=ref<HTMLElement|null>(null)
const selectedPlan=computed(()=>w.report?.planets.find(p=>p.id===w.resourceSite))
const selectedEnergy=computed(()=>w.energy.overrides.findIndex(p=>p.planetId===w.resourceSite))
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
        <!-- ~ 交付目标 -->
        <!-- * 说明：卡片突出产物与每分钟净产量，交付星球和移除操作放在次要行。 -->
        <!-- * 交互：没有目标时通过主按钮新增，已有目标时可以继续添加或移除。 -->
        <section class="control-section delivery-goals">
          <div class="goal-heading"><Crosshair :size="15" aria-hidden="true" /><h2>交付目标</h2></div>
          <div v-if="!w.targets.length" class="target-empty">
            <strong>还没有交付目标</strong>
            <span>选择要生产的物品和每分钟净产量。</span>
            <n-button size="small" type="primary" block @click="w.addTarget">
              <template #icon><Plus :size="15" /></template>添加目标
            </n-button>
          </div>
          <div v-else class="target-list">
            <div v-for="(target,index) in w.targets" :key="target.key" class="target-card ds-bracket">
              <div class="target-main">
                <ItemIcon :name="target.item ? w.itemName(target.item) : ''" :size="34" />
                <n-select v-model:value="target.item" size="small" filterable :options="w.items" :render-label="renderTargetItem" :title="target.item ? w.itemName(target.item) : undefined" placeholder="选择产物" class="target-item" aria-label="交付产物" />
                <n-input-number v-model:value="target.rate" size="small" :min=".001" :show-button="false" class="target-rate" aria-label="净产量（件/分钟）" title="每分钟净产量">
                  <template #suffix><span class="target-unit">/min</span></template>
                </n-input-number>
              </div>
              <div class="target-destination">
                <label>交付至<n-select v-model:value="target.delivery" size="tiny" filterable clearable :options="w.sites" placeholder="中心星球" aria-label="交付星球" /></label>
                <n-button size="tiny" quaternary class="target-remove" aria-label="移除目标" title="移除目标" @click="w.targets.splice(index,1)"><template #icon><X :size="14" /></template></n-button>
              </div>
            </div>
            <n-button size="small" dashed block class="target-add" :disabled="w.targets.length>=100" @click="w.addTarget">
              <template #icon><Plus :size="14" /></template>添加目标
            </n-button>
          </div>
        </section>
        <section class="control-section">
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
          <StarMap v-if="w.selection" :selection="w.selection" :report="w.report" :selected="w.resourceSite" :loading="w.busy" :inspector="inspector" @select="(id:number)=>w.resourceSite=id"/>
          <div v-else class="empty-map"><Orbit :size="56" :stroke-width=".8"/><h2>{{w.busy?'正在读取星区':'从一份存档开始'}}</h2><p>选择顶部存档，查看资源与可建设星球。</p><n-button v-if="!w.saves.length&&!w.busy" size="small" @click="w.openDirectory(true)">选择存档目录</n-button></div>
          <!-- ~ 星球详情悬浮窗 -->
          <!-- * 交互：标题按钮通过鼠标或键盘开合，选星只更新标题与内容。 -->
          <!-- * 边界：展开内容在窗内滚动，收起后退出键盘顺序。 -->
          <aside v-if="w.selection" ref="inspector" class="inspector map-hud ds-bracket" :class="{'is-expanded':inspectorOpen}" aria-label="星球详情">
            <h2>
              <button type="button" class="inspector-toggle" :aria-expanded="inspectorOpen" :aria-controls="inspectorId" @click="inspectorOpen=!inspectorOpen">
                <span :title="w.currentResources?.name??'星球详情'">{{w.currentResources?.name??'星球详情'}}</span>
                <ChevronDown :size="16" aria-hidden="true" />
              </button>
            </h2>
            <div :id="inspectorId" class="inspector-drawer" :inert="!inspectorOpen">
              <div class="inspector-scroll">
                <div class="inspector-content">
                  <n-select size="small" filterable clearable v-model:value="w.resourceSite" :options="w.allSites" placeholder="搜索或点击星球" aria-label="搜索星球" />
                  <template v-if="w.currentResources">
                    <p class="inspector-meta">{{w.currentResources.gas?'气态巨星':'固态星球'}} · {{sources[w.currentResources.source]??'资料待确认'}}</p>
                    <div v-if="selectedPlan" class="planet-summary"><strong>{{selectedPlan.modules}} 个制造模块</strong><span>{{number(selectedPlan.buildings)}} 座建筑</span><span>{{number(selectedPlan.power?.powerMw??selectedPlan.powerMw)}} MW</span><n-button v-for="(s,i) in selectedPlan.sheets" :key="s.id" size="small" @click="w.showPreview(s.id)">预览施工图 {{i+1}}</n-button></div>
                    <details><summary>星球资源 <span class="inspector-count">{{w.currentResources.resources.length}} 种</span></summary><ul><li v-for="res in w.currentResources.resources" :key="res.itemId"><span>{{w.itemName(res.itemId)}}</span><span v-if="res.availableCount!=null">{{number(res.availableCount)}} 处可用</span><span v-else-if="res.count!=null">{{number(res.count)}} 处</span><span v-else>待调查</span></li></ul><n-button v-if="w.currentResources.source==='candidate'" size="tiny" :disabled="w.running" :loading="w.inspecting" @click="w.inspectResources">调查这颗星球</n-button></details>
                    <details v-if="!w.currentResources.gas"><summary>这颗星球的电源</summary><EnergyChoice v-if="selectedEnergy>=0" v-model="w.energy.overrides[selectedEnergy]!.choice" :sites="w.sites" :item-name="w.itemName"/><n-button v-else size="small" @click="w.overrideEnergy(w.currentResources.id)">单独指定</n-button><n-button v-if="selectedEnergy>=0" size="tiny" quaternary @click="w.energy.overrides.splice(selectedEnergy,1)">恢复默认策略</n-button></details>
                  </template>
                </div>
              </div>
            </div>
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
.workbench{display:grid;grid-template-columns:clamp(266px,calc(15vw + 93px),310px) minmax(0,1fr);flex:1;min-height:0}.controls{overflow:auto;border-right:1px solid var(--ds-border-default);background:var(--ds-bg-sidebar)}.control-section{padding:16px;border-bottom:1px solid var(--ds-border-subtle);display:grid;gap:10px}.section-title{justify-content:space-between}.sphere-source,.recipe{display:grid;gap:8px;padding:10px 0;border-bottom:1px solid var(--ds-border-subtle)}.target-row>:first-child{flex:1;min-width:0}label{display:grid;gap:5px;font-size:12px;color:var(--ds-text-secondary)}.two-fields{display:grid;grid-template-columns:1fr 1fr;gap:8px;min-width:0}.check-list{display:grid;gap:8px}.hint{font-size:12px;line-height:1.65;color:var(--ds-text-secondary)}details.control-section{display:block}summary{display:flex;gap:7px;align-items:center;cursor:pointer;list-style:none;font-size:13px;font-weight:600}summary::before{content:'›';display:inline-block;font-size:17px;line-height:1;transition:transform .15s}details[open]>summary::before{transform:rotate(90deg)}details[open]>summary{margin-bottom:12px}details.control-section>label,details.control-section>.two-fields,details.control-section>.target-row,details.control-section>.check-list{margin-bottom:10px}
.visual-workspace{min-width:0;min-height:0;overflow:auto;display:flex;flex-direction:column;container-type:inline-size}
.visual-workspace>*{flex:none}
.map-wrap{--map-inspector-width:min(248px,48%);position:relative;flex:1 0 auto;min-height:300px;display:grid;grid-template-columns:minmax(0,1fr);grid-template-rows:1fr;border-bottom:1px solid var(--ds-border-default)}
.map-wrap>.star-map,.map-wrap>.empty-map{height:auto;min-height:0}
.visual-workspace.has-report>.map-wrap{min-height:clamp(360px,58vh,720px);flex-grow:0}
.empty-map{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px;padding:16px;text-align:center;color:var(--ds-text-secondary)}
.empty-map h2{font-size:19px;color:var(--ds-text-primary)}.empty-map p{font-size:13px}

// ~ 面包屑、视角工具与详情窗共用日夜主题的括角和边框。
.map-wrap :deep(.map-hud){
  --bracket-size:12px;
  border:1px solid var(--ds-border-default);
  border-radius:2px;
  background:var(--ds-bg-panel-elevated);
  color:var(--ds-text-primary);
  box-shadow:var(--ds-active-glow);
}
.inspector{
  position:absolute;z-index:1;top:12px;right:12px;
  width:var(--map-inspector-width);max-height:min(62%,440px);
  display:grid;grid-template-rows:auto minmax(0,1fr);
}
.inspector-toggle{
  width:100%;min-height:34px;display:flex;align-items:center;justify-content:space-between;gap:8px;
  padding:4px 10px 4px 12px;border-radius:inherit;text-align:left;font:inherit;
  &:hover,&:focus-visible{background:var(--ds-hover-bg);color:var(--ds-brand)}
  span{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  svg{flex:none;color:var(--ds-text-secondary);transition:transform .18s ease-out}
}
.inspector-drawer{display:grid;grid-template-rows:0fr;min-height:0;transition:grid-template-rows .18s ease-out}
.inspector.is-expanded{
  .inspector-drawer{grid-template-rows:1fr}
  .inspector-toggle svg{transform:rotate(180deg)}
  .inspector-scroll{overflow:auto}
}
.inspector-scroll{min-height:0;overflow:hidden;overscroll-behavior:contain}
.inspector-content{display:grid;gap:8px;padding:10px 12px 12px;border-top:1px solid var(--ds-border-default)}
.inspector-meta{font-size:12px;color:var(--ds-text-secondary)}
.inspector-count,.inspector li span:last-child,.planet-summary span{font-family:var(--ds-font-display);font-variant-numeric:tabular-nums}
.inspector-count,.inspector li span:last-child{color:var(--ds-text-secondary)}
.inspector details{padding-top:8px;border-top:1px solid var(--ds-border-subtle)}
.inspector ul{list-style:none;margin:0 0 10px;padding:0;display:grid;gap:7px}
.inspector li{display:flex;justify-content:space-between;font-size:12px;gap:6px}
.planet-summary{display:grid;grid-template-columns:1fr 1fr;gap:6px;font-size:12px}.planet-summary strong,.planet-summary button{grid-column:1/-1}
.run-bar{padding:14px 18px;border-bottom:1px solid var(--ds-border-subtle)}.run-status{flex:1;display:grid;gap:3px;font-size:13px}.run-status span{font-size:12px;color:var(--ds-text-secondary)}.problem{margin:14px 18px;border-left:3px solid var(--ds-border-strong);padding:8px 12px;font-size:13px}.result-message{padding:10px 18px}.planet-preview{display:grid;grid-template-rows:auto 1fr;height:min(80vh,850px);width:min(90vw,1200px);padding:14px;gap:12px;box-sizing:border-box;background:var(--ds-bg-panel);border:1px solid var(--ds-border-default);border-radius:4px}

// ~ 目标卡片沿用量化页的括角、物品图标与产量数字样式。
.delivery-goals{container-type:inline-size}
.goal-heading{display:flex;align-items:center;gap:8px;color:var(--ds-brand)}
.goal-heading h2{font-size:16px;color:var(--ds-text-primary)}
.target-list{display:grid;gap:8px}
.target-empty{display:grid;gap:5px;padding:12px;border:1px dashed var(--ds-border-strong);border-radius:2px;background:var(--ds-bg-panel);font-size:12px;color:var(--ds-text-secondary)}
.target-empty strong{font-size:13px;color:var(--ds-text-primary)}
.target-empty button{margin-top:5px}
.target-card{display:grid;gap:8px;padding:9px 10px 7px;border-radius:2px;background:var(--ds-bg-panel-elevated)}
.target-card:hover,.target-card:focus-within{--bracket-color:var(--ds-brand)}
.target-main{display:grid;grid-template-columns:auto minmax(0,1fr) 78px;align-items:center;gap:6px}
.target-item :deep(.n-base-selection){--n-border:1px solid transparent!important;--n-color:transparent!important;--n-text-color:var(--ds-text-primary)!important;--n-padding-single-left:2px!important;--n-font-size:14px!important;font-weight:700}
.target-rate :deep(.n-input){--n-color:var(--ds-hover-bg)!important;--n-border:1px solid transparent!important;--n-font-size:17px!important;--n-padding-left:6px!important;--n-padding-right:5px!important;font-family:var(--ds-font-display);font-weight:650;font-variant-numeric:tabular-nums}
.target-unit{font-size:10px;font-weight:500;color:var(--ds-text-secondary)}
.target-destination{display:flex;align-items:center;gap:5px;padding-top:5px;border-top:1px solid var(--ds-border-subtle)}
.target-destination label{display:flex;flex:1;min-width:0;align-items:center;gap:8px;white-space:nowrap}
.target-destination :deep(.n-select){flex:1;min-width:0}
.target-destination :deep(.n-base-selection){--n-border:1px solid transparent!important;--n-color:transparent!important;--n-padding-single-left:3px!important}
.target-remove{color:var(--ds-text-secondary)}
.label-line{display:inline-flex;align-items:center;gap:4px}.label-info{color:var(--ds-text-muted);cursor:help}.label-info:hover,.label-info:focus-visible{color:var(--ds-brand);outline:none}
@media(prefers-reduced-motion:reduce){summary::before,.inspector-drawer,.inspector-toggle svg{transition:none}}
@media(max-width:1150px){.directory-mode{display:none}}
@container (max-width:245px){
  .target-main{grid-template-columns:auto minmax(0,1fr);column-gap:8px;row-gap:2px}
  .target-main>.factory-item-icon{grid-row:span 2}
  .target-rate{width:100px}
}
@container (max-width:520px){.inspector{top:54px;width:min(220px,44%);max-height:39%}}
@media(max-width:800px){
  .industry-page{overflow:auto}
  .workbench{display:flex;flex-direction:column;flex:none}
  .controls{max-height:38vh;border-right:0;border-bottom:1px solid var(--ds-border-default)}
  .visual-workspace{overflow:visible}
  .map-wrap,.visual-workspace.has-report>.map-wrap{--map-inspector-width:min(220px,44%);min-height:0}
  .map-wrap>.star-map,.map-wrap>.empty-map{height:clamp(300px,49vh,540px)}
  .inspector{max-height:43%}
  .save-toolbar{width:100%}.save-select{flex:1;min-width:0}.toolbar{padding:12px}
}
</style>
