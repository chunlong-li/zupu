<template>
  <div class="test-zupu-page">
    <div class="toolbar">
      <span class="hint">🧪 传代树 · {{ canEdit() ? '数据联动自族谱系统，改动后点「同步到族谱」写回' : '数据联动自族谱系统（只读）' }}</span>
      <el-radio-group v-model="viewMode" size="small" @change="onViewModeChange">
        <el-radio-button value="内纪">内纪</el-radio-button>
        <el-radio-button value="外纪">外纪</el-radio-button>
      </el-radio-group>
      <el-select v-model="branchFilter" size="small" style="width:120px" @change="onBranchChange">
        <el-option label="全部" value="all" />
        <el-option label="未分房" value="unassigned" />
        <el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.id" />
      </el-select>
      <span v-if="dirty" class="dirty">● 有未同步修改</span>
      <div class="actions">
        <el-button size="small" @click="reloadFromSystem" :loading="loading">⟳ 重新加载</el-button>
        <el-button v-if="canEdit()" size="small" type="primary" @click="syncToSystem" :loading="syncing">同步到族谱</el-button>
      </div>
    </div>
    <div v-if="error" class="error-bar">⚠️ {{ error }}</div>
    <div class="frame-wrap">
      <iframe v-if="ready" :key="frameKey" src="/zupu-test/index.html" class="zupu-frame" title="zaxchou/zupu 传代树" />
      <div v-else class="loading">正在加载族谱数据…</div>
    </div>

    <el-dialog v-model="syncDialogVisible" title="同步到族谱" width="480px" :close-on-click-modal="false">
      <div class="sync-confirm">
        <p v-if="syncDeleted.length" class="del-warn">
          本次修改删除包括 {{ deletedNamesShown }} 等{{ syncDeleted.length }} 人。
        </p>
        <p v-else class="del-none">本次修改没有删除成员。</p>
      </div>
      <template #footer>
        <el-button @click="cancelSync">取消</el-button>
        <el-button @click="discardSync" :loading="loading">不同步</el-button>
        <el-button type="danger" @click="confirmSync" :loading="syncing">同步</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="branchPromptVisible" title="房支后代未归入" width="480px" :close-on-click-modal="false" :show-close="false">
      <div class="bp-body">
        <p>房支「{{ pendingBranchName }}」下有 <b>{{ unassignedDesc.length }}</b> 名后代尚未归入房支。</p>
        <p class="bp-sub">是否在传代树中显示这些后代？</p>
      </div>
      <template #footer>
        <el-button @click="branchPromptChoose('no')">否</el-button>
        <el-button @click="branchPromptChoose('show')">是，不加入房支</el-button>
        <el-button v-if="canEdit()" type="primary" @click="branchPromptChoose('join')">是，加入房支</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { canEdit } from '../utils/auth'
import { findUnassignedDescendants } from '../utils/branchTree'

const STORAGE_KEY = 'zupu_data_v4'

const frameKey = ref(0)
const ready = ref(false)
const loading = ref(false)
const syncing = ref(false)
const dirty = ref(false)
const error = ref('')
const viewMode = ref('内纪')
const branchFilter = ref('all') // 'all' = 全部，'unassigned' = 未分房，数字 = 房支 id
const branches = ref([])
const extraShowIds = ref([]) // 选择「不加入但显示」时额外显示的未分房后代 id
const branchPromptVisible = ref(false)
const unassignedDesc = ref([])
const pendingBranchId = ref(null)
const pendingBranchName = ref('')
const syncDialogVisible = ref(false)
const syncDeleted = ref([])

let familySurname = ''
let familyName = ''
let cachedMembers = []
let cachedGw = []
let loadedAt = 0 // iframe 刚加载时 zaxchou 首次渲染会自动写一次 localStorage，静默期避免误标 dirty

// 提取年份：支持 "1980"、"1980-05-20"、"1980年" 等，只取前 4 位数字
function yearOf(s) {
  if (!s) return ''
  const m = String(s).match(/\d{1,4}/)
  return m ? m[0] : ''
}

// 成员 → zaxchou 节点（id 用 "m"+id 保留映射）
function memberToNode(m) {
  const node = {
    id: 'm' + m.id,
    name: m.name || ((m.surname || '') + (m.given_name || '')),
    spouses: (m.spouses || []).map(s => s.name || '').filter(Boolean),
    birth: yearOf(m.birth_gregorian),
    death: yearOf(m.death_gregorian),
    note: m.biography || '',
    zi: m.zi || '',
    expanded: true,
    children: []
  }
  if (m.gender === 0) node.gender = 'f'
  return node
}

