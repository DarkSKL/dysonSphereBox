import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'
import type {JSONContent} from '@tiptap/core'
import type {
  ForumBodyBlock,
  ForumBlockState,
  ForumDraftBindings,
  ForumResource,
} from '../types/forum'

// =============== 社区论坛 Markdown 解析与转换管线 ===============
// 说明：
// 1. 阅读与预览均将正文作为「一篇完整 Markdown 文档」解析，因此引用式链接 [文本][ref] 与定义 [ref]: url 可以隔着 [[resource:...]] 正常工作；
// 2. 通过自定义 markdown-it 块级规则识别独立行 [[resource:key]]，位于缩进代码块与围栏代码块中的标签自动保持普通文本；
// 3. 禁用原始 HTML 与外部图片直连加载，仅允许安全的文本与链接；
// 4. 提供 TipTap 所见即所得文档与 CommonMark + [[resource:key]] 的双向转换。

const RESOURCE_LINE_PATTERN = /^\[\[resource:([A-Za-z0-9_-]+)\]\]$/

function createForumMarkdownParser() {
  const md = new MarkdownIt({
    html: false,
    linkify: true,
    breaks: true,
  })

  // 在 html_block 之前（code 和 fence 之后）注册独立行 [[resource:key]] 块级规则：
  // 与服务端 bodyReferences 保持完全一致：仅顶层零缩进独立行匹配，缩进/围栏代码块与嵌套块中的标签保留为纯文本。
  md.block.ruler.before('html_block', 'forum_resource', (state, startLine, _endLine, silent) => {
    if (state.parentType !== 'root' || state.tShift[startLine] !== 0 || state.sCount[startLine] !== 0) {
      return false
    }
    const line = state.src.slice(state.bMarks[startLine], state.eMarks[startLine]).replace(/\r$/, '')
    const match = RESOURCE_LINE_PATTERN.exec(line)
    if (!match) {
      return false
    }
    if (silent) {
      return true
    }
    const token = state.push('forum_resource_slot', '', 0)
    token.block = true
    token.map = [startLine, startLine + 1]
    token.markup = line
    token.meta = {key: match[1]}
    state.line = startLine + 1
    return true
  }, {alt: ['paragraph', 'reference', 'blockquote', 'list']})

  // 禁止直接加载外部图片：将标准 Markdown 图片退化为纯文本说明，防止 WebView 发起未受控外链图片请求。
  md.renderer.rules.image = (tokens, idx) => {
    const token = tokens[idx]
    const alt = String(token.content ?? '').trim()
    const src = String(token.attrGet('src') ?? '').trim()
    const text = alt ? `[外部图片：${alt}]` : src ? '[已拦截外部图片]' : '[图片]'
    return md.utils.escapeHtml(text)
  }

  // 安全渲染链接：仅允许 http/https，并标注 data-external-link 交由宿主桌面浏览器打开。
  const defaultLinkOpen = md.renderer.rules.link_open
  md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
    const token = tokens[idx]
    const href = String(token.attrGet('href') ?? '').trim()
    if (!/^https?:\/\//i.test(href)) {
      token.attrSet('href', '#')
    } else {
      token.attrSet('target', '_blank')
      token.attrSet('rel', 'noopener noreferrer')
      token.attrSet('data-forum-link', href)
    }
    return defaultLinkOpen
      ? defaultLinkOpen(tokens, idx, options, env, self)
      : self.renderToken(tokens, idx, options)
  }

  // 默认把未消费的槽位令牌渲染成安全纯文本，防止泄露空节点。
  md.renderer.rules.forum_resource_slot = (tokens, idx) => {
    const markup = tokens[idx].markup || `[[resource:${tokens[idx].meta?.key ?? ''}]]`
    return `<p>${md.utils.escapeHtml(markup)}</p>\n`
  }

  return md
}

const forumMd = createForumMarkdownParser()
type ForumMarkdownToken = ReturnType<typeof forumMd.parse>[number]

function sanitizeForumHtml(rawHtml: string): string {
  return DOMPurify.sanitize(rawHtml, {
    ALLOWED_TAGS: [
      'p', 'br', 'strong', 'em', 'del', 's', 'code', 'pre', 'blockquote',
      'ul', 'ol', 'li', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'hr', 'a',
      'table', 'thead', 'tbody', 'tr', 'th', 'td',
    ],
    ALLOWED_ATTR: ['href', 'title', 'target', 'rel', 'data-forum-link'],
    ALLOW_DATA_ATTR: false,
    ADD_ATTR: ['data-forum-link'],
  })
}

