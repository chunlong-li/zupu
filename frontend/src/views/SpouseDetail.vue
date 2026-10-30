<template>
  <div class="spouse-detail">
    <el-button @click="$router.back()" style="margin-bottom: 16px">← 返回</el-button>

    <el-row :gutter="16">
      <!-- 左侧主体 -->
      <el-col :xs="24" :md="16">
        <el-card class="section">
          <template #header>
            <div class="card-header">
              <div class="header-left">
                <el-avatar v-if="spouse.portrait" :src="thumbUrl(spouse.portrait, 120)" :size="48" shape="square" class="avatar" />
                <span class="serif">{{ spouse.name || (spouse.surname || '') + (spouse.given_name || '') }}</span>
                <el-tag size="small" :type="spouse.type === '正配' ? 'danger' : spouse.type === '继配' ? 'warning' : 'info'">{{ spouse.type }}</el-tag>
              </div>
              <el-button v-if="canEdit()" type="primary" size="small" @click="openEdit">编辑</el-button>
              <el-button size="small" text @click="openVersions">修改历史</el-button>
            </div>
          </template>

          <div class="info-grid">
            <div class="info-item"><span class="label">所属</span><span class="clickable" @click="goMember">{{ memberName }}</span></div>
            <div class="info-item"><span class="label">类型</span><span>{{ spouse.type }}</span></div>
            <div class="info-item"><span class="label">家世</span><span>{{ spouse.family || '—' }}</span></div>
            <div class="info-item"><span class="label">生卒</span><span>{{ formatLifeDates(spouse) }}</span></div>
            <div class="info-item" v-if="calcAges()"><span class="label">年龄</span><span>{{ calcAges() }}</span></div>
            <div class="info-item" v-if="spouse.occupation"><span class="label">职业</span><span>{{ spouse.occupation }}</span></div>
            <div class="info-item" v-if="spouse.education"><span class="label">学历</span><span>{{ spouse.education }}</span></div>
            <div class="info-item"><span class="label">手机</span><span>{{ spouse.phone || '—' }}</span></div>
            <div class="info-item" v-if="spouse.dna_haploid"><span class="label">单倍体</span><span>{{ spouse.dna_haploid }}</span></div>
            <div class="info-item" v-if="spouse.dna_diploid"><span class="label">二倍体</span><span>{{ spouse.dna_diploid }}</span></div>
            <div class="info-item" v-if="spouse.dna_mito"><span class="label">线粒体</span><span>{{ spouse.dna_mito }}</span></div>
            <div class="info-item"><span class="label">备注</span><span>{{ spouse.remark || '—' }}</span></div>
          </div>

          <div class="bio">
            <div class="bio-head">
              <h4>生平</h4>
              <div class="bio-head-actions">
                <el-button size="small" type="primary" text @click="goBiography()">{{ bioCount ? `查看传记（${bioCount}）` : '为此人立传' }}</el-button>
                <el-button v-if="canEdit()" size="small" type="primary" text @click="openBioDialog()">+ 新增条目</el-button>
              </div>
            </div>
            <div class="bio-entry" v-for="e in bioEntries" :key="e.id">
              <div class="bio-entry-head">
                <span class="bio-title serif">{{ e.title }}</span>
                <span v-if="canEdit()" class="bio-entry-actions">
                  <el-button size="small" text type="primary" @click="openBioDialog(e)">编辑</el-button>
                  <el-button size="small" text type="danger" @click="deleteBioEntry(e.id)">删除</el-button>
                </span>
              </div>
              <p class="bio-content">{{ e.content }}</p>
            </div>
            <el-empty v-if="!bioEntries.length" description="暂无生平条目" :image-size="40" />
          </div>
        </el-card>

        <!-- 多媒体 -->
        <el-card class="section">
          <template #header>
            <div class="card-header">
              <span>多媒体资料（{{ attachments.length }}）</span>
              <el-upload
                v-if="canEdit()"
                :action="`/api/upload`"
                :data="uploadData"
                :headers="uploadHeaders"
                :show-file-list="false"
                :on-success="onUploadSuccess"
                accept="image/*,video/*,audio/*"
                style="display: inline-block"
              >
                <el-button type="primary" size="small">上传多媒体</el-button>
              </el-upload>
            </div>
          </template>
          <el-radio-group v-model="mediaCategory" size="small" style="margin-bottom: 12px;">
            <el-radio-button value="全部">全部</el-radio-button>
            <el-radio-button value="照片">照片</el-radio-button>
            <el-radio-button value="视频">视频</el-radio-button>
            <el-radio-button value="音频">音频</el-radio-button>
          </el-radio-group>
          <div class="media-grid" v-if="filteredAttachments.length">
            <div class="media-item" v-for="a in filteredAttachments" :key="a.id" @click="openMediaViewer(a)" style="cursor:pointer">
              <div class="media-thumb" v-if="isVideo(a)">
                <el-image :src="thumbUrl(a.path, 400)" fit="cover" class="media-img" />
                <div class="play-overlay"><span class="play-icon">▶</span></div>
              </div>
              <el-image v-else-if="isPhoto(a)" :src="thumbUrl(a.path, 400)" fit="cover" class="media-img" :preview-src-list="[a.path]" />
              <audio v-else-if="isAudio(a)" :src="a.path" controls class="media-audio"></audio>
              <el-image v-else :src="thumbUrl(a.path, 400)" fit="cover" class="media-img" />
              <div class="media-actions" v-if="canEdit()">
                <el-button v-if="isPhoto(a)" size="small" text @click="setAsPortrait(a.path)">设头像</el-button>
                <el-button size="small" text type="danger" @click="deleteAttachment(a.id)">删除</el-button>
              </div>
            </div>
          </div>
          <el-empty v-else description="暂无多媒体" :image-size="40" />
        </el-card>
      </el-col>

      <!-- 右侧：子女 -->
      <el-col :xs="24" :md="8">
        <el-card class="section">
          <template #header><span>子女（{{ children.length }}）</span></template>
          <div class="child-item" v-for="ch in children" :key="ch.id">
            <router-link :to="`/member/${ch.id}`" class="child-link">{{ ch.name || (ch.surname || '') + (ch.given_name || '') }}</router-link>
            <span class="child-sub">{{ ch.gender === 1 ? '男' : '女' }} · 第{{ ch.generation }}世</span>
          </div>
          <el-empty v-if="!children.length" description="暂无子女" :image-size="40" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="editVisible" title="编辑配偶" width="500px">
      <el-form :model="editForm" label-width="80px">
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="姓"><div class="hz-inline"><el-input v-model="editForm.surname" placeholder="配偶姓氏" /><el-button size="small" text type="primary" @click="openHanzi('surname')">生僻字</el-button></div></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="名"><div class="hz-inline"><el-input v-model="editForm.given_name" placeholder="配偶名" /><el-button size="small" text type="primary" @click="openHanzi('given_name')">生僻字</el-button></div></el-form-item></el-col>
        </el-row>
        <el-form-item label="类型"><el-select v-model="editForm.type"><el-option label="正配" value="正配" /><el-option label="继配" value="继配" /><el-option label="侧室" value="侧室" /></el-select></el-form-item>
        <el-form-item label="家世"><el-input v-model="editForm.family" /></el-form-item>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="出生"><el-row :gutter="4"><el-col :span="8"><el-select v-model="editForm.birth_calendar" size="small"><el-option label="公历" value="solar" /><el-option label="农历" value="lunar" /></el-select></el-col><el-col :span="16"><el-input v-model="editForm.birth_date" placeholder="1900-08-01" @blur="spouseConvertDate" /></el-col></el-row></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="卒年"><el-input v-model="editForm.death_gregorian" placeholder="留空=在世" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="职业"><el-input v-model="editForm.occupation" placeholder="如：教师 / 医生" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="学历"><el-input v-model="editForm.education" placeholder="如：本科 / 举人" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="手机"><el-input v-model="editForm.phone" placeholder="手机号码" /></el-form-item></el-col>
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
        <el-form-item label="备注"><el-input v-model="editForm.remark" placeholder="例如：原配早逝，继配于某年续弦" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="saveSpouse">保存</el-button>
      </template>
    </el-dialog>

    <!-- 生僻字查字面板 -->
    <HanziPicker v-model="hanziVisible" @pick="onHanziPick" />

    <!-- 生平条目弹窗 -->
    <el-dialog v-model="bioVisible" :title="bioForm.id ? '编辑条目' : '新增条目'" width="640px">
      <el-form :model="bioForm" label-width="60px">
        <el-form-item label="标题"><el-input v-model="bioForm.title" placeholder="如：生平 / 迁居 / 功绩 / 轶事" /></el-form-item>
        <el-form-item label="内容"><el-input type="textarea" v-model="bioForm.content" :rows="6" placeholder="输入该条目的内容" /></el-form-item>

        <!-- AI 优化（病句排查 + 美化）-->
        <div class="ai-polish-box" v-if="canUseAI()">
          <div class="ai-polish-row">
            <span class="ai-polish-label">✨ AI 优化</span>
            <el-button size="small" type="warning" plain :loading="aiPolishing" @click="aiPolish">病句排查 + 美化</el-button>
          </div>
          <div class="ai-polish-row">
            <span class="ai-polish-label">📝 AI 生成小传</span>
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
            <el-button size="small" type="success" plain :loading="aiGenerating" @click="aiGenerateBio">按档案生成</el-button>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">约束等级</span>
            <el-button v-for="v in levelOptions" :key="'l'+v" size="small" :type="aiConstraint === v ? 'success' : ''" :plain="aiConstraint !== v" @click="aiConstraint = v">{{ v }}%</el-button>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">润色程度</span>
            <el-button v-for="v in levelOptions" :key="'d'+v" size="small" :type="aiDegree === v ? 'success' : ''" :plain="aiDegree !== v" @click="aiDegree = v">{{ v }}%</el-button>
          </div>
          <template v-if="aiPolishResult">
            <el-form-item label="标题"><el-input v-model="aiPolishResult.title" placeholder="AI 优化的标题" /></el-form-item>
            <div class="ai-polish-row">
              <span class="ai-polish-label">内容</span>
              <el-button size="small" type="primary" plain @click="useAiText(true)">使用这个（含标题）</el-button>
            </div>
            <el-input type="textarea" v-model="aiPolishResult.content" :rows="6" placeholder="AI 优化后的内容输出在此" />
            <div class="ai-use-row">
              <el-button size="small" @click="useAiText(false)">使用这个（仅内容，标题用我填的）</el-button>
            </div>
          </template>
          <div v-else class="ai-polish-tip">点「病句排查 + 美化」把标题和内容发给本地 AI 优化，结果输出在下方，可选择是否采用。</div>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="bioVisible = false">取消</el-button>
        <el-button type="primary" :loading="bioSaving" @click="saveBioEntry">保存</el-button>
      </template>
    </el-dialog>

    <!-- 修改历史 / 版本回滚 -->
    <el-dialog v-model="versionsVisible" title="修改历史" width="640px">
      <el-table :data="versions" size="small" max-height="420">
        <el-table-column prop="version_no" label="版本" width="70" />
        <el-table-column prop="note" label="操作" width="90" />
        <el-table-column prop="changed_by_name" label="操作人" width="110">
          <template #default="{ row }">{{ row.changed_by_name || '—' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="170" />
        <el-table-column label="操作" width="150" align="right">
          <template #default="{ row }">
            <el-button size="small" text type="info" @click="viewVersionChanges(row)">查看修改</el-button>
            <el-button v-if="canRollback()" size="small" text type="primary" @click="rollbackVersion(row)">回滚</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!versions.length" description="暂无修改记录" :image-size="40" />
      <template #footer>
        <el-button @click="versionsVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 版本修改详情 -->
    <el-dialog v-model="changesVisible" :title="`修改详情 · 版本 ${changesVersionNo}`" width="560px">
      <el-alert v-if="changesDeleted" type="danger" :closable="false" title="该版本操作为删除，记录已被删除" />
      <el-table v-else :data="changes" size="small" max-height="420" empty-text="无字段级差异">
        <el-table-column prop="field" label="字段" width="120" />
        <el-table-column label="原值" min-width="160">
          <template #default="{ row }"><span class="diff-old">{{ row.from }}</span></template>
        </el-table-column>
        <el-table-column label="新值" min-width="160">
          <template #default="{ row }"><span class="diff-new">{{ row.to }}</span></template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="changesVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 头像裁剪 -->
    <AvatarCropper v-model:visible="cropVisible" :src="cropSrc" @crop="onCropConfirm" />
    <!-- 媒体查看器 -->
    <el-dialog v-model="mediaViewer.show" :title="mediaViewer.title || '查看媒体'" width="800px" destroy-on-close>
      <div style="text-align:center">
        <video v-if="isVideo(mediaViewer)" :src="mediaViewer.path" controls style="max-width:100%;max-height:70vh"></video>
        <img v-else :src="mediaViewer.path" style="max-width:100%;max-height:70vh" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { canEdit, canUseAI, canRollback, canViewDNA } from '../utils/auth'
