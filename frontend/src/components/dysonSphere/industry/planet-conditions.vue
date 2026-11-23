<script setup lang="ts">
import {computed} from 'vue'
import type {industryplan} from '../../../../wailsjs/go/models'
// =============== 单星的开工、补给与电力条件 ===============
const props=defineProps<{entries:industryplan.External[];itemName:(id:number)=>string;siteName:(id:number)=>string}>()
const drones=computed(()=>[...new Set(props.entries.filter(e=>e.kind==='local-drones').map(e=>props.itemName(e.itemId)))])
const supplies=computed(()=>props.entries.filter(e=>!['local-drones','battery-startup','solar-conditions','grid-power','charging-peak'].includes(e.kind)))
const power=computed(()=>props.entries.filter(e=>e.kind==='grid-power'||e.kind==='charging-peak'))
const reasons:Record<string,string>={'declared-input':'外供','fuel-input':'补给燃料',warpers:'补给翘曲器',transport:'由玩家运输','mining-disabled':'采集已关闭，需外供','collector-disabled':'外供巨星资源','collector-placement':'供应巨星采集物','resource-data':'待确认资源','resource-shortfall':'补足采集或运力','surplus-removal':'持续移出','external-photons':'外供临界光子'}
const number=(value:number)=>value.toLocaleString('zh-CN',{maximumFractionDigits:3})
</script>
<template>
  <section v-if="entries.length" class="conditions">
    <p v-if="drones.length"><strong>开工配备</strong>　本地物流运输机，负责{{drones.join('、')}}。</p>
    <p v-if="entries.some(e=>e.kind==='battery-startup')"><strong>启动物资</strong>　投入第一批空蓄电器。</p>
    <p v-if="entries.some(e=>e.kind==='solar-conditions')"><strong>启动供电</strong>　确认日照并充满跨夜蓄电器。</p>
    <p v-for="e in power" :key="e.kind"><strong>{{e.kind==='grid-power'?'持续接入电网':'物流塔充电峰值'}}</strong>　{{number(e.powerMw??0)}} MW</p>
    <p v-if="power.some(e=>e.kind==='charging-peak')">此上限按这颗星球图中所有物流塔同时以设定功率充电计算。实际平均耗电随发船、配送和充电设置变化。</p>
    <p v-for="(e,i) in supplies" :key="i"><strong>{{reasons[e.kind]??'持续补给'}}</strong>　{{itemName(e.itemId)}} {{number(e.ratePerMinute)}} 件/分<span v-if="e.peerPlanetId"> → {{siteName(e.peerPlanetId)}}</span></p>
  </section>
</template>
<style scoped>
.conditions{margin:10px 0}.conditions p{margin:4px 0;font-size:12px;line-height:1.7;color:var(--ds-text-secondary)}.conditions strong{color:var(--ds-text-primary);font-weight:500}
</style>
