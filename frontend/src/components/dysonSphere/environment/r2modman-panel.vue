<script setup lang="ts">
import {computed, nextTick, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, toRef, watch} from 'vue'
import {LockKeyhole, Package} from '@lucide/vue'
import type {DropdownOption} from 'naive-ui'
import type {r2modman} from '../../../../wailsjs/go/models'
import {useR2ModmanState} from '../../../composables/dysonSphere/useR2ModmanState'
import ResourceCard from './resource-card.vue'

// =============== 父工具栏契约与缓存状态 ===============
// 说明：下拉选项和选中路径交给父页面；进入页面只恢复持久缓存，不触发实盘检索。
const props = defineProps<{readonly query: string}>()
const emit = defineEmits<{
  (event: 'summary', value: {total: number, enabled: number, disabled: number}): void
  (event: 'busy', value: boolean): void
  (event: 'profiles', value: r2modman.Profile[]): void
  (event: 'profile-path', value: string): void
}>()
const state = useR2ModmanState(toRef(props, 'query'))
const {
  profile, profiles, loading, hasLoaded, loadError, busy, operatingModName,
  profileReady, summary, filteredMods,
} = state
let pageActive = true
const refresh = async (): Promise<void> => { await state.refresh() }
const syncProfiles = async (): Promise<void> => { await state.syncProfiles() }
// 说明：取消定位或切换失败时仍回报实际选中路径，让父下拉恢复真实选择。
const selectProfile = async (path: string): Promise<void> => {
  if (!pageActive) return
  await state.selectProfile(path)
  if (pageActive) emit('profile-path', profile.value?.path ?? '')
}
const selectDirectory = async (): Promise<void> => {
  if (!pageActive) return
  await state.selectDirectory()
  if (pageActive) emit('profile-path', profile.value?.path ?? '')
}
defineExpose({refresh, syncProfiles, selectProfile, selectDirectory})
watch(summary, (value) => emit('summary', value), {immediate: true})
watch(busy, (value) => emit('busy', value), {immediate: true, flush: 'sync'})
watch(profiles, (value) => emit('profiles', value), {immediate: true})
watch(() => profile.value?.path ?? '', (value) => emit('profile-path', value), {immediate: true})

// =============== 详情与模组锁定 ===============
// 说明：保留后端 canDisable/canRemove 权限，锁定卡片只打开详情，不尝试写入。
const detailName = ref('')
const detailPath = ref('')
const detailVisible = ref(false)
// 说明：普通依赖包可以联动禁用；确认框保留完整包标识，提交时不会根据后来变化的列表扩大范围。
const disabling = ref<{path: string, name: string, names: string[], labels: string[]} | null>(null)
const detailMod = computed(() => profile.value?.path === detailPath.value
  ? state.mods.value.find((mod) => mod.name === detailName.value) ?? null : null)
const isModLocked = (mod: r2modman.Mod): boolean => !mod.canManage || (mod.enabled && !mod.canDisable)
const modLockReason = (mod: r2modman.Mod): string => mod.warning || (mod.enabled ? mod.disableReason : '') || ''
const openDetail = (path: string, name: string): void => {
  if (!pageActive || busy.value) return
  detailPath.value = path
  detailName.value = name
  detailVisible.value = true
}
const toggleMod = async (path: string, mod: r2modman.Mod): Promise<void> => {
  if (!pageActive || busy.value || !profileReady.value) return
  if (isModLocked(mod)) { openDetail(path, mod.name); return }
  if (mod.enabled && mod.disableWith?.length) {
    const names = [mod.name, ...mod.disableWith]
    disabling.value = {
      path, name: mod.name, names,
      labels: names.map((name) => state.mods.value.find((item) => item.name === name)?.displayName || name),
    }
    return
  }
  await state.setModEnabled(path, mod.name, !mod.enabled)
}

// 说明：依赖范围变化后关闭确认并由用户重新发起；真正的实盘范围仍由后端独立核对。
const canConfirmDisable = computed(() => {
  const target = disabling.value
  if (!target || !profileReady.value || profile.value?.path !== target.path) return false
  const mod = state.mods.value.find((item) => item.name === target.name)
  if (!mod?.enabled || !mod.canDisable) return false
  const names = [mod.name, ...(mod.disableWith ?? [])]
  return names.length === target.names.length && names.every((name) => target.names.includes(name))
})
const confirmDisable = async (): Promise<void> => {
  const target = disabling.value
  if (!pageActive || busy.value || !target || !canConfirmDisable.value) return
  if (await state.setModEnabled(target.path, target.name, false, target.names)) disabling.value = null
}
const updateDisableShow = (show: boolean): void => {
  if (!show && !busy.value) disabling.value = null
}