import { formatLifeDates, normalizeDeathFields } from '../utils/dateFormat'
import AvatarCropper from '../components/AvatarCropper.vue'
import HanziPicker from '../components/HanziPicker.vue'

const route = useRoute()
const router = useRouter()
const spouseId = parseInt(route.params.id)
const bioCount = ref(0)
function goBiography() {
  router.push({ path: '/culture', query: { tab: 'bios', spouse: String(spouseId), new: bioCount.value ? '0' : '1' } })
}
async function loadBioCount() {
  try {
    const { data } = await api.get('/biographies', { params: { spouse_id: spouseId } })
    bioCount.value = (data || []).length
  } catch (e) { bioCount.value = 0 }
}
const buildId = __BUILD_ID__ || 'dev'
function thumbUrl(path, size = 400) {
  if (!path) return ''
  return `/api/thumbnail?path=${encodeURIComponent(path)}&size=${size}&v=${buildId}`
}
const mediaViewer = reactive({ show: false, type: '', path: '', title: '' })
function openMediaViewer(a) {
  mediaViewer.show = true
  mediaViewer.type = a.category || '照片'
  mediaViewer.path = a.path
  mediaViewer.title = a.description || ''
}
const memberId = ref(0)
const spouse = reactive({})
const children = ref([])
const attachments = ref([])
const uploadHeaders = computed(() => ({ Authorization: 'Bearer ' + (localStorage.getItem('token') || '') }))
const mediaCategory = ref('全部')
const cropVisible = ref(false)
const cropSrc = ref('')
const uploadData = computed(() => ({ member_id: memberId.value, spouse_id: spouseId }))
const filteredAttachments = computed(() => {
  if (mediaCategory.value === '全部') return attachments.value
  return attachments.value.filter(a => {
    let c = (a.category || '照片').replace('配偶', '')
    if (c === '头像') c = '照片'
    return c === mediaCategory.value
  })
})
const isVideo = (a) => a.category === '视频'
const isAudio = (a) => a.category === '音频'
const isPhoto = (a) => !isVideo(a) && !isAudio(a)
const memberName = ref('')
const editVisible = ref(false)
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
const bioEntries = ref([])
const bioVisible = ref(false)
const bioSaving = ref(false)
const bioForm = reactive({ id: 0, title: '', content: '', sort_order: 0 })
const aiPolishing = ref(false)
const aiGenerating = ref(false)
const aiPolishResult = ref(null)
const aiGenStyle = ref('传记体')
const aiGenLength = ref('中')
const aiHasRun = ref(false)
const levelOptions = [0, 25, 50, 75, 100]
const aiConstraint = ref(50)
const aiDegree = ref(50)

