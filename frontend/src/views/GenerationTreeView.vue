<template>
  <!-- 世代分层图：一行一辈（最上为第一世），左侧世代竖排标签；
       节点＝圆圈（圈内写身份/排行：男＝排行、族内女＝女、配偶＝配），姓名竖排在圈正下方，配偶并排于本人右侧；
       父子连线为直角折线；滚轮缩放、拖拽平移、点击进成员详情 -->
  <div class="gentree">
    <div class="toolbar">
      <span class="serif title">世代分层图</span>

      <span class="hint-inline">房支</span>
      <el-select :model-value="branchCtx.current" size="small" style="width: 190px"
                 filterable @change="onBranchChange">
        <el-option label="宗谱（全部）" :value="0" />
        <el-option v-for="o in flatOptions" :key="o.id" :label="o.label" :value="o.id" />
      </el-select>
      <el-checkbox :model-value="branchCtx.includeChildren" size="small"
                   @change="onIncludeChildrenChange">含下级</el-checkbox>

      <el-input v-model="kw" placeholder="搜索姓名/字号" clearable size="small" style="width: 170px"
                @keyup.enter="doSearch" />
      <el-button size="small" @click="doSearch">定位</el-button>

      <el-button size="small" @click="zoomOut">－</el-button>
      <el-button size="small" @click="resetView">重置</el-button>
      <el-button size="small" @click="zoomIn">＋</el-button>
      <FontPicker />
      <el-dropdown size="small" trigger="click" @command="doExport">
        <el-button size="small" :loading="exporting">导出<el-icon class="el-icon--right"><arrow-down /></el-icon></el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="png300">🖼️ PNG 高清（300DPI）</el-dropdown-item>
            <el-dropdown-item command="svg">📐 SVG 矢量图</el-dropdown-item>
            <el-dropdown-item command="svgFont">📐 SVG（内嵌字体）</el-dropdown-item>
            <el-dropdown-item command="pdf">🖨️ PDF（打印用）</el-dropdown-item>
            <el-dropdown-item command="pagedPrint" divided>🖨️ 分页打印（A3/A4 分页 · 卷轴 · 索引页）</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <el-button size="small" :loading="loading" @click="load">刷新</el-button>

      <span class="hint">一行一辈 · 圈内：男排行 / 女「女」/ 配偶「配」 · 标记：失 > 夭 > 入 > 养 > 出 > 赘 > 祧 > 迁 > 止 · 女与配偶实心 · 名竖排 · 滚轮缩放 · 拖拽平移 · 点击看详情</span>
    </div>

    <div class="canvas-wrap" ref="wrap" v-loading="loading"
         @wheel.prevent="onWheel" @pointerdown.prevent="onPointerDown">
      <svg :width="svgW" :height="svgH" ref="svg" class="gen-svg" :class="{ 'tf-on': treeFontActive }" :style="treeFontStyle">
        <g :transform="`translate(${pan.x},${pan.y}) scale(${zoom})`">
          <!-- 世代横向分隔线 + 世代标签（先画底层，再画线，最后画节点） -->
          <g>
            <template v-for="r in rows" :key="'row' + r.gen">
              <line :x1="0" :x2="contentW" :y1="r.y - ROW_TOP" :y2="r.y - ROW_TOP"
                    stroke="#e3d6c2" stroke-width="1" />
              <g class="gen-tag">
                <rect :x="GEN_TAG_X" :y="r.y - ROW_TOP + 16" :width="GEN_TAG_W" :height="tagH(r)"
                      rx="2" fill="#8c2b1e" stroke="#5f1a11" stroke-width="1" />
                <text v-for="(ch, ci) in r.tagChars" :key="ci"
                      :x="GEN_TAG_X + GEN_TAG_W / 2" :y="r.y - ROW_TOP + 16 + 24 + ci * 20"
                      text-anchor="middle" font-family="Kaiti, STKaiti, KaiTi, Songti SC, SimSun, serif"
                      font-size="17" fill="#fdf6e6">{{ ch }}</text>
              </g>
            </template>
          </g>

          <!-- 连线（先画线，后画节点，保证线在圈下面） -->
          <g v-for="(c, i) in connections" :key="'c' + i">
            <path :d="c.d" :stroke="c.marry ? '#b08a68' : '#8c5a3c'" :stroke-width="c.marry ? 1 : 1.4"
                  :stroke-dasharray="c.marry ? '3,3' : undefined" fill="none" />
          </g>

          <!-- 节点 -->
          <g v-for="n in nodes" :key="'n' + n.id">
            <!-- 主成员：圈 + 排行 + 竖排姓名 -->
            <g class="p-node" :data-mid="n.id" @click="goMember(n.id)">
              <circle :cx="n.cx" :cy="n.y" :r="R"
                      :fill="n.female ? '#8c2b1e' : '#fffdf6'"
                      :stroke="n.female ? '#5f1a11' : '#8c5a3c'"
                      :stroke-width="n.female ? 1.2 : 1.4" />
              <!-- 圈内中央：族内女性写「女」；族内男性留空（排行不入圈） -->
              <text v-if="n.center" :x="n.cx" :y="n.y" text-anchor="middle"
                    dominant-baseline="middle"
                    font-family="Kaiti, STKaiti, KaiTi, Songti SC, SimSun, serif"
                    font-size="13"
                    :fill="n.female ? '#fdf6e6' : '#8c2b1e'">{{ n.center }}</text>
              <!-- 标注：止/失/出/入/赘。中央空闲（男性）→ 居中大字；中央被「女」占用 → 圈内上缘小字；
                   每个节点只显示一个（优先级 失>入>出>赘>止）；配偶不显示标注 -->
              <text v-if="n.mark" :x="n.cx" :y="n.center ? n.y - R + 11 : n.y" text-anchor="middle"
                    :dominant-baseline="n.center ? 'text-after-edge' : 'middle'"
                    font-family="Kaiti, STKaiti, KaiTi, Songti SC, SimSun, serif"
                    :font-size="n.center ? 7.5 : 13"
                    :fill="n.female ? '#fdf6e6' : '#8c2b1e'">{{ n.mark }}</text>
              <text v-for="(ch, ci) in n.chars" :key="'nc' + ci"
                    :x="n.cx" :y="n.y + R + 16 + ci * NAME_LH" text-anchor="middle"
                    font-family="Kaiti, STKaiti, KaiTi, Songti SC, SimSun, serif"
                    font-size="15" fill="#3a2a18">{{ ch }}</text>
            </g>

            <!-- 配偶：并排于本人右侧，同样 圈 + 竖排姓名 -->
            <g v-for="(s, si) in n.spouses" :key="'s' + s.key" class="p-node" :data-sid="n.id + '-' + si"
               @click="goSpouse(s.sid)">
              <circle :cx="s.cx" :cy="n.y" :r="R" fill="#8c2b1e" stroke="#5f1a11" stroke-width="1.2" />
              <!-- 配偶：圈内中央写「配」（实心圆反白） -->
              <text :x="s.cx" :y="n.y" text-anchor="middle" dominant-baseline="middle"
                    font-family="Kaiti, STKaiti, KaiTi, Songti SC, SimSun, serif"
                    font-size="13" fill="#fdf6e6">配</text>
              <text v-for="(ch, ci) in s.chars" :key="'sc' + ci"
                    :x="s.cx" :y="n.y + R + 16 + ci * NAME_LH" text-anchor="middle"
                    font-family="Kaiti, STKaiti, KaiTi, Songti SC, SimSun, serif"
                    font-size="15" fill="#5a3a2a">{{ ch }}</text>
            </g>
          </g>
        </g>
      </svg>
    </div>

    <el-dialog v-model="tipVisible" :title="tip.name" width="420px">
      <div class="tip-card">
        <div class="row"><span class="label">世代</span><span>第 {{ tip.generation }} 世</span></div>
        <div class="row" v-if="tip.rank"><span class="label">排行</span><span>{{ tip.rank }}</span></div>
        <div class="row"><span class="label">性别</span><span>{{ tip.gender === 1 ? '男' : '女' }}</span></div>
        <div class="row" v-if="tip.clan_type"><span class="label">宗法类型</span><span>{{ tip.clan_type }}</span></div>
        <div class="row" v-if="tip.mark"><span class="label">标注</span><span>{{ MARK_TEXT[tip.mark] }}</span></div>
        <div class="row" v-if="tip.spouseCount"><span class="label">配偶</span><span>{{ tip.spouseCount }} 位</span></div>
      </div>
      <template #footer>
        <el-button @click="tipVisible = false">关闭</el-button>
        <el-button type="primary" @click="goMember(tip.id)">查看详情</el-button>
      </template>
    </el-dialog>

    <!-- 打印 / 导出：A3/A4 分页（瓦片）· 卷轴 · 索引页 -->
    <PrintExportDialog
      v-model="printDlgVisible"
      :svg-el="svg"
      :content-w="contentW"
      :content-h="contentH"
      :rows="printRows"
      :bounds="null"
      title="世代分层图"
      :volume="printVolume"
      :theme="{ bg: '#ffffff', ink: '#3a2a18' }"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api'
