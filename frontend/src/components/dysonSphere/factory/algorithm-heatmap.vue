<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import {ACTIVITY_ROWS, useAlgorithmActivity, type ActivityColumn} from '../../../composables/dysonSphere/useAlgorithmActivity'

// =============== 算法活动热力图 ===============
// 说明：预览栏里的方块热力图（类似代码托管站的提交图）：每行一种指标，每列一个时间片，越新越靠右；
//       颜色深浅表示该指标在这一行内相对峰值的强弱。
// 交互：光标移到某个方块上出现浮标，列出该时间片全部指标，当前所指的一行加粗并标出色块。
// 边界：只读展示；列数随容器宽度自适应，不足的部分留白，只绘制有活动的方块；浮标挂到 body，不被预览栏裁切。

const {columns, latest} = useAlgorithmActivity()

const CELL = 5
const PITCH = 7
const host = ref<HTMLElement | null>(null)
const capacity = ref(40)
let observer: ResizeObserver | null = null
onMounted(() => {
  if (!host.value) return
  const measure = (): void => { capacity.value = Math.max(8, Math.floor((host.value?.clientWidth ?? 0) / PITCH)) }
  measure()
  observer = new ResizeObserver(measure)
  observer.observe(host.value)
})
onBeforeUnmount(() => observer?.disconnect())

// 每行按可见列内的峰值归一化为 0–4 档
const visible = computed<(ActivityColumn | null)[]>(() => {
  const list = columns.value.slice(-capacity.value)
  return [...Array<null>(capacity.value - list.length).fill(null), ...list]
})
const peaks = computed(() => Object.fromEntries(ACTIVITY_ROWS.map(row => [row.key, Math.max(1, ...visible.value.map(column => column?.[row.key] ?? 0))])))
const level = (column: ActivityColumn | null, key: keyof ActivityColumn): number => {
  const value = column?.[key] ?? 0
  return value > 0 ? Math.max(1, Math.ceil(value / peaks.value[key] * 4)) : 0
}
const summary = computed(() => {
  const value = latest.value
  return value ? `建筑 ${value.objects} · 新增 ${value.added} · 替换 ${value.replaced} · 撤销 ${value.removed} · 队列 ${value.pending}` : '运行算法后，这里显示建筑、新增、替换、撤销与队列的活动热力图'
})

// =============== 浮标 ===============
// 说明：按事件目标定位所在列，所在行由指针的纵向位置推算（方块只有 5px，不逐个挂监听）。
const tip = ref<{index: number; row: number; x: number; y: number} | null>(null)
const tipColumn = computed(() => (tip.value ? visible.value[tip.value.index] ?? null : null))
const onMove = (event: MouseEvent): void => {
  const column = (event.target as HTMLElement).closest<HTMLElement>('.heat-col')
  const index = column ? Number(column.dataset.index) : -1
  if (!column || !visible.value[index]) { tip.value = null; return }
  const rect = column.getBoundingClientRect()
  const row = Math.min(ACTIVITY_ROWS.length - 1, Math.max(0, Math.floor((event.clientY - rect.top) / PITCH)))
  tip.value = {index, row, x: rect.left + rect.width / 2, y: rect.bottom}
}
const tipStyle = computed(() => {
  const value = tip.value
  if (!value) return {}
  return {left: `${Math.min(Math.max(value.x, 90), window.innerWidth - 90)}px`, top: `${value.y + 10}px`}
})
</script>

<template>
  <!-- ~ 算法活动热力图 -->
  <!-- * 说明：行 = 建筑 / 新增 / 替换 / 撤销 / 队列，列 = 时间片（右侧最新）；颜色深浅 = 该行内的相对强度 -->
  <!-- * 交互：光标移到方块上显示浮标；无活动的位置不绘制 -->
  <div ref="host" class="algo-heat" role="img" :aria-label="summary" @mousemove="onMove" @mouseleave="tip = null">
    <div v-for="(column, index) in visible" :key="index" class="heat-col" :data-index="index">
      <i v-for="(row, rowIndex) in ACTIVITY_ROWS" :key="row.key" class="heat-cell" :class="[`lv-${level(column, row.key)}`, {'is-hover': tip?.index === index && tip.row === rowIndex}]" :style="{'--tone': row.tone}" />
    </div>
    <Teleport to="body">
      <div v-if="tip && tipColumn" class="heat-tip" :style="tipStyle" role="tooltip">
        <p v-for="(row, rowIndex) in ACTIVITY_ROWS" :key="row.key" :class="{'is-current': tip.row === rowIndex}">
          <i :style="{background: row.tone}" /><span>{{ row.label }}</span><strong>{{ row.cumulative ? '+' : '' }}{{ tipColumn[row.key] }}</strong>
        </p>
      </div>
    </Teleport>
  </div>
</template>

<style scoped lang="scss">
.algo-heat { display: flex; min-width: 96px; height: 33px; flex: 1 1 0; align-items: center; justify-content: flex-end; overflow: hidden; }
// 列宽 7px 含 2px 间隙，指针划过间隙也算在列内，浮标不会闪断
.heat-col { display: flex; width: 7px; height: 33px; flex: 0 0 7px; flex-direction: column; gap: 2px; }
// 无活动的方块不绘制底色，只保留有数据的方块；占位保留，行列位置不会错位
.heat-cell { display: block; width: 5px; height: 5px; border-radius: 1px; background: transparent; }
.heat-cell.lv-1 { background: color-mix(in srgb, var(--tone) 30%, var(--ds-hover-bg)); }
.heat-cell.lv-2 { background: color-mix(in srgb, var(--tone) 52%, var(--ds-hover-bg)); }
.heat-cell.lv-3 { background: color-mix(in srgb, var(--tone) 76%, var(--ds-hover-bg)); }
.heat-cell.lv-4 { background: var(--tone); }
.heat-cell.is-hover { outline: 1px solid var(--ds-text-primary); outline-offset: 1px; }
.heat-tip { position: fixed; z-index: 3000; min-width: 112px; border: 1px solid var(--ds-border-default); border-radius: 2px; padding: 6px 10px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 12px; line-height: 1.7; pointer-events: none; transform: translateX(-50%); box-shadow: 0 4px 14px rgb(0 0 0 / 28%); }
.heat-tip p { display: flex; align-items: center; gap: 6px; margin: 0; white-space: nowrap; }
.heat-tip i { width: 6px; height: 6px; flex: 0 0 6px; border-radius: 1px; }
.heat-tip span { flex: 1; }
.heat-tip strong { margin-left: 12px; font-weight: 500; font-variant-numeric: tabular-nums; }
.heat-tip p.is-current { color: var(--ds-text-primary); font-weight: 600; }
.heat-tip p.is-current strong { font-weight: 600; }
</style>
