<template>
  <div class="bp-wrap">
    <div class="bp-float">
      <el-button class="bp-fab" circle title="回到顶部" @click="scrollTop">↑</el-button>
      <el-button class="bp-fab" circle title="返回" @click="goBack">←</el-button>
    </div>
    <div class="bp-bar">
      <span class="bp-title">五世一提 · 书页版（A3 横版）</span>
      <span class="bp-hint">共 {{ pages.length }} 页 · 谱名「{{ tanghao }}」· 版心居中 / 书耳在右 / 行内自右向左</span>
      <el-button size="small" @click="printAll">打印 / 另存 PDF</el-button>
    </div>
    <div v-if="errMsg" class="bp-err">{{ errMsg }}</div>
    <div v-if="missingNote" class="bp-err">{{ missingNote }}</div>
    <div v-else-if="!pages.length" class="bp-hint" style="padding:20px">暂无成员数据</div>
    <div v-for="(p, pi) in pages" :key="'p' + pi" class="bp-page">
      <BookPageChrome :title="tanghao" :meta="p.meta" :page-no="pi + 1" :ear-text="p.ear">
        <svg :viewBox="`0 0 ${PAGE_W} ${PAGE_H}`" class="bp-svg" preserveAspectRatio="none">
          <defs>
            <clipPath :id="'bpclip' + pi">
              <rect x="0" y="0" :width="PAGE_W" :height="PAGE_H" />
            </clipPath>
          </defs>
          <g :clip-path="`url(#bpclip${pi})`">
            <g v-for="(b, bi) in p.blocks" :key="'b' + bi" :transform="`translate(${b.x} ${b.y}) scale(${b.s})`">
              <!-- 世代行：块内左侧小字 + 贯穿本块的细分隔线 -->
              <g v-for="(r, ri) in b.rows" :key="'r' + ri">
                <text :x="2" :y="r.y + 13" text-anchor="start" font-size="11" fill="#a08060">{{ r.label }}</text>
                <line :x1="LABEL_W - 4" :y1="r.y + 18" :x2="b.w - 6" :y2="r.y + 18" stroke="#ded5c4" stroke-width="0.8" />
              </g>
              <!-- 直角连线：父名下方 → 竖 → 横杠 → 各子名上方 -->
              <path v-for="(d, li) in b.lines" :key="'l' + li" :d="d" fill="none" stroke="#6b6152" stroke-width="1.1" />
              <!-- 成员：排行/标注（名左）+ 姓名（竖排）+ 配偶（名右） -->
              <g v-for="(n, ni) in b.nodes" :key="'n' + ni" :data-mid="n.id">
                <text v-for="(ch, ci) in n.rankChars" :key="'rk' + ci" :x="n.x - 16" :y="n.top + 12 + ci * 15"
                      text-anchor="middle" font-size="14" fill="#8a7f6d">{{ ch }}</text>
                <g v-for="(col, ci) in n.markCols" :key="'mk' + ci">
                  <text v-for="(ch, mi) in col" :key="mi" :x="n.x - 32 - ci * 14" :y="n.top + 12 + mi * 14"
                        text-anchor="middle" font-size="13" fill="#a08060">{{ ch }}</text>
                </g>
                <text v-for="(ch, ci) in n.chars" :key="'nm' + ci" :x="n.x" :y="n.top + 12 + ci * 18"
                      text-anchor="middle" font-size="17" fill="#2f2a22" class="nm">{{ ch }}</text>
                <g v-for="(sp, si) in n.spouses" :key="'sp' + si">
                  <text v-for="(ch, ci) in sp.chars" :key="'sc' + ci" :x="n.x + SPW + si * SPW" :y="n.top + 12 + ci * 17"
                        text-anchor="middle" font-size="15" fill="#7a6a52" class="nm">{{ ch }}</text>
                  <text v-for="(ch, ci) in sp.typeChars" :key="'st' + ci" :x="n.x + SPW + 13 + si * SPW" :y="n.top + 18 + ci * 12"
                        text-anchor="middle" font-size="11" fill="#b9b0a0">{{ ch }}</text>
                </g>
              </g>
            </g>
          </g>
        </svg>
      </BookPageChrome>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../api'
import { memberKnown, lostContactText } from '../utils/memberFlags'
import BookPageChrome from './BookPageChrome.vue'

defineProps({ tanghao: { type: String, default: '族譜' } })
const members = ref([])
const errMsg = ref('')
// 紧凑排版：半幅内先自右向左并排、摆满一棚再换棚（关掉 = 一块占一棚，一页两块，版面更疏）

