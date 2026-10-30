<template>
  <div>
    <div class="toolbar">
      <el-button type="primary" @click="doBackup" :loading="backing">一键备份</el-button>
      <el-button type="success" @click="doBackupGedcom" :loading="gedLoading">📤 备份为 GEDCOM</el-button>
      <el-button type="warning" @click="pickGedcomFile" :loading="importing">📥 导入 GEDCOM</el-button>
      <el-button v-if="isChief" type="danger" @click="doBackupGedcomAccounts" :loading="gedAccLoading">🔐 备份为 GEDCOM（带账号密码）</el-button>
      <input ref="fileInput" type="file" accept=".ged,.txt" style="display:none" @change="onFileChange" />
      <el-button type="info" @click="pickBackupFile" :loading="uploading">📦 上传备份还原</el-button>
      <input ref="backupInput" type="file" accept=".zip" style="display:none" @change="onBackupFileChange" />
      <el-button type="danger" @click="doReset" :loading="resetting">恢复初始设置</el-button>
      <el-tag type="success" size="small" v-if="autoBackup">🕐 自动备份：每日02:00（数据库+缩略图，附件/字体走同日硬链接快照；另生成 GEDCOM 快照）/ 每月1日03:00（完整数据）</el-tag>
    </div>

    <!-- 各类备份到底备了什么：属主反馈「备份漏数据」多因分不清这几种 -->
    <el-alert type="info" :closable="false" class="result">
      <template #title>各类备份包含什么？</template>
      <div class="legend">
        <div><b>一键备份 / 手动备份</b>：<span class="ok">完整数据</span> —— 数据库全部表（成员、<b>传记</b>、<b>账本</b>、文章、账号…）+ 上传附件（照片/视频）+ 上传字体 + 缩略图。可原样还原，推荐日常使用。</div>
        <div><b>每日备份</b>：数据库全部表 + 缩略图。<span class="warn">上传附件、上传字体体积大</span>，按天做硬链接快照（见「多媒体快照」「字体快照」分页），还原时选同一天的快照即可。</div>
        <div><b>每月备份</b>：<span class="ok">完整数据</span>（内容同「一键备份」），每月 1 日自动生成。</div>
        <div><b>系统备份</b>：<span class="bad">仅程序页面（web/）</span>，<b>不含任何族谱数据</b>；每次启动自动生成，用于程序文件回退，<b>不能当数据备份用</b>。</div>
        <div><b>GEDCOM / GEDCOM（带账号密码）</b>：<span class="bad">仅家谱关系</span>（族人、婚姻、生卒、籍贯、成员简介），<b>不含传记正文、账本账目、照片附件、字体</b>。GEDCOM 是通用交换格式，适合导入其它族谱软件，<b>不能当完整备份</b>；带账号密码版额外含登录账号与密码哈希，仅理事长/超管可导出。</div>
      </div>
    </el-alert>

    <el-alert v-if="result" type="success" :closable="false" class="result">
      <template #title>
        备份成功：{{ result.file }}
        <span v-if="result.font_summary || result.manifest?.font_policy_label" class="result-font">
          ｜{{ result.font_summary || ('字体：' + result.manifest.font_policy_label) }}
        </span>
      </template>
    </el-alert>

    <el-tabs v-model="activeTab" @tab-change="loadFiles">
      <el-tab-pane label="短期备份（每日）" name="daily" />
      <el-tab-pane label="长期备份（每月）" name="monthly" />
      <el-tab-pane label="手动备份" name="manual" />
      <el-tab-pane label="GEDCOM" name="gedcom" />
      <el-tab-pane v-if="isChief" label="GEDCOM（带账号密码）" name="gedcom_accounts" />
      <el-tab-pane label="多媒体快照（附件）" name="media" />
      <el-tab-pane label="字体快照" name="fonts" />
    </el-tabs>

    <el-alert v-if="activeTab === 'fonts'" type="info" :closable="false" class="result"
      title="字体快照：按天保存上传字体（未变文件硬链接复用，几乎不占额外空间）。还原会把该日期快照的字体覆盖回字体目录，还原后「字体设置」里的字体即可继续使用。" />
    <el-alert v-else-if="activeTab === 'media'" type="info" :closable="false" class="result"
      title="多媒体快照：按天保存上传附件（未变文件硬链接复用）。还原会把该日期快照的附件覆盖回附件目录。" />
    <el-alert v-else-if="activeTab === 'gedcom' || activeTab === 'gedcom_accounts'" type="warning" :closable="false" class="result"
      title="GEDCOM 只含家谱关系（不含传记正文/账本/照片/字体）。每份 .ged 旁边会生成同名 .manifest.json，列出「含什么、不含什么」，可下载核对。" />

    <div class="table-responsive">
      <el-table :data="files" border>
        <el-table-column prop="name" label="文件名" />
        <el-table-column label="大小" width="120">
          <template #default="s">{{ formatSize(s.row.size) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="240">
          <template #default="s">
            <el-button size="small" @click="download(s.row)">下载</el-button>
            <el-button v-if="activeTab !== 'gedcom' && activeTab !== 'gedcom_accounts'" size="small" type="warning" @click="restore(s.row)">还原</el-button>
            <el-button size="small" type="danger" @click="deleteBackup(s.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- GEDCOM 批量导入后：成员初始密码一览 -->
    <AccountPwDialog v-model:visible="gedcomPw.visible" :items="gedcomPw.items" />

    <!-- 一键备份前先问：字体怎么备？ -->
    <el-dialog v-model="fontDlg.visible" title="一键备份 —— 字体文件要不要一起备份？" width="660px" :close-on-click-modal="false">
      <div class="dlg-tip">
        字体文件通常较大（当前 <b>{{ fontList.length }}</b> 个，共 <b>{{ formatSize(fontTotalBytes) }}</b>），
        可以按需选择。数据库、上传附件、缩略图<b>始终会备份</b>。
      </div>
      <el-radio-group v-model="fontDlg.policy" class="dlg-radios">
        <el-radio value="all">全部备份（推荐）</el-radio>
        <el-radio value="selected">选择备份（自行勾选要备份的字体）</el-radio>
        <el-radio value="none">不备份（只备份数据库、附件、缩略图）</el-radio>
      </el-radio-group>

      <div v-if="fontDlg.policy === 'selected'" class="dlg-picker">
        <div class="picker-bar">
          <el-button size="small" @click="fontDlg.names = fontList.map(f => f.name)">全选</el-button>
          <el-button size="small" @click="fontDlg.names = []">全不选</el-button>
          <span class="picker-count">
            已选 <b>{{ fontDlg.names.length }}</b> / {{ fontList.length }} 个，合计
            <b>{{ formatSize(selectedFontBytes) }}</b>
          </span>
        </div>
        <div v-if="!fontList.length" class="picker-empty">当前没有任何上传字体，将不会打包字体。</div>
        <el-checkbox-group v-else v-model="fontDlg.names" class="picker-list">
          <el-checkbox v-for="f in fontList" :key="f.name" :value="f.name" class="picker-item">
            <span class="fi-label">{{ f.label || f.name }}</span>
            <span class="fi-name">{{ f.name }}</span>
            <span class="fi-size">{{ formatSize(f.size) }}</span>
            <span class="fi-time">{{ formatTime(f.created_at) }}</span>
          </el-checkbox>
        </el-checkbox-group>
        <div v-if="fontDlg.names.length === 0" class="picker-warn">未勾选任何字体 → 本次备份将<b>不含字体</b>。</div>
      </div>

      <template #footer>
        <span class="dlg-remember">选择会被记住（下次默认沿用）</span>
        <el-button @click="fontDlg.visible = false">取消</el-button>
        <el-button type="primary" :loading="backing" @click="confirmBackup">开始备份</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import AccountPwDialog from '../components/AccountPwDialog.vue'

