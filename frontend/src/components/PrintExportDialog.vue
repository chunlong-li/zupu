<template>
  <el-dialog
    :model-value="modelValue"
    title="打印 / 导出世系图（A3/A4 分页 · 卷轴）"
    width="860px"
    top="4vh"
    append-to-body
    class="print-paging-dialog"
    @update:model-value="v => emit('update:modelValue', v)"
    @open="onOpen"
    @closed="onClosed"
  >
    <div class="pp-body">
      <div class="pp-row">
        <span class="pp-lab">纸张</span>
        <el-radio-group v-model="opts.paper" size="small" :disabled="opts.mode === 'scroll'">
          <el-radio-button value="A4">A4</el-radio-button>
          <el-radio-button value="A3">A3</el-radio-button>
        </el-radio-group>
        <span class="pp-lab">方向</span>
        <el-radio-group v-model="opts.orientation" size="small" :disabled="opts.mode === 'scroll'">
          <el-radio-button value="landscape">横版</el-radio-button>
          <el-radio-button value="portrait">竖版</el-radio-button>
        </el-radio-group>
      </div>

      <div class="pp-row">
        <span class="pp-lab">方式</span>
        <el-radio-group v-model="opts.mode" size="small">
          <el-radio-button value="single">整张一页</el-radio-button>
          <el-radio-button value="tile">分页（瓦片）</el-radio-button>
          <el-radio-button value="scroll">卷轴（超长单页）</el-radio-button>
        </el-radio-group>
      </div>

      <div v-if="opts.mode === 'tile'" class="pp-row">
        <span class="pp-lab">重叠</span>
        <el-input-number v-model="opts.overlapMm" :min="0" :max="40" :step="1" size="small" style="width:110px" />
        <span class="pp-unit">mm</span>
        <span class="pp-tip">默认 12mm：相邻两张在切口处重叠，跨缝的连接线两张都能看到</span>
      </div>

      <div v-if="opts.mode === 'tile'" class="pp-row">
        <span class="pp-lab">含索引页</span>
        <el-switch v-model="opts.includeIndex" size="small" />
        <span class="pp-tip">索引页放在第一页：整图缩略 + 网格坐标（A1..） + 每格页码与世代范围</span>
      </div>

      <div v-if="opts.mode === 'tile'" class="pp-row">
        <span class="pp-lab">内容比例</span>
        <el-select v-model="scaleSel" size="small" style="width:220px">
          <el-option label="自动 · 切满纸面（推荐）" value="auto-save" />
          <el-option label="自动 · 切满且字更大" value="auto-big" />
          <el-option label="50%（更小 · 张数更少）" :value="50" />
          <el-option label="75%" :value="75" />
          <el-option label="100%（屏幕 1:1）" :value="100" />
          <el-option label="125%" :value="125" />
          <el-option label="150%（大字版）" :value="150" />
        </el-select>
        <span v-if="plan && plan.mode === 'tile'" class="pp-tip">
          实际采用 <b class="pp-pct">{{ plan.scalePct.toFixed(1) }}%</b>
          <template v-if="plan.auto">（自动：{{ plan.auto.target === 'big' ? '切满且字更大' : '切满纸面' }}）</template>
          <template v-else-if="plan.snappedFrom">（{{ plan.snappedFrom.toFixed(1) }}% → 微调到整页）</template>
        </span>
      </div>

      <div v-if="opts.mode === 'scroll'" class="pp-row">
        <span class="pp-lab">卷轴尺寸</span>
        <el-select v-model="opts.scrollPreset" size="small" style="width:330px">
          <el-option v-for="p in SCROLL_PRESETS" :key="p.value" :label="p.label" :value="p.value" />
        </el-select>
        <span class="pp-lab">流向</span>
        <el-radio-group v-model="opts.scrollAxis" size="small">
          <el-radio-button value="v">纵向（世代向下）</el-radio-button>
          <el-radio-button value="h">横向</el-radio-button>
        </el-radio-group>
      </div>

      <el-alert v-if="opts.mode === 'scroll'" :title="SCROLL_HINT" type="warning" :closable="false" show-icon class="pp-hint" />

      <div class="pp-summary">
        <div class="pp-est">
          <b>{{ estimateText }}</b>
          <span v-if="plan && plan.mode === 'tile'" class="pp-sub">
            图页 {{ plan.mapPageCount }} 张<template v-if="plan.indexPage"> + 索引页 1 张</template>
          </span>
        </div>
        <div v-if="plan" class="pp-meta">
          <span>版面 {{ plan.metrics.paperW }}×{{ plan.metrics.paperH }}mm · 纸面可用 {{ plan.metrics.areaW }}×{{ plan.metrics.areaH }}mm</span>
          <span v-if="plan.mode === 'tile'">网格 {{ plan.cols }} 列 × {{ plan.rows }} 行 · 重叠 {{ plan.overlapMmEff }}mm · 1px = {{ plan.mmPerPx.toFixed(3) }}mm</span>
          <span v-else-if="plan.mode === 'scroll'">单页 {{ plan.scroll.width }} × {{ plan.scroll.length }}mm · 比例 {{ plan.scalePct }}%</span>
          <span v-else>整张缩放到一页 · 比例 {{ plan.scalePct }}%</span>
          <span>切页只落在节点空隙（{{ boxes.length }} 个节点/文字包围盒参与避让）</span>
        </div>
        <div v-if="plan && plan.mode === 'tile'" class="pp-fill">
          <span class="pp-fill-item"><i>内容尺寸</i><b>{{ plan.contentWmm }} × {{ plan.contentHmm }} mm</b></span>
          <span class="pp-fill-item"><i>内容÷纸张（整体）</i><b>{{ (plan.coverage * 100).toFixed(1) }}%</b></span>
          <span class="pp-fill-item"><i>最右列空白</i><b>{{ plan.blankRightMm }} mm（{{ ((plan.blankRightMm / plan.metrics.areaW) * 100).toFixed(1) }}%）</b></span>
          <span class="pp-fill-item"><i>最下行空白</i><b>{{ plan.blankBottomMm }} mm（{{ ((plan.blankBottomMm / plan.metrics.areaH) * 100).toFixed(1) }}%）</b></span>
          <span class="pp-fill-item"><i>最后一页与相邻页重叠</i><b>{{ Math.max(plan.lastOverlapXmm || 0, plan.lastOverlapYmm || 0).toFixed(1) }} mm</b></span>
        </div>
        <div v-if="plan && plan.mode === 'tile'" class="pp-fill-note">
          含重叠 = 这张纸被内容占满的比例（含与邻页重复的那一条）；独有内容 = 第一次出现在这张纸上的内容比例。
          <template v-if="fullExceptLast">本方案除最后一页外全部 ≥ 99%。</template>
          <template v-else>本方案有 {{ notFullPages.length }} 张低于 99%：{{ notFullPages.join('、') }}（多数是「该比例下内容不够宽/最后一页回借」造成的，见下方最后一页说明）。</template>
        </div>
        <div v-if="plan && plan.auto" class="pp-auto">
          推导：内容 {{ plan.contentWmm }}×{{ plan.contentHmm }}mm ÷ 纸面 {{ plan.metrics.areaW }}×{{ plan.metrics.areaH }}mm
          → 取 <b>{{ plan.cols }} 列 × {{ plan.rows }} 行</b>、比例 <b>{{ plan.scalePct.toFixed(1) }}%</b>
          → 每张覆盖 <b>{{ (plan.coverage * 100).toFixed(1) }}%</b>（{{ plan.mapPageCount }} 张图页 + 索引 1 张）。
          <template v-if="plan.auto.alt">
            同样切满的另一种：{{ plan.auto.alt.pct.toFixed(1) }}% → {{ plan.auto.alt.pages }} 张图页（覆盖 {{ (plan.auto.alt.coverage * 100).toFixed(1) }}%）。
          </template>
          <template v-if="plan.auto.target === 'save'">
            如需更大的字，可切到「自动 · 切满且字更大」或直接选 100% / 125%。
          </template>
        </div>
      </div>

      <el-alert v-for="(w, i) in (plan ? plan.warnings : [])" :key="i" :title="w" type="warning" :closable="false" show-icon class="pp-warn" />

      <div v-if="plan && plan.mode === 'tile'" class="pp-grid-wrap">
        <div class="pp-grid-title">页面网格（索引页在最前；阅读顺序：行自上而下、行内自左而右）</div>
        <div class="pp-grid" :style="{ gridTemplateColumns: `64px repeat(${plan.cols}, minmax(42px, 1fr))` }">
          <div class="pp-ix" :style="{ gridRow: `1 / span ${plan.rows}` }">
            <b>索引</b><span>第 1 张</span>
          </div>
          <template v-for="p in plan.mapPages" :key="p.pageNo">
            <div class="pp-cell" :class="{ 'is-first': p.pageNo === 2 }" :style="{ gridColumn: p.col + 1, gridRow: p.row }" :title="`第 ${p.pageNo} 张 · ${p.grid} · ${p.genFrom}~${p.genTo}`">
              <b>{{ p.grid }}</b><span>P{{ p.pageNo }}</span>
            </div>
          </template>
        </div>
      </div>

      <div v-if="plan && plan.mode === 'tile'" class="pp-table-wrap">
        <table class="pp-table">
          <thead><tr><th>张</th><th>格</th><th>行/列</th><th>含重叠</th><th>独有内容</th><th>本张世代范围</th><th>承上/接下</th></tr></thead>
          <tbody>
            <tr v-for="p in plan.mapPages" :key="'r' + p.pageNo" :class="{ 'is-full': p.fill && p.fill.area >= 0.99, 'is-last': p.pageNo === plan.totalPages }">
              <td>第 {{ p.pageNo }} 张<template v-if="p.anchored"> ·末</template></td>
              <td class="gc">{{ p.grid }}</td>
              <td>{{ p.row }}/{{ p.rowCount }} · {{ p.col }}/{{ p.colCount }}</td>
              <td :class="p.fill && p.fill.area >= 0.99 ? 'ok' : 'warn'">{{ p.fill ? (p.fill.area * 100).toFixed(1) + '%' : '—' }}</td>
              <td>{{ p.fill ? (p.fill.unique * 100).toFixed(1) + '%' : '—' }}</td>
              <td>{{ p.genFrom || '—' }}<template v-if="p.genTo && p.genTo !== p.genFrom"> ~ {{ p.genTo }}</template></td>
              <td class="seam">
                <span v-if="p.seam.left">←{{ p.seam.left.pageNo }}</span>
                <span v-if="p.seam.right">{{ p.seam.right.pageNo }}→</span>
                <span v-if="p.seam.top">↑{{ p.seam.top.pageNo }}</span>
                <span v-if="p.seam.bottom">↓{{ p.seam.bottom.pageNo }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="plan && plan.mode !== 'tile'" class="pp-single">
        <span v-if="plan.mode === 'scroll'">整份内容 → 1 张超长单页（{{ plan.scroll.width }} × {{ plan.scroll.length }} mm）</span>
        <span v-else>整份内容缩放到 1 张 {{ opts.paper }} {{ opts.orientation === 'portrait' ? '竖版' : '横版' }}</span>
      </div>
    </div>

    <template #footer>
      <span class="pp-foot-tip">屏幕预览不受影响；点「打印」后在新窗口里选「另存为 PDF」即得矢量 PDF</span>
      <el-button size="small" @click="emit('update:modelValue', false)">取消</el-button>
      <el-button size="small" type="primary" :loading="working" @click="doPrint">打印 / 另存 PDF</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import {
  DEFAULT_OPTS, SCROLL_PRESETS, SCROLL_HINT, planTitle
} from '../utils/printPaging'
import { collectTreeBoxes, planPagedPrint, openPagedPrint } from '../utils/treeExport'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  svgEl: { type: Object, default: null },
  contentW: { type: Number, default: 0 },
  contentH: { type: Number, default: 0 },
  rows: { type: Array, default: () => [] },
  bounds: { type: Object, default: null },
  title: { type: String, default: '世系图' },
  volume: { type: String, default: '' },
  theme: { type: Object, default: () => ({ bg: '#ffffff', ink: '#3a2a18' }) }
})
const emit = defineEmits(['update:modelValue'])