/* ===== 纸张几何：1 SVG 单位 = 1 纸面 px =====
   .page 1587×1123 −页边 9×2 −外框(1+5)×2 −书芯边框 3×2 −书耳 44 = 1507 × 1087（实测 .page-body） */
const PAGE_W = 1507
const PAGE_H = 1087
const SPINE_W = 84                     // 版心宽度：左右半幅之间的禁区，名字不得进入（连线可穿）
const BOX_W = (PAGE_W - SPINE_W) / 2   // 半幅 711.5
const BOX_L = 0
const BOX_R = BOX_W + SPINE_W          // 795.5
const GUTTER = 10                      // 半幅内边距
const BOX_GAP = 26
const BAND_GAP = 30                    // 同一块折成上下几带时，带与带之间的空档（父子线从这里绕过去）                     // 同一半幅内两块之间（上下/左右同一个间距，留白更松快）

/* ===== 块内排版参数（纯文字连线：竖排姓名 + 直角连线） ===== */
const NAME_LH = 18        // 姓名竖排每字行高
const ROW_H = 96          // 一世行高（104→96：两个五世块正好上下摞进同一页，又不挤）
const LP = 50             // 名字左侧留给「行一」「出继/无嗣」竖排标注（字号+2 后放宽 4）
const SPW = 24            // 名字中心 → 配偶中心的距离（44→24：名字与配偶之间只留原来约 1/4 的空）
const COL_GAP = 9         // 相邻叶子之间横向间隔（18→9：成员之间不留那么多空）
const TOP_PAD = 26        // 块内顶部留白
const LABEL_W = 38        // 块内左侧「世代小字」栏宽度（标签写在块内，不越出书芯）

const nm = m => m.name || ((m.surname || '') + (m.given_name || '')) || '（无名）'
const cnNum = (n) => {
  const CN = ['', '一', '二', '三', '四', '五', '六', '七', '八', '九', '十']
  if (n <= 10) return CN[n] || String(n)
  const t = Math.floor(n / 10), r = n % 10
  return (t === 1 ? '十' : CN[t] + '十') + (r ? CN[r] : '')
}
// 世次：generation=0 是「外紀」先人（不入世次），其余按第 N 世
const genLabel = g => (g <= 0 ? '外紀' : '第' + cnNum(g) + '世')
function rankChars(m) {
  const s = String(m.rank || '').trim()
  if (!s) return []
  const d = s.match(/\d+/)
  if (d) return Array.from('行' + cnNum(parseInt(d[0], 10)))
  return Array.from(s.replace(/^(行|第|排行|排)/, '行'))
}
// 标注：迁居失联 / 出继·入继·收养 / 无嗣（与「纯文字连线」大方块图同一套文案）
function markCols(m, hasKids) {
  let mark = ''
  if (m.lost_contact) mark = lostContactText(memberKnown(m), !hasKids, false)
  else if (m.adopt_father_id) mark = (m.father_id || 0) > 0 ? '入继' : '收养'
  else if (m.no_heir && !hasKids) mark = '无嗣'
  const cols = []
  String(mark).split('，').filter(Boolean).forEach((ph) => {
    const cs = Array.from(ph)
    for (let i = 0; i < cs.length; i += 6) cols.push(cs.slice(i, i + 6))
  })
  return cols
}

// 五世一提：每页 5 世，相邻两页首尾重叠 1 世（第 1 页 1~5 世，第 2 页 5~9 世 …）
const STEP = 4
const SPAN = 5

