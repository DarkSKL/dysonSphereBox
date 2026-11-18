<script setup lang="ts">
import {computed} from 'vue'
import type {SelectFilter} from 'naive-ui'
import {FolderOpen} from '@lucide/vue'
import ProductionDiagnosis from '../../components/dysonSphere/production-diagnosis.vue'
import {useSaveAnalysis} from '../../composables/dysonSphere/useSaveAnalysis'

definePage({meta: {layout: 'main', title: '存档分析'}})

const {dialog, directory, saves, selected, world, star, planet, item, stars, planets, items,
  report, sorterReport, detail, busy, error, pickerError, itemName, planetName, openPicker, authorize, analyze, chooseStar, choosePlanet, chooseItem} = useSaveAnalysis()
const subtitle = computed(() => selected.value ? selected.value.fileName : '选一份存档，看看哪些机器卡住了，该先去哪里看。')
const filterProduct: SelectFilter = (pattern, option) => {
  const query = pattern.trim().toLowerCase()
  return String(option.label).toLowerCase().includes(query) || `#${option.value}`.includes(query)
}
function closeDialog() { if (!busy.value) dialog.value = null }
</script>

<template>
  <!-- 使用盒子现有外壳；选档、筛选及诊断共享一个页面。 -->
  <section class="analysis-page" aria-label="存档分析">
    <header class="heading">
      <div><h1>存档分析</h1><p :class="{'type-mono': selected}">{{ subtitle }}</p></div>
      <n-button :disabled="!!busy" :type="selected ? 'default' : 'primary'" @click="openPicker">
        <template #icon><FolderOpen :size="16" /></template>{{ selected ? '切换存档' : '选择存档' }}
      </n-button>
    </header>
    <div v-if="world" class="filters">
      <div><label>恒星</label><n-select :value="star" :options="stars" filterable placeholder="搜索恒星名称或编号" aria-label="搜索恒星" @update:value="chooseStar" /></div>
      <div><label>星球</label><n-select :value="planet" :options="planets" filterable placeholder="选择星球" aria-label="选择星球" @update:value="choosePlanet" /></div>
      <div><label>产物</label><n-select :value="item" :options="items" :disabled="!items.length" filterable :filter="filterProduct" placeholder="选择产物" aria-label="选择产物" @update:value="chooseItem" /></div>
    </div>
    <p v-if="world && !world.completePlanets" class="hint">星系数据不完整。{{ (world.missing || []).join('；') }}</p>
    <p v-if="busy" class="notice" role="status"><n-spin :size="16" />{{ busy }}…</p>
    <n-alert v-if="error" type="error" :show-icon="false">{{ error }}</n-alert>
    <ProductionDiagnosis v-if="report" :report="report" :sorter-report="sorterReport" :detail="detail" :item-name="itemName" :planet-name="planetName" />
    <p v-else-if="world && !busy && !error" class="hint">{{ !planet ? '选择要检查的星球。' : items.length ? '选择一种产物，查看这颗星球当时的生产情况。' : '在这颗星球上没有找到可诊断的生产设备。' }}</p>
  </section>

  <!-- 授权弹窗展示真实目录；同意后列档，点击具体存档才进行解析。 -->
  <n-modal :show="dialog !== null" :mask-closable="!busy" :close-on-esc="!busy" @update:show="v => { if (!v) closeDialog() }">
    <n-card class="save-dialog" role="dialog" aria-modal="true" :aria-label="dialog === 'authorize' ? '存档目录读取授权' : '选择存档'" :title="dialog === 'authorize' ? '允许读取存档目录？' : '选择存档'" :bordered="false">
      <template v-if="dialog === 'authorize'">
        <p>允许后列出目录中的存档，选定一份后才读取内容。离开存档分析时释放授权和快照。</p>
        <p class="directory type-mono">{{ directory?.path || '尚未找到存档目录' }}</p>
        <p v-if="!directory?.exists" class="hint">请先在资源管理中确认存档目录，再重新选择存档。</p>
      </template>
      <template v-else-if="dialog === 'saves'">
        <div v-if="saves.length" class="save-list">
          <button v-for="s in saves" :key="s.id" class="save-row" @click="analyze(s)">
            <strong>{{ s.name || s.fileName }}</strong><span>{{ s.relativePath }} · {{ (s.sizeBytes / 1048576).toFixed(1) }} MiB</span>
          </button>
        </div>
        <p v-else>授权目录中没有可分析的存档。</p>
      </template>
      <n-alert v-if="pickerError" type="warning" :show-icon="false">{{ pickerError }}</n-alert>
      <template #footer><div class="actions">
        <n-button :disabled="!!busy" @click="closeDialog">取消</n-button>
        <n-button v-if="dialog === 'authorize'" type="primary" :loading="!!busy" :disabled="!directory?.exists" @click="authorize">允许并查看存档</n-button>
      </div></template>
    </n-card>
  </n-modal>
</template>

<style scoped lang="scss">
.analysis-page { width: 100%; max-width: var(--ds-page-max-width); margin-inline: auto; padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom); container-type: inline-size; }
.heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 14px; padding-bottom: 16px; border-bottom: 1px solid var(--ds-border-default); }
.heading > div { min-width: 0; }
h1 { font-size: 22px; font-weight: 650; color: var(--ds-text-primary); }
.heading p, .hint, .notice { margin: 5px 0; color: var(--ds-text-secondary); overflow-wrap: anywhere; }
.filters { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.3fr) minmax(0, 1fr); gap: 12px; margin: 18px 0 22px; }
.filters label { display: block; margin-bottom: 4px; font-size: 13px; color: var(--ds-text-secondary); }
.notice, .actions { display: flex; align-items: center; gap: 8px; }
.notice { padding-block: 12px; }
.actions { justify-content: flex-end; }
.save-dialog { width: min(490px, calc(100% - 32px)); }
.directory { margin: 12px 0; padding: 8px; background: var(--ds-bg-panel); overflow-wrap: anywhere; }
.save-list { max-height: 54vh; overflow-y: auto; }
.save-row { display: block; width: 100%; text-align: left; padding: 9px 8px; border-bottom: 1px solid var(--ds-border-subtle); overflow-wrap: anywhere; }
.save-row span { display: block; color: var(--ds-text-secondary); font-size: 12px; font-family: var(--ds-font-mono); }
.save-row:hover, .save-row:focus-visible { background: var(--ds-bg-panel); color: var(--ds-brand); }
@container (max-width: 540px) { .filters { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); } .filters > :last-child { grid-column: 1/-1; } }
</style>
