<template>
  <div class="tradition-view">
    <h2 class="serif page-title">🎏 家族传统日程</h2>
    <p class="subtitle">按农历月日排列的家族传统活动，如祭祖、团拜等</p>
    <p class="subtitle hint-line">
      「不固定」的节日（如清明、端午、中秋、除夕）可填入<b>本年临时日期</b>：填了即显示该具体日期；
      日期一过自动回到「不固定」，不会沿用到下一年。
    </p>

    <!-- 理事及以上可新增 -->
    <div class="toolbar" v-if="isElder">
      <el-button v-if="isElder" @click="seedDefaults">补充通用节日</el-button>
      <el-button type="primary" size="small" @click="openAdd">+ 新增日程</el-button>
    </div>

    <el-table :data="list" stripe style="width: 100%" v-loading="loading" empty-text="暂无传统日程">
      <el-table-column label="农历月" width="90" sortable prop="lunar_month">
        <template #default="{ row }">{{ (row.is_leap_month ? '闰' : '') + lunarMonthName(row.lunar_month) }}</template>
      </el-table-column>
      <el-table-column label="农历日" width="90" prop="lunar_day">
        <template #default="{ row }">{{ lunarDayName(row.lunar_day) }}</template>
      </el-table-column>
      <el-table-column prop="title" label="活动名称" min-width="120" />
      <el-table-column prop="description" label="说明" min-width="180" show-overflow-tooltip />
      <el-table-column label="今年公历 / 临时日期（本年）" width="300">
        <template #default="{ row }">
          <div v-if="row.lunar_day === 0" class="temp-cell">
            <!-- 服务端能自动算出来的（如除夕：腊月最后一天）直接显示，不用手填 -->
            <div v-if="row.this_year_solar && !row.temp_solar_date" class="temp-ok">
              {{ row.this_year_solar }}
              <span class="temp-days">自动计算</span>
            </div>
            <div v-else-if="row.temp_solar_date" class="temp-ok">
              {{ row.temp_solar_date }}
              <span class="temp-days">{{ dayText(row.temp_days) }}</span>
            </div>
            <div v-else-if="row.temp_expired_date" class="temp-expired"
                 :title="row.temp_expired_date + ' 的临时日期已过期，已回到「不固定」'">
              {{ row.temp_expired_date }} 已过期，回到「不固定」
            </div>
            <div v-else class="temp-none">不固定</div>
            <el-button v-if="isElder && !row.this_year_solar" size="small" link type="primary" class="temp-btn" @click="openTemp(row)">
              {{ row.temp_solar_date ? '修改临时日期' : '填临时日期' }}
            </el-button>
          </div>
          <span v-else class="temp-none">{{ row.this_year_solar || '—' }}</span>
        </template>
      </el-table-column>
      <el-table-column v-if="isElder" label="操作" width="140" fixed="right">
        <template #default="{ row }">
          <el-button size="small" text type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" text type="danger" @click="del(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dlgVisible" :title="isEdit ? '编辑传统日程' : '新增传统日程'" width="420px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="活动名称" required>
          <el-input v-model="form.title" placeholder="如：清明祭祖" maxlength="32" />
        </el-form-item>
        <el-form-item label="农历月">
          <el-select v-model="form.lunar_month" placeholder="选择月份" style="width:100%">
            <el-option v-for="(n,i) in 12" :key="i" :label="lunarMonthName(i+1)" :value="i+1" />
          </el-select>
        </el-form-item>
        <el-form-item label="农历日">
          <el-select v-model="form.lunar_day" placeholder="选择日期" style="width:100%">
            <el-option label="不固定" :value="0" />
            <el-option v-for="d in 30" :key="d" :label="lunarDayName(d)" :value="d" />
          </el-select>
          <div class="form-hint">「不固定」用于没有固定农历日的日程，例如清明（按节气）、除夕（按年末）</div>
        </el-form-item>
        <el-form-item label="闰月">
          <el-switch v-model="form.is_leap_month" active-text="是" inactive-text="否" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="活动说明（可选）" maxlength="256" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">确定</el-button>
      </template>
    </el-dialog>

    <!-- 临时日期弹窗：仅「不固定」节日可填，按年份保存，过期自动失效 -->
    <el-dialog v-model="tempVisible" title="设置临时日期" width="440px">
      <div class="temp-dialog">
        <p class="temp-name">🎏 {{ tempForm.title }}</p>
        <p class="temp-hint">
          该节日为「不固定」（没有固定农历日）。请选择 <b>{{ tempForm.year }} 年（农历）</b>的月份和日期：
          选好会自动换算成公历显示；<b>日期一过即自动回到「不固定」</b>，也不会沿用到下一年。
        </p>
        <div class="lunar-pick">
          <el-select v-model="tempForm.lunar_month" placeholder="农历月" style="width:44%">
            <el-option v-for="m in 12" :key="m" :label="lunarMonthName(m)" :value="m" />
          </el-select>
          <el-select v-model="tempForm.lunar_day" placeholder="农历日" style="width:44%">
            <el-option v-for="d in 30" :key="d" :label="lunarDayName(d)" :value="d" />
          </el-select>
        </div>
        <el-checkbox v-model="tempForm.is_leap" style="margin-top:8px">闰月（该年有闰月时才需要勾）</el-checkbox>
        <p v-if="tempForm.current" class="temp-cur">当前：{{ tempForm.current }}</p>
      </div>
      <template #footer>
        <el-button v-if="tempForm.current" type="danger" text :loading="tempSaving" @click="clearTemp">清除临时日期</el-button>
        <el-button @click="tempVisible = false">取消</el-button>
        <el-button type="primary" :loading="tempSaving" @click="saveTemp">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'

const list = ref([])
const loading = ref(true)
const isElder = ref(false)
const dlgVisible = ref(false)
const isEdit = ref(false)
const saving = ref(false)
const form = ref({ id: 0, title: '', lunar_month: 1, lunar_day: 1, is_leap_month: false, description: '' })

// 「不固定」节日的本年临时日期（按 节日 + 年份 存，过期自动回到不固定）
const nowYear = new Date().getFullYear()
const tempVisible = ref(false)
const tempSaving = ref(false)
const tempForm = ref({ id: 0, title: '', year: nowYear, date: '', current: '', lunar_month: 1, lunar_day: 1, is_leap: false })

function dayText(d) { return Number(d) === 0 ? '今天' : `还有 ${d} 天` }

function openTemp(row) {
  const cur = row.temp_solar_date
    ? `${row.temp_solar_date}（${dayText(row.temp_days)}）`
    : (row.temp_expired_date ? `${row.temp_expired_date}（已过期，已回到不固定）` : '')
  tempForm.value = {
    id: row.id,
    title: row.title,
    year: nowYear,
    date: row.temp_solar_date || '',
    current: cur,
    lunar_month: row.lunar_month || 1,
    lunar_day: 1,
    is_leap: false
  }
  tempVisible.value = true
}

async function saveTemp() {
  tempSaving.value = true
  try {
    await api.put('/traditions/' + tempForm.value.id + '/temp-date', {
      year: tempForm.value.year,
      lunar_month: tempForm.value.lunar_month,
      lunar_day: tempForm.value.lunar_day,
      is_leap: tempForm.value.is_leap
    })
    ElMessage.success('已按农历设置临时日期')
    tempVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '保存失败')
  } finally {
    tempSaving.value = false
  }
}

