<template>
  <div>
    <div class="toolbar">
      <el-input v-model="kw" placeholder="搜索姓名/字辈/籍贯" clearable class="search-input" />
      <el-select :model-value="branchCtx.current" clearable placeholder="按房支筛选" class="branch-filter" @change="onBranchChange">
        <el-option :value="0" label="宗谱（全部）" />
        <el-option v-for="b in flatBranchOptions" :key="b.id" :label="b.label" :value="b.id" />
      </el-select>
      <el-checkbox v-model="branchCtx.includeChildren">含下级</el-checkbox>
      <el-button type="primary" @click="load">查询</el-button>
      <el-button v-if="canRollback()" @click="openExport">导出</el-button>
      <el-button v-if="canRollback()" @click="importVisible = true">导入</el-button>
      <el-button v-if="canEdit()" @click="openAdd">新增成员</el-button>
      <el-button v-if="canViewDNA()" @click="openExternalDNAMatch">DNA 认亲比对</el-button>
      <el-button @click="load" :icon="'Refresh'">刷新</el-button>
    </div>

    <div class="table-responsive">
    <el-table :data="shownMembers" border stripe style="width: 100%">
      <el-table-column label="头像" width="60">
        <template #default="s">
          <el-avatar v-if="s.row.portrait" :src="s.row.portrait" :size="36" shape="square" />
          <el-avatar v-else :size="36" shape="square" :style="{ background: '#d9cbb6', color: '#8c5a3c', fontSize: '14px' }">
            {{ (s.row.name || s.row.surname || '').charAt(0) }}
          </el-avatar>
        </template>
      </el-table-column>
      <el-table-column label="姓名" width="100">
        <template #default="s">{{ s.row.name || (s.row.surname || '') + (s.row.given_name || '') }}</template>
      </el-table-column>
      <el-table-column prop="generation" label="世代" width="85">
        <template #default="s">{{ genLabel2(s.row) }}</template>
      </el-table-column>
      <el-table-column prop="generation_word" label="字辈" width="70" />
      <el-table-column prop="zi" label="字" width="80" />
      <el-table-column prop="hao" label="号" width="80" />
      <el-table-column label="房支" width="100">
        <template #default="s">{{ branchName(s.row.branch_id) || s.row.branch || '—' }}</template>
      </el-table-column>
      <el-table-column label="性别" width="60">
        <template #default="s">{{ s.row.gender === 1 ? '男' : '女' }}</template>
      </el-table-column>
      <el-table-column prop="clan_type" label="宗法" width="70" />
      <el-table-column prop="record_type" label="类型" width="70" />
      <el-table-column label="父母" width="140">
        <template #default="s">
          <span v-if="s.row.father_id" class="parent-link" @click="goMember(s.row.father_id)">父:{{ getMemberName(s.row.father_id) }}</span>
          <span v-if="s.row.father_id && s.row.mother_id"> / </span>
          <span v-if="s.row.mother_id" class="parent-link" @click="goMember(s.row.mother_id)">母:{{ getMemberName(s.row.mother_id) }}</span>
          <span v-if="!s.row.father_id && !s.row.mother_id">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="birth_gregorian" label="生年" width="110" />
      <el-table-column prop="death_gregorian" label="卒年" width="110">
        <template #default="s">{{ s.row.death_gregorian || '在世' }}</template>
      </el-table-column>
      <el-table-column prop="native_place" label="籍贯" />
      <el-table-column prop="current_place" label="现居地" />
      <el-table-column label="操作" width="150" fixed="right" v-if="canEdit()">
        <template #default="s">
          <el-button size="small" @click="goMember(s.row.id)">详情</el-button>
          <el-button size="small" type="primary" @click="openEdit(s.row)">编辑</el-button>
          <el-button size="small" type="danger" @click="del(s.row)">删除</el-button>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="80" fixed="right" v-else>
        <template #default="s">
          <el-button size="small" @click="goMember(s.row.id)">详情</el-button>
        </template>
      </el-table-column>
    </el-table>
    </div>

    <el-dialog v-model="editVisible" :title="editForm.id ? '编辑成员' : '新增成员'" width="700px">
      <el-form :model="editForm" label-width="80px">
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
          <el-col :span="8"><el-form-item label="字辈"><div class="hz-inline"><el-input v-model="editForm.generation_word" /><el-button size="small" text type="primary" @click="openHanzi('generation_word')">生僻字</el-button></div></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="宗法"><el-select v-model="editForm.clan_type"><el-option v-for="t in clanTypes" :key="t" :label="t" :value="t" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="类型"><el-select v-model="editForm.record_type" @change="onRecordTypeChange"><el-option label="内纪" value="内纪" /><el-option label="外纪" value="外纪" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="排行"><el-input v-model="editForm.rank" placeholder="行三" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="生父">
              <el-select v-model="editForm.father_id" filterable clearable placeholder="选择生父" style="width:100%" @change="onFatherChange">
                <el-option v-for="m in fatherCandidates" :key="m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel2(m)}）`" :value="m.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12" v-if="editForm.record_type === '外纪'">
            <el-form-item label="儿子">
              <el-select v-model="editForm.son_id" filterable clearable placeholder="选择儿子（关键）" style="width:100%" @change="onSonChange">
                <el-option v-for="m in sonCandidates" :key="m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel2(m)}）`" :value="m.id" />
              </el-select>
              <div v-if="editForm.son_id" style="margin-top:4px;color:#909399;font-size:12px">{{ sonPositionText }}</div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12" v-if="editForm.record_type !== '外纪'">
          <el-col :span="12">
            <el-form-item label="生母">
              <el-select v-model="editForm.mother_id" filterable clearable placeholder="选择生母" style="width:100%">
                <el-option v-for="m in motherCandidates" :key="m.id" :label="`${m.name}（第${m.generation}世）`" :value="m.id" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="房支"><el-select v-model="editForm.branch_id" clearable filterable placeholder="选择房支" style="width:100%"><el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.id" /></el-select></el-form-item></el-col>
          <el-col :span="16">
            <el-form-item label="生年">
              <el-row :gutter="4" style="width:100%">
                <el-col :span="8"><el-select v-model="editForm.birth_calendar" size="small" @change="onCalendarChange"><el-option label="公历" value="solar" /><el-option label="农历" value="lunar" /></el-select></el-col>
                <el-col :span="16"><el-input v-model="editForm.birth_date" placeholder="1900-08-01" @blur="convertDate" /></el-col>
              </el-row>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="卒年"><el-input v-model="editForm.death_gregorian" placeholder="留空表示在世" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="籍贯"><el-input v-model="editForm.native_place" /></el-form-item>
        <el-form-item label="现居地"><el-input v-model="editForm.current_place" placeholder="如：浙江省杭州市西湖区（热力图按此统计）" /></el-form-item>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="职业"><el-input v-model="editForm.occupation" placeholder="如：教师 / 医生" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="学历"><el-input v-model="editForm.education" placeholder="如：本科 / 举人" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="单倍体"><el-input v-model="editForm.dna_haploid" placeholder="单倍体DNA编码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="二倍体"><el-input v-model="editForm.dna_diploid" placeholder="二倍体DNA编码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="线粒体"><el-input v-model="editForm.dna_mito" placeholder="线粒体DNA编码" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveMember">保存</el-button>
      </template>
    </el-dialog>

    <!-- 生僻字查字面板 -->
    <HanziPicker v-model="hanziVisible" @pick="onHanziPick" />

    <!-- 新增成员后自动建号：初始密码弹窗 -->
    <AccountPwDialog v-model:visible="acctPw.visible" :name="acctPw.name" :password="acctPw.password" />

    <!-- DNA 认亲比对：手动输入外部编码，与族内已录入 DNA 的成员比对 -->
    <el-dialog v-model="extDnaVisible" title="DNA 认亲比对" width="720px">
      <el-form label-width="110px">
        <el-form-item label="父系（Y-DNA）">
          <el-input v-model="extDnaForm.haploid" placeholder="如 O-M175（父系单倍群编码）" clearable />
        </el-form-item>
        <el-form-item label="母系（线粒体）">
          <el-input v-model="extDnaForm.mito" placeholder="如 M7（母系线粒体编码）" clearable />
        </el-form-item>
        <el-form-item label="二倍体（STR）">
          <el-input v-model="extDnaForm.diploid" type="textarea" :rows="3" placeholder="粘贴 STR 位点串：位点名/数1/数2，用 - 分隔（如 D8S1179/12/14-D21S11/29/31）" />
        </el-form-item>
      </el-form>
      <div style="color:#909399;font-size:12px;margin-bottom:12px">用于外地人 / 被拐卖者回来认亲：手动输入检测报告编码，直接与族内已录入 DNA 的成员比对（不依赖任何成员的已填字段）。</div>

      <div v-if="extDnaLoading" style="text-align:center;padding:24px">比对中…</div>
      <div v-else-if="extDnaSearched">
        <div v-if="extDnaMatches.length" style="margin-bottom:8px;color:#666">共匹配 {{ extDnaTotal }} 项，按亲缘可能性排序：</div>
        <el-table :data="extDnaMatches" v-if="extDnaMatches.length" border>
          <el-table-column prop="name" label="成员" min-width="90" />
          <el-table-column prop="relation" label="关系" width="110" />
          <el-table-column prop="match_type" label="匹配方式" width="100">
            <template #default="s">{{ s.row.match_type === 'exact' ? '完全一致' : (s.row.match_type === 'lineage' ? '同支系' : '前缀同支') }}</template>
          </el-table-column>
          <el-table-column prop="code" label="DNA 编码" min-width="140" />
          <el-table-column prop="generation" label="世代" width="80">
            <template #default="s">{{ genLabel2(s.row) }}</template>
          </el-table-column>
          <el-table-column prop="score" label="评分" width="60" />
          <el-table-column label="操作" width="80" fixed="right">
            <template #default="s"><el-button size="small" text type="primary" @click="goMember(s.row.member_id)">详情</el-button></template>
          </el-table-column>
        </el-table>
        <div v-else style="color:#999;text-align:center;padding:16px">未找到 DNA 匹配的族人</div>
      </div>
      <template #footer>
        <el-button @click="extDnaVisible = false">关闭</el-button>
        <el-button type="primary" :loading="extDnaLoading" @click="runExternalDNAMatch">开始比对</el-button>
      </template>
    </el-dialog>

    <!-- 导出成员档案（Excel / CSV） -->
    <el-dialog v-model="exportVisible" title="导出成员档案" width="580px">
      <el-form label-width="92px">
        <el-form-item label="导出范围">
          <el-radio-group v-model="exportForm.scope" class="export-scope">
            <el-radio value="branch">当前房支：{{ currentBranchLabel }}</el-radio>
            <el-radio value="filter">当前筛选（房支 + 关键词「{{ kw || '无' }}」）</el-radio>
            <el-radio value="all">全部成员（不受房支限制）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="文件格式">
          <el-radio-group v-model="exportForm.format">
            <el-radio value="xlsx">Excel（.xlsx）</el-radio>
            <el-radio value="csv">CSV（.csv，UTF-8 带 BOM）</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <el-alert type="info" :closable="false" show-icon>
        <template #title>导出说明</template>
        <ul class="export-tips">
          <li>列名为中文、带表头，包含成员全部档案字段（按当前账号权限隐藏无权字段）。</li>
          <li>公历日期统一为 <b>YYYY-MM-DD</b>（只知年份时写 YYYY）；农历日期在「出生/去世日期(农历)」列单独标注。</li>
          <li>本次预计导出 <b>{{ exportCount }}</b> 人；导出的文件可直接修改后再用「导入」回填。</li>
        </ul>
      </el-alert>
      <template #footer>
        <el-button @click="downloadTemplate">下载导入模板</el-button>
        <el-button @click="exportVisible = false">取消</el-button>
        <el-button type="primary" :loading="exporting" @click="doExport">导出</el-button>
      </template>
    </el-dialog>

    <!-- 导入成员档案（预览校验 → 事务写入） -->
    <MemberImportWizard v-model="importVisible" @imported="onImported" />
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { aiValidateSave, confirmSaveProblems, buildMemberText, localMemberProblems, mergeProblems } from '../utils/aiCheck'
import { canEdit, canViewDNA, canRollback } from '../utils/auth'
import { saveBlob, filenameFromDisposition } from '../utils/download'
import MemberImportWizard from '../components/MemberImportWizard.vue'
import { normalizeDeathFields } from '../utils/dateFormat'
import HanziPicker from '../components/HanziPicker.vue'
import AccountPwDialog from '../components/AccountPwDialog.vue'
import { branchCtx, branchParams, setCurrent, loadBranches, buildFlatOptions } from '../utils/branchContext'