// 按房支筛选成员：'all' 全部；'unassigned' 未分房；数字=该房支（含额外显示的后代）
function filterByBranch(members, branchId, extraIds) {
  if (branchId === 'all') return members
  if (branchId === 'unassigned') return members.filter(m => (m.branch_id || 0) === 0)
  const extra = new Set(extraIds || [])
  return members.filter(m => (m.branch_id || 0) === branchId || extra.has(m.id))
}

// 按 father_id 构建嵌套树（根 = father_id 为空或指向不存在成员）；mode 决定只看内纪还是外纪；branchId 决定只看某房支
function buildTree(members, mode, branchId, extraIds) {
  let filtered = filterByBranch(members, branchId, extraIds)
  filtered = filtered.filter(m => mode === '外纪' ? m.record_type === '外纪' : m.record_type !== '外纪')
  const byId = new Map(filtered.map(m => [m.id, m]))
  const roots = filtered.filter(m => !m.father_id || !byId.has(m.father_id))
  const byGenId = (a, b) => (a.generation || 0) - (b.generation || 0) || a.id - b.id
  function build(m) {
    const node = memberToNode(m)
    node.children = filtered.filter(c => c.father_id === m.id).sort(byGenId).map(build)
    return node
  }
  return roots.sort(byGenId).map(build)
}

// 构建 zaxchou 完整树（含 root / 谱序 / 字辈）
function buildZupuTree(members, gw, name, mode, branchId, extraIds) {
  let zibei = []
  if (gw && gw.length) {
    try { zibei = JSON.parse(gw[0].words_array || '[]') } catch (e) { /* ignore */ }
  }
  return {
    id: 'root',
    name,
    spouses: [],
    expanded: true,
    clan: { ming: name, tang: '', chain: '', yuanzu: '', shizu: '', qianzu: '', origin: '' },
    zibei,
    children: buildTree(members, mode, branchId, extraIds)
  }
}

async function loadZupuFromSystem() {
  loading.value = true
  error.value = ''
  try {
    const [mRes, gwRes, famRes, surnameRes, brRes] = await Promise.all([
      api.get('/members/with-spouses'),
      api.get('/generation-words'),
      api.get('/settings/family-name'),
      api.get('/settings/family-surname'),
      api.get('/branches')
    ])
    cachedMembers = mRes.data || []
    cachedGw = gwRes.data || []
    familyName = famRes.data.family_name || '族谱'
    familySurname = surnameRes.data.family_surname || ''
    branches.value = brRes.data || []
    rebuildTree()
  } catch (e) {
    error.value = '加载族谱数据失败：' + (e.response?.data?.error || e.message)
    ElMessage.error(error.value)
  } finally {
    loading.value = false
  }
}

function rebuildTree() {
  const tree = buildZupuTree(cachedMembers, cachedGw, familyName, viewMode.value, branchFilter.value, extraShowIds.value)
  localStorage.setItem(STORAGE_KEY, JSON.stringify(tree))
  dirty.value = false
  loadedAt = Date.now()
  frameKey.value++
  ready.value = true
}

function reloadFromSystem() { loadZupuFromSystem() }

// 切换内纪/外纪视图
function onViewModeChange(newMode) {
  if (!dirty.value) {
    rebuildTree()
    return
  }
  ElMessageBox.confirm('有未同步的修改，切换视图将丢弃这些修改，是否继续？', '提示', {
    type: 'warning', confirmButtonText: '继续切换', cancelButtonText: '取消'
  }).then(() => {
    rebuildTree()
  }).catch(() => {
    viewMode.value = newMode === '外纪' ? '内纪' : '外纪'
  })
}

// 切换房支视图
let branchBefore = 'all'
function onBranchChange() {
  if (!dirty.value) {
    branchBefore = branchFilter.value
    proceedBranchFilter()
    return
  }
  ElMessageBox.confirm('有未同步的修改，切换房支将丢弃这些修改，是否继续？', '提示', {
    type: 'warning', confirmButtonText: '继续切换', cancelButtonText: '取消'
  }).then(() => {
    branchBefore = branchFilter.value
    proceedBranchFilter()
  }).catch(() => {
    branchFilter.value = branchBefore
  })
}

function proceedBranchFilter() {
  const v = branchFilter.value
  if (v === 'all' || v === 'unassigned') {
    extraShowIds.value = []
    rebuildTree()
    return
  }
  const desc = findUnassignedDescendants(cachedMembers, v)
  if (!desc.length) {
    extraShowIds.value = []
    rebuildTree()
    return
  }
  unassignedDesc.value = desc
  pendingBranchId.value = v
  pendingBranchName.value = (branches.value.find(b => b.id === v) || {}).name || '该房支'
  branchPromptVisible.value = true
}

