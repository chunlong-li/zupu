<template>
  <div class="hall-page">
    <!-- 顶部工具栏 -->
    <div class="hall-toolbar">
      <span class="hall-title">🏛️ 祠堂</span>
      <div class="hall-actions">
        <el-button size="small" @click="handleCapture">📷 截图</el-button>
        <el-button size="small" text @click="handleRefresh">刷新</el-button>
      </div>
    </div>

    <!-- 祠堂场景（渲染器挂载点） -->
    <div ref="hallContainer" class="hall-scene">
      <!-- 3D 框内右上角固定按钮 -->
      <div class="hall-overlay-actions">
        <el-button size="small" @click="toggleView">
          {{ viewMode === 'first' ? '🎥 自由视角' : '👁️ 第一人称' }}
        </el-button>
        <el-button size="small" @click="handleSacrifice">🕯️ 上香</el-button>
      </div>
      <div v-if="loading" class="hall-loading">
        <el-icon class="is-loading"><i class="el-icon-loading"></i></el-icon>
        <span>祠堂加载中…</span>
      </div>
      <div v-else-if="!members.length" class="hall-empty">
        <div class="hall-empty-icon">🏛️</div>
        <div class="hall-empty-title">暂无成员数据</div>
        <div class="hall-empty-sub">请先在「成员管理」中录入族谱</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import { buildHallLayout } from '../utils/hallLayout'
import { HallRenderer3D } from '../utils/hallRenderer3D'

const hallContainer = ref(null)
const loading = ref(true)
const members = ref([])
const mounted = ref(false)
const viewMode = ref('first')

const renderer = new HallRenderer3D()

let incenseTimer = null

// 轮询同步香炉计数（多用户实时：别的族员上香，这里计数牌也能跟着变）
async function refreshIncenseCount() {
  try {
    const resp = await api.get('/incense/active')
    const offers = resp.data.offers || []
    const total = 3 + offers.reduce((s, o) => s + (o.count || 3), 0)
    renderer.setIncenseCount(total)
  } catch (e) {
    // 静默失败，不打扰用户
  }
}

onMounted(async () => {
  await loadMembers()
  schedule3DPreload()
  incenseTimer = setInterval(refreshIncenseCount, 3000)
})

async function loadMembers() {
  loading.value = true
  try {
    let data
    try {
      const resp = await api.get('/members')
      data = resp.data
    } catch (e) {
      console.error('[祠堂] API 请求失败:', e)
      ElMessage.error('祠堂数据请求失败：' + (e?.response?.data?.error || e?.message || e))
      return
    }
    members.value = data || []
    const layout = buildHallLayout(members.value)
    if (!mounted.value) {
      renderer.mount(hallContainer.value)
      renderer.onPick(onPickMember)
      renderer.setWatermark(true)
      mounted.value = true
    }
    // 图腾数据：姓氏 + 祖训；上香记录：服务端同步（失败静默，无数据则不显示）
    let houseInfo = { surname: '', hallName: '', totemImage: '', totemStyle: 'wood', totemFontSize: 200, mottoes: [], legends: [], offers: [] }
    try {
      const [fsResp, artResp, incResp] = await Promise.all([
        api.get('/settings/family-surname'),
        api.get('/articles'),
        api.get('/incense/active'),
      ])
      const fs = fsResp.data || {}
      houseInfo.surname = fs.family_surname || fs.family_name || ''
      houseInfo.hallName = fs.tanghao || ''
      houseInfo.totemImage = fs.totem_image || ''
      houseInfo.totemStyle = fs.totem_style || 'wood'
      houseInfo.totemFontSize = Number(fs.totem_font_size) || 200
      const articles = artResp.data || []
      houseInfo.mottoes = articles
        .filter((a) => a.category === '祖训')
        .map((a) => ({ title: (a.title || '').trim(), content: (a.content || '').trim() }))
        .filter((x) => x.title || x.content)
      houseInfo.legends = articles
        .filter((a) => a.category === '凡例')
        .map((a) => ({ title: (a.title || '').trim(), content: (a.content || '').trim() }))
        .filter((x) => x.title || x.content)
      const nowSec = Date.now() / 1000
      houseInfo.offers = (incResp.data.offers || []).map((o) => ({
        count: o.count || 3,
        burn: Math.max(0, Math.min(1, (nowSec - o.offered_at) / 1800)),
      }))
    } catch (e) {
      console.warn('[祠堂] 图腾/上香数据获取失败，跳过：', e)
    }
    renderer.render(layout, houseInfo)
  } catch (e) {
    console.error('[祠堂] 渲染失败:', e)
    ElMessage.error('祠堂渲染失败：' + (e?.message || e))
  } finally {
    loading.value = false
  }
}

// 点击牌位 → 在牌位上方弹出 3D 木牌面板（固定在场景中，可旋转看侧面/背面）
async function onPickMember(memberId, worldPos) {
  if (!memberId) return
  try {
    const { data } = await api.get(`/members/${memberId}/detail`)
    renderer.showMemberPanel(data, worldPos)
  } catch (e) {
    const fallback = members.value.find(x => x.id === memberId)
    if (fallback) {
      renderer.showMemberPanel(fallback, worldPos)
    } else {
      ElMessage.error('加载成员信息失败')
    }
  }
}

async function handleSacrifice() {
  try {
    const resp = await api.post('/incense/offer')
    const data = resp.data || {}
    if (data.ok === false) {
      ElMessage.warning(data.error || '香还未烧完，请稍后再上')
      return
    }
    renderer.addIncense()
    ElMessage.success('🙏 已上香')
  } catch (e) {
    ElMessage.error('上香失败：' + (e?.response?.data?.error || e?.message || e))
  }
}

function handleCapture() {
  try {
    renderer.capture().then(() => {
      ElMessage.success('截图已保存')
    })
  } catch (e) {
    ElMessage.info('截图功能开发中，导出将带水印')
  }
}

function handleRefresh() {
  loadMembers()
}

function toggleView() {
  viewMode.value = viewMode.value === 'first' ? 'free' : 'first'
  renderer.setViewMode(viewMode.value)
}

// 预留：真 3D 上线后，改为动态 import Three.js chunk（不阻塞主流程）
function schedule3DPreload() {
  const preload = () => {
    // 3D 未实现前为空操作；上线后改为：
    // import('./renderers/HallRenderer3D')
  }
  if ('requestIdleCallback' in window) {
    requestIdleCallback(preload, { timeout: 3000 })
  } else {
    setTimeout(preload, 2000)
  }
}

onUnmounted(() => {
  if (incenseTimer) clearInterval(incenseTimer)
  renderer.dispose()
})
</script>

<style scoped>
.hall-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.hall-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #e5e0d5;
}

.hall-title {
  font-size: 18px;
  font-weight: 600;
  color: #5a4632;
}

.hall-scene {
  flex: 1;
  min-height: 0;
  position: relative;
  overflow: hidden;
  background: #171008;
}

.hall-overlay-actions {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 20;
  display: flex;
  gap: 8px;
}

.hall-loading,
.hall-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #d9cbb6;
  flex-direction: column;
  gap: 8px;
}

.hall-empty-icon {
  font-size: 56px;
  line-height: 1;
  opacity: 0.9;
}

.hall-empty-title {
  font-size: 16px;
  color: #d9cbb6;
  letter-spacing: 1px;
}

.hall-empty-sub {
  font-size: 13px;
  color: #8a7a64;
}
</style>
