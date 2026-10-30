<template>
  <div class="merit-page">
    <div class="page-head no-print">
      <h2>🪦 功德碑</h2>
      <span class="hint">取自「捐款账簿」已审核的捐款 · 门槛筛选 · 传统竖排右起 · 可导出碑刻稿</span>
      <div class="head-actions">
        <el-button type="primary" :disabled="!donationBooks.length || !canManage" @click="openNew">＋ 新增功德碑</el-button>
        <el-button :disabled="!current || !canManage" @click="openEdit">✎ 编辑</el-button>
        <el-button type="danger" plain :disabled="!current || !canManage" @click="delStele">删除</el-button>
      </div>
    </div>

    <el-alert v-if="loaded && !donationBooks.length" type="info" show-icon :closable="false"
      title="还没有「捐款账簿」" class="no-print">
      <div style="margin-top:4px">
        请到「💰 收支公示」→ 新建账本时把「账本类型」选为 <b>捐款账簿</b>，把捐款单独记账；此处即可生成功德碑。
      </div>
    </el-alert>

    <el-alert v-else-if="loaded && !steles.length" type="info" show-icon :closable="false"
      title="还没有功德碑" class="no-print">
      <div style="margin-top:4px">
        点右上角 <b>「＋ 新增功德碑」</b> 创建；可保存多座，顶部标签随时切换，默认显示最新一座。
      </div>
    </el-alert>

    <template v-else-if="steles.length">
      <!-- 切换标签：最新在前 -->
      <div class="tabs no-print">
        <button v-for="s in steles" :key="s.id" class="tab" :class="{ active: s.id === currentId }" @click="select(s.id)">
          <span class="tab-name">{{ s.name || '功德碑' }}</span>
          <span class="tab-sub" v-if="s.book_title">{{ s.book_title }}</span>
        </button>
      </div>

      <div class="toolbar no-print">
        <div class="stat">
          共 <b>{{ donors.length }}</b> 位 · 捐款合计 <b>¥{{ grandTotal }}{{ mNumeral === 'chinese' ? '（' + toChineseNumber(grandTotal) + '）' : '' }}</b>
          <span v-if="anon.count">（含匿名 {{ anon.count }} 人 ¥{{ anon.total }}）</span>
        </div>
        <div class="export">
          <el-dropdown trigger="click" @command="doExport">
            <el-button :loading="exporting">导出<el-icon class="el-icon--right"><arrow-down /></el-icon></el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="png300">🖼️ PNG（高清 · 300DPI）</el-dropdown-item>
                <el-dropdown-item command="svg">📐 SVG（矢量图）</el-dropdown-item>
                <el-dropdown-item command="svgfont">📐 SVG（内嵌字体）</el-dropdown-item>
                <el-dropdown-item command="pdf">🖨️ PDF（打印用）</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>

      <div class="stage">
        <div class="zoom-wrap">
          <div ref="steleRef" class="stele" :class="['stele-pal-' + styleKey, 'stele-bd-' + borderKey]">
            <div class="stele-inner">
              <div class="bian-e">{{ title }}</div>
              <div v-if="!donors.length && !anon.count" class="empty">暂无达到门槛的捐款记录</div>
              <template v-else>
                <div v-if="donors.length" class="names">
                  <div v-for="(col, ci) in columns" :key="ci" class="col">
                    <div v-for="(d, di) in col" :key="di" class="donor">
                      <span class="dn">{{ d.name }}</span>
                      <span v-if="mShowAmount" class="da">{{ fmtAmount(d.amount) }}</span>
                    </div>
                  </div>
                </div>
                <div v-if="anonText" class="anon-line">{{ anonText }}</div>
              </template>
              <div v-if="mFooter" class="liukuan">{{ mFooter }}</div>
            </div>
          </div>
        </div>
      </div>
    </template>

    <!-- 新增 / 编辑 弹窗 -->
    <el-dialog v-model="dlg.show" :title="dlg.editing ? '编辑功德碑' : '新增功德碑'" width="520px" class="no-print"
      :close-on-click-modal="false" append-to-body>
      <el-form label-width="92px" label-position="left">
        <el-form-item label="碑名">
          <el-input v-model="dlg.form.name" maxlength="40" placeholder="用于顶部标签，留空自动取碑额标题" />
        </el-form-item>
        <el-form-item label="捐款账簿" required>
          <el-select v-model="dlg.form.book_id" placeholder="选择捐款账簿" style="width:100%">
            <el-option v-for="b in donationBooks" :key="b.id" :label="bookLabel(b)" :value="b.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="碑额标题">
          <el-input v-model="dlg.form.title" maxlength="40" :placeholder="dlgDefaultTitle" />
        </el-form-item>
        <el-form-item label="入碑门槛（元）">
          <el-input-number v-model="dlg.form.threshold" :min="0" :precision="0" controls-position="right" style="width:100%" />
          <div class="p-tip">默认 0，即所有捐款均入碑</div>
        </el-form-item>
        <el-form-item label="每列人数">
          <el-input-number v-model="dlg.form.per_column" :min="4" :max="40" controls-position="right" style="width:100%" />
        </el-form-item>
        <el-form-item label="数字写法">
          <el-radio-group v-model="dlg.form.numeral">
            <el-radio-button value="arabic">阿拉伯数字</el-radio-button>
            <el-radio-button value="chinese">中文数字</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="底色字色">
          <el-select v-model="dlg.form.style" style="width:100%">
            <el-option v-for="o in STYLE_OPTS" :key="o.v" :label="o.l" :value="o.v" />
          </el-select>
        </el-form-item>
        <el-form-item label="边框纹样">
          <el-select v-model="dlg.form.border" style="width:100%">
            <el-option v-for="o in BORDER_OPTS" :key="o.v" :label="o.l" :value="o.v" />
          </el-select>
        </el-form-item>
        <el-form-item label="显示金额">
          <el-switch v-model="dlg.form.show_amount" active-text="显示" inactive-text="只列名" />
        </el-form-item>
        <el-form-item label="落款">
          <el-input v-model="dlg.form.footer" maxlength="60" placeholder="如：××理事会 立 · 二〇二六年" />
        </el-form-item>
      </el-form>
      <template #footer>
        <span v-if="!canManage" class="p-tip" style="float:left">（仅财务管理 / 理事长可改）</span>
        <el-button @click="dlg.show = false">取消</el-button>
        <el-button type="primary" :loading="dlg.saving" @click="saveStele">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import api from '../api'
