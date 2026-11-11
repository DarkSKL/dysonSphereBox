<script setup lang="ts">
import type {FactoryFailure} from '../../../pages/services/dysonSphere/factory'

// =============== 生成失败说明 ===============
// 说明：弹窗和页内提示共用原因、操作建议及可展开的原始诊断；文本插值不执行后端返回的HTML。
// 边界：这里只展示错误，不重试、不修改用户配置，也不把技术诊断作为默认正文。
defineProps<{failure: FactoryFailure}>()
</script>

<template>
  <div class="generation-error">
    <p>{{ failure.message }}</p>
    <p v-if="failure.suggestion" class="suggestion">{{ failure.suggestion }}</p>
    <details v-if="failure.detail">
      <summary>技术详情 <span class="type-mono">{{ failure.code }}</span></summary>
      <pre class="type-mono">{{ failure.detail }}</pre>
    </details>
    <span v-else class="error-code type-mono">{{ failure.code }}</span>
  </div>
</template>

<style scoped lang="scss">
.generation-error { min-width: 0; overflow-wrap: anywhere; }
.generation-error p { margin: 0; }
.generation-error .suggestion { margin-top: 6px; color: var(--ds-text-secondary); }
.generation-error details { margin-top: 8px; }
.generation-error summary { color: var(--ds-text-secondary); font-size: 12px; cursor: pointer; }
.generation-error summary span { margin-left: 6px; }
.generation-error pre { max-height: min(240px, 40vh); margin: 8px 0 0; overflow: auto; white-space: pre-wrap; font-size: 11px; line-height: 1.6; }
.generation-error .error-code { display: block; margin-top: 6px; color: var(--ds-text-secondary); font-size: 11px; }
</style>
