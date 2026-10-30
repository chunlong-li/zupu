<template>
  <div class="layout">
    <!-- 顶部导航栏 -->
    <header class="topbar">
      <div class="topbar-inner">
        <!-- 移动端：汉堡按钮在左 -->
        <el-button class="menu-toggle" v-if="isMobile && isLoggedIn" @click="drawerVisible = true" text>
          <span style="font-size:20px">☰</span>
        </el-button>
        <span class="menu-toggle-placeholder" v-if="isMobile && !isLoggedIn"></span>
        <!-- 家族名 + 房支下拉（居中） -->
        <span class="center-area">
          <span class="serif logo" @click="$router.push('/dashboard')">{{ familyName || '族谱' }}</span>
          <span class="branch-picker" v-if="isLoggedIn && branchCtx.loaded">
            <el-tree-select
              :model-value="branchCtx.current"
              @update:model-value="onBranchChange"
              :data="branchTree"
              node-key="value"
              :props="{ label: 'label', children: 'children' }"
              check-strictly
              :render-after-expand="false"
              size="small"
              class="branch-select"
              popper-class="branch-select-popper"
              placeholder="选择房支"
            />
            <el-checkbox v-if="!isMobile" :model-value="branchCtx.includeChildren" @update:model-value="onIncludeChange" class="branch-inc">含下级</el-checkbox>
          </span>
        </span>
        <!-- 右侧：未登录显示登录按钮，已登录显示用户头像 + 退出 -->
        <span class="auth-area">
          <!-- 未登录、或本机免登录访客 → 显示「登录」按钮（访客想以管理员身份登录时直接点，无需先点退出） -->
          <el-button v-if="!isLoggedIn || isGuest" size="small" @click="$router.push('/login')" class="login-btn">登录</el-button>
          <template v-else>
            <el-avatar :size="30" class="user-avatar" @click="profileVisible = true">{{ avatarText }}</el-avatar>
            <el-button size="small" text class="logout-btn" @click="handleLogout">退出</el-button>
          </template>
        </span>
      </div>
    </header>

    <!-- 下方内容区：左侧菜单 + 右侧内容 -->
    <div class="body-area">
      <!-- 桌面端左侧菜单 -->
      <aside class="sidebar" v-if="!isMobile">
        <el-menu ref="desktopMenuRef" :default-active="$route.path" :default-openeds="dashboardOpen" router background-color="#3a3226" text-color="#d9cbb6" active-text-color="#fff">
          <el-sub-menu index="dashboard">
            <template #title>📊 数据看板</template>
            <el-menu-item index="/dashboard">📈 数据看板</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/birthday">🎂 生日</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/deathday">🕯️ 祭日</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/traditions">🎏 传统日程</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/heatmap">🗺️ 热力分布</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/migrations">🚶 迁徙记录</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/contacts">📇 通讯录</el-menu-item>
          </el-sub-menu>
          <template v-if="isLoggedIn">
          <el-sub-menu index="viz">
            <template #title>🌳 世系可视化</template>
            <el-menu-item index="/tree">🏯 宝塔式</el-menu-item>
            <el-menu-item index="/sutree">🎐 苏式垂吊图</el-menu-item>
            <el-menu-item index="/susi">🕸️ 苏式垂丝图</el-menu-item>
            <el-menu-item index="/suzhu">📿 苏氏垂珠体</el-menu-item>
            <el-menu-item index="/gentree">🪜 世代分层图</el-menu-item>
            <el-menu-item index="/demo-dieji">📃 竖排牒记式</el-menu-item>
            <el-menu-item index="/demo-dieji-land">📄 横版牒记式</el-menu-item>
            <el-menu-item index="/cover-design">📕 谱书封面</el-menu-item>
            <el-menu-item index="/test-zupu">🌲 传代树</el-menu-item>
          </el-sub-menu>
          <el-menu-item index="/members">👥 成员管理</el-menu-item>
          <el-menu-item index="/branches">🏡 房支管理</el-menu-item>
          <el-menu-item index="/media">🖼️ 家族多媒体</el-menu-item>
          <el-menu-item index="/spouses-list">💍 配偶资料</el-menu-item>
          <el-menu-item index="/generation">📜 字辈派语</el-menu-item>
          <!-- 谱头文化：家族大事记 / 谱序 / 名人传记（三个子路由，各自高亮；父级默认折叠） -->
          <el-sub-menu index="culture">
            <template #title>🏛️ 谱头文化</template>
            <el-menu-item index="/culture/events">🗓️ 家族大事记</el-menu-item>
            <el-menu-item index="/culture/articles">✒️ 谱序</el-menu-item>
            <el-menu-item index="/culture/bios">👤 名人传记</el-menu-item>
          </el-sub-menu>
          <!-- 祠墓：把「电子祠堂 / 真实祠宇 / 真实祖茔」三类收进同一个父级分类（三个子项各自高亮） -->
          <el-sub-menu index="cimu">
            <template #title>🏮 祠墓</template>
            <el-menu-item index="/hall">🛕 祠堂</el-menu-item>
            <el-menu-item index="/shrines">🏯 祠宇志</el-menu-item>
            <el-menu-item index="/graves">🪦 茔墓志</el-menu-item>
          </el-sub-menu>
