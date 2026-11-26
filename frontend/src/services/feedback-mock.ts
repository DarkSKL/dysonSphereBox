import {DISK_RADIUS, GALAXY_EDGE} from '../composables/feedback/galaxy-camera'
import {randomFor} from '../composables/feedback/galaxy-nebula'
import type {
  Bounds,
  Entry,
  FeedbackNotice,
  MapSnapshot,
  MyStar,
  Page,
  ReviewState,
  StarDetail,
  StarPoint,
  StarProgress,
  StarPreview,
  StarWrite,
} from '../types/feedback'

interface MockStarRecord {
  starId: string
  x: number
  y: number
  z: number
  progress: StarProgress
  brightness: number
  releaseVersion?: string
  likeCount: number
  liked: boolean
  owned: boolean
  canEdit: boolean
  withdrawn: boolean
  reviewReason?: string
  original: Entry
  officialReplies: Entry[]
  supplements: Entry[]
  comments: Entry[]
}

const AUTHORS = [
  '戴森总工', '伊卡洛斯09', '星际物流员', '紫糖流水线', '光子接收站',
  '黑雾清道夫', '曲速巡航者', '蓝图强迫症', '星云观测员', '分拣器魔术师',
  '绿糖爆仓中', '戴森壳建筑师', '硅矿掘进队', '太阳帆发射员', '临界光子',
  '小星云', '赤道环建队', '匿名', '星系漫游者', '重氢分馏塔',
]

const TOPICS = [
  '蓝图预览支持按建筑类型高亮筛选，复杂产线一眼定位关键节点。',
  '模组市场希望增加按兼容游戏版本过滤，避免装到旧版本依赖导致启动报错。',
  '联机服务器控制台增加在线玩家延迟与存档自动备份倒计时展示。',
  '产线计算器导出蓝图时支持记住上次选择的物流塔预设与增产剂等级。',
  '翻译管理完成批量校验后，希望给出更简短直观的完成状态提示。',
  '环境切换前自动检测是否有正在运行的无头服务端进程并给出提醒。',
  '日志查看器希望支持按错误等级快速折叠，排查 BepInEx 报错更省时间。',
  '本地存档卡片悬停时显示星系种子、游戏时长与最近一次备份时间。',
  '模组批量更新时增加一键跳过已锁定版本模组的开关，方便保留魔改配置。',
  '戴森球蓝图解析支持统计总耗电峰值与所需太阳帆发射井数量。',
  '联机房间列表增加按延迟排序与常用服务器收藏置顶功能。',
  '设置页增加一键导出诊断包，方便向开发组反馈启动异常和依赖冲突。',
  '资源管理支持按文件夹拖拽整理蓝图与存档，并同步显示占用空间。',
  '希望在星系反馈里查看已实现建议对应的具体更新日志与版本号。',
  '无头开服配置页支持保存多套开服预设，朋友联机时一键切换存档。',
  '日间水墨模式下的蓝图图标对比度很舒服，希望统计图表也同步优化配色。',
  '安装前置依赖遇到文件冲突时，希望能预览双方文件版本号再决定覆盖。',
  '游戏启动前自动检查 BepInEx 完整性，缺失关键组件时引导一键修复。',
]

const OFFICIAL_REPLIES = [
  '感谢建议！该能力已排入下一阶段开发计划，会优先覆盖最常用的桌面操作路径。',
  '已在新版本中实装并完成联机与本地场景回归验证，欢迎更新后体验反馈。',
  '收到，这条体验细节非常有价值，我们正在结合现有界面规范做一轮整体梳理。',
]

const COMMENTS = [
  '非常需要这个功能，开联机服的时候每次都要手动翻目录太麻烦了。',
  '附议！特别是蓝图一多之后筛选和分组简直是刚需。',
  '赞同，希望保持现在这种干净克制的界面风格不要太复杂。',
  '实测确实有这个痛点，顶上去让开发组看到！',
  '如果能顺便支持快捷键操作就更顺手了。',
]