function branchPromptChoose(action) {
  branchPromptVisible.value = false
  if (action === 'join') {
    joinUnassigned()
  } else if (action === 'show') {
    extraShowIds.value = unassignedDesc.value.map(m => m.id)
    rebuildTree()
  } else {
    extraShowIds.value = []
    rebuildTree()
  }
}

async function joinUnassigned() {
  const bid = pendingBranchId.value
  const ids = unassignedDesc.value.map(m => m.id)
  try {
    await api.post('/branches/set-members', { branch_id: bid, member_ids: ids, replace: false })
    ElMessage.success(`已将 ${ids.length} 名后代归入房支`)
    const { data } = await api.get('/members/with-spouses')
    cachedMembers = data || []
  } catch (e) {
    ElMessage.error('归入房支失败：' + (e.response?.data?.error || e.message))
  }
  extraShowIds.value = []
  rebuildTree()
}

// iframe 内 zaxchou 保存数据 → localStorage 变化 → 父页面收到 storage 事件
function onStorage(e) {
  if (e.key !== STORAGE_KEY) return
  if (Date.now() - loadedAt < 1500) return // 首次渲染静默期
  dirty.value = true
}

// 按家族姓氏拆分姓名
function splitName(name) {
  name = (name || '').trim()
  if (!name) return { surname: '', given_name: '' }
  if (familySurname && name.startsWith(familySurname)) {
    return { surname: familySurname, given_name: name.slice(familySurname.length) }
  }
  return { surname: name[0] || '', given_name: name.slice(1) }
}

async function updateMember(memberId, node, fatherId, current) {
  const { surname, given_name } = splitName(node.name)
  const gender = node.gender === 'f' ? 0 : 1
  const zi = node.zi || ''
  const biography = node.note || ''
  const birth = node.birth || ''
  const death = node.death || ''
  const newFatherId = fatherId || 0

  const curName = (current.surname || '') + (current.given_name || '')
  const changed =
    (surname + given_name) !== curName ||
    gender !== (current.gender == null ? 1 : current.gender) ||
    newFatherId !== (current.father_id || 0) ||
    birth !== yearOf(current.birth_gregorian) ||
    death !== yearOf(current.death_gregorian) ||
    zi !== (current.zi || '') ||
    biography !== (current.biography || '')

  if (!changed) return false

  // 基于现有完整数据做部分更新，避免把未同步字段清空
  const payload = { ...current, surname, given_name, gender, father_id: newFatherId, zi, biography }
  if (birth !== yearOf(current.birth_gregorian)) payload.birth_gregorian = birth
  if (death !== yearOf(current.death_gregorian)) payload.death_gregorian = death

  await api.put(`/members/${memberId}`, payload)
  return true
}

async function createMember(node, fatherId) {
  const { surname, given_name } = splitName(node.name)
  const payload = {
    surname,
    given_name,
    gender: node.gender === 'f' ? 0 : 1,
    father_id: fatherId || 0,
    record_type: viewMode.value === '外纪' ? '外纪' : '内纪',
    zi: node.zi || '',
    birth_gregorian: node.birth || '',
    death_gregorian: node.death || '',
    biography: node.note || ''
  }
  if (typeof branchFilter.value === 'number') payload.branch_id = branchFilter.value
  const { data } = await api.post('/members', payload)
  return data.id || null
}

async function syncSpouses(memberId, node, current) {
  const want = (node.spouses || []).map(s => (s || '').trim()).filter(Boolean)
  const curNames = (current?.spouses || []).map(s => (s.name || '').trim())
  let added = 0
  for (const wn of want) {
    if (!curNames.includes(wn)) {
      const { surname, given_name } = splitName(wn)
      await api.post('/spouses', { member_id: memberId, surname, given_name, type: '正配' })
      added++
    }
  }
  return added
}

// 被删成员姓名（弹窗展示，最多前 5 个）
const deletedNamesShown = computed(() => {
  const names = syncDeleted.value
    .map(m => (m.name || ((m.surname || '') + (m.given_name || '')).trim()))
    .filter(Boolean)
  return names.slice(0, 5).join('，')
})

