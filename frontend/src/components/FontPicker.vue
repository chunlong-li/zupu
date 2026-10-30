<template>
  <!-- 树图共用字体选择：内置系统字体栈 + 服务器上传字体；上传/删除需理事长及以上 -->
  <el-select :model-value="treeFont.key" size="small" style="width: 170px" :loading="fontLoading"
             @change="onPick">
    <el-option v-for="f in fontOptions" :key="f.key" :label="f.label" :value="f.key" />
  </el-select>
  <el-button size="small" :loading="fontUploading" @click="pickFile">⬆ 上传字体</el-button>
  <el-button v-if="treeFontIsCustom" size="small" type="danger" plain @click="removeFont">🗑 删除字体</el-button>
  <input ref="fileEl" type="file" accept=".ttf,.otf,.woff,.woff2,.ttc" style="display: none" @change="onFile" />
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useTreeFonts, canUploadFont, isNonTtfFont, fontExt, deleteTreeFont, selectServerFontByName } from '../utils/treeFonts'

const { treeFont, treeFontIsCustom, fontOptions, fontUploading, fontLoading, selectTreeFont, uploadTreeFont, loadTreeFonts } = useTreeFonts()
const fileEl = ref(null)

onMounted(() => { loadTreeFonts() })

// woff2/otf/ttc 能显示、但没法子集内嵌 → 选中时就说清楚，别等导出才发现
function warnNonTtf(f) {
  if (!isNonTtfFont(f)) return
  ElMessage.warning(`「${f.label}」是 ${fontExt(f.name || f.url)} 格式：可用于页面显示；但没有 TTF 版本时无法子集内嵌进导出文件（导出的 PNG/PDF/SVG 换机器会回到系统字体），如需内嵌请再上传一份 TTF`)
}

function onPick(key) {
  const f = selectTreeFont(key)
  warnNonTtf(f)
}

function pickFile() {
  if (!canUploadFont()) { ElMessage.error('权限不足：仅理事长及以上可上传字体'); return }
  if (fileEl.value) fileEl.value.click()
}

async function onFile(e) {
  const file = e.target.files && e.target.files[0]
  e.target.value = ''
  if (!file) return
  fontUploading.value = true
  try {
    const { item, recovered } = await uploadTreeFont(file)
    if (recovered) ElMessage.success(`上传超时，但已从服务器确认字体已保存并套用：${item.label}`)
    else ElMessage.success(`字体已上传并套用：${item.label}`)
    warnNonTtf(item)
  } catch (err) {
    if (err && err.kind === 'duplicate') {
      // 服务端拒绝（内容重复 / 同名不同内容）：原文弹出，并在「内容重复」时直接选中已有的那个
      ElMessage.warning(err.message)
      if (err.existing) {
        const hit = await selectServerFontByName(err.existing)
        if (hit) ElMessage.success(`已直接为你选中已有的「${hit.label}」`)
      }
    } else if (err && err.kind === 'load') ElMessage.warning(err.message)   // 上传成功但浏览器加载不了
    else ElMessage.error(err.message || '字体上传失败')
  } finally {
    fontUploading.value = false
  }
}

async function removeFont() {
  if (!canUploadFont()) { ElMessage.error('权限不足：仅理事长及以上可删除字体'); return }
  const f = treeFont.value
  if (!f || !f.custom) return
  try {
    const { ElMessageBox } = await import('element-plus')
    await ElMessageBox.confirm(`确定删除上传字体「${f.label}」吗？删除后所有树图都会回到默认字体。`, '删除字体', { type: 'warning' })
  } catch (e) { return }        // 用户取消
  try {
    await deleteTreeFont(f)
    ElMessage.success('已删除字体')
  } catch (err) {
    ElMessage.error('删除失败：' + (err?.response?.data?.error || err?.message || err))
  }
}
</script>
