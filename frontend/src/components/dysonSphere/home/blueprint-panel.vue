<script setup lang="ts">
import {computed, onBeforeUnmount, onMounted, ref} from 'vue'
import type {BlueprintAppearance, SphereBlueprint} from '../../../packages/dyson-sphere/src/blueprint'

const props = defineProps<{
  applied: SphereBlueprint | null
  appearance: BlueprintAppearance
  savingAppearance: boolean
  appearanceError: string
  preview: SphereBlueprint | null
  parsing: boolean
  saving: boolean
  supported: boolean
  hasSaved: boolean
  showHidden: boolean
  error: string
}>()
const emit = defineEmits<{
  (event: 'part', key: keyof BlueprintAppearance, visible: boolean): void
  (event: 'inspect', source: string | File): void
  (event: 'discard' | 'close' | 'apply' | 'reset'): void
  (event: 'hidden', visible: boolean): void
}>()
const displayed = computed(() => props.preview ?? props.applied)
const text = ref('')
const fileInput = ref<HTMLInputElement | null>(null)
const heading = ref<HTMLElement | null>(null)
const filename = ref('')
const stats = computed(() => {
  const layers = props.preview?.layers ?? []
  return [
    ['球层', layers.length],
    ['节点', layers.reduce((sum, layer) => sum + layer.nodes.length, 0)],
    ['框架', layers.reduce((sum, layer) => sum + layer.frames.length, 0)],
    ['壳面', layers.reduce((sum, layer) => sum + layer.shells.length, 0)],
  ]
})
const typeLabel = computed(() => ({1: '单层球壳', 2: '多层球壳', 4: '整球蓝图'}[props.preview?.type ?? 0]))

function selectFile(event: Event): void {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  text.value = ''
  filename.value = file.name
  emit('inspect', file)
}

function editText(value: string): void {
  text.value = value
  filename.value = ''
  emit('discard')
}

function escape(event: KeyboardEvent): void {
  if (event.key === 'Escape' && !event.isComposing && !props.saving) emit('close')
}

onMounted(() => {
  heading.value?.focus()
  window.addEventListener('keydown', escape)
})
onBeforeUnmount(() => window.removeEventListener('keydown', escape))
</script>

<template>
  <!-- 球壳直接在旁边的主页场景预览；此面板仅展示原文入口、解析结果和提交操作。 -->
  <section class="blueprint-panel" aria-labelledby="sphere-editor-title" :aria-busy="parsing || saving">
    <h2 id="sphere-editor-title" ref="heading" class="m-0 text-lg" tabindex="-1">主页球壳蓝图</h2>
    <template v-if="supported && !preview">
      <label for="sphere-blueprint-code">粘贴游戏内复制的球壳蓝图</label>
      <n-input class="type-mono" type="textarea" :rows="3" :value="text"
        :input-props="{id: 'sphere-blueprint-code'}" placeholder="DYBP:…" :disabled="saving"
        @update:value="editText" />
      <div class="flex flex-wrap items-center gap-2">
        <n-button size="small" type="primary" :disabled="!text.trim() || saving" :loading="parsing && !filename"
          @click="emit('inspect', text)">预览蓝图</n-button>
        <n-button size="small" :disabled="saving" :loading="parsing && !!filename"
          @click="fileInput?.click()">打开本地文件</n-button>
        <input ref="fileInput" type="file" accept=".txt,text/plain" hidden @change="selectFile" />
      </div>
    </template>
    <template v-if="preview">
      <p class="m-0"><strong>{{ typeLabel }}</strong> · 游戏版本 <span class="type-mono">{{ preview.gameVersion }}</span></p>
      <dl class="stats">
        <div v-for="[label, value] in stats" :key="label">
          <dt>{{ label }}</dt><dd>{{ value }}</dd>
        </div>
      </dl>
      <p v-if="preview.type === 4" class="m-0">蓝图含 {{ preview.orbits.length }} 条太阳帆轨道。</p>
      <n-checkbox v-if="preview.layers.some(layer => !layer.visible)" :checked="showHidden"
        :disabled="saving || !supported" @update:checked="emit('hidden', $event)">预览隐藏球层</n-checkbox>
      <p class="hint" v-if="supported">滚轮缩放，按住鼠标中键拖动查看。</p>
    </template>
    <fieldset v-if="supported && displayed" class="appearance">
      <legend>首页显示（自动保存）</legend>
      <n-checkbox :checked="appearance.structure" :disabled="savingAppearance"
        @update:checked="emit('part', 'structure', $event)">框架与节点</n-checkbox>
      <n-checkbox :checked="appearance.surfaceDetail" :disabled="savingAppearance"
        @update:checked="emit('part', 'surfaceDetail', $event)">壳面纹理</n-checkbox>
      <n-checkbox v-if="displayed.orbits.length" :checked="appearance.orbits" :disabled="savingAppearance"
        @update:checked="emit('part', 'orbits', $event)">太阳帆轨道线</n-checkbox>
      <p v-if="!appearance.structure && displayed.layers.every(layer => !layer.shells.length)" class="hint">
        这是纯框架蓝图，勾选「框架与节点」即可显示骨架。
      </p>
      <p v-if="appearanceError" role="alert" class="error">{{ appearanceError }}</p>
    </fieldset>
    <p v-if="filename" class="filename type-mono" :title="filename">{{ filename }}</p>
    <p v-if="!supported" class="hint" role="status">当前使用静态画面，实时预览需要 WebGL。</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <div class="actions">
      <n-button v-if="preview" size="small" type="primary" :disabled="!supported" :loading="saving"
        @click="emit('apply')">应用到主页</n-button>
      <n-button v-if="preview && supported" size="small" :disabled="saving" @click="emit('discard')">更换蓝图</n-button>
      <n-button size="small" :disabled="saving" @click="emit('close')">{{ preview || text || filename ? '取消' : '关闭' }}</n-button>
      <n-button v-if="hasSaved" size="small" text :disabled="saving" @click="emit('reset')">恢复默认球</n-button>
    </div>
  </section>
</template>

<style scoped lang="scss">
.blueprint-panel {
  position: absolute;
  top: 50%;
  left: clamp(12px, 3cqw, 48px);
  width: min(35cqw, 360px);
  min-width: min(250px, calc(100% - 24px));
  max-height: calc(100% - 24px);
  overflow: auto;
  transform: translateY(-50%);
  display: grid;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-primary);
  font-size: 13px;
}
.appearance {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  margin: 0;
  padding: 0;
  border: 0;
}
.appearance legend { margin-bottom: 6px; color: var(--ds-text-secondary); }
.stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 4px;
  margin: 0;
  color: var(--ds-text-secondary);
}
.stats dd {
  margin: 0;
  color: var(--ds-text-primary);
  font: 600 17px var(--ds-font-display);
}
.error, .hint, .filename {
  margin: 0;
  overflow-wrap: anywhere;
  color: var(--ds-text-secondary);
}
.error { color: var(--ds-error); }
.actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  border-top: 1px solid var(--ds-border-subtle);
  padding-top: 12px;
}
@container (max-width: 540px) {
  .blueprint-panel {
    top: auto;
    bottom: 8px;
    left: 8px;
    width: calc(100% - 16px);
    max-height: 48%;
    transform: none;
  }
}
</style>
