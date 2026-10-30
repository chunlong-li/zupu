<script setup>
// 通用多媒体库：不属于某个成员的公共素材（祠堂照片、传统节日图片/视频、其他族务影像）
// 上传走既有 /api/upload 目录（DataDir/upload）→ 自动纳入既有备份（备份已含 upload/）
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { thumbUrl, THUMB_LIST } from '../utils/thumb'
import { branchCtx } from '../utils/branchContext'

const CATS = ['家族多媒体', '祠堂', '传统节日', '族务活动', '其他']
const list = ref([])
const loading = ref(false)
const filterCat = ref('全部')
const uploadCat = ref('祠堂')
const editing = ref(null)          // { id, title, description, category }
const editVisible = ref(false)
const uploading = ref(false)

const shown = computed(() => (filterCat.value === '全部' ? list.value : list.value.filter((x) => x.category === filterCat.value)))
const catOptions = computed(() => {
  const s = new Set(CATS)
  list.value.forEach((x) => x.category && s.add(x.category))
  return [...s]
})
function url(p) { return p && p.startsWith('/') ? p : '/uploads/' + (p || '') }
// 归属跟着顶部「宗谱/房支」选择器走：0＝总谱，>0＝该房支
const ownerBranchId = computed(() => Number(branchCtx.current) || 0)
const ownerLabel = computed(() => {
  const b = (branchCtx.branches || []).find((x) => x.id === ownerBranchId.value)
  return ownerBranchId.value && b ? b.name : '总谱'
})
function ownerName(it) {
  const bid = Number(it.branch_id || 0)
  if (!bid) return '总谱'
  const b = (branchCtx.branches || []).find((x) => x.id === bid)
  return b ? b.name : ('房支 ' + bid)
}
function fmtSize(t) { return t === 'video' ? '视频' : '图片' }

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/media')
    list.value = data || []
  } catch (e) {
    ElMessage.error('读取多媒体库失败：' + (e?.response?.data?.error || e?.message || e))
  } finally { loading.value = false }
}

async function doUpload(opt) {
  const f = opt.file
  const isVideo = /\.(mp4|mov|m4v|webm|avi|mkv|ogv)$/i.test(f.name)
  const isImage = /\.(jpg|jpeg|png|gif|webp|bmp|svg)$/i.test(f.name)
  if (!isVideo && !isImage) { ElMessage.error('不支持的文件格式：' + f.name); opt.onError && opt.onError(new Error('bad ext')); return }
  if (f.size > 100 * 1024 * 1024) { ElMessage.error('文件过大（上限 100MB）'); opt.onError && opt.onError(new Error('too big')); return }
  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', f)
    fd.append('category', uploadCat.value)
    fd.append('title', f.name)
    fd.append('branch_id', String(ownerBranchId.value))   // 归属：跟顶部选择器（0=总谱）
    const { data } = await api.post('/media/upload', fd)
    const own = '归属：' + ownerLabel.value
    if (data && data.note) ElMessage.warning({ message: '已上传：' + f.name + '（' + own + '；' + data.note + '）', duration: 6000 })
    else ElMessage.success('已上传：' + f.name + '（' + own + '）')
    await load()
  } catch (e) {
    ElMessage.error('上传失败：' + (e?.response?.data?.error || e?.message || e))
  } finally { uploading.value = false }
}

function openEdit(it) { editing.value = { ...it }; editVisible.value = true }
async function saveEdit() {
  try {
    await api.put('/media/' + editing.value.id, {
      category: editing.value.category, title: editing.value.title, description: editing.value.description,
    })
    ElMessage.success('已保存')
    editVisible.value = false
    await load()
  } catch (e) { ElMessage.error('保存失败：' + (e?.response?.data?.error || e?.message || e)) }
}
async function del(it) {
  try {
    await ElMessageBox.confirm('确定删除该素材？若没有任何记录再引用该文件，源文件与缩略图缓存会一并删除（仍被引用则只删记录）。', '提示', { type: 'warning' })
  } catch (e) { return }
  try {
    const { data } = await api.delete('/media/' + it.id)
    ElMessage.success('已删除' + (data && data.note ? '：' + data.note : ''))
    await load(); await loadOrphans()
  } catch (e) { ElMessage.error('删除失败：' + (e?.response?.data?.error || e?.message || e)) }
}
// ===== 孤儿文件（upload/ 里既不被多媒体库也不被成员附件/头像引用）=====
const orphans = ref({ count: 0, bytes: 0 })
async function loadOrphans() {
  try {
    const { data } = await api.get('/media/orphans')
    orphans.value = data || { count: 0, bytes: 0 }
  } catch (e) { orphans.value = { count: 0, bytes: 0 } }
}
async function cleanOrphans() {
  if (!orphans.value.count) { ElMessage.info('没有需要清理的孤儿文件'); return }
  try {
    await ElMessageBox.confirm(
      '将删除 ' + orphans.value.count + ' 个孤儿文件（' + fmtBytes(orphans.value.bytes) + '）：' +
      '这些文件既不在多媒体库里、也不被成员附件/头像引用。删除后不可恢复（缩略图缓存一并清理）。',
      '清理孤儿文件', { type: 'warning' })
  } catch (e) { return }
  try {
    const { data } = await api.post('/media/orphans/clean')
    ElMessage.success(data?.note || '已清理')
    await loadOrphans()
  } catch (e) { ElMessage.error('清理失败：' + (e?.response?.data?.error || e?.message || e)) }
}
function fmtBytes(n) {
  n = Number(n) || 0
  if (n >= 1048576) return (n / 1048576).toFixed(1) + ' MB'
  if (n >= 1024) return (n / 1024).toFixed(1) + ' KB'
  return n + ' B'
}

