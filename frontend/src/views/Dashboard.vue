<template>
  <div class="dashboard">
    <h2 class="serif page-title">数据看板</h2>

    <!-- 未来 30 天：传统节日 + 族人生日 + 忌日（仅登录可见；游客/未登录不显示） -->
    <div v-if="upcomingVisible" class="upcoming-bar">
      <div class="upcoming-head">
        <span class="upcoming-title">📅 未来 30 天</span>
        <span class="upcoming-scope">{{ upcomingScope }}</span>
        <span class="upcoming-range">{{ upcoming.start }} ~ {{ upcoming.end }}</span>
        <span v-if="upcoming.count" class="upcoming-count">{{ upcoming.count }} 项</span>
      </div>
      <div v-if="upcoming.items.length" class="upcoming-scroll">
        <div v-for="(it, idx) in upcoming.items" :key="idx"
             class="upcoming-item" :class="['u-' + it.kind, { clickable: it.kind !== 'festival', 'is-today': it.days === 0 }]"
             :title="itemTitle(it)"
             @click="onUpcomingClick(it)">
          <div class="u-line1">
            <span class="u-date">{{ it.date_md }}</span>
            <span class="u-week">{{ it.weekday }}</span>
            <span class="u-tag">{{ kindLabel(it) }}</span>
          </div>
          <div class="u-line2">
            <span class="u-name">{{ it.title }}</span>
            <span class="u-days" :class="{ 'u-today': it.days === 0 }">{{ dayText(it.days) }}</span>
          </div>
          <div v-if="it.kind !== 'festival' && it.subtitle" class="u-sub">{{ it.subtitle }}</div>
        </div>
      </div>
      <div v-else class="upcoming-empty">未来 30 天暂无传统节日、族人生日或忌日</div>
    </div>

    <!-- 概览卡片 -->
    <el-row :gutter="16" class="stat-cards">
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num">{{ stats.total }}</div><div class="stat-label">总人口</div></el-card></el-col>
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num alive">{{ stats.alive }}</div><div class="stat-label">在世</div></el-card></el-col>
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num deceased">{{ stats.deceased }}</div><div class="stat-label">已故</div></el-card></el-col>
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num">{{ stats.male }} : {{ stats.female }}</div><div class="stat-label">男女比例</div></el-card></el-col>
    </el-row>

    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num">{{ stats.spouses || 0 }}</div><div class="stat-label">配偶总数</div></el-card></el-col>
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num alive">{{ stats.spouse_alive || 0 }}</div><div class="stat-label">配偶在世</div></el-card></el-col>
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num deceased">{{ stats.spouse_deceased || 0 }}</div><div class="stat-label">配偶过世</div></el-card></el-col>
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num">{{ stats.avg_lifespan ? stats.avg_lifespan.toFixed(1) + ' 岁' : '—' }}</div><div class="stat-label">平均寿命</div></el-card></el-col>
    </el-row>

    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num">{{ stats.generations ? stats.generations.length : 0 }}</div><div class="stat-label">传世代数</div></el-card></el-col>
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num">{{ uniqueSurnames }}</div><div class="stat-label">字辈数</div></el-card></el-col>
      <el-col :xs="12" :sm="8" :md="6"><el-card shadow="hover"><div class="stat-num">{{ uniqueBranches }}</div><div class="stat-label">支系数</div></el-card></el-col>
    </el-row>

    <!-- 图表区 -->
    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :xs="24" :md="12">
        <el-card>
          <template #header><span>各世代人数分布</span></template>
          <div ref="genChart" style="height: 300px"></div>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-card>
          <template #header><span>性别比例</span></template>
          <div ref="genderChart" style="height: 300px"></div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 职业占比 + 年龄结构 -->
    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :xs="24" :md="12">
        <el-card>
          <template #header>
            <span>职业占比</span>
            <el-select v-model="occScope" size="small" style="width: 88px; margin-left: 8px" @change="onOccScopeChange">
              <el-option label="在世" value="living" />
              <el-option label="全部" value="all" />
            </el-select>
            <span class="card-sub">（{{ occScope === 'living' ? '在世' : '全部' }} {{ occTotalShown }} 人<template v-if="occUnknownShown">，其中未填职业 {{ occUnknownShown }} 人</template>）</span>
          </template>
          <div v-show="hasOccupations" ref="occChart" style="height: 320px"></div>
          <el-empty v-if="!hasOccupations" description="暂无职业数据（在成员/配偶档案里填写「职业」后显示）" :image-size="80" />
        </el-card>
      </el-col>
      <el-col :xs="24" :md="12">
        <el-card>
          <template #header>
            <span>年龄结构</span>
            <span class="card-sub">（在世且已填出生日期 {{ stats.age_known || 0 }} 人<template v-if="stats.age_unknown">，另有 {{ stats.age_unknown }} 人未填出生日期</template>）</span>
          </template>
          <div v-show="stats.age_known" ref="ageChart" style="height: 320px"></div>
          <el-empty v-if="!stats.age_known" description="暂无出生日期数据" :image-size="80" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 字辈分布 -->
    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :span="24">
        <el-card>
          <template #header><span>字辈分布</span></template>
          <div ref="wordChart" style="height: 320px"></div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import api from '../api'