import { exportCoverPng, exportCoverSvg } from '../utils/coverExport'
import { SVG_FONT_NO_UPLOAD_MSG } from '../utils/treeExport'

// 复用 house 文案（treeExport.js 的 SVG_FONT_NO_UPLOAD_MSG），只把末句指向改掉：
// 原文案指向「牒记预览」页的字体上传入口，功德碑没有该入口。
const FONT_NO_UPLOAD_MSG = /或先在/.test(SVG_FONT_NO_UPLOAD_MSG)
  ? SVG_FONT_NO_UPLOAD_MSG.replace(/或先在.*$/, '或在谱书里上传字体')
  : SVG_FONT_NO_UPLOAD_MSG

// 底色字色（配色）与 边框纹样 分开、独立设置
const STYLE_OPTS = [
  { v: 'stone', l: '墨底金字' },
  { v: 'plain', l: '白底黑字' },
  { v: 'jade', l: '汉白玉 · 描金' },
  { v: 'gold', l: '朱漆描金' }
]
const BORDER_OPTS = [
  { v: 'none', l: '无（素框）' },
  { v: 'hui1', l: '回纹 · 单线' },
  { v: 'hui2', l: '回纹 · 带框线' },
]

const cfg = ref({})
const books = ref([])
const entries = ref([])
const surname = ref('')
const steleRef = ref(null)
const exporting = ref(false)
const loaded = ref(false)

const steles = ref([])
const currentId = ref(null)

const dlg = reactive({
  show: false,
  editing: false,
  saving: false,
  form: emptyForm()
})

const canManage = computed(() => !!cfg.value.can_manage_books)
const donationBooks = computed(() => books.value.filter(b => b.book_type === 'donation'))
const current = computed(() => steles.value.find(s => s.id === currentId.value) || null)
const curBook = computed(() => books.value.find(b => b.id === (current.value && current.value.book_id)) || {})

function bookLabel(b) {
  const proj = b.project ? ' · ' + b.project : ''
  return `${b.title}${proj}（${b.entry_count || 0} 笔）`
}
function emptyForm() {
  return { name: '', book_id: null, title: '', threshold: 0, per_column: 12, numeral: 'arabic', style: 'stone', border: 'none', show_amount: true, footer: '' }
}

// 当前碑的展示配置
const styleKey = computed(() => (current.value && current.value.style) || 'stone')
const borderKey = computed(() => (current.value && current.value.border) || 'none')
const mNumeral = computed(() => (current.value && current.value.numeral) || 'arabic')
const mShowAmount = computed(() => !current.value || current.value.show_amount !== false)
const mThreshold = computed(() => Number((current.value && current.value.threshold) || 0))
const rowsPerCol = computed(() => Math.max(1, Number((current.value && current.value.per_column) || 12)))
const mFooter = computed(() => (current.value && current.value.footer) || '')

