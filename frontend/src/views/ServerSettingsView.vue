<template>
  <div class="server-settings-page">
    <h2 class="page-title">🖥️ 服务器设置</h2>
    <p class="page-desc">登录防爆破与修谱助手（AI）配置，仅族长（理事长）与超级管理员可访问</p>
    <el-divider />

    <!-- 登录安全（防爆破） -->
    <el-card class="section">
      <template #header><span>登录安全</span></template>
      <el-form label-width="140px" class="settings-form">
        <el-form-item label="失败次数阈值">
          <el-input-number v-model="security.max_failures" :min="1" :max="100" />
          <span class="form-hint">同一 IP 连续登录失败达到此次数后锁定</span>
        </el-form-item>
        <el-form-item label="锁定时长（分钟）">
          <el-input-number v-model="security.lock_minutes" :min="1" :max="1440" />
          <span class="form-hint">锁定后需等待的分钟数</span>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="savingSecurity" @click="saveSecurity">保存安全设置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 本机访问 -->
    <el-card class="section">
      <template #header><span>本机访问</span></template>
      <el-form label-width="140px" class="settings-form">
        <el-form-item label="本机免登录">
          <el-switch v-model="localNoAuth.enabled" />
          <span class="hint" style="margin-left: 10px">开启后，来自 127.0.0.1 的访问无需登录，权限按「族员」（只读，不能修改数据）</span>
        </el-form-item>
        <el-alert type="warning" :closable="false" show-icon
          title="请勿在反向代理环境下开启"
          description="若族谱挂在同机反向代理（如 Lucky）后面，代理转发过来的请求对端也是 127.0.0.1 —— 开启后所有经代理访问的人都会变成免登录的「族员」。仅在确认「直连本机、无同机反代」时开启（例如本机脚本/打印服务需要免登录调用）。" />
        <el-form-item style="margin-top: 12px">
          <el-button type="primary" :loading="savingLocalNoAuth" @click="saveLocalNoAuth">保存</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 修谱助手（AI） -->
    <el-card class="section">
      <template #header><span>🤖 修谱助手（AI）</span></template>
      <el-form label-width="120px" class="settings-form">
        <el-form-item label="启用助手">
          <el-switch v-model="ai.enabled" />
          <span class="form-hint">{{ ai.enabled ? '已开启' : '已关闭（默认）' }}</span>
        </el-form-item>
        <el-form-item label="服务提供方">
          <el-select v-model="ai.provider" style="width: 280px" @change="onProviderChange">
            <el-option v-for="p in ai.presets" :key="p.id" :label="p.label" :value="p.id" />
          </el-select>
          <el-button class="ml" :loading="testingAI" @click="testAI">测试连接</el-button>
          <span class="form-hint">{{ currentPreset?.hint || '任意 OpenAI 兼容端点均可（云端或本地部署）' }}</span>
        </el-form-item>
        <el-form-item label="API Key" v-if="needKey">
          <el-input v-model="ai.api_key" type="password" show-password :placeholder="currentPreset?.key_placeholder || 'API Key（仅保存在服务端，不回显明文）'" />
          <span class="form-hint" v-if="ai.has_api_key">当前已存：{{ ai.api_key_masked }}（留空表示不修改）
            <el-button link type="danger" size="small" @click="clearKey">清除已存密钥</el-button>
          </span>
          <span class="form-hint" v-else>尚未保存密钥{{ currentPreset?.needs_key ? '（该供应商通常需要）' : '（本地部署一般不需要）' }}</span>
        </el-form-item>
        <el-form-item label="模型名称">
          <el-input v-model="ai.model" :placeholder="currentPreset?.model || '如 qwen2.5:7b / deepseek-chat / gpt-4o-mini'" />
        </el-form-item>
        <el-form-item label="接口地址">
          <el-input v-model="ai.base_url" :placeholder="currentPreset?.base_url || '如 http://127.0.0.1:11434/v1'" />
          <span class="form-hint">填到 <code>/v1</code> 或 <code>/api/v3</code> 一级即可，后端会自动补 <code>/chat/completions</code></span>
        </el-form-item>
        <el-form-item label="高级">
          <span class="adv-row">
            最大生成长度
            <el-input-number v-model="ai.max_tokens" :min="256" :max="32768" :step="256" size="small" controls-position="right" style="width: 130px" />
            <span class="form-hint">小传生成建议 ≥2048</span>
          </span>
          <span class="adv-row">
            超时（秒）
            <el-input-number v-model="ai.timeout_sec" :min="10" :max="900" :step="10" size="small" controls-position="right" style="width: 110px" />
          </span>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="savingAI" @click="saveAI">保存 AI 设置</el-button>
          <span class="form-hint" v-if="testResult" :style="{ color: testResult.ok ? '#67c23a' : '#c45656' }">{{ testResult.message }}</span>
        </el-form-item>
      </el-form>
      <el-alert type="info" :closable="false" class="ai-install-tip">
        <template #title>{{ currentPreset?.needs_key ? '☁️ 云端大模型：填好 API Key 即可用' : '📦 本地部署：需先自行启动模型服务' }}</template>
        <div class="install-steps">
          <span v-if="!currentPreset?.needs_key">
            1. 本地推理引擎任选：<b>llama.cpp llama-server</b>（默认 18110 端口）、<b>Ollama</b>（11434）、<b>vLLM</b>（8000）；<br/>
            2. 按上文「接口地址」启动对应服务（都提供 OpenAI 兼容接口）；<br/>
            3. 填模型名 → 点「测试连接」确认可达 → 保存并开启开关。<br/>
            <span class="install-note">提示：后端在 Docker 容器内时，本地服务地址要填宿主机可达地址（如 http://host.docker.internal:11434），不能用容器内的 127.0.0.1。</span>
          </span>
          <span v-else>
            1. 到对应厂商控制台开通并创建 API Key；<br/>
            2. 在「API Key」处粘贴 Key（仅保存在服务端，页面上只显示打码值）；<br/>
            3. 填模型名（如 deepseek-chat / qwen-plus / gpt-4o-mini / doubao 接入点 ep-xxx）；<br/>
            4. 「测试连接」通过后保存并开启开关即可，无需本地部署；<br/>
            5. 任何厂商都可选「自定义」手动填 Base URL 与模型名。
          </span>
        </div>
      </el-alert>
    </el-card>
    <!-- 操作日志 -->
    <el-card class="section">
      <template #header>
        <div class="log-card-head">
          <span>📋 操作日志</span>
          <div class="log-tools">
            <el-select v-model="logFilter.action" placeholder="全部动作" clearable size="small" style="width:120px" @change="loadLogs(1)">
              <el-option label="登录" value="登录" />
              <el-option label="登录失败" value="登录失败" />
              <el-option label="新增" value="新增" />
              <el-option label="修改" value="修改" />
              <el-option label="删除" value="删除" />
              <el-option label="审核" value="审核" />
              <el-option label="备份" value="备份" />
              <el-option label="还原" value="还原" />
              <el-option label="更新" value="更新" />
              <el-option label="回滚" value="回滚" />
              <el-option label="导入" value="导入" />
              <el-option label="上传" value="上传" />
            </el-select>
            <el-input v-model="logFilter.keyword" placeholder="搜索用户/内容" clearable size="small" style="width:170px" @keyup.enter="loadLogs(1)" @clear="loadLogs(1)">
              <template #append><el-button @click="loadLogs(1)">搜索</el-button></template>
            </el-input>
            <el-button size="small" @click="loadLogs(logPage)">刷新</el-button>
            <el-popconfirm title="确定清空全部操作日志？此操作不可恢复" @confirm="clearLogs">
              <template #reference><el-button size="small" type="danger" plain>清空</el-button></template>
            </el-popconfirm>
          </div>
        </div>
      </template>
      <div class="log-retention">
        <span class="lr-label">自动清理：</span>
        <el-switch v-model="retentionEnabled" @change="onRetentionToggle" />
        <template v-if="retentionEnabled">
          <span class="lr-hint">保留最近</span>
          <el-input-number v-model="retentionDays" :min="1" :max="3650" :step="10" size="small" controls-position="right" style="width:110px" />
          <span class="lr-hint">天的日志，超期自动删除</span>
          <el-button size="small" type="primary" plain @click="saveRetention">保存</el-button>
        </template>
        <span v-else class="lr-hint">已关闭自动清理（日志永久保留，需手动清空）</span>
      </div>
      <el-table :data="logs" v-loading="logsLoading" size="small" stripe empty-text="暂无操作记录" style="width:100%">
        <el-table-column label="时间" prop="created_at" width="170">
          <template #default="{ row }"><span class="log-time">{{ fmtTime(row.created_at) }}</span></template>
        </el-table-column>
        <el-table-column label="用户" width="110">
          <template #default="{ row }"><span>{{ row.username || ('用户#' + row.user_id) }}</span></template>
        </el-table-column>
        <el-table-column label="动作" width="80">
          <template #default="{ row }"><el-tag :type="actionTag(row.action)" size="small">{{ row.action }}</el-tag></template>
        </el-table-column>
        <el-table-column label="对象" prop="target" width="110" />
        <el-table-column label="内容" prop="detail" min-width="220" show-overflow-tooltip />
        <el-table-column label="IP" prop="ip" width="130" />
      </el-table>
      <div class="log-pager" v-if="logTotal > logFilter.size">
        <el-pagination
          background layout="prev, pager, next, total" :total="logTotal"
          :page-size="logFilter.size" :current-page="logPage"
          @current-change="loadLogs" />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'

