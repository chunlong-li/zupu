<template>
  <div class="culture">
    <el-tabs :model-value="activeTab" type="border-card" @tab-change="onTabChange">
      <!-- 家族大事记 -->
      <el-tab-pane label="家族大事记" name="events">
        <div class="tab-header">
          <span class="serif title">家族大事记</span>
          <el-button v-if="canEdit()" type="primary" size="small" @click="openEventDialog()">新增事件</el-button>
        </div>
        <el-timeline>
          <el-timeline-item v-for="e in events" :key="e.id" :timestamp="e.year" placement="top">
            <el-card>
              <h4>{{ e.title }}</h4>
              <p v-if="e.content">{{ e.content }}</p>
              <div class="item-actions" v-if="canEdit()">
                <el-button size="small" text @click="openEventDialog(e)">编辑</el-button>
                <el-button size="small" text type="danger" @click="deleteEvent(e.id)">删除</el-button>
              </div>
            </el-card>
          </el-timeline-item>
        </el-timeline>
        <el-empty v-if="!events.length" description="暂无大事记" />
      </el-tab-pane>

      <!-- 谱序/凡例/祖训 -->
      <el-tab-pane label="谱序" name="articles">
        <div class="tab-header">
          <el-radio-group v-model="articleCat" size="small">
            <el-radio-button value="谱序">谱序</el-radio-button>
            <el-radio-button value="凡例">凡例</el-radio-button>
            <el-radio-button value="祖训">祖训</el-radio-button>
            <el-radio-button value="族规">族规</el-radio-button>
          </el-radio-group>
          <el-button v-if="canEdit()" type="primary" size="small" @click="openArticleDialog()">新增文章</el-button>
        </div>
        <div class="article-list">
          <div class="article-item" v-for="a in filteredArticles" :key="a.id">
            <h4 class="serif">{{ a.title }}</h4>
            <div class="article-meta" v-if="a.author || a.year">
              {{ a.author ? a.author : '' }}{{ a.year ? ` · ${a.year}` : '' }}
            </div>
            <div class="article-content" v-html="a.content"></div>
            <div class="item-actions" v-if="canEdit()">
              <el-button size="small" text @click="openArticleDialog(a)">编辑</el-button>
              <el-button size="small" text type="danger" @click="deleteArticle(a.id)">删除</el-button>
            </div>
          </div>
          <el-empty v-if="!filteredArticles.length" description="暂无此分类文章" />
        </div>
      </el-tab-pane>

      <!-- 名人传记 -->
      <el-tab-pane label="名人传记" name="bios">
        <BiographyPanel />
      </el-tab-pane>
    </el-tabs>

    <!-- 事件弹窗 -->
    <el-dialog v-model="eventVisible" :title="eventForm.id ? '编辑事件' : '新增事件'" width="620px">
      <el-form :model="eventForm" label-width="60px">
        <el-form-item label="年份"><el-input v-model="eventForm.year" placeholder="如 清光绪二十六年" /></el-form-item>
        <el-form-item label="标题"><el-input v-model="eventForm.title" /></el-form-item>
        <el-form-item label="详情"><el-input type="textarea" v-model="eventForm.content" :rows="4" /></el-form-item>

        <!-- AI 优化（病句排查 + 美化）-->
        <div class="ai-polish-box" v-if="canUseAI()">
          <div class="ai-polish-row">
            <span class="ai-polish-label">✨ AI 优化</span>
            <el-button size="small" type="warning" plain :loading="eventAiPolishing" @click="eventAiPolish">病句排查 + 美化</el-button>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">约束等级</span>
            <el-button v-for="v in levelOptions" :key="'el'+v" size="small" :type="eventConstraint === v ? 'success' : ''" :plain="eventConstraint !== v" @click="eventConstraint = v">{{ v }}%</el-button>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">润色程度</span>
            <el-button v-for="v in levelOptions" :key="'ed'+v" size="small" :type="eventDegree === v ? 'success' : ''" :plain="eventDegree !== v" @click="eventDegree = v">{{ v }}%</el-button>
          </div>
          <template v-if="eventAiResult">
            <el-form-item label="标题"><el-input v-model="eventAiResult.title" placeholder="AI 优化的标题" /></el-form-item>
            <div class="ai-polish-row">
              <span class="ai-polish-label">内容</span>
              <el-button size="small" type="primary" plain @click="useEventAi(true)">使用这个（含标题）</el-button>
            </div>
            <el-input type="textarea" v-model="eventAiResult.content" :rows="6" placeholder="AI 优化后的内容输出在此" />
            <div class="ai-use-row">
              <el-button size="small" @click="useEventAi(false)">使用这个（仅内容，标题用我填的）</el-button>
            </div>
          </template>
          <div v-else class="ai-polish-tip">点「病句排查 + 美化」把标题和详情发给 AI 优化，结果输出在下方，可选择是否采用。</div>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="eventVisible = false">取消</el-button>
        <el-button type="primary" @click="saveEvent">保存</el-button>
      </template>
    </el-dialog>

    <!-- 文章弹窗 -->
    <el-dialog v-model="articleVisible" :title="articleForm.id ? '编辑文章' : '新增文章'" width="700px">
      <el-form :model="articleForm" label-width="60px">
        <el-form-item label="分类"><el-select v-model="articleForm.category"><el-option label="谱序" value="谱序" /><el-option label="凡例" value="凡例" /><el-option label="祖训" value="祖训" /><el-option label="族规" value="族规" /></el-select></el-form-item>
        <el-form-item label="标题"><el-input v-model="articleForm.title" /></el-form-item>
        <el-form-item label="作者"><el-input v-model="articleForm.author" /></el-form-item>
        <el-form-item label="年代"><el-input v-model="articleForm.year" /></el-form-item>
        <el-form-item label="内容"><el-input type="textarea" v-model="articleForm.content" :rows="8" /></el-form-item>

        <!-- AI 优化（病句排查 + 美化）-->
        <div class="ai-polish-box" v-if="canUseAI()">
          <div class="ai-polish-row">
            <span class="ai-polish-label">✨ AI 优化</span>
            <el-button size="small" type="warning" plain :loading="articleAiPolishing" @click="articleAiPolish">病句排查 + 美化</el-button>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">约束等级</span>
            <el-button v-for="v in levelOptions" :key="'l'+v" size="small" :type="articleConstraint === v ? 'success' : ''" :plain="articleConstraint !== v" @click="articleConstraint = v">{{ v }}%</el-button>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">润色程度</span>
            <el-button v-for="v in levelOptions" :key="'d'+v" size="small" :type="articleDegree === v ? 'success' : ''" :plain="articleDegree !== v" @click="articleDegree = v">{{ v }}%</el-button>
          </div>
          <template v-if="articleAiResult">
            <el-form-item label="标题"><el-input v-model="articleAiResult.title" placeholder="AI 优化的标题" /></el-form-item>
            <div class="ai-polish-row">
              <span class="ai-polish-label">内容</span>
              <el-button size="small" type="primary" plain @click="useArticleAi(true)">使用这个（含标题）</el-button>
            </div>
            <el-input type="textarea" v-model="articleAiResult.content" :rows="8" placeholder="AI 优化后的内容输出在此" />
            <div class="ai-use-row">
              <el-button size="small" @click="useArticleAi(false)">使用这个（仅内容，标题用我填的）</el-button>
            </div>
          </template>
          <div v-else class="ai-polish-tip">点「病句排查 + 美化」把标题和内容发给本地 AI 优化，结果输出在下方，可选择是否采用。</div>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="articleVisible = false">取消</el-button>
        <el-button type="primary" @click="saveArticle">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { canEdit, canUseAI } from '../utils/auth'