import { exportHighResPng, exportVectorSvg, exportPrintPDF } from '../utils/treeExport'
import PrintExportDialog from '../components/PrintExportDialog.vue'
import FontPicker from '../components/FontPicker.vue'
import { treeFontActive, treeFontStyle } from '../utils/treeFonts'
import { ArrowDown } from '@element-plus/icons-vue'
import {
  branchCtx, loadBranches, setCurrent, setIncludeChildren, buildFlatOptions
} from '../utils/branchContext'

const router = useRouter()

// ===== 版面常量（px，与既有世系视图同风格） =====
const R = 16              // 节点圆圈半径（放大以容纳圈内「排行 + 上缘标注」，不改变列距规则）
const NAME_LH = 17        // 竖排姓名行高
const COL_GAP = 26        // 相邻家庭（本人+配偶）之间的最小列间距
const SP_GAP = 10         // 本人与配偶圈、配偶圈之间的间距（加大以免父侧竖线擦到配偶姓名）
const GEN_TAG_X = 12      // 世代标签 x（内容坐标，随图一起平移缩放）
const GEN_TAG_W = 30      // 世代标签宽
const HALF_CHAR = 8       // 竖排姓名单字墨迹半宽（15px 字 ≈ 7.7px，取 8 留余量）
const ROW_TOP = 22        // 行顶部留白（放圈内上缘标注）
const ROW_BOTTOM = 20     // 姓名下方留白
const ABOVE_GAP = 16      // 「横杠 → 子圈顶」间距（属主指定 16px）
const NAME_DROP = 6       // 「该行姓名块下沿 → 横杠」的固定长度（一丁点）

const CN_NUM = { 一: 1, 二: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9, 十: 10 }
const CN_DIGIT = '〇一二三四五六七八九'
const MARK_TEXT = {
  失: '失（迁居失联）', 夭: '夭（幼年早逝，殁时未满 14 岁）', 入: '入（嗣子）', 养: '养（养子）', 出: '出（出继，有嗣父）',
  赘: '赘（入赘）', 祧: '祧（兼祧）', 迁: '迁（迁居外地）', 止: '止（无嗣无子女）'
}

const members = ref([])
const loading = ref(false)
const exporting = ref(false)

// ===== 分页打印（A3/A4 · 卷轴）=====
// 世代分层图每一行就是一个世代，直接把 rows 的 rowTop/rowH 交给通用分页模块。
const printDlgVisible = ref(false)
const printRows = computed(() => (rows.value || []).map(r => ({
  y0: r.rowTop,
  y1: r.rowTop + r.rowH,
  label: (r.tagChars || []).join('')
})))
const printVolume = computed(() => {
  const id = branchCtx.current
  if (!id) return '宗谱（全部）'
  const b = branchCtx.branches.find(x => x.id === id)
  return (b ? b.name : `房支 #${id}`) + (branchCtx.includeChildren ? '（含下级）' : '')
})
const migrations = ref([])      // 迁徙记录（用于「迁」标记：member_ids 含该成员即标）
const kw = ref('')
const wrap = ref(null)
const svg = ref(null)
const zoom = ref(1)
const pan = reactive({ x: 24, y: 24 })
const svgW = ref(1200)
const svgH = ref(800)
const contentW = ref(1000)
const contentH = ref(800)
const tipVisible = ref(false)
const tip = reactive({})
const marks = ref({ 失: 0, 入: 0, 出: 0, 赘: 0, 止: 0, bothChuRu: 0 })

const flatOptions = computed(() => buildFlatOptions())

// ===== 排行解析：'行一/长子/次子/行11/三' → 序号（无排行返回 null） =====
function rankNum(r) {
  if (r == null) return null
  const s = String(r).trim()
  if (!s) return null
  const m = s.match(/\d+/)
  if (m) return parseInt(m[0], 10)
  const ch = s.replace(/^(行|第|排行|排)/, '')
  for (const c of ch) {
    if (CN_NUM[c]) return CN_NUM[c]
    if (c === '长' || c === '長' || c === '元') return 1
    if (c === '次') return 2
    if (c === '末' || c === '幼') return 99
  }
  if (s.includes('长') || s.includes('長')) return 1
  if (s.includes('次')) return 2
  return null
}

// 排行的展示：'行一' → '一'；'长子' → '长'；过长的取末字
function rankLabel(r) {
  const s = String(r == null ? '' : r).trim()
  if (!s) return ''
  let t = s.replace(/^(行|第|排行|排)/, '')
  if (!t) t = s
  return t.length > 3 ? t.slice(0, 3) : t
}

function birthKey(m) {
  const s = String(m.birth_gregorian || '')
  const m2 = s.match(/(\d{1,4})(?:[-/年.](\d{1,2}))?(?:[-/月.](\d{1,2}))?/)
  if (!m2) return null
  return parseInt(m2[1], 10) * 10000 + parseInt(m2[2] || '0', 10) * 100 + parseInt(m2[3] || '0', 10)
}

// 兄弟排序：排行 → 出生日期 → id
function sortSiblings(arr) {
  arr.sort((a, b) => {
    const ra = rankNum(a.rank), rb = rankNum(b.rank)
    if (ra != null && rb != null && ra !== rb) return ra - rb
    if (ra != null && rb == null) return -1
    if (ra == null && rb != null) return 1
    const ba = birthKey(a), bb = birthKey(b)
    if (ba != null && bb != null && ba !== bb) return ba - bb
    if (ba != null && bb == null) return -1
    if (ba == null && bb != null) return 1
    return a.id - b.id
  })
}

function cnNum(n) {
  const v = Math.abs(Math.trunc(Number(n) || 0))
  if (v <= 10) return ['〇', '一', '二', '三', '四', '五', '六', '七', '八', '九', '十'][v]
  if (v < 20) return '十' + CN_DIGIT[v - 10]
  if (v < 100) return CN_DIGIT[Math.floor(v / 10)] + '十' + (v % 10 ? CN_DIGIT[v % 10] : '')
  return String(v)
}

function nameOf(m) {
  return m.name || ((m.surname || '') + (m.given_name || '')) || '（无名）'
}
function spouseNameOf(s) {
  // 有 given_name 用全名；只有姓氏显示「X氏」
  const full = s.name || ((s.surname || '') + (s.given_name || ''))
  if (s.given_name) return full
  if (s.surname) return s.surname + '氏'
  return full || '（配）'
}
function isFemaleGender(g) { return Number(g) !== 1 }

// 世代标签（竖排「一世」）高度：随行高自适应，不越出行范围
function tagH(r) { return Math.max(62, Math.min((r.rowH || 160) - 24, 100)) }

// 把 [x1,x2] 上被若干区间遮住的部分切开，返回未被遮挡的多段（用于横杠绕开姓名块）
function segmentsX(x1, x2, blocks) {
  const bs = (blocks || []).filter(b => b.x2 > x1 && b.x1 < x2).sort((a, b) => a.x1 - b.x1)
  const out = []
  let cur = x1
  bs.forEach(b => {
    if (b.x1 > cur) out.push([cur, Math.min(b.x1, x2)])
    cur = Math.max(cur, b.x2)
  })
  if (cur < x2) out.push([cur, x2])
  return out
}

// ===== 新增标记的数据解析（迁 / 夭）=====
// 「迁」：迁居一般整支一起搬 → **只标该支最顶端的成员**（属主最终口径）。
// 口径：对每条迁徙记录的 member_ids 名单，取名单内「没有任何祖先也在名单内」者（＝该支顶端）；
//   · 名单内**有**祖先 → 只标最上面那个（子孙不标）；
//   · 名单内**没有**列祖先（只列了子孙）→ 名单里**各分支的顶点都标**（即上述判定自然落到的结果）；
//   · 同代多个兄弟一起迁 → 都标（各自是所在小支的顶端）；名单只有一个成员 → 就标他。
function parseMigratedIds(list, byId) {
  const out = new Set()
  ;(list || []).forEach((mg) => {
    const ids = String(mg && mg.member_ids || '').split(/[,\s，、]+/)
      .map((x) => parseInt(x, 10)).filter((n) => Number.isFinite(n) && n > 0)
    const inList = new Set(ids)
    ids.forEach((id) => {
      let cur = byId && byId.get(id)
      let hasAncestorInList = false
      let guard = 0
      while (cur && guard++ < 200) {
        const pid = Number(cur.father_id) || 0
        if (!pid) break
        if (inList.has(pid)) { hasAncestorInList = true; break }
        cur = byId.get(pid)
      }
      if (!hasAncestorInList) out.add(id)
    })
  })
  return out
}