import { branchCtx, branchScopeLabel, loadBranches } from '../utils/branchContext'

const router = useRouter()

// ---- 未来 30 天（节日/生日/忌日）：仅登录账号可见，游客不请求、不显示 ----
// 数据范围跟随顶部「房支」选择器：当前房支（含下级就是本房支+全部下级）；宗谱 = 全族
const upcomingVisible = ref(false)
const upcoming = reactive({ start: '', end: '', days: 30, count: 0, items: [] })
const upcomingScope = computed(() => branchScopeLabel())

function dayText(d) { return Number(d) === 0 ? '今天' : `还有 ${d} 天` }
function kindLabel(it) {
  if (it.kind === 'birthday') return '生日'
  if (it.kind === 'deathday') return '忌日'
  return it.temp_fixed ? '节日·临时' : '节日'
}
function itemTitle(it) {
  const lunar = it.lunar ? `（农历 ${it.lunar}）` : ''
  return `${it.date} ${it.weekday} · ${it.title}${lunar}${it.subtitle ? ' · ' + it.subtitle : ''}`
}
function onUpcomingClick(it) {
  if (it.kind === 'festival') return            // 节日无需跳转
  if (it.spouse_id) router.push('/spouse/' + it.spouse_id)
  else if (it.member_id) router.push('/member/' + it.member_id)
}
async function loadUpcoming() {
  // 未登录（无 token）不请求，游客自然看不到；后端同样会 401
  if (!localStorage.getItem('token')) {
    upcomingVisible.value = false
    return
  }
  try {
    // 房支参数由 api 拦截器按当前房支自动附加（BRANCH_SCOPED 含 /dashboard/upcoming）
    const { data } = await api.get('/dashboard/upcoming')
    upcoming.start = data.start || ''
    upcoming.end = data.end || ''
    upcoming.days = data.days || 30
    upcoming.count = data.count || 0
    upcoming.items = Array.isArray(data.items) ? data.items : []
    upcomingVisible.value = true
  } catch (e) {
    upcomingVisible.value = false
  }
}

// 切房支 / 勾选「含下级」时该条跟着刷新（顶部选择器变化 → 重新请求）
watch([() => branchCtx.current, () => branchCtx.includeChildren], () => {
  loadUpcoming()
})

const stats = reactive({ total: 0, alive: 0, deceased: 0, male: 0, female: 0, generations: [], avg_lifespan: 0 })
const members = ref([])
const genChart = ref(null)
const genderChart = ref(null)
const wordChart = ref(null)
const occChart = ref(null)
const ageChart = ref(null)
const occScope = ref(localStorage.getItem('occScope') || 'living')   // 职业占比范围：living=在世（默认）/ all=全部
const occListShown = computed(() => occScope.value === 'living' ? (stats.occupations || []) : (stats.occupations_all || []))
const occTotalShown = computed(() => occScope.value === 'living' ? (stats.occupation_total || 0) : (stats.occupation_total_all || 0))
const occUnknownShown = computed(() => occScope.value === 'living' ? (stats.occupation_unknown || 0) : (stats.occupation_unknown_all || 0))
const hasOccupations = computed(() => occListShown.value.length > 0)
function onOccScopeChange(v) { localStorage.setItem('occScope', v); renderCharts() }

const uniqueSurnames = computed(() => {
  if (stats.unique_surnames != null) return stats.unique_surnames
  return new Set(members.value.map(m => m.generation_word).filter(Boolean)).size
})
const uniqueBranches = computed(() => {
  if (stats.unique_branches != null) return stats.unique_branches
  return new Set(members.value.map(m => m.branch).filter(Boolean)).size
})

