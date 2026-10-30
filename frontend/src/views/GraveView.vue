<template>
  <div class="grave-page">
    <div class="page-head">
      <div class="title-wrap">
        <span class="serif page-title">🪦 茔墓志</span>
        <span class="sub">每座祖茔一个条目 —— 世次 · 墓碑文 · 山向分金 · 坐标导航 · 四至图</span>
      </div>
      <div class="actions">
        <el-input
          v-model="keyword" size="small" class="kw" clearable
          placeholder="搜索墓主 / 名称 / 地址 / 碑文" @keyup.enter="load" @clear="load"
        />
        <el-input v-model.number="genFilter" size="small" class="gen" clearable placeholder="世次" @keyup.enter="load" @clear="load" />
        <el-checkbox v-model="onlyCoord" size="small" @change="load">仅有坐标</el-checkbox>
        <el-button size="small" @click="load">查询</el-button>
        <el-button v-if="canEdit()" type="primary" size="small" @click="openEdit()">＋ 新增茔墓</el-button>
      </div>
    </div>

    <LegacyCemeteryPanel ref="legacyRef" kind="grave" module-name="茔墓志" @classified="load" />

    <el-table :data="rows" v-loading="loading" border class="list" @row-click="openDetail">
      <el-table-column label="墓主" min-width="140">
        <template #default="{ row }">
          <span class="serif name-link">{{ row.owner_name || row.name }}</span>
          <el-tag v-if="row.member_id" size="small" type="success" effect="plain" class="linked">已关联</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="世次" width="88" align="center">
        <template #default="{ row }">
          <span v-if="row.generation">第 {{ row.generation }} 世</span>
          <span v-else class="none">—</span>
        </template>
      </el-table-column>
      <el-table-column label="生卒" width="170">
        <template #default="{ row }"><span>{{ lifeText(row) }}</span></template>
      </el-table-column>
      <el-table-column prop="mountain_orientation" label="山向分金" width="170" show-overflow-tooltip>
        <template #default="{ row }"><span>{{ row.mountain_orientation || '—' }}</span></template>
      </el-table-column>
      <el-table-column prop="address" label="地理位置" min-width="150" show-overflow-tooltip>
        <template #default="{ row }"><span>{{ row.address || '—' }}</span></template>
      </el-table-column>
      <el-table-column label="坐标" width="84" align="center">
        <template #default="{ row }">
          <el-tag v-if="hasCoord(row.lng, row.lat)" size="small" type="success" effect="plain">已填</el-tag>
          <el-tag v-else size="small" type="info" effect="plain">未填</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="照片" width="76" align="center">
        <template #default="{ row }"><span>{{ (row.photos || []).length }} 张</span></template>
      </el-table-column>
      <el-table-column label="操作" width="220" align="center" fixed="right">
        <template #default="{ row }">
          <el-button size="small" text type="primary" @click.stop="openDetail(row)">详情</el-button>
          <el-tooltip :content="hasCoord(row.lng, row.lat) ? '高德驾车导航到墓址' : '未填写坐标'" placement="top">
            <span class="btn-wrap">
              <el-button size="small" text :disabled="!hasCoord(row.lng, row.lat)" @click.stop="navigate(row)">导航</el-button>
            </span>
          </el-tooltip>
          <el-button v-if="canEdit()" size="small" text @click.stop="openEdit(row)">编辑</el-button>
          <el-button v-if="canEdit()" size="small" text type="danger" @click.stop="del(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="!loading && !rows.length" description="还没有茔墓记录 —— 点右上「＋ 新增茔墓」开始（每座祖茔一条）" />

    <!-- ============ 详情抽屉 ============ -->
    <el-drawer v-model="detailVisible" :size="isMobile ? '100%' : '760px'" :with-header="false" class="detail-drawer">
      <div v-if="detail" class="drawer-body">
        <div class="drawer-head">
          <div class="head-left">
            <span class="serif head-name">{{ detail.owner_name || detail.name }}</span>
            <el-tag v-if="detail.generation" size="small" effect="plain">第 {{ detail.generation }} 世</el-tag>
            <el-tag v-if="detail.member_id && detail.member_name" size="small" type="success" effect="plain">
              已关联成员：{{ detail.member_name }}
            </el-tag>
          </div>
          <div class="head-actions">
            <el-tooltip :content="detailCoordOk ? '在高德地图上查看墓址' : '未填写坐标'" placement="top">
              <span class="btn-wrap">
                <el-button size="small" :disabled="!detailCoordOk" @click="openMap(detail)">🗺️ 地图查看</el-button>
              </span>
            </el-tooltip>
            <el-tooltip :content="detailCoordOk ? '高德驾车导航到墓址（祭扫用）' : '未填写坐标'" placement="top">
              <span class="btn-wrap">
                <el-button size="small" type="primary" :disabled="!detailCoordOk" @click="navigate(detail)">🧭 祭扫导航</el-button>
              </span>
            </el-tooltip>
            <el-button v-if="canEdit()" size="small" @click="openEdit(detail)">编辑</el-button>
            <el-button size="small" text @click="detailVisible = false">关闭</el-button>
          </div>
        </div>

        <div class="info-grid">
          <div class="info-item"><span class="label">茔墓名称</span><span>{{ detail.name || '—' }}</span></div>
          <div class="info-item"><span class="label">墓主</span><span>{{ detail.owner_name || '—' }}</span></div>
          <div class="info-item"><span class="label">世次</span><span>{{ detail.generation ? '第 ' + detail.generation + ' 世' : '—' }}</span></div>
          <div class="info-item"><span class="label">生卒</span><span>{{ lifeText(detail) }}</span></div>
          <div class="info-item"><span class="label">山向分金</span><span>{{ detail.mountain_orientation || '—' }}</span></div>
          <div class="info-item"><span class="label">地理位置</span><span>{{ detail.address || '—' }}</span></div>
          <div class="info-item"><span class="label">经纬度</span><span>{{ fmtCoord(detail.lng, detail.lat) || '—' }}</span></div>
          <div class="info-item" v-if="detail.updated_at"><span class="label">更新时间</span><span>{{ detail.updated_at }}</span></div>
        </div>

        <div class="nav-bar" v-if="detailCoordOk">
          <span class="nav-tip">高德地图链接：</span>
          <a :href="markerUrl(detail)" target="_blank" rel="noopener" class="nav-link">{{ markerUrl(detail) }}</a>
        </div>
        <div class="nav-bar disabled-tip" v-else>
          <span>未填写坐标 —— 「地图查看」「祭扫导航」不可用；在「编辑」里填经纬度或粘贴地图链接即可。</span>
        </div>

        <div class="sec">
          <div class="sec-title serif">墓碑文</div>
          <div class="epitaph" v-if="detail.epitaph">{{ detail.epitaph }}</div>
          <div v-else class="empty-line">— 未填写 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">墓碑照片</div>
          <div class="img-show" v-if="(detail.photos || []).length">
            <el-image
              v-for="(p, i) in detail.photos" :key="'gp' + i"
              :src="thumbUrl(p, THUMB_LIST)" :preview-src-list="detail.photos.map(fullUrl)"
              :initial-index="i" preview-teleported fit="cover" class="show-img"
            />
          </div>
          <div v-else class="empty-line">— 未上传 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">墓域四至图</div>
          <div class="img-show" v-if="detail.boundary_image">
            <el-image
              :src="thumbUrl(detail.boundary_image, THUMB_LIST)"
              :preview-src-list="[fullUrl(detail.boundary_image)]" preview-teleported
              fit="cover" class="show-img wide"
            />
          </div>
          <div v-else class="empty-line">— 未上传 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">备注</div>
          <div class="free-text" v-if="detail.remark">{{ detail.remark }}</div>
          <div v-else class="empty-line">— 未填写 —</div>
        </div>
      </div>
    </el-drawer>

    <!-- ============ 编辑抽屉 ============ -->
    <el-drawer v-model="editVisible" :size="isMobile ? '100%' : '760px'" :with-header="false" class="edit-drawer">
      <div class="drawer-body">
        <div class="drawer-head">
          <span class="serif head-name">{{ form.id ? '编辑茔墓' : '新增茔墓' }}</span>
          <div class="head-actions" v-if="!isMobile">
            <el-button size="small" @click="editVisible = false">取消</el-button>
            <el-button v-if="canEdit()" size="small" type="primary" :loading="saving" @click="save">保存</el-button>
          </div>
        </div>

        <el-form :model="form" :label-width="isMobile ? '0px' : '96px'" :label-position="isMobile ? 'top' : 'right'" class="edit-form">
          <el-form-item label="关联墓主">
            <el-select
              v-model="form.member_id" filterable remote clearable class="w-full"
              placeholder="输入姓名搜索族谱成员（可留空，只写名字）"
              :remote-method="searchMembers" :loading="memberLoading" @change="onMemberChange"
            >
              <el-option
                v-for="m in memberOptions" :key="m.id"
                :label="`${m.name}（第 ${m.generation} 世${m.branch ? ' · ' + m.branch : ''}）`" :value="m.id"
              />
            </el-select>
            <div class="hint">关联后详情页可显示成员姓名；不关联也能只填墓主姓名。</div>
          </el-form-item>
          <el-form-item label="墓主姓名">
            <el-input v-model="form.owner_name" placeholder="如：始祖讳某某公 / 某氏" maxlength="60" />
          </el-form-item>
          <el-form-item label="茔墓名称">
            <el-input v-model="form.name" placeholder="留空则自动用墓主姓名（如：某某公墓）" maxlength="120" />
          </el-form-item>
          <el-form-item label="世次">
            <el-input v-model.number="form.generation" class="coord-in" placeholder="如 3" />
            <span class="hint inline">第几世（列表按「世次 → 姓名」排）</span>
          </el-form-item>
          <el-form-item label="生卒">
            <div class="coord-row">
              <el-input v-model="form.birth" placeholder="生（如：清乾隆十二年）" class="half" />
              <el-input v-model="form.death" placeholder="卒（如：嘉庆三年）" class="half" />
            </div>
          </el-form-item>

          <el-form-item label="山向分金">
            <el-input v-model="form.mountain_orientation" placeholder="如：坐乾向巽兼亥巳" maxlength="60" />
          </el-form-item>

          <el-form-item label="地理位置">
            <el-input v-model="form.address" placeholder="如：某村后山（可只写地址，坐标另填）" maxlength="240" />
          </el-form-item>
          <el-form-item label="经纬度">
            <div class="coord-row">
              <el-input v-model.number="form.lng" placeholder="经度 lng" class="coord-in" />
              <el-input v-model.number="form.lat" placeholder="纬度 lat" class="coord-in" />
              <el-button size="small" type="primary" plain @click="mapPickVisible = true">🗺️ 地图选点</el-button>
              <el-button size="small" @click="pasteCoords">粘贴链接提取</el-button>
              <el-button size="small" text @click="clearCoords">清空</el-button>
            </div>
            <div class="hint">
              可点「🗺️ 地图选点」在在线地图上单击取点（默认高德 GCJ-02，与祭扫导航同一坐标系）；也可粘贴高德/百度/腾讯地图链接后点「粘贴链接提取」自动换算（百度/腾讯的 lat,lng 会自动换序），或直接手填。
              填了坐标，详情页才有「地图查看」「祭扫导航」。
            </div>
          </el-form-item>

          <el-form-item label="墓碑文">
            <el-input v-model="form.epitaph" type="textarea" :autosize="{ minRows: 4, maxRows: 16 }" placeholder="按碑上原样分行录入（多行）" />
          </el-form-item>

          <el-form-item label="墓碑照片">
            <ImageListInput v-model="form.photos" category="茔墓" :can-upload="canEdit()" cover-badge
              tip="可多张：碑阳、碑阴、全景等；第一张作为列表封面。上传后自动进「家族多媒体」（分类：茔墓）。" />
          </el-form-item>
          <el-form-item label="墓域四至图">
            <ImageListInput v-model="form.boundaryList" category="茔墓" :can-upload="canEdit()" :limit="1"
              tip="单张（四至 / 墓域示意图）" />
          </el-form-item>

          <el-form-item label="备注">
            <el-input v-model="form.remark" type="textarea" :autosize="{ minRows: 2, maxRows: 8 }" placeholder="其他要记的事（保存于 cemeteries.description）" />
          </el-form-item>
        </el-form>

        <!-- 手机端：底部操作条吸底 -->
        <div v-if="isMobile" class="drawer-bottom-bar">
          <el-button size="large" @click="editVisible = false">取消</el-button>
          <el-button v-if="canEdit()" size="large" type="primary" :loading="saving" @click="save">保存</el-button>
        </div>
      </div>
    </el-drawer>

    <!-- 地图选点（复用「热力图 → 底图设置」的在线底图配置；默认高德 GCJ-02） -->
    <MapPicker
      v-model:visible="mapPickVisible"
      :lng="form.lng" :lat="form.lat" :name="form.owner_name || form.name"
      @confirm="onPickCoord"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { canEdit } from '../utils/auth'
