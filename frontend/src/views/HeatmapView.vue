<template>
  <div class="heatmap">
    <h2 class="serif page-title">家族热力分布</h2>

    <div class="hm-bar">
      <el-select v-model="branch" placeholder="全部房支" clearable style="width: 150px" @change="load">
        <el-option v-for="b in branchOptions" :key="b" :label="b" :value="b" />
      </el-select>
      <span class="hm-lbl">世代</span>
      <el-input-number v-model="genMin" :min="1" :max="99" :controls="false" placeholder="起" style="width: 68px" @change="load" />
      <span class="hm-lbl">–</span>
      <el-input-number v-model="genMax" :min="1" :max="99" :controls="false" placeholder="止" style="width: 68px" @change="load" />
      <el-select v-if="tab === 'geo'" v-model="geoField" style="width: 150px" @change="onFieldChange">
        <el-option label="按籍贯" value="native" />
        <el-option label="按现居地" value="current" />
        <el-option label="按葬地" value="burial" />
        <el-option label="全部（祖籍/现居地/葬地）" value="all" />
      </el-select>
      <el-button size="default" @click="reset">重置</el-button>
      <span class="hm-stat">共 {{ total }} 人</span>
    </div>

    <div class="hm-bar hm-bar2" v-if="tab === 'geo' || tab === 'migration'">
      <span class="hm-lbl">底图</span>
      <el-select v-model="baseMap" style="width: 156px" @change="onBaseMapChange">
        <el-option label="无底图（示意）" value="none" />
        <el-option label="在线底图（免下载）" value="online" />
        <el-option v-for="p in offlinePacks" :key="p.id" :label="'离线底图 · ' + p.name" :value="p.id" />
      </el-select>
      <el-button v-if="baseMap === 'online' && canConfig" link type="primary" @click="openOnlineSettings">⚙️ 在线底图设置</el-button>
      <template v-if="tab === 'geo'">
        <el-checkbox v-model="precise" @change="onPreciseChange">精确热力（需地理编码）</el-checkbox>
        <span class="hm-lbl" v-if="drill.prov">
          <el-button link type="primary" @click="backToChina">← 返回全国</el-button>
          {{ drill.prov }}
        </span>
        <span class="hm-cfg" v-if="precise">{{ preciseStatus }}</span>
        <span class="hm-cfg-err" v-if="precise && pointsHint">⚠️ {{ pointsHint }}</span>      </template>
      <el-button link type="primary" @click="openSettings">⚙️ 地理编码设置</el-button>
    </div>

    <el-tabs v-model="tab" class="hm-tabs" @tab-change="onTabChange">
      <el-tab-pane label="🗺️ 地域分布" name="geo" />
      <el-tab-pane label="🚶 迁徙轨迹" name="migration" />
      <el-tab-pane label="🏘️ 房支人口" name="branch" />
      <el-tab-pane label="📅 时间轴" name="timeline" />
    </el-tabs>

    <div class="hm-hint" v-if="!loading && geoHint">{{ geoHint }}</div>
    <div class="hm-hint" v-for="(line, i) in allHintLines" :key="'ah' + i">{{ line }}</div>
    <div class="hm-hint" v-if="renderFail">图表渲染出错：{{ renderFail }}（请刷新或反馈给管理员）</div>
    <div class="hm-hint" v-if="!loading && tab === 'geo' && precise && !pointsConfigured">
      未配置地理编码服务，暂为省级热力。点右上「⚙️ 地理编码设置」填写高德或百度 Key 后即可显示精确落点。
    </div>

    <div class="hm-hint" v-if="!loading && (tab === 'geo' || tab === 'migration') && baseMap !== 'none' && baseMap !== 'online' && !offlinePacks.length">
      尚未导入离线底图包。到「系统设置 → 🗺️ 离线地图」导入后，这里就能选真底图。
    </div>

    <div class="hm-hint" v-if="!loading && (tab === 'geo' || tab === 'migration') && baseMap === 'online' && onlineNeedTk() && !onlineTkReady()">
      尚未配置在线底图 Key。{{ canConfig ? '点上方「⚙️ 在线底图设置」填写天地图「浏览器端」Key（免费申请，约 1 分钟），或改用「在线底图设置」里不需要 Key 的 OpenStreetMap 等底图源。' : '请联系管理员在「在线底图设置」中选择底图源或填写天地图浏览器端 Key。' }}
    </div>

    <div class="hm-chart-wrap" ref="wrapEl">
      <div ref="chartEl" class="hm-chart" v-show="!leafActive()"></div>
      <div ref="leafEl" class="hm-leaf" v-show="leafActive()"></div>
      <div class="hm-leaf-attr" v-if="leafActive() && baseMap === 'online'">地图数据 {{ onlineAttr() }}</div>
      <div class="hm-leaf-err" v-if="leafActive() && leafTileErr">
        {{ baseMap === 'online' ? ('底图加载失败，可能' + (onlineNeedTk() ? '天地图 Key 额度用尽' : '网络不可达或该底图源暂时不可用') + '，稍后自动恢复；若服务端 Key 额度用尽会自动切换为浏览器直连') : '底图瓦片加载失败，请检查离线包' }}
      </div>
      <div class="hm-leaf-legend" v-if="leafActive() && leafMax > 0 && !(precise && points.length)">
        <span class="hm-lg-lbl">多</span>
        <i class="hm-lg-bar"></i>
        <span class="hm-lg-lbl">少</span>
        <b class="hm-lg-max">{{ leafMax }}</b>
      </div>
      <!-- 「全部」模式图例：颜色 → 类别 + 各自人数（可点选开关） -->
      <div class="hm-cat-legend" v-if="showCatLegend">
        <div class="hm-cl-title">
          <span>图例 · 全部类别</span>
          <span class="hm-cl-note">{{ precise ? '按落点上色' : '省级合计着色' }}</span>
        </div>
        <div class="hm-cl-row" v-for="c in catLegend" :key="c.key"
             :class="{ off: !c.on }" @click="toggleCat(c.key)">
          <i class="hm-cl-dot" :style="{ background: c.color, borderRadius: c.shape }"></i>
          <span class="hm-cl-name">{{ c.label }}</span>
          <b class="hm-cl-num">{{ c.on ? (c.count + ' 人') : '—' }}</b>
        </div>
        <div class="hm-cl-tip">点击一行可开关该类别（再点恢复）</div>
      </div>
      <div v-if="loading" class="hm-loading">加载中…</div>
    </div>

    <el-drawer v-model="drawer" :title="drawerTitle" size="680px">
      <div class="hm-tip">
        非超管账号的地址只显示到市一级（隐私保护）；「在世」指未登记去世日期。
        <template v-if="drawerAll"><br />本表为「全部」模式：每条标明命中的类别；<b>现居地同祖籍（未迁出）</b>表示该成员没填现居地，按祖籍坐标落点。</template>
        <template v-else-if="drawerField === 'current'"><br />现居地为空的成员按祖籍落点（未迁出），已在下表标注。</template>
      </div>
      <el-table :data="members" size="small" style="cursor: pointer" @row-click="goMember" v-loading="drawerLoading">
        <el-table-column prop="name" label="姓名" width="96" />
        <el-table-column prop="generation" label="世" width="46" />
        <el-table-column prop="branch" label="房支" width="88" />
        <el-table-column v-if="drawerField === 'all'" label="类别" width="176">
          <template #default="{ row }">
            <el-tag v-for="c in row.categories || []" :key="c" size="small"
                    :style="{ background: catMeta(c).color, borderColor: catMeta(c).color, color: '#fff', marginRight: '4px' }">
              {{ catMeta(c).label }}
            </el-tag>
            <span v-if="!(row.categories || []).length">—</span>
          </template>
        </el-table-column>
        <el-table-column prop="native_place" label="籍贯" show-overflow-tooltip />
        <el-table-column label="现居地" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ row.current_place || '—' }}</span>
            <el-tag v-if="row.current_from_native" type="warning" size="small" style="margin-left:4px">同祖籍（未迁出）</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="burial_place" label="葬地" show-overflow-tooltip />
        <el-table-column label="在世" width="62">
          <template #default="{ row }">
            <el-tag v-if="row.alive" type="success" size="small">在世</el-tag>
            <span v-else>—</span>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="!members.length && !drawerLoading" class="hm-empty-sm">该地区暂无明细</div>
      <div class="hm-tip">点击某一行可跳转到该成员详情页</div>
    </el-drawer>

    <el-dialog v-model="onlineDialog" title="在线底图设置" width="580px">
      <el-form label-width="96px">
        <el-form-item label="底图源">
          <el-select v-model="onlineForm.source" style="width: 320px">
            <el-option v-for="s in ONLINE_SOURCES" :key="s.id" :label="s.label" :value="s.id" />
          </el-select>
          <div class="hm-cfg-hint">除天地图外均为<b>免费、无需 Key</b>的底图源（高德 / Esri，国内可直连）。</div>
        </el-form-item>
        <el-form-item label="Key 走法" v-if="onlineForm.source === 'tianditu'">
          <el-switch v-model="onlineForm.proxy" active-text="经本系统服务器中转（Key 不下发浏览器）" inactive-text="浏览器直连（最快）" />
          <div class="hm-cfg-hint">直连：页面直接用「浏览器端」Key 向天地图取图，速度最好，但该 Key 会出现在浏览器请求里（登录用户可见）。<br />中转：瓦片经本系统服务器转发，浏览器完全拿不到 Key，代价是服务器多一份流量。</div>
        </el-form-item>
        <el-form-item label="浏览器端 Key" v-if="onlineForm.source === 'tianditu' && !onlineForm.proxy">
          <el-input v-model="onlineForm.tk" type="password" show-password
                    :placeholder="onlineTkSet ? '留空表示不修改' : '粘贴天地图「浏览器端」Key'" />
          <div class="hm-cfg-hint" v-if="onlineTkSet">当前已存：{{ onlineTkMasked }}（界面只显示脱敏值，留空即不修改）</div>
        </el-form-item>
        <el-form-item label="服务端 Key" v-if="onlineForm.source === 'tianditu' && onlineForm.proxy">
          <el-input v-model="onlineForm.server_tk" type="password" show-password
                    :placeholder="onlineServerTkSet ? '留空表示不修改' : '粘贴天地图「服务端」Key'" />
          <div class="hm-cfg-hint" v-if="onlineServerTkSet">当前已存：{{ onlineServerTkMasked }}（用于服务器中转；界面只显示脱敏值）</div>
        </el-form-item>
        <el-form-item label="图层" v-if="onlineForm.source === 'tianditu'">
          <el-select v-model="onlineForm.layer" style="width: 200px">
            <el-option label="矢量地图（推荐）" value="vec" />
            <el-option label="影像地图" value="img" />
            <el-option label="地形晕渲" value="ter" />
          </el-select>
        </el-form-item>
      </el-form>
      <div class="hm-cfg-hint" v-if="onlineForm.source === 'tianditu'">
        · 天地图信息最全（中文地名/行政区），但需 Key 且有日配额：控制台 → 应用管理 → 创建应用<br />
        · 「浏览器端」Key 用于直连渲染，「服务端」Key 用于服务器中转（额度用尽会自动改用浏览器直连）<br />
        · 页面上只显示脱敏后的 Key（如 eb55****0133），完整值不会显示，留空保存即表示不修改
      </div>
      <div class="hm-cfg-hint" v-else>
        · 当前底图源<b>不需要 Key</b>，也没有日配额，保存后所有人直接可用<br />
        · 底图瓦片由浏览器直连对应地图服务，需终端联网（高德/Esri 在国内可直连）
      </div>
      <template #footer>
        <el-button @click="onlineDialog = false">取消</el-button>
        <el-button link type="primary" @click="clearOnline" v-if="onlineTkSet || onlineServerTkSet">清除 Key</el-button>
        <el-button type="primary" :loading="savingOnline" @click="saveOnline">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="settingsDialog" title="地理编码设置（用于精确热力）" width="560px">
      <el-form label-width="90px">
        <el-form-item label="服务商">
          <el-select v-model="geoConfig.provider" style="width: 220px">
            <el-option label="不使用（仅省级热力）" value="none" />
            <el-option label="高德地图" value="amap" />
            <el-option label="百度地图" value="baidu" />
          </el-select>
        </el-form-item>
        <el-form-item label="API Key" v-if="geoConfig.provider !== 'none'">
          <el-input v-model="geoConfig.key" type="password" show-password
                    :placeholder="geoConfig.provider === 'amap' ? '高德 Web 服务 Key' : '百度服务端 AK'" />
          <div class="hm-cfg-hint" v-if="geoConfig.key_set">当前已存：{{ geoConfig.key_masked }}（留空表示不修改）</div>
        </el-form-item>
      </el-form>
      <div class="hm-cfg-hint">
        · 高德：控制台 → 应用管理 → 创建应用 → 添加 Key，服务平台选「Web 服务」<br />
        · 百度：控制台 → 应用管理 → 创建应用 → 选「服务端」，复制 AK<br />
        · Key 只保存在本机数据库，仅用于向所选服务商查询坐标；已缓存 {{ geoConfig.cached }} 个地名
      </div>
      <template #footer>
        <el-button @click="settingsDialog = false">取消</el-button>
        <el-button link type="primary" @click="clearGeoCache" v-if="geoConfig.cached">清空坐标缓存</el-button>
        <el-button type="primary" :loading="savingCfg" @click="saveSettings">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
