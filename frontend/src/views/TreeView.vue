<template>
  <div class="tree-view">
    <div class="toolbar">
      <el-input v-model="search" placeholder="搜索姓名/字号" clearable style="width: 220px" @keyup.enter="doSearch" />
      <el-button type="primary" @click="doSearch">定位</el-button>
      <el-button @click="zoomIn">放大</el-button>
      <el-button @click="zoomOut">缩小</el-button>
      <el-button @click="resetZoom">重置</el-button>
      <el-select v-model="branchFilter" size="small" style="width:120px" @change="onBranchChange">
        <el-option label="全部" value="all" />
        <el-option label="未分房" value="unassigned" />
        <el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.id" />
      </el-select>
      <el-select v-model="exportMode" size="small" style="width:120px">
        <el-option label="导出全部" value="all" />
        <el-option label="仅内纪" value="neiji" />
        <el-option label="仅外纪" value="waiji" />
      </el-select>
      <FontPicker />
      <el-dropdown size="small" trigger="click" @command="doExport">
        <el-button size="small" :loading="exporting">导出<el-icon class="el-icon--right"><arrow-down /></el-icon></el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="png300">🖼️ PNG（高清 · 300DPI）</el-dropdown-item>
            <el-dropdown-item command="svg">📐 SVG（矢量图）</el-dropdown-item>
            <el-dropdown-item command="svgPortrait">📐 SVG（内嵌头像）</el-dropdown-item>
            <el-dropdown-item command="svgFont">📐 SVG（内嵌字体）</el-dropdown-item>
            <el-dropdown-item command="svgPortraitFont">📐 SVG（内嵌头像 + 字体）</el-dropdown-item>
            <el-dropdown-item command="pdf">🖨️ PDF（打印用）</el-dropdown-item>
            <el-dropdown-item command="pagedPrint" divided>🖨️ 分页打印（A3/A4 分页 · 卷轴 · 索引页）</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <el-button v-if="portraitLoad === 'thumb'" type="warning" @click="portraitLoad = 'full'">📷 加载原图</el-button>
      <el-button v-if="portraitLoad === 'off'" type="warning" @click="portraitLoad = 'thumb'">📷 加载缩略图</el-button>
      <span v-if="portraitLoad === 'off'" class="hint-noportrait">头像未加载</span>
      <span v-else class="hint">拖拽平移 · 滚轮缩放 · 点击头像查看详情</span>
    </div>

    <div class="canvas-wrap" ref="wrap" @wheel.prevent="onWheel" @pointerdown.prevent="onPointerDown">
      <svg :width="svgW" :height="svgH" ref="svg" class="tree-svg" :class="{ 'tf-on': treeFontActive }" :style="treeFontStyle">
        <defs>
          <linearGradient id="bioGold" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="#fdf3c8" />
            <stop offset="30%" stop-color="#e8c65c" />
            <stop offset="65%" stop-color="#d0a52c" />
            <stop offset="100%" stop-color="#a8801a" />
          </linearGradient>
          <linearGradient id="bioPaper" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stop-color="#ffffff" />
            <stop offset="55%" stop-color="#fdf1d2" />
            <stop offset="100%" stop-color="#eddfb6" />
          </linearGradient>
          <clipPath v-for="n in nodes" :key="'clip'+n.id" :id="'clip'+n.id">
            <rect :x="n.x - 30" :y="n.y" :width="60" :height="60" rx="8" />
          </clipPath>
          <clipPath v-for="sn in allSpouseNodes" :key="'spclip'+sn.id" :id="'spclip'+sn.id">
            <rect :x="sn.x" :y="sn.y" :width="60" :height="60" rx="8" />
          </clipPath>
        </defs>
        <g :transform="`translate(${pan.x},${pan.y}) scale(${zoom})`">
          <!-- 连线 -->
          <g v-for="(edge, i) in edges" :key="'e'+i">
            <path :d="edge.d" :stroke="edge.sep ? '#8c5a3c' : '#8c5a3c'" :stroke-width="edge.sep ? 1 : 1.5" :stroke-dasharray="edge.dashed ? '8,4' : 'none'" fill="none" :opacity="edge.sep ? 0.5 : 1" />
          </g>
          <!-- 节点 -->
          <g v-for="n in nodes" :key="n.id" class="node-group">
            <!-- 成员头像 -->
            <g class="node" @click="showMember(n)">
              <image v-if="n.portrait && portraitLoad !== 'off'" :href="portraitSrc(n.portrait)" :x="n.x - 30" :y="n.y" :width="60" :height="60" :clip-path="`url(#clip${n.id})`" />
              <rect v-else :x="n.x - 30" :y="n.y" :width="60" :height="60" rx="8"
                :fill="n.dead ? '#d9cbb6' : '#e8d5b8'"
                :stroke="n.highlight ? '#d9534f' : '#8c5a3c'" stroke-width="1.5" />
              <text v-if="!n.portrait" :x="n.x" :y="n.y + 37" text-anchor="middle" font-size="22" fill="#8c5a3c">{{ n.displayName?.charAt(0) }}</text>
              <!-- 姓名竖排（头像左侧） -->
              <text v-for="(ch, ci) in n.nameChars" :key="'nc'+ci"
                :x="n.x - 42" :y="n.textStartY + ci * LINE_H" text-anchor="middle"
                class="node-name" :fill="n.dead ? '#7a6a52' : '#3a3226'">{{ ch }}</text>
              <!-- 字辈竖排（名字下方） -->
              <text v-for="(ch, ci) in n.subChars" :key="'sc'+ci"
                :x="n.x - 42" :y="n.textStartY + n.nameChars.length * LINE_H + ci * SUB_LINE_H" text-anchor="middle"
                class="node-sub" fill="#8c5a3c">{{ ch }}</text>
              <!-- 有传记：金色卷轴标记（旋转90°，上半露于方框上沿；点击弹出传记） -->
              <template v-if="n.has_bio">
              <g :transform="`translate(${n.x + 20} ${n.y}) rotate(90)`" class="bio-mark" @click.stop="openBio(n)">
              <g opacity="0.20" fill="#5a4632">
              <rect x="-6.3" y="-8.3" width="14" height="3.6" rx="1.8" />
              <rect x="-7.3" y="-4.8" width="16" height="11" rx="1.5" />
              <rect x="-6.3" y="6.2" width="14" height="3.6" rx="1.8" />
              </g>
              <rect x="-7" y="-9" width="14" height="3.6" rx="1.8" fill="url(#bioGold)" stroke="#a8801a" stroke-width="0.5" />
              <rect x="-5.8" y="-8.5" width="11.6" height="1.1" rx="0.55" fill="#fffdf2" opacity="0.85" />
              <rect x="-8" y="-5.5" width="16" height="11" rx="1.5" fill="url(#bioPaper)" stroke="#b8901f" stroke-width="0.9" />
              <path d="M-5.2 -1.6 h10.4 M-5.2 1.9 h10.4" stroke="#cdb079" stroke-width="0.7" fill="none" />
              <rect x="-7" y="5.5" width="14" height="3.6" rx="1.8" fill="url(#bioGold)" stroke="#a8801a" stroke-width="0.5" />
              <rect x="-5.8" y="6" width="11.6" height="1.1" rx="0.55" fill="#fffdf2" opacity="0.6" />
              </g>
              </template>
            </g>
            <!-- 配偶头像 -->
            <g v-for="sn in n.spouseNodes" :key="sn.id" class="node" @click.stop="showSpouseDetail(sn)">
              <image v-if="sn.portrait && portraitLoad !== 'off'" :href="portraitSrc(sn.portrait)" :x="sn.x" :y="sn.y" :width="60" :height="60" :clip-path="`url(#spclip${sn.id})`" />
              <rect v-else :x="sn.x" :y="sn.y" :width="60" :height="60" rx="8"
                :fill="'#f5efe0'" :stroke="'#c4a080'" stroke-width="1.5" />
              <text v-if="!sn.portrait" :x="sn.x + 30" :y="sn.y + 37" text-anchor="middle" font-size="22" fill="#c4a080">{{ sn.name?.charAt(0) }}</text>
              <!-- 姓名竖排（头像右侧） -->
              <text v-for="(ch, ci) in sn.nameChars" :key="'spnc'+ci"
                :x="sn.x + 72" :y="sn.textStartY + ci * LINE_H" text-anchor="middle"
                class="node-name" fill="#a08060">{{ ch }}</text>
              <!-- 类型竖排（名字下方） -->
              <text v-for="(ch, ci) in sn.typeChars" :key="'sptc'+ci"                :x="sn.x + 72" :y="sn.textStartY + sn.nameChars.length * LINE_H + ci * SUB_LINE_H" text-anchor="middle"
                class="node-sub" fill="#c4a080">{{ ch }}</text>
              <!-- 有传记：金色卷轴标记（旋转90°，上半露于方框上沿；点击弹出传记） -->
              <template v-if="sn.has_bio">
              <g :transform="`translate(${sn.x + 50} ${sn.y}) rotate(90)`" class="bio-mark" @click.stop="openBioSpouse(sn)">
              <g opacity="0.20" fill="#5a4632">
              <rect x="-6.3" y="-8.3" width="14" height="3.6" rx="1.8" />
              <rect x="-7.3" y="-4.8" width="16" height="11" rx="1.5" />
              <rect x="-6.3" y="6.2" width="14" height="3.6" rx="1.8" />
              </g>
              <rect x="-7" y="-9" width="14" height="3.6" rx="1.8" fill="url(#bioGold)" stroke="#a8801a" stroke-width="0.5" />
              <rect x="-5.8" y="-8.5" width="11.6" height="1.1" rx="0.55" fill="#fffdf2" opacity="0.85" />
              <rect x="-8" y="-5.5" width="16" height="11" rx="1.5" fill="url(#bioPaper)" stroke="#b8901f" stroke-width="0.9" />
              <path d="M-5.2 -1.6 h10.4 M-5.2 1.9 h10.4" stroke="#cdb079" stroke-width="0.7" fill="none" />
              <rect x="-7" y="5.5" width="14" height="3.6" rx="1.8" fill="url(#bioGold)" stroke="#a8801a" stroke-width="0.5" />
              <rect x="-5.8" y="6" width="11.6" height="1.1" rx="0.55" fill="#fffdf2" opacity="0.6" />
              </g>
              </template>
            </g>
          </g>
          <!-- 无嗣 / 迁居失联 标注 -->
          <g v-for="mk in markers" :key="mk.id">
            <path :d="`M${mk.x} ${mk.from} L${mk.x} ${mk.y}`" stroke="#8c5a3c" stroke-width="1.5" fill="none" />
            <rect :x="mk.x - mk.boxW / 2" :y="mk.y" :width="mk.boxW" :height="mk.boxH" rx="6"
              fill="#d9d9d9" stroke="#8c5a3c" stroke-width="1.2" />
            <text v-for="(ln, li) in mk.lines" :key="li" :x="mk.x"
              :y="mk.y + mk.boxH / 2 + mk.fs * 0.36 - ((mk.lines.length - 1) * mk.lineH) / 2 + li * mk.lineH"
              text-anchor="middle" :font-size="mk.fs" :class="mk.type === 'noheir' ? 'node-name' : ''"
              fill="#8c5a3c">{{ ln }}</text>
          </g>
        </g>
      </svg>
    </div>

    <el-dialog v-model="dialogVisible" :title="current.name || (current.surname || '') + (current.given_name || '')" width="520px">
      <div v-if="current.id" class="member-card">
        <div class="dialog-header" v-if="current.portrait">
          <el-image :src="current.portrait" style="width:80px;height:80px;border-radius:8px" fit="cover" />
        </div>
        <div class="row"><span class="label">世代</span><span>{{ genLabel(current) }}</span></div>
        <div class="row"><span class="label">字辈</span><span>{{ current.generation_word || '—' }}</span></div>
        <div class="row"><span class="label">字/号</span><span>{{ current.zi || '—' }} / {{ current.hao || '—' }}</span></div>
        <div class="row"><span class="label">宗法</span><span>{{ current.clan_type }}</span></div>
        <div class="row"><span class="label">生卒</span><span>{{ formatLifeDates(current) }}</span></div>
        <div class="row" v-if="calcAge(current)"><span class="label">年龄</span><span>{{ calcAge(current) }}</span></div>
        <div class="row" v-if="current.spouses && current.spouses.length"><span class="label">配偶</span><span><span v-for="(s,i) in current.spouses" :key="s.id"><router-link :to="`/spouse/${s.id}`" class="sp-link">{{ s.name }}</router-link>({{ s.type }})<span v-if="i < current.spouses.length - 1">、</span></span></span></div>
        <div class="row" v-if="current.biography"><span class="label">生平</span><span>{{ current.biography }}</span></div>
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
        <el-button type="primary" @click="goDetail(current)">查看详情</el-button>
        <el-button v-if="canEdit()" type="primary" @click="editMember(current)">编辑</el-button>
      </template>
    </el-dialog>

    <!-- 配偶弹窗 -->
    <el-dialog v-model="spouseDialogVisible" :title="currentSpouse.name" width="420px">
      <div v-if="currentSpouse.spouseId" class="member-card">
        <div class="dialog-header" v-if="currentSpouse.portrait">
          <el-image :src="currentSpouse.portrait" style="width:80px;height:80px;border-radius:8px" fit="cover" />
        </div>
        <div class="row"><span class="label">姓名</span><span>{{ currentSpouse.name }}</span></div>
        <div class="row"><span class="label">类型</span><span>{{ currentSpouse.type }}</span></div>
        <div class="row" v-if="currentSpouse.occupation"><span class="label">职业</span><span>{{ currentSpouse.occupation }}</span></div>
        <div class="row" v-if="currentSpouse.education"><span class="label">学历</span><span>{{ currentSpouse.education }}</span></div>
        <div class="row" v-if="calcAge(currentSpouse)"><span class="label">年龄</span><span>{{ calcAge(currentSpouse) }}</span></div>
      </div>
      <template #footer>
        <el-button @click="spouseDialogVisible = false">关闭</el-button>
        <el-button type="primary" @click="goSpouseDetail(currentSpouse)">查看详情</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="editVisible" :title="editForm.id ? '编辑成员' : '新增成员'" width="640px">
      <el-form :model="editForm" label-width="80px">
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="姓"><el-input v-model="editForm.surname" disabled /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="名"><el-input v-model="editForm.given_name" placeholder="输入名" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="世代">
            <template v-if="editForm.record_type === '外纪'">
              <el-input :model-value="waijiGenLabel" disabled />
            </template>
            <el-input-number v-else v-model="editForm.generation" :min="1" />
          </el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="性别"><el-select v-model="editForm.gender"><el-option label="男" :value="1" /><el-option label="女" :value="2" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="字"><el-input v-model="editForm.zi" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="号"><el-input v-model="editForm.hao" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="字辈"><el-input v-model="editForm.generation_word" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="宗法"><el-select v-model="editForm.clan_type"><el-option v-for="t in clanTypes" :key="t" :label="t" :value="t" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="类型"><el-select v-model="editForm.record_type" @change="onRecordTypeChange"><el-option label="内纪" value="内纪" /><el-option label="外纪" value="外纪" /></el-select></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="生父"><el-select v-model="editForm.father_id" filterable clearable style="width:100%" @change="onFatherChange"><el-option v-for="m in fatherCandidates" :key="m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel(m)}）`" :value="m.id" /></el-select></el-form-item>
          </el-col>
          <el-col :span="12" v-if="editForm.record_type === '外纪'">
            <el-form-item label="儿子"><el-select v-model="editForm.son_id" filterable clearable placeholder="选择儿子（关键）" style="width:100%" @change="onSonChange"><el-option v-for="m in sonCandidates" :key="m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel(m)}）`" :value="m.id" /></el-select>
              <div v-if="editForm.son_id" style="margin-top:4px;color:#909399;font-size:12px">{{ sonPositionText }}</div></el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12" v-if="editForm.record_type !== '外纪'">
          <el-col :span="12">
            <el-form-item label="生母"><el-select v-model="editForm.mother_id" filterable clearable style="width:100%"><el-option v-for="m in motherCandidates" :key="m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（第${m.generation}世）`" :value="m.id" /></el-select></el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="出生"><el-row :gutter="4"><el-col :span="8"><el-select v-model="editForm.birth_calendar" size="small"><el-option label="公历" value="solar" /><el-option label="农历" value="lunar" /></el-select></el-col><el-col :span="16"><el-input v-model="editForm.birth_date" placeholder="1900-08-01" @blur="convertDate" /></el-col></el-row></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="卒年"><el-input v-model="editForm.death_gregorian" placeholder="留空表示在世" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="saveMember">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="branchPromptVisible" title="房支后代未归入" width="480px" :close-on-click-modal="false" :show-close="false">
      <div class="bp-body">
        <p>房支「{{ pendingBranchName }}」下有 <b>{{ unassignedDesc.length }}</b> 名后代尚未归入房支。</p>
        <p class="bp-sub">是否在图中显示这些后代？</p>
      </div>
      <template #footer>
        <el-button @click="branchPromptChoose('no')">否</el-button>
        <el-button @click="branchPromptChoose('show')">是，不加入房支</el-button>
        <el-button v-if="canEdit()" type="primary" @click="branchPromptChoose('join')">是，加入房支</el-button>
      </template>
    </el-dialog>

    <!-- 新增成员后自动建号：初始密码弹窗 -->
    <AccountPwDialog v-model:visible="acctPw.visible" :name="acctPw.name" :password="acctPw.password" />

    <!-- 传记弹窗（点击金色卷轴标记） -->
    <BiographyPopup ref="bioPopup" />

    <!-- 打印 / 导出：A3/A4 分页（瓦片）· 卷轴 · 索引页 -->
    <PrintExportDialog
      v-model="printDlgVisible"
      :svg-el="svg"
      :content-w="contentW"
      :content-h="contentH"
      :rows="printRows"
      :bounds="printBounds"
      title="宝塔式世系图"
      :volume="printVolume"
      :theme="{ bg: '#ffffff', ink: '#3a2a18' }"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api'
