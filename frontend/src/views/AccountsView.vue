<template>
  <div class="acc-page">
    <div class="page-head">
      <h2>💰 收支公示</h2>
      <span class="hint">家族账目公开 · 理事提交 → 审核 → 族员可查 · 捐款关联成员（默认实名）</span>
    </div>

    <el-empty v-if="!cfg.enabled && !cfg.can_manage_list && !cfg.can_review" description="本家族暂未开启「收支公示」" />

    <template v-else>
      <el-alert v-if="!cfg.enabled" type="warning" show-icon :closable="false" class="enable-tip">
        <template #title>「收支公示」当前未开启</template>
        <div style="margin-top:4px">
          开启后族员即可在左侧菜单查看已审核公开的账目。
          <el-button v-if="cfg.can_manage_list" type="primary" size="small" style="margin-left:8px" @click="toggleEnabled(true)">立即开启</el-button>
        </div>
      </el-alert>

      <div class="toolbar">
        <el-button v-if="cfg.can_manage_books" type="primary" @click="openBookDialog()">＋ 新建账本</el-button>
        <el-button v-if="cfg.can_manage_list" @click="openListDialog">理账名单（提交权限）</el-button>
        <el-button v-if="cfg.can_manage_list && cfg.enabled" @click="toggleEnabled(false)">关闭功能</el-button>
        <el-button @click="load">刷新</el-button>
      </div>

      <div class="balance-banner" v-if="cfg.enabled || books.length">
        <div class="bb-left">
          <div class="bb-label">家族资金（当前可用余额）</div>
          <div class="bb-val">¥{{ fmt(cfg.fund) }}</div>
        </div>
        <div class="bb-right">
          未封账账本占用启动资金 ¥{{ fmt(allocatedOpen) }}（{{ openBooks.length }} 本）
          <div class="bb-tip">新建账本时从家族资金划拨启动资金（不超过家族资金）；封账时账本结余自动归还家族资金</div>
          <el-button v-if="cfg.can_manage_list" size="small" text type="primary" style="padding:0" @click="openFundDialog('in')">注入</el-button>
          <el-button v-if="cfg.can_manage_list" size="small" text type="primary" style="padding:0;margin-left:12px" @click="openFundDialog('out')">提取</el-button>
        </div>
      </div>

      <div class="fund-card" v-if="fundBook">
        <div class="fc-main">
          <div class="fc-title">🏦 {{ fundBook.title }}<el-tag size="small" type="info" effect="plain" style="margin-left:8px">系统账本</el-tag></div>
          <div class="fc-sub">记录注入 / 提取、各账本启动资金划拨与封账结余归还 · 条目由系统自动生成，不可编辑删除</div>
        </div>
        <div class="fc-right">
          <div class="fc-bal">¥{{ fmt(fundBook.balance) }}</div>
          <div class="fc-cnt">{{ fundBook.entry_count }} 笔流水</div>
          <el-button size="small" @click="goFund">查看流水</el-button>
        </div>
      </div>

      <el-card v-if="cfg.can_review && pending.length" class="pending-card" shadow="never">
        <template #header>
          <span class="pending-title">⏳ 待审核账目（{{ pending.length }}）</span>
        </template>
        <el-table :data="pending" size="small" border>
          <el-table-column label="账本" min-width="120">
            <template #default="s">
              {{ bookTitle(s.row.book_id) }}
              <el-tag v-if="bookClosed(s.row.book_id)" size="small" type="info">已封账</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="entry_date" label="日期" width="110" />
          <el-table-column label="类型" width="70">
            <template #default="s">
              <el-tag :type="s.row.kind === 'in' ? 'success' : 'danger'" size="small">{{ s.row.kind === 'in' ? '收入' : '支出' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="金额" width="110" align="right">
            <template #default="s"><span :class="s.row.kind === 'in' ? 'amt-in' : 'amt-out'">{{ s.row.kind === 'in' ? '+' : '-' }}{{ fmt(s.row.amount) }}</span></template>
          </el-table-column>
          <el-table-column prop="summary" label="摘要" min-width="160" show-overflow-tooltip />
          <el-table-column label="关联成员" width="110">
            <template #default="s">{{ s.row.member_name || '—' }}<span v-if="s.row.member_id && !s.row.donor_public" class="anon">（匿名）</span></template>
          </el-table-column>
          <el-table-column prop="submitted_by_name" label="提交人" width="90" />
          <el-table-column label="凭证" width="70">
            <template #default="s">
              <el-button v-if="s.row.vouchers && s.row.vouchers.length" link type="primary" @click="showVoucher(s.row.vouchers)">查看</el-button>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150" fixed="right">
            <template #default="s">
              <el-button size="small" type="success" :disabled="bookClosed(s.row.book_id)" @click="review(s.row, 'approve')">通过</el-button>
              <el-button size="small" type="danger" :disabled="bookClosed(s.row.book_id)" @click="review(s.row, 'reject')">驳回</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <h3 class="sec-title">账本列表</h3>
      <el-empty v-if="!normalBooks.length" description="还没有账本，点「新建账本」开始记账" />
      <div class="book-list">
        <el-card v-for="b in normalBooks" :key="b.id" class="book-card" shadow="hover" @click="$router.push('/accounts/' + b.id)">
          <div class="book-head">
            <span class="book-title">{{ b.title }}</span>
            <el-tag v-if="b.book_type === 'donation'" size="small" type="success" effect="light">捐款账簿</el-tag>
            <el-tag v-if="b.status === 'closed'" size="small" type="info">已封账</el-tag>
          </div>
          <div class="book-meta">
            <template v-if="b.year">{{ b.year }} 年</template>
            <template v-if="b.project"> · {{ b.project }}</template>
          </div>
          <div class="book-stats">
            <span class="s-in">收 ¥{{ fmt(b.total_in) }}</span>
            <span class="s-out">支 ¥{{ fmt(b.total_out) }}</span>
            <span class="s-bal">余 ¥{{ fmt(b.balance) }}</span>
          </div>
          <div class="book-foot">
            <span>
              <template v-if="b.is_pinned">📌 </template>
              {{ b.entry_count }} 笔明细
              <template v-if="cfg.can_review && b.pending_count"> · <span class="pend">待审 {{ b.pending_count }}</span></template>
            </span>
            <span class="foot-actions">
              <el-button
                v-if="cfg.can_manage_books"
                link :type="b.is_pinned ? 'warning' : 'primary'" size="small"
                @click.stop="togglePin(b)"
              >{{ b.is_pinned ? '取消置顶' : '置顶' }}</el-button>
              <el-button
                v-if="cfg.can_manage_books && b.status !== 'closed'"
                link type="danger" size="small"
                @click.stop="delBook(b)"
              >删除账本</el-button>
              <span v-else-if="cfg.can_manage_books && b.status === 'closed'" class="muted">已封账不可删除</span>
            </span>
          </div>
        </el-card>
      </div>
    </template>

    <!-- 新建 / 编辑账本 -->
    <el-dialog v-model="bookDialog" :title="bookForm.id ? '编辑账本' : '新建账本'" width="540px">
      <el-form label-width="90px">
        <el-form-item label="账本类型">
          <el-radio-group v-model="bookForm.book_type">
            <el-radio value="project">项目账本</el-radio>
            <el-radio value="donation">捐款账簿</el-radio>
          </el-radio-group>
          <span class="form-hint">捐款单独记账，可供「🪦 功德碑」取数</span>
        </el-form-item>
        <el-form-item label="账本名称" required>
          <el-input v-model="bookForm.title" maxlength="60" placeholder="如：2026 年修谱募捐" />
        </el-form-item>
        <el-form-item label="年度">
          <el-input-number v-model="bookForm.year" :min="1900" :max="2200" controls-position="right" />
        </el-form-item>
        <el-form-item label="项目">
          <el-input v-model="bookForm.project" maxlength="60" placeholder="如：祠堂修缮 / 助学基金（可留空）" />
        </el-form-item>
        <el-form-item label="启动资金">
          <el-input-number v-model="bookForm.initial_balance" :precision="2" :min="0" :max="fundMax" controls-position="right" />
          <span class="form-hint">从家族资金划拨（选填），最高 ¥{{ fmt(fundMax) }}</span>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="bookForm.status">
            <el-radio value="open">进行中</el-radio>
            <el-radio value="closed">封账（永久锁定，不可取消）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="族员可见">
          <el-switch v-model="bookForm.is_public" />
          <span class="form-hint">关闭后仅财务管理/理事长可见</span>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="bookForm.description" type="textarea" :rows="2" maxlength="200" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="bookDialog = false">取消</el-button>
        <el-button type="primary" @click="saveBook">保存</el-button>
      </template>
    </el-dialog>

    <!-- 理账名单 -->
    <el-dialog v-model="fundDialog" :title="fundForm.action === 'in' ? '家族资金注入' : '家族资金提取'" width="500px">
      <el-form label-width="90px">
        <el-form-item label="当前余额"><span class="fund-cur">¥{{ fmt(cfg.fund) }}</span></el-form-item>
        <el-form-item label="操作">
          <el-radio-group v-model="fundForm.action">
            <el-radio value="in">注入（进账）</el-radio>
            <el-radio value="out">提取（出账）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="金额">
          <el-input-number v-model="fundForm.amount" :precision="2" :min="0" :max="fundForm.action === 'out' ? Number(cfg.fund || 0) : Infinity" controls-position="right" />
        </el-form-item>
        <el-form-item label="备注" required>
          <el-input v-model="fundForm.note" type="textarea" :rows="3" maxlength="200" show-word-limit
            placeholder="请注明详细情况（来源 / 用途），如：2026年清明收到族亲捐款 5000 元 / 祠堂修缮支付木材款 1200 元" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="fundDialog = false">取消</el-button>
        <el-button type="primary" @click="saveFund">确定{{ fundForm.action === 'in' ? '注入' : '提取' }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="listDialog" title="理账名单（可提交账目的理事）" width="580px">
      <p class="hint-line">勾选允许提交账目的理事 / 房支理事。未勾选者只能查看，不能提交。</p>
      <el-table :data="cfg.submitter_users || []" size="small" border>
        <el-table-column label="选中" width="70">
          <template #default="s"><el-checkbox v-model="s.row.in_list" /></template>
        </el-table-column>
        <el-table-column prop="username" label="姓名（账号）" />
        <el-table-column prop="role" label="职位" width="110" />
      </el-table>
      <template #footer>
        <el-button @click="listDialog = false">取消</el-button>
        <el-button type="primary" @click="saveSubmitters">保存名单</el-button>
      </template>
    </el-dialog>

    <!-- 凭证预览 -->
    <el-dialog v-model="voucherVisible" title="凭证" width="720px">
      <img v-for="(v, i) in voucherList" :key="i" :src="v" class="voucher-img" @click="window_open(v)" />
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'

const cfg = ref({})
const books = ref([])
const pending = ref([])
const bookDialog = ref(false)
const listDialog = ref(false)
const voucherVisible = ref(false)
const voucherList = ref([])
const bookForm = ref({})
const fundDialog = ref(false)
const fundForm = ref({ action: 'in', amount: 0, note: '' })
const router = useRouter()

function fmt(n) { return Number(n || 0).toFixed(2) }
// 家族基金（系统账本）与普通账本分开
const fundBook = computed(() => books.value.find(b => b.is_fund) || null)
const normalBooks = computed(() => books.value.filter(b => !b.is_fund))
// 未封账（进行中）的账本
const openBooks = computed(() => normalBooks.value.filter(b => b.status !== 'closed'))
// 未封账账本当前占用的启动资金（已从家族资金划出、尚未归还）
const allocatedOpen = computed(() => openBooks.value.reduce((s, b) => s + Number(b.initial_balance || 0), 0))
// 启动资金上限：家族资金 + 编辑时本账本原先占用的那笔
const fundMax = computed(() => Number(cfg.value.fund || 0) + (bookForm.value.id ? Number(bookForm.value.__oldStart || 0) : 0))
function bookTitle(id) { const b = books.value.find(x => x.id === id); return b ? b.title : ('账本#' + id) }
function bookClosed(id) { const b = books.value.find(x => x.id === id); return !!b && b.status === 'closed' }
function window_open(u) { window.open(u, '_blank') }

async function loadCfg() {
  try { const { data } = await api.get('/accounts/config'); cfg.value = data } catch (e) { cfg.value = {} }
}
async function load() {
  await loadCfg()
  try { const { data } = await api.get('/accounts/books'); books.value = data || [] } catch (e) { books.value = [] }
  if (cfg.value.can_review) {
    try { const { data } = await api.get('/accounts/pending'); pending.value = data || [] } catch (e) { pending.value = [] }
  } else {
    pending.value = []
  }
}

async function toggleEnabled(on) {
  try {
    await api.put('/accounts/settings', { enabled: on })
    ElMessage.success(on ? '已开启「收支公示」' : '已关闭「收支公示」')
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '操作失败') }
}

function openBookDialog(b) {
  bookForm.value = b
    ? { ...b, __oldStart: Number(b.initial_balance || 0) }
    : { title: '', year: new Date().getFullYear(), project: '', book_type: 'project', initial_balance: 0, status: 'open', is_public: true, description: '' }
  bookDialog.value = true
}

// 注入 / 提取家族资金（自动记入家族基金系统账本，必须注明详细情况）
function openFundDialog(action) { fundForm.value = { action: action || 'in', amount: 0, note: '' }; fundDialog.value = true }
async function saveFund() {
  const f = fundForm.value
  if (!f.amount || f.amount <= 0) { ElMessage.warning('请填写金额'); return }
  if (!f.note || !String(f.note).trim()) { ElMessage.warning('请填写备注（详细情况）'); return }
  try {
    await api.put('/accounts/fund', { action: f.action, amount: f.amount, note: String(f.note).trim() })
    ElMessage.success(f.action === 'in' ? '已注入家族资金' : '已提取家族资金')
    fundDialog.value = false
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '操作失败') }
}
// 查看家族基金流水
function goFund() { if (fundBook.value) router.push('/accounts/' + fundBook.value.id) }
async function saveBook() {
  if (!bookForm.value.title) { ElMessage.warning('请填写账本名称'); return }
  if (bookForm.value.status === 'closed') {
    try {
      await ElMessageBox.confirm('封账后账本与全部账目将永久锁定：不能再记账、修改、删除、审核，也不能取消封账。确定封账？', '封账确认', { type: 'warning', confirmButtonText: '确定封账', cancelButtonText: '再想想' })
    } catch (e) { return }
  }
  try {
    if (bookForm.value.id) await api.put('/accounts/books/' + bookForm.value.id, bookForm.value)
    else await api.post('/accounts/books', bookForm.value)
    ElMessage.success('已保存')
    bookDialog.value = false
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '保存失败') }
}
async function deleteBook() {
  try {
    await ElMessageBox.confirm('删除账本会连同其全部明细一起删除，确定？', '提示', { type: 'warning' })
  } catch (e) { return }
  try {
    await api.delete('/accounts/books/' + bookForm.value.id)
    ElMessage.success('已删除')
    bookDialog.value = false
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '删除失败') }
}

