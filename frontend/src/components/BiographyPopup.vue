<template>
  <el-dialog v-model="visible" :title="title" width="720px" top="7vh" append-to-body class="bio-popup">
    <div v-loading="loading" class="bio-popup-body">
      <el-empty v-if="!loading && !items.length" description="暂无传记（未公开的传记需管理员查看）" />
      <div v-for="b in items" :key="b.id" class="bi-item">
        <div class="bi-board">
          <div class="bi-bin">
            <!-- 框一：人物资料 -->
            <div class="bi-card">
              <div class="bi-head">
                <div class="bi-portrait">
                  <img v-if="b.portrait" :src="b.portrait" alt="" />
                  <div v-else class="ph">{{ (b.name || '?').charAt(0) }}</div>
                </div>
                <div class="bi-meta">
                  <div class="bi-name">
                    {{ b.name }}
                    <span v-if="b.zi" class="sub">字{{ b.zi }}</span>
                    <span v-if="b.hao" class="sub">号{{ b.hao }}</span>
                    <el-tag v-if="!b.public" size="small" type="info" effect="plain">未公开</el-tag>
                  </div>
                  <div v-if="lifeLine(b)" class="bi-line">{{ lifeLine(b) }}</div>
                  <div v-if="b.native_place" class="bi-line">籍贯：{{ b.native_place }}</div>
                  <div v-if="b.current_place" class="bi-line">现居：{{ b.current_place }}</div>
                  <div v-if="b.title || b.occupation" class="bi-line">身份：{{ [b.title, b.occupation].filter(Boolean).join(' · ') }}</div>
                  <div v-if="b.honors" class="bi-line">荣誉：{{ b.honors }}</div>
                  <div v-if="b.summary" class="bi-summary">{{ b.summary }}</div>
                  <div v-if="b.reason" class="bi-reason">入传缘由：{{ b.reason }}</div>
                </div>
              </div>
            </div>
            <!-- 框二：传记正文 -->
            <div class="bi-text">
              <div class="bi-text-title">傳</div>
              <div class="bi-text-body">{{ b.content || '（正文待补充）' }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
      <el-button type="primary" @click="goList">在「名人传记」中查看</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import api from '../api'
import { formatBirth, formatDeath } from '../utils/dateFormat'

const router = useRouter()
const visible = ref(false)
const loading = ref(false)
const items = ref([])
const title = ref('传记')
const linked = ref({}) // {member, spouse}

function strip(s) {
  return String(s || '').replace(/^生於/, '生').replace(/^卒於/, '殁').replace(/^生/, '生')
}
function lifeLine(b) {
  const bd = formatBirth({ birth_era: b.birth_era, birth_lunar: b.birth_lunar, birth_gregorian: b.birth_gregorian })
  const dd = formatDeath({ birth_era: b.death_era, death_lunar: b.death_lunar, death_gregorian: b.death_gregorian })
  const parts = []
  if (bd && !/未詳/.test(bd)) parts.push(strip(bd))
  if (dd && !/未詳/.test(dd)) parts.push(strip(dd))
  return parts.join('　')
}

async function open(o = {}) {
  linked.value = { member: o.member_id || '', spouse: o.spouse_id || '' }
  title.value = (o.name ? o.name + ' 的' : '') + '名人传记'
  items.value = []
  visible.value = true
  loading.value = true
  try {
    const qs = o.member_id ? `?member_id=${o.member_id}` : (o.spouse_id ? `?spouse_id=${o.spouse_id}` : '')
    const { data } = await api.get('/biographies' + qs)
    items.value = Array.isArray(data) ? data : (data.items || [])
  } catch (e) {
    items.value = []
  } finally {
    loading.value = false
  }
}

function goList() {
  const q = { tab: 'bios' }
  if (linked.value.member) q.member = linked.value.member
  if (linked.value.spouse) q.spouse = linked.value.spouse
  visible.value = false
  router.push({ path: '/culture', query: q })
}

defineExpose({ open })
</script>

<style scoped>
/* 框与底色仿垂珠体谱页：米色底 + 外框 1px + 内框 3px，人物资料 / 传记各自成框 */
.bio-popup-body { max-height: 66vh; overflow: auto; background: #d8d2c2; padding: 14px; border-radius: 4px; }
.bi-item { background: #fffdf6; padding: 8px; border: 1px solid #c8b896; margin-bottom: 14px; }
.bi-item:last-child { margin-bottom: 0; }
.bi-board { border: 1px solid #1a1a1a; padding: 5px; }
.bi-bin { border: 3px solid #1a1a1a; padding: 8px; display: flex; flex-direction: column; gap: 8px; }
.bi-card { border: 1px solid #1a1a1a; padding: 10px 12px; }
.bi-text { border: 1px solid #1a1a1a; padding: 10px 14px 14px; }
.bi-text-title { text-align: center; font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif; font-size: 20px;
                 font-weight: 700; letter-spacing: 8px; padding-bottom: 6px; border-bottom: 1px solid #1a1a1a; margin-bottom: 8px; }
.bi-text-body { font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif; font-size: 15px; line-height: 1.95;
                color: #1f1c16; white-space: pre-wrap; }
.bi-head { display: flex; gap: 14px; }
.bi-portrait { flex: 0 0 92px; }
.bi-portrait img, .bi-portrait .ph {
  width: 92px; height: 116px; object-fit: cover; border: 1px solid #1a1a1a; border-radius: 2px;
  background: #f2ece0; color: #a3927c; font-size: 30px; display: flex; align-items: center; justify-content: center;
}
.bi-meta { flex: 1; min-width: 0; }
.bi-name { font-size: 17px; font-weight: 600; color: #1f1c16; font-family: "Songti SC", "SimSun", serif; }
.bi-name .sub { font-size: 13px; font-weight: 400; color: #6b5c48; margin-left: 8px; }
.bi-name .el-tag { margin-left: 8px; }
.bi-line { font-size: 13px; color: #4a4137; margin-top: 4px; }
.bi-summary { font-size: 13px; color: #6b5c48; margin-top: 6px; }
.bi-reason { margin-top: 8px; font-size: 12px; color: #8b7c66; }
</style>
