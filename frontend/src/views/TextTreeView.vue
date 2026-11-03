<template>
  <div class="text-tree-page">
    <div class="toolbar">
      <span class="title">纯文字世系图</span>
      <el-checkbox v-model="showGenLabel" label="世代/字辈" size="small" />
      <el-button size="small" @click="zoom = Math.min(2, +(zoom + 0.1).toFixed(2))">放大</el-button>
      <el-button size="small" @click="zoom = Math.max(0.4, +(zoom - 0.1).toFixed(2))">缩小</el-button>
      <el-button size="small" @click="zoom = 1">复位</el-button>
      <el-dropdown size="small" trigger="click" @command="doExport">
        <el-button size="small">导出<el-icon class="el-icon--right"><arrow-down /></el-icon></el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="png">🖼️ PNG（高清 2×）</el-dropdown-item>
            <el-dropdown-item command="svg">📐 SVG（矢量图）</el-dropdown-item>
            <el-dropdown-item command="pdf" divided>🖨️ PDF（打印用）</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <span class="hint">纯文字 + 直角连线 · 姓名竖排 · 排行/标注在名字左侧</span>
    </div>
    <div ref="wrapRef" class="canvas-wrap"
         @wheel.prevent="onWheel"
         @mousedown="onDown" @mousemove="onMove" @mouseup="onUp" @mouseleave="onUp"
         @dblclick="zoom = 1"
         @touchstart.passive="onTouchStart" @touchmove="onTouchMove" @touchend="onTouchEnd">
      <svg :width="width * zoom" :height="height * zoom" :viewBox="`0 0 ${width} ${height}`">
        <g v-for="(n, i) in layout.nodes" :key="i">
          <!-- 排行：竖排（行/一 各占一行），中文数字 -->
          <text v-for="(ch, ri) in n.rankChars" :key="'r' + ri" :x="n.x - 16" :y="n.top + 14 + ri * 15"
                text-anchor="middle" font-size="12" fill="#8a7f6d">{{ ch }}</text>
          <!-- 出继 / 入继 / 收养 / 失考 / 无嗣：竖排，按短语分列（两种失考各占一列） -->
          <g v-for="(col, ci) in n.markCols" :key="'mc' + ci">
            <text v-for="(ch, mi) in col" :key="mi" :x="n.x - 32 - ci * 14" :y="n.top + 14 + mi * 14"
                  text-anchor="middle" font-size="11" fill="#a08060">{{ ch }}</text>
          </g>
          <!-- 姓名：竖排一字一行 -->
          <text v-for="(ch, ci) in n.chars" :key="ci" :x="n.x" :y="n.top + 14 + ci * 18"
                text-anchor="middle" font-size="15" class="nm" :fill="n.outCopy ? '#8a7f6d' : '#2f2a22'">{{ ch }}</text>
          <!-- 女儿：在排行列下方加「女」 -->
          <text v-if="n.isFemale" :x="n.x - 16" :y="n.top + 14 + n.rankChars.length * 15"
                text-anchor="middle" font-size="12" fill="#8a7f6d">女</text>
          <!-- 配偶：与本人并排（右侧），正配/继配标签在配偶名字的右边 -->
          <g v-for="(sp, si) in n.spouses" :key="'sp' + si">
            <text v-for="(ch, ci) in sp.chars" :key="ci" :x="n.x + 46 + si * 44" :y="n.top + 14 + ci * 16"
                  text-anchor="middle" font-size="13" class="nm" fill="#7a6a52">{{ ch }}</text>
            <text v-for="(ch, ci) in sp.typeChars" :key="'t' + ci" :x="n.x + 60 + si * 44" :y="n.top + 20 + ci * 11"
                  text-anchor="middle" font-size="9" fill="#b9b0a0">{{ ch }}</text>
          </g>
        </g>
        <!-- 连线 -->
        <g>
          <path v-for="(d, i) in layout.lines" :key="'l' + i" :d="d" fill="none" stroke="#6b6152" stroke-width="1.1" />
        </g>
        <!-- 右侧世代标签（可开关） -->
        <g v-if="showGenLabel">
          <text v-for="(g, i) in layout.gens" :key="'g' + i" :x="width - 40" :y="g.y" font-size="11"
                fill="#8a7f6d" text-anchor="start">{{ g.label }}</text>
          <text v-for="(w, i) in layout.words" :key="'w' + i" :x="width - 12" :y="w.y" font-size="11"
                fill="#b9b0a0" text-anchor="start">{{ w.word }}</text>
        </g>
      </svg>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import api from '../api'
