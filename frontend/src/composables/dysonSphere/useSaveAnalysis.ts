import {computed, onBeforeUnmount, ref, shallowRef} from 'vue'
import * as api from '../../../wailsjs/go/environment/SaveAnalysisService'
import type {environment, saveanalysis} from '../../../wailsjs/go/models'
import {describeFactoryDataset} from '../../pages/services/dysonSphere/factory'

// 服务只保留一份授权快照。切页时的释放必须排在旧选档之后、新授权之前。
let mutations: Promise<unknown> = Promise.resolve()
function mutate<T>(operation: () => Promise<T>): Promise<T> {
  const result = mutations.then(operation)
  mutations = result.catch(() => undefined)
  return result
}

/** 管理只读授权、主动选档和单页筛选；过期回包不能覆盖新的筛选结果。 */
export function useSaveAnalysis() {
  const dialog = ref<'authorize' | 'saves' | null>(null)
  const directory = shallowRef<environment.RootResolution>()
  const saves = shallowRef<environment.EnvironmentSaveItem[]>([])
  const selected = shallowRef<environment.EnvironmentSaveItem>()
  const world = shallowRef<saveanalysis.WorldView>()
  const star = ref<number | null>(null), planet = ref<number | null>(null), item = ref<number | null>(null)
  const products = shallowRef<saveanalysis.ProductionItem[]>([])
  const report = shallowRef<saveanalysis.ProductionDiagnosis>()
  const sorterReport = shallowRef<saveanalysis.CircuitReport>()
  const detail = shallowRef<saveanalysis.PlanetItem>()
  const busy = ref(''), error = ref(''), pickerError = ref('')
  const names = shallowRef(new Map<number, string>())
  const itemName = (id: number) => names.value.get(id) || `物品 #${id}`
  let token = 0, alive = true, authorized = false, touched = false
  let summaryItems = new Set<number>()
  const stars = computed(() => (world.value?.stars || []).map(s => ({value: s.starId, label: `${s.name || '恒星'} · #${s.starId}`})))
  const planets = computed(() => (world.value?.planets || []).filter(p => p.starId === star.value).map(p => ({value: p.planetId, label: `${p.name || '星球'} · #${p.planetId}`})))
  const items = computed(() => products.value
    .filter(p => p.machines > 0)
    .map(p => ({value: p.itemId, label: names.value.get(p.itemId) || '未知产物'})))
  const planetNames = computed(() => new Map((world.value?.planets || []).map(p => [p.planetId, p.name])))
  const planetName = (id: number) => planetNames.value.get(id) || `${id} 号星球`
  const clearReport = () => { report.value = undefined; detail.value = undefined; sorterReport.value = undefined }
  function clearPlanet() { products.value = []; item.value = null; clearReport() }
  function clearSave() {
    selected.value = undefined; world.value = undefined; star.value = null; planet.value = null
    summaryItems.clear(); clearPlanet()
  }
  function message(e: unknown) { return e instanceof Error ? e.message : String(e) }
  function current(id: number) { return alive && token === id }
  async function openPicker() {
    if (busy.value) return
    const id = ++token
    pickerError.value = ''; busy.value = '读取目录信息'
    try {
      if (authorized) {
        try {
          const list = await api.ListSaves('')
          if (current(id)) { saves.value = list || []; dialog.value = 'saves' }
          return
        } catch (e) {
          if (!current(id)) return
          authorized = false; clearSave(); pickerError.value = message(e)
        }
      }
      const dir = await api.GetDirectory()
      if (current(id)) { directory.value = dir; dialog.value = 'authorize' }
    } catch (e) { if (current(id)) error.value = message(e) }
    finally { if (current(id)) busy.value = '' }
  }
  async function authorize() {
    if (!directory.value?.exists || busy.value) return
    const path = directory.value.path, id = ++token
    touched = true; busy.value = '列出存档'; pickerError.value = ''
    try {
      const list = await mutate(() => api.AuthorizeRead(path))
      if (current(id)) { authorized = true; saves.value = list || []; dialog.value = 'saves'; error.value = '' }
    } catch (e) { if (current(id)) pickerError.value = message(e) }
    finally { if (current(id)) busy.value = '' }
  }
  async function analyze(save: environment.EnvironmentSaveItem) {
    if (busy.value) return
    const id = ++token
    clearSave(); error.value = ''; busy.value = '正在解析 ' + save.name; dialog.value = null
    try {
      const result = await mutate(() => api.Analyze(save.id, 60))
      if (!current(id)) return
      const snapshot = await api.GetWorld(save.id)
      if (!current(id)) return
      summaryItems = new Set(result.summary.items.map(v => v.itemId))
      world.value = snapshot; selected.value = save
      const first = snapshot.planets?.find(p => p.hasFactory)
      star.value = first?.starId ?? snapshot.stars?.[0]?.starId ?? null
    } catch (e) { if (current(id)) error.value = message(e) }
    finally { if (current(id)) busy.value = '' }
  }
  function chooseStar(value: number) {
    ++token; busy.value = ''; error.value = ''; star.value = value; planet.value = null; clearPlanet()
  }
  async function choosePlanet(value: number) {
    const save = selected.value
    if (!save) return
    const id = ++token
    planet.value = value; clearPlanet(); error.value = ''; busy.value = '读取星球产物'
    try {
      const list = await api.GetProductionItems(save.id, value)
      if (current(id)) products.value = list || []
    } catch (e) { if (current(id)) error.value = message(e) }
    finally { if (current(id)) busy.value = '' }
  }
  async function chooseItem(value: number) {
    const save = selected.value, location = planet.value
    if (!save || location == null) return
    const id = ++token
    clearReport(); item.value = value; error.value = ''; busy.value = '正在追查产线'
    try {
      const [diagnosis, counts] = await Promise.all([
        api.DiagnoseProduction(save.id, location, value),
        summaryItems.has(value) ? api.GetItem(save.id, value, 60) : undefined,
      ])
      if (!current(id)) return
      report.value = diagnosis
      detail.value = counts?.planets?.find(p => p.planetId === location)
      // 追不到原料来源时，再查制造台分拣器的过滤设置。明细取不到也保留已有诊断。
      const needsSorters = diagnosis.roots.some(i => {
        const node = diagnosis.nodes[i]
        return node.machine && node.inputs?.some(input => input.itemId > 0 && !input.sources?.length)
      })
      if (needsSorters) {
        void api.GetProductionLine(save.id, location, value).then(line => {
          if (current(id) && line.sha256 === diagnosis.sha256 && line.tick === diagnosis.tick) sorterReport.value = line
        }).catch(() => {})
      }
    } catch (e) { if (current(id)) error.value = message(e) }
    finally { if (current(id)) busy.value = '' }
  }
  // 使用随盒子发布的物品原型名。未知或模组物品保留真实编号。
  void describeFactoryDataset('Vanilla').then(d => {
    if (alive) names.value = new Map(d.items.map(i => [i.id, i.name]))
  }).catch(() => {})
  onBeforeUnmount(() => {
    alive = false; ++token
    if (touched) void mutate(() => api.RevokeRead()).catch(e => console.error('释放存档快照失败', e))
  })
  return {dialog, directory, saves, selected, world, star, planet, item, stars, planets, items, report, detail, sorterReport, planetName,
    busy, error, pickerError, itemName, openPicker, authorize, analyze, chooseStar, choosePlanet, chooseItem}
}
