import {computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, reactive, ref, watch, type UnwrapNestedRefs} from 'vue'
import {useDialog, useMessage} from 'naive-ui'
import {useRoute, useRouter} from 'vue-router'
import {build, factorycontract, quant} from '../../../wailsjs/go/models'
import {ClipboardSetText} from '../../../wailsjs/runtime/runtime'
import {FactoryError, describeFactoryDataset, exportFactoryBlueprint, exportFactoryWorkspace, generateFactory, inspectFactoryBlueprint, listFactoryDatasets, solveFactory} from '../../services/dysonSphere/factory'
import {getEnvironmentBlueprintDetail, quickImportEnvironmentBlueprint} from '../../services/dysonSphere/environment'
import {requestBlueprintLibrary} from '../../services/dysonSphere/blueprint-navigation'

type RecipeOverride = {building: number; increasePoints: number; productionMode: number}
type WorkspaceTarget = {key: number; itemId: number | null; ratePerMinute: number | null}
export type PreviewState = 'idle' | 'loading' | 'ready' | 'stale' | 'error' | 'empty'

// 职责：工作区输入、异步快照与只读回放；使用：生成器页面；边界：所有配方/蓝图计算只走本机 Go，前端仅组装请求和展示。
// 实时链路：参数变化 => 防抖自动量化 => 量化稳定后防抖自动生成并刷新预览；生成串行执行，进行中再变化只保留最新一次。
export const useQuantWorkspace = () => {
  const message = useMessage()
  const dialog = useDialog()
  const route = useRoute()
  const router = useRouter()
  const active = ref(true)
  const datasets = ref<quant.DatasetInfo[]>([])
  const datasetKey = ref('Vanilla')
  const detail = ref<factorycontract.DatasetDetail | null>(null)
  const targets = ref<WorkspaceTarget[]>([{key: 1, itemId: 6001, ratePerMinute: 60}])
  let targetCounter = 1
  const itemRecipeChoices = reactive<Record<string, number>>({})
  const recipeOverrides = reactive<Record<number, RecipeOverride>>({})
  const mineralized = ref<string[]>([])
  const miningSpeed = ref<number | null>(1)
  const fixedNum = ref<number | null>(2)
  const researchStack = ref<number | null>(15)
  const sprayItself = ref(true)
  const spaceCost = ref<number | null>(1)
  const powerCost = ref<number | null>(0)
  const mode = ref(3)
  const name = ref('量化产线')
  const layout = reactive({beltLv: 1, inserterLv: 1, maxLays: 3, maxMachineInALine: 15, blockWidthLimit: 50, mergeType: 2, needPower: true, useMixed: false, mix2: true, needCoaterNet: false})
  const plan = ref<quant.Result | null>(null)
  const generated = ref<build.Result | null>(null)
  const inspection = ref<factorycontract.Inspection | null>(null)
  const code = ref('')
  const replay = ref(false)
  const loadingDataset = ref(false)
  const solving = ref(false)
  const generating = ref(false)
  const importing = ref(false)
  const saving = ref(false)
  const error = ref<{code: string; message: string; source?: 'generation'} | null>(null)
  const previewState = ref<PreviewState>('idle')
  const previewRevision = ref(0)
  const previewNotice = ref('')
  const showPreview = ref(true)
  const narrow = ref(false)
  const selectedBuilding = ref<number | null>(null)
  const selectedProductions = ref<number[]>([])
  const importVisible = ref(false)
  const importText = ref('')
  const sourceName = ref('导入的蓝图')
  const autoRun = ref(true)
  const solvedSignature = ref('')
  const generatedSignature = ref('')
  const itemMap = computed(() => new Map((detail.value?.items ?? []).map(item => [item.id, item])))
  const itemOptions = computed(() => (detail.value?.items ?? []).filter(item => item.id > 0).map(item => ({label: item.name, value: item.id})))
  const requestSettings = computed(() => ({miningSpeedMultiple: miningSpeed.value ?? 1, fixedNum: fixedNum.value ?? 2, stackResearchLab: researchStack.value ?? 15, proliferateItself: sprayItself.value, mineralizeList: Object.fromEntries(mineralized.value.map(value => [value, true]))}))
  const requestScheme = computed(() => ({itemRecipeChoices: {...itemRecipeChoices}, schemeForRecipe: (detail.value?.recipes ?? []).map((_recipe, index) => recipeOverrides[index] ?? {building: 0, productionMode: 0, increasePoints: 0}), costWeight: {space: spaceCost.value ?? 1, power: powerCost.value ?? 0}}))
  const querySignature = computed(() => JSON.stringify([datasetKey.value, targets.value.map(({itemId, ratePerMinute}) => ({itemId, ratePerMinute})), requestSettings.value, requestScheme.value]))
  const generationSignature = computed(() => JSON.stringify([querySignature.value, mode.value, {...layout}, name.value]))
  const quantStale = computed(() => Boolean(plan.value && solvedSignature.value !== querySignature.value))
  const previewStale = computed(() => Boolean(code.value && !replay.value && generatedSignature.value !== generationSignature.value))
  const expectedBuildings = computed(() => replay.value ? inspection.value?.summary.buildingCount ?? 0 : generated.value?.buildingCount ?? 0)
  const warnings = computed(() => replay.value ? inspection.value?.warnings ?? [] : [...(plan.value?.warnings ?? []), ...(generated.value?.warnings ?? [])])
  let datasetVersion = 0
  let initialized = false
  let importVersion = 0
  let solveVersion = 0
  let disposed = false
  let media: MediaQueryList | null = null
  let solveTimer: number | undefined
  let generateTimer: number | undefined
  let generateQueued = false
  const updateWidth = (): void => { narrow.value = media?.matches ?? false }
  // 生成操作在长结果列表下方；失败时就地保留详情，并弹窗通知，缓存页失活后不再打断其它页面。
  const report = (caught: unknown, source?: 'generation'): void => {
    const failure = caught instanceof FactoryError ? {code: caught.code, message: caught.message} : {code: 'REQUEST_FAILED', message: caught instanceof Error ? caught.message : String(caught)}
    error.value = {...failure, source}
    if (source === 'generation' && active.value && !disposed) {
      dialog.error({title: '蓝图生成失败', content: failure.message, positiveText: '知道了'})
    }
  }
  const number = (value: number | undefined): string => (value ?? 0).toLocaleString('zh-CN', {maximumFractionDigits: fixedNum.value ?? 2})
  const itemName = (id: number): string => itemMap.value.get(id)?.name ?? `物品 ${id}`
  // 选项 icon 为图鉴中文名：配方取首个产物，设备取设施本身，喷涂取对应等级增产剂；空串表示不显示图标。
  const recipeOptions = (id: number): {label: string; value: number; icon: string}[] => (detail.value?.recipes ?? []).filter(recipe => recipe.results.includes(id)).map((recipe, index) => ({label: recipe.name, value: index + 1, icon: itemName(recipe.results[0] ?? id)}))
  const selectedRecipe = (id: number): quant.Recipe | null => {
    const choices = (detail.value?.recipes ?? []).filter(recipe => recipe.results.includes(id))
    return choices[(itemRecipeChoices[itemName(id)] ?? 1) - 1] ?? null
  }
  const overrideFor = (recipe: quant.Recipe | null): RecipeOverride => {
    const index = detail.value?.recipes.findIndex(value => value.id === recipe?.id) ?? -1
    return recipeOverrides[index] ?? {building: 0, productionMode: 0, increasePoints: 0}
  }
  const updateRecipe = (id: number, choice: number): void => { itemRecipeChoices[itemName(id)] = choice }
  const updateOverride = (recipe: quant.Recipe | null, patch: Partial<RecipeOverride>): void => {
    const index = detail.value?.recipes.findIndex(value => value.id === recipe?.id) ?? -1
    if (index >= 0) recipeOverrides[index] = {...overrideFor(recipe), ...patch}
  }
  const buildingOptions = (recipe: quant.Recipe | null): {label: string; value: number; icon: string}[] => (recipe?.factories ?? []).map((id, index) => ({label: itemName(id), value: index, icon: itemName(id)}))
  const sprayModes = (recipe: quant.Recipe | null): {label: string; value: number; icon: string}[] => [{label: '不喷涂', value: 0, icon: ''}, ...[{label: '加速', value: 1, icon: '增产剂 Mk.III'}, {label: '额外产出', value: 2, icon: '增产剂 Mk.III'}, {label: '透镜', value: 3, icon: '引力透镜'}, {label: '分馏', value: 4, icon: '分馏塔'}].filter(value => Boolean((recipe?.proliferator ?? 0) & (1 << (value.value - 1))))]
  const pointOptions = computed(() => [{label: '默认', value: 0, icon: ''}, {label: '1 点', value: 1, icon: '增产剂 Mk.I'}, {label: '2 点', value: 2, icon: '增产剂 Mk.II'}, {label: '4 点', value: 4, icon: '增产剂 Mk.III'}, ...(datasetKey.value.includes('FractionateEverything') ? [{label: '10 点', value: 10, icon: '增产剂 Mk.III'}] : [])])

  const loadDataset = async (): Promise<void> => {
    const version = ++datasetVersion
    importVersion++
    solveVersion++
    importing.value = false
    solving.value = false
    loadingDataset.value = true
    try {
      const value = await describeFactoryDataset(datasetKey.value)
      if (version !== datasetVersion || disposed) return
      detail.value = value
      Object.keys(itemRecipeChoices).forEach(key => delete itemRecipeChoices[key])
      Object.keys(recipeOverrides).forEach(key => delete recipeOverrides[Number(key)])
      value.recipes.forEach((_recipe, index) => { recipeOverrides[index] = {building: 0, productionMode: 0, increasePoints: 0} })
      mineralized.value = []
      plan.value = null
    } catch (caught) { if (version === datasetVersion) report(caught) }
    finally { if (version === datasetVersion) loadingDataset.value = false }
  }
  const targetsReady = computed(() => targets.value.length > 0 && targets.value.every(target => target.itemId && target.ratePerMinute && target.ratePerMinute > 0))
  const calculate = async (): Promise<void> => {
    error.value = null
    if (!targetsReady.value) { error.value = {code: 'Q_E_BAD_REQUEST', message: '每个目标都需要选择物品并输入正产量'}; return }
    const signature = querySignature.value
    const version = ++solveVersion
    importVersion++
    const request = new quant.Request({dataset: datasetKey.value, targets: targets.value.map(target => ({itemId: target.itemId, ratePerMinute: target.ratePerMinute})), scheme: requestScheme.value, settings: requestSettings.value})
    solving.value = true
    try {
      const value = await solveFactory(request)
      if (disposed || version !== solveVersion || signature !== querySignature.value) return
      plan.value = value
      solvedSignature.value = signature
      replay.value = false
      inspection.value = null
      selectedBuilding.value = null
      selectedProductions.value = []
    } catch (caught) { if (version === solveVersion) report(caught) } finally { if (version === solveVersion) solving.value = false }
  }
  const exportWorkspace = async (): Promise<void> => {
    if (!plan.value || quantStale.value || solving.value || generating.value) return
    saving.value = true
    try {
      const request = new quant.Request({dataset: datasetKey.value, targets: targets.value.map(target => ({itemId: target.itemId, ratePerMinute: target.ratePerMinute})), scheme: requestScheme.value, settings: requestSettings.value})
      const path = await exportFactoryWorkspace(new factorycontract.WorkspaceRequest({name: name.value, request, plan: plan.value, setting: {...layout}, mode: mode.value}))
      if (path) message.success('计算方案与结果已导出为 JSON')
    } catch (caught) { report(caught) } finally { saving.value = false }
  }
  const generate = async (): Promise<void> => {
    if (generating.value) { generateQueued = true; return }
    if (!plan.value || quantStale.value) return
    error.value = null
    const signature = generationSignature.value
    const source = plan.value.planId
    generating.value = true
    try {
      const result = await generateFactory(new build.Request({plan: plan.value, mode: mode.value, setting: {...layout}, name: name.value}))
      if (disposed) return
      if (signature !== generationSignature.value || result.sourcePlanId !== source || plan.value?.planId !== source) { if (active.value && !autoRun.value) message.warning('参数已变化，本次结果未替换预览，请重新量化或生成'); return }
      generated.value = result
      generatedSignature.value = signature
      code.value = result.blueprint
      previewRevision.value++
      replay.value = false
      selectedBuilding.value = null
      selectedProductions.value = []
      previewNotice.value = ''
      previewState.value = result.buildingCount ? 'loading' : 'empty'
      showPreview.value = result.buildingCount <= 6000
      if (!showPreview.value) previewNotice.value = '建筑超过 6000，已折叠预览；可按需打开，不影响复制与导出'
    } catch (caught) { report(caught, 'generation') } finally {
      generating.value = false
      if (generateQueued) { generateQueued = false; scheduleGenerate(0) }
    }
  }
  const loadCode = async (text: string, title: string, version = ++importVersion): Promise<void> => {
    const dataset = datasetKey.value
    solveVersion++
    solving.value = false
    error.value = null
    importing.value = true
    try {
      const value = await inspectFactoryBlueprint(text.trim().replace(/^\uFEFF/, ''), dataset)
      if (disposed || version !== importVersion || dataset !== datasetKey.value) return
      inspection.value = value
      plan.value = null
      generated.value = null
      code.value = text.trim().replace(/^\uFEFF/, '')
      previewRevision.value++
      replay.value = true
      sourceName.value = title
      name.value = title
      previewState.value = value.summary.buildingCount ? 'loading' : 'empty'
      selectedBuilding.value = null
      selectedProductions.value = []
      importVisible.value = false
    } catch (caught) { if (version === importVersion) report(caught) } finally { if (version === importVersion) importing.value = false }
  }
  const loadLibraryItem = async (itemId: string): Promise<void> => {
    const version = ++importVersion
    importing.value = true
    try { const value = await getEnvironmentBlueprintDetail(itemId); if (!disposed && version === importVersion) await loadCode(value.code, value.item.name, version) }
    catch (caught) { if (version === importVersion) { report(caught); importing.value = false } }
  }
  const importFile = async (file: File): Promise<void> => {
    if (file.size > 8 * 1024 * 1024) { error.value = {code: 'BP_E_TOO_LARGE', message: '蓝图文件超过 8 MiB，请拆分后导入'}; return }
    await loadCode(await file.text(), file.name.replace(/\.[^.]+$/, ''))
  }
  const recalculateImport = (): void => {
    const positives = (inspection.value?.netRates ?? []).filter(value => value.ratePerMinute > 1e-7)
    if (!positives.length) { error.value = {code: 'Q_E_BAD_REQUEST', message: '未识别出正的净产物，请手动添加目标'}; return }
    targets.value = positives.slice(0, 100).map(value => ({key: ++targetCounter, itemId: value.itemId, ratePerMinute: value.ratePerMinute}))
    replay.value = false
    plan.value = null
    generatedSignature.value = ''
  }
  const selectProduction = (index: number): void => {
    if (previewStale.value || quantStale.value) return
    selectedProductions.value = [index]
    selectedBuilding.value = generated.value?.bindings?.find(binding => binding.productionIndex === index)?.machineIndices?.[0] ?? null
  }
  const onBuildingSelect = ({index}: {index: number | null}): void => {
    if (previewStale.value || quantStale.value) return
    selectedBuilding.value = index
    selectedProductions.value = index === null ? [] : (generated.value?.bindings ?? []).filter(binding => binding.buildingIndices.includes(index)).map(binding => binding.productionIndex)
  }
  const onPreviewReady = ({buildingCount}: {buildingCount: number}): void => {
    previewState.value = previewStale.value ? 'stale' : buildingCount ? 'ready' : 'empty'
    if (buildingCount !== expectedBuildings.value) previewNotice.value = `预览识别 ${buildingCount} 个建筑，Go 结果 ${expectedBuildings.value} 个；导出仍以 Go 自检为准`
  }
  const onPreviewError = ({code: failureCode, message: text}: {code: string; message: string}): void => {
    previewNotice.value = text
    if (failureCode !== 'PREVIEW_E_LIMIT') previewState.value = 'error'
  }
  const allowExport = async (): Promise<boolean> => {
    if (!code.value) return false
    if (!previewStale.value) return true
    return new Promise(resolve => {
      dialog.warning({title: '导出上次结果', content: '参数已经改变，当前蓝图属于上一次生成结果。确认仍导出上次结果？', positiveText: '导出上次结果', negativeText: '取消', onPositiveClick: () => resolve(true), onNegativeClick: () => resolve(false), onClose: () => resolve(false), onMaskClick: () => resolve(false)})
    })
  }
  const copy = async (): Promise<void> => {
    if (!await allowExport()) return
    try { if (!await ClipboardSetText(code.value)) throw new Error('写入剪贴板失败'); message.success('蓝图编码已复制') } catch (caught) { report(caught) }
  }
  const download = async (): Promise<void> => {
    if (!await allowExport()) return
    try { const path = await exportFactoryBlueprint(name.value || sourceName.value, code.value); if (path) message.success('蓝图文本已导出') }
    catch (caught) { report(caught) }
  }
  const save = async (): Promise<void> => {
    if (!await allowExport()) return
    saving.value = true
    try { const result = await quickImportEnvironmentBlueprint('', name.value || sourceName.value, code.value); message.success(result.message) }
    catch (caught) { report(caught) } finally { saving.value = false }
  }
  // =============== 多目标分区 ===============
  // 说明：生成结果按目标划分分区；颜色按分区顺序固定，节点图、预览栏共用同一套，未生成时按量化归属推断。
  const zonePalette = ['#4cc2ff', '#3ee0b0', '#ffc15a', '#c58bff', '#ff8a65', '#7bd88f', '#f06292', '#90a4ae']
  const activeZone = ref<string | null>(null)
  const zones = computed(() => {
    const source = generated.value?.zones?.length && !previewStale.value ? generated.value.zones : null
    const list = source ?? (plan.value ? planZoneFallback(plan.value) : [])
    return list.map((zone, index) => ({key: zone.key, targetItemId: zone.targetItemId, productionIndices: zone.productionIndices ?? [], label: zone.targetItemId ? itemName(zone.targetItemId) : '共用中间产物', color: zone.key === 'shared' ? '#8a8fa3' : zonePalette[index % zonePalette.length]}))
  })
  const zoneOfProduction = computed(() => {
    const map = new Map<number, {key: string; color: string; label: string}>()
    for (const zone of zones.value) for (const index of zone.productionIndices) map.set(index, zone)
    return map
  })
  const focusZone = (key: string | null): void => {
    activeZone.value = activeZone.value === key ? null : key
    const zone = zones.value.find(value => value.key === activeZone.value)
    if (!zone || previewStale.value || quantStale.value) return
    const binding = generated.value?.bindings?.find(value => zone.productionIndices.includes(value.productionIndex))
    selectedProductions.value = [...zone.productionIndices]
    selectedBuilding.value = binding?.machineIndices?.[0] ?? null
  }
  const targetFor = (id: number): WorkspaceTarget | undefined => targets.value.find(target => target.itemId === id)
  const setTargetRate = (id: number, rate: number): void => { const target = targetFor(id); if (target) target.ratePerMinute = rate }
  const removeTarget = (id: number): void => { if (targets.value.length > 1) targets.value = targets.value.filter(target => target.itemId !== id) }
  const addTargetItem = (id: number, rate = 60): void => {
    const existing = targetFor(id)
    if (existing) { existing.ratePerMinute = (existing.ratePerMinute ?? 0) + rate; return }
    if (targets.value.length < 100) targets.value.push({key: ++targetCounter, itemId: id, ratePerMinute: rate})
  }
  const isExternal = (id: number): boolean => mineralized.value.includes(itemName(id))
  const toggleExternal = (id: number, external: boolean): void => {
    const label = itemName(id)
    mineralized.value = external ? [...new Set([...mineralized.value, label])] : mineralized.value.filter(value => value !== label)
  }
  const openLibrary = (): void => { requestBlueprintLibrary(); void router.push('/dysonSphere/environment') }
  const addTarget = (): void => { if (targets.value.length < 100) targets.value.push({key: ++targetCounter, itemId: null, ratePerMinute: 60}) }
  const setExternal = (id: number | null): void => { if (id && !mineralized.value.includes(itemName(id))) mineralized.value.push(itemName(id)) }
  // =============== 实时量化与生成 ===============
  // 说明：量化 400ms 防抖；生成在量化结果稳定 700ms 后触发。导入回放、数据集加载或关闭自动时不调度。
  const scheduleSolve = (): void => {
    window.clearTimeout(solveTimer)
    if (!autoRun.value || !initialized || replay.value || !detail.value || !targetsReady.value) return
    solveTimer = window.setTimeout(() => { if (querySignature.value !== solvedSignature.value || !plan.value) void calculate() }, 400)
  }
  const scheduleGenerate = (delay = 700): void => {
    window.clearTimeout(generateTimer)
    if (!autoRun.value || replay.value || !plan.value || quantStale.value || !plan.value.productions.length) return
    if (code.value && !previewStale.value) return
    generateTimer = window.setTimeout(() => { if (!quantStale.value && (!code.value || previewStale.value)) void generate() }, delay)
  }
  watch(querySignature, scheduleSolve)
  watch(() => [plan.value?.planId, generationSignature.value], () => scheduleGenerate())
  watch(autoRun, value => { if (value) { scheduleSolve(); scheduleGenerate(0) } else { window.clearTimeout(solveTimer); window.clearTimeout(generateTimer) } })
  watch(generationSignature, () => { if (code.value && !replay.value) { previewState.value = 'stale'; selectedBuilding.value = null; selectedProductions.value = [] } })
  watch(datasetKey, () => { if (initialized) void loadDataset() })
  watch(() => route.query.itemId, value => { if (initialized && typeof value === 'string' && value) void loadLibraryItem(value) })
  onMounted(async () => {
    media = window.matchMedia('(max-width: 1099px)'); updateWidth(); media.addEventListener('change', updateWidth)
    try { datasets.value = await listFactoryDatasets(); await loadDataset(); initialized = true; if (typeof route.query.itemId === 'string') await loadLibraryItem(route.query.itemId); else scheduleSolve() } catch (caught) { report(caught) }
  })
  onActivated(() => { active.value = true; if (initialized && typeof route.query.itemId === 'string') void loadLibraryItem(route.query.itemId) })
  onDeactivated(() => { active.value = false })
  onBeforeUnmount(() => { window.clearTimeout(solveTimer); window.clearTimeout(generateTimer); disposed = true; datasetVersion++; importVersion++; solveVersion++; media?.removeEventListener('change', updateWidth) })
  return {zones, zoneOfProduction, activeZone, focusZone, number, autoRun, targetsReady, setTargetRate, removeTarget, addTargetItem, isExternal, toggleExternal, exportWorkspace, previewRevision, itemRecipeChoices, active, datasets, datasetKey, detail, targets, itemMap, itemOptions, mineralized, miningSpeed, fixedNum, researchStack, sprayItself, spaceCost, powerCost, mode, name, layout, plan, generated, inspection, code, replay, loadingDataset, solving, generating, importing, saving, error, previewState, previewNotice, showPreview, narrow, selectedBuilding, selectedProductions, importVisible, importText, sourceName, quantStale, previewStale, expectedBuildings, warnings, itemName, recipeOptions, selectedRecipe, overrideFor, updateRecipe, updateOverride, buildingOptions, sprayModes, pointOptions, calculate, generate, loadCode, importFile, recalculateImport, selectProduction, onBuildingSelect, onPreviewReady, onPreviewError, copy, download, save, openLibrary, addTarget, setExternal}
}
// planZoneFallback 职责：生成前按量化归属预估分区；使用：节点图着色；边界：与 Go planZones 规则一致，生成后以 Go 结果为准。
function planZoneFallback(plan: quant.Result): {key: string; targetItemId: number; productionIndices: number[]}[] {
  const targets = [...new Set((plan.targets ?? []).map(target => target.itemId))]
  const zones = targets.map(id => ({key: `target-${id}`, targetItemId: id, productionIndices: [] as number[]}))
  const shared = {key: 'shared', targetItemId: 0, productionIndices: [] as number[]}
  plan.productions.forEach((production, index) => {
    const owners = production.targetItemIds ?? []
    const zone = owners.length === 1 ? zones.find(value => value.targetItemId === owners[0]) : undefined
    ;(zone ?? shared).productionIndices.push(index)
  })
  if (zones.length <= 1) return zones.length ? [{...zones[0], productionIndices: [...shared.productionIndices, ...zones[0].productionIndices]}] : []
  return [...(shared.productionIndices.length ? [shared] : []), ...zones.filter(zone => zone.productionIndices.length)]
}
export type QuantWorkspace = UnwrapNestedRefs<ReturnType<typeof useQuantWorkspace>>