import BiographyPopup from '../components/BiographyPopup.vue'
import { canEdit } from '../utils/auth'
import { findUnassignedDescendants } from '../utils/branchTree'
import { exportHighResPng, exportVectorSvg, exportPrintPDF } from '../utils/treeExport'
import PrintExportDialog from '../components/PrintExportDialog.vue'
import { rowsFromNodes } from '../utils/printPaging'
import { branchCtx } from '../utils/branchContext'
import FontPicker from '../components/FontPicker.vue'
import { treeFontActive, treeFontStyle } from '../utils/treeFonts'
import { ArrowDown } from '@element-plus/icons-vue'
import { formatLifeDates, normalizeDeathFields } from '../utils/dateFormat'
import { memberKnown, lostContactText, NO_HEIR } from '../utils/memberFlags'
import AccountPwDialog from '../components/AccountPwDialog.vue'

const router = useRouter()
const acctPw = ref({ visible: false, name: '', password: '' })

const members = ref([])
const branches = ref([])
const branchFilter = ref('all') // 'all' = 全部，'unassigned' = 未分房，数字 = 房支 id
const extraShowIds = ref([]) // 选择「不加入但显示」时额外显示的未分房后代 id
const visibleMembers = computed(() => {
  const v = branchFilter.value
  if (v === 'all') return members.value
  if (v === 'unassigned') return members.value.filter(m => (m.branch_id || 0) === 0)
  const extra = new Set(extraShowIds.value || [])
  return members.value.filter(m => (m.branch_id || 0) === v || extra.has(m.id))
})
const branchPromptVisible = ref(false)
const unassignedDesc = ref([])
const pendingBranchId = ref(null)
const pendingBranchName = ref('')
const familySurname = ref('')
const portraitLoad = ref('thumb') // 'off' | 'thumb' | 'full'
// 用构建版本号打破浏览器不可变缓存（头像更换后缩略图刷新）
const buildId = __BUILD_ID__ || 'dev'
function portraitSrc(url) {
  if (!url) return ''
  if (portraitLoad.value === 'full') return url
  return `/api/thumbnail?path=${encodeURIComponent(url)}&size=120&v=${buildId}`
}
const search = ref('')
const wrap = ref(null)
const svg = ref(null)
const zoom = ref(1)
const pan = reactive({ x: 40, y: 40 })
const svgW = ref(1200)
const svgH = ref(800)
const contentW = ref(1000)
const contentH = ref(800)
const exportMode = ref('all') // 'all' | 'neiji' | 'waiji'
const waijiBoundaryY = ref(0) // 外纪/内纪分界线Y坐标
const dialogVisible = ref(false)
const editVisible = ref(false)
const spouseDialogVisible = ref(false)
const current = reactive({})
const currentSpouse = ref({})
const editForm = reactive({})
const clanTypes = ['本生', '嗣子', '兼祧', '入赘', '养子']

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

