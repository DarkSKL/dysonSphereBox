<script setup lang="ts">
import {computed} from 'vue'
import type {ClassifiedAddr, MultiplayerState} from '../../types/multiplayer'
// =============== 网络诊断投影 ===============
// 说明：只展示 Go 的观测事实与服务端授权；不推断 NAT 必然失败，不自动改变连接方式。
const props = defineProps<{state: MultiplayerState | null; online: boolean}>()
const network = computed(() => props.state?.network)
// 说明：仅按服务端能力展示中继政策，默认关闭；预约限额用于理解游戏连接何时可能需要重连。
const relayModeText = computed(() => {
  const caps = props.state?.session?.capabilities
  if (!caps?.nativeDcutr || !caps?.relayV2) return '中继未开启，尝试直接连接'
  return caps.relayFallback ? '允许中继游戏连接，后台尝试升级直连' : '中继仅用于直连协商，升级失败不会转发游戏'
})
const relayReservations = computed(() => props.state?.session?.relays ?? [])
const diagnostics = computed(() => {
  const net = network.value
  if (!net || !props.online) return []
  const items: string[] = []
  if (!net.firewallRule) items.push('Windows 防火墙已开启且未允许盒子入站，作为房主时成员可能连不上；请在防火墙中允许 DysonSphereBox（开发构建为 DysonSphereBox-dev.exe），或使用安装包安装')
  if (!net.ipv6) items.push('本机没有公网 IPv6 地址')
  if (!net.upnp) items.push('路由器未提供 UPnP 端口映射，可尝试原生直连升级或管理员授权的中继')
  return items
})

// 说明：两个观测节点看到的 QUIC 映射不同时显示诊断提示，不等加入房间；natConsistent 为 null 时不提示。
const selfSymmetric = computed(() => props.online && network.value?.natConsistent === false)
const natMappingsText = computed(() => (network.value?.natMappings ?? []).join(' / '))

// 说明：地址分组展示（本机上报、observed、房主下发），按 IP 标注私网/公网；房主地址只在成员侧有值。
const addrKindTexts: Record<string, string> = {public: '公网', private: '私网', loopback: '回环', cgnat: 'CGNAT', unknown: '未知', relay: '中继'}
const addrGroups = computed(() => {
  const net = network.value
  if (!net || !props.online) return []
  const groups: {key: string; title: string; addrs: ClassifiedAddr[]; empty: string; errors: string[]}[] = [
    {key: 'reported', title: '本机上报的地址', addrs: net.reportedAddrs ?? [], empty: '尚未上报', errors: []},
    {key: 'observed', title: 'observed 地址（观测节点看到的本机地址）', addrs: net.observed ?? [], empty: '未拿到', errors: net.observeErrors ?? []},
  ]
  if (props.state?.room && !(props.state?.room?.role === 'host')) groups.push({key: 'host', title: '房主下发的地址', addrs: net.hostAddrs ?? [], empty: '房主没有下发地址', errors: []})
  return groups
})
const hostNoPublic = computed(() => !!network.value?.hostNoPublic && !!props.state?.room && !(props.state?.room?.role === 'host'))

