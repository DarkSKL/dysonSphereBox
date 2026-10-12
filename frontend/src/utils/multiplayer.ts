// =============== 联机显示格式 ===============
// 说明：只格式化端口和人数，供大厅、预览与房间复用；不判断连接能力或改变后端状态。
import type {PortDecl} from '../types/multiplayer'

export function portText(decl: PortDecl): string {
  const range = decl.count > 1 ? `${decl.port}–${decl.port + decl.count - 1}` : `${decl.port}`
  return `${decl.proto === 'both' ? 'TCP+UDP' : decl.proto.toUpperCase()} ${range}`
}

export function protoLabel(proto: string): string {
  return proto === 'tcp' ? 'TCP' : proto === 'udp' ? 'UDP' : 'TCP+UDP'
}
