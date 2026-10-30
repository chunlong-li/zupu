<template>
  <!--
    图片列表输入（祠宇志 / 茔墓志共用）
    · 上传走既有 /api/media/upload（内容去重 + 预生成 120/240/400 缩略图，文件落在 data/upload，自动进备份）
    · 列表一律用 /api/thumbnail 缩略图；点开用原图预览（el-image 的 preview-src-list）
  -->
  <div class="img-input">
    <div class="img-grid">
      <div class="img-cell" v-for="(p, i) in list" :key="p + '#' + i">
        <el-image
          :src="thumbUrl(p, THUMB_LIST)"
          :preview-src-list="previewList"
          :initial-index="i"
          fit="cover"
          preview-teleported
          class="img-thumb"
        />
        <span class="img-del" title="移除这张图" @click.stop="remove(i)">×</span>
        <span v-if="i === 0 && coverBadge" class="img-cover">封面</span>
      </div>

      <el-upload
        v-if="canUpload && list.length < limit"
        class="img-add"
        :show-file-list="false"
        :http-request="doUpload"
        accept="image/*"
        :disabled="uploading"
      >
        <div class="img-add-box">
          <template v-if="uploading">上传中…</template>
          <template v-else>
            <span class="plus">＋</span>
            <small>上传图片</small>
          </template>
        </div>
      </el-upload>
    </div>
    <div class="img-tip" v-if="tip">{{ tip }}</div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import { thumbUrl, THUMB_LIST } from '../utils/thumb'

const props = defineProps({
  modelValue: { type: Array, default: () => [] },
  // 多媒体库分类：祠宇 / 茔墓
  category: { type: String, default: '祠宇' },
  limit: { type: Number, default: 20 },
  canUpload: { type: Boolean, default: true },
  coverBadge: { type: Boolean, default: false },
  tip: { type: String, default: '' }
})
const emit = defineEmits(['update:modelValue'])

const uploading = ref(false)
const list = computed(() => (Array.isArray(props.modelValue) ? props.modelValue : []))
// 预览用原图（缩略图看着糊，放大必须原图）
const previewList = computed(() => list.value.map(fullUrl))

function fullUrl(p) {
  const s = String(p || '')
  if (!s) return ''
  if (s.startsWith('http')) return s
  return s.startsWith('/') ? s : (s.startsWith('uploads/') ? '/' + s : '/uploads/' + s)
}

function setList(arr) {
  emit('update:modelValue', arr)
}

function remove(i) {
  const next = [...list.value]
  next.splice(i, 1)
  setList(next)
}

async function doUpload(opt) {
  const file = opt.file
  if (!file) return
  if (!/^image\//.test(file.type || '') && !/\.(jpg|jpeg|png|gif|webp|bmp|svg)$/i.test(file.name)) {
    ElMessage.error('只能上传图片：' + file.name)
    return
  }
  if (file.size > 60 * 1024 * 1024) {
    ElMessage.error('图片过大（上限 60MB）')
    return
  }
  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', file)
    fd.append('category', props.category)
    fd.append('title', file.name)
    fd.append('branch_id', '0') // 总谱素材：族员都能看到（房支隔离不影响祠庙档案）
    const { data } = await api.post('/media/upload', fd)
    const path = (data && data.path) || ''
    if (!path) {
      ElMessage.error('上传失败：服务端未返回图片路径')
      return
    }
    if (!list.value.includes(path)) setList([...list.value, path])
    ElMessage.success('已上传：' + file.name)
  } catch (e) {
    ElMessage.error('上传失败：' + (e?.response?.data?.error || e?.message || e))
  } finally {
    uploading.value = false
  }
}
</script>

<style scoped>
.img-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.img-cell {
  position: relative;
  width: 108px;
  height: 108px;
  border-radius: 6px;
  overflow: hidden;
  border: 1px solid var(--border);
  background: #f2ece1;
}
.img-thumb {
  width: 100%;
  height: 100%;
  display: block;
  cursor: zoom-in;
}
.img-del {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 20px;
  height: 20px;
  line-height: 18px;
  text-align: center;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.55);
  color: #fff;
  font-size: 14px;
  cursor: pointer;
  user-select: none;
}
.img-del:hover { background: #d9534f; }
.img-cover {
  position: absolute;
  left: 0;
  bottom: 0;
  padding: 1px 6px;
  font-size: 11px;
  color: #fff;
  background: rgba(140, 90, 60, 0.85);
  border-top-right-radius: 6px;
}
.img-add {
  width: 108px;
  height: 108px;
}
.img-add-box {
  width: 108px;
  height: 108px;
  border: 1px dashed var(--border);
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  color: #8c5a3c;
  background: #fbf8f3;
  cursor: pointer;
  font-size: 12px;
}
.img-add-box:hover { border-color: #8c5a3c; background: #f6efe4; }
.img-add-box .plus { font-size: 22px; line-height: 1; }
.img-tip { margin-top: 6px; font-size: 12px; color: #9a8a74; }

/* 手机端：一行 3 张（gap 更小），删除按钮放大到 28px 可点；上传格子同样缩小但保持可点 */
@media (max-width: 767px) {
  .img-grid { gap: 8px; }
  .img-cell { width: calc((100% - 16px) / 3); height: auto; aspect-ratio: 1 / 1; }
  .img-del { width: 28px; height: 28px; line-height: 26px; font-size: 16px; top: 0; right: 0; }
  .img-add { width: calc((100% - 16px) / 3); height: auto; aspect-ratio: 1 / 1; }
  .img-add-box { width: 100%; height: 100%; font-size: 13px; }
  .img-cover { font-size: 10px; padding: 1px 4px; }
}
</style>