const pages = computed(() => {
  errMsg.value = ''
  try {
    const list = members.value || []
    if (!list.length) return []
    const genOf = m => Number(m.generation) || 0
    const byId = new Map(list.map(m => [m.id, m]))
    const childrenOf = new Map()
    list.forEach((m) => {
      const f = m.father_id || 0
      if (!byId.has(f)) return
      if (!childrenOf.has(f)) childrenOf.set(f, [])
      childrenOf.get(f).push(m)
    })
    const rk = (s) => { const t = String(s || '').match(/\d+/); return t ? parseInt(t[0], 10) : 9999 }
    childrenOf.forEach(a => a.sort((x, y) => rk(x.rank) - rk(y.rank) || x.id - y.id))
    const kidsOf = id => childrenOf.get(id) || []
    const subtree = (id) => {           // 任意深度的后代
      const out = [], st = [id]
      while (st.length) { const c = st.pop(); out.push(c); kidsOf(c).forEach(k => st.push(k.id)) }
      return out
    }

    /* ===== 1. 分数据：按五世窗口切，窗口内再按「房」（窗口首世的人）拆块 ===== */
    const gens = list.map(genOf)
    const gMin = Math.min(...gens)
    const gMax = Math.max(...gens)
    const rawBlocks = []
    for (let g0 = gMin; g0 <= gMax; g0 += STEP) {
      const g1 = Math.min(g0 + SPAN - 1, gMax)
      const inWin = id => genOf(byId.get(id)) >= g0 && genOf(byId.get(id)) <= g1
      const used = new Set()
      list.filter(m => genOf(m) === g0).forEach((h) => {
        const ids = subtree(h.id).filter(inWin)
        if (!ids.length) return
        ids.forEach(id => used.add(id))
        rawBlocks.push({ g0, g1, head: h.id, ids })
      })
      // 父不在名单内 / 非这些房的后代：单独成块，别把人漏掉
      const rest = list.filter(m => inWin(m.id) && !used.has(m.id)).map(m => m.id)
      if (rest.length) rawBlocks.push({ g0, g1, head: 0, ids: rest })
    }

    /* ===== 2. 块内排版：宽了就**在半幅内折成上下几带**（属主：要上下，不要左右） =====
       带内：叶子按顺序占列、父居中；每带左侧留 LABEL_W 写「第X世」，名字不越出书芯。 */
    const MAXW = BOX_W - GUTTER * 2
    const usable = MAXW - LABEL_W - 8          // 每带可用的横向宽度
    function layoutBlock(ids) {
      const set = new Set(ids)
      const rpOf = (id) => {                   // 名字右侧到配偶结束的宽度
        const ns = (byId.get(id)?.spouses || []).length
        return ns ? SPW * ns + 20 : 12        // 配偶列 + 右侧「正配」小字
      }
      const wOf = id => LP + rpOf(id)
      const kidsIn = id => kidsOf(id).filter(k => set.has(k.id)).map(k => k.id)
      // ① 自然排版（先不分带）
      const nx = new Map()
      let cursor = 0
      const place = (id, depth) => {
        const startC = cursor
        const ks = kidsIn(id)
        if (!ks.length) { nx.set(id, cursor + LP); cursor += wOf(id) + COL_GAP; return }
        ks.forEach(k => place(k, depth + 1))
        const xs = ks.map(k => nx.get(k))
        let cx = (Math.min(...xs) + Math.max(...xs)) / 2
        if (cx - LP < startC) cx = startC + LP        // 父名不能压住左边已经排好的内容
        nx.set(id, cx)
        // 父比子女宽时，把游标推到父的右沿之后，免得下一个子树压上来
        cursor = Math.max(cursor, cx + rpOf(id) + COL_GAP)
      }
      const rootsIn = ids.filter(id => !set.has(byId.get(id)?.father_id || 0))
      rootsIn.forEach((r, i) => { if (i) cursor += 20; place(r, 0) })
      const natW = Math.max(...ids.map(id => nx.get(id) + rpOf(id))) + 8
      // ② 超过一带宽就按 x 折成上下几带
      const nBands = Math.max(1, Math.ceil(natW / usable))
      const bandOf = new Map()
      ids.forEach(id => bandOf.set(id, Math.max(0, Math.min(nBands - 1, Math.floor(nx.get(id) / usable)))))
      const bands = []
      let top = 0
      for (let b = 0; b < nBands; b++) {
        const inB = ids.filter(id => bandOf.get(id) === b)
        if (!inB.length) { bands.push({ top, h: 0, minG: 0, maxG: -1 }); continue }
        const gs = inB.map(id => genOf(byId.get(id)))
        const minG = Math.min(...gs), maxG = Math.max(...gs)
        const h = TOP_PAD + (maxG - minG + 1) * ROW_H + 8
        bands.push({ top, h, minG, maxG })
        top += h + BAND_GAP
      }
      // ③ 带内坐标
      const X = new Map(), Y = new Map()
      ids.forEach((id) => {
        const b = bandOf.get(id)
        const bd = bands[b]
        X.set(id, LABEL_W + (nx.get(id) - b * usable))
        Y.set(id, bd.top + TOP_PAD + (genOf(byId.get(id)) - bd.minG) * ROW_H)
      })
      // ④ 折带后按「同带的子女」重新给父名居中 —— 否则父名可能落在同带横杠的外面，横线看起来没对齐
      ids.forEach((id) => {
        const ks = kidsIn(id).filter(k => bandOf.get(k) === bandOf.get(id))
        if (!ks.length) return
        const xs = ks.map(k => X.get(k))
        X.set(id, (Math.min(...xs) + Math.max(...xs)) / 2)
      })
      const w = Math.max(...ids.map(id => X.get(id) + rpOf(id))) + 8   // 真实宽度（不夹紧，交给装页缩放兜底）
      return { X, Y, w, h: Math.max(top, TOP_PAD + ROW_H), bandOf, bands, nBands }
    }
    const laid = ids => ({ ids, L: layoutBlock(ids) })

    const pieces = []
    rawBlocks.forEach((b) => {
      const one = laid(b.ids)
      const set = new Set(b.ids)
      const headId = b.ids.find(id => !set.has(byId.get(id)?.father_id || 0)) || b.ids[0]
      pieces.push({ ...b, ...one, headId, parentId: byId.get(headId)?.father_id || 0 })
    })

    /* ===== 4. 生成绘制数据（节点 / 连线 / 世代行）；跨带的父子线在两条带之间绕过去 ===== */
    const drawables = pieces.map((b) => {
      const set = new Set(b.ids)
      const kidsIn = id => kidsOf(id).filter(k => set.has(k.id)).map(k => k.id)
      const nodes = b.ids.map((id) => {
        const m = byId.get(id)
        return {
          id, x: b.L.X.get(id), top: b.L.Y.get(id),
          chars: Array.from(nm(m)), rankChars: rankChars(m), markCols: markCols(m, kidsOf(id).length > 0),
          spouses: (m.spouses || []).map(sp => ({
            chars: Array.from(sp.name || ((sp.surname || '') + (sp.given_name || '')) || '氏'),
            typeChars: Array.from(sp.type || '配')
          }))
        }
      })
      const lines = []
      b.ids.forEach((id) => {
        const ks = kidsIn(id)
        if (!ks.length) return
        const px = b.L.X.get(id)
        const chs = Array.from(nm(byId.get(id)))
        const pBottom = b.L.Y.get(id) + 12 + chs.length * NAME_LH + 4
        const band = b.L.bandOf.get(id)
        const same = ks.filter(k => b.L.bandOf.get(k) === band)
        const cross = ks.filter(k => b.L.bandOf.get(k) !== band)
        let barY = 0
        if (same.length) {
          barY = Math.min(...same.map(k => b.L.Y.get(k))) - 22
          lines.push(`M${px} ${pBottom} L${px} ${barY}`)
          const xs = same.map(k => b.L.X.get(k))
          lines.push(`M${Math.min(...xs)} ${barY} L${Math.max(...xs)} ${barY}`)
          same.forEach((k) => lines.push(`M${b.L.X.get(k)} ${barY} L${b.L.X.get(k)} ${b.L.Y.get(k) - 14}`))
        }
        // 跨带：**从同一横杠的末端接着延伸**（不是从父名另起一条）——
        // 横杠右端（本带最右那个子女）沿本块边走下来，到两带之间的空档再横过去、竖直进入下一带的头
        cross.forEach((k) => {
          const kb = b.L.bandOf.get(k)
          const bd = b.L.bands[kb]
          const laneY = bd.top - BAND_GAP / 2            // 两条带之间的空档
          const cx = b.L.X.get(k)
          const cy = b.L.Y.get(k) - 14
          const edgeL = LABEL_W - 6
          const edgeR = Math.max(b.L.w - 6, edgeL + 40)
          // 起点：有同带子女就从横杠末端起，没有才从父名下方起
          const sx = same.length ? Math.max(...same.map(kk => b.L.X.get(kk))) : px
          const sy = same.length ? barY : pBottom
          const laneX = (Math.abs(sx - edgeR) < Math.abs(sx - edgeL)) ? edgeR : edgeL
          lines.push(`M${sx.toFixed(1)} ${sy.toFixed(1)} L${laneX.toFixed(1)} ${sy.toFixed(1)} ` +
            `L${laneX.toFixed(1)} ${laneY.toFixed(1)} L${cx.toFixed(1)} ${laneY.toFixed(1)} L${cx.toFixed(1)} ${cy.toFixed(1)}`)
        })
      })
      const rows = []
      b.L.bands.forEach((bd) => {
        for (let g = bd.minG; g <= bd.maxG; g++) {
          rows.push({ y: bd.top + (g - bd.minG) * ROW_H, label: genLabel(g) })
        }
      })
      return { w: b.L.w, h: b.L.h, nodes, lines, rows, g0: b.g0, g1: b.g1, headId: b.headId, parentId: b.parentId }
    })

    /* ===== 5. 装页：纯「上下」堆叠 =====
       属主 2026-11-06：**只要上下，不要左右**——一个块接一个块往下摞，摞满这半页就换下一页
       （左半 = 下一頁），绝不并排。块太宽就按房拆成几段，段与段也是上下摞。 */
    const availW = BOX_W - GUTTER * 2
    const availH = PAGE_H - GUTTER * 2
    const out = []
    let page = null
    let half = 1                        // 1 = 右半（书页的右页），0 = 左半（下一页）
    let yCur = 0
    const boxX0 = () => (half === 1 ? BOX_R : BOX_L) + GUTTER
    const newPage = () => { page = { blocks: [], g0: null, g1: null }; out.push(page); half = 1; yCur = 0 }
    const nextHalf = () => { if (half === 1) { half = 0; yCur = 0 } else newPage() }
    drawables.forEach((b) => {
      const s = Math.min(1, availW / b.w, availH / b.h)     // 单块比整页还大才等比缩小（正常不会）
      const w = b.w * s
      const h = b.h * s
      if (!page) newPage()
      if (yCur > 0 && yCur + h > availH) nextHalf()          // 这一页摞满了 → 下一页
      page.blocks.push({
        x: boxX0(), y: GUTTER + yCur, s, w: b.w, h,
        nodes: b.nodes, lines: b.lines, rows: b.rows,
        g0: b.g0, g1: b.g1, headId: b.headId, parentId: b.parentId, boxX0: boxX0()
      })
      if (page.g0 === null || b.g0 < page.g0) page.g0 = b.g0
      if (page.g1 === null || b.g1 > page.g1) page.g1 = b.g1
      yCur += h + BOX_GAP
    })

    return out.map(pg => ({
      blocks: pg.blocks,
      meta: '五世一圖',
      ear: genLabel(pg.g0) + '至' + genLabel(pg.g1)
    }))
  } catch (e) {
    errMsg.value = String((e && e.stack) || e)
    return []
  }
})

