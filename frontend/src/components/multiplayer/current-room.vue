<script setup lang="ts">
import {Copy, Crown, Link2, LogOut, Monitor, RotateCcw, Server, UserPlus, UserX} from '@lucide/vue'
import {useMessage} from 'naive-ui'
import {computed} from 'vue'
import {ClipboardSetText} from '../../../wailsjs/runtime/runtime'
import {linkErrorText, linkPathTexts, linkStateTexts, type LinkView, type MultiplayerState, type RoomMember} from '../../types/multiplayer'
import {portText, protoLabel} from '../../utils/multiplayer'
// =============== 当前房间与用户操作 ===============
// 说明：呈现真实快照与链路，房间变更交给页面；复制仅写剪贴板，不启动游戏。
const props = defineProps<{state: MultiplayerState; pending: Set<string>; online: boolean; busy: boolean}>()
const emit = defineEmits<{(event: 'invite' | 'leave' | 'retry'): void; (event: 'kick', userId: number): void}>()
const room = computed(() => props.state.room)
const isHost = computed(() => room.value?.role === 'host')
const message = useMessage()
// 说明：控制链路离线时不复制旧会话的邀请码或连接地址；在线时仅写入剪贴板。
async function copy(text: string): Promise<void> {
  if (!props.online) return
  try { await ClipboardSetText(text); message.success('已复制') }
  catch { message.error('复制失败，请手动复制地址') }
}
const snapshot = computed(() => props.state.room?.snapshot)
const linksByUser = computed(() => new Map((props.state?.links ?? []).map((link) => [link.userId, link])))
const selfLink = computed<LinkView | undefined>(() => (!isHost.value ? props.state?.links?.[0] : undefined))
const localPorts = computed(() => props.state?.localPorts ?? [])
const targets = computed(() => props.state.network?.targets ?? [])

type LinkLike = Pick<LinkView, 'state' | 'path' | 'ipFamily' | 'rttMs' | 'error'>

// 说明：房主侧优先用投影 links，其次用快照中成员最近一次 link.report。
const memberLink = (member: RoomMember): LinkLike | undefined => linksByUser.value.get(member.userId) ?? member.link ?? undefined

// 说明：链路状态与路径分开呈现：状态决定颜色，路径/协议族/延迟作为次要信息。
function linkTone(link?: LinkLike): string {
  if (!props.online) return 'offline'
  return link?.state ?? 'connecting'
}
function linkStateLabel(link?: LinkLike): string {
  if (!props.online) return '等待同步'
  if (!link) return '等待上报'
  return linkStateTexts[link.state] ?? link.state
}
function linkRoute(link?: LinkLike): string {
  if (!link) return ''
  const parts: string[] = []
  if (link.path) parts.push(linkPathTexts[link.path] ?? link.path)
  if (link.ipFamily) parts.push(link.ipFamily === 'v6' ? 'IPv6' : 'IPv4')
  return parts.join(' · ')
}

function targetTone(status?: string): string {
  if (!props.online) return 'offline'
  return status === 'ok' ? 'connected' : status === 'refused' ? 'failed' : 'idle'
}
function targetText(status?: string, address?: string): string {
  if (!props.online) return '等待大厅连接恢复'
  if (status === 'ok') return `游戏已在监听 · ${address}`
  if (status === 'refused') return '未检测到游戏监听'
  return '尚未探测'
}

