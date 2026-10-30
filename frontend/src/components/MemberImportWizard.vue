<template>
  <el-dialog
    :model-value="modelValue"
    title="导入成员档案（Excel / CSV）"
    width="920px"
    top="4vh"
    :close-on-click-modal="false"
    @update:model-value="onVisible"
    @closed="reset"
  >
    <el-steps :active="step" simple class="io-steps">
      <el-step title="选择文件" />
      <el-step title="预览校验" />
      <el-step title="导入结果" />
    </el-steps>

    <!-- 第 1 步：选择文件 -->
    <div v-if="step === 0" class="io-body">
      <el-upload
        drag
        :auto-upload="false"
        :limit="1"
        :file-list="fileList"
        accept=".xlsx,.csv,.txt"
        :on-change="onFileChange"
        :on-exceed="onExceed"
        :on-remove="onFileRemove"
      >
        <div class="upload-inner">
          <div class="upload-icon">📄</div>
          <div>把 Excel / CSV 文件拖到这里，或<em>点击选择文件</em></div>
          <div class="upload-tip">支持 .xlsx / .csv；CSV 支持 UTF-8 与 GBK（Windows Excel 另存）编码</div>
        </div>
      </el-upload>

      <el-alert type="info" :closable="false" show-icon class="io-alert">
        <template #title>导入规则（重要）</template>
        <ul class="rule-list">
          <li>必填列：<b>姓名</b>（也可用「姓」+「名」两列）；其余列留空即不填。</li>
          <li>日期统一写 <b>YYYY-MM-DD</b>（只知年份可写 YYYY）；农历日期写在「出生/去世日期(农历)」列，原样保留。</li>
          <li>父母/配偶匹配：优先用 <b>ID</b>；用姓名时按「<b>姓名 + 生父之父姓名</b>」区分同名；父母可先在本文件里新增，系统会按世代自动先插入父辈。</li>
          <li>更新语义：按 ID 或「姓名 + 生父姓名」匹配到已有成员 → 更新；<b>空单元格保持库中原值</b>，只有填了值才覆盖。</li>
          <li>只导入通过校验的行；失败行逐行给出原因，可下载失败清单 CSV 修改后再导。</li>
        </ul>
      </el-alert>

      <div class="tpl-row">
        <span class="tpl-label">没有模板？</span>
        <el-button size="small" @click="downloadTemplate('xlsx')">下载 Excel 模板</el-button>
        <el-button size="small" @click="downloadTemplate('csv')">下载 CSV 模板</el-button>
      </div>
    </div>

    <!-- 第 2 步：预览校验 -->
    <div v-else-if="step === 1" class="io-body">
      <div v-if="preview" class="stats-row">
        <div class="stat-card create">
          <div class="stat-num">{{ preview.stats.create }}</div>
          <div class="stat-label">将新增</div>
        </div>
        <div class="stat-card update">
          <div class="stat-num">{{ preview.stats.update }}</div>
          <div class="stat-label">将更新</div>
        </div>
        <div class="stat-card skip">
          <div class="stat-num">{{ preview.stats.skip }}</div>
          <div class="stat-label">无变化</div>
        </div>
        <div class="stat-card spouse">
          <div class="stat-num">{{ preview.stats.spouse_add }}</div>
          <div class="stat-label">新增配偶</div>
        </div>
        <div class="stat-card fail">
          <div class="stat-num">{{ preview.stats.invalid }}</div>
          <div class="stat-label">校验失败</div>
        </div>
        <div class="stat-card total">
          <div class="stat-num">{{ preview.stats.total }}</div>
          <div class="stat-label">数据总行数</div>
        </div>
      </div>

      <el-alert
        v-for="(w, i) in (preview && preview.warnings) || []"
        :key="i"
        :title="w"
        type="warning"
        :closable="false"
        show-icon
        class="io-alert"
      />

      <div class="preview-toolbar">
        <el-radio-group v-model="rowFilter" size="small">
          <el-radio-button value="all">全部</el-radio-button>
          <el-radio-button value="error">失败 {{ preview ? preview.stats.invalid : 0 }}</el-radio-button>
          <el-radio-button value="update">将更新 {{ preview ? preview.stats.update : 0 }}</el-radio-button>
        </el-radio-group>
        <div class="spacer" />
        <el-button v-if="preview && preview.stats.invalid > 0" size="small" type="warning" plain :loading="downloading" @click="downloadFailures">
          下载失败清单 CSV
        </el-button>
      </div>

      <el-table :data="shownRows" border size="small" max-height="340" class="io-table">
        <el-table-column prop="row_no" label="行号" width="60" />
        <el-table-column label="动作" width="82">
          <template #default="{ row }">
            <el-tag v-if="row.action === 'create'" type="success" size="small">新增</el-tag>
            <el-tag v-else-if="row.action === 'update'" type="primary" size="small">更新</el-tag>
            <el-tag v-else-if="row.action === 'skip'" type="info" size="small">无变化</el-tag>
            <el-tag v-else type="danger" size="small">失败</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="姓名" width="110" show-overflow-tooltip>
          <template #default="{ row }">{{ row.name || '—' }}</template>
        </el-table-column>
        <el-table-column prop="generation" label="世代" width="70">
          <template #default="{ row }">{{ row.generation ? `第${row.generation}世` : '—' }}</template>
        </el-table-column>
        <el-table-column prop="branch" label="房支" width="90">
          <template #default="{ row }">{{ row.branch || '—' }}</template>
        </el-table-column>
        <el-table-column prop="father" label="生父" width="130" show-overflow-tooltip>
          <template #default="{ row }">{{ row.father || '—' }}</template>
        </el-table-column>
        <el-table-column label="原因 / 将更新字段" min-width="260">
          <template #default="{ row }">
            <div v-if="row.reasons && row.reasons.length" class="reason-text">{{ row.reasons.join('；') }}</div>
            <div v-else-if="row.changes && row.changes.length" class="change-text">将更新：{{ row.changes.join('、') }}</div>
            <div v-else-if="row.warnings && row.warnings.length" class="warn-text">{{ row.warnings.join('；') }}</div>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
      </el-table>

      <el-collapse class="io-collapse">
        <el-collapse-item title="查看列名映射（后端识别结果）" name="cols">
          <el-table :data="preview ? preview.columns : []" border size="small" max-height="220">
            <el-table-column prop="index" label="列序" width="60" />
            <el-table-column prop="header" label="表头" min-width="140" />
            <el-table-column label="识别结果" min-width="200">
              <template #default="{ row }">
                <span v-if="row.status === 'ok'" class="ok-text">→ {{ row.label }}</span>
                <span v-else-if="row.status === 'readonly'" class="muted">只读列「{{ row.label }}」，导入时忽略</span>
                <span v-else-if="row.status === 'duplicate'" class="reason-text">列名重复，已忽略后出现的列</span>
                <span v-else class="warn-text">未识别列，导入时忽略</span>
              </template>
            </el-table-column>
          </el-table>
        </el-collapse-item>
      </el-collapse>

      <div class="mode-row">
        <span class="mode-label">写入策略：</span>
        <el-radio-group v-model="mode">
          <el-radio value="abort">整体成功才提交（遇数据库错误 → 全部回滚）</el-radio>
          <el-radio value="skip">跳过出错行，其余照常提交</el-radio>
        </el-radio-group>
      </div>
      <div class="mode-hint">
        校验失败的行两种策略都不会导入；两者都在 <b>一个事务</b> 里写入，避免出现只导入一半的谱。
      </div>
    </div>

    <!-- 第 3 步：结果 -->
    <div v-else class="io-body">
      <div v-if="result" class="stats-row">
        <div class="stat-card create">
          <div class="stat-num">{{ result.created }}</div>
          <div class="stat-label">已新增</div>
        </div>
        <div class="stat-card update">
          <div class="stat-num">{{ result.updated }}</div>
          <div class="stat-label">已更新</div>
        </div>
        <div class="stat-card spouse">
          <div class="stat-num">{{ result.spouse_add }}</div>
          <div class="stat-label">新增配偶</div>
        </div>
        <div class="stat-card skip">
          <div class="stat-num">{{ result.skipped }}</div>
          <div class="stat-label">无变化跳过</div>
        </div>
        <div class="stat-card fail">
          <div class="stat-num">{{ (result.failures || []).length }}</div>
          <div class="stat-label">失败 / 未导入</div>
        </div>
      </div>

      <el-alert
        :title="`导入完成：新增 ${result.created} 人，更新 ${result.updated} 人，跳过 ${result.skipped} 人。`"
        type="success"
        :closable="false"
        show-icon
      />

      <div v-if="result.failures && result.failures.length" class="result-fail">
        <div class="result-fail-head">
          <b>未导入的行（{{ result.failures.length }}）</b>
          <el-button size="small" type="warning" plain :loading="downloading" @click="downloadFailures">下载失败清单 CSV</el-button>
        </div>
        <el-table :data="result.failures" border size="small" max-height="260">
          <el-table-column prop="row_no" label="行号" width="60" />
          <el-table-column prop="name" label="姓名" width="120">
            <template #default="{ row }">{{ row.name || '—' }}</template>
          </el-table-column>
          <el-table-column label="原因" min-width="360">
            <template #default="{ row }">{{ (row.reasons || []).join('；') }}</template>
          </el-table-column>
        </el-table>
      </div>
      <div v-else class="all-ok">✅ 所有行都已成功处理，没有失败行。</div>
    </div>

    <template #footer>
      <el-button @click="onVisible(false)">取消</el-button>
      <el-button v-if="step === 1" @click="step = 0">上一步</el-button>
      <el-button v-if="step === 0" type="primary" :disabled="!file" :loading="busy" @click="doPreview">
        下一步：预览校验
      </el-button>
      <el-button v-else-if="step === 1" type="primary" :loading="busy" @click="doImport">
        开始导入{{ preview ? `（${preview.stats.valid} 行）` : '' }}
      </el-button>
      <el-button v-else type="primary" @click="finish">完成</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import { saveBlob, filenameFromDisposition } from '../utils/download'

