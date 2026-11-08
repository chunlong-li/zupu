<template>
    <el-dialog :model-value="modelValue" @update:model-value="v => emit('update:modelValue', v)" :title="editForm.id ? '编辑成员' : '新增成员'" width="750px">
      <el-form :model="editForm" label-width="90px">
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="姓"><el-input v-model="editForm.surname" disabled /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="名"><div class="hz-inline"><el-input v-model="editForm.given_name" placeholder="输入名" /><el-button size="small" text type="primary" @click="openHanzi('given_name')">生僻字</el-button></div></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="世代">
            <template v-if="editForm.record_type === '外纪'">
              <el-input :model-value="waijiGenLabel" disabled />
            </template>
            <el-input-number v-else v-model="editForm.generation" :min="1" />
          </el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="性别"><el-select v-model="editForm.gender"><el-option label="男" :value="1" /><el-option label="女" :value="2" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="字"><div class="hz-inline"><el-input v-model="editForm.zi" /><el-button size="small" text type="primary" @click="openHanzi('zi')">生僻字</el-button></div></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="号"><div class="hz-inline"><el-input v-model="editForm.hao" /><el-button size="small" text type="primary" @click="openHanzi('hao')">生僻字</el-button></div></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="排行"><el-input v-model="editForm.rank" placeholder="行三" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="字辈"><div class="hz-inline"><el-input v-model="editForm.generation_word" /><el-button size="small" text type="primary" @click="openHanzi('generation_word')">生僻字</el-button></div></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="房支"><el-select v-model="editForm.branch_id" clearable filterable placeholder="选择房支" style="width:100%"><el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.id" /></el-select></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="宗法"><el-select v-model="editForm.clan_type"><el-option v-for="t in clanTypes" :key="t" :label="t" :value="t" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="类型"><el-select v-model="editForm.record_type" @change="onRecordTypeChange"><el-option label="内纪" value="内纪" /><el-option label="外纪" value="外纪" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="出生"><el-row :gutter="4"><el-col :span="8"><el-select v-model="editForm.birth_calendar" size="small"><el-option label="公历" value="solar" /><el-option label="农历" value="lunar" /></el-select></el-col><el-col :span="16"><el-input v-model="editForm.birth_date" placeholder="1900-08-01" @blur="convertDate" /></el-col></el-row></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="卒年"><el-input v-model="editForm.death_gregorian" placeholder="留空=在世" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="父母">
          <div style="width:100%;display:flex;gap:8px;align-items:flex-start">
            <el-tooltip :visible="adoptTipVisible" placement="top" effect="light" trigger="manual">
              <template #content><div style="max-width:330px;line-height:1.75" v-html="adoptTipHtml"></div></template>
              <span ref="adoptQRef" class="adopt-q" @click.stop="toggleAdoptTip">?</span>
            </el-tooltip>
            <div style="flex:1">
              <!-- 生父 / 生母 -->
              <div style="display:flex;gap:8px;margin-bottom:8px;align-items:center;flex-wrap:wrap">
                <el-select :model-value="'生父'" disabled style="width:90px"><el-option label="生父" value="生父" /></el-select>
                <el-select v-model="editForm.father_id" filterable clearable placeholder="生父（本族）" style="width:160px" @change="onFatherChange">
                  <el-option v-for="m in fatherCandidates" :key="'f'+m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel(m)}）`" :value="m.id" />
                </el-select>
                <el-select v-if="editForm.record_type !== '外纪'" v-model="editForm.mother_spouse_id" filterable clearable placeholder="生母" style="width:160px" @focus="loadEditMotherCandidates">
                  <el-option v-for="m in editMotherCandidates" :key="'sp'+m.id" :label="`${m.name}（${m.type}）`" :value="m.id" />
                </el-select>
                <el-input v-if="!editForm.father_id" v-model="editForm.birth_father_text" placeholder="生父在谱外：姓名" style="width:180px" />
              </div>
              <!-- 嗣父 / 养父（各带母） -->
              <div v-for="(a, i) in editForm.adoptions" :key="'ad'+i" style="display:flex;gap:8px;margin-bottom:8px;align-items:center;flex-wrap:wrap">
                <el-select v-model="a.kind" style="width:90px" @change="syncClanTypeByAdoption">
                  <el-option label="嗣父" value="嗣父" />
                  <el-option label="养父" value="养父" />
                </el-select>
                <el-select v-model="a.father_id" filterable clearable placeholder="父（本族）" style="width:160px" @change="onAdoptFatherChange(a)">
                  <el-option v-for="m in fatherCandidates" :key="'af'+m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel(m)}）`" :value="m.id" />
                </el-select>
                <el-select v-model="a.mother_id" filterable clearable placeholder="母（本族）" style="width:160px">
                  <el-option v-for="m in (fatherSpouseCache[a.father_id] || [])" :key="'am'+m.id" :label="`${m.name}（${m.type}）`" :value="m.id" />
                </el-select>
                <el-button text type="danger" @click="removeAdoption(i)">删除</el-button>
              </div>
              <div>
                <el-button size="small" @click="addAdoption">+ 添加父母</el-button>
              </div>
            </div>
          </div>
        </el-form-item>
        <el-row :gutter="12" v-if="editForm.record_type === '外纪'">
          <el-col :span="12">
            <el-form-item label="儿子"><el-select v-model="editForm.son_id" filterable clearable placeholder="选择儿子（关键）" style="width:100%" @change="onSonChange"><el-option v-for="m in sonCandidates" :key="m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel(m)}）`" :value="m.id" /></el-select>
              <div v-if="editForm.son_id" style="margin-top:4px;color:#909399;font-size:12px">{{ sonPositionText }}</div></el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="籍贯"><el-input v-model="editForm.native_place" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="现居地"><el-input v-model="editForm.current_place" placeholder="如：浙江省杭州市西湖区" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="手机"><el-input v-model="editForm.phone" placeholder="手机号码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="单倍体"><el-input v-model="editForm.dna_haploid" placeholder="单倍体DNA编码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="二倍体（STR）"><el-input v-model="editForm.dna_diploid" type="textarea" :rows="3" placeholder="粘贴 STR 位点串：位点名/数1/数2，用 - 分隔（如 D8S1179/12/14-D21S11/29/31-…）" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="线粒体"><el-input v-model="editForm.dna_mito" placeholder="线粒体DNA编码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="职业"><el-input v-model="editForm.occupation" placeholder="如：教师 / 医生" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="学历"><el-input v-model="editForm.education" placeholder="如：本科 / 举人" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="功名/官职"><el-input v-model="editForm.title" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="出嫁信息"><el-input v-model="editForm.married_out" placeholder="适 XXX" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="葬地"><el-input v-model="editForm.burial_place" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="saveMember">保存</el-button>
      </template>
    </el-dialog>

    <!-- 生僻字查字面板 -->
    <HanziPicker v-model="hanziVisible" @pick="onHanziPick" />