// 「夭」：殁时未满 16 岁。按 birth_/death_ 的 gregorian → era → lunar 依次尝试解析年份；
// 解析不出年份就不标（宁缺勿错）。
const YAO_AGE = 14           // 早逝阈值（殁时未满 14 周岁；属主口径）
const CN_DIGITS = { 〇: 0, 零: 0, 一: 1, 二: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9 }
function cnToNum(str) {
  const t = String(str || '').replace(/[元正]/g, '一')
  if (!t) return null
  if (/^\d+$/.test(t)) return parseInt(t, 10)
  let total = 0, section = 0, num = 0, seen = false
  for (const ch of t) {
    if (ch in CN_DIGITS) { num = CN_DIGITS[ch]; seen = true }
    else if (ch === '十') { section += (num || 1) * 10; num = 0; seen = true }
    else if (ch === '百') { section += (num || 1) * 100; num = 0; seen = true }
    else if (ch === '千') { section += (num || 1) * 1000; num = 0; seen = true }
    else if (ch === '万') { total += (section + num) * 10000; section = 0; num = 0; seen = true }
  }
  return seen ? total + section + num : null
}
// 年號 → 公元元年（覆盖明清；表外的年號不解析）
const ERA_BASE = {
  洪武: 1368, 建文: 1399, 永樂: 1403, 永乐: 1403, 洪熙: 1425, 宣德: 1426, 正統: 1436, 正统: 1436,
  景泰: 1450, 天順: 1457, 天顺: 1457, 成化: 1465, 弘治: 1488, 正德: 1506, 嘉靖: 1522, 隆慶: 1567, 隆庆: 1567,
  萬曆: 1573, 万历: 1573, 泰昌: 1620, 天啟: 1621, 天启: 1621, 崇禎: 1628, 崇祯: 1628,
  順治: 1644, 顺治: 1644, 康熙: 1662, 雍正: 1723, 乾隆: 1736, 嘉慶: 1796, 嘉庆: 1796,
  道光: 1821, 咸豐: 1851, 咸丰: 1851, 同治: 1862, 光緒: 1875, 光绪: 1875, 宣統: 1909, 宣统: 1909
}
function yearFromText(s) {
  const t = String(s || '').trim()
  if (!t) return null
  // 公历：1700 / 1700-12-11 / 1700年…
  const m1 = t.match(/(\d{3,4})/)
  if (m1 && /^\d/.test(t)) return parseInt(m1[1], 10)
  // 年號 + 漢數字：康熙三十九 / 乾隆五十四
  const m2 = t.match(/^([\u4e00-\u9fa5]{2})(.*)$/)
  if (m2 && ERA_BASE[m2[1]]) {
    const n = cnToNum(m2[2])
    if (n) return ERA_BASE[m2[1]] + n - 1
  }
  // 純漢數字年份（如干支/民國前的農曆年記法）：一七三二年…
  const digits = [...t].filter((c) => c in CN_DIGITS).map((c) => CN_DIGITS[c])
  if (digits.length >= 3 && /^[〇零一二三四五六七八九]+/.test(t)) return parseInt(digits.join(''), 10)
  return null
}
function yearsOf(m, which) {
  return [which + '_gregorian', which + '_era', which + '_lunar']
    .map((k) => yearFromText(m[k])).find((y) => y && y > 800 && y < 2200) || null
}
// 精确到日：yyyy / yyyy-mm / yyyy-mm-dd（公历字段有就用精确算，缺失时退化为年份差近似）
function ymdFromText(s) {
  const t = String(s || '').trim()
  const m = t.match(/^(\d{3,4})(?:[-/.年](\d{1,2}))?(?:[-/.月](\d{1,2}))?/)
  if (!m) return null
  return { y: parseInt(m[1], 10), m: m[2] ? parseInt(m[2], 10) : null, d: m[3] ? parseInt(m[3], 10) : null }
}
function isEarlyDeath(m) {
  const bd = ymdFromText(m.birth_gregorian)
  const dd = ymdFromText(m.death_gregorian)
  // 精确分支：生、卒**都有完整公历 年-月-日** → 未满 14 周岁才算夭
  const exact = bd && dd && bd.m != null && bd.d != null && dd.m != null && dd.d != null
  if (exact) {
    let age = dd.y - bd.y
    if (dd.m < bd.m || (dd.m === bd.m && dd.d < bd.d)) age -= 1
    return age >= 0 && age < YAO_AGE
  }
  // 近似分支：缺月或缺日，或只有年號/農曆 → **边界一律算夭**（差满 14 年也算）
  const by = (bd && bd.y) || yearsOf(m, 'birth')
  const dy = (dd && dd.y) || yearsOf(m, 'death')
  if (!by || !dy) return false
  const age = dy - by
  return age >= 0 && age <= YAO_AGE
}

