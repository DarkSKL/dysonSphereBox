<script setup lang="ts">
import type {quant} from '../../../../wailsjs/go/models'
import ItemIcon from './item-icon.vue'

// 职责：展示已守恒的联产物流与必须持续外运的余量；使用：全局或产线检查面板；边界：缓存容量不抵扣过剩速率。
defineProps<{balances: quant.ByproductBalance[]; format: (value: number) => string; stale: boolean}>()
</script>

<template>
  <!-- 联产物料按相同口径显示供需，旧方案始终保留过期提示。 -->
  <section v-if="balances.length" class="balances" aria-label="副产物收支">
    <h3>联产与副产物 <small>/min</small><span v-if="stale">待重新计算</span></h3>
    <div v-for="balance in balances" :key="balance.itemId" class="balance">
      <header><ItemIcon :name="balance.name" :size="18" /><strong>{{ balance.name }}</strong></header>
      <dl>
        <div><dt>本厂净产出</dt><dd>{{ format(balance.producedPerMinute) }}</dd></div>
        <div><dt>内部净消耗</dt><dd>{{ format(balance.consumedPerMinute) }}</dd></div>
        <div v-if="balance.importedPerMinute > 0"><dt>外供补足</dt><dd>{{ format(balance.importedPerMinute) }}</dd></div>
        <div v-if="balance.targetPerMinute > 0"><dt>目标交付</dt><dd>{{ format(balance.targetPerMinute) }}</dd></div>
        <div :class="{'surplus': balance.surplusPerMinute > 0}"><dt>仍需持续外运</dt><dd>{{ format(balance.surplusPerMinute) }}</dd></div>
      </dl>
      <p v-if="balance.surplusPerMinute > 0">内部消耗已优先分配。请接通副产物出口；库存装满会阻塞同配方的其他产物，缓存只能延缓积压。</p>
    </div>
  </section>
</template>

<style scoped lang="scss">
.balances { border-top: 1px solid var(--ds-border-subtle); padding-top: 8px; font-size: 11px; }
h3 { display: flex; gap: 6px; align-items: baseline; margin: 0 0 4px; color: var(--ds-text-secondary); font-size: 11px; }
h3 small { font-weight: 400; }
h3 span { margin-left: auto; color: var(--ds-warning); font-size: 10px; }
.balance { padding: 8px 0; }
.balance + .balance { border-top: 1px solid var(--ds-border-subtle); }
header { display: flex; align-items: center; gap: 6px; }
dl { display: grid; grid-template-columns: 1fr 1fr; gap: 5px 10px; margin: 7px 0 0; }
dl div { display: flex; justify-content: space-between; gap: 6px; }
dt { color: var(--ds-text-secondary); }
dd { margin: 0; font-family: var(--ds-font-display); font-variant-numeric: tabular-nums; }
.surplus dt, .surplus dd { color: var(--ds-warning); }
p { margin: 6px 0 0; color: var(--ds-text-secondary); font-size: 10px; line-height: 1.6; }
</style>