import { memberKnown, lostContactText } from '../utils/memberFlags'
import { ArrowDown } from '@element-plus/icons-vue'

const members = ref([])
const zoom = ref(1)
const showGenLabel = ref(true)

const NAME_LH = 18      // 竖排每字行高
const TOP = 28          // 顶部留白
const ROW_H = 104       // 世代行高
const COL_W = 150       // 每个叶子槽位宽度（含配偶位）
const SIDE = 60         // 左右留白

function fullName(m) {
  return m.name || ((m.surname || '') + (m.given_name || '')) || '（无名）'
}
function rankNum(r) {
  if (r == null) return null
  const s = String(r).trim()
  if (!s) return null
  const m = s.match(/\d+/)
  if (m) return parseInt(m[0], 10)
  const cn = { 一: 1, 二: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9, 十: 10 }
  for (const ch of s.replace(/^(行|第|排行|排)/, '')) if (cn[ch]) return cn[ch]
  return null
}
// 行一 / 行二 …
const CN = ['', '一', '二', '三', '四', '五', '六', '七', '八', '九', '十']
// 1..99 → 中文数字（十一、二十、二十三…）
function cnNum(n) {
  if (n <= 10) return CN[n] || String(n)
  const t = Math.floor(n / 10), r = n % 10
  return (t === 1 ? '十' : CN[t] + '十') + (r ? CN[r] : '')
}
function rankLabel(r) {
  const n = rankNum(r)
  if (!n) return ''
  return '行' + cnNum(n)
}


// ===== 交互：滚轮缩放、双指捏合、拖拽平移、双击复位 =====
const wrapRef = ref(null)
function setZoomOf(nz, clientX, clientY) {
  const wrap = wrapRef.value
  const z = Math.min(3, Math.max(0.3, +nz.toFixed(2)))
  if (!wrap) { zoom.value = z; return }
  const rect = wrap.getBoundingClientRect()
  const cx = (clientX == null ? rect.left + wrap.clientWidth / 2 : clientX) - rect.left
  const cy = (clientY == null ? rect.top + wrap.clientHeight / 2 : clientY) - rect.top
  const ratio = z / zoom.value
  const sx = (wrap.scrollLeft + cx) * ratio - cx
  const sy = (wrap.scrollTop + cy) * ratio - cy
  zoom.value = z
  nextTick(() => { wrap.scrollLeft = sx; wrap.scrollTop = sy })
}
function onWheel(e) {
  const step = e.deltaY > 0 ? -0.08 : 0.08
  setZoomOf(zoom.value + step, e.clientX, e.clientY)
}
let pinch = null
function onTouchStart(e) {
  if (e.touches.length === 2) {
    const [a, b] = e.touches
    pinch = {
      dist: Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY),
      zoom: zoom.value,
      cx: (a.clientX + b.clientX) / 2,
      cy: (a.clientY + b.clientY) / 2
    }
  }
}
function onTouchMove(e) {
  if (e.touches.length === 2 && pinch) {
    e.preventDefault()
    const [a, b] = e.touches
    const dist = Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY)
    if (pinch.dist > 0) setZoomOf(pinch.zoom * (dist / pinch.dist), pinch.cx, pinch.cy)
  }
}
function onTouchEnd() { pinch = null }
let drag = null
function onDown(e) {
  if (e.button !== 0) return
  const wrap = wrapRef.value
  if (!wrap) return
  drag = { x: e.clientX, y: e.clientY, sl: wrap.scrollLeft, st: wrap.scrollTop }
}
function onMove(e) {
  if (!drag) return
  const wrap = wrapRef.value
  if (!wrap) return
  wrap.scrollLeft = drag.sl - (e.clientX - drag.x)
  wrap.scrollTop = drag.st - (e.clientY - drag.y)
}
function onUp() { drag = null }