const defaultTitle = computed(() => {
  const s = surname.value ? surname.value + '氏' : ''
  const proj = curBook.value.project || curBook.value.title || ''
  return `${s}${proj}乐捐芳名录`
})
const title = computed(() => ((current.value && current.value.title) || '').trim() || defaultTitle.value)

const formBook = computed(() => books.value.find(b => b.id === dlg.form.book_id) || {})
const dlgDefaultTitle = computed(() => {
  const s = surname.value ? surname.value + '氏' : ''
  const proj = formBook.value.project || formBook.value.title || ''
  return `${s}${proj}乐捐芳名录`
})

// 归集：已审核的「收入」——实名者按成员合并、上榜；匿名者单独汇总，列在最后
const donors = computed(() => {
  const map = new Map()
  for (const e of entries.value) {
    if (e.kind !== 'in' || e.status !== 'approved') continue
    const amt = Number(e.amount || 0)
    if (!(amt > 0)) continue
    const linked = e.member_id && e.member_id > 0
    if (!(linked && e.donor_public && e.member_name)) continue // 匿名不计入实名榜
    const key = 'm' + e.member_id
    if (!map.has(key)) map.set(key, { name: e.member_name, amount: 0 })
    map.get(key).amount += amt
  }
  const th = mThreshold.value
  const arr = Array.from(map.values()).filter(d => d.amount >= th)
  arr.sort((a, b) => b.amount - a.amount || (a.name > b.name ? 1 : -1))
  return arr
})

// 匿名捐款：按「人」汇总（有成员则按成员，否则按单笔），取入碑门槛后 → 人数 + 总额
const anon = computed(() => {
  const map = new Map()
  for (const e of entries.value) {
    if (e.kind !== 'in' || e.status !== 'approved') continue
    const amt = Number(e.amount || 0)
    if (!(amt > 0)) continue
    const linked = e.member_id && e.member_id > 0
    if (linked && e.donor_public && e.member_name) continue // 已实名的跳过
    const key = linked ? ('m' + e.member_id) : ('e' + e.id)
    map.set(key, (map.get(key) || 0) + amt)
  }
  const th = mThreshold.value
  const amts = Array.from(map.values()).filter(a => a >= th)
  return { count: amts.length, total: amts.reduce((s, a) => s + a, 0) }
})

// 碑上匿名行文案：显示金额时含总额，否则只给人数
const anonText = computed(() => {
  if (!anon.value.count) return ''
  const c = mNumeral.value === 'chinese' ? toChineseNumber(anon.value.count) : String(anon.value.count)
  if (!mShowAmount.value) return `匿名捐款 ${c} 人`
  return `匿名捐款 ${c} 人，共 ${fmtAmount(anon.value.total)}`
})

const columns = computed(() => {
  const n = rowsPerCol.value
  const out = []
  for (let i = 0; i < donors.value.length; i += n) out.push(donors.value.slice(i, i + n))
  return out
})

const totalDon = computed(() => donors.value.reduce((s, d) => s + d.amount, 0))
const grandTotal = computed(() => totalDon.value + anon.value.total)

function toChineseNumber(num) {
  num = Math.round(Number(num) || 0)
  if (num === 0) return '零'
  const D = ['零', '一', '二', '三', '四', '五', '六', '七', '八', '九']
  const U = ['', '十', '百', '千']
  const BIG = ['', '万', '亿', '兆']
  function sec(sec) {
    let s = '', zero = false, p = 0, t = sec
    while (t > 0) {
      const dig = t % 10
      if (dig === 0) { if (s) zero = true }
      else { if (zero) { s = '零' + s; zero = false } s = D[dig] + U[p] + s }
      t = Math.floor(t / 10); p++
    }
    return s
  }
  const secs = []
  let t = num
  while (t > 0) { secs.push(t % 10000); t = Math.floor(t / 10000) }
  let out = ''
  for (let i = secs.length - 1; i >= 0; i--) {
    const v = secs[i]
    if (v === 0) { if (out && !/零$/.test(out)) out += '零'; continue }
    if (out && v < 1000) out += '零'
    out += sec(v) + BIG[i]
  }
  out = out.replace(/^一十/, '十').replace(/零+$/, '')
  return out
}

function fmtAmount(a) {
  const n = Number(a || 0)
  if (mNumeral.value === 'chinese') return toChineseNumber(n) + '元'
  return (Number.isInteger(n) ? String(n) : n.toFixed(2)) + '元'
}

