<template>
  <div class="data-check">
    <div class="page-head">
      <div>
        <h2 class="page-title">数据体检报告</h2>
        <div class="page-sub">
          对全库族谱数据做一致性检查：父子年龄、生卒日期、重复人物、孤立点、配偶关系、世代连续性。
          点击任意问题可直接跳到该成员修正。
        </div>
      </div>
      <div class="head-actions">
        <span v-if="meta.generated_at" class="check-time">检查时间：{{ meta.generated_at }}</span>
        <el-button :loading="loading" type="primary" plain @click="load">重新体检</el-button>
      </div>
    </div>

    <el-skeleton v-if="loading && !loaded" :rows="6" animated />

    <template v-else>
      <div class="overview">
        <div class="overview-main">
          <div class="overview-num">{{ meta.total_issues }}</div>
          <div class="overview-label">问题总数</div>
        </div>
        <div class="overview-side">
          <div><b>{{ meta.total_members }}</b> 位成员</div>
          <div><b>{{ categoriesWithIssue }}</b> / {{ categories.length }} 类问题有命中</div>
          <div class="overview-tip">检查范围为全库，不受顶部房支筛选影响</div>
        </div>
      </div>

      <div class="cards">
        <button
          v-for="cat in categories"
          :key="cat.key"
          class="check-card"
          :class="{ active: activeKey === cat.key, zero: cat.count === 0 }"
          @click="toggle(cat.key)"
        >
          <div class="card-top">
            <span class="card-icon">{{ iconOf(cat.key) }}</span>
            <span class="card-count" :class="{ danger: cat.count > 0 }">{{ cat.count }}</span>
          </div>
          <div class="card-label">{{ cat.label }}</div>
          <div class="card-desc">{{ cat.description }}</div>
          <div class="card-bar"><i :style="{ width: barWidth(cat.count) }" /></div>
        </button>
      </div>

      <el-alert
        v-if="activeKey"
        :title="`已筛选：${activeLabel}（再次点击卡片可取消筛选）`"
        type="info"
        :closable="false"
        show-icon
        class="filter-tip"
      />

      <div v-for="cat in shownCategories" :key="cat.key" class="cat-block">
        <div class="cat-head">
          <span class="cat-icon">{{ iconOf(cat.key) }}</span>
          <span class="cat-title">{{ cat.label }}</span>
          <el-tag :type="cat.count > 0 ? 'danger' : 'success'" size="small" effect="plain">
            {{ cat.count > 0 ? `${cat.count} 条` : '通过' }}
          </el-tag>
          <span class="cat-desc">{{ cat.description }}</span>
        </div>

        <div v-if="cat.count === 0" class="cat-ok">✅ 未发现此类问题</div>

        <el-table v-else :data="cat.items" border size="small" class="cat-table">
          <el-table-column label="成员" width="150">
            <template #default="{ row }">
              <el-button link type="primary" class="member-link" @click="goMember(row.member_id)">
                {{ row.name || '（无姓名）' }}
              </el-button>
              <span class="member-id">ID:{{ row.member_id }}</span>
            </template>
          </el-table-column>
          <el-table-column label="世代" width="82">
            <template #default="{ row }">
              {{ row.generation > 0 ? `第${row.generation}世` : '—' }}
            </template>
          </el-table-column>
          <el-table-column label="房支" width="100">
            <template #default="{ row }">{{ row.branch || '未分房' }}</template>
          </el-table-column>
          <el-table-column prop="detail" label="问题描述" min-width="300" />
          <el-table-column label="关联人物" min-width="150">
            <template #default="{ row }">
              <template v-if="row.related && row.related.length">
                <el-tag
                  v-for="rel in row.related"
                  :key="rel.member_id"
                  size="small"
                  class="rel-tag"
                  @click="goMember(rel.member_id)"
                >
                  {{ rel.name }}<span v-if="rel.note">（{{ rel.note }}）</span>
                </el-tag>
              </template>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="90" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" plain @click="goMember(row.member_id)">去修正</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div v-if="cat.truncated" class="truncated-tip">
          仅显示前 {{ cat.items.length }} 条（共 {{ cat.count }} 条），建议先修正高频问题后重新体检。
        </div>
      </div>

      <el-empty v-if="!loading && meta.total_issues === 0" description="族谱数据很干净，没有发现问题 🎉" />
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api'

const router = useRouter()
const loading = ref(false)
const loaded = ref(false)
const categories = ref([])
const meta = ref({ generated_at: '', total_members: 0, total_issues: 0 })
const activeKey = ref('')

const categoryLabels = {
  parent_child_age: '父子年龄倒挂',
  birth_death: '生卒矛盾',
  duplicate: '重复人物',
  missing_required: '缺失必填',
  spouse_abnormal: '配偶关系异常',
  generation_gap: '世代断档'
}

// 用汉字做图标：不依赖 emoji 字体（部分服务器/无头浏览器无 emoji 字体时会显示方块）
function iconOf(key) {
  return {
    parent_child_age: '龄',
    birth_death: '卒',
    duplicate: '重',
    missing_required: '缺',
    spouse_abnormal: '配',
    generation_gap: '世'
  }[key] || '查'
}