// ===== 布局：一行一辈（最上为第一世）；层内先按长幼排序，再由下往上把父居中于子嗣组 =====
const layout = computed(() => {
  const list = members.value || []
  const empty = { nodes: [], connections: [], rows: [], w: 1000, h: 400 }
  if (!list.length) return empty

  const byId = new Map()
  list.forEach(m => byId.set(m.id, m))

  // 姓名竖排拆字（最多 7 字，超出截断）
  let maxNameLen = 1
  list.forEach(m => { maxNameLen = Math.max(maxNameLen, Math.min(7, Array.from(nameOf(m)).length)) })
  // 姓名块预留高度：按「该行实际最长姓名」算（行与行可不同高）；
  // 全树最长姓名仅用于探针/兼容统计。
  const globalMaxNameLen = Math.max(1, maxNameLen)
  function nameLenOf(layer) {
    return Math.max(1, ...layer.map(m => Array.from(nameOf(m)).length))
  }
  // 行高：按「全树最长姓名」预留统一的姓名块高度 —— 保证同层姓名块下沿一致，
  // 且该下沿 + K 仍在下一行圆圈顶之上（两段长度恒定）。
  // 行高：既要放下本行统一姓名块 + 横杠 + 上方段，也要保证「父姓名块下沿 + K」不会压到
  // 下一行的姓名（跨世代时同理），故取 reserveLen 与 (reserveLen - 3 字) 两档的较大值。
  function rowHOf(layer) {
    const rl = nameLenOf(layer)
    // 行高 = ROW_TOP + 2R + (姓名块上沿到首字基线的 16px − 与上方留白重复计入的 16px)
    //        + 该行最长姓名高度 + 下方段(NAME_DROP) + 上方段(ABOVE_GAP) + 少量余量
    // 行高 = ROW_TOP + 2R + 该行最长姓名块高 + NAME_DROP + ABOVE_GAP
    // 这样横杠恒在「姓名块下沿 + NAME_DROP」且距子圈顶 ABOVE_GAP，两段长度都不为负
    return ROW_TOP + 2 * R + rl * NAME_LH + NAME_DROP + ABOVE_GAP
  }

  // ① 父子的可见范围：父不在范围内（跨房支断链）→ 该成员作为根
  const kids = new Map()
  const parentIn = new Map()
  list.forEach(m => {
    const pid = m.father_id || 0
    const has = pid > 0 && byId.has(pid) && pid !== m.id
    parentIn.set(m.id, has)
    if (has) {
      if (!kids.has(pid)) kids.set(pid, [])
      kids.get(pid).push(m)
    }
  })
  kids.forEach(arr => sortSiblings(arr))

  // ② 有无子女（用于「止」判定：no_heir 且无子女）
  const hasChild = new Set()
  list.forEach(m => { if (parentIn.get(m.id)) hasChild.add(m.father_id) })

  // ③ 标注判定（每个节点只出一个）：优先级 失 > 夭 > 入 > 养 > 出 > 赘 > 祧 > 迁 > 止
  //    失 = lost_contact（迁居外地失联 / 存殁未详，1）
  //    夭 = 殁时未满 16 岁（由 birth_/death_ 的 gregorian/era/lunar 尽可能解析；解析不出不标）
  //    入 = 宗法类型 嗣子；养 = 宗法类型 养子；出 = 有嗣父(foster_father_id)；赘 = 宗法类型 入赘
  //    祧 = 宗法类型 兼祧；迁 = 出现在 migrations.member_ids 里
  //    止 = no_heir 且无子女；出嫁不做标记、传记标记不做
  const migratedIds = parseMigratedIds(migrations.value, byId)
  const markOf = new Map()
  const stat = { 失: 0, 夭: 0, 入: 0, 养: 0, 出: 0, 赘: 0, 祧: 0, 迁: 0, 止: 0, bothChuRu: 0, 夭未解析: 0 }
  list.forEach(m => {
    const ct = String(m.clan_type || '')
    const isLost = Number(m.lost_contact) === 1
    const isYao = isEarlyDeath(m)
    const isIn = ct === '嗣子'          // 入（嗣子）
    const isYang = ct === '养子'        // 养（养子）
    const isChu = (m.foster_father_id || 0) > 0
    const isZhui = ct === '入赘'
    const isTiao = ct === '兼祧'
    const isQian = migratedIds.has(Number(m.id))   // 仅该支最顶端
    let mk = ''
    if (isLost) mk = '失'
    else if (isYao) mk = '夭'
    else if (isIn) mk = '入'
    else if (isYang) mk = '养'
    else if (isChu) mk = '出'
    else if (isZhui) mk = '赘'
    else if (isTiao) mk = '祧'
    else if (isQian) mk = '迁'
    else if (m.no_heir && !hasChild.has(m.id)) mk = '止'
    if ((isIn || isYang) && isChu) stat.bothChuRu++
    if (mk) { markOf.set(m.id, mk); stat[mk]++ }
  })
  // 注：stat 在下面节点建完后回填 marks.value（此处 List 尚未生成节点，无需等待）

  // ④ 按世代分层（生成的世代号可能不连续，按序号逐行排）
  const gens = [...new Set(list.map(m => Number(m.generation) || 0))].sort((a, b) => a - b)
  const genIndex = new Map(gens.map((g, i) => [g, i]))
  const layers = gens.map(() => [])
  list.forEach(m => layers[genIndex.get(Number(m.generation) || 0)].push(m))

  const tagWidth = GEN_TAG_X + GEN_TAG_W + 24   // 左侧世代标签占用宽度

  // ⑤ 逐层分配 x：先按兄弟次序排序，再由下往上把「父」摆到其子嗣组的正上方居中；
  //    无子嗣者按当前游标顺序摆放；无父（跨房支断链的根）排在该层最前。
  //    这样既保证同一父的子女聚在一起，又让长支居中、树形不向右漂移。
  const nodes = []
  const rows = []
  const placed = new Map()    // id → 节点
  const layerNodes = []       // 每层的节点数组（按最终 x 升序）
  let y = 0

  // 每层先排好序并算出每个节点的自身宽度（圈 + 配偶圈）
  const sortedLayers = layers.map(layer => layer.slice().sort((a, b) => {
    const va = parentIn.get(a.id) ? 1 : 0            // 有父者排后（无父＝断链根，排该层最前）
    const vb = parentIn.get(b.id) ? 1 : 0
    if (va !== vb) return va - vb
    const ra = rankNum(a.rank), rb = rankNum(b.rank)
    if (ra != null && rb != null && ra !== rb) return ra - rb
    if (ra != null && rb == null) return -1
    if (ra == null && rb != null) return 1
    return a.id - b.id
  }))

  // 逐层建节点（先只算自身宽度与行位置，x 待定）
  const rowHs = sortedLayers.map(layer => rowHOf(layer))    // 每行按各自最长姓名
  const rowNameLens = sortedLayers.map(layer => nameLenOf(layer))
  sortedLayers.forEach((sorted, gi) => {
    const rowH = rowHs[gi]
    const rowNameLen = rowNameLens[gi]
    const rowTop = y
    y += rowH
    const rowNodes = sorted.map((m, idx) => {
      const sps = (m.spouses || []).map((s, i) => ({
        key: m.id + '_' + (s.id != null ? s.id : i),
        sid: s.id,                                // 配偶自身 id（点圈跳 /spouse/:id）
        rel: (i + 1) * (2 * R + SP_GAP),          // 相对本人圈心的偏移（layoutSubtree 里会按姓名宽度重算）
        relX: (i + 1) * (2 * R + SP_GAP),
        chars: Array.from(spouseNameOf(s)).slice(0, 7),
        center: '配'                              // 配偶圈内中央字（实心圆反白）
      }))
      const width = 2 * R + sps.length * (2 * R + SP_GAP)
      // 姓名块半宽：本人名与配偶名里最宽的那一列（竖排单字墨迹宽约 17.6px → 半宽 8.8）
      const halfW = Math.max(R, HALF_CHAR, ...sps.map(sp => Math.max(1, sp.chars.length) * HALF_CHAR))
      return {
        id: m.id,
        m,
        halfW,
        gen: Number(m.generation) || 0,
        col: idx,
        centerX: 0,
        rowTop,
        width,
        chars: Array.from(nameOf(m)).slice(0, 7),
        rowNameLen,                             // 该行统一姓名块长度（按行内最长姓名）
        female: isFemaleGender(m.gender),
        rank: rankLabel(m.rank),
        // 圈内中央字：族内女性＝「女」；族内男性＝留空（排行不再入圈）；配偶＝「配」（见 sps）
        center: isFemaleGender(m.gender) ? '女' : '',
        mark: markOf.get(m.id) || '',
        spouses: sps,
        y: rowTop + ROW_TOP + R
      }
    })
    rowNodes.forEach(nd => { placed.set(nd.id, nd); nodes.push(nd) })
    layerNodes.push(rowNodes)
    rows.push({
      gen: gens[gi],
      y: rowTop + ROW_TOP + R,
      rowTop,
      rowH,
      tagChars: Array.from(cnNum(gens[gi]) + '世')
    })
  })

  // ⑤′ x 分配：镜像「苏式垂丝图」的靠右主干排法 → 靠左主干
  //    · 长子（排行第一）作为主干：父圈心正对其**长子**（与垂丝图把长子竖线正对父节点同构）；
  //    · 次子/三子… 依次往**右**排开（垂丝图是往左），组间统一间距；
  //    · 自底向上算子树左右延伸，再自顶向下累加绝对位置 → 整树靠左、组内紧凑、组间不重叠。
  const AX_GAP = 7          // 相邻两列姓名块之间的最小可见间隙
  const relOf = new Map()   // id → { rel, leftExt, rightExt }
  // 节点姓名块整体：左半宽（最左列）与右半宽（最右列，含配偶）
  const leftH = n => n.halfW
  const rightH = n => {
    let h = n.halfW
    ;(n.spouses || []).forEach(sp => { h = Math.max(h, (sp.relX != null ? sp.relX : sp.rel) + Math.max(sp.chars.length, 1) * HALF_CHAR) })
    return h
  }
  // 取相邻列的姓名块半宽（用于补足「姓名块不重叠」所需的最小列距）
  function neighborHalf(list, i) {
    const hs = []
    if (i > 0) hs.push(list[i - 1].halfW)
    if (i + 1 < list.length) hs.push(list[i + 1].halfW)
    return hs.length ? Math.max(...hs) : 0
  }
  // 轴布局：长子居左主干（rel=0），弟弟们依次往右，列距不足时补到「姓名块不重叠」
  function layoutSubtree(node) {
    const ks = (kids.get(node.id) || []).map(k => placed.get(k.id)).filter(Boolean)
    if (!ks.length) {
      relOf.set(node.id, { rel: 0, leftExt: R, rightExt: Math.max(R, node.width - R) })
      return
    }
    ks.forEach(layoutSubtree)
    // 本人 ↔ 配偶：列距至少 = 两者姓名块半宽 + 间隙（配偶默认紧贴本人，可能压字）
    ks.forEach(k => {
      let rx = R + SP_GAP
      let prevHalf = k.halfW
      k.spouses.forEach(sp => {
        const need = prevHalf + Math.max(sp.chars.length, 1) * HALF_CHAR + AX_GAP
        rx = Math.max(rx + 2 * R + SP_GAP, need)
        sp.relX = rx
        prevHalf = Math.max(sp.chars.length, 1) * HALF_CHAR
        rx += 2 * R + SP_GAP
      })
    })
    let relX = 0
    ks.forEach((k, i) => {
      const rel = relOf.get(k.id)
      rel.rel = i === 0 ? 0 : relX
      const nat = rel.rel + rel.leftExt + AX_GAP + rel.rightExt
      const minGap = rightH(k) + (i + 1 < ks.length ? leftH(ks[i + 1]) : 0) + AX_GAP
      relX = Math.max(nat, minGap)
    })
    relOf.set(node.id, {
      rel: 0,                                        // 父对齐其最左（长）子
      leftExt: Math.max(R, leftH(ks[0])),
      rightExt: Math.max(...ks.map(k => relOf.get(k.id).rel + rightH(k)))
    })
  }
  const absOf = new Map()      // id → 绝对圈心 x（相对坐标起点 0，后面整体右移）
  const relRootOf = new Map()
  let cursorX = 0
  const placeSubtreeAt = (node) => {
    if (!relOf.has(node.id)) layoutSubtree(node)
    const rel = relOf.get(node.id)
    relRootOf.set(node.id, cursorX + (R - rel.rel))
    absOf.set(node.id, relRootOf.get(node.id))
    cursorX = relRootOf.get(node.id) + rel.rel + rel.rightExt + COL_GAP
  }
  ;(layerNodes[0] || []).forEach(r => { layoutSubtree(r); placeSubtreeAt(r) })
  // ② 自顶向下：父已定位 → 用父的绝对位置 + 相对偏移；父未定位 → 自成一组接在游标之后
  layerNodes.forEach((row, gi) => {
    if (gi === 0) return
    row.forEach(nd => {
      const pid = nd.m.father_id || 0
      const pAbs = absOf.get(pid)
      if (pAbs == null) {
        placeSubtreeAt(nd)
      } else {
        if (!relOf.has(nd.id)) layoutSubtree(nd)
        absOf.set(nd.id, pAbs + relOf.get(nd.id).rel)
      }
    })
  })
  // ③ 迭代收敛：既要「同层姓名块整体不重叠」，又要「父对齐其最左子女」。
  //    交替做两件事、跑若干轮：① 同层从左到右右推冲突列及其全部后代；② 父在子女定位后重新对齐长子。
  for (let iter = 0; iter < 4; iter++) {
    let moved = 0
    layerNodes.forEach(row => {
      const seq = row.slice().sort((a, b) => absOf.get(a.id) - absOf.get(b.id))
      for (let i = 1; i < seq.length; i++) {
        const prev = seq[i - 1], cur = seq[i]
        const need = rightH(prev) + leftH(cur) + AX_GAP
        const dx = (absOf.get(prev.id) + need) - absOf.get(cur.id)
        if (dx > 0.5) {
          moved++
          const stack = [cur.id], seen = new Set()
          while (stack.length) {
            const id = stack.pop()
            if (seen.has(id)) continue
            seen.add(id)
            absOf.set(id, absOf.get(id) + dx)
            ;(kids.get(id) || []).forEach(k => stack.push(k.id))   // 全部后代（含跨世代）
          }
        }
      }
    })
    // ② 父重新对齐其最左子女（左主干），保持整树靠左
    let realign = 0
    layerNodes.forEach((row, gi) => {
      if (gi === 0) return
      row.forEach(nd => {
        const pid = nd.m.father_id || 0
        const pAbs = absOf.get(pid)
        if (pAbs == null) return
        const ks = kids.get(nd.id) || []
        if (!ks.length) return
        const kidsAbs = ks.map(k => absOf.get(k.id)).filter(v => v != null)
        if (!kidsAbs.length) return
        const want = Math.min(...kidsAbs)
        const d = want - absOf.get(nd.id)
        if (Math.abs(d) > 0.5) {
          realign++
          // 只移动本节点（其子树由下一轮/上位节点再对齐处理）
          absOf.set(nd.id, want)
        }
      })
    })
    if (!moved && !realign) break
  }
  nodes.forEach(nd => { nd.centerX = absOf.get(nd.id) })

  marks.value = stat

  // 转绝对坐标（整体右移，留出世代标签位置）
  let maxRight = 0
  const minCenter = Math.min(...nodes.map(nd => nd.centerX - R))
  const shift = tagWidth - minCenter
  nodes.forEach(nd => {
    nd.cx = nd.centerX + shift
    nd.spouses.forEach(s => { s.cx = nd.cx + s.rel })
    nd.right = nd.cx - R + nd.width
    maxRight = Math.max(maxRight, nd.right)
  })

  // 图窄于视口时整体水平居中
  const wrapW = wrap.value ? wrap.value.clientWidth : 0
  const needW = maxRight + 48
  const off = needW < wrapW ? (wrapW - needW) / 2 : 0
  if (off > 0) {
    nodes.forEach(n => {
      n.cx += off
      n.left += off
      n.right += off
      n.spouses.forEach(s => { s.cx += off })
    })
    maxRight += off
  }
  contentW.value = Math.max(maxRight + 48, wrapW, 800)

  // ⑥ 连接线（最朴素的直角折线，只有两处折角）
  //    路径 = M(父cx, 姓名块下沿+5) V(横杠Y) H(子cx) V(子圈顶)
  //    · 竖线就在圈心的 x 上，不做任何横向偏移/绕行；
  //    · 横杠是一条完整直线段，x 范围＝该父「最左子女圈心 → 最右子女圈心」；
  //    · 横杠放在子行圆圈顶之上的空白带（子行姓名都在各自圈下方，故不会压字）。
  const connections = []
  // 本人姓名「实际字形」下沿（按各自的姓名长度算，不用预留块下沿）——竖线从这里起，保证接上名字
  // 实测：首字基线 = 圈心 + 32（圆半径 R + 圆圈与姓名间距 16），每字基线间距 NAME_LH，
  //       楷体/宋体汉字墨迹下沿约在 基线 + 0.34×字号(15) ≈ 基线 + 5.1，这里按 +5 取并留视觉余量
  const glyphInkBottomOf = n => n.y + R + 16 + (n.chars.length - 1) * NAME_LH + 5
  // 该行统一的姓名块下沿（按该行最长姓名预留）——横杠按它定 y，保证同层两段一致
  const blockBottomOf = n => n.y + R + 16 + n.rowNameLen * NAME_LH
  nodes.forEach(p => {
    p.spouses.forEach((s, i) => {
      const from = i === 0 ? p.cx + R : p.spouses[i - 1].cx + R
      connections.push({ kind: 'marry', y: p.y, segs: [[from, p.y, s.cx - R, p.y]] })
    })
    const ks = (kids.get(p.id) || []).map(k => placed.get(k.id)).filter(Boolean)
    if (!ks.length) return
    const dropY = glyphInkBottomOf(p) + 0.5           // 父侧竖线起点：本人姓名实际字形（墨迹）下沿 +0.5px
    const childTopY = ks[0].y - R                     // 子圈顶（同层子圈同一 y）
    // 横杠 = 统一姓名块下沿 + K；行高已保证它仍在子圈顶之上（≥ ABOVE_GAP）
    // 横杠 = 姓名块下沿 + NAME_DROP；再夹一手，保证仍在子圈顶上方 ABOVE_GAP（收太紧的行自动兜底）
    const barY = Math.max(blockBottomOf(p) + NAME_DROP, childTopY - ABOVE_GAP)
    const xs = ks.map(k => k.cx)
    const bx1 = Math.min(...xs), bx2 = Math.max(...xs)
    connections.push({ kind: 'drop', id: p.id, label: p.chars.join(''), chars: p.chars.length, rowNameLen: p.rowNameLen,
                       x: p.cx, y1: dropY, y2: barY, segs: [[p.cx, dropY, p.cx, barY]] })
    // 统一走「父姓名下沿 → 竖到 barY → 横到子 x → 竖到子圈顶」，**所有父子连线都画横段**：
    // 单子女时横段长度可能为 0（退化成一点），线自然连续；不允许出现「父侧竖线到 barY 停、
    // 子侧竖线从别处起」被省略成两段分离的情况（那正是此前的断线根因）。
    connections.push({ kind: 'bar', rowTop: ks[0].rowTop, y: barY, x1: bx1, x2: bx2, dropY, childTopY,
                       nameBottom: dropY - 6, rowNameLen: p.rowNameLen, parentLabel: p.chars.join(''),
                       segs: [[bx1, barY, bx2, barY]] })
    ks.forEach(k => connections.push({ kind: 'riser', x: k.cx, y1: barY, y2: k.y - R, segs: [[k.cx, barY, k.cx, k.y - R]] }))
  })

  const lastRow = rows[rows.length - 1]
  const bottom = (lastRow ? lastRow.rowTop + lastRow.rowH : 0) + 24
  contentH.value = Math.max(bottom, 600)

  return { nodes, connections, rows, w: contentW.value, h: contentH.value }
})

