// 职责：裁掉 KTX2 中超过发布分辨率的 mip 层，复用原始 ETC1S/UASTC 数据。
// 使用：仅供构建工具；边界：不重新编码、不改色彩描述，只处理非数组的二维 Basis 纹理。
// 格式依据：https://registry.khronos.org/KTX/specs/2.0/ktxspec.v2.html（含附录 C）。
const MAGIC = Buffer.from([0xab, 0x4b, 0x54, 0x58, 0x20, 0x32, 0x30, 0xbb, 13, 10, 26, 10]);
const align = (value, alignment) => Math.ceil(value / alignment) * alignment;

function slice(buffer, offset, length) {
  if (!Number.isSafeInteger(offset) || !Number.isSafeInteger(length) || offset < 0 || length < 0 || offset + length > buffer.length) {
    throw new Error('KTX2 数据块越界');
  }
  return buffer.subarray(offset, offset + length);
}

export function readKtx2(buffer) {
  if (buffer.length < 80 || !buffer.subarray(0, 12).equals(MAGIC)) throw new Error('不是有效的 KTX2 文件');
  const count = buffer.readUInt32LE(40);
  if (count < 1) throw new Error('KTX2 缺少显式 mip 层');
  slice(buffer, 80, count * 24);
  const levels = Array.from({length: count}, (_, i) => {
    const at = 80 + i * 24;
    const offset = Number(buffer.readBigUInt64LE(at));
    const length = Number(buffer.readBigUInt64LE(at + 8));
    return {data: slice(buffer, offset, length), uncompressed: buffer.readBigUInt64LE(at + 16)};
  });
  return {
    width: buffer.readUInt32LE(20), height: buffer.readUInt32LE(24),
    depth: buffer.readUInt32LE(28), layers: buffer.readUInt32LE(32), faces: buffer.readUInt32LE(36),
    scheme: buffer.readUInt32LE(44), levels,
    dfd: slice(buffer, buffer.readUInt32LE(48), buffer.readUInt32LE(52)),
    kvd: slice(buffer, buffer.readUInt32LE(56), buffer.readUInt32LE(60)),
    sgd: slice(buffer, Number(buffer.readBigUInt64LE(64)), Number(buffer.readBigUInt64LE(72))),
  };
}

/** 删除高分辨率 mip，并同步 BasisLZ 的逐图像描述；共享码本和每个保留层的压缩字节保持不变。 */
export function trimKtx2(buffer, maxSize) {
  if (!Number.isInteger(maxSize) || maxSize < 1) throw new Error('目标分辨率必须为正整数');
  const texture = readKtx2(buffer);
  let drop = 0;
  while (Math.max(Math.floor(texture.width / 2 ** drop), Math.floor(texture.height / 2 ** drop)) > maxSize) drop++;
  if (!drop) return {buffer, removedLevels: 0, width: texture.width, height: texture.height};
  if (drop >= texture.levels.length) throw new Error('现有 mip 层不足以满足目标分辨率');
  const width = Math.max(1, Math.floor(texture.width / 2 ** drop));
  const height = Math.max(1, Math.floor(texture.height / 2 ** drop));
  // KHR_texture_basisu 要求基准层边长为 4 的倍数，特殊裁切贴图保持原样以免破坏 GPU 兼容性。
  if (width % 4 || height % 4) return {buffer, removedLevels: 0, width: texture.width, height: texture.height, preservedReason: '缩小后的边长不是 4 的倍数'};
  if (texture.depth !== 0 || texture.layers !== 0 || texture.faces !== 1 || ![1, 2].includes(texture.scheme)) {
    throw new Error('仅支持非数组二维 ETC1S/BasisLZ 或 UASTC/Zstd 纹理');
  }
  const levels = texture.levels.slice(drop);
  let sgd = texture.sgd;
  if (texture.scheme === 1) {
    // 二维单面纹理每个 mip 对应一个 20 字节的 ImageDesc；码本位于全部描述之后。
    const descriptorsEnd = 20 + texture.levels.length * 20;
    slice(sgd, 0, descriptorsEnd);
    const dataLength = [4, 8, 12, 16].reduce((sum, offset) => sum + sgd.readUInt32LE(offset), 0);
    if (sgd.length !== descriptorsEnd + dataLength) throw new Error('BasisLZ 全局数据长度不匹配');
    sgd = Buffer.concat([sgd.subarray(0, 20), sgd.subarray(20 + drop * 20, descriptorsEnd), sgd.subarray(descriptorsEnd)]);
  }

  const header = Buffer.from(buffer.subarray(0, 80));
  header.writeUInt32LE(width, 20);
  header.writeUInt32LE(height, 24);
  header.writeUInt32LE(levels.length, 40);
  const dfdOffset = 80 + levels.length * 24;
  const kvdOffset = texture.kvd.length ? align(dfdOffset + texture.dfd.length, 4) : 0;
  const metadataEnd = texture.kvd.length ? kvdOffset + texture.kvd.length : dfdOffset + texture.dfd.length;
  const sgdOffset = sgd.length ? align(metadataEnd, 8) : 0;
  let cursor = sgd.length ? sgdOffset + sgd.length : metadataEnd;
  const levelIndex = Buffer.alloc(levels.length * 24);
  const payloads = [];
  // KTX2 的物理数据顺序为小图到大图，Level Index 的顺序为大图到小图。
  for (let i = levels.length - 1; i >= 0; i--) {
    levelIndex.writeBigUInt64LE(BigInt(cursor), i * 24);
    levelIndex.writeBigUInt64LE(BigInt(levels[i].data.length), i * 24 + 8);
    levelIndex.writeBigUInt64LE(levels[i].uncompressed, i * 24 + 16);
    payloads.push(levels[i].data);
    cursor += levels[i].data.length;
  }
  header.writeUInt32LE(dfdOffset, 48);
  header.writeUInt32LE(texture.dfd.length, 52);
  header.writeUInt32LE(kvdOffset, 56);
  header.writeUInt32LE(texture.kvd.length, 60);
  header.writeBigUInt64LE(BigInt(sgdOffset), 64);
  header.writeBigUInt64LE(BigInt(sgd.length), 72);
  const output = Buffer.alloc(cursor);
  header.copy(output);
  levelIndex.copy(output, 80);
  texture.dfd.copy(output, dfdOffset);
  if (texture.kvd.length) texture.kvd.copy(output, kvdOffset);
  if (sgd.length) sgd.copy(output, sgdOffset);
  cursor -= payloads.reduce((sum, item) => sum + item.length, 0);
  for (const payload of payloads) { payload.copy(output, cursor); cursor += payload.length; }
  return {buffer: output, removedLevels: drop, width, height};
}
