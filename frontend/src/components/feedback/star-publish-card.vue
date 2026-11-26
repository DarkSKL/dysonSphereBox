<script setup lang="ts">
import {AlertCircle, SendHorizontal, X} from '@lucide/vue'
import type {InputInst} from 'naive-ui'
import {computed, nextTick, ref, watch} from 'vue'

import {useGalaxyTheme} from '../../composables/feedback/useGalaxyTheme'
import {accountUser} from '../../pages/services/account'
import StarAvatar from './star-avatar.vue'

const props = defineProps<{
  open: boolean
  body: string
  showName: boolean
  length: number
  limit: number
  valid: boolean
  saving: boolean
  error: string
  disabled: boolean
}>()

const emit = defineEmits<{
  text: [value: string]
  name: [value: boolean]
  submit: []
  close: []
}>()

// =============== 发布一颗星弹窗卡片 ===============
// 说明：与星点详情弹窗（star-detail-card）共享同一套 8px 外框、括角装饰与双主题遮罩体系；
//       顶部实时预览发布身份（匿名默认头像 / 署名用户头像），底部收拢署名开关、字数统计与提交按钮。
// 交互：打开自动聚焦正文输入框，Ctrl/Cmd + Enter 快捷提交；点击遮罩、关闭图标或 Esc 收起并保留草稿。
// 边界：提交审核期间禁止关闭，遮罩点击要求按下与抬起均落在遮罩自身，避免划选文字拖出误关。
const theme = useGalaxyTheme()
const inputRef = ref<InputInst>()
let pointerStartedOnBackdrop = false

const currentNickName = computed(() => accountUser.value?.nickName || accountUser.value?.userName || '我的昵称')
const publishIdentityLabel = computed(() => (props.showName ? `以 ${currentNickName.value} 署名发布` : '匿名发布'))
const canSubmit = computed(() => props.valid && !props.disabled && !props.saving)

watch(
  () => props.open,
  (opened) => {
    pointerStartedOnBackdrop = false
    if (!opened) return
    void nextTick(() => inputRef.value?.focus())
  },
  {immediate: true},
)

function onBackdropPointerDown(): void {
  pointerStartedOnBackdrop = true
}

function onCardPointerDown(): void {
  pointerStartedOnBackdrop = false
}

function onBackdropClick(): void {
  const shouldClose = pointerStartedOnBackdrop
  pointerStartedOnBackdrop = false
  if (shouldClose) requestClose()
}

function requestClose(): void {
  if (props.saving) return
  emit('close')
}

function onKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    if (canSubmit.value) emit('submit')
  }
}
</script>

<template>
  <!-- ~ 发布反馈弹窗卡片 -->
  <!-- * 说明：在星图中央弹出卡片式发布框，替代旧版右侧栏表单，避免双重标题与狭长挤占。 -->
  <!-- * 交互：切换「显示我的昵称」即时联动顶栏头像与署名预览；支持 Ctrl/Cmd+Enter 快捷提交与 Esc 收起。 -->
  <!-- * 依赖：useFeedbackEditor 草稿状态、accountUser 账号资料与 StarAvatar。 -->
  <!-- * 边界：提交中禁止关闭；草稿在取消关闭后自动保留。 -->
  <Transition name="star-card-modal">
    <div
      v-if="open"
      class="star-card-backdrop"
      :class="'is-' + theme.mode"
      @pointerdown.self="onBackdropPointerDown"
      @click.self="onBackdropClick"
      @wheel.stop
      @keydown.esc.stop.prevent="requestClose"
    >
      <section
        class="star-publish-card ds-bracket"
        role="dialog"
        aria-modal="true"
        aria-labelledby="feedback-publish-card-title"
        @pointerdown="onCardPointerDown"
      >
        <header class="publish-header">
          <div class="publish-heading">
            <span class="publish-avatar" aria-hidden="true">
              <StarAvatar
                identity="publish-preview"
                :anonymous="!showName"
                :source="showName ? (accountUser?.headerImg ?? '') : ''"
              />
            </span>
            <div class="publish-titles">
              <h2 id="feedback-publish-card-title" class="publish-title">发布一颗星</h2>
              <p class="publish-sub">{{ publishIdentityLabel }}</p>
            </div>
          </div>

          <n-tooltip :delay="240" placement="bottom-end">
            <template #trigger>
              <button
                type="button"
                class="publish-close"
                aria-label="关闭发布窗口"
                :disabled="saving"
                @click="requestClose"
              >
                <X :size="16" :stroke-width="1.9" aria-hidden="true" />
              </button>
            </template>
            关闭 (Esc)
          </n-tooltip>
        </header>

        <form class="publish-form" @submit.prevent="emit('submit')" @keydown="onKeydown">
          <div class="publish-body">
            <n-input
              ref="inputRef"
              type="textarea"
              :value="body"
              :autosize="{minRows: 5, maxRows: 11}"
              :input-props="{'aria-label': '反馈内容', 'aria-describedby': 'feedback-publish-count'}"
              :status="length > limit ? 'error' : undefined"
              :disabled="saving"
              placeholder="想让开发组和其他玩家知道什么？（Ctrl+Enter 提交）"
              @update:value="emit('text', $event)"
            />

            <div v-if="error" class="publish-error" role="alert">
              <AlertCircle :size="14" class="error-icon" aria-hidden="true" />
              <span>{{ error }}</span>
            </div>
          </div>

          <footer class="publish-footer">
            <n-checkbox
              :checked="showName"
              :disabled="saving"
              @update:checked="emit('name', $event)"
            >
              显示我的昵称
            </n-checkbox>

            <div class="publish-actions">
              <span
                id="feedback-publish-count"
                class="publish-count type-display"
                :class="{'is-over': length > limit}"
              >
                {{ length > limit ? '超出上限 · ' : '' }}{{ length }} / {{ limit }}
              </span>
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
                提交审核
              </n-button>
            </div>
          </footer>
        </form>
      </section>
    </div>
  </Transition>