<el-menu-item index="/knowledge" v-if="showKnowledge">📚 家族知识库</el-menu-item>
<el-menu-item index="/accounts" v-if="showAccounts">💰 收支公示<template v-if="accountsPending"> ({{ accountsPending }})</template></el-menu-item>
<el-menu-item index="/merit" v-if="showAccounts">🪦 功德碑</el-menu-item>
<el-menu-item index="/ai-assistant" v-if="isChiefOrAdmin">🤖 修谱助手</el-menu-item>
<el-menu-item index="/review" v-if="isChiefOrAdmin">✅ 审核中心</el-menu-item>
          <el-sub-menu index="settings" v-if="isChiefOrAdmin">
            <template #title>⚙️ 系统设置</template>
            <el-menu-item index="/settings">🏷️ 家族设置</el-menu-item>
            <el-menu-item index="/data-check">🩺 数据体检</el-menu-item>
            <el-menu-item index="/backup">💾 备份管理</el-menu-item>
            <el-menu-item index="/update">⬆️ 版本更新</el-menu-item>
            <el-menu-item index="/server-settings">🖥️ 服务器设置</el-menu-item>
            <el-menu-item index="/map-offline">🗺️ 离线地图</el-menu-item>
            <el-menu-item index="/usage-guide">📖 使用说明</el-menu-item>
            <el-menu-item index="/about">ℹ️ 关于我们</el-menu-item>
          </el-sub-menu>
          </template>
        </el-menu>
        <div class="sidebar-footer">
          <span class="version">v{{ version }}</span>
        </div>
      </aside>

      <!-- 主内容区 -->
      <main class="main" :class="{ 'mobile-main': isMobile, 'kb-main': $route.path === '/knowledge' }">
        <router-view :key="$route.fullPath + '#' + branchKey" />
      </main>
    </div>

    <!-- 移动端抽屉导航 -->
    <el-drawer v-model="drawerVisible" direction="ltr" size="200px" :with-header="false" v-if="isMobile">
      <div class="drawer-nav">
        <div class="drawer-logo serif">{{ familyName || '族谱' }}</div>
        <el-menu ref="mobileMenuRef" :default-active="$route.path" :default-openeds="dashboardOpen" router background-color="#3a3226" text-color="#d9cbb6" active-text-color="#fff" @select="drawerVisible = false">
          <el-sub-menu index="dashboard">
            <template #title>📊 数据看板</template>
            <el-menu-item index="/dashboard">📈 数据看板</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/birthday">🎂 生日</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/deathday">🕯️ 祭日</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/traditions">🎏 传统日程</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/heatmap">🗺️ 热力分布</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/migrations">🚶 迁徙记录</el-menu-item>
            <el-menu-item v-if="isLoggedIn" index="/contacts">📇 通讯录</el-menu-item>
          </el-sub-menu>
          <template v-if="isLoggedIn">
          <el-sub-menu index="viz">
            <template #title>🌳 世系可视化</template>
            <el-menu-item index="/tree">🏯 宝塔式</el-menu-item>
            <el-menu-item index="/sutree">🎐 苏式垂吊图</el-menu-item>
            <el-menu-item index="/susi">🕸️ 苏式垂丝图</el-menu-item>
            <el-menu-item index="/suzhu">📿 苏氏垂珠体</el-menu-item>
            <el-menu-item index="/gentree">🪜 世代分层图</el-menu-item>
            <el-menu-item index="/demo-dieji">📃 竖排牒记式</el-menu-item>
            <el-menu-item index="/demo-dieji-land">📄 横版牒记式</el-menu-item>
            <el-menu-item index="/cover-design">📕 谱书封面</el-menu-item>
            <el-menu-item index="/test-zupu">🌲 传代树</el-menu-item>
          </el-sub-menu>
          <el-menu-item index="/members">👥 成员管理</el-menu-item>
          <el-menu-item index="/branches">🏡 房支管理</el-menu-item>
          <el-menu-item index="/media">🖼️ 家族多媒体</el-menu-item>
          <el-menu-item index="/spouses-list">💍 配偶资料</el-menu-item>
          <el-menu-item index="/generation">📜 字辈派语</el-menu-item>
          <!-- 谱头文化：家族大事记 / 谱序 / 名人传记（三个子路由，各自高亮；父级默认折叠） -->
          <el-sub-menu index="culture">
            <template #title>🏛️ 谱头文化</template>
            <el-menu-item index="/culture/events">🗓️ 家族大事记</el-menu-item>
            <el-menu-item index="/culture/articles">✒️ 谱序</el-menu-item>
            <el-menu-item index="/culture/bios">👤 名人传记</el-menu-item>
          </el-sub-menu>
          <!-- 祠墓：把「电子祠堂 / 真实祠宇 / 真实祖茔」三类收进同一个父级分类（三个子项各自高亮） -->
          <el-sub-menu index="cimu">
            <template #title>🏮 祠墓</template>
            <el-menu-item index="/hall">🛕 祠堂</el-menu-item>
            <el-menu-item index="/shrines">🏯 祠宇志</el-menu-item>
            <el-menu-item index="/graves">🪦 茔墓志</el-menu-item>
          </el-sub-menu>