const router = useRouter()
const members = ref([])
const familySurname = ref('')
const branches = ref([])
const flatBranchOptions = computed(() => buildFlatOptions())
// 关键词过滤（姓名/字/号/字辈/籍贯/现居地/曾用名）；「当前筛选」导出用的就是这份结果
const shownMembers = computed(() => {
  const q = (kw.value || '').trim()
  if (!q) return members.value
  return members.value.filter(m => [
    m.name, m.zi, m.hao, m.generation_word, m.native_place, m.current_place, m.former_name
  ].some(v => String(v || '').includes(q)))
})
function onBranchChange(v) { setCurrent(v == null ? 0 : v) }
const acctPw = ref({ visible: false, name: '', password: '' })
const branchFilter = ref(null)
const kw = ref('')
const editVisible = ref(false)
const editForm = reactive({})
const saving = ref(false)
const clanTypes = ['本生', '嗣子', '兼祧', '入赘', '养子']

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

const fatherCandidates = computed(() => {
  if (editForm.record_type === '外纪') {
    return members.value.filter(m => m.gender === 1)
  }
  return members.value.filter(m => m.gender === 1 && m.generation === editForm.generation - 1)
})
const motherCandidates = computed(() => {
  if (editForm.record_type === '外纪') {
    return members.value.filter(m => m.gender === 2)
  }
  return members.value.filter(m => m.gender === 2 && m.generation === editForm.generation - 1)
})

