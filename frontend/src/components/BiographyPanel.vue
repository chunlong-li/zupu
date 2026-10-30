<template>
  <div class="bio-panel">
    <div class="tab-header">
      <span class="serif title">名人传记</span>
      <div class="filters">
        <el-input v-model="q" placeholder="搜索姓名/字号/身份" clearable style="width: 190px"
          @keyup.enter="load" @clear="load" />
        <el-select v-model="personType" size="default" style="width: 128px" @change="load">
          <el-option label="全部人员" value="all" />
          <el-option label="族内成员" value="member" />
          <el-option label="配偶" value="spouse" />
          <el-option label="其他人员" value="other" />
        </el-select>
        <el-button @click="load">查询</el-button>
      </div>
      <div class="head-actions">
        <el-button v-if="canRollback()" @click="openCriteria">入传标准</el-button>
        <el-button type="primary" @click="openDialog()">新增传记</el-button>
        <el-button @click="openPrint">打印人物志</el-button>
      </div>
    </div>

    <el-alert v-if="criteria" type="info" :closable="false" class="criteria-bar"
      :title="'入传标准：' + criteria" />

    <div class="bio-list">
      <div class="bio-item" v-for="b in list" :key="b.id">
        <div class="bio-portrait">
          <img v-if="b.portrait" :src="b.portrait" alt="" />
          <div v-else class="ph">{{ (b.name || '传').slice(0, 1) }}</div>
        </div>
        <div class="bio-body">
          <h4 class="serif">
            {{ b.name }}
            <span class="bio-sub" v-if="b.zi">字{{ b.zi }}</span>
            <span class="bio-sub" v-if="b.hao">号{{ b.hao }}</span>
            <el-tag size="small" effect="plain" class="type-tag">{{ typeText(b.person_type) }}</el-tag>
            <el-tag v-if="b.public === 0" size="small" type="warning" effect="plain">未公开</el-tag>
          </h4>
          <div class="bio-meta" v-if="lifeLine(b)">{{ lifeLine(b) }}</div>
          <div class="bio-meta" v-if="infoLine(b)">{{ infoLine(b) }}</div>
          <div class="bio-meta" v-if="b.honors">荣誉：{{ b.honors }}</div>
          <div class="bio-summary" v-if="b.summary">{{ b.summary }}</div>
          <div class="bio-content" v-if="b.content">{{ b.content }}</div>
          <div class="bio-reason" v-if="b.reason">入选理由：{{ b.reason }}</div>
          <div class="item-actions">
            <el-button size="small" text @click="openDialog(b)">编辑</el-button>
            <el-button size="small" text type="danger" @click="remove(b)">删除</el-button>
          </div>
        </div>
      </div>
      <el-empty v-if="!list.length" description="暂无传记" />
    </div>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="visible" :title="form.id ? '编辑传记' : '新增传记'" width="760px" top="4vh">
      <el-form :model="form" label-width="86px">
        <el-form-item label="人员类型">
          <el-radio-group v-model="form.person_type" @change="onTypeChange">
            <el-radio-button value="member">族内成员</el-radio-button>
            <el-radio-button value="spouse">配偶</el-radio-button>
            <el-radio-button value="other">其他人员</el-radio-button>
          </el-radio-group>
          <span class="hint">配偶、姻亲、外族/地方名人同样可以立传</span>
        </el-form-item>
        <el-form-item v-if="form.person_type === 'member'" label="关联成员">
          <el-select v-model="form.member_id" filterable clearable placeholder="输入姓名搜索（可不选）"
            style="width: 100%" @change="fillFromMember">
            <el-option v-for="m in members" :key="m.id" :label="memberLabel(m)" :value="m.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.person_type === 'spouse'" label="关联配偶">
          <el-select v-model="form.spouse_id" filterable clearable placeholder="输入姓名搜索（可不选）"
            style="width: 100%" @change="fillFromSpouse">
            <el-option v-for="s in spousesFlat" :key="s.id" :label="s.label" :value="s.id" />
          </el-select>
        </el-form-item>

        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="姓名"><el-input v-model="form.name" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="字"><el-input v-model="form.zi" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="号"><el-input v-model="form.hao" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="性别">
              <el-radio-group v-model="form.gender">
                <el-radio-button value="男">男</el-radio-button>
                <el-radio-button value="女">女</el-radio-button>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="8"><el-form-item label="籍贯"><el-input v-model="form.native_place" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="现居"><el-input v-model="form.current_place" /></el-form-item></el-col>
        </el-row>

        <el-form-item label="生（年号）"><el-input v-model="form.birth_era" placeholder="如：民国十年 / 共和三十九" /></el-form-item>
        <el-form-item label="生（农历）"><el-input v-model="form.birth_lunar" placeholder="如：一九五〇年三月十二" /></el-form-item>
        <el-form-item label="生（公历）"><el-input v-model="form.birth_gregorian" placeholder="如：1950-04-20 或 1950" /></el-form-item>
        <el-form-item label="殁（年号）"><el-input v-model="form.death_era" placeholder="如：共和五十年" /></el-form-item>
        <el-form-item label="殁（农历）"><el-input v-model="form.death_lunar" /></el-form-item>
        <el-form-item label="殁（公历）"><el-input v-model="form.death_gregorian" /></el-form-item>

        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="身份"><el-input v-model="form.title" placeholder="如：教授 / 进士" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="职业"><el-input v-model="form.occupation" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="荣誉"><el-input v-model="form.honors" placeholder="如：全国劳模" /></el-form-item></el-col>
        </el-row>

        <el-form-item label="肖像">
          <div class="portrait-row">
            <div class="portrait-preview">
              <img v-if="form.portrait" :src="form.portrait" alt="" />
              <span v-else class="ph">无</span>
            </div>
            <el-upload :action="'/api/upload'" :headers="uploadHeaders" :show-file-list="false"
              :data="{ category: '传记肖像' }" accept="image/*" :on-success="onPortraitUpload"
              :on-error="onPortraitError">
              <el-button size="small">上传肖像</el-button>
            </el-upload>
            <el-button v-if="linkedPortrait" size="small" @click="form.portrait = linkedPortrait">用关联人照片</el-button>
            <el-button v-if="form.portrait" size="small" text type="danger" @click="form.portrait = ''">清除</el-button>
          </div>
        </el-form-item>

        <el-form-item label="摘要"><el-input v-model="form.summary" type="textarea" :rows="2"
          placeholder="列表展示用的一句话简介" /></el-form-item>
        <el-form-item label="正文"><el-input v-model="form.content" type="textarea" :rows="9"
          placeholder="传记正文" /></el-form-item>

        <div class="ai-polish-box" v-if="canUseAI()">
          <div class="ai-polish-row">
            <span class="ai-polish-label">📝 AI 生成传记</span>
            <el-select v-model="aiGenStyle" size="small" style="width: 104px">
              <el-option label="传记体" value="传记体" />
              <el-option label="简洁" value="简洁" />
              <el-option label="文言" value="文言" />
            </el-select>
            <el-select v-model="aiGenLength" size="small" style="width: 84px">
              <el-option label="短" value="短" />
              <el-option label="中" value="中" />
              <el-option label="长" value="长" />
            </el-select>
            <el-button size="small" type="success" plain :loading="aiGenerating" @click="aiGenerateBiography">按资料 + 生平生成</el-button>
            <span class="hint">依据本页已填资料，加上关联成员/配偶的档案与其生平条目生成正文</span>
          </div>
          <div class="ai-polish-row">
            <span class="ai-polish-label">✨ AI 优化</span>
            <el-button size="small" type="warning" plain :loading="aiLoading" @click="aiPolish">病句排查 + 美化</el-button>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">约束等级</span>
            <el-button v-for="v in levelOptions" :key="'l' + v" size="small" :type="aiLevel === v ? 'success' : ''"
              :plain="aiLevel !== v" @click="aiLevel = v">{{ v }}%</el-button>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">润色程度</span>
            <el-button v-for="v in levelOptions" :key="'d' + v" size="small" :type="aiDegree === v ? 'success' : ''"
              :plain="aiDegree !== v" @click="aiDegree = v">{{ v }}%</el-button>
          </div>
          <div v-if="aiResult" class="ai-result">
            <el-input v-model="aiResult" type="textarea" :rows="6" />
            <div class="ai-result-actions">
              <el-button size="small" type="primary" @click="form.content = aiResult; aiResult = ''">采用</el-button>
              <el-button size="small" @click="aiResult = ''">放弃</el-button>
            </div>
          </div>
        </div>

        <el-form-item label="入选理由"><el-input v-model="form.reason" type="textarea" :rows="2"
          placeholder="符合哪条入传标准，供审核参考" /></el-form-item>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="排序年份"><el-input v-model.number="form.sort_year" placeholder="生年，用于排序" @input="yearTouched = true" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="排序号"><el-input v-model.number="form.sort_order" /></el-form-item></el-col>
          <el-col :span="8">
            <el-form-item label="公开展示">
              <el-switch v-model="form.public" :active-value="1" :inactive-value="0" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>

    <!-- 入传标准 -->
    <el-dialog v-model="criteriaVisible" title="入传标准" width="560px">
      <el-input v-model="criteriaForm" type="textarea" :rows="6"
        placeholder="例如：对社会有重大贡献、获省级以上荣誉、捐资万元以上者，由理事会审定后立传" />
      <template #footer>
        <el-button @click="criteriaVisible = false">取消</el-button>
        <el-button type="primary" @click="saveCriteria">保存</el-button>
      </template>
    </el-dialog>

    <!-- 打印设置 -->
    <el-dialog v-model="printVisible" title="打印 / 导出人物志" width="440px">
      <el-form label-width="100px">
        <el-form-item label="页边距 (mm)">
          <el-input-number v-model="printMargin" :min="0" :max="60" :step="1" />
          <span class="hint">填 0 = 满版（内容到纸边）</span>
        </el-form-item>
        <el-form-item label="装订线 (mm)">
          <el-input-number v-model="printGutter" :min="0" :max="60" :step="1" />
          <span class="hint">在装订边额外加宽</span>
        </el-form-item>
        <el-form-item label="装订边">
          <el-radio-group v-model="printBindSide">
            <el-radio-button value="left">左</el-radio-button>
            <el-radio-button value="right">右</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="版式">
          <el-radio-group v-model="printLandscape">
            <el-radio-button :value="false">竖版</el-radio-button>
            <el-radio-button :value="true">横版</el-radio-button>
          </el-radio-group>
          <span class="hint">{{ printLandscape ? 'A4 横向 · 一人一页（左：肖像信息 / 右：传记正文）' : 'A4 竖版 · 一人一页（谱页样式）' }}</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="printVisible = false">取消</el-button>
        <el-button type="primary" @click="doPrint">打印</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { canRollback, canUseAI } from '../utils/auth'
