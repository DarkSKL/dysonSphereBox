<script setup lang="ts">
import {useAppUpdateState} from '../../composables/system/useAppUpdateState'

const {isUpdatingApp, appUpdatePercentage, appUpdateLabel, appUpdateDetail} = useAppUpdateState()
</script>

<template>
  <!-- ~ 应用更新下载进度 -->
  <!-- * 说明：设置页与弹窗共用真实字节进度；依赖：共享更新状态与 Naive UI -->
  <!-- * 边界：未知大小只显示已下载量；100% 表示下载结束，校验后仍需退出安装 -->
  <section v-if="isUpdatingApp" class="app-update-progress" aria-label="更新包下载进度">
    <div class="app-update-progress-text" role="status">
      <span>{{ appUpdateLabel }}</span>
      <span class="app-update-progress-detail">{{ appUpdateDetail }}</span>
    </div>
    <n-progress
      v-if="appUpdatePercentage !== undefined"
      type="line"
      :percentage="appUpdatePercentage"
      color="var(--ds-brand)"
      :show-indicator="false"
      :processing="true"
      :height="6"
      :border-radius="3"
    />
    <progress v-else class="app-update-indeterminate" :aria-label="appUpdateDetail" />
  </section>
</template>

<style scoped lang="scss">
.app-update-progress {
  margin: 12px 0;
}

.app-update-indeterminate {
  display: block;
  width: 100%;
  height: 6px;
  accent-color: var(--ds-brand);
}

.app-update-progress-text {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 4px 12px;
  margin-bottom: 8px;
  color: var(--ds-text-primary);
  font-size: 12px;
}

.app-update-progress-detail {
  color: var(--ds-text-secondary);
  font-family: var(--ds-font-display);
  font-variant-numeric: tabular-nums;
}
</style>