// =============== 按服务端同款旋臂算法生成 200 条模拟星点 ===============
// 说明：坐标分布与 server/service/feedback/coordinates.go 完全同构（20% 核球、76% 双对数旋臂、4% 盘面），
//       同时包含少量当前账号持有的已通过星与 2 颗待审闪烁星，方便一次性预览完整星云与卡片交互。
function createMockStore(totalCount: number) {
  const rand = randomFor(`feedback-mock-galaxy:${totalCount}`)
  const records = new Map<string, MockStarRecord>()
  const notices: FeedbackNotice[] = []
  let versionCounter = 1
  let nextId = 2000

  const placeOnGalaxy = (): {x: number; y: number; z: number} => {
    const population = rand()
    const radialSample = rand()
    let angle = Math.PI * 2 * rand()
    let r: number
    if (population < 0.2) {
      r = 0.15 * Math.pow(radialSample, 0.7)
    } else if (population < 0.96) {
      r = 0.08 + 0.92 * Math.pow(radialSample, 0.8)
      const arm = rand() < 0.5 ? 0 : 1
      const spread = 0.38 * (rand() - rand())
      angle = -0.65 + 3.15 * Math.log1p(5 * r) + arm * Math.PI + spread
    } else {
      r = Math.sqrt(radialSample)
    }
    const radius = DISK_RADIUS * r
    const x = Math.trunc(radius * Math.cos(angle))
    const y = Math.trunc(radius * Math.sin(angle))
    const rNorm = Math.min(Math.hypot(x, y) / DISK_RADIUS, 1)
    const halfThickness = 1800 + 8200 * Math.exp(-24 * rNorm * rNorm)
    const z = Math.round((rand() - rand()) * halfThickness)
    return {x, y, z}
  }

  const baseTime = Date.now() - 1000 * 60 * 60 * 24 * 35

  for (let i = 1; i <= totalCount; i++) {
    const starId = String(1000 + i)
    const {x, y, z} = placeOnGalaxy()
    const roll = rand()
    const progress: StarProgress = roll < 0.7 ? 'feedback' : roll < 0.88 ? 'accepted' : 'implemented'
    const likeCount = Math.floor(Math.pow(rand(), 2.2) * 86)
    const brightness = Math.min(1, 0.1 + Math.pow(rand(), 1.55) * 0.9)
    const owned = i <= 6
    const reviewState: ReviewState = i === 1 || i === 2 ? 'pending' : 'approved'
    const topic = TOPICS[(i - 1) % TOPICS.length]
    const author = owned ? '戴森工程师' : AUTHORS[Math.floor(rand() * AUTHORS.length)]
    const createdAt = new Date(baseTime + i * 1000 * 60 * 195).toISOString()
    const body =
      `【模拟反馈 ${String(i).padStart(3, '0')}】${topic}\n` +
      (i % 4 === 0 ? '补充说明：在长时间运行或切换日夜主题后也希望保持视觉与交互流畅。' : '')

    const officialReplies: Entry[] =
      progress !== 'feedback' && reviewState === 'approved'
        ? [
            {
              id: `${starId}-official-1`,
              kind: 'official',
              body: OFFICIAL_REPLIES[i % OFFICIAL_REPLIES.length],
              displayName: '开发组',
              createdAt: new Date(baseTime + (i + 3) * 1000 * 60 * 195).toISOString(),
              reviewState: 'approved',
              bodyVersion: '1',
              reviewVersion: '2',
            },
          ]
        : []

    const supplements: Entry[] =
      i % 9 === 0 && reviewState === 'approved'
        ? [
            {
              id: `${starId}-sup-1`,
              kind: 'supplement',
              body: '再补充一点：窄窗口下也希望操作按钮排布紧凑清晰，不需要横向滚动。',
              displayName: author === '匿名' ? '' : author,
              createdAt: new Date(baseTime + (i + 2) * 1000 * 60 * 195).toISOString(),
              reviewState: 'approved',
              bodyVersion: '1',
              reviewVersion: '2',
            },
          ]
        : []

    const commentCount = reviewState === 'approved' && i % 3 === 0 ? 1 + (i % 3) : 0
    const comments: Entry[] = Array.from({length: commentCount}, (_, idx) => ({
      id: `${starId}-c-${idx + 1}`,
      kind: 'comment',
      body: COMMENTS[(i + idx) % COMMENTS.length],
      displayName: AUTHORS[(i * 3 + idx) % AUTHORS.length],
      createdAt: new Date(baseTime + (i + 4 + idx) * 1000 * 60 * 195).toISOString(),
      reviewState: 'approved',
      bodyVersion: '1',
      reviewVersion: String(3 + idx),
    }))

    records.set(starId, {
      starId,
      x,
      y,
      z,
      progress,
      brightness,
      releaseVersion: progress === 'implemented' ? `1.${4 + (i % 3)}.${i % 8}` : undefined,
      likeCount,
      liked: i % 11 === 0,
      owned,
      canEdit: owned && reviewState === 'pending',
      withdrawn: false,
      original: {
        id: `${starId}-orig`,
        kind: 'star',
        body,
        displayName: author === '匿名' ? '' : author,
        createdAt,
        reviewState,
        bodyVersion: '1',
        reviewVersion: '1',
      },
      officialReplies,
      supplements,
      comments,
    })

    if (owned && progress !== 'feedback' && reviewState === 'approved') {
      notices.push({
        id: `notice-${starId}`,
        kind: progress === 'implemented' ? 'implemented' : 'accepted',
        starId,
        contentId: officialReplies[0]?.id ?? `${starId}-orig`,
        createdAt: officialReplies[0]?.createdAt ?? createdAt,
        readAt: null,
      })
    }
  }

  function currentVersion(): string {
    return `mock-v${versionCounter}`
  }

  function previewOf(record: MockStarRecord): StarPreview {
    return {
      starId: record.starId,
      preview: [...record.original.body.split('\n', 1)[0]].slice(0, 120).join(''),
      displayName: record.original.displayName || undefined,
    }
  }

  function map(input: Partial<Bounds> & {version?: string}): MapSnapshot {
    const version = currentVersion()
    const hasBounds =
      typeof input.minX === 'number' &&
      typeof input.minY === 'number' &&
      typeof input.maxX === 'number' &&
      typeof input.maxY === 'number'
    const bounds: Bounds = hasBounds
      ? {minX: input.minX!, minY: input.minY!, maxX: input.maxX!, maxY: input.maxY!}
      : {minX: -GALAXY_EDGE, minY: -GALAXY_EDGE, maxX: GALAXY_EDGE, maxY: GALAXY_EDGE}

    if (input.version && input.version === version) {
      return {version, bounds, unchanged: true, points: null}
    }

    const points: StarPoint[] = []
    for (const record of records.values()) {
      if (record.withdrawn || record.original.reviewState !== 'approved') continue
      if (hasBounds && (record.x < bounds.minX || record.x > bounds.maxX || record.y < bounds.minY || record.y > bounds.maxY)) {
        continue
      }
      points.push({
        starId: record.starId,
        x: record.x,
        y: record.y,
        z: record.z,
        progress: record.progress,
        brightness: record.brightness,
        previewVersion: record.original.reviewVersion,
      })
    }
    return {version, bounds, unchanged: false, points}
  }

  function detail(starId: string, page = 1, pageSize = 20): StarDetail | null {
    const record = records.get(starId)
    if (!record || record.withdrawn) return null
    const allReplies = [...record.officialReplies, ...record.supplements, ...record.comments]
    const start = Math.max(0, (page - 1) * pageSize)
    return {
      starId: record.starId,
      x: record.x,
      y: record.y,
      z: record.z,
      original: record.original,
      progress: record.progress,
      releaseVersion: record.releaseVersion,
      likeCount: record.likeCount,
      liked: record.liked,
      owned: record.owned,
      canEdit: record.canEdit,
      withdrawn: record.withdrawn,
      reviewReason: record.reviewReason,
      officialReplies: record.officialReplies,
      supplements: record.supplements,
      replies: {
        page,
        pageSize,
        total: allReplies.length,
        list: allReplies.slice(start, start + pageSize),
      },
    }
  }

  function mine(page = 1, pageSize = 12): Page<MyStar> {
    const list: MyStar[] = []
    for (const record of records.values()) {
      if (!record.owned) continue
      list.push({
        starId: record.starId,
        x: record.x,
        y: record.y,
        z: record.z,
        preview: previewOf(record).preview,
        displayName: record.original.displayName || undefined,
        reviewState: record.original.reviewState,
        bodyVersion: record.original.bodyVersion,
        reviewReason: record.reviewReason,
        progress: record.progress,
        withdrawn: record.withdrawn,
        newComment: false,
        createdAt: record.original.createdAt,
      })
    }
    const start = Math.max(0, (page - 1) * pageSize)
    return {page, pageSize, total: list.length, list: list.slice(start, start + pageSize)}
  }

  function listNotices(page = 1, pageSize = 12): Page<FeedbackNotice> & {unreadCount: number} {
    const start = Math.max(0, (page - 1) * pageSize)
    return {
      page,
      pageSize,
      total: notices.length,
      list: notices.slice(start, start + pageSize),
      unreadCount: notices.filter((item) => !item.readAt).length,
    }
  }

  function publish(body: string, showName: boolean): StarWrite {
    const starId = String(++nextId)
    const {x, y, z} = placeOnGalaxy()
    const createdAt = new Date().toISOString()
    records.set(starId, {
      starId,
      x,
      y,
      z,
      progress: 'feedback',
      brightness: 0.55,
      likeCount: 0,
      liked: false,
      owned: true,
      canEdit: true,
      withdrawn: false,
      original: {
        id: `${starId}-orig`,
        kind: 'star',
        body,
        displayName: showName ? '戴森工程师' : '',
        createdAt,
        reviewState: 'pending',
        bodyVersion: '1',
        reviewVersion: '1',
      },
      officialReplies: [],
      supplements: [],
      comments: [],
    })
    versionCounter++
    return {starId, x, y, z, reviewState: 'pending', bodyVersion: '1', withdrawn: false}
  }

  function edit(starId: string, body: string): StarWrite | null {
    const record = records.get(starId)
    if (!record || record.withdrawn) return null
    const nextVersion = String(Number(record.original.bodyVersion) + 1)
    record.original = {...record.original, body, bodyVersion: nextVersion}
    versionCounter++
    return {
      starId: record.starId,
      x: record.x,
      y: record.y,
      z: record.z,
      reviewState: record.original.reviewState,
      bodyVersion: nextVersion,
      withdrawn: false,
    }
  }

  function reply(starId: string, body: string, kind: 'comment' | 'supplement'): {id: string; reviewState: ReviewState; bodyVersion: string} | null {
    const record = records.get(starId)
    if (!record || record.withdrawn) return null
    const id = `${starId}-${kind}-${Date.now()}`
    const entry: Entry = {
      id,
      kind,
      body,
      displayName: '戴森工程师',
      createdAt: new Date().toISOString(),
      reviewState: 'approved',
      bodyVersion: '1',
      reviewVersion: String(Date.now()),
    }
    if (kind === 'supplement') record.supplements = [...record.supplements, entry]
    else record.comments = [...record.comments, entry]
    return {id, reviewState: 'approved', bodyVersion: '1'}
  }

  function like(starId: string, liked: boolean): {liked: boolean; likeCount: number} | null {
    const record = records.get(starId)
    if (!record || record.withdrawn) return null
    if (record.liked !== liked) {
      record.liked = liked
      record.likeCount = Math.max(0, record.likeCount + (liked ? 1 : -1))
      record.brightness = Math.min(1, record.brightness + (liked ? 0.04 : -0.04))
      versionCounter++
    }
    return {liked: record.liked, likeCount: record.likeCount}
  }

  function withdraw(starId: string): {starId: string; withdrawn: boolean} | null {
    const record = records.get(starId)
    if (!record) return null
    record.withdrawn = true
    versionCounter++
    return {starId, withdrawn: true}
  }

  function readNotice(id: string): {ok: boolean} {
    const target = notices.find((item) => item.id === id)
    if (target && !target.readAt) target.readAt = new Date().toISOString()
    return {ok: true}
  }

  return {
    map,
    preview: (starId: string) => {
      const record = records.get(starId)
      return record && !record.withdrawn ? previewOf(record) : null
    },
    previews: (starIds: readonly string[]) => ({
      items: starIds
        .map((id) => records.get(id))
        .filter((record): record is MockStarRecord => Boolean(record && !record.withdrawn && record.original.reviewState === 'approved'))
        .map(previewOf),
    }),
    detail,
    mine,
    notices: listNotices,
    publish,
    edit,
    reply,
    like,
    withdraw,
    readNotice,
  }
}

