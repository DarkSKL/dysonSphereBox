<script setup lang="ts">
import {computed, nextTick, onBeforeUnmount, onMounted, ref, watch} from 'vue'
import {X} from '@lucide/vue'
import type {InputInst} from 'naive-ui'

import type {EditorMode} from '../../composables/feedback/useFeedbackEditor'

const props = defineProps<{
  mode: EditorMode
  body: string
  showName: boolean
  length: number
  limit: number
  valid: boolean
  saving: boolean
  error: string
  disabled: boolean
  canTransfer: boolean
}>()

const emit = defineEmits<{
  text: [value: string]
  name: [value: boolean]
  submit: []
  close: []
  transfer: []
}>()

const titles: Record<EditorMode, string> = {
  publish: '发布一颗星', edit: '编辑正文', supplement: '追加补充', comment: '写留言',
}
const title = computed(() => titles[props.mode])
const input = ref<InputInst>()
const form = ref<HTMLElement>()
let previousFocus: HTMLElement | null = null

onMounted(() => {
  previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
  input.value?.focus()
})
watch(() => props.mode, () => { void nextTick(() => input.value?.focus()) })
onBeforeUnmount(() => {
  if (previousFocus?.isConnected && form.value?.contains(document.activeElement)) previousFocus.focus()
})
</script>

<template>
  <!-- ~ 纯文字编辑器 -->
  <!-- * 说明：发布、改稿、补充和留言共享字数与提交反馈。 -->
  <!-- * 交互：超长可继续编辑，取消关闭编辑器并保留草稿。 -->
  <!-- * 依赖：当前会话草稿、提交状态和服务端授予的正文权限。 -->
  <!-- * 边界：改稿锁定后可以沿用草稿改作补充。 -->
  <form ref="form" class="feedback-editor" aria-labelledby="feedback-editor-title" @submit.prevent="emit('submit')">
    <div class="editor-heading">
      <h3 id="feedback-editor-title">{{ title }}</h3>
      <n-button quaternary size="small" aria-label="收起编辑器" title="收起编辑器" :disabled="saving" @click="emit('close')">
        <X :size="16" aria-hidden="true" />
      </n-button>
    </div>
    <n-input ref="input" type="textarea" :value="body" :autosize="{minRows: mode === 'comment' ? 3 : 5, maxRows: 9}"
      :input-props="{'aria-label': title, 'aria-describedby': 'feedback-word-count'}"
      :status="length > limit ? 'error' : undefined" :disabled="saving"
      :placeholder="mode === 'publish' ? '想让开发组和其他玩家知道什么？' : '输入纯文字…'"
      @update:value="emit('text', $event)" />
    <div class="editor-meta">
      <n-checkbox v-if="mode === 'publish'" :checked="showName" :disabled="saving" @update:checked="emit('name', $event)">
        显示我的昵称
      </n-checkbox>
      <span v-else>送审通过后可供其他玩家阅读</span>
      <span id="feedback-word-count" :class="{over: length > limit}">
        {{ length > limit ? '超出字数上限 · ' : '' }}{{ length }} / {{ limit }} 字
      </span>
    </div>
    <p v-if="error" class="editor-error" role="alert">{{ error }}</p>
    <div class="editor-actions">
      <n-button v-if="canTransfer" size="small" :disabled="saving" @click="emit('transfer')">改作作者补充</n-button>
      <n-button type="primary" size="small" attr-type="submit" :loading="saving" :disabled="!valid || disabled">
        提交审核
      </n-button>
    </div>
  </form>
</template>

<style scoped lang="scss">
.feedback-editor { display: grid; gap: 12px; min-width: 0; }
.editor-heading, .editor-meta, .editor-actions { display: flex; align-items: center; gap: 8px; }
.editor-heading, .editor-meta { justify-content: space-between; }
.editor-heading h3 { margin: 0; color: var(--ds-text-primary); font-size: 15px; font-weight: 600; }
.editor-meta { flex-wrap: wrap; font-size: 12px; color: var(--ds-text-secondary); }
.editor-actions { justify-content: flex-end; flex-wrap: wrap; }
.over, .editor-error { color: var(--ds-error); }
.editor-error { margin: 0; font-size: 13px; overflow-wrap: anywhere; }
</style>