// 说明：本地端口分配与实际链路可用是不同状态，始终以链路投影提示能否连接游戏。
const connectionHint = computed(() => {
  if (!props.online) return '大厅连接已中断，请等待恢复后再连接游戏。'
  if (selfLink.value?.state === 'connected') return '在游戏的联机界面填写下方地址，即可连接房主。'
  if (selfLink.value?.state === 'failed') return '尚未连通房主，请重试连接；仍有问题时可查看右上角诊断。'
  if (selfLink.value?.state === 'reconnecting') return '与房主的连接已中断，正在重新连接，请稍候。'
  return '正在连接房主，连接成功后再使用下方地址进入游戏。'
})
const formatTime = (ms: number): string => new Date(ms).toLocaleString('zh-CN', {hour12: false, month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'})
const selfLinkError = computed(() => (selfLink.value?.error ? linkErrorText(selfLink.value.error) : null))

// =============== 座位网格 ===============
// 说明：参考游戏房间常见的"座位"布局，按人数上限补齐空位；房主固定首位，成员按快照顺序就座。
// 边界：空位只是视觉占位，不代表可邀请名额已预留；人数上限以快照 maxMembers 为准。
interface Seat {readonly key: string; readonly member?: RoomMember; readonly host?: boolean; readonly name?: string; readonly self?: boolean}
const seats = computed<Seat[]>(() => {
  const s = snapshot.value
  if (!s) return []
  const selfId = props.state.account.userId
  const list: Seat[] = [{key: 'host', host: true, name: s.host.nickname, self: s.host.userId === selfId}]
  for (const member of s.members) list.push({key: `m${member.userId}`, member, name: member.nickname, self: member.userId === selfId})
  for (let i = list.length; i < s.maxMembers; i++) list.push({key: `empty${i}`})
  return list
})
const initial = (name = ''): string => Array.from(name.trim())[0]?.toUpperCase() ?? '?'
// 说明：延迟按常见游戏阈值分三档，仅影响信号格颜色。
function rttLevel(ms?: number): number {
  if (!ms) return 0
  return ms < 80 ? 3 : ms < 180 ? 2 : 1
}
const selfTone = computed(() => isHost.value ? 'host' : linkTone(selfLink.value))
const selfStateText = computed(() => {
  if (isHost.value) return '房主'
  if (!props.online) return '等待大厅恢复'
  return selfLink.value ? linkStateLabel(selfLink.value) : '等待连接房主'
})

// =============== 链路波动线 ===============
// 说明：以"本机 ～ 路径 ～ 对端"的波动线表现链路状态：连通时波形流动，连接中缓慢起伏，失败/离线/空闲时为平直虚线。
// 边界：动画只反映投影中的链路状态，不额外探测；离线时流动停止并置灰。
// 说明：房主侧汇总成员链路：全部连通为 connected，有失败为 failed，否则连接中；无成员时为空闲。
const flowTone = computed(() => {
  if (!props.online) return 'offline'
  if (!isHost.value) return linkTone(selfLink.value)
  const states = (snapshot.value?.members ?? []).map((member) => memberLink(member)?.state ?? 'connecting')
  if (!states.length) return 'idle'
  if (states.every((state) => state === 'connected')) return 'connected'
  if (states.some((state) => state === 'failed')) return 'failed'
  return 'connecting'
})
const connectedCount = computed(() => (snapshot.value?.members ?? []).filter((member) => memberLink(member)?.state === 'connected').length)
const flowLabel = computed(() => {
  if (!props.online) return '等待大厅恢复'
  if (isHost.value) return snapshot.value?.members.length ? `${connectedCount.value} / ${snapshot.value.members.length} 名成员已连通` : '等待成员加入'
  return linkRoute(selfLink.value) || linkStateLabel(selfLink.value)
})
// 说明：生成 0~240 宽、周期 40 的正弦路径（共 6 个周期），供 CSS 平移一个视口宽度做无缝循环；振幅由 CSS scaleY 按状态控制。
const wavePath = (() => {
  const points: string[] = []
  for (let x = 0; x <= 240; x += 2) points.push(`${x},${(12 + Math.sin((x / 40) * Math.PI * 2) * 8).toFixed(2)}`)
  return `M${points.join(' L')}`
})()
const flowRtt = computed(() => {
  if (!props.online) return 0
  if (!isHost.value) return selfLink.value?.state === 'connected' ? selfLink.value.rttMs : 0
  const values = (snapshot.value?.members ?? []).map((member) => memberLink(member)).filter((link) => link?.state === 'connected' && link.rttMs).map((link) => link!.rttMs)
  return values.length ? Math.round(values.reduce((sum, value) => sum + value, 0) / values.length) : 0
})
</script>

<template>
  <!-- * 说明：参考游戏启动器的房间布局——页头展示房间身份，顶部横幅展示进入游戏信息，下方通栏座位网格（含空位），底部操作栏 -->
  <!-- * 交互：房间变更通过事件交给页面；复制只写剪贴板，不启动游戏 -->
  <!-- * 依赖：Go 投影的 room.snapshot、links、localPorts 与 network.targets -->
  <!-- * 边界：大厅离线时状态统一显示为等待恢复，不展示旧会话推断的连接结果 -->
  <section v-if="snapshot" class="room" aria-label="当前房间">
    <!-- * 进入游戏横幅：链路波动线、连接地址/端口与邀请码；房间身份在页面页头 -->
    <header class="room-banner">
      <!-- * 进入游戏：左上角为监听端口/本地地址，下方为提示、链路波动线与（房主）邀请码 -->
      <section class="banner-connect" :aria-label="isHost ? '游戏端口' : '进入游戏'">
        <!-- * 左上角为自身角色/链路状态；右上角为监听端口：房主为游戏端口与监听探测，成员为本地转发地址（点击复制） -->
        <div class="connect-head">
          <span class="connect-state" :class="`tone-${selfTone}`"><i />{{ selfStateText }}</span>
          <ul class="port-chips" :aria-label="isHost ? '房主游戏端口' : '游戏连接地址'">
            <template v-if="isHost">
              <li v-for="(decl, index) in snapshot.ports" :key="index" class="port-chip" :class="`tone-${targetTone(targets[index]?.status)}`" :title="`${decl.label || '游戏端口'} · ${targetText(targets[index]?.status, targets[index]?.address)}`">
                <i /><code class="type-mono">{{ portText(decl).replace(' ', '/') }}</code><small>{{ targets[index]?.status === 'ok' ? '监听中' : targets[index]?.status === 'refused' ? '未监听' : online ? '未探测' : '等待' }}</small>
              </li>
            </template>
            <template v-else>
              <li v-if="!localPorts.length" class="port-chip tone-idle"><i /><small>正在分配本地地址…</small></li>
              <li v-for="item in localPorts" :key="item.proto + item.address">
                <button type="button" class="port-chip is-copy" :disabled="!online" :title="`${item.label || '游戏端口'} · 点击复制`" :aria-label="`复制连接地址 ${item.address}`" @click="copy(item.address.split('-')[0])">
                  <code class="type-mono">{{ protoLabel(item.proto) }}/{{ item.address }}</code><Copy :size="13" />
                </button>
              </li>
            </template>
          </ul>
        </div>
        <p class="connect-hint">{{ isHost ? '先在游戏中开启联机，盒子会探测游戏是否正在监听。' : connectionHint }}</p>
        <!-- * 链路流动线：本机与对端之间的数据流动画，颜色随链路状态变化；连通时粒子流动，连接中时脉冲，失败/离线时静止 -->
        <div class="link-flow" :class="`flow-${flowTone}`" role="img" :aria-label="`链路状态：${flowLabel}${flowRtt ? `，平均延迟 ${flowRtt} 毫秒` : ''}`">
          <span class="flow-node"><Monitor :size="16" /><small>本机</small></span>
          <span class="flow-track" aria-hidden="true">
            <!-- * 波形为两倍宽的重复正弦路径，向左平移一个周期实现无缝滚动；两层不同速度叠加出波动感 -->
            <span class="wave-box"><svg class="wave" viewBox="0 0 240 24" preserveAspectRatio="none"><path class="wave-a" :d="wavePath" /><path class="wave-b" :d="wavePath" /></svg></span>
            <b>{{ flowLabel }}<template v-if="flowRtt"> · {{ flowRtt }} ms</template></b>
          </span>
          <span class="flow-node"><Server v-if="!isHost" :size="16" /><UserPlus v-else :size="16" /><small>{{ isHost ? '成员' : '房主' }}</small></span>
        </div>
        <!-- * 邀请码：仅房主可见，点击复制 -->
        <div v-if="isHost" class="invite-row">
          <button v-if="room?.invite" type="button" class="invite-code type-mono" :disabled="!online" :aria-label="`复制邀请码 ${room.invite.code}`" :title="`有效至 ${formatTime(room.invite.expiresAtMs)}`" @click="copy(room.invite.code)"><small>邀请码</small>{{ room.invite.code }}<Copy :size="14" /></button>
          <span v-else class="invite-code is-placeholder"><small>邀请码</small>尚未生成</span>
          <n-button size="small" :type="room?.invite ? 'default' : 'primary'" :secondary="!!room?.invite" :disabled="!online || busy" :loading="pending.has('invite')" @click="emit('invite')"><template #icon><Link2 :size="14" /></template>{{ room?.invite ? '更新' : '生成' }}</n-button>
        </div>
      </section>
    </header>

    <!-- * 链路失败提示与原始错误：横幅下方通栏，避免挤压连接区 -->
    <p v-if="selfLinkError" class="link-error" role="alert"><strong>{{ selfLinkError.title }}</strong>{{ selfLinkError.hint }}</p>
    <details v-if="selfLink?.detail && selfLink.state !== 'connected'" class="link-detail">
      <summary>查看最近一次连接错误</summary><code>{{ selfLink.detail }}</code>
    </details>

    <!-- * 座位网格：房主固定首位并带皇冠，自己高亮；空位显示虚线占位，占满整行宽度 -->
    <section class="seats-section" aria-labelledby="seats-title">
      <h3 id="seats-title" class="seats-title">房间成员</h3>
      <div class="seats">
        <article v-for="seat in seats" :key="seat.key" class="seat" :class="{'is-empty': !seat.name, 'is-host': seat.host, 'is-self': seat.self}">
          <template v-if="seat.name">
            <div class="seat-avatar" aria-hidden="true">{{ initial(seat.name) }}<Crown v-if="seat.host" class="seat-crown" :size="14" /></div>
            <div class="seat-info">
              <strong :title="seat.name">{{ seat.name }}<span v-if="seat.self" class="self-tag">我</span></strong>
              <template v-if="seat.host"><span class="seat-state tone-host">房主</span></template>
              <template v-else-if="seat.member">
                <span class="seat-state" :class="`tone-${linkTone(memberLink(seat.member))}`"><i />{{ linkStateLabel(memberLink(seat.member)) }}<small v-if="online && linkRoute(memberLink(seat.member))">{{ linkRoute(memberLink(seat.member)) }}</small></span>
                <small v-if="online && memberLink(seat.member)?.error" class="seat-error">{{ linkErrorText(memberLink(seat.member)?.error ?? 0)?.title }}</small>
              </template>
            </div>
            <div v-if="seat.member && online && memberLink(seat.member)?.rttMs" class="seat-ping" :class="`ping-${rttLevel(memberLink(seat.member)?.rttMs)}`" :title="`延迟 ${memberLink(seat.member)?.rttMs} ms`">
              <span class="bars" aria-hidden="true"><i /><i /><i /></span><span class="type-display">{{ memberLink(seat.member)?.rttMs }}</span>
            </div>
            <n-tooltip v-if="isHost && seat.member"><template #trigger><n-button class="seat-kick" size="tiny" quaternary type="error" :aria-label="`移出 ${seat.name}`" :disabled="!online || busy" :loading="pending.has('kick')" @click="emit('kick', seat.member.userId)"><template #icon><UserX :size="14" /></template></n-button></template>移出房间</n-tooltip>
          </template>
          <template v-else>
            <div class="seat-avatar" aria-hidden="true"><UserPlus :size="16" /></div>
            <span class="seat-empty-text">等待加入</span>
          </template>
        </article>
      </div>
    </section>

    <!-- * 底部操作栏：左侧房间摘要，右侧重试与离开/关闭，危险操作始终在最右 -->
    <footer class="room-bar">
      <span class="bar-summary">房主 {{ snapshot.host.nickname }}<span class="sep">·</span>{{ snapshot.visibility === 'public' ? '公开大厅 / 邀请码加入' : '仅限邀请码加入' }}</span>
      <div class="bar-actions">
        <n-button v-if="!isHost && selfLink && selfLink.state !== 'connected'" secondary :disabled="!online || busy" :loading="pending.has('retry')" @click="emit('retry')"><template #icon><RotateCcw :size="15" /></template>重试连接</n-button>
        <n-button v-if="!isHost" secondary :disabled="!online || busy" :loading="pending.has('leave')" @click="emit('leave')"><template #icon><LogOut :size="15" /></template>退出房间</n-button>
      </div>
    </footer>
  </section>
</template>

<style scoped lang="scss">
// 说明：游戏房间布局——横幅（身份 | 进入游戏）/ 通栏座位网格 / 底部操作栏；座位是重复项，允许使用细边框格子，但不嵌套卡片。
.room { display: flex; flex-direction: column; min-width: 0; container-type: inline-size; }
h3 { display: flex; align-items: center; gap: 8px; margin: 0; font-size: 14px; font-weight: 600; }
code { font-family: var(--ds-font-mono); font-variant-numeric: tabular-nums; }

// ---- 进入游戏横幅：房间身份已移至页头 ----
.room-banner { border: 1px solid var(--ds-border-default); border-radius: 4px; }
.banner-connect { display: flex; flex-direction: column; gap: 10px; min-width: 0; padding: 18px 20px; }
.connect-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.port-chips { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 6px; min-width: 0; margin: 0; padding: 0; list-style: none; }
.port-chip { display: inline-flex; align-items: center; gap: 8px; min-height: 30px; padding: 0 10px; border: 1px solid var(--ds-border-default); border-radius: 4px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-muted); font-size: 12px; white-space: nowrap; }
.port-chip.tone-connecting { border-color: var(--ds-warning-border); }
.port-chip code { color: var(--ds-text-primary); font-size: 13px; }
.port-chip small { font-size: 12px; }
.port-chip i { width: 6px; height: 6px; flex: none; border-radius: 50%; background: currentColor; }
.port-chip.tone-connected { border-color: var(--ds-success-border); }
.port-chip.tone-failed { border-color: var(--ds-error-border); }
.port-chip.is-copy { cursor: pointer; transition: background-color 120ms ease-out, border-color 120ms ease-out; }
.port-chip.is-copy svg { color: var(--ds-text-muted); }
.port-chip.is-copy:hover:not(:disabled) { border-color: var(--ds-border-strong); background: var(--ds-hover-bg); }
.port-chip.is-copy:focus-visible { outline: 2px solid var(--ds-focus-ring); outline-offset: 2px; }
.port-chip.is-copy:disabled { cursor: not-allowed; opacity: .6; }
// 说明：左侧状态标签与右侧端口标签共用同一套芯片规格（高 30px、4px 圆角、面板底、6px 状态点），仅颜色随状态变化。
.connect-state { display: inline-flex; flex: none; align-items: center; gap: 8px; min-height: 30px; padding: 0 10px; border: 1px solid var(--ds-border-default); border-radius: 4px; background: var(--ds-bg-panel-elevated); font-size: 12px; white-space: nowrap; }
.connect-state.tone-host { border-color: var(--ds-border-strong); }
.connect-state.tone-connected { border-color: var(--ds-success-border); }
.connect-state.tone-connecting, .connect-state.tone-reconnecting { border-color: var(--ds-warning-border); }
.connect-state.tone-failed { border-color: var(--ds-error-border); }
.connect-hint { margin: 0; color: var(--ds-text-secondary); font-size: 12px; line-height: 1.6; }
.invite-row { display: flex; align-items: stretch; gap: 8px; }
.invite-code { display: flex; flex: 1; align-items: center; gap: 10px; min-width: 0; min-height: 36px; padding: 0 10px; border: 1px dashed var(--ds-border-strong); border-radius: 4px; background: transparent; color: var(--ds-text-primary); font-size: 16px; letter-spacing: .1em; cursor: pointer; transition: background-color 120ms ease-out; }
.invite-code small { flex: none; color: var(--ds-text-muted); font-family: var(--ds-font-ui); font-size: 11px; letter-spacing: 0; }
.invite-code svg { flex: none; margin-left: auto; color: var(--ds-text-muted); }
.invite-code:hover:not(:disabled) { background: var(--ds-hover-bg); }
.invite-code:focus-visible { outline: 2px solid var(--ds-focus-ring); outline-offset: 2px; }
.invite-code:disabled { cursor: not-allowed; opacity: .6; }
.invite-code.is-placeholder { border-color: var(--ds-border-default); color: var(--ds-text-disabled); font-size: 12px; letter-spacing: 0; cursor: default; }
.invite-row :deep(.n-button) { height: auto; }

// ---- 链路波动线：SVG 正弦波横向滚动表示数据流；状态色与 tone-* 一致 ----
.link-flow { --flow-color: var(--ds-text-muted); display: flex; align-items: center; gap: 12px; padding: 4px 0; }
.flow-node { display: flex; flex: none; flex-direction: column; align-items: center; gap: 2px; color: var(--flow-color); }
.flow-node small { color: var(--ds-text-muted); font-size: 10px; }
.flow-track { display: flex; flex: 1; flex-direction: column; align-items: center; gap: 2px; min-width: 0; }
.wave-box { position: relative; width: 100%; height: 24px; overflow: hidden; -webkit-mask-image: linear-gradient(90deg, transparent, #000 10%, #000 90%, transparent); mask-image: linear-gradient(90deg, transparent, #000 10%, #000 90%, transparent); }
// 说明：SVG 宽度为轨道两倍，平移 -50% 恰好一个循环（路径为整数个周期），视觉上无接缝。
.wave { position: absolute; top: 0; left: 0; width: 200%; height: 100%; overflow: visible; }
.wave path { fill: none; stroke: var(--flow-color); stroke-linecap: round; vector-effect: non-scaling-stroke; transform-origin: center; transform-box: fill-box; transition: transform .4s ease-out, opacity .4s ease-out; }
.wave-a { stroke-width: 1.75; opacity: .95; }
.wave-b { stroke-width: 1; opacity: .4; }
.flow-track b { max-width: 100%; overflow: hidden; color: var(--ds-text-secondary); font-size: 11px; font-weight: 400; text-overflow: ellipsis; white-space: nowrap; }
.flow-connected { --flow-color: var(--ds-success); }
.flow-connecting, .flow-reconnecting { --flow-color: var(--ds-warning); }
.flow-failed { --flow-color: var(--ds-error); }
// 说明：连通——全振幅、两层不同速度滚动；连接中——小振幅、慢速并呼吸；失败/离线/空闲——压平为静止虚线。
.flow-connected .wave { animation: wave-run 2.6s linear infinite; }
.flow-connected .wave-b { transform: scaleY(-.6); }
.flow-connecting .wave, .flow-reconnecting .wave { animation: wave-run 5s linear infinite; }
.flow-connecting .wave path, .flow-reconnecting .wave path { animation: wave-breathe 1.6s ease-in-out infinite; }
.flow-connecting .wave-b, .flow-reconnecting .wave-b { animation-delay: -.8s; }
.flow-failed .wave path, .flow-offline .wave path, .flow-idle .wave path { transform: scaleY(.04); stroke-dasharray: 4 5; opacity: .6; }
.flow-failed .wave-b, .flow-offline .wave-b, .flow-idle .wave-b { opacity: 0; }
@keyframes wave-run { to { transform: translateX(-50%); } }
@keyframes wave-breathe { 0%, 100% { transform: scaleY(.25); } 50% { transform: scaleY(.6); } }
@media (prefers-reduced-motion: reduce) { .wave, .wave path { animation: none !important; } }
// ---- 链路错误（横幅下方通栏） ----
.link-error { display: flex; flex-wrap: wrap; gap: 4px 10px; margin: 12px 0 0; padding: 10px 12px; border: 1px solid var(--ds-warning-border); border-radius: 4px; background: var(--ds-warning-bg); color: var(--ds-warning); font-size: 12px; line-height: 1.7; }
.link-detail { margin-top: 8px; color: var(--ds-text-muted); font-size: 11px; }
.link-detail summary { cursor: pointer; }
.link-detail code { display: block; margin-top: 8px; padding: 10px; border-radius: 4px; background: var(--ds-bg-panel); white-space: pre-wrap; overflow-wrap: anywhere; }

// ---- 座位 ----
.seats-section { margin-top: 24px; }
.seats-title { margin-bottom: 12px; color: var(--ds-text-secondary); font-size: 12px; font-weight: 500; }
.seats { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 10px; min-width: 0; }
.seat { position: relative; display: flex; align-items: center; gap: 12px; min-height: 72px; padding: 12px 14px; border: 1px solid var(--ds-border-default); border-radius: 4px; background: var(--ds-bg-panel); transition: border-color 160ms ease-out, background-color 160ms ease-out; }

.seat.is-self { background: var(--ds-active-bg); }
.seat.is-empty { border-style: dashed; background: transparent; color: var(--ds-text-disabled); }
// 说明：已就座的座位沿用资源页启用卡片的流边效果——锥形渐变描边沿边框旋转流动，色值复用 --ds-res-on-* token；
// 房主座位改用品牌色环区分。边界：空位保持虚线占位，不显示流边；减少动态效果时环静止。
@property --seat-ring-angle { syntax: '<angle>'; inherits: false; initial-value: 0deg; }
@keyframes seat-ring-spin { to { --seat-ring-angle: 360deg; } }
.seat:not(.is-empty) { --seat-ring-a: var(--ds-res-on-ring-a); --seat-ring-b: var(--ds-res-on-ring-b); --seat-ring-c: var(--ds-res-on-ring-c); border-color: transparent; background: var(--ds-res-on-surface); box-shadow: var(--ds-res-on-glow); }
.seat:not(.is-empty)::before { position: absolute; inset: -1px; padding: 2px; border-radius: inherit; background: conic-gradient(from var(--seat-ring-angle), var(--seat-ring-a), var(--seat-ring-b) 25%, var(--seat-ring-c) 50%, var(--seat-ring-b) 75%, var(--seat-ring-a)); content: ''; pointer-events: none; animation: seat-ring-spin var(--ds-res-on-ring-duration) linear infinite; -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0); -webkit-mask-composite: xor; mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0); mask-composite: exclude; }
.seat:not(.is-empty):hover::before { animation-duration: calc(var(--ds-res-on-ring-duration) / 2); }
.seat.is-host { --seat-ring-a: var(--ds-brand); --seat-ring-b: var(--ds-brand-hover); --seat-ring-c: var(--ds-border-default); }
.seat.is-self { background: linear-gradient(var(--ds-active-bg), var(--ds-active-bg)), var(--ds-res-on-surface); }
.seat-kick { z-index: 1; }
@media (prefers-reduced-motion: reduce) { .seat::before { animation: none !important; } }
.seat-avatar { position: relative; display: grid; flex: none; place-items: center; width: 40px; height: 40px; border: 1px solid var(--ds-border-default); border-radius: 4px; background: var(--ds-bg-panel-elevated); color: var(--ds-text-secondary); font-size: 16px; font-weight: 700; }
.is-host .seat-avatar { color: var(--ds-brand); border-color: var(--ds-border-strong); }
.is-empty .seat-avatar { border: 0; background: transparent; color: var(--ds-text-disabled); }
.seat-crown { position: absolute; top: -8px; right: -7px; color: var(--ds-warning); }
.seat-info { display: flex; flex: 1; flex-direction: column; gap: 4px; min-width: 0; }
.seat-info strong { display: flex; align-items: center; gap: 6px; min-width: 0; overflow: hidden; color: var(--ds-text-primary); font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.self-tag { flex: none; padding: 0 5px; border: 1px solid var(--ds-border-default); border-radius: 3px; color: var(--ds-text-secondary); font-size: 10px; font-weight: 400; }
.seat-state { display: inline-flex; align-items: center; flex-wrap: wrap; gap: 6px; font-size: 11px; }
.seat-state small { color: var(--ds-text-muted); font-size: 11px; }
.seat-error { color: var(--ds-warning); font-size: 11px; }
.seat-empty-text { font-size: 12px; }
.seat-ping { display: flex; flex: none; flex-direction: column; align-items: flex-end; gap: 2px; color: var(--ds-text-secondary); font-size: 11px; }
.bars { display: flex; align-items: flex-end; gap: 2px; height: 12px; }
.bars i { width: 3px; background: var(--ds-border-default); border-radius: 1px; }
.bars i:nth-child(1) { height: 5px; } .bars i:nth-child(2) { height: 8px; } .bars i:nth-child(3) { height: 12px; }
.ping-3 .bars i { background: var(--ds-success); }
.ping-2 .bars i:nth-child(-n+2) { background: var(--ds-warning); }
.ping-1 .bars i:nth-child(1) { background: var(--ds-error); }
.seat-kick { position: absolute; top: 4px; right: 4px; opacity: 0; transition: opacity 120ms ease-out; }
.seat:hover .seat-kick, .seat:focus-within .seat-kick { opacity: 1; }
.seat:has(.seat-kick) .seat-ping { margin-right: 20px; }

// ---- 状态色 ----
.connect-state i, .seat-state i { width: 6px; height: 6px; flex: none; border-radius: 50%; background: currentColor; }
.tone-host { color: var(--ds-brand); }
.tone-connected { color: var(--ds-success); }
.tone-connecting, .tone-reconnecting { color: var(--ds-warning); }
.tone-connecting i, .tone-reconnecting i { animation: pulse 1.4s ease-in-out infinite; }
.tone-failed { color: var(--ds-error); }
.tone-offline, .tone-idle { color: var(--ds-text-muted); }
@keyframes pulse { 50% { opacity: .3; } }

// ---- 底部操作栏 ----
.room-bar { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; margin-top: 20px; padding-top: 16px; border-top: 1px solid var(--ds-border-subtle); }
.bar-summary { color: var(--ds-text-muted); font-size: 12px; }
.bar-summary .sep { margin: 0 8px; }
.bar-actions { display: flex; gap: 8px; margin-left: auto; }

@container (max-width: 520px) {
  .banner-connect { padding: 16px; }

  .seats { grid-template-columns: minmax(0, 1fr); }
}
@media (prefers-reduced-motion: reduce) { .seat, .seat-kick, .invite-code { transition: none; } .tone-connecting i, .tone-reconnecting i { animation: none; } }
</style>
