import {useMessage} from 'naive-ui'
import {computed, onBeforeUnmount, reactive, ref, watch} from 'vue'
import {useRouter} from 'vue-router'
import {accountSessionKey, verifiedAccountSession} from '../../pages/services/account'
import {
  createForumRequestId,
  fetchForumCategories,
  fetchForumPostDetail,
  fetchForumPosts,
  fetchForumReplies,
  fetchForumBlueprintCode,
  ForumError,
  importForumBlueprint,
  importForumMod,
  importForumSave,
  publishForumPost,
  publishForumReply,
  releaseForumLocalPreviews,
  reportForumTarget,
  setPendingForumModTarget,
  verifyForumResource,
} from '../../services/forum'
import type {
  ForumCategory,
  ForumDraftBindings,
  ForumPostDetail,
  ForumPostSummary,
  ForumReply,
  ForumReportTarget,
  ForumResource,
} from '../../types/forum'
import {buildThunderstorePackageUrl, extractReferencedResourceSlots} from '../../utils/forum-markdown'
import {formatErrorMessage} from '../../utils/format'

// =============== 社区论坛页面状态组合 ===============
// 说明：集中维护论坛分类、帖子分页列表、帖子详情/回复、发帖与回复草稿（含幂等 requestId 重试）、举报和资源获取导入流程。
// 边界：所有网络请求绑定当前已验证账号会话；账号切换或注销时自动清理远端视图，不把旧账号草稿提交给新账号。

export type ForumViewMode = 'list' | 'detail' | 'compose'

function createEmptyBindings(): ForumDraftBindings {
  return {
    images: {},
    video: null,
    blueprint: null,
    mod: null,
    save: null,
  }
}

function collectPreviewTokens(bindings: ForumDraftBindings): string[] {
  const tokens: string[] = []
  for (const file of Object.values(bindings.images)) {
    if (file?.previewToken) tokens.push(file.previewToken)
  }
  if (bindings.video?.previewToken) {
    tokens.push(bindings.video.previewToken)
  }
  return tokens
}

function clearBindings(bindings: ForumDraftBindings): void {
  const tokens = collectPreviewTokens(bindings)
  for (const key of Object.keys(bindings.images)) {
    delete bindings.images[key]
  }
  bindings.video = null
  bindings.blueprint = null
  bindings.mod = null
  bindings.save = null
  if (tokens.length) {
    void releaseForumLocalPreviews(tokens)
  }
}

function buildPostDraftSignature(categoryId: number | null, title: string, body: string, bindings: ForumDraftBindings): string {
  return JSON.stringify({
    categoryId,
    title: title.trim(),
    body,
    images: Object.keys(bindings.images).sort().map((slot) => [slot, bindings.images[slot]?.path]),
    video: bindings.video?.path ?? '',
    blueprint: bindings.blueprint
      ? [bindings.blueprint.mode, bindings.blueprint.name, bindings.blueprint.file?.path ?? '', bindings.blueprint.blueprintCode ?? '']
      : null,
    mod: bindings.mod
      ? [bindings.mod.mode, bindings.mod.file?.path ?? '', bindings.mod.namespace ?? '', bindings.mod.packageName ?? '', bindings.mod.version ?? '', bindings.mod.url ?? '']
      : null,
    save: bindings.save?.path ?? '',
  })
}

function buildReplyDraftSignature(postId: number, body: string, bindings: ForumDraftBindings): string {
  return JSON.stringify({
    postId,
    body,
    images: Object.keys(bindings.images).sort().map((slot) => [slot, bindings.images[slot]?.path]),
  })
}