const opts = reactive({ ...DEFAULT_OPTS, scrollAxis: 'v' })
// 内容比例：自动（auto-save / auto-big）或钉死百分比
const scaleSel = computed({
  get: () => (opts.scaleMode === 'fixed' ? opts.scalePct : opts.scaleMode),
  set: (v) => {
    if (typeof v === 'number') { opts.scaleMode = 'fixed'; opts.scalePct = v }
    else { opts.scaleMode = v; opts.scalePct = 100 }
  }
})
const boxes = ref([])
const plan = ref(null)
const working = ref(false)
const familyName = ref('')
const collecting = ref(false)

// 逐页满页情况（含重叠 ≥ 99% 视为满页）
const fullExceptLast = computed(() => {
  const p = plan.value
  if (!p || p.mode !== 'tile') return false
  return p.mapPages.slice(0, -1).every(x => x.fill && x.fill.area >= 0.99)
})
const notFullPages = computed(() => {
  const p = plan.value
  if (!p || p.mode !== 'tile') return []
  return p.mapPages.filter(x => x.fill && x.fill.area < 0.99).map(x => `第 ${x.pageNo} 张 ${(x.fill.area * 100).toFixed(0)}%`)
})

const estimateText = computed(() => {
  if (!plan.value) return '正在计算…'
  const p = plan.value
  if (p.mode === 'scroll') {
    return `卷轴单页 ${p.scroll.width} × ${p.scroll.length} mm（1 张）`
  }
  const label = `${p.opts.paper} ${p.opts.orientation === 'portrait' ? '竖版' : '横版'}`
  return `预计 ${label} ${p.totalPages} 张`
})