async function putTemp(date) {
  tempSaving.value = true
  try {
    await api.put('/traditions/' + tempForm.value.id + '/temp-date', { year: tempForm.value.year, date })
    ElMessage.success(date ? '已设置临时日期' : '已清除临时日期，恢复「不固定」')
    tempVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '操作失败')
  }
  tempSaving.value = false
}

async function clearTemp() {
  try {
    await ElMessageBox.confirm('清除后该节日将回到「不固定」，确定？', '确认', { type: 'warning' })
  } catch (e) { return }
  putTemp('')
}

const lunarMonthNames = ['', '正月', '二月', '三月', '四月', '五月', '六月', '七月', '八月', '九月', '十月', '十一月', '腊月']
const lunarDayNames = ['', '初一', '初二', '初三', '初四', '初五', '初六', '初七', '初八', '初九', '初十',
  '十一', '十二', '十三', '十四', '十五', '十六', '十七', '十八', '十九',
  '二十', '廿一', '廿二', '廿三', '廿四', '廿五', '廿六', '廿七', '廿八', '廿九', '三十']

function lunarMonthName(m) { return lunarMonthNames[m] || m }
function lunarDayName(d) { return Number(d) === 0 ? '不固定' : (lunarDayNames[d] || d) }

async function seedDefaults() {
  try {
    await ElMessageBox.confirm(
      '会自动补充：春节、除夕、元宵、龙抬头、清明、端午、七夕、中元、中秋、重阳、寒衣、下元、腊八、小年。\n已存在的同名节日会跳过，不会改动你已有的条目。\n\n其中「除夕」「清明」每年日期不同，补充后可逐个填临时日期（现在支持直接选农历月/日）。',
      '补充通用节日', { type: 'info', confirmButtonText: '开始补充' }
    )
  } catch (e) { return }
  try {
    const r = await api.post('/traditions/seed-defaults')
    ElMessage.success(r.data?.message || '已补充通用节日')
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '补充失败')
  }
}