// 在线底图规则/配置统一走 utils/basemap.js（祠宇志/茔墓志「地图选点」共用）
import { TDT_ANN, onlineTileURL, tiandituTileURL } from '../utils/basemap'

const router = useRouter()
const chartEl = ref(null)
const wrapEl = ref(null)
const leafEl = ref(null)
// 底图选择：none=无底图（仅示意多边形），否则为离线包 ID（A/B/C/D）
const baseMap = ref(localStorage.getItem('hm_basemap') || 'none')
const offlinePacks = ref([])
let chart = null
let chinaGeo = null
let centers = {}
let provAdcode = {}
const loadedMaps = {}
let cityGeo = null // 当前下钻省的地图数据

// ---------- 地图缩放（自研，桌面 / 手机分开设置）----------
// 地图 series 只负责平移（roam:'move'），缩放由我们自己算，灵敏度可控
const mapZoom = ref(1.08)   // 当前缩放倍率
const ZOOM_DEFAULT = 1.08   // 全国默认
const ZOOM_PROV = 1.05      // 下钻到省后默认
const ZOOM_MIN = 0.8
const ZOOM_MAX = 30
const WHEEL_SENS = 1.0018   // 【电脑端】滚轮灵敏度：越大越快（单档滚轮 120 约 1.24x）
const PINCH_SENS = 1.0      // 【手机端】双指灵敏度：1=跟手；<1 更迟钝，>1 更灵敏

const loading = ref(false)
const tab = ref('geo')
const branch = ref('')
const genMin = ref(null)
const genMax = ref(null)
const geoField = ref('native')
const drawer = ref(false)
const drawerLoading = ref(false)
const drawerField = ref('native')
const drawerProv = ref('')
const drawerCity = ref('')
const members = ref([])
const drawerTitle = ref('')

// ---------- 「全部」模式：三类落点分色 + 图例 ----------
// 类别 key 与后端 /heatmap/points 的 points_by_field[].kind 对应
// native=祖籍 / current=现居地 / fallback=现居地同祖籍（未迁出） / burial=葬地
const CAT_PALETTE = {
  native: { color: '#1f6fd0', shape: '50%', label: '祖籍', icon: '籍' },
  current: { color: '#e07b00', shape: '50%', label: '现居地', icon: '居' },
  fallback: { color: '#12a5a5', shape: '3px', label: '现居地同祖籍（未迁出）', icon: '未' },
  burial: { color: '#7b3fbf', shape: '1px', label: '葬地', icon: '葬' }
}
// 同一落点上多类重叠时，按类别做「屏幕像素」微错开（不改真实经纬度，仅视觉分离）
const CAT_OFFSET = { native: [-9, -5], current: [9, -5], fallback: [-9, 6], burial: [9, 6] }
const CAT_ORDER = ['native', 'current', 'fallback', 'burial']
const DRAWER_CAT = { fallback: 'current_fallback' }

function catMeta(kind) {
  if (kind === 'current_fallback') return { ...CAT_PALETTE.fallback, key: 'fallback' }
  return { key: kind, ...(CAT_PALETTE[kind] || { color: '#888', shape: '50%', label: kind, icon: '?' }) }
}
function catColor(kind) { return catMeta(kind).color }
// 落点 → 类别 key（后端 kind 为 current 且 fallback=true 时归到「未迁出」）
function pointCat(p) {
  if (p.kind === 'current' && p.fallback) return 'fallback'
  return p.kind
}
const catOn = ref({ native: true, current: true, fallback: true, burial: true })
function catEnabled(k) { return catOn.value[k] !== false }
// 图例行点选：必须同时驱动「数据过滤 + 两条渲染路径重绘」，否则只是样式变灰。
// 这里重新向后端取一次（cats 白名单），让图例人数、提示条、ECharts、Leaflet 全部同步。
async function toggleCat(k) {
  if (!isAll.value) return
  catOn.value = { ...catOn.value, [k]: !catEnabled(k) }
  try {
    await loadPoints(false)   // 重新取数：关掉的类别后端也不再统计
    if (drawer.value) await refreshDrawer()
  } finally {
    refreshView()             // 两条渲染路径（ECharts / Leaflet）都按新数据重绘
  }
}
const isAll = computed(() => geoField.value === 'all')
const showCatLegend = computed(() => tab.value === 'geo' && isAll.value)
// 抽屉里是否展示「类别」列（打开抽屉时按当时的模式定格，避免中途切换选择器错乱）
const drawerAll = computed(() => drawerField.value === 'all')

// 图例开关 → 后端 cats 参数（全部启用时不传，保持接口默认行为）
const anyCatOff = computed(() => CAT_ORDER.some(k => !catEnabled(k)))
function catsParam() {
  if (!anyCatOff.value) return ''
  const on = CAT_ORDER.filter(k => catEnabled(k))
  return on.length ? on.join(',') : 'none'
}
// 已关闭类别的显示名（提示条里列出）
const offCatLabels = computed(() => CAT_ORDER.filter(k => !catEnabled(k)).map(k => catMeta(k).label))

// 参与渲染/图例的类别批次（含颜色与人数）——图例关掉的类别在这里就被剔除
const catBatches = computed(() => {
  if (!isAll.value) return []
  const out = []
  for (const b of (pointsAll.value || [])) {
    const k = pointCat(b)
    if (!catEnabled(k)) continue
    const ex = out.find(o => o.key === k)
    if (ex) { ex.total += b.total || 0; ex.points = ex.points.concat(b.points || []) } else {
      out.push({ key: k, total: b.total || 0, points: (b.points || []).slice() })
    }
  }
  for (const c of (cats.value || [])) {
    if (!catEnabled(c.key)) continue
    const ex = out.find(o => o.key === c.key)
    if (ex) ex.geoTotal = c.total
    else out.push({ key: c.key, total: 0, points: [], geoTotal: c.total })
  }
  return out
})
// 图例行（颜色 → 类别 + 人数）；未开精确热力时用省级合计人数
// 注意：被点选关掉的类别仍要留在图例里（否则关掉后行就没了、无法再点开），
// 这类行的人数显示「—」（数据已从后端口径里剔除，不再虚报数字）。
const catLegend = computed(() => CAT_ORDER.map(k => {
  const b = catBatches.value.find(o => o.key === k)
  const on = catEnabled(k)
  const count = on ? (precise.value ? (b ? b.total || 0 : 0) : (b ? b.geoTotal || 0 : 0)) : null
  return {
    key: k,
    label: catMeta(k).label,
    color: catMeta(k).color,
    shape: catMeta(k).shape,
    on,
    count,
    mode: precise.value ? 'point' : 'province'
  }
}))
// 「全部」模式下各批次的省级列表（无精确热力时按类别分色渲染省份）——关掉的类别不返回。
// current 与 fallback 共用后端 geo.current（含落回者），人数分开取自 currentDeclaredGeo / currentDeclared。
const cats = computed(() => {
  if (!isAll.value) return []
  const g = (data.value && data.value.geo) || {}
  const curAll = g.current || []
  const out = []
  if (catEnabled('native')) {
    const list = g.native || []
    out.push({ key: 'native', field: 'native', list, total: list.reduce((s, i) => s + (i.count || 0), 0) })
  }
  // 现居地（真实填报）：省份用 current 全量（地图着色不让未迁出者凭空消失），人数用 currentDeclared
  if (catEnabled('current') && curAll.length) {
    out.push({ key: 'current', field: 'current', list: curAll, total: (g.currentDeclared || 0) })
  }
  // 未迁出（落回祖籍）：省份列表没有单独的落回聚合，这里沿用 current 全量，人数取 currentFromNative
  if (catEnabled('fallback') && curAll.length) {
    out.push({ key: 'fallback', field: 'current', list: curAll, total: (g.currentFromNative || 0) })
  }
  if (catEnabled('burial')) {
    const list = g.burial || []
    out.push({ key: 'burial', field: 'burial', list, total: list.reduce((s, i) => s + (i.count || 0), 0) })
  }
  return out
})

// 现居地「落回祖籍」的统计（来自后端 /heatmap 的 geo.currentFromNative）
const currentFallback = computed(() => {
  const g = (data.value && data.value.geo) || {}
  return { count: g.currentFromNative || 0, declared: g.currentDeclared || 0, enabled: !!g.currentFallback }
})

// 当前可见的落点数与可见人数（图例点选后随之变化，界面/验证都用它）
const visiblePoints = computed(() => isAll.value
  ? pointsAll.value.filter(b => catEnabled(pointCat(b))).reduce((s, b) => s + (b.points || []).length, 0)
  : points.value.length)
const visibleMembers = computed(() => pointsVisibleMembers.value)


const precise = ref(false)
const points = ref([])
const pointsAll = ref([])   // 「全部」模式：按类别分组的落点（points_by_field）
const pointsVisibleMembers = ref(0) // 当前点选开关下命中的成员数（后端 visible_members）
const pointsConfigured = ref(false)
const pointsPending = ref(0)
const pointsFailed = ref(0)
const pointsTotalPlaces = ref(0)
const pointsHint = ref('')
const pointsLoading = ref(false)
const renderFail = ref('')

const settingsDialog = ref(false)
const savingCfg = ref(false)
const geoConfig = reactive({ provider: 'none', key: '', key_set: false, key_masked: '', cached: 0 })

// 在线底图（服务端统一配置，全成员通用；除天地图外均免 Key）
const onlineSource = ref('tianditu')
const onlineTk = ref('')          // 浏览器端 Key（直连渲染用）
const onlineTkSet = ref(false)
const onlineLayer = ref('vec')
const onlineTkMasked = ref('')
const onlineServerTkSet = ref(false)
const onlineServerTkMasked = ref('')
const onlineProxy = ref(false)
const proxyQuotaOut = ref(false) // 服务端 Key 额度用尽 → 已自动切换为「浏览器直连」
function useProxy() { return onlineProxy.value && !proxyQuotaOut.value }
const onlineDialog = ref(false)
const savingOnline = ref(false)
const onlineForm = reactive({ source: 'tianditu', tk: '', server_tk: '', layer: 'vec', proxy: false })

// 可选底图源（id 需与后端 mapOnlineSources 一致）
const ONLINE_SOURCES = [
  { id: 'tianditu', label: '天地图（需 Key·中文最全）' },
  { id: 'amap', label: '高德街道图（免费·无需 Key·中文）' },
  { id: 'amap_sat', label: '高德卫星影像（免费·无需 Key）' },
  { id: 'esri_sat', label: 'Esri 卫星影像（免费·无需 Key）' }
]
const ONLINE_ATTR = {
  tianditu: '© 天地图',
  amap: '© 高德地图',
  amap_sat: '© 高德地图',
  esri_sat: 'Esri / Maxar / Earthstar Geographics'
}
function onlineNeedTk() { return (onlineSource.value || 'tianditu') === 'tianditu' }
function onlineTkReady() { return onlineProxy.value ? onlineServerTkSet.value : onlineTkSet.value }
function onlineAttr() { return ONLINE_ATTR[onlineSource.value] || '© 高德地图' }
const drill = reactive({ prov: '', adcode: '' })

const data = ref({
  total: 0,
  branchOptions: [],
  geo: { native: [], current: [], burial: [] },
  city: { native: {}, current: {}, burial: {} },
  branch: [],
  timeline: { birth: {}, death: {} },
  migration: [],
  migrationComputed: [],
  hasMigrationEvents: false
})

const total = computed(() => data.value.total || 0)
const branchOptions = computed(() => data.value.branchOptions || [])
const legacyKey = computed(() => (tab.value === 'migration' && !data.value.hasMigrationEvents))
const migList = computed(() => data.value.hasMigrationEvents
  ? (data.value.migration || [])
  : (data.value.migrationComputed || []))

const tabHint = computed(() => {
  const d = data.value
  if (tab.value === 'geo') {
    if (geoField.value === 'all') {
      return (cats.value.length ? '' : '当前筛选下没有可用于该视图的数据（成员资料里需填写「籍贯 / 现居地 / 葬地」）')
    }
    const list = (d.geo || {})[geoField.value] || []
    return list.length ? '' : '当前筛选下没有可用于该视图的数据（成员资料里需填写「籍贯 / 现居地 / 葬地」）'
  }
  if (tab.value === 'branch') return (d.branch || []).length ? '' : '当前筛选下没有房支数据'
  if (tab.value === 'migration') return migList.value.length ? '' : '暂无迁徙数据（可在「迁徙记录」页录入，或等成员填好籍贯与葬地后自动推算）'
  if (tab.value === 'timeline') {
    const b = Object.keys(d.timeline.birth || {}).length
    const de = Object.keys(d.timeline.death || {}).length
    return (b || de) ? '' : '成员资料里没有可识别的生卒公历年份'
  }
  return ''
})

