import type {saveanalysis as dto} from '../../wailsjs/go/models'

// 将同一快照的后端结论转成玩家可读的说明；设备归属、检查顺序和受阻判断沿用后端。
type Name = (id: number) => string
export interface Finding {
  title: string
  evidence: string
  summary?: string
  detail?: string
  supplyPath?: string
  roots: number[]
  shared: boolean
  unresolved: boolean
}
const states: Record<string, string> = {
  'output-blocked': '送不走成品', 'input-missing': '在等原料', processing: '正在生产',
  'return-blocked': '送不回剩下的原料', 'low-power': '电不够用', 'no-source': '采不到矿',
  idle: '没在开工', 'source-located': '看不出当时有没有开工', undetermined: '看不出当时有没有开工',
}
function inputStock(node: dto.ChainNode, item: number, name: Name): string {
  const part = (node.machine?.inputs ?? node.device?.inputs)?.find(v => v.itemId === item)
  if (part?.required) return `做一批要 ${part.required} 件${name(item)}，机器里${part.available ? '只有 ' + part.available + ' 件' : '一件也没有'}。`
  if (node.device?.kind === 'fractionator') return '分馏塔里没有原料。'
  return item ? `机器还在等${name(item)}，现有的料不够开工。` : '机器在等料，存档里看不出缺的是哪一种。'
}

// 分拣器检查用组件编号，页面展示建筑实体编号。只采用同档逐机检查确认为过滤不符的项。
function inputFilterSettings(line: dto.CircuitReport | undefined, name: Name) {
  const settings = new Map<string, string>()
  if (!line) return settings
  const machines = new Map(line.machines.map(m => [m.assemblerId, m]))
  for (const finding of line.findings || []) {
    if (finding.routeEvidence !== 'filter-mismatch' || finding.status !== 'input-missing') continue
    for (const id of finding.assemblers) {
      const m = machines.get(id)
      if (!m) continue
      const sorter = m.connections?.find(s => (s.direction === 'in' || s.reversible) && s.filter > 0 && s.filter !== finding.itemId)
      if (sorter) settings.set(`${m.entityId}/${finding.itemId}`, `机器 #${m.entityId} 的进料分拣器都不拿${name(finding.itemId)}。比如 #${sorter.sorterEntity} 只拿${name(sorter.filter)}，可以先看它的过滤设置。`)
    }
  }
  return settings
}