import { thumbUrl, THUMB_LIST } from '../utils/thumb'
import { hasCoord, fmtCoord, parseCoords, amapMarkerUrl, amapNavUrl } from '../utils/geo'
import ImageListInput from '../components/ImageListInput.vue'
import LegacyCemeteryPanel from '../components/LegacyCemeteryPanel.vue'
import { useIsMobile } from '../utils/useIsMobile'
import MapPicker from '../components/MapPicker.vue'

const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const genFilter = ref('')
const onlyCoord = ref(false)
const legacyRef = ref(null)
const mapPickVisible = ref(false)
// 窄屏（<768px）：抽屉全宽、表单项上下排、底部操作条吸底
const isMobile = useIsMobile()

const detailVisible = ref(false)
const detail = ref(null)
const editVisible = ref(false)
const form = reactive(emptyForm())

const memberOptions = ref([])
const memberLoading = ref(false)

function emptyForm() {
  return {
    id: 0, name: '', owner_name: '', member_id: null, generation: null,
    birth: '', death: '', epitaph: '', mountain_orientation: '',
    address: '', lng: null, lat: null, photos: [], boundaryList: [], remark: ''
  }
}

function fullUrl(p) {
  const s = String(p || '')
  if (!s) return ''
  return s.startsWith('/') ? s : (s.startsWith('uploads/') ? '/' + s : '/uploads/' + s)
}