async function loadCfg() {
  try { const { data } = await api.get('/accounts/config'); cfg.value = data || {} } catch (e) { cfg.value = {} }
}
async function loadBooks() {
  try { const { data } = await api.get('/accounts/books'); books.value = data || [] } catch (e) { books.value = [] }
}
async function loadSteles() {
  try { const { data } = await api.get('/merit/steles'); steles.value = Array.isArray(data) ? data : [] } catch (e) { steles.value = [] }
  if (steles.value.length) {
    if (!steles.value.some(s => s.id === currentId.value)) currentId.value = steles.value[0].id
  } else {
    currentId.value = null
  }
  await loadEntries()
}
async function loadEntries() {
  const bid = current.value && current.value.book_id
  if (!bid) { entries.value = []; return }
  try { const { data } = await api.get('/accounts/books/' + bid + '/entries'); entries.value = data || [] } catch (e) { entries.value = [] }
}
function select(id) { currentId.value = id }
watch(currentId, () => { loadEntries() })

function openNew() {
  dlg.editing = false
  dlg.form = emptyForm()
  dlg.form.book_id = donationBooks.value.length ? donationBooks.value[0].id : null
  if (current.value) {
    dlg.form.numeral = current.value.numeral || 'arabic'
    dlg.form.style = current.value.style || 'stone'
    dlg.form.border = current.value.border || 'none'
    dlg.form.per_column = current.value.per_column || 12
    dlg.form.show_amount = current.value.show_amount !== false
    dlg.form.threshold = Number(current.value.threshold || 0)
  }
  dlg.show = true
}
function openEdit() {
  if (!current.value) return
  dlg.editing = true
  dlg.form = {
    name: current.value.name || '',
    book_id: current.value.book_id,
    title: current.value.title || '',
    threshold: Number(current.value.threshold || 0),
    per_column: current.value.per_column || 12,
    numeral: current.value.numeral || 'arabic',
    style: current.value.style || 'stone',
    border: current.value.border || 'none',
    show_amount: current.value.show_amount !== false,
    footer: current.value.footer || ''
  }
  dlg.show = true
}
async function saveStele() {
  if (!dlg.form.book_id) { ElMessage.warning('请选择捐款账簿'); return }
  dlg.saving = true
  try {
    let savedId = currentId.value
    if (dlg.editing && current.value) {
      const { data } = await api.put('/merit/steles/' + current.value.id, dlg.form)
      savedId = (data && data.id) || current.value.id
      ElMessage.success('已保存')
    } else {
      const { data } = await api.post('/merit/steles', dlg.form)
      savedId = data && data.id
      ElMessage.success('已新增功德碑')
    }
    dlg.show = false
    await loadSteles()
    if (savedId) currentId.value = savedId
    await loadEntries()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  } finally {
    dlg.saving = false
  }
}
async function delStele() {
  if (!current.value) return
  try {
    await ElMessageBox.confirm(`确定删除「${current.value.name || '功德碑'}」？此操作不可恢复。`, '删除功德碑', { type: 'warning' })
  } catch (e) { return }
  try {
    await api.delete('/merit/steles/' + current.value.id)
    ElMessage.success('已删除')
    currentId.value = null
    await loadSteles()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  }
}

function applyPage() {
  let el = document.getElementById('merit-print-page')
  if (!el) { el = document.createElement('style'); el.id = 'merit-print-page'; document.head.appendChild(el) }
  el.textContent = '@page { size: A4 portrait; margin: 6mm; }'
}
// 导出分档：PNG 高清 300 / SVG 矢量 / PDF 打印
// 碑体是 HTML/CSS 绘制（竖排 writing-mode + border-image 纹样），走 coverExport 的自包含 SVG 管线
// 注：属主要求这两个视图**不显示 DPI**（管线按 A4 高度归一，CSS 像素口径的 DPI 会误导）
// 超大碑会触 canvas 单边上限（14000px）自动降倍率，此时如实提示
function capNote(r) {
  return Math.max(r.width, r.height) >= 13999 ? '，受 14000px 上限限制已自动降倍率' : ''
}