function chainFinding(c: dto.ChainCheck, nodes: dto.ChainNode[], planet: number, name: Name, place: Name): Finding {
  const node = nodes[c.node], f = node.freight
  const material = c.itemId ? name(c.itemId) : '原料'
  const building = node.machine?.building ?? node.device?.building
  const location = node.planetId === planet ? '' : place(node.planetId) + '上的'
  const machine = `${location}${building ? name(building) : '机器'} #${node.entityId}`
  let title = `看看${machine}有没有在工作`
  let evidence = '能找到这台机器，但光看这份存档，还说不准它当时有没有在生产。'
  let detail: string | undefined
  if (f) {
    const slot = `${node.planetId === planet ? '本星' : place(node.planetId)}的 ${f.slot.stationId} 号物流站第 ${f.slot.index + 1} 格`
    title = c.status === 'empty-slot' ? `去${slot}看看，${material}到了没有` : `去${slot}看看，${material}能不能送上传送带`
    evidence = f.slot.inventory ? `格子里还有 ${f.slot.inventory} 件${material}。` : `格子里一件${material}也没有。`
    if (f.cargoInFlight) evidence += `有 ${f.cargoInFlight} 件已经装好，正在运过来。`
    if (f.awaitingPickup) evidence += `还有 ${f.awaitingPickup} 件等着取货。`
    if (!f.cargoInFlight && !f.awaitingPickup) evidence += '存档时没看到来货或取货任务。'
    detail = `顺着 ${c.directEntities.length} 台缺料设备的进料线，都能找到这个格子。这里最多能放 ${f.slot.capacity} 件。`
    const loaded = [...new Set((f.tasks || []).filter(t => t.phase === 'carrying').map(t =>
      `${t.sourcePlanet === planet ? '本星' : place(t.sourcePlanet)} ${t.sourceStation} 号站${t.sourceIndex >= 0 ? '第 ' + (t.sourceIndex + 1) + ' 格' : '（看不出从哪一格拿货）'}`))]
    if (loaded.length) detail += ' 已经装上货的那几批来自' + loaded.join('、') + '。'
    if (f.modeCandidates?.length) detail += ` 有 ${f.modeCandidates.length} 个货格设置成了供应，光看设置还不能确定谁会送货。`
  } else if (c.status === 'input-missing') {
    title = `看看${machine}的进料口能不能拿到${material}`
    evidence = inputStock(node, c.itemId, name)
  } else if (c.status === 'output-blocked') {
    const output = node.machine?.outputs?.find(o => o.itemId === c.itemId)
    title = `去看${machine}出${material}的地方`
    evidence = `${material}留在机器里没送走。看一下还有没有地方接下一批。`
    if (c.drop) {
      title = `去看${machine}往外送${material}的那段传送带`
      evidence = `分拣器 #${c.drop.sorter} 手里拿着 ${c.drop.count} 件${material}，放不到 #${c.drop.belt} 这段传送带上。先看带上的货还能不能往前走。`
    }
    if (output) detail = `机器里堆了 ${output.buffered} 件${material}，要降到 ${output.deliveryLimit} 件或更少，才能放下下一批。`
    else if (node.device) detail = `机器里还放着 ${node.device.buffered} 件${material}。`
  } else if (c.status === 'low-power') {
    title = `去看${machine}连的电网`
    const power = node.machine?.power ?? node.device?.power
    evidence = power === undefined ? '这台机器电不够用。' : `这台机器当时拿到的电只有所需电量的 ${(power * 100).toFixed(1)}%。`
  } else if (c.status === 'return-blocked' && node.device) {
    title = `看看${machine}的回流带还能不能往前走`
    evidence = `剩下的原料送不回去，回流口已经堆了 ${node.device.fluidReturn} 件。`
  } else if (c.status === 'no-source') {
    title = `看看${machine}的采集范围里有没有矿点`
    evidence = '存档时，这台机器没有连到任何矿点。'
  }
  return {title, evidence, detail, roots: c.rootEntities || [], shared: c.shared, unresolved: c.unresolved}
}

/** 沿实物进料线和已载货任务为首查点选一条示例路径。共线与回环只访问一次，保留跨站运输的证据类型。 */
function supplyFindings(checks: dto.ChainCheck[], nodes: dto.ChainNode[], starts: number[], planet: number, name: Name, place: Name) {
  const queue = [...new Set(starts)]
  const parent = new Map(queue.map(i => [i, -1]))
  for (let i = 0; i < queue.length; i++) {
    const from = queue[i]
    const node = nodes[from]
    const sources = (node.inputs || []).flatMap(input => input.sources || [])
    for (const source of sources.concat(node.loadedSources || [])) {
      if (parent.has(source)) continue
      parent.set(source, from)
      queue.push(source)
    }
  }
  return checks.filter(c => c.priority === 1).map(c => {
    const finding = chainFinding(c, nodes, planet, name, place)
    if (!parent.has(c.node)) return finding
    const path: dto.ChainNode[] = []
    for (let i = c.node; i !== -1; i = parent.get(i)!) {
      path.push(nodes[i])
    }
    path.reverse()
    const steps: string[] = []
    for (let i = 1; i < path.length; i++) {
      const before = path[i - 1], after = path[i]
      const item = name(before.productId)
      if (before.planetId !== after.planetId) {
        steps.push(`${item}有一批已经从${place(after.planetId)}装货，正运往${place(before.planetId)}`)
      } else if (before.productId !== after.productId) {
        steps.push(`${before.planetId === planet ? '' : place(before.planetId) + '上'}做${item}要用${name(after.productId)}`)
      }
    }
    if (steps.length) finding.supplyPath = steps.join('，') + '。'
    return finding
  })
}

