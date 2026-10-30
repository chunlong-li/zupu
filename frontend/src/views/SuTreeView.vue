<template>
  <div class="su-tree">
    <div class="toolbar">
      <span class="serif title">蘇式垂吊圖</span>
      <el-input v-model="search" placeholder="搜索姓名/字号" clearable style="width: 200px" @keyup.enter="doSearch" />
      <el-button type="primary" @click="doSearch">定位</el-button>
      <el-button @click="zoomIn">放大</el-button>
      <el-button @click="zoomOut">缩小</el-button>
      <el-button @click="resetView">重置</el-button>
      <el-select v-model="branchFilter" size="small" style="width:120px" @change="onBranchChange">
        <el-option label="全部" value="all" />
        <el-option label="未分房" value="unassigned" />
        <el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.id" />
      </el-select>
      <el-select v-model="exportMode" size="small" style="width:120px">
        <el-option label="导出全部" value="all" />
        <el-option label="仅内纪" value="neiji" />
        <el-option label="仅外纪" value="waiji" />
      </el-select>
      <FontPicker />
      <el-dropdown size="small" trigger="click" @command="doExport">
        <el-button size="small" :loading="suExporting">导出<el-icon class="el-icon--right"><arrow-down /></el-icon></el-button>
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
      <span class="hint">垂珠吊线 · 父子竖线、兄弟横线 · 滚轮/双指缩放 · 拖拽平移 · 点击看详情</span>
    </div>

    <div class="canvas-wrap" ref="wrap" @wheel.prevent="onWheel" @pointerdown.prevent="onPointerDown">
      <svg :width="svgW" :height="svgH" ref="svg" class="su-svg" :class="{ 'tf-on': treeFontActive }" :style="treeFontStyle">
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
          <!-- 连线（先画线，后画节点，保证线在框下面） -->
          <g v-for="(c, i) in connections" :key="'c' + i">
            <path :d="c.d" stroke="#8c5a3c" stroke-width="1.5" fill="none" />
          </g>
          <!-- 外纪/内纪分界线 -->
          <line v-if="waijiBoundaryY" :x1="SIDE_MARGIN" :x2="contentW - SIDE_MARGIN" :y1="waijiBoundaryY" :y2="waijiBoundaryY" stroke="#8c5a3c" stroke-width="1.2" stroke-dasharray="8,4" opacity="0.5" />
          <!-- 节点 -->
          <g v-for="n in layoutNodes" :key="n.id" class="s-node-group">
            <g class="s-node" @click="showMember(n)">
              <!-- 主成员框 -->
              <rect :x="n.x" :y="n.y" :width="NODE_W" :height="NODE_H" rx="4"
                :fill="n.subAnnot ? '#d9cbb6' : (n.dead ? '#ece4d6' : '#fffef8')"
                :stroke="n.highlight ? '#d9534f' : '#8c5a3c'" :stroke-width="n.highlight ? 2 : 1.2" />
              <text :x="n.x + NODE_W / 2" :y="n.y + 26" text-anchor="middle"
                font-family="Kaiti, STKaiti, KaiTi, serif" fill="#3a2a18" font-size="15" font-weight="600">
                {{ n.displayName }}
              </text>
              <text :x="n.x + NODE_W / 2" :y="n.y + 44" text-anchor="middle"
                font-family="Songti SC, SimSun, serif" fill="#7a6858" font-size="11">
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
                  font-family="Songti SC, SimSun, serif" fill="#8c5a3c" font-size="15">
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
          <!-- 失考 / 无嗣 标注：成员框下方连线 + 灰框（与宝塔式一致） -->
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
        <div class="dialog-header" v-if="current.portrait">
          <el-image :src="current.portrait" style="width:80px;height:80px;border-radius:8px" fit="cover" />
        </div>
        <div class="row"><span class="label">世代</span><span>第 {{ current.generation }} 世</span></div>
        <div class="row"><span class="label">字辈</span><span>{{ current.generation_word || '—' }}</span></div>
        <div class="row"><span class="label">字/号</span><span>{{ current.zi || '—' }} / {{ current.hao || '—' }}</span></div>
        <div class="row"><span class="label">宗法</span><span>{{ current.clan_type || '本生' }}</span></div>
        <div class="row" v-if="current.spouses && current.spouses.length">
          <span class="label">配偶</span>
          <span><span v-for="(s, i) in current.spouses" :key="s.id">{{ s.name }}（{{ s.type }}）<span v-if="i < current.spouses.length - 1">、</span></span></span>
        </div>
        <div class="row"><span class="label">生卒</span><span>{{ formatLifeDates(current) }}</span></div>
        <div class="row" v-if="calcAge(current)"><span class="label">年龄</span><span>{{ calcAge(current) }}</span></div>
        <div class="row" v-if="current.biography"><span class="label">生平</span><span>{{ current.biography }}</span></div>
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
        <el-button type="primary" @click="$router.push(`/member/${current.id}`)">查看详情</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="branchPromptVisible" title="房支后代未归入" width="480px" :close-on-click-modal="false" :show-close="false">
      <div class="bp-body">
        <p>房支「{{ pendingBranchName }}」下有 <b>{{ unassignedDesc.length }}</b> 名后代尚未归入房支。</p>
        <p class="bp-sub">是否在图中显示这些后代？</p>
      </div>
      <template #footer>
        <el-button @click="branchPromptChoose('no')">否</el-button>
        <el-button @click="branchPromptChoose('show')">是，不加入房支</el-button>
        <el-button v-if="canEdit()" type="primary" @click="branchPromptChoose('join')">是，加入房支</el-button>
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
      :bounds="printBounds"
      title="蘇式垂吊圖"
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
import { canEdit } from '../utils/auth'
import { findUnassignedDescendants } from '../utils/branchTree'
import { exportHighResPng, exportVectorSvg, exportPrintPDF } from '../utils/treeExport'
import PrintExportDialog from '../components/PrintExportDialog.vue'
import { rowsFromNodes } from '../utils/printPaging'
import { branchCtx } from '../utils/branchContext'
import FontPicker from '../components/FontPicker.vue'
import { treeFontActive, treeFontStyle } from '../utils/treeFonts'
import { ArrowDown } from '@element-plus/icons-vue'
import { formatLifeDates } from '../utils/dateFormat'
import { memberKnown, lostContactText, NO_HEIR_TRAD } from '../utils/memberFlags'

