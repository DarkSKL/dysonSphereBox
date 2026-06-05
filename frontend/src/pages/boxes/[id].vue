<script setup lang="ts">
import {useRoute} from 'vue-router'

const route = useRoute('/boxes/[id]')
</script>

<template>
  <main class="mx-auto w-full max-w-3xl">
    <n-card title="Box Detail">
      <p class="text-lg font-semibold text-sky-400">当前 Box ID：{{ route.params.id }}</p>
    </n-card>
  </main>
</template>

<style scoped lang="scss"></style>