function downstreamFinding(c: dto.OutputCheck, out: dto.OutputDiagnosis, name: Name): Finding {
  const v = out.receivers[c.receivers?.[0]]
  const machine = `${v?.building ? name(v.building) : '机器'} #${c.entityId}`
  const item = name(c.itemId)
  let title = `看看${machine}旁边的带子有没有在走货`
  let evidence = '存档时没能追到具体是哪台机器挡住了出货。'
  if (v?.slot) {
    title = `去 ${v.slot.stationId} 号物流站看看，第 ${v.slot.index + 1} 格的${item}有没有运走`
    evidence = `这一格已经装了 ${v.slot.inventory} 件，最多能收 ${v.slot.capacity} 件。要先运走一些，新的才进得来。`
  } else if (c.status === 'consumer-input-missing') {
    title = `去${machine}看看，送${item}的分拣器能不能拿到货`
    const part = v?.machine?.inputs?.find(i => i.itemId === c.itemId)
    evidence = part ? `做一批要 ${part.required} 件${item}，机器里只有 ${part.available} 件。` : `这台机器还在等${item}。`
  } else if (c.status === 'belt-drop-blocked') {
    title = `去看分拣器 #${c.entityId} 放货的那段传送带`
    const drop = out.sources.flatMap(s => s.ports).find(p => p.drop?.sorter === c.entityId && p.drop.itemId === c.itemId)?.drop
    evidence = drop ? `它拿着 ${drop.count} 件${item}，放不上 #${drop.belt} 这段带。看看带上的货能不能往前走。` : `分拣器拿着${item}，没能放上传送带。`
  } else if (c.status === 'consumer-output-blocked') {
    title = `去${machine}看看，做好的${item}能不能送走`
    evidence = `这台机器自己的${item}堆着出不去，它也暂时不再收${v ? name(v.itemId) : '来料'}。`
  } else if (c.status === 'consumer-low-power') {
    title = `看看${machine}连的电网`
    evidence = '接货的机器当时电不够用，也没在要新的原料。'
  } else if (c.status === 'not-requesting') {
    evidence = `存档时这台机器没在要${item}。先看看它手头的料用完了没有。`
  }
  return {title, evidence, roots: c.rootEntities || [], shared: c.shared, unresolved: c.unresolved}
}

// 从目标机器确实积压的物品出发，沿后端记录的收货口与下一道工序找一条实际相连的出货路线。
function firstOutputSteps(out: dto.OutputDiagnosis, roots: dto.ChainNode[]) {
  const blocked = new Set(roots.flatMap(n => (n.constraints || []).filter(c => c.status === 'output-blocked').map(c => `${n.entityId}/${c.itemId}`)))
  type Step = {sent: number; next?: number}
  const sources = new Map<number, Step>(), receivers = new Map<number, Step>()
  for (const [i, source] of out.sources.entries()) {
    if (blocked.has(`${source.entityId}/${source.itemId}`)) sources.set(i, {sent: source.itemId})
  }
  // Map 按插入顺序遍历，合流处只保留最先找到的真实路线，回环不会重复入队。
  for (const [i, step] of sources) {
    for (const port of out.sources[i].ports) {
      for (const receiver of port.receivers || []) {
        if (receivers.has(receiver)) continue
        receivers.set(receiver, step)
        for (const next of out.receivers[receiver].next || []) {
          if (!sources.has(next)) sources.set(next, {sent: step.sent, next: step.next ?? out.sources[next].itemId})
        }
      }
    }
  }
  return receivers
}