const router = useRouter()

const members = ref([])
const branches = ref([])
const branchFilter = ref('all') // 'all' = 全部，'unassigned' = 未分房，数字 = 房支 id
const extraShowIds = ref([]) // 选择「不加入但显示」时额外显示的未分房后代 id
const visibleMembers = computed(() => {
  const v = branchFilter.value
  if (v === 'all') return members.value
  if (v === 'unassigned') return members.value.filter(m => (m.branch_id || 0) === 0)
  const extra = new Set(extraShowIds.value || [])
  return members.value.filter(m => (m.branch_id || 0) === v || extra.has(m.id))
})
const branchPromptVisible = ref(false)
const unassignedDesc = ref([])
const pendingBranchId = ref(null)
const pendingBranchName = ref('')
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
const waijiBoundaryY = ref(0)

// 布局常量
const NODE_W = 90      // 单个节点框宽
const NODE_H = 58      // 单个节点框高
const V_GAP = 90       // 世代间垂直间距（含节点高）
const LEAF_SPACING = 200  // 每个叶子槽位横向宽度（容纳夫+妻+间隙）
const TOP_MARGIN = 50
const SIDE_MARGIN = 60
const WAIGI_GAP = 80   // 外纪/内纪分界间距

/**
 * 核心：构建垂吊树并做 tidy 布局
 * 返回：
 *  - layoutNodes: 带 x/y 坐标的节点
 *  - connections: 垂珠连线路径
 */
