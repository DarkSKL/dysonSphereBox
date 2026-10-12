<script setup lang="ts">
import {CircleHelp, PlugZap, UserRound, Users} from '@lucide/vue'
import {useDialog, useMessage} from 'naive-ui'
import {computed, ref, watch} from 'vue'
import LobbyBrowser from '../../components/multiplayer/lobby-browser.vue'
import JoinDialog from '../../components/multiplayer/join-dialog.vue'
import CreateDialog from '../../components/multiplayer/create-dialog.vue'
import CurrentRoom from '../../components/multiplayer/current-room.vue'
import NetworkDiagnostics from '../../components/multiplayer/network-diagnostics.vue'
import {useMultiplayerState} from '../../composables/multiplayer/useMultiplayerState'
import {codeText, connectionStateTexts, type LobbyQuery} from '../../types/multiplayer'

definePage({meta: {layout: 'main', title: '联机', keepAlive: true}})

// =============== 页面意图与连接反馈 ===============
// 说明：页面只编排大厅列表与创建/加入对话框，房间和网络状态始终来自 Go 投影。
const mp = useMultiplayerState()
const {state, lastAction, resolved, online, room, isHost, pending, roomBusy, query, lobbyReady, lobbyLoading, lobbyError} = mp
const message = useMessage()
const dialog = useDialog()
const createOpen = ref(false)
const joinOpen = ref(false)
const joinMode = ref<'invite' | 'room'>('invite')
const joinTarget = ref('')
const connectionState = computed(() => state.value?.connection.state ?? 'offline')
const connectionText = computed(() => state.value ? connectionStateTexts[connectionState.value] ?? '未连接' : '读取状态')
const connectionError = computed(() => {
  const code = state.value?.connection.lastErrorCode ?? 0
  return code ? codeText(code) : ''
})
const connectionTitle = computed(() => !state.value ? '正在读取联机状态' : !state.value.account.loggedIn ? '登录后，开启联机旅程' : connectionState.value === 'reconnecting' ? '正在重新连接大厅' : connectionState.value === 'connecting' ? '正在连接联机大厅' : '连接大厅，找到你的同行者')
const connectionHint = computed(() => !state.value ? '请在戴森球盒子桌面应用中使用联机。' : !state.value.account.loggedIn ? '点击左侧头像登录账号，再连接大厅。' : connectionState.value === 'reconnecting' ? '连接暂时中断，盒子正在自动重试。连接恢复后会重新读取房间。' : connectionState.value === 'connecting' ? '连接成功后，可浏览公开房间或通过邀请码加入好友。' : connectionError.value || '连接后可浏览公开房间，也可以创建房间邀请好友。')
const maxPorts = computed(() => state.value?.session?.limits.maxPorts || 8)
const maxMembers = computed(() => state.value?.session?.limits.maxMembers || 16)

watch(lastAction, (action) => {
  if (!action) return
  if (action.ok) message.success(action.text)
  else message.error(action.text)
})
// 说明：目标变化即使尚在 resolve 请求中也使旧预览失效，防止使用 A 房间密码加入 B 房间。
watch(joinTarget, () => mp.clearResolved(), {flush: 'sync'})
// 说明：进入房间（创建或加入成功）后收起对话框；断线时关闭加入对话框，创建草稿仍保留在组件内。
watch(room, (next) => { if (next) { createOpen.value = false; joinOpen.value = false } })
watch(online, (next) => { if (!next) joinOpen.value = false })
// 说明：关闭大厅选房对话框时放弃该房间目标，邀请码草稿则保留供下次继续输入。
watch(joinOpen, (open) => { if (!open && joinMode.value === 'room') joinTarget.value = '' })

// =============== 大厅搜索与房间选择 ===============
// 说明：服务器负责筛选和分页；明确提交搜索，修改排序或每页条数回到第一页。
function search(keyword: string): void {
  if (!online.value) return
  if (query.keyword === keyword && query.page === 1) { void mp.refreshLobby(); return }
  query.keyword = keyword
  query.page = 1
}
function sort(value: LobbyQuery['sort']): void { if (!online.value) return; query.sort = value; query.page = 1 }
function pageSize(value: number): void { if (!online.value) return; query.pageSize = value; query.page = 1 }
function openCreate(): void { if (online.value && !roomBusy.value) createOpen.value = true }
function openInvite(): void {
  if (!online.value || roomBusy.value) return
  if (joinMode.value === 'room') joinTarget.value = ''
  joinMode.value = 'invite'
  joinOpen.value = true
}
// 说明：大厅选房直接查询房间详情，在对话框中确认密码与网络地址共享后再加入。
async function openRoom(roomId: string): Promise<void> {
  if (!online.value || roomBusy.value) return
  joinMode.value = 'room'
  joinTarget.value = roomId
  joinOpen.value = true
  await mp.resolve(roomId)
}
async function preview(): Promise<void> {
  const target = joinTarget.value.trim()
  if (online.value && target) await mp.resolve(target)
}
async function join(password: string): Promise<void> { await mp.join(joinTarget.value.trim(), password) }

// =============== 房主关闭房间 ===============
// 说明：关闭会使所有成员退出，因此在实际操作前说明后果；控制链路始终保持，不提供主动断开。
function closeRoom(): void {
  if (!online.value || roomBusy.value) return
  dialog.warning({title: '关闭当前房间？', content: '所有成员都会退出房间，当前游戏连接也会断开。', positiveText: '关闭房间', negativeText: '继续联机', onPositiveClick: async () => { await mp.close() }})
}
</script>

