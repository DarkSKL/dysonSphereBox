// =============== 社区论坛类型定义 ===============
// 说明：集中声明论坛分类、帖子、回复、资源块、草稿绑定与举报参数，严格对齐 forum-api.md 与 Wails 绑定。
// 边界：服务端返回的动态分类与资源状态不做硬编码假设。

export type ForumBodyFormat = 'plain' | 'markdown'

export type ForumResourceType = 'blueprint' | 'save' | 'mod' | 'image' | 'video'

export type ForumResourceSource = 'upload' | 'inline' | 'thunderstore'

export type ForumBlockState = 'visible' | 'takenDown' | 'unavailable'

export type ForumReportTargetType = 'post' | 'reply' | 'resource'

export interface ForumCategory {
  id: number
  code: string
  name: string
  sort: number
  enabled: boolean
}

export interface ForumPostSummary {
  id: number
  category: ForumCategory
  title: string
  authorName: string
  createdAt: string
  replyCount: number
}

export interface ForumBodyBlock {
  type: 'markdown' | 'resource' | string
  text?: string
  resourceId?: number
  state?: ForumBlockState | string
}

export interface ForumResource {
  id: number
  type: ForumResourceType | string
  source: ForumResourceSource | string
  name: string
  blueprintCode?: string
  size?: number
  sha256?: string
  namespace?: string
  packageName?: string
  version?: string
  url?: string
  contentType?: string
}

export interface ForumPostDetail extends ForumPostSummary {
  body: string
  bodyFormat: ForumBodyFormat | string
  bodyBlocks?: ForumBodyBlock[]
  resources: ForumResource[]
}

export interface ForumReply {
  id: number
  postId: number
  authorName: string
  createdAt: string
  body: string
  bodyFormat: ForumBodyFormat | string
  bodyBlocks?: ForumBodyBlock[]
  resources: ForumResource[]
}

export interface ForumPage<T> {
  page: number
  pageSize: number
  total: number
  list: T[]
}

export interface ForumSelectedLocalFile {
  path: string
  name: string
  size: number
  ext: string
  contentType: string
  previewUrl?: string
  previewToken?: string
}

export interface ForumBlueprintDraft {
  mode: 'file' | 'inline'
  name: string
  file?: ForumSelectedLocalFile
  blueprintCode?: string
}

export interface ForumModDraft {
  mode: 'file' | 'thunderstore'
  label: string
  file?: ForumSelectedLocalFile
  namespace?: string
  packageName?: string
  version?: string
  url?: string
}

export interface ForumDraftBindings {
  /** key 为 image1..image8 */
  images: Record<string, ForumSelectedLocalFile>
  video: ForumSelectedLocalFile | null
  blueprint: ForumBlueprintDraft | null
  mod: ForumModDraft | null
  save: ForumSelectedLocalFile | null
}

/** 从论坛模组卡跳转到模组市场时的定位目标 */
export interface PendingForumModTarget {
  namespace: string
  packageName: string
  version?: string
  url?: string
}

export interface ForumReportTarget {
  targetType: ForumReportTargetType
  targetId: number
  title: string
}
