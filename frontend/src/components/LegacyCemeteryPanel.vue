<template>
  <!--
    旧「祠墓」记录（原「谱头文化 → 祠墓」tab 的数据，kind 为空；该 tab 已于 2026-10-25 下线）：
    在祠宇志 / 茔墓志页面顶部提示，可一键「归入」本模块，归入后即可继续完善。
    数据不删、不猜类型：只有点了「归入」才会写 kind。
  -->
  <div class="legacy" v-if="rows.length">
    <el-alert type="warning" :closable="false" show-icon>
      <template #title>
        有 <b>{{ rows.length }}</b> 条旧「祠墓」记录还没归入{{ moduleName }}。
        （旧「谱头文化 → 祠墓」tab 已下线，这些记录仍保留在库里）
        归入后就能在本页补充世次 / 楹联 / 坐标等字段，数据不会丢。
      </template>
    </el-alert>
    <el-table :data="rows" size="small" border class="legacy-table">
      <el-table-column prop="name" label="名称" min-width="120" show-overflow-tooltip />
      <el-table-column prop="location" label="位置" min-width="140" show-overflow-tooltip />
      <el-table-column prop="description" label="描述" min-width="160" show-overflow-tooltip />
      <el-table-column label="照片" width="80" align="center">
        <template #default="{ row }">
          <el-image
            v-if="row.photo"
            :src="thumbUrl(row.photo, THUMB_MINI)"
            :preview-src-list="[fullUrl(row.photo)]"
            preview-teleported
            fit="cover"
            class="mini"
          />
          <span v-else class="none">—</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="130" align="center">
        <template #default="{ row }">
          <el-button v-if="canEdit()" size="small" type="primary" plain :loading="busy === row.id" @click="classify(row)">
            归入{{ moduleName }}
          </el-button>
          <span v-else class="none">只读</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { canEdit } from '../utils/auth'
import { thumbUrl, THUMB_MINI } from '../utils/thumb'

const props = defineProps({
  // 归入哪一边：'shrine' | 'grave'
  kind: { type: String, required: true },
  // 显示名：祠宇志 / 茔墓志
  moduleName: { type: String, required: true }
})
const emit = defineEmits(['classified'])

const rows = ref([])
const busy = ref(0)

function fullUrl(p) {
  const s = String(p || '')
  if (!s) return ''
  return s.startsWith('/') ? s : (s.startsWith('uploads/') ? '/' + s : '/uploads/' + s)
}

async function load() {
  try {
    const { data } = await api.get('/cemeteries', { params: { kind: 'legacy' } })
    rows.value = Array.isArray(data) ? data : []
  } catch (e) {
    rows.value = []
  }
}

async function classify(row) {
  try {
    await ElMessageBox.confirm(
      `把「${row.name || '未命名'}」归入${props.moduleName}？归入后可在本页继续编辑，原名称/位置/描述/照片都会保留。`,
      `归入${props.moduleName}`,
      { type: 'warning', confirmButtonText: '归入' }
    )
  } catch (e) { return }
  busy.value = row.id
  try {
    await api.post(`/cemeteries/${row.id}/classify`, { kind: props.kind })
    ElMessage.success('已归入' + props.moduleName)
    await load()
    emit('classified', row.id)
  } catch (e) {
    ElMessage.error('归入失败：' + (e?.response?.data?.error || e?.message || e))
  } finally {
    busy.value = 0
  }
}

onMounted(load)
// 供父页面在保存/删除后刷新
defineExpose({ load })
</script>

<style scoped>
.legacy { margin-bottom: 14px; }
.legacy-table { margin-top: 8px; background: #fff; }
.mini { width: 36px; height: 36px; border-radius: 4px; }
.none { color: #bbb; }
</style>