import { formatBirth, formatDeath } from '../utils/dateFormat'

const route = useRoute()
const list = ref([])
const members = ref([])
const spousesFlat = ref([])
const criteria = ref('')
const q = ref('')
const personType = ref('all')
const visible = ref(false)
const criteriaVisible = ref(false)
const criteriaForm = ref('')
const saving = ref(false)
const aiLoading = ref(false)
const aiGenerating = ref(false)
const aiResult = ref('')
const aiLevel = ref(50)
const aiGenStyle = ref('传记体')   // AI 生成传记的文体
const aiGenLength = ref('中')      // AI 生成传记的篇幅
const aiDegree = ref(50)
const levelOptions = [0, 25, 50, 75, 100]

const form = reactive(blank())
const yearTouched = ref(false) // 排序年份是否被手改过（手改后不再自动覆盖）

// —— 自动填「排序年份 / 排序号」（仍可修改）——
function cnNum(s) {
  const d = { '〇': 0, '零': 0, '○': 0, '〇': 0, '一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9 }
  if (!s) return 0
  if (s === '十') return 10
  if (s.includes('十')) {
    const [a, b] = s.split('十')
    const tens = a ? (d[a] || 0) : 1
    const ones = b ? (d[b] || 0) : 0
    return tens * 10 + ones
  }
  let n = 0
  for (const ch of s) n = n * 10 + (d[ch] || 0)
  return n
}
// 从生年推断排序年份：公历 4 位 → 农历「一九五〇年」→ 年号（民国/共和 N 年）
function guessYear() {
  const g = String(form.birth_gregorian || '')
  const gm = g.match(/(\d{4})/)
  if (gm) return Number(gm[1])
  const l = String(form.birth_lunar || '')
  const lm = l.match(/([〇零○一二三四五六七八九]{4})年/)
  if (lm) { const y = cnNum(lm[1]); if (y > 1000) return y }
  const e = String(form.birth_era || '')
  const em = e.match(/(民国|共和)([〇零○一二三四五六七八九十]+)/)
  if (em) {
    const base = em[1] === '民国' ? 1911 : 1949
    const n = cnNum(em[2])
    if (n > 0) return base + n
  }
  return 0
}
function nextOrder() {
  if (!list.value.length) return 1
  return Math.max(...list.value.map(b => Number(b.sort_order) || 0)) + 1
}
watch(() => [form.birth_era, form.birth_lunar, form.birth_gregorian], () => {
  if (yearTouched.value) return
  const y = guessYear()
  if (y) form.sort_year = y
})

const uploadHeaders = computed(() => ({ Authorization: 'Bearer ' + (localStorage.getItem('token') || '') }))

function blank() {
  return {
    id: 0, person_type: 'member', member_id: null, spouse_id: null,
    name: '', zi: '', hao: '', gender: '男',
    birth_era: '', birth_lunar: '', birth_gregorian: '',
    death_era: '', death_lunar: '', death_gregorian: '',
    native_place: '', current_place: '', title: '', occupation: '', honors: '',
    portrait: '', summary: '', content: '', reason: '', sort_year: 0, sort_order: 0, public: 1
  }
}

function typeText(t) {
  return t === 'member' ? '族内成员' : (t === 'spouse' ? '配偶' : '其他人员')
}
function memberLabel(m) {
  const n = m.name || ((m.surname || '') + (m.given_name || ''))
  return `${n}（第${m.generation || '?'}世${m.branch ? ' · ' + m.branch : ''}）`
}
function lifeLine(b) {
  const parts = []
  const bt = formatBirth(b)
  const dt = formatDeath(b)
  if (bt && bt !== '生年未詳') parts.push(bt.replace(/^生於/, '生 '))
  if (dt && dt !== '卒年未詳') parts.push(dt.replace(/^卒於/, '殁 '))
  return parts.join('　')
}
function infoLine(b) {
  return [b.native_place ? '籍贯 ' + b.native_place : '', b.current_place ? '现居 ' + b.current_place : '',
    b.title || '', b.occupation || ''].filter(Boolean).join(' · ')
}
const linkedPortrait = computed(() => {
  if (form.person_type === 'member' && form.member_id) {
    const m = members.value.find(x => x.id === form.member_id)
    return (m && m.portrait) || ''
  }
  if (form.person_type === 'spouse' && form.spouse_id) {
    const s = spousesFlat.value.find(x => x.id === form.spouse_id)
    return (s && s.portrait) || ''
  }
  return ''
})

async function load() {
  try {
    const { data } = await api.get('/biographies', { params: { q: q.value, person_type: personType.value } })
    list.value = data || []
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '加载传记失败')
  }
}

