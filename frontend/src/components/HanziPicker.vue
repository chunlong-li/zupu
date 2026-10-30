<template>
  <el-dialog v-model="visible" title="生僻字查字（按部首/笔画）" width="760px" top="6vh" append-to-body>
    <div class="hz-picker">
      <div class="hz-toolbar">
        <span class="hz-label">部首</span>
        <el-select v-model="radical" filterable placeholder="选择部首" style="width: 200px">
          <el-option v-for="(r, i) in radicals" :key="i + 1" :label="r + '（' + (i + 1) + '画部）'" :value="i + 1" />
        </el-select>
        <span class="hz-label">笔画</span>
        <el-input-number v-model="strokeFilter" :min="1" :max="80" :controls="false" placeholder="全部" style="width: 110px" />
        <el-button size="small" @click="strokeFilter = null">清除笔画</el-button>
        <span class="hz-count">共 {{ filteredChars.length }} 字</span>
      </div>
      <div class="hz-chars" v-loading="loading">
        <template v-if="radical">
          <span
            v-for="c in filteredChars"
            :key="c.ch"
            class="hz-char"
            :title="'笔画 ' + c.stroke + (c.pinyin ? ' · ' + c.pinyin : '')"
            @click="pick(c.ch)"
          >{{ c.ch }}</span>
        </template>
        <div v-else class="hz-empty">请先选择一个部首</div>
        <div v-if="radical && filteredChars.length === 0 && !loading" class="hz-empty">该部首下没有匹配的字</div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false }
})
const emit = defineEmits(['update:modelValue', 'pick'])

const visible = computed({
  get: () => props.modelValue,
  set: v => emit('update:modelValue', v)
})

const radicals = ref([])      // 214 部首汉字
const byRadical = ref({})     // 部首编号 → [{ch, stroke, pinyin}]（按笔画排序）
const radical = ref(null)     // 选中的部首编号
const strokeFilter = ref(null)
const loading = ref(false)
let loaded = false

async function load() {
  if (loaded) return
  loading.value = true
  try {
    const res = await fetch('/hanzi-data.json')
    const data = await res.json()
    radicals.value = data.radicals || []
    // 预建索引：部首编号 → 字列表
    const idx = {}
    for (const [ch, arr] of Object.entries(data.chars || {})) {
      const stroke = arr[0]
      const rad = arr[1]
      const pinyin = arr[2] || ''
      if (!idx[rad]) idx[rad] = []
      idx[rad].push({ ch, stroke, pinyin })
    }
    for (const k in idx) idx[k].sort((a, b) => a.stroke - b.stroke || a.ch.localeCompare(b.ch, 'zh'))
    byRadical.value = idx
    loaded = true
  } catch (e) {
    console.warn('生僻字数据加载失败', e)
  } finally {
    loading.value = false
  }
}

const filteredChars = computed(() => {
  const list = byRadical.value[radical.value] || []
  if (!strokeFilter.value) return list
  return list.filter(c => c.stroke === strokeFilter.value)
})

function pick(ch) {
  emit('pick', ch)
  visible.value = false
}

// 打开面板时加载数据
import { watch } from 'vue'
watch(() => props.modelValue, v => { if (v) load() }, { immediate: true })
</script>

<style scoped>
.hz-picker { display: flex; flex-direction: column; gap: 12px; }
.hz-toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; }
.hz-label { color: #666; font-size: 13px; }
.hz-count { color: #999; font-size: 12px; margin-left: auto; }
.hz-chars {
  min-height: 200px;
  max-height: 380px;
  overflow-y: auto;
  border: 1px solid #eee;
  border-radius: 6px;
  padding: 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-content: flex-start;
}
.hz-char {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  font-size: 22px;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  cursor: pointer;
  transition: all .12s;
  user-select: none;
}
.hz-char:hover { background: #2563eb; color: #fff; border-color: #2563eb; }
.hz-empty { color: #999; font-size: 13px; width: 100%; text-align: center; padding: 40px 0; }
</style>