const categoriesWithIssue = computed(() => categories.value.filter(c => c.count > 0).length)

const shownCategories = computed(() => {
  if (!activeKey.value) return categories.value
  return categories.value.filter(c => c.key === activeKey.value)
})

const activeLabel = computed(() => categoryLabels[activeKey.value] || '')

const maxCount = computed(() => Math.max(1, ...categories.value.map(c => c.count)))

function barWidth(n) {
  return Math.max(n > 0 ? 6 : 0, Math.round((n / maxCount.value) * 100)) + '%'
}

function toggle(key) {
  activeKey.value = activeKey.value === key ? '' : key
}

function goMember(id) {
  if (id > 0) router.push(`/member/${id}`)
}

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/members/check', { timeout: 60000 })
    categories.value = data.categories || []
    meta.value = {
      generated_at: data.generated_at || '',
      total_members: data.total_members || 0,
      total_issues: data.total_issues || 0
    }
    loaded.value = true
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '体检失败'
    ElMessage.error(msg)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.data-check { max-width: 1200px; }
.page-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 12px; flex-wrap: wrap; margin-bottom: 14px; }
.page-title { margin: 0 0 6px; font-size: 20px; color: #3a3226; }
.page-sub { color: #7a6c5b; font-size: 13px; line-height: 1.7; max-width: 760px; }
.head-actions { display: flex; align-items: center; gap: 12px; }
.check-time { color: #909399; font-size: 12px; }

.overview {
  display: flex; align-items: center; gap: 24px;
  background: linear-gradient(135deg, #f7f1e7, #efe6d8);
  border: 1px solid #e2d5c2; border-radius: 12px;
  padding: 16px 22px; margin-bottom: 14px;
}
.overview-main { text-align: center; min-width: 120px; }
.overview-num { font-size: 40px; font-weight: 700; color: #8c5a3c; line-height: 1.1; }
.overview-label { font-size: 13px; color: #7a6c5b; margin-top: 4px; }
.overview-side { font-size: 13px; color: #5b4e3f; line-height: 1.9; }
.overview-side b { color: #3a3226; font-size: 15px; }
.overview-tip { color: #999; font-size: 12px; }

.cards { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 10px; margin-bottom: 14px; }
.check-card {
  text-align: left; cursor: pointer; font: inherit; color: inherit;
  background: #fff; border: 1px solid #e8dfd2; border-radius: 10px;
  padding: 10px 12px 8px; transition: all .15s; box-shadow: 0 1px 2px rgba(90, 74, 58, .04);
}
.check-card:hover { border-color: #c9b294; transform: translateY(-1px); box-shadow: 0 4px 12px rgba(90, 74, 58, .1); }
.check-card.active { border-color: #8c5a3c; box-shadow: 0 0 0 2px rgba(140, 90, 60, .15); }
.check-card.zero { opacity: .62; }
.card-top { display: flex; justify-content: space-between; align-items: center; }
.card-icon { font-size: 13px; width: 24px; height: 24px; line-height: 24px; text-align: center; border-radius: 50%; background: #f1e8db; color: #8c5a3c; font-weight: 700; }
.card-count { font-size: 22px; font-weight: 700; color: #9e9e9e; }
.card-count.danger { color: #c62828; }
.card-label { font-size: 14px; font-weight: 600; color: #3a3226; margin-top: 4px; }
.card-desc { font-size: 11px; color: #8a7b68; line-height: 1.5; margin-top: 3px; min-height: 33px; }
.card-bar { height: 4px; background: #f1ece4; border-radius: 2px; margin-top: 6px; overflow: hidden; }
.card-bar i { display: block; height: 100%; background: linear-gradient(90deg, #d8b48c, #c62828); border-radius: 2px; }

.filter-tip { margin-bottom: 12px; }
.cat-block { background: #fff; border: 1px solid #e8dfd2; border-radius: 10px; padding: 12px 14px; margin-bottom: 12px; }
.cat-head { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 10px; }
.cat-icon { font-size: 12px; width: 22px; height: 22px; line-height: 22px; text-align: center; border-radius: 50%; background: #f1e8db; color: #8c5a3c; font-weight: 700; }
.cat-title { font-size: 16px; font-weight: 600; color: #3a3226; }
.cat-desc { font-size: 12px; color: #909399; }
.cat-ok { color: #2e7d32; font-size: 13px; padding: 6px 2px 2px; }
.cat-table { width: 100%; }
.member-link { font-weight: 600; padding: 0; }
.member-id { color: #b0a79a; font-size: 11px; margin-left: 4px; }
.rel-tag { cursor: pointer; margin-right: 4px; margin-bottom: 2px; }
.muted { color: #bbb; }
.truncated-tip { color: #b26a00; font-size: 12px; margin-top: 8px; }

@media (max-width: 1100px) {
  .cards { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (max-width: 767px) {
  .cards { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .overview { flex-direction: column; align-items: flex-start; gap: 8px; }
  .page-head { flex-direction: column; }
}
</style>