// =============== 阅读模式：全文解析并按 bodyBlocks 确认插入点 ===============

export type ForumReadingSegment =
  | {kind: 'html'; html: string}
  | {kind: 'resource'; resourceId: number; state: ForumBlockState; resource?: ForumResource}

export function buildForumReadingSegments(
  body: string,
  bodyBlocks: ForumBodyBlock[] | undefined,
  resources: ForumResource[] | undefined,
): ForumReadingSegment[] {
  const resourceMap = new Map<number, ForumResource>()
  for (const item of resources ?? []) {
    resourceMap.set(item.id, item)
  }

  const confirmedBlocks = (bodyBlocks ?? []).filter((block) => block.type === 'resource')
  const env: Record<string, unknown> = {}
  const tokens = forumMd.parse(body || '', env)

  const segments: ForumReadingSegment[] = []
  let pendingTokens: ForumMarkdownToken[] = []
  let confirmCursor = 0

  const flushPending = (): void => {
    if (!pendingTokens.length) return
    const rendered = sanitizeForumHtml(forumMd.renderer.render(pendingTokens, forumMd.options, env))
    if (rendered.trim()) {
      segments.push({kind: 'html', html: rendered})
    }
    pendingTokens = []
  }

  for (const token of tokens) {
    if (token.type === 'forum_resource_slot') {
      const slotKey = String(token.meta?.key ?? '')
      const parsedId = /^\d+$/.test(slotKey) ? Number(slotKey) : 0
      const expected = confirmedBlocks[confirmCursor]
      if (expected && parsedId > 0 && expected.resourceId === parsedId) {
        flushPending()
        const state = normalizeBlockState(expected.state)
        segments.push({
          kind: 'resource',
          resourceId: parsedId,
          state,
          resource: state === 'visible' ? resourceMap.get(parsedId) : undefined,
        })
        confirmCursor += 1
        continue
      }
    }
    pendingTokens.push(token)
  }
  flushPending()

  // 兜底：若服务端确认的 bodyBlocks 中仍有未匹配到的资源块（例如位于嵌套容器内），按序补齐展示，确保不丢失任何资源或下架提示。
  while (confirmCursor < confirmedBlocks.length) {
    const block = confirmedBlocks[confirmCursor]
    confirmCursor += 1
    const resourceId = block.resourceId ?? 0
    const state = normalizeBlockState(block.state)
    segments.push({
      kind: 'resource',
      resourceId,
      state,
      resource: state === 'visible' ? resourceMap.get(resourceId) : undefined,
    })
  }

  return segments
}

function normalizeBlockState(raw?: string): ForumBlockState {
  if (raw === 'takenDown' || raw === 'unavailable') return raw
  return 'visible'
}

// =============== 编辑实时预览：按完整文档解析并映射草稿绑定 ===============

export type ForumPreviewSegment =
  | {kind: 'html'; html: string}
  | {kind: 'slot'; slot: string; bound: boolean}

export function buildForumPreviewSegments(markdown: string, bindings: ForumDraftBindings): ForumPreviewSegment[] {
  const env: Record<string, unknown> = {}
  const tokens = forumMd.parse(markdown || '', env)
  const segments: ForumPreviewSegment[] = []
  let pendingTokens: ForumMarkdownToken[] = []

  const flushPending = (): void => {
    if (!pendingTokens.length) return
    const rendered = sanitizeForumHtml(forumMd.renderer.render(pendingTokens, forumMd.options, env))
    if (rendered.trim()) {
      segments.push({kind: 'html', html: rendered})
    }
    pendingTokens = []
  }

  for (const token of tokens) {
    if (token.type === 'forum_resource_slot') {
      flushPending()
      const slot = String(token.meta?.key ?? '')
      segments.push({
        kind: 'slot',
        slot,
        bound: isSlotBound(slot, bindings),
      })
      continue
    }
    pendingTokens.push(token)
  }
  flushPending()
  return segments
}

export function isSlotBound(slot: string, bindings: ForumDraftBindings): boolean {
  if (slot === 'video') return Boolean(bindings.video)
  if (slot === 'blueprint') return Boolean(bindings.blueprint)
  if (slot === 'mod') return Boolean(bindings.mod)
  if (slot === 'save') return Boolean(bindings.save)
  if (/^image[1-8]$/.test(slot)) return Boolean(bindings.images[slot])
  return false
}