const props = defineProps({ modelValue: { type: Boolean, default: false } })
const emit = defineEmits(['update:modelValue', 'imported'])

const step = ref(0)
const file = ref(null)
const fileList = ref([])
const preview = ref(null)
const result = ref(null)
const busy = ref(false)
const downloading = ref(false)
const rowFilter = ref('all')
const mode = ref('abort')

const shownRows = computed(() => {
  const rows = (preview.value && preview.value.rows) || []
  if (rowFilter.value === 'error') return rows.filter(r => r.action === 'error')
  if (rowFilter.value === 'update') return rows.filter(r => r.action === 'update')
  return rows
})

function onVisible(v) {
  emit('update:modelValue', v)
}

function reset() {
  step.value = 0
  file.value = null
  fileList.value = []
  preview.value = null
  result.value = null
  busy.value = false
  downloading.value = false
  rowFilter.value = 'all'
  mode.value = 'abort'
}

function onFileChange(f, list) {
  // 只保留最后选择的文件
  if (list && list.length > 1) list.splice(0, list.length - 1)
  fileList.value = list || []
  file.value = f && f.raw ? f.raw : null
  preview.value = null
  result.value = null
}

function onFileRemove() {
  file.value = null
  fileList.value = []
}

function onExceed(files) {
  // 超出限制时替换成新文件
  const f = files && files[0]
  if (!f) return
  fileList.value = [{ name: f.name, raw: f }]
  file.value = f
  preview.value = null
  result.value = null
}