// 地理视图提示：把「现居地落回祖籍」说清楚（含人数），其余沿用 tabHint
const geoHint = computed(() => {
  if (tab.value !== 'geo') return tabHint.value
  // 现居地模式：明确标注其中有多少人未迁出（按祖籍坐标显示）
  if (geoField.value === 'current' && currentFallback.value.enabled && currentFallback.value.count > 0) {
    const list = ((data.value.geo || {}).current || [])
    if (list.length) {
      const n = currentFallback.value.count, dec = currentFallback.value.declared
      return `现居地未填写者已落回其祖籍显示（未迁出，共 ${n} 人）；有现居地的 ${dec} 人按其现居地显示。明细表里以「同祖籍（未迁出）」标注。`
    }
  }
  return tabHint.value
})

// 「全部」模式的分步提示（配置项、落点解析进度等）
const allHintLines = computed(() => {
  if (tab.value !== 'geo' || !isAll.value || loading.value) return []
  const out = []
  if (!precise.value) {
    out.push('「全部」模式：三类落点分色显示（祖籍青蓝 / 现居地橙 / 现居地同祖籍（未迁出）青色 / 葬地紫）。当前未开「精确热力」，先按省份分色着色；勾选「精确热力（需地理编码）」可看到精确落点。')
  }
  if (precise.value && !pointsConfigured.value) {
    out.push('未配置地理编码服务，暂时只能按省级分色显示；点上方「⚙️ 地理编码设置」填写高德或百度 Key 后即可显示精确落点。')
  }
  if (precise.value && pointsConfigured.value && points.value.length === 0) {
    out.push('精确落点尚未解析出坐标（可在「⚙️ 地理编码设置」里确认 Key / 稍后重试）。')
  }
  if (offCatLabels.value.length) {
    out.push(`已按图例点选关闭：${offCatLabels.value.join('、')}——该类别的落点、省份着色、图例人数与明细表都已同步剔除（再点图例行可恢复）。`)
  }
  return out
})

const preciseStatus = computed(() => {
  if (pointsLoading.value) return '正在解析地址…'
  if (!pointsConfigured.value) return '未配置地理编码'
  if (isAll.value) {
    const parts = [`已定位 ${points.value.length} 个落点（当前显示 ${visiblePoints.value} 个）`, `可见 ${visibleMembers.value} 人`]
    for (const b of catBatches.value) {
      parts.push(`${catMeta(b.key).label} ${b.total} 人`)
    }
    if (pointsPending.value) parts.push(`待解析 ${pointsPending.value}`)
    if (pointsFailed.value) parts.push(`失败 ${pointsFailed.value}`)
    return parts.join(' / ')
  }
  const parts = [`已定位 ${points.value.length} 个点`]
  if (pointsPending.value) parts.push(`待解析 ${pointsPending.value}`)
  if (pointsFailed.value) parts.push(`失败 ${pointsFailed.value}`)
  return parts.join(' / ')
})

const role = computed(() => {
  try {
    const u = JSON.parse(localStorage.getItem('user') || '{}')
    return u.role || localStorage.getItem('role') || ''
  } catch { return localStorage.getItem('role') || '' }
})
const canConfig = computed(() => !role.value || ['理事长', '超级管理员'].includes(role.value))

async function initMap() {
  const geo = await fetch('/china.json').then(r => r.json())
  centers = {}
  provAdcode = {}
  chinaGeo = geo
  for (const f of (geo.features || [])) {
    const p = f.properties || {}
    const c = p.center || p.centroid
    if (p.name && c) centers[p.name] = c
    if (p.name && p.adcode) provAdcode[p.name] = p.adcode
  }
  echarts.registerMap('china', geo)
  loadedMaps.china = true
}

function queryParams() {
  const p = {}
  if (branch.value) p.branch = branch.value
  if (genMin.value) p.gen_min = genMin.value
  if (genMax.value) p.gen_max = genMax.value
  // 「全部」模式下图例点选的类别白名单（关掉的类别后端也不统计 → 数字不会包含它）
  if (isAll.value) { const c = catsParam(); if (c) p.cats = c }
  return p
}

async function load() {
  loading.value = true
  try {
    const r = await api.get('/heatmap', { params: queryParams() })
    data.value = r.data
  } catch (e) {
    // 失败时保留上一次数据
  } finally {
    loading.value = false
    await nextTick()
    if (precise.value && tab.value === 'geo') await loadPoints(false)
    render()
    fitChartHeight()
    // Leaflet 模式：首次进入就绘制，不必等用户切页签
    if (leafActive()) { ensureLeaf(); if (leafMap) leafMap.invalidateSize(); renderLeaf() }
  }
}

function reset() {
  branch.value = ''
  genMin.value = null
  genMax.value = null
  drill.prov = ''
  drill.adcode = ''
  load()
}

async function onFieldChange() {
  drill.prov = ''
  drill.adcode = ''
  // 切换类别（尤其进入/离开「全部」）后旧落点缓存要作废
  points.value = []
  pointsAll.value = []
  if (isAll.value) catOn.value = { native: true, current: true, fallback: true, burial: true }
  // 必须等新字段的落点拉回来再渲染，否则会先渲染上一次选择的结果
  if (precise.value) { await loadPoints(true) }
  render()
}

// ---------- 离线底图（Leaflet 真地图）----------
// 之前试过「在 ECharts 画布下层铺瓦片 <img>」，问题很多：ECharts 的 map 系列是
// 「纬度线性」投影、漫游状态也不便同步，导致瓦片错位/重影/拖动缩放不同步。
// 现在改为：选了离线底图时，整张地图（瓦片 + 省界 + 精确落点）交给 Leaflet 渲染。
let leafMap = null
let leafTiles = { base: null, label: null }
let leafPoly = null
let leafPts = null
let leafPack = ''
let leafViewKey = ''
// 迁徙轨迹「行走的人」动画
let migAnimRAF = 0
let migAnimTracks = []
let leafArrows = null
let migArrowSpecs = []
function stopMigAnim() {
  if (migAnimRAF) { cancelAnimationFrame(migAnimRAF); migAnimRAF = 0 }
  migAnimTracks = []
}
// 箭头流：一颗带头箭头 + 若干尾随箭头（越靠后越小越淡），整体沿轨迹滑行 → 「带尾巴的箭头」
const MIG_ARROW_N = 6        // 每段轨迹的箭头数（1 头 + 尾巴）
const MIG_ARROW_STEP = 13    // 相邻箭头间距（屏幕像素）
const MIG_ARROW_SPEED = 40   // 滑行速度（像素/秒）
const MIG_ROW_GAP = 17       // 队形「排」间距（屏幕像素，沿行进方向）
const MIG_COL_GAP = 16       // 同一排内小人的横向间距（屏幕像素）
// 三角队形：第 1 排 1 个、第 2 排 2 个、第 3 排 3 个…
function migFormation(k) {
  let r = 1, acc = 0
  while (acc + r <= k) { acc += r; r++ }
  return { row: r, col: k - acc, n: r }
}
// 小人随地图缩放同步缩小（zoom 13 及以上 = 1，越小越缩，最低 0.45）
let migWS = 1
function migWalkerScale() {
  const z = leafMap ? leafMap.getZoom() : 13
  return Math.max(0.45, Math.min(1, 0.45 + 0.092 * (z - 7)))
}
function applyMigWalkerScale() {
  migWS = migWalkerScale()
  migArrowSpecs.forEach(sp => (sp.walkers || []).forEach(mk => {
    const el = mk.getElement()
    if (el) el.style.setProperty('--mws', migWS.toFixed(3))
  }))
}
// 以轨迹点为基准做横向（垂直于走向）像素偏移
function migOffset(q, offPx) {
  if (!offPx || !leafMap) return [q.lat, q.lng]
  const p = leafMap.latLngToContainerPoint(L.latLng(q.lat, q.lng))
  const rd = q.deg * Math.PI / 180
  const tx = Math.sin(rd), ty = -Math.cos(rd) // 屏幕切线（北= -y）
  return leafMap.containerPointToLatLng(L.point(p.x - ty * offPx, p.y + tx * offPx))
}
function migArrowScale(k, n) { return 1.05 - 0.62 * (k / Math.max(1, n - 1)) }
function migArrowAlpha(k, n) { return 1 - 0.66 * (k / Math.max(1, n - 1)) }
// 按像素里程 d 取轨迹上的点 + 该处的走向角
function migPointAt(cum, cs, d) {
  let i = 1
  while (i < cum.length - 1 && cum[i] < d) i++
  const seg = Math.max(1e-6, cum[i] - cum[i - 1])
  const t = Math.max(0, Math.min(1, (d - cum[i - 1]) / seg))
  return {
    lat: cs[i - 1][0] + (cs[i][0] - cs[i - 1][0]) * t,
    lng: cs[i - 1][1] + (cs[i][1] - cs[i - 1][1]) * t,
    deg: bearing(cs[i - 1], cs[i])
  }
}
function startMigAnim(specs) {
  stopMigAnim()
  if (!specs.length) return
  migAnimTracks = specs
  const t0 = performance.now()
  let last = t0
  // 慢一点：跑完一整条轨迹 15s，到终点后停留一下再重来
  const cycle = 15000, walkFrac = 0.82
  const step = now => {
    const dt = Math.min(0.06, (now - last) / 1000)
    last = now
    const p = ((now - t0) % cycle) / cycle
    const prog = p < walkFrac ? p / walkFrac : 1
    migAnimTracks.forEach(sp => {
      // 小人：三角队形（第 1 排 1 个、第 2 排 2 个…）沿轨迹前进
      const ws = sp.walkers
      if (ws && ws.length && sp.total > 0) {
        const s0 = prog * sp.total
        for (let k = 0; k < ws.length; k++) {
          const f = migFormation(k)
          const d = Math.max(0, s0 - (f.row - 1) * MIG_ROW_GAP * migWS)
          const q = migPointAt(sp.cum, sp.cs, d)
          ws[k].setLatLng(migOffset(q, (f.col - (f.n - 1) / 2) * MIG_COL_GAP * migWS))
          ws[k].setOpacity(prog >= 1 ? 0.5 : 1)
        }
      }
      // 箭头沿轨迹流动（头在前，尾巴渐弱）
      const mks = sp.markers
      if (!mks || !mks.length || !(sp.total > 4)) return
      const span = (mks.length - 1) * MIG_ARROW_STEP
      sp.head += MIG_ARROW_SPEED * dt
      if (sp.head > sp.total + span) sp.head = 0
      for (let k = 0; k < mks.length; k++) {
        const d = sp.head - k * MIG_ARROW_STEP
        const mk = mks[k]
        if (d < 0 || d > sp.total) { mk.setOpacity(0); continue }
        const q = migPointAt(sp.cum, sp.cs, d)
        mk.setLatLng([q.lat, q.lng])
        const el = mk.getElement()
        if (el) {
          el.style.opacity = String(migArrowAlpha(k, mks.length))
          const ic = el.firstElementChild
          if (ic) ic.style.transform = 'rotate(' + q.deg.toFixed(1) + 'deg) scale(' + migArrowScale(k, mks.length).toFixed(3) + ')'
        }
      }
    })
    migAnimRAF = requestAnimationFrame(step)
  }
  migAnimRAF = requestAnimationFrame(step)
}
const leafMax = ref(0)
// 瓦片加载健康度：连续多张失败 → 提示（有瓦片成功加载则清除，避免误报）
const leafTileErr = ref(false)
let tileErrN = 0
let tileOkN = 0
function watchTileHealth(layer) {
  layer.on('tileerror', () => {
    tileOkN = 0
    tileErrN++
    if (tileErrN >= 4) {
      leafTileErr.value = true
      probeQuotaFallback()
    }
  })
  layer.on('tileload', () => {
    tileOkN++
    if (tileOkN >= 2) { tileErrN = 0; leafTileErr.value = false }
  })
}

// 中转模式下底图连续报错：问一下服务端是不是天地图 Key 额度用尽 → 自动改用「浏览器直连」
let quotaProbeBusy = false
async function probeQuotaFallback() {
  if (!useProxy() || quotaProbeBusy) return
  quotaProbeBusy = true
  try {
    const { data } = await api.get('/map/online-config')
    if (data && (data.quota_exhausted || !data.server_tk_set)) switchToBrowserDirect(!!data?.quota_exhausted)
  } catch (e) { /* 忽略 */ } finally { quotaProbeBusy = false }
}

function switchToBrowserDirect(quota = true) {
  proxyQuotaOut.value = true
  leafTileErr.value = false
  tileErrN = 0
  if (!onlineTkSet.value) {
    ElMessage.warning('天地图「服务端」Key ' + (quota ? '额度已用尽' : '不可用') + '，且未配置「浏览器端」Key，无法自动切换；请在「在线底图设置」里填写浏览器端 Key，或改用免 Key 的底图源。')
  } else {
    ElMessage.info('天地图「服务端」Key ' + (quota ? '额度已用尽' : '不可用') + '，已自动切换为「浏览器直连」（浏览器端 Key），可继续浏览')
  }
  if (leafMap) setupLeafTiles()
}
const RAMP = ['#eaf3ff', '#a9cbea', '#4b86c4', '#1b3f77']
const CN_BOUNDS = [[16, 71], [55.7, 138]]

// 当前是否由 Leaflet 渲染（「地域分布」/「迁徙轨迹」页 + 选了底图）
function leafActive() { return (tab.value === 'geo' || tab.value === 'migration') && baseMap.value !== 'none' }