// 修改历史 / 版本回滚
const versionsVisible = ref(false)
const versions = ref([])
const changesVisible = ref(false)
const changes = ref([])
const changesVersionNo = ref(0)
const changesDeleted = ref(false)

async function load() {
  loadBioCount()
  try {
    // 尝试新接口，失败则回退到全量列表
    let data = null
    try {
      const resp = await api.get(`/spouses/${spouseId}`)
      data = resp.data
    } catch (e) {
      const resp = await api.get('/spouses')
      data = resp.data.find(x => x.id === spouseId)
    }
    if (!data) return
    Object.assign(spouse, data)
    memberId.value = data.member_id
    // 加载所属成员
    try {
      const { data: m } = await api.get(`/members/${data.member_id}`)
      memberName.value = m.name || (m.surname || '') + (m.given_name || '')
    } catch (e) { memberName.value = `ID:${data.member_id}` }
    // 加载子女
    try {
      const { data: ch } = await api.get(`/members/${data.member_id}/children`)
      children.value = ch.filter(c => c.mother_spouse_id === spouseId)
    } catch (e) { /* ignore */ }
    // 加载附件（新接口）
    try {
      const { data: att } = await api.get(`/spouses/${spouseId}/attachments`)
      attachments.value = att
    } catch (e) { /* ignore */ }
    // 加载生平条目
    try {
      const { data: bd } = await api.get(`/spouses/${spouseId}/bio-entries`)
      bioEntries.value = bd || []
    } catch (e) { /* ignore */ }
  } catch (e) { /* ignore */ }
}

