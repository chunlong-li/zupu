<template>
  <div class="gp" :class="['gp-' + variant, { 'gp-stack': stack }]">
    <header v-if="variant !== 'chuizhu' || stack" class="gp-head">
      <div class="gp-name">
        <span v-if="grave.generation" class="gp-gen">第{{ grave.generation }}世</span>
        {{ grave.owner_name || grave.member_name || grave.name }}
      </div>
      <div class="gp-sub">
        <span v-if="grave.birth">生 {{ grave.birth }}</span>
        <span v-if="grave.death">　卒 {{ grave.death }}</span>
        <span v-if="grave.address">　{{ grave.address }}</span>
      </div>
    </header>

    <template v-if="stack">
      <div class="gp-stack-text">
        <section v-if="grave.epitaph" class="gp-block">
          <h4>墓碑文</h4>
          <p>{{ clean(grave.epitaph) }}</p>
        </section>
        <section v-if="grave.mountain_orientation" class="gp-block">
          <h4>山向分金</h4>
          <p>{{ clean(grave.mountain_orientation) }}</p>
        </section>
        <section v-if="grave.remark" class="gp-block">
          <h4>附记</h4>
          <p>{{ clean(grave.remark) }}</p>
        </section>
      </div>
      <div v-if="images.length" class="gp-stack-media">
        <figure v-for="(im, i) in images" :key="i">
          <img :src="imgSrc(im.path)" :alt="im.cap" />
          <figcaption v-if="im.cap">{{ im.cap }}</figcaption>
        </figure>
      </div>
    </template>

    <div v-else class="gp-body">
      <div class="gp-half gp-right">
        <header v-if="variant === 'chuizhu'" class="gp-head">
          <div class="gp-name">
            <span v-if="grave.generation" class="gp-gen">第{{ grave.generation }}世</span>
            {{ grave.owner_name || grave.member_name || grave.name }}
          </div>
          <div class="gp-sub">
            <span v-if="grave.birth">生 {{ grave.birth }}</span>
            <span v-if="grave.death">　卒 {{ grave.death }}</span>
            <span v-if="grave.address">　{{ grave.address }}</span>
          </div>
        </header>

        <section v-if="grave.epitaph" class="gp-block">
          <h4>墓碑文</h4>
          <p>{{ clean(grave.epitaph) }}</p>
        </section>
        <section v-if="grave.mountain_orientation" class="gp-block">
          <h4>山向分金</h4>
          <p>{{ clean(grave.mountain_orientation) }}</p>
        </section>
      </div>

      <div class="gp-half gp-left">
        <section v-if="images.length" class="gp-block gp-images">
          <h4>墓碑照 · 墓域四至图</h4>
          <div class="gp-grid">
            <figure v-for="(im, i) in images" :key="i">
              <img :src="imgSrc(im.path)" :alt="im.cap" />
              <figcaption v-if="im.cap">{{ im.cap }}</figcaption>
            </figure>
          </div>
        </section>
        <section v-if="grave.remark" class="gp-block">
          <h4>附记</h4>
          <p>{{ clean(grave.remark) }}</p>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { thumbUrl } from '../../utils/thumb'

const props = defineProps({
  grave: { type: Object, required: true },
  variant: { type: String, default: 'dieji' },
  stack: { type: Boolean, default: false },   // 竖版：文字在上、图片在下
})

// 打印时 PNG（尤其 RGBA）会让 Chrome 光栅化极慢/卡住；PNG 走服务器 JPEG 缩略图，JPG 用原图
function imgSrc(p) { return p ? thumbUrl(p, 512) : '' }

function clean(s) { return String(s == null ? '' : s).replace(/\*\*/g, '') }

const images = computed(() => {
  const out = []
  for (const p of (props.grave.photos || [])) if (p) out.push({ path: p, cap: '墓碑照' })
  if (props.grave.boundary_image) out.push({ path: props.grave.boundary_image, cap: '墓域四至图' })
  return out
})
</script>

<style scoped>
.gp { height: 100%; width: 100%; font-size: var(--sg-fs, 17px); color: #1a1a1a; box-sizing: border-box; display: flex; flex-direction: column; }
/* 竖版茔墓：文字在上、图片在下（上下分布），图片尽量大 */
.gp.gp-stack { display: flex; flex-direction: column; }
.gp-stack-text { flex: 0 1 auto; }
.gp-stack-media { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; gap: 8px; margin-top: 4px; }
.gp-stack-media figure { flex: 1 1 0; min-height: 0; margin: 0; display: flex; align-items: center; justify-content: center; position: relative; }
.gp-stack-media img { display: block; width: 100%; height: 100%; object-fit: contain; background: #fff; }
.gp-stack-media figcaption { position: absolute; left: 0; right: 0; bottom: 2px; font-size: 0.66em; color: #6b5a44; text-align: center; letter-spacing: 1px; background: rgba(255,255,255,.82); padding: 1px 0; }
.gp-head { flex: 0 0 auto; text-align: center; margin: 0 0 10px; }
.gp-name { font-size: 1.42em; font-weight: 700; letter-spacing: 4px; line-height: 1.35; }
.gp-gen { font-size: 0.7em; font-weight: 400; color: #6b5a44; margin-right: 0.35em; letter-spacing: 2px; }
.gp-sub { font-size: 0.72em; color: #6b5a44; margin-top: 0.28em; letter-spacing: 1px; }

.gp-body { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: row-reverse; gap: 28px; }
.gp-chuizhu .gp-body { gap: 84px; }
.gp-half { flex: 1 1 0; min-width: 0; min-height: 0; overflow: hidden; }

.gp-block { margin: 0 0 0.8em; }
.gp-block h4 { font-size: 0.95em; font-weight: 700; letter-spacing: 3px; margin: 0 0 0.3em; padding-bottom: 2px; border-bottom: 1px solid #1a1a1a; }
.gp-block p { margin: 0 0 0.25em; line-height: 1.6; text-align: justify; }

.gp-images .gp-grid { display: flex; flex-direction: column; gap: 10px; }
.gp-images figure { margin: 0; text-align: center; }
.gp-images img {
  display: block; margin: 0 auto; max-width: 100%; max-height: 320px;
  object-fit: contain; border: 1px solid #b9ad91; background: #fff;
}
.gp-images figcaption { font-size: 0.7em; color: #6b5a44; margin-top: 3px; letter-spacing: 1px; }

/* 垂珠体茔墓：保持 1025.10 的“文字右、图片左（上下叠）”结构，只把左半图片尽量放大 */
.gp-chuizhu .gp-images .gp-grid { gap: 4px; }
.gp-chuizhu .gp-images figure { margin: 0; }
.gp-chuizhu .gp-images img { width: 100%; height: auto; max-height: 470px; max-width: 100%; object-fit: contain; }
.gp-chuizhu .gp-images figcaption { margin-top: 2px; }
</style>
