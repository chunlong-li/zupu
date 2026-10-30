<template>
  <div class="migration-view">
    <div class="toolbar">
      <h2>🚶 家族迁徙记录</h2>
      <div class="spacer" />
      <el-button v-if="canEdit()" type="primary" @click="openForm()">新增迁徙事件</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-alert type="info" :closable="false" class="tip">
      录入「迁出地 → 迁入地」后，热力分布页的迁徙轨迹会优先使用这些事件（含年代与说明），比按「籍贯→葬地」推算更准确。
      「涉及成员」填写成员 ID，用英文逗号分隔（如 12,15,33），仅用于统计人数。
    </el-alert>

    <el-table :data="list" v-loading="loading" border stripe style="width:100%">
      <el-table-column prop="year" label="年代" width="90" />
      <el-table-column prop="era" label="纪年" width="140" />
      <el-table-column prop="from_place" label="迁出地" min-width="160" />
      <el-table-column label="" width="40" align="center">→</el-table-column>
      <el-table-column prop="to_place" label="迁入地" min-width="160" />
      <el-table-column prop="branch" label="房支" width="110" />
      <el-table-column prop="member_count" label="人数" width="70" />
      <el-table-column prop="description" label="说明" min-width="180" show-overflow-tooltip />
      <el-table-column v-if="canEdit()" label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openForm(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="form.id ? '编辑迁徙事件' : '新增迁徙事件'" width="560px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="迁出地">
          <el-input v-model="form.from_place" placeholder="如：江西省吉安市吉州区" />
          <el-select
            v-if="selectedPlaces.length"
            v-model="pickedPlace"
            class="mig-place-picker"
            size="small"
            clearable
            :placeholder="selectedPlaces.length > 1 ? '多个地点，请选择' : '取成员现居地/籍贯'"
            @change="onPickPlace">
            <el-option v-for="p in selectedPlaces" :key="p" :label="p" :value="p" />
          </el-select>
          <div v-if="selectedPlaces.length > 1" class="mig-place-tip">涉及成员有 {{ selectedPlaces.length }} 个不同地点（现居地/籍贯），请选择迁出地</div>
        </el-form-item>
        <el-form-item label="迁入地">
          <el-input v-model="form.to_place" placeholder="如：湖南省长沙市" />
        </el-form-item>
        <el-form-item label="公元年份">
          <el-input-number v-model="form.year" :min="0" :max="2999" :controls="false" placeholder="如 1369" />
        </el-form-item>
        <el-form-item label="纪年">
          <el-input v-model="form.era" placeholder="如：明洪武二年（可空）" />
        </el-form-item>
        <el-form-item label="房支">
          <el-select v-model="form.branch" clearable placeholder="可空，留空表示不区分房支" style="width:100%" @change="onBranchChange">
            <el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.name" />
          </el-select>
        </el-form-item>
        <el-form-item label="涉及成员">
          <el-button @click="openMemberPicker">选择成员</el-button>
          <span class="mig-sel">{{ selectedSummary }}</span>
        </el-form-item>
        <el-form-item label="同步现居地">
          <el-checkbox v-model="form.sync_residence">保存后把涉及成员的「现居地」更新为迁入地</el-checkbox>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="如：洪武大移民，自江西填湖广" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="memberDialog" title="选择涉及成员" width="600px">
      <div class="mp-bar">
        <el-input v-model="memberSearch" placeholder="搜索姓名" clearable style="width:180px" />
        <el-select v-model="memberBranchFilter" placeholder="全部房支" clearable style="width:140px">
          <el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.name" />
        </el-select>
        <el-button size="small" @click="selectAllFiltered">全选当前</el-button>
        <el-button size="small" @click="clearSelected">清空</el-button>
        <span class="mp-cnt">已选 {{ selectedIds.length }} 人</span>
      </div>
      <div class="mp-list">
        <el-checkbox-group v-model="selectedIds">
          <el-checkbox v-for="m in filteredMembers" :key="m.id" :value="m.id" class="mp-item">
            {{ memberName(m) }}<span class="mp-meta">（{{ m.generation }}世 · {{ m.branch || '未分房' }}）</span>
          </el-checkbox>
        </el-checkbox-group>
        <div v-if="!filteredMembers.length" class="mp-empty">无匹配成员</div>
      </div>
      <template #footer>
        <el-button @click="memberDialog = false">完成</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { canEdit } from '../utils/auth'

const list = ref([])
const loading = ref(false)
const saving = ref(false)
const dialog = ref(false)
const branches = ref([])
const members = ref([])
const selectedIds = ref([])
const pickedPlace = ref('')
const lastAutoPlace = ref('')
const memberDialog = ref(false)
const memberSearch = ref('')
const memberBranchFilter = ref('')

const emptyForm = () => ({ id: 0, from_place: '', to_place: '', era: '', year: null, branch: '', member_ids: '', description: '', sync_residence: true })
const form = reactive(emptyForm())

const memberMap = computed(() => { const map = {}; members.value.forEach(m => { map[m.id] = m }); return map })
const memberName = m => m.name || ((m.surname || '') + (m.given_name || ''))
const filteredMembers = computed(() => {
  const q = memberSearch.value.trim().toLowerCase()
  const bf = memberBranchFilter.value
  return members.value.filter(m => {
    if (bf && (m.branch || '') !== bf) return false
    if (!q) return true
    return (memberName(m) || '').toLowerCase().includes(q)
  })
})
// 所选成员的现居地（无则回退籍贯）；去重，按出现次数由多到少
const selectedPlaces = computed(() => {
  const counter = new Map()
  selectedIds.value.forEach(id => {
    const m = memberMap.value[id]
    const p = m ? (String(m.current_place || '').trim() || String(m.native_place || '').trim()) : ''
    if (p) counter.set(p, (counter.get(p) || 0) + 1)
  })
  return [...counter.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(e => e[0])
})
// 现居地唯一时自动填入迁出地（不覆盖手工填写的内容）
watch(selectedPlaces, ps => {
  if (ps.length === 1) {
    if (!form.from_place || form.from_place === lastAutoPlace.value) {
      form.from_place = ps[0]
      lastAutoPlace.value = ps[0]
    }
  } else if (ps.length === 0 && form.from_place === lastAutoPlace.value) {
    form.from_place = ''
    lastAutoPlace.value = ''
  }
})
function onPickPlace(p) {
  if (p) {
    form.from_place = p
    lastAutoPlace.value = p
  }
}
const selectedSummary = computed(() => {
  const n = selectedIds.value.length
  if (!n) return '未选择'
  const names = selectedIds.value.map(id => { const m = memberMap.value[id]; return m ? memberName(m) : ('#' + id) })
  return `已选 ${n} 人：` + names.slice(0, 8).join('、') + (names.length > 8 ? ` 等 ${n} 人` : '')
})

async function loadRefs() {
  try { const { data } = await api.get('/branches'); branches.value = data || [] } catch (e) { /* 忽略 */ }
  try { const { data } = await api.get('/members'); members.value = Array.isArray(data) ? data : (data.list || []) } catch (e) { /* 忽略 */ }
}

function openMemberPicker() { memberSearch.value = ''; memberBranchFilter.value = ''; memberDialog.value = true }
function selectAllFiltered() { const s = new Set(selectedIds.value); filteredMembers.value.forEach(m => s.add(m.id)); selectedIds.value = [...s] }
function clearSelected() { selectedIds.value = [] }
// 选房支时询问是否一并选中该房支全部成员
async function onBranchChange(name) {
  if (!name) return
  const ids = members.value.filter(m => (m.branch || '') === name).map(m => m.id)
  if (!ids.length) return
  try {
    await ElMessageBox.confirm(`是否同时选中「${name}」房支下的全部 ${ids.length} 名成员？`, '房支成员', { confirmButtonText: '一起选中', cancelButtonText: '不用', type: 'info' })
  } catch { return }
  const s = new Set(selectedIds.value)
  ids.forEach(i => s.add(i))
  selectedIds.value = [...s]
  ElMessage.success(`已加入「${name}」房支 ${ids.length} 名成员`)
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/migrations')
    list.value = Array.isArray(data) ? data : (data.list || [])
  } catch (e) {
    ElMessage.error('加载失败：' + (e.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

function openForm(row) {
  Object.assign(form, emptyForm())
  pickedPlace.value = ''
  lastAutoPlace.value = ''
  if (row) Object.assign(form, row)
  selectedIds.value = String(form.member_ids || '').split(',').map(s => parseInt(s, 10)).filter(n => n > 0)
  dialog.value = true
}

async function save() {
  if (!form.from_place || !form.to_place) {
    ElMessage.warning('请填写迁出地与迁入地')
    return
  }
  saving.value = true
  const payload = { ...form, year: form.year || 0, member_ids: selectedIds.value.join(',') }
  try {
    if (form.id) await api.put(`/migrations/${form.id}`, payload)
    else await api.post('/migrations', payload)
    ElMessage.success('已保存')
    dialog.value = false
    await loadRefs()
    load()
  } catch (e) {
    ElMessage.error('保存失败：' + (e.response?.data?.error || e.message))
  } finally {
    saving.value = false
  }
}

async function remove(row) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.from_place} → ${row.to_place}」这条迁徙记录？`, '确认删除', { type: 'warning' })
  } catch { return }
  try {
    await api.delete(`/migrations/${row.id}`)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    ElMessage.error('删除失败：' + (e.response?.data?.error || e.message))
  }
}

onMounted(() => { load(); loadRefs() })
</script>

<style scoped>
.migration-view { padding: 16px; }
.toolbar { display: flex; align-items: center; margin-bottom: 12px; }
.toolbar h2 { margin: 0; font-size: 18px; }
.spacer { flex: 1; }
.tip { margin-bottom: 12px; }
.mig-sel { margin-left: 10px; color: #6b7280; font-size: 13px; }
.mig-place-picker { margin-left: 8px; width: 200px; }
.mig-place-tip { width: 100%; margin-top: 4px; color: #e6a23c; font-size: 12px; line-height: 1.4; }
.mp-bar { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
.mp-cnt { margin-left: auto; color: #6b7280; font-size: 13px; }
.mp-list { max-height: 360px; overflow-y: auto; border: 1px solid var(--border, #e5e7eb); border-radius: 6px; padding: 8px 12px; }
.mp-list :deep(.el-checkbox) { display: flex; height: auto; margin: 2px 0; }
.mp-item { width: 100%; }
.mp-meta { color: #9aa2ad; font-size: 12px; }
.mp-empty { color: #9aa2ad; font-size: 13px; padding: 12px 4px; text-align: center; }
</style>