function recompute() {
  if (!props.svgEl) { plan.value = null; return }
  const bounds = props.bounds && props.bounds.w > 0 ? props.bounds : null
  plan.value = planPagedPrint({
    contentW: props.contentW,
    contentH: props.contentH,
    boxes: boxes.value,
    rows: props.rows,
    bounds,
    opts: { ...opts }
  })
}

async function onOpen() {
  working.value = false
  plan.value = null
  boxes.value = []
  collecting.value = true
  try {
    if (!familyName.value) {
      try {
        const { data } = await api.get('/settings/family-name')
        familyName.value = data.family_name || ''
      } catch (e) { /* 取不到就不显示谱名，不影响打印 */ }
    }
    await nextTick()
    boxes.value = collectTreeBoxes(props.svgEl)
  } finally {
    collecting.value = false
    recompute()
    // 自测钩子：只读暴露当前方案（自动化验证脚本读取切页线/张数/世代范围）
    try {
      window.__zupuPrintPlan = () => ({
        plan: plan.value,
        boxes: boxes.value,
        meta: buildMeta()
      })
    } catch (e) { /* ignore */ }
  }
}

function onClosed() {
  try { delete window.__zupuPrintPlan } catch (e) { window.__zupuPrintPlan = null }
}

watch(() => [opts.paper, opts.orientation, opts.mode, opts.overlapMm, opts.includeIndex, opts.scaleMode, opts.scalePct, opts.scrollPreset, opts.scrollAxis],
  () => recompute())

