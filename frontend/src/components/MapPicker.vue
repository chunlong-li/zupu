<template>
  <!--
    地图选点（祠宇志 / 茔墓志 共用）
    · 底图复用「热力图 → 底图设置」那一套：GET /api/map/online-config + utils/basemap.js 的瓦片规则
    · 默认用高德（GCJ-02，与「祭扫导航」uri.amap.com 一致）；切到天地图/Esri 会提示坐标系差异
    · 不做任何坐标换算：点哪就是哪，回填的经纬度 = 底图坐标系下的值
  -->
  <el-dialog
    :model-value="visible" :title="title" :width="isMobile ? '96vw' : '920px'" :top="isMobile ? '2vh' : '6vh'"
    :close-on-click-modal="false" append-to-body class="mp-dialog" :class="{ 'mp-dialog-mobile': isMobile }"
    @update:model-value="(v) => emit('update:visible', v)" @opened="onOpened" @closed="onClosed"
  >
    <div class="mp-bar">
      <el-select v-model="source" size="small" class="mp-src" @change="applySource">
        <el-option v-for="s in ONLINE_SOURCES" :key="s.id" :label="s.label" :value="s.id" />
      </el-select>
      <span class="mp-coord">
        选中坐标：
        <b v-if="picked">{{ pickedText }}</b>
        <span v-else class="mp-none">在地图上单击选点</span>
      </span>
      <el-button size="small" text @click="clearPick">清除选点</el-button>
    </div>

    <el-alert
      v-if="coordWarn" type="warning" :closable="false" show-icon class="mp-alert"
      :title="`当前底图坐标系为 ${coordWarn}，与高德（GCJ-02）不一致：选点后「祭扫导航」可能有约 50–500 米偏移。建议改用「高德街道图 / 高德卫星影像」底图。`"
    />
    <el-alert
      v-if="tileError" type="error" :closable="false" show-icon class="mp-alert"
      :title="failReason || '在线底图加载失败（可能无网络、底图服务不可用或被拦截）。可点「重试」，或关掉本对话框用「手填经纬度 / 粘贴链接提取」。'"
    >
      <template #default>
        <div class="mp-alert-row">
          <el-button size="small" @click="reloadTiles">重试加载底图</el-button>
          <el-button size="small" type="primary" plain @click="applySource">只重建当前底图</el-button>
        </div>
        <div class="mp-alert-note">底图不可用时仍可在左侧地图区域单击（灰底）取点，或直接用「手填 / 粘贴链接提取」。</div>
      </template>
    </el-alert>

    <div class="mp-map-wrap">
      <div ref="mapEl" class="mp-map"></div>
      <div v-if="!tileOk && !tileError" class="mp-loading">底图加载中…</div>
    </div>

    <div class="mp-tip">
      提示：高德底图是 GCJ-02（与高德导航同源）。点击地图落点后会回填「经度, 纬度」；确定后写入表单，保存时与手填/粘贴链接等价。
    </div>

    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :disabled="!picked" @click="confirm">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import {
  ONLINE_SOURCES, ONLINE_ATTR, ONLINE_COORD, buildBasemap, isGcj02Source, loadOnlineConfig
} from '../utils/basemap'
import { useIsMobile } from '../utils/useIsMobile'

const props = defineProps({
  visible: { type: Boolean, default: false },
  lng: { type: [Number, String], default: 0 },
  lat: { type: [Number, String], default: 0 },
  name: { type: String, default: '' },
  title: { type: String, default: '🗺️ 地图选点' }
})
const emit = defineEmits(['update:visible', 'confirm'])

// 无坐标时的默认视野：福建省（需求指定）
const DEFAULT_CENTER = [25.9, 118.1]
const DEFAULT_ZOOM = 7
const PICK_ZOOM = 16

const isMobile = useIsMobile()
const mapEl = ref(null)
const source = ref('amap')
const picked = ref(null)     // { lng, lat }
const tileOk = ref(false)
const tileError = ref(false)
const failReason = ref('')
const cfg = ref({})

let map = null
let marker = null
let baseLayer = null
let labelLayer = null
let tileErrN = 0
let tileOkN = 0
let loadTimer = null

// 看门狗：8 秒内没有任何瓦片加载成功 → 明确提示（无网 / 被拦截 / 服务不可用），不白屏
const LOAD_TIMEOUT_MS = 8000
function armWatchdog() {
  if (loadTimer) clearTimeout(loadTimer)
  loadTimer = setTimeout(() => {
    if (tileOkN === 0 && map) {
      tileError.value = true
      if (!failReason.value) failReason.value = '底图瓦片在 8 秒内没有加载成功（可能无网络、底图服务不可用或被浏览器拦截）'
    }
  }, LOAD_TIMEOUT_MS)
}
function disarmWatchdog() {
  if (loadTimer) { clearTimeout(loadTimer); loadTimer = null }
}

