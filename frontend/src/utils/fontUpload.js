// 字体上传的共用容错（谱书 / 树图 FontPicker 共用）
//   · 大字体（几 MB ~ 30MB）走外网反代经常 >30s：超时放宽到 300s
//   · 客户端超时/断网 ≠ 上传失败：服务端可能已经写完文件了 → 重新拉一次 /api/fonts，
//     若本次上传的文件（含后端避重名生成的 xxx-2.woff2）已存在，就按「成功」处理，别让用户看到英文 network error
import api from '../api'

// 上传超时（毫秒）。测试可用 window.__FONT_UPLOAD_TIMEOUT_MS__ 覆盖（只影响调试，正常为 undefined）
export function uploadTimeoutMs() {
  try {
    const t = Number(window.__FONT_UPLOAD_TIMEOUT_MS__)
    if (t > 0) return t
  } catch (e) { /* ignore */ }
  return 300000
}

export const UPLOAD_TIMEOUT_MSG =
  '上传超时/网络中断：文件较大时可能需要 1~2 分钟，请稍后刷新字体列表确认；若已出现即为上传成功'

// 超时 / 网络类错误（axios：ECONNABORTED 超时；无 response 但有 request = 网络层失败）
export function isTimeoutOrNetworkErr(err) {
  if (!err) return false
  const code = String(err.code || '')
  if (code === 'ECONNABORTED' || code === 'ETIMEDOUT' || code === 'ERR_NETWORK') return true
  if (!err.response && err.request) return true
  return /timeout|network error|networkerror/i.test(String(err.message || ''))
}

function escapeRe(s) { return String(s).replace(/[.*+?^${}()|[\]\\]/g, '\\$&') }

// 失败后多问几次：客户端超时/断连时，服务端可能还在写盘（实测 1.5s 超时场景下文件 1~2s 后才出现）
export async function recoverUploadedFont(fileName, sinceSec, tries = 4, gapMs = 2500) {
  for (let i = 0; i < tries; i++) {
    const hit = await findUploadedFont(fileName, sinceSec)
    if (hit) return hit
    if (i < tries - 1) await new Promise(r => setTimeout(r, gapMs))
  }
  return null
}

// 服务端是否已有「本次上传」的这个文件（同名，或后端避重名生成的 `名字-N.扩展名`）
export async function findUploadedFont(fileName, sinceSec) {
  try {
    const { data } = await api.get('/fonts')
    const list = Array.isArray(data) ? data : []
    const m = String(fileName).match(/^(.*?)(\.[^.]+)?$/)
    const base = escapeRe(m[1] || fileName)
    const ext = escapeRe(m[2] || '')
    const re = new RegExp('^' + base + '(-\\d+)?' + ext + '$', 'i')
    let cands = list.filter(f => re.test(String(f.name || '')))
    if (!cands.length) return null
    // 只认「本次上传期间新建」的；没有再退回全部候选里最新的一个
    const fresh = cands.filter(f => Number(f.created_at || 0) >= Number(sinceSec || 0) - 30)
    if (fresh.length) cands = fresh
    cands.sort((a, b) => Number(b.created_at || 0) - Number(a.created_at || 0))
    return cands[0]
  } catch (e) {
    return null
  }
}

// 上传一个字体文件：返回 { info, recovered }
//   recovered = true 表示「前端超时了，但服务端已保存」→ 调用方应按成功提示
export async function uploadFontWithRecovery(file) {
  const sinceSec = Math.floor(Date.now() / 1000)
  const fd = new FormData()
  fd.append('file', file)
  try {
    const resp = await api.post('/fonts', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: uploadTimeoutMs()
    })
    return { info: resp.data, recovered: false }
  } catch (err) {
    // 409 = 服务端明确拒绝（内容重复 / 同名不同内容）：绝不能再走「复查列表当成功」，
    // 否则会因为列表里本来就有那个字体而误判成上传成功
    if (err && err.response && err.response.status === 409) {
      const d = err.response.data || {}
      throw Object.assign(new Error(d.error || '该字体已存在，已被拒绝上传'), {
        kind: 'duplicate', status: 409, code: d.code || '', existing: d.existing || ''
      })
    }
    // 其它错误都先问一次服务端：反代超时/502/连接被掐时，文件其实可能已经写完了
    // （只认「本次上传这段时间里新建」的同名文件，避免把以前的同名文件误判成成功）
    const hit = await recoverUploadedFont(file.name, sinceSec)
    if (hit) return { info: hit, recovered: true }
    if (isTimeoutOrNetworkErr(err)) {
      throw Object.assign(new Error(UPLOAD_TIMEOUT_MSG), { kind: 'timeout', cause: err })
    }
    const st = err && err.response && err.response.status
    const msg = err && err.response && err.response.data && err.response.data.error
    let text = msg || ('字体上传到服务器失败：' + file.name)
    if (st === 403) text = msg || '权限不足：仅理事长及以上可上传字体'
    else if (st === 404) text = '接口不存在，请确认后端已更新到最新版'
    else if (st === 413) text = '字体文件过大（上限 30MB）'
    throw Object.assign(new Error(text), { kind: 'upload', status: st, cause: err })
  }
}

// 校验（两处上传共用）：扩展名 + 体积
export function validateFontFile(file) {
  if (!/\.(ttf|otf|woff2?|ttc)$/i.test(file.name)) return '请上传 .ttf / .otf / .woff / .woff2 / .ttc 字体文件'
  if (file.size > 30 * 1024 * 1024) return '字体文件过大（上限 30MB）'
  return ''
}