async function loadRefs() {
  try {
    const [{ data: ms }, { data: ws }, { data: cr }] = await Promise.all([
      api.get('/members'),
      api.get('/members/with-spouses'),
      api.get('/biography-criteria')
    ])
    members.value = ms || []
    const flat = []
    ;(ws || []).forEach(m => {
      (m.spouses || []).forEach(s => {
        flat.push({ id: s.id, name: s.name || s.given_name || '', type: s.type || '', portrait: s.portrait || '',
          label: `${s.name || ''}${s.type ? '（' + s.type + '）' : ''} — ${m.name || ''} 之配偶` })
      })
    })
    spousesFlat.value = flat
    criteria.value = (cr && cr.criteria) || ''
  } catch (e) { /* 参考数据失败不阻塞 */ }
}

function onTypeChange() { /* 切换类型不清空已填内容 */ }

function fillFromMember(id) {
  const m = members.value.find(x => x.id === id)
  if (!m) return
  form.name = m.name || ((m.surname || '') + (m.given_name || ''))
  form.zi = m.zi || ''
  form.hao = m.hao || ''
  form.gender = m.gender === 2 ? '女' : '男'
  form.birth_era = m.birth_era || ''
  form.birth_lunar = m.birth_lunar || ''
  form.birth_gregorian = m.birth_gregorian || ''
  form.death_era = m.death_era || ''
  form.death_lunar = m.death_lunar || ''
  form.death_gregorian = m.death_gregorian || ''
  form.native_place = m.native_place || ''
  form.current_place = m.current_place || ''
  form.title = m.title || ''
  form.occupation = m.occupation || ''
  if (!form.portrait) form.portrait = m.portrait || ''
}