const tree = computed(() => {
  const list = visibleMembers.value
  if (!list.length) return { layoutNodes: [], connections: [] }

  // 1. 建索引
  const byId = new Map()
  list.forEach(m => byId.set(m.id, { ...m, children: [] }))

  // 2. 挂父子关系；找不到父的作为根（含断代/始祖）
  const roots = []
  byId.forEach(node => {
    if (node.father_id && byId.has(node.father_id)) {
      byId.get(node.father_id).children.push(node)
    } else {
      roots.push(node)
    }
  })

  // 3. 兄弟按长幼排序（有排行按排行数字，否则按 id）
  function sortSiblings(arr) {
    const rankNum = r => {
      if (!r || !r.rank) return null
      const m = String(r.rank).match(/(\d+)/)
      return m ? parseInt(m[1], 10) : null
    }
    arr.sort((a, b) => {
      const ra = rankNum(a), rb = rankNum(b)
      if (ra != null && rb != null) return ra - rb
      if (ra != null) return -1
      if (rb != null) return 1
      return a.id - b.id
    })
  }
  byId.forEach(node => sortSiblings(node.children))

  // 多根时按世代再按 id 排序
  roots.sort((a, b) => a.generation - b.generation || a.id - b.id)

  // 4. tidy 布局：父节点 = 首末子节点中心的中点（而非叶子区间中心）
  //    每个节点记录 center（横向中心，单位=叶子槽位）、min/max（子树槽位范围）
  let leafCounter = 0

  function layout(node) {
    const kids = node.children
    if (!kids.length) {
      node.center = leafCounter
      node.min = leafCounter
      node.max = leafCounter
      leafCounter++
    } else {
      kids.forEach(layout)
      // 父对中子：取最左子与最右子的中心中点
      node.center = (kids[0].center + kids[kids.length - 1].center) / 2
      node.min = Math.min(...kids.map(k => k.min))
      node.max = Math.max(...kids.map(k => k.max))
    }
  }

  function shiftSubtree(node, shift) {
    node.center += shift
    node.min += shift
    node.max += shift
    node.children.forEach(c => shiftSubtree(c, shift))
  }

  // 每个根之间横向错开：为多根分别预留叶子区间
  let base = 0
  roots.forEach(r => {
    layout(r)
    // 将该根子树整体平移到 base 之后
    const w = r.max - r.min
    const shift = base - r.min
    shiftSubtree(r, shift)
    base += (w + 1) + 1 // 多一个槽位作为根之间的间隔
  })

  // 4.5 同世代横向解重叠：从左到右扫描，若单元重叠则整体右移该子树。
  // 解重叠（平移子树、不动祖先）与父居中重算（父移到子女中点、可能撞同世代旁支）相互影响，
  // 故交替迭代到收敛。
  const minGenForGap = Math.min(...list.map(m => m.generation))
  const maxGenForGap = Math.max(...list.map(m => m.generation))
  const unitWOf = n => NODE_W * (1 + spouseList(n).length)
  const MIN_GAP = 14
  const genListS = (g) => [...byId.values()].filter(n => n.generation === g).sort((a, b) => a.center - b.center)
  const resolveOverlapS = () => {
    let moved = false
    for (let g = minGenForGap; g <= maxGenForGap; g++) {
      let rightEdge = -Infinity
      genListS(g).forEach(n => {
        const uw = unitWOf(n)
        const left = n.center * LEAF_SPACING - uw / 2
        const right = n.center * LEAF_SPACING + uw / 2
        if (left < rightEdge + MIN_GAP) {
          const d = (rightEdge + MIN_GAP - left) / LEAF_SPACING
          const moveSub = (nd, delta) => {
            nd.center += delta
            nd.children.forEach(c => moveSub(c, delta))
          }
          moveSub(n, d)
          moved = true
          rightEdge = right + (d * LEAF_SPACING)
        } else {
          rightEdge = Math.max(rightEdge, right)
        }
      })
    }
    return moved
  }
  // 4.6 父居中：父 center = 首末子女 center 中点（自底向上）
  const recenterParentsS = () => {
    let changed = false
    for (let g = maxGenForGap; g >= minGenForGap; g--) {
      genListS(g).forEach(n => {
        if (!n.children.length) return
        const cs = n.children.map(c => c.center)
        const nc = (Math.min(...cs) + Math.max(...cs)) / 2
        if (Math.abs(nc - n.center) > 1e-6) { n.center = nc; changed = true }
      })
    }
    return changed
  }
  for (let iter = 0; iter < 20; iter++) {
    const a = resolveOverlapS()
    const b = recenterParentsS()
    if (!a && !b) break
  }

  // 7. 计算 x/y
  const minGen = Math.min(...list.map(m => m.generation))
  const layoutNodes = []
  byId.forEach(node => {
    const center = node.center
    const x = center * LEAF_SPACING + SIDE_MARGIN
    const y = TOP_MARGIN + (node.generation - minGen) * V_GAP
    // 家庭单元宽度（含配偶）用于居中
    const spCount = spouseList(node).length
    const unitW = NODE_W * (1 + spCount)
    const nodeX = x - unitW / 2
    layoutNodes.push({
      ...node,
      x: nodeX,
      y,
      unitW,
      dead: !!node.death_gregorian,
      subAnnot: false, // 失考/無嗣标注：框下方连線 + 灰框
      displayName: node.name || (node.surname || '') + (node.given_name || ''),
      subLabel: node.zi ? `字 ${node.zi}` : (node.generation_word ? `字辈 ${node.generation_word}` : `第${node.generation}世`),
      spouseNodes: spouseList(node).map((s, si) => ({
        id: s.id,
        name: s.name || (s.surname ? s.surname + '氏' : s.type),
        type: s.type,
        has_bio: !!s.has_bio
      }))
    })
  })

  // 7.1 失考 / 无嗣标注（与宝塔式一致：框下方连線 + 灰框），需为标注预留纵向空间
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
  // 7.5 整体居中：先确定画布宽，再让树在画布中水平居中
  if (layoutNodes.length) {
    const minX = Math.min(...layoutNodes.map(n => n.x))
    const maxX = Math.max(...layoutNodes.map(n => n.x + n.unitW))
    const treeW = maxX - minX
    // 画布宽：至少容纳树 + 两侧留白，且不小于容器宽/最小值
    const wrapW = wrap.value ? wrap.value.clientWidth : 0
    const canvasW = Math.max(treeW + SIDE_MARGIN * 2, wrapW, 1000)
    contentW.value = canvasW
    // 树水平居中：左侧留白 = (canvasW - treeW) / 2
    const offset = (canvasW - treeW) / 2 - minX
    layoutNodes.forEach(n => { n.x += offset })
  }

  // 7.6 外纪/内纪分界：外纪在上，内纪在下，中间虚线分隔
  const hasWaiji = layoutNodes.some(n => n.record_type === '外纪')
  const hasNeiji = layoutNodes.some(n => n.record_type === '内纪')
  if (hasWaiji && hasNeiji) {
    // 外纪世代编号
    const waijiNodes = layoutNodes.filter(n => n.record_type === '外纪')
    const waijiMinGen = Math.min(...waijiNodes.map(n => n.generation))
    waijiNodes.forEach(n => {
      const wjNum = n.generation - waijiMinGen + 1
      n.subLabel = `外纪${wjNum}世`
    })

    const waijiBottom = Math.max(...waijiNodes.map(n => n.y + NODE_H))
    const neijiTop = Math.min(...layoutNodes.filter(n => n.record_type !== '外纪').map(n => n.y))
    // 外纪延伸到内纪世代区域时，动态增加间距
    const overlap = waijiBottom - neijiTop
    const totalGap = Math.max(WAIGI_GAP, overlap * 2 + WAIGI_GAP)
    const boundaryY = waijiBottom + totalGap / 2
    waijiBoundaryY.value = boundaryY

    layoutNodes.forEach(n => {
      if (n.record_type !== '外纪') {
        n.y += totalGap
      }
    })
  } else {
    waijiBoundaryY.value = 0
  }

  // 6. 垂珠连线：父底 → 竖线 → 横线（串兄弟）→ 各子竖线
  const connections = []
  byId.forEach(node => {
    const kids = node.children
    if (!kids.length) return
    const f = layoutNodes.find(n => n.id === node.id)
    if (!f) return

    const fx = f.x + f.unitW / 2          // 父单元中心
    const fyBottom = f.y + NODE_H          // 父框底

    const kidNodes = kids.map(k => layoutNodes.find(n => n.id === k.id)).filter(Boolean)
    if (!kidNodes.length) return

    if (kids.length === 1) {
      // 独子：直接用子节点实际 Y（兼容外纪→内纪跨区移位）
      connections.push({ d: `M${fx} ${fyBottom} L${fx} ${kidNodes[0].y}` })
    } else {
      // 多子：竖线到中线，横线串起，再各自下垂
      const minChildY = Math.min(...kidNodes.map(kn => kn.y))
      const midY = (fyBottom + minChildY) / 2
      const childXs = kidNodes.map(kn => kn.x + kn.unitW / 2)
      const minX = Math.min(...childXs)
      const maxX = Math.max(...childXs)
      connections.push({ d: `M${fx} ${fyBottom} L${fx} ${midY}` })
      connections.push({ d: `M${minX} ${midY} L${maxX} ${midY}` })
      kidNodes.forEach(kn => {
        const cx = kn.x + kn.unitW / 2
        connections.push({ d: `M${cx} ${midY} L${cx} ${kn.y}` })
      })
    }
  })

  // 8. 计算 SVG 画布高度（宽度已在居中时计算）
  const maxY = Math.max(...layoutNodes.map(n => n.y + NODE_H), 0) + 80
  contentH.value = Math.max(maxY, 700)
  return { layoutNodes, connections }
})

