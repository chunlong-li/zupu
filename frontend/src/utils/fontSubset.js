// 极简 TTF(glyf) 子集化：只保留图上真正用到的字形，用于「SVG 内嵌字体」档，
// 让导出的 SVG 自包含且体积可控（整包中文字体几 MB~十几 MB → 子集后几百 KB）。
//
// 支持：sfnt 版本 0x00010000 / 'true' 的 TrueType(glyf) 字体（含中文 CJK）。
// 不支持：CFF/OTF('OTTO')、woff/woff2（需额外解压）、ttc（字体集合）——调用方据此给出明确提示。
//
// 做法：解析 sfnt/cmap/loca/glyf/hmtx → 收集用到的字形（复合字形递归包含部件）→
// 重新编号 → 重建 glyf/loca/hmtx/cmap，复制 name/OS2，post 用 3.0 空表，丢弃 hinting 与
// 一切带字形索引的表（GSUB/GPOS/kern/hdmx/LTSH…），最后重算表校验和与 head.checkSumAdjustment。

const TAG = (dv, o) => String.fromCharCode(dv.getUint8(o), dv.getUint8(o + 1), dv.getUint8(o + 2), dv.getUint8(o + 3))

function u8(buf) { return buf instanceof Uint8Array ? buf : new Uint8Array(buf) }

function checksum(bytes) {
  let sum = 0
  const n = bytes.length
  for (let i = 0; i < n; i += 4) {
    const b0 = bytes[i] || 0, b1 = bytes[i + 1] || 0, b2 = bytes[i + 2] || 0, b3 = bytes[i + 3] || 0
    sum = (sum + ((b0 << 24) | (b1 << 16) | (b2 << 8) | b3)) >>> 0
  }
  return sum >>> 0
}
function pad4(arr) {
  const pad = (4 - (arr.length % 4)) % 4
  if (!pad) return arr
  const out = new Uint8Array(arr.length + pad)
  out.set(arr)
  return out
}
function concat(list) {
  let n = 0
  list.forEach(a => { n += a.length })
  const out = new Uint8Array(n)
  let p = 0
  list.forEach(a => { out.set(a, p); p += a.length })
  return out
}

class FontError extends Error {
  constructor(code, message) { super(message); this.code = code }
}

// 解析 cmap，返回 charCode → glyphID 的查询函数
function makeCmapLookup(dv, cmapOff) {
  const nSub = dv.getUint16(cmapOff + 2)
  let best = null, bestScore = -1
  for (let i = 0; i < nSub; i++) {
    const rec = cmapOff + 4 + i * 8
    const plat = dv.getUint16(rec), enc = dv.getUint16(rec + 2), off = cmapOff + dv.getUint32(rec + 4)
    const fmt = dv.getUint16(off)
    let score = -1
    if (plat === 3 && enc === 10 && fmt === 12) score = 5
    else if (plat === 0 && fmt === 12) score = 4
    else if (plat === 3 && enc === 1 && fmt === 4) score = 3
    else if (plat === 0 && fmt === 4) score = 2
    if (score > bestScore) { bestScore = score; best = { off, fmt } }
  }
  if (!best) throw new FontError('FONT_UNSUPPORTED', '字体缺少可用的 Unicode cmap 表')
  const { off, fmt } = best
  const gidOf = (code) => {
    if (fmt === 4) {
      if (code > 0xFFFF) return 0
      const segX2 = dv.getUint16(off + 6)
      const seg = segX2 / 2
      const endO = off + 14, startO = endO + segX2 + 2, deltaO = startO + segX2, rangeO = deltaO + segX2
      let lo = 0, hi = seg - 1, found = -1
      while (lo <= hi) {
        const mid = (lo + hi) >> 1
        const end = dv.getUint16(endO + mid * 2)
        if (code <= end) { found = mid; hi = mid - 1 } else lo = mid + 1
      }
      if (found < 0) return 0
      const start = dv.getUint16(startO + found * 2)
      if (code < start) return 0
      const idDelta = dv.getInt16(deltaO + found * 2)
      const idRangeOff = dv.getUint16(rangeO + found * 2)
      if (idRangeOff === 0) return (code + idDelta) & 0xFFFF
      const addr = rangeO + found * 2 + idRangeOff + (code - start) * 2
      if (addr + 1 >= dv.byteLength) return 0
      const g = dv.getUint16(addr)
      return g === 0 ? 0 : (g + idDelta) & 0xFFFF
    }
    if (fmt === 12) {
      const nGroups = dv.getUint32(off + 12)
      let lo = 0, hi = nGroups - 1
      while (lo <= hi) {
        const mid = (lo + hi) >> 1
        const g = off + 16 + mid * 12
        const s = dv.getUint32(g), e = dv.getUint32(g + 4)
        if (code < s) hi = mid - 1
        else if (code > e) lo = mid + 1
        else return dv.getUint32(g + 8) + (code - s)
      }
      return 0
    }
    return 0
  }
  return gidOf
}

