<template>
  <div v-if="variant === 'dieji'" class="epc">
    <div class="epc-grid" ref="gridRef" :style="{ '--cols': paddedCells.length }">
      <div v-for="(cell, ci) in paddedCells" :key="'cell' + ci"
           class="epc-cell" :class="['epc-' + cell.type, { blank: !cell.text }]">
        <span v-if="cell.text" class="epc-vtext">{{ cell.text }}</span>
      </div>
    </div>
  </div>

  <div v-else class="ep" :class="'ep-' + variant">
    <header v-if="variant !== 'chuizhu'" class="ep-head">
      家族大事記<template v-if="part > 1">（續）</template>
      <span v-if="total > 1" class="ep-count">{{ part }} / {{ total }}</span>
    </header>
    <div class="ep-body">
      <div class="ep-half ep-right">
        <header v-if="variant === 'chuizhu'" class="ep-head">
          家族大事記<template v-if="part > 1">（續）</template>
          <span v-if="total > 1" class="ep-count">{{ part }} / {{ total }}</span>
        </header>
        <article v-for="e in right" :key="'r' + e.id" class="ep-item">
          <div class="ep-year">{{ e.year }}</div>
          <div class="ep-title">{{ e.title }}</div>
          <p class="ep-text">{{ clean(e.content) }}</p>
        </article>
      </div>
      <div class="ep-half ep-left">
        <article v-for="e in left" :key="'l' + e.id" class="ep-item">
          <div class="ep-year">{{ e.year }}</div>
          <div class="ep-title">{{ e.title }}</div>
          <p class="ep-text">{{ clean(e.content) }}</p>
        </article>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'

const gridRef = ref(null)
const measuredH = ref(0)
function measureGrid() {
  const h = gridRef.value?.clientHeight || 0
  if (h > 0) measuredH.value = h
}
let ro = null
function beforePrint() { measureGrid() }
onMounted(() => {
  measureGrid()
  if (typeof ResizeObserver !== 'undefined' && gridRef.value) {
    ro = new ResizeObserver(measureGrid)
    ro.observe(gridRef.value)
  }
  window.addEventListener('beforeprint', beforePrint)
})
onBeforeUnmount(() => {
  ro?.disconnect()
  window.removeEventListener('beforeprint', beforePrint)
})
const props = defineProps({
  items: { type: Array, default: () => [] },
  part: { type: Number, default: 1 },
  total: { type: Number, default: 1 },
  variant: { type: String, default: 'dieji' }, // dieji: 竖格；chuizhu/branch: 原两半
  perPage: { type: Number, default: 24 },
  contentH: { type: Number, default: 741 }, // 定值版心高（不读 layout）
  pageTitle: { type: String, default: '家族大事记' },
  pageNo: { type: [Number, String], default: '' },
})

function clean(s) { return String(s == null ? '' : s).replace(/\*\*/g, '') }

// 版心高用定值传入，避免 auto 父级 / 内容撑高造成自引用。
// 竖版 A3：741px；横版 A3：562px；支谱 A4：1132px（真实页面量得/页面框定值）。
// 竖排时字符沿“列高”方向的步进是 font-size（18），line-height 32 只决定列宽；
// 因此每列字数必须用 18 而不是 32，否则只能填一半高度。
const effectiveH = computed(() => measuredH.value || props.contentH)
const charsPerCol = computed(() => Math.max(6, Math.floor((effectiveH.value - 8) / 18) + 1))
const cells = computed(() => {
  const out = [{ type: 'pagetitle', text: props.pageTitle }]
  for (const e of (props.items || [])) {
    const head = [e.title, e.year].filter(Boolean).join(' · ')
    if (head) out.push({ type: 'head', text: head })
    const text = clean(e.content)
    const n = charsPerCol.value
    for (let i = 0; i < text.length; i += n) out.push({ type: 'text', text: text.slice(i, i + n) })
  }
  return out
})
const paddedCells = computed(() => {
  const out = cells.value.slice()
  while (out.length < props.perPage) out.push({ type: 'blank', text: '' })
  return out
})
</script>

<style scoped>
/* ===== 牒记式：一格一排字，固定列宽 32px，竖线分隔，满页竖格 ===== */
.epc { position: relative; height: 100%; width: 100%; min-height: 0; overflow: visible; box-sizing: border-box; color: #1a1a1a; font-size: 18px; display: flex; }
.epc-grid { flex: 1 1 auto; min-height: 0; width: 100%; display: grid;
  grid-template-columns: repeat(var(--cols), minmax(0, 1fr)); grid-auto-rows: 100%;
  direction: rtl; align-items: stretch; }
.epc-cell {
  width: 100%; min-width: 0; max-width: none; height: 100%; direction: ltr;
  display: flex; align-items: center; justify-content: flex-start;
  border-left: 1px solid #b9ad91; overflow: hidden;
  font-size: 18px; letter-spacing: 0; padding: 0; box-sizing: border-box;
}
.epc-vtext {
  writing-mode: vertical-rl; text-orientation: upright;
  line-height: 1; text-align: center;
  /* 竖排字形左右侧边距不对称，墨迹略偏左；按约1px补偿（大事记/谱头共用） */
  transform: translateX(2px);
}
.epc-cell.epc-head { justify-content: center; font-weight: 700; color: #8a3b2a; font-size: 18px; }
.epc-cell.epc-pagetitle { justify-content: center; font-weight: 700; font-size: 18px; letter-spacing: 0; }
.epc-cell.blank { color: transparent; }

/* ===== 垂珠体/支谱：原版式，仅字号变化 ===== */
.ep { height: 100%; width: 100%; font-size: calc(var(--sg-fs, 16px) + 2px); color: #1a1a1a; box-sizing: border-box; display: flex; flex-direction: column; }
.ep-chuizhu { font-size: 24px; }
.ep-head { flex: 0 0 auto; text-align: center; font-size: 1.35em; font-weight: 700; letter-spacing: 6px; margin: 0 0 10px; }
.ep-count { font-size: 0.55em; font-weight: 400; letter-spacing: 1px; color: #6b5a44; margin-left: 8px; }
.ep-body { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: row-reverse; gap: 28px; }
.ep-chuizhu .ep-body { gap: 84px; }
.ep-half { flex: 1 1 0; min-width: 0; min-height: 0; overflow: hidden; }
.ep-item { margin: 0 0 0.85em; break-inside: avoid; }
.ep-year { font-size: 1.25em; font-weight: 700; letter-spacing: 1px; color: #8a3b2a; line-height: 1.3; }
.ep-title { font-size: 1.02em; font-weight: 700; margin: 0.05em 0 0.2em; letter-spacing: 2px; }
.ep-text { margin: 0; line-height: 1.6; text-align: justify; font-size: 0.95em; }
</style>