async function loadPacks() {
  try {
    const { data } = await api.get('/map/packs')
    offlinePacks.value = (data.packs || []).filter(p => p.has_tiles)
    if (baseMap.value !== 'none' && baseMap.value !== 'online' && !offlinePacks.value.some(p => p.id === baseMap.value)) {
      baseMap.value = 'none'
    }
  } catch (e) { /* 忽略 */ }
  // 首次进入时「底图包列表」可能比热力图数据晚返回：早先这里不补渲染，
  // 导致 Leaflet 拿不到包定义（setupLeafPack 返回 null）→ 底图空白，
  // 必须手动切一次页签/底图才显示。这里补一次。
  refreshView() // 包列表到位后再完整刷一次（首屏空白的老问题）
}

function onBaseMapChange() {
  localStorage.setItem('hm_basemap', baseMap.value)
  leafViewKey = ''
  refreshView()
  if (baseMap.value === 'online' && onlineNeedTk() && !onlineTkReady()) {
    if (canConfig.value) { ElMessage.info('当前底图源需要天地图 Key（' + (onlineProxy.value ? '服务端' : '浏览器端') + '），请先配置（或改用免 Key 的底图源）'); openOnlineSettings() }
    else { ElMessage.info('在线底图尚未配置，请联系管理员设置') }
  }
}

// 统一的「刷新当前视图」：重画 + 重新量高度 + 纠正画布尺寸 + 重绘底图/瓦片。
// 首次进入时页面布局/接口返回顺序都可能还没稳定，单独一次渲染会得到空白
// （以前表现为「必须切换一次页签/底图才显示」），所以首帧后再完整走一遍这个流程。
function refreshView() {
  render()
  fitChartHeight()
  nextTick(() => {
    if (leafActive()) { ensureLeaf(); if (leafMap) leafMap.invalidateSize(); renderLeaf() }
    else if (chart && chartEl.value && chartEl.value.offsetParent !== null) chart.resize()
  })
}

// 与 ECharts 侧色带保持一致
function rampColor(ratio) {
  const t = Math.max(0, Math.min(1, ratio || 0)) * (RAMP.length - 1)
  const i = Math.min(RAMP.length - 2, Math.floor(t)), f = t - i
  const c = (hex, k) => parseInt(hex.slice(k, k + 2), 16)
  const mix = k => Math.round(c(RAMP[i], k) + (c(RAMP[i + 1], k) - c(RAMP[i], k)) * f)
  return '#' + [1, 3, 5].map(k => mix(k).toString(16).padStart(2, '0')).join('')
}

function escapeHtml(v) {
  return String(v == null ? '' : v).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]))
}

function tileURL(pack, layer) {
  const t = encodeURIComponent(localStorage.getItem('token') || '')
  return `/api/map/tiles/${pack}/${layer}/{z}/{x}/{y}?token=${t}`
}

function ensureLeaf() {
  if (leafMap || !leafEl.value) return
  leafMap = L.map(leafEl.value, {
    center: [35, 104], zoom: 4, zoomControl: true, attributionControl: false,
    minZoom: 2, maxZoom: 12, zoomSnap: 0.5, maxBounds: CN_BOUNDS, maxBoundsViscosity: 0.7
  })
  // 注记瓦片单独一层，压在底图之上
  leafMap.createPane('hmLabel')
  leafMap.getPane('hmLabel').style.zIndex = 350
  leafPoly = L.layerGroup().addTo(leafMap)
  leafPts = L.layerGroup().addTo(leafMap)
  leafArrows = L.layerGroup().addTo(leafMap)
  // 缩放后箭头间距会变，重算一次（保持「连在一起」的观感）
  leafMap.on('zoomend', () => {
    if (tab.value === 'migration' && leafActive()) { drawMigArrows(); applyMigWalkerScale() }
  })
}

// 天地图在线瓦片：默认浏览器直连（用「浏览器端」Key，速度最好）；也可切成经本系统服务器中转（Key 不下发浏览器）
// 注记层：矢量→cva、影像→cia、地形→cta
// 瓦片地址规则与 TDT_ANN 已抽到 utils/basemap.js（「祠宇志 / 茔墓志 → 地图选点」共用同一套，勿另写）
function tiandituURL(layer) {
  return tiandituTileURL(layer, {
    proxy: useProxy(),
    tk: onlineTk.value || '',
    token: localStorage.getItem('token') || ''
  })
}

function clearLeafTiles() {
  if (!leafMap) return
  if (leafTiles.base) { leafMap.removeLayer(leafTiles.base); leafTiles.base = null }
  if (leafTiles.label) { leafMap.removeLayer(leafTiles.label); leafTiles.label = null }
  leafPack = ''
  leafTileErr.value = false
  tileErrN = 0
  tileOkN = 0
}

// 配置当前底图的瓦片层；返回是否已就绪
// - 离线包：从 /api/map/tiles 读本地瓦片
// - 在线：天地图（浏览器端 Key）/ 免 Key 的 OSM 系列源，浏览器直连加载
function setupLeafTiles() {
  if (!leafMap) return false
  if (baseMap.value === 'online') {
    const src = onlineSource.value || 'tianditu'
    const eff = useProxy()
    const tdtReady = eff ? onlineServerTkSet.value : onlineTkSet.value
    if (src === 'tianditu' && !tdtReady) { clearLeafTiles(); return false }
    const key = 'online:' + src + ':' + onlineLayer.value + ':' + (eff ? 'p' : 'd') + ':' + (tdtReady ? '1' : '0') + ':' + (proxyQuotaOut.value ? 'q' : '')
    if (leafTiles.base && leafPack === key) return true
    clearLeafTiles()
    if (src === 'tianditu') {
      const ann = TDT_ANN[onlineLayer.value] || 'cva'
      const opt = { minZoom: 1, maxZoom: 19, noWrap: true, keepBuffer: 1, subdomains: '01234567', maxNativeZoom: 18 }
      leafTiles.base = L.tileLayer(tiandituURL(onlineLayer.value), opt).addTo(leafMap)
      leafTiles.label = L.tileLayer(tiandituURL(ann), Object.assign({ pane: 'hmLabel' }, opt)).addTo(leafMap)
    } else {
      const u = onlineTileURL(src)
      if (!u) { clearLeafTiles(); return false }
      const opt = { minZoom: 1, maxZoom: 19, noWrap: true, keepBuffer: 1, subdomains: u.subdomains || 'abc', maxNativeZoom: u.maxNativeZoom || 19 }
      leafTiles.base = L.tileLayer(u.url, opt).addTo(leafMap)
      if (u.label) leafTiles.label = L.tileLayer(u.label, Object.assign({ pane: 'hmLabel' }, opt)).addTo(leafMap)
    }
    watchTileHealth(leafTiles.base)
    if (leafTiles.label) watchTileHealth(leafTiles.label)
    leafPack = key
    leafMap.setMinZoom(1)
    leafMap.setMaxZoom(19)
    return true
  }
  const def = offlinePacks.value.find(p => p.id === baseMap.value)
  if (!def) return false
  if (!leafTiles.base || leafPack !== def.id) {
    clearLeafTiles()
    const opt = {
      minZoom: 1, maxZoom: 18, noWrap: true, keepBuffer: 1,
      minNativeZoom: def.min_zoom || 3, maxNativeZoom: def.max_zoom || 8
    }
    leafTiles.base = L.tileLayer(tileURL(def.id, 'base'), opt).addTo(leafMap)
    leafTiles.label = L.tileLayer(tileURL(def.id, 'label'), Object.assign({ pane: 'hmLabel' }, opt)).addTo(leafMap)
    watchTileHealth(leafTiles.base)
    watchTileHealth(leafTiles.label)
    leafPack = def.id
    leafMap.setMinZoom(Math.max(1, (def.min_zoom || 3) - 1))
    leafMap.setMaxZoom((def.max_zoom || 8) + 3)
  }
  return true
}

// 渲染省/市面 + 精确落点
function renderLeaf() {
  if (!leafActive() || !leafEl.value) return
  ensureLeaf()
  if (!setupLeafTiles()) return
  if (tab.value === 'migration') { renderLeafMigration(); return }
  const field = geoField.value
  const label = field === 'burial' ? '葬地' : (field === 'current' ? '现居地' : '籍贯')
  const d = data.value || {}
  leafPoly.clearLayers()
  leafPts.clearLayers()
  leafMax.value = 0

  const addGeo = (geo, byName, max, cityLevel) => {
    return L.geoJSON(geo, {
      style: f => ({ fillColor: rampColor((byName[f.properties.name] || 0) / max), fillOpacity: 0.42, color: '#ffffff', weight: 0.7 }),
      onEachFeature: (f, lyr) => {
        const n = byName[f.properties.name] || 0
        lyr.bindTooltip(`${f.properties.name}<br/>${label}：${n} 人${n ? '<br/><span style="color:#999">点击查看明细</span>' : ''}`, { sticky: true })
        lyr.on('click', () => {
          if (!n) return
          if (cityLevel) openMembers(drill.prov, f.properties.name)
          else drillTo(f.properties.name)
        })
      }
    }).addTo(leafPoly)
  }

  if (drill.prov && cityGeo) {
    if (field === 'all') {
      // 三类合一（同一个市可能既有祖籍者也有现居者/葬者），中性色阶 + 分类明细提示
      // 图例关掉的类别：不参与城市合计，也不出现在 tooltip 明细里
      const useCity = ['native', 'current', 'burial'].filter(f => catEnabled(f))
      const acc = {}, detail = {}
      for (const f of useCity) {
        (((d.city || {})[f] || {})[drill.prov] || []).forEach(i => {
          acc[i.name] = (acc[i.name] || 0) + (i.count || 0)
          detail[i.name] = detail[i.name] || {}
          detail[i.name][f] = (detail[i.name][f] || 0) + (i.count || 0)
        })
      }
      const max = Math.max(1, ...Object.values(acc))
      leafMax.value = Object.values(acc).length ? Math.max(...Object.values(acc)) : 0
      const lyr = L.geoJSON(cityGeo, {
        style: f => ({ fillColor: rampColor((acc[f.properties.name] || 0) / max), fillOpacity: 0.42, color: '#ffffff', weight: 0.7 }),
        onEachFeature: (f, lyr2) => {
          const n = acc[f.properties.name] || 0
          const det = detail[f.properties.name] || {}
          const rows = useCity.filter(k => det[k]).map(k => `${catMeta(k).label}：${det[k]} 人`).join('<br/>')
          lyr2.bindTooltip(`${f.properties.name}<br/>${rows || '无数据'}${n ? '<br/><span style="color:#999">点击查看明细</span>' : ''}`, { sticky: true })
          lyr2.on('click', () => { if (n) openMembers(drill.prov, f.properties.name) })
        }
      }).addTo(leafPoly)
      addAllLeafPoints()
      const key = 'p:' + drill.prov
      if (key !== leafViewKey) {
        const b = lyr.getBounds()
        if (b.isValid()) leafMap.fitBounds(b, { padding: [10, 10] })
        leafViewKey = key
      }
      return
    }
    const cities = ((d.city || {})[field] || {})[drill.prov] || []
    const max = Math.max(1, ...cities.map(i => i.count))
    leafMax.value = cities.length ? Math.max(...cities.map(i => i.count || 0)) : 0
    const byName = {}
    cities.forEach(i => { byName[i.name] = i.count || 0 })
    const lyr = addGeo(cityGeo, byName, max, true)
    const key = 'p:' + drill.prov
    if (key !== leafViewKey) {
      const b = lyr.getBounds()
      if (b.isValid()) leafMap.fitBounds(b, { padding: [10, 10] })
      leafViewKey = key
    }
  } else {
    if (field === 'all') {
      // 省界用中性灰（避免与类别颜色混淆），图例/落点承担分色
      L.geoJSON(chinaGeo, {
        style: () => ({ fillColor: '#eef1f5', fillOpacity: 0.5, color: '#ffffff', weight: 0.7 }),
        onEachFeature: (f, lyr) => {
          const prov = f.properties.name
          const rows = ['native', 'current', 'burial']
            .filter(k => catEnabled(k))
            .map(k => [catMeta(k).label, (((d.geo || {})[k] || []).find(x => x.province === prov) || {}).count || 0])
            .map(([lb, n]) => `${lb}：${n} 人`).join('<br/>')
          lyr.bindTooltip(`${prov}<br/>${rows || '无数据'}${precise.value && points.value.length ? '' : '<br/><span style="color:#999">勾选「精确热力」看落点</span>'}`, { sticky: true })
          lyr.on('click', () => drillTo(prov))
        }
      }).addTo(leafPoly)
      addAllLeafPoints()
      if (leafViewKey !== 'china') { leafMap.setView([35, 104], 4); leafViewKey = 'china' }
      return
    }
    const list = ((d.geo || {})[field] || [])
    const max = Math.max(1, ...list.map(i => i.count))
    leafMax.value = list.length ? Math.max(...list.map(i => i.count || 0)) : 0
    const byName = {}
    list.forEach(i => { byName[i.province] = i.count || 0 })
    addGeo(chinaGeo, byName, max, false)
    if (leafViewKey !== 'china') { leafMap.setView([35, 104], 4); leafViewKey = 'china' }
  }

  // 精确落点（半径固定像素，与「无底图」模式观感一致；不随缩放变化）
  if (precise.value && points.value.length) {
    points.value.forEach(p => {
      const lat = Number(p.lat), lng = Number(p.lng)
      if (!isFinite(lat) || !isFinite(lng)) return
      const n = Number(p.count) || 1
      L.circleMarker([lat, lng], { radius: 16 + Math.min(n, 6) * 5, stroke: false, fillColor: '#c0392b', fillOpacity: 0.16, interactive: false }).addTo(leafPts)
      L.circleMarker([lat, lng], { radius: 4, color: '#ffffff', weight: 1, fillColor: '#8e2b20', fillOpacity: 1, interactive: false }).addTo(leafPts)
      if (p.name) {
        L.marker([lat, lng], { interactive: false, icon: L.divIcon({ className: 'hm-leaf-name', html: escapeHtml(p.name), iconSize: [0, 0], iconAnchor: [-7, 3] }) }).addTo(leafPts)
      }
    })
  }
}

