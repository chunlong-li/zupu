<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <h1 class="login-title">{{ familyName || '族谱系统' }}</h1>
        <p class="login-sub">用户登录</p>
      </div>
      <el-form ref="formRef" :model="form" :rules="rules" @submit.prevent="handleLogin">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="请输入姓名" size="large" clearable autocomplete="username">
            <template #prefix>👤</template>
          </el-input>
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="请输入密码" size="large" show-password autocomplete="current-password" @keyup.enter="handleLogin">
            <template #prefix>🔒</template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" :loading="loading" @click="handleLogin" style="width:100%">
            登 录
          </el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import { useRouter } from 'vue-router'
import { resetBranchContext } from '../utils/branchContext'

const router = useRouter()
const familyName = ref('')
const loading = ref(false)
const form = ref({ username: '', password: '' })
const rules = {
  username: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

async function handleLogin() {
  loading.value = true
  try {
    const { data } = await api.post('/auth/login', { ...form.value })
    localStorage.setItem('token', data.token)
    localStorage.setItem('user', JSON.stringify(data.user))
    if (data.role_aliases) {
      localStorage.setItem('role_aliases', JSON.stringify(data.role_aliases))
    }
    api.defaults.headers.Authorization = 'Bearer ' + data.token

    // 丢弃上一个账号残留的房支上下文；本账号自己的选择记录仍在 localStorage 里，
    // 进首页后 loadBranches() 会按「本账号上次的选择 / 账号所属房支」重新定默认
    resetBranchContext()

    if (data.user.need_change_pw) {
      router.push('/change-password')
    } else {
      router.push('/dashboard')
    }
  } catch (e) {
    const msg = e.response?.data?.error || '登录失败'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

async function loadFamilyName() {
  try {
    const { data } = await api.get('/settings/family-surname')
    familyName.value = data.family_name || data.family_surname || ''
  } catch (e) { /* */ }
}

onMounted(loadFamilyName)
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
  width: 380px;
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
</style>