const pickedText = computed(() => {
  if (!picked.value) return ''
  const f = (v) => String(Math.round(Number(v) * 1e6) / 1e6)
  return f(picked.value.lng) + ', ' + f(picked.value.lat)
})
const coordWarn = computed(() => (isGcj02Source(source.value) ? '' : (ONLINE_COORD[source.value] || '')))

function validCoord(lng, lat) {
  const x = Number(lng)
  const y = Number(lat)
  return Number.isFinite(x) && Number.isFinite(y) && x !== 0 && y !== 0 && Math.abs(x) <= 180 && Math.abs(y) <= 90
}

function pinIcon() {
  // 用 divIcon 内联 SVG，避免打包后 Leaflet 默认图标（marker-icon.png）路径失效
  return L.divIcon({
    className: 'mp-pin',
    html: '<svg width="30" height="42" viewBox="0 0 30 42" xmlns="http://www.w3.org/2000/svg">'
      + '<path d="M15 41C15 41 28 25.6 28 15A13 13 0 1 0 2 15C2 25.6 15 41 15 41Z" fill="#c0392b" stroke="#fff" stroke-width="2"/>'
      + '<circle cx="15" cy="15" r="5" fill="#fff"/></svg>',
    iconSize: [30, 42],
    iconAnchor: [15, 41]
  })
}

function ensureMap() {
  if (map || !mapEl.value) return
  map = L.map(mapEl.value, {
    center: DEFAULT_CENTER,
    zoom: DEFAULT_ZOOM,
    zoomControl: true,
    attributionControl: true,
    minZoom: 3,
    maxZoom: 18
  })
  map.attributionControl.setPrefix('')
  map.on('click', (e) => setPoint(e.latlng.lng, e.latlng.lat, true))
}

function setPoint(lng, lat, fromClick) {
  if (!validCoord(lng, lat)) return
  picked.value = { lng: Number(lng), lat: Number(lat) }
  if (!map) return
  if (!marker) {
    marker = L.marker([lat, lng], { icon: pinIcon(), draggable: true, keyboard: false }).addTo(map)
    marker.on('dragend', () => {
      const p = marker.getLatLng()
      picked.value = { lng: p.lng, lat: p.lat }
    })
  } else {
    marker.setLatLng([lat, lng])
  }
  // 点地图时不自动平移，避免「点一下中心跳一下」；拖标记/定位时才居中
  if (!fromClick) map.setView([lat, lng], Math.max(map.getZoom(), PICK_ZOOM))
}

function clearPick() {
  picked.value = null
  if (marker && map) { map.removeLayer(marker); marker = null }
}

function dropTiles() {
  if (baseLayer) { map.removeLayer(baseLayer); baseLayer = null }
  if (labelLayer) { map.removeLayer(labelLayer); labelLayer = null }
  tileErrN = 0
  tileOkN = 0
  tileOk.value = false
  tileError.value = false
  failReason.value = ''
}

function watchHealth(layer) {
  layer.on('tileerror', () => {
    tileErrN++
    if (tileErrN >= 5 && tileOkN === 0) {
      tileError.value = true
      failReason.value = ''
    }
  })
  layer.on('tileload', () => {
    tileOkN++
    if (tileOkN >= 1) { tileOk.value = true; tileError.value = false; disarmWatchdog() }
  })
}

// 按当前 source 重建底图（同一套 URL/Key 规则，见 utils/basemap.js）
function applySource() {
  if (!map) return
  dropTiles()
  const b = buildBasemap(cfg.value, source.value, localStorage.getItem('token') || '')
  if (!b.ok) {
    tileError.value = true
    failReason.value = b.reason || '底图不可用'
    return
  }
  const opt = { minZoom: 3, maxZoom: 18, noWrap: false, keepBuffer: 1, subdomains: b.base.subdomains, maxNativeZoom: b.base.maxNativeZoom }
  baseLayer = L.tileLayer(b.base.url, Object.assign({ attribution: ONLINE_ATTR[b.source] || '' }, opt)).addTo(map)
  watchHealth(baseLayer)
  armWatchdog()
  if (b.label) {
    map.createPane('mpLabel')
    map.getPane('mpLabel').style.zIndex = 350
    labelLayer = L.tileLayer(b.label.url, Object.assign({ pane: 'mpLabel' }, opt)).addTo(map)
    watchHealth(labelLayer)
  }
}