const layoutNodes = computed(() => tree.value.layoutNodes)
const annotNodes = computed(() => layoutNodes.value.filter(n => n.subAnnot))
const connections = computed(() => tree.value.connections)

function spouseList(m) {
  const sps = (m.spouses || []).filter(s => s.type === '正配' || s.type === '继配')
  // 苏式吊图主图显示正配/继配各一个；其余（侧室等）仅在详情弹窗展示
  return sps.slice(0, 2)
}

async function load() {
  const { data } = await api.get('/members/with-spouses')
  members.value = data
  try {
    const { data: brData } = await api.get('/branches')
    branches.value = brData || []
  } catch (e) { /* ignore */ }
  await nextTick()
  fitToView()
}

function onBranchChange() {
  const v = branchFilter.value
  if (v === 'all' || v === 'unassigned') {
    extraShowIds.value = []
    applyBranchView()
    return
  }
  const desc = findUnassignedDescendants(members.value, v)
  if (!desc.length) {
    extraShowIds.value = []
    applyBranchView()
    return
  }
  unassignedDesc.value = desc
  pendingBranchId.value = v
  pendingBranchName.value = (branches.value.find(b => b.id === v) || {}).name || '该房支'
  branchPromptVisible.value = true
}

function branchPromptChoose(action) {
  branchPromptVisible.value = false
  if (action === 'join') {
    joinUnassigned()
  } else if (action === 'show') {
    extraShowIds.value = unassignedDesc.value.map(m => m.id)
    applyBranchView()
  } else {
    extraShowIds.value = []
    applyBranchView()
  }
}