function checkRole() {
  try {
    const u = JSON.parse(localStorage.getItem('user') || 'null')
    isElder.value = !!u && (u.role === '理事' || u.role === '理事长' || u.role === '超级管理员')
  } catch (e) { /* ignore */ }
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/traditions')
    list.value = data
  } catch (e) { /* ignore */ }
  loading.value = false
}

function openAdd() {
  form.value = { id: 0, title: '', lunar_month: 1, lunar_day: 1, is_leap_month: false, description: '' }
  isEdit.value = false
  dlgVisible.value = true
}

function openEdit(row) {
  form.value = { ...row }
  isEdit.value = true
  dlgVisible.value = true
}

async function save() {
  if (!form.value.title) { ElMessage.warning('请输入活动名称'); return }
  saving.value = true
  try {
    if (isEdit.value) {
      await api.put('/traditions/' + form.value.id, form.value)
      ElMessage.success('已更新')
    } else {
      await api.post('/traditions', form.value)
      ElMessage.success('已新增')
    }
    dlgVisible.value = false
    load()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '操作失败')
  }
  saving.value = false
}

async function del(row) {
  try {
    await ElMessageBox.confirm('确定删除「' + row.title + '」？', '确认', { type: 'warning' })
  } catch (e) { return }
  try {
    await api.delete('/traditions/' + row.id)
    ElMessage.success('已删除')
    load()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '删除失败')
  }
}

onMounted(() => {
  checkRole()
  load()
})
</script>

<style scoped>
.tradition-view { max-width: 960px; margin: 0 auto; }
.page-title { color: #3a3226; margin-bottom: 4px; }
.subtitle { color: #8b7355; font-size: 13px; margin-bottom: 16px; }
.hint-line { margin-top: -10px; padding: 8px 10px; background: #fbf6ec; border: 1px solid #ece0cb; border-radius: 6px; line-height: 1.6; }
.hint-line b { color: #8c5a3c; }
.toolbar { margin-bottom: 12px; }
.form-hint { margin-top: 4px; font-size: 12px; color: #909399; line-height: 1.5; }
.temp-cell { display: flex; flex-direction: column; align-items: flex-start; gap: 2px; line-height: 1.4; }
.temp-ok { color: #4e8f2f; font-weight: 600; }
.temp-days { color: #7a6a52; font-weight: 400; font-size: 12px; margin-left: 2px; }
.temp-expired { color: #c0392b; font-size: 12px; }
.temp-none { color: #909399; }
.temp-btn { padding: 0; height: auto; font-size: 12px; }
.temp-dialog { line-height: 1.7; }
.temp-name { font-size: 15px; font-weight: 700; color: #3a3226; margin: 0 0 6px; }
.temp-hint { font-size: 13px; color: #7a6a52; background: #fbf6ec; border: 1px solid #ece0cb; border-radius: 6px; padding: 8px 10px; margin: 0 0 12px; }
.temp-hint b { color: #8c5a3c; }
.temp-cur { font-size: 12px; color: #8c5a3c; margin: 8px 0 0; }
</style>