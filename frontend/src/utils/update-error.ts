// =============== 更新检查错误说明 ===============
// 说明：把 Go/Gitee 返回的英文错误链归类为可展示的中文原因与处理建议，原始错误由调用方保留在“技术详情”中。
// 使用：设置页应用更新区域在检查失败时按 summary/hint 渲染，未识别的错误给出通用重试建议。
// 边界：只读映射，不修改原始错误；匹配顺序从具体到通用，避免限流被普通 403 或网络错误覆盖。
export interface UpdateErrorDescription {
  readonly summary: string
  readonly hint: string
}

const RATE_LIMIT_HINT = '应用启动时会自动检查一次，短时间内重复检查可能触发限流；请等待几分钟后重试。'
const NETWORK_HINT = '请检查网络或代理设置，确认可以访问 gitee.com 后重新检查。'

const descriptions: ReadonlyArray<{ readonly pattern: RegExp, readonly description: UpdateErrorDescription }> = [
  {pattern: /rate limit|too many requests|\b429\b/i, description: {summary: 'Gitee 接口请求过于频繁（触发限流）', hint: RATE_LIMIT_HINT}},
  {pattern: /\b403\b|forbidden/i, description: {summary: 'Gitee 接口拒绝访问（403）', hint: NETWORK_HINT}},
  {pattern: /\b404\b|not found/i, description: {summary: 'Gitee 接口或发布仓库不存在（404）', hint: '请确认发布仓库仍可访问；若仓库已迁移，请联系维护者更新更新源。'}},
  {pattern: /timeout|timed out|deadline exceeded/i, description: {summary: '连接 Gitee 超时', hint: NETWORK_HINT}},
  {pattern: /no such host|dial tcp|connection refused|connection reset|network is unreachable|tls handshake|unexpected eof|\beof\b/i, description: {summary: '无法连接 Gitee', hint: NETWORK_HINT}},
  {pattern: /checksum|validation|校验/i, description: {summary: '更新包校验失败', hint: '请稍后重试；若持续失败，请前往 Gitee 发布页手动下载。'}},
]

const fallback: UpdateErrorDescription = {
  summary: '更新服务暂时不可用',
  hint: '请稍后重新检查；若持续失败，请前往 Gitee 发布页查看最新版本。',
}

export function describeUpdateError(raw: string): UpdateErrorDescription {
  const message = raw.trim()
  if (!message) return fallback
  for (const entry of descriptions) {
    if (entry.pattern.test(message)) return entry.description
  }
  return fallback
}
