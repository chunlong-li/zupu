<template>
  <div class="sg" :class="['sg-' + variant, 'sg-' + mode]">
    <template v-if="mode === 'image'">
      <div class="sg-plate" :class="[plateClass]">
        <figure v-for="(im, i) in images" :key="i">
          <img :src="imgSrc(im)" :alt="imageKind === 'plan' ? '平面布局图' : '实景图'" />
          <figcaption>{{ imageKind === 'plan' ? '平面布局图' : '实景图' }}</figcaption>
        </figure>
      </div>
    </template>

    <template v-else>
      <header v-if="variant !== 'chuizhu'" class="sg-head">
        <div class="sg-name">{{ shrine.name }}</div>
        <div class="sg-sub">〔{{ shrine.shrine_type || '祠堂' }}〕<template v-if="shrine.address">　{{ shrine.address }}</template><template v-if="part === '2'">　（续）</template></div>
      </header>

      <div class="sg-body">
        <div class="sg-half sg-right">
          <header v-if="variant === 'chuizhu'" class="sg-head">
            <div class="sg-name">{{ shrine.name }}</div>
            <div class="sg-sub">〔{{ shrine.shrine_type || '祠堂' }}〕<template v-if="shrine.address">　{{ shrine.address }}</template><template v-if="part === '2'">　（续）</template></div>
          </header>

          <template v-if="part === '1'">
            <section v-if="history.length" class="sg-block">
              <h4>始建重修</h4>
              <p v-for="h in history" :key="h.id"><span class="sg-year">{{ clean(h.year) }}</span>　{{ clean(h.event) }}<template v-if="h.note">　{{ clean(h.note) }}</template></p>
            </section>
            <section v-if="shrine.architecture" class="sg-block">
              <h4>建筑规制</h4>
              <p>{{ clean(shrine.architecture) }}</p>
            </section>
          </template>

          <template v-else-if="part === '2'">
            <section v-if="shrine.ritual" class="sg-block">
              <h4>祭祀仪轨</h4>
              <p>{{ clean(shrine.ritual) }}</p>
            </section>
          </template>

          <template v-else>
            <section v-if="history.length" class="sg-block">
              <h4>始建重修</h4>
              <p v-for="h in history" :key="h.id"><span class="sg-year">{{ clean(h.year) }}</span>　{{ clean(h.event) }}<template v-if="h.note">　{{ clean(h.note) }}</template></p>
            </section>
            <section v-if="shrine.architecture" class="sg-block">
              <h4>建筑规制</h4>
              <p>{{ clean(shrine.architecture) }}</p>
            </section>
            <section v-if="shrine.ritual" class="sg-block">
              <h4>祭祀仪轨</h4>
              <p>{{ clean(shrine.ritual) }}</p>
            </section>
            <section v-if="shrine.spirit_layout" class="sg-block">
              <h4>神位排布</h4>
              <p>{{ clean(shrine.spirit_layout) }}</p>
            </section>
          </template>
        </div>

        <div class="sg-half sg-left">
          <template v-if="part === '1'">
            <section v-if="shrine.spirit_layout" class="sg-block">
              <h4>神位排布</h4>
              <p>{{ clean(shrine.spirit_layout) }}</p>
            </section>
          </template>

          <template v-else-if="part === '2'">
            <section v-if="properties.length" class="sg-block">
              <h4>族产公田</h4>
              <p v-for="p in properties" :key="p.id">{{ clean(p.name) }}<template v-if="p.area">　{{ clean(p.area) }}</template><template v-if="p.note">　{{ clean(p.note) }}</template></p>
            </section>
            <section v-if="shrine.remark" class="sg-block">
              <h4>附记</h4>
              <p>{{ clean(shrine.remark) }}</p>
            </section>
            <section v-if="couplets.length" class="sg-block">
              <h4>楹联匾额</h4>
              <div v-for="c in couplets" :key="c.id" class="sg-couplet-item">
                <div v-if="c.kind === '匾额'" class="sg-plaque">{{ clean(c.content) }}</div>
                <div v-else-if="c.kind === '对联'" class="sg-couplet">
                  <span class="sg-c-line up">{{ splitCouplet(c.content).up }}</span>
                  <span class="sg-c-line low">{{ splitCouplet(c.content).low }}</span>
                </div>
                <div v-else class="sg-other">{{ clean(c.content) }}</div>
                <div v-if="c.position" class="sg-cap">{{ c.position }}</div>
              </div>
            </section>
          </template>

          <template v-else>
            <section v-if="properties.length" class="sg-block">
              <h4>族产公田</h4>
              <p v-for="p in properties" :key="p.id">{{ clean(p.name) }}<template v-if="p.area">　{{ clean(p.area) }}</template><template v-if="p.note">　{{ clean(p.note) }}</template></p>
            </section>
            <section v-if="shrine.remark" class="sg-block">
              <h4>附记</h4>
              <p>{{ clean(shrine.remark) }}</p>
            </section>
            <section v-if="couplets.length" class="sg-block">
              <h4>楹联匾额</h4>
              <div v-for="c in couplets" :key="c.id" class="sg-couplet-item">
                <div v-if="c.kind === '匾额'" class="sg-plaque">{{ clean(c.content) }}</div>
                <div v-else-if="c.kind === '对联'" class="sg-couplet">
                  <span class="sg-c-line up">{{ splitCouplet(c.content).up }}</span>
                  <span class="sg-c-line low">{{ splitCouplet(c.content).low }}</span>
                </div>
                <div v-else class="sg-other">{{ clean(c.content) }}</div>
                <div v-if="c.position" class="sg-cap">{{ c.position }}</div>
              </div>
            </section>
            <figure v-if="inlinePhoto" class="sg-inline-photo">
              <img :src="imgSrc(inlinePhoto)" alt="实景图" />
              <figcaption>实景图</figcaption>
            </figure>
            <p v-if="!inlinePhoto && images.length" class="sg-next">（图版见次页）</p>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { thumbUrl } from '../../utils/thumb'

