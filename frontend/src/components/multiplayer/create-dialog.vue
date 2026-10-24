<script setup lang="ts">
import {Plus, Trash2, X} from '@lucide/vue'
import {computed, reactive, ref, watch} from 'vue'
import type {GamePortPreset, PortProto, RoomCreateForm} from '../../types/multiplayer'

// =============== 创建表单与端口校验 ===============
// 说明：控制链路在线时编辑和提交；人数/端口上限以服务端会话为准，关闭对话框或断线都保留已有草稿。
const props = defineProps<{show: boolean; online: boolean; loading: boolean; busy: boolean; maxMembers: number; maxPorts: number; gamePortPresets: GamePortPreset[]; presetsLoading: boolean}>()
const emit = defineEmits<{(event: 'create', form: RoomCreateForm): void; (event: 'update:show', value: boolean): void}>()
type PortMode = PortProto | `game:${string}`
interface PortDraft {id: number; mode: PortMode; proto: PortProto; port: number | null; count: number | null; label: string}
const form = reactive<{name: string; visibility: 'public' | 'private'; password: string; maxMembers: number | null}>({name: '', visibility: 'public', password: '', maxMembers: 8})
const ports = ref<PortDraft[]>([{id: 0, mode: 'tcp', proto: 'tcp', port: 8469, count: 1, label: '游戏'}])
let nextPortId = 1
const attempted = ref(false)
const visibilityOptions = [{label: '公开 · 大厅可见', value: 'public'}, {label: '私密 · 邀请码加入', value: 'private'}]
// =============== 游戏端口自动识别选项 ===============
// 说明：协议下拉同时提供手动 TCP/UDP 与后端登记的游戏选项；选中游戏项会回填端口并锁定端口输入。
// 使用：页面打开创建房间时探测并把 gamePortPresets 传入；本组件只负责展示、禁用与回填。
// 边界：游戏未运行或无端口时选项禁用；自动结果不可用或过期时自动行退回手动协议，避免端口被锁死。
type PortModeOption = {label: string; value: string; disabled?: boolean}
type PortModeGroup = {type: 'group'; key: string; label: string; children: PortModeOption[]}
const manualModeOptions: PortModeOption[] = [
  {label: 'TCP', value: 'tcp'},
  {label: 'UDP', value: 'udp'},
  {label: 'TCP + UDP', value: 'both'},
]
const protoLabels: Record<PortProto, string> = {tcp: 'TCP', udp: 'UDP', both: 'TCP + UDP'}
const GAME_MODE_PREFIX = 'game:'

