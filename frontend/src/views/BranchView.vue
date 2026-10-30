<template>
  <div>
    <div class="toolbar">
      <el-button type="primary" v-if="canEdit" @click="openAdd(null)">➕ 新增一级房支</el-button>
      <el-button type="warning" v-if="canBackfill" @click="backfill">🔧 按祖先回填未分房成员</el-button>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-alert type="info" :closable="false" class="tip">
      房支支持多级：宗谱 → 长房 / 二房 → 其下的子房（如「长房」下的「长房 · 廷玉公」）。
      命名建议「序数（长房/二房）+ 名讳」区分；成员可归属到任意一级房支，未归入的进「未分房」。
    </el-alert>

    <div class="table-responsive">
      <el-table :data="tree" border stripe style="width: 100%" row-key="id"
        :tree-props="{ children: 'children' }" default-expand-all>
        <el-table-column label="房支" min-width="200">
          <template #default="s">
            <span class="bname">{{ s.row.name }}</span>
            <span v-if="s.row.subtitle" class="bsub">· {{ s.row.subtitle }}</span>
            <el-tag v-if="s.row.level === 1" size="small" type="info" effect="plain" class="ltag">一级</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="层级" width="70" align="center">
          <template #default="s">第 {{ s.row.level }} 层</template>
        </el-table-column>
        <el-table-column prop="sort_order" label="排序" width="70" align="center" />
        <el-table-column label="直属成员" width="90" align="center">
          <template #default="s">{{ s.row.count || 0 }}</template>
        </el-table-column>
        <el-table-column label="含下级" width="80" align="center">
          <template #default="s">{{ s.row.count_all || 0 }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="120" show-overflow-tooltip />
        <el-table-column label="操作" width="360" v-if="canEdit">
          <template #default="s">
            <el-button size="small" type="primary" plain @click="openAdd(s.row)">➕ 下级</el-button>
            <el-button size="small" @click="openMembers(s.row)">分配成员</el-button>
            <el-button size="small" type="primary" @click="openEdit(s.row)">编辑</el-button>
            <el-button size="small" type="danger" @click="del(s.row)">删除</el-button>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" v-else>
          <template #default="s">
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 新增/编辑房支 -->
    <el-dialog v-model="editVisible" :title="form.id ? '编辑房支' : '新增房支'" width="min(460px, 94vw)">
      <el-form :model="form" label-width="80px">
        <el-form-item label="上级房支">
          <el-tree-select v-model="form.parent_id" :data="parentOptions" node-key="value"
            :props="{ label: 'label', children: 'children' }" check-strictly :render-after-expand="false"
            style="width:100%" placeholder="宗谱（顶级）" />
        </el-form-item>
        <el-form-item label="房支名">
          <el-input v-model="form.name" placeholder="如：长房 / 二房 / 三房" />
        </el-form-item>
        <el-form-item label="名讳">
          <el-input v-model="form.subtitle" placeholder="选填，如：廷玉公（用于区分同名房支）" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" placeholder="选填" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 分配成员 -->
    <el-dialog v-model="memberVisible" :title="`分配成员到「${curLabel}」`" width="min(560px, 94vw)">
      <el-alert type="info" :closable="false" class="tip">
        勾选要归入该房支的成员，保存后立即生效：取消勾选的成员会被移出（归入未分房）。
      </el-alert>
      <el-table :data="allMembers" border max-height="360" @selection-change="onSelectChange" ref="memberTable">
        <el-table-column type="selection" width="45" />
        <el-table-column label="姓名" min-width="100">
          <template #default="s">{{ s.row.name || (s.row.surname || '') + (s.row.given_name || '') }}</template>
        </el-table-column>
        <el-table-column prop="generation" label="世代" width="70">
          <template #default="s">第 {{ s.row.generation }} 世</template>
        </el-table-column>
        <el-table-column label="当前房支" min-width="110">
          <template #default="s">{{ branchName(s.row.branch_id) || '未分房' }}</template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="memberVisible = false">取消</el-button>
        <el-button type="primary" @click="saveMembers">保存分配</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { branchCtx, branchLabel, loadBranches } from '../utils/branchContext'

const router = useRouter()
const branches = ref([])
const allMembers = ref([])
const editVisible = ref(false)
const memberVisible = ref(false)
const currentBranch = ref({})
const form = reactive({ id: 0, name: '', subtitle: '', parent_id: 0, sort_order: 0, remark: '' })
const memberTable = ref(null)
let selectedMembers = []

const me = ref(null)
const canEdit = ref(false)
const canBackfill = ref(false)

const curLabel = computed(() => branchLabel(currentBranch.value))

function updateRole() {
  try {
    const u = JSON.parse(localStorage.getItem('user') || 'null')
    me.value = u
    canEdit.value = !!u && ['理事', '房支理事', '理事长', '超级管理员'].includes(u.role)
    canBackfill.value = !!u && ['理事长', '超级管理员'].includes(u.role)
  } catch (e) {
    canEdit.value = false
    canBackfill.value = false
  }
}

