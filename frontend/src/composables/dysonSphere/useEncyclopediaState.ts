import {computed, nextTick, onBeforeUnmount, onMounted, reactive, shallowRef, watch} from 'vue'
import {useMessage} from 'naive-ui'
import {useRoute, useRouter} from 'vue-router'
import {cachedCatalog, encyclopediaGuides, encyclopediaManifest, loadAllCatalog, loadCatalogShard, loadLocalIconPack, resolveCatalogEntry} from '../../services/encyclopedia/catalog'
import {encyclopediaAchievementGroups, encyclopediaAchievements} from '../../data/encyclopedia/achievements'
import {defaultEncyclopediaCategory, encyclopediaCategories} from '../../services/encyclopedia/categories'
import {buildSearchIndex, EncyclopediaDataError, normalizeKeyword, searchCatalog} from '../../services/encyclopedia/catalog-core'
import type {EncyclopediaCategory, EncyclopediaEntry, EncyclopediaKey, EncyclopediaKind} from '../../types/encyclopedia'

// =============== 页面会话状态 ===============
// 说明：外壳以 fullPath 作为页面 key，详情 query 会重新挂载；模块级筛选状态避免丢失搜索与分页。
const view = reactive({search: '', achievementSearch: '', category: defaultEncyclopediaCategory, tag: '', page: 1, showNames: false})
const detailTrail: EncyclopediaKey[] = []
const warnedQueries = new Set<string>()
let visited = false
let detailOrigin: {type: 'entry' | 'guide'; id: string} | undefined
let focusOriginAfterClose = false
const categoryKinds: Partial<Record<EncyclopediaCategory, readonly EncyclopediaKind[]>> = {building: ['building'], item: ['item'], technology: ['technology'], achievement: []}
const categories: readonly string[] = encyclopediaCategories.map(category => category.key)