<AccountPwDialog v-model:visible="acctPw.visible" :name="acctPw.name" :password="acctPw.password" />
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { normalizeDeathFields } from '../utils/dateFormat'
import { canViewDNA } from '../utils/auth'
import HanziPicker from './HanziPicker.vue'
import AccountPwDialog from './AccountPwDialog.vue'

// 成员编辑弹窗（全站唯一）：成员管理列表 与 成员资料页 都用这一个，
// 避免两处字段/校验不一致。2026-11-07 属主要求合并。
const props = defineProps({
  modelValue: { type: Boolean, default: false },
  member: { type: Object, default: null },     // 传 null = 新增
  members: { type: Array, default: () => [] }, // 父母/子女下拉的候选
  branches: { type: Array, default: () => [] },
  familySurname: { type: String, default: '' }
})
const emit = defineEmits(['update:modelValue', 'saved'])
const acctPw = ref({ visible: false, name: '', password: '' })
watch(() => props.modelValue, (v) => { if (v) openEdit() })

const editForm = reactive({})

// 生僻字查字面板
const hanziVisible = ref(false)
const hanziTarget = ref('given_name')
function openHanzi(field) {
  hanziTarget.value = field
  hanziVisible.value = true
}
function onHanziPick(ch) {
  const k = hanziTarget.value
  if (!k) return
  editForm[k] = (editForm[k] || '') + ch
}
const originalBranchId = ref(0)  // 打开编辑时的原房支，用于判断房支是否变更
const clanTypes = ['本生', '嗣子', '兼祧', '入赘', '养子', '出继外姓']
const fatherCandidates = computed(() => {
  if (editForm.record_type === '外纪') {
    return props.members || [].filter(m => m.gender === 1)
  }
  return props.members || [].filter(m => m.gender === 1 && m.generation === editForm.generation - 1)
})