// =============== 右键菜单与移除确认 ===============
// 说明：菜单和确认框捕获原 Profile 路径；失活或版本变化后的异步菜单不得重新打开。
const menu = ref<{show: boolean, x: number, y: number, path: string, name: string}>({
  show: false, x: 0, y: 0, path: '', name: '',
})
const menuMod = computed(() => profile.value?.path === menu.value.path
  ? state.mods.value.find((mod) => mod.name === menu.value.name) ?? null : null)
const menuOptions = computed<DropdownOption[]>(() => [
  {key: 'detail', label: '查看详情', disabled: busy.value || !menuMod.value},
  {
    key: 'toggle', label: menuMod.value?.enabled ? '禁用模组' : '启用模组',
    disabled: busy.value || !profileReady.value || !menuMod.value || isModLocked(menuMod.value),
  },
  {
    key: 'remove', label: '移除模组',
    disabled: busy.value || !profileReady.value || !menuMod.value?.canManage || !menuMod.value?.canRemove,
  },
])
let menuVersion = 0
const closeMenu = (): void => { menuVersion += 1; menu.value.show = false }
const openMenu = async (event: MouseEvent, mod: r2modman.Mod): Promise<void> => {
  event.preventDefault()
  if (!pageActive || busy.value || !profile.value) return
  const path = profile.value.path
  closeMenu()
  const version = menuVersion
  await nextTick()
  if (!pageActive || version !== menuVersion || busy.value || profile.value?.path !== path) return
  menu.value = {show: true, x: event.clientX, y: event.clientY, path, name: mod.name}
}
const removal = ref<{path: string, profileName: string, mod: r2modman.Mod} | null>(null)
const canRemoveTarget = computed(() => {
  const target = removal.value
  if (!target || !profileReady.value || profile.value?.path !== target.path) return false
  const mod = state.mods.value.find((item) => item.name === target.mod.name)
  return Boolean(mod?.canManage && mod.canRemove)
})
const handleMenuSelect = async (key: string | number): Promise<void> => {
  const path = menu.value.path
  const mod = menuMod.value
  closeMenu()
  if (!pageActive || busy.value || !mod || profile.value?.path !== path) return
  if (key === 'detail') openDetail(path, mod.name)
  else if (key === 'toggle') await toggleMod(path, mod)
  else if (key === 'remove' && profileReady.value && mod.canManage && mod.canRemove) {
    removal.value = {path, profileName: profile.value.name, mod}
  }
}
const confirmRemove = async (): Promise<void> => {
  const target = removal.value
  if (!pageActive || !target || busy.value || !canRemoveTarget.value) return
  if (await state.removeMod(target.path, target.mod.name)) removal.value = null
}
const updateRemovalShow = (show: boolean): void => {
  if (!show && !busy.value) removal.value = null
}

// =============== 缓存页生命周期 ===============
// 说明：激活只合并缓存读取；失活关闭浮层，已授权写入可完成但不会重开交互。
watch(busy, (value) => { if (value) closeMenu() }, {flush: 'sync'})
watch(() => profile.value?.path, () => {
  closeMenu()
  detailVisible.value = false
  removal.value = null
  disabling.value = null
})
onMounted(() => { void refresh() })
onActivated(() => { pageActive = true; void refresh() })
onDeactivated(() => {
  pageActive = false
  closeMenu()
  detailVisible.value = false
  removal.value = null
  disabling.value = null
})
onBeforeUnmount(() => { pageActive = false; closeMenu(); emit('busy', false) })
</script>

