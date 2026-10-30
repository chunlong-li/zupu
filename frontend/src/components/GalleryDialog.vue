<template>
  <el-dialog :model-value="modelValue" title="图片展示 · 条目 / 选图 / 布局" width="min(880px, 96vw)" class="no-print" append-to-body
             @update:model-value="$emit('update:modelValue', $event)" @close="sync">
    <!-- 条目列表：顺序即书内顺序 -->
    <div class="gal-toolbar">
      <el-button size="small" type="primary" @click="addEntry">＋ 新增条目</el-button>
      <span class="gal-hint">共 {{ list.length }} 个条目（按顺序排入书里；空条目不生成页）</span>
    </div>
    <div class="gal-entries">
      <div class="gal-entry" v-for="(en, ei) in list" :key="en.id" :class="{ on: ei === activeIdx }" @click="activeIdx = ei">
        <span class="ge-idx">{{ ei + 1 }}</span>
        <span class="ge-name" :title="entryTitle(en, ei, list.length)">{{ entryTitle(en, ei, list.length) }}</span>
        <span class="ge-meta">{{ en.images.length }} 张 · {{ layoutLabel(en.layout) }}</span>
        <span class="ge-ops" @click.stop>
          <el-button size="small" text @click="moveEntry(ei, -1)" :disabled="ei === 0">↑</el-button>
          <el-button size="small" text @click="moveEntry(ei, 1)" :disabled="ei === list.length - 1">↓</el-button>
          <el-button size="small" text @click="copyEntry(ei)">复制</el-button>
          <el-button size="small" text type="danger" @click="removeEntry(ei)">删除</el-button>
        </span>
      </div>
      <div v-if="!list.length" class="gal-empty">尚无条目 —— 点「＋ 新增条目」开始；不新增则不生成任何图片页</div>
    </div>

    <template v-if="active">
      <div class="gal-toolbar">
        <span class="lbl">布局</span>
        <el-radio-group v-model="active.layout" size="small" @change="sync">
          <el-radio-button :value="1">单张</el-radio-button>
          <el-radio-button :value="2">2 宫格</el-radio-button>
          <el-radio-button :value="4">4 宫格</el-radio-button>
          <el-radio-button :value="6">6 宫格</el-radio-button>
          <el-radio-button :value="9">9 宫格</el-radio-button>
        </el-radio-group>
        <el-input v-model="active.title" size="small" style="width:180px" placeholder="条目标题（可空）" @input="sync" />
        <el-checkbox v-model="active.showCaption" size="small" @change="sync">显示图说</el-checkbox>
        <el-upload :show-file-list="false" :http-request="(o) => $emit('upload', o)" accept="image/*">
          <el-button size="small">上传新图片</el-button>
        </el-upload>
        <el-button size="small" @click="clearEntry">清空本条目</el-button>
      </div>

      <div class="gal-sec">本条目已选（{{ active.images.length }} 张，按顺序排版；可上移/下移/移除）</div>
      <div class="gal-selected">
        <div class="gal-item" v-for="(g, gi) in active.images" :key="'sel' + g.id + '-' + gi">
          <img :src="thumbUrl(g.path, THUMB_LIST)" alt="" loading="lazy" @error="onGalleryImgError" />
          <div class="gal-cap">{{ g.description || g.memberName || '（无图说）' }}</div>
          <div class="gal-ops">
            <el-button size="small" text @click="moveImage(gi, -1)" :disabled="gi === 0">↑</el-button>
            <el-button size="small" text @click="moveImage(gi, 1)" :disabled="gi === active.images.length - 1">↓</el-button>
            <el-button size="small" text type="danger" @click="removeImage(gi)">移除</el-button>
          </div>
        </div>
        <div v-if="!active.images.length" class="gal-empty">本条目尚未选择图片（不选则不生成任何图片页）</div>
      </div>

      <div class="gal-sec">
        可选图片 · 来源
        <span class="gal-src-hint">{{ ownHint }}</span>
      </div>
      <div class="gal-src" v-if="sourceOptions.length">
        <el-radio-group v-model="srcFilter" size="small">
          <el-radio-button v-for="o in sourceOptions" :key="o.key" :value="o.key">
            {{ o.label }}（{{ o.count }}）
          </el-radio-button>
        </el-radio-group>
        <el-button size="small" text @click="pickAll">加入全部（本来源）</el-button>
      </div>
      <div class="gal-pool">
        <div class="gal-item" v-for="g in shownPool" :key="'pool' + g.sourceKey + '-' + g.id"
             :class="{ on: hasImage(g.id) }" @click="toggleImage(g)">
          <img :src="thumbUrl(g.path, THUMB_MINI)" alt="" loading="lazy" @error="onGalleryImgError" />
          <div class="gal-src-tag" :class="'src-' + g.source">{{ g.sourceLabel }}</div>
          <div class="gal-cap">{{ g.memberName }}<template v-if="g.description"> · {{ g.description }}</template>
            <template v-if="g.alsoIn && g.alsoIn.length"><span class="gal-also">（也在{{ g.alsoIn.join('、') }}）</span></template>
          </div>
        </div>
        <div v-if="!shownPool.length" class="gal-empty">
          {{ poolLoading ? '正在载入可选图片…（已先取多媒体库，成员附件与房支补齐中）'
             : (!permittedPool.length ? '暂无可选素材（无对应房支权限）' : '该来源暂无图片') }}
        </div>
      </div>
    </template>

    <template #footer><el-button @click="$emit('update:modelValue', false)">关闭</el-button></template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { thumbUrl, THUMB_MINI, THUMB_LIST, onGalleryImgError } from '../utils/thumb'
