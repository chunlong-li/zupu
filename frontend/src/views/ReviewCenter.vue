<template>
  <div class="review">
    <el-tabs v-model="tab">
      <el-tab-pane label="待审核" name="pending">
        <el-table :data="pending" v-loading="loading" empty-text="暂无待审核的修改">
          <el-table-column label="对象" min-width="140">
            <template #default="{ row }">
              <el-tag size="small" :type="entityTagType(row.entity_type)">
                {{ entityText(row.entity_type) }}
              </el-tag>
              <span class="target-name">{{ draftTarget(row) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="actionType(row.action)">{{ actionText(row.action) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="内容" min-width="200">
            <template #default="{ row }">{{ draftSummary(row) }}</template>
          </el-table-column>
          <el-table-column label="提交人" prop="submitted_by_name" width="100" />
          <el-table-column label="提交时间" prop="created_at" width="170" />
          <el-table-column label="操作" width="230" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="info" @click="viewChanges(row)">查看修改</el-button>
              <el-button size="small" type="success" @click="review(row, 'approve')">通过</el-button>
              <el-button size="small" type="danger" @click="review(row, 'reject')">驳回</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="已处理" name="done">
        <el-table :data="done" v-loading="loading" empty-text="暂无已处理的修改">
          <el-table-column label="对象" min-width="140">
            <template #default="{ row }">
              <el-tag size="small" :type="entityTagType(row.entity_type)">
                {{ entityText(row.entity_type) }}
              </el-tag>
              <span class="target-name">{{ draftTarget(row) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80">
            <template #default="{ row }">
              <el-tag size="small" :type="actionType(row.action)">{{ actionText(row.action) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="内容" min-width="180">
            <template #default="{ row }">{{ draftSummary(row) }}</template>
          </el-table-column>
          <el-table-column label="提交人" prop="submitted_by_name" width="100" />
          <el-table-column label="结果" width="90">
            <template #default="{ row }">
              <el-tag size="small" :type="row.status === 'approved' ? 'success' : 'danger'">
                {{ row.status === 'approved' ? '已通过' : '已驳回' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="审核人" prop="reviewer_name" width="100" />
          <el-table-column label="说明" prop="reason" min-width="140" show-overflow-tooltip />
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <el-button size="small" text type="info" @click="viewChanges(row)">查看修改</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <!-- 修改详情弹窗 -->
    <el-dialog v-model="changesVisible" :title="`修改详情 · ${changesTarget}`" width="600px">
      <el-alert v-if="changesDeleted" type="danger" :closable="false" :title="changesDeletedDesc || '该修改为删除操作，相关记录将被删除'" />
      <el-table v-else :data="changes" size="small" max-height="440" empty-text="无字段级差异（可能是新增或改名）">
        <el-table-column prop="field" label="字段" width="130" />
        <el-table-column label="原值" min-width="180">
          <template #default="{ row }"><span class="diff-old">{{ row.from }}</span></template>
        </el-table-column>
        <el-table-column label="新值" min-width="180">
          <template #default="{ row }"><span class="diff-new">{{ row.to }}</span></template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="changesVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { formatLifeDates } from '../utils/dateFormat'

const tab = ref('pending')
const pending = ref([])
const done = ref([])
const loading = ref(false)

const changesVisible = ref(false)
const changes = ref([])
const changesTarget = ref('')
const changesDeleted = ref(false)
const changesDeletedDesc = ref('')

function actionText(a) {
  return { create: '新增', update: '修改', delete: '删除', rename: '改名' }[a] || a
}
function actionType(a) {
  return { create: 'success', update: 'primary', delete: 'danger', rename: 'warning' }[a] || 'info'
}
function entityText(t) {
  return { member: '成员', spouse: '配偶', event: '大事记', article: '文章', cemetery: '祠墓', generation_word: '字辈派语', member_bio: '成员生平', spouse_bio: '配偶生平' }[t] || t
}
function entityTagType(t) {
  return { member: 'primary', spouse: 'warning', event: 'success', article: 'info', cemetery: 'info', generation_word: 'info', member_bio: 'primary', spouse_bio: 'warning' }[t] || 'info'
}

function parsePayload(row) {
  try { return JSON.parse(row.payload || '{}') } catch (e) { return {} }
}

function draftTarget(row) {
  const p = parsePayload(row)
  switch (row.entity_type) {
    case 'member':
    case 'spouse':
      if (row.action === 'create' || row.action === 'update') {
        return p.name || p.given_name || p.surname || `#${row.entity_id}`
      }
      return `#${row.entity_id}`
    case 'event':
      return p.title || `#${row.entity_id}`
    case 'article':
      return p.title || `#${row.entity_id}`
    case 'cemetery':
      return p.name || `#${row.entity_id}`
    case 'generation_word':
      return p.branch || '总谱'
    case 'member_bio':
    case 'spouse_bio':
      if (row.action === 'create' || row.action === 'update') return p.title || `#${row.entity_id}`
      return `#${row.entity_id}`
  }
  return `#${row.entity_id}`
}

function draftSummary(row) {
  const p = parsePayload(row)
  if (row.action === 'rename') return `改为「${p.given_name || ''}」`
  if (row.action === 'delete') {
    // 删除类无字段级差异，展示对象提示
    return '删除该记录'
  }
  if (row.action === 'create') {
    // 新增：展示核心字段
    if (row.entity_type === 'member' || row.entity_type === 'spouse') {
      const parts = []
      if (p.generation) parts.push(`${p.generation}世`)
      if (p.given_name) parts.push(p.given_name)
      if (p.birth_gregorian) parts.push(formatLifeDates(p))
      return parts.join(' · ') || '（详见修改）'
    }
    if (p.title) return `「${p.title}」`
    if (p.name) return `「${p.name}」`
    if (row.entity_type === 'generation_word') return `派语「${p.branch || '总谱'}」`
    return '（详见修改）'
  }
  // update：展示变更字段概览
  const ch = row.changes || []
  if (ch.length) {
    const names = ch.map(c => c.field)
    return names.slice(0, 4).join('、') + (names.length > 4 ? ` 等${names.length}项` : '')
  }
  return '（详见修改）'
}

// 打开「查看修改」弹窗
function viewChanges(row) {
  changesTarget.value = `${entityText(row.entity_type)} · ${draftTarget(row)}（${actionText(row.action)}）`
  changes.value = []
  changesDeleted.value = false
  if (row.action === 'delete') {
    changesDeleted.value = true
    // 删除对象描述（后端固化于 changes 或 summary）
    const desc = (row.changes && row.changes[0] && row.changes[0].from) || row.summary || draftTarget(row)
    changesDeletedDesc.value = `删除：${desc}`
  } else {
    changes.value = row.changes || []
  }
  changesVisible.value = true
}

async function load() {
  loading.value = true
  try {
    const [a, b] = await Promise.all([
      api.get('/drafts', { params: { status: 'pending' } }),
      api.get('/drafts', { params: { status: 'done' } })
    ])
    pending.value = a.data || []
    done.value = b.data || []
  } catch (e) {
    ElMessage.error('加载审核队列失败：' + (e?.response?.data?.error || e.message))
  } finally {
    loading.value = false
  }
}

async function review(row, action) {
  const verb = action === 'approve' ? '通过' : '驳回'
  let reason = ''
  if (action === 'reject') {
    try {
      const { value } = await ElMessageBox.prompt('请填写驳回理由', '驳回', {
        inputPlaceholder: '例如：缺少史料依据，请补充后重新提交',
        confirmButtonText: '确认驳回',
        cancelButtonText: '取消'
      })
      reason = value || ''
    } catch (e) {
      return
    }
  } else {
    try {
      await ElMessageBox.confirm('确认通过这条修改？通过后立即写入正式谱系。', '审核通过', {
        type: 'warning', confirmButtonText: '确认通过', cancelButtonText: '取消'
      })
    } catch (e) {
      return
    }
  }
  try {
    await api.post(`/drafts/${row.id}/review`, { action, reason })
    ElMessage.success(action === 'approve' ? '已通过并写入正式库' : '已驳回')
    load()
  } catch (e) {
    ElMessage.error('审核失败：' + (e?.response?.data?.error || e.message))
  }
}

onMounted(load)
</script>

<style scoped>
.review { padding: 4px; }
.target-name { margin-left: 8px; font-weight: 500; }
.diff-old { color: #c45656; }
.diff-new { color: #67c23a; }
</style>
