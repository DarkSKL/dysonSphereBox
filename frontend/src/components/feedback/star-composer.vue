<script setup lang="ts">
import {FilePlus2, Heart, MessageCircle, PencilLine, SendHorizontal, X} from '@lucide/vue'
import type {InputInst} from 'naive-ui'
import {computed, nextTick, ref, watch} from 'vue'

export type CardComposerMode = 'comment' | 'edit' | 'supplement'

const props = defineProps<{
  published: boolean
  liked: boolean
  likeCount: number
  discussionCount: number
  busy: boolean
  writing: boolean
  mode: CardComposerMode | null
  body: string
  length: number
  limit: number
  valid: boolean
  saving: boolean
  error: string
  disabled: boolean
  canTransfer: boolean
}>()

const emit = defineEmits<{
  like: []
  openComment: []
  text: [value: string]
  submit: []
  close: []
  transfer: []
}>()

// =============== 融合互动与评论底栏 ===============
// 说明：常态将快捷评论输入条与点赞、讨论计数图标融合在同一行（参考现代商业社区卡片底栏），彻底省去中段空旷的独立按钮行；
//       点击输入条或顶栏编辑/补充图标时原地平滑展开输入区。
// 交互：Ctrl/Cmd + Enter 快捷发送；Escape 优先收起输入区并保留草稿，不关闭整张卡片。
// 边界：未过审的私有星隐藏互动与评论底栏，仅在作者点击顶栏编辑按钮时展开编辑区。
const modeMeta: Record<CardComposerMode, {label: string; placeholder: string; submitLabel: string}> = {
  comment: {label: '写留言', placeholder: '写下你的看法或建议…（Ctrl+Enter 发送）', submitLabel: '发送'},
  edit: {label: '编辑正文', placeholder: '修改反馈正文…', submitLabel: '提交审核'},
  supplement: {label: '追加作者补充', placeholder: '补充后续进展或复现细节…', submitLabel: '提交审核'},
}

const inputRef = ref<InputInst>()
const currentMeta = computed(() => (props.mode ? modeMeta[props.mode] : null))
const showIdleBar = computed(() => !props.mode && props.published)
const visible = computed(() => Boolean(props.mode) || showIdleBar.value)
const canSubmit = computed(() => props.valid && !props.disabled && !props.saving)

watch(
  () => props.mode,
  (next) => {
    if (!next) return
    void nextTick(() => inputRef.value?.focus())
  },
  {immediate: true},
)

function onKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    if (canSubmit.value) emit('submit')
  }
}

function onEscape(): void {
  if (props.saving) return
  emit('close')
}
</script>

<template>
  <!-- ~ 卡片底部融合互动与评论栏 -->
  <!-- * 说明：单行并排呈现快捷留言触发条、点赞图标按钮与留言数图标按钮；展开态承接留言、改稿与补充。 -->
  <!-- * 交互：点击输入条或留言图标展开评论；点击爱心即时点赞；Ctrl/Cmd+Enter 发送，Esc 收起并保留草稿。 -->
  <!-- * 依赖：点赞态、讨论总数与 useFeedbackEditor 草稿状态。 -->
  <!-- * 边界：写入期间禁用重复点击与收起。 -->
  <div v-if="visible" class="star-composer" :class="{'is-expanded': Boolean(mode)}">
    <div v-if="showIdleBar" class="dock-idle">
      <button
        type="button"
        class="composer-trigger"
        :disabled="saving || busy"
        aria-label="写条简洁留言"
        @click="emit('openComment')"
      >
        <span class="trigger-placeholder">写条留言…</span>
      </button>

      <div class="dock-metrics" role="toolbar" aria-label="互动操作">
        <n-tooltip :delay="220" placement="top">
          <template #trigger>
            <button
              type="button"
              class="dock-icon-btn"
              :class="{'is-liked': liked}"
              :aria-label="liked ? '取消点赞，当前 ' + likeCount + ' 赞' : '点赞支持，当前 ' + likeCount + ' 赞'"
              :aria-pressed="liked"
              :disabled="writing || busy"
              @click="emit('like')"
            >
              <Heart
                :size="15"
                :stroke-width="1.9"
                :fill="liked ? 'currentColor' : 'none'"
                class="heart-icon"
                aria-hidden="true"
              />
              <span class="dock-num type-display">{{ likeCount }}</span>
            </button>
          </template>
          {{ liked ? '取消点赞' : '点赞支持' }}
        </n-tooltip>

        <n-tooltip :delay="220" placement="top">
          <template #trigger>
            <button
              type="button"
              class="dock-icon-btn"
              :aria-label="'共 ' + discussionCount + ' 条讨论，点击写留言'"
              :disabled="writing || busy"
              @click="emit('openComment')"
            >
              <MessageCircle :size="15" :stroke-width="1.9" aria-hidden="true" />
              <span class="dock-num type-display">{{ discussionCount }}</span>
            </button>
          </template>
          写留言（共 {{ discussionCount }} 条）
        </n-tooltip>
      </div>
    </div>

    <form
      v-else-if="mode && currentMeta"
      class="composer-form"
      :aria-label="currentMeta.label"
      @submit.prevent="emit('submit')"
      @keydown.esc.stop.prevent="onEscape"
      @keydown="onKeydown"
    >
      <div class="composer-bar">
        <span class="composer-mode">
          <PencilLine v-if="mode === 'edit'" :size="13" aria-hidden="true" />
          <FilePlus2 v-else-if="mode === 'supplement'" :size="13" aria-hidden="true" />
          <MessageCircle v-else :size="13" aria-hidden="true" />
          <span>{{ currentMeta.label }}</span>
        </span>
        <n-tooltip :delay="220" placement="top">
          <template #trigger>
            <button
              type="button"
              class="composer-close"
              aria-label="收起输入区"
              :disabled="saving"
              @click="emit('close')"
            >
              <X :size="14" :stroke-width="1.9" aria-hidden="true" />
            </button>
          </template>
          收起（草稿会保留）
        </n-tooltip>
      </div>

      <!-- 编辑正文时文本框就地显示在正文位置（star-detail），这里只保留计数与提交条 -->
      <n-input
        v-if="mode !== 'edit'"
        ref="inputRef"
        type="textarea"
        size="small"
        :value="body"
        :autosize="{minRows: mode === 'comment' ? 2 : 3, maxRows: 6}"
        :input-props="{'aria-label': currentMeta.label, 'aria-describedby': 'star-composer-count'}"
        :status="length > limit ? 'error' : undefined"
        :disabled="saving"
        :placeholder="currentMeta.placeholder"
        @update:value="emit('text', $event)"
      />

      <p v-if="error" class="composer-error" role="alert">{{ error }}</p>

      <div class="composer-foot">
        <div class="composer-meta">
          <span class="composer-note">送审通过后可见</span>
          <span
            id="star-composer-count"
            class="composer-count type-display"
            :class="{'is-over': length > limit}"
          >
            {{ length }}/{{ limit }}
          </span>
        </div>

        <div class="composer-actions">
          <n-button
            v-if="canTransfer"
            size="tiny"
            secondary
            :disabled="saving"
            @click="emit('transfer')"
          >
            改作作者补充
          </n-button>
          <n-button
            type="primary"
            size="small"
            attr-type="submit"
            :loading="saving"
            :disabled="!canSubmit"
          >
            <template #icon>
              <SendHorizontal :size="13" :stroke-width="2" aria-hidden="true" />
            </template>
            {{ currentMeta.submitLabel }}
          </n-button>
        </div>
      </div>
    </form>
  </div>
