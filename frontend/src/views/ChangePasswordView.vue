<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <h1 class="login-title">修改密码</h1>
        <p class="login-sub" v-if="user">欢迎，{{ user.username }}</p>
      </div>
      <el-form ref="formRef" :model="form" :rules="rules" @submit.prevent="handleChange">
        <el-form-item prop="old_password" v-if="!isFirstLogin">
          <el-input v-model="form.old_password" type="password" placeholder="请输入旧密码" size="large" show-password autocomplete="current-password">
            <template #prefix>🔑</template>
          </el-input>
        </el-form-item>
        <el-form-item prop="new_password">
          <el-input v-model="form.new_password" type="password" placeholder="请输入新密码" size="large" show-password autocomplete="new-password">
            <template #prefix>🔒</template>
          </el-input>
        </el-form-item>
        <el-form-item prop="confirm_password">
          <el-input v-model="form.confirm_password" type="password" placeholder="请确认新密码" size="large" show-password autocomplete="new-password">
            <template #prefix>🔒</template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" :loading="loading" @click="handleChange" style="width:100%">
            确认修改
          </el-button>
        </el-form-item>
        <el-form-item>
          <el-button size="large" @click="handleSkip" style="width:100%" v-if="isFirstLogin">
            暂不修改，进入系统
          </el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>

  <!-- 首次登录使用说明弹窗（盖在修改密码上层） -->
  <el-dialog v-model="guideVisible" title="📖 使用说明" width="700px" top="6vh" :close-on-click-modal="false">
    <div class="guide-scroll">
      <UsageGuideContent :is-admin="isAdminUser" />
    </div>
    <template #footer>
      <el-button type="primary" size="large" style="width:100%" @click="guideVisible = false">我知道了</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import { useRouter } from 'vue-router'
import UsageGuideContent from '../components/UsageGuideContent.vue'

const router = useRouter()
const loading = ref(false)
const guideVisible = ref(false)
const user = ref(null)

const form = ref({ old_password: '', new_password: '', confirm_password: '' })

const isFirstLogin = computed(() => {
  const u = user.value || JSON.parse(localStorage.getItem('user') || '{}')
  return u.need_change_pw
})

const isAdminUser = computed(() => {
  const u = user.value || JSON.parse(localStorage.getItem('user') || '{}')
  return u.role === '理事长' || u.role === '超级管理员'
})

const rules = {
  old_password: isFirstLogin.value ? [] : [{ required: true, message: '请输入旧密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 4, message: '密码至少4位', trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== form.value.new_password) {
          callback(new Error('两次密码不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

async function handleChange() {
  loading.value = true
  try {
    await api.post('/auth/change-password', {
      old_password: form.value.old_password,
      new_password: form.value.new_password
    })
    ElMessage.success('密码修改成功')
    const u = JSON.parse(localStorage.getItem('user') || '{}')
    u.need_change_pw = false
    localStorage.setItem('user', JSON.stringify(u))
    router.push('/dashboard')
  } catch (e) {
    const msg = e.response?.data?.error || '修改失败'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

function handleSkip() {
  router.push('/dashboard')
}

onMounted(() => {
  user.value = JSON.parse(localStorage.getItem('user') || 'null')
  // 首次登录（需修改密码）时，弹出使用说明
  if (user.value && user.value.need_change_pw) {
    guideVisible.value = true
  }
})
</script>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--mibai);
}
.login-card {
  width: 400px;
  padding: 40px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 4px 24px rgba(0,0,0,0.08);
}
.login-header {
  text-align: center;
  margin-bottom: 32px;
}
.login-title {
  font-size: 24px;
  color: #3a3226;
  letter-spacing: 4px;
  margin: 0 0 8px;
}
.login-sub {
  color: #8c5a3c;
  font-size: 14px;
  margin: 0;
}
.guide-scroll {
  max-height: 62vh;
  overflow-y: auto;
  padding-right: 8px;
}
</style>