// ===== 登录安全（防爆破） =====
const security = reactive({ max_failures: 5, lock_minutes: 15 })
const savingSecurity = ref(false)

// 本机（127.0.0.1）访问免登录开关（默认关；仅理事长/超管可改）
const localNoAuth = reactive({ enabled: false })
const savingLocalNoAuth = ref(false)
async function loadLocalNoAuth() {
  try {
    const { data } = await api.get('/settings/local-no-auth')
    localNoAuth.enabled = !!data.enabled
  } catch (e) { /* 无权限或未启用该接口时忽略 */ }
}
async function saveLocalNoAuth() {
  savingLocalNoAuth.value = true
  try {
    const { data } = await api.put('/settings/local-no-auth', { enabled: localNoAuth.enabled })
    localNoAuth.enabled = !!data.enabled
    ElMessage.success(data.enabled ? '已开启本机免登录（127.0.0.1 访问按族员放行）' : '已关闭本机免登录')
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '保存失败')
  } finally {
    savingLocalNoAuth.value = false
  }
}
onMounted(loadLocalNoAuth)

// ===== 修谱助手（AI） =====
const ai = reactive({
  enabled: false, provider: 'local', base_url: '', model: '',
  api_key: '', api_key_masked: '', has_api_key: false,
  max_tokens: 2048, timeout_sec: 180,
  presets: []
})
const savingAI = ref(false)
const testingAI = ref(false)
const testResult = ref(null)
const clearingKey = ref(false)