// 说明：提取正文中实际生效的（非代码块内）资源标签键列表，用于发布前核对资源与正文标签的一致性。
export function extractReferencedResourceSlots(markdown: string): string[] {
  const env: Record<string, unknown> = {}
  const tokens = forumMd.parse(markdown || '', env)
  const result: string[] = []
  const walk = (list: ForumMarkdownToken[]): void => {
    for (const token of list) {
      if (token.type === 'forum_resource_slot') {
        const key = String(token.meta?.key ?? '')
        if (key) result.push(key)
      }
      if (token.children?.length) {
        walk(token.children)
      }
    }
  }
  walk(tokens)
  return result
}

// =============== TipTap 所见即所得与 Markdown 双向转换 ===============

// 说明：将 Markdown 转为供 TipTap 加载的 HTML；独立行 [[resource:key]] 转为自定义块节点占位。
export function markdownToTiptapHtml(markdown: string): string {
  const trimmed = (markdown || '').replace(/\r\n/g, '\n')
  if (!trimmed.trim()) return ''

  const env: Record<string, unknown> = {}
  const tokens = forumMd.parse(trimmed, env)
  const chunks: string[] = []
  let pending: ForumMarkdownToken[] = []

  const flush = (): void => {
    if (!pending.length) return
    chunks.push(sanitizeForumHtml(forumMd.renderer.render(pending, forumMd.options, env)))
    pending = []
  }

  for (const token of tokens) {
    if (token.type === 'forum_resource_slot') {
      flush()
      const slot = String(token.meta?.key ?? '').replace(/[^A-Za-z0-9_-]/g, '')
      if (slot) {
        chunks.push(`<div data-forum-resource="" data-slot="${slot}"></div>`)
      }
      continue
    }
    pending.push(token)
  }
  flush()
  return chunks.join('\n')
}

// 说明：将 TipTap ProseMirror JSON 文档无损序列化为 CommonMark Markdown；forumResource 节点输出独立行 [[resource:slot]]。
export function tiptapDocToMarkdown(doc: JSONContent | null | undefined): string {
  if (!doc || !Array.isArray(doc.content)) return ''
  const blocks: string[] = []
  for (const node of doc.content) {
    const serialized = serializeBlockNode(node, 0)
    if (serialized !== null) {
      blocks.push(serialized)
    }
  }
  return blocks.join('\n\n').replace(/\n{3,}/g, '\n\n').trimEnd()
}

