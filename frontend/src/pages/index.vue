<script setup lang="ts">
definePage({
  meta: {
    layout: 'main',
    title: '首页',
  },
})
</script>

<template>
  <section class="mx-auto max-w-6xl px-8 py-8">
    <!-- ~ 首页占位 -->
    <!-- * 说明：在正式首页内容确定前，保留一个轻量占位提示 -->
    <!-- * 交互：当前不提供操作入口，只展示临时状态 -->
    <!-- * 依赖：依赖静态文案，不连接后端数据 -->
    <!-- * 边界：后续首页功能明确后可整体替换该区域 -->
    <div class="grid min-h-[24rem] place-items-center">
      <p class="text-sm text-zinc-400">暂时没想好首页放什么 (。・∀・)ノ゛</p>
    </div>
  </section>
</template>

<style scoped lang="scss"></style>
