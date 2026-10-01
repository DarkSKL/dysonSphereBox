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
// =============== 主题组件映射 ===============
// 说明：让 Naive UI 控件跟随全局色彩（日间白灰黑无彩色、夜间深邃黑加炫蓝），色值需与 style.scss 中的 --ds-* Token 保持同步。
const naiveThemeOverrides = computed<GlobalThemeOverrides>(() => {
  const dark = isDarkMode.value
  const brand = dark ? '#4cc2ff' : '#1c1c1e'
  const brandHover = dark ? '#7dd4ff' : '#3a3a3d'
  const brandPressed = dark ? '#2a9be6' : '#000000'
  const brandContrast = dark ? '#03101f' : '#f5f5f5'
  const textPrimary = dark ? '#eceef8' : '#18181a'
  const textSecondary = dark ? 'rgb(214 216 236 / 0.76)' : 'rgb(24 24 26 / 0.74)'
  const textMuted = dark ? 'rgb(170 172 200 / 0.6)' : '#707072'
  const border = dark ? 'rgb(140 170 230 / 0.15)' : 'rgb(0 0 0 / 0.13)'
  const surface = dark ? '#0a0f1a' : '#f3f3f3'
  const surfaceRaised = dark ? '#0f1624' : '#f8f8f8'
  const page = dark ? '#03060d' : '#ebebeb'
  const input = dark ? '#060a13' : '#f8f8f8'

  return {
    common: {
      primaryColor: brand,
      primaryColorHover: brandHover,
      primaryColorPressed: brandPressed,
      primaryColorSuppl: brand,
      successColor: dark ? '#3ee0b0' : '#4a7560',
      infoColor: dark ? '#6aa8ff' : '#5a5a5e',
      warningColor: dark ? '#ffc15a' : '#94682a',
      errorColor: dark ? '#ff6b7f' : '#a8434a',
      bodyColor: page,
      cardColor: surface,
      modalColor: surfaceRaised,
      popoverColor: surfaceRaised,
      tableColor: surface,
      baseColor: page,
      borderColor: border,
      dividerColor: dark ? 'rgb(140 170 230 / 0.09)' : 'rgb(0 0 0 / 0.07)',
      textColorBase: textPrimary,
      textColor1: textPrimary,
      textColor2: textSecondary,
      textColor3: textMuted,
      placeholderColor: dark ? 'rgb(170 172 200 / 0.4)' : 'rgb(24 24 26 / 0.4)',
      inputColor: input,
      inputColorDisabled: surface,
      hoverColor: dark ? 'rgb(190 190 255 / 0.06)' : 'rgb(0 0 0 / 0.045)',
    },
    Button: {
      borderRadiusMedium: '4px',
      borderRadiusSmall: '4px',
      textColorPrimary: brandContrast,
      textColorHoverPrimary: brandContrast,
      textColorPressedPrimary: brandContrast,
      textColorFocusPrimary: brandContrast,
      colorPrimary: brand,
      colorHoverPrimary: brandHover,
      colorPressedPrimary: brandPressed,
      borderPrimary: `1px solid ${brand}`,
      borderHoverPrimary: `1px solid ${brandHover}`,
      borderPressedPrimary: `1px solid ${brandPressed}`,
    },
    Card: {
      borderRadius: '4px',
      color: surface,
      borderColor: border,
      titleTextColor: textPrimary,
      textColor: textSecondary,
    },
    Input: {
      borderRadius: '4px',
      color: input,
      colorFocus: input,
      border: `1px solid ${border}`,
      borderHover: `1px solid ${dark ? 'rgb(76 194 255 / 0.45)' : 'rgb(0 0 0 / 0.36)'}`,
      borderFocus: `1px solid ${brand}`,
      boxShadowFocus: `0 0 0 2px ${dark ? 'rgb(76 194 255 / 0.28)' : 'rgb(0 0 0 / 0.12)'}`,
      textColor: textPrimary,
      placeholderColor: textMuted,
    },
    Dialog: {
      borderRadius: '4px',
      color: surfaceRaised,
      titleTextColor: textPrimary,
      textColor: textSecondary,
    },
  }
})

onMounted(() => {
  void loadThemeMode()
})
</script>

<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="naiveThemeOverrides">
    <n-message-provider>
      <n-dialog-provider>
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
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<style lang="scss"></style>
