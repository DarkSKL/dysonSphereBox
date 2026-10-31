<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import type {build, quant} from '../../../../wailsjs/go/models'
import ItemIcon from './item-icon.vue'

// =============== 后端物流诊断 ===============
// 说明：直接展示生成器的实际 lane、边界端口和检查结果；入口数与利用率不在浏览器重新估算。
// 边界：旧蓝图可没有摘要；过期结果仍可查阅，但不能选择当前预览建筑，也不声称游戏内已验收。
const props = defineProps<{
  summary: build.LogisticsSummary
  audit?: build.RuleAudit | null
  graph?: quant.FlowGraph | null
  stale: boolean
  itemName: (id: number) => string
  format: (value: number) => string
}>()
const emit = defineEmits<{(event: 'select', index: number): void}>()
const visibleLanes = ref(40)
const visiblePorts = ref(40)
watch(() => props.summary, () => { visibleLanes.value = 40; visiblePorts.value = 40 })
const lanes = computed(() => props.summary.lanes ?? [])
const ports = computed(() => props.summary.externalPorts ?? [])
const checks = computed(() => props.summary.checks ?? [])
const auditChecks = computed(() => props.audit?.checks ?? [])
const metrics = computed(() => props.summary.metrics)
const nodes = computed(() => new Map((props.graph?.digest === props.summary.flowGraphDigest ? props.graph.nodes : []).map(node => [node.id, node])))
const kindLabels: Record<string, string> = {'raw-source': '原料', 'external-source': '外供', production: '产线', 'target-sink': '目标', 'leftover-sink': '副产物'}
const flowLabels: Record<string, string> = {'external-input': '外部输入', 'target-output': '目标输出', leftover: '剩余输出', internal: '内部物流', proliferator: '增产剂输入'}
const statusLabels: Record<string, string> = {passed: '通过', verified: '通过', ok: '通过', warning: '警告', partial: '部分覆盖', unverified: '未核验', failed: '失败'}
const nodeLabel = (id: string): string => {
  const node = nodes.value.get(id)
  return node ? `${kindLabels[node.kind] ?? node.kind} · ${props.itemName(node.itemId)}${node.kind === 'production' ? ` #${node.productionIndex + 1}` : ''}` : id
}
const percent = (ratio: number): string => `${props.format(ratio * 100)}%`
const beltLabel = computed(() => ['兼容默认', 'Mk.I', 'Mk.II', 'Mk.III'][props.summary.beltLevel] ?? `等级 ${props.summary.beltLevel}`)
const coordinateLabel = computed(() => props.audit?.coordinateSpace === 'planet-surface' ? '行星表面' : props.audit?.coordinateSpace === 'equatorial-tangent' ? '赤道切平面' : props.audit?.coordinateSpace || '未提供')
</script>