<el-menu-item index="/knowledge" v-if="showKnowledge">📚 家族知识库</el-menu-item>
<el-menu-item index="/accounts" v-if="showAccounts">💰 收支公示<template v-if="accountsPending"> ({{ accountsPending }})</template></el-menu-item>
<el-menu-item index="/merit" v-if="showAccounts">🪦 功德碑</el-menu-item>
<el-menu-item index="/ai-assistant" v-if="isChiefOrAdmin">🤖 修谱助手</el-menu-item>
<el-menu-item index="/review" v-if="isChiefOrAdmin">✅ 审核中心</el-menu-item>
          <el-sub-menu index="settings" v-if="isChiefOrAdmin">
            <template #title>⚙️ 系统设置</template>
            <el-menu-item index="/settings">🏷️ 家族设置</el-menu-item>
            <el-menu-item index="/data-check">🩺 数据体检</el-menu-item>
            <el-menu-item index="/backup">💾 备份管理</el-menu-item>
            <el-menu-item index="/update">⬆️ 版本更新</el-menu-item>
            <el-menu-item index="/server-settings">🖥️ 服务器设置</el-menu-item>
            <el-menu-item index="/map-offline">🗺️ 离线地图</el-menu-item>
            <el-menu-item index="/usage-guide">📖 使用说明</el-menu-item>
            <el-menu-item index="/about">ℹ️ 关于我们</el-menu-item>
          </el-sub-menu>
          </template>
        </el-menu>
        <div class="drawer-footer">
          <div class="version">v{{ version }}</div>
        </div>
      </div>
    </el-drawer>

    <!-- 我的资料卡弹窗 -->
    <el-dialog v-model="profileVisible" title="我的资料" width="360px">
      <div class="profile-card">
        <el-avatar :size="56" class="profile-avatar">{{ avatarText }}</el-avatar>
        <div class="profile-name">{{ user.username }}</div>
        <el-tag :type="roleTagType(user.role)" effect="dark" size="small">{{ displayRole(user.role) }}</el-tag>
      </div>
      <el-descriptions :column="1" border size="small" class="profile-desc">
        <el-descriptions-item label="用户ID">{{ user.id }}</el-descriptions-item>
        <el-descriptions-item label="支系">{{ user.branch || '—' }}</el-descriptions-item>
      </el-descriptions>
      <div class="profile-actions">
        <el-button @click="guideVisible = true">使用帮助</el-button>
        <el-button type="primary" @click="openPwd">修改密码</el-button>
      </div>
    </el-dialog>

    <!-- 修改密码弹窗 -->
    <el-dialog v-model="pwdVisible" title="修改密码" width="360px">
      <el-form :model="pwdForm" label-width="84px">
        <el-form-item label="当前密码" v-if="!isFirstLogin">
          <el-input v-model="pwdForm.old_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="pwdForm.new_password" type="password" show-password />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input v-model="pwdForm.confirm_password" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdVisible = false">取消</el-button>
        <el-button type="primary" :loading="pwdLoading" @click="submitPwd">确定</el-button>
      </template>
    </el-dialog>

    <!-- 使用帮助弹窗（所有登录用户可用） -->
    <el-dialog v-model="guideVisible" title="📖 使用帮助" width="700px" top="6vh" :close-on-click-modal="false">
      <div class="guide-scroll">
        <UsageGuideContent :is-admin="isChiefOrAdmin" />
      </div>
      <template #footer>
        <el-button type="primary" @click="guideVisible = false">我知道了</el-button>
      </template>
    </el-dialog>

    <!-- 赞助作者：全局仅挂一次；谱书/打印视图不显示 -->
    <SponsorFab v-if="!isBookView" />

  </div>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter, useRoute } from 'vue-router'
