<template>
  <div class="susi-tree">
    <div class="toolbar">
      <span class="serif title">蘇式垂絲圖</span>
      <el-input v-model="search" placeholder="搜索姓名/字号" clearable style="width: 200px" @keyup.enter="doSearch" />
      <el-button type="primary" @click="doSearch">定位</el-button>
      <el-button @click="zoomIn">放大</el-button>
      <el-button @click="zoomOut">缩小</el-button>
      <el-button @click="resetView">重置</el-button>
      <el-select v-model="recordFilter" size="small" style="width:120px" @change="onLayoutChange">
        <el-option label="内纪外纪" value="all" />
        <el-option label="仅内纪" value="内纪" />
        <el-option label="仅外纪" value="外纪" />
      </el-select>
      <el-select v-model="branchFilter" size="small" style="width:110px" @change="onLayoutChange">
        <el-option label="全部" value="all" />
        <el-option label="未分房" value="unassigned" />
        <el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.id" />
      </el-select>
      <el-checkbox v-model="includeDaughters" @change="onLayoutChange">显示女儿</el-checkbox>
      <FontPicker />
      <el-dropdown size="small" trigger="click" @command="doExport">
        <el-button size="small" :loading="exporting">导出<el-icon class="el-icon--right"><arrow-down /></el-icon></el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="png300">🖼️ PNG（高清 · 300DPI）</el-dropdown-item>
            <el-dropdown-item command="svg">📐 SVG（矢量图）</el-dropdown-item>
            <el-dropdown-item command="svgFont">📐 SVG（内嵌字体）</el-dropdown-item>
            <el-dropdown-item command="pdf">🖨️ PDF（打印用）</el-dropdown-item>
            <el-dropdown-item command="pagedPrint" divided>🖨️ 分页打印（A3/A4 分页 · 卷轴 · 索引页）</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <span class="hint">長子居右主幹 · 父子豎線、兄弟橫線 · 滾輪/雙指縮放 · 拖拽平移 · 點擊看詳情</span>
    </div>

    <div class="canvas-wrap" ref="wrap" @wheel.prevent="onWheel" @pointerdown.prevent="onPointerDown">
      <svg :width="svgW" :height="svgH" ref="svg" class="susi-svg" :class="{ 'tf-on': treeFontActive }" :style="treeFontStyle">
        <defs>
          <linearGradient id="bioGold" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#fdf3c8" />
            <stop offset="30%" stop-color="#e8c65c" />
            <stop offset="65%" stop-color="#d0a52c" />
            <stop offset="100%" stop-color="#a8801a" />
          </linearGradient>
          <linearGradient id="bioPaper" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stop-color="#ffffff" />
            <stop offset="55%" stop-color="#fdf1d2" />
            <stop offset="100%" stop-color="#eddfb6" />
          </linearGradient>
        </defs>
        <g :transform="`translate(${pan.x},${pan.y}) scale(${zoom})`">
          <!-- 連線（先畫線，後畫節點） -->
          <g v-for="(c, i) in connections" :key="'c' + i">
            <path :d="c.d" stroke="#8c5a3c" stroke-width="1.4" fill="none" />
          </g>
          <!-- 外纪/内纪分界线 -->
          <line v-if="waijiBoundaryY" :x1="SIDE_MARGIN" :x2="contentW - SIDE_MARGIN" :y1="waijiBoundaryY" :y2="waijiBoundaryY" stroke="#8c5a3c" stroke-width="1.2" stroke-dasharray="8,4" opacity="0.5" />
          <!-- 節點 -->
          <g v-for="n in layoutNodes" :key="n.id" class="s-node-group">
            <g class="s-node" :class="{ 'is-daughter': n.isDaughter }" @click="showMember(n)">
              <!-- 主成員框 -->
              <rect :x="n.x" :y="n.y" :width="NODE_W" :height="NODE_H" rx="4"
                :fill="n.isDaughter ? '#fdf2f2' : (n.subAnnot ? '#d9cbb6' : (n.dead ? '#ece4d6' : '#fffef8'))"
                :stroke="n.isDaughter ? '#d8a0a0' : (n.highlight ? '#d9534f' : '#8c5a3c')"
                :stroke-width="n.highlight ? 2 : 1.2"
                :stroke-dasharray="n.isDaughter ? '4,3' : undefined" />
              <text :x="n.x + NODE_W / 2" :y="n.y + 26" text-anchor="middle"
                font-family="Kaiti, STKaiti, KaiTi, serif" fill="#3a2a18" font-size="15" font-weight="600">
                {{ n.displayName }}
              </text>
              <text :x="n.x + NODE_W / 2" :y="n.y + 44" text-anchor="middle"
                font-family="Songti SC, SimSun, serif" :fill="n.isDaughter ? '#c08a8a' : '#7a6858'" font-size="11">
                {{ n.subLabel }}
              </text>
              <!-- 有传记：金色卷轴标记（旋转90°，上半露于方框上沿；点击弹出传记） -->
              <template v-if="n.has_bio">
              <g :transform="`translate(${n.x + NODE_W - 10} ${n.y}) rotate(90)`" class="bio-mark" @click.stop="openBio(n)">
              <g opacity="0.20" fill="#5a4632">
              <rect x="-6.3" y="-8.3" width="14" height="3.6" rx="1.8" />
              <rect x="-7.3" y="-4.8" width="16" height="11" rx="1.5" />
              <rect x="-6.3" y="6.2" width="14" height="3.6" rx="1.8" />
              </g>
              <rect x="-7" y="-9" width="14" height="3.6" rx="1.8" fill="url(#bioGold)" stroke="#a8801a" stroke-width="0.5" />
              <rect x="-5.8" y="-8.5" width="11.6" height="1.1" rx="0.55" fill="#fffdf2" opacity="0.85" />
              <rect x="-8" y="-5.5" width="16" height="11" rx="1.5" fill="url(#bioPaper)" stroke="#b8901f" stroke-width="0.9" />
              <path d="M-5.2 -1.6 h10.4 M-5.2 1.9 h10.4" stroke="#cdb079" stroke-width="0.7" fill="none" />
              <rect x="-7" y="5.5" width="14" height="3.6" rx="1.8" fill="url(#bioGold)" stroke="#a8801a" stroke-width="0.5" />
              <rect x="-5.8" y="6" width="11.6" height="1.1" rx="0.55" fill="#fffdf2" opacity="0.6" />
              </g>
              </template>

              <!-- 配偶框（夫右妻左，横向并排） -->
              <template v-for="(sp, si) in n.spouseNodes" :key="'sp' + sp.id">
                <rect :x="n.x + NODE_W + si * NODE_W" :y="n.y" :width="NODE_W" :height="NODE_H" rx="4"
                  fill="#f5efe0" stroke="#c4a080" stroke-width="1.2" />
                <text :x="n.x + NODE_W + si * NODE_W + NODE_W / 2" :y="n.y + 26" text-anchor="middle"
                  font-family="Songti SC, SimSun, serif" fill="#8c5a3c" font-size="14">
                  {{ sp.name }}
                </text>
                <text :x="n.x + NODE_W + si * NODE_W + NODE_W / 2" :y="n.y + 44" text-anchor="middle"
                  font-family="Songti SC, SimSun, serif" fill="#c4a080" font-size="11">
                  {{ sp.type }}
                </text>
                <!-- 有传记：金色卷轴标记（旋转90°，上半露于方框上沿；点击弹出传记） -->
                <template v-if="sp.has_bio">
                <g :transform="`translate(${n.x + NODE_W * (2 + si) - 10} ${n.y}) rotate(90)`" class="bio-mark" @click.stop="openBioSpouse(sp)">
                <g opacity="0.20" fill="#5a4632">
                <rect x="-6.3" y="-8.3" width="14" height="3.6" rx="1.8" />
                <rect x="-7.3" y="-4.8" width="16" height="11" rx="1.5" />
                <rect x="-6.3" y="6.2" width="14" height="3.6" rx="1.8" />
                </g>
                <rect x="-7" y="-9" width="14" height="3.6" rx="1.8" fill="url(#bioGold)" stroke="#a8801a" stroke-width="0.5" />
                <rect x="-5.8" y="-8.5" width="11.6" height="1.1" rx="0.55" fill="#fffdf2" opacity="0.85" />
                <rect x="-8" y="-5.5" width="16" height="11" rx="1.5" fill="url(#bioPaper)" stroke="#b8901f" stroke-width="0.9" />
                <path d="M-5.2 -1.6 h10.4 M-5.2 1.9 h10.4" stroke="#cdb079" stroke-width="0.7" fill="none" />
                <rect x="-7" y="5.5" width="14" height="3.6" rx="1.8" fill="url(#bioGold)" stroke="#a8801a" stroke-width="0.5" />
                <rect x="-5.8" y="6" width="11.6" height="1.1" rx="0.55" fill="#fffdf2" opacity="0.6" />
                </g>
                </template>
              </template>
            </g>
          </g>
          <!-- 失考 / 無嗣 標注：節點框下方連線 + 灰框（與寶塔式一致） -->
          <g v-for="n in annotNodes" :key="'mk' + n.id">
            <path :d="`M${n.x + NODE_W / 2} ${n.y + NODE_H} L${n.x + NODE_W / 2} ${n.y + NODE_H + 8}`" stroke="#8c5a3c" stroke-width="1.5" fill="none" />
            <rect :x="n.x + NODE_W / 2 - n.mkBoxW / 2" :y="n.y + NODE_H + 8" :width="n.mkBoxW" :height="n.mkBoxH" rx="6"
              fill="#d9d9d9" stroke="#8c5a3c" stroke-width="1.2" />
            <text v-for="(ln, li) in n.mkLines" :key="li" :x="n.x + NODE_W / 2"
              :y="n.y + NODE_H + 8 + n.mkBoxH / 2 + n.mkFs * 0.36 - ((n.mkLines.length - 1) * n.mkLineH) / 2 + li * n.mkLineH"
              text-anchor="middle" :font-size="n.mkFs" fill="#8c5a3c">{{ ln }}</text>
          </g>
        </g>
      </svg>
    </div>

    <el-dialog v-model="dialogVisible" :title="current.name" width="480px">
      <div v-if="current.id" class="member-card">
        <div class="row"><span class="label">世代</span><span>第 {{ current.generation }} 世</span></div>
        <div class="row" v-if="current.rank"><span class="label">排行</span><span>{{ current.rank }}</span></div>
        <div class="row" v-if="current.zi || current.hao"><span class="label">字/号</span><span>{{ current.zi || '—' }} / {{ current.hao || '—' }}</span></div>
        <div class="row" v-if="current.spouses && current.spouses.length">
          <span class="label">配偶</span>
          <span><span v-for="(s, i) in current.spouses" :key="s.id">{{ s.name || (s.surname + '氏') }}（{{ s.type }}）<span v-if="i < current.spouses.length - 1">、</span></span></span>
        </div>
        <div class="row" v-if="current.biography"><span class="label">生平</span><span>{{ current.biography }}</span></div>
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
        <el-button type="primary" @click="$router.push(`/member/${current.id}`)">查看详情</el-button>
      </template>
    </el-dialog>

    <!-- 传记弹窗（点击金色卷轴标记） -->
    <BiographyPopup ref="bioPopup" />

    <!-- 打印 / 导出：A3/A4 分页（瓦片）· 卷轴 · 索引页 -->
    <PrintExportDialog
      v-model="printDlgVisible"
      :svg-el="svg"
      :content-w="contentW"
      :content-h="contentH"
      :rows="printRows"
      :bounds="null"
      title="蘇式垂絲圖"
      :volume="printVolume"
      :theme="{ bg: '#ffffff', ink: '#3a2a18' }"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api'