// 检测被删成员：当前视图模式下，缓存成员中不在 iframe 树里的
function detectDeleted() {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) return []
  let tree
  try { tree = JSON.parse(raw) } catch (e) { return [] }
  const treeIds = new Set()
  ;(function walk(node) {
    (node.children || []).forEach(c => {
      const mm = /^m(\d+)$/.exec(c.id || '')
      if (mm) treeIds.add(parseInt(mm[1], 10))
      walk(c)
    })
  })(tree)
  const mode = viewMode.value
  const bid = branchFilter.value
  const extra = new Set(extraShowIds.value || [])
  return cachedMembers.filter(m => {
    const inMode = mode === '外纪' ? m.record_type === '外纪' : m.record_type !== '外纪'
    const inBranch = bid === 'all'
      ? true
      : bid === 'unassigned'
        ? (m.branch_id || 0) === 0
        : ((m.branch_id || 0) === bid || extra.has(m.id))
    return inMode && inBranch && !treeIds.has(m.id)
  })
}

async function syncToSystem() {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) { ElMessage.warning('没有可同步的数据'); return }
  const del = detectDeleted()
  if (!dirty.value && del.length === 0) { ElMessage.info('没有需要同步的修改'); return }
  syncDeleted.value = del
  syncDialogVisible.value = true
}

function confirmSync() {
  syncDialogVisible.value = false
  doSync()
}

function discardSync() {
  syncDialogVisible.value = false
  loadZupuFromSystem()
}

function cancelSync() {
  syncDialogVisible.value = false
}

async function doSync() {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) return
  syncing.value = true
  try {
    const tree = JSON.parse(raw)
    const { data: members } = await api.get('/members/with-spouses')
    const existing = new Map((members || []).map(m => [m.id, m]))

    let created = 0, updated = 0, spouseAdded = 0, deleted = 0

    // 删除被删成员（含其全部后代，detectDeleted 已展开）
    for (const m of syncDeleted.value) {
      await api.delete(`/members/${m.id}`)
      deleted++
    }

    async function walk(node, fatherId) {
      for (const child of (node.children || [])) {
        let memberId = null
        const mm = /^m(\d+)$/.exec(child.id || '')
        if (mm) memberId = parseInt(mm[1], 10)

        let curId = memberId
        if (memberId && existing.has(memberId)) {
          if (await updateMember(memberId, child, fatherId, existing.get(memberId))) updated++
        } else {
          curId = await createMember(child, fatherId)
          if (curId) {
            child.id = 'm' + curId
            created++
          }
        }
        if (curId) {
          spouseAdded += await syncSpouses(curId, child, existing.get(curId))
        }
        await walk(child, curId)
      }
    }

    await walk(tree, 0)

    // 回写 id 映射，避免下次重复创建
    localStorage.setItem(STORAGE_KEY, JSON.stringify(tree))
    dirty.value = false
    syncDeleted.value = []
    // 刷新成员缓存（含新增/不含已删），避免下次同步误判删除
    try {
      const { data: freshMembers } = await api.get('/members/with-spouses')
      cachedMembers = freshMembers || []
    } catch (e) { /* ignore */ }
    ElMessage.success(`同步完成：新增 ${created} 人，更新 ${updated} 人，删除 ${deleted} 人，新增配偶 ${spouseAdded} 人`)
  } catch (e) {
    ElMessage.error('同步失败：' + (e.response?.data?.error || e.message))
  } finally {
    syncing.value = false
  }
}

onMounted(() => {
  loadZupuFromSystem()
  window.addEventListener('storage', onStorage)
})
onBeforeUnmount(() => {
  window.removeEventListener('storage', onStorage)
})
</script>

<style scoped>
.test-zupu-page {
  margin: -20px;
  height: calc(100vh - 52px);
  display: flex;
  flex-direction: column;
  background: #f5f0e6;
}
.toolbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  background: #fff;
  border-bottom: 1px solid #e6ddd0;
}
.hint { font-size: 13px; color: #6b5d4a; }
.dirty { font-size: 12px; color: #c0392b; }
.actions { margin-left: auto; display: flex; gap: 8px; }
.error-bar { flex-shrink: 0; padding: 6px 16px; background: #fdf0ef; color: #c0392b; font-size: 12px; }
.frame-wrap { flex: 1; overflow: hidden; position: relative; }
.zupu-frame { width: 100%; height: 100%; border: none; display: block; }
.loading { display: flex; align-items: center; justify-content: center; height: 100%; color: #9a8b74; font-size: 14px; }
.sync-confirm { font-size: 14px; line-height: 1.7; }
.sync-confirm .del-warn { color: #c0392b; margin: 0; }
.sync-confirm .del-none { color: #666; margin: 0; }
.bp-body { font-size: 14px; line-height: 1.7; }
.bp-body .bp-sub { color: #666; margin: 4px 0 0; }
</style>
