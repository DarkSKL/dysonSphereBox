import * as feedbackBindings from '../../wailsjs/go/feedback/FeedbackService'
import {accountSessionKey, resetAccountSession} from '../pages/services/account'
import type {
  FeedbackNotice, MapSnapshot, MyStar, Page, ReviewState, StarDetail, StarPreview, StarWrite,
} from '../types/feedback'

interface FeedbackResponses {
  Map: MapSnapshot
  Preview: StarPreview
  Previews: {items?: StarPreview[] | null}
  Detail: StarDetail
  Mine: Page<MyStar>
  Publish: StarWrite
  Edit: StarWrite
  Reply: {id: string; reviewState: ReviewState; bodyVersion: string}
  Like: {liked: boolean; likeCount: number}
  Withdraw: {starId: string; withdrawn: boolean}
  Seen: {ok: boolean}
  Notices: Page<FeedbackNotice> & {unreadCount: number}
  ReadNotice: {ok: boolean}
}

type Operation = keyof FeedbackResponses
type Request<K extends Operation> = Parameters<typeof feedbackBindings[K]>[0]
type NativeResponse<K extends Operation> = Awaited<ReturnType<typeof feedbackBindings[K]>>
type FeedbackBridge = Partial<Record<Operation, unknown>>

export class FeedbackError extends Error {
  constructor(readonly reason: string, message: string) {
    super(message)
  }
}

const errorLabels: Record<string, string> = {
  unavailable: '这颗星已不可用。',
  versionChanged: '正文已更新。请阅读当前版本后再操作，草稿已保留。',
  editLocked: '正文已锁定，可以把当前草稿作为作者补充提交。',
  requestConflict: '这次提交与原请求不一致，请核对草稿后重新提交。',
  unsupported: '当前服务器暂未提供反馈星系。',
  timeout: '连接超时，请重试。已输入的内容会保留。',
  stale: '账号会话已切换。',
  canceled: '请求已取消。',
  unauthorized: '登录已过期，请重新登录。',
  passwordRequired: '请先在账号窗口修改密码。',
}

function normalizeError(error: unknown): FeedbackError {
  const messageField = error && typeof error === 'object' && 'message' in error ? error.message : ''
  const raw = typeof error === 'string' ? error : typeof messageField === 'string' ? messageField : ''
  let payload: {reason?: string; message?: string} = {}
  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === 'object') payload = parsed
  } catch {
    // Wails 普通异常统一显示连接失败，避免将底层诊断内容带进业务界面。
  }
  const reason = typeof payload.reason === 'string' ? payload.reason : 'requestFailed'
  const message = typeof payload.message === 'string' ? payload.message : ''
  return new FeedbackError(reason, errorLabels[reason] || message || '连接反馈星系失败，请重试。')
}

// =============== 桌面传输 ===============
// 说明：固定在调用开始时的已验证会话，收回旧会话回执。读取和写入均交给 Wails。
export async function feedbackCall<K extends Operation>(
  operation: K,
  input: Request<K>,
): Promise<FeedbackResponses[K]> {
  const sessionKey = accountSessionKey.value
  if (!sessionKey) throw new FeedbackError('stale', errorLabels.stale)
  try {
    const bridge = (window as Window & {go?: {feedback?: {FeedbackService?: FeedbackBridge}}}).go?.feedback?.FeedbackService
    if (typeof bridge?.[operation] !== 'function') throw new FeedbackError('unsupported', errorLabels.unsupported)
    const invoke = feedbackBindings[operation] as (request: Request<K>) => Promise<NativeResponse<K>>
    const result = await invoke({...input, sessionKey})
    if (sessionKey !== accountSessionKey.value) throw new FeedbackError('stale', errorLabels.stale)
    // 生成绑定把审核与处理的契约枚举声明为 string，并将可空列表声明为数组。
    return result as NativeResponse<K> & FeedbackResponses[K]
  } catch (error) {
    if (sessionKey !== accountSessionKey.value) throw new FeedbackError('stale', errorLabels.stale)
    const failure = error instanceof FeedbackError ? error : normalizeError(error)
    if (failure.reason === 'unauthorized' || failure.reason === 'passwordRequired' || failure.reason === 'stale') {
      resetAccountSession()
    }
    throw failure
  }
}