function formData() {
  const fd = new FormData()
  fd.append('file', file.value, file.value.name || 'import')
  return fd
}

async function doPreview() {
  if (!file.value) { ElMessage.warning('请先选择文件'); return }
  busy.value = true
  try {
    const { data } = await api.post('/members/import/preview', formData(), {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000
    })
    preview.value = data
    step.value = 1
    if (data.stats.valid === 0) ElMessage.warning('没有任何一行通过校验，请按原因修改后再导入')
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || e.message || '预览失败')
  } finally {
    busy.value = false
  }
}

async function doImport() {
  if (!preview.value) return
  if (preview.value.stats.valid === 0) { ElMessage.warning('没有可导入的行'); return }
  busy.value = true
  try {
    const fd = formData()
    fd.append('mode', mode.value)
    const { data } = await api.post('/members/import', fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 300000
    })
    result.value = data
    step.value = 2
    ElMessage.success(`导入完成：新增 ${data.created}，更新 ${data.updated}`)
    emit('imported', data)
  } catch (e) {
    const d = e?.response?.data
    if (d && d.error) {
      // 事务整体回滚 / 校验失败：展示后端给的原因清单
      result.value = {
        created: d.created || 0,
        updated: d.updated || 0,
        skipped: d.skipped || 0,
        spouse_add: d.spouse_add || 0,
        failures: d.failures || []
      }
      step.value = 2
      ElMessage.error(d.error + (d.rolled_back ? '（已整体回滚，库中数据未改变）' : ''))
    } else {
      ElMessage.error(e.message || '导入失败')
    }
  } finally {
    busy.value = false
  }
}