const nodes = computed(() => layout.value.nodes)
const rows = computed(() => layout.value.rows)
const connectionsRaw = computed(() => layout.value.connections)

// 姓名/文字的真实字形盒（渲染后量得；首帧为空 → 精修在第二帧生效）
const textBoxes = ref([])
let measuring = false

// 第二段：输出连线（不做任何字形避让）
//   唯一处理：同层内不同组的横杠，若端点相接/重叠，则两组各自内缩到至少 GAP 间隙
//   （属主允许的唯一「收紧」，不是绕行/断线）
const BAR_GAP = 8
const connections = computed(() => {
  const raw = connectionsRaw.value
  const out = []
  // 按「同一子行」分组，逐组收紧端点
  const bars = raw.filter(c => c.kind === 'bar')
  const fixed = bars.map(b => ({ ...b, x1: b.x1, x2: b.x2 }))
  for (let i = 0; i < fixed.length; i++) {
    for (let j = i + 1; j < fixed.length; j++) {
      const a = fixed[i], b = fixed[j]
      if (Math.abs(a.rowTop - b.rowTop) > 1) continue       // 不同行不比较
      if (a.x1 <= b.x1) {
        const gap = b.x1 - a.x2
        if (gap < BAR_GAP) {
          const need = (BAR_GAP - gap) / 2
          a.x2 = Math.max(a.x1 + 1, a.x2 - need)
          b.x1 = Math.min(b.x2 - 1, b.x1 + need)
        }
      } else {
        const gap = a.x1 - b.x2
        if (gap < BAR_GAP) {
          const need = (BAR_GAP - gap) / 2
          b.x2 = Math.max(b.x1 + 1, b.x2 - need)
          a.x1 = Math.min(a.x2 - 1, a.x1 + need)
        }
      }
    }
  }
  const barOf = b => fixed.find(f => f.rowTop === b.rowTop && Math.abs(f.y - b.y) < 1 &&
                                     Math.abs(f.x1 - b.x1) < 40 && Math.abs(f.x2 - b.x2) < 40)
  raw.forEach(c => {
    if (c.kind === 'bar') {
      const f = barOf(c)
      out.push({ bar: true, y: f ? f.y : c.y, x1: f ? f.x1 : c.x1, x2: f ? f.x2 : c.x2,
                 segs: [[f ? f.x1 : c.x1, f ? f.y : c.y, f ? f.x2 : c.x2, f ? f.y : c.y]],
                 d: `M${f ? f.x1 : c.x1} ${f ? f.y : c.y} L${f ? f.x2 : c.x2} ${f ? f.y : c.y}` })
      return
    }
    c.segs.forEach(g => out.push({ marry: c.kind === 'marry', segs: [g], d: `M${g[0]} ${g[1]} L${g[2]} ${g[3]}` }))
  })
  return out
})

// 渲染后量一次字形盒（量完触发精修；有防重入保护）
async function measureTextBoxes() {
  if (measuring || !svg.value) return
  measuring = true
  await nextTick()
  const list = []
  svg.value.querySelectorAll('.p-node').forEach(el => {
    el.querySelectorAll('text').forEach(t => {
      const bb = t.getBBox()
      list.push({ x1: bb.x, y1: bb.y, x2: bb.x + bb.width, y2: bb.y + bb.height })
    })
  })
  textBoxes.value = list
  measuring = false
}

// ===== 数据加载 =====
// 首屏用时（数据返回 → 布局渲染完成），自测探针用
let loadMs = 0