// 自检：把「本库里所有成员」与「已经排进书页的成员」比一遍，漏了谁直接列出来
const missingNote = computed(() => {
  const list = members.value || []
  if (!list.length) return ''
  const placed = new Set()
  ;(pages.value || []).forEach((pg) => pg.blocks.forEach((b) => b.nodes.forEach((n) => placed.add(n.id))))
  const miss = list.filter(m => !placed.has(m.id))
  return miss.length ? `⚠ 有 ${miss.length} 人没排进书页：${miss.slice(0, 10).map(nm).join('、')}${miss.length > 10 ? '…' : ''}` : ''
})

function printAll() { window.print() }
function scrollTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
  const el = document.scrollingElement || document.documentElement
  if (el) el.scrollTop = 0
}
function goBack() {
  if (window.history.length > 1) window.history.back()
}

onMounted(async () => {
  try {
    const { data } = await api.get('/members/with-spouses')
    members.value = data || []
  } catch (e) { members.value = [] }
})
</script>

<style scoped>
.bp-wrap { padding: 10px; }
.bp-float { position: fixed; right: 24px; bottom: 28px; z-index: 3000; display: flex; flex-direction: column;
            align-items: center; gap: 12px; }   /* 两个按钮同一列、居中对齐 */
/* 悬浮按钮：两个一样大、同一右沿；直径 52px、箭头 22px（原来 small 太小且没对齐） */
.bp-fab { width: 52px !important; height: 52px !important; min-width: 52px !important; padding: 0 !important;
          font-size: 22px !important; line-height: 1 !important; display: flex; align-items: center; justify-content: center;
          box-shadow: 0 4px 14px rgba(0, 0, 0, .18); }
/* Element Plus 给相邻按钮加 margin-left:12px，竖排时会把第二个挤偏 6px —— 去掉 */
.bp-fab + .bp-fab { margin-left: 0 !important; }
.bp-bar { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; }
.bp-title { font-weight: 600; }
.bp-hint { color: #999; font-size: 12px; }
.bp-err { background: #fff1f0; border: 1px solid #ffa39e; color: #a8071a; padding: 6px 10px; font-size: 12px; white-space: pre-wrap; margin-bottom: 10px; }
.bp-page { background: #fff; margin-bottom: 16px; }
/* SVG 与 .page-body 同尺寸（1507×1087）：1 单位 = 1px，不 letterbox、不裁底 */
.bp-svg { width: 100%; height: 100%; display: block; }
.nm { font-family: "Songti SC", "SimSun", "STSong", "宋体", serif; }
@media print {
  @page { size: A3 landscape; margin: 0; }
  .bp-bar, .bp-float { display: none !important; }
  .bp-wrap { padding: 0; }
  .bp-page { margin: 0; page-break-after: always; break-after: page; }
}
</style>