const backing = ref(false)
const resetting = ref(false)
const gedLoading = ref(false)
const gedAccLoading = ref(false)
const importing = ref(false)
const gedcomPw = ref({ visible: false, items: [] })
const uploading = ref(false)
const fileInput = ref(null)
const backupInput = ref(null)
const result = ref(null)
const files = ref([])
const activeTab = ref('daily')
const autoBackup = ref(false)

// 当前角色（仅理事长/超级管理员可见带账号密码的 GEDCOM 备份）
const role = ref('')
try {
  const u = JSON.parse(localStorage.getItem('user') || '{}')
  role.value = u.role || localStorage.getItem('role') || ''
  role.value = role.value || ''
} catch (err) {
  role.value = localStorage.getItem('role') || ''
}
const isChief = computed(() => ['理事长', '超级管理员'].includes(role.value))

// —— 一键备份：先弹框问字体怎么备 ——
const FONT_POLICY_KEY = 'backupFontPolicy'
const FONT_NAMES_KEY = 'backupFontNames'
const fontList = ref([])
const fontDlg = ref({ visible: false, policy: 'all', names: [] })
const fontTotalBytes = computed(() => fontList.value.reduce((s, f) => s + (f.size || 0), 0))
const selectedFontBytes = computed(() => {
  const picked = new Set(fontDlg.value.names)
  return fontList.value.reduce((s, f) => s + (picked.has(f.name) ? (f.size || 0) : 0), 0)
})