async function load() {
  try {
    const statsRes = await api.get('/members/stats')
    Object.assign(stats, statsRes.data)
  } catch (e) {
    // 统计接口失败，保持默认值
  }

  // 成员列表（需登录），失败不影响统计数据展示
  try {
    const membersRes = await api.get('/members')
    members.value = membersRes.data
  } catch (e) {
    members.value = []
  }

  await nextTick()
  renderCharts()
}

function renderCharts() {
  // 各世代人数
  if (genChart.value) {
    const chart = echarts.init(genChart.value)
    const gens = stats.generations || []
    chart.setOption({
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', data: gens.map(g => `第${g.generation}世`) },
      yAxis: { type: 'value' },
      series: [{
        data: gens.map(g => g.count),
        type: 'bar',
        itemStyle: { color: '#8c5a3c' },
        barMaxWidth: 40
      }]
    })
  }

  // 性别比例
  if (genderChart.value) {
    const chart = echarts.init(genderChart.value)
    chart.setOption({
      tooltip: { trigger: 'item' },
      legend: { bottom: 0 },
      series: [{
        type: 'pie',
        radius: ['45%', '70%'],
        center: ['50%', '45%'],
        itemStyle: { borderRadius: 4 },
        label: { show: true, formatter: '{b}: {c}人 ({d}%)' },
        data: [
          { value: stats.male || 0, name: '男性', itemStyle: { color: '#3a3226' } },
          { value: stats.female || 0, name: '女性', itemStyle: { color: '#c49a6c' } }
        ]
      }]
    })
  }

  // 职业占比（在世 / 全部 由下拉切换）
  if (occChart.value && occListShown.value.length) {
    const chart = echarts.init(occChart.value)
    const palette = ['#8c5a3c', '#c49a6c', '#3a3226', '#a8845c', '#6b5b45', '#d8b98c',
                     '#7a6a52', '#b08968', '#5c4a32', '#e0c9a6', '#8a7355', '#9c7b56', '#cfae83']
    const data = occListShown.value.map((o, i) => ({
      name: o.name, value: o.count,
      // 「未知」用灰色，与年龄结构的「未填出生」观感一致
      itemStyle: { color: o.name === '未知' ? '#c9c2b6' : palette[i % palette.length] }
    }))
    chart.setOption({
      tooltip: { trigger: 'item', formatter: '{b}: {c}人 ({d}%)' },
      legend: { type: 'scroll', bottom: 0, textStyle: { fontSize: 11 } },
      series: [{
        type: 'pie',
        radius: ['40%', '65%'],
        center: ['50%', '42%'],
        itemStyle: { borderRadius: 4 },
        label: { show: true, formatter: '{b} {d}%', fontSize: 11 },
        data
      }]
    })
  }

  // 年龄结构
  if (ageChart.value && stats.age_known) {
    const chart = echarts.init(ageChart.value)
    const groups = (stats.age_groups || []).slice()
    if (stats.age_unknown) groups.push({ label: '未填出生', count: stats.age_unknown })
    chart.setOption({
      tooltip: { trigger: 'axis', formatter: p => `${p[0].name}: ${p[0].value} 人` },
      grid: { left: 40, right: 16, bottom: 30, top: 20 },
      xAxis: { type: 'category', data: groups.map(g => g.label), axisLabel: { fontSize: 11 } },
      yAxis: { type: 'value' },
      series: [{
        data: groups.map(g => ({ value: g.count, itemStyle: { color: g.label === '未填出生' ? '#c9c2b6' : '#8c5a3c' } })),
        type: 'bar',
        barMaxWidth: 44,
        label: { show: true, position: 'top', fontSize: 11 }
      }]
    })
  }

  // 字辈分布
  if (wordChart.value) {
    const chart = echarts.init(wordChart.value)
    const words = {}
    members.value.forEach(m => {
      if (m.generation_word) {
        words[m.generation_word] = (words[m.generation_word] || 0) + 1
      }
    })
    const entries = Object.entries(words).sort((a, b) => b[1] - a[1])
    chart.setOption({
      tooltip: { trigger: 'axis' },
      xAxis: { type: 'category', data: entries.map(e => e[0]), axisLabel: { rotate: 45 } },
      yAxis: { type: 'value' },
      series: [{
        data: entries.map(e => e[1]),
        type: 'bar',
        itemStyle: { color: '#8c5a3c' },
        barMaxWidth: 40
      }]
    })
  }
}

