import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'

const markdown = new MarkdownIt({html: true, breaks: true, linkify: true, typographer: false})
const allowedTags = ['p', 'br', 'hr', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'strong', 'em', 'b', 'i', 's', 'del', 'ul', 'ol', 'li', 'blockquote', 'pre', 'code', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'a', 'img']
const allowedAttrs = ['href', 'title', 'src', 'alt']

// 说明：只接受不含账号信息、反斜杠和控制字符的 https 地址，避免伪协议或凭据进入正文。
function validHttps(raw: string): string | null {
  if (!/^https:\/\//i.test(raw) || /[\u0000-\u0020\u007f\\]/.test(raw)) return null
  try {
    const url = new URL(raw)
    if (url.protocol !== 'https:' || url.username || url.password) return null
    return url.toString()
  } catch {
    return null
  }
}

// =============== 发布说明 Markdown 渲染 ===============
// 说明：把 Release 正文从 Markdown 清洗为可交给 v-html 的 HTML，保留标题、列表、强调、引用、代码和表格。
// 使用：应用更新弹窗等展示远端发布说明的位置；链接改写为 data-external-href，由调用方点击后交给系统浏览器。
// 边界：禁止脚本、事件属性、内联样式和非 https 链接/图片；无法识别的链接降级为纯文本，无法识别的图片直接移除。
export function renderReleaseNotes(source: string): string {
  const clean = DOMPurify.sanitize(markdown.render(source), {
    ALLOWED_TAGS: allowedTags,
    ALLOWED_ATTR: allowedAttrs,
    ALLOW_DATA_ATTR: false,
    ALLOW_ARIA_ATTR: false,
    FORBID_TAGS: ['style', 'script', 'iframe', 'form', 'input', 'object', 'embed', 'svg'],
    FORBID_ATTR: ['id', 'name', 'srcset', 'style', 'target', 'rel', 'class'],
  })
  const template = document.createElement('template')
  template.innerHTML = clean
  template.content.querySelectorAll<HTMLAnchorElement>('a').forEach((node) => {
    const href = validHttps(node.getAttribute('href') ?? '')
    if (!href) {
      node.replaceWith(...Array.from(node.childNodes))
      return
    }
    node.dataset.externalHref = href
    node.removeAttribute('href')
    node.setAttribute('role', 'link')
    node.tabIndex = 0
  })
  template.content.querySelectorAll<HTMLImageElement>('img').forEach((node) => {
    const src = validHttps(node.getAttribute('src') ?? '')
    if (!src) {
      node.remove()
      return
    }
    node.setAttribute('src', src)
    node.setAttribute('referrerpolicy', 'no-referrer')
    node.setAttribute('loading', 'lazy')
  })
  return template.innerHTML.trim()
}
