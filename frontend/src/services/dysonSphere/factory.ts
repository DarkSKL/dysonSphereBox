import {Decode, DescribeDataset, Encode, ExportBlueprint, ExportWorkspace, Generate, Inspect, ListDatasets, ReadBlueprintChunk, Solve} from '../../../wailsjs/go/factory/Service'
import {blueprint, build, factorycontract, quant} from '../../../wailsjs/go/models'

// 职责：统一稳定错误码和大对象分片；使用：量化工作区；边界：不在浏览器重算配方或拼装蓝图。
export class FactoryError extends Error {
  constructor(readonly code: string, message: string) { super(message) }
}
const ready = (): void => {
  const bridge = window as Window & {go?: {factory?: {Service?: unknown}}}
  if (!bridge.go?.factory?.Service) throw new FactoryError('BRIDGE_UNAVAILABLE', '请在戴森球盒子桌面应用中使用量化计算器；单独打开网页没有本机计算服务。')
}
const unwrap = <T>(response: {code: string; message: string; data?: T | null}): T => {
  if (response.code !== 'OK' || response.data == null) throw new FactoryError(response.code, response.message)
  return response.data
}
const readGenerated = async (value: {blueprint: string; blueprintRef?: string; blueprintLength?: number}): Promise<string> => {
  if (value.blueprint) return value.blueprint
  if (!value.blueprintRef) return ''
  const parts: string[] = []
  let offset = 0
  for (let part = 0; part < 64; part++) {
    const result = unwrap(await ReadBlueprintChunk(new factorycontract.ChunkRequest({token: value.blueprintRef, offset})))
    parts.push(result.text)
    if (result.done) return parts.join('')
    if (result.nextOffset <= offset) throw new FactoryError('BP_E_FORMAT', '生成内容读取没有前进，请重新生成')
    offset = result.nextOffset
  }
  throw new FactoryError('BP_E_TOO_LARGE', '生成内容超过分片读取上限')
}
export const listFactoryDatasets = async (): Promise<quant.DatasetInfo[]> => { ready(); return unwrap(await ListDatasets()) }
export const describeFactoryDataset = async (key: string): Promise<factorycontract.DatasetDetail> => { ready(); return unwrap(await DescribeDataset(key)) }
export const solveFactory = async (request: quant.Request): Promise<quant.Result> => { ready(); return unwrap(await Solve(request)) }
export const generateFactory = async (request: build.Request): Promise<build.Result> => {
  ready()
  const result = unwrap(await Generate(request))
  result.blueprint = await readGenerated(result)
  return result
}
export const inspectFactoryBlueprint = async (code: string, dataset: string): Promise<factorycontract.Inspection> => {
  ready(); return unwrap(await Inspect(new factorycontract.InspectRequest({blueprint: code, dataset})))
}
export const decodeFactoryBlueprint = async (code: string, strict = true): Promise<blueprint.Document> => {
  ready(); return unwrap(await Decode(new factorycontract.DecodeRequest({blueprint: code, strict})))
}
export const encodeFactoryBlueprint = async (document: blueprint.Document): Promise<string> => {
  ready()
  const result = unwrap(await Encode(new factorycontract.EncodeRequest({document, envelope: document.envelopeVersion, recompute: true})))
  return readGenerated(result)
}
export const exportFactoryBlueprint = async (name: string, code: string): Promise<string> => { ready(); return ExportBlueprint(name, code) }
export const exportFactoryWorkspace = async (request: factorycontract.WorkspaceRequest): Promise<string> => { ready(); return ExportWorkspace(request) }
