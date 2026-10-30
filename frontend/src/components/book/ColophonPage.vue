<template>
  <!-- 牌記（版權頁）：全書末頁，三行小字（堂號／修次／紀年）＋「X堂藏板」
       实物依据：參考圖集 实体族谱/牌记-01（牌记三行式）、牌记-02（「敦本堂藏板」）
       口径：行数少反而庄重；無堂號時不寫「堂號」「藏板」兩行，只留修次與紀年。 -->
  <div class="clp" :style="{ '--clp-u': unit + 'px' }">
    <div class="clp-frame">
      <div class="clp-in">
        <div class="clp-head">牌　記</div>
        <div class="clp-lines">
          <div class="clp-line" v-if="tanghao">{{ tanghao }}</div>
          <div class="clp-line" v-if="xiuci">{{ xiuci }}</div>
          <div class="clp-line year" v-if="yearLabel">{{ yearLabel }}</div>
          <div class="clp-line cang" v-if="tanghao">{{ tanghao }}藏板</div>
        </div>
        <div class="clp-foot" v-if="note">{{ note }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  tanghao: { type: String, default: '' },       // 堂號；空 → 不寫堂號與藏板行
  xiuci: { type: String, default: '重修' },      // 修次（創修/初修/二修…）
  yearLabel: { type: String, default: '' },      // 紀年（歲次丙午／公元二〇二六年）
  note: { type: String, default: '' },           // 底注（可空）
  unit: { type: Number, default: 16 },           // 基準字號（px）
})
</script>

<style scoped>
.clp {
  width: 100%; height: 100%; box-sizing: border-box;
  display: flex; align-items: center; justify-content: center;
  font-size: var(--clp-u, 16px); color: #1a1a1a;
  font-family: "Songti SC", "SimSun", "STSong", "宋体", serif;
}
.clp-frame { border: 3px double #1a1a1a; padding: 0.45em; background: #fffdf6; }
.clp-in { border: 1px solid #1a1a1a; padding: 1.2em 1.6em 1em;
          display: flex; flex-direction: column; align-items: center; gap: 1.1em; }
.clp-head {
  writing-mode: vertical-rl; text-orientation: upright;
  font-size: 1.5em; font-weight: 700; letter-spacing: 0.3em; color: #111;
}
.clp-lines { display: flex; flex-direction: row-reverse; justify-content: center; gap: 1.15em; }
.clp-line {
  writing-mode: vertical-rl; text-orientation: upright;
  font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
  font-size: 1.45em; letter-spacing: 0.26em; line-height: 1.25;
}
.clp-line.year { font-size: 1.25em; color: #3a2a18; }
.clp-line.cang { font-size: 1.55em; color: #3a2a18; }
.clp-foot { font-size: 0.85em; color: #6b5a44; letter-spacing: 0.08em; }
</style>