const detailCoordOk = computed(() => !!detail.value && hasCoord(detail.value.lng, detail.value.lat))

function lifeText(row) {
  if (!row) return '—'
  const b = row.birth || ''
  const d = row.death || ''
  if (!b && !d) return '—'
  return (b ? '生 ' + b : '') + (b && d ? ' · ' : '') + (d ? '卒 ' + d : '')
}

function markerUrl(item) { return amapMarkerUrl(item.lng, item.lat, item.owner_name || item.name) }
function navUrl(item) { return amapNavUrl(item.lng, item.lat, item.owner_name || item.name) }

async function load() {
  loading.value = true
  try {
    const params = {
      keyword: keyword.value || undefined,
      generation: genFilter.value === '' || genFilter.value === null ? undefined : genFilter.value,
      has_coord: onlyCoord.value ? 1 : undefined
    }
    const { data } = await api.get('/graves', { params })
    rows.value = (data && data.items) || []
  } catch (e) {
    ElMessage.error('读取茔墓志失败：' + (e?.response?.data?.error || e?.message || e))
    rows.value = []
  } finally {
    loading.value = false
  }
}

async function openDetail(row) {
  try {
    const { data } = await api.get('/graves/' + row.id)
    detail.value = data
    detailVisible.value = true
  } catch (e) {
    ElMessage.error('读取详情失败：' + (e?.response?.data?.error || e?.message || e))
  }
}

