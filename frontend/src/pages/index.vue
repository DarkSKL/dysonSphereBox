<script setup lang="ts">
definePage({
  meta: {
    layout: 'main',
    title: 'Home',
  },
})
</script>

<template>
  <section class="grid gap-4">
    <n-card title="Home">
      <p class="text-base text-slate-700">
        这个页面使用 main layout，不开启 KeepAlive。
      </p>
    </n-card>

    <div class="flex flex-wrap gap-3">
      <RouterLink to="/dashboard">
        <n-button type="primary">Open Cached Dashboard</n-button>
      </RouterLink>
      <RouterLink to="/plain">
        <n-button>Open Blank Layout</n-button>
      </RouterLink>
    </div>
  </section>
</template>

<style scoped lang="scss"></style>
