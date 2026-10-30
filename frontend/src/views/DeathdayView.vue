<template>
  <div class="calendar-view">
    <h2 class="serif page-title">🕯️ 先人祭日</h2>
    <p class="subtitle">按农历月日排列，从正月初一开始</p>

    <el-table :data="list" stripe style="width: 100%" v-loading="loading" empty-text="暂无祭日数据">
      <el-table-column prop="lunar_month" label="农历月" width="80" sortable>
        <template #default="{ row }">{{ (row.is_leap_month ? '闰' : '') + lunarMonthName(row.lunar_month) }}</template>
      </el-table-column>
      <el-table-column prop="lunar_day" label="农历日" width="80">
        <template #default="{ row }">{{ lunarDayName(row.lunar_day) }}</template>
      </el-table-column>
      <el-table-column prop="name" label="姓名" min-width="100">
        <template #default="{ row }">
          <router-link v-if="row.kind === 'spouse'" :to="'/spouse/' + row.id" class="member-link">{{ row.name }}</router-link>
          <router-link v-else :to="'/member/' + row.id" class="member-link">{{ row.name }}</router-link>
        </template>
      </el-table-column>
      <el-table-column label="性别" width="60">
        <template #default="{ row }">{{ row.kind === 'spouse' ? '配偶' : (row.gender === 1 ? '男' : '女') }}</template>
      </el-table-column>
      <el-table-column prop="generation" label="世系" width="70">
        <template #default="{ row }">{{ row.kind === 'spouse' ? '—' : '第' + row.generation + '世' }}</template>
      </el-table-column>
      <el-table-column prop="lifespan" label="享年" width="70" sortable />
      <el-table-column label="今年祭日" width="120">
        <template #default="{ row }">{{ row.this_year_solar || '—' }}</template>
      </el-table-column>
      <el-table-column label="农历祭日" min-width="140">
        <template #default="{ row }">{{ row.death_lunar }}</template>
      </el-table-column>
      <el-table-column prop="death_gregorian" label="公历忌日" width="120" />
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import api from '../api'

const list = ref([])
const loading = ref(true)

const lunarMonthNames = ['', '正月', '二月', '三月', '四月', '五月', '六月', '七月', '八月', '九月', '十月', '十一月', '腊月']
const lunarDayNames = ['', '初一', '初二', '初三', '初四', '初五', '初六', '初七', '初八', '初九', '初十',
  '十一', '十二', '十三', '十四', '十五', '十六', '十七', '十八', '十九',
  '二十', '廿一', '廿二', '廿三', '廿四', '廿五', '廿六', '廿七', '廿八', '廿九', '三十']

function lunarMonthName(m) { return lunarMonthNames[m] || m }
function lunarDayName(d) { return lunarDayNames[d] || d }

onMounted(async () => {
  try {
    const { data } = await api.get('/members/deathdays')
    list.value = data
  } catch (e) { /* ignore */ }
  loading.value = false
})
</script>

<style scoped>
.calendar-view { height: 100%; overflow: auto; }
.page-title { margin-bottom: 4px; }
.subtitle { color: #7a6a52; font-size: 13px; margin: 0 0 16px; }
.member-link { color: #8c5a3c; text-decoration: none; font-weight: 500; }
.member-link:hover { color: #3a3226; text-decoration: underline; }
</style>