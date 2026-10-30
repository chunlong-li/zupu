<template>
  <el-dialog v-model="vis" :title="title" width="520px" :close-on-click-modal="false">
    <template v-if="listMode">
      <div class="apw-tip">
        共 <b>{{ items.length }}</b> 位成员的初始密码（账号 = 姓名，<b>首次登录时自动创建账号</b>）。
        已有账号/已恢复原密码的成员不再重新生成，不在此列表中。
        请先保存/转告本人，<b>首次登录需修改密码</b>：
      </div>
      <div class="apw-list">
        <div class="apw-li apw-li-head"><span>姓名</span><span>初始密码</span></div>
        <div v-for="(it, i) in items" :key="i" class="apw-li">
          <span class="apw-nm">{{ it.name }}</span>
          <span class="apw-code-sm">{{ it.password }}</span>
        </div>
      </div>
      <div class="apw-warn">⚠️ 此密码只在此处显示，关闭后无法再次查看；忘了可在「系统设置 → 用户管理」里重置。</div>
    </template>
    <template v-else>
      <div class="apw-tip">
        已算出该成员的初始密码（账号 = 姓名，<b>首次登录时自动创建账号</b>）。
        请转告本人，<b>首次登录需修改密码</b>：
      </div>
      <div class="apw-row"><span class="apw-label">账号</span><span class="apw-user">{{ name }}</span></div>
      <div class="apw-pw"><span class="apw-code">{{ password }}</span></div>
      <div class="apw-warn">⚠️ 此密码只在此处显示，关闭后无法再次查看；忘了可在「系统设置 → 用户管理」里重置。</div>
    </template>
    <template #footer>
      <el-button @click="copyTextToClip">{{ copied ? '已复制' : (listMode ? '复制全部' : '复制密码') }}</el-button>
      <el-button type="primary" @click="vis = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'

const props = defineProps({
  visible: { type: Boolean, default: false },
  name: { type: String, default: '' },
  password: { type: String, default: '' },
  items: { type: Array, default: () => [] }
})
const emit = defineEmits(['update:visible'])
const vis = computed({ get: () => props.visible, set: v => emit('update:visible', v) })
const copied = ref(false)

const listMode = computed(() => Array.isArray(props.items) && props.items.length > 0)
const title = computed(() => (listMode.value ? '成员初始密码一览' : '成员初始密码'))

watch(() => props.visible, v => { if (v) copied.value = false })

function writeClipboard(txt) {
  if (navigator.clipboard && window.isSecureContext) return navigator.clipboard.writeText(txt)
  return new Promise((resolve, reject) => {
    try {
      const ta = document.createElement('textarea')
      ta.value = txt
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(ta)
      ok ? resolve() : reject(new Error('copy failed'))
    } catch (e) { reject(e) }
  })
}

async function copyTextToClip() {
  const txt = listMode.value
    ? props.items.map(it => `${it.name} ${it.password}`).join('\n')
    : (props.password || '')
  if (!txt) return
  try {
    await writeClipboard(txt)
    copied.value = true
  } catch (e) {
    ElMessage.warning('复制失败，请手动记录')
  }
}
</script>

<style scoped>
.apw-tip { font-size: 13px; color: #6b5a44; line-height: 1.7; margin-bottom: 10px; }
.apw-row { display: flex; align-items: center; gap: 8px; font-size: 14px; margin-bottom: 8px; }
.apw-label { color: #8a7a64; }
.apw-user { font-weight: 600; color: #3a3226; }
.apw-pw { display: flex; align-items: center; gap: 10px; background: #faf6ef; border: 1px dashed #d8c6a8; border-radius: 8px; padding: 10px 12px; }
.apw-code { font-family: ui-monospace, Menlo, Consolas, monospace; font-size: 22px; letter-spacing: 3px; color: #8c5a3c; font-weight: 700; }
.apw-code-sm { font-family: ui-monospace, Menlo, Consolas, monospace; font-size: 15px; letter-spacing: 1px; color: #8c5a3c; font-weight: 700; }
.apw-warn { font-size: 12px; color: #b07a3a; margin-top: 10px; line-height: 1.6; }
.apw-list { max-height: 48vh; overflow: auto; border: 1px solid #ece2d2; border-radius: 8px; }
.apw-li { display: flex; justify-content: space-between; gap: 12px; padding: 7px 12px; font-size: 14px; border-bottom: 1px solid #f4ecdf; }
.apw-li:last-child { border-bottom: none; }
.apw-li-head { background: #faf6ef; color: #8a7a64; font-size: 12px; font-weight: 600; position: sticky; top: 0; }
.apw-nm { color: #3a3226; }
</style>