const currentPreset = computed(() => ai.presets.find(p => p.id === ai.provider) || null)
const needKey = computed(() => !!currentPreset.value?.needs_key || ai.provider === 'custom' || ai.has_api_key)

async function loadSecurity() {
  try {
    const { data } = await api.get('/settings/login-security')
    if (data) {
      security.max_failures = data.max_failures ?? 5
      security.lock_minutes = data.lock_minutes ?? 15
    }
  } catch (e) { /* ignore */ }
}

async function saveSecurity() {
  savingSecurity.value = true
  try {
    await api.put('/settings/login-security', { max_failures: security.max_failures, lock_minutes: security.lock_minutes })
    ElMessage.success('登录安全设置已保存')
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '保存失败'
    ElMessage.error('保存失败：' + msg)
  } finally {
    savingSecurity.value = false
  }
}

async function loadAI() {
  try {
    const { data } = await api.get('/ai/config')
    if (data) {
      ai.enabled = !!data.enabled
      ai.provider = data.provider || 'local'
      ai.presets = Array.isArray(data.provider_presets) ? data.provider_presets : []
      const preset = ai.presets.find(p => p.id === ai.provider)
      ai.base_url = data.base_url || preset?.base_url || ''
      ai.model = data.model || preset?.model || ''
      ai.api_key_masked = data.api_key_masked || ''
      ai.has_api_key = !!data.has_api_key || !!data.api_key_masked
      ai.max_tokens = data.max_tokens || 2048
      ai.timeout_sec = data.timeout_sec || 180
      ai.api_key = '' // 不把明文回填，留空表示不修改
      testResult.value = null
    }
  } catch (e) { /* ignore */ }
}

// 切换提供方时套用该预设的默认地址/模型（用户仍可手改）
function onProviderChange(val) {
  const p = ai.presets.find(x => x.id === val)
  if (p) {
    ai.base_url = p.base_url || ''
    ai.model = p.model || ''
  }
  ai.api_key = ''
  testResult.value = null
}

// 清除已保存的密钥（后端 clear_api_key）
async function clearKey() {
  try {
    await ElMessageBox.confirm('确定清除服务端已保存的 API Key？清除后该供应商将无法调用。', '清除密钥', { type: 'warning' })
  } catch (e) { return }
  try {
    await api.put('/ai/config', { clear_api_key: true })
    ElMessage.success('已清除 API Key')
    loadAI()
  } catch (e) {
    ElMessage.error('清除失败：' + (e?.response?.data?.error || e.message))
  }
}

// 测试连通性（探测 {base}/models）
async function testAI() {
  testingAI.value = true
  testResult.value = null
  try {
    const { data } = await api.post('/ai/test', {
      provider: ai.provider, base_url: ai.base_url, model: ai.model, api_key: ai.api_key || undefined
    }, { timeout: 20000 })
    testResult.value = {
      ok: !!data.ok,
      message: (data.ok ? '✅ 连接成功' : '❌ ' + (data.message || '连接失败')) +
               (data.models && data.models.length ? '（可用模型 ' + data.models.length + ' 个）' : '')
    }
  } catch (e) {
    testResult.value = { ok: false, message: '❌ ' + (e?.response?.data?.error || e.message || '测试失败') }
  } finally {
    testingAI.value = false
  }
}

