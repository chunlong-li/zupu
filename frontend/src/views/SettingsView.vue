<template>
  <div class="settings-page">
    <el-card class="section">
      <template #header><span>家族信息</span></template>
      <el-form label-width="100px" class="settings-form">
        <el-form-item label="家族姓氏">
          <el-input v-model="form.surname" placeholder="例如：李" @input="onSurnameChange" />
        </el-form-item>
        <el-form-item label="家族名称">
          <el-input v-model="form.family_name" placeholder="自动生成" />
        </el-form-item>
        <el-form-item label="堂口">
          <el-input v-model="form.tanghao" placeholder="如：陇西堂（选填）" clearable />
          <span class="form-hint">宗族堂号/堂口，可不填；祠堂牌匾中间优先显示堂号，未填则显示姓氏</span>
        </el-form-item>
        <el-form-item label="牌匾样式">
          <el-select v-model="form.totem_style" style="width: 200px">
            <el-option label="木质金匾" value="wood" />
            <el-option label="红底金字" value="red" />
            <el-option label="蓝底金字" value="blue" />
            <el-option label="石刻灰底" value="stone" />
          </el-select>
          <span class="form-hint">祠堂图腾牌匾的底色样式</span>
        </el-form-item>
        <el-form-item label="图腾字号">
          <el-slider v-model="form.totem_font_size" :min="80" :max="300" :step="10" show-input style="width: 320px" />
          <span class="form-hint">祠堂牌匾居中大字（姓氏/堂号）的字号，默认 200</span>
        </el-form-item>
        <el-form-item label="图腾底图">
          <el-upload :show-file-list="false" :before-upload="beforeTotemUpload" accept="image/*">
            <el-button size="small">上传底图</el-button>
          </el-upload>
          <img v-if="form.totem_image" :src="form.totem_image" class="totem-preview" />
          <el-button v-if="form.totem_image" size="small" text type="danger" @click="form.totem_image = ''">清除</el-button>
          <span class="form-hint">作为牌匾底纹（文字叠加其上），透明底 PNG 最佳</span>
        </el-form-item>
        <el-form-item label="郡望">
          <el-input v-model="form.junwang" placeholder="如：陇西郡（选填）" clearable />
        </el-form-item>
        <el-form-item label="衍派">
          <el-input v-model="form.branch_designation" placeholder="如：陇西衍派（选填，闽台常用）" clearable />
          <span class="form-hint">闽台地区常见宗族标识，灯号/衍派，如"陇西衍派"，可不填</span>
        </el-form-item>
        <el-form-item label="祖籍">
          <el-input v-model="form.zuji" placeholder="如：陇西成纪（选填）" clearable />
        </el-form-item>
        <el-form-item label="修次">
          <el-select v-model="form.xiuci" style="width: 140px" filterable allow-create default-first-option>
            <el-option v-for="o in XIUCI_OPTS" :key="o" :label="o" :value="o" />
          </el-select>
          <span class="form-hint">封面「歲次…X」与牌记版次都用它（可直接输入自定义，如「創修」）；默认「重修」</span>
        </el-form-item>
        <el-form-item label="题名含修次">
          <el-switch v-model="form.title_with_xiuci" />
          <span class="form-hint">开启后谱名如「李氏創修族譜」（默认关，避免改变现有外观）</span>
        </el-form-item>
        <el-form-item label="牒记式姐妹">
          <el-switch v-model="form.dieji_show_sisters" />
          <span class="form-hint">竖排牒记式中，本支（内纪）人物条目是否列出同父亲的姐/妹。默认开；遵循传统谱法（本支只记男丁世系）可关闭。</span>
        </el-form-item>
        <el-form-item label="垂珠体女儿">
          <el-switch v-model="form.chuizhu_show_daughters" />
          <span class="form-hint">苏氏垂珠体中是否显示女儿。默认开；关闭后女儿整支不显示（只走男丁世系）。女儿照常列出其子女（含招赘续房一支）。</span>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="section">
      <template #header><span>始祖设置</span></template>
      <el-alert type="info" :closable="false" class="ancestor-tip">
        始祖指宗族共同的开基祖先（一世祖）；始迁祖指迁入现居地的祖先（移民始祖），两者可为同一人，也可不同。信息将显示在谱书卷首。
      </el-alert>
      <el-form label-width="100px" class="settings-form">
        <el-form-item label="始祖名讳">
          <el-input v-model="form.shizu_name" placeholder="如：李XX（选填）" clearable />
        </el-form-item>
        <el-form-item label="始祖简介">
          <el-input v-model="form.shizu_bio" type="textarea" :rows="3" placeholder="始祖生平事迹简述（选填）" />
        </el-form-item>
        <el-form-item label="始迁祖名讳">
          <el-input v-model="form.shiqianzu_name" placeholder="如：李YY（选填，与始祖相同可留空）" clearable />
        </el-form-item>
        <el-form-item label="始迁地">
          <el-input v-model="form.shiqianzu_place" placeholder="如：由山西洪洞迁居山东济南（选填）" clearable />
        </el-form-item>
      </el-form>
    </el-card>

    <div class="actions">
      <el-button type="primary" @click="saveSettings">保存家族设置</el-button>
    </div>

    <!-- 用户权限 / 隐私与权限：按钮 → 弹窗 -->
    <div class="entry-buttons">
      <el-button type="primary" plain @click="openUsersDialog">👥 用户权限（家族职务分配）</el-button>
      <el-button v-if="isChief" type="primary" plain @click="openPrivacyDialog">🔒 隐私与权限</el-button>
    </div>

    <el-dialog v-model="usersDialog" title="👥 用户权限（家族职务分配）" width="min(960px, 94vw)" top="6vh" append-to-body class="users-dlg">
      <el-alert type="info" :closable="false" class="role-legend">
        <div class="legend-grid">
          <div class="legend-item"><span class="role-tag role-visitor">游客</span>未登录，仅可查看数据看板</div>
          <div class="legend-item"><span class="role-tag role-member">{{ displayRole('族员') }}</span>可查看所有数据</div>
          <div class="legend-item"><span class="role-tag role-elder">{{ displayRole('理事') }}</span>可添加、修改成员数据</div>
          <div class="legend-item"><span class="role-tag role-elder-branch">{{ displayRole('房支理事') }}</span>理事分配支系后自动升级，仅管理所属支系</div>
          <div class="legend-item"><span class="role-tag role-finance">{{ displayRole('财务管理') }}</span>收支公示职位：审核账目、管理账本</div>
          <div class="legend-item"><span class="role-tag role-chief">{{ displayRole('理事长') }}</span>可任命/罢免理事，并拥有以上权限</div>
          <div class="legend-item"><span class="role-tag role-super">{{ displayRole('超级管理员') }}</span>拥有所有权限，可任命/罢免理事长、理事</div>
        </div>
      </el-alert>

      <div class="toolbar">
        <el-button type="primary" @click="openCreate">➕ 新增用户</el-button>
        <el-button @click="openRoleAliases">✏️ 职位名称</el-button>
      </div>

      <el-table :data="users" border class="table-responsive">
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="username" label="用户名" width="140" />
        <el-table-column label="角色" width="140">
          <template #default="s">
            <el-tag :type="roleTagType(s.row.role)" effect="dark">{{ displayRole(s.row.role) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="branch" label="支系" />
        <el-table-column label="操作" width="400">
          <template #default="s">
            <el-button size="small" type="primary" @click="openSetRole(s.row)">任命/罢免</el-button>
            <el-button size="small" @click="openSetBranch(s.row)">设支系</el-button>
            <el-button size="small" type="warning" @click="openResetPw(s.row)">重置密码</el-button>
            <el-dropdown trigger="click" style="margin-left:4px">
              <el-button size="small" type="danger"><el-icon><ArrowDown /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-item>
                  <el-button size="small" type="danger" text @click="delUser(s.row)">删除</el-button>
                </el-dropdown-item>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- 隐私与权限（项目 × 职位 矩阵）：弹窗 -->
    <el-dialog v-model="privacyDialog" title="🔒 隐私与权限" width="min(1120px, 96vw)" top="6vh" append-to-body class="privacy-dlg">
      <!-- 总开关：取消隐私 · 全公开（默认关闭 = 保持隐私保护） -->
      <div class="public-all-box" :class="{ 'is-on': privacy.switches.public_all }">
        <div class="pa-head">
          <el-switch v-model="privacy.switches.public_all" size="large" />
          <span class="pa-title">取消隐私 · 全公开</span>
          <el-tag :type="privacy.switches.public_all ? 'danger' : 'info'" effect="dark" size="small">
            {{ privacy.switches.public_all ? '已全公开' : '默认关闭 · 隐私保护中' }}
          </el-tag>
        </div>
        <div v-if="privacy.switches.public_all" class="pa-warn">
          ⚠️ 打开后，所有成员（含在世者）的个人信息——生卒日期 · 籍贯/现居地 · 手机号 · DNA · 职业学历 · 生平 · 葬地 · 配偶信息等——
          将对<b>所有登录用户</b>完全公开，不再脱敏，也不再受「五服」与职位门限限制。
          下方的「信息可见性」矩阵与五服 / 已故开关<b>暂时不生效</b>（已置灰）；「操作权限」仍按矩阵执行。
        </div>
        <div v-else class="pa-hint">
          默认关闭 = 保持现有隐私保护（按下表矩阵 + 五服 + 已故开关脱敏）。
          打开 = 取消隐私、全部公开；只对<b>已登录用户</b>生效，未登录游客仍按「游客」列受限。
          本开关保存在服务端设置，换设备/浏览器一致。
        </div>
      </div>

      <el-alert type="info" :closable="false" class="role-legend">
        按「项目 × 职位」打钩：勾选=该职位可见/可用。手机号·DNA·配偶手机号可选「完整/打码/隐藏」（打码如 138****1234）。
        世系框架与超级管理员为锁定项（不可取消，避免把自己锁死）。
      </el-alert>

      <div class="privacy-switches">
        <div class="ps-item">
          <el-switch v-model="privacy.switches.five_fu" :disabled="privacy.switches.public_all" />
          <span class="ps-label">启用「五服」近亲分级</span>
          <span class="ps-hint">族员 / 房支理事（非本房支）按亲属距离细分：五服内可见生卒等信息，五服外只看世系框架。关闭 = 该列整体放开。</span>
          <span v-if="privacy.switches.public_all" class="ps-off">（全公开时不生效）</span>
        </div>
        <div class="ps-item">
          <el-switch v-model="privacy.switches.dead_public" :disabled="privacy.switches.public_all" />
          <span class="ps-label">逝世者信息一律公开</span>
          <span class="ps-hint">在世者按上表脱敏；已故者其生卒 / 生平 / 葬地等对五服内成员完全可见（DNA 除外）。</span>
          <span v-if="privacy.switches.public_all" class="ps-off">（全公开时不生效）</span>
        </div>
      </div>

      <el-alert v-if="privacy.switches.public_all" type="warning" :closable="false" class="matrix-off-tip">
        全公开已打开：「信息可见性」矩阵不生效（已置灰）；「操作权限」仍按矩阵生效。关闭总开关后矩阵立即恢复作用。
      </el-alert>

      <el-table :data="privacy.items" border size="small" class="privacy-table" :row-class-name="privacyRowClass">
        <el-table-column prop="label" label="项目" :width="isMobile ? 120 : 240" fixed />
        <el-table-column v-for="col in privacy.columns" :key="col" :label="privacy.role_names[col] || col" align="center" min-width="84">
          <template #default="s">
            <el-select v-if="s.row.level" :model-value="s.row.values[col]" size="small"
              :disabled="matrixDisabled(s.row, col)" style="width:76px"
              @change="v => (s.row.values[col] = v)">
              <el-option label="隐藏" :value="0" />
              <el-option label="打码" :value="1" />
              <el-option label="完整" :value="2" />
            </el-select>
            <el-checkbox v-else :model-value="s.row.values[col] > 0"
              :disabled="matrixDisabled(s.row, col)"
              @change="v => (s.row.values[col] = (v ? 2 : 0))" />
          </template>
        </el-table-column>
      </el-table>

      <div class="actions">
        <el-button type="primary" @click="savePrivacy">保存隐私与权限</el-button>
        <el-button @click="resetPrivacyDefaults">恢复默认设置</el-button>
      </div>
    </el-dialog>

    <!-- 设置支系对话框 -->
    <el-dialog v-model="branchVisible" title="设置支系" width="400px">
      <el-alert type="info" :closable="false" class="branch-tip">可为该用户分配一个房支（如长房、二房），选填。</el-alert>
      <el-form label-width="80px">
        <el-form-item label="理事">
          <el-input :value="branchUser?.username" readonly />
        </el-form-item>
        <el-form-item label="支系">
          <el-select v-model="branchForm.branch" clearable filterable placeholder="选择房支" style="width:100%">
            <el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.name" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="branchVisible = false">取消</el-button>
        <el-button type="primary" @click="doSetBranch">确定</el-button>
      </template>
    </el-dialog>

    <!-- 新增用户 -->
    <el-dialog v-model="createVisible" title="新增用户" width="400px">
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="用户名">
          <el-input v-model="createForm.username" placeholder="登录用户名" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="createForm.password" type="password" show-password placeholder="留空 = 按姓名自动生成" />
          <div class="pw-tip">留空则按「用户名（姓名）」自动生成 12 位数字临时密码，创建后弹窗显示给你；成员首次登录须自行修改。</div>
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="createForm.role" style="width:100%">
            <el-option label="族员" value="族员" />
            <el-option label="理事" value="理事" />
            <el-option label="财务管理" value="财务管理" />
            <el-option label="理事长" value="理事长" />
            <el-option label="超级管理员" value="超级管理员" />
          </el-select>
        </el-form-item>
        <el-form-item label="支系" v-if="createForm.role === '理事' || createForm.role === '房支理事'">
          <el-select v-model="createForm.branch" clearable filterable placeholder="选择房支（可空）" style="width:100%">
            <el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.name" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="doCreate">确定</el-button>
      </template>
    </el-dialog>

    <!-- 新密码展示（新增/重置后弹出，可复制）-->
    <el-dialog v-model="pwVisible" title="密码" width="420px">
      <div class="pw-for">用户「{{ pwFor }}」的密码</div>
      <div class="pw-show">{{ pwText }}</div>
      <div class="pw-tip">请转告本人妥善保存；成员首次登录后会被要求修改密码。</div>
      <template #footer>
        <el-button @click="copyPw">复制</el-button>
        <el-button type="primary" @click="pwVisible = false">我已记下</el-button>
      </template>
    </el-dialog>

    <!-- 职位名称映射 -->
    <el-dialog v-model="roleAliasesVisible" title="职位名称映射" width="450px">
      <el-alert type="info" :closable="false" style="margin-bottom:16px">
        将系统默认职位名映射为家族使用的称呼。
        映射后所有界面中的职位名称会相应改变，但权限层级不变。
      </el-alert>
      <el-form label-width="100px">
        <el-form-item :label="'族员 →'">
          <el-input v-model="roleAliasForm['族员']" placeholder="族员" />
        </el-form-item>
        <el-form-item :label="'理事 →'">
          <el-input v-model="roleAliasForm['理事']" placeholder="理事" />
        </el-form-item>
        <el-form-item :label="'房支理事 →'">
          <el-input v-model="roleAliasForm['房支理事']" placeholder="房支理事" />
        </el-form-item>
        <el-form-item :label="'财务管理 →'">
          <el-input v-model="roleAliasForm['财务管理']" placeholder="财务管理" />
        </el-form-item>
        <el-form-item :label="'理事长 →'">
          <el-input v-model="roleAliasForm['理事长']" placeholder="理事长" />
        </el-form-item>
        <el-form-item label="超级管理员 →">
          <el-input v-model="roleAliasForm['超级管理员']" placeholder="超级管理员" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="roleAliasesVisible = false">取消</el-button>
        <el-button type="primary" @click="saveRoleAliasesConfig">保存</el-button>
      </template>
    </el-dialog>


  </div>

  <AvatarCropper v-model:visible="totemCropVisible" :src="totemCropSrc" title="裁剪图腾" confirm-text="设为底图" @crop="onTotemCrop" />
</template>

<script setup>
import { reactive, ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import api from '../api'
import { displayRole, saveRoleAliases } from '../utils/auth'
import AvatarCropper from '../components/AvatarCropper.vue'

// ===== 家族信息 + 始祖设置 =====
const form = reactive({
  surname: '', family_name: '', tanghao: '', junwang: '', branch_designation: '', zuji: '',
  shizu_name: '', shizu_bio: '', shiqianzu_name: '', shiqianzu_place: '',
  totem_image: '', totem_style: 'wood', totem_font_size: 200,
  dieji_show_sisters: true,
  chuizhu_show_daughters: true,
  xiuci: '重修',                 // 修次：創修/初修/二修/三修/四修/五修/重修/續修（或自定义）
  title_with_xiuci: false        // 题名含修次（默认不勾）→「李氏創修族譜」
})
const XIUCI_OPTS = ['創修', '初修', '二修', '三修', '四修', '五修', '重修', '續修']

// ===== 用户权限（家族职务分配） =====
const users = ref([])
const createVisible = ref(false)
const createForm = ref({ username: '', password: '', role: '族员', branch: '' })
const pwVisible = ref(false)
const pwText = ref('')
const pwFor = ref('')

const me = ref(null)

// ===== 手机端断点（用于表格列宽等自适应） =====
const isMobile = ref(typeof window !== 'undefined' && window.innerWidth <= 767)
function onWindowResize() { isMobile.value = window.innerWidth <= 767 }
if (typeof window !== 'undefined') window.addEventListener('resize', onWindowResize)
onBeforeUnmount(() => { if (typeof window !== 'undefined') window.removeEventListener('resize', onWindowResize) })

// ===== 用户权限 / 隐私与权限 弹窗 =====
const usersDialog = ref(false)
const privacyDialog = ref(false)
function openUsersDialog() { usersDialog.value = true; loadUsers() }
function openPrivacyDialog() { privacyDialog.value = true; loadPrivacy() }

// ===== 隐私与权限矩阵 =====
// public_all = 总开关「取消隐私 · 全公开」，默认关闭（服务端默认值同为 false）
const privacy = reactive({
  columns: [], role_names: {}, items: [],
  switches: { five_fu: true, dead_public: true, public_all: false },
  defaults: null // 服务端下发的系统默认值（恢复默认用）
})
function lockedCell(key, col) { return key === 'base' || col === '超级管理员' }
// 全公开时「信息可见性」矩阵整体置灰（操作权限组仍可编辑）
function matrixDisabled(row, col) {
  return lockedCell(row.key, col) || (privacy.switches.public_all && row.group === 'info')
}
function privacyRowClass({ row }) {
  return privacy.switches.public_all && row.group === 'info' ? 'row-matrix-off' : ''
}
async function loadPrivacy() {
  try {
    const { data } = await api.get('/settings/privacy-matrix')
    privacy.columns = data.columns || []
    privacy.role_names = data.role_names || {}
    privacy.items = data.items || []
    if (data.switches) Object.assign(privacy.switches, data.switches)
    privacy.defaults = data.defaults || null
  } catch (e) { /* 无权限时忽略 */ }
}
async function savePrivacy() {
  try {
    const items = {}
    for (const it of privacy.items) items[it.key] = it.values
    await api.put('/settings/privacy-matrix', { items, switches: privacy.switches })
    ElMessage.success('隐私与权限已保存')
    loadPrivacy()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  }
}
// 恢复默认设置：两段式 —— 只把弹窗表单恢复为系统默认值并提示，需再点「保存」才落库
async function resetPrivacyDefaults() {
  try {
    await ElMessageBox.confirm(
      '将把弹窗内全部隐私设置恢复为系统默认值：包括「取消隐私 · 全公开」总开关（恢复为关闭）、' +
      '「五服」/「逝世者公开」开关，以及「项目 × 职位」矩阵的每一个勾选项。\n\n' +
      '恢复后不会立即生效，需要再点「保存隐私与权限」才会写入服务端；在此之前可以点「取消」或自行改回。',
      '恢复默认设置',
      { type: 'warning', confirmButtonText: '恢复默认', cancelButtonText: '取消', dangerouslyUseHTMLString: false }
    )
  } catch (e) { return }
  const d = privacy.defaults
  if (!d) {
    ElMessage.error('未取到系统默认值，请关闭弹窗后重试')
    return
  }
  // 开关（总开关必须回到关闭）
  privacy.switches.five_fu = !!d.switches.five_fu
  privacy.switches.dead_public = !!d.switches.dead_public
  privacy.switches.public_all = !!d.switches.public_all
  // 矩阵每一项逐格回填默认值（默认值表来自服务端 defaultPrivacyConfig）
  for (const it of privacy.items) {
    const dv = d.items && d.items[it.key]
    if (!dv) continue
    for (const col of privacy.columns) {
      if (dv[col] === undefined) continue
      it.values[col] = dv[col]
    }
  }
  ElMessage.warning('已恢复默认值，点「保存隐私与权限」后生效')
}

const branches = ref([]) // 房支列表
const branchVisible = ref(false) // 设置支系弹窗
const branchUser = ref(null)
const branchForm = ref({ branch: '' })

const myRole = computed(() => me.value?.role || '')
const isChief = computed(() => myRole.value === '理事长' || myRole.value === '超级管理员')
const isSuperAdmin = computed(() => myRole.value === '超级管理员')

function roleTagType(role) {
  const map = { '游客': 'info', '族员': 'info', '理事': 'warning', '房支理事': 'warning', '财务管理': 'success', '理事长': '', '超级管理员': 'danger' }
  return map[role] || 'info'
}

function canModify(tUser) {
  // 超级管理员可以改任何人；理事长不能改理事长和超级管理员
  if (isSuperAdmin.value) return true
  if (isChief.value && tUser.role !== '理事长' && tUser.role !== '超级管理员') return true
  return false
}

async function saveRoleAliasesConfig() {
  try {
    const { data } = await api.put('/settings/role-aliases', { aliases: roleAliasForm.value })
    roleAliases.value = data.aliases || {}
    saveRoleAliases(data.aliases)
    ElMessage.success('职位别名已保存')
    roleAliasesVisible.value = false
    loadUsers()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '保存失败')
  }
}

function openRoleAliases() {
  roleAliasForm.value = { ...roleAliases.value }
  roleAliasesVisible.value = true
}

async function loadRoleAliasesConfig() {
  try {
    const { data } = await api.get('/settings/role-aliases')
    roleAliases.value = data.aliases || {}
  } catch (e) { /* ignore */ }
}

async function loadUsers() {
  try {
    const { data } = await api.get('/users')
    users.value = data
  } catch (e) { /* ignore */ }
}

async function loadBranches() {
  try {
    const { data } = await api.get('/branches')
    branches.value = data
  } catch (e) { /* ignore */ }
}

function openCreate() {
  createForm.value = { username: '', password: '', role: '族员', branch: '' }
  createVisible.value = true
}

async function doCreate() {
  if (!createForm.value.username) {
    ElMessage.warning('请填写用户名')
    return
  }
  try {
    const { data } = await api.post('/users', {
      username: createForm.value.username,
      password: createForm.value.password || '',
      role: createForm.value.role,
      branch: createForm.value.branch || ''
    })
    ElMessage.success('已创建')
    createVisible.value = false
    loadUsers()
    if (data && data.password) showPw(createForm.value.username, data.password)
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '创建失败')
  }
}

function showPw(name, pw) {
  pwFor.value = name
  pwText.value = pw
  pwVisible.value = true
}

async function copyPw() {
  try {
    await navigator.clipboard.writeText(pwText.value)
    ElMessage.success('已复制')
  } catch (e) {
    ElMessage.warning('复制失败，请手动记录')
  }
}

function openSetRole(user) {
  const roles = ['族员', '财务管理', '理事', '房支理事', '理事长', '超级管理员']
  const currentIdx = roles.indexOf(user.role)
  if (currentIdx < 0) { ElMessage.warning('无法识别当前角色'); return }
  const available = roles.slice(currentIdx)
  // 超级管理员可以任命任何人；理事长不能任命理事长和超级管理员
  const filtered = isSuperAdmin.value ? roles : (isChief.value ? roles.filter(r => r !== '理事长' && r !== '超级管理员') : [])
  if (filtered.length === 0) { ElMessage.warning('无权限'); return }
  ElMessageBox.prompt(`请选择「${user.username}」的新职务`, '任命/罢免', {
    inputType: 'select',
    inputOptions: filtered,
    inputValue: user.role,
    confirmButtonText: '任命',
    cancelButtonText: '取消'
  }).then(async ({ value }) => {
    await api.put(`/users/${user.id}/role`, { role: value })
    ElMessage.success('已更新')
    loadUsers()
  }).catch(() => {})
}

function openSetBranch(user) {
  branchUser.value = user
  branchForm.value.branch = user.branch || ''
  branchVisible.value = true
}

async function doSetBranch() {
  try {
    await api.put(`/users/${branchUser.value.id}/branch`, { branch: branchForm.value.branch || '' })
    ElMessage.success('已设置')
    branchVisible.value = false
    loadUsers()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '设置失败')
  }
}