function applyBranchView() {
  clearLock()
  nextTick(fitToView)
}

async function joinUnassigned() {
  const bid = pendingBranchId.value
  const ids = unassignedDesc.value.map(m => m.id)
  try {
    await api.post('/branches/set-members', { branch_id: bid, member_ids: ids, replace: false })
    ElMessage.success(`已将 ${ids.length} 名后代归入房支`)
    const { data } = await api.get('/members/with-spouses')
    members.value = data || []
  } catch (e) {
    ElMessage.error('归入房支失败：' + (e.response?.data?.error || e.message))
  }
  extraShowIds.value = []
  applyBranchView()
}

function showMember(n) {
  Object.assign(current, n)
  dialogVisible.value = true
}

// 计算年龄（周岁/虚岁）；无出生日期返回空
function calcAge(o) {
  const b = (o?.birth_gregorian || '').trim()
  if (!b) return ''
  const m = b.match(/^(\d{4})-(\d{1,2})-(\d{1,2})/)
  if (!m) return ''
  const by = parseInt(m[1]), bm = parseInt(m[2]), bd = parseInt(m[3])
  let cy, cm, cd
  const d = (o?.death_gregorian || '').trim()
  const dm = d.match(/^(\d{4})-(\d{1,2})-(\d{1,2})/)
  if (dm) {
    cy = parseInt(dm[1]); cm = parseInt(dm[2]); cd = parseInt(dm[3])
  } else {
    const now = new Date()
    cy = now.getFullYear(); cm = now.getMonth() + 1; cd = now.getDate()
  }
  let zhousui = cy - by
  if (cm < bm || (cm === bm && cd < bd)) zhousui--
  const xusui = cy - by + 1
  if (zhousui < 0) return ''
  return `周岁 ${zhousui} · 虚岁 ${xusui}`
}