function gameMode(id: string): PortMode { return `${GAME_MODE_PREFIX}${id}` }
function isGameMode(mode: string): boolean { return mode.startsWith(GAME_MODE_PREFIX) }
function asPortProto(value: string): PortProto { return value === 'udp' || value === 'both' ? value : 'tcp' }
function presetLabel(preset: GamePortPreset): string {
  if (!preset.available) return `${preset.name}（${preset.reason || '不可用'}）`
  const first = preset.ports[0]
  return first ? `${preset.name} · ${protoLabels[asPortProto(first.proto)]}` : preset.name
}
function portModeOptions(decl: PortDraft): PortModeGroup[] {
  const used = new Set(ports.value.filter(item => item.id !== decl.id && isGameMode(item.mode)).map(item => item.mode))
  const gameOptions: PortModeOption[] = props.gamePortPresets.map(preset => {
    const value = gameMode(preset.id)
    const usedByOther = used.has(value)
    return {label: usedByOther ? `${presetLabel(preset)}（已用于其它端口）` : presetLabel(preset), value, disabled: props.presetsLoading || !preset.available || usedByOther}
  })
  if (!gameOptions.length) gameOptions.push({label: props.presetsLoading ? '正在检测游戏…' : '暂不可用', value: `${GAME_MODE_PREFIX}unavailable`, disabled: true})
  return [
    {type: 'group', key: 'manual', label: '手动设置', children: manualModeOptions},
    {type: 'group', key: 'game', label: '游戏自动识别', children: gameOptions},
  ]
}
function onModeChange(decl: PortDraft, value: unknown): void {
  if (typeof value !== 'string') return
  if (value.startsWith(GAME_MODE_PREFIX)) {
    const preset = props.gamePortPresets.find(item => gameMode(item.id) === value)
    if (preset) applyGamePreset(decl, preset)
    return
  }
  if (value === 'tcp' || value === 'udp' || value === 'both') {
    decl.mode = value
    decl.proto = value
  }
}
function applyGamePreset(decl: PortDraft, preset: GamePortPreset): void {
  const first = preset.ports[0]
  if (!preset.available || !first) return
  if (ports.value.some(item => item.id !== decl.id && item.mode === gameMode(preset.id))) return
  decl.mode = gameMode(preset.id)
  decl.proto = asPortProto(first.proto)
  decl.port = first.port
  for (const extra of preset.ports.slice(1)) {
    if (ports.value.length >= props.maxPorts) break
    const proto = asPortProto(extra.proto)
    if (ports.value.some(item => item.proto === proto && item.port === extra.port)) continue
    ports.value.push({id: nextPortId++, mode: proto, proto, port: extra.port, count: 1, label: ''})
  }
}
// 说明：弹窗每次打开都会重新探测；自动行跟随最新结果，游戏不可用时退回手动协议。
watch([() => props.gamePortPresets, () => props.presetsLoading], ([presets, loading]) => {
  if (loading) return
  for (const decl of ports.value) {
    if (!isGameMode(decl.mode)) continue
    const preset = presets.find(item => gameMode(item.id) === decl.mode)
    const first = preset?.ports[0]
    if (preset?.available && first) {
      decl.proto = asPortProto(first.proto)
      decl.port = first.port
    } else {
      decl.mode = decl.proto
    }
  }
})

// =============== 端口高级选项显隐 ===============
// 说明：暂时隐藏“连续端口与名称”入口，创建流程只暴露协议与端口号；字段、校验与提交结构全部保留。
// 使用：模板只根据该常量决定是否渲染 details；需要恢复入口时改为 true，不改动 PortDraft 与提交载荷。
// 边界：隐藏期间连续数按默认 1 提交、端口名称保留默认值，不会触发对应的输入与校验界面。
const PORT_OPTIONS_VISIBLE = false

const error = computed(() => {
  const nameLength = Array.from(form.name.trim()).length
  if (!nameLength || nameLength > 32) return '房间名需填写 1–32 个字符。'
  const passwordLength = Array.from(form.password).length
  if (passwordLength && (passwordLength < 4 || passwordLength > 32)) return '房间密码需填写 4–32 个字符，或留空。'
  if (!form.maxMembers || !Number.isInteger(form.maxMembers) || form.maxMembers < 2 || form.maxMembers > props.maxMembers) return `人数上限需为 2–${props.maxMembers} 人（含房主）。`
  if (!ports.value.length || ports.value.length > props.maxPorts) return `请保留 1–${props.maxPorts} 条端口声明。`
  const occupied = new Set<string>()
  for (const [index, decl] of ports.value.entries()) {
    if (!decl.port || !Number.isInteger(decl.port) || decl.port < 1 || decl.port > 65535) return `第 ${index + 1} 条端口需为 1–65535 的整数。`
    if (!decl.count || !Number.isInteger(decl.count) || decl.count < 1 || decl.count > 32 || decl.port + decl.count - 1 > 65535) return `第 ${index + 1} 条连续端口数需为 1–32，末端不能超过 65535。`
    if (Array.from(decl.label).length > 16) return `第 ${index + 1} 条端口名称不能超过 16 个字符。`
    for (const proto of decl.proto === 'both' ? ['tcp', 'udp'] : [decl.proto]) {
      for (let port = decl.port; port < decl.port + decl.count; port++) {
        const key = `${proto}:${port}`
        if (occupied.has(key)) return `第 ${index + 1} 条端口与已有 ${proto.toUpperCase()} 端口重叠。`
        occupied.add(key)
      }
    }
  }
  return ''
})
// 说明：新增端口不猜测用户游戏配置；稳定 id 避免删除行后复用错误的输入控件。
function addPort(): void {
  if (props.online && !props.loading && ports.value.length < props.maxPorts) ports.value.push({id: nextPortId++, mode: 'udp', proto: 'udp', port: null, count: 1, label: ''})
}
function submit(): void {
  attempted.value = true
  if (!props.online || props.busy || error.value) return
  emit('create', {...form, name: form.name.trim(), maxMembers: form.maxMembers ?? 0, ports: ports.value.map(({proto, port, count, label}) => ({proto, port: port ?? 0, count: count ?? 0, label}))})
}
function close(): void { if (!props.loading) emit('update:show', false) }
</script>