function branchName(id) {
  const b = branches.value.find(x => x.id === id)
  return b ? branchLabel(b) : ''
}

// 扁平 → 树
const tree = computed(() => {
  const nodes = branches.value.map(b => ({ ...b, children: [] }))
  const byId = {}
  nodes.forEach(n => { byId[n.id] = n })
  const roots = []
  nodes.forEach(n => {
    if (n.parent_id && byId[n.parent_id]) byId[n.parent_id].children.push(n)
    else roots.push(n)
  })
  const sortRec = list => {
    list.sort((a, b) => (a.sort_order - b.sort_order) || (a.id - b.id))
    list.forEach(x => sortRec(x.children))
  }
  sortRec(roots)
  return roots
})

// 上级候选（编辑时排除自己及后代，避免循环）
const parentOptions = computed(() => {
  const exclude = new Set()
  if (form.id) {
    const collect = id => {
      exclude.add(id)
      branches.value.filter(b => b.parent_id === id).forEach(b => collect(b.id))
    }
    collect(form.id)
  }
  const nodes = branches.value.filter(b => !exclude.has(b.id))
    .map(b => ({ value: b.id, label: branchLabel(b), parent_id: b.parent_id || 0, children: [] }))
  const byId = {}
  nodes.forEach(n => { byId[n.value] = n })
  const roots = []
  nodes.forEach(n => {
    if (n.parent_id && byId[n.parent_id]) byId[n.parent_id].children.push(n)
    else roots.push(n)
  })
  return [{ value: 0, label: '宗谱（顶级）', children: roots }]
})

async function load() {
  const { data } = await api.get('/branches')
  branches.value = data || []
  branchCtx.branches = branches.value
  branchCtx.loaded = true
}

function goPushu(row) {
  router.push(`/branch/${row.id}/pushu`)
}

function openAdd(parentRow) {
  Object.assign(form, {
    id: 0, name: '', subtitle: '',
    parent_id: parentRow ? parentRow.id : 0,
    sort_order: parentRow ? (branches.value.filter(b => b.parent_id === parentRow.id).length) : branches.value.filter(b => !b.parent_id).length,
    remark: ''
  })
  editVisible.value = true
}
function openEdit(row) {
  Object.assign(form, { id: row.id, name: row.name, subtitle: row.subtitle || '', parent_id: row.parent_id || 0, sort_order: row.sort_order, remark: row.remark })
  editVisible.value = true
}

async function save() {
  if (!form.name || !form.name.trim()) { ElMessage.warning('请输入房支名'); return }
  try {
    if (form.id) await api.put(`/branches/${form.id}`, form)
    else await api.post('/branches', form)
    ElMessage.success('已保存')
    editVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  }
}

async function del(row) {
  try {
    await ElMessageBox.confirm(`确定删除房支「${branchLabel(row)}」？该房支下的成员将变为「未分房」，成员数据不受影响。`, '删除房支', { type: 'warning' })
  } catch { return }
  try {
    await api.delete(`/branches/${row.id}`)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  }
}

async function openMembers(row) {
  currentBranch.value = row
  const { data } = await api.get('/members')
  allMembers.value = data || []
  memberVisible.value = true
  selectedMembers = []
  setTimeout(() => {
    if (!memberTable.value) return
    allMembers.value.forEach(m => {
      if (m.branch_id === row.id) memberTable.value.toggleRowSelection(m, true)
    })
  }, 0)
}

function onSelectChange(rows) {
  selectedMembers = rows
}

async function saveMembers() {
  const ids = selectedMembers.map(m => m.id)
  try {
    const { data } = await api.post('/branches/set-members', { branch_id: currentBranch.value.id, member_ids: ids, replace: true })
    const extra = data?.cascaded ? `，${data.cascaded} 个后代已同步归入` : ''
    ElMessage.success('分配已保存' + extra)
    memberVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '分配失败')
  }
}

async function backfill() {
  try {
    await ElMessageBox.confirm(
      '将把所有未分房成员，按「最近有房支的祖先」自动归位到对应房支（已有房支的成员不受影响）。是否继续？',
      '按祖先回填房支',
      { type: 'warning', confirmButtonText: '回填', cancelButtonText: '取消' }
    )
  } catch { return }
  try {
    const { data } = await api.post('/branches/backfill')
    ElMessage.success(`已回填 ${data.filled ?? 0} 人`)
    load()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '回填失败')
  }
}

onMounted(() => {
  updateRole()
  load()
  loadBranches()
})
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.tip { margin-bottom: 12px; }
.table-responsive { overflow-x: auto; -webkit-overflow-scrolling: touch; }
.bname { font-weight: 600; }
.bsub { color: #8c5a3c; margin-left: 6px; font-size: 13px; }
.ltag { margin-left: 8px; }
</style>