import { sourcePermitted } from '../utils/galleryPool'
import { entryTitle, layoutLabel, newEntryId, cloneEntries } from '../utils/galleryStore'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  entries: { type: Array, default: () => [] },
  pool: { type: Array, default: () => [] },
  poolLoading: { type: Boolean, default: false },
  // 'family'＝总谱三视图（默认只看总谱素材）；'branch'＝某支谱（默认只看本房支素材）
  mode: { type: String, default: 'family' },
  branchId: { type: [Number, String], default: 0 },
})
const emit = defineEmits(['update:modelValue', 'update:entries', 'upload'])

const list = ref([])          // 弹窗内部编辑副本（避免直接改父层对象）
const activeIdx = ref(0)
const active = computed(() => list.value[activeIdx.value] || null)

// ===== 图片池「来源」分组 =====
// 总谱（mode=family）：默认**只显示总谱素材**，另有「含支谱素材」开关（默认关）→ 打开后各房支素材/支谱图片、成员附件、全部都可看；
// 支谱（mode=branch）：默认**只显示本房支素材**，另可开「含总谱素材」（默认关）。
const ownKey = computed(() => (props.mode === 'branch' ? ('branch:' + Number(props.branchId || 0)) : 'family'))
const ownLabel = computed(() => {
  const hit = (permittedPool.value || []).find((g) => (g.sourceKey || '') === ownKey.value)
  return hit ? hit.sourceLabel : (props.mode === 'branch' ? '本房支素材' : '总谱素材')
})
const ownHint = computed(() => (props.mode === 'branch'
  ? '（按你的房支权限列出：本房支素材默认选中；总谱素材/成员附件可切换查看。点缩图即加入本条目，只是「借用」，不改对方数据）'
  : '（按你的房支权限列出：总谱素材默认选中；有权限的房支素材/成员附件可切换查看，不混在一起。点缩图即加入本条目，只是「借用」，不改支谱数据）'))