function openResetPw(user) {
  ElMessageBox.confirm(`确定重置「${user.username}」的密码？将随机生成一个 12 位数字临时密码，其首次登录需修改。`, '重置密码', {
    type: 'warning', confirmButtonText: '重置', cancelButtonText: '取消'
  }).then(async () => {
    try {
      const { data } = await api.post(`/users/${user.id}/reset-password`)
      ElMessage.success('已重置')
      if (data && data.password) showPw(user.username, data.password)
    } catch (e) {
      ElMessage.error(e.response?.data?.error || '重置失败')
    }
  }).catch(() => {})
}

async function delUser(user) {
  try {
    await ElMessageBox.confirm(`确定删除用户「${user.username}」？`, '删除', { type: 'warning' })
    await api.delete(`/users/${user.id}`)
    ElMessage.success('已删除')
    loadUsers()
  } catch (e) { /* ignore */ }
}

onMounted(async () => {
  me.value = JSON.parse(localStorage.getItem('user') || 'null')
  loadUsers()
  loadBranches()
  loadPrivacy()
  try {
    const { data } = await api.get('/settings/family-surname')
    form.surname = data.family_surname || ''
    form.family_name = data.family_name || ''
    form.tanghao = data.tanghao || ''
    form.junwang = data.junwang || ''
    form.branch_designation = data.branch_designation || ''
    form.zuji = data.zuji || ''
    form.shizu_name = data.shizu_name || ''
    form.shizu_bio = data.shizu_bio || ''
    form.shiqianzu_name = data.shiqianzu_name || ''
    form.shiqianzu_place = data.shiqianzu_place || ''
    form.totem_image = data.totem_image || ''
    form.totem_style = data.totem_style || 'wood'
    form.totem_font_size = Number(data.totem_font_size) || 200
    form.dieji_show_sisters = data.dieji_show_sisters !== false
    form.xiuci = data.xiuci || '重修'
    form.title_with_xiuci = !!data.title_with_xiuci
    form.chuizhu_show_daughters = data.chuizhu_show_daughters !== false
  } catch (e) { /* ignore */ }
})