<template>
  <n-modal :show="show" :mask-closable="!loading" :close-on-esc="!loading" @update:show="emit('update:show', $event)">
  <!-- ~ 创建房间：基础信息优先，端口扩展按需展开；创建后端口表不可修改 -->
  <!-- * 当前 vueuc 的焦点锁定只识别 div 根节点，保留此结构以支持输入和 Tab 导航。 -->
  <div class="create-dialog" role="dialog" aria-modal="true" aria-labelledby="create-title">
    <header class="dialog-header">
      <div><h2 id="create-title">创建房间</h2><p class="panel-intro">你来当房主，邀请好友一起探索。</p></div>
      <n-button quaternary circle size="small" aria-label="关闭" :disabled="loading" @click="close"><template #icon><X :size="16" /></template></n-button>
    </header>
    <form @submit.prevent="submit">
      <div class="form-fields">
        <label class="field"><span>房间名</span><n-input v-model:value="form.name" placeholder="给这次星际旅程起个名字" maxlength="32" :disabled="!online || loading" /></label>
        <div class="fields-pair">
          <label class="field"><span>可见性</span><n-select v-model:value="form.visibility" :options="visibilityOptions" :disabled="!online || loading" /></label>
          <label class="field"><span>人数上限 · 含房主</span><n-input-number v-model:value="form.maxMembers" :min="2" :max="maxMembers" :precision="0" :disabled="!online || loading" /></label>
        </div>
        <label class="field"><span>房间密码 <small>可选</small></span><n-input v-model:value="form.password" type="password" show-password-on="click" placeholder="留空则无需密码" maxlength="32" :disabled="!online || loading" /></label>
      </div>

      <!-- ~ 游戏端口：默认首行直接编辑；协议下拉提供手动 TCP/UDP 与游戏自动识别 -->
      <!-- * 说明：自动识别项由后端探测游戏进程与主机端口，选中后回填端口并锁定输入 -->
      <!-- * 边界：游戏未运行或无端口时选项禁用；连续端口与名称入口暂时隐藏（见 PORT_OPTIONS_VISIBLE） -->
      <div class="ports-heading"><h3>游戏端口</h3><span>{{ ports.length }} / {{ maxPorts }}</span></div>
      <p class="port-hint">填写房主游戏实际使用的端口，或在协议中选择“戴森球计划”自动识别。</p>
      <div v-for="(decl, index) in ports" :key="decl.id" class="port-editor">
        <div class="port-row"><n-select :value="decl.mode" :options="portModeOptions(decl)" to="body" :menu-props="{class: 'port-mode-menu'}" :disabled="!online || loading" :aria-label="`第 ${index + 1} 条端口类型`" @update:value="onModeChange(decl, $event)" /><n-input-number v-model:value="decl.port" class="type-mono" :min="1" :max="65535" :precision="0" placeholder="端口号" :show-button="false" :disabled="!online || loading || isGameMode(decl.mode)" :input-props="{'aria-label': `第 ${index + 1} 条端口号`}" /><n-tooltip><template #trigger><n-button quaternary :disabled="!online || loading || ports.length <= 1" :aria-label="`删除第 ${index + 1} 条端口`" @click="ports = ports.filter(item => item.id !== decl.id)"><template #icon><Trash2 :size="14" /></template></n-button></template>删除端口</n-tooltip></div>
        <!-- * 说明：连续端口数与端口名称的编辑入口暂时隐藏，避免创建流程暴露未定稿的高级配置 -->
        <!-- * 边界：隐藏期间仍按 count=1、label 默认值提交；恢复入口只需把 PORT_OPTIONS_VISIBLE 改为 true -->
        <details v-if="PORT_OPTIONS_VISIBLE" class="port-options"><summary>连续端口与名称</summary><div class="fields-pair"><label class="field"><span>连续端口数</span><n-input-number v-model:value="decl.count" :min="1" :max="32" :precision="0" :disabled="!online || loading" /></label><label class="field"><span>端口名称</span><n-input v-model:value="decl.label" placeholder="例如：游戏" maxlength="16" :disabled="!online || loading" /></label></div></details>
      </div>
      <n-button size="small" dashed block :disabled="!online || loading || ports.length >= maxPorts" @click="addPort"><template #icon><Plus :size="14" /></template>添加端口</n-button>
      <p class="privacy-hint">{{ form.visibility === 'public' ? '公开房间将在大厅显示，加入者可以看到你的网络地址。' : '私密房间不在大厅显示。创建后生成邀请码，分享给好友。' }}<br>创建后无法修改游戏端口。</p>
      <p v-if="attempted && error" class="form-error" role="alert">{{ error }}</p>
      <p v-if="!online" class="offline-hint">连接已中断。表单已保留，恢复连接后可继续创建。</p>
      <div class="dialog-actions">
        <n-button :disabled="loading" @click="close">取消</n-button>
        <n-button type="primary" attr-type="submit" :loading="loading" :disabled="!online || busy">创建并进入房间</n-button>
      </div>
    </form>
  </div>
  </n-modal>