// Leaflet 侧的类别微错开：按屏幕像素偏移（保持「位置不变、只是错开一点」的观感）
function leafOffsetLatLng(lat, lng, off) {
  if (!off || (!off[0] && !off[1]) || !leafMap) return [lat, lng]
  const p = leafMap.latLngToContainerPoint(L.latLng(lat, lng))
  return leafMap.containerPointToLatLng(L.point(p.x + off[0], p.y + off[1]))
}

// 「全部」模式（Leaflet 真地图）：三类 + 未迁出落回，分色画点，颜色与图例一致。
// 图例关掉的类别在这里就被 activeCatPoints() 剔除 —— 不生成任何图层（不是改透明度）。
function addAllLeafPoints() {
  if (!(precise.value && points.value.length)) return
  activeCatPoints().forEach(b => {
    const color = catColor(b.key)
    const off = catOffsetOf(b.key)
    b.points.forEach(p => {
      const lat = Number(p.lat), lng = Number(p.lng)
      if (!isFinite(lat) || !isFinite(lng)) return
      const pos = leafOffsetLatLng(lat, lng, off)
      const n = Number(p.count) || 1
      L.circleMarker(pos, { radius: 16 + Math.min(n, 6) * 5, stroke: false, fillColor: color, fillOpacity: 0.16, interactive: false }).addTo(leafPts)
      L.circleMarker(pos, {
        radius: 5, color: '#ffffff', weight: 1, fillColor: color, fillOpacity: 1,
        dashArray: b.key === 'fallback' ? '3,2' : undefined
      }).addTo(leafPts)
      if (p.name) {
        L.marker(pos, { interactive: false, icon: L.divIcon({ className: 'hm-leaf-name', html: escapeHtml(p.name), iconSize: [0, 0], iconAnchor: [-7, 3] }) }).addTo(leafPts)
      }
    })
  })
}

// 迁徙轨迹（Leaflet 真地图）：按经纬度画落点级连线 + 两端标记
function renderLeafMigration() {
  stopMigAnim()
  leafPoly.clearLayers()
  leafPts.clearLayers()
  if (leafArrows) leafArrows.clearLayers()
  migArrowSpecs = []
  leafMax.value = 0
  const list = migList.value.map(i => {
    const f = i.from_province || i.from
    const t = i.to_province || i.to
    const hasFrom = !!(i.from_lng && i.from_lat)
    const hasTo = !!(i.to_lng && i.to_lat)
    const fc = hasFrom ? [i.from_lng, i.from_lat] : centers[f]
    const tc = hasTo ? [i.to_lng, i.to_lat] : centers[t]
    if (!fc || !tc) return null
    return { i, f, t, fc, tc, exactFrom: hasFrom, exactTo: hasTo }
  }).filter(Boolean)
  if (!list.length) return
  const max = Math.max(1, ...list.map(x => x.i.count))
  const all = []
  list.forEach(x => {
    const a = [x.fc[1], x.fc[0]], b = [x.tc[1], x.tc[0]] // Leaflet 用 [lat, lng]
    const same = Math.abs(a[0] - b[0]) < 1e-4 && Math.abs(a[1] - b[1]) < 1e-4
    const i0 = x.i
    const tip = `${escapeHtml(i0.from)} → ${escapeHtml(i0.to)}<br/>约 ${i0.count} 人` +
      (i0.era ? `<br/>${escapeHtml(i0.era)}` : '') +
      (i0.year ? `（${i0.year}）` : '') +
      (i0.description ? `<br/>${escapeHtml(i0.description)}` : '')
    if (!same) {
      const cs = arcCoords(a, b)
      L.polyline(cs, { color: '#c0392b', weight: 1 + 2.5 * (x.i.count / max), opacity: 0.8, className: 'hm-mig-path' })
        .bindTooltip(tip, { sticky: true }).addTo(leafPoly)
      // 行走的人：按人数决定「一组几个小人 + 颜色」，沿轨迹从迁出地走到迁入地
      const ci = migCountInfo(i0.count)
      const spec = { cs, head: 0, walkers: [] }
      for (let k = 0; k < ci.figures; k++) {
        spec.walkers.push(L.marker(a, { interactive: false, zIndexOffset: 900, icon: migWalkerIcon(ci.color) }).addTo(leafPts))
      }
      migArrowSpecs.push(spec)
    }
    ;[[a, i0.from, x.exactFrom], [b, i0.to, x.exactTo]].forEach(([p, nm, exact]) => {
      L.circleMarker(p, { radius: exact ? 5 : 4, color: '#ffffff', weight: 1, fillColor: '#8e2b20', fillOpacity: 1, interactive: false }).addTo(leafPts)
      if (nm) L.marker(p, { interactive: false, icon: L.divIcon({ className: 'hm-leaf-name', html: escapeHtml(nm), iconSize: [0, 0], iconAnchor: [-7, 3] }) }).addTo(leafPts)
      all.push(p)
    })
  })
  if (!all.length) return
  drawMigArrows()
  applyMigWalkerScale()
  startMigAnim(migArrowSpecs)
  const b = L.latLngBounds(all)
  if (b.isValid()) {
    const key = 'mig:' + all.map(p => p[0].toFixed(3) + ',' + p[1].toFixed(3)).join(';')
    if (key !== leafViewKey) {
      leafMap.fitBounds(b, { padding: [40, 40], maxZoom: 13 })
      leafViewKey = key
    }
  }
}

// 创建／重建每段轨迹的箭头流（缩放后间距变化，重算）
function drawMigArrows() {
  if (!leafArrows || !leafMap) return
  leafArrows.clearLayers()
  migArrowSpecs.forEach(sp => {
    const cs = sp.cs
    if (!cs || cs.length < 2) { sp.markers = []; sp.total = 0; return }
    const cum = [0]
    for (let i = 1; i < cs.length; i++) {
      const p0 = leafMap.latLngToContainerPoint(cs[i - 1])
      const p1 = leafMap.latLngToContainerPoint(cs[i])
      cum.push(cum[i - 1] + p0.distanceTo(p1))
    }
    sp.cum = cum
    sp.total = cum[cum.length - 1]
    if (!(sp.total > 6)) { sp.markers = []; return }
    const mks = []
    for (let k = 0; k < MIG_ARROW_N; k++) {
      mks.push(L.marker(cs[0], { interactive: false, icon: migArrowIcon(0) }).addTo(leafArrows))
    }
    sp.markers = mks
    if (!sp.head || sp.head > sp.total) sp.head = 0
  })
}

// 人数 → 分档配色 + 小人个数（base = 10^floor(log10 n)，不足一档按一档算）
const MIG_COLOR_SCALE = [
  { max: 9, color: '#2e7d32' },        // 个
  { max: 99, color: '#1565c0' },       // 十
  { max: 999, color: '#ef6c00' },      // 百
  { max: 9999, color: '#7b1fa2' },     // 千
  { max: Infinity, color: '#c62828' }  // 万及以上
]
function migCountInfo(n) {
  n = Math.max(1, Math.round(Number(n) || 1))
  const base = Math.pow(10, Math.floor(Math.log10(n)))
  const figures = Math.max(1, Math.min(10, Math.ceil(n / base)))
  let color = MIG_COLOR_SCALE[MIG_COLOR_SCALE.length - 1].color
  for (const b of MIG_COLOR_SCALE) { if (n <= b.max) { color = b.color; break } }
  return { base, figures, color }
}
// 单个 SVG 小人（四肢摆动 = 行走动画）
function migWalkerSVG(color) {
  return '<svg viewBox="0 0 22 30" width="22" height="30">' +
    '<circle cx="11" cy="4.6" r="3.6" fill="' + color + '"/>' +
    '<line x1="11" y1="8.4" x2="11" y2="17" stroke="' + color + '" stroke-width="2.8" stroke-linecap="round"/>' +
    '<line class="mw-limb mw-leg-a" x1="11" y1="17" x2="11" y2="27.5" stroke="' + color + '" stroke-width="2.8" stroke-linecap="round"/>' +
    '<line class="mw-limb mw-leg-b" x1="11" y1="17" x2="11" y2="27.5" stroke="' + color + '" stroke-width="2.8" stroke-linecap="round"/>' +
    '<line class="mw-limb mw-arm-a" x1="11" y1="10.5" x2="11" y2="16.5" stroke="' + color + '" stroke-width="2.2" stroke-linecap="round"/>' +
    '<line class="mw-limb mw-arm-b" x1="11" y1="10.5" x2="11" y2="16.5" stroke="' + color + '" stroke-width="2.2" stroke-linecap="round"/>' +
    '</svg>'
}
// 单个小人图标（一队人沿轨迹前后排队走）
function migWalkerIcon(color) {
  return L.divIcon({ className: 'hm-mig-walker', html: migWalkerSVG(color), iconSize: [22, 30], iconAnchor: [11, 28] })
}

// 方向箭头图标（三角形，deg 为地理方位角，0 = 正北）
function migArrowIcon(deg) {
  return L.divIcon({
    className: 'hm-mig-arrow',
    html: `<i style="transform:rotate(${deg.toFixed(1)}deg)"></i>`,
    iconSize: [12, 12], iconAnchor: [6, 6]
  })
}

// a→b 的地理方位角（度，0=北，顺时针）
function bearing(a, b) {
  const φ1 = a[0] * Math.PI / 180, φ2 = b[0] * Math.PI / 180, Δλ = (b[1] - a[1]) * Math.PI / 180
  const y = Math.sin(Δλ) * Math.cos(φ2)
  const x = Math.cos(φ1) * Math.sin(φ2) - Math.sin(φ1) * Math.cos(φ2) * Math.cos(Δλ)
  return (Math.atan2(y, x) * 180 / Math.PI + 360) % 360
}

// 两点间画一条微弧（更像轨迹，bend 为弧顶偏移比例）
function arcCoords(a, b, n = 32, bend = 0.18) {
  const mLat = (a[0] + b[0]) / 2, mLng = (a[1] + b[1]) / 2
  const dLat = b[0] - a[0], dLng = b[1] - a[1]
  const cLat = mLat - dLng * bend, cLng = mLng + dLat * bend
  const out = []
  for (let i = 0; i <= n; i++) {
    const t = i / n
    out.push([
      (1 - t) * (1 - t) * a[0] + 2 * (1 - t) * t * cLat + t * t * b[0],
      (1 - t) * (1 - t) * a[1] + 2 * (1 - t) * t * cLng + t * t * b[1]
    ])
  }
  return out
}

// ECharts 侧的底图样式只在「无底图」模式用到（选了底图时画布是隐藏的）
function mapAreaStyle() {
  return { areaColor: '#f4f6fa', borderColor: '#ffffff', borderWidth: 0.6 }
}
function mapLabelShow() { return true }

function baseMapSeries(extra = {}) {
  return {
    type: 'map', map: 'china', roam: 'move', zoom: mapZoom.value, top: 24, bottom: 24,
    label: { show: mapLabelShow(), fontSize: 8, color: '#4a5568' },
    itemStyle: mapAreaStyle(),
    emphasis: { label: { show: true, color: '#1b3f77' }, itemStyle: { areaColor: 'rgba(247,201,72,0.75)' } },
    select: { disabled: true },
    ...extra
  }
}

