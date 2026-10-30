<template>
  <!-- 五服圖（服制圖）：以己身為中心的本宗九族五服
       实物依据：參考圖集 实体族谱/图版-01（服制圖：斬衰／齊衰／大功／小功／緦麻）
       全部用 SVG 矢量繪製（方框＋連線＋可選中文字），不用點陣圖，A3 打印清晰。
       版式：直系居一竖軸，旁系按「親→堂→再從→族」逐級向左階梯展開；
             右側（豎版為下方）為「五服服制表＋九族／旁系／圖說」，即五服圖例。
       layout：wide = A3 橫版（谱书一叶）；tall = A3 豎版（牒記式／支谱） -->
  <svg class="ffc" :viewBox="L.vb" preserveAspectRatio="xMidYMid meet" :style="vars"
       xmlns="http://www.w3.org/2000/svg" role="img" aria-label="五服圖：本宗九族服制">
    <!-- 連線（吊線式，先畫線後畫框，框壓住線端） -->
    <g class="ffc-links">
      <path v-for="(d, i) in links" :key="'l' + i" :d="d" fill="none" />
    </g>

    <!-- 二十五格：直系九 + 旁系十六 -->
    <g class="ffc-nodes">
      <g v-for="n in nodes" :key="'n' + n.key" :class="['ffc-node', { self: n.self }]">
        <rect :x="n.x - L.bw / 2" :y="n.y" :width="L.bw" :height="L.bh" rx="1.5" />
        <text class="ffc-name" :x="n.x" :y="n.y + L.bh * 0.40" text-anchor="middle">{{ n.name }}</text>
        <text class="ffc-fu" :x="n.x" :y="n.y + L.bh * 0.74" text-anchor="middle">{{ n.fu }}</text>
      </g>
    </g>

    <!-- 五服服制表（＝圖例：服等與所服親屬） -->
    <g class="ffc-table">
      <text class="ffc-thead" :x="T.x" :y="T.headY">{{ T.head }}</text>
      <line class="ffc-rule" :x1="T.x" :y1="T.ruleY" :x2="T.x + T.w" :y2="T.ruleY" />
      <g v-for="(r, i) in FU_ROWS" :key="'r' + i">
        <text class="ffc-grade" :x="T.gradeX" :y="T.rowY + i * T.rh">{{ r.grade }}</text>
        <text class="ffc-scope" :x="T.scopeX" :y="T.rowY + i * T.rh">{{ r.scope }}</text>
      </g>
    </g>

    <!-- 圖注：九族／旁系四等／五服要義 -->
    <g class="ffc-extra">
      <template v-for="(e, i) in L.extra" :key="'x' + i">
        <line v-if="e.rule" class="ffc-rule" :x1="e.x" :y1="e.y - 24" :x2="e.x + (e.w || 0)" :y2="e.y - 24" />
        <text :class="e.head ? 'ffc-xhead' : 'ffc-xtext'" :x="e.x" :y="e.y">{{ e.text }}</text>
      </template>
    </g>

    <!-- 圖注（己身／中軸說明） -->
    <text class="ffc-selfnote" :x="L.selfNote.x" :y="L.selfNote.y">{{ L.selfNote.text }}</text>
  </svg>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  layout: { type: String, default: 'wide' },   // wide（A3 橫版）| tall（A3 豎版）
})

// 二十五格：row（0 高祖 … 4 己身 … 8 玄孫）× k（0 直系，1 親，2 堂，3 再從，4 族）
// 服制依明清《家禮》本宗五服：斬衰三年／齊衰（三年·期·五月·三月）／大功九月／小功五月／緦麻三月
const TABLE = [
  // 上四世
  [0, 0, '高祖父母', '齊衰三月'],
  [1, 0, '曾祖父母', '齊衰五月'], [1, 1, '曾伯叔祖父母', '緦麻三月'],
  [2, 0, '祖父母', '齊衰期'], [2, 1, '伯叔祖父母', '小功五月'], [2, 2, '族伯叔祖父母', '緦麻三月'],
  [3, 0, '父母', '斬衰·齊衰'], [3, 1, '伯叔父母', '齊衰期'], [3, 2, '堂伯叔父母', '小功五月'], [3, 3, '族伯叔父母', '緦麻三月'],
  // 己身一世（居中）
  [4, 0, '己身', '五服中心'], [4, 1, '兄弟', '齊衰期'], [4, 2, '堂兄弟', '大功九月'],
  [4, 3, '再從兄弟', '小功五月'], [4, 4, '族兄弟', '緦麻三月'],
  // 下四世
  [5, 0, '子', '齊衰期'], [5, 1, '姪', '齊衰期'], [5, 2, '堂姪', '小功五月'], [5, 3, '再從姪', '緦麻三月'],
  [6, 0, '孫', '大功九月'], [6, 1, '姪孫', '小功五月'], [6, 2, '堂姪孫', '緦麻三月'],
  [7, 0, '曾孫', '小功五月'], [7, 1, '姪曾孫', '緦麻三月'],
  [8, 0, '玄孫', '緦麻三月'],
]

const FU_ROWS = [
  { grade: '斬衰三年', scope: '父' },
  { grade: '齊衰', scope: '母、祖父母、曾祖父母、高祖父母、伯叔父母、兄弟、子、姪' },
  { grade: '大功九月', scope: '堂兄弟、孫' },
  { grade: '小功五月', scope: '伯叔祖父母、堂伯叔父母、再從兄弟、堂姪、姪孫、曾孫' },
  { grade: '緦麻三月', scope: '曾伯叔祖父母、族伯叔父母、族兄弟、再從姪、堂姪孫、姪曾孫、玄孫' },
]