</template>

<style scoped lang="scss">
.create-dialog { width: 520px; max-width: calc(100vw - 32px); max-height: calc(100vh - 64px); overflow-y: auto; padding: 24px; border: 1px solid var(--ds-border-default); border-radius: 4px; background: var(--ds-bg-panel); color: var(--ds-text-primary); }
.dialog-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 24px; }
h2 { margin: 0 0 6px; font-size: 18px; }
.panel-intro { margin: 0; font-size: 13px; color: var(--ds-text-secondary); }
.dialog-actions { display: flex; justify-content: flex-end; gap: 8px; }
.form-fields { display: grid; gap: 18px; }
.field { display: flex; flex-direction: column; gap: 8px; font-size: 12px; color: var(--ds-text-secondary); }
.field small { margin-left: 6px; color: var(--ds-text-muted); font-size: 11px; }
.fields-pair { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr); gap: 12px; }
.field :deep(.n-input-number) { width: 100%; }
.ports-heading { display: flex; justify-content: space-between; align-items: center; margin: 28px 0 6px; }
h3 { margin: 0; font-size: 13px; }
.ports-heading > span { color: var(--ds-text-secondary); font-family: var(--ds-font-display); font-size: 12px; }
.port-hint { color: var(--ds-text-secondary); font-size: 12px; margin: 0 0 12px; }
.port-editor { margin-bottom: 14px; }
.port-row { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 30px; gap: 8px; align-items: center; }
/* 端口类型菜单挂在 body，固定最大高度并内部滚动，避免展开项撑高弹窗容器。 */
:global(.port-mode-menu .n-scrollbar) { max-height: 240px; }
.port-options { margin-top: 8px; font-size: 11px; color: var(--ds-text-secondary); }
.port-options summary { cursor: pointer; padding: 4px 0; }
.port-options .fields-pair { margin-top: 8px; }
.privacy-hint { margin: 20px 0; font-size: 11px; color: var(--ds-text-secondary); line-height: 1.8; }
.form-error { margin: 0 0 14px; color: var(--ds-error); font-size: 12px; line-height: 1.8; }
.offline-hint { margin: 0 0 14px; font-size: 12px; color: var(--ds-text-secondary); }
</style>