import api from './api'
import { displayRole } from './utils/auth'
import UsageGuideContent from './components/UsageGuideContent.vue'
import SponsorFab from './components/SponsorFab.vue'
import { branchCtx, branchTree, loadBranches, setCurrent, setIncludeChildren, resetBranchContext } from './utils/branchContext'

const router = useRouter()
const route = useRoute()
const familyName = ref('')
const version = ref(__BUILD_VERSION__)
const isMobile = ref(false)
const isBookView = computed(() => /\/(demo-dieji|demo-dieji-land|suzhu|suzhuti|cover-design|branch\/.*\/pushu)/.test(route.path))
const drawerVisible = ref(false)
const isLoggedIn = ref(false)
const isGuest = ref(false)        // 本机（127.0.0.1）免登录访客：菜单可用，但**不是登录状态**（顶栏仍显示「登录」，便于随时以管理员身份登录）
const isChiefOrAdmin = ref(false)
const accountsVisible = ref(false)
const accountsPending = ref(0)
const kbOnline = ref(false)
// 默认展开的子菜单：只有「数据看板」。
// 「祠墓」「谱头文化」两个父级默认**折叠**（属主要求）；进入各自子路由时会自动展开（见 openGroupIfNeeded），
// 这样用户点进那一项时仍能看到自己当前在哪。
const dashboardOpen = ['dashboard']