const motherCandidates = computed(() => {
  if (editForm.record_type === '外纪') return props.members || [].filter(m => m.gender === 2)
  return props.members || [].filter(m => m.gender === 2 && m.generation === editForm.generation - 1)
})

function genLabel(m) {
  if (m.record_type === '外纪') {
    const waiji = props.members || [].filter(x => x.record_type === '外纪')
    if (waiji.length > 0) {
      const minGen = Math.min(...waiji.map(x => x.generation))
      return `外纪${m.generation - minGen + 1}世`
    }
    return `外纪${m.generation + 1}世`
  }
  return `第${m.generation}世`
}

const waijiGenLabel = computed(() => {
  if (!editForm.generation) return '—'
  const waiji = props.members || [].filter(x => x.record_type === '外纪')
  if (waiji.length === 0) return `外纪${editForm.generation + 1}世`
  const base = Math.min(...waiji.map(x => x.generation))
  return `外纪${editForm.generation - base + 1}世`
})

const sonCandidates = computed(() => {
  return props.members || [].filter(m => m.id !== editForm.id)
})

const sonPositionText = computed(() => {
  if (!editForm.son_id) return ''
  const son = props.members || [].find(m => m.id === editForm.son_id)
  if (!son) return ''
  return `保存后 ${son.name || (son.surname||'')+(son.given_name||'')} 的生父将自动设为当前成员`
})

function onFatherChange(fid) {
  if (!fid) {
    if (editForm.record_type === '外纪') {
      editForm.generation = 0
    }
    return
  }
  const father = props.members || [].find(m => m.id === fid)
  // 选了生父：拉他的配偶作为生母候选，并默认选正配
  loadEditMotherCandidates().then(() => {
    if (!editForm.mother_spouse_id) {
      const main = editMotherCandidates.value.find(x => x.type === '正配') || editMotherCandidates.value[0]
      if (main) editForm.mother_spouse_id = main.id
    }
  })
  if (father) {
    editForm.generation = father.generation + 1
    // 自动继承父亲的房支：未手动选择房支时，跟随父亲
    if ((!editForm.branch_id || editForm.branch_id === 0) && father.branch_id) {
      editForm.branch_id = father.branch_id
    }
  }
  loadEditMotherCandidates()
}

function onSonChange(sid) {
  if (!sid) {
    if (editForm.record_type === '外纪') {
      editForm.generation = 0
    }
    return
  }
  const son = props.members || [].find(m => m.id === sid)
  if (son) {
    editForm.generation = son.generation - 1
  }
}

function onRecordTypeChange(val) {
  if (val === '外纪') {
    editForm.father_id = 0
    editForm.son_id = 0
    editForm.generation = 0
  } else {
    editForm.father_id = 0
    editForm.son_id = 0
    editForm.generation = null
  }
}
const motherSpouseCandidates = ref([]) // 当前成员的配偶（生母候选人）
const fatherSpouses = ref([])           // 父亲的配偶（用于显示生母名称）
const editMotherCandidates = ref([])    // 编辑时生母下拉选项
const fatherSpouseCache = ref({})
async function loadFatherSpouses(fid) {
  if (!fid) return []
  if (!fatherSpouseCache.value[fid]) {
    try {
      const { data } = await api.get(`/members/${fid}/spouses`)
      fatherSpouseCache.value[fid] = data || []
    } catch (e) {
      fatherSpouseCache.value[fid] = []
    }
  }
  return fatherSpouseCache.value[fid]
}
async function onAdoptFatherChange(a) {
  a.mother_id = null
  const list = await loadFatherSpouses(a.father_id)
  const main = list.find(x => x.type === '正配') || list[0]
  if (main) a.mother_id = main.id
  syncClanTypeByAdoption()
}


