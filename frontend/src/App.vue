<script lang="ts" setup>
</script>

<template>
  <div class="min-h-screen bg-slate-950 p-8 text-slate-100">
    <nav class="mb-8 flex flex-wrap justify-center gap-3">
      <RouterLink to="/">
        <n-button quaternary >Home</n-button>
      </RouterLink>
      <RouterLink to="/about">
        <n-button quaternary>About</n-button>
      </RouterLink>
      <RouterLink to="/boxes/demo-001">
        <n-button quaternary>Box Demo</n-button>
      </RouterLink>
    </nav>
    <RouterView />
  </div>

</template>

<style lang="scss"></style>