// 可折叠父级：进入其下任一子路由时自动展开（用户手动折叠后不再强行打开）
//   cimu    = 祠墓（祠堂 / 祠宇志 / 茔墓志）
//   culture = 谱头文化（家族大事记 / 谱序 / 名人传记）
const desktopMenuRef = ref(null)
const mobileMenuRef = ref(null)
const GROUP_ROUTES = {
  cimu: ['/hall', '/shrines', '/graves'],
  culture: ['/culture/events', '/culture/articles', '/culture/bios']
}
function openGroupIfNeeded(path) {
  const groups = Object.keys(GROUP_ROUTES).filter((g) => GROUP_ROUTES[g].includes(path))
  if (!groups.length) return
  nextTick(() => {
    for (const refObj of [desktopMenuRef, mobileMenuRef]) {
      for (const g of groups) {
        try { refObj.value && refObj.value.open && refObj.value.open(g) } catch (e) { /* ignore */ }
      }
    }
  })
}

// 「收支公示」菜单是否显示（功能已开启，或是可管理/审核的人员）
const showAccounts = computed(() => isLoggedIn.value && accountsVisible.value)
// 「家族知识库」菜单是否显示（传承服务器在线时）
const showKnowledge = computed(() => isLoggedIn.value && kbOnline.value)

// 我的资料卡
const user = ref({ id: 0, username: '', role: '', branch: '', need_change_pw: false })
const profileVisible = ref(false)
const guideVisible = ref(false)
const pwdVisible = ref(false)
const pwdLoading = ref(false)
const pwdForm = ref({ old_password: '', new_password: '', confirm_password: '' })

const avatarText = computed(() => {
  const n = user.value.username || '族'
  return n.charAt(0)
})

const isFirstLogin = computed(() => !!user.value.need_change_pw)

function roleTagType(role) {
  return { '族员': 'info', '理事': 'warning', '财务管理': 'success', '理事长': 'primary', '超级管理员': 'danger' }[role] || 'info'
}

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

// 顶部房支下拉
const branchKey = ref(0)
function onBranchChange(v) {
  setCurrent(v)
}
function onIncludeChange(v) {
  setIncludeChildren(v)
}
// 房支（或含下级）变化时，重挂当前页 → 触发各页重新加载
watch([() => branchCtx.current, () => branchCtx.includeChildren], () => {
  branchKey.value++
})

function handleLogout() {
  api.post('/auth/logout').catch(() => {})
  localStorage.removeItem('token')
  localStorage.removeItem('user')
  delete api.defaults.headers.Authorization
  isLoggedIn.value = false
  isChiefOrAdmin.value = false
  accountsVisible.value = false
  accountsPending.value = 0
  kbOnline.value = false
  localStorage.removeItem('local_guest')   // 退出后不再以本机访客身份直接进入（刷新页面会重新按本机访客放行）
  resetBranchContext()                      // 清掉内存/镜像里的房支上下文（各账号自己的选择记录保留）
  router.push('/dashboard')
}

function updateLoginState() {
  // 有 token = 正常登录；local_guest = 本机（127.0.0.1）免登录访客，按「族员」展示
  const hasToken = !!localStorage.getItem('token')
  isGuest.value = !hasToken && localStorage.getItem('local_guest') === '1'
  isLoggedIn.value = hasToken || isGuest.value
  try {
    const u = JSON.parse(localStorage.getItem('user') || 'null')
    user.value = u || { id: 0, username: '', role: '', branch: '', need_change_pw: false }
    isChiefOrAdmin.value = !!u && (u.role === '理事长' || u.role === '超级管理员')
  } catch (e) {
    user.value = { id: 0, username: '', role: '', branch: '', need_change_pw: false }
    isChiefOrAdmin.value = false
  }
}

function openPwd() {
  pwdForm.value = { old_password: '', new_password: '', confirm_password: '' }
  pwdVisible.value = true
}