// 检测导出元素里**实际用到**的字体族中，是否有一个来自「服务器上传字体」（族名 ZupuFont_*）。
// 现状必然返回 null：碑体没有任何 font-family 声明，继承 style.css 的 "Songti SC","SimSun",serif（系统字体），
// 扫描结果里不会出现 ZupuFont_*，因此**不会**发出多余的 /api/fonts 请求。
async function findUploadedFontInUse(el) {
  const used = new Set()
  try {
    ;[el, ...el.querySelectorAll('*')].forEach(n => {
      String(getComputedStyle(n).fontFamily || '').split(',').forEach(s => {
        const v = s.trim().replace(/^["']|["']$/g, '')
        if (v) used.add(v)
      })
    })
  } catch (e) { return null }
  const hit = [...used].filter(v => v.startsWith('ZupuFont_'))
  if (!hit.length) return null
  try {
    // 应用用 axios 带 Bearer token（不是 cookie），raw fetch 必须自己带上，否则 401 会被误判成「没有上传字体」
    const t = localStorage.getItem('token')
    const r = await fetch('/api/fonts', { credentials: 'include', headers: t ? { Authorization: 'Bearer ' + t } : {} })
    if (!r.ok) return null
    const list = await r.json()
    return (Array.isArray(list) ? list : []).find(f => hit.includes(f.family)) || null
  } catch (e) { return null }
}

// 「SVG（内嵌字体）」档：把用到的字体子集化后以 @font-face 内嵌进导出的 SVG，让字形随文件走。
//
// 现状：碑体用的是**系统字体**（继承 Songti SC/SimSun），浏览器读不到系统字体文件、无法子集化，
// 按属主口径「有提示即可」：只给明确提示，**不产出任何文件**
// （绝不能静默导出一个「以为带了其实没带」的 SVG）。
//
// 预留（future-proof）：若将来碑体改用服务器上传字体（族名 ZupuFont_*），findUploadedFontInUse() 就会返回该字体，
// 届时按下面四步接线即可（本轮不动 coverExport，故到提示为止）：
//   ① const buf = await (await fetch(font.url, { credentials:'include', headers:{Authorization:'Bearer '+localStorage.getItem('token')} })).arrayBuffer()
//   ② 收集 el 内所有文本节点的字符（碑体是 HTML，不是 SVG 的 text/tspan），const sub = subsetTtf(buf, chars)   // utils/fontSubset.js
//   ③ const extraCss = `@font-face{font-family:'ZupuSubsetFont';src:url(data:font/ttf;base64,${bytesToBase64(sub.data)}) format('truetype');}`
//                     + `.stele,.stele *{font-family:'ZupuSubsetFont',serif;}`
//   ④ await exportCoverSvg(el, filename, { extraCss })
//      注入点 = coverExport 里 collectCss() 的结果之后、buildCoverSvg 的 <style> 内；
//      注意：extraCss 必须是**纯 CSS 规则**（不要 <style> 标签）、**家族名用单引号**（双引号塞进 style="…" 会破坏 XML 且静默失败）、
//      且**必须同时覆盖 font-family**（否则继承来的宋体声明会赢，内嵌字体永远用不上）。
//      需要 coverExport.exportCoverSvg 增加 extraCss 形参。
async function exportSvgWithFont(el, filename) {
  const font = await findUploadedFontInUse(el)
  if (font) {
    // TODO(下一轮)：按上面 ①②③④ 接线后，这里就会真的内嵌字体（filename 即导出文件名）
    void filename
  }
  ElMessage.warning(FONT_NO_UPLOAD_MSG)
}

async function doExport(cmd) {
  const el = steleRef.value
  if (!el || exporting.value) return
  const base = (title.value || '功德碑').replace(/[\\/:*?"<>|]/g, '')
  exporting.value = true
  try {
    if (cmd === 'png300') {
      const r = await exportCoverPng(el, `${base}-300dpi.png`)
      ElMessage.success(`已导出 PNG（${r.width}×${r.height}px${capNote(r)}）`)
    } else if (cmd === 'svg') {
      exportCoverSvg(el, base + '.svg')
      ElMessage.success('已导出矢量 SVG（线条/文字无限清晰，可印刷放大）')
    } else if (cmd === 'svgfont') {
      await exportSvgWithFont(el, base + '.svg')
    } else if (cmd === 'pdf') {
      applyPage()
      window.print()
      ElMessage.success('已打开打印视图（在打印对话框选择「另存为 PDF」）')
    }
  } catch (e) {
    ElMessage.error('导出失败：' + (e && e.message ? e.message : e))
  } finally {
    exporting.value = false
  }
}

onMounted(async () => {
  await loadCfg()
  try { const { data } = await api.get('/settings/family-surname'); surname.value = data.family_surname || '' } catch (e) { /* ignore */ }
  await loadBooks()
  await loadSteles()
  loaded.value = true
})
onBeforeUnmount(() => { const el = document.getElementById('merit-print-page'); if (el) el.remove() })
</script>

<style scoped>
.merit-page { padding: 4px; }
.page-head { display: flex; align-items: baseline; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
.page-head h2 { margin: 0; color: #5a4632; }
.hint { color: #97a0ab; font-size: 13px; }
.head-actions { margin-left: auto; display: flex; gap: 8px; flex-wrap: wrap; }

/* 切换标签 */
.tabs { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; }
.tab {
  display: flex; flex-direction: column; align-items: flex-start;
  border: 1px solid #e0d6c4; background: #faf7f1; border-radius: 8px;
  padding: 6px 14px; cursor: pointer; transition: all .15s; min-width: 92px;
}
.tab:hover { border-color: #c9a86a; background: #fff; }
.tab.active { border-color: #b8860b; background: #fff7e6; box-shadow: 0 2px 8px rgba(184,134,11,.18); }
.tab-name { font-size: 14px; color: #5a4632; font-weight: 600; }
.tab.active .tab-name { color: #8a6516; }
.tab-sub { font-size: 11px; color: #b3a48c; margin-top: 2px; }

.toolbar { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 10px; }
.toolbar .stat { font-size: 13px; color: #5a4632; }
.toolbar .stat b { color: #b8860b; }
.toolbar .export { margin-left: auto; display: flex; gap: 8px; flex-wrap: wrap; }

.stage { overflow: auto; padding: 6px 0 30px; }
.zoom-wrap { text-align: center; }
.p-tip { font-size: 12px; color: #b3a48c; margin-top: 4px; }

/* ===== 碑体（外框提供边框/纹样，内层承载内容与底色）===== */
.stele { display: inline-block; min-width: 420px; margin: 0 auto; border-radius: 6px; text-align: center; }
.stele-inner { padding: 24px 30px 20px; border-radius: 3px; }
.bian-e { font-size: 30px; font-weight: 700; letter-spacing: 10px; padding: 6px 0 16px; }
.empty { color: #999; padding: 60px 10px; font-size: 15px; }

.names { display: flex; flex-direction: row-reverse; justify-content: center; gap: 20px; margin-top: 18px; }
.col { display: flex; flex-direction: column; gap: 18px; align-items: center; }
.donor { display: flex; flex-direction: column; align-items: center; }
.dn {
  writing-mode: vertical-rl; text-orientation: upright;
  font-size: 20px; font-weight: 600; letter-spacing: 2px;
  min-height: 4.2em;
}
.da {
  writing-mode: vertical-rl; text-orientation: upright;
  font-size: 13px; letter-spacing: 1px; margin-top: 6px; opacity: .88;
}
.liukuan { margin-top: 22px; font-size: 14px; letter-spacing: 2px; }
.anon-line { margin-top: 24px; font-size: 15px; font-weight: 600; letter-spacing: 1px; }

/* —— 墨底金字 —— */
.stele-pal-stone { background: #7a1d1d; padding: 12px; box-shadow: 0 8px 22px rgba(0,0,0,.35); }
.stele-pal-stone .stele-inner { background: linear-gradient(180deg, #2a2a2a, #131313); color: #f2d488; box-shadow: inset 0 0 0 2px rgba(242,212,136,.5); }
.stele-pal-stone .bian-e { color: #ffe9a8; border-bottom: 2px solid rgba(242,212,136,.45); }
.stele-pal-stone .dn { color: #ffe9a8; text-shadow: 0 1px 1px rgba(0,0,0,.6); }
.stele-pal-stone .liukuan, .stele-pal-stone .anon-line { color: #e9cf8f; }

/* —— 白底黑字 —— */
.stele-pal-plain { background: #444; padding: 8px; box-shadow: 0 6px 18px rgba(0,0,0,.15); }
.stele-pal-plain .stele-inner { background: #fff; color: #1a1a1a; box-shadow: inset 0 0 0 1px #bbb; }
.stele-pal-plain .bian-e { color: #111; border-bottom: 2px solid #999; }
.stele-pal-plain .dn { color: #1a1a1a; }
.stele-pal-plain .liukuan, .stele-pal-plain .anon-line { color: #333; }

/* —— 汉白玉 · 描金 —— */
.stele-pal-jade { background: #b8964a; padding: 12px; box-shadow: 0 8px 22px rgba(0,0,0,.2); }
.stele-pal-jade .stele-inner { background: linear-gradient(180deg, #fbf8f1, #efe9da); color: #6b4e1a; box-shadow: inset 0 0 0 2px #b8964a, inset 0 0 0 5px rgba(184,150,74,.32); }
.stele-pal-jade .bian-e { color: #7a5a1e; border-bottom: 2px solid #c9a86a; }
.stele-pal-jade .dn { color: #5a4318; }
.stele-pal-jade .liukuan, .stele-pal-jade .anon-line { color: #6b4e1a; }

/* —— 朱漆描金 —— */
.stele-pal-gold { background: #d4af37; padding: 12px; box-shadow: 0 8px 22px rgba(0,0,0,.25); }
.stele-pal-gold .stele-inner { background: linear-gradient(180deg, #8a2020, #5c1414); color: #f6dd9a; box-shadow: inset 0 0 0 2px rgba(246,221,154,.7); }
.stele-pal-gold .bian-e { color: #ffe9a8; border-bottom: 2px solid rgba(246,221,154,.6); }
.stele-pal-gold .dn { color: #f6dd9a; text-shadow: 0 1px 1px rgba(0,0,0,.5); }
.stele-pal-gold .liukuan, .stele-pal-gold .anon-line { color: #f0d79a; }

/* —— 边框纹样（独立设置；border-image 九宫格拼出含「四边+转角」的完整框）—— */
.stele.stele-bd-hui1, .stele.stele-bd-hui2 {
  padding: 0; border: 26px solid transparent; border-radius: 2px;
  background: #17110a; box-shadow: 0 8px 22px rgba(0,0,0,.4);
  border-image-slice: 40; border-image-width: 26px; border-image-repeat: round;
}
.stele.stele-bd-hui1 { border-image-source: url("data:image/svg+xml;base64,PHN2ZyB4bWxucz0naHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmcnIHdpZHRoPScxMjAnIGhlaWdodD0nMTIwJz48ZyBmaWxsPSdub25lJyBzdHJva2U9JyNkNGFmMzcnIHN0cm9rZS13aWR0aD0nMycgc3Ryb2tlLWxpbmVqb2luPSdtaXRlcicgc3Ryb2tlLWxpbmVjYXA9J3NxdWFyZSc+PHBhdGggZD0nTTQwIDIwIEgzMCBWMTAgSDEwIFYzMCBIMjAgVjQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSgwLDApJy8+PHBhdGggZD0nTTAgMjAgSDEwIFYxMiBIMzAgVjI4IEgyMCBWMjAgSDQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSg0MCwwKScvPjxwYXRoIGQ9J000MCAyMCBIMzAgVjEwIEgxMCBWMzAgSDIwIFY0MCcgdHJhbnNmb3JtPSd0cmFuc2xhdGUoMTIwLDApIHNjYWxlKC0xLDEpJy8+PHBhdGggZD0nTTAgMjAgSDEwIFYxMiBIMzAgVjI4IEgyMCBWMjAgSDQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSgxMjAsNDApIHJvdGF0ZSg5MCknLz48cGF0aCBkPSdNNDAgMjAgSDMwIFYxMCBIMTAgVjMwIEgyMCBWNDAnIHRyYW5zZm9ybT0ndHJhbnNsYXRlKDEyMCwxMjApIHNjYWxlKC0xLC0xKScvPjxwYXRoIGQ9J00wIDIwIEgxMCBWMTIgSDMwIFYyOCBIMjAgVjIwIEg0MCcgdHJhbnNmb3JtPSd0cmFuc2xhdGUoODAsMTIwKSByb3RhdGUoMTgwKScvPjxwYXRoIGQ9J000MCAyMCBIMzAgVjEwIEgxMCBWMzAgSDIwIFY0MCcgdHJhbnNmb3JtPSd0cmFuc2xhdGUoMCwxMjApIHNjYWxlKDEsLTEpJy8+PHBhdGggZD0nTTAgMjAgSDEwIFYxMiBIMzAgVjI4IEgyMCBWMjAgSDQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSg0MCw0MCkgcm90YXRlKDkwKScvPjwvZz48L3N2Zz4="); }
.stele.stele-bd-hui2 { border-image-source: url("data:image/svg+xml;base64,PHN2ZyB4bWxucz0naHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmcnIHdpZHRoPScxMjAnIGhlaWdodD0nMTIwJz48ZyBmaWxsPSdub25lJyBzdHJva2U9JyNkNGFmMzcnIHN0cm9rZS13aWR0aD0nMycgc3Ryb2tlLWxpbmVqb2luPSdtaXRlcicgc3Ryb2tlLWxpbmVjYXA9J3NxdWFyZSc+PHBhdGggZD0nTTQwIDQgSDQgVjQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSgwLDApJy8+PHBhdGggZD0nTTQwIDM2IEgzNiBWNDAnIHRyYW5zZm9ybT0ndHJhbnNsYXRlKDAsMCknLz48cGF0aCBkPSdNNDAgMjAgSDMwIFYxMCBIMTAgVjMwIEgyMCBWNDAnIHRyYW5zZm9ybT0ndHJhbnNsYXRlKDAsMCknLz48cGF0aCBkPSdNMCA0IEg0MCcgdHJhbnNmb3JtPSd0cmFuc2xhdGUoNDAsMCknLz48cGF0aCBkPSdNMCAzNiBINDAnIHRyYW5zZm9ybT0ndHJhbnNsYXRlKDQwLDApJy8+PHBhdGggZD0nTTAgMjAgSDEwIFYxMiBIMzAgVjI4IEgyMCBWMjAgSDQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSg0MCwwKScvPjxwYXRoIGQ9J000MCA0IEg0IFY0MCcgdHJhbnNmb3JtPSd0cmFuc2xhdGUoMTIwLDApIHNjYWxlKC0xLDEpJy8+PHBhdGggZD0nTTQwIDM2IEgzNiBWNDAnIHRyYW5zZm9ybT0ndHJhbnNsYXRlKDEyMCwwKSBzY2FsZSgtMSwxKScvPjxwYXRoIGQ9J000MCAyMCBIMzAgVjEwIEgxMCBWMzAgSDIwIFY0MCcgdHJhbnNmb3JtPSd0cmFuc2xhdGUoMTIwLDApIHNjYWxlKC0xLDEpJy8+PHBhdGggZD0nTTAgNCBINDAnIHRyYW5zZm9ybT0ndHJhbnNsYXRlKDEyMCw0MCkgcm90YXRlKDkwKScvPjxwYXRoIGQ9J00wIDM2IEg0MCcgdHJhbnNmb3JtPSd0cmFuc2xhdGUoMTIwLDQwKSByb3RhdGUoOTApJy8+PHBhdGggZD0nTTAgMjAgSDEwIFYxMiBIMzAgVjI4IEgyMCBWMjAgSDQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSgxMjAsNDApIHJvdGF0ZSg5MCknLz48cGF0aCBkPSdNNDAgNCBINCBWNDAnIHRyYW5zZm9ybT0ndHJhbnNsYXRlKDEyMCwxMjApIHNjYWxlKC0xLC0xKScvPjxwYXRoIGQ9J000MCAzNiBIMzYgVjQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSgxMjAsMTIwKSBzY2FsZSgtMSwtMSknLz48cGF0aCBkPSdNNDAgMjAgSDMwIFYxMCBIMTAgVjMwIEgyMCBWNDAnIHRyYW5zZm9ybT0ndHJhbnNsYXRlKDEyMCwxMjApIHNjYWxlKC0xLC0xKScvPjxwYXRoIGQ9J00wIDQgSDQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSg4MCwxMjApIHJvdGF0ZSgxODApJy8+PHBhdGggZD0nTTAgMzYgSDQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSg4MCwxMjApIHJvdGF0ZSgxODApJy8+PHBhdGggZD0nTTAgMjAgSDEwIFYxMiBIMzAgVjI4IEgyMCBWMjAgSDQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSg4MCwxMjApIHJvdGF0ZSgxODApJy8+PHBhdGggZD0nTTQwIDQgSDQgVjQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSgwLDEyMCkgc2NhbGUoMSwtMSknLz48cGF0aCBkPSdNNDAgMzYgSDM2IFY0MCcgdHJhbnNmb3JtPSd0cmFuc2xhdGUoMCwxMjApIHNjYWxlKDEsLTEpJy8+PHBhdGggZD0nTTQwIDIwIEgzMCBWMTAgSDEwIFYzMCBIMjAgVjQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSgwLDEyMCkgc2NhbGUoMSwtMSknLz48cGF0aCBkPSdNMCA0IEg0MCcgdHJhbnNmb3JtPSd0cmFuc2xhdGUoNDAsNDApIHJvdGF0ZSg5MCknLz48cGF0aCBkPSdNMCAzNiBINDAnIHRyYW5zZm9ybT0ndHJhbnNsYXRlKDQwLDQwKSByb3RhdGUoOTApJy8+PHBhdGggZD0nTTAgMjAgSDEwIFYxMiBIMzAgVjI4IEgyMCBWMjAgSDQwJyB0cmFuc2Zvcm09J3RyYW5zbGF0ZSg0MCw0MCkgcm90YXRlKDkwKScvPjwvZz48L3N2Zz4="); }

@media print {
  .no-print { display: none !important; }
  .merit-page { padding: 0; }
  .stage { overflow: visible; padding: 0; }
  .stele { max-width: none; margin: 0 auto; }
}
@media (max-width: 767px) {
  .head-actions { margin-left: 0; width: 100%; }
  .stage { width: 100%; }
  .toolbar .export { margin-left: 0; width: 100%; }
}
</style>