function genLabel2(m) {
  if (m.record_type === '外纪') {
    const waiji = members.value.filter(x => x.record_type === '外纪')
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
  const waiji = members.value.filter(x => x.record_type === '外纪')
  if (waiji.length === 0) return `外纪${editForm.generation + 1}世`
  const base = Math.min(...waiji.map(x => x.generation))
  return `外纪${editForm.generation - base + 1}世`
})

const sonCandidates = computed(() => {
  // 外纪：儿子候选为所有成员（不限性别，因为女儿也可能是后代）
  return members.value.filter(m => m.id !== editForm.id)
})

const sonPositionText = computed(() => {
  if (!editForm.son_id) return ''
  const son = members.value.find(m => m.id === editForm.son_id)
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
  const father = members.value.find(m => m.id === fid)
  if (father) {
    editForm.generation = father.generation + 1
  }
}

function onSonChange(sid) {
  if (!sid) {
    if (editForm.record_type === '外纪') {
      editForm.generation = 0
    }
    return
  }
  const son = members.value.find(m => m.id === sid)
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

function getMemberName(id) {
  if (!id) return '—'
  const p = members.value.find(m => m.id === id)
  return p ? (p.name || p.surname + p.given_name) : `ID:${id}`
}

function goMember(id) {
  if (id) router.push(`/member/${id}`)
}

function branchName(id) {
  const b = branches.value.find(x => x.id === id)
  return b ? b.name : ''
}

async function load() {
  const { data } = await api.get('/members', { params: branchParams() })
  members.value = data
  try {
    const { data: fsData } = await api.get('/settings/family-surname')
    familySurname.value = fsData.family_surname || ''
  } catch (e) { /* ignore */ }
  try {
    const { data: bData } = await api.get('/branches')
    branches.value = bData || []
  } catch (e) { /* ignore */ }
}

function openAdd() {
  doOpenAdd()
}

function doOpenAdd() {
  Object.keys(editForm).forEach(k => delete editForm[k])
  editForm.surname = familySurname.value
  editForm.given_name = ''
  editForm.generation = null
  editForm.gender = 1
  editForm.record_type = '内纪'
  editForm.clan_type = '本生'
  editForm.father_id = 0
  editForm.mother_id = 0
  editForm.branch_id = 0
  editForm.birth_calendar = 'solar'
  editForm.birth_date = ''
  editForm.birth_gregorian = ''
  editForm.birth_lunar = ''
  editVisible.value = true
}

function openEdit(row) {
  Object.assign(editForm, JSON.parse(JSON.stringify(row)))
  if (!editForm.surname) editForm.surname = familySurname.value
  editForm.birth_calendar = editForm.birth_calendar || 'solar'
  editForm.birth_date = editForm.birth_date || editForm.birth_gregorian || ''
  editVisible.value = true
}

function onCalendarChange() {
  // 切换历法时清空已转换的日期，等待用户重新输入并触发 convertDate
  editForm.birth_date = ''
  editForm.birth_gregorian = ''
  editForm.birth_lunar = ''
  editForm.birth_ganzhi = ''
  editForm.birth_era = ''
}

async function convertDate() {
  const d = (editForm.birth_date || '').trim()
  if (!d) {
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

async function saveMember() {
  if (saving.value) return
  await convertDate()  // 确保转换已执行
  const payload = { ...editForm }
  payload.name = (payload.surname || '') + (payload.given_name || '')
  // 房支规范化：未选择时清空 branch_id 与 branch 文本（避免旧文本被后端重新匹配）
  payload.branch_id = payload.branch_id || 0
  if (!payload.branch_id) payload.branch = ''
  // 卒年规范化：留空=在世；"已逝"等标记转存 death_era
  normalizeDeathFields(payload)
  delete payload.birth_date

  // 新增成员：保存前校验（编辑不校验）
  if (!editForm.id) {
    saving.value = true
    try {
      const local = localMemberProblems(editForm, members.value)
      const ai = await aiValidateSave(buildMemberText(editForm))
      const problems = mergeProblems(local, ai)
      if (problems.length) {
        const goAhead = await confirmSaveProblems(problems)
        if (!goAhead) return // 去修改：留在弹窗，不保存
      }
    } finally {
      saving.value = false
    }
  }

  saving.value = true
  let r
  try {
    if (editForm.id) {
      r = await api.put(`/members/${editForm.id}`, payload)
    } else {
      r = await api.post('/members', payload)
    }
    if (r.data && r.data.draft) { editVisible.value = false; ElMessage.success(r.data.message || '已提交审核'); return }
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '保存失败'
    ElMessage.error('保存失败：' + msg)
    return
  } finally {
    saving.value = false
  }
  editVisible.value = false
  if (r?.data?.account_password) {
    acctPw.value = { visible: true, name: r.data.name || editForm.name || '', password: r.data.account_password }
  } else {
    ElMessage.success('已保存')
  }
  load()
}

async function del(row) {
  await ElMessageBox.confirm(`确定删除「${row.name}」？`, '提示', { type: 'warning' })
  let r
  try {
    r = await api.delete(`/members/${row.id}`)
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '删除失败'
    ElMessage.error('删除失败：' + msg)
    return
  }
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已删除')
  load()
}

// DNA 认亲比对（手动输入外部编码，全库匹配）
const extDnaVisible = ref(false)
const extDnaLoading = ref(false)
const extDnaSearched = ref(false)
const extDnaForm = reactive({ haploid: '', mito: '', diploid: '' })
const extDnaMatches = ref([])
const extDnaTotal = ref(0)

function openExternalDNAMatch() {
  extDnaForm.haploid = ''
  extDnaForm.mito = ''
  extDnaForm.diploid = ''
  extDnaMatches.value = []
  extDnaTotal.value = 0
  extDnaSearched.value = false
  extDnaVisible.value = true
}

async function runExternalDNAMatch() {
  const h = (extDnaForm.haploid || '').trim()
  const m = (extDnaForm.mito || '').trim()
  const d = (extDnaForm.diploid || '').trim()
  if (!h && !m && !d) { ElMessage.warning('请至少输入一项 DNA 编码'); return }
  extDnaLoading.value = true
  try {
    const { data } = await api.post('/members/dna-match-external', { haploid: h, mito: m, diploid: d })
    extDnaMatches.value = data.matches || []
    extDnaTotal.value = data.total || 0
    extDnaSearched.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '比对失败')
  } finally {
    extDnaLoading.value = false
  }
}

// ===== 导出成员档案 =====
const exportVisible = ref(false)
const exporting = ref(false)
const exportForm = reactive({ scope: 'branch', format: 'xlsx' })
const importVisible = ref(false)
const currentBranchLabel = computed(() => {
  if (!branchCtx.current) return '宗谱（全部）'
  const hit = flatBranchOptions.value.find(o => o.id === branchCtx.current)
  return hit ? hit.label : '当前房支'
})
const exportCount = computed(() => {
  if (exportForm.scope === 'all') return members.value.length
  if (exportForm.scope === 'filter') return shownMembers.value.length
  return members.value.length
})

function openExport() {
  exportForm.scope = 'branch'
  exportForm.format = 'xlsx'
  exportVisible.value = true
}

async function doExport() {
  exporting.value = true
  try {
    const params = { format: exportForm.format, scope: exportForm.scope === 'all' ? 'all' : 'branch' }
    if (exportForm.scope === 'filter') params.kw = (kw.value || '').trim()
    const { data, headers } = await api.get('/members/export', {
      params,
      responseType: 'blob',
      timeout: 180000
    })
    const fallback = `成员档案-${new Date().toISOString().slice(0, 10)}.${exportForm.format}`
    saveBlob(data, filenameFromDisposition(headers['content-disposition'], fallback))
    ElMessage.success(`已导出 ${exportForm.format.toUpperCase()} 文件`)
    exportVisible.value = false
  } catch (e) {
    ElMessage.error('导出失败：' + (e?.response?.data?.error || e.message || '未知错误'))
  } finally {
    exporting.value = false
  }
}

async function downloadTemplate() {
  try {
    const { data, headers } = await api.get('/members/import/template', {
      params: { format: exportForm.format },
      responseType: 'blob',
      timeout: 60000
    })
    const fallback = exportForm.format === 'csv' ? '成员导入模板.csv' : '成员导入模板.xlsx'
    saveBlob(data, filenameFromDisposition(headers['content-disposition'], fallback))
  } catch (e) {
    ElMessage.error('下载模板失败：' + (e.message || ''))
  }
}

function onImported() {
  load()
}

onMounted(() => { loadBranches(); load() })
</script>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.parent-link { color: #3a3226; cursor: pointer; text-decoration: underline; text-decoration-style: dotted; font-size: 12px; }
.parent-link:hover { color: #8c5a3c; }
.table-responsive { overflow-x: auto; -webkit-overflow-scrolling: touch; }
.search-input { max-width: 240px; width: 100%; }
.branch-filter { width: 160px; }
.hz-inline { display: flex; align-items: center; gap: 4px; width: 100%; }
.export-scope { display: flex; flex-direction: column; align-items: flex-start; gap: 6px; }
.export-tips { margin: 4px 0 0; padding-left: 18px; line-height: 1.8; font-size: 13px; }
</style>