</template>

<style scoped lang="scss">
.star-composer {
  padding: 9px 14px 9px 18px;
  border-top: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel-elevated);

  &.is-expanded {
    padding: 10px 18px 12px;
    border-top-color: var(--ds-border-default);
  }
}

.dock-idle {
  display: flex;
  align-items: center;
  gap: 8px;
}

.composer-trigger {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  height: 32px;
  padding: 0 11px;
  border: 1px solid var(--ds-border-default);
  border-radius: 4px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-muted);
  font: inherit;
  font-size: 12.5px;
  text-align: left;
  cursor: text;
  transition: border-color 150ms ease, color 150ms ease;

  &:hover:not(:disabled) {
    border-color: var(--ds-border-strong);
    color: var(--ds-text-secondary);
  }

  &:focus-visible {
    outline: 2px solid var(--ds-brand);
    outline-offset: 1px;
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.trigger-placeholder {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dock-metrics {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

@keyframes heart-pop {
  0% { transform: scale(1); }
  45% { transform: scale(1.24); }
  100% { transform: scale(1); }
}

.dock-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  height: 32px;
  padding: 0 9px;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-secondary);
  font: inherit;
  cursor: pointer;
  transition: background-color 140ms ease, color 140ms ease;

  &:hover:not(:disabled) {
    background: var(--ds-hover-bg);
    color: var(--ds-text-primary);
  }

  &:focus-visible {
    outline: 2px solid var(--ds-brand);
    outline-offset: 1px;
  }

  &:disabled {
    opacity: 0.42;
    cursor: not-allowed;
  }

  &.is-liked {
    background: var(--ds-error-bg);
    color: var(--ds-error);
    .heart-icon {
      animation: heart-pop 260ms cubic-bezier(0.22, 1, 0.36, 1);
    }
    &:hover:not(:disabled) {
      background: color-mix(in srgb, var(--ds-error) 18%, transparent);
      color: var(--ds-error);
    }
  }
}

.dock-num {
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
}

.composer-form {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.composer-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.composer-mode {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--ds-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.composer-close {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-muted);
  cursor: pointer;
  transition: background-color 140ms ease, color 140ms ease;

  &:hover:not(:disabled) {
    background: var(--ds-hover-bg);
    color: var(--ds-text-primary);
  }

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
}

.composer-error {
  margin: 0;
  color: var(--ds-error);
  font-size: 12px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.composer-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
}

.composer-meta {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 11.5px;
  color: var(--ds-text-muted);
}

.composer-count.is-over {
  color: var(--ds-error);
  font-weight: 600;
}

.composer-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

@media (prefers-reduced-motion: reduce) {
  .dock-icon-btn.is-liked .heart-icon {
    animation: none;
  }
}

@media (max-width: 540px) {
  .star-composer {
    padding: 8px 12px 8px 14px;
    &.is-expanded {
      padding: 8px 14px 10px;
    }
  }
}
</style>