import BiographyPopup from '../components/BiographyPopup.vue'
import { exportHighResPng, exportVectorSvg, exportPrintPDF } from '../utils/treeExport'
import PrintExportDialog from '../components/PrintExportDialog.vue'
import { rowsFromNodes } from '../utils/printPaging'
import { branchCtx } from '../utils/branchContext'
import FontPicker from '../components/FontPicker.vue'
import { treeFontActive, treeFontStyle } from '../utils/treeFonts'
import { ArrowDown } from '@element-plus/icons-vue'
import { memberKnown, lostContactText, NO_HEIR_TRAD } from '../utils/memberFlags'

const router = useRouter()

const members = ref([])
const branches = ref([])
const branchFilter = ref('all') // 'all' | 'unassigned' | 数字
const recordFilter = ref('all') // 'all' | '内纪' | '外纪'
const includeDaughters = ref(false)
const search = ref('')
const wrap = ref(null)
const svg = ref(null)
const zoom = ref(1)
const pan = reactive({ x: 40, y: 40 })
const svgW = ref(1400)
const svgH = ref(900)
const contentW = ref(1000)
const contentH = ref(800)
const dialogVisible = ref(false)
const current = reactive({})
const exporting = ref(false)

// ===== 分页打印（A3/A4 · 卷轴）=====
const printDlgVisible = ref(false)
const printRows = computed(() => {
  const items = layoutNodes.value || []
  const wj = items.filter(n => n.record_type === '外纪')
  const wjMin = wj.length ? Math.min(...wj.map(n => n.generation)) : 0
  return rowsFromNodes(items, {
    y: n => n.y,
    h: () => NODE_H,
    label: n => n.record_type === '外纪'
      ? `外纪${n.generation - wjMin + 1}世`
      : `第${n.generation}世${n.generation_word ? '（' + n.generation_word + '）' : ''}`
  })
})
const printVolume = computed(() => {
  const v = branchFilter.value
  if (v === 'unassigned') return '未分房'
  if (v !== 'all') {
    const b = branches.value.find(x => x.id === v)
    return b ? b.name : ''
  }
  if (branchCtx.current) {
    const b = branchCtx.branches.find(x => x.id === branchCtx.current) || branches.value.find(x => x.id === branchCtx.current)
    if (b) return b.name + (branchCtx.includeChildren ? '（含下级）' : '')
  }
  return '宗谱（全部）'
})
const waijiBoundaryY = ref(0)

