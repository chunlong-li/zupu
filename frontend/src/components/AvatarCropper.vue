<template>
  <el-dialog
    :model-value="visible"
    :title="title"
    width="440px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:visible', $event)"
    @opened="onOpened"
  >
    <div class="cropper">
      <div
        ref="viewportRef"
        class="cropper-viewport"
        @wheel.prevent="onWheel"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="onPointerUp"
        @pointercancel="onPointerUp"
        @pointerleave="onPointerUp"
      >
        <img
          v-if="src"
          ref="imgRef"
          :src="src"
          class="cropper-img"
          :style="imgStyle"
          draggable="false"
          @load="init"
        />
        <div class="cropper-grid" aria-hidden="true"></div>
      </div>
      <div class="cropper-toolbar">
        <el-button size="small" circle @click="zoomBy(1 / 1.2)" :disabled="scale <= minScale + 0.001">−</el-button>
        <el-slider
          v-model="scale"
          :min="Math.max(0.05, minScale)"
          :max="maxScale"
          :step="0.01"
          class="cropper-slider"
          @input="onSlider"
        />
        <el-button size="small" circle @click="zoomBy(1.2)" :disabled="scale >= maxScale - 0.001">＋</el-button>
      </div>
      <div class="cropper-hint">滚轮 / 拖动滑块缩放，按住拖动图片调整位置</div>
    </div>
    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button type="primary" @click="confirmCrop">{{ confirmText }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, nextTick } from 'vue'
import { ElMessage } from 'element-plus'

const props = defineProps({
  src: { type: String, default: '' },
  title: { type: String, default: '裁剪头像' },
  confirmText: { type: String, default: '设为头像' }
})
const visible = defineModel('visible', { type: Boolean, default: false })
const emit = defineEmits(['update:visible', 'crop'])

const VIEW_SIZE = 300 // viewport 边长（正方形）
const OUT_SIZE = 512  // 输出头像边长
const MAX_ZOOM = 8    // 最大放大倍数

const viewportRef = ref(null)
const imgRef = ref(null)

const scale = ref(1)
const offsetX = ref(0)
const offsetY = ref(0)
const imgW = ref(0)
const imgH = ref(0)
const minScale = ref(0.1)

const maxScale = computed(() => Math.max(minScale.value * MAX_ZOOM, 4))

const imgStyle = computed(() => ({
  transform: `translate(${offsetX.value}px, ${offsetY.value}px) scale(${scale.value})`,
  transformOrigin: '0 0'
}))

function init() {
  const img = imgRef.value
  if (!img) return
  const w = img.naturalWidth
  const h = img.naturalHeight
  if (!w || !h) return
  imgW.value = w
  imgH.value = h
  const s = Math.max(VIEW_SIZE / w, VIEW_SIZE / h) // cover 铺满
  minScale.value = s
  scale.value = s
  offsetX.value = (VIEW_SIZE - w * s) / 2
  offsetY.value = (VIEW_SIZE - h * s) / 2
}

function onOpened() {
  const img = imgRef.value
  if (img && img.complete) init()
}

function clampOffset() {
  const dw = imgW.value * scale.value
  const dh = imgH.value * scale.value
  if (dw >= VIEW_SIZE) {
    offsetX.value = Math.min(0, Math.max(VIEW_SIZE - dw, offsetX.value))
  } else {
    offsetX.value = (VIEW_SIZE - dw) / 2
  }
  if (dh >= VIEW_SIZE) {
    offsetY.value = Math.min(0, Math.max(VIEW_SIZE - dh, offsetY.value))
  } else {
    offsetY.value = (VIEW_SIZE - dh) / 2
  }
}

function zoomBy(factor) {
  const target = Math.min(maxScale.value, Math.max(minScale.value, scale.value * factor))
  zoomAt(VIEW_SIZE / 2, VIEW_SIZE / 2, target)
}

function onSlider(v) {
  zoomAt(VIEW_SIZE / 2, VIEW_SIZE / 2, Number(v) || minScale.value)
}

function onWheel(e) {
  const factor = e.deltaY < 0 ? 1.1 : 1 / 1.1
  const rect = viewportRef.value?.getBoundingClientRect()
  const mx = rect ? e.clientX - rect.left : VIEW_SIZE / 2
  const my = rect ? e.clientY - rect.top : VIEW_SIZE / 2
  const target = Math.min(maxScale.value, Math.max(minScale.value, scale.value * factor))
  zoomAt(mx, my, target)
}

// 以 viewport 内 (ax, ay) 为锚点缩放
function zoomAt(ax, ay, target) {
  const ratio = target / scale.value
  offsetX.value = ax - (ax - offsetX.value) * ratio
  offsetY.value = ay - (ay - offsetY.value) * ratio
  scale.value = target
  clampOffset()
}

let dragging = false
let startX = 0
let startY = 0
let startOffsetX = 0
let startOffsetY = 0

function onPointerDown(e) {
  dragging = true
  startX = e.clientX
  startY = e.clientY
  startOffsetX = offsetX.value
  startOffsetY = offsetY.value
  try { e.currentTarget.setPointerCapture(e.pointerId) } catch (_) {}
}

function onPointerMove(e) {
  if (!dragging) return
  offsetX.value = startOffsetX + (e.clientX - startX)
  offsetY.value = startOffsetY + (e.clientY - startY)
  clampOffset()
}

function onPointerUp() {
  dragging = false
}

function confirmCrop() {
  const img = imgRef.value
  if (!img) return
  const srcX = -offsetX.value / scale.value
  const srcY = -offsetY.value / scale.value
  const srcW = VIEW_SIZE / scale.value
  const srcH = VIEW_SIZE / scale.value
  const canvas = document.createElement('canvas')
  canvas.width = OUT_SIZE
  canvas.height = OUT_SIZE
  const ctx = canvas.getContext('2d')
  if (!ctx) { ElMessage.error('浏览器不支持裁剪'); return }
  ctx.imageSmoothingEnabled = true
  ctx.imageSmoothingQuality = 'high'
  try {
    ctx.drawImage(img, srcX, srcY, srcW, srcH, 0, 0, OUT_SIZE, OUT_SIZE)
  } catch (e) {
    ElMessage.error('图片裁剪失败，请重试')
    return
  }
  canvas.toBlob((blob) => {
    if (blob) {
      emit('crop', blob)
    } else {
      ElMessage.error('裁剪失败')
    }
  }, 'image/png')
}
</script>

<style scoped>
.cropper {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
.cropper-viewport {
  position: relative;
  width: 300px;
  height: 300px;
  overflow: hidden;
  border: 2px solid #2563eb;
  border-radius: 8px;
  background: #1f2937;
  cursor: grab;
  touch-action: none;
  user-select: none;
}
.cropper-viewport:active {
  cursor: grabbing;
}
.cropper-img {
  position: absolute;
  top: 0;
  left: 0;
  pointer-events: none;
  max-width: none;
}
.cropper-grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(to right, transparent calc(33.33% - 1px), rgba(255,255,255,.35) 33.33%, transparent calc(33.33% + 1px)),
    linear-gradient(to bottom, transparent calc(33.33% - 1px), rgba(255,255,255,.35) 33.33%, transparent calc(33.33% + 1px));
}
.cropper-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 300px;
}
.cropper-slider {
  flex: 1;
}
.cropper-hint {
  font-size: 12px;
  color: #999;
}
</style>
