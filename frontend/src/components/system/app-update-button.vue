<script lang="ts">
// 说明：事件和禁用属性直接交给 Naive UI；下载进度由共用进度条展示。
export default {inheritAttrs: false}
</script>

<script setup lang="ts">
import {useAppUpdateState} from '../../composables/system/useAppUpdateState'

const props = defineProps<{readonly secondary?: boolean; readonly disabled?: boolean}>()
const {isUpdatingApp, appUpdateLabel} = useAppUpdateState()
</script>

<template>
  <!-- ~ 应用更新按钮 -->
  <!-- * 说明：显示准备或下载状态；交互：禁止重复更新；依赖：共享更新状态与 Naive UI -->
  <!-- * 边界：下载完成不代表已替换程序，退出安装入口由调用方展示 -->
  <n-button
    v-bind="$attrs"
    type="primary"
    :secondary="props.secondary"
    :disabled="props.disabled || isUpdatingApp"
    :loading="isUpdatingApp"
    :aria-busy="isUpdatingApp"
  >
    <template #icon><slot name="icon" /></template>
    <span v-if="isUpdatingApp">{{ appUpdateLabel }}</span>
    <slot v-else />
  </n-button>
</template>
