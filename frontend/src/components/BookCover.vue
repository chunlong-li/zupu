<template>
  <div class="bk" :class="[styleKey, side, famClass, { mirror: mirrored, landscape: props.landscape, portrait: !props.landscape, framed: cfg.frameGold, lbig: cfg.layout === 'big' }]" :style="rootStyle">
    <div class="tx"></div>
    <div class="frame" v-if="cfg.frameGold"></div>
    <div class="stitch" v-if="!cfg.noStitch"><i></i><i></i><i></i><i></i></div>

    <!-- ===== 封面 ===== -->
    <template v-if="side === 'front'">
      <div class="sign" v-if="cfg.layout === 'sign'" :style="signStyle">
        <div class="st" :class="{ gold: cfg.gold }">{{ title }}</div>
        <div class="svrow" v-if="!props.landscape && (tanghao || volume)">
          <div class="sv" v-if="volume">{{ volume }}</div>
          <div class="sv" v-if="tanghao">{{ tanghao }}</div>
        </div>
        <div class="sv" v-else-if="subText">{{ subText }}</div>
      </div>
      <div class="big" v-else>
        <div class="bt" :class="{ gold: cfg.gold }" :style="bigTitleStyle">{{ title }}</div>
        <div class="bs" v-if="subText" :class="{ gold: cfg.gold }">{{ subText }}</div>
      </div>
      <!-- 印章：始终保留（不再因书名长度隐藏）。竖版题签款本就把印章放在题签对侧，题签长高不会压到它 -->
      <div class="seal" :class="{ round: cfg.sealRound }" v-if="sealChar">{{ sealChar }}</div>
      <div class="foot">{{ footText }}</div>
    </template>

    <!-- ===== 封底 ===== -->
    <template v-else>
      <div class="colophon" v-if="backType === 'colophon'">
        <div class="ct">牌　記</div>
        <div class="cr"><span>譜名</span><span>{{ title }}</span></div>
        <div class="cr" v-if="colophon.volume"><span>卷次</span><span>{{ colophon.volume }}</span></div>
        <div class="cr" v-if="colophon.chief"><span>主修</span><span>{{ colophon.chief }}</span></div>
        <div class="cr" v-if="colophon.editor"><span>編纂</span><span>{{ colophon.editor }}</span></div>
        <div class="cr" v-if="colophon.press"><span>印製</span><span>{{ colophon.press }}</span></div>
        <div class="cr"><span>版次</span><span>{{ colophon.edition || (yearG ? `公元${yearG}年 ${xiuci}` : xiuci) }}</span></div>
        <div class="cnote">版權歸本族所有 · 翻印必究</div>
      </div>
      <div class="seal center" :class="{ round: cfg.sealRound }" v-else-if="backType === 'seal' && sealChar">{{ sealChar }}</div>
      <div class="foot back-foot">{{ backFoot }}</div>
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  styleKey: { type: String, default: 'indigo' },
  side: { type: String, default: 'front' },       // front | back
  base: { type: Number, default: 820 },           // 设计基准宽（px）
  ratio: { type: Number, default: 1.414 },        // 高 / 宽
  title: { type: String, default: '家族族譜' },
  volume: { type: String, default: '卷一' },
  subText: { type: String, default: '' },
  tanghao: { type: String, default: '' },
  junwang: { type: String, default: '' },
  zuji: { type: String, default: '' },
  surname: { type: String, default: '' },
  ganzhi: { type: String, default: '' },
  yearG: { type: [String, Number], default: '' },
  backType: { type: String, default: 'seal' },    // seal | plain | colophon
  colophon: { type: Object, default: () => ({}) },
  xiuci: { type: String, default: '重修' },        // 修次（創修/初修/二修/…/重修/續修/自定义）
  mirror: { type: Boolean, default: false },      // 镜像：装订线/题签/印章左右对调
  landscape: { type: Boolean, default: false },    // 横版：宽 > 高，布局横向
})