</template>

<style scoped lang="scss">
// =============== 遮罩与弹窗卡片外壳 ===============
// 说明：与 star-detail-card 保持同一套遮罩、圆角、细边框与括角视觉规范。
.star-card-backdrop {
  position: absolute;
  z-index: 4;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgb(1 4 12 / 0.56);
  backdrop-filter: blur(8px);

  &.is-light {
    background: rgb(235 235 235 / 0.56);
    backdrop-filter: blur(6px);
  }
}

.star-publish-card {
  --bracket-size: 16px;
  --bracket-color: color-mix(in srgb, var(--ds-brand) 52%, transparent);
  position: relative;
  display: flex;
  flex-direction: column;
  width: min(540px, calc(100% - 12px));
  max-height: min(620px, calc(100% - 12px));
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-radius: 8px;
  background: var(--ds-bg-panel);
  color: var(--ds-text-primary);
  box-shadow: 0 26px 76px rgb(0 0 0 / 0.44);
  isolation: isolate;

  &.ds-bracket::after {
    z-index: 3;
  }
}

.is-light .star-publish-card {
  --bracket-color: var(--ds-border-strong);
  box-shadow: 0 22px 58px rgb(0 0 0 / 0.14);
}

// =============== 单排顶栏 ===============
.publish-header {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 12px 12px 18px;
  border-bottom: 1px solid var(--ds-border-subtle);
}

.publish-heading {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.publish-avatar {
  flex: none;
  width: 32px;
  height: 32px;
  overflow: hidden;
  border: 1px solid var(--ds-border-default);
  border-radius: 50%;
  background: var(--ds-bg-panel-elevated);
}

.publish-titles {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.publish-title {
  margin: 0;
  color: var(--ds-text-primary);
  font-size: 14.5px;
  font-weight: 600;
  line-height: 1.35;
}

.publish-sub {
  margin: 0;
  color: var(--ds-text-muted);
  font-size: 11.5px;
  line-height: 1.3;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.publish-close {
  flex: none;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--ds-text-secondary);
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
    opacity: 0.4;
    cursor: not-allowed;
  }
}

// =============== 输入区与融合底栏 ===============
.publish-form {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1 1 auto;
}

.publish-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px 18px;
  overflow: auto;
}

.publish-error {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--ds-error-border);
  border-radius: 4px;
  background: var(--ds-error-bg);
  color: var(--ds-error);
  font-size: 12.5px;
  line-height: 1.45;
  overflow-wrap: anywhere;
}

.error-icon {
  flex: none;
}

.publish-footer {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  padding: 10px 18px;
  border-top: 1px solid var(--ds-border-subtle);
  background: var(--ds-bg-panel-elevated);
}

.publish-actions {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  margin-left: auto;
}

.publish-count {
  color: var(--ds-text-muted);
  font-size: 12px;
  &.is-over {
    color: var(--ds-error);
    font-weight: 600;
  }
}

// =============== 弹窗进出场过渡 ===============
.star-card-modal-enter-active {
  transition: opacity 200ms cubic-bezier(0.22, 1, 0.36, 1);
  .star-publish-card {
    transition:
      opacity 220ms cubic-bezier(0.22, 1, 0.36, 1),
      transform 240ms cubic-bezier(0.22, 1, 0.36, 1);
  }
}

.star-card-modal-leave-active {
  pointer-events: none;
  transition: opacity 160ms ease-in;
  .star-publish-card {
    transition:
      opacity 150ms ease-in,
      transform 160ms ease-in;
  }
}

.star-card-modal-enter-from,
.star-card-modal-leave-to {
  opacity: 0;
  .star-publish-card {
    opacity: 0;
    transform: translateY(8px) scale(0.98);
  }
}

@media (max-width: 540px) {
  .star-card-backdrop {
    padding: 10px;
  }
  .publish-header {
    padding: 11px 12px 11px 14px;
  }
  .publish-body {
    padding: 12px 14px;
  }
  .publish-footer {
    padding: 9px 14px;
  }
}
</style>