async function searchMembers(q) {
  const kw = String(q || '').trim()
  if (!kw) { memberOptions.value = []; return }
  memberLoading.value = true
  try {
    const { data } = await api.get('/members/search', { params: { q: kw } })
    memberOptions.value = Array.isArray(data) ? data : []
  } catch (e) {
    memberOptions.value = []
  } finally {
    memberLoading.value = false
  }
}

function onMemberChange(id) {
  const m = memberOptions.value.find((x) => x.id === id)
  if (m && !String(form.owner_name || '').trim()) form.owner_name = m.name
}

function openEdit(row) {
  Object.assign(form, emptyForm())
  if (row) {
    api.get('/graves/' + row.id).then(({ data }) => {
      Object.assign(form, {
        id: data.id,
        name: data.name || '',
        owner_name: data.owner_name || '',
        member_id: data.member_id || null,
        generation: data.generation || null,
        birth: data.birth || '',
        death: data.death || '',
        epitaph: data.epitaph || '',
        mountain_orientation: data.mountain_orientation || '',
        address: data.address || '',
        lng: data.lng || null,
        lat: data.lat || null,
        photos: [...(data.photos || [])],
        boundaryList: data.boundary_image ? [data.boundary_image] : [],
        remark: data.remark || ''
      })
      if (data.member_id && data.member_name) {
        memberOptions.value = [{ id: data.member_id, name: data.member_name, generation: data.generation || 0, branch: '' }]
      }
      editVisible.value = true
    }).catch((e) => {
      ElMessage.error('读取详情失败：' + (e?.response?.data?.error || e?.message || e))
    })
    return
  }
  editVisible.value = true
}