async function load() {
  const t0 = performance.now()
  loading.value = true
  try {
    // 房支参数由 api 拦截器按 branchCtx 自动附加（/members 前缀的 GET 接口）
    const [mr, mg] = await Promise.all([
      api.get('/members/with-spouses'),
      api.get('/migrations').catch(() => ({ data: [] })),      // 迁徙记录（失败不影响主图）
    ])
    members.value = mr.data || []
    migrations.value = Array.isArray(mg.data) ? mg.data : []
  } catch (e) {
    ElMessage.error('加载成员失败：' + (e?.message || e))
  } finally {
    loading.value = false
  }
  if (!branchCtx.loaded) await loadBranches()
  await nextTick()
  fitToView()
  await measureTextBoxes()
  await nextTick()
  loadMs = Math.round(performance.now() - t0)
}

function onBranchChange(v) {
  setCurrent(v)
  load()
}
function onIncludeChildrenChange(v) {
  setIncludeChildren(v)
  load()
}

// ===== 视图交互：缩放 / 平移 / 定位 / 导出 =====
function fitToView() {
  if (!wrap.value) return
  const vw = wrap.value.clientWidth || 1
  const vh = wrap.value.clientHeight || 1
  svgW.value = vw
  svgH.value = vh
  const cw = contentW.value || 1
  const ch = contentH.value || 1
  const pad = 20
  // 树很宽时若按宽度适配会缩到看不清：改为放大到高度适配（多出的宽度用拖拽查看）
  const fitW = (vw - pad * 2) / cw
  const fitH = (vh - pad * 2) / ch
  const s = (fitW < 0.55 && fitH > fitW) ? Math.min(fitH, 1.1) : Math.min(fitW, fitH, 1.6)
  zoom.value = Math.min(Math.max(s, 0.12), 3)
  pan.x = (vw - cw * zoom.value) / 2
  pan.y = pad
}

function zoomIn() { zoom.value = Math.min(zoom.value * 1.2, 3) }
function zoomOut() { zoom.value = Math.max(zoom.value / 1.2, 0.12) }
function resetView() { fitToView() }

function onWheel(e) {
  const rect = wrap.value.getBoundingClientRect()
  const sx = e.clientX - rect.left
  const sy = e.clientY - rect.top
  const nz = Math.min(Math.max(zoom.value * (e.deltaY > 0 ? 0.9 : 1.1), 0.12), 3)
  const ratio = nz / zoom.value
  pan.x = sx - (sx - pan.x) * ratio
  pan.y = sy - (sy - pan.y) * ratio
  zoom.value = nz
}

// 拖拽平移（同时支持双指缩放）
const pointers = new Map()
let dragDist = 0
let startX = 0, startY = 0
let pinchDist = 0, pinchZoom = 1
let moved = false

function onPointerDown(e) {
  pointers.set(e.pointerId, { sx: e.clientX, sy: e.clientY, cx: e.clientX, cy: e.clientY })
  if (pointers.size === 1) {
    dragDist = 0; moved = false
    startX = pan.x; startY = pan.y
  }
  if (pointers.size === 2) {
    const p = [...pointers.values()]
    pinchDist = Math.hypot(p[1].sx - p[0].sx, p[1].sy - p[0].sy)
    pinchZoom = zoom.value
  }
  try { e.target.setPointerCapture(e.pointerId) } catch (err) { /* ignore */ }
}

function onPointerMove(e) {
  if (!pointers.has(e.pointerId)) return
  const entry = pointers.get(e.pointerId)
  entry.cx = e.clientX; entry.cy = e.clientY
  const p = [...pointers.values()]
  if (pointers.size >= 2) {
    const d = Math.hypot(p[1].cx - p[0].cx, p[1].cy - p[0].cy)
    const midX = (p[0].cx + p[1].cx) / 2
    const midY = (p[0].cy + p[1].cy) / 2
    const nz = Math.min(Math.max(pinchZoom * (pinchDist > 0 ? d / pinchDist : 1), 0.12), 3)
    const ratio = nz / zoom.value
    pan.x = midX - (midX - pan.x) * ratio
    pan.y = midY - (midY - pan.y) * ratio
    zoom.value = nz
    moved = true
  } else if (pointers.size === 1) {
    const dx = p[0].cx - p[0].sx
    const dy = p[0].cy - p[0].sy
    dragDist = Math.hypot(dx, dy)
    if (dragDist > 4) moved = true
    pan.x = startX + dx
    pan.y = startY + dy
  }
}

function onPointerUp(e) {
  pointers.delete(e.pointerId)
  if (pointers.size < 2) pinchDist = 0
}
function onPointerCancel(e) { pointers.delete(e.pointerId) }

function doSearch() {
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur()
  const k = kw.value.trim()
  if (!k) return
  const hit = (members.value || []).find(m =>
    nameOf(m).includes(k) || String(m.zi || '').includes(k) || String(m.hao || '').includes(k))
  if (!hit) { ElMessage.warning('未找到匹配成员'); return }
  const n = nodes.value.find(x => x.id === hit.id)
  if (!n) return
  const vw = wrap.value.clientWidth || 1
  const vh = wrap.value.clientHeight || 1
  zoom.value = Math.min(Math.max(zoom.value, 1.2), 2)
  pan.x = vw / 2 - n.cx * zoom.value
  pan.y = vh / 2 - n.y * zoom.value
}

function goMember(id) {
  if (moved) return          // 拖拽结束的误触不跳转
  router.push(`/member/${id}`)
}
// 点配偶圈 → 配偶详情页（/spouse/:id；配偶 id 由节点数据带出）
function goSpouse(sid) {
  if (moved) return
  if (sid) router.push(`/spouse/${sid}`)
}

// 导出分档：PNG 高清/超清、矢量 SVG、打印 PDF（复用 utils/treeExport，行为与其它视图一致）
async function doExport(cmd) {
  if (exporting.value) return
  if (cmd === 'pagedPrint') { printDlgVisible.value = true; return }
  exporting.value = true
  try {
    if (cmd === 'png300') {
      const r = await exportHighResPng(svg.value, contentW.value, contentH.value,
        '世代分层图-300dpi.png', '#fdfaf2', null, undefined, { treeFont: true })
      // 实际 DPI：导出按 300DPI 光栅化，但超大树会触 MAX_CANVAS_PX=14000 上限自动降倍率
      const realDpi = Math.round(r.width / Math.max(1, contentW.value) * 96)
      const capped = realDpi < 295
      ElMessage.success(
        `已导出 PNG ${r.width}×${r.height}px · 实际约 ${realDpi} DPI` +
        (capped ? '（受 14000px 上限限制已自动降倍率）' : ''))
    } else if (cmd === 'svg') {
      await exportVectorSvg(svg.value, contentW.value, contentH.value, '世代分层图-矢量.svg', null, { treeFont: true })
      ElMessage.success('已导出矢量 SVG（线条/文字无限清晰，可印刷放大）')
    } else if (cmd === 'svgFont') {
      // 「内嵌字体」档：仅当当前选中的是上传字体才子集内嵌；系统/内置字体由 treeExport 抛 FONT_NONE
      const r = await exportVectorSvg(svg.value, contentW.value, contentH.value, '世代分层图-内嵌字体.svg', null,
        { withFont: true, treeFont: true })
      const f = r.font || {}
      const kb = Math.max(1, Math.round(r.size / 1024))
      ElMessage.success(`已导出 SVG（内嵌字体 · 子集 ${f.chars || 0} 字 · ${kb}KB${f.missing ? ' · ' + f.missing + ' 字缺字形' : ''}）`)
    } else if (cmd === 'pdf') {
      await exportPrintPDF(svg.value, contentW.value, contentH.value, '世代分层图-打印.pdf', '#fdfaf2', null, { treeFont: true })
      ElMessage.success('已打开打印视图（在打印对话框选择「另存为 PDF」）')
    }
  } catch (e) {
    if (e && (e.code === 'FONT_NONE' || e.code === 'FONT_UNSUPPORTED')) ElMessage.warning(e.message)
    else ElMessage.error('导出失败：' + (e?.message || e))
  } finally {
    exporting.value = false
  }
}

function onResize() {
  if (!wrap.value) return
  svgW.value = wrap.value.clientWidth
  svgH.value = wrap.value.clientHeight
  measureTextBoxes()   // 版面变化后重新量字形盒，连线精修随之更新
}