<template>
  <!-- 说明：没有 Profile 或缓存时不渲染 R2 分区；组件继续保留同步能力，发现 Profile 后再显示。 -->
  <section v-if="profile || profiles.length > 0" class="r2modman-panel min-w-0" :aria-busy="busy || (!hasLoaded && loading)" aria-label="r2modman Profile">
    <!-- ~ Profile 持久快照 -->
    <!-- * 说明：只显示分区标题、数量和缓存卡片；下拉选择及同步位于父工具栏 -->
    <!-- * 交互：普通刷新保留旧画面，不禁用操作；首次缓存返回前不显示读取过程 -->
    <!-- * 依赖：GetProfile、ListProfiles 只读持久缓存，显式同步才检索实盘 -->
    <!-- * 边界：首次读取失败可重试；正常操作失败只显示 toast -->
    <h2 class="r2modman-title"><span>r2modman</span><em v-if="hasLoaded">{{ summary.total }}</em></h2>
    <div v-if="!hasLoaded && !loading" class="r2modman-empty">
      <p>{{ loadError ? 'Profile 缓存暂不可用。' : '点击同步检索 Profile，或从下拉栏手动定位。' }}</p>
      <n-button size="tiny" @click="refresh">重试</n-button>
    </div>
    <div v-else-if="hasLoaded && !profile" class="r2modman-empty">
      <Package :size="22" :stroke-width="1.7" />
      <p>点击同步检索 Profile，或从下拉栏手动定位。</p>
    </div>
    <template v-else-if="profile">
      <p v-if="filteredMods.length === 0" class="r2modman-empty">
        {{ query.trim() ? '没有匹配的 Profile 模组。' : '暂无已缓存模组，点击同步更新所选 Profile。' }}
      </p>
      <div v-else class="r2modman-grid">
        <ResourceCard
          v-for="mod in filteredMods"
          :key="`${profile.path}:${mod.name}`"
          class="r2modman-resource"
          :name="mod.displayName || mod.name"
          :meta="mod.version || '未知版本'"
          :status="`${mod.enabled ? '已启用' : '已禁用'}${isModLocked(mod) ? ' · 锁定' : ''}`"
          :tone="mod.enabled ? 'enabled' : 'disabled'"
          :operating="busy && operatingModName === mod.name"
          :disabled="busy"
          :aria-disabled="busy || !profileReady || isModLocked(mod)"
          :title="[mod.displayName || mod.name, modLockReason(mod) || mod.description].filter(Boolean).join('：')"
          @click="toggleMod(profile.path, mod)"
          @contextmenu="openMenu($event, mod)"
        >
          <LockKeyhole v-if="isModLocked(mod)" :size="24" :stroke-width="1.7" />
          <Package v-else :size="24" :stroke-width="1.7" />
        </ResourceCard>
      </div>
    </template>
    <n-dropdown
      trigger="manual" placement="bottom-start" :animated="false"
      :show="menu.show" :x="menu.x" :y="menu.y" :options="menuOptions"
      @clickoutside="closeMenu" @select="handleMenuSelect"
      @update:show="(show: boolean) => { if (!show) closeMenu() }"
    />
  </section>

  <!-- ~ 模组完整详情 -->
  <!-- * 说明：展示原始名称、展示名、作者、版本、说明、网址、依赖及管理限制 -->
  <!-- * 交互：详情按当前 Profile 和模组名称读取最新状态 -->
  <!-- * 依赖：r2modman.Mod 全部字段；技术内容使用等宽字体并允许换行 -->
  <!-- * 边界：条目消失或切换 Profile 后不展示旧目标的数据 -->
  <n-modal
    v-model:show="detailVisible" preset="card" title="Profile 模组详情"
    class="r2modman-modal" :style="{width: 'min(640px, calc(100vw - 32px))'}"
  >
    <div v-if="detailMod" class="r2modman-dialog-body">
      <dl class="r2modman-details">
        <dt>Profile</dt><dd>{{ profile?.name }}</dd>
        <dt>目录</dt><dd class="type-mono">{{ detailPath }}</dd>
        <dt>名称</dt><dd>{{ detailMod.displayName || detailMod.name }}</dd>
        <dt>标识</dt><dd class="type-mono">{{ detailMod.name }}</dd>
        <dt>作者</dt><dd>{{ detailMod.author || '未知作者' }}</dd>
        <dt>版本</dt><dd class="type-mono">{{ detailMod.version || '未知版本' }}</dd>
        <dt>状态</dt><dd>{{ detailMod.enabled ? '已启用' : '已禁用' }}</dd>
        <dt>管理状态</dt><dd>{{ isModLocked(detailMod) ? '锁定' : '可直接管理' }}</dd>
        <dt>说明</dt><dd class="r2modman-description">{{ detailMod.description || '暂无说明' }}</dd>
        <dt>网址</dt><dd class="type-mono">{{ detailMod.websiteUrl || '未提供' }}</dd>
        <dt>依赖</dt>
        <dd>
          <ul v-if="detailMod.dependencies?.length" class="r2modman-dependencies type-mono">
            <li v-for="(dependency, index) in detailMod.dependencies" :key="`${dependency}:${index}`">{{ dependency }}</li>
          </ul>
          <span v-else>无依赖</span>
        </dd>
        <dt>锁定原因</dt><dd :class="{'r2modman-warning': modLockReason(detailMod)}">{{ modLockReason(detailMod) || '无' }}</dd>
        <dt>移除限制</dt><dd>{{ detailMod.canRemove ? '可移除' : detailMod.removeReason || '只读' }}</dd>
      </dl>
    </div>
    <p v-else class="r2modman-hint">此模组已不在当前 Profile 中，请刷新后查看。</p>
  </n-modal>

  <!-- ~ 依赖联动禁用 -->
  <!-- * 说明：列出禁用当前包时受影响的全部启用模组 -->
  <!-- * 交互：确认后一次禁用，取消保持原状；依赖范围变化时禁止提交旧确认 -->
  <!-- * 依赖：disableWith 和 SetModEnabled；边界：不影响未列出的包，不处理框架文件 -->
  <n-modal
    :show="Boolean(disabling)" preset="card" title="联动禁用模组"
    :style="{width: 'min(480px, calc(100vw - 32px))'}"
    :mask-closable="!busy" :close-on-esc="!busy" :closable="!busy" @update:show="updateDisableShow"
  >
    <p>存在依赖关系，将一起禁用以下模组：</p>
    <ul class="r2modman-dependencies">
      <li v-for="(label, index) in disabling?.labels ?? []" :key="index">{{ label }}</li>
    </ul>
    <template #footer>
      <div class="flex justify-end gap-2">
        <n-button :disabled="busy" @click="updateDisableShow(false)">取消</n-button>
        <n-button type="primary" :loading="busy" :disabled="busy || !canConfirmDisable" @click="confirmDisable">全部禁用</n-button>
      </div>
    </template>
  </n-modal>

  <!-- ~ Profile 写入确认 -->
  <!-- * 说明：移除显示捕获的目标 Profile 与路径 -->
  <!-- * 交互：确认期间禁用按钮和关闭；失败通过 toast 提示并恢复缓存 -->
  <!-- * 依赖：RemoveMod；备份与依赖约束由后端执行 -->
  <!-- * 边界：确认只作用于原目标，Profile 变化后必须重新发起 -->
  <n-modal
    :show="Boolean(removal)" preset="card" title="移除 Profile 模组"
    class="r2modman-modal" :style="{width: 'min(520px, calc(100vw - 32px))'}"
    :mask-closable="!busy" :close-on-esc="!busy" :closable="!busy" @update:show="updateRemovalShow"
  >
    <template v-if="removal">
      <p>将“{{ removal.mod.displayName || removal.mod.name }}”从此 Profile 移除，盒子会保留备份。</p>
      <p>这会更改 r2modman 中的 Profile“{{ removal.profileName }}”。存在已安装依赖者时将阻止移除。</p>
      <p class="r2modman-path type-mono">{{ removal.path }}</p>

    </template>
    <template #footer>
      <div class="flex flex-wrap justify-end gap-2">
        <n-button :disabled="busy" @click="updateRemovalShow(false)">取消</n-button>
        <n-button type="error" :disabled="busy || !canRemoveTarget" @click="confirmRemove">确认从此 Profile 移除</n-button>
      </div>
    </template>
  </n-modal>