let cachedStore: ReturnType<typeof createMockStore> | null = null
let cachedCount = 0

// =============== 前端本地模拟桥接 ===============
// 说明：用于演示与视觉验收的本地 200 条星云数据模拟器，不向后端服务器写入任何脏数据。
// 边界：由 services/feedback.ts 按需启用；关闭模拟开关后所有调用立即恢复真实 Wails 后端服务。
export function mockFeedbackCall(count: number, operation: string, input: Record<string, unknown>): unknown {
  if (!cachedStore || cachedCount !== count) {
    cachedStore = createMockStore(count)
    cachedCount = count
  }
  switch (operation) {
    case 'Map':
      return cachedStore.map(input as Partial<Bounds> & {version?: string})
    case 'Preview': {
      const item = cachedStore.preview(String(input.starId ?? ''))
      if (!item) throw new Error('unavailable')
      return item
    }
    case 'Previews':
      return cachedStore.previews(Array.isArray(input.starIds) ? input.starIds.map(String) : [])
    case 'Detail': {
      const result = cachedStore.detail(
        String(input.starId ?? ''),
        typeof input.page === 'number' ? input.page : 1,
        typeof input.pageSize === 'number' ? input.pageSize : 20,
      )
      if (!result) throw new Error('unavailable')
      return result
    }
    case 'Mine':
      return cachedStore.mine(
        typeof input.page === 'number' ? input.page : 1,
        typeof input.pageSize === 'number' ? input.pageSize : 12,
      )
    case 'Notices':
      return cachedStore.notices(
        typeof input.page === 'number' ? input.page : 1,
        typeof input.pageSize === 'number' ? input.pageSize : 12,
      )
    case 'Publish':
      return cachedStore.publish(String(input.body ?? ''), Boolean(input.showName))
    case 'Edit': {
      const result = cachedStore.edit(String(input.starId ?? ''), String(input.body ?? ''))
      if (!result) throw new Error('unavailable')
      return result
    }
    case 'Reply': {
      const kind = input.kind === 'supplement' ? 'supplement' : 'comment'
      const result = cachedStore.reply(String(input.starId ?? ''), String(input.body ?? ''), kind)
      if (!result) throw new Error('unavailable')
      return result
    }
    case 'Like': {
      const result = cachedStore.like(String(input.starId ?? ''), Boolean(input.liked))
      if (!result) throw new Error('unavailable')
      return result
    }
    case 'Withdraw': {
      const result = cachedStore.withdraw(String(input.starId ?? ''))
      if (!result) throw new Error('unavailable')
      return result
    }
    case 'Seen':
      return {ok: true}
    case 'ReadNotice':
      return cachedStore.readNotice(String(input.id ?? ''))
    default:
      throw new Error('unsupported')
  }
}
