<script setup lang="ts">
withDefaults(defineProps<{readonly size?: number}>(), {size: 20})
</script>

<template>
  <span class="feedback-nebula-icon" :style="{width: `${size}px`, height: `${size}px`}" />
</template>

<style scoped>
.feedback-nebula-icon {
  display: inline-block;
  background-color: currentColor;
  mask: url('../assets/images/feedback-nebula.png') center / contain no-repeat;
}
</style>
