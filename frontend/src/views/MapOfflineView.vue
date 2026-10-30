<template>
  <div class="map-offline">
    <div class="toolbar">
      <el-button @click="load">刷新</el-button>
      <el-button type="success" :loading="importing" @click="pickImportFile">{{ importing ? `导入中 ${importPct}%` : '⬆️ 导入离线包' }}</el-button>
      <input ref="importInput" type="file" accept=".gz,.tar.gz,application/gzip" style="display:none" @change="onImportFile" />
      <span class="hint">导入的包按「上传的文件名」列在下方，可逐个删除；热力图的「底图」下拉里用同样的名字。</span>
    </div>

    <el-alert type="info" :closable="false" class="tip">
      离线地图包把地图瓦片存到服务器 <code>map/</code> 目录，之后热力图无需联网即可显示真实底图（街道、地名、行政区）。<br />
      <b>导入离线包：</b>把离线包文件（<code>.tar.gz</code>）放到本机，点右上角「⬆️ 导入离线包」上传即可，解压后直接作为底图选用——无需联网、不依赖链接。<br />
      <span class="muted">说明：本系统不提供「从在线地图批量下载瓦片」，也不提供离线包导出/分发（前者违反地图服务条款，可能导致封号）。离线数据只从导入获得。</span>
    </el-alert>

    <el-table :data="packs" border>
      <el-table-column prop="name" label="地图包（文件名）" min-width="180" />
      <el-table-column label="缩放级别" width="120">
        <template #default="s">{{ s.row.min_zoom }}–{{ s.row.max_zoom }} 级</template>
      </el-table-column>
      <el-table-column label="瓦片数" width="120">
        <template #default="s">{{ (s.row.tiles || 0).toLocaleString() }}</template>
      </el-table-column>
      <el-table-column label="体积" width="110">
        <template #default="s">{{ fmtSize(s.row.bytes) }}</template>
      </el-table-column>
      <el-table-column prop="imported_at" label="导入时间" width="180" />
      <el-table-column label="状态" width="100">
        <template #default="s">
          <el-tag :type="stateType(s.row)" effect="plain" size="small">{{ stateText(s.row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120">
        <template #default="s">
          <el-button size="small" type="danger" plain @click="remove(s.row)">删除</el-button>
        </template>
      </el-table-column>
      <template #empty>还没有离线地图包，点右上角「⬆️ 导入离线包」上传。</template>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'

const packs = ref([])
const importing = ref(false)
const importPct = ref(0)
const importInput = ref(null)
let timer = null

function fmtSize(b) {
  b = b || 0
  if (b < 1024) return b + ' B'
  if (b < 1024 * 1024) return (b / 1024).toFixed(0) + ' KB'
  if (b < 1024 * 1024 * 1024) return (b / 1024 / 1024).toFixed(1) + ' MB'
  return (b / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

function stateText(p) {
  const st = p.status?.state
  if (st === 'error') return '失败'
  if (st === 'partial') return '不完整'
  if (st === 'downloading') return p.status?.phase || '处理中'
  return p.has_tiles || p.tiles > 0 ? '已导入' : '无数据'
}
function stateType(p) {
  const st = p.status?.state
  if (st === 'error') return 'danger'
  if (st === 'partial' || st === 'downloading') return 'warning'
  return p.has_tiles || p.tiles > 0 ? 'success' : 'info'
}

async function load() {
  try {
    const { data } = await api.get('/map/packs')
    packs.value = data.packs || []
  } catch (e) {
    ElMessage.error('加载失败：' + (e.response?.data?.error || e.message))
  }
}

async function remove(p) {
  try {
    await ElMessageBox.confirm(`确定删除地图包「${p.name}」？该包在本机已解压的瓦片会被全部删除。`, '确认删除', { type: 'warning', confirmButtonText: '删除' })
  } catch { return }
  try {
    await api.delete(`/map/packs/${encodeURIComponent(p.id)}`)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  }
}

// 导入离线包：按上传的文件名建立条目，解压到服务器 map/<文件名>/
function pickImportFile() {
  importInput.value?.click()
}

async function onImportFile(e) {
  const file = e.target.files?.[0]
  e.target.value = ''
  if (!file) return
  if (file.size > 10 * 1024 * 1024 * 1024) {
    ElMessage.error('文件过大（上限 10GB）')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定导入「${file.name}」（${fmtSize(file.size)}）？将以该文件名作为一个地图包，解压到本机 map 目录；同名包会被覆盖。`,
      '导入离线包',
      { type: 'warning', confirmButtonText: '确定导入' }
    )
  } catch (err) { return }
  importing.value = true
  importPct.value = 0
  try {
    const fd = new FormData()
    fd.append('file', file)
    const { data } = await api.post('/map/import-pack', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 0,
      onUploadProgress: (ev) => {
        if (ev.total) importPct.value = Math.min(100, Math.floor((ev.loaded / ev.total) * 100))
      }
    })
    ElMessage.success(`导入成功：${data.name || data.id}，共 ${(data.tiles || 0).toLocaleString()} 个瓦片文件（${fmtSize(data.bytes)}）`)
    await load()
  } catch (err) {
    ElMessage.error('导入失败：' + (err.response?.data?.error || err.message))
  } finally {
    importing.value = false
    importPct.value = 0
  }
}

onMounted(() => {
  load()
  timer = setInterval(() => { load() }, 5000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })
</script>

<style scoped>
.map-offline { padding: 16px; }
.toolbar { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.hint { color: #9aa2ad; font-size: 13px; }
.tip { margin-bottom: 12px; }
.muted { color: #9aa2ad; }
</style>