async function saveAI() {
  savingAI.value = true
  try {
    const payload = {
      enabled: ai.enabled,
      provider: ai.provider,
      base_url: ai.base_url,
      model: ai.model,
      max_tokens: ai.max_tokens,
      timeout_sec: ai.timeout_sec
    }
    // 只有填了新的 Key 才传（留空则不修改已保存的 Key）
    if (ai.api_key && ai.api_key.trim()) {
      payload.api_key = ai.api_key.trim()
    }
    await api.put('/ai/config', payload)
    ElMessage.success('AI 设置已保存')
    loadAI()
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '保存失败'
    ElMessage.error('保存失败：' + msg)
  } finally {
    savingAI.value = false
  }
}

onMounted(() => {
  loadSecurity()
  loadAI()
  loadLogs(1)
  loadRetention()
})

// ===== 操作日志 =====
const logs = ref([])
const logsLoading = ref(false)
const logTotal = ref(0)
const logPage = ref(1)
const logFilter = reactive({ action: '', keyword: '', size: 20 })

async function loadLogs(page) {
  if (page) logPage.value = page
  logsLoading.value = true
  try {
    const params = new URLSearchParams()
    params.set('page', logPage.value)
    params.set('size', logFilter.size)
    if (logFilter.action) params.set('action', logFilter.action)
    if (logFilter.keyword && logFilter.keyword.trim()) params.set('keyword', logFilter.keyword.trim())
    const { data } = await api.get('/operation-logs?' + params.toString())
    logs.value = data.list || []
    logTotal.value = data.total || 0
  } catch (e) {
    logs.value = []
  } finally {
    logsLoading.value = false
  }
}

async function clearLogs() {
  try {
    await api.delete('/operation-logs')
    ElMessage.success('操作日志已清空')
    loadLogs(1)
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '清空失败'
    ElMessage.error('清空失败：' + msg)
  }
}

function actionTag(a) {
  return ({ '删除': 'danger', '还原': 'danger', '登录失败': 'danger', '登录': 'success', '新增': 'primary', '上传': 'primary', '更新': 'warning', '回滚': 'warning', '审核': 'warning', '修改': 'info', '备份': 'info', '导入': 'info' })[a] || ''
}

// 操作日志保留策略（默认自动清理 180 天，可关闭、可调天数）
const retentionEnabled = ref(true)
const retentionDays = ref(180)

async function loadRetention() {
  try {
    const { data } = await api.get('/settings/oplog-retention')
    const d = data.retention_days ?? 180
    retentionDays.value = d > 0 ? d : 180
    retentionEnabled.value = d > 0
  } catch (e) { /* 默认 180 */ }
}

function onRetentionToggle(v) {
  if (v) retentionDays.value = retentionDays.value || 180
  saveRetention()
}

async function saveRetention() {
  const days = retentionEnabled.value ? Math.max(1, Math.min(3650, retentionDays.value || 180)) : 0
  try {
    await api.put('/settings/oplog-retention', { retention_days: days })
    ElMessage.success(days > 0 ? `已设置：自动清理 ${days} 天前的日志` : '已关闭自动清理')
    retentionDays.value = days > 0 ? days : 180
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '保存失败'
    ElMessage.error('保存失败：' + msg)
  }
}

function fmtTime(s) {
  if (!s) return ''
  // 兼容数据库 UTC 时间，转本地显示
  let d = new Date(s.includes('T') ? s : s.replace(' ', 'T') + 'Z')
  if (isNaN(d)) return s
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}
</script>

<style scoped>
.server-settings-page { padding: 0; max-width: 1000px; }
.page-title { font-size: 18px; color: var(--ink); margin: 0; }
.page-desc { color: #7a6a52; font-size: 13px; margin: 8px 0 0; }
.section { margin-bottom: 16px; }
.settings-form { max-width: 500px; }
.form-hint { margin-left: 10px; color: #999; font-size: 12px; }
.ai-install-tip { margin-top: 4px; }
.install-steps { font-size: 13px; line-height: 1.9; color: #666; }
.install-note { color: #b87333; }
.log-card-head { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px; }
.log-tools { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.log-time { white-space: nowrap; color: #555; }
.log-pager { margin-top: 12px; display: flex; justify-content: flex-end; }
.log-retention { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 12px; padding: 10px 12px; background: #f7f4ef; border-radius: 6px; font-size: 13px; color: #555; }
.log-retention .lr-label { font-weight: 600; color: #333; }
.log-retention .lr-hint { color: #777; }
@media (max-width: 767px) { .server-settings-page { max-width: 100%; } .settings-form { max-width: 100%; } }
.adv-row { display: inline-flex; align-items: center; gap: 6px; margin-right: 18px; }
.install-note { color: #9a8b78; }
.ml { margin-left: 10px; }
</style>