const props = defineProps({
  shrine: { type: Object, required: true },
  mode: { type: String, default: 'text' },     // text | image
  part: { type: String, default: 'all' },      // all | 1 | 2
  variant: { type: String, default: 'dieji' }, // chuizhu | dieji | branch
  inlinePhoto: { type: String, default: '' },
  images: { type: Array, default: () => [] },
  imageKind: { type: String, default: 'photo' }, // photo | plan
  layout: { type: String, default: 'rows' },      // 按书版向：竖版 rows / 横版 cols
})

const history = computed(() => (props.shrine.history || []).filter(h => h && (h.year || h.event || h.note)))
const properties = computed(() => (props.shrine.properties || []).filter(p => p && (p.name || p.area || p.note)))
const couplets = computed(() => (props.shrine.couplets || []).filter(c => c && c.content))
const images = computed(() => props.images || [])
// 按书版向（页面/版心宽高比）由父视图显式指定：横版 cols（左右），竖版 rows（上下）。
const plateClass = computed(() => images.value.length < 2 ? 'n1' : (props.layout === 'cols' ? 'cols' : 'rows'))

function imgSrc(p) { return p ? thumbUrl(p, 512) : '' }
function clean(s) { return String(s == null ? '' : s).replace(/\*\*/g, '') }

function splitCouplet(s) {
  const raw = clean(s).replace(/^\s*(?:对联|联|匾额)\s*[：:]\s*/, '').trim()
  const parts = raw.split(/[；;]/).map(x => x.trim()).filter(Boolean)
  if (parts.length >= 2) return { up: parts[0], low: parts.slice(1).join('；') }
  return { up: raw, low: '' }
}
</script>