function doSearch() {
  // 手机端：先失焦收起键盘，避免键盘压缩可视区 + 收起时 resize 把定位重置
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur()
  const kw = search.value.trim()
  if (!kw) return
  const target = visibleMembers.value.find(m => {
    const name = m.name || (m.surname || '') + (m.given_name || '')
    return name.includes(kw) || m.zi?.includes(kw) || m.hao?.includes(kw)
  })
  if (!target) { ElMessage.warning('未找到匹配成员'); return }
  const node = layoutNodes.value.find(n => n.id === target.id)
  if (node) {
    // 定位：放大并把对应人物居中（不再直接打开资料卡）。
    // 苏氏 node.x 是单元左边缘，中心要加 unitW/2；zoom 与 pan 同步计算，避免要点两下。
    centerOnPoint(node.x + (node.unitW || NODE_W) / 2, node.y + NODE_H / 2)
  }
}

// 把内容坐标点 (cx,cy) 居中到可视区并放大；zoom 与 pan 同步，保证一次到位
// 把内容坐标点 (cx,cy) 居中到可视区并放大；zoom 与 pan 同步，保证一次到位。
// 记录锁定点：之后 resize（手机键盘收起/转屏）对同一目标重新居中，不被 fitToView 重置。
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

// 将整棵垂吊图缩放并居中到可视区内（解决画布边界截断问题）
function fitToView() {
  if (!wrap.value) return
  const vw = wrap.value.clientWidth || 1
  const vh = wrap.value.clientHeight || 1
  // SVG 视口 = 可视区尺寸，避免内容被 SVG 自身边界裁切（与 TreeView 同源 bug）
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
  // 以鼠标悬停位置为焦点缩放：保持鼠标下的内容点不动
  const rect = wrap.value.getBoundingClientRect()
  const sx = e.clientX - rect.left
  const sy = e.clientY - rect.top
  const newZoom = Math.min(Math.max(zoom.value * (e.deltaY > 0 ? 0.9 : 1.1), 0.1), 3)
  const ratio = newZoom / zoom.value
  pan.x = sx - (sx - pan.x) * ratio
  pan.y = sy - (sy - pan.y) * ratio
  zoom.value = newZoom
}