// ===== 导出 =====
function svgMarkup() {
  const svg = wrapRef.value && wrapRef.value.querySelector('svg')
  if (!svg) return ''
  const clone = svg.cloneNode(true)
  clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg')
  // 导出用 100% 尺寸（不受当前缩放影响）
  clone.setAttribute('width', width.value)
  clone.setAttribute('height', height.value)
  clone.setAttribute('viewBox', `0 0 ${width.value} ${height.value}`)
  clone.style.background = '#fff'
  return new XMLSerializer().serializeToString(clone)
}
function downloadBlob(name, blob) {
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = name
  a.click()
  setTimeout(() => URL.revokeObjectURL(a.href), 3000)
}
function doExport(cmd) {
  if (cmd === 'png') return exportPng()
  if (cmd === 'svg') return exportSvg()
  if (cmd === 'pdf') return exportPdf()
}
function exportPdf() {
  const str = svgMarkup()
  if (!str) return
  const w = window.open('', '_blank')
  if (!w) return
  // 注意：SFC 里不能出现字面量 script 标签，拆开拼接
  const sc = '<' + 'script>window.onload=function(){setTimeout(function(){window.print()},300)}</' + 'script>'
  w.document.write(`<html><head><title>世系图</title></head><body style="margin:0;background:#fff">${str}${sc}</body></html>`)
  w.document.close()
}
function exportSvg() {
  const str = svgMarkup()
  if (!str) return
  downloadBlob(`世系图-纯文字-${Date.now()}.svg`, new Blob([str], { type: 'image/svg+xml;charset=utf-8' }))
}
function exportPng() {
  const str = svgMarkup()
  if (!str) return
  const img = new Image()
  const url = URL.createObjectURL(new Blob([str], { type: 'image/svg+xml;charset=utf-8' }))
  img.onload = () => {
    const scale = 2   // 两倍图，打印清晰
    const canvas = document.createElement('canvas')
    canvas.width = width.value * scale
    canvas.height = height.value * scale
    const ctx = canvas.getContext('2d')
    ctx.fillStyle = '#fff'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.drawImage(img, 0, 0, canvas.width, canvas.height)
    canvas.toBlob(b => { if (b) downloadBlob(`世系图-纯文字-${Date.now()}.png`, b) }, 'image/png')
    URL.revokeObjectURL(url)
  }
  img.src = url
}

