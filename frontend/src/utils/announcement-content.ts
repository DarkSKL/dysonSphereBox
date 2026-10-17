import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'

const markdown = new MarkdownIt({html: true, breaks: true, linkify: false, typographer: false})
const allowedTags = ['p', 'br', 'hr', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'strong', 'em', 'b', 'i', 's', 'del', 'ul', 'ol', 'li', 'blockquote', 'pre', 'code', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'a', 'img']
const allowedAttrs = ['href', 'title', 'src', 'alt', 'colspan', 'rowspan', 'class']
const languageClasses = new Set(['language-text', 'language-json', 'language-go', 'language-typescript', 'language-javascript', 'language-html', 'language-css', 'language-bash'])

function validHttps(raw: string): string | null {
  if (!/^https:\/\//i.test(raw)) return null
  if (!raw || /[\u0000-\u0020\u007f\\]/.test(raw)) return null
  try {
    const url = new URL(raw)
    if (url.protocol !== 'https:' || url.username || url.password) return null
    return url.toString()
  } catch { return null }
}

function normalizeImage(raw: string, origin: string): string | null {
  if (!raw || /[\u0000-\u0020\u007f\\]/.test(raw) || raw.startsWith('//')) return null
  if (raw.startsWith('/')) {
    try { const url = new URL(raw, origin); return url.origin === new URL(origin).origin ? url.toString() : null } catch { return null }
  }
  return validHttps(raw)
}

export function renderAnnouncement(format: 'html' | 'markdown', source: string, serverOrigin: string): string {
  const parsed = format === 'markdown' ? markdown.render(source) : source
  const clean = DOMPurify.sanitize(parsed, {ALLOWED_TAGS: allowedTags, ALLOWED_ATTR: allowedAttrs, ALLOW_DATA_ATTR: false, ALLOW_ARIA_ATTR: false, FORBID_TAGS: ['style', 'script', 'iframe', 'form', 'input', 'object', 'embed', 'svg'], FORBID_ATTR: ['id', 'name', 'srcset', 'style', 'target', 'rel']})
  const template = document.createElement('template'); template.innerHTML = clean
  template.content.querySelectorAll<HTMLElement>('*').forEach((node) => {
    const permitted = node.tagName === 'A' ? new Set(['href', 'title']) : node.tagName === 'IMG' ? new Set(['src', 'alt', 'title']) : new Set<string>()
    for (const attribute of Array.from(node.attributes)) {
      if (attribute.name === 'class' && node.tagName === 'CODE') {
        const supplied = attribute.value.split(/\s+/)
        const classes = supplied.length === 1 && languageClasses.has(supplied[0]) ? supplied : []
        if (classes.length) node.className = classes.join(' '); else node.removeAttribute('class')
      } else if ((attribute.name === 'colspan' || attribute.name === 'rowspan') && (node.tagName === 'TD' || node.tagName === 'TH')) {
        if (!/^(?:[1-9]|[1-9]\d|100)$/.test(attribute.value)) node.removeAttribute(attribute.name)
      } else if (!permitted.has(attribute.name)) node.removeAttribute(attribute.name)
    }
  })
  template.content.querySelectorAll<HTMLAnchorElement>('a').forEach((node) => {
    const href = validHttps(node.getAttribute('href') ?? '')
    if (!href) node.removeAttribute('href')
    else { node.dataset.externalHref = href; node.removeAttribute('href'); node.setAttribute('role', 'link'); node.tabIndex = 0 }
  })
  template.content.querySelectorAll<HTMLImageElement>('img').forEach((node) => {
    const src = normalizeImage(node.getAttribute('src') ?? '', serverOrigin)
    if (!src) node.remove(); else { node.dataset.announcementSrc = src; node.removeAttribute('src'); node.referrerPolicy = 'no-referrer' }
  })
  const hasImage = template.content.querySelector('img[data-announcement-src]') !== null
  const hasRule = template.content.querySelector('hr') !== null
  return template.content.textContent?.trim() || hasImage || hasRule ? template.innerHTML.trim() : ''
}