<style scoped>
.sg { height: 100%; width: 100%; font-size: var(--sg-fs, 17px); color: #1a1a1a; box-sizing: border-box; display: flex; flex-direction: column; }
.sg-head { flex: 0 0 auto; text-align: center; margin: 0 0 10px; }
.sg-name { font-size: 1.45em; font-weight: 700; letter-spacing: 5px; line-height: 1.35; }
.sg-sub { font-size: 0.72em; color: #6b5a44; margin-top: 0.25em; letter-spacing: 1px; }

.sg-body { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: row-reverse; gap: 28px; }
.sg-chuizhu .sg-body { gap: 84px; }
.sg-half { flex: 1 1 0; min-width: 0; min-height: 0; overflow: hidden; }

.sg-block { margin: 0 0 0.55em; }
.sg-block h4 { font-size: 0.95em; font-weight: 700; letter-spacing: 3px; margin: 0 0 0.3em; padding-bottom: 2px; border-bottom: 1px solid #1a1a1a; }
.sg-block p { margin: 0 0 0.25em; line-height: 1.58; text-align: justify; }
.sg-year { font-weight: 700; }
.sg-next { font-size: 0.7em; color: #6b5a44; margin: 0.5em 0 0; }

/* 匾额：横排、单行、居中、细边框 */
.sg-plaque {
  display: block; width: max-content; max-width: 100%; margin: 0.3em auto 0.15em;
  padding: 0.2em 0.8em; box-sizing: border-box;
  border: 2px solid #1a1a1a; background: #fbf6ea;
  font-size: 1.05em; letter-spacing: 0.3em; text-indent: 0.3em;
  text-align: center; line-height: 1.5; white-space: nowrap;
}
/* 对联：竖排双行，上联在右、下联在左 */
.sg-couplet {
  display: flex; flex-direction: row-reverse; justify-content: center; align-items: flex-start;
  gap: 0.75em; margin: 0.15em 0 0.1em; min-height: 6.2em;
}
.sg-c-line {
  writing-mode: vertical-rl; text-orientation: upright;
  font-size: 1.06em; letter-spacing: 0.1em; line-height: 1.25;
  border-right: 1px solid #d8cdb8; padding-right: 0.15em;
}
.sg-c-line:first-child { border-right: none; padding-right: 0; }
.sg-other { font-size: 0.92em; margin: 0.25em 0; line-height: 1.55; }
.sg-cap { font-size: 0.66em; color: #6b5a44; text-align: center; margin-top: 0.05em; letter-spacing: 1px; }

/* 文短时图随文走：页内小图 */
.sg-inline-photo { margin: 0.8em 0 0; text-align: center; }
.sg-inline-photo img { display: block; margin: 0 auto; max-width: 92%; max-height: 260px; object-fit: contain; border: 1px solid #b9ad91; background: #fff; }
.sg-inline-photo figcaption { font-size: 0.7em; color: #6b5a44; margin-top: 3px; letter-spacing: 1px; }

/* 图版页：1 列满宽；单张占 86%，两张各 46%（填满版心） */
.sg-image { padding: 0; }
.sg-plate { height: 100%; width: 100%; gap: 8px; align-items: center; justify-items: center; }
/* 2 张横图/混合：上下排，每张满版心宽，各约 45% 页高 */
.sg-plate.rows { display: flex; flex-direction: column; justify-content: center; }
.sg-plate.rows figure { margin: 0; width: 100%; flex: 1 1 0; min-height: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; }
.sg-plate.rows img { width: 100%; height: calc(100% - 14px); object-fit: contain; }
/* 2 张竖图：左右并排 */
.sg-plate.cols { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: center; }
.sg-plate.cols figure { margin: 0; width: 100%; height: 100%; min-height: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; }
.sg-plate.cols img { width: 100%; height: calc(100% - 14px); object-fit: contain; }
/* 单张（奇数最后一张/平面图）：独占整行，满版心 */
.sg-plate.n1 { display: flex; flex-direction: column; justify-content: center; }
.sg-plate.n1 figure { margin: 0; width: 100%; height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; }
.sg-plate.n1 img { width: 100%; height: calc(100% - 14px); object-fit: contain; }
.sg-plate img { display: block; margin: 0 auto; object-fit: contain; background: #fff; }
.sg-plate figcaption { font-size: 0.7em; color: #6b5a44; margin-top: 3px; letter-spacing: 1px; }
</style>