/** 按同一目标集合归并终点。共线端点保留各自身份，不把覆盖数相加。 */
function outputCauses(out: dto.OutputDiagnosis, roots: dto.ChainNode[], name: Name) {
  const groups = new Map<string, {label: string; entities: number[]; finding: Finding}>()
  const steps = firstOutputSteps(out, roots)
  for (const c of out.checks) {
    if (!['consumer-input-missing', 'station-full', 'consumer-low-power'].includes(c.status)) continue
    const v = out.receivers[c.receivers?.[0]]
    const product = v?.machine?.outputs?.map(o => name(o.itemId)).join('、')
    const label = v?.slot ? `${v.slot.stationId} 号物流站的第 ${v.slot.index + 1} 格装满了${name(c.itemId)}，新货进不去` :
      c.status === 'consumer-low-power' ? '收货的机器电不够用' :
        `${product ? '做' + product + '的机器' : '收货的机器'}缺${name(c.itemId)}${v?.requested === false && v.itemId !== c.itemId ? '，暂时用不上更多' + name(v.itemId) : ''}`
    const key = label + '/' + c.rootEntities.join(',')
    const existing = groups.get(key)
    if (existing) {
      existing.entities.push(c.entityId)
      existing.finding.shared ||= c.shared; existing.finding.unresolved ||= c.unresolved
    } else {
      const finding = downstreamFinding(c, out, name)
      const step = steps.get(c.receivers[0])
      const firstStep = step?.next && step.next !== step.sent ? `做${name(step.next)}的机器已经不收${name(step.sent)}了。再往后看，` : '顺着出货的带子往后看，'
      finding.summary = firstStep + label + '。'
      if (v?.machine && v.requested === false) {
        const stock = v.machine.inputs?.find(i => i.itemId === v.itemId)
        if (stock && stock.itemId !== c.itemId) finding.evidence += `${name(v.itemId)}倒是还有 ${stock.available} 件，每批要 ${stock.required} 件。`
        finding.evidence += `顺着带子可以找到 ${c.directEntities.length} 台往这里送料的机器。`
      }
      groups.set(key, {label, entities: [c.entityId], finding})
    }
  }
  return [...groups.values()].map(g => ({...g.finding, title: g.label,
    evidence: `${g.entities.map(e => '#' + e).join('、')}${g.entities.length > 1 ? ' 都在这条线上。拿 #' + g.entities[0] + ' 来说，' : ' 在这条线上。'}${g.finding.evidence}`}))
}

// 相同的一批设备只列一次。不同集合即使台数相等也分开，保留同一设备同时缺几种原料的证据。
function causeGroups(findings: Finding[], downstream: boolean) {
  const groups = new Map<string, {roots: number[]; scope: string; findings: Finding[]}>()
  for (const finding of findings) {
    const key = finding.roots.join(',')
    const group = groups.get(key)
    if (group) {
      group.findings.push(finding)
    } else {
      groups.set(key, {roots: finding.roots, scope: downstream ? '出不了货' : '缺料', findings: [finding]})
    }
  }
  return [...groups.values()].map(group => ({
    ...group,
    summary: downstream ? group.findings[0].summary : group.findings.map(f => f.summary || f.title).join(''),
  }))
}