function clearCoords() {
  form.lng = null
  form.lat = null
}

async function pasteCoords() {
  let text = ''
  try {
    if (navigator.clipboard && navigator.clipboard.readText) text = await navigator.clipboard.readText()
  } catch (e) { text = '' }
  if (!text) {
    try {
      const r = await ElMessageBox.prompt('把高德/百度/腾讯地图链接或「经度,纬度」粘贴到这里', '粘贴坐标', {
        confirmButtonText: '提取', cancelButtonText: '取消',
        inputPlaceholder: 'https://api.map.baidu.com/marker?location=39.909,116.397'
      })
      text = r.value || ''
    } catch (e) { return }
  }
  const p = parseCoords(text)
  if (!p) {
    ElMessage.warning('没能从这段内容里认出坐标，请手动填写经纬度')
    return
  }
  form.lng = p.lng
  form.lat = p.lat
  ElMessage.success(`已提取坐标：${p.lng}, ${p.lat}`)
}

async function save() {
  if (!String(form.owner_name || '').trim() && !String(form.name || '').trim()) {
    ElMessage.warning('请至少填写墓主姓名或茔墓名称')
    return
  }
  saving.value = true
  try {
    const payload = {
      name: String(form.name || '').trim(),
      owner_name: String(form.owner_name || '').trim(),
      member_id: form.member_id || 0,
      generation: Number(form.generation) || 0,
      birth: form.birth || '',
      death: form.death || '',
      epitaph: form.epitaph || '',
      mountain_orientation: form.mountain_orientation || '',
      address: form.address || '',
      lng: Number(form.lng) || 0,
      lat: Number(form.lat) || 0,
      photos: form.photos || [],
      boundary_image: (form.boundaryList || [])[0] || '',
      remark: form.remark || ''
    }
    let res
    if (form.id) res = await api.put('/graves/' + form.id, payload)
    else res = await api.post('/graves', payload)
    ElMessage.success('已保存')
    editVisible.value = false
    await load()
    if (detailVisible.value) detail.value = res.data
    if (legacyRef.value) legacyRef.value.load()
  } catch (e) {
    ElMessage.error('保存失败：' + (e?.response?.data?.error || e?.message || e))
  } finally {
    saving.value = false
  }
}

async function del(row) {
  try {
    await ElMessageBox.confirm(`删除茔墓「${row.owner_name || row.name}」？不可恢复。`, '确认删除', {
      type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger'
    })
  } catch (e) { return }
  try {
    await api.delete('/graves/' + row.id)
    ElMessage.success('已删除')
    if (detail.value && detail.value.id === row.id) detailVisible.value = false
    await load()
  } catch (e) {
    ElMessage.error('删除失败：' + (e?.response?.data?.error || e?.message || e))
  }
}

function openMap(item) {
  const url = markerUrl(item)
  if (!url) { ElMessage.warning('未填写坐标'); return }
  window.open(url, '_blank')
}

function navigate(item) {
  const url = navUrl(item)
  if (!url) { ElMessage.warning('未填写坐标'); return }
  window.open(url, '_blank')
}

// 地图选点回填（底图坐标系即存库坐标系；默认高德 = GCJ-02，与「祭扫导航」一致）
function onPickCoord(p) {
  if (!p) return
  form.lng = p.lng
  form.lat = p.lat
  ElMessage.success(`已选点：${p.lng}, ${p.lat}`)
}

onMounted(load)
</script>