onMounted(async () => {
  // 先确定房支范围（登录后默认按账号所属房支），再发数据请求，避免先按「全族」取一次
  if (localStorage.getItem('token') && !branchCtx.loaded) await loadBranches()
  load()
  loadUpcoming()
})
</script>

<style scoped>
.dashboard { height: 100%; overflow: auto; }
.page-title { margin-bottom: 16px; }

/* ---- 未来 30 天 ---- */
.upcoming-bar {
  background: linear-gradient(135deg, #fbf6ec 0%, #f6efe1 100%);
  border: 1px solid #e6d9bf;
  border-radius: 10px;
  padding: 10px 12px 12px;
  margin-bottom: 16px;
  max-width: 100%;
  overflow: hidden;
}
.upcoming-head { display: flex; align-items: baseline; gap: 10px; margin-bottom: 8px; flex-wrap: wrap; }
.upcoming-title { font-weight: 700; color: #3a3226; font-size: 15px; letter-spacing: 1px; }
.upcoming-scope {
  color: #6b5a44; font-size: 12px; padding: 1px 8px; border-radius: 9px;
  background: #efe4cf; border: 1px solid #e0d0b2;
}
.upcoming-range { color: #9a8b78; font-size: 12px; }
.upcoming-count { color: #8c5a3c; font-size: 12px; }
.upcoming-scroll {
  display: flex;
  flex-wrap: nowrap;
  gap: 8px;
  overflow-x: auto;
  overflow-y: hidden;
  -webkit-overflow-scrolling: touch;
  padding-bottom: 4px;
  max-width: 100%;
}
.upcoming-scroll::-webkit-scrollbar { height: 6px; }
.upcoming-scroll::-webkit-scrollbar-thumb { background: #d8c7a8; border-radius: 3px; }
.upcoming-item {
  flex: 0 0 auto;
  width: 168px;
  background: #fff;
  border: 1px solid #ece0cb;
  border-left: 3px solid #c49a6c;
  border-radius: 8px;
  padding: 7px 9px;
  box-sizing: border-box;
  transition: box-shadow .15s, transform .15s;
}
.upcoming-item.clickable { cursor: pointer; }
.upcoming-item.clickable:hover { box-shadow: 0 3px 10px rgba(140, 90, 60, .16); transform: translateY(-1px); }
.upcoming-item.u-festival { border-left-color: #b8860b; background: #fffdf5; }
.upcoming-item.u-birthday { border-left-color: #67c23a; }
.upcoming-item.u-deathday { border-left-color: #8c5a3c; background: #faf8f4; }
.upcoming-item.is-today { box-shadow: 0 0 0 2px rgba(196, 154, 108, .35); }
.u-line1 { display: flex; align-items: center; gap: 5px; font-size: 12px; color: #7a6a52; }
.u-date { font-weight: 700; color: #3a3226; font-variant-numeric: tabular-nums; }
.u-week { color: #9a8b78; }
.u-tag {
  margin-left: auto;
  font-size: 11px;
  color: #8c5a3c;
  background: #f3e8d6;
  border-radius: 3px;
  padding: 0 4px;
  line-height: 16px;
  white-space: nowrap;
}
.u-birthday .u-tag { color: #4e8f2f; background: #eaf6e2; }
.u-deathday .u-tag { color: #6b4a33; background: #efe7dc; }
.u-festival .u-tag { color: #8a6d09; background: #fbf0cd; }
.u-line2 { display: flex; align-items: baseline; gap: 6px; margin-top: 3px; }
.u-name {
  color: #3a3226;
  font-size: 14px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
  flex: 1 1 auto;
}
.u-days { flex: 0 0 auto; font-size: 12px; color: #8c5a3c; white-space: nowrap; }
.u-days.u-today { color: #fff; background: #c0392b; border-radius: 9px; padding: 0 6px; font-weight: 700; }
.u-sub { color: #9a8b78; font-size: 11px; margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.upcoming-empty { color: #9a8b78; font-size: 13px; padding: 6px 2px; }

.stat-num { font-size: 28px; font-weight: bold; text-align: center; color: #3a3226; }
.stat-num.alive { color: #67c23a; }
.stat-num.deceased { color: #8c5a3c; }
.stat-label { text-align: center; color: #7a6a52; font-size: 13px; margin-top: 4px; }
.card-sub { color: #9a8b78; font-size: 12px; margin-left: 6px; }
</style>