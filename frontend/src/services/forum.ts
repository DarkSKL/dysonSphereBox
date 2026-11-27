import {ref, shallowRef} from 'vue'
import * as forumBindings from '../../wailsjs/go/forum/ForumService'
import {forum as forumModels} from '../../wailsjs/go/models'
import {accountSessionKey, resetAccountSession, verifiedAccountSession} from '../pages/services/account'
import type {
  ForumCategory,
  ForumPage,
  ForumPostDetail,
  ForumPostSummary,
  ForumReply,
  ForumReportTargetType,
  ForumResource,
  ForumSelectedLocalFile,
  PendingForumModTarget,
} from '../types/forum'

// =============== 社区论坛 Wails 服务封装 ===============
// 说明：统一注入已验证账号 sessionKey、校验并发会话切走与标准化错误原因；媒体读取走 AssetServer 鉴权代理路径。
// 边界：不保存敏感令牌，不直接访问应用服务器；遇到登录失效或强制改密时同步重置前端会话。

export class ForumError extends Error {
  constructor(readonly reason: string, message: string) {
    super(message)
  }
}

const errorLabels: Record<string, string> = {
  invalid: '提交内容或正文资源标签不符合要求，请检查后重试。',
  invalidInput: '请检查输入内容或所选文件后重试。',
  notFound: '目标内容不存在、已隐藏或资源已下架。',
  categoryDisabled: '该分类已停用，请重新选择其他分类。',
  saveTooLarge: '存档超过 512 MiB 限制。',
  fileTooLarge: '上传文件超过该类型的大小限制。',
  quotaExceeded: '当前账号托管文件已超过 2 GiB 容量上限。',
  alreadyHandled: '该举报已处理。',
  conflict: '请求标识已用于其他内容，或目标状态已变化。',
  unavailable: '论坛服务暂不可用，请稍后重试。',
  unsupported: '当前服务器暂未提供社区论坛功能。',
  timeout: '请求超时，请稍后重试。已填写的内容和所选文件会保留。',
  stale: '账号会话已切换。',
  canceled: '请求已取消。',
  unauthorized: '登录已失效，请重新登录。',
  passwordRequired: '请先在账号窗口修改密码。',
}

function normalizeForumError(error: unknown): ForumError {
  if (error instanceof ForumError) return error
  const messageField = error && typeof error === 'object' && 'message' in error ? error.message : ''
  const raw = typeof error === 'string' ? error : typeof messageField === 'string' ? messageField : ''
  let payload: {reason?: string; message?: string} = {}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === 'object') payload = parsed
  } catch {
    // 非结构化异常走兜底提示
  }
  const reason = typeof payload.reason === 'string' ? payload.reason : 'requestFailed'
  const message = typeof payload.message === 'string' && payload.message.trim()
    ? payload.message.trim()
    : errorLabels[reason] || '连接社区论坛失败，请稍后重试。'
  return new ForumError(reason, message)
}

function assertBridgeMethod(name: keyof typeof forumBindings): void {
  const bridge = (window as Window & {go?: {forum?: {ForumService?: Record<string, unknown>}}}).go?.forum?.ForumService
  if (typeof bridge?.[name] !== 'function') {
    throw new ForumError('unsupported', errorLabels.unsupported)
  }
}

async function runSessionCall<T>(methodName: keyof typeof forumBindings, invoke: (sessionKey: string) => Promise<T>): Promise<T> {
  const sessionKey = accountSessionKey.value
  if (!sessionKey) {
    throw new ForumError('stale', errorLabels.stale)
  }
  assertBridgeMethod(methodName)
  try {
    const result = await invoke(sessionKey)
    if (sessionKey !== accountSessionKey.value) {
      throw new ForumError('stale', errorLabels.stale)
    }
    return result
  } catch (error) {
    if (sessionKey !== accountSessionKey.value) {
      throw new ForumError('stale', errorLabels.stale)
    }
    const failure = normalizeForumError(error)
    if (failure.reason === 'unauthorized' || failure.reason === 'passwordRequired' || failure.reason === 'stale') {
      resetAccountSession()
    }
    throw failure
  }
}

