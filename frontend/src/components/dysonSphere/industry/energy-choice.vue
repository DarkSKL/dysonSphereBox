<script setup lang="ts">
import {computed} from 'vue'
import type {industryplan} from '../../../../wailsjs/go/models'

// =============== 供电条件 ===============
// 说明：默认策略和逐星覆盖使用同一组输入；储能容量由后台连同充电负荷计算。
const choice = defineModel<industryplan.EnergyChoice>({required:true})
const props = defineProps<{sites:{label:string;value:number}[];itemName:(id:number)=>string}>()
const modes = [{label:'自动推荐',value:'auto'},{label:'指定电源',value:'fixed'},{label:'已有电网',value:'grid'}]
const facilities = computed(()=>[2203,2205,2204,2211,2210,2213,2208,2209].map(value=>({value,label:props.itemName(value)})))
const fuels = computed(()=>{
  const ids:Record<number,number[]>={2204:[1006,1109,1120,1114,1801,1007,1011,1030,1031,1112,1121,1123,1124,1117,1128,1129,1130,1141,1142,1143,5206],2211:[1802],2210:[1803,1804]}
  return [{label:'常规燃料',value:0},...(ids[choice.value.facilityId]??[]).map(value=>({value,label:props.itemName(value)}))]
})
</script>

<template>
  <!-- ~ 电源选择：保持指定电源的环境前提可见 -->
  <div class="energy-choice">
    <n-select size="small" v-model:value="choice.mode" :options="modes" />
    <template v-if="choice.mode==='fixed'">
      <n-select size="small" :value="choice.facilityId" :options="facilities" @update:value="(id:number)=>{choice.facilityId=id;choice.fuelItemId=0}" />
      <label v-if="[2204,2210,2211].includes(choice.facilityId)">燃料<n-select size="small" v-model:value="choice.fuelItemId" :options="fuels" /></label>
      <template v-if="choice.facilityId===2205">
        <label>日照期间可保证的发电倍率<n-input-number size="small" v-model:value="choice.solarFactor" :min=".01" :step=".1" /></label>
        <label>可保证的日照比例<n-input-number size="small" v-model:value="choice.solarDuty" :min=".01" :max="1" :step=".05" /></label>
        <label>最长连续夜晚（秒）<n-input-number size="small" v-model:value="choice.nightSeconds" :min="0" /></label>
        <p>自动配套跨夜蓄电器。请按建设位置确认日照条件。</p>
      </template>
      <label v-if="choice.facilityId===2209">充电星球<n-select size="small" v-model:value="choice.chargePlanetId" :options="sites" filterable /></label>
      <p v-if="choice.facilityId===2209">请在该星的详情中设置独立电源。满、空电池往返运输一并规划。</p>
      <p v-if="choice.facilityId===2213">按可核验的熔岩落点规划，保留取热区域。</p>
      <p v-if="choice.facilityId===2208">与光子接收共享下方声明的戴森球可用功率。</p>
    </template>
    <label v-if="choice.mode==='grid'">新增产线可用余量（MW）<n-input-number size="small" v-model:value="choice.gridCapacityMw" :min=".1" /></label>
    <p v-if="choice.mode==='auto'">结合负荷、星球条件和燃料需求推荐电源；可在星图中逐星指定。</p>
  </div>
</template>

<style scoped>
.energy-choice{display:grid;gap:8px}label{display:grid;gap:4px;font-size:12px;color:var(--ds-text-secondary)}p{font-size:12px;line-height:1.6;color:var(--ds-text-secondary);margin:0}
</style>