// 佈局常量
const NODE_W = 92      // 節點框寬
const NODE_H = 58      // 節點框高
const V_GAP = 90       // 世代間垂直間距
const LEAF_SPACING = 200  // 每個葉子槽位橫向寬度
const TOP_MARGIN = 50
const SIDE_MARGIN = 60
const SIB_GAP = 0.7    // 兄弟子樹之間的槽位間隙
const WAIGI_GAP = 180  // 外纪/内纪分界間距（拉遠）

// 排行數字解析：支持「行一/行二…行十」「行11/行12…」「长子/次子」及纯数字
const CN_NUM = { '一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9, '十': 10 }
function rankNum(r) {
  if (r == null || r === '') return null
  const s = String(r).trim()
  const m = s.match(/(\d+)/)
  if (m) return parseInt(m[1], 10)
  if (s.includes('长') || s.includes('長')) return 1
  if (s.includes('次')) return 2
  for (const [k, v] of Object.entries(CN_NUM)) {
    if (s.includes(k)) return v
  }
  return null
}

// 出生日期排序键（年/月/日），排行缺失时按年龄（出生先后）排
function birthSortKey(m) {
  const s = String(m.birth_gregorian || '')
  const m2 = s.match(/(\d{1,4})(?:[-/年.](\d{1,2}))?(?:[-/月.](\d{1,2}))?/)
  if (!m2) return null
  return parseInt(m2[1], 10) * 10000 + parseInt(m2[2] || '0', 10) * 100 + parseInt(m2[3] || '0', 10)
}

function sortSiblings(arr) {
  arr.sort((a, b) => {
    const ra = rankNum(a.rank), rb = rankNum(b.rank)
    if (ra != null && rb != null) return ra - rb
    if (ra != null) return -1
    if (rb != null) return 1
    // 排行缺失 → 按出生日期（年龄）
    const ba = birthSortKey(a), bb = birthSortKey(b)
    if (ba != null && bb != null) return ba - bb
    if (ba != null) return -1
    if (bb != null) return 1
    return a.id - b.id
  })
}

// 配偶列表：正配/继配（最多 2 个上主图）
function spouseList(m) {
  return (m.spouses || []).filter(s => s.type === '正配' || s.type === '繼配' || s.type === '继配').slice(0, 2)
}

const visibleMembers = computed(() => {
  let list = members.value
  const v = branchFilter.value
  if (v === 'unassigned') list = list.filter(m => (m.branch_id || 0) === 0)
  else if (v !== 'all') list = list.filter(m => (m.branch_id || 0) === v)
  // 内纪/外纪筛选
  if (recordFilter.value !== 'all') list = list.filter(m => m.record_type === recordFilter.value)
  // 女兒默認不上主幹
  if (!includeDaughters.value) list = list.filter(m => m.gender !== 0)
  return list
})

/**
 * 核心：蘇式垂絲圖佈局（長子居右主幹）
 * 1. 建樹（father_id → children），兄弟按排行/年龄排序
 * 2. 長子（排行第一）居右，其豎線正對父節點；次子/三子/四子…依次往左排開
 * 3. 外纪在上、内纪在下，中间虚线分隔并拉远
 * 4. 連線：父底豎線 → 兄弟橫線 → 各子豎線
 */
const tree = computed(() => {
  const list = visibleMembers.value
  if (!list.length) return { layoutNodes: [], connections: [] }

  // 1. 建索引
  const byId = new Map()
  list.forEach(m => byId.set(m.id, {
    ...m,
    children: [],
    isDaughter: m.gender === 0,
    _leftExt: 0.5, _rightExt: 0.5, _offset: 0, _absCenter: 0
  }))

  // 2. 掛父子關係；找不到父的作為根
  const roots = []
  byId.forEach(node => {
    if (node.father_id && byId.has(node.father_id)) {
      byId.get(node.father_id).children.push(node)
    } else {
      roots.push(node)
    }
  })

  // 3. 兄弟按長幼排序
  byId.forEach(node => sortSiblings(node.children))
  roots.sort((a, b) => a.generation - b.generation || a.id - b.id)

  // 女兒（若顯示）作為末端節點：不延續子孫
  byId.forEach(node => {
    if (node.isDaughter) node.children = []
  })

  // 4. 長子居右主幹佈局：自底向上算每個節點的子樹左右延伸（相對自身中心，即長子位置）
  function layoutNode(node) {
    const kids = node.children
    if (!kids.length) {
      node._leftExt = 0.5
      node._rightExt = 0.5
      return
    }
    kids.forEach(layoutNode)
    // 長子（kids[0]）居右主幹：offset = 0，其豎線正對父節點
    kids[0]._offset = 0
    let leftEdge = kids[0]._leftExt
    const rightEdge = kids[0]._rightExt
    // 次子/三子/四子… 按排行依次往左排開
    for (let i = 1; i < kids.length; i++) {
      const k = kids[i]
      k._offset = -(leftEdge + SIB_GAP + k._rightExt)
      leftEdge = -k._offset + k._leftExt
    }
    node._leftExt = leftEdge
    node._rightExt = rightEdge
  }

  // 5. 自頂向下累加絕對中心
  function assignAbs(node, absCenter) {
    node._absCenter = absCenter
    node.children.forEach(k => assignAbs(k, absCenter + k._offset))
  }

  // 多根橫向錯開
  let cursor = 0
  roots.forEach(r => {
    layoutNode(r)
    assignAbs(r, cursor + r._leftExt)
    cursor += (r._leftExt + r._rightExt) + 2
  })

  // 6. 計算 x/y
  const minGen = Math.min(...list.map(m => m.generation))
  const layoutNodes = []
  byId.forEach(node => {
    const xCenter = node._absCenter * LEAF_SPACING + SIDE_MARGIN
    const x = xCenter - NODE_W / 2
    const y = TOP_MARGIN + (node.generation - minGen) * V_GAP
    layoutNodes.push({
      ...node,
      x,
      y,
      dead: !!node.death_gregorian,
      subAnnot: false, // 副標題為失考/無嗣標注時，加灰色底框
      displayName: node.name || ((node.surname || '') + (node.given_name || '')),
      subLabel: node.isDaughter
        ? (node.married_out ? `適${node.married_out}` : '女')
        : `第${node.generation}世`,
      spouseNodes: node.isDaughter ? [] : spouseList(node).map(s => ({
        id: s.id,
        name: s.name || (s.surname ? s.surname + '氏' : s.type),
        type: s.type,
        has_bio: !!s.has_bio
      }))
    })
  })

  // 7. 失考 / 无嗣标注（与宝塔式一致：框下方连線 + 灰框），需为标注预留纵向空间
  layoutNodes.forEach(n => {
    const noKids = !(n.children && n.children.length)
    let txt = ''
    if (n.lost_contact) txt = lostContactText(memberKnown(n), !noKids, false)
    else if (n.no_heir && noKids) txt = NO_HEIR_TRAD
    if (!txt) return
    const lines = txt.split('，')
    const maxLen = Math.max(...lines.map(l => l.length))
    n.subAnnot = true
    n.mkLines = lines
    n.mkFs = n.lost_contact ? 12 : 17
    n.mkLineH = 16
    n.mkBoxW = n.lost_contact ? Math.max(60, maxLen * 13 + 16) : 60
    n.mkBoxH = n.lost_contact ? lines.length * 16 + 10 : 30
  })
  // 8. 外纪/内纪分离：外纪在上、内纪在下，重编号外纪，中间拉远
  const hasWaiji = layoutNodes.some(n => n.record_type === '外纪')
  const hasNeiji = layoutNodes.some(n => n.record_type === '内纪')
  if (hasWaiji && hasNeiji) {
    const waijiNodes = layoutNodes.filter(n => n.record_type === '外纪')
    const waijiMinGen = Math.min(...waijiNodes.map(n => n.generation))
    waijiNodes.forEach(n => {
      const wjNum = n.generation - waijiMinGen + 1  // 最早的先祖 = 外纪1世
      n.subLabel = `外纪${wjNum}世`
    })
    const waijiBottom = Math.max(...waijiNodes.map(n => n.y + NODE_H))
    const neijiTop = Math.min(...layoutNodes.filter(n => n.record_type !== '外纪').map(n => n.y))
    const overlap = waijiBottom - neijiTop
    const totalGap = Math.max(WAIGI_GAP, overlap * 2 + WAIGI_GAP)
    waijiBoundaryY.value = waijiBottom + totalGap / 2
    layoutNodes.forEach(n => {
      if (n.record_type !== '外纪') n.y += totalGap
    })
  } else {
    waijiBoundaryY.value = 0
  }

  // 8. 整體水平居中
  if (layoutNodes.length) {
    const minX = Math.min(...layoutNodes.map(n => n.x))
    const maxX = Math.max(...layoutNodes.map(n => n.x + NODE_W * (1 + (n.spouseNodes ? n.spouseNodes.length : 0))))
    const treeW = maxX - minX
    const wrapW = wrap.value ? wrap.value.clientWidth : 0
    const canvasW = Math.max(treeW + SIDE_MARGIN * 2, wrapW, 1000)
    contentW.value = canvasW
    const offset = (canvasW - treeW) / 2 - minX
    layoutNodes.forEach(n => { n.x += offset })
  }

  // 9. 垂絲連線
  const connections = []
  byId.forEach(node => {
    const kids = node.children
    if (!kids.length) return
    const f = layoutNodes.find(n => n.id === node.id)
    if (!f) return
    const fx = f.x + NODE_W / 2
    const fyBottom = f.y + NODE_H
    const kidNodes = kids.map(k => layoutNodes.find(n => n.id === k.id)).filter(Boolean)
    if (!kidNodes.length) return

    if (kids.length === 1) {
      connections.push({ d: `M${fx} ${fyBottom} L${fx} ${kidNodes[0].y}` })
    } else {
      const minChildY = Math.min(...kidNodes.map(kn => kn.y))
      const midY = (fyBottom + minChildY) / 2
      const childXs = kidNodes.map(kn => kn.x + NODE_W / 2)
      const minX = Math.min(...childXs)
      const maxX = Math.max(...childXs)
      connections.push({ d: `M${fx} ${fyBottom} L${fx} ${midY}` })
      connections.push({ d: `M${minX} ${midY} L${maxX} ${midY}` })
      kidNodes.forEach(kn => {
        const cx = kn.x + NODE_W / 2
        connections.push({ d: `M${cx} ${midY} L${cx} ${kn.y}` })
      })
    }
  })

  const maxY = Math.max(...layoutNodes.map(n => n.y + NODE_H), 0) + 80
  contentH.value = Math.max(maxY, 700)

  return { layoutNodes, connections }
})

const layoutNodes = computed(() => tree.value.layoutNodes)
const annotNodes = computed(() => layoutNodes.value.filter(n => n.subAnnot))
const connections = computed(() => tree.value.connections)

async function load() {
  const { data } = await api.get('/members/with-spouses')
  members.value = data || []
  try {
    const { data: brData } = await api.get('/branches')
    branches.value = brData || []
  } catch (e) { /* ignore */ }
  await nextTick()
  fitToView()
}

function onLayoutChange() {
  clearLock()
  nextTick(fitToView)
}

function showMember(n) {
  Object.assign(current, n)
  dialogVisible.value = true
}

function doSearch() {
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur()
  const kw = search.value.trim()
  if (!kw) return
  const target = visibleMembers.value.find(m => {
    const name = m.name || ((m.surname || '') + (m.given_name || ''))
    return name.includes(kw) || m.zi?.includes(kw) || m.hao?.includes(kw)
  })
  if (!target) { ElMessage.warning('未找到匹配成员'); return }
  const node = layoutNodes.value.find(n => n.id === target.id)
  if (node) centerOnPoint(node.x + NODE_W / 2, node.y + NODE_H / 2)
}

let lockPoint = null
function centerOnPoint(cx, cy, targetZoom = 2) {
  const vw = wrap.value?.clientWidth || svgW.value || 1
  const vh = wrap.value?.clientHeight || svgH.value || 1
  svgW.value = vw; svgH.value = vh
  zoom.value = targetZoom
  pan.x = vw / 2 - cx * targetZoom
  pan.y = vh / 2 - cy * targetZoom
  lockPoint = { cx, cy, zoom: targetZoom }
}
function clearLock() { lockPoint = null }

function zoomIn() { clearLock(); zoom.value = Math.min(zoom.value * 1.2, 3) }
function zoomOut() { clearLock(); zoom.value = Math.max(zoom.value / 1.2, 0.1) }
function resetView() { clearLock(); fitToView() }

function fitToView() {
  if (!wrap.value) return
  const vw = wrap.value.clientWidth || 1
  const vh = wrap.value.clientHeight || 1
  svgW.value = vw
  svgH.value = vh
  const pad = 24
  const cw = contentW.value || 1
  const ch = contentH.value || 1
  const scale = Math.min((vw - pad * 2) / cw, (vh - pad * 2) / ch)
  zoom.value = Math.min(Math.max(scale, 0.1), 3)
  pan.x = (vw - cw * zoom.value) / 2
  pan.y = (vh - ch * zoom.value) / 2
}

function onWheel(e) {
  clearLock()
  const rect = wrap.value.getBoundingClientRect()
  const sx = e.clientX - rect.left
  const sy = e.clientY - rect.top
  const newZoom = Math.min(Math.max(zoom.value * (e.deltaY > 0 ? 0.9 : 1.1), 0.1), 3)
  const ratio = newZoom / zoom.value
  pan.x = sx - (sx - pan.x) * ratio
  pan.y = sy - (sy - pan.y) * ratio
  zoom.value = newZoom
}

// 多點觸控：單指拖拽、雙指縮放
let activePointers = new Map()
let dragDist = 0
let startPanX = 0, startPanY = 0
let pinchDist = 0
let pinchStartZoom = 1
let multiTouch = false
let suppressClick = false

function onPointerDown(e) {
  clearLock()
  activePointers.set(e.pointerId, { sx: e.clientX, sy: e.clientY, cx: e.clientX, cy: e.clientY })
  if (activePointers.size >= 2) multiTouch = true
  if (activePointers.size === 1) {
    suppressClick = false
    dragDist = 0
    startPanX = pan.x
    startPanY = pan.y
  }
  if (activePointers.size === 2) {
    const pts = [...activePointers.values()]
    pinchDist = Math.hypot(pts[1].sx - pts[0].sx, pts[1].sy - pts[0].sy)
    pinchStartZoom = zoom.value
  }
  e.target.setPointerCapture(e.pointerId)
}

function onPointerMove(e) {
  if (!activePointers.has(e.pointerId)) return
  const entry = activePointers.get(e.pointerId)
  entry.cx = e.clientX
  entry.cy = e.clientY
  const pts = [...activePointers.values()]
  if (activePointers.size >= 2) {
    const d = Math.hypot(pts[1].cx - pts[0].cx, pts[1].cy - pts[0].cy)
    const midX = (pts[0].cx + pts[1].cx) / 2
    const midY = (pts[0].cy + pts[1].cy) / 2
    const scale = pinchDist > 0 ? d / pinchDist : 1
    const newZoom = Math.min(Math.max(pinchStartZoom * scale, 0.1), 3)
    const ratio = newZoom / zoom.value
    pan.x = midX - (midX - pan.x) * ratio
    pan.y = midY - (midY - pan.y) * ratio
    zoom.value = newZoom
    dragDist = 999
  } else if (activePointers.size === 1) {
    const p = pts[0]
    const dx = p.cx - p.sx
    const dy = p.cy - p.sy
    dragDist = Math.sqrt(dx * dx + dy * dy)
    pan.x = startPanX + dx
    pan.y = startPanY + dy
  }
}

function onPointerUp(e) {
  const wasSize = activePointers.size
  const dragged = dragDist >= 3
  activePointers.delete(e.pointerId)
  if (dragged) suppressClick = true
  if (wasSize === 1 && dragDist < 3 && !multiTouch) {
    const el = document.elementFromPoint(e.clientX, e.clientY)
    if (el) el.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: e.clientX, clientY: e.clientY }))
  }
  if (activePointers.size === 0) {
    dragDist = 0
    multiTouch = false
  } else if (activePointers.size === 1) {
    const p = [...activePointers.values()][0]
    dragDist = 0
    startPanX = pan.x
    startPanY = pan.y
    const key = [...activePointers.keys()][0]
    activePointers.set(key, { sx: p.cx, sy: p.cy, cx: p.cx, cy: p.cy })
  }
}