function onSurnameChange() {
  if (form.surname && !form.family_name.startsWith(form.surname)) {
    form.family_name = form.surname + '氏家族'
  }
}

// 图腾底图裁剪（复用头像裁剪组件）：选图 → 裁剪成正方形 → 上传拿路径
const totemCropVisible = ref(false)
const totemCropSrc = ref('')

function beforeTotemUpload(file) {
  const reader = new FileReader()
  reader.onload = () => {
    totemCropSrc.value = reader.result
    totemCropVisible.value = true
  }
  reader.readAsDataURL(file)
  return false // 阻止 el-upload 默认上传，走裁剪流程
}

async function onTotemCrop(blob) {
  const fd = new FormData()
  fd.append('file', blob, 'totem.png')
  try {
    const { data } = await api.post('/upload', fd)
    form.totem_image = data.path || ''
    totemCropVisible.value = false
    ElMessage.success('底图已设置，记得点「保存家族设置」')
  } catch (e) {
    ElMessage.error('上传失败：' + (e?.response?.data?.error || e?.message || e))
  }
}

async function saveSettings() {
  await api.put('/settings/family-surname', {
    surname: form.surname, family_name: form.family_name,
    tanghao: form.tanghao, junwang: form.junwang, branch_designation: form.branch_designation, zuji: form.zuji,
    shizu_name: form.shizu_name, shizu_bio: form.shizu_bio,
    shiqianzu_name: form.shiqianzu_name, shiqianzu_place: form.shiqianzu_place,
    totem_image: form.totem_image, totem_style: form.totem_style, totem_font_size: Number(form.totem_font_size),
    dieji_show_sisters: form.dieji_show_sisters,
    chuizhu_show_daughters: form.chuizhu_show_daughters,
    xiuci: form.xiuci,
    title_with_xiuci: form.title_with_xiuci
  })
  ElMessage.success('已保存')
  location.reload()
}

