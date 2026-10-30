<template>
  <!-- 內封（書名頁）：封面之後、譜序之前的一頁
       实物依据：參考圖集 实体族谱/题名-01（一頁只有书名大字＋雙邊框）、牌记-02（内封右/左下小字「敦本堂藏板」）
       版式口径：
         · 竖排居中书名（楷体大字，字号随字数自适应，绝不溢出）；
         · 雙線邊框由宿主頁的版框提供（三本谱书各自已有：.board+.bin / .cover+.cover-inner 皆为雙線）；
         · 右下角「X堂藏板」——堂号取自设置 tanghao，**没有堂号则整行不显示**；
         · 左下角小字「歲次XX　修次」（可选）。 -->
  <div class="itp" :style="{ '--itp-u': unit + 'px' }">
    <div class="itp-title"
         :style="{ fontSize: titleFs + 'px', letterSpacing: titleLs + 'px' }">{{ title }}</div>
    <div class="itp-date" v-if="yearLabel || xiuci">
      <div class="itp-y" v-if="yearLabel">{{ yearLabel }}</div>
      <div class="itp-x" v-if="xiuci">{{ xiuci }}</div>
    </div>
    <div class="itp-cang" v-if="cang">{{ cang }}</div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  title: { type: String, default: '族譜' },        // 书名（与封面同题）
  tanghao: { type: String, default: '' },          // 堂号；空 → 不显示「藏板」行
  yearLabel: { type: String, default: '' },        // 紀年，如「歲次丙午」；空则不显示
  xiuci: { type: String, default: '' },            // 修次，如「重修」；與紀年各占一竖列
  unit: { type: Number, default: 16 },             // 基准字号（px）：A3 横版 16、A3 竖版 ≈8.8
  pageH: { type: Number, default: 1123 },          // 宿主页内框高（px），用于书名自适应
})

const chars = computed(() => Array.from(String(props.title || '')))
const cang = computed(() => (props.tanghao ? String(props.tanghao).trim() + '藏板' : ''))

// 书名自适应：可用高 = 0.68 × 页内高；单字推进（字号+字距）× 字数 ≤ 可用高
const titleFs = computed(() => {
  const n = Math.max(1, chars.value.length)
  const avail = Math.max(120, props.pageH * 0.68)
  const cap = props.unit * 9.2                      // 设计字号（4 字时 = 封面同款）
  return Math.max(20, Math.round(Math.min(cap, (avail / n) * 0.82)))
})
const titleLs = computed(() => {
  const n = Math.max(1, chars.value.length)
  const avail = Math.max(120, props.pageH * 0.68)
  return Math.max(2, Math.round(Math.min(props.unit * 2.4, avail / n - titleFs.value)))
})
</script>

<style scoped>
.itp {
  position: relative; width: 100%; height: 100%; box-sizing: border-box;
  font-size: var(--itp-u, 16px);
  display: flex; align-items: center; justify-content: center;
  font-family: "Songti SC", "SimSun", "STSong", "宋体", serif;
  color: #1a1a1a; overflow: hidden;
}
.itp-title {
  writing-mode: vertical-rl; text-orientation: upright;
  font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
  font-weight: 700; line-height: 1; color: #111;
  padding: 0 0.18em; white-space: nowrap;
}
/* 左下小字：紀年一竖列 + 修次一竖列（由右而左），不再用全角空格免得自动换列 */
.itp-date {
  position: absolute; left: 8%; bottom: 10%;
  display: flex; flex-direction: row-reverse; align-items: flex-start; gap: 0.7em;
}
.itp-y, .itp-x {
  writing-mode: vertical-rl; text-orientation: upright;
  font-size: 1.35em; letter-spacing: 0.34em; color: #3a2a18; line-height: 1.2; white-space: nowrap;
}
/* 右下小字：X堂藏板（堂号为空则此行不渲染） */
.itp-cang {
  position: absolute; right: 8%; bottom: 10%;
  writing-mode: vertical-rl; text-orientation: upright;
  font-size: 1.75em; letter-spacing: 0.30em; color: #3a2a18; line-height: 1.2;
}
</style>