// 说明：生成经 Wails AssetServer 鉴权代理的远端媒体地址；附带会话代次避免切换账号后读到缓存。
export function buildForumMediaUrl(resourceId: number): string {
  const rawGen = String(verifiedAccountSession.value?.generation ?? '').trim()
  return rawGen && rawGen !== '0'
    ? `/__dsbox/forum/media/${resourceId}?g=${encodeURIComponent(rawGen)}`
    : `/__dsbox/forum/media/${resourceId}`
}

// 说明：生成用于发帖和带图回复重试幂等的 UUID v4。
export function createForumRequestId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (char) => {
    const rand = Math.floor(Math.random() * 16)
    const value = char === 'x' ? rand : (rand & 0x3) | 0x8
    return value.toString(16)
  })
}

/** 读取论坛分类，显式刷新时跳过 Go 层已保存的结果。 */
export async function fetchForumCategories(force = false): Promise<ForumCategory[]> {
  return runSessionCall('Categories', (sessionKey) =>
    forumBindings.Categories(forumModels.SessionRequest.createFrom({sessionKey, force})),
  ) as Promise<ForumCategory[]>
}

/** 读取指定论坛分类和页码，将刷新意图随分页参数交给 Go 层。 */
export async function fetchForumPosts(
  input: Omit<forumModels.PostListRequest, 'sessionKey'>,
): Promise<ForumPage<ForumPostSummary>> {
  return runSessionCall('Posts', (sessionKey) =>
    forumBindings.Posts(forumModels.PostListRequest.createFrom({...input, sessionKey})),
  ) as Promise<ForumPage<ForumPostSummary>>
}

/** 读取帖子详情，刷新资源状态时跳过 Go 层已保存的结果。 */
export async function fetchForumPostDetail(postId: number, force = false): Promise<ForumPostDetail> {
  return runSessionCall('PostDetail', (sessionKey) =>
    forumBindings.PostDetail(forumModels.PostDetailRequest.createFrom({sessionKey, postId, force})),
  ) as Promise<ForumPostDetail>
}

export async function fetchForumReplies(postId: number, page: number, pageSize: number): Promise<ForumPage<ForumReply>> {
  return runSessionCall('Replies', (sessionKey) =>
    forumBindings.Replies(forumModels.ReplyListRequest.createFrom({sessionKey, postId, page, pageSize})),
  ) as Promise<ForumPage<ForumReply>>
}

// =============== 发布审核通知 ===============
// 说明：按审核记录 ID 查询作者可见结果。通过的目标交给当前论坛页消费，账号作用域随目标一起传递。
export function fetchForumPublicationReview(id: string): Promise<forumModels.PublicationReview> {
  return runSessionCall('PublicationReview', (sessionKey) => forumBindings.PublicationReview(sessionKey, id))
}

export const forumReviewTarget = shallowRef<{sessionKey: string; postId: number; replyId: number} | null>(null)

export interface PublishForumPostPayload {
  requestId: string
  categoryId: number
  title: string
  body: string
  bodyFormat?: 'markdown' | 'plain'
  blueprint?: {name: string; blueprintCode: string}
  blueprintFilePath?: string
  mod?: {namespace?: string; packageName?: string; version?: string; url?: string}
  modFilePath?: string
  saveFilePath?: string
  media?: Array<{slot: string; path: string}>
}

export async function publishForumPost(payload: PublishForumPostPayload): Promise<{id: number}> {
  return runSessionCall('PublishPost', (sessionKey) =>
    forumBindings.PublishPost(forumModels.PublishPostRequest.createFrom({
      sessionKey,
      requestId: payload.requestId,
      categoryId: payload.categoryId,
      title: payload.title,
      body: payload.body,
      bodyFormat: payload.bodyFormat || 'markdown',
      blueprint: payload.blueprint,
      blueprintFilePath: payload.blueprintFilePath || '',
      mod: payload.mod,
      modFilePath: payload.modFilePath || '',
      saveFilePath: payload.saveFilePath || '',
      media: payload.media ?? [],
    })),
  )
}

export interface PublishForumReplyPayload {
  postId: number
  requestId: string
  body: string
  bodyFormat?: 'markdown' | 'plain'
  images?: Array<{slot: string; path: string}>
}

export async function publishForumReply(payload: PublishForumReplyPayload): Promise<ForumReply> {
  return runSessionCall('PublishReply', (sessionKey) =>
    forumBindings.PublishReply(forumModels.PublishReplyRequest.createFrom({
      sessionKey,
      postId: payload.postId,
      requestId: payload.requestId,
      body: payload.body,
      bodyFormat: payload.bodyFormat || 'markdown',
      images: payload.images ?? [],
    })),
  ) as Promise<ForumReply>
}