onMounted(async () => {
  await nextTick()
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerCancel)
  window.addEventListener('resize', onResize)
  // 自测探针：节点矩形 / 配偶圈矩形 / 每层 y 与 x 区间（含 pan、zoom 变换后的屏幕坐标）
  window.__gentreeProbe = () => {
    const g = svg.value && svg.value.querySelector('g')
    const m = g ? (g.getAttribute('transform') || '') : ''
    const mm = m.match(/translate\(([-\d.]+),\s*([-\d.]+)\)\s*scale\(([-\d.]+)\)/)
    const tx = mm ? +mm[1] : 0, ty = mm ? +mm[2] : 0, sc = mm ? +mm[3] : 1
    const rect = (x, y, w, h) => ({ x: +(tx + x * sc).toFixed(2), y: +(ty + y * sc).toFixed(2), w: +(w * sc).toFixed(2), h: +(h * sc).toFixed(2) })
    const box = []
    const spBox = []
    const nodeEls = svg.value ? [...svg.value.querySelectorAll('.p-node')] : []
    nodes.value.forEach(n => {
      box.push(Object.assign({ id: n.id, gen: n.gen, label: n.chars.join(''), mark: n.mark, cx: n.cx, cy: n.y },
        rect(n.cx - R, n.y - R, 2 * R, 2 * R)))
      n.spouses.forEach(s => spBox.push(Object.assign({ id: n.id + 's', label: s.chars.join(''), cx: s.cx, cy: n.y },
        rect(s.cx - R, n.y - R, 2 * R, 2 * R))))
    })
    // 连线 vs 姓名字形 相交检测：每条正交线段与所有文字的 getBBox 做矩形相交
    const textBoxes = []
    nodeEls.forEach(el => {
      el.querySelectorAll('text').forEach(t => {
        const bb = t.getBBox()
        textBoxes.push({ id: el.getAttribute('data-mid') || el.getAttribute('data-sid'), txt: t.textContent,
                         x1: bb.x, y1: bb.y, x2: bb.x + bb.width, y2: bb.y + bb.height })
      })
    })
    // 「父侧竖线起点」是否接上本人姓名：用真实字形盒量姓名下沿，与线的起点比
    const dropStartBad = []
    const dropStartInfo = []
    connectionsRaw.value.forEach(c => {
      if (c.kind !== 'drop') return
      const el = nodeEls.find(e => +e.getAttribute('data-mid') === c.id)
      let glyphBottom = null
      if (el) {
        const ts = [...el.querySelectorAll('text')]
        ts.forEach(t => {
          const bb = t.getBBox()
          if (glyphBottom === null || bb.y + bb.height > glyphBottom) glyphBottom = bb.y + bb.height
        })
      }
      const gap = glyphBottom === null ? null : +(c.y1 - glyphBottom).toFixed(2)
      const rec = { id: c.id, label: c.label, chars: c.chars, rowNameLen: c.rowNameLen,
                    glyphBottom, lineStart: +c.y1.toFixed(1), barY: +c.y2.toFixed(1), gap }
      dropStartInfo.push(rec)
      if (gap === null || gap > 3 || gap < -1) dropStartBad.push(rec)
    })

    // 同层各「父+子女」横杠区间两两检查（要求 ≥8px 间隙，不同 y 也算分开）
    const barSegs = []
    connections.value.forEach(c => {
      if (!c.bar) return
      ;(c.segs || []).forEach(g => barSegs.push({ x1: Math.min(g[0], g[2]), x2: Math.max(g[0], g[2]), y: g[1] }))
    })
    const barBad = []
    let minGap = Infinity
    for (let i = 0; i < barSegs.length; i++) for (let j = i + 1; j < barSegs.length; j++) {
      const a = barSegs[i], b = barSegs[j]
      if (Math.abs(a.y - b.y) > 60) continue                 // 不同子行不比较
      const gap = Math.max(a.x1, b.x1) - Math.min(a.x2, b.x2)
      if (gap < minGap) minGap = gap
      if (gap < 8 - 0.01) barBad.push({ a: [a.x1, a.x2, a.y].map(v => +v.toFixed(1)), b: [b.x1, b.x2, b.y].map(v => +v.toFixed(1)), gap: +gap.toFixed(2) })
    }

    const segLen = []
    connectionsRaw.value.forEach(c => {
      if (c.kind !== 'bar') return
      segLen.push({
        aboveGap: +(c.childTopY - c.y).toFixed(1),               // 横杠 → 子圈顶
        nameDrop: +(c.y - c.nameBottom).toFixed(1),              // 姓名块下沿 → 横杠
        nameBottom: +c.nameBottom.toFixed(1),
        blockBottom: +(c.nameBottom + 16).toFixed(1),   // 预留下沿（含姓名上方 16px 间距）
        barY: +c.y.toFixed(1), rowTop: +c.rowTop.toFixed(1),
        rowNameLen: c.rowNameLen, parentLabel: c.parentLabel
      })
    })

    // 连线连续性（逐条父子连线）：
    //   ① 折点处相邻两段首尾相接（端点距离 = 0）—— 按 path 检查
    //   ② 终点 ↔ 子圈顶 ≤1px；③ 起点 ↔ 父姓名字形下沿 ≤3px
    //   ④ 断头/悬空端点 = 0（每个端点都必须落在「折点 / 子圈顶 / 父字形下沿 / 圈缘」上）
    const glyphInkBottom = n => n.y + R + 16 + (n.chars.length - 1) * NAME_LH + 5 + 0.5
    let jointBad = 0
    const segsAll = []
    connections.value.forEach(c => {
      const ss = (c.segs || []).map(g => [g[0], g[1], g[2], g[3]])
      for (let i = 1; i < ss.length; i++) {
        const a = ss[i - 1], b = ss[i]
        if (Math.hypot(a[2] - b[0], a[3] - b[1]) > 0.01) jointBad++
      }
      ss.forEach(g => segsAll.push(g))
    })
    // ④ 端点归类
    const endKey = (x, y) => `${x.toFixed(1)},${y.toFixed(1)}`
    const touchTargets = new Set()
    nodes.value.forEach(n => {
      touchTargets.add(endKey(n.cx, n.y - R))                                  // 子圈顶
      touchTargets.add(endKey(n.cx, n.y + R))                                  // 圈底
      touchTargets.add(endKey(n.cx - R, n.y)); touchTargets.add(endKey(n.cx + R, n.y))
      touchTargets.add(endKey(n.cx, glyphInkBottom(n)))                        // 父字形下沿（线起点）
      n.spouses.forEach(sp => {
        touchTargets.add(endKey(sp.cx, n.y - R)); touchTargets.add(endKey(sp.cx, n.y + R))
        touchTargets.add(endKey(sp.cx - R, n.y)); touchTargets.add(endKey(sp.cx + R, n.y))
      })
    })
    const deg = new Map()
    const bump = (x, y) => { const k = endKey(x, y); deg.set(k, (deg.get(k) || 0) + 1) }
    segsAll.forEach(g => { bump(g[0], g[1]); bump(g[2], g[3]) })
    // 端点是否落在另一条线段上（T 形接头：如子侧竖线起点落在横杠上）
    const onOtherSeg = (x, y, self) => segsAll.some(g => {
      if (g === self) return false
      const [x1, y1, x2, y2] = g
      if (Math.abs(x1 - x2) < 0.01) {                       // 竖段（端点含在内）
        return Math.abs(x - x1) < 0.6 && y >= Math.min(y1, y2) - 0.6 && y <= Math.max(y1, y2) + 0.6
      }
      if (Math.abs(y1 - y2) < 0.01) {                       // 横段（端点含在内）
        return Math.abs(y - y1) < 0.6 && x >= Math.min(x1, x2) - 0.6 && x <= Math.max(x1, x2) + 0.6
      }
      return false
    })
    let freeEnd = 0
    const freeList = []
    segsAll.forEach(g => {
      ;[[g[0], g[1]], [g[2], g[3]]].forEach(([x, y]) => {
        const k = endKey(x, y)
        const isJoint = (deg.get(k) || 0) >= 2        // 与其他段共端点
        const isTarget = touchTargets.has(k)          // 贴圈缘/圈顶底/父字形下沿
        const isT = onOtherSeg(x, y, g)               // 落在另一条线段上（T 型接头）
        if (!isJoint && !isTarget && !isT) { freeEnd++; if (freeList.length < 6) freeList.push({ x: +x.toFixed(1), y: +y.toFixed(1) }) }
      })
    })
    // ② 每条 riser 的终点必须贴某个子圈顶；③ 每条 drop 的起点贴父字形下沿
    let endFar = 0, startFar = 0
    const badPairs = []
    connections.value.forEach(c => {
      if (c.kind === 'riser') {
        const dy = Math.abs(c.y2 - (c.y2))   // 占位，实际按子圈顶比对
        const childOf = nodes.value.find(n => Math.abs(n.cx - c.x) < 0.6 && Math.abs((n.y - R) - c.y2) < 1.5)
        if (!childOf) { endFar++; if (badPairs.length < 5) badPairs.push({ kind: 'riser', x: c.x, yEnd: c.y2 }) }
      }
      if (c.kind === 'drop') {
        const owner = nodes.value.find(n => n.id === c.id)
        if (owner && Math.abs(c.y1 - glyphInkBottom(owner)) > 3) { startFar++; if (badPairs.length < 8) badPairs.push({ kind: 'drop', who: owner.chars.join('') }) }
      }
    })

    // 姓名文本框 × 姓名文本框 相交检测（含配偶姓名）——每个节点的姓名块 = 本人名+配偶名的 union
    const nameBoxes = []
    nodeEls.forEach(el => {
      const ts = []
      el.querySelectorAll('text').forEach(t => {
        if (['失', '入', '出', '赘', '止', '女', '配'].includes(t.textContent) && t.textContent.length === 1) return
        const bb = t.getBBox()
        ts.push({ x1: bb.x, y1: bb.y, x2: bb.x + bb.width, y2: bb.y + bb.height })
      })
      if (!ts.length) return
      const u = {
        id: el.getAttribute('data-mid') || el.getAttribute('data-sid'),
        x1: Math.min(...ts.map(t => t.x1)), y1: Math.min(...ts.map(t => t.y1)),
        x2: Math.max(...ts.map(t => t.x2)), y2: Math.max(...ts.map(t => t.y2))
      }
      if (u.x2 - u.x1 < 2) return          // 「女」「配」等单字已排除，这里只对比真正的姓名块
      nameBoxes.push(u)
    })
    const nameBad = []
    for (let i = 0; i < nameBoxes.length; i++) for (let j = i + 1; j < nameBoxes.length; j++) {
      const a = nameBoxes[i], b = nameBoxes[j]
      const xo = Math.min(a.x2, b.x2) - Math.max(a.x1, b.x1)
      const yo = Math.min(a.y2, b.y2) - Math.max(a.y1, b.y1)
      if (xo > 0.5 && yo > 0.5) nameBad.push({ a: a.id, b: b.id, ox: +xo.toFixed(1), oy: +yo.toFixed(1) })
    }

    const segAll = []
    connections.value.forEach(c => (c.segs || []).forEach(g => segAll.push(g)))
    const cross = []
    segAll.forEach(g => {
      const [x1, y1, x2, y2] = g
      textBoxes.forEach(t => {
        const xo = Math.min(x1, x2) < t.x2 - 0.5 && Math.max(x1, x2) > t.x1 + 0.5
        const yo = Math.min(y1, y2) < t.y2 - 0.5 && Math.max(y1, y2) > t.y1 + 0.5
        if (xo && yo) cross.push({ seg: [x1, y1, x2, y2].map(v => +v.toFixed(1)), text: t.txt, owner: t.id })
      })
    })

    // 圈内文字是否溢出：对每个圈内的 text 取 getBBox → 外接圆半径 <= 圆内可用半径(R-边框)
    const avail = R - 0.5
    const textFit = []
    nodeEls.forEach(el => {
      const ci = el.querySelector('circle')
      const cx = +ci.getAttribute('cx'), cy = +ci.getAttribute('cy'), r = +ci.getAttribute('r')
      el.querySelectorAll('text').forEach(t => {
        const bb = t.getBBox()
        const cxc = bb.x + bb.width / 2, cyc = bb.y + bb.height / 2
        const maxD = Math.max(
          Math.hypot(bb.x - cx, bb.y - cy), Math.hypot(bb.x + bb.width - cx, bb.y - cy),
          Math.hypot(bb.x - cx, bb.y + bb.height - cy), Math.hypot(bb.x + bb.width - cx, bb.y + bb.height - cy),
          Math.hypot(cxc - cx, cyc - cy)
        )
        // 仅校验圈内文字（排行/女/标注）：竖直姓名在圈下方，不参与
        const inCircle = Math.abs(cyc - cy) <= r + 2
        if (inCircle) {
          // getBBox 的高度含 1.2 倍行高留白，非真实墨迹；这里再按「墨迹行高 ≈ 字号」
          // 与单字宽 ≈ 字号 ×0.95 估一个保守的墨迹框，两者都要在圆内
          const fs = +(t.getAttribute('font-size') || 0)
          const inkW = fs * 0.98 * t.textContent.length
          const inkH = fs * 1.05
          const inkD = Math.hypot(inkW / 2, inkH / 2 + (cyc - cy < 0 ? 0 : 0)) + Math.abs((bb.y + bb.height / 2) - cy) * 0.0
          textFit.push({
            id: el.getAttribute('data-mid') || el.getAttribute('data-sid'),
            txt: t.textContent, fs, maxD: +maxD.toFixed(2), r,
            bb: [+bb.x.toFixed(2), +bb.y.toFixed(2), +bb.width.toFixed(2), +bb.height.toFixed(2)],
            cxy: [cx, cy],
            boxOk: maxD <= r - 0.5,
            inkD: +inkD.toFixed(2),
            inkOk: inkD <= r - 0.5,
            ok: inkD <= r - 0.5
          })
        }
      })
    })
    // 整树靠左：每层最左节点的圈左缘 x（应都等于左边距）、每层 x 跨度
    const leftInfo = rows.value.map(r => {
      const inRow = nodes.value.filter(n => n.rowTop === r.rowTop)
      if (!inRow.length) return { gen: r.gen, count: 0 }
      const lefts = inRow.map(n => tx + (n.cx - R) * sc)
      const rights = inRow.map(n => tx + (n.right) * sc)
      return { gen: r.gen, count: inRow.length,
               minLeft: +Math.min(...lefts).toFixed(2), maxRight: +Math.max(...rights).toFixed(2) }
    })
    // 父 x 是否等于其最左子女 x
    const alignInfo = []
    let alignBad = 0
    nodes.value.forEach(p => {
      const ks = nodes.value.filter(n => (n.m.father_id || 0) === p.id)
      if (!ks.length) return
      const minKid = Math.min(...ks.map(k => k.cx - R))
      const rec = { parent: p.chars.join(''), parentX: +p.cx.toFixed(2), leftKidX: +(minKid + R).toFixed(2),
                    diff: +(p.cx - minKid - R).toFixed(2) }
      alignInfo.push(rec)
      // 容忍「为避开姓名重叠被整体右推」的少数列（阈值 = 一列最小间距 2R+AX_GAP）
      if (Math.abs(rec.diff) > 2 * R + 8) alignBad++
    })

    const rowsOut = rows.value.map(r => {
      const inRow = nodes.value.filter(n => n.rowTop === r.rowTop)
      const xs = inRow.map(n => n.cx)
      return {
        gen: r.gen,
        y: +(ty + r.y * sc).toFixed(2),
        count: inRow.length,
        minX: xs.length ? +(tx + (Math.min(...xs) - R) * sc).toFixed(2) : null,
        maxX: xs.length ? +(tx + (Math.max(...xs) + R) * sc).toFixed(2) : null
      }
    })
    return {
      scale: sc, tx, ty, loadMs,
      memberCount: members.value.length,
      nodeCount: nodes.value.length,
      spouseCount: spBox.length,
      spouseTotal: members.value.reduce((a, m) => a + ((m.spouses || []).length), 0),
      rowCount: rowsOut.length,
      emptyRows: rowsOut.filter(r => r.count === 0).length,
      contentW: contentW.value, contentH: contentH.value,
      marks: marks.value,
      rowDebug: rows.value.slice(0, 6).map(r => ({ gen: r.gen, rowTop: r.rowTop, rowH: r.rowH, tag: r.tagChars.join('') })),
      multiSpouse: nodes.value.filter(n => n.spouses.length > 1).length,
      circleR: R,
      markChars: [...new Set([...nodeEls].map(el => {
        const ts = [...el.querySelectorAll('text')]
        const t = ts.find(x => ['失', '入', '出', '赘', '止'].includes(x.textContent) && x.textContent.length === 1)
        return t ? t.textContent : ''
      }).filter(Boolean))],
      segCount: segAll.length,
      continuity: { jointBad, endFar, startFar, freeEnd, freeList, badPairs, segCount: segsAll.length },
      nameBoxCount: nameBoxes.length,
      nameOverlapCount: nameBad.length,
      nameOverlap: nameBad.slice(0, 8),
      barCount: barSegs.length,
      barGapBad: barBad,
      barGapBadCount: barBad.length,
      barGapMin: minGap === Infinity ? null : +minGap.toFixed(2),
      dropStartBadCount: dropStartBad.length,
      dropStartBad,
      dropStartInfo,
      segLen,
      leftInfo,
      alignBadCount: alignBad,
      alignSample: alignInfo.slice(0, 12),
      lineTextCross: cross,
      lineTextCrossCount: cross.length,
      textFitCount: textFit.length,
      textFitBad: textFit.filter(t => !t.ok),
      textFit,
      boxes: box, spouseBoxes: spBox, rows: rowsOut
    }
  }
  // 自测/联调用：切换房支与「含下级」（等价于工具栏下拉）
  window.__gentreeSetBranch = (id, includeChildren) => {
    if (includeChildren !== undefined) setIncludeChildren(!!includeChildren)
    onBranchChange(id)
  }
  await load()
})

