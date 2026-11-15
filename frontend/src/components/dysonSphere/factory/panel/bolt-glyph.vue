<script setup lang="ts">
withDefaults(defineProps<{size?: number}>(), {size: 20})
</script>

<template>
  <!-- ~ 游戏电力图标 -->
  <!-- * 说明：戴森球计划界面中的斜切闪电，填充色继承 currentColor -->
  <!-- * 边界：纯装饰，文字说明由外层提供 -->
  <svg class="bolt-glyph" viewBox="0 0 24 24" :width="size" :height="size" aria-hidden="true"><path d="M15.2 1.5 6.4 13.4h5.1L8.6 22.5 18 9.8h-5.3l2.5-8.3Z" fill="currentColor" /></svg>
</template>

<style scoped>
.bolt-glyph { flex: 0 0 auto; }
</style>