<style scoped>
.grave-page { min-height: 100%; }
.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.title-wrap { display: flex; align-items: baseline; gap: 10px; flex-wrap: wrap; }
.page-title { font-size: 20px; color: #3a3226; }
.sub { font-size: 12px; color: #9a8a74; }
.actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.kw { width: 200px; }
.gen { width: 80px; }
.list { background: #fff; }
.list :deep(.el-table__row) { cursor: pointer; }
.name-link { color: #3a3226; font-size: 15px; }
.linked { margin-left: 6px; }
.none { color: #bbb; }
.hint { font-size: 12px; color: #9a8a74; line-height: 1.5; }
.hint.inline { margin-left: 8px; }
.btn-wrap { display: inline-block; }

.drawer-body { padding: 16px 20px 28px; }
.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  background: #fff;
  z-index: 5;
  flex-wrap: wrap;
}
.head-left { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.head-name { font-size: 20px; color: #3a3226; }
.head-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin: 14px 0 6px; }
@media (max-width: 767px) { .info-grid { grid-template-columns: 1fr; } }
.info-item { display: flex; padding: 4px 0; font-size: 14px; }
.info-item .label { width: 80px; color: #8c5a3c; flex-shrink: 0; }
.nav-bar {
  margin: 10px 0 4px;
  padding: 8px 10px;
  border-radius: 6px;
  background: #f3f8f3;
  font-size: 12px;
  word-break: break-all;
}
.nav-bar.disabled-tip { background: #f7f3ec; color: #9a8a74; }
.nav-tip { color: #8c5a3c; }
.nav-link { color: #2f7d32; }
.sec { margin-top: 18px; padding-top: 12px; border-top: 1px dashed var(--border); }
.sec-title { font-size: 16px; color: #8c5a3c; margin-bottom: 10px; }
.free-text { font-size: 14px; line-height: 1.9; white-space: pre-wrap; color: #3a3226; }
.epitaph {
  font-family: "Songti SC", "SimSun", serif;
  font-size: 15px;
  line-height: 2;
  white-space: pre-wrap;
  color: #2f2a22;
  background: #faf7f1;
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 12px 14px;
}
.empty-line { font-size: 13px; color: #b0a494; }
.img-show { display: flex; flex-wrap: wrap; gap: 10px; }
.show-img { width: 124px; height: 124px; border-radius: 6px; border: 1px solid var(--border); cursor: zoom-in; }
.show-img.wide { width: 100%; max-width: 420px; height: 220px; }

.edit-form { margin-top: 14px; }

/* ============ 手机端（<768px）============ */
@media (max-width: 767px) {
  .drawer-body { padding: 12px 12px calc(84px + env(safe-area-inset-bottom)); }
  .drawer-head { padding-bottom: 8px; }
  .head-name { font-size: 17px; }
  .head-actions { flex-wrap: wrap; gap: 6px; }
  /* 详情/编辑抽屉头部的按钮也放大到 ≥40px（手机上更好点） */
  .head-actions .el-button { min-height: 40px; }

  .edit-form :deep(.el-form-item) { margin-bottom: 14px; }
  .edit-form :deep(.el-form-item__label) {
    font-size: 13px; line-height: 1.6; color: #8c5a3c;
    padding: 0 0 2px; margin-bottom: 2px; width: 100% !important; justify-content: flex-start;
  }
  .edit-form :deep(.el-form-item__content) { line-height: 1.5; }
  .edit-form :deep(.el-input__inner),
  .edit-form :deep(.el-textarea__inner),
  .edit-form :deep(.el-select__selected-item),
  .edit-form :deep(.el-select__placeholder) { font-size: 16px; }
  .edit-form :deep(.el-textarea__inner) { line-height: 1.7; }

  .coord-row { gap: 8px; }
  .coord-in { width: calc(50% - 4px); }
  .coord-row .el-button { min-height: 40px; flex: 1 1 auto; margin-left: 0; }
  .half { width: 100%; }

  .info-grid { grid-template-columns: 1fr; gap: 4px; }
  .info-item { font-size: 14px; }
  .info-item .label { width: 76px; }
  .img-show { gap: 8px; }
  .img-show .show-img { width: calc((100% - 16px) / 3); height: auto; aspect-ratio: 1 / 1; }
  .img-show .show-img.wide { width: 100%; max-width: 100%; height: auto; aspect-ratio: 4 / 3; }
  .nav-bar { font-size: 11px; }
  .epitaph { font-size: 14px; line-height: 1.9; padding: 10px; }
  .sec-title { font-size: 15px; }

  .drawer-bottom-bar {
    position: fixed; left: 0; right: 0; bottom: 0; z-index: 3000;
    display: flex; gap: 10px;
    padding: 10px 12px calc(10px + env(safe-area-inset-bottom));
    background: #fff; border-top: 1px solid var(--border);
    box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.06);
  }
  .drawer-bottom-bar .el-button { flex: 1; min-height: 44px; font-size: 16px; margin-left: 0; }

  .grave-page { overflow-x: hidden; }
  .actions { width: 100%; }
  .actions .kw { width: 100%; }
}
.coord-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; width: 100%; }
.coord-in { width: 130px; }
.half { width: calc(50% - 6px); }
.w-full { width: 100%; }
</style>