// 嗣父/养父选了人 → 宗法自动跟着变（有嗣父＝嗣子，有养父＝养子）
function syncClanTypeByAdoption() {
  const rows = editForm.adoptions || []
  const hasFoster = rows.some(a => a.kind === '嗣父' && a.father_id)
  const hasRaise = rows.some(a => a.kind === '养父' && a.father_id)
  if (hasFoster) editForm.clan_type = '嗣子'
  else if (hasRaise) editForm.clan_type = '养子'
  // 行删光了：如果当前宗法是自动设上去的（嗣子/养子），回退成「本生」
  else if (editForm.clan_type === '嗣子' || editForm.clan_type === '养子') editForm.clan_type = '本生'
}
function removeAdoption(i) {
  editForm.adoptions.splice(i, 1)
  syncClanTypeByAdoption()
}

function addAdoption() {
  if (!Array.isArray(editForm.adoptions)) editForm.adoptions = []
  editForm.adoptions.push({ father_id: null, mother_id: null, father_text: '', kind: '嗣父', note: '' })
}
// 过继/收养说明气泡：点「?」打开/关闭；点页面其它地方也会关闭
const adoptTipVisible = ref(false)
const adoptTipHtml = '<div>每行是「父母」一对：<b>生父/生母</b>、<b>嗣父/嗣母</b>、<b>养父/养母</b>。父与母都从本族成员里选。</div><div style="margin-top:6px"><b>外族人是生父</b>：生父不在本谱时，把姓名填在生父框右边的输入框里。</div><div style="margin-top:6px">兼祧＝加多条「嗣父」；生父在本谱＝出继（生父侧）/ 入继（嗣父侧）；生父在谱外＝收养。</div><div style="margin-top:6px;color:#999">点问号打开；再点问号或点页面其它地方关闭。</div>'
const adoptQRef = ref(null)
function toggleAdoptTip() {
  adoptTipVisible.value = !adoptTipVisible.value
}
function onDocClickForTip(e) {
  if (!adoptTipVisible.value) return
  const el = adoptQRef.value
  if (el && (el === e.target || el.contains(e.target))) return
  adoptTipVisible.value = false
}
onMounted(() => document.addEventListener('click', onDocClickForTip))
onBeforeUnmount(() => document.removeEventListener('click', onDocClickForTip))


function openEdit() {
  const copy = JSON.parse(JSON.stringify(props.member || {}))
  // 先清空：birth_date 是纯 UI 字段（接口不返回），不清会把上一个成员的出生日期带进来
  Object.keys(editForm).forEach(k => delete editForm[k])
  Object.assign(editForm, copy)
  originalBranchId.value = (props.member || {}).branch_id || 0
  if (!editForm.surname) editForm.surname = props.familySurname || ''
  // 初始化出生日期字段（只用本成员自己的生年）
  editForm.birth_calendar = editForm.birth_calendar || 'solar'
  editForm.birth_date = editForm.birth_gregorian || ''
  // 已故（卒年不详）：把「已故」回显到卒年框，避免显示成「留空=在世」
  {
    const w = String(editForm.death_era || '').trim()
    if (/已逝|已故|逝世|已卒|殁|病故|亡故|去世/.test(w)) editForm.death_gregorian = w
  }
  // 过继/收养关系（嗣父/养父，可多条）
  editForm.adoptions = JSON.parse(JSON.stringify((props.member || {}).adoptions || []))
  // 0 不是有效 id：转成 null，选择框才会显示灰色「父 / 母」占位
  editForm.father_id = editForm.father_id || null
  editForm.mother_id = editForm.mother_id || null
  editForm.mother_spouse_id = editForm.mother_spouse_id || null
  editForm.adoptions.forEach(a => { a.father_id = a.father_id || null; a.mother_id = a.mother_id || null })
  emit('update:modelValue', true)
  // 预加载编辑时的生母候选人
  loadEditMotherCandidates()
}

async function loadEditMotherCandidates() {
  if (!editForm.father_id) { editMotherCandidates.value = []; return }
  try {
    const { data } = await api.get(`/members/${editForm.father_id}/spouses`)
    editMotherCandidates.value = data || []
  } catch (e) { editMotherCandidates.value = [] }
}

