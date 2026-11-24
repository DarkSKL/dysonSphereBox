import {computed, onScopeDispose, ref, shallowRef, watch} from 'vue'

import {accountSessionKey} from '../../pages/services/account'
import {feedbackCall, FeedbackError} from '../../services/feedback'
import type {StarDetail, StarWrite} from '../../types/feedback'

export type EditorMode = 'publish' | 'edit' | 'supplement' | 'comment'
interface Editor {mode: EditorMode; starId: string}
interface Draft {body: string; showName: boolean}
interface Attempt extends Draft {requestId: string; bodyVersion: string}

const emptyDraft: Draft = {body: '', showName: false}

export function normalizedFeedbackText(value: string): string {
  return value.replace(/\r\n?/g, '\n')
}

function draftKey(editor: Editor): string {
  return editor.mode + ':' + editor.starId
}

// =============== 文本提交与恢复 ===============
// 说明：草稿和请求号按写入语义与星分开。网络结果未知时沿用原请求 ID 和正文版本。
export function useFeedbackEditor(
  onSaved: (mode: EditorMode, result: StarWrite | undefined, body: string) => void,
  onConflict: (reason: string, starId: string) => void,
) {
  const editor = shallowRef<Editor | null>(null)
  const draft = shallowRef<Draft>(emptyDraft)
  const error = ref('')
  const saving = ref(false)
  const drafts = new Map<string, Draft>()
  const attempts = new Map<string, Attempt>()
  let generation = 0

  const limit = computed(() => editor.value?.mode === 'comment' ? 300 : 1000)
  const length = computed(() => [...normalizedFeedbackText(draft.value.body)].length)
  const valid = computed(() => length.value <= limit.value && !!draft.value.body.trim())

  function update(next: Draft): void {
    if (!editor.value) return
    draft.value = next
    drafts.set(draftKey(editor.value), next)
  }

  function open(mode: EditorMode, detail?: StarDetail): void {
    if (saving.value) return
    const next = {mode, starId: detail?.starId ?? ''}
    editor.value = next
    draft.value = drafts.get(draftKey(next)) ?? (mode === 'edit' && detail
      ? {body: detail.original.body, showName: false}
      : emptyDraft)
    error.value = ''
  }

  function close(discard = false): void {
    if (discard && editor.value) {
      drafts.delete(draftKey(editor.value))
      attempts.delete(draftKey(editor.value))
    }
    editor.value = null
    error.value = ''
  }

  function transferToSupplement(detail: StarDetail): void {
    const body = draft.value.body
    close(true)
    open('supplement', detail)
    update({body, showName: false})
  }

  async function submit(detail?: StarDetail): Promise<void> {
    const target = editor.value
    if (!target || saving.value || !valid.value || (target.starId && target.starId !== detail?.starId)) return
    const scope = generation
    const key = draftKey(target)
    const body = normalizedFeedbackText(draft.value.body)
    const showName = draft.value.showName
    let attempt = attempts.get(key)
    if (!attempt || attempt.body !== body || attempt.showName !== showName) {
      attempt = {requestId: crypto.randomUUID(), body, showName, bodyVersion: detail?.original.bodyVersion ?? ''}
      attempts.set(key, attempt)
    }
    const {requestId, bodyVersion} = attempt
    saving.value = true
    error.value = ''
    try {
      let result: StarWrite | undefined
      switch (target.mode) {
        case 'publish':
          result = await feedbackCall('Publish', {requestId, body, showName})
          break
        case 'edit':
          result = await feedbackCall('Edit', {requestId, body, starId: target.starId, bodyVersion})
          break
        default:
          await feedbackCall('Reply', {requestId, body, starId: target.starId, bodyVersion, kind: target.mode})
      }
      if (scope !== generation) return
      close(true)
      onSaved(target.mode, result, body)
    } catch (failure) {
      if (scope !== generation) return
      error.value = failure instanceof Error ? failure.message : '提交失败，草稿已保留。'
      if (failure instanceof FeedbackError) {
        if (['invalidInput', 'versionChanged', 'editLocked', 'requestConflict', 'unavailable'].includes(failure.reason)) attempts.delete(key)
        onConflict(failure.reason, target.starId)
      }
    } finally {
      if (scope === generation) saving.value = false
    }
  }

  function reset(): void {
    generation++
    saving.value = false
    close()
    draft.value = emptyDraft
    drafts.clear()
    attempts.clear()
  }

  watch(accountSessionKey, reset, {flush: 'sync'})
  onScopeDispose(reset)

  return {editor, draft, error, saving, limit, length, valid, open, close, update, submit, transferToSupplement}
}