function fillFromSpouse(id) {
  const s = spousesFlat.value.find(x => x.id === id)
  if (!s) return
  form.name = s.name || ''
  form.gender = '女'
  if (!form.portrait) form.portrait = s.portrait || ''
}

function openDialog(b) {
  Object.assign(form, blank())
  yearTouched.value = false
  if (b) {
    Object.assign(form, { ...b, member_id: b.member_id || null, spouse_id: b.spouse_id || null })
    yearTouched.value = true // 已有值，不自动覆盖
  } else if (route.query.member) {
    form.person_type = 'member'
    form.member_id = Number(route.query.member) || null
    fillFromMember(form.member_id)
  } else if (route.query.spouse) {
    form.person_type = 'spouse'
    form.spouse_id = Number(route.query.spouse) || null
    fillFromSpouse(form.spouse_id)
  }
  if (!b) {
    if (!form.sort_order) form.sort_order = nextOrder()
    const y = guessYear()
    if (y) form.sort_year = y
  }
  aiResult.value = ''
  visible.value = true
}

function onPortraitUpload(resp) {
  if (resp && resp.path) { form.portrait = resp.path; ElMessage.success('肖像已上传') }
  else ElMessage.error('上传失败')
}
function onPortraitError() { ElMessage.error('上传失败') }