function goMember() { if (memberId.value) router.push(`/member/${memberId.value}`) }

// 打开修改历史
async function openVersions() {
  versions.value = []
  try {
    const { data } = await api.get(`/spouses/${spouseId}/versions`)
    versions.value = data || []
  } catch (e) {
    ElMessage.error('加载修改历史失败')
  }
  versionsVisible.value = true
}

// 查看某版本的字段级修改
async function viewVersionChanges(row) {
  changesVersionNo.value = row.version_no
  changes.value = []
  changesDeleted.value = false
  changesVisible.value = true
  try {
    const { data } = await api.get(`/entity-versions/${row.id}/changes`)
    changes.value = data.changes || []
    changesDeleted.value = !!data.deleted
  } catch (e) {
    ElMessage.error('加载修改详情失败')
  }
}

// 回滚到某版本
async function rollbackVersion(row) {
  try {
    await ElMessageBox.confirm(`确定回滚到版本 ${row.version_no}（${row.note}）吗？回滚前当前状态会另存一份，不会丢数据。`, '确认回滚', {
      confirmButtonText: '回滚', cancelButtonText: '取消', type: 'warning'
    })
  } catch (e) { return }
  try {
    await api.post(`/entity-versions/${row.id}/rollback`, { note: `回滚到版本 ${row.version_no}` })
    ElMessage.success('已回滚')
    versionsVisible.value = false
    load()
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '回滚失败'
    ElMessage.error('回滚失败：' + msg)
  }
}

