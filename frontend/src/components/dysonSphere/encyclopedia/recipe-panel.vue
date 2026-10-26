<script setup lang="ts">
import {ArrowRight, Clock3} from '@lucide/vue'
import {computed} from 'vue'
import {resolveCatalogEntry} from '../../../services/encyclopedia/catalog'
import type {EncyclopediaEntry, EncyclopediaKey} from '../../../types/encyclopedia'
import EntryIcon from './entry-icon.vue'

const props = defineProps<{recipe: EncyclopediaEntry}>()
const emit = defineEmits<{(event: 'open', key: EncyclopediaKey): void}>()
// =============== 配方流程 ===============
// 说明：输入、产出与基础耗时直接读取原始数量关系，不计算存档加成或每分钟产量。
const inputs = computed(() => props.recipe.relations.filter(relation => relation.type === 'ingredient').map(relation => ({...relation, entry: resolveCatalogEntry(relation.target)})))
const outputs = computed(() => props.recipe.relations.filter(relation => relation.type === 'product').map(relation => ({...relation, entry: resolveCatalogEntry(relation.target)})))
const duration = computed(() => props.recipe.properties.find(property => property.label === '基础耗时'))
const producers = computed(() => props.recipe.relations.filter(relation => relation.type === 'producer').map(relation => ({...relation, entry: resolveCatalogEntry(relation.target)})))
</script>

<template>
  <!-- ~ 配方输入输出 -->
  <!-- * 说明：将数量关系呈现为可追溯的生产流程 -->
  <!-- * 交互：点击任一材料或产物继续查询，键盘操作与鼠标一致 -->
  <!-- * 依赖：已经加载的关系目标和配方基础耗时 -->
  <!-- * 边界：采集配方没有输入时显示“采集”，不将研究哈希标作物品 -->
  <div class="recipe-panel">
    <h3><button type="button" class="recipe-title-button" :aria-label="`查看${recipe.name.zh}的完整配方`" @click="emit('open', `recipe:${recipe.id}`)">{{ recipe.name.zh }}<ArrowRight :size="14" /></button></h3>
    <div class="recipe-flow">
      <div class="recipe-side">
        <p class="flow-label">输入</p>
        <div class="flow-items">
          <span v-if="!inputs.length" class="recipe-natural">采集</span>
          <template v-for="input in inputs" :key="input.target">
            <button v-if="input.entry" type="button" class="flow-item" :aria-label="`查看${input.entry.name.zh}`" @click="emit('open', input.target)">
              <EntryIcon :entry="input.entry" :size="38" /><span>{{ input.entry.name.zh }}</span><strong>× {{ input.quantity }}</strong>
            </button>
          </template>
        </div>
      </div>
      <div class="recipe-direction"><span v-if="duration" class="recipe-time"><Clock3 :size="13" />{{ duration.value }} s</span><ArrowRight :size="23" /></div>
      <div class="recipe-side">
        <p class="flow-label">产出</p>
        <div class="flow-items">
          <template v-for="output in outputs" :key="output.target">
            <button v-if="output.entry" type="button" class="flow-item" :aria-label="`查看${output.entry.name.zh}`" @click="emit('open', output.target)">
              <EntryIcon :entry="output.entry" :size="38" /><span>{{ output.entry.name.zh }}</span><strong>× {{ output.quantity }}{{ output.unit === '哈希' ? ' 哈希' : '' }}</strong>
            </button>
          </template>
        </div>
      </div>
    </div>
    <div v-if="producers.length" class="recipe-producers"><span>制作设备</span><template v-for="producer in producers" :key="producer.target"><button v-if="producer.entry" type="button" @click="emit('open', producer.target)">{{ producer.entry.name.zh }}</button></template></div>
  </div>
</template>

<style scoped lang="scss">
.recipe-panel { border: 1px solid var(--ds-border-default); border-radius: 4px; padding: 16px; background: var(--ds-hover-bg); }
h3 { margin: 0 0 15px; color: var(--ds-text-primary); font-size: 14px; font-weight: 700; }
.recipe-title-button { display: inline-flex; align-items: center; gap: 7px; border: 0; padding: 0; background: transparent; color: inherit; font-size: inherit; font-weight: inherit; cursor: pointer; }
.recipe-title-button:hover { color: var(--ds-brand); }
.recipe-flow { display: grid; grid-template-columns: minmax(0, 1fr) 48px minmax(0, 1fr); align-items: center; gap: 8px; }
.flow-label { margin: 0 0 8px; color: var(--ds-text-secondary); font-size: 13px; }
.flow-items { display: flex; flex-wrap: wrap; align-items: flex-start; gap: 8px; }
.flow-item { display: flex; max-width: 110px; flex-direction: column; align-items: center; gap: 3px; border: 1px solid transparent; border-radius: 3px; padding: 4px; background: transparent; color: var(--ds-text-primary); font-size: 13px; line-height: 1.5; cursor: pointer; }
.flow-item:hover, .flow-item:focus-visible { outline: none; border-color: var(--ds-border-strong); background: var(--ds-active-bg); }
.flow-item strong { font-family: var(--ds-font-display); font-weight: 600; }
.recipe-direction { display: flex; flex-direction: column; align-items: center; gap: 7px; color: var(--ds-text-secondary); }
.recipe-time { display: inline-flex; align-items: center; gap: 4px; font-family: var(--ds-font-display); font-size: 13px; white-space: nowrap; }
.recipe-natural { padding: 12px 0; color: var(--ds-text-secondary); font-size: 14px; }
.recipe-producers { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 9px; margin-top: 15px; border-top: 1px solid var(--ds-border-default); padding-top: 11px; color: var(--ds-text-secondary); font-size: 13px; }
.recipe-producers button { border: 0; padding: 2px 0; background: transparent; color: var(--ds-text-primary); font-size: 13px; cursor: pointer; }
.recipe-producers button:hover { color: var(--ds-brand); }
</style>