// 删除账本（列表卡片）——封账的账本不可删
async function delBook(b) {
  try {
    await ElMessageBox.confirm(`删除「${b.title}」将连同其 ${b.entry_count} 笔明细一并删除，不可恢复。确定删除？`, '删除账本', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch (e) { return }
  try {
    await api.delete('/accounts/books/' + b.id)
    ElMessage.success('已删除')
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '删除失败') }
}

// 置顶/取消置顶（封账账本也可操作）
async function togglePin(b) {
  try {
    await api.put('/accounts/books/' + b.id + '/pin', { is_pinned: !b.is_pinned })
    ElMessage.success(b.is_pinned ? '已取消置顶' : '已置顶')
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '操作失败') }
}

function openListDialog() { listDialog.value = true }
async function saveSubmitters() {
  const ids = (cfg.value.submitter_users || []).filter(u => u.in_list).map(u => u.id)
  try {
    await api.put('/accounts/submitters', { ids })
    ElMessage.success('理账名单已保存')
    listDialog.value = false
    loadCfg()
  } catch (e) { ElMessage.error(e.response?.data?.error || '保存失败') }
}

async function review(row, action) {
  let reason = ''
  if (action === 'reject') {
    try {
      const r = await ElMessageBox.prompt('驳回原因（可选）', '驳回账目', { inputPlaceholder: '如：金额与凭证不符' })
      reason = r.value || ''
    } catch (e) { return }
  }
  try {
    await api.post('/accounts/entries/' + row.id + '/review', { action, reason })
    ElMessage.success(action === 'approve' ? '已通过并公开' : '已驳回')
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '操作失败') }
}

