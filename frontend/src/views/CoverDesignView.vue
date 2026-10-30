<template>
  <div class="cover-design" :class="{ fullbleed: noMargin }">
    <div class="toolbar no-print">
      <span class="serif title">谱书封面</span>
      <span class="lbl">样式</span>
      <el-select v-model="styleKey" size="small" style="width: 210px">
        <el-option v-for="s in styleList" :key="s.key" :label="s.name" :value="s.key" />
      </el-select>
      <span class="lbl">修次</span>
      <el-select v-model="xiuci" size="small" style="width:120px" filterable allow-create default-first-option @change="saveCoverSettings">
        <el-option v-for="o in XIUCI_OPTS" :key="o" :label="o" :value="o" />
      </el-select>
      <el-input v-model="xiuci" size="small" style="width:110px" placeholder="自定义修次" @change="saveCoverSettings" />
      <el-checkbox v-model="titleWithXiuci" size="small" @change="saveCoverSettings">题名含修次</el-checkbox>
      <span class="lbl">卷次</span>
      <el-input v-model="volume" size="small" style="width: 90px" />
      <span class="lbl">主修</span>
      <el-input v-model="chief" size="small" style="width: 110px" placeholder="主修人" />
      <span class="lbl">编纂</span>
      <el-input v-model="editorName" size="small" style="width: 110px" placeholder="编纂人" />
      <span class="lbl">封底</span>
      <el-select v-model="backType" size="small" style="width: 130px">
        <el-option label="印章" value="seal" />
        <el-option label="牌记" value="colophon" />
        <el-option label="纯素" value="plain" />
      </el-select>
      <el-checkbox v-model="noMargin" size="small">无边距（满版）</el-checkbox>
      <el-checkbox v-model="mirror" size="small">镜像（装订在左）</el-checkbox>
      <el-checkbox v-model="landscape" size="small">横版</el-checkbox>
      <span class="lbl">缩放</span>
      <el-slider v-model="zoomPct" :min="20" :max="150" :step="5" :show-tooltip="false"
                 style="width: 120px; margin: 0 6px" />
      <span class="zoom-val">{{ zoomPct }}%</span>
      <el-button size="small" @click="fitZoom">适应屏幕</el-button>
      <el-dropdown size="small" trigger="click" @command="doExport">
        <el-button size="small" :loading="exporting">导出<el-icon class="el-icon--right"><arrow-down /></el-icon></el-button>
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

    <div class="stage" ref="stage">
      <div class="pair" :style="{ zoom: zoomRatio }">
        <div class="sheet">
          <BookCover ref="frontRef" side="front" :style-key="styleKey" :base="820" :title="title" :volume="volume"
                     :tanghao="tanghao" :junwang="junwang" :zuji="zuji" :surname="surname"
                     :ganzhi="ganzhi" :year-g="yearG" :mirror="mirror" :landscape="landscape" :xiuci="xiuci" />
        </div>
        <div class="sheet">
          <BookCover ref="backRef" side="back" :style-key="styleKey" :base="820" :title="title" :volume="volume"
                     :tanghao="tanghao" :junwang="junwang" :zuji="zuji" :surname="surname"
                     :ganzhi="ganzhi" :year-g="yearG" :back-type="backType" :mirror="mirror" :landscape="landscape" :xiuci="xiuci"
                     :colophon="{ volume, chief: chief, editor: editorName, press: '', edition: `${yearG}年 ${ganzhi} ${xiuci}` }" />
        </div>
      </div>
    </div>

    <h3 class="sec no-print">全部样式一览</h3>
    <div class="gallery no-print">
      <div v-for="s in styleList" :key="s.key" class="cell" :class="{ on: s.key === styleKey }" @click="styleKey = s.key">
        <BookCover side="front" :style-key="s.key" :base="300" :title="title" :volume="volume"
                   :tanghao="tanghao" :surname="surname" :ganzhi="ganzhi" :year-g="yearG" :landscape="landscape" :xiuci="xiuci" />
        <div class="cap">{{ s.name }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import api from '../api'
import BookCover from '../components/BookCover.vue'
import { ElMessage } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import { exportCoverPng, exportCoverSvg, sleep } from '../utils/coverExport'
import { SVG_FONT_NO_UPLOAD_MSG } from '../utils/treeExport'

// 复用 house 文案（treeExport.js 的 SVG_FONT_NO_UPLOAD_MSG），只把末句指向改掉：
// 原文案指向「牒记预览」页的字体上传入口，谱书封面/封底没有该入口。
const FONT_NO_UPLOAD_MSG = /或先在/.test(SVG_FONT_NO_UPLOAD_MSG)
  ? SVG_FONT_NO_UPLOAD_MSG.replace(/或先在.*$/, '或在谱书里上传字体')
  : SVG_FONT_NO_UPLOAD_MSG

const styleList = [
  { key: 'indigo',  name: '① 靛蓝布面 · 经典线装' },
  { key: 'navy',    name: '② 藏青烫金' },
  { key: 'crimson', name: '③ 朱红缎面' },
  { key: 'jade',    name: '④ 墨绿布面' },
  { key: 'bronze',  name: '⑤ 古铜函套 · 纹样边框' },
  { key: 'paper',   name: '⑥ 米黄仿古 · 宣纸' },
]

const styleKey = ref(localStorage.getItem('coverStyle') || 'indigo')
const volume = ref(localStorage.getItem('coverVolume') || '卷一')
const backType = ref('seal')
const noMargin = ref(localStorage.getItem('coverNoMargin') === '1')
const mirror = ref(localStorage.getItem('coverMirror') === '1')
const landscape = ref(localStorage.getItem('coverLandscape') === '1')

const frontRef = ref(null)
const backRef = ref(null)
const stage = ref(null)
const exporting = ref(false)

// 预览缩放（手机端可缩放查看）
const zoomPct = ref(Number(localStorage.getItem('coverZoom')) || 100)
const zoomRatio = computed(() => zoomPct.value / 100)
watch(zoomPct, v => localStorage.setItem('coverZoom', String(v)))
function fitZoom() {
  const el = stage.value
  if (!el) return
  const avail = (el.clientWidth || window.innerWidth) - 20
  const pairW = 820 * 2 + 28
  let pct = Math.floor((avail / pairW) * 100)
  pct = Math.max(20, Math.min(150, pct))
  zoomPct.value = pct
}

const surname = ref('')
const familyName = ref('')
const tanghao = ref('')
const xiuci = ref('重修')            // 修次：封面「歲次…X」与牌记版次都用它
const titleWithXiuci = ref(false)    // 题名含修次（默认不勾）→「李氏創修族譜」
const XIUCI_OPTS = ['創修', '初修', '二修', '三修', '四修', '五修', '重修', '續修']
const junwang = ref('')
const zuji = ref('')
const chief = ref(localStorage.getItem('coverChief') || '')
const editorName = ref(localStorage.getItem('coverEditor') || '')
watch(chief, v => localStorage.setItem('coverChief', v))
watch(editorName, v => localStorage.setItem('coverEditor', v))

const yearG = new Date().getFullYear()
const ganzhi = computed(() => {
  const gan = '甲乙丙丁戊己庚辛壬癸', zhi = '子丑寅卯辰巳午未申酉戌亥'
  return gan[(yearG - 4 + 10) % 10] + zhi[(yearG - 4 + 12) % 12]
})
// 题名：默认「X氏族譜」；勾选「题名含修次」→「X氏<修次>族譜」（如「李氏創修族譜」）
const title = computed(() => {
  const base = (surname.value ? surname.value + '氏' : '') + '族譜'
  if (!titleWithXiuci.value) return base
  const xc = (xiuci.value || '').trim()
  return xc ? (surname.value ? surname.value + '氏' : '') + xc + '族譜' : base
})

// 保存「修次 / 题名含修次」到服务器设置（与堂号等同一接口），保证每台设备/每次导出一致
async function saveCoverSettings() {
  try {
    await api.put('/settings/family-surname', { xiuci: xiuci.value, title_with_xiuci: titleWithXiuci.value })
    const { data } = await api.get('/settings/family-surname')
    xiuci.value = data.xiuci || '重修'
    titleWithXiuci.value = !!data.title_with_xiuci
    ElMessage.success('修次已保存：' + xiuci.value)
  } catch (e) {
    ElMessage.error('保存失败：' + (e?.response?.data?.error || e?.message || e))
  }
}

function applyPage() {
  let el = document.getElementById('cover-print-page')
  if (!el) { el = document.createElement('style'); el.id = 'cover-print-page'; document.head.appendChild(el) }
  el.textContent = `@page { size: A4 ${landscape.value ? 'landscape' : 'portrait'}; margin: 0; }`
}
watch(noMargin, v => localStorage.setItem('coverNoMargin', v ? '1' : '0'))
watch(mirror, v => localStorage.setItem('coverMirror', v ? '1' : '0'))
watch(landscape, v => localStorage.setItem('coverLandscape', v ? '1' : '0'))

function exportPDF() {
  localStorage.setItem('coverStyle', styleKey.value)
  localStorage.setItem('coverVolume', volume.value)
  applyPage()
  window.print()
}

// 导出分档：PNG 高清 300 / SVG 矢量 / PDF 打印（封面、封底各出一份文件）
// 封面是 HTML/CSS 绘制，走 coverExport 的「HTML → 自包含 foreignObject SVG」管线
// 注：属主要求这两个视图**不显示 DPI**（管线按 A4 高度归一，CSS 像素口径的 DPI 会误导）
// 超大内容会触 canvas 单边上限（14000px）自动降倍率，此时如实提示
function capNote(r) {
  return Math.max(r.width, r.height) >= 13999 ? '，受 14000px 上限限制已自动降倍率' : ''
}

// 检测导出元素里**实际用到**的字体族中，是否有一个来自「服务器上传字体」（族名 ZupuFont_*）。
// 现状必然返回 null：封面/封底只用系统字体（见 BookCover.vue 的 "KaiTi","STKaiti","Kaiti SC","SimSun",serif），
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
// 现状：封面/封底用的是**系统字体**（楷体/宋体），浏览器读不到系统字体文件、无法子集化，
// 按属主口径「有提示即可」：只给明确提示，**不产出任何文件**
// （绝不能静默导出一个「以为带了其实没带」的 SVG）。
//
// 预留（future-proof）：若将来封面改用服务器上传字体（族名 ZupuFont_*），findUploadedFontInUse() 就会返回该字体，
// 届时按下面四步接线即可（本轮不动 coverExport，故到提示为止）：
//   ① const buf = await (await fetch(font.url, { credentials:'include', headers:{Authorization:'Bearer '+localStorage.getItem('token')} })).arrayBuffer()
//   ② 收集 el 内所有文本节点的字符（封面是 HTML，不是 SVG 的 text/tspan），const sub = subsetTtf(buf, chars)   // utils/fontSubset.js
//   ③ const extraCss = `@font-face{font-family:'ZupuSubsetFont';src:url(data:font/ttf;base64,${bytesToBase64(sub.data)}) format('truetype');}`
//                     + `.bk,.bk *{font-family:'ZupuSubsetFont',serif;}`
//   ④ await exportCoverSvg(el, filename, { extraCss })
//      注入点 = coverExport 里 collectCss() 的结果之后、buildCoverSvg 的 <style> 内；
//      注意：extraCss 必须是**纯 CSS 规则**（不要 <style> 标签）、**家族名用单引号**（双引号塞进 style="…" 会破坏 XML 且静默失败）、
//      且**必须同时覆盖 font-family**（否则既有的楷体/宋体声明会赢，内嵌字体永远用不上）。
//      需要 coverExport.exportCoverSvg 增加 extraCss 形参；封面/封底各导一份，需对这个函数调用两次。
async function exportSvgWithFont(el, filename) {
  const font = await findUploadedFontInUse(el)
  if (font) {
    // TODO(下一轮)：按上面 ①②③④ 接线后，这里就会真的内嵌字体（filename 即导出文件名）
    void filename
  }
  ElMessage.warning(FONT_NO_UPLOAD_MSG)
}

async function doExport(cmd) {
  if (exporting.value) return
  const f = frontRef.value && (frontRef.value.$el || frontRef.value)
  const b = backRef.value && (backRef.value.$el || backRef.value)
  if (!f || !b) return
  const base = `${title.value}${volume.value ? '-' + volume.value : ''}`
  exporting.value = true
  try {
    if (cmd === 'png300') {
      const rf = await exportCoverPng(f, `${base}-封面-300dpi.png`)
      await sleep(350)
      const rb = await exportCoverPng(b, `${base}-封底-300dpi.png`)
      ElMessage.success(`已导出 PNG（封面 ${rf.width}×${rf.height}px、封底 ${rb.width}×${rb.height}px${capNote(rf)}）`)
    } else if (cmd === 'svg') {
      exportCoverSvg(f, `${base}-封面.svg`)
      await sleep(350)
      exportCoverSvg(b, `${base}-封底.svg`)
      ElMessage.success('已导出矢量 SVG（封面/封底，线条/文字无限清晰，可印刷放大）')
    } else if (cmd === 'svgfont') {
      // 封面/封底共用同一套字体声明，取封面检测即可
      await exportSvgWithFont(f, `${base}-封面.svg`)
    } else if (cmd === 'pdf') {
      exportPDF()
      ElMessage.success('已打开打印视图（在打印对话框选择「另存为 PDF」）')
    }
  } catch (e) {
    ElMessage.error('导出失败：' + (e && e.message ? e.message : e))
  } finally {
    exporting.value = false
  }
}

onMounted(applyPage)
onMounted(() => { if (!localStorage.getItem('coverZoom')) fitZoom() })
onBeforeUnmount(() => { const el = document.getElementById('cover-print-page'); if (el) el.remove() })

onMounted(async () => {
  try {
    const { data: fs } = await api.get('/settings/family-surname')
    surname.value = fs.family_surname || ''
    familyName.value = fs.family_name || ''
    tanghao.value = fs.tanghao || ''
    xiuci.value = fs.xiuci || '重修'
    titleWithXiuci.value = !!fs.title_with_xiuci
    junwang.value = fs.junwang || ''
    zuji.value = fs.zuji || ''
  } catch (e) { /* ignore */ }
  try {
    const u = JSON.parse(localStorage.getItem('user') || 'null')
    if (u && u.username && !editorName.value) editorName.value = u.username
  } catch (e) { /* ignore */ }
})
</script>

<style scoped>
.cover-design { max-width: 1100px; margin: 0 auto; }
.toolbar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-bottom: 16px; }
.toolbar .title { font-size: 18px; margin-right: 8px; }
.toolbar .lbl { font-size: 12px; color: #8c5a3c; margin-left: 4px; }
.stage { display: flex; overflow: auto; padding: 10px 0 26px; }
.pair { margin: 0 auto; display: flex; gap: 28px; flex-wrap: wrap; justify-content: center; }
.zoom-val { font-size: 12px; color: #5d5140; width: 38px; text-align: center; }
.sec { font-size: 15px; letter-spacing: 2px; margin: 10px 0 14px; }
.gallery { display: flex; flex-wrap: wrap; gap: 18px; padding-bottom: 30px; }
.cell { cursor: pointer; padding: 8px; border-radius: 8px; border: 2px solid transparent; }
.cell.on { border-color: #8c5a3c; background: #f3ecdd; }
.cell .cap { text-align: center; font-size: 12px; color: #5d5140; margin-top: 8px; }

/* 打印：每页一张（.sheet 为一个物理页），页内居中留白 */
.sheet { }
@media print {
  .no-print { display: none !important; }
  .cover-design { max-width: none; }
  .stage { padding: 0; overflow: visible; }
  .pair { display: block; gap: 0; zoom: 1 !important; }
  .sheet { display: flex; align-items: center; justify-content: center;
           height: 297mm; box-sizing: border-box;
           break-after: page; page-break-after: always; overflow: hidden; }
  .sheet:last-child { break-after: auto; page-break-after: auto; }
  .cover-design:not(.fullbleed) .sheet { padding: 5mm; }
  .cover-design:not(.fullbleed) .sheet :deep(.bk) { zoom: 0.922; }
  .cover-design.fullbleed .sheet { padding: 0; }
  .cover-design.fullbleed .sheet :deep(.bk) { zoom: 0.968; }
}
</style>