function genLabel(m) {
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

const AVATAR = 60     // 头像宽高
const GAP_S = 0        // 头像间距（成员与配偶之间，0=紧贴）
const GAP_F = 40       // 家庭单元间距
const NODE_H = 100     // 单个节点总高（头像+文字）
const GEN_GAP = 50     // 世代间垂直间距
const WAIGI_GAP = 80   // 外纪/内纪分界间距
const LEAF_SPACING = 170  // 叶子槽位横向宽度
const SIDE_MARGIN = 60
const TOP_MARGIN = 40
const NAME_W = 24     // 头像侧边竖排文字列宽（一列一字）
const LINE_H = 16     // 竖排名字每字行高
const SUB_LINE_H = 13 // 竖排字辈/类型每字行高

// 配偶列表：正配→继配→侧室 依次排列
function spouseList(m) {
  const order = { '正配': 1, '继配': 2, '侧室': 3 }
  return (m.spouses || [])
    .filter(s => s.type === '正配' || s.type === '继配' || s.type === '侧室')
    .sort((a, b) => (order[a.type] || 9) - (order[b.type] || 9))
}

// 字符串拆成字符数组（处理代理对，保证竖排每字一列）
function toChars(s) {
  return Array.from(String(s || ''))
}

// 家庭单元宽度（成员名在左 + 成员头像 + 各配偶头像及其右侧名字）
function unitWOf(m) {
  const spCount = spouseList(m).length
  return NAME_W + AVATAR + spCount * (AVATAR + NAME_W) + (spCount ? (spCount - 1) * GAP_S : 0)
}

// 兄弟长幼排序：有排行按排行数字，否则按 id
function sortSiblings(arr) {
  const rankNum = r => {
    if (!r || !r.rank) return null
    const m = String(r.rank).match(/(\d+)/)
    return m ? parseInt(m[1], 10) : null
  }
  arr.sort((a, b) => {
    const ra = rankNum(a), rb = rankNum(b)
    if (ra != null && rb != null) return ra - rb
    if (ra != null) return -1
    if (rb != null) return 1
    return a.id - b.id
  })
}

const nodes = computed(() => {
  const list = visibleMembers.value
  if (!list.length) return []

  // 1. 建索引 + 挂父子
  const byId = new Map()
  list.forEach(m => byId.set(m.id, { ...m, children: [] }))
  const roots = []
  byId.forEach(node => {
    if (node.father_id && byId.has(node.father_id)) {
      byId.get(node.father_id).children.push(node)
    } else {
      roots.push(node)
    }
  })
  byId.forEach(node => sortSiblings(node.children))
  roots.sort((a, b) => a.generation - b.generation || a.id - b.id)

  // 2. tidy 布局：父 = 首末子中心的中点
  let leafCounter = 0
  function layout(node) {
    const kids = node.children
    if (!kids.length) {
      node.center = leafCounter
      node.min = leafCounter
      node.max = leafCounter
      leafCounter++
    } else {
      kids.forEach(layout)
      node.center = (kids[0].center + kids[kids.length - 1].center) / 2
      node.min = Math.min(...kids.map(k => k.min))
      node.max = Math.max(...kids.map(k => k.max))
    }
  }
  function shiftSubtree(node, shift) {
    node.center += shift
    node.min += shift
    node.max += shift
    node.children.forEach(c => shiftSubtree(c, shift))
  }
  let base = 0
  roots.forEach(r => {
    layout(r)
    const w = r.max - r.min
    const shift = base - r.min
    shiftSubtree(r, shift)
    base += (w + 1) + 1
  })

  // 3. 同世代横向解重叠（迭代到收敛）。
  // 解重叠会平移子树（不动祖先），父居中重算（3.5）又会移动父节点到子女中点、可能撞同世代旁支；
  // 两者相互影响，故反复交替直到某轮无重叠。
  const minGen = Math.min(...list.map(m => m.generation))
  const maxGen = Math.max(...list.map(m => m.generation))
  const MIN_GAP = 30
  const genList = (g) => [...byId.values()].filter(n => n.generation === g).sort((a, b) => a.center - b.center)
  const resolveOverlap = () => {
    let moved = false
    for (let g = minGen; g <= maxGen; g++) {
      let rightEdge = -Infinity
      genList(g).forEach(n => {
        const uw = unitWOf(n)
        const left = n.center * LEAF_SPACING - NAME_W - AVATAR / 2
        const right = left + uw
        if (left < rightEdge + MIN_GAP) {
          const d = (rightEdge + MIN_GAP - left) / LEAF_SPACING
          const moveSub = (nd, delta) => {
            nd.center += delta
            nd.children.forEach(c => moveSub(c, delta))
          }
          moveSub(n, d)
          moved = true
          rightEdge = rightEdge + MIN_GAP + (d * LEAF_SPACING)  // 平移后右边界同步右移
        } else {
          rightEdge = Math.max(rightEdge, right)
        }
      })
    }
    return moved
  }
  // 3.5 父居中：父 center = 首末子女 center 中点（自底向上）
  const recenterParents = () => {
    let changed = false
    for (let g = maxGen; g >= minGen; g--) {
      genList(g).forEach(n => {
        if (!n.children.length) return
        const cs = n.children.map(c => c.center)
        const nc = (Math.min(...cs) + Math.max(...cs)) / 2
        if (Math.abs(nc - n.center) > 1e-6) { n.center = nc; changed = true }
      })
    }
    return changed
  }
  for (let iter = 0; iter < 20; iter++) {
    const a = resolveOverlap()   // 先拉开同世代重叠
    const b = recenterParents()  // 再让父居中于子女
    if (!a && !b) break          // 收敛
  }

  // 4. 计算坐标（n.x = 成员头像中心；spouseNodes[].x = 配偶头像左上角）
  const result = []
  byId.forEach(m => {
    const displayName = m.name || (m.surname || '') + (m.given_name || '')
    const spList = spouseList(m)
    const unitW = unitWOf(m)
    // 成员头像中心（单元左边界 = 名字区 + 头像中心 - 头像/2）
    const cx = m.center * LEAF_SPACING + SIDE_MARGIN
    const y = TOP_MARGIN + (m.generation - minGen) * (NODE_H + GEN_GAP)

    // 配偶头像：紧贴成员头像右侧，每个配偶头像后跟其名字区，依次右移
    const spNodes = spList.map((s, si) => {
      const spName = s.name || (s.surname || '') + (s.given_name || '')
      const nameChars = toChars(spName)
      const typeChars = toChars(s.type)
      // 竖排文字块：首字 baseline 补偿 12px，使姓氏文字顶部与头像顶部对齐
      const startY = y + 12
      return {
        id: `sp_${m.id}_${s.id}`,
        spouseId: s.id,
        name: spName,
        type: s.type,
        portrait: s.portrait || '',
        birth_gregorian: s.birth_gregorian || '',
        death_gregorian: s.death_gregorian || '',
        occupation: s.occupation || '',
        education: s.education || '',
        has_bio: !!s.has_bio,
        x: cx + AVATAR / 2 + si * (AVATAR + NAME_W + GAP_S),
        y: y,
        nameChars,
        typeChars,
        textStartY: startY
      }
    })

    const nameChars = toChars(displayName)
    const genLabel = m.record_type === '外纪' ? '' : (m.generation_word || `第${m.generation}世`)
    const subChars = toChars(genLabel)
    // 竖排文字块：首字 baseline 补偿 12px，使姓氏文字顶部与头像顶部对齐
    const startY = y + 12

    result.push({
      ...m, x: cx, y, unitW,
      dead: !!m.death_gregorian,
      sub: m.record_type === '外纪' ? '' : (m.generation_word || `第${m.generation}世`),
      highlight: false,
      displayName,
      nameChars,
      subChars,
      textStartY: startY,
      spouseNodes: spNodes
    })
  })

  // 4.5 外纪世代编号：外纪一世、外纪二世...
  const waijiNodes = result.filter(n => n.record_type === '外纪')
  if (waijiNodes.length > 0) {
    const waijiMinGen = Math.min(...waijiNodes.map(n => n.generation))
    waijiNodes.forEach(n => {
      const wjNum = n.generation - waijiMinGen + 1
      n.sub = `外纪${wjNum}世`
      n.subChars = toChars(`外纪${wjNum}世`)
    })
  }

  // 4.6 外纪/内纪分界：在内纪节点前插入间距
  const hasWaiji = result.some(n => n.record_type === '外纪')
  const hasNeiji = result.some(n => n.record_type === '内纪')
  let boundaryY = 0
  if (hasWaiji && hasNeiji) {
    const waijiBottom = Math.max(...result.filter(n => n.record_type === '外纪').map(n => n.y + NODE_H))
    const neijiTop = Math.min(...result.filter(n => n.record_type !== '外纪').map(n => n.y))
    // 外纪延伸到内纪世代区域时，动态增加间距（避免虚线贴到内纪头像）
    const overlap = waijiBottom - neijiTop  // >0 表示外纪伸入内纪领空
    const totalGap = Math.max(WAIGI_GAP, overlap * 2 + WAIGI_GAP)
    boundaryY = waijiBottom + totalGap / 2
    waijiBoundaryY.value = boundaryY
    result.forEach(n => {
      if (n.record_type !== '外纪') {
        n.y += totalGap
        n.textStartY += totalGap
        n.spouseNodes.forEach(sn => {
          sn.y += totalGap
          sn.textStartY += totalGap
        })
      }
    })
  } else {
    waijiBoundaryY.value = 0
  }

  // 5. 整体水平居中（以单元左边界到右边界为准）
  const minX = Math.min(...result.map(n => n.x - NAME_W - AVATAR / 2))
  const maxX = Math.max(...result.map(n => n.x - NAME_W - AVATAR / 2 + n.unitW))
  const treeW = maxX - minX
  const wrapW = wrap.value ? wrap.value.clientWidth : 0
  const canvasW = Math.max(treeW + SIDE_MARGIN * 2, wrapW, 1000)
  const offset = (canvasW - treeW) / 2 - minX
  result.forEach(n => { n.x += offset; n.spouseNodes.forEach(sn => sn.x += offset) })

  // 6. 画布高度
  const maxY = Math.max(...result.map(n => n.y + NODE_H), 0) + 60
  const canvasH = Math.max(maxY, 400)

  // 内容尺寸（供自适应缩放/导出使用）；SVG 视口尺寸在 fitToView 里按可视区设置
  contentW.value = canvasW
  contentH.value = canvasH

  return result
})

// 无嗣 / 迁居失联 标注（画在成员头像正下方）
const markers = computed(() => {
  const res = []
  nodes.value.forEach(n => {
    const noKids = !n.children || !n.children.length
    const y = n.y + NODE_H + 8
    const from = n.y + AVATAR
    if (n.lost_contact) {
      const lines = lostContactText(memberKnown(n), !noKids, false).split('，')
      const maxLen = Math.max(...lines.map(l => l.length))
      res.push({ type: 'lost', id: 'lost_' + n.id, x: n.x, y, from, lines,
        fs: 12, lineH: 16, boxW: Math.max(60, maxLen * 13 + 16), boxH: lines.length * 16 + 10 })
    } else if (n.no_heir && noKids) {
      res.push({ type: 'noheir', id: 'noheir_' + n.id, x: n.x, y, from, lines: [NO_HEIR],
        fs: 17, lineH: 18, boxW: 60, boxH: 30 })
    }
  })
  return res
})

// 所有配偶节点扁平列表（用于 defs clipPath）
const allSpouseNodes = computed(() => {
  const arr = []
  nodes.value.forEach(n => { n.spouseNodes.forEach(sn => arr.push(sn)) })
  return arr
})

// 连线：正交垂线（父底→竖线→横线串兄弟→各子竖线），避免交叉
const edges = computed(() => {
  const map = {}
  nodes.value.forEach(n => { map[n.id] = n })
  const result = []
  nodes.value.forEach(n => {
    if (!n.children || !n.children.length) return
    // 父头像中心（n.x 即中心）
    const fx = n.x
    const fyBottom = n.y + AVATAR
    const childTops = n.children.map(k => {
      const kn = map[k.id]
      return kn ? kn.x : fx
    })
    if (n.children.length === 1) {
      const kn = map[n.children[0].id]
      const childY = kn ? kn.y - 4 : n.y + NODE_H + GEN_GAP - 4
      result.push({ d: `M${fx} ${fyBottom} L${fx} ${childY}` })
    } else {
      const midY = n.y + NODE_H + GEN_GAP / 2
      const minX = Math.min(...childTops)
      const maxX = Math.max(...childTops)
      result.push({ d: `M${fx} ${fyBottom} L${fx} ${midY}` })
      result.push({ d: `M${minX} ${midY} L${maxX} ${midY}` })
      n.children.forEach(k => {
        const kn = map[k.id]
        if (kn) {
          const cx = kn.x
          result.push({ d: `M${cx} ${midY} L${cx} ${kn.y - 4}` })
        }
      })
    }
  })
  // 外纪/内积分界线（虚线）
  if (waijiBoundaryY.value > 0) {
    const sepX1 = Math.min(...nodes.value.map(n => n.x - NAME_W - AVATAR / 2))
    const sepX2 = Math.max(...nodes.value.map(n => n.x - NAME_W - AVATAR / 2 + n.unitW))
    result.push({ d: `M${sepX1} ${waijiBoundaryY.value} L${sepX2} ${waijiBoundaryY.value}`, dashed: true, sep: true })
  }
  return result
})

async function load() {
  const { data } = await api.get('/members/with-spouses')
  members.value = data
  try {
    const { data: fsData } = await api.get('/settings/family-surname')
    familySurname.value = fsData.family_surname || ''
  } catch (e) { /* ignore */ }
  try {
    const { data: brData } = await api.get('/branches')
    branches.value = brData || []
  } catch (e) { /* ignore */ }
  await nextTick()
  fitToView()
}

function onBranchChange() {
  const v = branchFilter.value
  if (v === 'all' || v === 'unassigned') {
    extraShowIds.value = []
    applyBranchView()
    return
  }
  const desc = findUnassignedDescendants(members.value, v)
  if (!desc.length) {
    extraShowIds.value = []
    applyBranchView()
    return
  }
  unassignedDesc.value = desc
  pendingBranchId.value = v
  pendingBranchName.value = (branches.value.find(b => b.id === v) || {}).name || '该房支'
  branchPromptVisible.value = true
}

function branchPromptChoose(action) {
  branchPromptVisible.value = false
  if (action === 'join') {
    joinUnassigned()
  } else if (action === 'show') {
    extraShowIds.value = unassignedDesc.value.map(m => m.id)
    applyBranchView()
  } else {
    extraShowIds.value = []
    applyBranchView()
  }
}

function applyBranchView() {
  clearLock()
  nextTick(fitToView)
}

async function joinUnassigned() {
  const bid = pendingBranchId.value
  const ids = unassignedDesc.value.map(m => m.id)
  try {
    await api.post('/branches/set-members', { branch_id: bid, member_ids: ids, replace: false })
    ElMessage.success(`已将 ${ids.length} 名后代归入房支`)
    const { data } = await api.get('/members/with-spouses')
    members.value = data || []
  } catch (e) {
    ElMessage.error('归入房支失败：' + (e.response?.data?.error || e.message))
  }
  extraShowIds.value = []
  applyBranchView()
}

function showMember(n) { Object.assign(current, n); dialogVisible.value = true }
function goDetail(m) { dialogVisible.value = false; router.push(`/member/${m.id}`) }
function showSpouseDetail(sn) {
  currentSpouse.value = sn
  spouseDialogVisible.value = true
}

// 计算年龄（周岁/虚岁），接受成员或配偶对象；无出生日期返回空
function calcAge(o) {
  const b = (o?.birth_gregorian || '').trim()
  if (!b) return ''
  let by, bm = 1, bd = 1
  const mFull = b.match(/^(\d{4})-(\d{1,2})-(\d{1,2})/)
  const mYear = b.match(/^(\d{4})$/)
  if (mFull) { by = parseInt(mFull[1]); bm = parseInt(mFull[2]); bd = parseInt(mFull[3]) }
  else if (mYear) { by = parseInt(mYear[1]) }
  else return ''
  let cy, cm, cd
  const d = (o?.death_gregorian || '').trim()
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
function goSpouseDetail(sn) { spouseDialogVisible.value = false; router.push(`/spouse/${sn.spouseId}`) }
function editMember(m) {
  const copy = JSON.parse(JSON.stringify(m))
  Object.assign(editForm, copy)
  if (!editForm.surname) editForm.surname = familySurname.value
  editForm.birth_calendar = editForm.birth_calendar || 'solar'
  editForm.birth_date = editForm.birth_date || editForm.birth_gregorian || ''
  dialogVisible.value = false
  editVisible.value = true
}

async function saveMember() {
  await convertDate()  // 确保转换已执行
  const payload = { ...editForm }
  payload.name = (payload.surname || '') + (payload.given_name || '')
  // 卒年规范化：留空=在世；"已逝"等标记转存 death_era
  normalizeDeathFields(payload)
  // birth_gregorian 已由 convertDate 设置
  delete payload.birth_date
  delete payload.birth_calendar
  let r
  if (editForm.id) { r = await api.put(`/members/${editForm.id}`, payload) }
  else { r = await api.post('/members', payload) }
  if (r.data && r.data.draft) { editVisible.value = false; ElMessage.success(r.data.message || '已提交审核'); return }
  editVisible.value = false
  if (r?.data?.account_password) {
    acctPw.value = { visible: true, name: r.data.name || editForm.name || '', password: r.data.account_password }
  } else {
    ElMessage.success('已保存')
  }
  load()
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

function doSearch() {
  // 手机端：先让搜索框失焦收起键盘，避免键盘弹出压缩可视区导致定位偏、键盘收起后 resize 重置
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur()
  const kw = search.value.trim()
  if (!kw) return
  const target = visibleMembers.value.find(m => {
    const name = m.name || (m.surname || '') + (m.given_name || '')
    return name.includes(kw) || m.zi?.includes(kw) || m.hao?.includes(kw)
  })
  if (!target) { ElMessage.warning('未找到匹配成员'); return }
  const node = nodes.value.find(n => n.id === target.id)
  if (node) {
    // 定位：放大并把对应人物居中（不再直接打开资料卡）。
    // 用统一的居中函数，避免 pan.y 漏乘 zoom 导致第一次点击错位、要点两下。
    centerOnPoint(node.x, node.y + AVATAR / 2)
  }
}

// 把内容坐标点 (cx,cy) 居中到可视区并放大；zoom 与 pan 同步计算，保证一次到位
// 同时记录锁定点：之后窗口 resize（手机键盘收起/转屏）会对同一目标重新居中，
// 而不是被 fitToView 重置回整树（修复手机不關键盘点定位后弹回）。
let lockPoint = null
function centerOnPoint(cx, cy, targetZoom = 2) {
  const vw = wrap.value?.clientWidth || svgW.value || 1
  const vh = wrap.value?.clientHeight || svgH.value || 1
  svgW.value = vw; svgH.value = vh
  zoom.value = targetZoom
  pan.x = vw / 2 - cx * targetZoom
  pan.y = vh / 2 - cy * targetZoom
  lockPoint = { cx, cy, zoom: targetZoom }
}
function clearLock() { lockPoint = null }

function zoomIn() { zoom.value = Math.min(zoom.value * 1.2, 3) }
function zoomOut() { zoom.value = Math.max(zoom.value / 1.2, 0.1) }
function resetZoom() { clearLock(); fitToView() }

// 将整棵世系图缩放并居中到可视区域内（解决画布边界截断问题）
function fitToView() {
  if (!wrap.value) return
  const vw = wrap.value.clientWidth || 1
  const vh = wrap.value.clientHeight || 1
  // SVG 视口 = 可视区尺寸，避免内容被 SVG 自身边界裁切（原 bug：svgH/svgW 被设成内容尺寸）
  svgW.value = vw
  svgH.value = vh
  const pad = 24
  const cw = contentW.value || 1
  const ch = contentH.value || 1
  const scale = Math.min((vw - pad * 2) / cw, (vh - pad * 2) / ch)
  zoom.value = Math.min(Math.max(scale, 0.1), 3)
  pan.x = (vw - cw * zoom.value) / 2
  pan.y = (vh - ch * zoom.value) / 2
}
function onWheel(e) {
  clearLock() // 手动缩放，解除定位锁定
  // 以鼠标悬停位置为焦点缩放：保持鼠标下的内容点不动
  const rect = wrap.value.getBoundingClientRect()
  const sx = e.clientX - rect.left
  const sy = e.clientY - rect.top
  const newZoom = Math.min(Math.max(zoom.value * (e.deltaY > 0 ? 0.9 : 1.1), 0.1), 3)
  const ratio = newZoom / zoom.value
  pan.x = sx - (sx - pan.x) * ratio
  pan.y = sy - (sy - pan.y) * ratio
  zoom.value = newZoom
}

// 多点触控状态
let activePointers = new Map()    // pointerId -> { sx, sy, cx, cy } (start + current)
let dragDist = 0
let startPanX = 0, startPanY = 0
let pinchMidSX = 0, pinchMidSY = 0   // 双指中点初始屏幕坐标
let pinchDist = 0
let pinchStartZoom = 1
let multiTouch = false   // 本次手势是否出现过双指（用于禁止双指缩放后的误点击）
let suppressClick = false   // 本次手势是否发生过拖动（用于禁止拖动后的误点击）

function onPointerDown(e) {
  clearLock() // 用户开始拖拽/双指操作，视为手动浏览，解除定位锁定
  activePointers.set(e.pointerId, { sx: e.clientX, sy: e.clientY, cx: e.clientX, cy: e.clientY })
  if (activePointers.size >= 2) multiTouch = true
  if (activePointers.size === 1) {
    suppressClick = false
    dragDist = 0
    startPanX = pan.x
    startPanY = pan.y
  }
  if (activePointers.size === 2) {
    const pts = [...activePointers.values()]
    pinchDist = Math.hypot(pts[1].sx - pts[0].sx, pts[1].sy - pts[0].sy)
    pinchMidSX = (pts[0].sx + pts[1].sx) / 2
    pinchMidSY = (pts[0].sy + pts[1].sy) / 2
    pinchStartZoom = zoom.value
  }
  e.target.setPointerCapture(e.pointerId)
}
function onPointerMove(e) {
  if (!activePointers.has(e.pointerId)) return
  // 更新当前坐标
  const entry = activePointers.get(e.pointerId)
  entry.cx = e.clientX; entry.cy = e.clientY

  const pts = [...activePointers.values()]
  if (activePointers.size >= 2) {
    const d = Math.hypot(pts[1].cx - pts[0].cx, pts[1].cy - pts[0].cy)
    const midX = (pts[0].cx + pts[1].cx) / 2
    const midY = (pts[0].cy + pts[1].cy) / 2
    const scale = pinchDist > 0 ? d / pinchDist : 1
    const newZoom = Math.min(Math.max(pinchStartZoom * scale, 0.1), 3)
    // 以双指中点为焦点缩放：保持中点对应的 SVG 位置不变
    const ratio = newZoom / zoom.value
    pan.x = midX - (midX - pan.x) * ratio
    pan.y = midY - (midY - pan.y) * ratio
    zoom.value = newZoom
    pinchMidSX = midX
    pinchMidSY = midY
    dragDist = 999
  } else if (activePointers.size === 1) {
    const p = pts[0]
    const dx = p.cx - p.sx
    const dy = p.cy - p.sy
    dragDist = Math.sqrt(dx * dx + dy * dy)
    pan.x = startPanX + dx
    pan.y = startPanY + dy
  }
}
function onPointerUp(e) {
  const wasSize = activePointers.size
  const dragged = dragDist >= 3   // 拖动后抑制紧随的原生 click
  activePointers.delete(e.pointerId)
  if (dragged) suppressClick = true
  if (wasSize === 1 && dragDist < 3 && !multiTouch) {
    const el = document.elementFromPoint(e.clientX, e.clientY)
    if (el) {
      el.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: e.clientX, clientY: e.clientY }))
    }
  }
  if (activePointers.size === 0) {
    dragDist = 0
    multiTouch = false
  } else if (activePointers.size === 1) {
    // 双指→单指：新起点
    const p = [...activePointers.values()][0]
    dragDist = 0
    startPanX = pan.x
    startPanY = pan.y
    // 更新剩余指针的起始坐标
    const key = [...activePointers.keys()][0]
    activePointers.set(key, { sx: p.cx, sy: p.cy, cx: p.cx, cy: p.cy })
  }
}