import BiographyPanel from '../components/BiographyPanel.vue'

const route = useRoute()
const router = useRouter()
// tab 由路由决定（/culture/events|articles|bios）→ 左侧菜单才能正确高亮；
// 页面里的 tab 栏保留（顺手），点 tab 走 router.replace，菜单高亮跟着变。
const TAB_NAMES = ['events', 'articles', 'bios']
const TAB_ROUTES = { events: '/culture/events', articles: '/culture/articles', bios: '/culture/bios' }
const activeTab = computed(() => {
  const t = String(route.params.tab || '')
  return TAB_NAMES.includes(t) ? t : 'events'
})
function onTabChange(name) {
  if (!TAB_NAMES.includes(name)) return
  if (String(route.params.tab || '') === name) return
  // 保留 query（如名人传记的 member/spouse/new），只换路径
  router.replace({ path: TAB_ROUTES[name], query: route.query })
}
const events = ref([])
const articles = ref([])
const articleCat = ref('谱序')

// 事件
const eventVisible = ref(false)
const eventForm = reactive({})
const eventAiPolishing = ref(false)
const eventAiResult = ref(null)
const eventConstraint = ref(50)
const eventDegree = ref(50)

// 文章
const articleVisible = ref(false)
const articleForm = reactive({})
const articleAiPolishing = ref(false)
const articleAiResult = ref(null)
const levelOptions = [0, 25, 50, 75, 100]
const articleConstraint = ref(50) // 约束等级（越低越严）
const articleDegree = ref(50) // 润色程度（越高改动越大）

const filteredArticles = computed(() => articles.value.filter(a => a.category === articleCat.value))

async function load() {
  const [ev, art] = await Promise.all([
    api.get('/events'),
    api.get('/articles')
  ])
  events.value = ev.data
  articles.value = art.data
}

