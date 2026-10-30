// 地理坐标工具（祠宇志 / 茔墓志共用）
//
// 目标：
//   1. 判断一条记录是否真的填了坐标（0/0 视为没填）；
//   2. 从用户粘贴的**高德 / 百度 / 腾讯**地图链接或「lng,lat」文本里尽力提取经纬度；
//   3. 生成高德网页地图的「地图查看」「祭扫导航」链接（PC 与手机浏览器都能打开）。
//
// 注意：百度系链接的参数顺序是 **lat,lng**（如 ?location=39.909,116.397），
// 高德/腾讯是 **lng,lat**（如 ?position=116.397,39.909）—— 这里会自动换序，
// 换序失败时宁可返回 null（让用户手填），也不要把坐标填反。

// hasCoord 是否已填坐标（0/0 = 未填；越界 = 非法）
export function hasCoord(lng, lat) {
  const x = Number(lng)
  const y = Number(lat)
  if (!Number.isFinite(x) || !Number.isFinite(y)) return false
  if (x === 0 || y === 0) return false
  return Math.abs(x) <= 180 && Math.abs(y) <= 90
}

// fmtCoord 显示用：最多 6 位小数，去掉末尾 0
export function fmtCoord(lng, lat) {
  if (!hasCoord(lng, lat)) return ''
  const f = (v) => String(Number(v)).replace(/(\.\d*?)0+$/, '$1').replace(/\.$/, '')
  return f(lng) + ',' + f(lat)
}

const NUM = '(-?\\d{1,3}(?:\\.\\d+)?)'

function pick(lng, lat) {
  return hasCoord(lng, lat) ? { lng: Number(lng), lat: Number(lat) } : null
}

// parseCoords 从任意粘贴内容中提取坐标；提不出来返回 null
// 支持：
//   https://uri.amap.com/marker?position=116.397,39.909&name=某祠
//   https://uri.amap.com/navigation?to=116.397,39.909,某墓&mode=car
//   https://www.amap.com/place/B000?lng=116.397&lat=39.909
//   https://api.map.baidu.com/marker?location=39.909,116.397&title=某墓
//   https://map.baidu.com/?latlng=39.909,116.397&title=...
//   https://map.qq.com/?what=...&coord=39.909,116.397   （腾讯为 lat,lng）
//   纯文本：116.397,39.909 / 116.397 39.909 / 116.397，39.909
export function parseCoords(input) {
  const s = String(input || '').trim()
  if (!s) return null

  const isLatLngHost = /baidu\.com/i.test(s) || /map\.qq\.com/i.test(s) || /apis\.map\.qq\.com/i.test(s)

  // ① 显式 lng= / lat= 参数（最可靠）
  const lngM = s.match(new RegExp('[?&#]lng=' + NUM, 'i'))
  const latM = s.match(new RegExp('[?&#]lat=' + NUM, 'i'))
  if (lngM && latM) return pick(lngM[1], latM[1])

  // ② position= / to= / location= / latlng= / center= / coord= / pt= 参数（两个数）
  const pairRe = new RegExp('[?&#](?:position|to|location|latlng|center|coord|pt)=' + NUM + ',' + NUM, 'i')
  const m = s.match(pairRe)
  if (m) {
    const a = Number(m[1])
    const b = Number(m[2])
    // 百度/腾讯：lat,lng；高德等：lng,lat
    return isLatLngHost ? pick(b, a) : pick(a, b)
  }

  // ③ 裸坐标文本：lng,lat（用中文逗号/空格也认）
  const bare = s.match(new RegExp(NUM + '\\s*[,，]\\s*' + NUM))
  if (bare) return pick(bare[1], bare[2])
  const bare2 = s.match(new RegExp(NUM + '\\s+' + NUM))
  if (bare2) return pick(bare2[1], bare2[2])

  return null
}

// amapMarkerUrl 「地图查看」：在高德网页地图上打点
// 格式：https://uri.amap.com/marker?position=lng,lat&name=…
export function amapMarkerUrl(lng, lat, name) {
  if (!hasCoord(lng, lat)) return ''
  return `https://uri.amap.com/marker?position=${lng},${lat}&name=${encodeURIComponent(name || '')}`
}

// amapNavUrl 「祭扫导航」：以该坐标为终点、驾车路线
// 格式：https://uri.amap.com/navigation?to=lng,lat,name&mode=car
export function amapNavUrl(lng, lat, name) {
  if (!hasCoord(lng, lat)) return ''
  return `https://uri.amap.com/navigation?to=${lng},${lat},${encodeURIComponent(name || '')}&mode=car`
}