// 计算导出裁剪区（内纪/外纪只做纵向裁剪，横向取整幅）；返回 {x,y,w,h} 或 null
function exportCrop() {
  const mode = exportMode.value
  if (mode !== 'neiji' && mode !== 'waiji') return null
  const filtered = nodes.value.filter(n => mode === 'neiji' ? n.record_type !== '外纪' : n.record_type === '外纪')
  if (filtered.length === 0) { ElMessage.warning('没有可导出的内容'); return 'empty' }
  const minY = Math.min(...filtered.map(n => n.y))
  const maxY = Math.max(...filtered.map(n => n.y + NODE_H))
  return { x: 0, y: minY - 20, w: contentW.value, h: maxY - minY + 40 }
}
const exportNames = { all: '世系图', neiji: '世系图-内纪', waiji: '世系图-外纪' }

// ===== 分页打印（A3/A4 · 卷轴）=====
// 只把「世代行」和「房支/卷册」交给通用分页模块；分页算法在 utils/printPaging.js，
// 屏幕预览（整张显示 / 缩放拖动）完全不受影响。
const printDlgVisible = ref(false)
const printRows = computed(() => {
  const items = nodes.value || []
  const wj = items.filter(n => n.record_type === '外纪')
  const wjMin = wj.length ? Math.min(...wj.map(n => n.generation)) : 0
  return rowsFromNodes(items, {
    y: n => n.y,
    h: () => NODE_H,
    label: n => n.record_type === '外纪'
      ? `外纪${n.generation - wjMin + 1}世`
      : `第${n.generation}世${n.generation_word ? '（' + n.generation_word + '）' : ''}`
  })
})
const printVolume = computed(() => {
  const v = branchFilter.value
  if (v === 'unassigned') return '未分房'
  if (v !== 'all') {
    const b = branches.value.find(x => x.id === v)
    return b ? b.name : ''
  }
  if (branchCtx.current) {
    const b = branchCtx.branches.find(x => x.id === branchCtx.current) || branches.value.find(x => x.id === branchCtx.current)
    if (b) return b.name + (branchCtx.includeChildren ? '（含下级）' : '')
  }
  return '宗谱（全部）'
})
// 内纪/外纪裁剪（与导出下拉的「仅内纪/仅外纪」一致），纯计算、不弹提示
const printBounds = computed(() => {
  const mode = exportMode.value
  if (mode !== 'neiji' && mode !== 'waiji') return null
  const filtered = nodes.value.filter(n => mode === 'neiji' ? n.record_type !== '外纪' : n.record_type === '外纪')
  if (!filtered.length) return null
  const minY = Math.min(...filtered.map(n => n.y))
  const maxY = Math.max(...filtered.map(n => n.y + NODE_H))
  return { x: 0, y: minY - 20, w: contentW.value, h: maxY - minY + 40 }
})
const exporting = ref(false)

