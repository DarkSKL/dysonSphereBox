import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, reactive, ref, shallowRef, watch} from 'vue'
import {industryworkspace, type environment, type factorycontract, type quant, type industryplan} from '../../../wailsjs/go/models'
import {describeFactoryDataset} from '../../pages/services/dysonSphere/factory'
import {gameDirectory, loadGameDirectory} from '../../pages/services/dysonSphere/game'
import {industryService as api, readIndustryBlueprint} from '../../services/industry-planner'

type Target = {key: number; item: number | null; rate: number | null; delivery: number | null}
type Rule = {item: number; choice: number; building: number}
type Choice = {value: number; label: string}
const issue = (e: unknown) => e instanceof Error ? e.message : String(e)
const directoryKey = (path: string) => path.replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase()

/** 编排跨星球规划页。昂贵的计算仅由生成按钮触发，路由缓存期间保留已选存档与结果。 */
export function useIndustryPlanner() {
  const detail = shallowRef<factorycontract.DatasetDetail | null>(null)
  const root = ref(''), suggested = ref(''), rootExists = ref(false)
  const temporaryRoot = ref(false)
  const saves = shallowRef<environment.EnvironmentSaveItem[]>([])
  const saveId = ref<string | null>(null)
  const selection = shallowRef<industryworkspace.Selection | null>(null)
  const task = shallowRef<industryworkspace.Task | null>(null)
  const report = shallowRef<industryworkspace.Report | null>(null)
  const error = ref(''), exported = ref(''), busy = ref(false), exporting = ref(false), active = ref(true)
  const targets = ref<Target[]>([])
  let nextTargetKey = 0
  const addTarget = () => {
    if (targets.value.length < 100) targets.value.push({key: ++nextTargetKey, item: null, rate: 60, delivery: null})
  }
  const rules = ref<Rule[]>([]), ruleItem = ref<number | null>(null)
  const previewId = ref<number | null>(null), previewCode = ref(''), previewError = ref('')
  const resourceSite = ref<number | null>(null)
  const inspecting = ref(false)
  const form = reactive({
    center: null as number | null, warehouse: null as number | null, preference: 'construction', profile: 'standard',
    mining: true, facility: 2301, miningSpeed: 1, minerSpeed: 1, logistics: true, storage: true, minutes: 60,
    power: true, spray: false, sprayMode: 2, sprayPoints: 4, fuel: true, collectors: false, buffer: false,
    developFully: false, pauseBetweenWindows: false, technologyMode: 'save', warpEnabled: true,
    foundation: 'none', belt: 1, sorter: 1, cargo: 200, cruise: 300, warp: 1000000, warpDistance: 120000, turnaround: 10, ships: 10,
    externalItems: [] as number[],
  })
  const items = computed<Choice[]>(() => (detail.value?.items ?? []).map(x => ({value: x.id, label: x.name})))
  const energy = reactive({default: {mode:'auto', facilityId:2211, fuelItemId:0, gridCapacityMw:0, chargePlanetId:0, solarFactor:1, solarDuty:.5, nightSeconds:300}, allowedFacilityIds:[] as number[], overrides:[] as industryplan.EnergyOverride[]})
  const dyson = reactive({mode:'unconfigured', supplies:[] as industryplan.DysonSupply[], lenses:true})
  const overrideEnergy = (id:number) => {
    if (!energy.overrides.some(row=>row.planetId===id)) energy.overrides.push({planetId:id,choice:{...energy.default}} as industryplan.EnergyOverride)
  }
  const names = computed(() => new Map(items.value.map(x => [x.value, x.label])))
  const itemName = (id: number) => names.value.get(id) ?? String(id)
  const siteById = computed(() => new Map((selection.value?.sites ?? []).map(s => [s.id, s])))
  const siteName = (id: number) => { const site = siteById.value.get(id); return site ? site.star + ' / ' + site.name : String(id) }
  const sites = computed(() => (selection.value?.sites ?? []).filter(s => !s.gas).map(s => ({label: siteName(s.id), value: s.id})))
  const allSites = computed(() => (selection.value?.sites ?? []).map(s => ({label: siteName(s.id), value: s.id})))
  const currentResources = computed(() => resourceSite.value ? siteById.value.get(resourceSite.value) : undefined)
  async function inspectResources() {
    const selected = selection.value, id = resourceSite.value
    if (!selected || !id || inspecting.value) return
    inspecting.value = true
    try {
      const result = await api.inspect(selected.saveId, selected.snapshotId, id)
      if (selection.value !== selected) return
      selection.value = new industryworkspace.Selection({...selected, sites: selected.sites.map(s => s.id === id ? {...s, source: result.source, resources: result.resources} : s)})
    } catch (e) { if (selection.value === selected) error.value = issue(e) }
    finally { inspecting.value = false }
  }
  const candidates = (id: number) => (detail.value?.recipes ?? []).filter(x => x.results.includes(id))
  const directResourceItems = computed(() => {
    const result = new Set<number>()
    for (const site of selection.value?.sites ?? []) {
      if (site.gas) continue
      for (const resource of site.resources) {
        if (resource.kind === 'liquid' ||
          resource.kind === 'solid' && (site.source === 'candidate' || resource.source === 'candidate' ||
            (resource.availableCount ?? (resource.count ?? 0) - (resource.occupiedCount ?? 0)) > 0)) {
          result.add(resource.itemId)
        }
      }
    }
    return result
  })
  const recipeChoices = (r: Rule) => {
    const choices = candidates(r.item).map((x, i) => ({label: x.name, value: i + 1}))
    if (directResourceItems.value.has(r.item) || r.choice === 0) choices.push({label: '直接采集星球资源', value: 0})
    return choices
  }
  const directRules = computed(() => rules.value.filter(r => r.choice === 0))
  const directResourceError = computed(() => {
    if (!directRules.value.length) return ''
    if (!form.mining) return '直接采集星球资源需要启用新建采矿设施'
    for (const {item} of directRules.value) {
      if (form.externalItems.includes(item)) return itemName(item) + '不能同时直接采集和由玩家外供'
      if (!directResourceItems.value.has(item)) return '所选存档缺少可查的' + itemName(item) + '采集来源'
    }
    return ''
  })
  const selectedRecipe = (r: Rule): quant.Recipe | undefined => candidates(r.item)[r.choice - 1]
  const buildings = (r: Rule) => (selectedRecipe(r)?.factories ?? []).map((id, i) => ({label: itemName(id), value: i}))
  const addRule = () => {
    if (ruleItem.value && !rules.value.some(r => r.item === ruleItem.value) && candidates(ruleItem.value).length) {
      rules.value.push({item: ruleItem.value, choice: 1, building: 0})
    }
    ruleItem.value = null
  }
  const ready = computed(() => Boolean(selection.value && detail.value && !directResourceError.value && targets.value.length && targets.value.every(t => t.item && t.rate && t.rate > 0)))
  const running = computed(() => ['running', 'cancelling'].includes(task.value?.state ?? ''))
  const paused = computed(() => task.value?.state === 'paused')
  let version = 0, pageSerial = 0, directorySerial = 0, initialized = false
  let directoryRefresh: Promise<void> | undefined
  let timer: ReturnType<typeof setTimeout> | undefined, preview: AbortController | null = null
  const stopTimer = () => { clearTimeout(timer); timer = undefined }
  const closePreview = () => { preview?.abort(); preview = null; previewId.value = null; previewCode.value = ''; previewError.value = '' }
  const discard = () => {
    ++version
    stopTimer()
    closePreview()
    const previous = task.value
    if (previous && ['running', 'paused'].includes(previous.state)) void api.cancel(previous.id).catch(() => {})
    report.value = null; task.value = null; exported.value = ''; error.value = ''
  }
  watch([form, targets, rules, energy, dyson], discard, {deep: true, flush: 'sync'})

  // =============== 存档会话 ===============
  // 说明：首次打开目录时选择修改时间最新的存档；相同目录刷新保留手动选择和快照。
  async function selectLatestSave(items: environment.EnvironmentSaveItem[]) {
    saves.value = items
    let latest: environment.EnvironmentSaveItem | undefined
    let latestTime = -Infinity
    for (const item of items) {
      const time = Date.parse(item.lastModifiedAt)
      if (!latest || time > latestTime) {
        latest = item
        latestTime = Number.isNaN(time) ? -Infinity : time
      }
    }
    if (latest) await chooseSave(latest.id)
  }
  const clearSelection = () => {
    ++pageSerial; discard(); saveId.value = null; selection.value = null; resourceSite.value = null
    form.center = null; form.warehouse = null
    energy.overrides = []; energy.default.chargePlanetId = 0
    dyson.supplies = []
    for (const t of targets.value) t.delivery = null
  }
  function refreshDefaultDirectory(reloadGame = true): Promise<void> {
    if (directoryRefresh) return directoryRefresh
    const serial = directorySerial
    directoryRefresh = (async () => {
      let opening = false
      try {
        if (reloadGame) await loadGameDirectory()
        const resolved = await api.directory()
        if (serial !== directorySerial) return
        suggested.value = resolved.path; rootExists.value = resolved.exists
        if (temporaryRoot.value) return
        if (root.value && resolved.exists && directoryKey(root.value) === directoryKey(resolved.path)) {
          const result = await api.saves()
          if (serial === directorySerial) {
            if (saveId.value) saves.value = result
            else await selectLatestSave(result)
          }
          return
        }
        if (root.value || selection.value || task.value) clearSelection()
        root.value = ''; saves.value = []
        if (!resolved.exists) { await api.revoke(); return }
        opening = true; busy.value = true
        const result = await api.authorize(resolved.path)
        if (serial !== directorySerial) return
        root.value = resolved.path
        await selectLatestSave(result)
      } catch (e) { if (serial === directorySerial) error.value = issue(e) }
      finally { if (opening && serial === directorySerial) busy.value = false }
    })().finally(() => { directoryRefresh = undefined })
    return directoryRefresh
  }
  async function initialize() {
    busy.value = true
    const outcomes = await Promise.allSettled([refreshDefaultDirectory(), describeFactoryDataset('Vanilla')] as const)
    if (outcomes[1].status === 'fulfilled') detail.value = outcomes[1].value
    for (const event of outcomes) if (event.status === 'rejected') error.value = issue(event.reason)
    initialized = true; busy.value = false
  }
  async function openDirectory(picker: boolean) {
    if (!picker) { await restoreDirectory(); return }
    const serial = ++directorySerial
    busy.value = true; error.value = ''
    try {
      const path = await api.choose()
      if (!path || serial !== directorySerial) return
      clearSelection(); temporaryRoot.value = true; root.value = path; saves.value = []
      const result = await api.saves()
      if (serial === directorySerial) await selectLatestSave(result)
    } catch (e) {
      if (serial === directorySerial) { clearSelection(); root.value = ''; saves.value = []; error.value = issue(e) }
    } finally { if (serial === directorySerial) busy.value = false }
  }
  async function restoreDirectory() {
    if (directoryRefresh) await directoryRefresh
    if (temporaryRoot.value) {
      ++directorySerial; temporaryRoot.value = false; clearSelection(); root.value = ''; saves.value = []
    }
    await refreshDefaultDirectory()
  }
  async function chooseSave(id: string) {
    clearSelection()
    const serial = pageSerial
    error.value = ''; busy.value = true; saveId.value = id
    try {
      const selected = await api.select(id)
      if (serial === pageSerial) {
        selection.value = selected
        setTechnology(selected.technology ? 'save' : 'custom')
      }
    } catch (e) { if (serial === pageSerial) error.value = issue(e) }
    finally { if (serial === pageSerial) busy.value = false }
  }
  function setTechnology(mode: string) {
    form.technologyMode = mode
    const tech = selection.value?.technology
    if (mode === 'save' && tech) {
      Object.assign(form, {miningSpeed:tech.miningSpeed, cargo:tech.shipCargo, cruise:tech.cruiseSpeed, warp:tech.warpSpeed, warpEnabled:tech.warpUnlocked})
    }
  }
  async function revoke() {
    const serial = ++directorySerial
    clearSelection(); temporaryRoot.value = false; root.value = ''; saves.value = []; busy.value = false
    try { await api.revoke() } catch (e) { if (serial === directorySerial) error.value = issue(e) }
  }
  watch(gameDirectory, () => { if (initialized && !directoryRefresh) void refreshDefaultDirectory(false) })

  // =============== 量化输入与任务进度 ===============
  // 说明：物品选择使用量化模块的一基配方序号，设备使用零基序号。关闭喷涂时所有配方均无加成。
  function request(): industryworkspace.Request {
    const overrides = new Map(rules.value.map(r => [selectedRecipe(r)?.id, r.building]))
    const schemeForRecipe = (detail.value?.recipes ?? []).map(r => {
      const mode = form.spray && (r.proliferator & form.sprayMode) ? form.sprayMode : 0
      return {building: overrides.get(r.id) ?? 0, productionMode: mode, increasePoints: mode ? form.sprayPoints : 0}
    })
    return new industryworkspace.Request({
      saveId: saveId.value, snapshotId: selection.value?.snapshotId,
      planning: {centerPlanetId: form.center ?? 0, storagePlanetId: form.warehouse ?? 0, preference: form.preference, profile: form.profile,
        energy, dyson, developFully:form.developFully,
        targetPlanetIds: targets.value.map(t => t.delivery ?? 0),
        enableMining: form.mining, enableOrbitalCollectors: form.collectors, enableLogistics: form.logistics, enableStorage: form.storage, storageMinutes: form.minutes,
        mining: {facilityId: form.facility, speedMultiplier: form.miningSpeed, facilityMultiplier: form.facility === 2316 ? form.minerSpeed : 1},
        logistics: {cargoPerVessel: form.cargo, cruiseSpeed: form.cruise, warpSpeed: form.warp, warpEnabled: form.warpEnabled, warpDistance: form.warpDistance, turnaroundSeconds: form.turnaround, vesselsPerStation: form.ships},
        quant: {dataset: 'Vanilla', targets: targets.value.map(t => ({itemId: t.item, ratePerMinute: t.rate})),
          scheme: {itemRecipeChoices: Object.fromEntries(rules.value.filter(r => r.choice > 0).map(r => [itemName(r.item), r.choice])), schemeForRecipe},
          settings: {miningSpeedMultiple: form.miningSpeed, mineralizeList: Object.fromEntries(form.externalItems.map(id => [itemName(id), true])),
            directResourceItemIds: directRules.value.map(r => r.item)}}},
      options: {technologyMode: form.technologyMode, continuous: !form.pauseBetweenWindows, power: form.power, spray: form.spray, fuel: form.fuel, buffer: form.buffer, foundation: form.foundation, beltLevel: form.belt, inserterLevel: form.sorter},
    })
  }
  async function poll(token: number, id: number) {
    if (!active.value || token !== version) return
    try {
      const update = await api.task(id)
      if (token !== version) return
      task.value = update
      if (update.state === 'ready') {
        const result = await api.report(id)
        if (token === version) report.value = result
      } else if (update.state === 'failed') error.value = update.error || '规划失败'
      else if (running.value && active.value) timer = setTimeout(() => { void poll(token, id) }, 450)
    } catch (e) { if (token === version) error.value = issue(e) }
  }
  async function start() {
    if (!ready.value) return
    discard(); error.value = ''
    const token = version
    busy.value = true
    try {
      const next = await api.start(request())
      if (token !== version) { await api.cancel(next.id); return }
      task.value = next
      await poll(token, next.id)
    } catch (e) { if (token === version) error.value = issue(e) }
    finally { busy.value = false }
  }
  async function cancel() {
    const previous = task.value, token = version
    if (!previous || !running.value && !paused.value) return
    stopTimer()
    task.value = new industryworkspace.Task({...previous, state: 'cancelling'})
    try { await api.cancel(previous.id) } catch (e) { if (token === version) error.value = issue(e) }
    if (token === version) await poll(token, previous.id)
  }
  async function resume() {
    const previous = task.value, token = version
    if (!previous || !paused.value || busy.value) return
    busy.value = true; error.value = ''; stopTimer()
    try {
      const next = await api.resume(previous.id)
      if (token !== version) { await api.cancel(next.id); return }
      task.value = next
      await poll(token, next.id)
    } catch (e) { if (token === version) error.value = issue(e) }
    finally { busy.value = false }
  }
  async function showPreview(id: number) {
    if (!task.value) return
    closePreview(); const controller = new AbortController(); preview = controller; previewId.value = id
    try {
      const code = await readIndustryBlueprint(task.value.id, id, controller.signal)
      if (!controller.signal.aborted) previewCode.value = code
    } catch (e) { if (!controller.signal.aborted) previewError.value = issue(e) }
  }
  const previewName = computed(() => report.value?.planets?.flatMap(site => site.sheets ?? []).find(sheet => sheet.id === previewId.value)?.name ?? '')
  async function exportPack(destination: 'file' | 'library' = 'file') {
    if (!task.value || !report.value?.exportable) return
    const token = version
    exporting.value = true
    try { const path = await (destination === 'library' ? api.library : api.export)(task.value.id); if (token === version && path) exported.value = path }
    catch (e) { if (token === version) error.value = issue(e) }
    finally { exporting.value = false }
  }
  onMounted(() => { void initialize() })
  onActivated(async () => {
    active.value = true; stopTimer()
    if (initialized) await refreshDefaultDirectory()
    if (active.value && task.value && running.value) void poll(version, task.value.id)
  })
  onDeactivated(() => { active.value = false; stopTimer(); closePreview() })
  onBeforeUnmount(() => { active.value = false; ++directorySerial; ++pageSerial; discard(); void api.revoke().catch(() => {}) })
  return {form, energy, dyson, overrideEnergy, detail, root, suggested, rootExists, temporaryRoot, saves, saveId, selection, task, report, targets, addTarget, items, itemName, siteName, sites, allSites, resourceSite, currentResources,
    rules, ruleItem, recipeChoices, buildings, addRule, directResourceError, ready, running, paused, error, exported, busy, exporting, previewId, previewCode, previewError,
    inspecting, inspectResources, openDirectory, restoreDirectory, chooseSave, setTechnology, revoke, start, resume, cancel, showPreview, previewName, closePreview, exportPack}
}