function buildMeta() {
  return {
    familyName: familyName.value,
    viewTitle: props.title,
    volume: props.volume,
    title: `${familyName.value || ''}${familyName.value ? ' · ' : ''}${props.title} 打印稿`,
    filename: `${familyName.value || '族谱'}-${props.title}-${opts.paper}-${opts.orientation === 'portrait' ? '竖版' : '横版'}-${opts.mode}.pdf`,
    bg: props.theme.bg || '#ffffff',
    ink: props.theme.ink || '#3a2a18'
  }
}

async function doPrint() {
  if (!plan.value || working.value) return
  if (plan.value.mode === 'tile' && plan.value.mapPageCount === 0) {
    ElMessage.warning('当前没有可打印的内容')
    return
  }
  working.value = true
  try {
    const html = await openPagedPrint(props.svgEl, plan.value, buildMeta())
    void html
    ElMessage.success(`已打开打印视图（${estimateText.value.replace('预计 ', '')}）`)
  } catch (e) {
    ElMessage.error('生成打印稿失败：' + (e && e.message ? e.message : e))
  } finally {
    working.value = false
  }
}
</script>

<style scoped>
.pp-body { display: flex; flex-direction: column; gap: 10px; }
.pp-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.pp-lab { color: #5a4632; font-size: 13px; min-width: 56px; }
.pp-unit { color: #8c5a3c; font-size: 12px; margin-left: -4px; }
.pp-tip { color: #8a7a64; font-size: 12px; }
.pp-hint { margin-top: 2px; }
.pp-summary { background: #faf6ec; border: 1px solid #e6dcc6; border-radius: 6px; padding: 8px 10px; }
.pp-est { display: flex; align-items: baseline; gap: 10px; }
.pp-est b { font-size: 17px; color: #8c2b1e; }
.pp-sub { color: #7a6852; font-size: 12px; }
.pp-meta { display: flex; flex-wrap: wrap; gap: 4px 16px; color: #6b5a44; font-size: 12px; margin-top: 4px; }
.pp-fill { display: flex; flex-wrap: wrap; gap: 4px 18px; margin-top: 6px; padding-top: 6px; border-top: 1px dashed #e0d2b4; }
.pp-fill-item { display: flex; align-items: baseline; gap: 5px; font-size: 12px; }
.pp-fill-item i { font-style: normal; color: #8a7a64; }
.pp-fill-item b { color: #8c2b1e; font-size: 13px; }
.pp-fill-note { margin-top: 4px; font-size: 11.5px; color: #8a7a64; line-height: 1.5; }
.pp-auto { margin-top: 6px; font-size: 12px; color: #6b5a44; line-height: 1.6; }
.pp-table td.ok { color: #2e7d32; font-weight: 600; }
.pp-table td.warn { color: #b26a00; font-weight: 600; }
.pp-table tr.is-last td { background: #fff8e6; }
.pp-auto b { color: #8c2b1e; }
.pp-pct { color: #8c2b1e; }
.pp-warn { margin-top: -2px; }
.pp-grid-wrap { border: 1px solid #eee3cf; border-radius: 6px; padding: 8px; }
.pp-grid-title { font-size: 12px; color: #7a6852; margin-bottom: 6px; }
.pp-grid { display: grid; gap: 4px; align-items: stretch; }
.pp-ix { background: #8c2b1e; color: #fdf6e6; border-radius: 4px; display: flex; flex-direction: column; align-items: center; justify-content: center; font-size: 11px; padding: 4px; }
.pp-ix b { font-size: 13px; }
.pp-cell { border: 1px solid #d9c8a8; border-radius: 4px; padding: 3px 4px; font-size: 11px; display: flex; flex-direction: column; align-items: center; color: #5a4632; background: #fffdf7; }
.pp-cell b { font-size: 12px; color: #8c2b1e; }
.pp-cell span { color: #8a7a64; }
.pp-cell.is-first { border-color: #c9a227; box-shadow: 0 0 0 1px #c9a227 inset; }
.pp-table-wrap { max-height: 190px; overflow: auto; border: 1px solid #eee3cf; border-radius: 6px; }
.pp-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.pp-table th, .pp-table td { border-bottom: 1px solid #f0e8d8; padding: 3px 8px; text-align: left; white-space: nowrap; }
.pp-table th { position: sticky; top: 0; background: #f7f1e3; color: #5a4632; z-index: 1; }
.pp-table td.gc { font-weight: 700; color: #8c2b1e; }
.pp-table td.seam { color: #8c5a3c; letter-spacing: 2px; }
.pp-single { color: #6b5a44; font-size: 13px; padding: 4px 2px; }
.pp-foot-tip { float: left; color: #8a7a64; font-size: 12px; line-height: 30px; }
</style>