// 计算年龄（周岁/虚岁），返回显示文本；无出生日期返回空
function calcAges() {
  const b = (spouse.birth_gregorian || '').trim()
  if (!b) return ''
  let by, bm = 1, bd = 1
  const mFull = b.match(/^(\d{4})-(\d{1,2})-(\d{1,2})/)
  const mYear = b.match(/^(\d{4})$/)
  if (mFull) { by = parseInt(mFull[1]); bm = parseInt(mFull[2]); bd = parseInt(mFull[3]) }
  else if (mYear) { by = parseInt(mYear[1]) }
  else return ''
  let cy, cm, cd
  const d = (spouse.death_gregorian || '').trim()
  const dmFull = d.match(/^(\d{4})-(\d{1,2})-(\d{1,2})/)
  const dmYear = d.match(/^(\d{4})$/)
  if (dmFull) {
    cy = parseInt(dmFull[1]); cm = parseInt(dmFull[2]); cd = parseInt(dmFull[3])
  } else if (dmYear) {
    cy = parseInt(dmYear[1]); cm = 12; cd = 31
  } else {
    const now = new Date()
    cy = now.getFullYear(); cm = now.getMonth() + 1; cd = now.getDate()
  }
  let zhousui = cy - by
  if (cm < bm || (cm === bm && cd < bd)) zhousui--
  const xusui = cy - by + 1
  if (zhousui < 0) return ''
  return `周岁 ${zhousui} · 虚岁 ${xusui}`
}

function openEdit() {
  const copy = JSON.parse(JSON.stringify(spouse))
  copy.birth_calendar = copy.birth_calendar || 'solar'
  copy.birth_date = copy.birth_date || copy.birth_gregorian || ''
  Object.assign(editForm, copy)
  editVisible.value = true
}

async function saveSpouse() {
  await spouseConvertDate()  // 确保转换已执行
  const payload = { ...editForm }
  payload.name = (payload.surname || '') + (payload.given_name || '')
  // 卒年规范化：留空=在世；"已逝"等标记转存 death_era
  normalizeDeathFields(payload)
  // birth_gregorian 已由 spouseConvertDate 设置
  delete payload.birth_date
  let r
  try {
    r = await api.put(`/spouses/${spouseId}`, payload)
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '保存失败'
    ElMessage.error('保存失败：' + msg)
    return
  }
  if (r.data && r.data.draft) { editVisible.value = false; ElMessage.success(r.data.message || '已提交审核'); return }
  editVisible.value = false; ElMessage.success('已保存'); load()
}

async function spouseConvertDate() {
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
    const { data } = await api.get(`/lunar/to-solar?y=${m[1]}&m=${m[2]}&d=${m[3]}`)
    editForm.birth_gregorian = data.gregorian
    editForm.birth_lunar = data.lunar; editForm.birth_ganzhi = data.ganzhi; editForm.birth_era = data.era + data.eraYear
    ElMessage.success(`公历：${data.gregorian}`)
  } else {
    const { data } = await api.get(`/lunar/convert?y=${m[1]}&m=${m[2]}&d=${m[3]}`)
    editForm.birth_gregorian = d
    editForm.birth_lunar = data.lunar; editForm.birth_ganzhi = data.ganzhi; editForm.birth_era = data.era + data.eraYear
    ElMessage.success(`农历：${data.lunar}`)
  }
}