<template>
  <!-- ~ 物流容量与外部接口 -->
  <!-- * 说明：摘要、每路速率和边界坐标均来自当前生成结果；用户只调整原有带等级，不编辑入口数 -->
  <!-- * 交互：展开检查、接口及线路；点击端口可定位预览，长列表分批展开以限制首屏开销 -->
  <!-- * 依赖：build.LogisticsSummary；流图 digest 一致时补充规范来源/消费者名称 -->
  <!-- * 边界：过期摘要明确标识；代码级检查不代表完整 Unity Physics、游戏放置或产出验收 -->
  <section class="logistics" aria-label="物流诊断">
    <header><h3>物流诊断</h3><span class="belt-level type-display">{{ beltLabel }}</span><span v-if="stale" class="stale">上次结果</span></header>
    <p v-if="stale" class="hint is-warning">下列入口与线路属于上一次成功结果，不代表最近一次请求已通过。</p>
    <div class="summary-grid">
      <div><span>单带容量</span><strong class="type-display">{{ format(summary.beltCapacityPerMinute) }}<small>/min</small></strong></div>
      <div><span>自动线路</span><strong class="type-display">{{ format(metrics.laneCount) }}<small>路</small></strong></div>
      <div><span>外部入口</span><strong class="type-display">{{ format(metrics.externalInputCount) }}<small>个</small></strong></div>
      <div><span>边界接口</span><strong class="type-display">{{ format(ports.length) }}<small>个</small></strong></div>
    </div>
    <p class="hint">入口由实际流量自动分配；需改变容量时调整传送带等级后重新生成。仅边界端口需玩家接入蓝图外部物流。</p>

    <n-collapse :default-expanded-names="['checks']" class="diagnostic-sections">
      <n-collapse-item name="checks" title="检查与覆盖边界">
        <ul class="check-list">
          <li v-for="(check, index) in checks" :key="`${check.ruleId}-${index}`" :class="`is-${check.status}`"><span class="check-state">{{ statusLabels[check.status] ?? check.status }}</span><code>{{ check.ruleId }}</code><p>{{ check.message }}</p></li>
        </ul>
        <p class="hint">以上为代码级物流/数值规则预检，不代表游戏内可放置或连续运行已验收。</p>
      </n-collapse-item>

      <n-collapse-item v-if="audit" name="rule-audit" title="规则来源与游戏验收边界">
        <div class="audit-source">
          <span>规则版本</span><code>{{ audit.profile.version }}</code>
          <span>核对游戏版本</span><code>{{ audit.profile.gameVersion }} · Build {{ audit.profile.steamBuildId }}</code>
          <span>坐标作用域</span><strong>{{ coordinateLabel }}</strong>
          <span>覆盖等级</span><code>{{ audit.coverage }}</code>
          <span>量化数据</span><code>{{ audit.quantDataVersion }}</code>
          <span>电力网络</span><strong>{{ format(audit.powerNetworkCount) }} 个待接入网络</strong>
        </div>
        <ul v-if="auditChecks.length" class="check-list audit-checks">
          <li v-for="(check, index) in auditChecks" :key="`${check.ruleId}-${index}`" :class="`is-${check.status}`"><span class="check-state">{{ statusLabels[check.status] ?? check.status }}</span><code>{{ check.ruleId }}</code><p>{{ check.message }}</p></li>
        </ul>
        <ul v-if="audit.unverified?.length" class="unverified-list">
          <li v-for="item in audit.unverified" :key="item">未验证：{{ item }}</li>
        </ul>
        <p class="hint is-warning audit-boundary">这是基于指定离线规则版本的部分数值检查，不是 Unity Physics 结果。游戏内放置、碰撞、供电与连续产出仍未验证；必须在实际游戏中验收。</p>
      </n-collapse-item>

      <n-collapse-item name="ports" :title="`外部接口 · ${ports.length} 个`">
        <p class="hint">输入连接蓝图外供给，目标与副产物出口连接蓝图外接收端；坐标为蓝图本地网格。</p>
        <ol class="route-list">
          <li v-for="(port, index) in ports.slice(0, visiblePorts)" :key="`${port.edgeId}-${port.buildingIndex}-${index}`">
            <div class="item-line"><ItemIcon :name="itemName(port.itemId)" :size="16" /><strong>{{ itemName(port.itemId) }}</strong><span class="type-display">{{ format(port.ratePerMinute) }}/min</span></div>
            <div class="route-meta"><span>{{ flowLabels[port.kind] ?? port.kind }}</span><button type="button" :disabled="stale" :aria-label="`定位${itemName(port.itemId)}接口，建筑 ${port.buildingIndex}`" @click="emit('select', port.buildingIndex)">建筑 #{{ port.buildingIndex }}</button></div>
            <code class="coordinate">({{ format(port.x) }}, {{ format(port.y) }}, {{ format(port.z) }})</code>
          </li>
        </ol>
        <n-button v-if="ports.length > visiblePorts" size="tiny" block secondary @click="visiblePorts += 40">再显示 {{ Math.min(40, ports.length - visiblePorts) }} 个接口</n-button>
      </n-collapse-item>

      <n-collapse-item name="lanes" :title="`线路分配 · ${lanes.length} 路`">
        <ol class="route-list">
          <li v-for="(lane, index) in lanes.slice(0, visibleLanes)" :key="`${lane.edgeId}-${index}`">
            <div class="item-line"><ItemIcon :name="itemName(lane.itemId)" :size="16" /><strong>{{ itemName(lane.itemId) }}</strong><span class="type-display">{{ format(lane.ratePerMinute) }}/min</span></div>
            <p class="route-ends">{{ nodeLabel(lane.sourceNodeId) }} → {{ nodeLabel(lane.targetNodeId) }}</p>
            <div class="route-meta"><span>{{ flowLabels[lane.flowKind] ?? lane.flowKind }}</span><span class="type-display">{{ format(lane.ratePerMinute) }} / {{ format(lane.capacityPerMinute) }} · {{ percent(lane.utilization) }}</span></div>
            <meter :value="lane.utilization" min="0" max="1" :aria-label="`${itemName(lane.itemId)}线路利用率 ${percent(lane.utilization)}`" />
            <p class="connection" :class="{'is-warning': lane.flowKind === 'internal' && !lane.internalConnected}">{{ lane.flowKind === 'internal' ? lane.internalConnected ? '内部连接已核验' : '内部连接未核验' : '蓝图边界接口' }} · {{ lane.buildingIndices?.length ?? 0 }} 个路线建筑</p>
          </li>
        </ol>
        <n-button v-if="lanes.length > visibleLanes" size="tiny" block secondary @click="visibleLanes += 40">再显示 {{ Math.min(40, lanes.length - visibleLanes) }} 路</n-button>
      </n-collapse-item>

      <n-collapse-item name="layout" title="布局指标与搜索预算">
        <dl class="metric-list">
          <div><dt>总包络面积</dt><dd class="type-display">{{ format(metrics.envelopeArea) }}</dd></div>
          <div><dt>Footprint 并集面积</dt><dd class="type-display">{{ format(metrics.footprintArea) }}</dd></div>
          <div><dt>空白率</dt><dd class="type-display">{{ percent(metrics.blankRatio) }}</dd></div>
          <div><dt>内部带长</dt><dd class="type-display">{{ format(metrics.internalBeltLength) }}</dd></div>
          <div><dt>路线交叉 / 跨 Area</dt><dd class="type-display">{{ format(metrics.routeCrossings) }} / {{ format(metrics.crossAreaConnections) }}</dd></div>
          <div><dt>已评估 / 候选预算</dt><dd class="type-display">{{ format(metrics.evaluatedCandidates) }} / {{ format(metrics.candidateBudget) }}</dd></div>
          <div><dt>被拒绝候选</dt><dd class="type-display">{{ format(metrics.rejectedCandidates) }}</dd></div>
        </dl>
        <p class="hint">固定搜索预算与规则覆盖范围内选择可行方案，不宣称数学上的全局最优。</p>
        <p class="digest type-mono" :title="summary.flowGraphDigest">物料图 {{ summary.flowGraphDigest }}</p>
      </n-collapse-item>
    </n-collapse>
  </section>