const layout = computed(() => {
  const list = members.value || []
  const empty = { nodes: [], lines: [], gens: [], words: [], width: 800, height: 400 }
  if (!list.length) return empty

  // 出继/入继：本人挂承嗣父，生父下加一个只写说明的副本
  const ids = new Set(list.map(m => m.id))
  const items = []
  list.forEach(m => {
    const bioId = m.father_id || 0
    const adoptId = m.adopt_father_id || 0
    if (adoptId > 0 && adoptId !== bioId && ids.has(adoptId)) {
      items.push({ ...m, father_id: adoptId, _bioFather: bioId })
      if (bioId > 0 && ids.has(bioId)) {
        items.push({ ...m, id: -m.id, father_id: bioId, adopt_father_id: 0,
          _outCopy: true, _uid: m.id + '-out' })
      }
    } else {
      items.push(m)
    }
  })

  const byId = new Map()
  items.forEach(m => byId.set(m.id, { ...m, children: [] }))
  const roots = []
  byId.forEach(n => {
    const pid = n.father_id || 0
    if (pid && byId.has(pid)) byId.get(pid).children.push(n)
    else roots.push(n)
  })
  // 兄弟：组内按排行 → id（同父的子女天然在一组）
  const sortKids = arr => arr.sort((a, b) => {
    const ra = rankNum(a.rank), rb = rankNum(b.rank)
    if (ra != null && rb != null && ra !== rb) return ra - rb
    if (ra != null && rb == null) return -1
    if (ra == null && rb != null) return 1
    return a.id - b.id
  })
  byId.forEach(n => sortKids(n.children))
  roots.sort((a, b) => (a.generation || 0) - (b.generation || 0) || a.id - b.id)

  // tidy 布局：叶子占槽位，父居中
  let leaf = 0
  const walk = n => {
    if (!n.children.length) { n.slot = leaf++ }
    else { n.children.forEach(walk); n.slot = (n.children[0].slot + n.children[n.children.length - 1].slot) / 2 }
  }
  roots.forEach(walk)

  const minGen = Math.min(...items.map(m => m.generation || 0))
  const nodes = []
  const lines = []
  const pos = new Map()
  byId.forEach(n => {
    const chars = Array.from(fullName(n))
    const x = SIDE + n.slot * COL_W
    const top = TOP + ((n.generation || 0) - minGen) * ROW_H
    // 标记优先级：失考 > 出继/入继/收养 > 无嗣（同一节点只出一个）
    const noKids = !(n.children && n.children.length)
    let mark = ''
    if (n.lost_contact) {
      // 保留「，」：下面按短语分列，两种失考各占一列（去掉逗号会被按 6 字硬切）
      mark = lostContactText(memberKnown(n), !noKids, false)
    } else if (n._outCopy) {
      mark = '出继'
    } else if (n.adopt_father_id) {
      mark = (n._bioFather || 0) > 0 ? '入继' : '收养'
    } else if (n.no_heir && noKids) {
      mark = '无嗣'
    }
    const node = {
      id: n.id, x, top, chars,
      rankChars: Array.from(rankLabel(n.rank || '')),
      isFemale: Number(n.gender) === 2,
      spouses: (n.spouses || []).map(sp => ({
        chars: Array.from(sp.name || ((sp.surname || '') + (sp.given_name || '')) || '氏'),
        typeChars: Array.from(sp.type || '配')
      })),
      markCols: (() => {
        // 每个短语一列；单列超过 6 字再折列（失考两句各占一列）
        const cols = []
        String(mark).split('，').forEach(ph => {
          const cs = Array.from(ph)
          for (let i = 0; i < cs.length; i += 6) cols.push(cs.slice(i, i + 6))
        })
        return cols
      })(),
      outCopy: !!n._outCopy
    }
    nodes.push(node)
    pos.set(n.id, node)
  })
  // 连线：父姓名下沿 → 竖 → 横 → 各子姓名上沿
  byId.forEach(n => {
    const p = pos.get(n.id)
    if (!p || !n.children.length) return
    const pBottom = p.top + p.chars.length * NAME_LH + 6
    const kidNodes = n.children.map(c => pos.get(c.id)).filter(Boolean)
    if (!kidNodes.length) return
    const barY = Math.min(...kidNodes.map(k => k.top)) - 20
    lines.push(`M${p.x} ${pBottom} L${p.x} ${barY}`)
    const xs = kidNodes.map(k => k.x)
    lines.push(`M${Math.min(...xs)} ${barY} L${Math.max(...xs)} ${barY}`)
    kidNodes.forEach(k => lines.push(`M${k.x} ${barY} L${k.x} ${k.top - 6}`))
  })

  // 右侧世代/字辈标签
  const gens = []
  const words = []
  const genSet = [...new Set(items.map(m => m.generation || 0))].sort((a, b) => a - b)
  genSet.forEach(g => {
    const y = TOP + (g - minGen) * ROW_H + 14
    gens.push({ y, label: `${g}世` })
    const w = items.find(m => (m.generation || 0) === g && m.generation_word)
    words.push({ y, word: w ? w.generation_word : '' })
  })

  const maxSlot = Math.max(...nodes.map(n => (n.x - SIDE) / COL_W))
  const width = SIDE * 2 + maxSlot * COL_W + 80
  const maxTop = Math.max(...nodes.map(n => n.top + n.chars.length * NAME_LH))
  const height = maxTop + 60
  return { nodes, lines, gens, words, width, height }
})

const width = computed(() => layout.value.width)
const height = computed(() => layout.value.height)

onMounted(async () => {
  try {
    const { data } = await api.get('/members/with-spouses')
    members.value = data || []
  } catch (e) {
    members.value = []
  }
})
</script>

<style scoped>
.text-tree-page { padding: 12px; }
.toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.toolbar .title { font-weight: 600; }
.toolbar .hint { color: #999; font-size: 12px; }
.canvas-wrap { height: calc(100vh - 150px); overflow: auto; border: 1px solid #eee; background: #fff; touch-action: none; cursor: grab; }
.nm { font-family: "Songti SC", "SimSun", serif; }
</style>
