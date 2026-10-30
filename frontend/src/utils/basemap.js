// 在线底图公共模块
//
// ⚠️ 这里就是「热力图 → 底图设置」那一套（HeatmapView.vue）的瓦片规则与配置读取，
// 抽出来给「祠宇志 / 茔墓志 → 地图选点」复用 —— **不要另写第二套地址/Key 逻辑**：
//   · 配置统一来自 GET /api/map/online-config（source / layer / tk / proxy / quota_exhausted）
//   · 四个源与后端 mapOnlineSources 一致：tianditu / amap / amap_sat / esri_sat
//
// 坐标系（重要）：
//   amap / amap_sat   → GCJ-02（火星坐标），与「祭扫导航」uri.amap.com 完全一致 —— **默认用这个**
//   tianditu          → CGCS2000（约等于 WGS-84）
//   esri_sat          → WGS-84
// 本模块**不做任何坐标换算**：选点得到的经纬度就是底图坐标系下的值。
import api from '../api'

// 天地图注记层：矢量→cva、影像→cia、地形→cta
export const TDT_ANN = { vec: 'cva', img: 'cia', ter: 'cta' }

// 可选底图源（id 必须与后端 handler/mappack.go 的 mapOnlineSources 一致）
export const ONLINE_SOURCES = [
  { id: 'amap', label: '高德街道图（推荐 · 无需 Key · GCJ-02）' },
  { id: 'amap_sat', label: '高德卫星影像（无需 Key · GCJ-02）' },
  { id: 'tianditu', label: '天地图（需 Key · CGCS2000）' },
  { id: 'esri_sat', label: 'Esri 卫星影像（无需 Key · WGS-84）' }
]

export const ONLINE_ATTR = {
  tianditu: '© 天地图',
  amap: '© 高德地图',
  amap_sat: '© 高德地图',
  esri_sat: 'Esri / Maxar / Earthstar Geographics'
}

// 各源的坐标系（写清是为了在非高德底图上提醒偏移，而不是悄悄换算）
export const ONLINE_COORD = {
  amap: 'GCJ-02',
  amap_sat: 'GCJ-02',
  tianditu: 'CGCS2000（≈WGS-84）',
  esri_sat: 'WGS-84'
}

// 是否需要天地图 Key
export function onlineNeedTk(source) {
  return (source || 'tianditu') === 'tianditu'
}

// 是否是 GCJ-02 底图（与高德导航同源）
export function isGcj02Source(source) {
  return source === 'amap' || source === 'amap_sat'
}

// 免 Key 在线底图瓦片地址（与 HeatmapView 完全相同的规则）
export function onlineTileURL(s) {
  switch (s) {
    case 'amap':
      return { url: 'https://webrd0{s}.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}', subdomains: '1234', maxNativeZoom: 18 }
    case 'amap_sat':
      return { url: 'https://webst0{s}.is.autonavi.com/appmaptile?style=6&x={x}&y={y}&z={z}', subdomains: '1234', maxNativeZoom: 18 }
    case 'esri_sat':
      return {
        url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
        subdomains: 'abc', maxNativeZoom: 17,
        label: 'https://services.arcgisonline.com/ArcGIS/rest/services/Reference/World_Boundaries_and_Places/MapServer/tile/{z}/{y}/{x}'
      }
    default:
      return null
  }
}

// 天地图瓦片地址：默认浏览器直连（浏览器端 Key）；proxy=true 时走本系统服务器中转（Key 不下发）
export function tiandituTileURL(layer, opts = {}) {
  const { proxy = false, tk = '', token = '' } = opts
  if (proxy) {
    return `/api/map/online/${layer}/{z}/{x}/{y}?token=${encodeURIComponent(token)}`
  }
  return `https://t{s}.tianditu.gov.cn/${layer}_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=${layer}&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILEMATRIX={z}&TILEROW={y}&TILECOL={x}&tk=${encodeURIComponent(tk)}`
}

// 离线地图包瓦片（服务端已下载的包）
export function mapTileURL(pack, layer, token) {
  return `/api/map/tiles/${pack}/${layer}/{z}/{x}/{y}?token=${encodeURIComponent(token || '')}`
}

// 读取在线底图配置（与热力图页面同一接口、同一份 settings）
export async function loadOnlineConfig() {
  const { data } = await api.get('/map/online-config')
  return data || {}
}

// 按配置组装一个可直接喂给 L.tileLayer 的底图描述：
//   { ok, reason, coord, base: {url, subdomains, maxNativeZoom}, label: {...}|null }
// 高德系 → 免 Key；天地图 → 需要 Key（浏览器端或服务端中转）；缺 Key 时 ok=false 并给出原因。
export function buildBasemap(cfg, source, token) {
  const src = source || (cfg && cfg.source) || 'amap'
  const coord = ONLINE_COORD[src] || ''
  if (src === 'tianditu') {
    const proxy = !!(cfg && cfg.proxy) && !(cfg && cfg.quota_exhausted)
    const tk = (cfg && cfg.tk) || ''
    const ready = proxy ? !!(cfg && cfg.server_tk_set) : !!tk
    if (!ready) {
      return { ok: false, reason: '天地图未配置 Key（可在「热力图 → 底图设置」里填，或改用免 Key 的高德底图）', coord, source: src }
    }
    const ann = TDT_ANN[(cfg && cfg.layer) || 'vec'] || 'cva'
    const layerId = (cfg && cfg.layer) || 'vec'
    return {
      ok: true, coord, source: src,
      base: { url: tiandituTileURL(layerId, { proxy, tk, token }), subdomains: '01234567', maxNativeZoom: 18 },
      label: { url: tiandituTileURL(ann, { proxy, tk, token }), subdomains: '01234567', maxNativeZoom: 18 }
    }
  }
  const u = onlineTileURL(src)
  if (!u) return { ok: false, reason: '未知的在线底图源：' + src, coord, source: src }
  return {
    ok: true, coord, source: src,
    base: { url: u.url, subdomains: u.subdomains || 'abc', maxNativeZoom: u.maxNativeZoom || 19 },
    label: u.label ? { url: u.label, subdomains: u.subdomains || 'abc', maxNativeZoom: u.maxNativeZoom || 19 } : null
  }
}