async function submitPwd() {
  if (!pwdForm.value.new_password) {
    ElMessage.warning('请输入新密码')
    return
  }
  if (pwdForm.value.new_password.length < 4) {
    ElMessage.warning('密码至少4位')
    return
  }
  if (pwdForm.value.new_password !== pwdForm.value.confirm_password) {
    ElMessage.warning('两次密码不一致')
    return
  }
  if (!isFirstLogin.value && !pwdForm.value.old_password) {
    ElMessage.warning('请输入当前密码')
    return
  }
  pwdLoading.value = true
  try {
    await api.post('/auth/change-password', {
      old_password: pwdForm.value.old_password,
      new_password: pwdForm.value.new_password
    })
    ElMessage.success('密码修改成功')
    const u = JSON.parse(localStorage.getItem('user') || '{}')
    u.need_change_pw = false
    localStorage.setItem('user', JSON.stringify(u))
    user.value.need_change_pw = false
    pwdVisible.value = false
    profileVisible.value = false
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '修改失败')
  } finally {
    pwdLoading.value = false
  }
}

async function loadSettings() {
  try {
    const { data } = await api.get('/settings/family-surname')
    familyName.value = data.family_name || data.family_surname || ''
    document.title = familyName.value || '族谱系统'
  } catch (e) { /* ignore */ }
  try {
    const { data } = await api.get('/settings/version')
    version.value = data.version || __BUILD_VERSION__
  } catch (e) { /* ignore */ }
}

// 收支公示：菜单可见性 + 待审核数
async function loadAccounts() {
  if (!localStorage.getItem('token')) {
    accountsVisible.value = false
    accountsPending.value = 0
    return
  }
  try {
    const { data } = await api.get('/accounts/config')
    accountsVisible.value = !!data.enabled || !!data.can_review || !!data.can_manage_list
    if (data.can_review) {
      try {
        const r = await api.get('/accounts/pending')
        accountsPending.value = (r.data || []).length
      } catch (e) { accountsPending.value = 0 }
    } else {
      accountsPending.value = 0
    }
  } catch (e) {
    accountsVisible.value = false
  }
}

// 家族知识库（传承服务器）在线探测
async function loadKbStatus() {
  if (!localStorage.getItem('token')) {
    kbOnline.value = false
    return
  }
  try {
    const { data } = await api.get('/kb/status')
    kbOnline.value = !!data.enabled && !!data.online
  } catch (e) {
    kbOnline.value = false
  }
}


onMounted(() => {
  updateLoginState()
  openGroupIfNeeded(router.currentRoute.value.path)
  loadSettings()
  loadAccounts()
  loadKbStatus()
  loadBranches()
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

// 路由变化时重新加载家族名和登录状态
watch(() => router.currentRoute.value.path, (p) => {
  updateLoginState()
  openGroupIfNeeded(p)
  if (localStorage.getItem('token')) {
    loadSettings()
    loadAccounts()
    loadKbStatus()
    loadBranches()
  } else {
    branchCtx.loaded = false
  }
})
onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
.layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--mibai);
}