function reloadTiles() {
  applySource()
  // 强制让 Leaflet 重新请求当前视野的瓦片
  if (map) { map.invalidateSize(); map.setZoom(map.getZoom()) }
}

async function openWith() {
  await nextTick()
  ensureMap()
  if (!map) return
  map.invalidateSize()
  // 初始点：已有坐标 → 定位并落标记；否则默认视野（福建省）
  if (validCoord(props.lng, props.lat)) {
    setPoint(Number(props.lng), Number(props.lat), false)
  } else {
    picked.value = null
    if (marker) { map.removeLayer(marker); marker = null }
    map.setView(DEFAULT_CENTER, DEFAULT_ZOOM)
  }
  if (tileError.value && baseLayer) { reloadTiles(); return }
  if (!baseLayer) {
    try {
      cfg.value = await loadOnlineConfig()
    } catch (e) {
      cfg.value = {}
    }
    // 默认底图用高德（GCJ-02）；系统配置本身就是高德系就沿用它
    const configured = cfg.value.source || 'amap'
    source.value = (configured === 'amap' || configured === 'amap_sat') ? configured : 'amap'
    applySource()
  }
}

function onOpened() { openWith() }

function onClosed() {
  // 对话框内容常驻（未用 destroy-on-close），地图与容器一直在 DOM 里；
  // 关闭时只停看门狗、保留瓦片（下次打开更快）
  disarmWatchdog()
}

function confirm() {
  if (!picked.value) return
  // 6 位小数 ≈ 0.1 米，够用且表单/库里不会出现一长串浮点尾巴
  const r6 = (v) => Math.round(Number(v) * 1e6) / 1e6
  emit('confirm', { lng: r6(picked.value.lng), lat: r6(picked.value.lat), source: source.value })
  emit('update:visible', false)
}

// 兼容外部直接置 visible=true（不依赖 @opened）
watch(() => props.visible, (v) => { if (v) openWith() })

onUnmounted(() => {
  disarmWatchdog()
  if (map) { map.remove(); map = null }
})
</script>

<style scoped>
.mp-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}
.mp-src { width: 280px; }
.mp-coord { font-size: 13px; color: #5a4a3a; }
.mp-coord b { color: #8c5a3c; font-family: ui-monospace, Menlo, Consolas, monospace; }
.mp-none { color: #9a8a74; }
.mp-alert { margin-bottom: 8px; }
.mp-alert-row { margin-top: 6px; display: flex; gap: 8px; }
.mp-alert-note { margin-top: 6px; color: #9a8a74; }
.mp-map-wrap { position: relative; }
.mp-map {
  width: 100%;
  height: 460px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: #eef3f7;
}
.mp-loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #8c5a3c;
  background: rgba(247, 243, 236, 0.75);
  border-radius: 8px;
  pointer-events: none;
  font-size: 14px;
}
.mp-tip { margin-top: 8px; font-size: 12px; color: #9a8a74; line-height: 1.6; }

/* ===== 手机端（<768px）：近全屏 + 大按钮 + 底部操作条固定 ===== */
@media (max-width: 767px) {
  .mp-bar { gap: 8px; }
  .mp-src { width: 100%; }
  .mp-coord { font-size: 14px; width: 100%; }
  .mp-map { height: 52vh; min-height: 260px; }
  .mp-alert-row { flex-wrap: wrap; }
  .mp-alert-row .el-button { flex: 1 1 46%; min-height: 40px; margin: 0; }
  .mp-tip { font-size: 12px; line-height: 1.5; }
}
</style>

<style>
/* divIcon 的容器默认会有白底边框，去掉 */
.mp-pin { background: none; border: none; }

/* 手机端选点对话框：近全屏（96vw/96vh），底部按钮固定且够大（≥44px），不被键盘/安全区遮住 */
.mp-dialog-mobile .el-dialog {
  height: 96vh;
  max-height: 96vh;
  margin-bottom: 2vh;
  display: flex;
  flex-direction: column;
}
.mp-dialog-mobile .el-dialog__body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
}
.mp-dialog-mobile .el-dialog__header { padding: 10px 12px; }
.mp-dialog-mobile .el-dialog__footer {
  padding: 8px 12px calc(8px + env(safe-area-inset-bottom));
  border-top: 1px solid var(--border);
  background: #fff;
  position: sticky;
  bottom: 0;
  display: flex;
  gap: 10px;
}
.mp-dialog-mobile .el-dialog__footer .el-button {
  flex: 1;
  min-height: 44px;
  font-size: 16px;
}
</style>