// 复合字形：列出部件字形 id（并可选改写为新 id）
function walkComponents(data, dvAll, base, patchMap) {
  const out = []
  const nc = (data[0] << 8 | data[1]) << 16 >> 16
  if (nc >= 0) return out
  let p = 10
  while (p + 4 <= data.length) {
    const flags = (data[p] << 8) | data[p + 1]
    const gi = (data[p + 2] << 8) | data[p + 3]
    out.push(gi)
    if (patchMap) {
      const ng = patchMap.get(gi)
      if (ng != null) { data[p + 2] = (ng >> 8) & 0xFF; data[p + 3] = ng & 0xFF }
    }
    p += 4
    p += (flags & 0x0001) ? 4 : 2            // ARG_1_AND_2_ARE_WORDS
    if (flags & 0x0008) p += 2               // WE_HAVE_A_SCALE
    else if (flags & 0x0040) p += 4          // WE_HAVE_AN_X_AND_Y_SCALE
    else if (flags & 0x0080) p += 8          // WE_HAVE_A_TWO_BY_TWO
    if (!(flags & 0x0020)) break             // MORE_COMPONENTS
  }
  return out
}

function buildCmap(entries) {
  // entries: [{code, gid}] 已按 code 升序
  const bmp = entries.filter(e => e.code <= 0xFFFF)
  const full = entries.filter(e => e.code > 0xFFFF)
  const subtables = []
  const records = []
  let off = 4 + 8 * ((bmp.length ? 1 : 0) + (full.length ? 1 : 0) + (bmp.length ? 1 : 0) + (full.length ? 1 : 0))
  // format 4（BMP）：每字一个 segment（idDelta 常量）
  let fmt4 = null
  if (bmp.length) {
    const segCount = bmp.length + 1
    const len = 16 + segCount * 8
    fmt4 = new Uint8Array(len)
    const dv = new DataView(fmt4.buffer)
    dv.setUint16(0, 4); dv.setUint16(2, len); dv.setUint16(4, 0)
    dv.setUint16(6, segCount * 2)
    const searchRange = 2 * Math.pow(2, Math.floor(Math.log2(segCount)))
    dv.setUint16(8, searchRange)
    dv.setUint16(10, Math.log2(searchRange / 2))
    dv.setUint16(12, segCount * 2 - searchRange)
    const endO = 14, startO = endO + segCount * 2 + 2, deltaO = startO + segCount * 2, rangeO = deltaO + segCount * 2
    bmp.forEach((e, i) => {
      dv.setUint16(endO + i * 2, e.code)
      dv.setUint16(startO + i * 2, e.code)
      dv.setUint16(deltaO + i * 2, (e.gid - e.code) & 0xFFFF)
      dv.setUint16(rangeO + i * 2, 0)
    })
    const last = bmp.length
    dv.setUint16(endO + last * 2, 0xFFFF)
    dv.setUint16(startO + last * 2, 0xFFFF)
    dv.setUint16(deltaO + last * 2, 1)
    dv.setUint16(rangeO + last * 2, 0)
  }
  let fmt12 = null
  if (full.length) {
    const nGroups = full.length
    const len = 16 + nGroups * 12
    fmt12 = new Uint8Array(len)
    const dv = new DataView(fmt12.buffer)
    dv.setUint16(0, 12); dv.setUint16(2, 0); dv.setUint32(4, len); dv.setUint32(8, 0); dv.setUint32(12, nGroups)
    full.forEach((e, i) => {
      const g = 16 + i * 12
      dv.setUint32(g, e.code); dv.setUint32(g + 4, e.code); dv.setUint32(g + 8, e.gid)
    })
  }
  // 记录顺序：(0,3)/(0,4) + (3,1)/(3,10)
  if (fmt4) { records.push({ plat: 0, enc: 3, sub: fmt4 }); }
  if (fmt12) { records.push({ plat: 0, enc: 4, sub: fmt12 }); }
  if (fmt4) { records.push({ plat: 3, enc: 1, sub: fmt4 }); }
  if (fmt12) { records.push({ plat: 3, enc: 10, sub: fmt12 }); }
  const header = new Uint8Array(4 + records.length * 8)
  const hdv = new DataView(header.buffer)
  hdv.setUint16(0, 0); hdv.setUint16(2, records.length)
  let p = 4 + records.length * 8
  records.forEach((r, i) => {
    hdv.setUint16(4 + i * 8, r.plat)
    hdv.setUint16(6 + i * 8, r.enc)
    hdv.setUint32(8 + i * 8, p)
    p += r.sub.length
  })
  void off
  return concat([header, ...records.map(r => r.sub)])
}

