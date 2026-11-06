<template>
  <div class="page" :style="zoomStyle">
    <div class="board"><div class="bin">
      <div class="content">
        <!-- 内容区（谱图/文字），版心会盖在中间 84px 上 -->
        <div class="page-body"><slot /></div>
        <!-- 版心（中）：谱名 + 章节 + 页码 -->
        <div class="spine">
          <div class="spine-title">{{ title }}</div>
          <div class="spine-meta">{{ meta }}</div>
          <div class="spine-page">{{ pageNo }}</div>
        </div>
      </div>
      <!-- 书耳（右）：房派/章节 -->
      <div class="ear">
        <div class="ear-branch">{{ earText }}</div>
        <div class="ear-page" v-if="earPage">{{ earPage }}</div>
      </div>
    </div></div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
const props = defineProps({
  title: { type: String, default: '族譜' },      // 版心：谱名（竖排大字）
  meta: { type: String, default: '' },           // 版心：章节/世次
  pageNo: { type: [String, Number], default: '' },// 版心：页码
  earText: { type: String, default: '' },        // 书耳：房派
  earPage: { type: String, default: '' },        // 书耳：当前第X页
  zoom: { type: Number, default: 1 }
})
const zoomStyle = computed(() => (props.zoom === 1 ? '' : { zoom: props.zoom }))
</script>

<style scoped>
/* ===== A3 横版（与苏式垂珠体同一套） ===== */
.page { width: 1587px; height: 1123px; background: #fffdf6; display: flex; padding: 9px;
        color: #1a1a1a; font-family: "Songti SC", "SimSun", "STSong", "宋体", serif; }
.board { flex: 1; border: 1px solid #1a1a1a; padding: 5px; }
.bin   { border: 3px solid #1a1a1a; height: 100%; display: flex; }
.content { flex: 1; display: flex; flex-direction: column; min-width: 0; position: relative; }
.page-body { flex: 1; min-height: 0; overflow: hidden; }
.spine { position: absolute; top: 0; bottom: 0; left: 50%; transform: translateX(-50%); width: 84px;
         background: #fffdf6; border-left: 1px solid #1a1a1a; border-right: 1px solid #1a1a1a;
         display: flex; flex-direction: column; align-items: center; padding: 16px 0 10px; z-index: 2; }
.spine-title { writing-mode: vertical-rl; text-orientation: upright; font-size: 46px; font-weight: 700;
               letter-spacing: 12px; color: #111; }
.spine-meta  { writing-mode: vertical-rl; text-orientation: upright; font-size: 15px; letter-spacing: 6px;
               color: #333; margin-top: 18px; }
.spine-page  { margin-top: auto; writing-mode: vertical-rl; text-orientation: upright; font-size: 22px;
               font-weight: 700; letter-spacing: 2px; color: #111; }
.ear { flex: 0 0 44px; align-self: stretch; border-left: 1px solid #1a1a1a; border-right: 1px solid #1a1a1a;
       display: flex; flex-direction: column; align-items: center; padding-top: 44px; }
.ear-branch { writing-mode: vertical-rl; text-orientation: upright; font-size: 26px; font-weight: 700; letter-spacing: 9px; }
.ear-page { writing-mode: vertical-rl; text-orientation: upright; font-size: 15px; letter-spacing: 4px;
            color: #333; margin-top: 12px; }
</style>