const STYLES = {
  indigo:  { fam: 'cloth', c1: '#2b4d80', c2: '#16304f', layout: 'sign', thread: '#f2eee2', seal: '#a52020' },
  navy:    { fam: 'cloth', c1: '#17304f', c2: '#08131f', layout: 'big',  gold: true, thread: 'gold', seal: '#c9a03c' },
  crimson: { fam: 'cloth', c1: '#95262a', c2: '#5e1313', layout: 'sign', thread: '#f5e6c8', seal: '#f0d79a', sealRound: true },
  jade:    { fam: 'cloth', c1: '#255040', c2: '#123026', layout: 'sign', thread: '#eef3ea', seal: '#c9a03c' },
  bronze:  { fam: 'cloth', c1: '#5f4429', c2: '#3a2716', layout: 'sign', gold: true, frameGold: true, thread: 'gold', seal: '#a52020' },
  paper:   { fam: 'paper', c1: '#f7f0dc', c2: '#e6d9bd', layout: 'big',  frameInk: true, noStitch: true, seal: '#a52020' },
}

const cfg = computed(() => STYLES[props.styleKey] || STYLES.indigo)
const nTitle = computed(() => Array.from(String(props.title || '')).length)
// 题签书名自适应（保证任何长度都不溢出题签框）：
//   竖版（题签高 0.64u、内边距 2×0.018u、书名行高≈0.865×字号）→
//     可用高 = 0.604u −（下方堂号/卷号一竖 0.15u + 间隙），字号 = 可用高 / (0.865 × 字数)
//   横版（题签高 0.64u、书名横排）→ 同样按可用高 / (0.865 × 字数)；宽度侧另有富余
//   另：长标题（≥6 字）时收紧与堂号之间的间隙，避免互相挤压
const signStyle = computed(() => {
  const n = Math.max(1, Array.from(String(props.title || '')).length)
  // ⚠️ --u = 设计基准「宽」；封面高 = ratio×u（竖版 1.414u、横版 0.76u）。
  //    题签 top/height 百分比是相对封面高的，字号 calc(var(--u)*k) 是相对宽的 → 除以 ratio 才能同尺度。
  const RATIO = props.landscape ? 0.76 : 1.414
  const uToH = (v) => `calc(var(--u) * ${(v / RATIO).toFixed(4)})`
  const PAD = 0.040     // 上下内边距合计（封面高比例，实测 0.0398）
  const SUB = 0.20      // 卷次+堂号两竖并排（封面高比例；实测含内边距后 ≤0.17，取 0.20 留余量）
  const LS = 0.022      // 字距：CSS 写 0.022u（宽）→ 封面高比例 0.022×ratio
  const LS_H = LS * RATIO
  const A = 0.50        // 「字号→单字实高」斜率（实测 0.38~0.44，留安全余量）
  const CAP_H = 0.115 * RATIO
  // 题签高上限：≤4 字保持原设计 0.64；≥5 字需长高容纳「书名+堂号」，此时印章隐去 → 可到 0.80
  const boxCap = n <= 4 ? 0.64 : 0.80
  const GAP_LO = 0.010, GAP_HI = 0.07
  const room = boxCap - PAD - SUB - GAP_LO
  const unitAdv = A + LS_H / CAP_H                   // 以「原设计字号」为基准的单字实高系数
  // ≤6 字：字号不缩（与原 4 字相同）；>6 字才缩
  const fsH = n <= 6 ? CAP_H : Math.max(0.02, Math.min(CAP_H, (room / (n * unitAdv)) * 0.96))
  const gap = n <= 4 ? GAP_HI : GAP_LO
  const adv = fsH * A + LS_H
  // 4 字框高固定 0.64（外观不变）；5 字起按内容长高（封顶 boxCap）
  const boxH = n <= 4 ? 0.64 : Math.min(boxCap, Math.max(0.64, PAD + n * adv + SUB + gap))
  return {
    '--st-fs': uToH(fsH),
    '--sign-gap': uToH(gap),
    '--sign-h': `${(boxH * 100).toFixed(2)}%`,
  }
})
// 大字款（② 藏青烫金 / ⑥ 米黄仿古·宣纸，无题签）：书名按封面高自适应
//   实测（u=525 基准、竖版）：标题墨迹高 ≈ 1.19×字号×字数；封面高 = (竖版 1.414 / 横版 0.76)×u
//   底部「岁次/修次」行占下部约 0.06u（含内边距），顶部也需留白 → 标题可用高按 0.72u（横版 0.55u）
//   ≤4 字保持原设计 0.17u；≥5 字才缩小（属主：②⑥ 字号要小一点，别再压到时间行）
const bigTitleStyle = computed(() => {
  const n = Math.max(1, Array.from(String(props.title || '')).length)
  if (props.landscape) {
    // 横排：单字推进 = 字号 + 字距（字距一并收缩）；原设计字号 0.10u
    const cap = 0.10
    const avail = 0.84
    const fs = n <= 4 ? cap : Math.min(cap, (avail / (n * 1.62)) * 0.97)
    const ls = n <= 4 ? 0.05 : Math.min(0.05, Math.max(0.015, (avail - n * fs) / Math.max(1, n - 1)))
    return { '--bt-fs': `calc(var(--u) * ${fs.toFixed(4)})`, '--bt-ls': `calc(var(--u) * ${ls.toFixed(4)})` }
  }
  // 竖版（② 藏青烫金 / ⑥ 米黄仿古·宣纸）：逐字数实测标定（保持上一轮已验收的现状）
  //   6 字 0.115u→距时间行 22.8px；8/10 字 0.07/0.044u→32.8px；12 字 0.034u→+6.8px（不重叠）
  const TABLE = { 4: 0.17, 5: 0.14, 6: 0.115, 7: 0.09, 8: 0.07, 9: 0.048, 10: 0.044, 11: 0.038, 12: 0.034 }
  let fs
  if (n <= 4) fs = 0.17
  else if (TABLE[n] !== undefined) fs = TABLE[n]
  else fs = Math.max(0.030, TABLE[12] - (n - 12) * 0.001)
  return { '--bt-fs': `calc(var(--u) * ${fs.toFixed(4)})` }
})
const famClass = computed(() => cfg.value.fam)
// 封面与封底分居书脊两侧：封底的镜像方向与封面相反，装订线才左右对称
const mirrored = computed(() => props.side === 'back' ? !props.mirror : props.mirror)
const sealChar = computed(() => (props.surname || '').trim().charAt(0))
const subText = computed(() => props.subText || [props.tanghao, props.volume].filter(Boolean).join(' · '))
const footText = computed(() => {
  const a = props.yearG ? `公元${props.yearG}年` : ''
  const xc = (props.xiuci || '').trim() || '重修'
  const b = props.ganzhi ? `${props.ganzhi}　${xc}` : ''
  return [a, b].filter(Boolean).join('　') || xc
})
const backFoot = computed(() => {
  const y = props.yearG ? `公元${props.yearG}年` : ''
  return [props.tanghao ? props.tanghao + ' 藏' : '', y].filter(Boolean).join('　')
})