// 多点触控：单指拖拽平移，双指缩放（pinch）
let activePointers = new Map()   // pointerId -> { sx, sy, cx, cy }
let dragDist = 0
let startPanX = 0, startPanY = 0
let pinchDist = 0
let pinchStartZoom = 1
let multiTouch = false   // 本次手势是否出现过双指（禁止双指缩放后的误点击）
let suppressClick = false   // 本次手势是否发生过拖动（禁止拖动后的误点击）

function onPointerDown(e) {
  clearLock() // 用户开始拖拽/双指，视为手动浏览，解除定位锁定
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
    // 双指缩放：以双指中点为中心
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
    // 单指拖拽平移
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
  const dragged = dragDist >= 3   // 拖动后抑制紧随的原生 click
  activePointers.delete(e.pointerId)
  if (dragged) suppressClick = true
  if (wasSize === 1 && dragDist < 3 && !multiTouch) {
    // 轻微移动视为点击：派发 click 事件（支持节点点击看详情）
    const el = document.elementFromPoint(e.clientX, e.clientY)
    if (el) {
      el.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: e.clientX, clientY: e.clientY }))
    }
  }
  if (activePointers.size === 0) {
    dragDist = 0
    multiTouch = false
  } else if (activePointers.size === 1) {
    // 双指→单指：重置拖拽起点
    const p = [...activePointers.values()][0]
    dragDist = 0
    startPanX = pan.x
    startPanY = pan.y
    const key = [...activePointers.keys()][0]
    activePointers.set(key, { sx: p.cx, sy: p.cy, cx: p.cx, cy: p.cy })
  }
}

const suExporting = ref(false)

// ===== 分页打印（A3/A4 · 卷轴）：只提供世代行与房支/卷册，切页算法在 utils/printPaging.js =====
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
const printBounds = computed(() => {
  const mode = exportMode.value
  if (mode !== 'neiji' && mode !== 'waiji') return null
  const filtered = layoutNodes.value.filter(n => mode === 'neiji' ? n.record_type !== '外纪' : n.record_type === '外纪')
  if (!filtered.length) return null
  const minY = Math.min(...filtered.map(n => n.y))
  const maxY = Math.max(...filtered.map(n => n.y + NODE_H))
  return { x: 0, y: minY - 20, w: contentW.value, h: maxY - minY + 40 }
})
const exportMode = ref('all') // 'all' | 'neiji' | 'waiji'

// 苏氏内外纪分离导出裁剪区（内纪/外纪只做纵向裁剪）
function suExportCrop() {
  const mode = exportMode.value
  if (mode !== 'neiji' && mode !== 'waiji') return null
  const filtered = layoutNodes.value.filter(n => mode === 'neiji' ? n.record_type !== '外纪' : n.record_type === '外纪')
  if (filtered.length === 0) { ElMessage.warning('没有可导出的内容'); return 'empty' }
  const minY = Math.min(...filtered.map(n => n.y))
  const maxY = Math.max(...filtered.map(n => n.y + NODE_H))
  return { x: 0, y: minY - 20, w: contentW.value, h: maxY - minY + 40 }
}
const suNameSuffix = () => ({
  all: '蘇式垂吊圖', neiji: '蘇式垂吊圖-內紀', waiji: '蘇式垂吊圖-外紀'
}[exportMode.value] || '蘇式垂吊圖')

// 实际落盘的 DPI = 导出像素宽 ÷ 裁剪后内容宽 × 96（大树会被 MAX_CANVAS_PX=14000 上限自动降倍率）
function pngActualDpi(r, crop) {
  const cw = (crop && crop.w) ? crop.w : contentW.value
  return Math.round(r.width / cw * 96)
}

