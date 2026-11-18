<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import type {saveanalysis} from '../../../wailsjs/go/models'
import {presentDiagnosis} from '../../services/productionDiagnosis'

const props = defineProps<{
  report: saveanalysis.ProductionDiagnosis
  sorterReport?: saveanalysis.CircuitReport
  detail?: saveanalysis.PlanetItem
  itemName: (id: number) => string
  planetName: (id: number) => string
}>()
const result = computed(() => presentDiagnosis(props.report, props.itemName, props.planetName, props.sorterReport))
const index = ref(0), expanded = ref(false)
watch(() => props.report, () => { index.value = 0; expanded.value = false })
const first = computed(() => result.value.first[index.value])
const options = computed(() => result.value.first.map((f, value) => ({value, label: f.title})))
const fmt = (n: number) => n.toLocaleString('zh-CN')
const towerStock = computed(() => (props.detail?.slots || []).reduce((sum,s) => sum + s.inventory, 0))
const shared = computed(() => first.value?.shared || result.value.groups.some(g => g.findings.some(f => f.shared)))
const unresolved = computed(() => first.value?.unresolved || result.value.groups.some(g => g.findings.some(f => f.unresolved)))
const product = computed(() => props.itemName(props.report.productId))
const rootsLabel = (ids: number[]) => ids.map(id => '#' + id).join('、')
const firstGroup = computed(() => {
  const roots = first.value?.roots.join(',')
  return result.value.groups.find(g => g.roots.join(',') === roots)
})
</script>

<template>
  <!-- 先给出本机状态、原因和首查点，共用线路的实体清单按需展开。 -->
  <section class="diagnosis" aria-live="polite">
    <h2>{{ result.headline }}</h2>
    <p class="statistics">
      <span v-if="report.roots.length">保存时{{ result.countsText }}</span>
      <span v-if="detail?.history">存档前 1 分钟做了 <b>{{ fmt(detail.history.produced) }}</b> 件，用了 <b>{{ fmt(detail.history.consumed) }}</b> 件</span>
      <span v-else>存档前 1 分钟做了多少、用了多少，这份存档里没有记录</span>
      <span v-if="detail">物流塔里有 {{ fmt(towerStock) }} 件</span>
    </p>
    <div v-if="result.groups.length || result.untraced" class="answer">
      <h3>为什么卡住</h3>
      <ul>
        <li v-for="g in result.groups.slice(0, 4)" :key="g.scope + g.roots.join(',')">
          <b>{{ g.roots.length }} 台{{ g.scope }}</b><span>{{ g.summary }}</span>
        </li>
      </ul>
      <p v-if="result.groups.length > 4" class="note">展开可查看其余 {{ result.groups.length - 4 }} 组原因。</p>
      <p v-if="result.untraced" class="note">{{ result.untraced }} 台机器做完了也送不走，顺着带子还查不到是哪里堵了。</p>
    </div>
    <p v-else-if="report.status === 'history-only'" class="note">只知道以前做过，已经看不出当时是哪台机器做的。</p>
    <div v-if="first" class="first">
      <h3>先去哪里看</h3>
      <strong>{{ first.title }}</strong>
      <p>{{ first.evidence }}</p>
      <p v-if="first.supplyPath" class="supply-path">{{ first.supplyPath }}</p>
      <p v-if="first.shared" class="note">沿着这条线能找到 {{ first.roots.length }} 台生产{{ product }}的设备。线路有共用的地方，只修这里能让几台开工，还说不准。</p>
      <p v-if="options.length > 1" class="note">还有 {{ options.length - 1 }} 处也值得先看，展开可以换一处。</p>
    </div>
    <details v-if="first || result.groups.length" :open="expanded" @toggle="expanded = ($event.target as HTMLDetailsElement).open">
      <summary>看具体是哪几台、机器里还剩什么</summary>
      <div v-if="expanded">
        <template v-if="first">
          <n-select v-if="options.length > 1" v-model:value="index" :options="options" filterable size="small" aria-label="其他值得先看的位置" />
          <p v-if="first.detail">{{ first.detail }}</p>
          <details v-if="!firstGroup"><summary>看这条线上 {{ first.roots.length }} 台设备的编号</summary><p class="ids">{{ rootsLabel(first.roots) }}</p></details>
        </template>
        <section v-for="g in result.groups" :key="g.scope + g.roots.join(',')" class="cause">
          <h4>{{ g.roots.length }} 台{{ g.scope }}<template v-if="g.findings.length > 1">，下面 {{ g.findings.length }} 条说的都是这批机器</template></h4>
          <div v-for="finding in g.findings" :key="finding.title" class="evidence">
            <strong>{{ finding.title }}</strong>
            <p>{{ finding.evidence }}</p>
          </div>
          <details><summary>查看这 {{ g.roots.length }} 台设备的编号</summary><p class="ids">{{ rootsLabel(g.roots) }}</p></details>
        </section>
        <p class="note">这份存档的游戏时刻 <span class="type-mono">tick {{ fmt(report.tick) }}</span></p>
      </div>
    </details>
    <p v-if="unresolved || (shared && !first?.shared)" class="note">{{ shared && !first?.shared ? '几台机器共用传送带，每台分到多少货，从这一次存档里看不出来。' : '' }}{{ unresolved ? '有些货从哪里来、送到哪里，存档里还看不全。' : '' }}</p>
  </section>
</template>

<style scoped lang="scss">
.diagnosis { color: var(--ds-text-primary); overflow-wrap: anywhere; }
h2 { font-size: 21px; font-weight: 650; }
h3, h4 { font-size: 15px; font-weight: 600; }
.statistics { display: flex; flex-wrap: wrap; gap: 2px 14px; margin: 4px 0 12px; font-size: 13px; color: var(--ds-text-secondary); }
.answer { border-inline-start: 3px solid var(--ds-brand); padding: 12px 16px; background: var(--ds-bg-panel); }
.answer ul { display: grid; gap: 3px; margin-top: 6px; }
.answer li { display: grid; grid-template-columns: minmax(3em, max-content) 1fr; align-items: baseline; column-gap: 12px; line-height: 1.55; }
.answer li b { font-size: 13px; white-space: nowrap; }
p { margin: 4px 0; line-height: 1.65; }
.first { padding: 12px 0; }
.first h3 { margin-bottom: 4px; color: var(--ds-text-secondary); }
.first strong { font-size: 16px; }
.supply-path { font-size: 13px; }
.note, .ids { font-size: 13px; color: var(--ds-text-secondary); }
details { border-top: 1px solid var(--ds-border-default); padding: 8px 0; }
summary { cursor: pointer; padding: 3px 0; }
.cause { padding: 8px 0; }
.evidence { margin-top: 6px; }
.cause details { border: 0; font-size: 13px; padding: 0; }
.ids { font-family: var(--ds-font-mono); }
</style>