const rootStyle = computed(() => ({
  '--u': props.base + 'px',
  '--ar': props.landscape ? 0.707 : props.ratio,
  '--c1': cfg.value.c1,
  '--c2': cfg.value.c2,
  ...(cfg.value.thread && cfg.value.thread !== 'gold' ? { '--thread': cfg.value.thread } : {}),
}))
</script>

<style scoped>
/* 全部尺寸以 --u（基准宽）为基准 calc，随 --u 等比缩放 */
.bk {
  position: relative; width: var(--u); height: calc(var(--u) * var(--ar));
  overflow: hidden; color: #fff; border-radius: calc(var(--u) * 0.004);
  -webkit-print-color-adjust: exact; print-color-adjust: exact;
}
.bk.cloth {
  background-image:
    repeating-linear-gradient(0deg, rgba(0,0,0,.07) 0 1px, transparent 1px 3px),
    repeating-linear-gradient(90deg, rgba(0,0,0,.06) 0 1px, transparent 1px 3px),
    radial-gradient(120% 120% at 20% 10%, rgba(255,255,255,.12), transparent 50%),
    linear-gradient(160deg, var(--c1), var(--c2));
}
.bk.paper {
  color: #241b0e;
  background-image:
    repeating-linear-gradient(48deg, rgba(120,90,40,.05) 0 2px, transparent 2px 6px),
    repeating-linear-gradient(-48deg, rgba(120,90,40,.04) 0 2px, transparent 2px 7px),
    radial-gradient(140% 140% at 12% 0%, rgba(255,255,255,.7), transparent 55%),
    linear-gradient(160deg, var(--c1), var(--c2));
}
/* 体积高光 */
.tx {
  position: absolute; inset: 0; pointer-events: none; z-index: 5;
  background:
    radial-gradient(130% 130% at 0% 0%, rgba(255,255,255,.14), transparent 42%),
    linear-gradient(90deg, transparent 88%, rgba(0,0,0,.28) 100%);
}
.bk.paper .tx { background: radial-gradient(120% 120% at 0% 0%, rgba(255,255,255,.35), transparent 45%),
                            linear-gradient(90deg, transparent 90%, rgba(120,90,40,.14)); }