function onUploadSuccess(resp) { if (resp.id) { ElMessage.success('上传成功'); load() } }
function setAsPortrait(path) {
  cropSrc.value = path
  cropVisible.value = true
}
async function onCropConfirm(blob) {
  const fd = new FormData()
  fd.append('file', blob, 'avatar.png')
  fd.append('member_id', String(memberId.value || 0))
  fd.append('spouse_id', String(spouseId))
  fd.append('category', '配偶头像')
  try {
    await api.post('/upload', fd)
    ElMessage.success('已设为头像')
    cropVisible.value = false
    load()
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '设置头像失败'
    ElMessage.error('设置头像失败：' + msg)
  }
}

async function deleteAttachment(id) {
  await ElMessageBox.confirm('确定删除该附件？', '确认', { type: 'warning' })
  await api.delete(`/attachments/${id}`)
  ElMessage.success('已删除'); load()
}

// —— 生平条目 ——
function openBioDialog(e = null) {
  aiPolishResult.value = null
  aiHasRun.value = false
  if (e) {
    Object.assign(bioForm, { id: e.id, title: e.title, content: e.content, sort_order: e.sort_order || 0 })
  } else {
    Object.assign(bioForm, { id: 0, title: '', content: '', sort_order: bioEntries.value.length })
  }
  bioVisible.value = true
}

async function aiPolish() {
  if (!bioForm.content || !bioForm.content.trim()) { ElMessage.warning('请先填写内容'); return }
  aiPolishing.value = true
  aiPolishResult.value = null
  try {
    const { data } = await api.post('/ai/polish-bio', {
      title: bioForm.title || '',
      content: bioForm.content.trim(),
      level: aiConstraint.value,
      degree: aiDegree.value
    }, { timeout: 190000 })
    aiPolishResult.value = { title: data.title || bioForm.title || '', content: data.content || bioForm.content }
    aiHasRun.value = true
    if (data.protected) {
      ElMessage.warning('已拦截 AI 对日期/数字的改动，保留原文')
    }
  } catch (e) {
    aiHasRun.value = true
    ElMessage.error(e?.response?.data?.error || 'AI 优化失败，请确认修谱助手已启用并安装模型')
  } finally {
    aiPolishing.value = false
  }
}

// AI 依据配偶档案生成小传草稿（保存仍走原有 bio-entries 审核流程）
async function aiGenerateBio() {
  if (bioForm.content && bioForm.content.trim()) {
    try {
      await ElMessageBox.confirm('将根据该配偶档案生成小传草稿，并覆盖下方「内容」里已写的文字。继续？', '生成小传', { type: 'warning' })
    } catch (e) { return }
  }
  aiGenerating.value = true
  aiPolishResult.value = null
  try {
    const { data } = await api.post('/ai/generate-bio', {
      spouse_id: spouseId,
      style: aiGenStyle.value,
      length: aiGenLength.value,
      level: aiConstraint.value,
      degree: aiDegree.value
    }, { timeout: 190000 })
    aiPolishResult.value = { title: data.title || '生平', content: data.content || '' }
    aiHasRun.value = true
    if (data.protected) ElMessage.warning('AI 对日期/数字的改动已被拦截，保留原文纪年')
    else ElMessage.success('已生成小传草稿，确认后点「使用这个」填入')
  } catch (e) {
    aiHasRun.value = true
    ElMessage.error(e?.response?.data?.error || 'AI 生成失败，请确认修谱助手已启用并配置好模型')
  } finally {
    aiGenerating.value = false
  }
}

function useAiText(withTitle) {
  if (!aiPolishResult.value) return
  if (withTitle && aiPolishResult.value.title) bioForm.title = aiPolishResult.value.title
  bioForm.content = aiPolishResult.value.content || ''
  ElMessage.success(withTitle ? '已采用 AI 标题与内容' : '已采用 AI 内容（标题保留手动填写）')
}