function renderGeo(d) {
  // 选了离线底图 → 整张地图交给 Leaflet
  if (leafActive()) { nextTick(renderLeaf); return }
  const field = geoField.value
  if (field === 'all') { renderGeoAll(d); return }
  const label = field === 'burial' ? '葬地' : (field === 'current' ? '现居地' : '籍贯')

  // 已下钻到某省：按城市着色
  if (drill.prov && cityGeo) {
    const cities = ((d.city || {})[field] || {})[drill.prov] || []
    const max = Math.max(1, ...cities.map(i => i.count))
    chart.clear()
    chart.setOption({
      tooltip: {
        trigger: 'item',
        formatter: p => `${p.name}<br/>${label}：${p.value || 0} 人${p.value ? '<br/><span style="color:#999">点击查看明细</span>' : ''}`
      },
      visualMap: {
        min: 0, max, left: 20, bottom: 24, text: ['多', '少'], calculable: true,
        inRange: { color: ['#eaf3ff', '#a9cbea', '#4b86c4', '#1b3f77'] }
      },
      series: [{
        type: 'map', map: drill.mapName, roam: 'move', zoom: mapZoom.value, top: 24, bottom: 24,
        label: { show: mapLabelShow(), fontSize: 9, color: '#4a5568' },
        itemStyle: mapAreaStyle(),
        emphasis: { label: { show: true, color: '#1b3f77' }, itemStyle: { areaColor: 'rgba(247,201,72,0.75)' } },
        select: { disabled: true },
        data: cities.map(i => ({ name: i.name, value: i.count }))
      }]
    }, true)
    return
  }

  const list = ((d.geo || {})[field] || [])
  const max = Math.max(1, ...list.map(i => i.count))
  const useOverlay = precise.value && points.value.length > 0
  chart.clear()
  chart.setOption({
    tooltip: {
      trigger: 'item',
      formatter: p => `${p.name}<br/>${label}：${p.value || 0} 人${p.value ? '<br/><span style="color:#999">点击查看明细</span>' : ''}`
    },
    visualMap: {
      show: !useOverlay, min: 0, max, left: 20, bottom: 24, text: ['多', '少'], calculable: true,
      inRange: { color: ['#eaf3ff', '#a9cbea', '#4b86c4', '#1b3f77'] }
    },
    series: [baseMapSeries({ data: list.map(i => ({ name: i.province, value: i.count })) })]
  }, true)
  // 精确落点：直接用「地图 series 自己的投影」把经纬度换算成像素再画，
  // 不再另建 geo 坐标系（两套布局容易对不上，点会跑到地图外）
  if (useOverlay) drawPoints()
}

