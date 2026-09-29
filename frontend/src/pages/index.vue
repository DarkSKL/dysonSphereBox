<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

const router = useRouter()

definePage({ meta: { layout: 'main', title: '戴森球计划' } })

onMounted(() => {
  void router.replace('/dysonSphere')
})
</script>

<template><div aria-hidden="true" /></template>
