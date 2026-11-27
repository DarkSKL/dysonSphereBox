<script setup lang="ts">
import {computed, ref, watch} from 'vue'
import type {ForumBlueprintDraft} from '../../types/forum'

const props = defineProps<{
  show: boolean
  initial?: ForumBlueprintDraft | null
}>()

const emit = defineEmits<{
  (event: 'update:show', value: boolean): void
  (event: 'confirm', draft: ForumBlueprintDraft): void
}>()

const MAX_BLUEPRINT_BYTES = 512 * 1024
const name = ref('')
const code = ref('')

watch(() => props.show, (open) => {
  if (!open) return
  if (props.initial?.mode === 'inline') {
    name.value = props.initial.name
    code.value = props.initial.blueprintCode || ''
  } else {
    name.value = ''
    code.value = ''
  }
})

const codeBytes = computed(() => new TextEncoder().encode(code.value.trim()).length)
const isCodeTooLarge = computed(() => codeBytes.value > MAX_BLUEPRINT_BYTES)
const canConfirm = computed(() => name.value.trim().length > 0 && code.value.trim().length > 0 && !isCodeTooLarge.value)

function submit(): void {
  if (!canConfirm.value) return
  emit('confirm', {
    mode: 'inline',
    name: name.value.trim(),
    blueprintCode: code.value.trim(),
  })
  emit('update:show', false)
}
</script>

<template>
  <!-- ~ 内联蓝图编码弹窗 -->
  <!-- * 说明：填写蓝图展示名与蓝图编码文本，确认后绑定到 blueprint 槽位并插入 [[resource:blueprint]] -->
  <n-modal :show="show" preset="card" title="插入蓝图编码" style="width: min(92vw, 560px);" @update:show="emit('update:show', $event)">
    <div class="blueprint-form">
      <div class="form-field">
        <label class="field-label">蓝图名称</label>
        <n-input v-model:value="name" placeholder="例如：黑雾防线全自动产线" maxlength="120" clearable />
      </div>
      <div class="form-field">
        <label class="field-label">蓝图编码</label>
        <n-input
          v-model:value="code"
          type="textarea"
          :autosize="{minRows: 5, maxRows: 10}"
          placeholder="粘贴游戏内复制的 BLUEPRINT:... 编码"
        />
      </div>
      <p v-if="isCodeTooLarge" class="field-hint is-error">
        蓝图编码已超过 512 KiB 上限，无法提交。
      </p>
      <p v-else class="field-hint">
        上限 512 KiB；也可以取消后在工具栏选择「上传 .txt 蓝图文件」。
      </p>
    </div>

    <template #footer>
      <div class="modal-footer">
        <n-button secondary @click="emit('update:show', false)">取消</n-button>
        <n-button type="primary" :disabled="!canConfirm" @click="submit">插入到正文</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped lang="scss">
.blueprint-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.field-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ds-text-primary);
}

.field-hint {
  margin: 0;
  font-size: 12px;
  color: var(--ds-text-muted);

  &.is-error {
    color: var(--ds-error);
  }
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
