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
</script>

<template>
  <n-message-provider>
    <component :is="layoutComponent">
      <!-- ~ 路由页面渲染 -->
      <!-- * 说明：根据路由 meta.keepAlive 决定页面是否进入缓存 -->
      <!-- * 交互：切换侧边栏页面时，使用当前 RouterView 插槽中的 route 保证组件和缓存判断同步 -->
      <!-- * 依赖：依赖 unplugin-vue-router 生成的页面组件和 route.fullPath -->
      <!-- * 边界：缺少 key 会让缓存页和普通页在快速切换时复用边界不稳定，因此按完整路径固定组件身份 -->
      <RouterView v-slot="{ Component, route }">
        <KeepAlive>
          <component :is="Component" v-if="route.meta.keepAlive === true" :key="route.fullPath" />
        </KeepAlive>
        <component :is="Component" v-if="route.meta.keepAlive !== true" :key="route.fullPath" />
      </RouterView>
    </component>
  </n-message-provider>
</template>

<style lang="scss"></style>