/**
 * 子集化 TrueType 字体。
 * @param {ArrayBuffer|Uint8Array} buf 原字体文件
 * @param {Set<string>|string[]} chars 需要保留的字符
 * @returns {{ data: Uint8Array, numGlyphs: number, keptChars: number, missing: string[] }}
 */
export function subsetTtf(buf, chars) {
  const bytes = u8(buf)
  const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength)
  const version = dv.getUint32(0)
  if (version === 0x4F54544F) throw new FontError('FONT_UNSUPPORTED', '该字体是 CFF/OTF 格式，暂不支持子集内嵌')
  if (version === 0x774F4646 || version === 0x774F4632) throw new FontError('FONT_UNSUPPORTED', '该字体是 woff/woff2 格式，暂不支持子集内嵌')
  if (version === 0x74746366) throw new FontError('FONT_UNSUPPORTED', '该字体是 ttc 字体集合，暂不支持子集内嵌')
  const numTables = dv.getUint16(4)
  const tables = {}
  for (let i = 0; i < numTables; i++) {
    const o = 12 + i * 16
    tables[TAG(dv, o)] = { offset: dv.getUint32(o + 8), length: dv.getUint32(o + 12) }
  }
  for (const t of ['head', 'maxp', 'loca', 'glyf', 'hmtx', 'hhea', 'cmap']) {
    if (!tables[t]) throw new FontError('FONT_UNSUPPORTED', `字体缺少 ${t} 表，暂不支持子集内嵌`)
  }
  const numGlyphs = dv.getUint16(tables.maxp.offset + 4)
  const indexToLocFormat = dv.getInt16(tables.head.offset + 50)
  const loca = new Int32Array(numGlyphs + 1)
  for (let i = 0; i <= numGlyphs; i++) {
    loca[i] = indexToLocFormat === 0 ? dv.getUint16(tables.loca.offset + i * 2) * 2 : dv.getUint32(tables.loca.offset + i * 4)
  }
  const gidOf = makeCmapLookup(dv, tables.cmap.offset)

  // 收集字形（含复合部件）
  const wanted = new Set([0])
  const missing = []
  const charGids = []
  for (const ch of chars) {
    const code = ch.codePointAt(0)
    const g = gidOf(code)
    if (g > 0 && g < numGlyphs) { wanted.add(g); charGids.push({ code, gid: g }) }
    else missing.push(ch)
  }
  const queue = [...wanted]
  while (queue.length) {
    const g = queue.pop()
    const s = loca[g], e = loca[g + 1]
    if (e <= s) continue
    const glyph = new Uint8Array(bytes.buffer, bytes.byteOffset + tables.glyf.offset + s, e - s)
    walkComponents(glyph, dv, 0, null).forEach(c => { if (c < numGlyphs && !wanted.has(c)) { wanted.add(c); queue.push(c) } })
  }
  const keep = [...wanted].filter(g => g < numGlyphs).sort((a, b) => a - b)
  const newIndex = new Map()
  keep.forEach((g, i) => newIndex.set(g, i))
  const n = keep.length

  // 重建 glyf / loca（long 格式，**大端**写入：Uint32Array 的 buffer 是小端，必须用 DataView 逐项写）
  const glyphChunks = []
  const newLoca = new Array(n + 1).fill(0)
  let pos = 0
  keep.forEach((g, i) => {
    const s = loca[g], e = loca[g + 1]
    let data = e > s
      ? new Uint8Array(bytes.buffer, bytes.byteOffset + tables.glyf.offset + s, e - s).slice()
      : new Uint8Array(0)
    if (data.length) {
      const comps = walkComponents(data, dv, 0, null)
      if (comps.length) walkComponents(data, dv, 0, newIndex)
    }
    const padded = pad4(data)
    glyphChunks.push(padded)
    pos += padded.length
    newLoca[i + 1] = pos
  })
  const glyfTable = concat(glyphChunks)
  const locaTable = new Uint8Array((n + 1) * 4)
  const locaDv = new DataView(locaTable.buffer)
  for (let i = 0; i <= n; i++) locaDv.setUint32(i * 4, newLoca[i])   // 大端

  // 重建 hmtx（每个字形都写 metrics）
  const numHMetrics = dv.getUint16(tables.hhea.offset + 34)
  const lastAW = numHMetrics > 0 ? dv.getUint16(tables.hmtx.offset + (numHMetrics - 1) * 4) : 0
  const hmtxTable = new Uint8Array(n * 4)
  const hdv2 = new DataView(hmtxTable.buffer)
  keep.forEach((g, i) => {
    let aw, lsb
    if (g < numHMetrics) { aw = dv.getUint16(tables.hmtx.offset + g * 4); lsb = dv.getInt16(tables.hmtx.offset + g * 4 + 2) }
    else {
      aw = lastAW
      const idx = g - numHMetrics
      lsb = tables.hmtx.offset + numHMetrics * 4 + idx * 2 + 1 < dv.byteLength ? dv.getInt16(tables.hmtx.offset + numHMetrics * 4 + idx * 2) : 0
    }
    hdv2.setUint16(i * 4, aw); hdv2.setInt16(i * 4 + 2, lsb)
  })

  // head / hhea / maxp 拷贝并改字段
  const headTable = new Uint8Array(bytes.buffer, bytes.byteOffset + tables.head.offset, Math.min(tables.head.length, 54)).slice()
  const hdvh = new DataView(headTable.buffer)
  hdvh.setUint32(8, 0)              // checkSumAdjustment 先置 0
  hdvh.setInt16(50, 1)              // indexToLocFormat = long
  const hheaTable = new Uint8Array(bytes.buffer, bytes.byteOffset + tables.hhea.offset, Math.min(tables.hhea.length, 36)).slice()
  new DataView(hheaTable.buffer).setUint16(34, n)
  const maxpTable = new Uint8Array(bytes.buffer, bytes.byteOffset + tables.maxp.offset, Math.min(tables.maxp.length, 32)).slice()
  new DataView(maxpTable.buffer).setUint16(4, n)

  // cmap
  const entries = charGids.map(e => ({ code: e.code, gid: newIndex.get(e.gid) }))
    .filter(e => e.gid != null).sort((a, b) => a.code - b.code)
  const cmapTable = buildCmap(entries)

  // post 3.0（32 字节空表）
  const post = new Uint8Array(32)
  new DataView(post.buffer).setUint32(0, 0x00030000)

  const out = { head: headTable, hhea: hheaTable, maxp: maxpTable, cmap: cmapTable, glyf: glyfTable, loca: locaTable, hmtx: hmtxTable, post }
  if (tables.name) out.name = new Uint8Array(bytes.buffer, bytes.byteOffset + tables.name.offset, tables.name.length).slice()
  if (tables['OS/2']) out['OS/2'] = new Uint8Array(bytes.buffer, bytes.byteOffset + tables['OS/2'].offset, tables['OS/2'].length).slice()

  // 组装 sfnt
  const tags = Object.keys(out).sort()
  const nt = tags.length
  const searchRange = Math.pow(2, Math.floor(Math.log2(nt))) * 16
  const entrySelector = Math.floor(Math.log2(nt))
  const header = new Uint8Array(12 + nt * 16)
  const hv = new DataView(header.buffer)
  hv.setUint32(0, 0x00010000)
  hv.setUint16(4, nt)
  hv.setUint16(6, searchRange)
  hv.setUint16(8, entrySelector)
  hv.setUint16(10, nt * 16 - searchRange)
  const padded = tags.map(t => pad4(out[t]))
  let off = header.length
  const checkSums = []
  tags.forEach((t, i) => {
    const data = padded[i]
    const cs = checksum(data)
    checkSums.push(cs)
    const o = 12 + i * 16
    for (let k = 0; k < 4; k++) header[o + k] = t.charCodeAt(k)
    hv.setUint32(o + 4, cs)
    hv.setUint32(o + 8, off)
    hv.setUint32(o + 12, out[t].length)
    off += data.length
  })
  const file = concat([header, ...padded])
  // head.checkSumAdjustment
  const headIdx = tags.indexOf('head')
  const fileSum = checksum(file)
  const adj = (0xB1B0AFBA - fileSum) >>> 0
  const headStart = hv.getUint32(12 + headIdx * 16 + 8)
  new DataView(file.buffer).setUint32(headStart + 8, adj)
  void checkSums
  return { data: file, numGlyphs: n, keptChars: entries.length, missing }
}

export function bytesToBase64(data) {
  let s = ''
  const CH = 0x8000
  for (let i = 0; i < data.length; i += CH) {
    s += String.fromCharCode.apply(null, data.subarray(i, i + CH))
  }
  return btoa(s)
}

export { FontError }