// 只保留该用户有权使用的来源（无权限整个不出现；支谱口径下不出现「总谱素材」）
const permittedPool = computed(() => {
  const branchOk = sourcePermitted('branch:' + Number(props.branchId || 0))   // 本房支是否在授权范围内
  return (props.pool || []).filter((g) => {
    const k = g.sourceKey || 'family'
    if (!sourcePermitted(k)) return false
    if (props.mode === 'branch') {
      if (k === 'family') return false                                            // 支谱：不显示总谱素材
      if (String(k).indexOf('bsheet:') === 0) return false                        // 支谱：不显示支谱图片来源
      if (k === 'member' && !branchOk) return false                               // 支谱：无本房支权限者连成员附件也不列
    }
    return true
  })
})
const allSources = computed(() => {
  const m = new Map()
  ;(permittedPool.value || []).forEach((g) => {
    const k = g.sourceKey || 'family'
    if (!m.has(k)) m.set(k, { key: k, label: g.sourceLabel || '其他', count: 0, kind: g.source || 'media' })
    m.get(k).count += 1
  })
  return [...m.values()]
})
// 来源选项＝接口返回的素材里出现过的来源（后端已按授权房支过滤）：
// 自家来源（总谱/本房支）排第一并作为默认，其余按「房支素材 → 支谱图片 → 成员附件」后置，另有「全部」可看全部可见来源
const sourceOptions = computed(() => {
  const opts = allSources.value
    .slice()
    .sort((a, b) => {
      const rank = (o) => (o.key === ownKey.value ? 0 : (o.kind === 'member' ? 1 : 2))
      return (rank(a) - rank(b)) || a.label.localeCompare(b.label, 'zh')
    })
  if (opts.length > 1 && !(props.mode === 'branch' && opts.every((o) => o.kind === 'member'))) {
    opts.push({ key: 'all', label: '全部', count: permittedPool.value.length })
  }
  return opts
})
const shownPool = computed(() => {
  const k = srcFilter.value
  if (k === 'all') return permittedPool.value
  return permittedPool.value.filter((g) => (g.sourceKey || 'family') === k)
})
const srcFilter = ref('family')
// 池子变化时保证当前来源仍存在（默认落到自家来源）
watch(() => props.pool, () => {
  const opts = sourceOptions.value
  if (!opts.some((o) => o.key === srcFilter.value)) {
    srcFilter.value = opts.length ? opts[0].key : ownKey.value
  }
}, { immediate: true })
watch(() => props.modelValue, (v) => {
  if (v) srcFilter.value = ownKey.value
})

// 打开时从父层复制一份；关闭时 sync() 回写
watch(() => props.modelValue, (v) => {
  if (!v) return
  list.value = cloneEntries(props.entries)
  if (activeIdx.value >= list.value.length) activeIdx.value = Math.max(0, list.value.length - 1)
})

function sync() {
  emit('update:entries', cloneEntries(list.value))
}
function blankEntry() {
  return { id: newEntryId(), title: '', images: [], layout: 4, showCaption: true }
}
function addEntry() {
  list.value.push(blankEntry())
  activeIdx.value = list.value.length - 1
  sync()
}
function copyEntry(i) {
  const src = list.value[i]
  if (!src) return
  const c = cloneEntries([src])[0]
  c.id = newEntryId()
  c.title = (src.title || '') + '（副本）'
  list.value.splice(i + 1, 0, c)
  activeIdx.value = i + 1
  sync()
}
function removeEntry(i) {
  list.value.splice(i, 1)
  if (activeIdx.value >= list.value.length) activeIdx.value = Math.max(0, list.value.length - 1)
  sync()
}
function moveEntry(i, d) {
  const j = i + d
  if (j < 0 || j >= list.value.length) return
  const [x] = list.value.splice(i, 1)
  list.value.splice(j, 0, x)
  activeIdx.value = j
  sync()
}
// 已选判定：同 id 或**同路径**都算已加入（同一张图从别的来源再点一次不会重复进书）
function imgIndex(g) {
  if (!active.value) return -1
  const arr = active.value.images
  const byId = arr.findIndex((x) => x.id === g.id)
  if (byId >= 0) return byId
  return arr.findIndex((x) => x.path === g.path)
}
function hasImage(id) {
  if (!active.value) return false
  const arr = active.value.images
  const hit = (props.pool || []).find((g) => g.id === id)
  if (arr.some((x) => x.id === id)) return true
  return !!(hit && arr.some((x) => x.path === hit.path))
}
function toggleImage(g) {
  if (!active.value) return
  const arr = active.value.images
  const i = imgIndex(g)
  if (i >= 0) arr.splice(i, 1)
  else arr.push({ ...g })
  sync()
}
function removeImage(gi) {
  if (!active.value) return
  active.value.images.splice(gi, 1)
  sync()
}
function moveImage(gi, d) {
  if (!active.value) return
  const arr = active.value.images
  const j = gi + d
  if (j < 0 || j >= arr.length) return
  const [x] = arr.splice(gi, 1)
  arr.splice(j, 0, x)
  sync()
}
function pickAll() {
  if (!active.value) return
  const arr = active.value.images
  shownPool.value.forEach((g) => { if (imgIndex(g) < 0) arr.push({ ...g }) })
  sync()
}
function clearEntry() {
  if (!active.value) return
  active.value.images = []
  sync()
}
</script>