/* ===== 顶部导航栏 ===== */
.topbar {
  background: #3a3226;
  color: #fff;
  flex-shrink: 0;
  z-index: 100;
  border: none;
  outline: none;
  box-shadow: none;
}
.topbar-inner {
  display: flex;
  align-items: center;
  padding: 0 20px;
  height: 52px;
}
.center-area {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0 auto;
  min-width: 0;
}
.logo {
  font-size: 20px;
  letter-spacing: 3px;
  color: #fff;
  cursor: pointer;
  white-space: nowrap;
  user-select: none;
}
.logo:hover { opacity: 0.85; }
.branch-picker { display: flex; align-items: center; gap: 6px; }
.branch-select { width: 200px; }
.branch-select :deep(.el-select__caret) { font-size: 22px; }
.branch-select :deep(.el-select__caret svg) { width: 22px; height: 22px; }
.branch-inc :deep(.el-checkbox__label) { color: #d9cbb6; }
.branch-inc :deep(.el-checkbox__inner) { background: transparent; }
@media (max-width: 767px) {
  .center-area { gap: 6px; }
  .logo { font-size: 16px; letter-spacing: 1px; }
  .branch-select { width: 132px; }
}
.auth-area {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  min-width: 52px;
  justify-content: flex-end;
}
.user-avatar {
  cursor: pointer;
  background: #8c5a3c;
  color: #fff;
  font-size: 14px;
  flex-shrink: 0;
}
.login-btn {
  color: #fff !important;
  border: 1px solid rgba(255,255,255,0.4) !important;
  background: transparent !important;
  outline: none !important;
  box-shadow: none !important;
}
.login-btn:hover {
  background: rgba(255,255,255,0.1) !important;
}
.logout-btn {
  color: #fff !important;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
  background: transparent !important;
}
.logout-btn:hover {
  background: rgba(255,255,255,0.1) !important;
}
.menu-toggle-placeholder {
  display: inline-block;
  width: 38px;
  flex-shrink: 0;
}
.version {
  color: #5a4a3a;
  font-size: 11px;
  white-space: nowrap;
  text-align: center;
}
.menu-toggle {
  color: #fff !important;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
  background: transparent !important;
  flex-shrink: 0;
}

/* ===== 下方内容区 ===== */
.body-area {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* ===== 左侧菜单 ===== */
.sidebar {
  width: 200px;
  flex-shrink: 0;
  background: #3a3226;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}
.sidebar-footer {
  margin-top: auto;
  text-align: center;
  padding: 12px;
}

/* ===== 主内容区 ===== */
.main {
  flex: 1;
  overflow: auto;
  padding: 20px;
  background: var(--mibai);
}
.kb-main { padding: 0; overflow: hidden; }
.mobile-main { padding: 12px; }

/* ===== 移动端抽屉 ===== */
.drawer-nav { background: #3a3226; height: 100%; display: flex; flex-direction: column; }
.drawer-logo { color: #fff; font-size: 20px; text-align: center; padding: 20px 0; letter-spacing: 4px; }
.drawer-footer { margin-top: auto; padding: 12px; text-align: center; }

/* ===== 我的资料卡 ===== */
.profile-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 8px 0 16px;
}
.profile-avatar { background: #8c5a3c; color: #fff; font-size: 22px; }
.profile-name { font-size: 18px; font-weight: 600; color: #3a3226; }
.profile-desc { margin-bottom: 12px; }
.profile-actions { text-align: center; }
.guide-scroll { max-height: 62vh; overflow-y: auto; padding-right: 8px; }

/* ===== 免费版导出水印层（屏幕隐藏，打印/导出 PDF 显示） ===== */
.global-wm {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 99999;
  display: none;
  background-repeat: repeat;
  background-image: url("data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyNDAiIGhlaWdodD0iMTYwIj48dGV4dCB4PSIxMjAiIHk9IjgwIiBmb250LXNpemU9IjI0IiBmaWxsPSIjMDAwIiB0ZXh0LWFuY2hvcj0ibWlkZGxlIiB0cmFuc2Zvcm09InJvdGF0ZSgtMjggMTIwIDgwKSI+6K+V55So54mIwrfml4/osLHns7vnu588L3RleHQ+PC9zdmc+");
}
@media print {
  .global-wm { display: block; }
  .global-wm.wm-1 { opacity: 0.45; background-size: 400px 270px; }
  .global-wm.wm-2 { opacity: 0.75; background-size: 240px 160px; }
  .global-wm.wm-3 { opacity: 1; background-size: 150px 105px; }
}
</style>

<!-- 全局样式：去掉 el-drawer 白边 -->
<style>
.el-drawer.ltr { box-shadow: none !important; }
.el-drawer__header { margin-bottom: 0 !important; }
.el-drawer__body { padding: 0 !important; }
/* 房支下拉面板：加大树节点展开箭头 */
.branch-select-popper .el-tree-node__expand-icon { font-size: 20px; }
.branch-select-popper .el-tree-node__expand-icon svg { width: 20px; height: 20px; }
</style>
