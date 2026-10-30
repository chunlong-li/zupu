// 树图（宝塔式/苏式垂吊/苏式垂丝/世代分层）共用的字体选择：
//   · 内置系统字体栈 + 服务器上传字体（GET /api/fonts，FontFace 注册）
//   · 4 个视图共享同一份选择，存 localStorage['treeFont']，刷新后仍生效
//   · 视图把 --tree-font 变量挂到自己的 <svg> 上，并用 .tf-on 之类的 class 打开
//     `svg text { font-family: var(--tree-font) }` 规则（CSS 规则优先级高于 <text> 上的
//     font-family 表现属性，所以能覆盖各视图原有的楷体/宋体声明）
//   · 导出时由 treeExport 把「计算后的 font-family」内联到克隆出来的 text 上（见 treeExport.js），
//     所以导出的 SVG / PNG / PDF 都跟着当前选择走
import { ref, computed, onMounted } from 'vue'
import api from '../api'
import { currentRole } from './auth'
import { uploadFontWithRecovery, validateFontFile, UPLOAD_TIMEOUT_MSG } from './fontUpload'

// 只有 TTF(glyf) 才支持子集内嵌；woff2/otf/ttc 在导出「内嵌字体」时会被明确提示
export const NON_TTF_EXT = ['woff', 'woff2', 'otf', 'ttc']
export function fontExt(name) { return String(name || '').split('.').pop().toLowerCase() }
export function isNonTtfFont(f) { return !!f && NON_TTF_EXT.includes(fontExt(f.name || f.url || '')) }

export const TREE_FONT_LS = 'treeFont'
const CHIEF_ROLES = ['理事长', '超级管理员']

// 内置档：第一项「默认」= 不覆盖各视图原有字体栈（保持现状），其余为常见系统字体
export const BUILTIN_TREE_FONTS = [
  { key: 'tree-default', label: '默认（各视图原字体）', stack: '', custom: false },
  { key: 'tree-kai', label: '楷体（系统）', stack: '"KaiTi","楷体","STKaiti","Kaiti SC","KaiTi_GB2312","Noto Serif CJK SC",serif', custom: false },
  { key: 'tree-song', label: '宋体（系统）', stack: '"SimSun","宋体","STSong","Noto Serif CJK SC","Songti SC",serif', custom: false },
  { key: 'tree-fang', label: '仿宋（系统）', stack: '"FangSong","仿宋","STFangsong","仿宋_GB2312",serif', custom: false },
  { key: 'tree-hei', label: '黑体（系统）', stack: '"SimHei","黑体","Heiti SC","STHeiti","Noto Sans CJK SC",sans-serif', custom: false },
  { key: 'tree-li', label: '隶书（系统）', stack: '"LiSu","隶书","STLiti","Baoli SC",serif', custom: false }
]

function readSaved() {
  try {
    const s = JSON.parse(localStorage.getItem(TREE_FONT_LS) || 'null')
    if (s && typeof s.key === 'string' && typeof s.stack === 'string') return s
  } catch (e) { /* 忽略坏数据 */ }
  return null
}

// 全局单例状态：4 个视图共享
export const treeFont = ref(readSaved() || { ...BUILTIN_TREE_FONTS[0] })
export const serverFonts = ref([])
export const fontOptions = computed(() => [...BUILTIN_TREE_FONTS, ...serverFonts.value])
export const fontUploading = ref(false)
export const fontLoading = ref(false)
let loadedOnce = false

// 视图用：是否启用自定义字体（决定是否打开覆盖规则 / 传 --tree-font）
export const treeFontActive = computed(() => !!treeFont.value.stack)
export const treeFontStyle = computed(() => (treeFont.value.stack ? { '--tree-font': treeFont.value.stack } : {}))
export const treeFontIsCustom = computed(() => !!treeFont.value.custom)

export function canUploadFont() {
  return CHIEF_ROLES.includes(currentRole())
}

function persist() {
  try { localStorage.setItem(TREE_FONT_LS, JSON.stringify(treeFont.value)) } catch (e) { /* 忽略 */ }
}

export function selectTreeFont(key) {
  const f = fontOptions.value.find(x => x.key === key)
  if (!f) return null
  treeFont.value = {
    key: f.key, label: f.label, stack: f.stack || '',
    family: f.family || '', url: f.url || '', name: f.name || '', custom: !!f.custom
  }
  persist()
  return treeFont.value
}

// 把一个服务器字体注册进浏览器（FontFace），失败只跳过该字体
async function registerFont(info) {
  const family = info.family
  const face = new FontFace(family, `url(${encodeURI(info.url)})`, { display: 'swap' })
  await face.load()
  document.fonts.add(face)
  try { await document.fonts.load('16px "' + family + '"') } catch (e) { /* 忽略 */ }
  return { key: 'custom-' + family, label: (info.label || info.name || family) + '（上传）', stack: `"${family}",serif`, family, url: info.url, name: info.name, custom: true }
}

