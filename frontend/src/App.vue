<script lang="ts" setup>
import { darkTheme, type GlobalThemeOverrides } from 'naive-ui'
import { computed, onMounted } from 'vue'
import {useRoute} from 'vue-router'
import BlankLayout from './layouts/BlankLayout.vue'
import MainLayout from './layouts/MainLayout.vue'
import { useThemeMode } from './composables/useThemeMode'

const route = useRoute()
const { isDarkMode, loadThemeMode } = useThemeMode()

const layouts = {
  main: MainLayout,
  blank: BlankLayout,
}

const layoutComponent = computed(() => layouts[route.meta.layout || 'main'])
const naiveTheme = computed(() => (isDarkMode.value ? darkTheme : null))
const naiveThemeOverrides = computed<GlobalThemeOverrides>(() =>
  isDarkMode.value
    ? {
        common: {
          primaryColor: '#e2b95d',
          primaryColorHover: '#f0c96e',
          primaryColorPressed: '#b98d38',
          primaryColorSuppl: '#e2b95d',
          successColor: '#7cd483',
          infoColor: '#7ebea2',
          warningColor: '#e2b95d',
          errorColor: '#e87d63',
          bodyColor: '#050806',
          cardColor: '#101713',
          modalColor: '#17211c',
          popoverColor: '#17211c',
          tableColor: '#101713',
          baseColor: '#050806',
          borderColor: 'rgb(229 204 143 / 0.22)',
          dividerColor: 'rgb(229 204 143 / 0.14)',
          textColorBase: '#f4ead0',
          textColor1: '#f4ead0',
          textColor2: 'rgb(225 232 218 / 0.76)',
          textColor3: 'rgb(225 232 218 / 0.52)',
          placeholderColor: 'rgb(225 232 218 / 0.32)',
          inputColor: 'rgb(5 8 6 / 0.72)',
          inputColorDisabled: 'rgb(16 23 19 / 0.72)',
          hoverColor: 'rgb(244 234 208 / 0.06)',
        },
        Button: {
          borderRadiusMedium: '4px',
          borderRadiusSmall: '4px',
          textColorPrimary: '#050806',
          textColorHoverPrimary: '#050806',
          textColorPressedPrimary: '#050806',
          colorPrimary: '#e2b95d',
          colorHoverPrimary: '#f0c96e',
          colorPressedPrimary: '#b98d38',
          borderPrimary: '1px solid #e2b95d',
          borderHoverPrimary: '1px solid #f0c96e',
          borderPressedPrimary: '1px solid #b98d38',
        },
        Card: {
          borderRadius: '4px',
          color: 'rgb(16 23 19 / 0.94)',
          borderColor: 'rgb(229 204 143 / 0.18)',
          titleTextColor: '#f4ead0',
          textColor: 'rgb(225 232 218 / 0.76)',
        },
        Input: {
          borderRadius: '4px',
          color: 'rgb(5 8 6 / 0.72)',
          colorFocus: 'rgb(5 8 6 / 0.72)',
          border: '1px solid rgb(229 204 143 / 0.22)',
          borderHover: '1px solid rgb(226 185 93 / 0.38)',
          borderFocus: '1px solid #e2b95d',
          boxShadowFocus: '0 0 0 2px rgb(226 185 93 / 0.42)',
          textColor: '#f4ead0',
          placeholderColor: 'rgb(225 232 218 / 0.32)',
        },
        Dialog: {
          borderRadius: '4px',
          color: '#17211c',
          titleTextColor: '#f4ead0',
          textColor: 'rgb(225 232 218 / 0.76)',
        },
      }
    : {},
)

onMounted(() => {
  void loadThemeMode()
})
</script>

<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="naiveThemeOverrides">
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
  </n-config-provider>
</template>

<style lang="scss"></style>