export function useForumState() {
  const router = useRouter()
  const message = useMessage()

  const viewMode = ref<ForumViewMode>('list')
  const isAuthenticated = computed(() => Boolean(verifiedAccountSession.value && accountSessionKey.value))

  // 分类与列表状态
  const categories = ref<ForumCategory[]>([])
  const categoriesLoading = ref(false)
  const selectedCategoryId = ref<number>(0)
  const posts = ref<ForumPostSummary[]>([])
  const postsTotal = ref(0)
  const postsPage = ref(1)
  const postsPageSize = ref(20)
  const postsLoading = ref(false)
  const postsError = ref('')

  // 详情与回复列表状态
  const currentPostId = ref<number>(0)
  const postDetail = ref<ForumPostDetail | null>(null)
  const detailLoading = ref(false)
  const detailError = ref('')
  const replies = ref<ForumReply[]>([])
  const repliesTotal = ref(0)
  const repliesPage = ref(1)
  const repliesPageSize = ref(20)
  const repliesLoading = ref(false)
  const repliesError = ref('')

  // 发帖草稿状态（保留重试 requestId 与已选文件）
  const postDraft = reactive({
    categoryId: null as number | null,
    title: '',
    body: '',
    bindings: createEmptyBindings(),
    requestId: createForumRequestId(),
    lastSubmittedSignature: '',
    submitting: false,
    lastError: '',
  })

  // 回复草稿状态（保留重试 requestId 与已选图片）
  const replyDraft = reactive({
    body: '',
    bindings: createEmptyBindings(),
    requestId: createForumRequestId(),
    lastSubmittedSignature: '',
    submitting: false,
    lastError: '',
  })

  // 资源操作与举报弹窗状态
  const busyResourceAction = ref('')
  const reportModalOpen = ref(false)
  const reportTarget = ref<ForumReportTarget | null>(null)
  const reportSubmitting = ref(false)

  const enabledCategories = computed(() => categories.value.filter((item) => item.enabled))

  // =============== 数据加载 ===============
  async function loadCategories(): Promise<void> {
    if (!isAuthenticated.value) return
    categoriesLoading.value = true
    try {
      const list = await fetchForumCategories()
      categories.value = list
      if (selectedCategoryId.value > 0 && !list.some((item) => item.id === selectedCategoryId.value)) {
        selectedCategoryId.value = 0
      }
      if (postDraft.categoryId === null && enabledCategories.value.length > 0) {
        postDraft.categoryId = enabledCategories.value[0].id
      }
    } catch (error) {
      message.error(formatErrorMessage(error))
    } finally {
      categoriesLoading.value = false
    }
  }

  async function loadPosts(page = postsPage.value): Promise<void> {
    if (!isAuthenticated.value) return
    postsLoading.value = true
    postsError.value = ''
    try {
      const result = await fetchForumPosts(selectedCategoryId.value, page, postsPageSize.value)
      posts.value = result.list ?? []
      postsTotal.value = result.total ?? 0
      postsPage.value = result.page || page
    } catch (error) {
      postsError.value = formatErrorMessage(error)
    } finally {
      postsLoading.value = false
    }
  }

  async function refreshList(): Promise<void> {
    await Promise.all([loadCategories(), loadPosts(postsPage.value)])
  }

  function selectCategory(categoryId: number): void {
    if (selectedCategoryId.value === categoryId) return
    selectedCategoryId.value = categoryId
    postsPage.value = 1
    void loadPosts(1)
  }

  function changePostsPage(page: number): void {
    postsPage.value = page
    void loadPosts(page)
  }

  async function openPostDetail(postId: number): Promise<void> {
    if (!postId || !isAuthenticated.value) return
    if (currentPostId.value !== postId) {
      resetReplyDraft()
    }
    currentPostId.value = postId
    viewMode.value = 'detail'
    repliesPage.value = 1
    await Promise.all([loadPostDetail(postId), loadReplies(postId, 1)])
  }

  async function loadPostDetail(postId = currentPostId.value): Promise<void> {
    if (!postId || !isAuthenticated.value) return
    detailLoading.value = true
    detailError.value = ''
    try {
      postDetail.value = await fetchForumPostDetail(postId)
    } catch (error) {
      detailError.value = formatErrorMessage(error)
      if (error instanceof ForumError && error.reason === 'notFound') {
        postDetail.value = null
      }
    } finally {
      detailLoading.value = false
    }
  }

  async function loadReplies(postId = currentPostId.value, page = repliesPage.value): Promise<void> {
    if (!postId || !isAuthenticated.value) return
    repliesLoading.value = true
    repliesError.value = ''
    try {
      const result = await fetchForumReplies(postId, page, repliesPageSize.value)
      replies.value = result.list ?? []
      repliesTotal.value = result.total ?? 0
      repliesPage.value = result.page || page
    } catch (error) {
      repliesError.value = formatErrorMessage(error)
    } finally {
      repliesLoading.value = false
    }
  }

  function changeRepliesPage(page: number): void {
    repliesPage.value = page
    void loadReplies(currentPostId.value, page)
  }

  function openCompose(): void {
    if (!categories.value.length && isAuthenticated.value) {
      void loadCategories()
    } else if (postDraft.categoryId === null && enabledCategories.value.length > 0) {
      postDraft.categoryId = enabledCategories.value[0].id
    }
    viewMode.value = 'compose'
  }

  function backToList(reload = false): void {
    viewMode.value = 'list'
    if (reload) {
      void loadPosts(postsPage.value)
    }
  }

  function resetPostDraft(): void {
    postDraft.title = ''
    postDraft.body = ''
    clearBindings(postDraft.bindings)
    postDraft.requestId = createForumRequestId()
    postDraft.lastSubmittedSignature = ''
    postDraft.lastError = ''
    postDraft.categoryId = enabledCategories.value[0]?.id ?? null
  }

  function resetReplyDraft(): void {
    replyDraft.body = ''
    clearBindings(replyDraft.bindings)
    replyDraft.requestId = createForumRequestId()
    replyDraft.lastSubmittedSignature = ''
    replyDraft.lastError = ''
  }

  // =============== 提交前资源标签校验 ===============
  function validateDraftBindings(body: string, bindings: ForumDraftBindings, allowPostOnlyResources: boolean): string | null {
    const referenced = new Set(extractReferencedResourceSlots(body))
    const boundSlots = new Set<string>()

    for (const slot of Object.keys(bindings.images)) {
      if (bindings.images[slot]) boundSlots.add(slot)
    }
    if (allowPostOnlyResources) {
      if (bindings.video) boundSlots.add('video')
      if (bindings.blueprint) boundSlots.add('blueprint')
      if (bindings.mod) boundSlots.add('mod')
      if (bindings.save) boundSlots.add('save')
    }

    for (const slot of boundSlots) {
      if (!referenced.has(slot)) {
        return `已选择的资源 [[resource:${slot}]] 尚未插入到正文中，请点击下方「插入正文」或移除该资源。`
      }
    }
    for (const slot of referenced) {
      if (!boundSlots.has(slot)) {
        return `正文中的 [[resource:${slot}]] 未绑定对应资源，请通过工具栏插入资源或删除该标签。`
      }
    }
    return null
  }

  // =============== 发帖提交与失败重试 ===============
  async function submitPost(): Promise<void> {
    if (postDraft.submitting) return
    if (!postDraft.categoryId) {
      message.warning('请选择帖子分类')
      return
    }
    const title = postDraft.title.trim()
    if (!title) {
      message.warning('请输入帖子标题')
      return
    }
    if (Array.from(title).length > 120) {
      message.warning('帖子标题不能超过 120 个字符')
      return
    }
    if (!postDraft.body.trim()) {
      message.warning('请输入帖子正文')
      return
    }
    if (Array.from(postDraft.body).length > 10000) {
      message.warning('帖子正文不能超过 10,000 个字符')
      return
    }
    const bindingError = validateDraftBindings(postDraft.body, postDraft.bindings, true)
    if (bindingError) {
      message.warning(bindingError)
      return
    }

    const signature = buildPostDraftSignature(postDraft.categoryId, title, postDraft.body, postDraft.bindings)
    if (postDraft.lastSubmittedSignature && postDraft.lastSubmittedSignature !== signature) {
      postDraft.requestId = createForumRequestId()
    }
    postDraft.lastSubmittedSignature = signature
    postDraft.submitting = true
    postDraft.lastError = ''

    try {
      const media: Array<{slot: string; path: string}> = []
      for (const [slot, file] of Object.entries(postDraft.bindings.images)) {
        if (file?.path) media.push({slot, path: file.path})
      }
      if (postDraft.bindings.video?.path) {
        media.push({slot: 'video', path: postDraft.bindings.video.path})
      }

      const result = await publishForumPost({
        requestId: postDraft.requestId,
        categoryId: postDraft.categoryId,
        title,
        body: postDraft.body,
        bodyFormat: 'markdown',
        blueprint: postDraft.bindings.blueprint?.mode === 'inline'
          ? {name: postDraft.bindings.blueprint.name, blueprintCode: postDraft.bindings.blueprint.blueprintCode || ''}
          : undefined,
        blueprintFilePath: postDraft.bindings.blueprint?.mode === 'file'
          ? postDraft.bindings.blueprint.file?.path
          : undefined,
        mod: postDraft.bindings.mod?.mode === 'thunderstore'
          ? (postDraft.bindings.mod.url
              ? {
                  url: postDraft.bindings.mod.url,
                  version: postDraft.bindings.mod.version || undefined,
                }
              : postDraft.bindings.mod.version
                ? {
                    namespace: postDraft.bindings.mod.namespace,
                    packageName: postDraft.bindings.mod.packageName,
                    version: postDraft.bindings.mod.version,
                  }
                : {
                    url: buildThunderstorePackageUrl(
                      postDraft.bindings.mod.namespace || '',
                      postDraft.bindings.mod.packageName || '',
                    ),
                  })
          : undefined,
        modFilePath: postDraft.bindings.mod?.mode === 'file'
          ? postDraft.bindings.mod.file?.path
          : undefined,
        saveFilePath: postDraft.bindings.save?.path,
        media,
      })

      message.success('帖子发布成功')
      resetPostDraft()
      selectedCategoryId.value = 0
      postsPage.value = 1
      await loadPosts(1)
      if (result.id > 0) {
        await openPostDetail(result.id)
      } else {
        viewMode.value = 'list'
      }
    } catch (error) {
      const text = formatErrorMessage(error)
      postDraft.lastError = text
      // 保留草稿、已选文件与原 requestId 供用户安全重试；若返回 conflict 则轮换新 requestId。
      if (error instanceof ForumError && error.reason === 'conflict') {
        postDraft.requestId = createForumRequestId()
      }
      if (error instanceof ForumError && error.reason === 'categoryDisabled') {
        void loadCategories()
      }
      message.error(text)
    } finally {
      postDraft.submitting = false
    }
  }

  // =============== 回复提交与失败重试 ===============
  async function submitReply(): Promise<void> {
    const postId = currentPostId.value
    if (!postId || replyDraft.submitting) return
    if (!replyDraft.body.trim()) {
      message.warning('请输入回复内容')
      return
    }
    if (Array.from(replyDraft.body).length > 2000) {
      message.warning('回复正文不能超过 2,000 个字符')
      return
    }
    const bindingError = validateDraftBindings(replyDraft.body, replyDraft.bindings, false)
    if (bindingError) {
      message.warning(bindingError)
      return
    }

    const signature = buildReplyDraftSignature(postId, replyDraft.body, replyDraft.bindings)
    if (replyDraft.lastSubmittedSignature && replyDraft.lastSubmittedSignature !== signature) {
      replyDraft.requestId = createForumRequestId()
    }
    replyDraft.lastSubmittedSignature = signature
    replyDraft.submitting = true
    replyDraft.lastError = ''

    try {
      const images = Object.entries(replyDraft.bindings.images)
        .filter(([, file]) => Boolean(file?.path))
        .map(([slot, file]) => ({slot, path: file.path}))

      await publishForumReply({
        postId,
        requestId: replyDraft.requestId,
        body: replyDraft.body,
        bodyFormat: 'markdown',
        images,
      })

      message.success('回复已发送')
      resetReplyDraft()
      await Promise.all([loadPostDetail(postId), loadReplies(postId, repliesPage.value)])
    } catch (error) {
      const text = formatErrorMessage(error)
      replyDraft.lastError = text
      if (error instanceof ForumError && error.reason === 'conflict') {
        replyDraft.requestId = createForumRequestId()
      }
      message.error(text)
    } finally {
      replyDraft.submitting = false
    }
  }

  // =============== 举报处理 ===============
  function openReport(target: ForumReportTarget): void {
    reportTarget.value = target
    reportModalOpen.value = true
  }

  async function submitReport(reason: string): Promise<void> {
    if (!reportTarget.value || reportSubmitting.value) return
    reportSubmitting.value = true
    try {
      await reportForumTarget(reportTarget.value.targetType, reportTarget.value.targetId, reason)
      message.success('举报已提交，感谢你的反馈')
      reportModalOpen.value = false
      reportTarget.value = null
    } catch (error) {
      message.error(formatErrorMessage(error))
    } finally {
      reportSubmitting.value = false
    }
  }

  // =============== 游戏资源复用（蓝图/存档/模组） ===============
  async function handleResourceError(error: unknown): Promise<void> {
    if (error instanceof ForumError && error.reason === 'notFound') {
      message.warning('该资源已下架或不可用，已为你刷新最新状态。')
      if (currentPostId.value > 0) {
        await loadPostDetail(currentPostId.value)
      }
      return
    }
    if (error instanceof ForumError && error.reason === 'importConflict') {
      message.warning(error.message)
      return
    }
    message.error(formatErrorMessage(error))
  }

  async function handleCopyBlueprint(resource: ForumResource): Promise<void> {
    const key = `${resource.id}:copy-blueprint`
    if (busyResourceAction.value) return
    busyResourceAction.value = key
    try {
      const result = await fetchForumBlueprintCode(resource.id)
      await navigator.clipboard.writeText(result.blueprintCode)
      message.success(`已复制蓝图「${result.name}」的编码`)
    } catch (error) {
      await handleResourceError(error)
    } finally {
      busyResourceAction.value = ''
    }
  }

  async function handleImportBlueprint(resource: ForumResource): Promise<void> {
    const key = `${resource.id}:import-blueprint`
    if (busyResourceAction.value) return
    busyResourceAction.value = key
    try {
      const result = await importForumBlueprint(resource.id)
      message.success(result.message || `蓝图「${result.name}」已导入资源库`)
    } catch (error) {
      await handleResourceError(error)
    } finally {
      busyResourceAction.value = ''
    }
  }

  async function handleImportSave(resource: ForumResource): Promise<void> {
    const key = `${resource.id}:import-save`
    if (busyResourceAction.value) return
    busyResourceAction.value = key
    try {
      const result = await importForumSave(resource.id)
      message.success(result.message || `存档「${result.name}」已导入`)
    } catch (error) {
      await handleResourceError(error)
    } finally {
      busyResourceAction.value = ''
    }
  }

  async function handleImportMod(resource: ForumResource): Promise<void> {
    const key = `${resource.id}:import-mod`
    if (busyResourceAction.value) return
    busyResourceAction.value = key
    try {
      const result = await importForumMod(resource.id)
      message.success(result.message || `模组「${result.name}」已导入本地库`)
    } catch (error) {
      await handleResourceError(error)
    } finally {
      busyResourceAction.value = ''
    }
  }

  async function handleOpenThunderstore(resource: ForumResource): Promise<void> {
    const key = `${resource.id}:open-thunderstore`
    if (busyResourceAction.value) return
    busyResourceAction.value = key
    try {
      const latest = await verifyForumResource(resource.id)
      if (!latest.namespace || !latest.packageName) {
        message.warning('该模组资源缺少 Thunderstore 包坐标信息')
        return
      }
      setPendingForumModTarget({
        namespace: latest.namespace,
        packageName: latest.packageName,
        version: latest.version || undefined,
        url: latest.url || undefined,
      })
      await router.push('/dysonSphere/mod')
    } catch (error) {
      await handleResourceError(error)
    } finally {
      busyResourceAction.value = ''
    }
  }

  // =============== 会话变化监听 ===============
  watch(accountSessionKey, (nextKey, prevKey) => {
    if (nextKey === prevKey) return
    categories.value = []
    posts.value = []
    postsTotal.value = 0
    postsError.value = ''
    postDetail.value = null
    detailError.value = ''
    replies.value = []
    repliesTotal.value = 0
    repliesError.value = ''
    selectedCategoryId.value = 0
    postsPage.value = 1
    currentPostId.value = 0
    reportModalOpen.value = false
    reportTarget.value = null
    viewMode.value = 'list'
    resetPostDraft()
    resetReplyDraft()
    if (nextKey) {
      void refreshList()
    }
  }, {immediate: true})

  onBeforeUnmount(() => {
    clearBindings(postDraft.bindings)
    clearBindings(replyDraft.bindings)
  })

  return {
    viewMode,
    isAuthenticated,
    categories,
    enabledCategories,
    categoriesLoading,
    selectedCategoryId,
    posts,
    postsTotal,
    postsPage,
    postsPageSize,
    postsLoading,
    postsError,
    currentPostId,
    postDetail,
    detailLoading,
    detailError,
    replies,
    repliesTotal,
    repliesPage,
    repliesPageSize,
    repliesLoading,
    repliesError,
    postDraft,
    replyDraft,
    busyResourceAction,
    reportModalOpen,
    reportTarget,
    reportSubmitting,
    refreshList,
    selectCategory,
    changePostsPage,
    openPostDetail,
    loadPostDetail,
    loadReplies,
    changeRepliesPage,
    openCompose,
    backToList,
    resetPostDraft,
    resetReplyDraft,
    submitPost,
    submitReply,
    openReport,
    submitReport,
    handleCopyBlueprint,
    handleImportBlueprint,
    handleImportSave,
    handleImportMod,
    handleOpenThunderstore,
  }
}