// ===== 职务显示名 =====
const roleAliases = ref({})
const roleAliasesVisible = ref(false)
const roleAliasForm = ref({})

</script>

<style scoped>
.settings-page { max-width: 1000px; }
.settings-form { max-width: 520px; }
.form-hint { margin-left: 10px; color: #999; font-size: 12px; }
.surname-locked { color: #909399; font-size: 14px; user-select: none; }
.totem-preview { width: 96px; height: 96px; object-fit: contain; border: 1px solid #dcdfe6; border-radius: 4px; margin: 0 8px; vertical-align: middle; }.section { margin-bottom: 16px; }
.ancestor-tip { margin-bottom: 12px; }
.actions { margin-top: 4px; margin-bottom: 16px; }
.role-legend { margin-bottom: 16px; }
.legend-grid { display: flex; flex-wrap: wrap; gap: 8px 20px; }
.legend-item { display: flex; align-items: center; gap: 6px; font-size: 13px; color: #666; }
.role-tag { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; color: #fff; }
.role-visitor { background: #909399; }
.role-member { background: #a8a8a8; }
.role-elder { background: #e6a23c; }
.role-elder-branch { background: #d48806; }
.role-finance { background: #13a8a8; }
.role-chief { background: #409eff; }
.role-super { background: #f56c6c; }
.toolbar { margin-bottom: 12px; }
.branch-tip { color: #666; font-size: 13px; }
.table-responsive { width: 100%; }
.license-alert { margin-bottom: 12px; }
.license-section { display: flex; flex-direction: column; gap: 12px; }
.license-field { display: flex; align-items: center; gap: 8px; }
.license-label { min-width: 100px; font-size: 13px; color: #666; }
@media (max-width: 767px) { .settings-page { max-width: 100%; } .settings-form { max-width: 100%; } }
.pw-tip { font-size: 12px; color: #909399; line-height: 1.5; margin-top: 4px; }
.privacy-switches { display: flex; flex-direction: column; gap: 10px; margin-bottom: 14px; }
.ps-item { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 13px; color: #606266; }
.ps-label { font-weight: 600; }
.ps-hint { color: #909399; font-size: 12px; }
.entry-buttons { display: flex; flex-wrap: wrap; gap: 12px; margin: 4px 0 16px; }
.privacy-table { margin-bottom: 8px; }
.privacy-table :deep(.el-table__cell) { padding: 4px 0; }
.pw-for { font-size: 13px; color: #606266; margin-bottom: 8px; }
.pw-show { font-size: 26px; letter-spacing: 4px; font-weight: 700; text-align: center; padding: 12px; background: #f5f7fa; border-radius: 6px; font-family: ui-monospace, Menlo, Consolas, monospace; user-select: all; }
</style>

<style>
/* 用户权限 / 隐私与权限 弹窗：手机端适配（弹窗以 append-to-body 挂到 body，需非 scoped） */
.users-dlg, .privacy-dlg { max-width: 96vw; }
.users-dlg .el-dialog__body, .privacy-dlg .el-dialog__body { max-height: 76vh; overflow: auto; }

/* 隐私弹窗：全公开总开关（醒目卡片 + 打开后警示） */
.public-all-box { border: 1px solid #e4e7ed; border-radius: 8px; padding: 12px 14px; margin-bottom: 14px; background: #f8f9fb; }
.public-all-box.is-on { border-color: #f56c6c; background: #fef0f0; }
.pa-head { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.pa-title { font-size: 16px; font-weight: 700; color: #303133; }
.pa-warn { margin-top: 10px; font-size: 13px; line-height: 1.7; color: #c45656; background: #fff; border-left: 3px solid #f56c6c; padding: 8px 10px; border-radius: 4px; }
.pa-hint { margin-top: 8px; font-size: 12px; line-height: 1.6; color: #909399; }
.ps-off { color: #f56c6c; font-size: 12px; }
.matrix-off-tip { margin-bottom: 8px; }
.privacy-dlg .privacy-table .row-matrix-off { opacity: .45; }

@media (max-width: 767px) {
  .users-dlg, .privacy-dlg { width: 96vw !important; }
  .users-dlg .el-dialog__body, .privacy-dlg .el-dialog__body { max-height: 80vh; }
  .privacy-dlg .el-table { font-size: 12px; }
  .privacy-dlg .el-table .cell { padding: 0 3px; }
}
</style>