export function useEncyclopediaState() {
  const route = useRoute()
  const router = useRouter()
  const message = useMessage()
  const entries = shallowRef<EncyclopediaEntry[]>(cachedCatalog())
  const loading = shallowRef(entries.value.length === 0)
  const error = shallowRef('')
  let active = true
  const animateEntrance = !visited && document.visibilityState === 'visible'
  visited = true
  if (typeof route.query.category === 'string' && categories.includes(route.query.category)) view.category = route.query.category as EncyclopediaCategory
  const searchKeyword = computed({
    get: () => view.category === 'achievement' ? view.achievementSearch : view.search,
    set: (value: string) => {
      if (view.category === 'achievement') view.achievementSearch = value
      else view.search = value
    },
  })
  if (typeof route.query.q === 'string') searchKeyword.value = route.query.q
  if (typeof route.query.tag === 'string') view.tag = route.query.tag
  if (typeof route.query.page === 'string') view.page = Math.max(1, Number.parseInt(route.query.page) || 1)

  // =============== 搜索与分组 ===============
  // 说明：顶部搜索按分类选择实体或成就关键词；实体关键词跨全部实体类型搜索（含配方与资源），成就关键词与分区搜索共用状态。
  const index = computed(() => buildSearchIndex(entries.value))
  const options = computed(() => ({kinds: categoryKinds[view.category], combat: view.category === 'combat'}))
  const categoryEntries = computed(() => searchCatalog(index.value, '', options.value))
  const tagOptions = computed(() => view.category === 'achievement' ? achievementGroupOptions.value : [...new Set(categoryEntries.value.flatMap(entry => entry.tags))].sort().map(tag => ({label: tag, value: tag})))
  const searching = computed(() => view.category !== 'achievement' && Boolean(normalizeKeyword(view.search)))
  const matches = computed(() => searchCatalog(index.value, view.search, searching.value ? {} : {...options.value, tag: view.tag}))
  const pageCount = computed(() => Math.max(1, Math.ceil(matches.value.length / 60)))
  const pageEntries = computed(() => matches.value.slice((Math.min(view.page, pageCount.value) - 1) * 60, Math.min(view.page, pageCount.value) * 60))
  const guides = computed(() => encyclopediaGuides.filter(guide => guide.category === view.category
    && (!view.search.trim() || normalizeKeyword(`${guide.title} ${guide.summary}`).includes(normalizeKeyword(view.search)))))

  // =============== 成就索引 ===============
  // 说明：成就按编号、名称、条件与提示筛选；顶部和分区搜索保持同步，关键词与分组随详情链接保存，编号兼容列表中的三位格式。
  const achievementMatches = computed(() => {
    if (view.category !== 'achievement') return []
    entries.value
    const query = normalizeKeyword(view.achievementSearch)
    return encyclopediaAchievements.filter(achievement => !query || normalizeKeyword(`${achievement.id} ${String(achievement.id).padStart(3, '0')} ${achievement.name} ${achievement.condition} ${achievement.tip ?? ''}`).includes(query))
      .map(achievement => ({...achievement, entries: (achievement.related ?? []).flatMap(key => { const entry = resolveCatalogEntry(key); return entry ? [{key, entry}] : [] })}))
  })
  const achievementGroupOptions = computed(() => encyclopediaAchievementGroups.map(group => ({label: `${group.label} · ${achievementMatches.value.filter(achievement => achievement.group === group.key).length}`, value: group.key})))
  const achievements = computed(() => view.tag ? achievementMatches.value.filter(achievement => achievement.group === view.tag) : achievementMatches.value)
  watch([() => view.search, () => view.category, () => view.tag], () => { view.page = 1 }, {flush: 'sync'})

  const selectedEntry = computed(() => {
    entries.value
    if (typeof route.query.kind !== 'string' || typeof route.query.id !== 'string') return undefined
    return resolveCatalogEntry(`${route.query.kind}:${route.query.id}` as EncyclopediaKey)
  })
  const selectedGuide = computed(() => encyclopediaGuides.find(guide => guide.id === route.query.guide))
  const drawerOpen = computed(() => Boolean(selectedEntry.value || selectedGuide.value))
  const canGoBack = computed(() => drawerOpen.value && detailTrail.length > 0)

  // =============== 详情与 URL 同步 ===============
  // 说明：显式保存筛选快照到详情链接；刷新、快速关系跳转和关闭抽屉均保持当前浏览上下文。
  function viewQuery(): Record<string, string> {
    return {category: view.category, ...(searchKeyword.value ? {q: searchKeyword.value} : {}), ...(view.tag ? {tag: view.tag} : {}), ...(view.page > 1 ? {page: String(view.page)} : {})}
  }
  function openEntry(key: EncyclopediaKey, remember = true): void {
    if (!resolveCatalogEntry(key)) { message.warning('这个关联条目暂未收录'); return }
    if (selectedEntry.value && `${selectedEntry.value.kind}:${selectedEntry.value.id}` === key) return
    if (!selectedEntry.value && !selectedGuide.value) detailOrigin = {type: 'entry', id: key}
    if (remember && selectedEntry.value) detailTrail.push(`${selectedEntry.value.kind}:${selectedEntry.value.id}`)
    const separator = key.indexOf(':')
    void router.replace({path: '/dysonSphere/encyclopedia', query: {...viewQuery(), kind: key.slice(0, separator), id: key.slice(separator + 1)}})
  }
  function openGuide(id: string): void {
    detailTrail.length = 0
    detailOrigin = {type: 'guide', id}
    void router.replace({path: '/dysonSphere/encyclopedia', query: {...viewQuery(), guide: id}})
  }
  function closeDetail(): void {
    detailTrail.length = 0
    focusOriginAfterClose = true
    void router.replace({path: '/dysonSphere/encyclopedia', query: viewQuery()})
  }
  function backDetail(): void {
    const previous = detailTrail.pop()
    if (previous) openEntry(previous, false)
  }
  function selectCategory(category: EncyclopediaCategory): void {
    view.tag = ''
    view.search = ''
    view.achievementSearch = ''
    view.category = category
    void router.replace({path: '/dysonSphere/encyclopedia', query: viewQuery()})
  }
  function resetFilters(): void {
    view.search = ''
    view.achievementSearch = ''
    view.tag = ''
    view.page = 1
    void router.replace({path: '/dysonSphere/encyclopedia', query: viewQuery()})
  }
  function commitSearch(): void {
    void router.replace({path: '/dysonSphere/encyclopedia', query: viewQuery()})
  }
  function selectTag(tag: string): void {
    view.tag = tag
    commitSearch()
  }
  function selectPage(page: number): void {
    view.page = Math.min(Math.max(1, page), pageCount.value)
    commitSearch()
  }

  // =============== 数据加载与失效边界 ===============
  // 说明：先加载建筑首屏，其余分片随后按需 import 并缓存；失败保留筛选，卸载后不回写旧页面。
  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      await loadCatalogShard('building')
      if (!active) return
      entries.value = cachedCatalog()
      const all = await loadAllCatalog()
      if (!active) return
      entries.value = all
      view.page = Math.min(view.page, pageCount.value)
      const hasSelection = route.query.id !== undefined || route.query.kind !== undefined || route.query.guide !== undefined
      if (hasSelection && !selectedEntry.value && !selectedGuide.value) {
        if (!warnedQueries.has(route.fullPath)) { warnedQueries.add(route.fullPath); message.warning('未找到这个图鉴条目，已返回列表') }
        await router.replace({path: '/dysonSphere/encyclopedia', query: viewQuery()})
      }
      if (!hasSelection && focusOriginAfterClose && detailOrigin) {
        focusOriginAfterClose = false
        const selector = `[data-${detailOrigin.type}="${CSS.escape(detailOrigin.id)}"]`
        await nextTick()
        if (active) document.querySelector<HTMLButtonElement>(selector)?.focus({preventScroll: true})
      }
    } catch (failure) {
      if (active) error.value = failure instanceof EncyclopediaDataError ? failure.message : '图鉴加载失败，请重试'
    } finally {
      if (active) loading.value = false
    }
  }
  onMounted(() => { void loadLocalIconPack(); void load() })
  onBeforeUnmount(() => { active = false })

  return {view, searchKeyword, loading, error, matches, searching, achievements, pageEntries, pageCount, tagOptions, guides, selectedEntry, selectedGuide, drawerOpen, canGoBack, animateEntrance, openEntry, openGuide, closeDetail, backDetail, selectCategory, resetFilters, commitSearch, selectTag, selectPage, retry: load}
}