function showVoucher(list) { voucherList.value = list || []; voucherVisible.value = true }

onMounted(load)
</script>

<style scoped>
.acc-page { padding: 4px; }
.page-head { display: flex; align-items: baseline; gap: 12px; flex-wrap: wrap; margin-bottom: 12px; }
.page-head h2 { margin: 0; color: #5a4632; }
.hint { color: #97a0ab; font-size: 13px; }
.enable-tip { margin-bottom: 12px; }
.balance-banner { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; background: linear-gradient(135deg,#fdf6e9,#f7e9cf); border: 1px solid #e6ded1; border-radius: 8px; padding: 10px 18px; margin-bottom: 12px; }
.bb-label { font-size: 13px; color: #8a7550; }
.bb-val { font-size: 26px; font-weight: 700; color: #b8860b; line-height: 1.25; }
.bb-right { text-align: right; font-size: 12px; color: #97a0ab; }
.bb-tip { margin-top: 2px; }
.fund-cur { font-weight: 600; color: #b8860b; }
.toolbar { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 14px; }
.sec-title { color: #5a4632; margin: 18px 0 10px; font-size: 16px; }
.pending-card { margin-bottom: 8px; }
.pending-title { color: #b8860b; font-weight: 600; }
.book-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 12px; }
.book-card { cursor: pointer; }
.book-head { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.book-title { font-size: 16px; font-weight: 600; color: #5a4632; }
.book-meta { color: #97a0ab; font-size: 12px; margin: 4px 0 8px; min-height: 16px; }
.book-stats { display: flex; justify-content: space-between; font-size: 13px; }
.s-in { color: #0a7d3c; }
.s-out { color: #c0392b; }
.s-bal { color: #b8860b; font-weight: 600; }
.book-foot { margin-top: 8px; font-size: 12px; color: #97a0ab; border-top: 1px dashed #e6ded1; padding-top: 6px; display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.foot-actions { flex-shrink: 0; }
.pend { color: #b8860b; }
.amt-in { color: #0a7d3c; font-weight: 600; }
.amt-out { color: #c0392b; font-weight: 600; }
.anon { color: #97a0ab; font-size: 12px; }
.muted { color: #c0c4cc; }
.fund-card { display: flex; justify-content: space-between; align-items: center; gap: 16px; flex-wrap: wrap; background: #fff; border: 1px solid #e4e7ed; border-left: 4px solid #b8860b; border-radius: 8px; padding: 12px 18px; margin-bottom: 12px; }
.fc-title { font-size: 15px; font-weight: 600; color: #5a4632; }
.fc-sub { font-size: 12px; color: #97a0ab; margin-top: 4px; }
.fc-right { display: flex; align-items: center; gap: 14px; }
.fc-bal { font-size: 18px; font-weight: 700; color: #b8860b; }
.fc-cnt { font-size: 12px; color: #97a0ab; }
.form-hint { margin-left: 10px; color: #97a0ab; font-size: 12px; }
.hint-line { color: #97a0ab; font-size: 13px; margin: 0 0 10px; }
.voucher-img { max-width: 100%; margin-bottom: 10px; border: 1px solid #e6ded1; border-radius: 4px; cursor: zoom-in; }
</style>