export function presentDiagnosis(r: dto.ProductionDiagnosis, name: Name, place: Name, line?: dto.CircuitReport) {
  const roots = r.roots.map(i => r.nodes[i])
  const filters = inputFilterSettings(line, name)
  const counts = new Map<string, number>(), missing = new Map<number, number[]>()
  for (const n of roots) {
    const status = n.machine?.status || n.device?.status || 'undetermined'
    counts.set(status, (counts.get(status) || 0) + 1)
    for (const c of n.constraints || []) {
      if (c.status !== 'input-missing') continue
      const ids = missing.get(c.itemId) || []
      ids.push(n.entityId); missing.set(c.itemId, ids)
    }
  }
  const ranked = [...counts].sort((a,b) => b[1] - a[1])
  const leading = ranked.filter(([k]) => ['output-blocked','input-missing','return-blocked','low-power','no-source'].includes(k)).slice(0, 2)
  const product = name(r.productId)
  let headline = leading.length ? `生产${product}的设备里，${leading.map(([k, n]) => n + ' 台' + states[k]).join('，')}` :
    counts.get('processing') === roots.length ? `生产${product}的设备都在工作` : `存档时，生产${product}的设备怎么样了？`
  if (r.status === 'history-only') headline = `这颗星球以前产过${product}，现在找不到做它的设备`
  const otherStates = ranked.filter(([k]) => !leading.some(([shown]) => shown === k))
  const countsText = [`共 ${roots.length} 台`, ...otherStates.map(([k,n]) => `${n} 台${states[k] || '看不出当时有没有开工'}`)].join('，')
  const out = r.downstream
  const downstream = out ? outputCauses(out, roots, name) : []
  const local: Finding[] = []
  for (const [id, ids] of missing) {
    const related = r.checks.filter(c => c.itemId === id && c.status === 'input-missing' && ids.includes(r.nodes[c.node].entityId) && r.nodes[c.node].planetId === r.planetId && r.nodes[c.node].productId === r.productId)
    const badFilters = ids.filter(entity => filters.has(`${entity}/${id}`))
    const material = id ? name(id) : '原料'
    if (badFilters.length) {
      local.push({title: `进料分拣器不拿${material}。`, summary: `在等${material}，进料分拣器却都设成了只拿别的东西。先看过滤设置。`,
        evidence: filters.get(`${badFilters[0]}/${id}`)!, roots: badFilters, shared: false, unresolved: true})
    }
    const remaining = ids.filter(entity => !filters.has(`${entity}/${id}`))
    if (!remaining.length) continue
    const checks = related.filter(c => !filters.has(`${r.nodes[c.node].entityId}/${id}`))
    const c = checks[0]
    const sources = new Set(checks.flatMap(v => (r.nodes[v.node].inputs || []).filter(input => input.itemId === id).flatMap(input => input.sources || [])))
    const upstream = new Map<string, number>()
    for (const index of sources) {
      const n = r.nodes[index]
      const facts = n.freight ? [`${n.freight.slot.stationId} 号物流站第 ${n.freight.slot.index + 1} 格有 ${n.freight.slot.inventory} 件${material}`] :
        (n.constraints || []).filter(issue => issue.status === 'input-missing').map(issue => issue.itemId ? '等' + name(issue.itemId) : '等原料')
      for (const fact of facts) upstream.set(fact, (upstream.get(fact) || 0) + 1)
    }
    const supply = [...upstream].map(([why, count]) => why.startsWith('等') ? `有 ${count} 台做${material}的设备也在${why}` : why)
    const title = id ? `在等${material}，料不够做下一批。` : '原料还没进来，存档里看不出缺的是哪一种。'
    local.push({title, summary: title + (supply.length ? '沿进料线往前看，' + supply[0] + '。' : ''),
      evidence: (c ? `拿 #${r.nodes[c.node].entityId} 来说，${inputStock(r.nodes[c.node], id, name)}` : title) +
        (supply.length ? '沿进料线往前看，' + supply.join('，') + '。' : '可以先去看看进料口的传送带上有没有它需要的料。'),
      roots: remaining, shared: checks.some(v=>v.shared), unresolved: checks.some(v=>v.unresolved)})
  }
  let first: Finding[] = []
  if (r.firstCheckGroup === 'downstream.supply.checks' && out?.supply) {
    const supply = out.supply
    const starts = out.checks.filter(c => c.status === 'consumer-input-missing').flatMap(c =>
      c.receivers.map(i => out.receivers[i].supplyNode).filter((i): i is number => i !== undefined && i !== null))
    first = supplyFindings(supply.checks, supply.nodes, starts, r.planetId, name, place)
  } else if (r.firstCheckGroup === 'downstream.checks' && out) {
    first = out.checks.filter(c=>c.priority===1).map(c=>downstreamFinding(c,out,name))
  } else {
    first = supplyFindings(r.checks, r.nodes, r.roots, r.planetId, name, place)
    for (const [i, check] of r.checks.filter(c => c.priority === 1).entries()) {
      const node = r.nodes[check.node]
      const setting = node.planetId === r.planetId && filters.get(`${node.entityId}/${check.itemId}`)
      if (check.status === 'input-missing' && setting) {
        first[i].title = `去看机器 #${node.entityId} 进料分拣器的过滤设置`
        first[i].evidence = setting
      }
    }
  }
  const groups = [...causeGroups(downstream, true), ...causeGroups(local, false)]
  const traced = new Set(downstream.flatMap(c => c.roots))
  const untraced = roots.filter(n => (n.machine?.status ?? n.device?.status) === 'output-blocked' && !traced.has(n.entityId)).length
  return {headline, countsText, groups, first, untraced}
}