function serializeBlockNode(node: JSONContent, listDepth: number): string | null {
  switch (node.type) {
    case 'forumResource': {
      const slot = String(node.attrs?.slot ?? '').trim()
      return slot ? `[[resource:${slot}]]` : null
    }
    case 'paragraph': {
      const inline = serializeInlineNodes(node.content)
      return inline
    }
    case 'heading': {
      const level = Math.min(6, Math.max(1, Number(node.attrs?.level ?? 2)))
      const inline = serializeInlineNodes(node.content)
      return `${'#'.repeat(level)} ${inline}`.trimEnd()
    }
    case 'blockquote': {
      const inner = (node.content ?? [])
        .map((child) => serializeBlockNode(child, listDepth))
        .filter((part): part is string => part !== null)
        .join('\n\n')
      return inner
        .split('\n')
        .map((line) => (line ? `> ${line}` : '>'))
        .join('\n')
    }
    case 'bulletList': {
      const indent = '  '.repeat(listDepth)
      return (node.content ?? [])
        .map((item) => serializeListItem(item, `${indent}- `, `${indent}  `, listDepth + 1))
        .join('\n')
    }
    case 'orderedList': {
      const indent = '  '.repeat(listDepth)
      const start = Number(node.attrs?.start ?? 1)
      return (node.content ?? [])
        .map((item, idx) => {
          const prefix = `${indent}${start + idx}. `
          const cont = `${indent}${' '.repeat(`${start + idx}. `.length)}`
          return serializeListItem(item, prefix, cont, listDepth + 1)
        })
        .join('\n')
    }
    case 'codeBlock': {
      const lang = String(node.attrs?.language ?? '').trim()
      const text = (node.content ?? []).map((child) => child.text ?? '').join('')
      let maxRun = 2
      for (const match of text.matchAll(/`+/g)) {
        if (match[0].length > maxRun) maxRun = match[0].length
      }
      const fence = '`'.repeat(maxRun + 1)
      return `${fence}${lang}\n${text}\n${fence}`
    }
    case 'horizontalRule':
      return '---'
    default: {
      if (node.content?.length) {
        return serializeInlineNodes(node.content)
      }
      return null
    }
  }
}

function serializeListItem(item: JSONContent, firstPrefix: string, contPrefix: string, nextDepth: number): string {
  const parts = (item.content ?? [])
    .map((child) => serializeBlockNode(child, nextDepth))
    .filter((part): part is string => part !== null)
  if (!parts.length) return firstPrefix.trimEnd()
  const lines = parts.join('\n').split('\n')
  return lines
    .map((line, idx) => (idx === 0 ? `${firstPrefix}${line}` : `${contPrefix}${line}`))
    .join('\n')
}

function serializeInlineNodes(nodes: JSONContent[] | undefined): string {
  if (!nodes?.length) return ''
  return nodes.map((node) => {
    if (node.type === 'hardBreak') return '  \n'
    if (node.type !== 'text') return ''
    let text = node.text ?? ''
    const marks = node.marks ?? []
    const hasCode = marks.some((mark) => mark.type === 'code')

    if (hasCode) {
      let maxRun = 0
      for (const match of text.matchAll(/`+/g)) {
        if (match[0].length > maxRun) maxRun = match[0].length
      }
      const ticks = '`'.repeat(maxRun + 1)
      const needsPad = text.startsWith('`') || text.endsWith('`') || (text.startsWith(' ') && text.endsWith(' ') && text.trim().length > 0)
      text = needsPad ? `${ticks} ${text} ${ticks}` : `${ticks}${text}${ticks}`
    } else {
      for (const mark of marks) {
        if (mark.type === 'bold') text = `**${text}**`
        else if (mark.type === 'italic') text = `*${text}*`
        else if (mark.type === 'strike') text = `~~${text}~~`
      }
    }

    const linkMark = marks.find((mark) => mark.type === 'link')
    if (linkMark && linkMark.attrs?.href) {
      const href = String(linkMark.attrs.href).trim()
      if (/^https?:\/\//i.test(href)) {
        text = `[${text}](${href})`
      }
    }
    return text
  }).join('')
}

// =============== Thunderstore 链接解析辅助 ===============
// 说明：与服务端 thunderstore.go 保持完全一致：接受 https://thunderstore.io 的包页、/v/{version}/ 版本页和官方下载链接。
const MOD_ID_PATTERN = /^[a-zA-Z0-9_]{1,120}$/
const MOD_VERSION_PATTERN = /^[0-9]+\.[0-9]+\.[0-9]+$/

export function isValidThunderstoreVersion(version: string): boolean {
  return version.length <= 120 && MOD_VERSION_PATTERN.test(version)
}

export function buildThunderstorePackageUrl(namespace: string, packageName: string, version?: string): string {
  const base = `https://thunderstore.io/c/dyson-sphere-program/p/${namespace}/${packageName}/`
  return version ? `${base}v/${version}/` : base
}

export function parseThunderstoreUrl(raw: string): {
  valid: boolean
  url: string
  namespace?: string
  packageName?: string
  version?: string
} {
  const trimmed = raw.trim()
  if (!trimmed) return {valid: false, url: ''}
  try {
    const parsed = new URL(trimmed)
    if (parsed.protocol !== 'https:' || parsed.hostname.toLowerCase() !== 'thunderstore.io' || parsed.username || parsed.password) {
      return {valid: false, url: trimmed}
    }
    const parts = parsed.pathname.replace(/\/+$/, '').split('/').filter(Boolean)
    let namespace = ''
    let packageName = ''
    let version: string | undefined

    // /package/download/{namespace}/{packageName}/{version}
    if (parts.length === 5 && parts[0] === 'package' && parts[1] === 'download') {
      namespace = parts[2]
      packageName = parts[3]
      version = parts[4]
      if (!isValidThunderstoreVersion(version)) {
        return {valid: false, url: trimmed}
      }
    } else if ((parts.length === 5 || parts.length === 7) && parts[0] === 'c' && parts[1] === 'dyson-sphere-program' && parts[2] === 'p') {
      // /c/dyson-sphere-program/p/{namespace}/{packageName} [/v/{version}]
      namespace = parts[3]
      packageName = parts[4]
      if (parts.length === 7) {
        if (parts[5] !== 'v' || !isValidThunderstoreVersion(parts[6])) {
          return {valid: false, url: trimmed}
        }
        version = parts[6]
      }
    } else {
      return {valid: false, url: trimmed}
    }

    if (!MOD_ID_PATTERN.test(namespace) || !MOD_ID_PATTERN.test(packageName)) {
      return {valid: false, url: trimmed}
    }
    return {valid: true, url: trimmed, namespace, packageName, version}
  } catch {
    return {valid: false, url: trimmed}
  }
}