export async function reportForumTarget(targetType: ForumReportTargetType, targetId: number, reason: string): Promise<{id: number}> {
  return runSessionCall('Report', (sessionKey) =>
    forumBindings.Report(forumModels.ReportRequest.createFrom({
      sessionKey,
      targetType,
      targetId,
      reason,
    })),
  )
}

export async function verifyForumResource(resourceId: number): Promise<ForumResource> {
  return runSessionCall('GetResource', (sessionKey) =>
    forumBindings.GetResource(forumModels.ResourceActionRequest.createFrom({sessionKey, resourceId})),
  ) as Promise<ForumResource>
}

export async function fetchForumBlueprintCode(resourceId: number): Promise<{name: string; blueprintCode: string}> {
  return runSessionCall('FetchBlueprintCode', (sessionKey) =>
    forumBindings.FetchBlueprintCode(forumModels.ResourceActionRequest.createFrom({sessionKey, resourceId})),
  )
}

export async function importForumBlueprint(resourceId: number): Promise<{message: string; name: string; type: string; source: string}> {
  return runSessionCall('ImportBlueprint', (sessionKey) =>
    forumBindings.ImportBlueprint(forumModels.ResourceActionRequest.createFrom({sessionKey, resourceId})),
  )
}

export async function importForumSave(resourceId: number): Promise<{message: string; name: string; type: string; source: string}> {
  return runSessionCall('ImportSave', (sessionKey) =>
    forumBindings.ImportSave(forumModels.ResourceActionRequest.createFrom({sessionKey, resourceId})),
  )
}

export async function importForumMod(resourceId: number): Promise<{message: string; name: string; type: string; source: string}> {
  return runSessionCall('ImportMod', (sessionKey) =>
    forumBindings.ImportMod(forumModels.ResourceActionRequest.createFrom({sessionKey, resourceId})),
  )
}

export async function selectForumImages(maxCount: number): Promise<{files: ForumSelectedLocalFile[]; canceled: boolean}> {
  assertBridgeMethod('SelectImages')
  try {
    const result = await forumBindings.SelectImages(maxCount)
    return {
      files: (result.files ?? []) as ForumSelectedLocalFile[],
      canceled: Boolean(result.canceled),
    }
  } catch (error) {
    throw normalizeForumError(error)
  }
}

export async function selectForumVideo(): Promise<{files: ForumSelectedLocalFile[]; canceled: boolean}> {
  assertBridgeMethod('SelectVideo')
  try {
    const result = await forumBindings.SelectVideo()
    return {
      files: (result.files ?? []) as ForumSelectedLocalFile[],
      canceled: Boolean(result.canceled),
    }
  } catch (error) {
    throw normalizeForumError(error)
  }
}

export async function selectForumResourceFile(kind: 'blueprint' | 'mod' | 'save'): Promise<{files: ForumSelectedLocalFile[]; canceled: boolean}> {
  assertBridgeMethod('SelectResourceFile')
  try {
    const result = await forumBindings.SelectResourceFile(kind)
    return {
      files: (result.files ?? []) as ForumSelectedLocalFile[],
      canceled: Boolean(result.canceled),
    }
  } catch (error) {
    throw normalizeForumError(error)
  }
}

export async function releaseForumLocalPreviews(tokens: string[]): Promise<void> {
  if (!tokens.length) return
  try {
    assertBridgeMethod('ReleaseLocalPreviews')
    await forumBindings.ReleaseLocalPreviews(tokens)
  } catch {
    // 清理预览令牌失败不阻塞界面状态更新
  }
}

// =============== 模组市场跳转协同状态 ===============
// 说明：论坛模组卡点击「在模组市场查看」时写入目标包坐标，由模组页激活后消费并自动定位打开对应包或版本。
export const pendingForumModTarget = ref<PendingForumModTarget | null>(null)

export function setPendingForumModTarget(target: PendingForumModTarget | null): void {
  pendingForumModTarget.value = target
}

export function consumePendingForumModTarget(): PendingForumModTarget | null {
  const current = pendingForumModTarget.value
  pendingForumModTarget.value = null
  return current
}