export async function loadTreeFonts(force) {
  if (loadedOnce && !force) return
  loadedOnce = true
  fontLoading.value = true
  // 记录「开始加载时」的选择：加载期间用户可能刚上传并选中了新字体，
  // 结束时不能拿旧列表把新选择顶掉（大字体注册慢，很容易撞上）
  const startKey = treeFont.value.key
  const startFamily = treeFont.value.family
  try {
    const { data } = await api.get('/fonts')
    const list = []
    for (const info of (data || [])) {
      try { list.push(await registerFont(info)) } catch (e) { console.warn('字体加载失败，已跳过：', info && info.name, e) }
    }
    serverFonts.value = list
    // 上次选的是上传字体：套回去；字体已被删除则回退默认。
    // 只处理「本次加载开始后就没人动过」的选择（否则会把刚上传选中的字体顶掉）
    const cur = treeFont.value
    if (cur && cur.key === startKey && cur.custom && startFamily) {
      const hit = list.find(f => f.family === startFamily)
      if (hit) treeFont.value = { key: hit.key, label: hit.label, stack: hit.stack, family: hit.family, url: hit.url, name: hit.name, custom: true }
      else if (treeFont.value.key === startKey) { treeFont.value = { ...BUILTIN_TREE_FONTS[0] }; persist() }
    }
  } catch (e) {
    // 拉不到字体列表（未登录/网络异常）不影响其它功能
  } finally {
    fontLoading.value = false
  }
}

// 上传字体（POST /api/fonts，需理事长及以上）。返回 { item, recovered }（已选中）
//   超时/断网时会重新拉 /api/fonts：若本次文件已在服务端，按成功处理（recovered = true）
export async function uploadTreeFont(file) {
  const bad = validateFontFile(file)
  if (bad) throw Object.assign(new Error(bad), { kind: 'validate' })
  const { info, recovered } = await uploadFontWithRecovery(file)
  let item = null, loadErr = null
  try {
    item = await registerFont(info)
  } catch (e) {
    loadErr = e        // 文件已在服务端，但当前浏览器加载不了（损坏/格式不受支持，如实测某些 woff2）
  }
  if (!item) {
    await loadTreeFonts(true)      // 让它在列表里可见（其它设备/修好后仍能用）
    throw Object.assign(new Error(
      `字体已上传到系统，但当前浏览器无法加载它（文件可能损坏或格式不受支持）：${info.label || info.name}。` +
      '请确认字体文件完整，或改用 TTF 版本（浏览器读不到系统字体，但 TTF 可以子集内嵌）'), { kind: 'load', uploaded: true })
  }
  serverFonts.value = serverFonts.value.filter(f => f.family !== item.family).concat([item])
  selectTreeFont(item.key)
  return { item, recovered }
}

// 按文件名选中服务器上已有的上传字体（「内容重复」被拒时，直接让用户用起来）
export async function selectServerFontByName(name) {
  if (!name) return null
  let f = serverFonts.value.find(x => x.name === name)
  if (!f) {
    try {
      const { data } = await api.get('/fonts')
      const info = (data || []).find(x => x.name === name)
      if (!info) return null
      f = await registerFont(info)
      serverFonts.value = serverFonts.value.filter(x => x.family !== f.family).concat([f])
    } catch (e) { return null }
  }
  selectTreeFont(f.key)
  return f
}

// 删除一个上传字体（DELETE /api/fonts/:name，需理事长及以上）
export async function deleteTreeFont(font) {
  const f = font || treeFont.value
  if (!f || !f.custom) throw new Error('只能删除上传字体')
  const name = f.name || (f.family ? f.family.replace(/^ZupuFont_/, '') : '')
  if (!name) throw new Error('缺少字体文件名，无法删除')
  await api.delete('/fonts/' + encodeURIComponent(name))
  serverFonts.value = serverFonts.value.filter(x => x.name !== name)
  if (treeFont.value.name === name || treeFont.value.key === f.key) {
    treeFont.value = { ...BUILTIN_TREE_FONTS[0] }
    persist()
  }
  return name
}

export { UPLOAD_TIMEOUT_MSG }

// 视图里用：<script setup> 中调用一次即可（自动拉取服务器字体）
export function useTreeFonts() {
  onMounted(() => { loadTreeFonts() })
  return { treeFont, treeFontActive, treeFontStyle, treeFontIsCustom, fontOptions, serverFonts, fontUploading, fontLoading, selectTreeFont, uploadTreeFont, loadTreeFonts }
}