async function downloadFailures() {
  if (!file.value) return
  downloading.value = true
  try {
    const { data, headers } = await api.post('/members/import/failures', formData(), {
      headers: { 'Content-Type': 'multipart/form-data' },
      responseType: 'blob',
      timeout: 120000
    })
    const name = filenameFromDisposition(headers['content-disposition'], '导入失败清单.csv')
    saveBlob(data, name)
  } catch (e) {
    ElMessage.error('下载失败清单失败：' + (e.message || ''))
  } finally {
    downloading.value = false
  }
}

async function downloadTemplate(format) {
  try {
    const { data, headers } = await api.get('/members/import/template', {
      params: { format },
      responseType: 'blob',
      timeout: 60000
    })
    const name = filenameFromDisposition(headers['content-disposition'], format === 'csv' ? '成员导入模板.csv' : '成员导入模板.xlsx')
    saveBlob(data, name)
  } catch (e) {
    ElMessage.error('下载模板失败：' + (e.message || ''))
  }
}

function finish() {
  emit('imported', result.value)
  onVisible(false)
}
</script>

<style scoped>
.io-steps { margin-bottom: 16px; }
.io-body { min-height: 220px; }
.upload-inner { padding: 12px 0; }
.upload-icon { font-size: 34px; line-height: 1.2; }
.upload-inner em { color: #8c5a3c; font-style: normal; font-weight: 600; }
.upload-tip { color: #909399; font-size: 12px; margin-top: 6px; }
.io-alert { margin: 12px 0; }
.rule-list { margin: 4px 0 0; padding-left: 18px; line-height: 1.8; font-size: 13px; }
.tpl-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
.tpl-label { color: #606266; font-size: 13px; }
.stats-row { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 8px; margin-bottom: 10px; }
.stat-card { border-radius: 8px; padding: 10px 8px; text-align: center; background: #f7f4ee; border: 1px solid #e8dfd2; }
.stat-num { font-size: 22px; font-weight: 700; color: #3a3226; line-height: 1.2; }
.stat-label { font-size: 12px; color: #7a6c5b; margin-top: 2px; }
.stat-card.create .stat-num { color: #2e7d32; }
.stat-card.update .stat-num { color: #1565c0; }
.stat-card.skip .stat-num { color: #757575; }
.stat-card.spouse .stat-num { color: #8c5a3c; }
.stat-card.fail .stat-num { color: #c62828; }
.preview-toolbar { display: flex; align-items: center; margin: 8px 0; }
.spacer { flex: 1; }
.io-table { width: 100%; }
.reason-text { color: #c62828; font-size: 12px; }
.change-text { color: #1565c0; font-size: 12px; }
.warn-text { color: #b26a00; font-size: 12px; }
.ok-text { color: #2e7d32; font-size: 12px; }
.muted { color: #999; font-size: 12px; }
.io-collapse { margin-top: 8px; }
.mode-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-top: 10px; }
.mode-label { font-size: 13px; color: #606266; }
.mode-hint { font-size: 12px; color: #909399; margin-top: 4px; }
.result-fail { margin-top: 12px; }
.result-fail-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.all-ok { margin-top: 16px; color: #2e7d32; font-size: 14px; }
@media (max-width: 767px) {
  .stats-row { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
</style>
