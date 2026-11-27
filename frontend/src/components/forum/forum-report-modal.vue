<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import type {ForumReportTarget} from '../../types/forum'

const props = defineProps<{
  show: boolean
  target: ForumReportTarget | null
  submitting: boolean
}>()

const emit = defineEmits<{
  (event: 'update:show', value: boolean): void
  (event: 'submit', reason: string): void
}>()

const presetReasons = [
  '广告引流或无关灌水',
  '包含违规、攻击或不当内容',
  '资源文件损坏、恶意或与描述严重不符',
  '侵犯他人原创权益或抄袭',
]

const reason = ref('')

watch(() => props.show, (open) => {
  if (open) reason.value = ''
})

const targetLabel = computed(() => {
  if (!props.target) return '内容'
  if (props.target.targetType === 'post') return '帖子'
  if (props.target.targetType === 'reply') return '回复'
  return '资源'
})

function submit(): void {
  const trimmed = reason.value.trim()
  if (!trimmed || props.submitting) return
  emit('submit', trimmed)
}
</script>

<template>
  <!-- ~ 论坛举报弹窗 -->
  <!-- * 说明：支持举报帖子、回复或资源，提供常用理由快捷填入与自定义说明 -->
  <n-modal :show="show" preset="card" :title="`举报${targetLabel}`" style="width: min(92vw, 480px);" @update:show="emit('update:show', $event)">
    <div class="report-body">
      <div v-if="target" class="report-target">
        举报对象：<strong>{{ target.title }}</strong>
      </div>
      <div class="preset-list">
        <button
          v-for="item in presetReasons"
          :key="item"
          type="button"
          class="preset-chip"
          :class="{'is-active': reason === item}"
          @click="reason = item"
        >
          {{ item }}
        </button>
      </div>
      <n-input
        v-model:value="reason"
        type="textarea"
        :autosize="{minRows: 3, maxRows: 6}"
        maxlength="300"
        show-count
        placeholder="请说明具体的举报理由，便于管理员核查处置"
      />
    </div>

    <template #footer>
      <div class="modal-footer">
        <n-button secondary :disabled="submitting" @click="emit('update:show', false)">取消</n-button>
        <n-button type="error" :loading="submitting" :disabled="!reason.trim()" @click="submit">提交举报</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped lang="scss">
.report-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.report-target {
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--ds-bg-panel);
  border: 1px solid var(--ds-border-subtle);
  font-size: 13px;
  color: var(--ds-text-secondary);
}

.preset-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.preset-chip {
  padding: 4px 10px;
  border-radius: 4px;
  border: 1px solid var(--ds-border-default);
  background: transparent;
  color: var(--ds-text-secondary);
  font-size: 12px;
  cursor: pointer;

  &:hover {
    background: var(--ds-hover-bg);
    color: var(--ds-text-primary);
  }

  &.is-active {
    border-color: var(--ds-brand);
    background: var(--ds-active-bg);
    color: var(--ds-text-primary);
    font-weight: 600;
  }
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