<template>
  <main class="mp-page" aria-labelledby="mp-title">
    <!-- ~ 页头：左侧标题，右侧连接状态与账号；离开此页面不会主动退出房间 -->
    <header class="mp-header">
      <div class="title-block"><h1 id="mp-title">多人联机</h1><p class="page-subtitle">浏览公开房间，或用邀请码加入好友的星系。</p></div>
      <div class="header-status" aria-live="polite">
        <span v-if="room" class="in-room-label"><Users :size="14" />{{ isHost ? '你是房主' : '已加入房间' }}</span>
        <span class="state-label" :class="`state-${connectionState}`"><i />{{ connectionText }}</span>
        <span v-if="state?.account.nickname" class="account-name" :title="state.account.nickname"><UserRound :size="14" />{{ state.account.nickname }}</span>
        <!-- * 控制链路强制保持：连接中/重连中由 Go 自动重试，仅在不可重连的原因停止后提供重新连接 -->
        <n-button v-if="connectionState === 'offline'" size="small" secondary :disabled="!state?.account.loggedIn" :loading="pending.has('connect')" @click="mp.connect()"><template #icon><PlugZap :size="14" /></template>重新连接</n-button>
      </div>
    </header>

    <!-- ~ 操作阻塞提示：未连接、连接中及重连中统一禁用业务操作，连接入口保持可用 -->
    <p v-if="!online" class="connection-notice" role="status"><CircleHelp :size="15" /><span>控制链路未建立，相关操作已禁用。<template v-if="connectionError">{{ connectionError }}。</template><template v-if="connectionState === 'reconnecting'">正在自动重试，请检查网络或服务器设置。</template><template v-else-if="connectionState === 'connecting'">正在连接大厅，请稍候。</template><template v-else-if="state?.account.loggedIn">请先连接大厅。</template><template v-else-if="state">请先登录账号，再连接大厅。</template></span></p>

    <!-- ~ 当前房间：突出游戏连接地址、成员与邀请，隐藏大厅列表 -->
    <CurrentRoom v-if="room && state" :state="state" :pending="pending" :online="online" :busy="roomBusy" @invite="mp.invite()" @close="closeRoom" @leave="mp.leave()" @retry="mp.retry()" @kick="mp.kick($event)" />
    <LobbyBrowser v-else :state="state" :query="query" :online="online" :loading="lobbyLoading" :ready="lobbyReady" :error="lobbyError" :selected="joinOpen && joinMode === 'room' ? joinTarget : ''" :busy="roomBusy" :connection-title="connectionTitle" :connection-hint="connectionHint" @search="search" @sort="sort" @page="query.page = $event" @page-size="pageSize" @join="openRoom" @refresh="mp.refreshLobby()" @create="openCreate" @invite="openInvite" @connect="mp.connect()" />

    <!-- ~ 技术诊断：在末尾按需展开 -->
    <NetworkDiagnostics :state="state" :online="online" />

    <!-- ~ 对话框：创建与加入不占用列表空间；创建草稿在关闭后保留 -->
    <CreateDialog v-model:show="createOpen" :online="online" :loading="pending.has('createRoom')" :busy="roomBusy" :max-members="maxMembers" :max-ports="maxPorts" @create="mp.create($event)" />
    <JoinDialog v-model:show="joinOpen" v-model:target="joinTarget" :mode="joinMode" :resolved="resolved" :online="online" :resolving="pending.has('resolveRoom')" :joining="pending.has('joinRoom')" :busy="roomBusy" @preview="preview" @join="join" />
  </main>
</template>

<style scoped lang="scss">
.mp-page { width: 100%; max-width: 1680px; min-height: 100%; margin-inline: auto; padding: var(--ds-page-padding-top) var(--ds-page-padding-x) var(--ds-page-padding-bottom); color: var(--ds-text-primary); }
.mp-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; margin-bottom: 28px; }
.title-block { min-width: 0; }
h1 { margin: 0; font-size: 26px; font-weight: 700; line-height: 1.4; }
.page-subtitle { margin: 6px 0 0; font-size: 13px; color: var(--ds-text-secondary); }
.header-status { display: flex; align-items: center; flex-wrap: wrap; justify-content: flex-end; gap: 8px 16px; flex-shrink: 0; padding-top: 6px; }
.in-room-label, .account-name { display: inline-flex; align-items: center; gap: 6px; color: var(--ds-text-secondary); font-size: 12px; }
.account-name { max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.account-name svg { flex-shrink: 0; }
.state-label { display: inline-flex; align-items: center; gap: 8px; padding: 2px 10px; border: 1px solid var(--ds-border-default); border-radius: 4px; color: var(--ds-text-secondary); font-size: 12px; white-space: nowrap; }
.state-label i { width: 6px; height: 6px; border-radius: 50%; background: var(--ds-text-muted); }
.state-online { color: var(--ds-success); border-color: var(--ds-success-border); } .state-online i { background: var(--ds-success); }
.state-connecting, .state-reconnecting { color: var(--ds-warning); border-color: var(--ds-warning-border); } .state-connecting i, .state-reconnecting i { background: var(--ds-warning); }
.connection-notice { display: flex; align-items: flex-start; gap: 8px; padding: 12px 14px; margin: 0 0 24px; background: var(--ds-warning-bg); color: var(--ds-warning); border: 1px solid var(--ds-warning-border); border-radius: 4px; font-size: 12px; line-height: 1.8; }
.connection-notice svg { flex-shrink: 0; margin-top: 3px; }
@media (max-width: 760px) { .mp-header { flex-direction: column; gap: 12px; } .header-status { justify-content: flex-start; padding-top: 0; } h1 { font-size: 22px; } }
</style>
