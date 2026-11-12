import {trimKtx2} from './ktx2.mjs';

// 职责：重新封装内嵌纹理缩小后的 GLB；边界：保留网格、材质、节点与扩展的语义及非纹理字节。
const JSON_CHUNK = 0x4e4f534a;
const BIN_CHUNK = 0x004e4942;
const align4 = value => Math.ceil(value / 4) * 4;

export function readGlb(buffer) {
  if (buffer.length < 20 || buffer.readUInt32LE(0) !== 0x46546c67 || buffer.readUInt32LE(4) !== 2 || buffer.readUInt32LE(8) !== buffer.length) {
    throw new Error('GLB 文件头或总长度无效');
  }
  const chunks = [];
  for (let offset = 12; offset < buffer.length;) {
    if (offset + 8 > buffer.length) throw new Error('GLB 块头越界');
    const length = buffer.readUInt32LE(offset);
    const type = buffer.readUInt32LE(offset + 4);
    if (length % 4 || offset + 8 + length > buffer.length) throw new Error('GLB 块长度或对齐无效');
    chunks.push({type, data: buffer.subarray(offset + 8, offset + 8 + length)});
    offset += 8 + length;
  }
  if (chunks[0]?.type !== JSON_CHUNK) throw new Error('GLB 首块必须为 JSON');
  const document = JSON.parse(chunks[0].data.toString('utf8').trim());
  const binary = chunks.find(chunk => chunk.type === BIN_CHUNK)?.data ?? Buffer.alloc(0);
  // 传送带等文件只有空节点，没有缓冲与贴图；原样保留这些合法的占位模型。
  if (document.buffers?.length > 1 || document.buffers?.[0]?.uri || (!binary.length && document.bufferViews?.length)) throw new Error('只支持单一内嵌二进制缓冲的 GLB');
  return {document, binary, extras: chunks.filter(chunk => ![JSON_CHUNK, BIN_CHUNK].includes(chunk.type))};
}

export function bufferViewData(glb, index) {
  const view = glb.document.bufferViews?.[index];
  if (!view || view.buffer !== 0) throw new Error(`无效的 bufferView：${index}`);
  const start = view.byteOffset ?? 0;
  if (start < 0 || view.byteLength < 0 || start + view.byteLength > glb.binary.length) throw new Error('GLB bufferView 越界');
  return glb.binary.subarray(start, start + view.byteLength);
}

export function optimizeGlb(buffer, maxSize) {
  const glb = readGlb(buffer);
  const replacements = new Map();
  const textures = [];
  for (const image of glb.document.images ?? []) {
    if (image.mimeType !== 'image/ktx2') continue;
    if (image.bufferView === undefined) throw new Error('压缩构建需要 GLB 内嵌 KTX2 贴图');
    if (replacements.has(image.bufferView)) continue;
    const original = bufferViewData(glb, image.bufferView);
    const result = trimKtx2(original, maxSize);
    replacements.set(image.bufferView, result.buffer);
    textures.push({name: image.name ?? '', beforeBytes: original.length, afterBytes: result.buffer.length,
      width: result.width, height: result.height, removedLevels: result.removedLevels, preservedReason: result.preservedReason});
  }
  if (!textures.some(texture => texture.removedLevels)) return {buffer, textures};
  const binaryParts = [];
  let cursor = 0;
  for (let i = 0; i < glb.document.bufferViews.length; i++) {
    const data = replacements.get(i) ?? bufferViewData(glb, i);
    const padding = align4(cursor) - cursor;
    if (padding) binaryParts.push(Buffer.alloc(padding));
    cursor += padding;
    Object.assign(glb.document.bufferViews[i], {byteOffset: cursor, byteLength: data.length});
    binaryParts.push(data);
    cursor += data.length;
  }
  glb.document.buffers[0].byteLength = cursor;
  const binary = Buffer.concat(binaryParts, align4(cursor));
  const json = Buffer.from(JSON.stringify(glb.document));
  const jsonPadded = Buffer.alloc(align4(json.length), 0x20);
  json.copy(jsonPadded);
  const chunks = [{type: JSON_CHUNK, data: jsonPadded}, {type: BIN_CHUNK, data: binary}, ...glb.extras];
  const length = 12 + chunks.reduce((sum, chunk) => sum + 8 + chunk.data.length, 0);
  const output = Buffer.alloc(length);
  output.writeUInt32LE(0x46546c67, 0);
  output.writeUInt32LE(2, 4);
  output.writeUInt32LE(length, 8);
  cursor = 12;
  for (const chunk of chunks) {
    output.writeUInt32LE(chunk.data.length, cursor);
    output.writeUInt32LE(chunk.type, cursor + 4);
    chunk.data.copy(output, cursor + 8);
    cursor += 8 + chunk.data.length;
  }
  return {buffer: output, textures};
}