/* 线装针眼 + 白线 */
.stitch { position: absolute; right: 0; top: 0; bottom: 0; width: calc(var(--u) * 0.1); z-index: 6; }
.stitch i { position: absolute; right: calc(var(--u) * 0.015); width: calc(var(--u) * 0.066);
  height: calc(var(--u) * 0.009); border-radius: 2px; background: var(--thread, #f2eee2);
  box-shadow: 0 0 1px rgba(0,0,0,.5); }
.stitch i::after { content: ''; position: absolute; right: calc(var(--u) * -0.015); top: calc(var(--u) * -0.003);
  width: calc(var(--u) * 0.015); height: calc(var(--u) * 0.015); border-radius: 50%; background: rgba(0,0,0,.45); }
.stitch i:nth-child(1) { top: 12%; } .stitch i:nth-child(2) { top: 37%; }
.stitch i:nth-child(3) { top: 63%; } .stitch i:nth-child(4) { top: 88%; }
.stitch::after { content: ''; position: absolute; right: calc(var(--u) * 0.033); width: calc(var(--u) * 0.006);
  top: 0; bottom: 0; background: var(--thread, #f2eee2); opacity: .9; }
.navy .stitch i, .bronze .stitch i { background: linear-gradient(90deg, #f7e9a8, #c9a03c); }
.navy .stitch::after, .bronze .stitch::after { background: linear-gradient(180deg, #f7e9a8, #c9a03c); }

/* 烫金字 */
.gold {
  background: linear-gradient(160deg,#f7e9a8 0%,#d9b552 30%,#8f6a1e 52%,#f0d97e 72%,#b98d2c 100%);
  -webkit-background-clip: text; background-clip: text; color: transparent;
}

/* 题签 */
.sign { position: absolute; top: 6%; left: 11%; width: 30%; height: var(--sign-h, 64%);
  background: #f4ecd6; border: calc(var(--u) * 0.005) solid #20190f; padding: calc(var(--u) * 0.018);
  z-index: 4; box-shadow: 0 calc(var(--u)*0.012) calc(var(--u)*0.03) rgba(0,0,0,.35);
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: var(--sign-gap, calc(var(--u) * 0.07)); }
.sign::before { content: ''; position: absolute; inset: calc(var(--u) * 0.012);
  border: 1px solid #20190f; pointer-events: none; }
.sign .st { writing-mode: vertical-rl; text-orientation: upright; text-align: center;
  font-family: "KaiTi","STKaiti","Kaiti SC","SimSun",serif;
  font-size: min(var(--st-fs, calc(var(--u) * 0.115)), calc(var(--u) * 0.115)); font-weight: 700; letter-spacing: calc(var(--u) * 0.022);
  white-space: nowrap; line-height: 1; color: #191307; }
.sign .sv { writing-mode: vertical-rl; text-orientation: upright; white-space: nowrap;
  font-size: calc(var(--u) * 0.04); letter-spacing: calc(var(--u) * 0.01); color: #3a2c14; }
/* 竖版题签款：堂号与卷号拆成左右两竖（无分隔点） */
.svrow { width: 100%; display: flex; flex-direction: row; justify-content: center;
  gap: calc(var(--u) * 0.022); align-items: flex-start; flex: none; }
/* 竖排副信息绝不压缩/换行（压缩会让竖排文字折行、整体变高顶出题签框底） */
.sign .svrow > .sv { flex: none; }
.sign .st { flex: none; }
.bronze .sign { background: #5f4429; border-color: #f0e1b4; box-shadow: none; }
.bronze .sign::before { border-color: #f0e1b4; }

/* 无题签大字 */
.big { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center;
  gap: calc(var(--u) * 0.08); z-index: 4; }
.big .bt { writing-mode: vertical-rl; text-orientation: upright; font-weight: 700;
  font-family: "KaiTi","STKaiti","Kaiti SC","SimSun",serif;
  font-size: min(var(--bt-fs, calc(var(--u) * 0.17)), calc(var(--u) * 0.17)); letter-spacing: calc(var(--u) * 0.05); }
.big .bs { writing-mode: vertical-rl; text-orientation: upright; font-size: calc(var(--u) * 0.055);
  letter-spacing: calc(var(--u) * 0.025); opacity: .92; }

/* 印章 */
.seal { position: absolute; left: 11%; bottom: 12%; width: calc(var(--u) * 0.155); height: calc(var(--u) * 0.155);
  border: calc(var(--u) * 0.009) solid currentColor; border-radius: calc(var(--u) * 0.014);
  display: flex; align-items: center; justify-content: center; color: #a52020;
  font-size: calc(var(--u) * 0.085); font-weight: 700; opacity: .9; z-index: 4; }
.seal.round { border-radius: 50%; font-size: calc(var(--u) * 0.06); }
.seal.center { left: 50%; transform: translateX(-50%); top: 42%; bottom: auto; }
.sign ~ .seal { color: var(--seal-color, #a52020); }
/* 竖版题签款：印章移到题签（家族名）对侧，避免与家族名同一竖列 */
.bk.portrait:not(.lbig) .seal:not(.center) { left: auto; right: 11%; }
.bk.portrait.mirror:not(.lbig) .seal:not(.center) { right: auto; left: 11%; }

/* 底部信息 */
.foot { position: absolute; bottom: 5.5%; left: 0; right: 0; text-align: center;
  font-size: calc(var(--u) * 0.042); letter-spacing: calc(var(--u) * 0.016); z-index: 4; }
/* 有外框（古铜函套）时底部文字上移，避免压框 */
.bk.framed .foot { bottom: 8.5%; }

/* 边框 */
.frame { position: absolute; inset: calc(var(--u) * 0.042); border: calc(var(--u)*0.004) solid rgba(240,225,180,.85); z-index: 3; }
.frame::before { content: ''; position: absolute; inset: calc(var(--u) * 0.014); border: 1px solid rgba(240,225,180,.55); }

/* 牌记 */
.colophon { position: absolute; top: 14%; left: 13%; right: 13%; bottom: 12%;
  background: #f4ecd6; border: calc(var(--u)*0.004) solid #20190f; padding: calc(var(--u)*0.045);
  z-index: 4; color: #241b0e; box-shadow: 0 calc(var(--u)*0.012) calc(var(--u)*0.03) rgba(0,0,0,.3); }
.colophon::before { content: ''; position: absolute; inset: calc(var(--u)*0.012); border: 1px solid #20190f; }
.colophon .ct { text-align: center; font-family: "KaiTi","STKaiti",serif; font-size: calc(var(--u)*0.06);
  letter-spacing: calc(var(--u)*0.03); font-weight: 700; margin-bottom: calc(var(--u)*0.04); }
.colophon .cr { display: flex; justify-content: space-between; font-size: calc(var(--u)*0.035);
  letter-spacing: 1px; padding: calc(var(--u)*0.014) 2px; border-bottom: 1px dashed rgba(40,30,15,.35); }
.colophon .cr span:first-child { color: #6a5a3a; }
.colophon .cnote { margin-top: calc(var(--u)*0.035); font-size: calc(var(--u)*0.03); line-height: 1.9;
  letter-spacing: 1px; color: #4a3d28; text-align: center; }

/* 镜像：装订线/题签/印章左右对调（文字不翻转） */
.bk.mirror .stitch { left: 0; right: auto; }
.bk.mirror .stitch i { left: calc(var(--u) * 0.015); right: auto; }
.bk.mirror .stitch i::after { left: calc(var(--u) * -0.015); right: auto; }
.bk.mirror .stitch::after { left: calc(var(--u) * 0.033); right: auto; }
.bk.mirror .sign { right: 11%; left: auto; }
.bk.mirror .seal:not(.center) { right: 11%; left: auto; }
.bk.mirror .tx { transform: scaleX(-1); }

/* ===== 横版 ===== */
/* 装订线：保持右侧竖向，占宽 10%（与竖版一致） */
.bk.landscape .stitch { right: 0; left: auto; top: 0; bottom: 0; width: calc(var(--u) * 0.1);
  height: auto; transform: none; }
.bk.landscape .stitch i { right: calc(var(--u) * 0.015); left: auto; width: calc(var(--u) * 0.066);
  height: calc(var(--u) * 0.009); border-radius: 2px; background: var(--thread, #f2eee2);
  box-shadow: 0 0 1px rgba(0,0,0,.5); transform: none; }
.bk.landscape .stitch i::after { content: ''; position: absolute; right: calc(var(--u) * -0.015);
  top: calc(var(--u) * -0.003); width: calc(var(--u) * 0.015); height: calc(var(--u) * 0.015);
  border-radius: 50%; background: rgba(0,0,0,.45); }
.bk.landscape .stitch::after { content: ''; position: absolute; right: calc(var(--u) * 0.033);
  left: auto; width: calc(var(--u) * 0.006); top: 0; bottom: 0;
  background: var(--thread, #f2eee2); opacity: .9; }
.bk.landscape .stitch i:nth-child(1) { top: 12%; } .bk.landscape .stitch i:nth-child(2) { top: 37%; }
.bk.landscape .stitch i:nth-child(3) { top: 63%; } .bk.landscape .stitch i:nth-child(4) { top: 88%; }

/* 横版 · 题签：左右对称居中（两侧各 12%，右侧避开装订线），垂直居中偏上，家族名居中、卷次换行 */
.bk.landscape .sign { top: 45%; left: 12%; right: 12%; width: auto; height: auto;
  transform: translateY(-50%);
  padding: calc(var(--u) * 0.024) calc(var(--u) * 0.02);
  flex-direction: column; align-items: center; justify-content: center;
  gap: calc(var(--u) * 0.012); }
.bk.landscape .sign .st { writing-mode: horizontal-tb; text-orientation: mixed; text-align: center;
  white-space: nowrap; font-size: calc(var(--u) * 0.078); letter-spacing: calc(var(--u) * 0.032); }
.bk.landscape .sign .sv { writing-mode: horizontal-tb; text-orientation: mixed; text-align: center;
  white-space: nowrap; font-size: calc(var(--u) * 0.038); letter-spacing: calc(var(--u) * 0.014); }

/* 横版 · 大字：横排居中（无题签模式）——整体上移，避免过于偏下 */
.bk.landscape .big { flex-direction: column; gap: calc(var(--u) * 0.025); transform: translateY(-9%); }
.bk.landscape .big .bt { writing-mode: horizontal-tb; text-orientation: mixed; text-align: center;
  white-space: nowrap; font-size: min(var(--bt-fs, calc(var(--u) * 0.1)), calc(var(--u) * 0.1));
  letter-spacing: var(--bt-ls, calc(var(--u) * 0.05)); }
.bk.landscape .big .bs { writing-mode: horizontal-tb; text-orientation: mixed; text-align: center;
  white-space: nowrap; font-size: calc(var(--u) * 0.04); letter-spacing: calc(var(--u) * 0.025); }

/* 横版 · 印章：卷二下方、年份上方，水平居中（与族名同一中轴） */
.bk.landscape .seal { left: 50%; right: auto; top: 68%; bottom: auto;
  transform: translateX(-50%);
  width: calc(var(--u) * 0.085); height: calc(var(--u) * 0.085);
  font-size: calc(var(--u) * 0.048); }
.bk.landscape .seal.round { font-size: calc(var(--u) * 0.038); }
/* 横版 · 无题签（大字）款式的印章也上移，与整体协调 */
.bk.landscape.lbig .seal { top: 64%; }

/* 横版 · 封底印章：居中 */
.bk.landscape .seal.center { left: 50%; right: auto; top: 50%; bottom: auto;
  transform: translate(-50%, -50%);
  width: calc(var(--u) * 0.14); height: calc(var(--u) * 0.14);
  font-size: calc(var(--u) * 0.075); }
.bk.landscape .seal.center.round { font-size: calc(var(--u) * 0.055); }

/* 横版 · 底部信息 */
.bk.landscape .foot { bottom: 7%; font-size: calc(var(--u) * 0.028); letter-spacing: calc(var(--u) * 0.01);
  text-align: center; }
.bk.landscape.framed .foot { bottom: 9%; }

/* 横版 · 牌记：缩小、左右对称（避开装订线） */
.bk.landscape .colophon { left: 12%; right: 12%; top: 12%; bottom: 14%; padding: calc(var(--u) * 0.03); }
.bk.landscape .colophon .ct { font-size: calc(var(--u) * 0.046); letter-spacing: calc(var(--u) * 0.02);
  margin-bottom: calc(var(--u) * 0.022); }
.bk.landscape .colophon .cr { font-size: calc(var(--u) * 0.026); padding: calc(var(--u) * 0.009) 2px; }
.bk.landscape .colophon .cnote { font-size: calc(var(--u) * 0.021); margin-top: calc(var(--u) * 0.018); }

/* 横版 · 高光适配 */
.bk.landscape .tx {
  background: radial-gradient(120% 120% at 10% 10%, rgba(255,255,255,.15), transparent 50%),
              linear-gradient(180deg, transparent 88%, rgba(0,0,0,.2) 100%); }

/* 横版 · 镜像：装订线/题签/印章左右对调 */
.bk.landscape.mirror .stitch { left: 0; right: auto; }
.bk.landscape.mirror .stitch i { left: calc(var(--u) * 0.015); right: auto; }
.bk.landscape.mirror .stitch i::after { left: calc(var(--u) * -0.015); right: auto; }
.bk.landscape.mirror .stitch::after { left: calc(var(--u) * 0.033); right: auto; }
.bk.landscape.mirror .sign { left: 12%; right: 12%; }
.bk.landscape.mirror .seal:not(.center) { left: 50%; right: auto; }
</style>
