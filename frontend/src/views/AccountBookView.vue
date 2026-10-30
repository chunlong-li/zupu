<template>
  <div class="ab-page">
    <div class="page-head">
      <el-button link @click="$router.push('/accounts')">← 返回账本列表</el-button>
    </div>

    <el-card v-if="book.id" class="sum-card" shadow="never">
      <div class="sum-head">
        <div>
          <span class="sum-title">{{ book.title }}</span>
          <el-tag v-if="book.is_fund" size="small" type="info" style="margin-left:8px">系统账本</el-tag>
          <el-tag v-if="book.status === 'closed'" size="small" type="info" style="margin-left:8px">已封账（定稿，不可修改）</el-tag>
        </div>
        <div class="sum-actions">
          <el-button v-if="canPost && !book.is_fund && book.status !== 'closed'" type="primary" @click="openEntryDialog()">＋ 记一笔</el-button>
          <el-button v-if="cfg.can_manage_books && !book.is_fund && book.status !== 'closed'" @click="editBook">编辑账本</el-button>
        </div>
      </div>
      <div class="sum-meta">
        <template v-if="book.year">{{ book.year }} 年</template>
        <template v-if="book.project"> · {{ book.project }}</template>
        <template v-if="book.description"> · {{ book.description }}</template>
      </div>
      <div class="sum-stats">
        <div class="stat" v-if="!book.is_fund"><span class="lbl">启动资金</span><span class="val">¥{{ fmt(book.initial_balance) }}</span></div>
        <div class="stat"><span class="lbl">收入合计</span><span class="val in">¥{{ fmt(book.total_in) }}</span></div>
        <div class="stat"><span class="lbl">支出合计</span><span class="val out">¥{{ fmt(book.total_out) }}</span></div>
        <div class="stat"><span class="lbl">当前结余</span><span class="val bal">¥{{ fmt(book.balance) }}</span></div>
      </div>
    </el-card>

    <el-alert v-if="book.is_fund" type="warning" show-icon :closable="false" class="tip" style="margin-top:12px">
      本账本为「家族基金」系统账本：条目由系统自动生成（资金注入/提取、各账本启动资金划拨、封账结余归还），不可新增、修改或删除。
    </el-alert>

    <el-alert v-if="cfg.can_submit && !cfg.can_review" type="info" show-icon :closable="false" class="tip">
      您提交的账目需经财务管理 / 理事长审核通过后才会公示。
    </el-alert>

    <el-table :data="entries" border stripe style="width:100%; margin-top:12px">
      <el-table-column prop="entry_date" label="日期" width="108" />
      <el-table-column label="录入时间" width="90">
        <template #default="s"><span class="muted">{{ fmtTime(s.row.created_at) }}</span></template>
      </el-table-column>
      <el-table-column label="类型" width="72">
        <template #default="s">
          <el-tag :type="s.row.kind === 'in' ? 'success' : 'danger'" size="small">{{ s.row.kind === 'in' ? '收入' : '支出' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="category" label="分类" width="100" />
      <el-table-column label="金额" width="120" align="right">
        <template #default="s"><span :class="s.row.kind === 'in' ? 'amt-in' : 'amt-out'">{{ s.row.kind === 'in' ? '+' : '-' }}{{ fmt(s.row.amount) }}</span></template>
      </el-table-column>
      <el-table-column prop="summary" label="摘要" min-width="160" show-overflow-tooltip />
      <el-table-column prop="handler" label="经手人" width="90" />
      <el-table-column label="关联成员" width="120">
        <template #default="s">
          <template v-if="s.row.member_id">
            {{ s.row.member_name }}<span v-if="!s.row.donor_public" class="anon">（匿名）</span>
          </template>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column label="凭证" width="70">
        <template #default="s">
          <el-button v-if="s.row.vouchers && s.row.vouchers.length" link type="primary" @click="showVoucher(s.row.vouchers)">查看</el-button>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="reviewer_name" label="审核人" width="90" />
      <el-table-column label="状态" width="90">
        <template #default="s">
          <el-tag v-if="s.row.status === 'approved'" type="success" size="small">已公开</el-tag>
          <el-tag v-else-if="s.row.status === 'pending'" type="warning" size="small">待审核</el-tag>
          <el-tooltip v-else :content="s.row.reason || '未通过'" placement="top">
            <el-tag type="danger" size="small">已驳回</el-tag>
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="200" fixed="right">
        <template #default="s">
          <span v-if="book.is_fund" class="muted">系统账本 · 不可修改</span>
          <span v-else-if="book.status === 'closed'" class="muted">已封账 · 不可修改</span>
          <template v-else-if="s.row.status === 'pending'">
            <template v-if="cfg.can_review">
              <el-button size="small" type="success" @click="review(s.row, 'approve')">通过</el-button>
              <el-button size="small" type="danger" @click="review(s.row, 'reject')">驳回</el-button>
            </template>
            <template v-if="canEditEntry(s.row)">
              <el-button size="small" @click="openEntryDialog(s.row)">修改</el-button>
              <el-button size="small" type="danger" plain @click="delEntry(s.row)">撤回</el-button>
            </template>
          </template>
          <template v-else-if="cfg.can_review">
            <el-button size="small" @click="openEntryDialog(s.row)">编辑</el-button>
            <el-button size="small" type="danger" plain @click="delEntry(s.row)">删除</el-button>
          </template>
          <span v-else class="muted">—</span>
        </template>
      </el-table-column>
    </el-table>

    <!-- 记一笔 / 编辑 -->
    <el-dialog v-model="entryDialog" :title="entryForm.id ? '编辑账目' : '记一笔'" width="620px">
      <el-form label-width="90px">
        <el-form-item label="类型" required>
          <el-radio-group v-model="entryForm.kind">
            <el-radio value="in">收入</el-radio>
            <el-radio value="out">支出</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="金额" required>
          <el-input-number v-model="entryForm.amount" :min="0" :precision="2" controls-position="right" />
        </el-form-item>
        <el-form-item label="日期">
          <el-date-picker v-model="entryForm.entry_date" type="date" value-format="YYYY-MM-DD" placeholder="选择日期" />
        </el-form-item>
        <el-form-item label="分类">
          <el-input v-model="entryForm.category" maxlength="20" placeholder="如：捐款 / 修缮 / 助学 / 祭祀" />
        </el-form-item>
        <el-form-item label="摘要" required>
          <el-input v-model="entryForm.summary" type="textarea" :rows="2" maxlength="200" placeholder="这笔钱是什么" />
        </el-form-item>
        <el-form-item label="经手人">
          <el-input v-model="entryForm.handler" maxlength="20" placeholder="收款 / 付款经手人" />
        </el-form-item>
        <el-form-item :label="entryForm.kind === 'in' ? '捐款人' : '关联成员'">
          <el-select
            v-model="entryForm.member_id"
            filterable
            remote
            clearable
            reserve-keyword
            placeholder="输入姓名搜索族员（可留空）"
            :remote-method="searchMembers"
            :loading="memberLoading"
            style="width: 100%"
          >
            <el-option v-for="m in memberOptions" :key="m.id" :label="memberLabel(m)" :value="m.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="entryForm.member_id" label="公示姓名">
          <el-switch v-model="entryForm.donor_public" active-text="实名公示" inactive-text="匿名（隐去姓名）" />
        </el-form-item>
        <el-form-item label="凭证">
          <div class="voucher-box">
            <div v-for="(v, i) in entryForm.vouchers" :key="i" class="voucher-item">
              <img :src="v" />
              <el-button link type="danger" @click="entryForm.vouchers.splice(i, 1)">删除</el-button>
            </div>
            <el-upload :show-file-list="false" :before-upload="uploadVoucher" accept="image/*">
              <el-button :loading="uploading">上传图片</el-button>
            </el-upload>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="entryDialog = false">取消</el-button>
        <el-button type="primary" @click="saveEntry">保存</el-button>
      </template>
    </el-dialog>

    <!-- 编辑账本 -->
    <el-dialog v-model="bookDialog" title="编辑账本" width="520px">
      <el-form label-width="90px">
        <el-form-item label="账本名称" required><el-input v-model="bookForm.title" maxlength="60" /></el-form-item>
        <el-form-item label="年度"><el-input-number v-model="bookForm.year" :min="1900" :max="2200" controls-position="right" /></el-form-item>
        <el-form-item label="项目"><el-input v-model="bookForm.project" maxlength="60" /></el-form-item>
        <el-form-item label="启动资金">
          <el-input-number v-model="bookForm.initial_balance" :precision="2" :min="0" :max="startMax" controls-position="right" />
          <span class="f-hint">从家族资金划拨，最高 ¥{{ fmt(startMax) }}</span>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="bookForm.status">
            <el-radio value="open">进行中</el-radio>
            <el-radio value="closed">封账（永久锁定，不可取消）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="族员可见"><el-switch v-model="bookForm.is_public" /></el-form-item>
        <el-form-item label="说明"><el-input v-model="bookForm.description" type="textarea" :rows="2" maxlength="200" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button v-if="cfg.can_manage_books" type="danger" plain style="float:left" @click="delBook">删除账本</el-button>
        <el-button @click="bookDialog = false">取消</el-button>
        <el-button type="primary" @click="saveBook">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="voucherVisible" title="凭证" width="720px">
      <img v-for="(v, i) in voucherList" :key="i" :src="v" class="voucher-img" @click="window_open(v)" />
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'

const route = useRoute()
const router = useRouter()
const bookId = route.params.id

const cfg = ref({})
const book = ref({})
const entries = ref([])
const entryDialog = ref(false)
const bookDialog = ref(false)
const entryForm = ref({})
const bookForm = ref({})
const memberOptions = ref([])
const memberLoading = ref(false)
const uploading = ref(false)
const voucherVisible = ref(false)
const voucherList = ref([])

const currentUser = (() => {
  try { return JSON.parse(localStorage.getItem('user') || '{}') } catch (e) { return {} }
})()
const userId = Number(currentUser.id || 0)
const canPost = computed(() => cfg.value.can_submit || cfg.value.can_review)
// 启动资金上限 = 家族资金 + 本账本原先占用的那笔
const startMax = computed(() => Number(cfg.value.fund || 0) + Number(bookForm.value.__oldStart || 0))

function fmt(n) { return Number(n || 0).toFixed(2) }
// 录入时间：后端已转为本地时区，仅展示时间部分（日期列已单列）
function fmtTime(t) { return t && String(t).length >= 16 ? String(t).slice(11, 16) : (t || '—') }
function window_open(u) { window.open(u, '_blank') }
function memberLabel(m) { return `${m.name || (m.surname || '') + (m.given_name || '')}（第${m.generation || '?'}世）` }
function canEditEntry(row) { return row.status === 'pending' && row.submitted_by === userId }

async function loadCfg() {
  try { const { data } = await api.get('/accounts/config'); cfg.value = data } catch (e) { cfg.value = {} }
}
async function load() {
  await loadCfg()
  try { const { data } = await api.get('/accounts/books/' + bookId); book.value = data } catch (e) { ElMessage.error('账本不存在或无权限'); router.push('/accounts'); return }
  try { const { data } = await api.get('/accounts/books/' + bookId + '/entries'); entries.value = data || [] } catch (e) { entries.value = [] }
}

async function searchMembers(q) {
  if (!q) { memberOptions.value = []; return }
  memberLoading.value = true
  try { const { data } = await api.get('/members/search', { params: { q } }); memberOptions.value = data || [] }
  catch (e) { memberOptions.value = [] } finally { memberLoading.value = false }
}

function openEntryDialog(row) {
  entryForm.value = row
    ? { ...row, vouchers: [...(row.vouchers || [])] }
    : { kind: 'in', amount: 0, category: '', entry_date: new Date().toISOString().slice(0, 10), summary: '', handler: '', member_id: null, donor_public: true, vouchers: [] }
  entryDialog.value = true
}
async function uploadVoucher(file) {
  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', file)
    const { data } = await api.post('/accounts/upload', fd, { headers: { 'Content-Type': 'multipart/form-data' } })
    if (data.path) { entryForm.value.vouchers = [...(entryForm.value.vouchers || []), data.path]; ElMessage.success('已上传') }
  } catch (e) { ElMessage.error(e.response?.data?.error || '上传失败') } finally { uploading.value = false }
  return false
}
async function saveEntry() {
  const f = entryForm.value
  if (!f.amount || f.amount <= 0) { ElMessage.warning('请填写金额'); return }
  if (!f.summary) { ElMessage.warning('请填写摘要'); return }
  const payload = { ...f, member_id: f.member_id || 0 }
  try {
    if (f.id) await api.put('/accounts/entries/' + f.id, payload)
    else {
      const { data } = await api.post('/accounts/books/' + bookId + '/entries', payload)
      if (f.status === undefined && data.message) ElMessage.success(data.message)
      else ElMessage.success('已保存')
    }
    entryDialog.value = false
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '保存失败') }
}
async function delEntry(row) {
  try { await ElMessageBox.confirm('确定删除这笔账目？', '提示', { type: 'warning' }) } catch (e) { return }
  try { await api.delete('/accounts/entries/' + row.id); ElMessage.success('已删除'); load() }
  catch (e) { ElMessage.error(e.response?.data?.error || '删除失败') }
}
async function review(row, action) {
  let reason = ''
  if (action === 'reject') {
    try { const r = await ElMessageBox.prompt('驳回原因（可选）', '驳回账目', { inputPlaceholder: '如：金额与凭证不符' }); reason = r.value || '' }
    catch (e) { return }
  }
  try {
    await api.post('/accounts/entries/' + row.id + '/review', { action, reason })
    ElMessage.success(action === 'approve' ? '已通过并公开' : '已驳回')
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '操作失败') }
}
function editBook() { bookForm.value = { ...book.value, __oldStart: Number(book.value.initial_balance || 0) }; bookDialog.value = true }
// 删除账本（未封账才可删）
async function delBook() {
  try {
    await ElMessageBox.confirm(`删除「${book.value.title}」将连同其全部明细一并删除，不可恢复。确定删除？`, '删除账本', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
  } catch (e) { return }
  try {
    await api.delete('/accounts/books/' + bookId)
    ElMessage.success('已删除')
    bookDialog.value = false
    router.push('/accounts')
  } catch (e) { ElMessage.error(e.response?.data?.error || '删除失败') }
}
async function saveBook() {
  if (!bookForm.value.title) { ElMessage.warning('请填写账本名称'); return }
  if (bookForm.value.status === 'closed' && book.status !== 'closed') {
    try {
      await ElMessageBox.confirm('封账后账本与全部账目将永久锁定：不能再记账、修改、删除、审核，也不能取消封账。确定封账？', '封账确认', { type: 'warning', confirmButtonText: '确定封账', cancelButtonText: '再想想' })
    } catch (e) { return }
  }
  try {
    await api.put('/accounts/books/' + bookId, bookForm.value)
    ElMessage.success('已保存')
    bookDialog.value = false
    load()
  } catch (e) { ElMessage.error(e.response?.data?.error || '保存失败') }
}
function showVoucher(list) { voucherList.value = list || []; voucherVisible.value = true }

onMounted(load)
</script>

<style scoped>
.ab-page { padding: 4px; }
.page-head { margin-bottom: 8px; }
.sum-card { margin-bottom: 8px; }
.sum-head { display: flex; justify-content: space-between; align-items: center; gap: 10px; flex-wrap: wrap; }
.sum-title { font-size: 18px; font-weight: 700; color: #5a4632; }
.sum-meta { color: #97a0ab; font-size: 13px; margin: 4px 0 10px; }
.sum-stats { display: flex; gap: 28px; flex-wrap: wrap; border-top: 1px dashed #e6ded1; padding-top: 10px; }
.stat { display: flex; flex-direction: column; }
.f-hint { margin-left: 10px; color: #97a0ab; font-size: 12px; }
.stat .lbl { font-size: 12px; color: #97a0ab; }
.stat .val { font-size: 16px; font-weight: 600; color: #5a4632; }
.stat .val.in { color: #0a7d3c; }
.stat .val.out { color: #c0392b; }
.stat .val.bal { color: #b8860b; }
.tip { margin-bottom: 4px; }
.amt-in { color: #0a7d3c; font-weight: 600; }
.amt-out { color: #c0392b; font-weight: 600; }
.anon { color: #97a0ab; font-size: 12px; }
.muted { color: #c0c4cc; }
.voucher-box { display: flex; flex-wrap: wrap; gap: 10px; align-items: center; }
.voucher-item { position: relative; }
.voucher-item img { width: 80px; height: 80px; object-fit: cover; border: 1px solid #e6ded1; border-radius: 4px; }
.voucher-img { max-width: 100%; margin-bottom: 10px; border: 1px solid #e6ded1; border-radius: 4px; cursor: zoom-in; }
</style>
