<template>
  <div>
    <div class="toolbar">
      <el-button v-if="canEdit()" type="primary" @click="openAdd">新增派语</el-button>
    </div>

    <el-card v-for="g in words" :key="g.id" class="poem-card">
      <div class="poem-head">
        <span class="serif branch">{{ g.branch || '总谱' }}</span>
        <span class="meta">起始第 {{ g.start_generation }} 世</span>
        <div class="poem-actions" v-if="canEdit()">
          <el-button type="warning" size="small" @click="openEdit(g)">编辑</el-button>
          <el-popconfirm title="确定删除该派语吗？" @confirm="remove(g.id)">
            <template #reference>
              <el-button type="danger" size="small">删除</el-button>
            </template>
          </el-popconfirm>
        </div>
      </div>
      <div class="poem serif">{{ g.poem }}</div>
      <div class="chars">
        <span v-for="(c, i) in parseWords(g.words_array)" :key="i" class="char">
          {{ c }}
          <small>{{ g.start_generation + i }}</small>
        </span>
      </div>
    </el-card>

    <el-dialog v-model="editVisible" title="字辈派语" :width="dialogWidth">
      <el-form label-width="80px">
        <el-form-item label="支系"><el-input v-model="editForm.branch" placeholder="留空为总谱" /></el-form-item>
        <el-form-item label="起始世代"><el-input-number v-model="editForm.start_generation" :min="1" /></el-form-item>
        <el-form-item label="派语诗文"><el-input v-model="editForm.poem" type="textarea" :rows="4" placeholder="如：忠孝传家久，诗书继世长" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import { canEdit } from '../utils/auth'

const words = ref([])
const editVisible = ref(false)
const editForm = reactive({})
const isMobile = ref(false)

function checkMobile() {
  isMobile.value = window.innerWidth < 768
}

const dialogWidth = computed(() => isMobile.value ? '90%' : '560px')

function parseWords(arr) {
  try {
    return typeof arr === 'string' ? JSON.parse(arr) : (arr || [])
  } catch {
    return []
  }
}

async function load() {
  const { data } = await api.get('/generation-words')
  words.value = data
}

function openAdd() {
  Object.keys(editForm).forEach(k => delete editForm[k])
  editForm.start_generation = 1
  editVisible.value = true
}

function openEdit(g) {
  Object.assign(editForm, {
    id: g.id,
    branch: g.branch,
    start_generation: g.start_generation,
    poem: g.poem,
    words_array: g.words_array
  })
  editVisible.value = true
}

async function remove(id) {
  const r = await api.delete(`/generation-words/${id}`)
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已删除')
  load()
}

async function save() {
  // 从诗文自动拆字
  const chars = editForm.poem.replace(/[，。、\s]/g, '').split('')
  editForm.words_array = JSON.stringify(chars)
  let r
  if (editForm.id) {
    r = await api.put(`/generation-words/${editForm.id}`, editForm)
  } else {
    r = await api.post('/generation-words', editForm)
  }
  editVisible.value = false
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已保存')
  load()
}

onMounted(() => {
  load()
  checkMobile()
  window.addEventListener('resize', checkMobile)
})
onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
.toolbar { margin-bottom: 12px; display: flex; flex-wrap: wrap; gap: 8px; }
.poem-card { margin-bottom: 16px; }
.poem-head { display: flex; align-items: center; margin-bottom: 8px; }
.poem-head .branch { flex: 1; }
.poem-head .meta { margin-right: 12px; }
.poem-actions { display: flex; gap: 6px; flex-shrink: 0; }
.branch { font-size: 18px; font-weight: bold; }
.meta { color: #8c5a3c; font-size: 12px; }
.poem { font-size: 20px; letter-spacing: 4px; margin-bottom: 12px; }
.chars { display: flex; flex-wrap: wrap; gap: 8px; }
.char {
  width: 44px; height: 44px;
  border: 1px solid var(--border);
  border-radius: 4px;
  display: flex; flex-direction: column;
  align-items: center; justify-content: center;
  font-size: 18px;
}
.char small { font-size: 10px; color: #8c5a3c; }
</style>