onBeforeUnmount(() => {
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerCancel)
  window.removeEventListener('resize', onResize)
  delete window.__gentreeProbe
  delete window.__gentreeSetBranch
})
</script>

<style scoped>
/* 树图字体选择：作用到本视图 SVG 里的文字（CSS 规则优先于 <text> 的 font-family 表现属性） */
svg.tf-on text, svg.tf-on tspan { font-family: var(--tree-font); }
.gentree { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.title { font-size: 18px; margin-right: 8px; font-family: "Songti SC", "SimSun", serif; color: #3a2a18; }
.hint-inline { color: #8c5a3c; font-size: 12px; }
.hint { margin-left: auto; color: #8c5a3c; font-size: 12px; }
@media (max-width: 767px) { .hint { display: none; } }
.canvas-wrap { flex: 1; overflow: hidden; border: 1px solid var(--border); border-radius: 4px;
  cursor: grab; touch-action: none; user-select: none; background: #fdfaf2; }
.gen-svg { display: block; }
.p-node { cursor: pointer; }
.p-node:hover circle { filter: brightness(0.94); }
.p-mark { cursor: pointer; }
.tip-card .row { display: flex; margin-bottom: 8px; }
.tip-card .label { width: 84px; color: #8c5a3c; flex-shrink: 0; }
</style>