async function saveBioEntry() {
  if (!bioForm.title || !bioForm.title.trim()) { ElMessage.warning('请输入标题'); return }
  if (bioSaving.value) return
  bioSaving.value = true
  try {
    let r
    if (bioForm.id) {
      r = await api.put(`/spouse-bio-entries/${bioForm.id}`, bioForm)
    } else {
      r = await api.post(`/spouses/${spouseId}/bio-entries`, bioForm)
    }
    if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); bioVisible.value = false; return }
    ElMessage.success('已保存')
    bioVisible.value = false
    const { data } = await api.get(`/spouses/${spouseId}/bio-entries`)
    bioEntries.value = data || []
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '保存失败')
  } finally {
    bioSaving.value = false
  }
}

async function deleteBioEntry(id) {
  try {
    await ElMessageBox.confirm('确定删除该条目？', '确认', { type: 'warning' })
  } catch { return }
  let r
  try {
    r = await api.delete(`/spouse-bio-entries/${id}`)
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '删除失败'
    ElMessage.error('删除失败：' + msg)
    return
  }
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已删除')
  const { data } = await api.get(`/spouses/${spouseId}/bio-entries`)
  bioEntries.value = data || []
}

onMounted(load)
</script>

<style scoped>
.section { margin-bottom: 16px; }
.card-header { display: flex; align-items: center; justify-content: space-between; }
.header-left { display: flex; align-items: center; gap: 12px; }
.serif { font-size: 20px; font-family: "Songti SC", "SimSun", serif; }
.avatar { border: 2px solid #d9cbb6; }
.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
@media (max-width: 767px) { .info-grid { grid-template-columns: 1fr; } }
.info-item { display: flex; padding: 4px 0; }
.info-item .label { width: 80px; color: #8c5a3c; flex-shrink: 0; }
.info-item em { color: #8c5a3c; font-style: normal; margin-left: 6px; font-size: 12px; }
.clickable { color: #3a3226; cursor: pointer; text-decoration: underline; text-decoration-style: dotted; }
.clickable:hover { color: #8c5a3c; }
.child-item { padding: 6px 0; border-bottom: 1px solid #f0ebe0; display: flex; justify-content: space-between; align-items: center; }
.child-link { color: #3a3226; text-decoration: none; font-weight: 500; }
.child-link:hover { color: #8c5a3c; }
.child-sub { font-size: 12px; color: #8c5a3c; }
.media-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 12px; }
.media-item { border: 1px solid #e0d8c8; border-radius: 8px; overflow: hidden; }
.media-img { width: 100%; height: 140px; }
.media-audio { width: 100%; margin: 8px 0; }
.media-actions { display: flex; justify-content: space-around; padding: 4px; background: #f7f3ec; }
.media-thumb { position: relative; display: inline-block; width: 100%; }
.play-overlay { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.3); display: flex; align-items: center; justify-content: center; border-radius: 8px; }
.play-icon { color: #fff; font-size: 36px; text-shadow: 0 2px 4px rgba(0,0,0,0.5); }
.media-audio { width: 100%; margin: 8px 0; }
.media-actions { display: flex; justify-content: space-around; padding: 4px; background: #f7f3ec; }
.bio { margin-top: 16px; padding-top: 16px; border-top: 1px solid #d9cbb6; }
.bio-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.bio-head-actions { display: flex; align-items: center; gap: 8px; }
.bio h4 { margin-bottom: 8px; color: #8c5a3c; }
.bio-entry { padding: 10px 0; border-bottom: 1px solid #f0ebe0; }
.bio-entry-head { display: flex; align-items: center; justify-content: space-between; }
.bio-title { font-weight: bold; font-size: 15px; color: #3a3226; }
.bio-entry-actions { display: inline-flex; gap: 4px; }
.bio-content { margin-top: 4px; line-height: 1.8; white-space: pre-wrap; }
.ai-polish-box {
  margin-top: 8px; padding: 12px;
  border: 1px dashed #e6a23c; border-radius: 8px; background: #fdf6ec;
}
.ai-polish-row { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 8px; }
.ai-polish-label { font-weight: 600; color: #8c5a3c; }
.ai-level-row { display: flex; align-items: center; gap: 6px; margin: 6px 0; flex-wrap: wrap; }
.ai-level-label { font-size: 12px; color: #8c5a3c; font-weight: 500; min-width: 56px; }
.ai-use-row { display: flex; justify-content: flex-end; margin-top: 8px; }
.ai-polish-tip { font-size: 12px; color: #8c5a3c; }
.hz-inline { display: flex; align-items: center; gap: 4px; width: 100%; }
</style>