// 拉取字体列表（与后端 GET /api/fonts 同口径）；失败不阻断备份，只是少了个参考信息
async function loadFonts() {
  try {
    const { data } = await api.get('/fonts')
    fontList.value = Array.isArray(data) ? data : []
  } catch (e) {
    fontList.value = []
  }
}

// 打开弹框：沿用上次选择；首次（无记录）默认「全部备份」
async function doBackup() {
  await loadFonts()
  let policy = localStorage.getItem(FONT_POLICY_KEY) || 'all'
  if (!['all', 'selected', 'none'].includes(policy)) policy = 'all'
  let names = []
  try {
    names = JSON.parse(localStorage.getItem(FONT_NAMES_KEY) || '[]')
    if (!Array.isArray(names)) names = []
  } catch (e) { names = [] }
  // 过滤掉已经被删掉的字体名
  const alive = new Set(fontList.value.map(f => f.name))
  names = names.filter(n => alive.has(n))
  // 上次选的是「选择备份」但现在已无字体可选 → 退回全部
  if (policy === 'selected' && !fontList.value.length) policy = 'all'
  fontDlg.value = { visible: true, policy, names }
}

function rememberFontChoice(policy, names) {
  try {
    localStorage.setItem(FONT_POLICY_KEY, policy)
    localStorage.setItem(FONT_NAMES_KEY, JSON.stringify(policy === 'selected' ? names : []))
  } catch (e) { /* 隐私模式下写入失败无所谓 */ }
}

// 确认备份：带上字体策略调用接口
async function confirmBackup() {
  const policy = fontDlg.value.policy
  const names = policy === 'selected' ? [...fontDlg.value.names] : []
  if (policy === 'selected' && names.length === 0) {
    await ElMessageBox.confirm('没有勾选任何字体，本次备份将【不含字体】。确定继续？', '确认', { type: 'warning' })
  }
  rememberFontChoice(policy, names)
  backing.value = true
  try {
    const { data } = await api.post('/backup', { fonts: policy, font_names: names })
    result.value = data
    fontDlg.value.visible = false
    const summary = data.font_summary || ''
    ElMessage.success('备份完成' + (summary ? '（' + summary + '）' : ''))
    if (data.font_ignored && data.font_ignored.length) {
      ElMessage.warning('以下字体名无效已忽略：' + data.font_ignored.join('、'))
    }
    activeTab.value = 'manual'
    loadFiles()
  } catch (err) {
    ElMessage.error('备份失败：' + (err?.response?.data?.error || err.message || err))
  } finally {
    backing.value = false
  }
}