</script>
<template>
  <!-- ~ 折叠诊断：地址、NAT 观测、防火墙与中继政策；大厅先突出房间操作 -->
  <section v-if="online" class="network-panel" aria-label="网络诊断">
    <details class="addr-diagnostics" :open="hostNoPublic">
      <summary>连接诊断与网络地址<span class="summary-meta">连接遇到问题时查看</span></summary>
    <!-- * NAT 诊断：本机两个观测映射不同，显示观测事实，不断言 NAT 类型或升级结果 -->
    <div v-if="selfSymmetric" class="notice notice-warning" role="status">
      <p>观测节点看到的 NAT 映射随目标变化，原生直连升级可能受限；这不代表必然失败。服务器允许中继回退时仍可尝试联机</p>
      <p v-if="natMappingsText" class="nat-mappings">观测节点看到的映射：<code>{{ natMappingsText }}</code></p>
    </div>
    <ul v-if="diagnostics.length" class="diagnostics" aria-label="网络诊断">
      <li v-for="item in diagnostics" :key="item">{{ item }}</li>
    </ul>
    <!-- * 网络地址：本机上报、observed、房主下发，按 IP 标注私网/公网；用于判断地址能否从外网访问 -->
    <p v-if="hostNoPublic" class="notice notice-warning" role="alert">房主没有直接公网地址：检查观测节点、安全组或路由器 UPnP；服务器已授权的中继可作为另一条连接路径</p>
      <!-- ~ 中继能力与预约 -->
      <!-- * 说明：展示服务端授权模式和有限资源；交互：随会话更新；依赖：session capabilities/relays；边界：能力缺失显示关闭，不表示用户可自行选择公共中继 -->
      <div class="addr-group">
        <h3>连接方式</h3>
        <p class="hint">{{ relayModeText }}</p>
        <p v-for="reservation in relayReservations" :key="reservation.peerId" class="hint">
          {{ reservation.expiresAtMs ? `预约至 ${new Date(reservation.expiresAtMs).toLocaleTimeString('zh-CN', {hour12: false})}` : '尚未预约' }}
          <template v-if="reservation.limitDurationSeconds"> · 每次中继连接 {{ reservation.limitDurationSeconds }} 秒 / {{ Math.floor(reservation.limitDataBytes / 1048576) }} MiB</template>
          <template v-if="reservation.error"> · {{ reservation.error }}</template>
        </p>
      </div>
      <div v-for="group in addrGroups" :key="group.key" class="addr-group">
        <h3>{{ group.title }}</h3>
        <ul v-if="group.addrs.length" class="addr-list">
          <li v-for="item in group.addrs" :key="item.addr" :title="item.addr">
            <span class="addr-kind" :class="`kind-${item.kind}`">{{ addrKindTexts[item.kind] ?? item.kind }}</span>
            <code>{{ item.display }}</code>
          </li>
        </ul>
        <p v-else class="hint">{{ group.empty }}</p>
        <p v-for="error in group.errors" :key="error" class="hint addr-error">{{ error }}</p>
      </div>
    </details>

  </section>
</template>
<style scoped lang="scss">
.network-panel { margin-top: 24px; border-top: 1px solid var(--ds-border-subtle); }
.summary-meta { margin-left: auto; color: var(--ds-text-muted); font-size: 12px; font-weight: 400; }
.addr-diagnostics summary { padding-block: 16px; color: var(--ds-text-primary); font-weight: 700; }
code { font-family: var(--ds-font-mono); font-size: 12px; overflow-wrap: anywhere; }
.hint { color: var(--ds-text-secondary); font-size: 12px; line-height: 1.8; }
.notice { margin: 12px 0 0; padding: 8px 12px; border: 1px solid; border-radius: 4px; font-size: 12px; }
.notice-error { color: var(--ds-error); background: var(--ds-error-bg); border-color: var(--ds-error-border); }
.notice-warning { color: var(--ds-warning); background: var(--ds-warning-bg); border-color: var(--ds-warning-border); }
.notice p { margin: 0; }
.notice .nat-mappings { margin-top: 4px; }
.diagnostics { margin: 12px 0 0; padding-left: 18px; color: var(--ds-text-secondary); font-size: 12px; line-height: 1.8; }
.addr-diagnostics { margin: 12px 0 0; font-size: 12px; color: var(--ds-text-secondary); }
.addr-diagnostics summary { cursor: pointer; }
.addr-group h3 { margin: 10px 0 4px; font-size: 12px; }
.addr-list { display: flex; flex-wrap: wrap; gap: 6px 14px; margin: 0; padding: 0; list-style: none; }
.addr-list li { display: inline-flex; align-items: center; gap: 6px; }
.addr-kind { padding: 0 5px; border: 1px solid var(--ds-border-default); border-radius: 3px; font-size: 11px; }
.kind-public { color: var(--ds-success); border-color: var(--ds-success-border); }
.kind-private, .kind-cgnat { color: var(--ds-warning); border-color: var(--ds-warning-border); }
.addr-error { margin: 4px 0 0; word-break: break-all; }

</style>
