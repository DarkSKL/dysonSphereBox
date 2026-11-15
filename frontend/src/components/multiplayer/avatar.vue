<script setup lang="ts">
import {computed, onMounted, ref} from 'vue'
import {defaultAvatarUrl, resolveAvatarUrl} from '../../pages/services/account'
import {loadServerAddress, serverAddressReady, serverEffectiveAddress} from '../../pages/services/server'

// =============== 会话头像与失败隔离 ===============
// 说明：各实例按身份、原始地址和服务器隔离加载失败；调用方传入握手快照，不读取当前账户头像。
// 边界：仅展示安全 HTTP(S) 图片，缺省或加载失败使用本地默认图，不附凭证、不重试默认图。
const props = defineProps<{source?: string; identity: string | number}>()
const source = computed(() => resolveAvatarUrl(props.source, serverEffectiveAddress.value, serverAddressReady.value))
const imageKey = computed(() => JSON.stringify([props.identity, props.source, serverEffectiveAddress.value, source.value]))
const failedKey = ref('')
const displaySource = computed(() => source.value && failedKey.value !== imageKey.value ? source.value : defaultAvatarUrl)
// 说明：旧图片的迟到错误只标记其原始 key，不能使切换后的头像进入失败态。
function imageError(event: Event): void {
  const key = (event.target as HTMLImageElement).dataset.avatarKey
  if (key) failedKey.value = key
}
onMounted(() => { void loadServerAddress() })
</script>

<template>
  <!-- ~ 联机会话头像 -->
  <!-- * 说明：展示握手头像或本地默认图；交互：图片失败仅影响本实例；依赖：安全解析器与服务器基址；边界：不请求账户资料。 -->
  <img :key="imageKey" :src="displaySource" :data-avatar-key="imageKey" class="multiplayer-avatar" alt="" referrerpolicy="no-referrer" @error="imageError" />
</template>

<style scoped>
.multiplayer-avatar { display: block; width: 100%; height: 100%; object-fit: cover; border-radius: inherit; }
</style>
