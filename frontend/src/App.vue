<script lang="ts" setup>
import {computed} from 'vue'
import {useRoute} from 'vue-router'
import BlankLayout from './layouts/BlankLayout.vue'
import MainLayout from './layouts/MainLayout.vue'

const route = useRoute()

const layouts = {
  main: MainLayout,
  blank: BlankLayout,
}

const layoutComponent = computed(() => layouts[route.meta.layout || 'main'])
const shouldKeepAlive = computed(() => route.meta.keepAlive === true)
</script>

<template>
  <component :is="layoutComponent">
    <RouterView v-slot="{ Component }">
      <KeepAlive>
        <component :is="Component" v-if="shouldKeepAlive" />
      </KeepAlive>
      <component :is="Component" v-if="!shouldKeepAlive" />
    </RouterView>
  </component>
</template>

<style lang="scss"></style>