// colorHexToRgba #1f6fd0 → rgba(31,111,208,alpha)
function hexA(hex, a) {
  const s = String(hex || '#888').replace('#', '')
  const v = s.length === 3 ? s.split('').map(c => c + c).join('') : s
  const n = parseInt(v, 16)
  return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${a})`
}

// 「全部」模式（无底图 / ECharts）：
// - 开了精确热力：三类（+未迁出落回）落点分色 + 图例
// - 没开精确热力：三个 map series 叠在同一张省界图上，按类别分色（图例给类别与人数）
function renderGeoAll(d) {
  const hasPoints = precise.value && points.value.length > 0
  const active = cats.value.filter(c => catEnabled(c.key))
  chart.clear()

  // 已下钻到某省：城市口径三类合一（同一个市可能同时有祖籍/现居地的人），
  // 明细抽屉里再用「类别」列区分，这里用中性色阶画城市总量。
  // 图例点选关掉的类别：既不进城市合计，也不进 tooltip 明细；关到没有可见数据时连色阶一起收掉。
  if (drill.prov && cityGeo) {
    const useCity = ['native', 'current', 'burial'].filter(f => catEnabled(f))
    const acc = {}
    const cityFieldAcc = {}
    for (const f of useCity) {
      const cities = ((d.city || {})[f] || {})[drill.prov] || []
      cities.forEach(i => {
        acc[i.name] = (acc[i.name] || 0) + (i.count || 0)
        cityFieldAcc[i.name] = cityFieldAcc[i.name] || {}
        cityFieldAcc[i.name][f] = (cityFieldAcc[i.name][f] || 0) + (i.count || 0)
      })
    }
    const names = Object.keys(acc)
    const max = Math.max(1, ...names.map(n => acc[n]))
    chart.setOption({
      tooltip: {
        trigger: 'item',
        formatter: p => {
          const det = cityFieldAcc[p.name] || {}
          const rows = useCity
            .filter(f => det[f])
            .map(f => `${catMeta(f).label}：${det[f]} 人`).join('<br/>')
          return `${p.name}<br/>${rows || '无数据'}${p.value ? '<br/><span style="color:#999">点击查看明细</span>' : ''}`
        }
      },
      visualMap: {
        show: !hasPoints && names.length > 0, min: 0, max, left: 20, bottom: 24, text: ['多', '少'], calculable: true,
        inRange: { color: ['#eaf3ff', '#a9cbea', '#4b86c4', '#1b3f77'] }
      },
      series: [{
        type: 'map', map: drill.mapName, roam: 'move', zoom: mapZoom.value, top: 24, bottom: 24,
        label: { show: mapLabelShow(), fontSize: 9, color: '#4a5568' },
        itemStyle: mapAreaStyle(),
        emphasis: { label: { show: true, color: '#1b3f77' }, itemStyle: { areaColor: 'rgba(247,201,72,0.75)' } },
        select: { disabled: true },
        data: names.map(n => ({ name: n, value: acc[n] }))
      }]
    }, true)
    if (hasPoints) drawPoints()
    return
  }

  if (hasPoints) {
    // 底图省界用中性灰，避免色带与「类别颜色」混淆
    chart.setOption({
      tooltip: { trigger: 'item', formatter: p => `${p.name}<br/>（背景为省界，落点见图中彩色圆点）` },
      series: [baseMapSeries({ data: [], itemStyle: { areaColor: '#f2f5f9', borderColor: '#e3e8ef', borderWidth: 0.6 } })]
    }, true)
    drawPoints()
    return
  }

  const series = active.map(c => baseMapSeries({
    name: catMeta(c.key).label,
    data: (c.list || []).map(i => ({ name: i.province, value: i.count })),
    itemStyle: { areaColor: catMeta(c.key).color, borderColor: '#ffffff', borderWidth: 0.4, opacity: 0.62 },
    emphasis: { label: { show: true, color: '#1b3f77' }, itemStyle: { areaColor: 'rgba(247,201,72,0.75)' } }
  }))
  chart.setOption({
    tooltip: {
      trigger: 'item',
      formatter: p => {
        let out = `${p.name}<br/><b>${p.seriesName}</b>：${p.value || 0} 人`
        if (p.value) out += '<br/><span style="color:#999">点击查看明细</span>'
        return out
      }
    },
    series: series.length ? series : [baseMapSeries({ data: [] })]
  }, true)
}

// 落点圆心的类别微错开（catItem: {points, key}）——只在「全部」模式用到
// kind 为 '' 时返回原点（单类模式行为完全不变）
function catOffsetOf(kind) { return (kind && CAT_OFFSET[kind]) ? CAT_OFFSET[kind] : [0, 0] }

// 「全部」模式的落点批次（仅保留开启的类别）
function activeCatPoints() {
  return catBatches.value
    .filter(b => catEnabled(b.key))
    .map(b => ({ key: b.key, points: b.points || [] }))
}

// 精确落点绘制（像素坐标，随地图缩放/平移实时重算）
// 「全部」模式下按类别分色（图例同色），并对同类重叠做屏幕像素级微错开。
function drawPoints() {
  if (!chart) return
  const isAllMode = isAll.value
  const batches = isAllMode ? activeCatPoints() : [{ key: '', points: points.value }]
  const g = []
  batches.forEach(b => {
    const off = catOffsetOf(b.key)
    const color = b.key ? catColor(b.key) : ''
    b.points.forEach(p => {
      const xy = chart.convertToPixel({ seriesIndex: 0 }, [Number(p.lng), Number(p.lat)])
      if (!xy || !isFinite(xy[0]) || !isFinite(xy[1])) return
      const cx = xy[0] + off[0], cy = xy[1] + off[1]
      const n = Number(p.count) || 1
      const r = 16 + Math.min(n, 6) * 5
      g.push({
        // 注意：graphic 圆要用绝对坐标（不设 left/top），设 left/top 会按包围盒定位 → 圆心偏移 r
        type: 'circle', zlevel: 2, silent: true,
        shape: { cx, cy, r },
        style: color
          ? {
              fill: {
                type: 'radial', x: 0.5, y: 0.5, r: 0.5,
                colorStops: [
                  { offset: 0, color: hexA(color, 0.34) },
                  { offset: 0.55, color: hexA(color, 0.14) },
                  { offset: 1, color: hexA(color, 0) }
                ]
              }
            }
          : {
              fill: {
                type: 'radial', x: 0.5, y: 0.5, r: 0.5,
                colorStops: [
                  { offset: 0, color: 'rgba(192,57,43,0.55)' },
                  { offset: 0.55, color: 'rgba(240,123,40,0.32)' },
                  { offset: 1, color: 'rgba(255,224,138,0)' }
                ]
              }
            }
      })
      g.push({
        type: 'circle', zlevel: 3, silent: true,
        shape: { cx, cy, r: isAllMode ? 5 : 4 },
        style: { fill: color || '#8e2b20', stroke: '#fff', lineWidth: 1 }
      })
      if (p.name) {
        g.push({
          type: 'text', left: cx + 9, top: cy - 16, zlevel: 3, silent: true,
          style: {
            text: String(p.name || ''), fill: color || '#333', fontSize: 9,
            backgroundColor: 'rgba(255,255,255,0.85)', padding: [2, 4]
          }
        })
      }
    })
  })
  chart.setOption({ graphic: g })
}

function renderMigration(d) {
  // 已选底图（Leaflet）时交给 renderLeaf 画
  if (leafActive()) { nextTick(renderLeaf); return }
  // 优先用「迁出地/迁入地」解析出的经纬度画落点级轨迹（同省也能看到一条线）；
  // 没有坐标的退回省级中心。
  const list = migList.value.map(i => {
    const f = i.from_province || i.from
    const t = i.to_province || i.to
    const hasFrom = !!(i.from_lng && i.from_lat)
    const hasTo = !!(i.to_lng && i.to_lat)
    const fc = hasFrom ? [i.from_lng, i.from_lat] : centers[f]
    const tc = hasTo ? [i.to_lng, i.to_lat] : centers[t]
    if (!fc || !tc) return null
    return { i, f, t, fc, tc, exact: hasFrom && hasTo }
  }).filter(Boolean)
  chart.clear()
  if (!list.length) {
    chart.setOption({ title: { text: '暂无迁徙数据', left: 'center', top: 'middle', textStyle: { fontSize: 14, color: '#999', fontWeight: 'normal' } } }, true)
    return
  }
  const max = Math.max(1, ...list.map(x => x.i.count))
  // 落点聚合（同经纬度合并）
  const ptAcc = {}
  list.forEach(x => {
    [{ n: x.i.from, c: x.fc }, { n: x.i.to, c: x.tc }].forEach(o => {
      const k = o.c[0].toFixed(3) + ',' + o.c[1].toFixed(3)
      if (ptAcc[k]) ptAcc[k].count += x.i.count
      else ptAcc[k] = { name: o.n, coord: o.c, count: x.i.count }
    })
  })
  const pts = Object.values(ptAcc).map(o => ({ name: o.name, value: [...o.coord, o.count] }))
  const maxPt = Math.max(1, ...Object.values(ptAcc).map(o => o.count))
  const exactAll = list.every(x => x.exact) && pts.length > 1
  chart.setOption({
    title: legacyKey.value ? {
      text: '按「籍贯 → 葬地」推算（未录入迁徙记录）', left: 12, top: 6,
      textStyle: { fontSize: 12, color: '#b07d1a', fontWeight: 'normal' }
    } : undefined,
    tooltip: {
      trigger: 'item',
      formatter: p => {
        if (p.data && p.data.from) {
          const i = p.data.raw || {}
          return `${i.from} → ${i.to}<br/>约 ${i.count} 人${i.era ? '<br/>' + i.era : ''}${i.year ? '（' + i.year + '）' : ''}${i.description ? '<br/>' + i.description : ''}`
        }
        return `${p.name}<br/>关联 ${p.value ? p.value[2] : 0} 人`
      }
    },
    geo: {
      map: 'china', roam: true, zoom: 1.08, top: 24, bottom: 24,
      label: { show: true, fontSize: 8, color: '#6b7280' },
      itemStyle: { areaColor: '#f6f7fa', borderColor: '#dfe3ea' },
      emphasis: { itemStyle: { areaColor: '#e8f1ff' } }
    },
    series: [
      {
        type: 'lines', coordinateSystem: 'geo', zlevel: 2,
        effect: { show: true, period: 5, trailLength: 0.25, symbol: 'arrow', symbolSize: 7, color: '#c0392b' },
        data: list.map(x => ({
          coords: [x.fc, x.tc], from: x.f, to: x.t, count: x.i.count, raw: x.i,
          lineStyle: { width: 1.5 + 6 * (x.i.count / max), opacity: 0.75, curveness: 0.28, color: '#c0392b' }
        }))
      },
      {
        type: 'effectScatter', coordinateSystem: 'geo', zlevel: 3,
        symbolSize: v => 5 + 10 * Math.min(1, (v[2] || 0) / maxPt),
        rippleEffect: { brushType: 'stroke', scale: 2.4 },
        itemStyle: { color: '#8e2b20' },
        label: { show: true, position: 'right', fontSize: 9, color: '#333', formatter: '{b}' },
        data: pts
      }
    ]
  }, true)
  if (exactAll) fitMigrationGeo(list)
}

// 全部为落点级坐标时：把地图缩放到轨迹范围，避免同省轨迹在国界图上看不见
function fitMigrationGeo(list) {
  try {
    if (!chart || typeof chart.convertToPixel !== 'function') return
    const xs = [], ys = []
    list.forEach(x => { xs.push(x.fc[0], x.tc[0]); ys.push(x.fc[1], x.tc[1]) })
    const minX = Math.min(...xs), maxX = Math.max(...xs)
    const minY = Math.min(...ys), maxY = Math.max(...ys)
    const z0 = 1.08
    // 探测当前缩放下的「像素/度」（经纬度线性投影，缩放正比于像素/度）
    const p0 = chart.convertToPixel({ geoIndex: 0 }, [minX, minY])
    const p1 = chart.convertToPixel({ geoIndex: 0 }, [minX + 1, minY])
    const p2 = chart.convertToPixel({ geoIndex: 0 }, [minX, minY + 1])
    if (!p0 || !p1 || !p2) return
    const kx = Math.abs(p1[0] - p0[0])
    const ky = Math.abs(p2[1] - p0[1])
    if (!(kx > 0) || !(ky > 0)) return
    const W = chart.getWidth() * 0.6
    const H = Math.max(120, chart.getHeight() - 48) * 0.6
    const spanX = Math.max(maxX - minX, 0.03)
    const spanY = Math.max(maxY - minY, 0.03)
    let z = z0 * Math.min(W / (spanX * kx), H / (spanY * ky))
    z = Math.max(z0, Math.min(z, 50))
    chart.setOption({ geo: { center: [(minX + maxX) / 2, (minY + maxY) / 2], zoom: z } })
  } catch (e) { /* 保持默认视野 */ }
}

function renderBranch(d) {
  const list = d.branch || []
  chart.clear()
  if (!list.length) {
    chart.setOption({ title: { text: '暂无房支数据', left: 'center', top: 'middle', textStyle: { fontSize: 14, color: '#999', fontWeight: 'normal' } } }, true)
    return
  }
  chart.setOption({
    tooltip: { trigger: 'item', formatter: p => `${p.name}<br/>${p.value} 人` },
    series: [{
      type: 'treemap', roam: false, nodeClick: false, breadcrumb: { show: false },
      top: 16, left: 8, right: 8, bottom: 16,
      label: { show: true, formatter: p => `${p.name}\n${p.value} 人`, fontSize: 13, color: '#fff', lineHeight: 18 },
      upperLabel: { show: false },
      itemStyle: { borderColor: '#fff', borderWidth: 2, gapWidth: 2 },
      levels: [{ itemStyle: { borderWidth: 0, gapWidth: 3 }, color: ['#1b3f77', '#2f6fb0', '#4b86c4', '#7fb0dc', '#a9cbea', '#cfe3f7'] }],
      data: list.map(i => ({ name: i.name, value: i.count }))
    }]
  }, true)
}

function renderTimeline(d) {
  const birth = d.timeline.birth || {}
  const death = d.timeline.death || {}
  const years = Object.keys(birth).concat(Object.keys(death)).map(Number).filter(y => y >= 1000)
  chart.clear()
  if (!years.length) {
    chart.setOption({ title: { text: '暂无生卒年份数据', left: 'center', top: 'middle', textStyle: { fontSize: 14, color: '#999', fontWeight: 'normal' } } }, true)
    return
  }
  const minY = Math.min(...years)
  const maxY = Math.max(...years)
  const step = Math.max(1, Math.ceil((maxY - minY + 1) / 60))
  const buckets = []
  for (let y = minY; y <= maxY; y += step) buckets.push(y)
  const bucketOf = y => buckets[Math.floor((y - minY) / step)]
  const rows = ['出生', '去世']
  const acc = { 出生: {}, 去世: {} }
  Object.keys(birth).forEach(k => { const b = bucketOf(+k); if (b != null) acc['出生'][b] = (acc['出生'][b] || 0) + birth[k] })
  Object.keys(death).forEach(k => { const b = bucketOf(+k); if (b != null) acc['去世'][b] = (acc['去世'][b] || 0) + death[k] })
  const cells = []
  let max = 1
  rows.forEach((r, ri) => buckets.forEach((y, ci) => {
    const v = acc[r][y] || 0
    if (v > max) max = v
    cells.push([ci, ri, v])
  }))
  chart.setOption({
    tooltip: {
      trigger: 'item',
      formatter: p => {
        if (!p.data) return ''
        const y = buckets[p.data[0]]
        return `${rows[p.data[1]]} · ${y}${step > 1 ? '~' + (y + step - 1) : ''} 年<br/>${p.data[2]} 人`
      }
    },
    grid: { left: 64, right: 32, top: 40, bottom: 76 },
    xAxis: {
      type: 'category', data: buckets.map(String), name: '年份',
      nameGap: 26, axisLabel: { interval: Math.max(0, Math.ceil(buckets.length / 20)), rotate: 0 }
    },
    yAxis: { type: 'category', data: rows, name: '事件' },
    visualMap: {
      min: 0, max, calculable: true, orient: 'horizontal', left: 'center', bottom: 14,
      text: ['密集', '稀疏'], inRange: { color: ['#eaf3ff', '#7fb0dc', '#2f6fb0', '#12345e'] }
    },
    series: [{
      type: 'heatmap', data: cells,
      label: { show: buckets.length <= 40, fontSize: 9, color: '#12345e' },
      emphasis: { itemStyle: { shadowBlur: 8, shadowColor: 'rgba(0,0,0,0.25)' } }
    }]
  }, true)
}

function render() {
  stopMigAnim()
  if (!chart) return
  const d = data.value
  try { renderInner(d) } catch (e) { chart.clear(); renderFail.value = String((e && e.message) || e) }
}
function renderInner(d) {
  renderFail.value = ''
  if (tab.value === 'geo') renderGeo(d)
  else if (tab.value === 'migration') renderMigration(d)
  else if (tab.value === 'branch') renderBranch(d)
  else renderTimeline(d)
}

// ---------- 省市下钻 ----------
async function drillTo(provName) {
  const adcode = provAdcode[provName]
  if (!adcode) return
  const mapName = 'prov_' + adcode
  if (!loadedMaps[mapName]) {
    try {
      const geo = await fetch('/maps/' + adcode + '.json').then(r => r.json())
      if (!geo || !geo.features) throw new Error('bad geo')
      echarts.registerMap(mapName, geo)
      loadedMaps[mapName] = geo
    } catch (e) {
      ElMessage.warning('该地区暂无下级地图数据')
      return
    }
  }
  cityGeo = loadedMaps[mapName]
  drill.prov = provName
  drill.adcode = adcode
  drill.mapName = mapName
  mapZoom.value = ZOOM_PROV
  render()
}

function backToChina() {
  drill.prov = ''
  drill.adcode = ''
  cityGeo = null
  mapZoom.value = ZOOM_DEFAULT
  render()
}

async function openMembers(province, city) {
  const fld = geoField.value
  drawerField.value = fld
  drawerProv.value = province
  drawerCity.value = city || ''
  const catName = fld === 'all' ? '全部类别' : (fld === 'current' ? '现居地' : (fld === 'burial' ? '葬地' : '籍贯'))
  drawerTitle.value = city ? `${province} · ${city} · ${catName}` : `${province} · ${catName} · 成员明细`
  drawerLoading.value = true
  members.value = []
  drawer.value = true
  try {
    // field=all 用 all；其余按类别 key 传（fallback 归在 current 里，后端按 cats 白名单区分）
    const field = fld === 'all' ? 'all' : (fld === 'fallback' ? 'current' : fld)
    const params = { ...queryParams(), field, province }
    if (city) params.city = city
    const r = await api.get('/heatmap/members', { params })
    members.value = r.data.list || []
  } catch (e) {
    members.value = []
  } finally {
    drawerLoading.value = false
  }
}

// 图例点选后就地刷新已打开的明细抽屉（口径与地图一致：关掉的类别不出现）
async function refreshDrawer() {
  if (!drawer.value || !drawerProv.value) return
  await openMembers(drawerProv.value, drawerCity.value)
}

function onChartClick(p) {
  if (p.seriesType !== 'map' || !p.name) return
  if (drill.prov) {
    if (p.name !== drill.prov) openMembers(drill.prov, p.name)
  } else {
    drillTo(p.name)
  }
}
function onResize() { fitChartHeight(); if (leafActive() && leafMap) leafMap.invalidateSize() }
function onTabChange() {
  // 迁徙轨迹默认用在线瓦片（真地图）：没选底图时自动切到「在线底图」
  if (tab.value === 'migration' && baseMap.value === 'none') {
    baseMap.value = 'online'
    localStorage.setItem('hm_basemap', baseMap.value)
  }
  leafViewKey = ''
  refreshView()
}

// 【手机端】画布高度自适应：从画布顶部一直铺到屏幕底边（贴边）
function fitChartHeight() {
  const wrap = wrapEl.value
  if (!wrap) return
  if (window.innerWidth <= 768) {
    const top = wrap.getBoundingClientRect().top
    wrap.style.height = Math.max(240, window.innerHeight - top - 4) + 'px'
  } else {
    wrap.style.height = ''
  }
  nextTick(() => {
    if (chart && chartEl.value && chartEl.value.offsetParent !== null) chart.resize()
    if (leafMap && leafActive()) leafMap.invalidateSize()
  })
}

function goMember(row) {
  if (row && row.id) router.push('/member/' + row.id)
}

// ---------- 精确热力 / 地理编码 ----------
async function loadPoints(loop = false) {
  if (!precise.value) { pointsVisibleMembers.value = 0; return }
  pointsLoading.value = true
  let rounds = 0
  try {
    do {
      const r = await api.get('/heatmap/points', { params: { ...queryParams(), field: geoField.value, limit: 50 } })
      const d = r.data || {}
      pointsConfigured.value = !!d.configured
      points.value = d.points || []
      // 「全部」模式：三类（+未迁出落回）分色落点，kind/fallback 由后端标注
      pointsAll.value = Array.isArray(d.points_by_field) ? d.points_by_field : []
      if (isAll.value) points.value = pointsAll.value.flatMap(b => b.points || [])
      pointsVisibleMembers.value = d.visible_members || 0
      pointsPending.value = d.pending || 0
      pointsFailed.value = d.failed || 0
      pointsTotalPlaces.value = d.total_places || 0
      pointsHint.value = d.hint || ''
      rounds++
      if (loop && pointsPending.value > 0 && pointsFailed.value === 0 && rounds < 10) render()
    } while (loop && pointsPending.value > 0 && pointsFailed.value === 0 && rounds < 10)
  } catch (e) {
    pointsConfigured.value = false
    pointsAll.value = []
    pointsVisibleMembers.value = 0
  } finally {
    pointsLoading.value = false
  }
}

async function onPreciseChange() {
  if (!precise.value) { render(); return }
  await loadPoints(true)
  render()
  if (!pointsConfigured.value) {
    ElMessage.info('尚未配置地理编码，可在「⚙️ 地理编码设置」中填写高德或百度 Key')
  } else if (pointsFailed.value > 0) {
    ElMessage.warning(pointsHint.value || `有 ${pointsFailed.value} 个地址解析失败，请检查 Key 是否正确、对应服务是否已开通`)
  } else if (pointsPending.value > 0) {
    ElMessage.info(`已定位 ${points.value.length} 个点，还有 ${pointsPending.value} 个地名待解析，可再次勾选或稍后重试`)
  }
}

async function openSettings() {
  settingsDialog.value = true
  try {
    const r = await api.get('/geo/config')
    const d = r.data || {}
    geoConfig.provider = d.provider || 'none'
    geoConfig.key_set = !!d.key_set
    geoConfig.key_masked = d.key_masked || ''
    geoConfig.cached = d.cached || 0
    geoConfig.key = ''
  } catch (e) {
    ElMessage.error('读取配置失败：' + (e.response?.data?.error || e.message))
  }
}

async function saveSettings() {
  savingCfg.value = true
  try {
    await api.put('/geo/config', { provider: geoConfig.provider, key: geoConfig.key || '' })
    ElMessage.success('已保存')
    settingsDialog.value = false
    await openSettings()
    if (precise.value) { await loadPoints(true); render() }
  } catch (e) {
    ElMessage.error('保存失败：' + (e.response?.data?.error || e.message))
  } finally {
    savingCfg.value = false
  }
}

async function clearGeoCache() {
  try {
    await ElMessageBox.confirm('清空已缓存的坐标？下次会重新向服务商查询。', '确认', { type: 'warning' })
  } catch { return }
  try {
    await api.delete('/geo/cache')
    geoConfig.cached = 0
    points.value = []
    pointsConfigured.value = false
    ElMessage.success('坐标缓存已清空')
    render()
  } catch (e) {
    ElMessage.error('清空失败：' + (e.response?.data?.error || e.message))
  }
}

// ---------- 在线底图配置（天地图浏览器端 Key，服务端统一保存）----------

async function loadOnlineConfig() {
  try {
    const { data } = await api.get('/map/online-config')
    onlineSource.value = data.source || 'tianditu'
    onlineTk.value = data.tk || ''
    onlineTkSet.value = !!data.tk_set
    onlineLayer.value = data.layer || 'vec'
    onlineTkMasked.value = data.tk_masked || ''
    onlineServerTkSet.value = !!data.server_tk_set
    onlineServerTkMasked.value = data.server_tk_masked || ''
    onlineProxy.value = !!data.proxy
    proxyQuotaOut.value = !!data.quota_exhausted
  } catch (e) { /* 忽略：无权限或未配置 */ }
}

function openOnlineSettings() {
  onlineForm.source = onlineSource.value || 'tianditu'
  onlineForm.tk = ''
  onlineForm.server_tk = ''
  onlineForm.layer = onlineLayer.value || 'vec'
  onlineForm.proxy = onlineProxy.value
  onlineDialog.value = true
}

async function saveOnline() {
  savingOnline.value = true
  try {
    const body = { source: onlineForm.source, layer: onlineForm.layer, proxy: !!onlineForm.proxy }
    if (onlineForm.tk) body.tk = onlineForm.tk
    if (onlineForm.server_tk) body.server_tk = onlineForm.server_tk
    await api.put('/map/online-config', body)
    ElMessage.success('已保存')
    onlineDialog.value = false
    await loadOnlineConfig()
    refreshView()
  } catch (e) {
    ElMessage.error('保存失败：' + (e.response?.data?.error || e.message))
  } finally {
    savingOnline.value = false
  }
}

async function clearOnline() {
  try {
    await ElMessageBox.confirm('清除已配置的在线底图 Key？', '确认', { type: 'warning' })
  } catch { return }
  try {
    const body = onlineProxy.value ? { clear_server: true } : { clear: true }
    await api.put('/map/online-config', body)
    await loadOnlineConfig()
    ElMessage.success('已清除')
    refreshView()
  } catch (e) {
    ElMessage.error('清除失败：' + (e.response?.data?.error || e.message))
  }
}

onMounted(async () => {
  const el = chartEl.value
  // 注意：zrender 把 touch/pointer 监听挂在同一个 dom 上，且注册早于我们，
  // 所以节流监听必须在 echarts.init 之前注册，才能在最前面拦截。
  el.addEventListener('wheel', onWheelZoom, { capture: true, passive: false })
  el.addEventListener('touchstart', onTouchStartZoom, { capture: true, passive: true })
  el.addEventListener('touchmove', onTouchMoveZoom, { capture: true, passive: false })
  el.addEventListener('touchend', onTouchEndZoom, { capture: true, passive: true })
  el.addEventListener('touchcancel', onTouchEndZoom, { capture: true, passive: true })
  await initMap()
  chart = echarts.init(el)
  await loadOnlineConfig()
  await loadPacks()
  // 地图缩放/平移后重算精确落点位置
  chart.on('georoam', () => { if (precise.value && points.value.length) drawPoints() })
  chart.on('click', onChartClick)
  window.addEventListener('resize', onResize)
  window.addEventListener('orientationchange', onResize)
  // 双指缩放/滚轮缩放节流：echarts 每次 pinch 事件固定 ×1.1，手指微动就会放大缩小一大截，
  // 这里在捕获阶段丢掉过密的事件，降低灵敏度（不支持灵敏度配置项）
  await load()
  // 首次进入：等两帧让布局（页签栏/筛选栏/提示条）稳定后再完整刷一次，
  // 否则会出现「第一次进来空白、切换一次才显示」。
  requestAnimationFrame(() => requestAnimationFrame(() => refreshView()))
})

function clampZoom(z) { return Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, z)) }

let zoomRaf = 0, zoomPending = null
// 一帧最多应用一次，画面更顺。
// 关键：用 dispatchAction('geoRoam') 走 echarts 的 updateTransform 快速通道
// （只改视图变换，不重绘地图多边形）；早先用 chart.setOption({series:[{zoom}]})
// 会触发完整更新管线、每帧重绘整张地图 → 手机上地图缩放明显卡顿。
function applyZoom(z, originX, originY) {
  zoomPending = { z: clampZoom(z), ox: originX, oy: originY }
  if (zoomRaf) return
  zoomRaf = requestAnimationFrame(() => {
    zoomRaf = 0
    const p = zoomPending
    zoomPending = null
    if (!p || !chart) return
    if (Math.abs(p.z - mapZoom.value) < 1e-4) return
    const factor = p.z / mapZoom.value   // geoRoam 的 zoom 是「相对倍数」
    mapZoom.value = p.z
    chart.dispatchAction({
      type: 'geoRoam',
      componentType: 'series',
      seriesIndex: 0,
      zoom: factor,
      originX: p.ox,
      originY: p.oy
    })
    if (precise.value && points.value.length) {
      drawPoints()
      requestAnimationFrame(() => { if (precise.value && points.value.length) drawPoints() })
    }
  })
}
function elOrigin(clientX, clientY) {
  const r = chartEl.value ? chartEl.value.getBoundingClientRect() : { left: 0, top: 0 }
  return [clientX - r.left, clientY - r.top]
}

// 【电脑端】滚轮 / 触控板：按滚动量比例缩放
function onWheelZoom(e) {
  if (tab.value !== 'geo') return
  e.preventDefault()
  e.stopPropagation()
  const dy = e.deltaMode === 1 ? e.deltaY * 16 : e.deltaY
  if (!dy) return
  const [ox, oy] = elOrigin(e.clientX, e.clientY)
  applyZoom(mapZoom.value * Math.pow(WHEEL_SENS, -dy), ox, oy)
}

// 【手机端】双指捏合：按「两指距离比」缩放（跟手），不再用 echarts 固定每事件 ×1.1
let pinchStart = null
function pinchMid(e) {
  return [(e.touches[0].clientX + e.touches[1].clientX) / 2, (e.touches[0].clientY + e.touches[1].clientY) / 2]
}
function onTouchStartZoom(e) {
  if (tab.value !== 'geo') return
  if (e.touches && e.touches.length >= 2) {
    const a = e.touches[0], b = e.touches[1]
    pinchStart = {
      d0: Math.max(1, Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY)),
      z0: mapZoom.value
    }
  }
}
function onTouchMoveZoom(e) {
  if (tab.value !== 'geo' || !pinchStart) return
  if (!e.touches || e.touches.length < 2) return
  e.preventDefault()
  e.stopPropagation()
  e.stopImmediatePropagation()
  const d = Math.hypot(e.touches[0].clientX - e.touches[1].clientX, e.touches[0].clientY - e.touches[1].clientY)
  const [ox, oy] = elOrigin(...pinchMid(e))
  applyZoom(pinchStart.z0 * Math.pow(d / pinchStart.d0, PINCH_SENS), ox, oy)
}
function onTouchEndZoom(e) {
  if (!e.touches || e.touches.length < 2) pinchStart = null
}

onBeforeUnmount(() => {
  stopMigAnim()
  window.removeEventListener('resize', onResize)
  window.removeEventListener('orientationchange', onResize)
  if (chartEl.value) {
    chartEl.value.removeEventListener('wheel', onWheelZoom, { capture: true })
    chartEl.value.removeEventListener('touchstart', onTouchStartZoom, { capture: true })
    chartEl.value.removeEventListener('touchmove', onTouchMoveZoom, { capture: true })
    chartEl.value.removeEventListener('touchend', onTouchEndZoom, { capture: true })
    chartEl.value.removeEventListener('touchcancel', onTouchEndZoom, { capture: true })
  }
  if (chart) { chart.dispose(); chart = null }
})
</script>

<style scoped>
.heatmap { padding: 4px 0 24px; }
.hm-bar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin: 8px 0 4px; }
.hm-bar2 { margin: 0 0 4px; }
.hm-lbl { color: #6b7280; font-size: 13px; }
.hm-stat { color: #6b7280; font-size: 13px; margin-left: auto; }
.hm-cfg { color: #6b7280; font-size: 12px; }
.hm-cfg-err { color: #c0392b; font-size: 12px; }
.hm-tabs { margin-top: 6px; }
.hm-hint { color: #b07d1a; background: #fffaf0; border: 1px solid #ffe3a3; border-radius: 6px; padding: 6px 12px; font-size: 13px; margin: 4px 0 8px; }
.hm-chart-wrap { position: relative; height: 620px; }
.hm-chart { position: relative; z-index: 1; width: 100%; height: 100%; }
.hm-leaf { position: absolute; inset: 0; z-index: 1; }
.hm-leaf-legend { position: absolute; left: 10px; bottom: 18px; z-index: 600; display: flex; flex-direction: column; align-items: center; gap: 2px; pointer-events: none; }
.hm-lg-bar { width: 12px; height: 74px; border-radius: 3px; background: linear-gradient(to top, #eaf3ff, #a9cbea, #4b86c4, #1b3f77); box-shadow: 0 0 0 1px rgba(0, 0, 0, .06); }
.hm-lg-lbl { font-size: 10px; color: #6b7280; }
.hm-lg-max { font-size: 10px; color: #374151; }
.hm-leaf-attr { position: absolute; right: 6px; bottom: 6px; z-index: 600; font-size: 11px; color: #57606a; background: rgba(255, 255, 255, .75); padding: 0 4px; border-radius: 3px; pointer-events: none; }
.hm-leaf-err { position: absolute; left: 50%; top: 12px; transform: translateX(-50%); z-index: 600; font-size: 12px; line-height: 1.4; color: #8a5a00; background: rgba(255, 247, 230, .96); border: 1px solid #f0d9a8; border-radius: 14px; padding: 4px 12px; pointer-events: none; box-shadow: 0 1px 4px rgba(0, 0, 0, .08); }
/* 「全部」模式图例（颜色 → 类别 + 人数，可点选开关） */
.hm-cat-legend { position: absolute; right: 10px; top: 10px; z-index: 650; background: rgba(255, 255, 255, .94); border: 1px solid #e3e8ef; border-radius: 8px; padding: 6px 10px 7px; box-shadow: 0 2px 8px rgba(0, 0, 0, .08); font-size: 12px; cursor: pointer; user-select: none; }
.hm-cl-title { display: flex; align-items: baseline; gap: 6px; font-weight: 600; color: #374151; margin-bottom: 4px; }
.hm-cl-note { font-weight: normal; font-size: 11px; color: #9aa2ad; }
.hm-cl-row { display: flex; align-items: center; gap: 6px; padding: 1px 2px; border-radius: 4px; }
.hm-cl-row:hover { background: #f3f6fb; }
.hm-cl-row.off { opacity: .38; text-decoration: line-through; }
.hm-cl-dot { display: inline-block; width: 11px; height: 11px; box-shadow: 0 0 0 1px rgba(0, 0, 0, .12); }
.hm-cl-name { color: #4a5568; }
.hm-cl-num { margin-left: auto; color: #1b3f77; }
.hm-cl-tip { color: #9aa2ad; font-size: 11px; margin-top: 3px; }
.hm-loading { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; color: #6b7280; background: rgba(255, 255, 255, 0.6); font-size: 14px; }
.hm-empty-sm { color: #9aa2ad; font-size: 13px; padding: 12px 4px; }
.hm-tip { color: #9aa2ad; font-size: 12px; padding: 10px 4px; }
.hm-cfg-hint { color: #9aa2ad; font-size: 12px; line-height: 1.7; }
@media (max-width: 768px) {
  .heatmap { padding-bottom: 0; }
  .hm-chart-wrap { height: 420px; } /* 会被 fitChartHeight() 按可视高度覆盖 */
  .hm-stat { margin-left: 0; }
}
</style>

<style>
/* Leaflet 生成的内容不在 scoped 作用域内，样式写这里 */
.leaflet-container { background: #eef3f7; font: inherit; }
.hm-leaf-name { background: rgba(255, 255, 255, 0.85); color: #333; font-size: 10px; line-height: 14px; padding: 0 3px; border-radius: 2px; white-space: nowrap; }
/* 迁徙轨迹：流动虚线 */
.hm-mig-path { stroke-dasharray: 10 7; animation: hm-dash 1.2s linear infinite; }
@keyframes hm-dash { to { stroke-dashoffset: -17; } }
/* 迁徙方向箭头 */
.hm-mig-arrow i { display: block; width: 0; height: 0; border-left: 6px solid transparent; border-right: 6px solid transparent; border-bottom: 11px solid #c0392b; opacity: .95; filter: drop-shadow(0 0 1px rgba(255, 255, 255, .95)); }
/* 迁徙「行走的人」：SVG 小人 + 四肢摆动动画 */
.hm-mig-walker svg { display: block; filter: drop-shadow(0 0 1.5px #fff) drop-shadow(0 0 1.5px #fff); transform-origin: 11px 28px; transform: scale(var(--mws, 1)); }
.hm-mig-walker .mw-limb { transform-box: fill-box; transform-origin: 50% 0; }
.hm-mig-walker .mw-leg-a { animation: hm-swing-a .58s ease-in-out infinite alternate; }
.hm-mig-walker .mw-leg-b { animation: hm-swing-b .58s ease-in-out infinite alternate; }
.hm-mig-walker .mw-arm-a { animation: hm-swing-arm-a .58s ease-in-out infinite alternate; }
.hm-mig-walker .mw-arm-b { animation: hm-swing-arm-b .58s ease-in-out infinite alternate; }
@keyframes hm-swing-a { from { transform: rotate(26deg) } to { transform: rotate(-26deg) } }
@keyframes hm-swing-b { from { transform: rotate(-26deg) } to { transform: rotate(26deg) } }
@keyframes hm-swing-arm-a { from { transform: rotate(-19deg) } to { transform: rotate(19deg) } }
@keyframes hm-swing-arm-b { from { transform: rotate(19deg) } to { transform: rotate(-19deg) } }
</style>