<style scoped>
.gal-hint { font-size: 12px; color: #8c5a3c; }
.gal-entries { display: flex; flex-direction: column; gap: 6px; max-height: 190px; overflow: auto; margin-bottom: 10px; }
.gal-entry { display: flex; align-items: center; gap: 8px; padding: 4px 8px; border: 1px solid #e2d9c6; border-radius: 6px;
  background: #faf7f0; cursor: pointer; }
.gal-entry.on { border-color: #8c5a3c; background: #f3ecdd; }
.gal-entry .ge-idx { flex: 0 0 22px; text-align: center; font-weight: 700; color: #8c5a3c; }
.gal-entry .ge-name { flex: 1 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
.gal-entry .ge-meta { flex: 0 0 auto; font-size: 12px; color: #6b5c46; }
.gal-entry .ge-ops { flex: 0 0 auto; display: flex; gap: 2px; }
.gal-entry .ge-ops .el-button { margin: 0 !important; padding: 0 4px !important; font-size: 12px; height: 22px; }
.gal-entry .ge-ops .el-button + .el-button { margin-left: 0 !important; }

.gal-toolbar { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; margin-bottom: 10px; }
.gal-toolbar .lbl { font-size: 12px; color: #8c5a3c; }
.gal-sec { font-weight: 700; margin: 10px 0 6px; }
.gal-selected, .gal-pool { display: flex; flex-wrap: wrap; gap: 10px; max-height: 220px; overflow: auto; }
/* 每张图＝图片在上、图说居中、操作按钮在**图片下面**，各自占位、互不覆盖。
   旧写法：卡片宽 104px（内容 92px），三个按钮实宽 32+12+32+12+46 = 134px（Element Plus 的
   .el-button + .el-button 还带 margin-left:12px）→「移除」向右溢出 42px，被右边邻卡的不透明底色盖住：
   实测 13 张时 13 个「移除」的 elementFromPoint 全部命中邻卡，真实点击 0/13 生效。 */
.gal-item { width: 120px; border: 2px solid transparent; border-radius: 6px; padding: 4px; cursor: pointer;
  background: #faf7f0; display: flex; flex-direction: column; align-items: stretch; overflow: hidden; }
.gal-item.on { border-color: #8c5a3c; background: #f3ecdd; }
.gal-item img { width: 100%; height: 72px; object-fit: cover; display: block; flex: 0 0 auto; }
.gal-cap { font-size: 11px; color: #5d5140; margin-top: 3px; word-break: break-all; max-height: 28px; overflow: hidden; }
.gal-ops { display: flex; gap: 4px; justify-content: space-between; margin-top: auto; padding-top: 4px; }
.gal-ops .el-button { flex: 1 1 0; min-width: 0; margin: 0 !important; padding: 0 2px !important; font-size: 11px; height: 22px; }
.gal-ops .el-button + .el-button { margin-left: 0 !important; }
.gal-src-hint { font-weight: 400; font-size: 11px; color: #8c5a3c; }
.gal-src { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 6px; max-height: 64px; overflow: auto; }
.gal-src .el-radio-group { flex-wrap: wrap; }
.gal-src-tag { display: inline-block; align-self: flex-start; margin-top: 3px; padding: 0 5px; border-radius: 8px;
  font-size: 10px; line-height: 15px; color: #fff; background: #8c5a3c; max-width: 100%; overflow: hidden;
  white-space: nowrap; text-overflow: ellipsis; }
.gal-src-tag.src-media { background: #8c5a3c; }
.gal-src-tag.src-member { background: #4d7a52; }
.gal-src-tag.src-branch { background: #3f6d94; }
.gal-also { color: #8c5a3c; }
.gal-empty { color: #8c5a3c; font-size: 12px; padding: 8px; }
/* 源文件已丢失的图片：显示占位，不显示碎图 */
.gal-item.img-missing { background: repeating-linear-gradient(45deg,#f7f3ea,#f7f3ea 8px,#efe8da 8px,#efe8da 16px); }
.gal-item.img-missing img { display: none; }
.gal-item.img-missing::before { content: '图片不可用'; display: block; height: 72px; line-height: 72px; text-align: center; color: #b08840; font-size: 12px; }
</style>