</template>

<style scoped lang="scss">
.logistics { min-width: 0; color: var(--ds-text-secondary); font-size: 11px; }
header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
h3 { margin: 0; color: var(--ds-text-primary); font-size: 12px; font-weight: 600; }
.belt-level, .stale { margin-left: auto; color: var(--ds-text-muted); }
.stale, .is-warning { color: var(--ds-warning); }
.summary-grid { display: grid; grid-template-columns: 1fr 1fr; border-top: 1px solid var(--ds-border-subtle); border-bottom: 1px solid var(--ds-border-subtle); }
.summary-grid > div { display: flex; flex-direction: column; padding: 6px 0; }
.summary-grid span { color: var(--ds-text-muted); font-size: 10px; }
.summary-grid strong { color: var(--ds-text-primary); font-size: 16px; font-weight: 500; }
.summary-grid small { margin-left: 3px; color: var(--ds-text-muted); font-size: 10px; }
.hint { margin: 6px 0; font-size: 10px; line-height: 1.6; }
.diagnostic-sections { margin-top: 10px; }
.diagnostic-sections :deep(.n-collapse-item__header-main) { font-size: 11px; }
.check-list, .route-list { margin: 0; padding: 0; list-style: none; }
.check-list li { margin-bottom: 8px; }
.check-state { margin-right: 6px; color: var(--ds-text-secondary); }
.is-passed .check-state, .is-verified .check-state, .is-ok .check-state { color: var(--ds-success); }
.is-partial .check-state, .is-unverified .check-state, .is-warning .check-state { color: var(--ds-warning); }
.is-failed .check-state { color: var(--ds-error); }
.check-list code { color: var(--ds-text-muted); font-size: 9px; overflow-wrap: anywhere; }
.check-list p { margin: 2px 0 0; line-height: 1.5; overflow-wrap: anywhere; }
.audit-source { display: grid; grid-template-columns: minmax(74px, auto) minmax(0, 1fr); gap: 4px 8px; font-size: 10px; }
.audit-source > :nth-child(odd) { color: var(--ds-text-muted); }
.audit-source code, .audit-source strong { min-width: 0; color: var(--ds-text-secondary); overflow-wrap: anywhere; }
.audit-checks { margin-top: 10px; }
.unverified-list { display: grid; gap: 4px; margin: 8px 0; padding-left: 16px; color: var(--ds-warning); font-size: 10px; line-height: 1.5; }
.audit-boundary { margin-top: 10px; color: var(--ds-warning); }
.route-list li { border-bottom: 1px solid var(--ds-border-subtle); padding: 7px 0; }
.route-list li:last-child { margin-bottom: 6px; }
.item-line, .route-meta { display: flex; align-items: center; gap: 5px; }
.item-line strong { min-width: 0; flex: 1; color: var(--ds-text-primary); font-size: 11px; overflow-wrap: anywhere; }
.item-line > span { flex: 0 0 auto; }
.route-meta { justify-content: space-between; flex-wrap: wrap; margin-top: 4px; font-size: 10px; }
.route-meta button { border: 0; border-radius: 2px; padding: 0 3px; background: var(--ds-hover-bg); color: var(--ds-brand); font-size: 10px; cursor: pointer; }
.route-meta button:disabled { color: var(--ds-text-disabled); cursor: not-allowed; }
.coordinate { display: block; margin-top: 3px; color: var(--ds-text-muted); font-size: 9px; }
.route-ends { margin: 4px 0; color: var(--ds-text-muted); font-size: 10px; line-height: 1.5; overflow-wrap: anywhere; }
meter { width: 100%; height: 4px; margin-top: 4px; accent-color: var(--ds-brand); }
.connection { margin: 2px 0 0; color: var(--ds-text-muted); font-size: 9px; }
.connection.is-warning { color: var(--ds-warning); }
.metric-list { display: grid; gap: 4px; margin: 0; }
.metric-list > div { display: flex; justify-content: space-between; gap: 8px; }
.metric-list dt { color: var(--ds-text-muted); }
.metric-list dd { margin: 0; color: var(--ds-text-primary); }
.digest { margin: 8px 0 0; color: var(--ds-text-muted); font-size: 9px; overflow-wrap: anywhere; }
</style>