const LAYOUTS = {
  wide: {
    vb: '0 0 1400 1010', w: 1400, h: 1010,
    x0: 520, dx: 112, bw: 106, bh: 64, y0: 26, dy: 96,
    fs: { name: 15, fu: 10, thead: 18, grade: 15, scope: 13, xhead: 15, xtext: 13.5, note: 13 },
    table: {
      head: '五服服制', x: 806, w: 588, headY: 118, ruleY: 134,
      gradeX: 810, scopeX: 966, rowY: 170, rh: 66,
    },
    extra: [
      { text: '直系九族', x: 806, y: 566, head: true },
      { text: '高祖·曾祖·祖·父·己身·子·孫·曾孫·玄孫', x: 806, y: 598 },
      { text: '旁系四等', x: 806, y: 650, head: true },
      { text: '兄弟（期）·堂兄弟（大功）', x: 806, y: 682 },
      { text: '再從兄弟（小功）·族兄弟（緦麻）', x: 806, y: 710 },
      { text: '五服要義', x: 806, y: 764, head: true, rule: true, w: 588 },
      { text: '己身居中，上推四世、下推四世、旁推四世；', x: 806, y: 796 },
      { text: '直系服重，旁系遞殺，出五服則無服（袒免）。', x: 806, y: 824 },
    ],
    selfNote: { x: 72, y: 950, text: '圖以己身為中心：中軸為直系，向左依次為兄弟、堂、再從、族四等旁系。' },
  },
  tall: {
    vb: '0 0 1200 1640', w: 1200, h: 1640,
    x0: 940, dx: 170, bw: 150, bh: 84, y0: 56, dy: 128,
    fs: { name: 19, fu: 13, thead: 20, grade: 18, scope: 17, xhead: 18, xtext: 17, note: 16 },
    table: {
      head: '五服服制', x: 100, w: 1000, headY: 1248, ruleY: 1266,
      gradeX: 106, scopeX: 306, rowY: 1304, rh: 60,
    },
    extra: [],
    selfNote: { x: 100, y: 1608, text: '己身居中：中軸直系，向左依次為兄弟、堂、再從、族四等旁系；上殺下殺旁殺，五服而止。' },
  },
}

const L = computed(() => LAYOUTS[props.layout] || LAYOUTS.wide)
const T = computed(() => L.value.table)
const vars = computed(() => {
  const f = L.value.fs
  return {
    '--fs-name': f.name + 'px', '--fs-fu': f.fu + 'px',
    '--fs-thead': f.thead + 'px', '--fs-grade': f.grade + 'px', '--fs-scope': f.scope + 'px',
    '--fs-xhead': f.xhead + 'px', '--fs-xtext': f.xtext + 'px', '--fs-note': f.note + 'px',
  }
})

const nodes = computed(() => {
  const cfg = L.value
  return TABLE.map(([r, k, name, fu]) => ({
    key: r + '-' + k, r, k, name, fu, self: name === '己身',
    x: cfg.x0 - cfg.dx * k, y: cfg.y0 + cfg.dy * r,
  }))
})

// 每行最多幾級旁系：1,2,3,4,5,4,3,2,1
const MAXK = [0, 1, 2, 3, 4, 3, 2, 1, 0]

const links = computed(() => {
  const cfg = L.value
  const byKey = {}
  nodes.value.forEach((n) => { byKey[n.r + ',' + n.k] = n })
  const at = (r, k) => byKey[r + ',' + Math.min(k, MAXK[r])]
  const out = []
  nodes.value.forEach((n) => {
    let p = null
    if (n.r <= 4) p = at(n.r - 1, n.k === 0 ? 0 : n.k - 1)   // 上四世 + 己身：旁系向內收一級
    else p = at(n.r - 1, n.k)                                // 下四世：旁系同一支垂直下延
    if (!p) return
    const px = p.x, py = p.y + cfg.bh, cx = n.x, cy = n.y
    if (Math.abs(px - cx) < 0.5) {
      out.push('M' + px + ',' + py + 'L' + cx + ',' + cy)
    } else {
      const my = (py + cy) / 2
      out.push('M' + px + ',' + py + 'L' + px + ',' + my + 'L' + cx + ',' + my + 'L' + cx + ',' + cy)
    }
  })
  return out
})
</script>

<style scoped>
.ffc {
  width: 100%; height: 100%; display: block;
  font-family: "Songti SC", "SimSun", "STSong", "宋体", serif;
  background: transparent;
}
.ffc-links path { stroke: #1a1a1a; stroke-width: 1.3; }
.ffc-node rect { fill: #fffdf6; stroke: #1a1a1a; stroke-width: 1.5; }
.ffc-node .ffc-name { font-size: var(--fs-name); font-weight: 700; fill: #1a1a1a; }
.ffc-node .ffc-fu { font-size: var(--fs-fu); fill: #8c2f1f; }
.ffc-node.self rect { fill: #2b2b2b; stroke: #1a1a1a; stroke-width: 2.4; }
.ffc-node.self .ffc-name { fill: #fffdf6; }
.ffc-node.self .ffc-fu { fill: #f0d79a; }
.ffc-rule { stroke: #b9ad91; stroke-width: 1; }
.ffc-thead { font-size: var(--fs-thead); font-weight: 700; fill: #111; letter-spacing: 2; }
.ffc-grade { font-size: var(--fs-grade); font-weight: 700; fill: #1a1a1a; }
.ffc-scope { font-size: var(--fs-scope); fill: #333; }
.ffc-xhead { font-size: var(--fs-xhead); font-weight: 700; fill: #3a2a18; letter-spacing: 1; }
.ffc-xtext { font-size: var(--fs-xtext); fill: #4a4137; }
.ffc-selfnote { font-size: var(--fs-note); fill: #6b5a44; }
</style>