// 实际落盘的 DPI = 导出像素宽 ÷ 裁剪后内容宽 × 96（大树会被 MAX_CANVAS_PX=14000 上限自动降倍率）
function pngActualDpi(r, crop) {
  const cw = (crop && crop.w) ? crop.w : contentW.value
  return Math.round(r.width / cw * 96)
}

// 导出「内嵌头像」前：把头切到原图并等图片就绪（导出后由 doExport 恢复原状态）
async function loadFullPortraits() {
  if (portraitLoad.value !== 'full') { portraitLoad.value = 'full'; await nextTick() }
  const urls = [...new Set((nodes.value || []).filter(n => n.portrait).map(n => n.portrait))]
  const load = Promise.all(urls.map(u => new Promise(res => {
    const im = new Image()
    im.onload = im.onerror = () => res()
    im.src = u
  })))
  await Promise.race([load, new Promise(res => setTimeout(res, 20000))])
  await nextTick()
  await new Promise(res => requestAnimationFrame(() => requestAnimationFrame(res)))
}

// 导出分档：PNG 高清、矢量 SVG（可按需内嵌头像/字体）、打印 PDF
async function doExport(cmd) {
  if (exporting.value) return
  if (cmd === 'pagedPrint') { printDlgVisible.value = true; return }
  const crop = exportCrop()
  if (crop === 'empty') return
  exporting.value = true
  const mode = exportMode.value
  const name = exportNames[mode] || '世系图'
  const wantPortrait = cmd === 'svgPortrait' || cmd === 'svgPortraitFont'
  const wantFont = cmd === 'svgFont' || cmd === 'svgPortraitFont'
  const prevPortrait = portraitLoad.value
  try {
    if (cmd === 'png300') {
      const r = await exportHighResPng(svg.value, contentW.value, contentH.value, `${name}-300dpi.png`, '#f7f3ec', crop, undefined, { treeFont: true })
      const dpi = pngActualDpi(r, crop)
      ElMessage.success(`已导出 PNG 高清 300DPI（${r.width}×${r.height}px，实际 ≈ ${dpi} DPI${dpi < 294 ? '，受 14000px 上限限制已自动降倍率' : ''}）`)
    } else if (cmd === 'svg' || wantPortrait || wantFont) {
      if (wantPortrait) await loadFullPortraits()
      const suffix = cmd === 'svg' ? '' : (wantPortrait && wantFont ? '-内嵌头像字体' : (wantPortrait ? '-内嵌头像' : '-内嵌字体'))
      const r = await exportVectorSvg(svg.value, contentW.value, contentH.value, `${name}${suffix}.svg`, crop,
        { withPortrait: wantPortrait, withFont: wantFont, treeFont: true })
      const kb = Math.max(1, Math.round(r.size / 1024))
      if (wantFont) {
        const f = r.font || {}
        ElMessage.success(`已导出 SVG（${wantPortrait ? '内嵌头像 + ' : ''}内嵌字体 · 子集 ${f.chars || 0} 字 · ${kb}KB${f.missing ? ' · ' + f.missing + ' 字缺字形' : ''}）`)
      } else if (wantPortrait) {
        ElMessage.success(`已导出 SVG（内嵌头像 · ${kb}KB）`)
      } else {
        ElMessage.success(`已导出 SVG（矢量图 · ${kb}KB）`)
      }
    } else if (cmd === 'pdf') {
      await exportPrintPDF(svg.value, contentW.value, contentH.value, `${name}.pdf`, '#f7f3ec', crop, { treeFont: true })
      ElMessage.success('已打开打印视图（在打印对话框选择「另存为 PDF」）')
    }
  } catch (e) {
    if (e && (e.code === 'FONT_NONE' || e.code === 'FONT_UNSUPPORTED' || e.code === 'PORTRAIT_NONE')) ElMessage.warning(e.message)
    else ElMessage.error('导出失败：' + (e?.message || e))
  } finally {
    if (portraitLoad.value !== prevPortrait) portraitLoad.value = prevPortrait
    exporting.value = false
  }
}