async function save() {
  if (!form.name || !form.name.trim()) { ElMessage.warning('请填写姓名'); return }
  saving.value = true
  try {
    const payload = { ...form, member_id: form.member_id || 0, spouse_id: form.spouse_id || 0 }
    let r
    if (form.id) r = await api.put(`/biographies/${form.id}`, payload)
    else r = await api.post('/biographies', payload)
    if (r.data && r.data.draft) ElMessage.success(r.data.message || '已提交审核')
    else ElMessage.success('已保存')
    visible.value = false
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove(b) {
  try {
    await ElMessageBox.confirm(`确定删除「${b.name}」的传记？`, '确认', { type: 'warning' })
  } catch (e) { return }
  try {
    const r = await api.delete(`/biographies/${b.id}`)
    if (r.data && r.data.draft) ElMessage.success(r.data.message || '已提交审核')
    else ElMessage.success('已删除')
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '删除失败')
  }
}

function openCriteria() {
  criteriaForm.value = criteria.value
  criteriaVisible.value = true
}
async function saveCriteria() {
  try {
    await api.put('/biography-criteria', { criteria: criteriaForm.value })
    criteria.value = criteriaForm.value
    criteriaVisible.value = false
    ElMessage.success('已保存入传标准')
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '保存失败')
  }
}

