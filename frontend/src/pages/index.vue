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
    <div class="grid min-h-[24rem] place-items-center border border-dashed border-zinc-200 bg-white">
      <div class="max-w-md text-center">
        <p class="text-xs uppercase tracking-[0.22em] text-zinc-400">Dyson Workshop</p>
        <h1 class="mt-3 text-2xl font-semibold text-zinc-950">戴森工坊</h1>
        <p class="mt-3 text-sm leading-6 text-zinc-500">
          用于管理《戴森球计划》Mod 包、下载源、缓存与本地游戏路径的桌面工具。
        </p>
        <div class="mt-6 flex justify-center gap-3">
          <RouterLink to="/mod">
            <n-button type="primary">打开 Mod 管理</n-button>
          </RouterLink>
          <RouterLink to="/settings">
            <n-button secondary>检查设置</n-button>
          </RouterLink>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped lang="scss"></style>
