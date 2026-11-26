<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'

import {defaultAvatarUrl, resolveAvatarUrl} from '../../pages/services/account'
import {loadServerAddress, serverAddressReady, serverEffectiveAddress} from '../../pages/services/server'

const props = withDefaults(defineProps<{
  /** 原始头像地址（可为服务端相对路径或 HTTP(S) 地址）；留空或匿名时展示默认头像。 */
  source?: string
  /** 是否匿名发言：匿名时强制展示默认头像，绝不展示用户头像。 */
  anonymous?: boolean
  /** 隔离图片加载失败态的稳定身份标识。 */
  identity: string | number
}>(), {
  source: '',
  anonymous: false,
})

// =============== 反馈星系用户头像与失败隔离 ===============
// 说明：复用账号服务的安全头像解析器与统一默认头像（avatar_dsq.png）；
//       匿名发言、未配置头像或图片加载失败时均展示默认头像，不显示首字占位。
// 边界：仅展示安全 HTTP(S) 图片，不附凭证，加载失败按 (identity, source, server) 局部隔离。
const resolvedSource = computed(() => (
  props.anonymous
    ? ''
    : resolveAvatarUrl(props.source, serverEffectiveAddress.value, serverAddressReady.value)
))
const imageKey = computed(() => JSON.stringify([
  props.identity,
  props.anonymous,
  props.source,
  serverEffectiveAddress.value,
  resolvedSource.value,
]))
const failedKey = ref('')
const displaySource = computed(() => (
  resolvedSource.value && failedKey.value !== imageKey.value ? resolvedSource.value : defaultAvatarUrl
))

function onImageError(event: Event): void {
  const key = (event.target as HTMLImageElement).dataset.avatarKey
  if (key) failedKey.value = key
}

onMounted(() => { void loadServerAddress() })
</script>

<template>
  <!-- ~ 反馈星系用户头像 -->
  <!-- * 说明：展示玩家头像或统一默认头像；交互：单图失败仅回退当前实例；依赖：安全头像解析器与服务器基址。 -->
  <img
    :key="imageKey"
    :src="displaySource"
    :data-avatar-key="imageKey"
    class="star-avatar-img"
    alt=""
    referrerpolicy="no-referrer"
    @error="onImageError"
  />
</template>

<style scoped lang="scss">
.star-avatar-img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: inherit;
}
</style>