// AI 生成传记：把本页表单资料（含未保存的修改）交给后端，后端再补上关联成员/配偶的档案与生平条目
async function aiGenerateBiography() {
  if (form.content && form.content.trim()) {
    try {
      await ElMessageBox.confirm('将依据本页资料与关联人的档案/生平生成传记正文，并覆盖下方「正文」里已写的文字（可用「放弃」保留）。继续？', '生成传记', { type: 'warning' })
    } catch (e) { return }
  }
  aiGenerating.value = true
  aiResult.value = ''
  try {
    const { data } = await api.post('/ai/generate-biography', {
      member_id: form.member_id || 0,
      spouse_id: form.spouse_id || 0,
      name: form.name || '', zi: form.zi || '', hao: form.hao || '', gender: form.gender || '',
      birth_era: form.birth_era || '', birth_lunar: form.birth_lunar || '', birth_gregorian: form.birth_gregorian || '',
      death_era: form.death_era || '', death_lunar: form.death_lunar || '', death_gregorian: form.death_gregorian || '',
      native_place: form.native_place || '', current_place: form.current_place || '',
      title: form.title || '', occupation: form.occupation || '', honors: form.honors || '',
      summary: form.summary || '', reason: form.reason || '',
      style: aiGenStyle.value, length: aiGenLength.value,
      level: aiLevel.value, degree: aiDegree.value
    }, { timeout: 190000 })
    aiResult.value = data.content || ''
    if (data.protected) ElMessage.warning('AI 对日期/数字的改动已被拦截，保留材料原文')
    else ElMessage.success('已生成传记草稿，确认后点「采用」填入正文')
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || 'AI 生成失败，请确认修谱助手已启用并配置好模型')
  } finally {
    aiGenerating.value = false
  }
}

async function aiPolish() {
  if (!form.content || !form.content.trim()) { ElMessage.warning('请先填写正文'); return }
  aiLoading.value = true
  aiResult.value = ''
  try {
    const { data } = await api.post('/ai/polish-bio', {
      title: form.name || '',
      content: form.content.trim(),
      level: aiLevel.value,
      degree: aiDegree.value
    }, { timeout: 190000 })
    aiResult.value = data.content || form.content
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || 'AI 优化失败，请确认修谱助手已启用')
  } finally {
    aiLoading.value = false
  }
}

