<template>
  <div class="spouse-list">
    <h2 class="serif page-title">配偶资料</h2>

    <div class="table-responsive">
    <el-table :data="spouses" border stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column label="姓名" width="120">
        <template #default="s">
          <router-link :to="`/spouse/${s.row.id}`" class="member-link">{{ s.row.name || (s.row.surname || '') + (s.row.given_name || '') }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="80">
        <template #default="s">
          <el-tag size="small" :type="s.row.type === '正配' ? 'danger' : s.row.type === '继配' ? 'warning' : 'info'">{{ s.row.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="所属成员" width="140">
        <template #default="s">
          <router-link :to="`/member/${s.row.member_id}`" class="member-link">{{ getMemberName(s.row.member_id) }}</router-link>
        </template>
      </el-table-column>
      <el-table-column prop="family" label="家世" width="120" />
      <el-table-column label="生卒" min-width="180">
        <template #default="s">
          <span>{{ formatLifeDates(s.row) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="occupation" label="职业" width="110" show-overflow-tooltip>
        <template #default="s">{{ s.row.occupation || '—' }}</template>
      </el-table-column>
      <el-table-column prop="education" label="学历" width="110" show-overflow-tooltip>
        <template #default="s">{{ s.row.education || '—' }}</template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" width="150" show-overflow-tooltip />
    </el-table>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'
import { formatLifeDates } from '../utils/dateFormat'

const loading = ref(false)
const spouses = ref([])
const allMembers = ref([])

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/members/with-spouses')
    allMembers.value = data || []
    const allSpouses = []
    for (const m of data) {
      if (m.spouses && m.spouses.length) {
        allSpouses.push(...m.spouses)
      }
    }
    spouses.value = allSpouses
  } finally {
    loading.value = false
  }
}

function getMemberName(id) {
  const m = allMembers.value.find(x => x.id === id)
  return m ? (m.name || (m.surname || '') + (m.given_name || '')) : `ID:${id}`
}

onMounted(load)
</script>

<style scoped>
.page-title { font-size: 20px; margin-bottom: 16px; }
.era { color: #8c5a3c; font-size: 12px; margin-left: 4px; }
.member-link { color: #3a3226; text-decoration: none; }
.member-link:hover { color: #8c5a3c; text-decoration: underline; }
.table-responsive { overflow-x: auto; -webkit-overflow-scrolling: touch; }
</style>