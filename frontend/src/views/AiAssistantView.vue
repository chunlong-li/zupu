<template>
  <div class="ai-page">
    <h2 class="page-title">🤖 修谱助手</h2>
    <p class="page-desc">AI 辅助修谱：校验、语病提醒、语言美化、修谱建议</p>
    <el-divider />

    <!-- 未启用提示 -->
    <el-alert v-if="statusLoaded && !enabled" type="warning" :closable="false" show-icon class="disable-tip">
      <template #title>
        修谱助手未启用。请联系管理员（理事长/超级管理员）在「系统设置 → 服务器设置」中开启，并选择「本地模型」或「火山方舟」进行配置后即可使用。
      </template>
    </el-alert>

    <template v-if="enabled">
      <!-- 模式选择 -->
      <el-card class="section">
        <template #header><span>选择处理方式</span></template>
        <el-radio-group v-model="mode" class="mode-group">
          <el-radio-button value="check">🔍 校验</el-radio-button>
          <el-radio-button value="grammar">✏️ 语病提醒</el-radio-button>
          <el-radio-button value="polish">✨ 语言美化</el-radio-button>
          <el-radio-button value="advice">📋 修谱建议</el-radio-button>
        </el-radio-group>
        <p class="mode-hint">{{ modeHint }}</p>
      </el-card>

      <!-- 输入 -->
      <el-card class="section">
        <template #header><span>输入文字</span></template>
        <el-input
          v-model="content"
          type="textarea"
          :rows="8"
          maxlength="20000"
          show-word-limit
          placeholder="粘贴要处理的谱序、传记、世系记录等文字…"
        />
        <div class="actions">
          <el-button type="primary" :loading="loading" :disabled="!content.trim()" @click="submit">
            提交处理
          </el-button>
          <el-button v-if="reply" @click="clear">清空</el-button>
        </div>
      </el-card>

      <!-- 输出 -->
      <el-card class="section" v-if="reply || loading">
        <template #header><span>处理结果</span></template>
        <div v-if="loading" class="loading-box">
          <el-skeleton :rows="6" animated />
          <p class="loading-text">AI 推理中，请稍候（约几秒到几十秒）…</p>
        </div>
        <pre v-else class="result-box">{{ reply }}</pre>
      </el-card>
    </template>

    <el-alert type="info" :closable="false" class="redline-tip">
      <template #title>
        ⚠️ AI 只做文字加工与建议，不编造人名、世系、生卒年月等事实数据；处理结果请人工核对后使用。
      </template>
    </el-alert>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'

const enabled = ref(false)
const statusLoaded = ref(false)
const mode = ref('check')
const content = ref('')
const reply = ref('')
const loading = ref(false)

const modeHint = computed(() => ({
  check: '检查世系接续、生卒年月、父子年龄、辈分是否矛盾或可疑',
  grammar: '指出错别字、病句、用词不当、标点问题并给修改建议',
  polish: '润色为庄重典雅的文言/书面语，保持原意、不增删事实',
  advice: '从体例、内容完整性、资料补全角度给出专业建议'
}[mode.value]))

async function loadStatus() {
  try {
    const { data } = await api.get('/ai/status')
    enabled.value = !!data.enabled
  } catch (e) {
    /* ignore */
  } finally {
    statusLoaded.value = true
  }
}

async function submit() {
  if (!content.value.trim()) return
  loading.value = true
  reply.value = ''
  try {
    const { data } = await api.post('/ai/chat',
      { mode: mode.value, content: content.value.trim() },
      { timeout: 190000 } // 本地模型推理较慢，单独放宽超时
    )
    reply.value = data.reply || ''
    if (!reply.value) ElMessage.warning('模型未返回内容')
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '处理失败'
    ElMessage.error('处理失败：' + msg)
  } finally {
    loading.value = false
  }
}

function clear() {
  content.value = ''
  reply.value = ''
}

onMounted(loadStatus)
</script>

<style scoped>
.ai-page { max-width: 760px; }
.page-title { font-size: 18px; color: #3a3226; margin: 0; }
.page-desc { color: #7a6a52; font-size: 13px; margin: 8px 0 0; }
.disable-tip { margin-bottom: 16px; }
.section { margin-bottom: 16px; }
.mode-group { flex-wrap: wrap; }
.mode-hint { color: #999; font-size: 12px; margin: 10px 0 0; }
.actions { margin-top: 12px; }
.loading-box { padding: 8px 0; }
.loading-text { color: #999; font-size: 12px; margin-top: 8px; }
.result-box {
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
  font-size: 14px;
  line-height: 1.8;
  color: #3a3226;
  background: #faf6ef;
  border: 1px solid #eee4d5;
  border-radius: 6px;
  padding: 14px;
  margin: 0;
  max-height: 520px;
  overflow: auto;
}
.redline-tip { margin-top: 4px; }
@media (max-width: 767px) { .ai-page { max-width: 100%; } }
</style>