// --- Events ---
function openEventDialog(e = null) {
  eventAiResult.value = null
  if (e) Object.assign(eventForm, { ...e })
  else Object.assign(eventForm, { id: 0, year: '', title: '', content: '' })
  eventVisible.value = true
}
// AI 优化：家族大事记的标题+详情
async function eventAiPolish() {
  if (!eventForm.content || !eventForm.content.trim()) { ElMessage.warning('请先填写详情'); return }
  eventAiPolishing.value = true
  eventAiResult.value = null
  try {
    const { data } = await api.post('/ai/polish-bio', {
      title: eventForm.title || '',
      content: eventForm.content.trim(),
      level: eventConstraint.value,
      degree: eventDegree.value
    }, { timeout: 190000 })
    eventAiResult.value = { title: data.title || eventForm.title || '', content: data.content || eventForm.content }
    if (data.protected) {
      ElMessage.warning('已拦截 AI 对日期/数字的改动，保留原文（小模型纪年换算不可靠，已自动保护）')
    }
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || 'AI 优化失败，请确认修谱助手已启用并安装模型')
  } finally {
    eventAiPolishing.value = false
  }
}
function useEventAi(withTitle) {
  if (!eventAiResult.value) return
  if (withTitle && eventAiResult.value.title) eventForm.title = eventAiResult.value.title
  eventForm.content = eventAiResult.value.content || ''
  ElMessage.success(withTitle ? '已采用 AI 标题与详情' : '已采用 AI 详情（标题保留手动填写）')
}
async function saveEvent() {
  let r
  if (eventForm.id) r = await api.put(`/events/${eventForm.id}`, eventForm)
  else r = await api.post('/events', eventForm)
  eventVisible.value = false
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已保存')
  load()
}
async function deleteEvent(id) {
  await ElMessageBox.confirm('确定删除？', '确认', { type: 'warning' })
  const r = await api.delete(`/events/${id}`)
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已删除')
  load()
}

// --- Articles ---
function openArticleDialog(a = null) {
  articleAiResult.value = null
  if (a) Object.assign(articleForm, { ...a })
  else Object.assign(articleForm, { id: 0, category: articleCat.value, title: '', author: '', year: '', content: '' })
  articleVisible.value = true
}

// AI 优化：谱序/凡例/祖训/族规的标题+内容交本地模型排查病句并美化
async function articleAiPolish() {
  if (!articleForm.content || !articleForm.content.trim()) { ElMessage.warning('请先填写内容'); return }
  articleAiPolishing.value = true
  articleAiResult.value = null
  try {
    const { data } = await api.post('/ai/polish-bio', {
      title: articleForm.title || '',
      content: articleForm.content.trim(),
      level: articleConstraint.value,
      degree: articleDegree.value
    }, { timeout: 190000 })
    articleAiResult.value = { title: data.title || articleForm.title || '', content: data.content || articleForm.content }
    if (data.protected) {
      ElMessage.warning('已拦截 AI 对日期/数字的改动，保留原文（小模型纪年换算不可靠，已自动保护）')
    }
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || 'AI 优化失败，请确认修谱助手已启用并安装模型')
  } finally {
    articleAiPolishing.value = false
  }
}

// 采用 AI 结果：withTitle=true 时连标题一起采用；false 时只采用内容，标题保留用户手填的
function useArticleAi(withTitle) {
  if (!articleAiResult.value) return
  if (withTitle && articleAiResult.value.title) articleForm.title = articleAiResult.value.title
  articleForm.content = articleAiResult.value.content || ''
  ElMessage.success(withTitle ? '已采用 AI 标题与内容' : '已采用 AI 内容（标题保留手动填写）')
}
async function saveArticle() {
  let r
  if (articleForm.id) r = await api.put(`/articles/${articleForm.id}`, articleForm)
  else r = await api.post('/articles', articleForm)
  articleVisible.value = false
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已保存')
  load()
}
async function deleteArticle(id) {
  await ElMessageBox.confirm('确定删除？', '确认', { type: 'warning' })
  const r = await api.delete(`/articles/${id}`)
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已删除')
  load()
}

onMounted(() => {
  // 旧「祠墓」tab 已删除；旧 ?tab=xxx 由路由 redirect 统一处理（见 router/index.js）。
  // 若有人手动进到 /culture（无子路由），补一次跳转，避免空白页。
  if (!TAB_NAMES.includes(String(route.params.tab || ''))) {
    router.replace({ path: '/culture/events', query: route.query })
  }
  load()
})
</script>

<style scoped>
.culture { height: 100%; }
.tab-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 8px;
}
.title { font-size: 18px; }
.article-list { margin-top: 16px; }
.article-item {
  padding: 16px;
  margin-bottom: 12px;
  background: #fff;
  border: 1px solid var(--border);
  border-radius: 8px;
}
.article-meta { font-size: 12px; color: #8c5a3c; margin: 4px 0; }
.article-content { font-size: 14px; line-height: 1.8; margin-top: 8px; white-space: pre-wrap; }
.item-actions { text-align: right; margin-top: 8px; }
.ai-polish-box {
  margin-top: 8px;
  padding: 12px;
  border: 1px dashed #e6a23c;
  border-radius: 8px;
  background: #fdf6ec;
}
.ai-polish-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}
.ai-polish-label { font-weight: 600; color: #8c5a3c; }
.ai-level-row { display: flex; align-items: center; gap: 6px; margin: 6px 0; flex-wrap: wrap; }
.ai-level-label { font-size: 12px; color: #8c5a3c; font-weight: 500; min-width: 56px; }
.ai-use-row { display: flex; justify-content: flex-end; margin-top: 8px; }
.ai-polish-tip { font-size: 12px; color: #8c5a3c; }
</style>