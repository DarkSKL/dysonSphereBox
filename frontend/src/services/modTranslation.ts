import {TranslateDescriptions} from '../../wailsjs/go/modDependence/MarketplaceService'
import {getAccountSessionRevision, syncSessionState, verifiedAccountSession} from './account'
import type {TranslationBatch} from '../types/translation'

// 说明：作用域只含服务器、已验证主体和代次；token 始终由 Go 捕获，不进入前端。
export function translationContextKey(): string {
  const state = verifiedAccountSession.value
  return state ? [state.serverKey, state.subjectId, state.generation].join('\n') : ''
}

// 说明：请求完成后同步 401/409 结果；切号或切服时拒绝迟到结果，不自动重试。
export async function translateDescriptions(texts: string[], context: string): Promise<TranslationBatch> {
  const revision = getAccountSessionRevision()
  if (!context || context !== translationContextKey()) throw new Error('translationSessionUnavailable')
  try {
    const result = await TranslateDescriptions(texts, context)
    if (revision !== getAccountSessionRevision() || context !== translationContextKey()) throw new Error('staleTranslation')
    return result
  } finally {
    if (revision === getAccountSessionRevision()) await syncSessionState(revision)
  }
}