function onCaptureClick(e) {
  if (suppressClick) {
    e.stopPropagation()
    e.preventDefault()
    suppressClick = false
  }
}

// 实际落盘的 DPI = 导出像素宽 ÷ 内容宽 × 96（大树会被 MAX_CANVAS_PX=14000 上限自动降倍率）
function pngActualDpi(r) {
  return Math.round(r.width / contentW.value * 96)
}

// 导出分档：PNG 高清、矢量 SVG（可选内嵌字体）、打印 PDF
async function doExport(cmd) {
  if (exporting.value) return
  if (cmd === 'pagedPrint') { printDlgVisible.value = true; return }
  exporting.value = true
  try {
    if (cmd === 'png300') {
      const r = await exportHighResPng(svg.value, contentW.value, contentH.value, '蘇式垂絲圖-300dpi.png', '#f8f4e8', null, undefined, { treeFont: true })
      const dpi = pngActualDpi(r)
      ElMessage.success(`已导出 PNG 高清 300DPI（${r.width}×${r.height}px，实际 ≈ ${dpi} DPI${dpi < 294 ? '，受 14000px 上限限制已自动降倍率' : ''}）`)
    } else if (cmd === 'svg' || cmd === 'svgFont') {
      const r = await exportVectorSvg(svg.value, contentW.value, contentH.value,
        `蘇式垂絲圖${cmd === 'svgFont' ? '-内嵌字体' : ''}.svg`, null, { withFont: cmd === 'svgFont', treeFont: true })
      const kb = Math.max(1, Math.round(r.size / 1024))
      if (cmd === 'svgFont') {
        const f = r.font || {}
        ElMessage.success(`已导出 SVG（内嵌字体 · 子集 ${f.chars || 0} 字 · ${kb}KB${f.missing ? ' · ' + f.missing + ' 字缺字形' : ''}）`)
      } else {
        ElMessage.success(`已导出 SVG（矢量图 · ${kb}KB）`)
      }
    } else if (cmd === 'pdf') {
      await exportPrintPDF(svg.value, contentW.value, contentH.value, '蘇式垂絲圖.pdf', '#f8f4e8', null, { treeFont: true })
      ElMessage.success('已打开打印视图（在打印对话框选择「另存为 PDF」）')
    }
  } catch (e) {
    if (e && (e.code === 'FONT_NONE' || e.code === 'FONT_UNSUPPORTED')) ElMessage.warning(e.message)
    else ElMessage.error('导出失败：' + (e?.message || e))
  } finally {
    exporting.value = false
  }
}