onMounted(async () => { await load(); await loadOrphans() })
// 顶部切换房支后，卡片归属标签/下次上传归属都要跟着变 → 直接读响应式 branchCtx，无需重载
</script>

<template>
  <div class="media-page">
    <div class="toolbar">
      <span class="serif title">🖼️ 家族多媒体</span>
      <span class="hint">祠堂照片、传统节日图片/视频等公共素材（不属于某位成员）；上传后可被谱书「图片展示」板块按条目选用</span>
      <span class="hint" title="归属跟着顶部「宗谱/房支」选择器走">归属 <b>{{ ownerLabel }}</b></span>
      <span class="hint">分类</span>
      <el-select v-model="uploadCat" size="small" style="width:130px">
        <el-option v-for="c in catOptions" :key="c" :label="c" :value="c" />
      </el-select>
      <el-upload :show-file-list="false" :http-request="doUpload" accept="image/*,video/*" multiple>
        <el-button size="small" type="primary" :loading="uploading">上传图片 / 视频</el-button>
      </el-upload>
      <span class="hint">筛选</span>
      <el-select v-model="filterCat" size="small" style="width:130px">
        <el-option label="全部" value="全部" />
        <el-option v-for="c in catOptions" :key="'f' + c" :label="c" :value="c" />
      </el-select>
      <el-button size="small" @click="load" :loading="loading">刷新</el-button>
      <el-button size="small" :disabled="!orphans.count" @click="cleanOrphans">
        清理孤儿文件<template v-if="orphans.count">（{{ orphans.count }} / {{ fmtBytes(orphans.bytes) }}）</template>
      </el-button>
      <span class="hint">共 {{ shown.length }} 项</span>
      <span class="hint" title="孤儿＝upload/ 里既不在多媒体库、也不被成员附件/头像引用的文件">孤儿 {{ orphans.count }} 个 / {{ fmtBytes(orphans.bytes) }}</span>
    </div>

    <div v-loading="loading" class="grid">
      <div class="card" v-for="it in shown" :key="it.id">
        <div class="thumb">
          <!-- 缩略预览走 /api/thumbnail（上传时已预生成 120/240/400 三档，磁盘缓存）；原图只在点开/下载/书页里用 -->
          <img v-if="it.type !== 'video'" :src="thumbUrl(it.path, THUMB_LIST)" alt="" loading="lazy" />
          <video v-else :src="url(it.path)" :poster="thumbUrl(it.path, THUMB_LIST)" controls preload="metadata"></video>
        </div>
        <div class="meta">
          <div class="ln1"><el-tag size="small" :type="Number(it.branch_id||0) ? 'warning' : 'success'">{{ ownerName(it) }}</el-tag><el-tag size="small">{{ it.category || '其他' }}</el-tag><span class="t">{{ it.title || '（未命名）' }}</span></div>
          <div class="ln2">{{ fmtSize(it.type) }} · {{ it.description || '（无说明）' }}</div>
          <div class="ops">
            <el-button size="small" text @click="openEdit(it)">编辑</el-button>
            <el-button size="small" text type="danger" @click="del(it)">删除</el-button>
          </div>
        </div>
      </div>
      <div v-if="!shown.length && !loading" class="empty">暂无素材 —— 用上方「上传图片 / 视频」添加祠堂照片、传统节日影像等（分类可选「家族多媒体」）</div>
    </div>

    <el-dialog v-model="editVisible" title="编辑素材" width="520px">
      <el-form v-if="editing" label-width="70px">
        <el-form-item label="分类">
          <el-select v-model="editing.category" allow-create filterable style="width:200px">
            <el-option v-for="c in catOptions" :key="'e' + c" :label="c" :value="c" />
          </el-select>
        </el-form-item>
        <el-form-item label="标题"><el-input v-model="editing.title" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="editing.description" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.media-page { padding: 12px 16px; }
.toolbar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-bottom: 12px; }
.toolbar .title { font-size: 18px; margin-right: 8px; }
.toolbar .hint { font-size: 12px; color: #8c5a3c; }
.grid { display: flex; flex-wrap: wrap; gap: 14px; min-height: 120px; }
.card { width: 210px; border: 1px solid #e2d9c6; border-radius: 8px; background: #fffdf7; overflow: hidden; }
.thumb { height: 140px; background: #f2ece0; display: flex; align-items: center; justify-content: center; }
.thumb img, .thumb video { max-width: 100%; max-height: 100%; object-fit: contain; }
.meta { padding: 8px; }
.ln1 { display: flex; gap: 6px; align-items: center; font-size: 13px; font-weight: 600; }
.ln1 .t { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ln2 { font-size: 12px; color: #6b5c46; margin-top: 4px; min-height: 32px; }
.ops { display: flex; justify-content: flex-end; }
.empty { color: #8c5a3c; font-size: 13px; padding: 24px; }
</style>