</template>

<style scoped lang="scss">
.r2modman-panel {
  margin-top: 18px;
  color: var(--ds-text-primary);
  font-size: 12px;
}
.r2modman-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  color: var(--ds-text-secondary);
  font-size: 12px;
  font-weight: 600;
  overflow-wrap: anywhere;
  em {
    color: var(--ds-text-muted);
    font-family: var(--ds-font-display);
    font-style: normal;
    font-weight: 400;
    font-variant-numeric: tabular-nums;
  }
}
.r2modman-path,
.r2modman-hint {
  margin: 6px 0;
  color: var(--ds-text-secondary);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.r2modman-path { font-family: var(--ds-font-mono); }
.r2modman-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 22px 0;
  color: var(--ds-text-muted);
  p { margin: 0; }
}
.r2modman-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(118px, 100%), 1fr));
  gap: 12px;
}
.r2modman-error {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin: 10px 0;
  border: 1px solid var(--ds-error-border);
  border-radius: 4px;
  background: var(--ds-error-bg);
  padding: 8px;
  color: var(--ds-error);
  span { min-width: 0; flex: 1 1 180px; overflow-wrap: anywhere; }
}
.r2modman-dialog-body { max-height: 60vh; overflow-y: auto; overflow-wrap: anywhere; }
.r2modman-dialog-body h3 { margin: 0; font-size: 13px; font-weight: 600; }
.r2modman-sync-local {
  margin-bottom: 16px;
  border-bottom: 1px solid var(--ds-border-subtle);
  padding-bottom: 12px;
  p { margin: 4px 0 0; color: var(--ds-text-secondary); }
}
.r2modman-profile-list { display: block; width: 100%; min-width: 0; }
.r2modman-profile-row {
  border-bottom: 1px solid var(--ds-border-subtle);
  padding: 10px 0;
  :deep(.n-radio) { max-width: 100%; white-space: normal; }
  :deep(.n-radio__label) { min-width: 0; overflow-wrap: anywhere; }
}
.r2modman-details {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr);
  gap: 10px 12px;
  margin: 0;
  dt { color: var(--ds-text-muted); }
  dd { min-width: 0; margin: 0; color: var(--ds-text-primary); overflow-wrap: anywhere; }
  .r2modman-warning { color: var(--ds-warning); }
}
.r2modman-description { white-space: pre-wrap; }
.r2modman-dependencies { margin: 0; padding-left: 16px; }

</style>