function formatTime(ts) {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  const p = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

// 备份为 GEDCOM：保存到服务器 GEDCOM 分类
async function doBackupGedcom() {
  gedLoading.value = true
  try {
    const { data } = await api.post('/backup/gedcom')
    result.value = data
    ElMessage.success('已备份为 GEDCOM')
    activeTab.value = 'gedcom'
    loadFiles()
  } finally {
    gedLoading.value = false
  }
}

// 备份为 GEDCOM（带账号密码）：仅理事长/超管，文件含账号与密码哈希，可恢复原登录密码
async function doBackupGedcomAccounts() {
  gedAccLoading.value = true
  try {
    const { data } = await api.post('/backup/gedcom-accounts')
    result.value = data
    ElMessage.success('已备份为 GEDCOM（带账号密码）')
    activeTab.value = 'gedcom_accounts'
    loadFiles()
  } finally {
    gedAccLoading.value = false
  }
}

// 导入 GEDCOM：弹出文件选择，读原始文本 POST 到后端导入接口
function pickGedcomFile() {
  fileInput.value?.click()
}

async function onFileChange(e) {
  const file = e.target.files?.[0]
  e.target.value = '' // 允许重复选择同一文件
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.ged') && !file.name.toLowerCase().endsWith('.txt')) {
    ElMessage.error('请选择 .ged 或 .txt 格式的 GEDCOM 文件')
    return
  }
  await ElMessageBox.confirm(
    `确定导入「${file.name}」？导入会把文件中的成员追加到现有族谱数据中。`,
    '导入 GEDCOM',
    { type: 'warning', confirmButtonText: '确定导入' }
  )
  importing.value = true
  try {
    const text = await file.text()
    const token = localStorage.getItem('token') || ''
    const res = await fetch('/api/gedcom/import', {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain; charset=utf-8', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
      body: text
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      ElMessage.error(data.error || '导入失败')
      return
    }
    const warn = data.warnings || []
    const accts = data.accounts || []
    const restored = data.restored || 0
    const restoredTip = restored ? `，已恢复 ${restored} 个账号的原密码` : ''
    if (warn.length) {
      ElMessage.warning(`导入完成：成功 ${data.imported} 人${restoredTip}，${warn.length} 条警告`)
      console.warn('GEDCOM 导入警告:', warn)
    } else {
      ElMessage.success(`导入成功：新增 ${data.imported} 人${restoredTip}`)
    }
    if (accts.length) {
      gedcomPw.value = { visible: true, items: accts }
    }
  } catch (err) {
    ElMessage.error('导入失败：' + (err.message || err))
  } finally {
    importing.value = false
  }
}

async function doReset() {
  await ElMessageBox.confirm(
    '恢复初始设置会清空全部业务数据：成员、配偶、传记、账本与账目、文章、事件、祠堂、附件、上传的图片/视频、缩略图、操作日志等（与备份内容同一口径）。\n\n会保留：登录账号、系统设置与授权、备份文件、上传字体。\n\n此操作不可撤销，建议先做一次「一键备份」。确定继续？',
    '危险操作',
    { type: 'warning', confirmButtonText: '确定清空', confirmButtonClass: 'el-button--danger' }
  )
  resetting.value = true
  try {
    await api.post('/backup/reset')
    ElMessage.success('已恢复初始状态')
  } finally {
    resetting.value = false
  }
}

async function loadFiles() {
  const { data } = await api.get('/backup/list', { params: { type: activeTab.value } })
  files.value = data || []
}

async function loadStatus() {
  try {
    const { data } = await api.get('/backup/status')
    autoBackup.value = data.auto_backup || false
  } catch (e) { /* ignore */ }
}

function formatSize(b) {
  if (!b) return '—'
  if (b < 1024) return b + ' B'
  if (b < 1024 * 1024) return (b / 1024).toFixed(1) + ' KB'
  return (b / 1024 / 1024).toFixed(1) + ' MB'
}

function download(f) {
  const token = localStorage.getItem('token') || ''
  const sep = `/api/backup/download?file=${encodeURIComponent(f.name)}&type=${activeTab.value}`
  window.open(`${sep}${token ? `&token=${encodeURIComponent(token)}` : ''}`, '_blank')
}

async function restore(f) {
  const isSnap = activeTab.value === 'media' || activeTab.value === 'fonts'
  const what = activeTab.value === 'fonts' ? '上传字体（字体目录会被该快照覆盖）'
    : activeTab.value === 'media' ? '上传附件（附件目录会被该快照覆盖）'
    : '数据库与附件；字体只还原包里有的，且【完全同名同格式的字体自动跳过】，不会产生重复字体'
  await ElMessageBox.confirm(
    `确定从「${f.name}」还原？\n\n将覆盖：${what}。\n\n还原完成后程序会自动重启。`,
    '危险操作',
    { type: 'warning', confirmButtonText: '确定还原', confirmButtonClass: 'el-button--danger' }
  )
  const { data } = await api.post('/restore', { file: f.name, type: activeTab.value })
  if (data && data.needRestart) {
    // 还原结果里把字体情况单独再提示一次（还原 N 个 / 跳过 M 个同名同格式）
    if (data.font_note) ElMessage.info({ message: data.font_note, duration: 12000 })
    if (data.autoRestart) {
      ElMessage.success((data.message || '还原完成') + '；系统正在重启，10 秒后自动刷新…')
      setTimeout(() => location.reload(), 10000)
    } else {
      ElMessage.success(data.message || '还原完成，请重新启动程序')
    }
  } else {
    ElMessage.success(data.message || '还原完成，请刷新')
    if (isSnap) loadFiles()
  }
}

async function deleteBackup(f) {
  await ElMessageBox.confirm(`确定删除备份「${f.name}」？不可恢复！`, '确认删除', { type: 'warning' })
  await api.post('/backup/delete', { file: f.name, type: activeTab.value })
  ElMessage.success('已删除')
  loadFiles()
}

// 上传备份还原：选择本机 .zip 备份，上传后由后端自动识别类型并还原（还原前自动留底）
function pickBackupFile() {
  backupInput.value?.click()
}

async function onBackupFileChange(e) {
  const file = e.target.files?.[0]
  e.target.value = '' // 允许重复选择同一文件
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.zip')) {
    ElMessage.error('请选择 .zip 格式的备份文件')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定从「${file.name}」还原？\n\n会覆盖当前族谱数据与附件（还原前系统会自动把当前状态备份到「手动备份」留底）。\n\n字体：只还原包里有的，且【完全同名同格式的字体自动跳过】，不会覆盖、不会改名产生重复字体。\n\n提示：GEDCOM（.ged）不是备份包，请用「导入 GEDCOM」；且它只含家谱关系，不含传记正文/账本/照片/字体。`,
      '上传备份还原',
      { type: 'warning', confirmButtonText: '确定还原', confirmButtonClass: 'el-button--danger' }
    )
  } catch { return }
  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', file)
    fd.append('type', 'auto')
    const token = localStorage.getItem('token') || ''
    const res = await fetch('/api/restore/upload', {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
      body: fd
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      ElMessage.error(data.error || '还原失败')
      return
    }
    if (data.needRestart) {
      if (data.font_note) ElMessage.info({ message: data.font_note, duration: 12000 })
      if (data.autoRestart) {
        ElMessage.success((data.message || '还原完成') + '；系统正在重启，10 秒后自动刷新…')
        setTimeout(() => location.reload(), 10000)
      } else {
        ElMessage.success(data.message || '还原完成')
      }
    } else {
      ElMessage.success(data.message || '还原完成，请刷新')
    }
  } catch (err) {
    ElMessage.error('还原失败：' + (err.message || err))
  } finally {
    uploading.value = false
  }
}

onMounted(() => {
  loadFiles()
  loadStatus()
})
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.hint { margin-left: auto; color: #8c5a3c; font-size: 12px; }
.result { margin-bottom: 12px; }
.legend { font-size: 12px; line-height: 1.9; }
.legend b { color: #303133; }
.legend .ok { color: #529b2e; font-weight: 600; }
.legend .warn { color: #b88230; font-weight: 600; }
.legend .bad { color: #c45656; font-weight: 600; }
.result-font { color: #529b2e; font-weight: 600; }
.dlg-tip { font-size: 13px; color: #606266; line-height: 1.7; margin-bottom: 10px; }
.dlg-radios { display: flex; flex-direction: column; align-items: flex-start; gap: 8px; margin-bottom: 6px; }
.dlg-picker { border: 1px solid #ebeef5; border-radius: 4px; padding: 10px; margin-top: 8px; background: #fafafa; }
.picker-bar { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.picker-count { color: #606266; font-size: 12px; margin-left: auto; }
.picker-count b { color: #409eff; }
.picker-empty { color: #909399; font-size: 13px; padding: 8px 0; }
.picker-list { max-height: 260px; overflow-y: auto; display: flex; flex-direction: column; width: 100%; }
.picker-item { display: flex; align-items: center; width: 100%; height: 30px; margin-right: 0; }
.picker-item :deep(.el-checkbox__label) { display: flex; align-items: center; gap: 8px; width: 100%; font-size: 13px; }
.fi-label { min-width: 96px; color: #303133; }
.fi-name { flex: 1; color: #909399; font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fi-size { width: 62px; text-align: right; color: #606266; font-size: 12px; }
.fi-time { width: 110px; text-align: right; color: #c0c4cc; font-size: 12px; }
.picker-warn { color: #e6a23c; font-size: 12px; margin-top: 8px; }
.dlg-remember { float: left; color: #909399; font-size: 12px; line-height: 32px; }
.table-responsive { overflow-x: auto; -webkit-overflow-scrolling: touch; }
</style>