<template>
  <div class="contact-view">
    <h2 class="serif page-title">📇 通讯录</h2>
    <p class="subtitle">仅显示已填写手机号的族人，敏感信息按隐私规则显示（手机号等依角色自动打码）</p>

    <div class="toolbar">
      <el-input v-model="kw" placeholder="搜索姓名 / 房支" clearable style="width: 240px" />
      <el-select v-model="branchFilter" placeholder="全部房支" clearable filterable style="width: 180px">
        <el-option v-for="b in branchOptions" :key="b" :label="b" :value="b" />
      </el-select>
      <span class="count">共 {{ filtered.length }} 人</span>
    </div>

    <el-table :data="pageRows" stripe style="width: 100%" v-loading="loading" empty-text="暂无数据">
      <el-table-column type="index" label="#" width="60" :index="indexMethod" />
      <el-table-column prop="name" label="姓名" min-width="110">
        <template #default="{ row }">
          <router-link :to="'/member/' + row.id" class="member-link">{{ row.name || (row.surname || '') + (row.given_name || '') }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="性别" width="70">
        <template #default="{ row }">{{ row.gender === 1 ? '男' : (row.gender === 2 ? '女' : '—') }}</template>
      </el-table-column>
      <el-table-column label="世系" width="90">
        <template #default="{ row }">{{ row.record_type === '外纪' ? '外纪' : '第' + row.generation + '世' }}</template>
      </el-table-column>
      <el-table-column label="房支" width="140">
        <template #default="{ row }">{{ row.branch || '—' }}</template>
      </el-table-column>
      <el-table-column label="手机" min-width="150">
        <template #default="{ row }">{{ row.phone || '—' }}</template>
      </el-table-column>
      <el-table-column label="籍贯" min-width="150">
        <template #default="{ row }">{{ row.native_place || '—' }}</template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">{{ row.death_gregorian ? '已故' : '在世' }}</template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        background
        layout="total, sizes, prev, pager, next, jumper"
        :total="filtered.length"
        :page-size="pageSize"
        :current-page="page"
        :page-sizes="[10, 20, 50, 100]"
        @current-change="p => (page = p)"
        @size-change="s => { pageSize = s; page = 1 }"
      />
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import api from '../api'

const members = ref([])
const loading = ref(true)
const kw = ref('')
const branchFilter = ref('')
const page = ref(1)
const pageSize = ref(20)

const branchOptions = computed(() =>
  Array.from(new Set(members.value.map(m => m.branch).filter(Boolean))).sort()
)

const filtered = computed(() => {
  const k = kw.value.trim().toLowerCase()
  return members.value.filter(m => {
    if (!m.phone) return false
    if (branchFilter.value && m.branch !== branchFilter.value) return false
    if (!k) return true
    const name = (m.name || ((m.surname || '') + (m.given_name || ''))).toLowerCase()
    return name.includes(k) || (m.branch || '').toLowerCase().includes(k)
  })
})

const pageRows = computed(() =>
  filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value)
)

function indexMethod(i) {
  return (page.value - 1) * pageSize.value + i + 1
}

watch([kw, branchFilter], () => { page.value = 1 })

onMounted(async () => {
  try {
    const { data } = await api.get('/members')
    members.value = data || []
  } catch (e) { /* ignore */ }
  loading.value = false
})
</script>

<style scoped>
.contact-view { height: 100%; overflow: auto; }
.page-title { margin-bottom: 4px; }
.subtitle { color: #7a6a52; font-size: 13px; margin: 0 0 16px; }
.toolbar { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; flex-wrap: wrap; }
.toolbar .count { color: #7a6a52; font-size: 13px; }
.member-link { color: #8c5a3c; text-decoration: none; font-weight: 500; }
.member-link:hover { color: #3a3226; text-decoration: underline; }
.pager { display: flex; justify-content: flex-end; margin-top: 16px; }
</style>