function esc(s) {
  return String(s == null ? '' : s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
const printVisible = ref(false)
const printMargin = ref(15)      // 页边距 mm（0 = 满版）
const printGutter = ref(0)       // 装订线 mm（在装订边额外加宽）
const printBindSide = ref('left')
const printLandscape = ref(localStorage.getItem('bioPrintLandscape') === '1')  // 横版（A4 横向，左肖像信息 / 右传记正文）
watch(printLandscape, v => localStorage.setItem('bioPrintLandscape', v ? '1' : '0'))

function openPrint() {
  if (!list.value.length) { ElMessage.warning('暂无可打印的传记'); return }
  printVisible.value = true
}
function doPrint() {
  printVisible.value = false
  setTimeout(printBios, 120)
}

function printBios() {
  if (!list.value.length) { ElMessage.warning('暂无可打印的传记'); return }
  const m = Math.max(0, Number(printMargin.value) || 0)
  const g = Math.max(0, Number(printGutter.value) || 0)
  const side = printBindSide.value
  const pad = { top: m, bottom: m, left: m, right: m }
  if (side === 'left') pad.left = m + g
  else pad.right = m + g
  const land = !!printLandscape.value
  const total = list.value.length
  const items = list.value.map((b, i) => `
    <section class="sheet${land ? ' land' : ''}">
      ${g > 0 ? `<div class="gutter" style="${side === 'left' ? `left:${m + g}mm;border-left:1px dashed #c9c2b2` : `right:${m + g}mm;border-right:1px dashed #c9c2b2`}"></div>` : ''}
      <div class="board"><div class="bin${land ? ' land' : ''}">
        <div class="card">
          <div class="head">
            ${b.portrait ? `<img src="${esc(b.portrait)}" />` : `<div class="ph">${esc((b.name || '传').slice(0, 1))}</div>`}
            <div class="meta-wrap">
              <h2>${esc(b.name)}${b.zi ? ` <small>字${esc(b.zi)}</small>` : ''}${b.hao ? ` <small>号${esc(b.hao)}</small>` : ''}</h2>
              ${lifeLine(b) ? `<div class="meta">${esc(lifeLine(b))}</div>` : ''}
              ${infoLine(b) ? `<div class="meta">${esc(infoLine(b))}</div>` : ''}
              ${b.honors ? `<div class="meta">荣誉：${esc(b.honors)}</div>` : ''}
              ${b.summary ? `<div class="summary">${esc(b.summary)}</div>` : ''}
              ${b.reason ? `<div class="reason">入传缘由：${esc(b.reason)}</div>` : ''}
            </div>
          </div>
        </div>
        <div class="text">
          <div class="text-title">傳</div>
          <div class="text-body">${(b.content || '（正文待补充）').split('\n').map(p => `<p>${esc(p)}</p>`).join('')}</div>
        </div>
        <div class="pager">第 ${i + 1} 页 / 共 ${total} 页</div>
      </div></div>
    </section>`).join('')
  const w = window.open('', '_blank')
  if (!w) { ElMessage.warning('浏览器拦截了打印窗口'); return }
  w.document.write(`<!doctype html><html><head><meta charset="utf-8"><title>人物志</title><style>
    @page { size: A4 ${land ? 'landscape' : 'portrait'}; margin: 0; }
    html, body { margin: 0; padding: 0; background: #fff; }
    body { font-family: "Songti SC", "SimSun", serif; color: #1f1c16; -webkit-print-color-adjust: exact; print-color-adjust: exact; }
    .sheet { width: ${land ? '297' : '210'}mm; min-height: ${land ? '210' : '297'}mm; box-sizing: border-box; background: #fffdf6;
             display: flex; flex-direction: column; position: relative;
             padding: ${pad.top}mm ${pad.right}mm ${pad.bottom}mm ${pad.left}mm;
             page-break-after: always; break-after: page; }
    .sheet:last-child { page-break-after: auto; break-after: auto; }
    .gutter { position: absolute; top: 0; bottom: 0; width: 0; }
    .board { flex: 1 1 auto; border: 1px solid #1a1a1a; padding: 4px; box-sizing: border-box; display: flex; }
    .bin { flex: 1 1 auto; border: 3px solid #1a1a1a; box-sizing: border-box; padding: 10px; display: flex; flex-direction: column; }
    .card { border: 1px solid #1a1a1a; padding: 10px 12px; }
    .head { display: flex; gap: 14px; align-items: flex-start; }
    .head img, .head .ph { width: 32mm; height: 40mm; object-fit: cover; border: 1px solid #1a1a1a; }
    .head .ph { display: flex; align-items: center; justify-content: center; font-size: 28pt; color: #a3927c; }
    .meta-wrap { flex: 1; min-width: 0; }
    h2 { margin: 0 0 4px; font-size: 17pt; }
    small { font-size: 11pt; color: #6b5c48; margin-left: 6px; }
    .meta { font-size: 10.5pt; color: #4a4137; margin-top: 2px; }
    .summary { margin-top: 6px; font-size: 11pt; color: #3d3225; }
    .reason { margin-top: 6px; font-size: 9.5pt; color: #8b7c66; }
    .text { flex: 1 1 auto; margin-top: 8px; border: 1px solid #1a1a1a; padding: 8px 14px 12px; }
    .text-title { text-align: center; font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif; font-size: 16pt;
                  font-weight: 700; letter-spacing: 8px; padding-bottom: 6px; border-bottom: 1px solid #1a1a1a; margin-bottom: 8px; }
    .text-body { font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif; font-size: 12pt; line-height: 1.95; }
    .text-body p { margin: 4px 0; text-indent: 2em; }
    .pager { margin-top: 6px; text-align: center; font-size: 9pt; color: #7a6858; }
    ${land ? `
    /* 横版：左栏肖像+信息卡，右栏「傳」正文，页码通栏 */
    .bin.land { display: grid; grid-template-columns: 92mm 1fr; grid-template-rows: 1fr auto; gap: 3mm 7mm; }
    .bin.land .card { grid-column: 1; grid-row: 1; align-self: start; }
    .bin.land .head { flex-direction: column; align-items: center; text-align: center; gap: 8px; }
    .bin.land .head img, .bin.land .head .ph { width: 44mm; height: 55mm; }
    .bin.land .text { grid-column: 2; grid-row: 1; margin-top: 0; }
    .bin.land .summary, .bin.land .reason { text-align: left; }   /* 摘要/入传缘由左对齐（居中折行太散） */
    .bin.land .pager { grid-column: 1 / -1; grid-row: 2; margin-top: 0; }
    ` : ''}
  </style></head><body>
    ${items}
  </body></html>`)
  w.document.close()
  w.focus()
  setTimeout(() => w.print(), 500)
}

onMounted(async () => {
  await loadRefs()
  if (route.query.person_type) personType.value = String(route.query.person_type)
  if (route.query.member) personType.value = 'member'
  if (route.query.spouse) personType.value = 'spouse'
  await load()
  if (route.query.new === '1') openDialog()
})
</script>

<style scoped>
.bio-panel { padding: 4px 2px; }
.tab-header { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 10px; }
.tab-header .title { font-size: 18px; color: #5b4636; margin-right: 6px; }
.tab-header .filters { display: flex; align-items: center; gap: 8px; }
.tab-header .head-actions { margin-left: auto; display: flex; gap: 8px; }
.criteria-bar { margin-bottom: 12px; }
.bio-list { display: flex; flex-direction: column; gap: 12px; }
.bio-item { display: flex; gap: 14px; padding: 14px; border: 1px solid #e6dccb; border-radius: 6px; background: #fdfbf6; }
.bio-portrait { flex: 0 0 96px; }
.bio-portrait img, .bio-portrait .ph { width: 96px; height: 120px; object-fit: cover; border: 1px solid #d6c9b4; border-radius: 4px; display: flex; align-items: center; justify-content: center; background: #f2ece0; color: #a3927c; font-size: 30px; }
.bio-body { flex: 1; min-width: 0; }
.bio-body h4 { margin: 0 0 6px; font-size: 17px; color: #3d2f22; }
.bio-sub { font-size: 13px; color: #7a6858; margin-left: 8px; font-weight: normal; }
.type-tag { margin-left: 8px; }
.bio-meta { font-size: 13px; color: #7a6858; margin-bottom: 3px; }
.bio-summary { margin-top: 6px; font-weight: 600; color: #4a3a2a; }
.bio-content { margin-top: 6px; white-space: pre-wrap; line-height: 1.75; color: #3d2f22; }
.bio-reason { margin-top: 6px; font-size: 13px; color: #8c6b4a; }
.item-actions { margin-top: 8px; }
.hint { margin-left: 10px; font-size: 12px; color: #a3927c; }
.portrait-row { display: flex; align-items: center; gap: 10px; }
.portrait-preview img, .portrait-preview .ph { width: 66px; height: 82px; object-fit: cover; border: 1px solid #d6c9b4; border-radius: 4px; display: flex; align-items: center; justify-content: center; background: #f2ece0; color: #a3927c; font-size: 12px; }
.ai-polish-box { margin: 0 0 12px 86px; padding: 8px 10px; border: 1px dashed #d6c9b4; border-radius: 6px; background: #fffdf8; }
.ai-polish-row, .ai-level-row { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.ai-polish-label { font-size: 13px; color: #8c5a3c; }
.ai-level-label { font-size: 12px; color: #a3927c; width: 56px; }
.ai-result { margin-top: 6px; }
.ai-result-actions { margin-top: 6px; display: flex; gap: 8px; }
</style>