function updateSvgSize() {
  // 若处于搜索定位锁定状态（常见于手机键盘收起触发 resize），
  // 按新可视区尺寸对同一目标重新居中，保持定位结果；否则才自适应整树。
  if (lockPoint) {
    centerOnPoint(lockPoint.cx, lockPoint.cy, lockPoint.zoom)
  } else {
    fitToView()
  }
}

// 捕获阶段拦截 click：拖动后吞掉原生 click，避免误弹资料卡
function onCaptureClick(e) {
  if (suppressClick) {
    e.stopPropagation()
    e.preventDefault()
    suppressClick = false
  }
}

onMounted(() => {
  load()
  const el = wrap.value
  el.addEventListener('pointerdown', onPointerDown)
  el.addEventListener('pointermove', onPointerMove)
  el.addEventListener('pointerup', onPointerUp)
  el.addEventListener('pointerleave', onPointerUp)
  el.addEventListener('click', onCaptureClick, true)
  window.addEventListener('resize', updateSvgSize)
  updateSvgSize()
})

// 传记弹窗：点击世系图上的金色卷轴标记
const bioPopup = ref(null)
function openBio(n) {
  if (bioPopup.value) bioPopup.value.open({ member_id: n.id, name: n.displayName || n.name || '' })
}
function openBioSpouse(s) {
  if (bioPopup.value) bioPopup.value.open({ spouse_id: s.id, name: s.name || '' })
}
</script>

