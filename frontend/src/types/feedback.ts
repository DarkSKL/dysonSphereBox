/** 星图、详情与作者收件箱共享的首版业务字段。标识与版本全程保留字符串。 */
export type StarProgress = 'feedback' | 'accepted' | 'implemented'
export type ReviewState = 'pending' | 'manual' | 'approved' | 'rejected'
export type EntryKind = 'star' | 'comment' | 'supplement' | 'official'
export type NoticeKind = 'rejected' | 'official' | 'accepted' | 'implemented'

export interface Bounds {
  minX: number
  minY: number
  maxX: number
  maxY: number
}

export interface Coordinates {
  x: number
  y: number
}

/** 星图世界坐标。z 为沿盘面法向的高度，缺省时按 0 投影。 */
export interface GalaxyPosition extends Coordinates {
  z?: number
}

export interface StarPoint extends GalaxyPosition {
  starId: string
  previewVersion?: string
  progress: StarProgress
  brightness: number
  count?: number
}

/** 可见星的首行与服务端署名。旧响应省略署名。 */
export interface StarPreview {
  starId: string
  preview: string
  displayName?: string
}

/** 当前账号可见的反馈星；作者待审的星同时携带审核状态。 */
export interface DisplayStar extends StarPoint {
  privateLabel?: string
}

export interface MapSnapshot {
  version: string
  bounds: Bounds
  unchanged: boolean
  points?: StarPoint[] | null
}

export interface Page<T> {
  page: number
  pageSize: number
  total: number
  list?: T[] | null
}

export interface PageRequest {
  page?: number
  pageSize?: number
}

export interface Entry {
  id: string
  kind: EntryKind
  body: string
  displayName: string
  avatar?: string
  headerImg?: string
  createdAt: string
  reviewState: ReviewState
  bodyVersion: string
  reviewVersion: string
  reviewReason?: string
}

export interface StarDetail extends GalaxyPosition {
  starId: string
  original: Entry
  progress: StarProgress
  releaseVersion?: string
  likeCount: number
  liked: boolean
  owned: boolean
  canEdit: boolean
  withdrawn: boolean
  reviewReason?: string
  replies: Page<Entry>
  officialReplies?: Entry[] | null
  supplements?: Entry[] | null
}

export interface MyStar extends GalaxyPosition, StarPreview {
  reviewState: ReviewState
  bodyVersion: string
  reviewReason?: string
  progress: StarProgress
  withdrawn: boolean
  newComment: boolean
  createdAt: string
}

export interface FeedbackNotice {
  id: string
  kind: NoticeKind
  starId: string
  contentId: string
  createdAt: string
  readAt?: string | null
}

export interface StarWrite extends GalaxyPosition {
  starId: string
  reviewState: ReviewState
  bodyVersion: string
  withdrawn: boolean
}

export const progressLabels: Record<StarProgress, string> = {
  feedback: '玩家反馈',
  accepted: '已采纳',
  implemented: '已实现',
}

export const reviewLabels: Record<ReviewState, string> = {
  pending: '审核中',
  manual: '待人工复核',
  approved: '已通过',
  rejected: '未通过',
}

export const noticeLabels: Record<NoticeKind, string> = {
  rejected: '内容审核退回',
  official: '收到开发组回应',
  accepted: '反馈已采纳',
  implemented: '反馈已实现',
}

export function formatFeedbackTime(value: string): string {
  return value ? new Date(value).toLocaleString('zh-CN', {dateStyle: 'medium', timeStyle: 'short'}) : ''
}