// 导出分档：PNG 高清、矢量 SVG（可选内嵌字体）、打印 PDF
async function doExport(cmd) {
  if (suExporting.value) return
  if (cmd === 'pagedPrint') { printDlgVisible.value = true; return }
  const crop = suExportCrop()
  if (crop === 'empty') return
  suExporting.value = true
  const name = suNameSuffix()
  try {
    if (cmd === 'png300') {
      const r = await exportHighResPng(svg.value, contentW.value, contentH.value, `${name}-300dpi.png`, '#f8f4e8', crop, undefined, { treeFont: true })
      const dpi = pngActualDpi(r, crop)
      ElMessage.success(`已导出 PNG 高清 300DPI（${r.width}×${r.height}px，实际 ≈ ${dpi} DPI${dpi < 294 ? '，受 14000px 上限限制已自动降倍率' : ''}）`)
    } else if (cmd === 'svg' || cmd === 'svgFont') {
      const r = await exportVectorSvg(svg.value, contentW.value, contentH.value,
        `${name}${cmd === 'svgFont' ? '-内嵌字体' : ''}.svg`, crop, { withFont: cmd === 'svgFont', treeFont: true })
      const kb = Math.max(1, Math.round(r.size / 1024))
      if (cmd === 'svgFont') {
        const f = r.font || {}
        ElMessage.success(`已导出 SVG（内嵌字体 · 子集 ${f.chars || 0} 字 · ${kb}KB${f.missing ? ' · ' + f.missing + ' 字缺字形' : ''}）`)
      } else {
        ElMessage.success(`已导出 SVG（矢量图 · ${kb}KB）`)
      }
    } else if (cmd === 'pdf') {
      await exportPrintPDF(svg.value, contentW.value, contentH.value, `${name}.pdf`, '#f8f4e8', crop, { treeFont: true })
      ElMessage.success('已打开打印视图（在打印对话框选择「另存为 PDF」）')
    }
  } catch (e) {
    if (e && (e.code === 'FONT_NONE' || e.code === 'FONT_UNSUPPORTED')) ElMessage.warning(e.message)
    else ElMessage.error('导出失败：' + (e?.message || e))
  } finally {
    suExporting.value = false
  }
}

function updateSvgSize() {
  // 定位锁定中（手机键盘收起/转屏触发 resize）：按新视口对同一目标重新居中；否则自适应整树
  if (lockPoint) {
    centerOnPoint(lockPoint.cx, lockPoint.cy, lockPoint.zoom)
  } else {
    fitToView()
  }
}

// 捕获阶段拦截 click：拖动后吞掉原生 click，避免误弹资料卡
function onCaptureClick(e) {
  if (suppressClick) {
    e.stopPropagation()
    e.preventDefault()
    suppressClick = false
  }
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
.su-tree { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.title { font-size: 18px; margin-right: 16px; font-family: "Songti SC", "SimSun", serif; color: #3a2a18; }
.hint { margin-left: auto; color: #8c5a3c; font-size: 12px; }
@media (max-width: 767px) { .hint { display: none; } }
.canvas-wrap { flex: 1; overflow: hidden; border: 1px solid var(--border); border-radius: 4px; cursor: grab; touch-action: none; user-select: none; background: #f8f4e8; }
.su-svg { display: block; }
.s-node { cursor: pointer; }
.s-node:hover rect { filter: brightness(0.97); }
.member-card .row { display: flex; margin-bottom: 8px; }
.member-card .label { width: 70px; color: #8c5a3c; flex-shrink: 0; }
.dialog-header { text-align: center; margin-bottom: 12px; }
.bp-body { font-size: 14px; line-height: 1.7; }
.bp-body .bp-sub { color: #666; margin: 4px 0 0; }
.bio-mark { cursor: pointer; }
.bio-mark:hover rect { filter: brightness(1.08); }
</style>