<style scoped>
/* 树图字体选择：作用到本视图 SVG 里的文字（CSS 规则优先于 <text> 的 font-family 表现属性） */
svg.tf-on text, svg.tf-on tspan { font-family: var(--tree-font); }
.tree-view { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.hint { margin-left: auto; color: #8c5a3c; font-size: 12px; }
.hint-noportrait { margin-left: auto; color: #e6a23c; font-size: 12px; font-weight: bold; }
@media (max-width: 767px) { .hint, .hint-noportrait { display: none; } }
.canvas-wrap { flex: 1; overflow: hidden; border: 1px solid var(--border); border-radius: 8px; background: #fff; cursor: grab; touch-action: none; user-select: none; }
.tree-svg { display: block; }
.node { cursor: pointer; }
.node-name { font-size: 13px; font-weight: bold; font-family: "Songti SC", "SimSun", serif; }
.node-sub { font-size: 10px; }
.member-card .row { display: flex; margin-bottom: 8px; }
.member-card .label { width: 70px; color: #8c5a3c; flex-shrink: 0; }
.member-card em { color: #8c5a3c; font-style: normal; margin-left: 6px; }
.dialog-header { text-align: center; margin-bottom: 12px; }
.sp-link { color: #3a3226; text-decoration: none; }
.sp-link:hover { color: #8c5a3c; text-decoration: underline; }
.bp-body { font-size: 14px; line-height: 1.7; }
.bp-body .bp-sub { color: #666; margin: 4px 0 0; }
.bio-mark { cursor: pointer; }
.bio-mark:hover rect { filter: brightness(1.08); }
</style>