function updateSvgSize() {
  if (lockPoint) centerOnPoint(lockPoint.cx, lockPoint.cy, lockPoint.zoom)
  else fitToView()
}

onMounted(() => {
  load()
  const el = wrap.value
  el.addEventListener('pointermove', onPointerMove)
  el.addEventListener('pointerup', onPointerUp)
  el.addEventListener('pointerleave', onPointerUp)
  el.addEventListener('click', onCaptureClick, true)
  window.addEventListener('resize', updateSvgSize)
  updateSvgSize()
})

// 传记弹窗：点击世系图上的金色卷轴标记
const bioPopup = ref(null)
function openBio(n) {
  if (bioPopup.value) bioPopup.value.open({ member_id: n.id, name: n.displayName || n.name || '' })
}
function openBioSpouse(s) {
  if (bioPopup.value) bioPopup.value.open({ spouse_id: s.id, name: s.name || '' })
}
</script>

<style scoped>
/* 树图字体选择：作用到本视图 SVG 里的文字（CSS 规则优先于 <text> 的 font-family 表现属性） */
svg.tf-on text, svg.tf-on tspan { font-family: var(--tree-font); }
.susi-tree { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.title { font-size: 18px; margin-right: 16px; font-family: "Songti SC", "SimSun", serif; color: #3a2a18; }
.hint { margin-left: auto; color: #8c5a3c; font-size: 12px; }
@media (max-width: 767px) { .hint { display: none; } }
.canvas-wrap { flex: 1; overflow: hidden; border: 1px solid var(--border); border-radius: 4px; cursor: grab; touch-action: none; user-select: none; background: #f8f4e8; }
.susi-svg { display: block; }
.s-node { cursor: pointer; }
.s-node:hover rect { filter: brightness(0.97); }
.member-card .row { display: flex; margin-bottom: 8px; }
.member-card .label { width: 70px; color: #8c5a3c; flex-shrink: 0; }
.bio-mark { cursor: pointer; }
.bio-mark:hover rect { filter: brightness(1.08); }
</style>