// 统计某成员的后代数量（沿 father_id 遍历）
function countDescendants(memberId) {
  let count = 0
  const queue = [memberId]
  const visited = new Set()
  while (queue.length) {
    const pid = queue.shift()
    for (const m of props.members || []) {
      if (m.father_id === pid && !visited.has(m.id)) {
        visited.add(m.id)
        count++
        queue.push(m.id)
      }
    }
  }
  return count
}

async function saveMember() {
  await convertDate()  // 确保转换已执行
  const payload = { ...editForm }
  // 选择框空着是 null，提交前统一回 0
  ;['father_id', 'mother_id', 'mother_spouse_id', 'son_id'].forEach(k => { if (!payload[k]) payload[k] = 0 })
  if (Array.isArray(payload.adoptions)) {
    payload.adoptions = payload.adoptions.map(a => ({ ...a, father_id: a.father_id || 0, mother_id: a.mother_id || 0 }))
  }
  payload.name = (payload.surname || '') + (payload.given_name || '')
  // 房支规范化：未选择时清空 branch_id 与 branch 文本（避免旧文本被后端重新匹配）
  payload.branch_id = payload.branch_id || 0
  if (!payload.branch_id) payload.branch = ''
  // 卒年规范化：留空=在世；"已逝"等标记转存 death_era
  normalizeDeathFields(payload)
  // birth_gregorian 已由 convertDate 设置，不再从 birth_date 覆盖
  delete payload.birth_date
  delete payload.birth_calendar
  // 编辑已有成员且房支变更：若有后代，确认是否一起归入新分支
  if (editForm.id && (payload.branch_id || 0) !== (originalBranchId.value || 0)) {
    const desc = countDescendants(editForm.id)
    if (desc > 0) {
      const bname = props.branches || [].find(b => b.id === payload.branch_id)?.name || '未分房'
      try {
        await ElMessageBox.confirm(
          `该成员有 ${desc} 个后代，将一起归入「${bname}」，是否继续？`,
          '房支变更',
          { type: 'warning', confirmButtonText: '继续', cancelButtonText: '取消' }
        )
      } catch { return }
    }
  }
  let r
  try {
    if (editForm.id) { r = await api.put(`/members/${editForm.id}`, payload) }
    else { r = await api.post('/members', payload) }
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '保存失败'
    ElMessage.error('保存失败：' + msg)
    return
  }
  if (r.data && r.data.draft) { emit('update:modelValue', false); ElMessage.success(r.data.message || '已提交审核'); return }
  emit('update:modelValue', false)
  if (r?.data?.account_password) {
    acctPw.value = { visible: true, name: r.data.name || editForm.name || '', password: r.data.account_password }
  } else {
    ElMessage.success('已保存')
  }
  emit('saved')
}

async function convertDate() {
  const d = (editForm.birth_date || '').trim()
  if (!d) {
    // 用户清空了出生日期 → 清空所有出生相关字段
    editForm.birth_gregorian = ''
    editForm.birth_lunar = ''
    editForm.birth_ganzhi = ''
    editForm.birth_era = ''
    return
  }
  // 只填年份（YYYY）→ 直接存年份，不转换月日
  const mYear = d.match(/^(\d{4})$/)
  if (mYear) {
    editForm.birth_gregorian = mYear[1]
    editForm.birth_lunar = ''
    editForm.birth_ganzhi = ''
    editForm.birth_era = ''
    return
  }
  const m = d.match(/^(\d{4})-(\d{2})-(\d{2})$/); if (!m) return
  if (editForm.birth_calendar === 'lunar') {
    // 农历 → 公历
    const { data } = await api.get(`/lunar/to-solar?y=${m[1]}&m=${m[2]}&d=${m[3]}`)
    editForm.birth_gregorian = data.gregorian
    editForm.birth_lunar = data.lunar
    editForm.birth_ganzhi = data.ganzhi
    editForm.birth_era = data.era + data.eraYear
    ElMessage.success(`公历：${data.gregorian}，${data.lunar}`)
  } else {
    // 公历 → 农历
    const { data } = await api.get(`/lunar/convert?y=${m[1]}&m=${m[2]}&d=${m[3]}`)
    editForm.birth_gregorian = d
    editForm.birth_lunar = data.lunar
    editForm.birth_ganzhi = data.ganzhi
    editForm.birth_era = data.era + data.eraYear
    ElMessage.success(`农历：${data.lunar}`)
  }
}
</script>
