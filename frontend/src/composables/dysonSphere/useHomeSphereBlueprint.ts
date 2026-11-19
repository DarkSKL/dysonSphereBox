import {onBeforeUnmount, ref, shallowRef} from 'vue'
import {PreviewSphereBlueprint} from '../../../wailsjs/go/factory/Service'
import {GetConfig, SetConfig} from '../../../wailsjs/go/system/ConfigurationService'
import {DEFAULT_BLUEPRINT_APPEARANCE, type BlueprintAppearance, type SphereBlueprint} from '../../packages/dyson-sphere/src/blueprint'
import {formatErrorMessage} from '../../utils/format'

// 配置只保存确认过的原文。切页后再进入时，先等待上一页发出的写操作完成。
const CONFIG_GROUP = 'Home'
const CONFIG_KEY = 'SphereBlueprint'
// 与 Go 球壳解析器的原文上限一致，在读取大文件或进入桌面桥前即可提示。
const MAX_TEXT_BYTES = 8 * 1024 * 1024
let pendingSave: Promise<void> = Promise.resolve()

const HIDDEN_PARTS_KEY = 'SphereBlueprintHiddenParts'
const appearanceKeys = Object.keys(DEFAULT_BLUEPRINT_APPEARANCE) as (keyof BlueprintAppearance)[]
let pendingAppearanceSave: Promise<void> = Promise.resolve()

async function decode(code: string): Promise<SphereBlueprint> {
  if (code.length > MAX_TEXT_BYTES) throw new Error('蓝图文本超过 8 MiB')
  const model: SphereBlueprint = await PreviewSphereBlueprint(code)
  if (model.type === 3) throw new Error('这份蓝图仅包含太阳帆轨道，请选择含球壳的蓝图。')
  // Go 的空切片可能通过 JSON 表示为 null；骨架作品允许零壳面，不能凭空补全几何。
  return {...model, orbits: model.orbits ?? [], layers: (model.layers ?? []).map(layer => ({
    ...layer, nodes: layer.nodes ?? [], frames: layer.frames ?? [], shells: layer.shells ?? [],
  }))}
}

// 预览和已应用模型分别持有，避免 Vue 深度代理几何数据。只有场景成功构建后才能提交原文。
export function useHomeSphereBlueprint(render: (model: SphereBlueprint | null) => void) {
  const appearance = shallowRef<BlueprintAppearance>({...DEFAULT_BLUEPRINT_APPEARANCE})
  const savingAppearance = ref(false)
  const appearanceError = ref('')

  async function setPart(key: keyof BlueprintAppearance, visible: boolean): Promise<void> {
    if (savingAppearance.value) return
    const previous = appearance.value
    const next = {...previous, [key]: visible}
    appearance.value = next
    appearanceError.value = ''
    savingAppearance.value = true
    try {
      const hidden = appearanceKeys.filter(part => !next[part]).join(',')
      const write = SetConfig(CONFIG_GROUP, HIDDEN_PARTS_KEY, hidden)
      pendingAppearanceSave = write.catch(() => {})
      await write
    } catch (err) {
      appearance.value = previous
      appearanceError.value = '保存显示设置失败：' + formatErrorMessage(err)
    } finally {
      savingAppearance.value = false
    }
  }

  const editing = ref(false)
  const restoring = ref(true)
  const parsing = ref(false)
  const saving = ref(false)
  const supported = ref(false)
  const hasSaved = ref(false)
  const error = ref('')
  const applied = shallowRef<SphereBlueprint | null>(null)
  const preview = shallowRef<SphereBlueprint | null>(null)
  const showHidden = ref(false)
  let previewCode = ''
  let revision = 0
  let displayed: SphereBlueprint | null = null
  let redraw = false

  function display(model: SphereBlueprint | null): void {
    if (!supported.value || (model === displayed && !redraw)) return
    render(model)
    displayed = model
    redraw = false
  }

  function discardPreview(): void {
    ++revision
    parsing.value = false
    previewCode = ''
    preview.value = null
    showHidden.value = false
    error.value = ''
    try {
      display(applied.value)
    } catch (err) {
      error.value = '恢复上次作品失败：' + formatErrorMessage(err)
    }
  }

  async function restore(available: boolean): Promise<void> {
    supported.value = available
    const current = ++revision
    try {
      await Promise.all([pendingSave, pendingAppearanceSave])
      const [raw, hidden] = await Promise.all([
        GetConfig(CONFIG_GROUP, CONFIG_KEY),
        GetConfig(CONFIG_GROUP, HIDDEN_PARTS_KEY),
      ])
      if (current !== revision) return
      const parts = new Set(hidden.split(','))
      const next = {...DEFAULT_BLUEPRINT_APPEARANCE}
      for (const key of appearanceKeys) next[key] = !parts.has(key)
      appearance.value = next
      hasSaved.value = Boolean(raw)
      if (!raw || !available) return
      const model = await decode(raw)
      if (current !== revision) return
      display(model)
      applied.value = model
    } catch (err) {
      if (current === revision) error.value = '读取主页蓝图失败：' + formatErrorMessage(err)
    } finally {
      if (current === revision) restoring.value = false
    }
  }

  async function inspect(source: string | File): Promise<void> {
    if (!supported.value || saving.value || restoring.value) return
    discardPreview()
    const current = revision
    parsing.value = true
    try {
      if (typeof source !== 'string' && source.size > MAX_TEXT_BYTES) throw new Error('蓝图文件超过 8 MiB')
      const raw = typeof source === 'string' ? source : await source.text()
      if (current !== revision) return
      const model = await decode(raw)
      if (current !== revision || !supported.value) return
      display(model)
      preview.value = model
      previewCode = raw
    } catch (err) {
      if (current === revision) error.value = '无法预览：' + formatErrorMessage(err)
    } finally {
      if (current === revision) parsing.value = false
    }
  }

  function toggleHidden(visible: boolean): void {
    const model = preview.value
    if (!model || saving.value) return
    try {
      display(visible ? {...model, layers: model.layers.map(layer => ({...layer, visible: true}))} : model)
      showHidden.value = visible
    } catch (err) {
      discardPreview()
      error.value = '无法显示球层：' + formatErrorMessage(err)
    }
  }

  async function commit(reset = false): Promise<void> {
    if (saving.value || restoring.value || (!reset && (!preview.value || !supported.value))) return
    const model = reset ? null : preview.value
    const raw = reset ? '' : previewCode
    const current = ++revision
    parsing.value = false
    saving.value = true
    try {
      // 预览可临时包含隐藏层；真正应用前先以蓝图原始可见性构建，成功后才更新 SQLite。
      display(model)
      const write = SetConfig(CONFIG_GROUP, CONFIG_KEY, raw)
      pendingSave = write.catch(() => {})
      await write
      if (current !== revision) return
      applied.value = model
      hasSaved.value = Boolean(raw)
      editing.value = false
      discardPreview()
    } catch (err) {
      if (current !== revision) return
      discardPreview()
      error.value = '保存主页蓝图失败：' + formatErrorMessage(err)
    } finally {
      saving.value = false
    }
  }

  function close(): void {
    if (saving.value) return
    discardPreview()
    editing.value = false
  }

  function setAvailable(available: boolean): void {
    supported.value = available
    if (!available) {
      // 丢失上下文时 GPU 无法执行回退。废弃未确认预览，等设备恢复后重新绘制已应用作品。
      redraw = true
      if (!restoring.value && !saving.value) discardPreview()
      return
    }
    try {
      display(preview.value ?? applied.value)
    } catch (err) {
      error.value = '重新绘制主页蓝图失败：' + formatErrorMessage(err)
    }
  }

  onBeforeUnmount(() => { ++revision })
  return {appearance, savingAppearance, appearanceError, setPart, editing, restoring, parsing, saving, supported, hasSaved, error, applied, preview, showHidden,
    restore, inspect, discardPreview, toggleHidden, commit, close, setAvailable}
}
