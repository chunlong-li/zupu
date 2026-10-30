<template>
  <div class="demo-root" ref="demoRoot">
    <!-- 工具栏（打印时隐藏） -->
    <div class="toolbar no-print">
      <el-button class="no-print" size="small" @click="$router.push('/branches')">← 返回房支管理</el-button>
      <span class="serif title">支谱 · 竖排牒记式</span>
      <el-tag size="small" effect="plain" class="scope-tag">{{ branchCtx.includeChildren ? '含下级支' : '仅本支' }}</el-tag>
      <span class="lbl">修次</span>
      <el-select v-model="xiuci" size="small" class="no-print" style="width:130px" filterable allow-create default-first-option @change="saveXiuci">
        <el-option v-for="o in XIUCI_OPTS" :key="o" :label="o" :value="o" />
      </el-select>
      <el-input v-model="xiuci" size="small" class="no-print" style="width:110px" placeholder="自定义修次" @change="saveXiuci" />
      <el-button size="small" class="no-print" @click="galleryOpen = true">图片展示（{{ galleryImageTotal }}）</el-button>
      <el-select v-model="fontKey" class="font-sel no-print" size="small" style="width: 150px">
        <el-option v-for="f in FONTS" :key="f.key" :label="f.label" :value="f.key" />
      </el-select>
      <el-select v-model="fontSize" class="font-sel no-print" size="small" style="width: 118px" @change="onSizeChange">
        <el-option v-for="o in SIZE_OPTS" :key="o.pt" :label="o.label" :value="o.pt" />
      </el-select>
      <el-button class="no-print" size="small" @click="pickFontFile">⬆ 上传字体</el-button>
      <el-button v-if="currentFontCustom" class="no-print" size="small" type="danger" plain @click="deleteCurrentFont">删除</el-button>
      <input ref="fontInput" type="file" accept=".ttf,.otf,.woff,.woff2"
             class="font-file-input" @change="onFontFile" />
      <el-button type="primary" @click="exportPDF">🖨️ 打印 / 导出 PDF</el-button>
      <span class="hint no-print">缩放</span>
      <el-button class="no-print" size="small" @click="zoomOut">－</el-button>
      <el-button class="no-print" size="small" @click="zoomReset"
                 :title="explicitManual ? '点击回到自动适应宽度' : '当前为自动适应宽度（不超出屏幕）'">{{ zoomLabel }}</el-button>
      <el-button class="no-print" size="small" @click="zoomIn">＋</el-button>
      <span class="hint">{{ familyTitle }} · 牒记式表格 · 每人一竖格 · 自上而下、自右而左</span>
    </div>

    <!-- 谱书：封面 + 一页页网格（sheet），flex 自右而左排，每表固定列数，排满续页，空格补框线 -->
    <div class="book" ref="book">
      <!-- 封面（譜名頁）：雙邊框、豎排書名、堂號郡望、歲次重修 -->
      <div class="cover">
        <div class="cover-inner">
          <div class="cover-tanghao" v-if="tanghao">{{ tanghao }}</div>
          <div class="cover-title">
            <span class="vtitle" :style="coverTitleStyle">{{ familyTitle }}</span>
          </div>
          <div class="cover-meta">
            <div class="meta-line" v-if="junwang"><span>郡望　{{ junwang }}</span></div>
            <div class="meta-line" v-if="zuji"><span>祖籍　{{ zuji }}</span></div>
            <div class="meta-line"><span>歲次{{ currentYearGanzhi }}　{{ xiuci }}</span></div>
          </div>
        </div>
      </div>

      <!-- 內封（書名頁）：封面之後、目錄之前。竖排書名 + 雙線邊框 + 右下「X堂藏板」（無堂號則不顯示）。
           取舍（三本谱书一致）：與封面/目錄同屬「卷首」，不進目錄、不佔正文頁碼。 -->
      <div class="cover inner-cover">
        <div class="cover-inner it-host">
          <InnerTitlePage :title="familyTitle" :tanghao="tanghao" :year-label="'歲次' + currentYearGanzhi" :xiuci="xiuci"
                          :unit="8.8" :page-h="1058" />
        </div>
      </div>

      <!-- 目錄頁：封面之後、譜頭之前（傳統版式：封面→目錄→譜序→凡例→祖訓→族規→字輩→世系）。
           注意：**不可**用 class="sheet"——.sheet 帶 counter-increment: sheetpage，會頂掉既有書頁號；
           故用新類 .toc，並沿用同檔 .preface 的分頁規則（固定頁高 + break-after: page + 命名頁 page: dieji-sheet）。 -->
      <div class="preface toc" data-page-no="toc">
        <div class="preface-inner">
          <div class="toc-title">目錄</div>
          <div class="toc-list">
            <div class="toc-row" v-for="(e, ei) in tocEntries" :key="'toc' + ei"
                 :class="{ 'toc-clickable': e.pageNo != null }"
                 :title="e.pageNo != null ? ('点击跳转到第 ' + e.pageNo + ' 页') : ''"
                 @click="jumpToPage(e.pageNo)">
              <span class="toc-name">{{ e.name }}</span>
              <span class="toc-note" v-if="e.note">（{{ e.note }}）</span>
              <span class="toc-dots"></span>
              <span class="toc-page">{{ e.page }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 谱头页：凡例短篇→成员牒记同款表格（标题在姓名格位、正文在资料格，一条一列，排满续页）；谱序/祖训/族规长篇→整页竖排 -->

      <!-- 圖片展示（多條目）：傳記之後、譜頭/世系之前；每個條目各自佈局/圖說，按條目順序連續續頁 -->
      <div class="preface gallery-page" v-for="(gp, gi) in galleryPages" :key="'gal' + gp.entryIndex + '-' + gp.partNo"
           :data-page-no="gi + 1"
           :class="{ 'page-flash': highlightPageNo === gi + 1 }">
        <div class="preface-inner gallery-inner">
          <div class="gallery-doc" :class="'g' + gp.per">
            <div class="gallery-grid">
              <figure class="g-cell" v-for="(g, k) in gp.images" :key="'gk' + gi + '-' + k">
                <img :src="g.path" :alt="g.description || ''" @error="onGalleryImgError" />
                <figcaption v-if="gp.showCaption">{{ g.description || '' }}</figcaption>
              </figure>
            </div>
          </div>
        </div>
        <div class="preface-cat">{{ gp.title }}</div>
        <div class="preface-no">第 {{ gp.partNo }} / {{ gp.partTotal }} 页</div>
      </div>

      <template v-for="(pg, pi) in prefacePages" :key="'pp'+pi">
        <!-- 凡例（短篇）：与成员表完全同构，每条占一列——关系格空、姓名格放标题、资料格放整条正文（格内竖排换行，不跨格） -->
        <div v-if="pg.type === 'grid'" class="sheet rule-sheet" :data-page-no="pg.pageNo + galleryPageCount"
             :class="{ 'page-flash': highlightPageNo === pg.pageNo + galleryPageCount }">
          <div class="col-title">凡例</div>
          <div class="person-col rule-col" v-for="(it, ci) in pg.items" :key="'rc'+pi+'-'+ci" :style="{ '--rule-name-h': pg.nameH + 'px' }">
            <div class="cell-name"><span class="vtext rule-name">{{ it.title }}</span></div>
            <div class="cell-detail">{{ it.text }}</div>
          </div>
          <div class="person-col blank" v-for="b in pg.blanks" :key="'rb'+pi+'-'+b" :style="{ '--rule-name-h': pg.nameH + 'px' }">
            <div class="cell-name"></div>
            <div class="cell-detail"></div>
          </div>
        </div>
        <!-- 长篇整页：谱序/祖训/族规（横排大标题）；凡例长条目（竖排标题右起、正文紧凑） -->
        <div v-else-if="pg.type === 'long-event'" class="preface sg-page ep-page epc-page" :data-page-no="pg.pageNo + galleryPageCount"
             :class="{ 'page-flash': highlightPageNo === pg.pageNo + galleryPageCount }">
          <div class="preface-inner sg-inner" :data-cont="pg.total > 1 ? pg.part + '/' + pg.total : ''"><EventsPage :items="[{ title: '', year: '', content: pg.text }]" :page-title="(pg.category ? pg.category + ' · ' : '') + pg.title" :part="pg.part" :total="pg.total" variant="dieji" :per-page="23" :content-h="1118" :page-no="pg.pageNo + galleryPageCount" /></div>
        </div>
        <div v-else class="preface preface-long" :class="{ 'rule-long': pg.kind === 'rule', 'page-flash': highlightPageNo === pg.pageNo + galleryPageCount }" :data-page-no="pg.pageNo + galleryPageCount">
          <div class="preface-inner">
            <template v-if="pg.isFirst && pg.kind !== 'rule'">
              <div class="preface-cat">{{ pg.article.category }}</div>
              <div class="preface-title">{{ pg.article.title }}</div>
              <div class="preface-meta" v-if="pg.article.author || pg.article.year">{{ pg.article.author }}<template v-if="pg.article.author && pg.article.year"> · </template>{{ pg.article.year }}</div>
            </template>
            <div class="vgrid" :class="{ 'rule-vgrid': pg.kind === 'rule' }">
              <div v-if="pg.isFirst && pg.kind === 'rule'" class="vcol rule-vcol-title">{{ pg.article.title }}</div>
              <div class="vcol" v-for="(c, ci) in pg.cols" :key="'lc'+pi+'-'+ci">{{ c.text }}</div>
            </div>
            </div>
        </div>
      </template>

      <div class="preface genword" v-for="(g, gi) in generationWords" :key="'gw'+g.id"
           :data-page-no="galleryPageCount + prefacePages.length + gi + 1"
           :class="{ 'page-flash': highlightPageNo === galleryPageCount + prefacePages.length + gi + 1 }">
        <div class="preface-inner">
          <div class="preface-cat">字辈派语</div>
          <div class="genword-branch">{{ g.branch || '总谱' }}</div>
          <div class="genword-poem">{{ g.poem }}</div>
          <div class="genword-chars">
            <span v-for="(c, i) in parseWords(g.words_array)" :key="i" class="genword-char">{{ c }}<small>{{ g.start_generation + i }}</small></span>
          </div>
        </div>
      </div>

      <!-- 五服圖（服制圖）：字輩派語之後、世系之前 —— 譜頭文化之後、五世一圖之前（三本谱书同序）。
           整頁一幅 SVG 矢量圖，佔正文頁碼（頁腳由 .preface 的 bookpage 計數自動給出），並進目錄。 -->
      <div class="preface fivefu-page" :data-page-no="prePageCount + 1"
           :class="{ 'page-flash': highlightPageNo === prePageCount + 1 }">
        <div class="preface-inner ff-inner">
          <div class="ff-head">
            <span class="ff-title">五服圖</span>
            <span class="ff-meta">{{ tanghao ? tanghao + '　' : '' }}本宗九族服制（斬衰·齊衰·大功·小功·緦麻）</span>
          </div>
          <div class="ff-body"><FiveFuChart layout="tall" /></div>
        </div>
      </div>

      <!-- 名人传记：一传一页（照牒记式竖版）；只收「本支成员」的传记（配偶随夫判断归属）；
           排在五服圖之后、世系之前；正文过长按页容量自动续页，页尾「n / N」旗标。 -->
      <!-- 家族大事记（五服圖之后、名人傳記之前） -->
      <div class="preface sg-page ep-page epc-page" v-for="(ep, ei) in eventPages" :key="'ev' + ei"
           :data-page-no="prePageCount + 2 + ei"
           :class="{ 'page-flash': highlightPageNo === prePageCount + 2 + ei }">
        <div class="preface-inner sg-inner" :data-cont="ep.total > 1 ? ep.part + '/' + ep.total : ''"><EventsPage :items="ep.items" :part="ep.part" :total="ep.total" variant="dieji" :per-page="23" :content-h="1118" :page-no="prePageCount + 2 + ei" /></div>
      </div>

      <div class="preface bio-page is-bio" v-for="(bp, bi) in bioPages" :key="'bio' + bi"
           :data-page-no="prePageCount + eventPages.length + bi + 2"
           :class="{ 'bio-cont': !bp.isFirst, 'page-flash': highlightPageNo === prePageCount + eventPages.length + bi + 2 }">
        <div class="preface-inner bio-inner" :data-cont="bp.total > 1 ? (bp.seq + 1) + '/' + bp.total : ''" :class="{ 'bio-cont': !bp.isFirst }"
             :style="{ '--bio-head-h': bp.headH + 'px', '--bio-grid-h': bp.gridH + 'px' }">
          <div class="bio-head" v-if="bp.isFirst">
            <div class="bio-info">
              <div class="preface-cat">名人传记</div>
              <div class="preface-title">{{ bp.bio.name }}<span v-if="bioZiHao(bp.bio)" class="bio-zh">　{{ bioZiHao(bp.bio) }}</span></div>
              <div class="preface-meta" v-if="bioMeta(bp.bio)">{{ bioMeta(bp.bio) }}</div>
            </div>
            <div class="bio-portrait">
              <img v-if="bp.bio.portrait" :src="bp.bio.portrait" :alt="bp.bio.name || ''" />
              <div v-else class="ph">{{ (bp.bio.name || '傳').slice(0, 1) }}</div>
            </div>
          </div>
          <div class="vgrid" :style="{ '--vcol-n': bp.maxCols }">
            <div class="vcol" v-for="(c, ci) in bp.cols" :key="'bc' + bi + '-' + ci">{{ c.text }}</div>
          </div>
          </div>
      </div>

      <!-- 祠宇志（每个祠宇独立起页；有图则正文页 + 图版页） → 茔墓志 → 世系 -->
      <div class="preface sg-page sg-fs16" v-for="(sp, si) in shrinePageList" :key="'sh' + si"
           :data-page-no="prePageCount + bioPages.length + eventPages.length + si + 2"
           :class="{ 'page-flash': highlightPageNo === prePageCount + bioPages.length + eventPages.length + si + 2, 'sg-plate-page': sp.mode === 'image' }">
        <div class="preface-inner sg-inner"><ShrinePage :shrine="sp.item" :mode="sp.mode" :part="sp.part || 'all'"
                        :inline-photo="sp.inlinePhoto || ''" :images="sp.images || []" :image-kind="sp.imageKind || 'photo'" layout="rows" variant="branch" /></div>
      </div>
      <div class="preface sg-page sg-fs16" v-for="(gp, gi) in gravePageList" :key="'gr' + gi"
           :data-page-no="prePageCount + bioPages.length + eventPages.length + shrinePageList.length + gi + 2"
           :class="{ 'page-flash': highlightPageNo === prePageCount + bioPages.length + eventPages.length + shrinePageList.length + gi + 2 }">
        <div class="preface-inner sg-inner"><GravePage :grave="gp.item" variant="branch" :stack="true" /></div>
      </div>

      <div class="sheet" v-for="(sheet, si) in sheets" :key="si"
           :data-page-no="prePageCount + bioPages.length + eventPages.length + shrinePageList.length + gravePageList.length + si + 2"
           :class="{ 'page-flash': highlightPageNo === prePageCount + bioPages.length + eventPages.length + shrinePageList.length + gravePageList.length + si + 2 }">
        <!-- 谱名列（最右，row-reverse 下首项居最右） -->
        <div class="col-title">{{ familyTitle }}</div>
        <!-- 人物列：关系小格 / 姓名大字 / 资料格（竖排，黑条起首） -->
        <div class="person-col" v-for="(p, ci) in sheet.persons" :key="'p'+ci">
          <div class="cell-rel"><span class="rel2" v-if="relParts(p.rel).f"><span class="rel2-f">{{ relParts(p.rel).f }}</span><span class="rel2-r" v-if="relParts(p.rel).r">{{ relParts(p.rel).r }}</span></span></div>
          <div class="cell-name" :class="{ 'cont-name': p.isCont }"><span class="vtext">{{ p.isCont ? '续' : p.name }}</span></div>
          <div class="cell-detail">
            <span v-if="p.elder && !p.isCont" class="gen-bar">{{ cn(p.gen) }}世</span>{{ p.contText || detailText(p) }}<span v-if="p.src && !p.isCont" class="src-note">　{{ p.src }}</span>
          </div>
        </div>
        <!-- 空白列：补满框线 -->
        <div class="person-col blank" v-for="b in sheet.blanks" :key="'b'+b">
          <div class="cell-rel"></div>
          <div class="cell-name"></div>
          <div class="cell-detail"></div>
        </div>
      </div>

      <!-- 牌記（版權頁）：全書末頁 —— 三行小字（堂號／修次／紀年）＋「X堂藏板」。
           刻意不用 .preface/.cover → 不參與 bookpage 計數（不佔正文頁碼）、不進目錄。 -->
      <div class="colo-page" data-page-no="colophon">
        <div class="colo-inner">
          <ColophonPage :tanghao="tanghao" :xiuci="xiuci" :year-label="coloYear" :unit="26" />
        </div>
      </div>
    </div>
    <!-- 长篇竖行容量测量探针（隐藏，不占布局） -->
    <!-- 隱藏量測探針的外層裁剪盒：探針不吃 zoom、寬度可達 880px，
         不裁剪會把整個文檔撐寬（手機上出現可左右拖走的空白）。只裁溢出，不影響探針自身量測。 -->
    <div class="probe-clip" aria-hidden="true">
    <div class="measure-probe" aria-hidden="true">
      <div class="preface preface-long">
        <div class="preface-inner" ref="probeInner">
          <div ref="probeTitle">
            <div class="preface-cat">族规</div>
            <div class="preface-title">合编家宗族规</div>
            <div class="preface-meta">古训选编·近现代选编</div>
          </div>
          <div class="vcol" ref="probeCol">字</div>
        </div>
      </div>
    </div>
    </div><!-- /.probe-clip -->

    <!-- 圖片展示選擇器（不參與打印） -->
    <!-- 圖片展示選擇器（多條目；共用組件，不參與打印） -->
    <GalleryDialog v-model="galleryOpen" :entries="galleryEntries" :pool="galleryPool"
                   :pool-loading="galleryPoolLoading" mode="branch" :branch-id="Number(branchId)"
                   @update:entries="onGalleryEntries" @upload="galleryUpload" />

    <!-- 回到顶部（右下角悬浮，fixed 于视口）：滚过 200px 才淡入；打印隐藏 -->
    <transition name="ph-fade">
      <el-button v-show="showTopBtn" class="back-top no-print" circle size="large"
                 title="回到顶部" @click="backToTop">
        <el-icon><ArrowUp /></el-icon>
      </el-button>
    </transition>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import api from '../api'
import { ArrowUp } from '@element-plus/icons-vue'
import { formatLifeDates } from '../utils/dateFormat'
import { lostContactText, memberKnown, NO_HEIR_TRAD } from '../utils/memberFlags'
import { relHeadOf, relHeadPartsOfText } from '../utils/lineageRel'
import { fetchAllSources, groupSourcesByMember, sourceFoot } from '../utils/memberSources'
import { branchCtx } from '../utils/branchContext'
import InnerTitlePage from '../components/book/InnerTitlePage.vue'
import ColophonPage from '../components/book/ColophonPage.vue'
import FiveFuChart from '../components/book/FiveFuChart.vue'
import ShrinePage from '../components/book/ShrinePage.vue'
import GravePage from '../components/book/GravePage.vue'
import { fetchShrineGrave } from '../utils/shrineGrave'
import EventsPage from '../components/book/EventsPage.vue'

const route = useRoute()
const branchId = route.params.id
const branchName = ref('')

const book = ref(null)
const membersRaw = ref([])   // 接口返回的全量成员（本视图自行按 URL 房支范围过滤）
const branchList = ref([])   // /branches（含 path，用于「含下级支」前缀判断）
// 支谱成员范围：根＝URL 的 branchId（优先于顶部 current）；是否含下级＝顶部「含下级」开关。
// 与排序/过滤同一份 members → 世系、名人傳記都自动跟随，不需要各自再算。
const branchPath = computed(() => {
  const b = branchList.value.find(x => String(x.id) === String(branchId))
  return b ? String(b.path || '') : ''
})
const scopeBranchIds = computed(() => {
  const ids = new Set([String(branchId)])          // 本支成员一定包含
  if (!branchCtx.includeChildren) return ids      // 关闭：只本支（与旧行为一致）
  const root = branchPath.value
  if (!root) return ids                           // 无 path（异常/旧数据）→ 退回只本支
  for (const b of branchList.value) {             // 含下级：path 前缀判断（1 / 1.x / 1.x.y …）
    const p = String(b.path || '')
    if (p === root || p.startsWith(root + '.')) ids.add(String(b.id))
  }
  return ids
})
const members = computed(() =>
  (membersRaw.value || []).filter(m => scopeBranchIds.value.has(String(m.branch_id))))
const sourceMap = ref({})   // memberId → 資料來源腳注（'据：…；…'；無來源則無此鍵）
const familyName = ref('')
const surname = ref('')
const tanghao = ref('')
const xiuci = ref('重修')   // 修次：封面「歲次…X」用（服务器设置）
// 封面書名自適應：4 字用原設計 56px，更長按比例縮小，保證不溢出封面框
const coverTitleStyle = computed(() => {
  const n = Math.max(1, Array.from(familyTitle.value || '').length)
  const fs = n <= 6 ? 60 : Math.max(30, Math.round(60 * 6 / n))
  const ls = n <= 6 ? 22 : Math.max(10, Math.round(22 * 6 / n))
  const fsL = fs, lsL = ls
  return { '--title-fs': fs + 'px', '--title-ls': ls + 'px', '--title-fs-land': fsL + 'px', '--title-ls-land': lsL + 'px' }
})
const junwang = ref('')
const zuji = ref('')
const articles = ref([])
const generationWords = ref([])
const biographies = ref([])   // 名人传记（/api/biographies，已由后端按世代→排行→生年→id 排序）
const shrines = ref([])       // 祠宇志（/api/shrines，含子项）
const graves = ref([])        // 茔墓志
const events = ref([])        // 家族大事记（/api/graves）

// ===== 干支纪年（封面「歲次XX」）=====
const TIAN_GAN = ['甲', '乙', '丙', '丁', '戊', '己', '庚', '辛', '壬', '癸']
const DI_ZHI = ['子', '丑', '寅', '卯', '辰', '巳', '午', '未', '申', '酉', '戌', '亥']
function yearToGanzhi(y) {
  const offset = (y - 4) % 60
  if (offset < 0) return ''
  return TIAN_GAN[offset % 10] + DI_ZHI[offset % 12]
}
const currentYearGanzhi = computed(() => yearToGanzhi(new Date().getFullYear()))
// 牌記紀年（內封左下的「歲次／修次」由組件分兩竖列渲染）
const coloYear = computed(() => '歲次' + currentYearGanzhi.value)
// 五服圖之前的頁數（圖片展示 + 譜頭各塊 + 字輩派語）：五服圖固定佔其後 1 頁，世系頁碼全體順延 1
const MEMBER_CAP_BRANCH = 174
function splitByCap(text, cap) {
  // 按句装箱：。！？； 为句界；单句超 cap 才硬切兜底
  const chars = Array.from(text)
  const out = []
  const sentence = []
  for (const ch of chars) {
    sentence.push(ch)
    if ('。！？；'.includes(ch)) { out.push(sentence.join('')); sentence.length = 0 }
  }
  if (sentence.length) out.push(sentence.join(''))
  const pieces = []
  let buf = ''
  for (const s0 of out) {
    if (Array.from(s0).length > cap) {
      if (buf) { pieces.push(buf); buf = '' }
      const a = Array.from(s0)
      for (let i = 0; i < a.length; i += cap) pieces.push(a.slice(i, i + cap).join(''))
      continue
    }
    if (buf && Array.from(buf + s0).length > cap) { pieces.push(buf); buf = '' }
    buf += s0
  }
  if (buf) pieces.push(buf)
  if (pieces.reduce((n,c)=>n+Array.from(c).length,0) !== chars.length) throw new Error('member split lost chars')
  if (pieces.some(c => Array.from(c).length > cap)) throw new Error('member split > cap')
  return pieces
}
function splitMember(p) {
  const parts = splitByCap(detailText(p), MEMBER_CAP_BRANCH)
  if (parts.length < 2) return [p]
  return parts.map((t, i) => i === 0 ? { ...p, contText: t } : { ...p, name: '续', rel: '', elder: false, src: '', isCont: true, contText: t })
}
const sheets = computed(() => {
  const out = []
  const per = PERSON_COLS.value
  const expanded = []
  for (const p of people.value) expanded.push(...splitMember(p))
  for (let i = 0; i < expanded.length; i += per) {
    const persons = expanded.slice(i, i + per)
    out.push({ persons, blanks: per - persons.length })
  }
  return out
})
// 實際打印頁號：與目錄/頁腳同一口徑
const pageNoMap = ref({})
function measurePageNos() {
  const root = document.querySelector('.book')
  if (!root) return
  const map = {}
  const list = root.querySelectorAll('.preface:not(.toc), .sheet')
  let n = 0
  list.forEach((el) => { if (el.dataset.pageNo) map[el.dataset.pageNo] = ++n })
  pageNoMap.value = map
}
const prePageCount = computed(() =>
  galleryPageCount.value + prefacePages.value.length + (generationWords.value || []).length)

// 祠宇志/茔墓志：每个祠宇一页正文；有实景图/平面图再加一页图版；每座茔墓一页。无数据 → 空数组（不生成页/目录行）。
const hasShrineImages = (s) => !!((s && s.photos && s.photos.length) || (s && s.plan_images && s.plan_images.length))
const hasShrinePart2 = (s) => !!(s && (s.ritual || s.remark || (s.couplets && s.couplets.length) || (s.properties && s.properties.length)))
const shrineTextLen = (s) => {
  if (!s) return 0
  const parts = [
    ...(s.history || []).map(h => (h.year || '') + (h.event || '') + (h.note || '')),
    s.architecture || '', s.spirit_layout || '', s.ritual || '', s.remark || '',
    ...(s.properties || []).map(p => (p.name || '') + (p.area || '') + (p.note || '')),
    ...(s.couplets || []).map(c => c.content || ''),
  ]
  return parts.join('').length
}
// 每个祠宇的页序由内容决定：文本一次能放下就 1 页（短文本再把首张实景图接排在下半页）；
// 文本 > 760 字才拆 2 页。实景图每页 2 张（两张各占约 46% 版心高，填满），平面图每张独占 1 页满版心。
const shrinePageList = computed(() => {
  const out = []
  for (const s of shrines.value) {
    const photos = (s.photos || []).filter(Boolean).slice()
    const plans = (s.plan_images || []).filter(Boolean)
    const split = shrineTextLen(s) > 2000
    if (split) {
      out.push({ kind: 'shrine', item: s, mode: 'text', part: '1' })
      out.push({ kind: 'shrine', item: s, mode: 'text', part: '2' })
    } else {
      const inlinePhoto = (shrineTextLen(s) < 500 && photos.length) ? photos.shift() : ''
      out.push({ kind: 'shrine', item: s, mode: 'text', part: 'all', inlinePhoto })
    }
    for (let i = 0; i < photos.length; i += 2) {
      out.push({ kind: 'shrine', item: s, mode: 'image', imageKind: 'photo', images: photos.slice(i, i + 2) })
    }
    for (const p of plans) {
      out.push({ kind: 'shrine', item: s, mode: 'image', imageKind: 'plan', images: [p] })
    }
  }
  return out
})
const gravePageList = computed(() => graves.value.map(g => ({ kind: 'grave', item: g })))

const EVENT_PAGE_CHARS = 1800
const eventPages = computed(() => {
  const list = events.value || []
  const pages = []; let cur = [], len = 0
  for (const e of list) {
    const n = String(e.year || '').length + String(e.title || '').length + String(e.content || '').length + 24
    if (cur.length && len + n > EVENT_PAGE_CHARS) { pages.push(cur); cur = []; len = 0 }
    cur.push(e); len += n
  }
  if (cur.length) pages.push(cur)
  return pages.map((items, i) => ({ items, part: i + 1, total: pages.length }))
})

// ===== 谱书字体切换（内置系统字体栈 + 系统级上传字体：存服务器，换浏览器/设备都能用）=====
const BUILTIN_FONTS = [
  { key: 'song', label: '宋体', stack: '"SimSun","宋体","STSong","Noto Serif CJK SC","Songti SC",serif' },
  { key: 'kai',  label: '楷体', stack: '"KaiTi","楷体","STKaiti","Kaiti SC","KaiTi_GB2312","Noto Serif CJK SC",serif' },
  { key: 'fang', label: '仿宋', stack: '"FangSong","仿宋","STFangsong","仿宋_GB2312","Noto Serif CJK SC",serif' },
  { key: 'hei',  label: '黑体', stack: '"SimHei","黑体","Heiti SC","STHeiti","Noto Sans CJK SC",sans-serif' },
  { key: 'li',   label: '隶书', stack: '"LiSu","隶书","STLiti","Baoli SC",serif' },
]
// FONTS = 内置 + 服务器自定义字体；自定义字体 key = 'custom-' + family
const FONTS = ref([...BUILTIN_FONTS])
const fontKey = ref(localStorage.getItem('diejiFont') || 'song')
// 字号：传统号数制（初号→八号），选号即整页牒记内容等比缩放（仅谱书内容，工具栏不变）
// 以「四号 14pt」为标准大小(scale=1)，其余号数按 pt 比例换算缩放因子
const BASE_PT = 14
const SIZE_OPTS = [
  { pt: 22, label: '二号（22pt · 最大）' },
  { pt: 18, label: '小二（18pt）' },
  { pt: 16, label: '三号（16pt）' },
  { pt: 15, label: '小三（15pt）' },
  { pt: 14, label: '四号（标准 · 14pt）' },
  { pt: 12, label: '小四（12pt）' },
  { pt: 10.5, label: '五号（10.5pt）' },
  { pt: 9, label: '小五（9pt）' },
  { pt: 7.5, label: '六号（7.5pt）' },
  { pt: 6.5, label: '小六（6.5pt）' },
  { pt: 5.5, label: '七号（5.5pt）' },
  { pt: 5, label: '八号（5pt）' },
]
const PTVALS = SIZE_OPTS.map(o => o.pt)
// 旧版存的是缩放因子(0.92~1.16)，不匹配 pt 列表则回退标准四号
const storedPt = parseFloat(localStorage.getItem('diejiFontSize') || '14')
const fontSize = ref(PTVALS.includes(storedPt) ? storedPt : 14)
const fontInput = ref(null)
// 当前缩放倍数（四号=1）；字号放大 s 倍时：每页列数 ÷s、每列字数 ÷s、标题格高 ×s
const scaleVal = computed(() => {
  const pt = PTVALS.includes(fontSize.value) ? fontSize.value : BASE_PT
  return pt / BASE_PT
})
function applySize(pt) {
  const val = PTVALS.includes(pt) ? pt : BASE_PT
  const scale = val / BASE_PT
  document.documentElement.style.setProperty('--dieji-scale', String(scale))
  localStorage.setItem('diejiFontSize', String(val))
}
function onSizeChange() {
  applySize(fontSize.value)
  nextTick(() => scheduleMeasure())
}
const currentFontCustom = computed(() => { const f = FONTS.value.find(x => x.key === fontKey.value); return !!(f && f.custom) })
function applyFont(k) {
  const f = FONTS.value.find(x => x.key === k) || FONTS.value[0]
  document.documentElement.style.setProperty('--dieji-font', f.stack)
  localStorage.setItem('diejiFont', f.key)
}
watch(fontKey, applyFont, { immediate: true })
applySize(fontSize.value)

// ===== 预览缩放：默认「适应屏幕」，可手动放大（－/＋；点百分比回到适应屏幕）=====
// 与垂珠体/竖版牒记式同一口径：纸张宽 = 880px / 字号缩放（本地 px），
// 屏幕显示宽 = 纸张宽 × 字号缩放 × --preview-scale；自动档取 (容器宽-48)/纸张宽 → 恰好铺满、无横向滚动；
// 手动档用用户值，可超出屏幕 → 由 .book 的 overflow:auto 横向+纵向滚动看全。
const demoRoot = ref(null)
const previewViewW = ref(0)
const paperW = ref(880)          // 纸张设计宽（实测，随字号缩放变化）
const previewScale = ref(1)
const explicitManual = ref(false)
const ZOOM_STEP = 0.15

function measurePaperW() {
  const bk = book.value
  if (!bk) return
  // .book 上挂 zoom；纸张本地宽 = 设备宽 / zoom（与 zoom 无关，故一次量测即收敛）
  const z = parseFloat(getComputedStyle(bk).zoom) || 1
  let mx = 0
  bk.querySelectorAll('.cover, .preface, .sheet').forEach((el) => {
    const w = el.getBoundingClientRect().width / z
    if (w > mx) mx = w
  })
  if (mx > 100) paperW.value = Math.min(2000, mx)
}
function updatePreviewViewW() {
  previewViewW.value = demoRoot.value ? demoRoot.value.clientWidth : (window.innerWidth || 1200)
  measurePaperW()
}
const fitPreviewScale = computed(() => {
  const baseW = paperW.value || 880
  return Math.max(0.2, Math.min((previewViewW.value - 48) / baseW, 3))
})
const effectivePreviewScale = computed(() => (
  explicitManual.value ? previewScale.value : Math.min(previewScale.value, fitPreviewScale.value)
))
watch(effectivePreviewScale, (v) => {
  document.documentElement.style.setProperty('--preview-scale', String(v))
}, { immediate: true })
const zoomLabel = computed(() => Math.round(effectivePreviewScale.value * 100) + '%')
const round2 = (v) => Math.round(v * 100) / 100
function zoomIn() { explicitManual.value = true; previewScale.value = Math.min(3, round2(previewScale.value + ZOOM_STEP)) }
function zoomOut() { explicitManual.value = true; previewScale.value = Math.max(0.2, round2(previewScale.value - ZOOM_STEP)) }
function zoomReset() { explicitManual.value = false; previewScale.value = 1 }

// 用一个字体 URL 注册 @font-face（系统返回的 family 名作 CSS font-family）
async function registerServerFont(info, buf) {
  const key = 'custom-' + info.family
  // 优先用刚上传文件的 ArrayBuffer 直接注册（手机/微信不必再走网络拉取，最稳）；
  // 否则用服务器 URL 注册。woff2/ttf/otf 都走 FontFace，浏览器支持即可。
  const source = buf ? buf : `url(${encodeURI(info.url)})`
  const face = new FontFace(info.family, source, { display: 'swap' })
  await face.load()
  document.fonts.add(face)
  FONTS.value = FONTS.value.filter(f => f.key !== key)
  FONTS.value.push({ key, label: info.label + '（上传）', stack: `"${info.family}","SimSun","宋体",serif`, family: info.family, url: info.url, name: info.name, custom: true })
  try { await document.fonts.load('16px "' + info.family + '"') } catch (e) {}
  return key
}

// 启动：从服务器拉取已上传字体并注册，若上次选的是某自定义字体则套回
async function loadServerFonts() {
  try {
    const { data } = await api.get('/fonts')
    let wantKey = localStorage.getItem('diejiFont') || 'song'
    let restored = false
    for (const info of (data || [])) {
      try {
        const key = await registerServerFont(info)
        if (key === wantKey) restored = true
      } catch (err) {
        // 单个字体加载失败（损坏文件/浏览器不支持）只跳过它，不中断其余字体
        console.warn('字体加载失败，已跳过：', info && info.name, err)
      }
    }
    if (wantKey.startsWith('custom-')) {
      fontKey.value = restored ? wantKey : 'song'
    }
    applyFont(fontKey.value)
    await nextTick(); scheduleMeasure()
  } catch (err) { console.warn('加载系统字体失败', err) }
}

function pickFontFile() { fontInput.value && fontInput.value.click() }
async function onFontFile(e) {
  const file = e.target.files && e.target.files[0]
  e.target.value = ''
  if (!file) return
  if (!/\.(ttf|otf|woff2?|ttc)$/i.test(file.name)) {
    ElMessage.warning('请上传 .ttf / .otf / .woff / .woff2 / .ttc 字体文件')
    return
  }
  if (file.size > 30 * 1024 * 1024) { ElMessage.warning('字体文件过大（上限 30MB）'); return }
  // 第一步：上传到服务器（这一步成功就代表字体已存进系统）
  let info
  try {
    const fd = new FormData()
    fd.append('file', file)
    const resp = await api.post('/fonts', fd, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 30000 })
    info = resp.data
  } catch (err) {
    console.error(err)
    const st = err && err.response && err.response.status
    const msg = err && err.response && err.response.data && err.response.data.error
    if (st === 403) ElMessage.error(msg || '权限不足：仅理事长及以上可上传字体')
    else if (st === 404) ElMessage.error('接口不存在，请确认后端已更新到最新版')
    else if (st === 413) ElMessage.error('字体文件过大（上限 30MB）')
    else ElMessage.error(msg || '字体上传到服务器失败：' + file.name)
    return
  }
  // 第二步：在本机浏览器加载并套用（失败不影响已保存，换个浏览器仍可用）
  try {
    const buf = await file.arrayBuffer()
    const key = await registerServerFont(info, buf)
    fontKey.value = key
    await nextTick(); scheduleMeasure()
    ElMessage.success('字体已保存到系统并套用：' + file.name)
  } catch (err) {
    console.error(err)
    ElMessage.warning('字体已上传到系统，但当前浏览器不支持/无法加载该字体文件（woff2 需较新浏览器）；换设备或改用 ttf/otf 即可')
    await loadServerFonts()
  }
}
async function deleteCurrentFont() {
  const f = FONTS.value.find(x => x.key === fontKey.value)
  if (!f || !f.custom) return
  try {
    const { ElMessageBox } = await import('element-plus')
    await ElMessageBox.confirm(`确定删除上传字体「${f.label}」吗？`, '删除字体', { type: 'warning' })
    await api.delete('/fonts/' + encodeURIComponent(f.name || (f.family.replace(/^ZupuFont_/, '') + '.ttf')))
    FONTS.value = FONTS.value.filter(x => x.key !== f.key)
    fontKey.value = 'song'
    ElMessage.success('已删除字体')
  } catch (e) { /* 取消 */ }
}

// ===== 谱头页：谱序 → 凡例 → 祖训 → 族规 → 字辈派语 =====
const CAT_ORDER = { 谱序: 0, 凡例: 1, 祖训: 2, 族规: 3 }
const prefaceArticles = computed(() => {
  const list = articles.value.filter(a => a.category in CAT_ORDER)
  list.sort((a, b) => (CAT_ORDER[a.category] - CAT_ORDER[b.category]) || (a.id - b.id))
  return list
})

// 清洗正文：去掉 Markdown「> 」引用符，段落间以全角空格分隔（竖排连续排布）
function cleanText(s) {
  return (s || '').replace(/^\s*>\s?/gm, '').replace(/\s*\n\s*/g, '　').trim()
}

// ===== 谱头排版：凡例短篇→成员牒记同款表格（每条占一整列，标题在姓名格、正文整条在资料格内竖排换行）；谱序/祖训/族规长篇→整页竖格 =====
const BASE_GRID_COLS = 12   // 标准字号下凡例表每表条数（=成员表人数列数）
const GRID_COLS = computed(() => Math.max(5, Math.round(BASE_GRID_COLS / Math.pow(scaleVal.value, 1.5))))
const LONG_COLS = computed(() => Math.max(7, Math.round(14 / scaleVal.value)))  // 长篇每页竖行数，随字号收缩
// 长篇每竖行字数：按实际渲染测量（竖行撑满框内高度），避免固定字数导致竖行下半截空白却提前翻页
const longCap = ref({ first: 24, cont: 30 })   // 首行(有标题) / 续行 容量；挂载后实测覆盖（保守值防裁字）
const probeInner = ref(null)
const probeTitle = ref(null)
const probeCol = ref(null)
let measureTimer = null
function measureLongCap() {
  if (!probeInner.value || !probeTitle.value || !probeCol.value) return
  const inner = probeInner.value
  const cs = getComputedStyle(inner)
  const contentH = inner.clientHeight - parseFloat(cs.paddingTop) - parseFloat(cs.paddingBottom)
  const titleH = probeTitle.value.offsetHeight + 16
  // 探针竖行：塞入 20 字，按高度反推每字竖向步长（含 padding）
  probeCol.value.textContent = '字'.repeat(20)
  const csCol = getComputedStyle(probeCol.value)
  const padV = parseFloat(csCol.paddingTop) + parseFloat(csCol.paddingBottom)
  const step = (probeCol.value.offsetHeight - padV) / 20
  if (!(step > 0)) return
  // 安全余量 0.88：手机端字体渲染往往比测量值偏大，宁留底白、不可裁字
  // 字号缩放：探针在 zoom 下框高与步长同步放大、容量比被抵消，这里显式 ÷缩放倍数，
  // 保证字号放大后每竖行装的字数相应减少、长篇自动多翻页（不溢出框线）
  const SAFETY = 0.88
  const sc = scaleVal.value || 1
  const first = Math.max(8, Math.floor((contentH - titleH) / step * SAFETY / sc))
  const cont  = Math.max(10, Math.floor((contentH - 24) / step * SAFETY / sc))
  longCap.value = { first, cont, step }
}
function scheduleMeasure() {
  clearTimeout(measureTimer)
  measureTimer = setTimeout(() => nextTick(measureLongCap), 120)
}

function splitCols(text, cap) {
  const out = []
  for (let i = 0; i < text.length; i += cap) out.push(text.slice(i, i + cap))
  return out
}

const SHORT_MAX = 120
// 凡例标题格固定高度：每个凡例页按**该页最长标题**字数自动计算，加少量余量；
// 竖排每字约 24px（17px 字号 × 1.3 行高，留余量）。同一页各列标题格等高、正文起点对齐。
const RULE_NAME_CHAR_H = 24
watch(prefaceArticles, scheduleMeasure)
onMounted(() => { scheduleMeasure(); updatePreviewViewW(); window.addEventListener('resize', () => { scheduleMeasure(); updatePreviewViewW() }); loadServerFonts(); loadGallery()
  if (book.value) book.value.addEventListener('scroll', onScrollTopBtn, { passive: true })
})
onBeforeUnmount(() => { if (book.value) book.value.removeEventListener('scroll', onScrollTopBtn) })

const PREFACE_GRID_COLS = 23
const PREFACE_GRID_CHARS = Math.floor((1118 - 8) / 18) + 1

const prefacePages = computed(() => {
  const pages = []
  let shortItems = []          // 凡例 [{title,text}]
  const flushGrid = () => {
    if (shortItems.length) {
      const maxTitle = shortItems.reduce((m, it) => Math.max(m, (it.title || '').length), 4)
      const cols = GRID_COLS.value
      // cats：本页含哪些谱头类别（仅供目錄页推算起始页；纯新增字段，不影响既有排版/分页）
      pages.push({ type: 'grid', items: shortItems, blanks: cols - shortItems.length, pageNo: pages.length + 1,
                   nameH: Math.ceil(maxTitle * RULE_NAME_CHAR_H),
                   cats: [...new Set(shortItems.map(it => it.cat))] })
      shortItems = []
    }
  }
  for (const a of prefaceArticles.value) {
    const text = cleanText(a.content)
    // 字号放大时单格容量变小：阈值 ÷缩放倍数，超出的条目走长篇竖排页（定列分页、可续页，绝不挤格/溢出）
    const shortMax = Math.round(SHORT_MAX / Math.pow(scaleVal.value, 1.3))
    if (text.length <= shortMax) {
      shortItems.push({ title: a.title || '', text, cat: a.category })
      if (shortItems.length >= GRID_COLS.value) flushGrid()
    } else {
      flushGrid()
      // 逐列填充：每竖行塞满该页可用高度（首页有标题→first 容量，续页→cont 容量）
      const perBody = Math.max(1, PREFACE_GRID_COLS - 1)
      const chunkSize = PREFACE_GRID_CHARS * perBody
      const totalPages = Math.max(1, Math.ceil(text.length / chunkSize))
      for (let pi = 0; pi < totalPages; pi++) {
        pages.push({
          type: 'long-event', article: a, text: text.slice(pi * chunkSize, (pi + 1) * chunkSize),
          title: (a.title || ''), category: a.category || '', part: pi + 1, total: totalPages, pageNo: pages.length + 1,
        })
      }
    }
  }
  flushGrid()
  return pages
})

// ===== 名人传记（支谱）：只收本支成员；照牒记式竖版「一传一页」=====
// 归属判断：直接复用本视图「世系」所用的成员集合 members（已按本支 branch_id 过滤，与世系同一口径）。
//   · member_id 在本支成员集合内 → 收录；
//   · spouse_id → 找到其配偶（丈夫）的 member_id，丈夫在本支 → 收录（传统「配附于夫」）；
//   · 关联不到成员（member_id/spouse_id 均为 0 或关联记录不在本支）→ 不显示（无法判断归属）。
const branchMemberIdSet = computed(() => new Set((members.value || []).map(m => m.id)))
const spouseOwnerMemberId = computed(() => {
  const map = new Map()
  for (const m of (members.value || [])) {
    for (const s of (m.spouses || [])) map.set(s.id, m.id)
  }
  return map
})
const branchBiographies = computed(() => (biographies.value || []).filter((b) => {
  // 只收公开展示的传记（与牒记式/垂珠体谱书一致）
  if (!(b.public === 1 || b.public === '1' || b.public == null)) return false
  if (b.member_id) return branchMemberIdSet.value.has(b.member_id)
  if (b.spouse_id) {
    const owner = spouseOwnerMemberId.value.get(b.spouse_id)
    return owner != null && branchMemberIdSet.value.has(owner)
  }
  return false
}))

// 传记名后的「字X 號Y」小字
function bioZiHao(b) {
  const parts = []
  if (b && b.zi) parts.push('字' + b.zi)
  if (b && b.hao) parts.push('號' + b.hao)
  return parts.join('　')
}
// 传记名下的信息行（性别 / 生卒 / 籍貫 / 現居 / 身分 / 職業 / 榮譽）
function bioMeta(b) {
  const parts = []
  if (b.gender) parts.push(b.gender)
  const bt = [b.birth_era, b.birth_lunar, b.birth_gregorian].filter(Boolean).join(' ')
  const dt = [b.death_era, b.death_lunar, b.death_gregorian].filter(Boolean).join(' ')
  if (bt) parts.push('生 ' + bt)
  if (dt) parts.push('卒 ' + dt)
  if (!dt && bt) parts.push('在世')
  if (b.native_place) parts.push('籍貫 ' + b.native_place)
  if (b.current_place) parts.push('現居 ' + b.current_place)
  if (b.title) parts.push(b.title)
  if (b.occupation) parts.push(b.occupation)
  if (b.honors) parts.push(b.honors)
  return parts.join(' · ')
}
// 估算折行长度的「字重」：全角 1、半角 0.55（只用于估资料行占几行）
function textWeight(s) {
  let n = 0
  for (const ch of String(s || '')) n += /[\x00-\xff]/.test(ch) ? 0.55 : 1
  return n
}

// 版式常量（逻辑 px；整本书由 --dieji-scale 统一缩放）
const BIO_PORTRAIT_W = 120
const BIO_PORTRAIT_H = 150
const BIO_HEAD_PAD = 12
const BIO_HEAD_BORDER = 1
const BIO_HEAD_GAP = 12
const BIO_HEAD_INFO_W = 380
const BIO_PAGE_CONTENT_H = 1060   // .preface-inner 1180 − 上下 padding 60×2
const BIO_FLAG_RESERVE = 26       // 页脚页码/旗标位
const BIO_GRID_MIN = 240

// 首页「资料 + 头像」横带高（与 CSS 的 --bio-head-h 落成同一高度；只多不少，宁少装字不裁字）
function bioHeadH(b) {
  // 字号放大时纸张逻辑宽度变窄（880/scale）→ 资料栏实际可用宽度随之变小，折行数按比例重算
  const infoW = Math.max(160, BIO_HEAD_INFO_W / (scaleVal.value || 1))
  const perLine = Math.max(6, Math.floor(infoW / 13))
  const lines = Math.max(1, Math.min(9, Math.ceil(textWeight(bioMeta(b)) / perLine)))
  const infoH = 24 + 8 + 36 + 6 + lines * Math.ceil(13 * 1.5)
  const content = Math.max(BIO_PORTRAIT_H, infoH)
  return Math.max(BIO_PORTRAIT_H + 2 * BIO_HEAD_PAD + 2 * BIO_HEAD_BORDER,
    Math.min(420, content + 2 * BIO_HEAD_PAD + 2 * BIO_HEAD_BORDER + 12))
}
function bioGridFirst(b) {
  return Math.max(BIO_GRID_MIN, BIO_PAGE_CONTENT_H - bioHeadH(b) - BIO_HEAD_GAP - BIO_FLAG_RESERVE)
}
function bioGridCont() {
  return Math.max(BIO_GRID_MIN, BIO_PAGE_CONTENT_H - BIO_FLAG_RESERVE)
}
// 正文分页：一传一页；过长续页。每竖行字数 = 该页竖格高 ÷ 实测每字步长 × 安全余量。
function splitBioCols(text, b) {
  const chars = Array.from(text)
  // 竖排字步进是 font-size 18px；不用行高32
  const step = 18
  const sc = scaleVal.value || 1
  const maxCols = LONG_COLS.value
  const pages = []
  let i = 0
  while (i < chars.length && pages.length < 500) {
    const isFirst = pages.length === 0
    const gridH = isFirst ? bioGridFirst(b) : bioGridCont()
    const cap = Math.max(6, Math.floor(gridH / step * 0.98 / sc))
    const cols = []
    while (cols.length < maxCols && i < chars.length) {
      cols.push({ text: chars.slice(i, i + cap).join('') })
      i += cap
    }
    if (cols.length) pages.push({ cols, maxCols, gridH })
  }
  return pages
}
const bioPages = computed(() => {
  const pages = []
  for (const b of branchBiographies.value) {
    const text = cleanText(b.content || '')
    if (!text) continue   // 正文为空的传记不占页（与牒记式一致）
    const list = splitBioCols(text, b)
    const total = Math.max(1, list.length)
    const headH = bioHeadH(b)
    list.forEach((p, pi) => {
      const isFirst = pi === 0
      pages.push({
        bio: b, cols: p.cols, maxCols: p.maxCols, isFirst, seq: pi, total,
        headH: isFirst ? headH : 0,
        gridH: p.gridH
      })
    })
  }
  return pages
})

function parseWords(arr) {
  try {
    return typeof arr === 'string' ? JSON.parse(arr) : (arr || [])
  } catch {
    return []
  }
}

// ===== 汉字数字 =====
const D = ['', '一', '二', '三', '四', '五', '六', '七', '八', '九']
function cn(n) {
  n = Number(n)
  if (n == null || isNaN(n)) return ''
  if (n === 0) return '〇'
  if (n <= 10) return ['', '一', '二', '三', '四', '五', '六', '七', '八', '九', '十'][n]
  if (n < 20) return '十' + (n % 10 ? D[n % 10] : '')
  if (n < 100) return D[Math.floor(n / 10)] + '十' + (n % 10 ? D[n % 10] : '')
  return String(n)
}

// 排行序（数字，用于长幼排序）
function rankOrder(r) {
  if (!r) return null
  const m = String(r).match(/(\d+)/)
  return m ? parseInt(m[1], 10) : null
}

function fullName(m) {
  return m.name || (m.surname || '') + (m.given_name || '')
}

// 條首小字「某公第幾子」→ 固定兩段：{ f: 父名＋公, r: 行第 }（缓存，模板每格只解析一次）
//   兩段各自成豎列（右＝父名，左＝行第），格內不換行；字號由 CSS 按格高固定給（屏幕 14px / 打印 12px），
//   不再按字數縮字號。
const relPartCache = new Map()
function relParts(rel) {
  const k = String(rel == null ? '' : rel)
  let v = relPartCache.get(k)
  if (!v) { v = relHeadPartsOfText(k); relPartCache.set(k, v) }
  return v
}

// ===== 谱书标题：支谱名 + 支谱 =====
const familyTitle = computed(() => {
  if (branchName.value) return branchName.value + '支譜'
  if (surname.value) return surname.value + '氏支譜'
  return '支譜'
})

// ===== 接真实成员数据（/members/with-spouses）=====
// 结构对齐原写死 PEOPLE：gen/rel/name/elder/spouses[{name,children[]}]/birth/death/lifespan/origin/marriedOut/detail
// 谱序：世代升序，代内 DFS 先根序（长房先、男先女后、兄弟按排行长幼）
const people = computed(() => {
  const list = members.value
  if (!list.length) return []

  // 建索引：挂父子、区分儿女
  const idMap = {}
  list.forEach(m => {
    idMap[m.id] = { ...m, sons: [], daughters: [] }
  })

  list.forEach(m => {
    if (m.father_id && idMap[m.father_id]) {
      const fa = idMap[m.father_id]
      if (m.gender === 2) fa.daughters.push(idMap[m.id])
      else fa.sons.push(idMap[m.id])
    }
  })

  // 兄弟排序：排行数字优先，其次 id
  function sortSib(arr) {
    arr.sort((a, b) => {
      const ra = rankOrder(a.rank), rb = rankOrder(b.rank)
      if (ra != null && rb != null) return ra - rb
      if (ra != null) return -1
      if (rb != null) return 1
      return a.id - b.id
    })
  }
  Object.values(idMap).forEach(n => { sortSib(n.sons); sortSib(n.daughters) })

  // 找根：father_id 不在成员表中的，视为始祖/断代根
  const roots = list
    .filter(m => !(m.father_id && idMap[m.father_id]))
    .map(m => idMap[m.id])
  roots.sort((a, b) => a.generation - b.generation || a.id - b.id)

  // 全谱 DFS 先根序：父先于子，长房先于幼房，男先女后
  const order = []
  const dfs = (node) => {
    order.push(node)
    ;[...node.sons, ...node.daughters].forEach(dfs)
  }
  roots.forEach(dfs)

  // 断代链未访问到的成员补上
  const visited = new Set(order.map(n => n.id))
  list.forEach(m => { if (!visited.has(m.id)) order.push(idMap[m.id]) })

  // 按世代分组，组内保持 DFS 序
  const genMap = {}
  order.forEach(node => {
    if (!genMap[node.generation]) genMap[node.generation] = []
    genMap[node.generation].push(node)
  })
  const genList = Object.keys(genMap).map(Number).sort((a, b) => a - b)

  // 组装牒记人物（结构与原 PEOPLE 一致）
  const out = []
  genList.forEach(g => {
    genMap[g].forEach((node, idx) => {
      // 子女（子先女后，按排行），挂到第一个配偶
      const children = [...node.sons, ...node.daughters].map(c => fullName(c))
      const spouses = (node.spouses || []).map(s => ({ name: fullName(s), children: [] }))
      if (children.length) {
        if (spouses.length) spouses[0].children = children
        else spouses.push({ name: '', children })
      }

      out.push({
        gen: node.generation,
        // 條首小字「某公第幾子」：行第按同父同胞（同性别）排序位置取（长先幼后）
        rel: (() => {
          const dad = node.father_id ? idMap[node.father_id] : null
          if (!dad) return '支祖'
          const nm = dad.given_name || dad.name || ''
          const sib = node.gender === 2 ? dad.daughters : dad.sons
          const rankIdx = sib.findIndex((x) => x.id === node.id) + 1
          return relHeadOf(nm, rankIdx, node.gender) || (nm + (node.gender === 2 ? '之女' : '之子'))
        })(),
        name: fullName(node),
        elder: idx === 0,
        spouses,
        birth: node.birth_lunar || node.birth_gregorian || '',
        death: node.death_lunar || node.death_gregorian || '',
        lifeDates: formatLifeDates(node),
        known: memberKnown(node),
        lifespan: node.lifespan ? cn(node.lifespan) : '',
        origin: node.native_place || '',
        marriedOut: node.married_out || '',
        burial: node.burial_place || '',
        noHeir: node.no_heir === 1,
        lostContact: node.lost_contact === 1,
        detail: node.biography || '',
        src: sourceFoot(sourceMap.value, node.id)
      })
    })
  })
  return out
})

// 资料文字：竖排连贯而下（与原 demo 一致：配偶X子女Y、適、生卒、葬、小传）
function detailText(p) {
  // 迁居外地失联：生殁葬、子嗣缺啥补啥 + 失考
  if (p.lostContact) {
    const ls = []
    const hasKids = (p.spouses || []).some(s => s.children && s.children.length)
    ls.push(lostContactText(p.known || {}, hasKids))
    if (p.detail) ls.push(p.detail)
    return ls.join(' ')
  }
  const parts = []
  if (p.spouses) {
    for (const s of p.spouses) {
      let t = `配偶${s.name}`
      if (s.children && s.children.length) t += ' 子女' + s.children.join('、')
      parts.push(t)
    }
  }
  if (p.marriedOut) parts.push(`適${p.marriedOut}`)
  if (p.lifeDates && p.lifeDates !== '生卒未詳') parts.push(p.lifeDates + (p.lifespan ? ` 享年${p.lifespan}歲` : ''))
  else if (p.lifespan) parts.push(`享年${p.lifespan}歲`)
  if (p.burial) parts.push(`葬${p.burial}`)
  if (p.detail) parts.push(p.detail)
  // 无嗣：名下无子女者，资料格末尾注「無嗣」
  const hasKids = (p.spouses || []).some(s => s.children && s.children.length)
  if (p.noHeir && !hasKids) parts.push(NO_HEIR_TRAD)
  return parts.join(' ')
}

// ===== 分页：每表 13 列（谱名 1 + 人物 12），自右而左；排满一表续下一表，空格补框线 =====
const BASE_COLS = 13        // 标准字号：谱名 1 + 人物 12
const PERSON_COLS = computed(() => Math.max(4, Math.round((BASE_COLS - 1) / Math.pow(scaleVal.value, 2))))
const TOC_FOOT = ''
const tocEntries = computed(() => {
  // 圖片展示頁排在目錄之後、譜頭之前 → 先佔掉它們的頁序（空條目 0 頁）
  const galSums = entrySummaries(galleryEntries.value)
  let galCursor = 0
  for (const gs of galSums) { gs.start = 1 + galCursor; galCursor += gs.pages }
  const galN0 = galCursor
  let leaf = 1 + galN0   // 下一页的页码（封面、目录不参与编号 → 有图时图片展示＝第 1 页，谱头顺延）
  let bookNo = 0        // 已用掉的書頁號（counter(sheetpage)）
  // ① 封面、② 目錄（本頁）都不参与页码编号（属主要求）

  const catOrder = Object.keys(CAT_ORDER)
  const catStat = {}
  catOrder.forEach(c => { catStat[c] = { start: null, blocks: 0, bookStart: null, bookEnd: null, firstIsGrid: false } })

  // ③ 譜頭：逐塊累加（凡例短篇表是 .sheet → 佔書頁號；長篇是 .preface → 不佔）
  for (const pg of prefacePages.value) {
    const isGrid = pg.type === 'grid'
    const cats = (isGrid ? (pg.cats || []) : [pg.article.category]).filter(c => catStat[c])
    for (const c of cats) {
      const st = catStat[c]
      if (st.start == null) { st.start = leaf; st.firstIsGrid = isGrid }   // 该类的「首页」是不是 .sheet
      if (isGrid && st.bookStart == null) st.bookStart = bookNo + 1
    }
    // 一頁只給「首個類別」計一頁（短篇譜序與凡例同頁時不重複計）
    if (cats.length) catStat[cats[0]].blocks += 1
    if (isGrid) {
      bookNo += 1
      for (const c of cats) catStat[c].bookEnd = bookNo
    }
    leaf += 1
  }

  // ④ 字輩派語：每支 1 頁（.preface.genword，非 .sheet）
  const genList = generationWords.value || []
  const genStart = genList.length ? leaf : null
  leaf += genList.length

  // ④′ 五服圖（字輩之後、名人传记之前）：固定 1 頁
  const fiveFuStart = leaf
  leaf += 1

  // ④″ 家族大事记（五服圖之後、名人传记之前）
  const eventStart = eventPages.value.length ? leaf : null
  leaf += eventPages.value.length

  // ④‴ 名人传记（五服圖之後、世系之前）：一传一页（过长续页）
  const bioStart = bioPages.value.length ? leaf : null
  leaf += bioPages.value.length

  // ④‴ 祠宇志 → 茔墓志（名人传记之后、世系之前）：每页一个物理页
  const shrineStart = shrinePageList.value.length ? leaf : null
  leaf += shrinePageList.value.length
  const graveStart = gravePageList.value.length ? leaf : null
  leaf += gravePageList.value.length

  // ⑤ 世系牒記表（本房支）：每個 .sheet 一頁
  const sec = { pages: 0, start: leaf, bookStart: bookNo + 1, bookEnd: bookNo, genMin: null, genMax: null }
  for (const sh of sheets.value) {
    if (sec.pages === 0) { sec.start = leaf; sec.bookStart = bookNo + 1 }
    sec.pages += 1
    sec.bookEnd = bookNo + 1
    for (const p of (sh.persons || [])) {
      const g = Number(p.gen)
      if (!Number.isFinite(g)) continue
      if (sec.genMin == null || g < sec.genMin) sec.genMin = g
      if (sec.genMax == null || g > sec.genMax) sec.genMax = g
    }
    bookNo += 1
    leaf += 1
  }

  const rows = []
  // 谱序/凡例/祖训/族规（按 CAT_ORDER；同名多篇合并为一行并标篇数）
  for (const c of catOrder) {
    const st = catStat[c]
    if (!st || st.start == null) continue
    const nArt = prefaceArticles.value.filter(a => a.category === c && cleanText(a.content)).length
    const notes = []
    if (nArt > 1) notes.push(`${nArt} 篇`)
    if (st.blocks > 1) notes.push(`共 ${st.blocks} 页`)
    else if (st.blocks === 0) notes.push('与上篇同页')
    rows.push({
      name: c,
      note: notes.join(' · '),
      // 只标该内容所在的第一页（全书页序，封面起算）；下一节的起始页即上一节的结束位置
      page: `第 ${st.start} 页`, pageNo: st.start
    })
  }
  // 字輩派語（標支數）
  if (genList.length) {
    const names = genList.map(g => g.branch || '总谱')
    rows.push({
      name: '字辈派语',
      note: `${names.length} 支${names.length <= 3 ? '：' + names.join('、') : ''}`,
      page: `第 ${genStart} 页`, pageNo: genStart
    })
  }
  // 五服圖（譜頭文化之後、世系之前）
  rows.push({
    name: '五服圖', note: '本宗九族服制',
    page: `第 ${fiveFuStart} 页`, pageNo: fiveFuStart
  })
  // 家族大事记（五服圖之後、名人传记之前）
  if (eventPages.value.length) {
    rows.push({ name: '家族大事记', note: `共 ${eventPages.value.length} 页`, page: `第 ${eventStart} 页`, pageNo: eventStart })
  }
  // 名人传记（五服圖之後、世系之前）：只标起始页 + 共几页
  if (bioPages.value.length) {
    rows.push({
      name: '名人传记', note: `共 ${bioPages.value.length} 页`,
      page: `第 ${bioStart} 页`, pageNo: bioStart
    })
  }
  // 祠宇志 / 茔墓志（名人传记之后、世系之前）
  if (shrinePageList.value.length) {
    rows.push({ name: '祠宇志', note: `共 ${shrinePageList.value.length} 页`, page: `第 ${shrineStart} 页`, pageNo: shrineStart })
  }
  if (gravePageList.value.length) {
    rows.push({ name: '茔墓志', note: `共 ${gravePageList.value.length} 页`, page: `第 ${graveStart} 页`, pageNo: graveStart })
  }
  // 世系正文（本房支）：世代范围 + 页数（只标起始页）
  if (sec.pages > 0) {
    const notes = []
    if (branchName.value) notes.push(branchName.value)
    if (sec.genMin != null) {
      notes.push(sec.genMin === sec.genMax ? `${cn(sec.genMin)}世` : `${cn(sec.genMin)}世–${cn(sec.genMax)}世`)
    }
    notes.push(`共 ${sec.pages} 页`)
    rows.push({
      name: '世系',
      note: notes.join(' · '),
      page: `第 ${sec.start} 页`, pageNo: sec.start
    })
  }
  // 图片展示（谱头/世系之前）：**每个条目一行**（用条目标题；未填则「图片展示 1/2/3…」）
  const galRows = galSums.map((gs) => {
    const notes = [`${gs.count} 张`]
    if (gs.pages > 1) notes.push(`共 ${gs.pages} 页`)
    return { name: gs.title, note: notes.join(' · '), page: `第 ${gs.start} 页`, pageNo: gs.start }
  })
  if (galRows.length) rows.splice(0, 0, ...galRows)
  return rows.map((r) => {
    const actual = pageNoMap.value[r.pageNo]
    return actual ? { ...r, page: `第 ${actual} 页`, pageNo: actual } : r
  })
})


// ===== 修次（服务器设置，与封面/其它视图同一键 xiuci）=====
const XIUCI_OPTS = ['創修', '初修', '二修', '三修', '四修', '五修', '重修', '續修']
async function saveXiuci() {
  try {
    await api.put('/settings/family-surname', { xiuci: xiuci.value })
    // 立即重取设置，确保本页封面/牌记同步刷新
    const { data } = await api.get('/settings/family-surname')
    xiuci.value = data.xiuci || '重修'
    ElMessage.success('修次已保存：' + xiuci.value)
  } catch (e) {
    ElMessage.error('修次保存失败：' + (e?.response?.data?.error || e?.message || e))
  }
}


// ===== 圖片展示（多條目；localStorage 持久化；舊「單塊」數據自動遷移成 1 個條目；空條目不產頁）=====
import GalleryDialog from '../components/GalleryDialog.vue'
import { createGalleryStore, buildEntryPages, entrySummaries } from '../utils/galleryStore'
import { fetchMediaImages, fetchMemberImages } from '../utils/galleryPool'
import { onGalleryImgError } from '../utils/thumb'

const GALLERY_KEY = 'branchGallery:' + branchId
const gallery = createGalleryStore(GALLERY_KEY)
const galleryEntries = gallery.entries
const galleryOpen = ref(false)
const galleryPool = ref([])
const galleryPoolLoading = ref(false)
const galleryImageTotal = computed(() => galleryEntries.value.reduce((n, e) => n + (e.images ? e.images.length : 0), 0))
const galleryPages = computed(() => buildEntryPages(galleryEntries.value))
const galleryPageCount = computed(() => galleryPages.value.length)
function loadGallery() { gallery.load() }
function onGalleryEntries(list) { galleryEntries.value = list; gallery.save() }
async function loadGalleryPool(force = false) {
  if ((galleryPool.value.length || galleryPoolLoading.value) && !force) return
  galleryPoolLoading.value = true
  try {
    const media = await fetchMediaImages()          // ① 家族多媒體：1 個請求，秒出
    galleryPool.value = media
    const atts = await fetchMemberImages(members.value || [])   // ② 成員附件：併發 8 路
    galleryPool.value = media.concat(atts)
  } finally {
    galleryPoolLoading.value = false
  }
}
async function galleryUpload(opt) {
  try {
    const fd = new FormData()
    fd.append('file', opt.file)
    fd.append('category', '照片')
    fd.append('description', opt.file.name || '')
    fd.append('branch_id', String(Number(branchId) || 0))   // 支谱图片上传：归属本房支
    if (members.value && members.value[0]) fd.append('member_id', members.value[0].id)
    await api.post('/upload', fd)
    ElMessage.success('图片已上传')
    galleryPool.value = []
    await loadGalleryPool(true)
    opt.onSuccess && opt.onSuccess({})
  } catch (e) {
    ElMessage.error('上传失败：' + (e?.response?.data?.error || e?.message || e))
    opt.onError && opt.onError(e)
  }
}
watch(galleryOpen, (v) => { if (v) loadGalleryPool() })

// ===== 回到頂部（右下角懸浮按鈕）=====
// 本視圖的滾動容器是 .book（overflow:auto，見 book ref），不是 window
const showTopBtn = ref(false)
const TOP_SHOW_AT = 200
function onScrollTopBtn() {
  const el = book.value
  showTopBtn.value = !!el && el.scrollTop > TOP_SHOW_AT
}
function backToTop() {
  const el = book.value
  if (el) { try { el.scrollTo({ top: 0, behavior: 'smooth' }) } catch (e) { el.scrollTop = 0 } }
  else { try { window.scrollTo({ top: 0, behavior: 'smooth' }) } catch (e) { window.scrollTo(0, 0) } }
}

// ===== 目錄點擊跳轉（锚点＝每頁塊上的 data-page-no，條目按自己的頁碼定位）=====
const highlightPageNo = ref(null)
let highlightTimer = null
function jumpToPage(pageNo) {
  if (pageNo == null) return
  const el = document.querySelector(`[data-page-no="${pageNo}"]`)
  if (!el) return
  try { el.scrollIntoView({ behavior: 'smooth', block: 'start' }) } catch (e) { el.scrollIntoView() }
  highlightPageNo.value = pageNo
  if (highlightTimer) clearTimeout(highlightTimer)
  // 平滑滚动可能耗时 1~2s：高亮保持 2.6s，确保滚动落位后仍能看到描边，再自动消失
  highlightTimer = setTimeout(() => { highlightPageNo.value = null }, 2600)
}

// ===== 打印纸型：统一由「默认 @page」给出（不再给元素挂命名页 page: dieji-sheet）=====
// 末尾空白页修复：命名页与默认页混用时，最后一个命名页块（牌記）结束后浏览器会补一个
// 「默认页盒」收尾（实测那页 MediaBox = A4 横版 841.92×594.96pt，而正文全是 A4 竖版）
// → 全文档只用默认 @page（A4 竖版 12mm，与原命名页取值逐项相同）后，页数 == 逻辑页数。
function applyDefaultPrintPage() {
  let el = document.getElementById('__branch-print-page')
  if (!el) {
    el = document.createElement('style')
    el.id = '__branch-print-page'
    document.head.appendChild(el)
  }
  el.textContent = '@page { size: A4 portrait; margin: 12mm; }'
}
onMounted(() => { applyDefaultPrintPage();  })
onBeforeUnmount(() => {
  const el = document.getElementById('__branch-print-page')
  if (el) el.remove()
})

async function exportPDF() {
  // 打印前确保所有字体（含用户上传的自定义字体）已加载，避免 PDF 里字体回退
  try { await document.fonts.ready } catch (e) {}
  applyDefaultPrintPage()
  window.print()
}

async function load() {
  // 载入房支信息（支谱名）
  try {
    const { data: bl } = await api.get('/branches')
    branchList.value = bl || []
    const b = (bl || []).find(x => String(x.id) === String(branchId))
    branchName.value = b ? b.name : ''
  } catch (e) { /* ignore */ }
  // 载入成员并过滤本房支（资料来源与成员并行取；失败返回空数组，不影响谱书主体）
  const [mv, srcList, bios] = await Promise.all([
    // skipBranchScope：本视图以 URL 的 branchId 为根自行过滤（顶部 current 可能停在别的房支）；
    // 含不含下级只取顶部 includeChildren，见 scopeBranchIds。
    api.get('/members/with-spouses', { skipBranchScope: true }),
    fetchAllSources(),
    api.get('/biographies').catch(() => ({ data: [] })),   // 名人传记（失败不影响谱书主体）
  ])
  membersRaw.value = mv.data || []
  biographies.value = Array.isArray(bios.data) ? bios.data : []
  sourceMap.value = groupSourcesByMember(srcList)
  try {
    const { data: fs } = await api.get('/settings/family-surname')
    surname.value = fs.family_surname || ''
    familyName.value = fs.family_name || ''
    tanghao.value = fs.tanghao || ''
    junwang.value = fs.junwang || ''
    zuji.value = fs.zuji || ''
    xiuci.value = fs.xiuci || '重修'   // 修次（服务器设置，默认「重修」）
  } catch (e) { /* ignore */ }
  // 谱头文化：谱序/凡例/祖训/族规 + 字辈派语
  try {
    const { data: arts } = await api.get('/articles')
    articles.value = arts
  } catch (e) { /* ignore */ }
  try {
    const { data: gws } = await api.get('/generation-words')
    generationWords.value = gws
  } catch (e) { /* ignore */ }
  try {
    const { shrines: ss, graves: gg } = await fetchShrineGrave()
    shrines.value = ss
    graves.value = gg
  } catch (e) { /* 祠宇/茔墓取数失败不影响主体 */ }
  try {
    const { data: evs } = await api.get('/events')
    events.value = Array.isArray(evs) ? evs : []
  } catch (e) { events.value = [] }
  await nextTick()
  measurePageNos()
}

onMounted(load)
</script>

<style scoped>
.demo-root { display: flex; flex-direction: column; height: 100%; }

.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.toolbar .title { font-size: 18px; margin-right: 8px; }
.scope-tag { margin-right: 8px; }
.hint { margin-left: auto; color: #8c5a3c; font-size: 12px; }

/* 隐藏的字体文件选择框 */
.font-file-input { display: none; }

/* 谱书容器 */
.book {
  flex: 1;
  overflow: auto;
  padding: calc(24px / var(--dieji-scale, 1));
  background: #f5f0e6;
  border: 1px solid #d8cdb8;
  border-radius: 8px;
  font-family: var(--dieji-font, "SimSun", "宋体", "STSong", serif);
  /* 字号缩放 × 预览缩放（预览缩放自动按屏幕 fit，可手动覆盖）；打印时预览缩放不生效 */
  zoom: calc(var(--dieji-scale, 1) * var(--preview-scale, 1));
}

/* ===== 封面（譜名頁） ===== */
.cover {
  width: 100%;
  width: max-content; min-width: calc(880px / var(--dieji-scale, 1)); max-width: none;
  margin: 0 auto calc(28px / var(--dieji-scale, 1));
  background: #fffdf5;
  border: 3px double #1a1a1a;   /* 雙邊框 */
  padding: calc(14px / var(--dieji-scale, 1));
}
.cover-inner {
  border: 1px solid #1a1a1a;
  min-height: calc(1180px / var(--dieji-scale, 1));
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
  padding: 60px 30px;
}
/* 堂號：上方居中 */
.cover-tanghao {
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: 26px;
  font-weight: bold;
  letter-spacing: 10px;
  color: #3a2a18;
  margin-bottom: 70px;
}
/* 書名：豎排大字居中 */
.cover-title {
  display: flex;
  justify-content: center;
}
.cover-title .vtitle {
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: var(--title-fs, 60px);
  font-weight: bold;
  letter-spacing: var(--title-ls, 22px);
  color: #1a1a1a;
  line-height: 1.1;
}
/* 底部信息：郡望 / 祖籍 / 歲次重修，豎排三欄 */
.cover-meta {
  margin-top: 52px;
  display: flex;
  flex-direction: row-reverse;
  gap: 40px;
}
.cover-meta .meta-line span {
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: 18px;
  letter-spacing: 6px;
  color: #3a2a18;
}

/* 谱头页：与封面同风格，双边框 */
.preface {
  width: 100%;
  width: max-content; min-width: calc(880px / var(--dieji-scale, 1)); max-width: none;
  margin: 0 auto calc(28px / var(--dieji-scale, 1));
  background: #fffdf5;
  border: 3px double #1a1a1a;
  padding: calc(14px / var(--dieji-scale, 1));
}
.preface-inner {
  border: 1px solid #1a1a1a;
  min-height: 0;
  padding: 0;
  box-sizing: border-box;
  overflow: hidden;
}
/* 祠宇志 / 茔墓志：文章页（两栏流），沿用既有页框/页高/书耳/页码 */
.preface.sg-page { width: calc(880px / var(--dieji-scale, 1)); min-width: 0; max-width: none;
  height: calc(1180px + 34px); }
.sg-inner { min-height: 0; height: 100%; padding: calc(24px / var(--dieji-scale, 1)) calc(30px / var(--dieji-scale, 1)); --sg-fs: 14px; }
.sg-page.sg-fs16 .sg-inner { --sg-fs: 16px; }   /* 祠宇/茔墓：与牒记竖版同步 14→16；大事记仍14 */
.epc-page .sg-inner { padding-left: 0; padding-right: 0; }   /* 大事记整页竖格：左右不留白 */
/* 圖片展示頁：
   ① .preface 必須是 flex 縱向容器、內框 flex:1（不能再 height:100%）：
      否則內框獨佔整頁高，標籤與頁碼被擠出頁外、被 overflow:hidden 裁掉。
   ② 紙張寬寫死成基準寬（＝其它頁的 min-width）：圖片的固有寬度會參與 width:max-content 計算，
      不寫死時整頁會被圖片撐到 2 倍寬。 */
.preface.gallery-page {
  display: flex;
  flex-direction: column;
  width: calc(880px / var(--dieji-scale, 1));
  min-width: 0;
  max-width: none;
  /* 固定頁高＝其他支譜書頁（版心 min-height 1180px ＋ .preface 內邊距 28 ＋ 雙邊框 6 ＝ 34px）：
     否則頁高被圖片撐高（實測 2 張 3505px、4 張 1810px）→ 格子無法固定。 */
  height: calc(1180px + 34px);
}
.preface.gallery-page > .preface-inner { flex: 1 1 auto; height: auto; min-height: 0; }
.preface-cat {
  text-align: center;
  font-size: 20px;
  letter-spacing: 6px;
  color: #8c5a3c;
  margin-bottom: 8px;
}
.preface-title {
  text-align: center;
  font-size: 30px;
  font-weight: bold;
  letter-spacing: 8px;
  color: #1a1a1a;
  margin-bottom: 6px;
}
.preface-meta {
  text-align: center;
  font-size: 13px;
  color: #8c5a3c;
  margin-bottom: 30px;
}

/* ===== 內封（書名頁）宿主：讓共用組件填滿既有封面版框（雙線邊框由 .cover 提供） ===== */
.it-host { padding: 0 !important; align-items: stretch !important; justify-content: stretch !important; }
.it-host > * { flex: 1 1 auto; min-height: 0; }

/* ===== 五服圖頁（服制圖）：整頁一幅 SVG 矢量圖（支谱用 tall 版式） ===== */
/* 屏幕：钉死與 .cover 同高的內框（min-height 1180 的盒模型），避免圖把紙張撐高 */
@media screen { .fivefu-page .ff-inner { height: 1180px; min-height: 0; } }
.fivefu-page .ff-inner { display: flex; flex-direction: column; gap: calc(10px / var(--dieji-scale, 1)); }
.ff-head { flex: 0 0 auto; display: flex; align-items: baseline; justify-content: center;
           gap: calc(16px / var(--dieji-scale, 1)); flex-wrap: wrap; }
.ff-title { font-size: calc(26px * var(--body-size, 1) / var(--dieji-scale, 1)); font-weight: 700;
            letter-spacing: 8px; color: #111; }
.ff-meta { font-size: calc(13px * var(--body-size, 1) / var(--dieji-scale, 1)); color: #6b5a44; letter-spacing: 1px; }
.ff-body { flex: 1 1 auto; min-height: 0; display: flex; align-items: center; justify-content: center; }

/* ===== 牌記頁（全書末頁）：不叫 .preface/.cover → 不參與 bookpage 計數 =====
   屏幕页高与其它整页一致（1180 内框 + 34 边距/边框 = 1214px）；打印页高 268mm（与 .preface 同）。
   修前屏幕/打印都写死 280mm：屏幕比其它页短 156px，打印 280mm > 可用 273mm → 牌記被挤到第二页。 */
.colo-page {
  width: var(--paper-w, calc(880px / var(--dieji-scale, 1)));
  margin: 0 auto calc(28px / var(--dieji-scale, 1));
  background: #fffdf5;
  border: 3px double #1a1a1a;
  padding: calc(14px / var(--dieji-scale, 1));
  box-sizing: border-box;
  height: calc(1180px + 34px);
  overflow: hidden;
}
.colo-inner { border: 1px solid #1a1a1a; height: 100%; }

/* ===== 目錄頁：沿用 .preface 的頁框與分頁（height 268mm/scale + break-after: page + page: dieji-sheet），
   只新增條目樣式；標題風格與本檔譜頭頁（.preface-title）一致（橫排、居中、大字距）。
   條目內用 flex 自適應撐開點線（不寫死寬度），隨 --dieji-scale 自動縮放。 ===== */
.toc-title {
  text-align: center;
  font-size: 30px;
  font-weight: bold;
  letter-spacing: 8px;
  color: #1a1a1a;
  margin-bottom: 18px;
}
.toc-list {
  min-height: 0;
  overflow: hidden;
  padding: 0 6px;
}
/* 條目：名稱＋小注（左）／點線引導（中，自適應撐開）／頁碼（右） */
/* 回到頂部：右下角固定，圓形；打印隱藏 */
.back-top { position: fixed; right: 24px; bottom: 24px; z-index: 2000; box-shadow: 0 4px 14px rgba(0,0,0,.22); }
@media (min-width: 768px) { .back-top { right: 44px !important; bottom: 44px !important; } }
.ph-fade-enter-active, .ph-fade-leave-active { transition: opacity .2s ease, transform .2s ease; }
.ph-fade-enter-from, .ph-fade-leave-to { opacity: 0; transform: translateY(8px); }
@media print { .back-top { display: none !important; } }
/* 圖片展示：格線佈局（頁面內容區內，不得溢出） */
.gallery-inner { display: flex; align-items: center; justify-content: center; padding: 10px; }
.gallery-doc { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; }
.gallery-grid { display: grid; gap: 14px; width: 100%; height: 100%; align-content: stretch; justify-items: stretch; align-items: stretch; }
.gallery-doc.g1 .gallery-grid { grid-template-columns: 1fr; grid-template-rows: 1fr; }
.gallery-doc.g2 .gallery-grid { grid-template-columns: 1fr; grid-template-rows: repeat(2, 1fr); }
.gallery-doc.g4 .gallery-grid { grid-template-columns: repeat(2, 1fr); grid-template-rows: repeat(2, 1fr); }
.gallery-doc.g6 .gallery-grid { grid-template-columns: repeat(2, 1fr); grid-template-rows: repeat(3, 1fr); }
.gallery-doc.g9 .gallery-grid { grid-template-columns: repeat(3, 1fr); grid-template-rows: repeat(3, 1fr); }
.g-cell { margin: 0; min-width: 0; min-height: 0; display: flex; flex-direction: column; align-items: center; justify-content: center;
  box-sizing: border-box; padding: 3px; background: #fbf8f1; }   /* 淡底＝讓「固定格子」看得見 */
.g-cell img { width: 100%; height: 100%; min-height: 0; flex: 0 1 auto; object-fit: contain;
  border: 1px solid #b9ad91; background: #fff; display: block; }   /* 格內等比縮放：不裁切、不拉伸（比例不同就留白） */
.g-cell figcaption { flex: 0 0 auto; width: 100%; height: 17px; line-height: 17px; margin-top: 3px;
  font-size: calc(14px * var(--cover-size, 1) / var(--dieji-scale, 1)); color: #333; text-align: center;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }   /* 格內預留圖說高度（固定 17px）：圖說再長也不壓到下一格 */
.g-cell.img-missing { background: repeating-linear-gradient(45deg,#f7f3ea,#f7f3ea 8px,#efe8da 8px,#efe8da 16px); }
.g-cell.img-missing img { display: none; }
.g-cell.img-missing::before { content: '图片不可用'; color: #b08840; font-size: 13px; line-height: 1.4; }
.toc-row.toc-clickable { cursor: pointer; border-radius: 4px; transition: background-color .15s ease; }
.toc-row.toc-clickable:hover { background: #f3e7cf; }
[data-page-no].page-flash { outline: 3px solid #b08840; outline-offset: 4px; }
@media print { [data-page-no].page-flash { outline: none !important; } }
.toc-row {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: 17px;
  line-height: 2.2;
  color: #1a1a1a;
}
.toc-name { flex: 0 0 auto; letter-spacing: 2px; }
.toc-note { flex: 0 0 auto; font-size: 12px; color: #8c5a3c; }
.toc-dots {
  flex: 1 1 auto;
  align-self: baseline;      /* baseline 對齊時空盒基線在底邊 → 點線正好落在字腳 */
  min-width: 20px;
  border-bottom: 1px dotted #b08840;
}
.toc-page { flex: 0 0 auto; white-space: nowrap; letter-spacing: 1px; }
.toc-foot {
  margin-top: 12px;
  padding-top: 6px;
  
  font-size: 12px;
  line-height: 1.6;
  color: #8c5a3c;
}

/* ===== 凡例短篇：与成员牒记同一张表（每条占一整列：标题→姓名格，正文→资料格格内竖排换行） ===== */
/* 列/格全部继承 .person-col / .cell-rel / .cell-name / .cell-detail，保证高度、框线、页高与成员表完全一致 */
/* 凡例标题格文字顶部对齐（居中会显得标题长短不一时参差不齐） */
.rule-sheet .cell-name { align-items: flex-start; justify-content: center; padding-top: 10px; }
.rule-name { white-space: normal; }
/* 凡例短表：正文/标题统一 18px */
.rule-sheet .cell-detail { font-size: 18px; line-height: 1.5; }
.rule-sheet .cell-name .rule-name { font-size: 18px; line-height: 1.3; }
/* 凡例列已取消最上面的关系格；标题格高度由每页最长标题算出（--rule-name-h），整页等高 */
.rule-sheet .rule-col .cell-name,
.rule-sheet .blank .cell-name {
  flex: 0 0 var(--rule-name-h, 80px);
}
.cell-name .rule-name {
  /* 凡例标题比人名长（如「十三、本支源流及修谱缘起」），姓名格内竖排多竖行 */
  font-size: 17px;
  line-height: 1.3;
  letter-spacing: 1px;
}

/* ===== 长篇竖行容量测量探针（隐藏） ===== */
.measure-probe {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  max-width: calc(880px / var(--dieji-scale, 1));
  visibility: hidden;
  pointer-events: none;
  z-index: -1;
}
/* 探针外层裁剪盒：探针不吃 zoom、宽度可达 880px，且绝对定位在文档原点；
   不裁剪会把文档 scrollWidth 撑到 880px（手机 390 宽时多出可左右拖走的空白）。
   height:0 + overflow:hidden 只裁溢出，不参与布局，探针自身量测值不变。 */
.probe-clip {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  height: 0;
  overflow: hidden;
}
.measure-probe .vcol { width: 64px; }

/* ===== 长篇整页（谱序/祖训/族规）：竖排竖行、定列不溢出 ===== */
.vgrid {
  display: flex;
  flex-direction: row-reverse;
  align-items: stretch;
}
.bio-long .vcol { font-size: 18px; line-height: 2.0; }
.vcol {
  writing-mode: vertical-rl;
  text-orientation: upright;
  flex: 1 1 0;
  min-width: 0;
  padding: 6px 2px;
  font-size: 16px;
  line-height: 2.0;
  color: #222;
  overflow: hidden;
  word-break: break-all;
  text-align: start;
  -webkit-text-size-adjust: 100%;
}

/* ===== 凡例长条目（整页竖排）：复用族规定列等分竖排（稳定不重叠），标题竖排作右起首列 ===== */
.rule-vgrid {
  align-items: stretch;
}
.rule-vgrid .vcol {
  flex: 1 1 0;                       /* 与族规一致：等分整宽，绝不坍缩叠列 */
  min-width: 0;
  max-width: none;
  padding: 6px 8px;
  font-size: 16px;
  line-height: 1.9;
  overflow: hidden;
}
.rule-vcol-title {
  flex: 1 1 0 !important;
  max-width: none !important;
  font-size: 19px !important;
  font-weight: bold;
  letter-spacing: 2px;
  padding: 6px 10px 6px 14px !important;
  color: #1a1a1a;
  line-height: 1.6 !important;
  border-left: 1px solid #1a1a1a;    /* 标题列与正文之间竖界（右缘起首） */
}

/* 字辈派语页 —— 横排布局（仅此页横排，其余谱页仍竖排）
   与其它整页同高：内框 1180px ＋ 内外边距/边框 34px = 1214px（与 .cover/.preface 完全一致）。
   修前此页 .preface-inner 是 min-height:auto，页面高度随字数变化 → 字辈多时预览被撑得很长。 */
.preface.genword {
  /* 与其它页同宽（.preface 基类是 width:max-content；字辈诗是一整行长文，会把 max-content
     撑到 1780px —— 修前预览里这一页特别长/宽、与其它页宽度不一致。这里钉回标准纸宽，诗自动折行）。 */
  width: calc(880px / var(--dieji-scale, 1));
  min-width: 0;
  max-width: 100%;
  height: calc(1180px + 34px);
}
.genword .preface-inner {
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: center;
  box-sizing: border-box;
  overflow: hidden;
}
.genword-branch {
  text-align: center;
  font-size: 20px;
  font-weight: bold;
  color: #3a2a18;
  margin-bottom: 14px;
}
.genword-poem {
  text-align: center;
  font-size: 20px;
  letter-spacing: 6px;
  color: #1a1a1a;
  line-height: 1.8;
  margin: 0 auto 26px;
  max-width: 100%;
  overflow-wrap: anywhere;   /* 一整行长诗按纸宽折行，不把页撑宽 */
}
.genword-chars {
  display: grid;
  /* 与打印同一列数（8 列）：64 字＝8 行，固定页高内排得下；修前 auto-fit 列数随宽度变化，行数不可控 */
  grid-template-columns: repeat(8, 1fr);
  gap: 10px;
  justify-content: center;
  max-width: 600px;
  margin: 0 auto;
}
.genword-char {
  border: 1px solid #1a1a1a;
  aspect-ratio: 1 / 1.05;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  color: #1a1a1a;
}
.genword-char small { font-size: 11px; color: #8c5a3c; margin-top: 2px; }

/* 每页网格：flex 行 + row-reverse（首项=谱名居最右），列等高、等宽 */
/* ===== 譜書頁碼（屏幕預覽與列印都顯示）：置於框內右下角；封面、目錄不參與編號 ===== */
.book { counter-reset: bookpage; }
.preface:not(.toc), .sheet { position: relative; }
/* 屏幕预览与列印都要自增（只在列印规则里写会导致预览里同类页页码全都相同） */
.preface:not(.toc), .sheet { counter-increment: bookpage; }
/* 表格页：贴表格外框内侧右下角；谱头页：贴内框（.preface-inner）内侧右下角，两者观感一致 */
.preface:not(.toc) .preface-inner { position: relative; }
.sheet::after,
/* 续页 n/N：左侧，与原右下角页码分开 */
.preface-inner[data-cont]::before {
  content: attr(data-cont);
  position: absolute; left: 3mm; bottom: calc(2.5mm / var(--dieji-scale, 1));
  font-size: calc(10pt / var(--dieji-scale, 1)); line-height: 1; color: #000; text-align: left;
}
.preface:not(.toc) .preface-inner::after {
  content: counter(bookpage);
  position: absolute;
  right: calc(3mm / var(--dieji-scale, 1));
  bottom: calc(2.5mm / var(--dieji-scale, 1));
  font-size: calc(10pt / var(--dieji-scale, 1));
  color: #000;
  line-height: 1;
  text-align: right;
}
.sheet {
  display: flex;
  flex-direction: row-reverse;
  align-items: stretch;
  width: 100%;
  width: max-content; min-width: calc(880px / var(--dieji-scale, 1)); max-width: none;
  /* 屏幕页高与封面/谱头/字辈/牌記一致（1180 内框 + 34 边距/边框 = 1214px）；
     打印侧仍由 @media print 的 256mm 固定页高接管（列容量按打印页高算，不受屏幕页高影响）。 */
  height: calc(1180px + 34px);
  box-sizing: border-box;
  margin: 0 auto calc(28px / var(--dieji-scale, 1));
  background: #fffdf5;
  border: 1.5px solid #1a1a1a;
}
.sheet:last-child { margin-bottom: 0; }

/* 谱名列（最右）：竖排 */
.col-title {
  flex: 0 0 42px;
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: 15px;
  font-weight: bold;
  letter-spacing: 4px;
  text-align: center;
  padding-top: 14px;
  background: #faf6ec;
  border-left: 1px solid #1a1a1a;
}

/* 人物列：竖向 flex 三格（关系/姓名/资料），竖格边框（左缘+格间横线） */
.person-col {
  flex: 1 1 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  border-left: 1px solid #1a1a1a;
}

/* 关系小格：固定高度（对齐所有列的行线），水平流居中，内部文字竖排 */
.cell-rel {
  flex: 0 0 80px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: #333;
  border-bottom: 1px solid #1a1a1a;
  overflow: hidden;
}
/* 條首小字＝固定兩段（屬主口徑）：右＝父名＋公，左＝行第。兩段各自成豎列、格內不換行；
   字號固定（不隨字數縮、不隨正文字號放大，保證 5 字長父名也不裁字）：
   屏幕格高 80px → 14px；打印格高 62px → 12px（見打印塊）。 */
.cell-rel .rel2 {
  display: flex;
  flex-direction: row-reverse;
  align-items: center;
  justify-content: center;
  gap: 2px;
  height: 100%;
  max-width: 100%;
  max-height: 100%;
  overflow: hidden;
}
.cell-rel .rel2 > span {
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: min(calc(14px * var(--body-size, 1)), 14px);
  line-height: 1;
  padding: 0;
  letter-spacing: 0;
  white-space: nowrap;
}

/* 姓名大字格：固定高度，居中 */
.cell-name.cont-name .vtext { color: #8a7a5a; }
.cell-name {
  flex: 0 0 96px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 19px;
  font-weight: bold;
  letter-spacing: 2px;
  border-bottom: 1px solid #1a1a1a;
}

/* 竖排文字（在水平流格内竖排，用于关系/姓名的居中） */
.vtext {
  writing-mode: vertical-rl;
  text-orientation: upright;
  line-height: 1.35;
  padding: 4px;
}

/* 资料格：占满剩余高度（flex:1 给确定高度），竖排文字从格顶起、
   向下排、满列向左换列、新列从顶起；黑条是首个行内块、贴右缘格顶 */
.cell-detail {
  writing-mode: vertical-rl;
  text-orientation: upright;
  flex: 1 1 0;
  min-height: 0;
  overflow: hidden;
  font-size: calc(18px * var(--body-size, 1));
  color: #222;
  line-height: 1.5;
  padding: 6px 4px;
}

/* 資料來源小字腳注：接在资料格末尾（同一條竖列 inline 续写），0.72em 与资料格字号同比缩放，
   格高/格宽/列数一律不动；无来源时不渲染（模板 v-if），不留空行。 */
.src-note { font-size: 0.72em; color: #4a4a4a; letter-spacing: 0; }

/* 世代黑条：竖排流首个行内块（贴右缘、格顶起），资料文字接在它下方 */
.gen-bar {
  display: inline-block;
  background: #111;
  color: #fff;
  font-size: 18px;
  font-weight: bold;
  letter-spacing: 2px;
  padding: 6px 2px;
  margin: 0 0 0 8px;
  white-space: nowrap;
}

/* 空白列：补满框线 */
.person-col.blank { background: transparent; }

/* ============ 移动端 ============ */
@media (max-width: 767px) {
  .book { padding: 10px 6px; }
  .sheet { max-width: none; }
  .col-title { font-size: 12px; letter-spacing: 2px; flex-basis: 34px; }
  .cell-rel { font-size: 10px; }
  .cell-name { font-size: 15px; letter-spacing: 1px; }
  .cell-detail { font-size: 18px; line-height: 1.5; min-height: 120px; }
  .cell-name .rule-name { font-size: 18px; }
  .gen-bar { font-size: 18px; }
  /* 长篇竖行手机端字号略收，锁定不被浏览器自动放大，配合 0.88 安全余量不裁字 */
  .vcol { font-size: 14px; line-height: 1.9; -webkit-text-size-adjust: 100%; }
  /* 字辈派语手机端：横排紧凑，格子缩小保证一整页放得下、不裁底部格子 */
  .genword .preface-inner { padding: 28px 18px; overflow: visible; }
  .genword-branch { font-size: 17px; margin-bottom: 10px; }
  .genword-poem { font-size: 16px; letter-spacing: 3px; margin-bottom: 18px; line-height: 1.7; }
  .genword-chars { grid-template-columns: repeat(4, 1fr); gap: 8px; max-width: 360px; }
  .genword-char { font-size: 18px; }
  .genword-char small { font-size: 10px; }
}

/* ============ 打印 / 导出 PDF（scoped 部分） ============ */
@media print {
  .bio-inner { padding-top: 8px !important; }
  /* 谱头/大事记：只去掉 sg-inner 上 padding（24px≈6.35mm），保留 .preface 的纸张内边距 14px */
  .demo-root .epc-page .sg-inner,
  .demo-root .epc-page.sg-page .sg-inner { padding: 0 !important; }
  .epc-page .epc, .epc-page .epc-grid { margin-top: 0 !important; }
  .preface { box-sizing: border-box !important; }
  .preface-inner { height: 100% !important; min-height: 0 !important; box-sizing: border-box !important; }
  .demo-root { height: auto; display: block; }
  .book {
    overflow: visible;
    padding: 0;
    background: #fff;
    border: none;
    border-radius: 0;
    display: block;
    counter-reset: sheetpage bookpage;
    /* 打印只吃字号缩放，不吃预览缩放（屏幕专用的 --preview-scale 必须清掉） */
    zoom: 0.802 !important;
    transform: none !important;
    margin-left: auto !important;
    margin-right: auto !important;
  }
  /* min-width:0 必要：屏幕样式里 .cover/.preface/.sheet 有 min-width: calc(880px/scale)（手機「適應屏幕」用），
     min-width 會壓過 width/max-width:100%，不清掉的話列印時紙張被撐到 880px（232mm）> A4 內容區，右緣溢出。 */
  .cover, .preface, .sheet { min-width: 0 !important; }
  .preface.gallery-page { width: 100% !important; max-width: 100% !important; }
  /* 統一：A4 直印內容區固定 265mm（頁高297-上下邊距32），手機/桌面一致不拆頁 */
  /* 封面獨佔一頁；固定頁高 + 塊級流，避免手機 WebView 把封面拆成兩頁 */
  .cover {
    max-width: none;
    width: 100%;
    margin: 0;
    height: calc(268mm / var(--dieji-scale, 1));
    box-sizing: border-box;
    page-break-after: always;
    break-after: page;
    page-break-inside: avoid;
    break-inside: avoid;
  }
  .cover-inner {
    display: block;
    min-height: 0;
    height: auto;
    padding: 0;
    box-sizing: border-box;
    text-align: center;
  }
  /* 內封（書名頁）：內框必須撐滿整頁，豎排書名才居中、藏板才落在右下角 */
  .inner-cover .cover-inner {
    display: flex;
    height: 100%;
    min-height: 0;
    padding: 0;
    page-break-inside: avoid;
    break-inside: avoid;
  }
  .cover-tanghao {
    display: inline-block;
    margin: calc(22mm / var(--dieji-scale, 1)) 0 0;
  }
  .cover-title {
    display: block;
    margin-top: calc(18mm / var(--dieji-scale, 1));
  }
  .cover-meta {
    margin-top: calc(24mm / var(--dieji-scale, 1));
    justify-content: center;
  }
  /* 谱头页：每个排版块（网格页或长篇页）独占一页 */
  .preface {
    max-width: none;
    width: 100%;
    margin: 0;
    height: calc(268mm / var(--dieji-scale, 1));
    box-sizing: border-box;
    page-break-after: always;
    break-after: page;
    page-break-inside: avoid;
    break-inside: avoid;
  }
  .preface-inner {
    display: block;
    min-height: 0;
    height: 100%;
    padding: calc(16mm / var(--dieji-scale, 1)) calc(14mm / var(--dieji-scale, 1));
    box-sizing: border-box;
  }
  /* 长篇页：右对齐（标题居中不变，正文竖排贴右） */
  .preface-long .preface-inner {
    display: block;
  }
  /* 凡例表完全复用成员表打印样式（256mm 页高 / rel 62px / name 78px / detail flex:1 裁切） */
  /* 长篇竖行：占满框内高度，定列裁切，绝不溢出框线 */
  .vgrid {
    height: calc(190mm / var(--dieji-scale, 1));
    box-sizing: border-box;
  }
  .vcol { overflow: hidden; font-size: 11px; line-height: 1.7; padding: 4px 1px; }
  .rule-sheet .cell-name { padding-top: 4px; }
  .genword-poem { max-height: none; }
  /* 字辈派语：固定一页高（与其它整页同为 268mm），内容在框内居中；不再 height:auto
     （height:auto 时字辈多会把页撑高 → 预览/打印长度不可控）。 */
  .preface.genword {
    height: calc(268mm / var(--dieji-scale, 1));
    min-height: 0;
    padding: 0;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .preface.genword .preface-inner {
    height: 100%;
    min-height: 0;
    overflow: hidden;
    padding: calc(20mm / var(--dieji-scale, 1)) calc(16mm / var(--dieji-scale, 1));
    width: 100%;
  }
  .genword-chars { grid-template-columns: repeat(8, 1fr); gap: 6px; max-width: none; }
  .genword-char { font-size: 18px; }
  .genword-poem { font-size: 17px; letter-spacing: 4px; margin-bottom: calc(14mm / var(--dieji-scale, 1)); }
  .genword-branch { font-size: 18px; margin-bottom: calc(8mm / var(--dieji-scale, 1)); }
  /* 牒記表：每個 sheet 固定一頁高（留底部空白放頁碼），強制每表一頁 */
  .sheet {
    max-width: none;
    width: 100%;
    height: calc(256mm / var(--dieji-scale, 1));
    margin: 0;
    box-sizing: border-box;
    align-items: stretch;
    position: relative;
    page-break-after: always;
    break-after: page;
    page-break-inside: avoid;
    break-inside: avoid;
    counter-increment: sheetpage bookpage;
  }
  .sheet:last-child {
    page-break-after: auto;
    break-after: auto;
  }
  /* 牌記（全書末頁）：另起一頁、頁後不再分頁；页高与其它整页同为 268mm（可用区 273mm） */
  .colo-page {
    max-width: none;
    width: 100%;
    margin: 0;
    height: calc(268mm / var(--dieji-scale, 1));
    box-sizing: border-box;
    page-break-before: always;
    break-before: page;
    page-break-after: auto;
    break-after: auto;
    page-break-inside: avoid;
    break-inside: avoid;
  }
  .colo-inner { height: 100%; }
  /* 固定頁框內小注豎排自適應；去掉列的 break-inside（手機引擎會因此把列推到下一頁） */
  .person-col {
    page-break-inside: auto;
    break-inside: auto;
    overflow: hidden;
  }
  .cell-detail {
    min-height: 0;
    flex: 1 1 0;
    overflow: hidden;
    font-size: 18px;
    line-height: 1.5;
    padding: 6px 4px;
  }
  /* 列的關係/姓名格打印時略壓縮，給小注騰空間 */
  .cell-rel { flex-basis: 62px; }
  .cell-name { flex-basis: 78px; }
  /* 條首小字兩段：打印格高 62px → 字號 12px（最長一段 5 字＝60px，不裁字）。 */
  .cell-rel .rel2 > span { font-size: 12px; }
}

/* ===== 名人传记页（支谱）：照牒记式竖版「一传一页」 =====
   页面下部＝正文竖格；上部（仅首页）＝资料带（左资料 / 右头像）；
   竖格高度由 JS（splitBioCols 的同一公式）经 --bio-grid-h 给出，屏幕与打印都不裁字。 */
.bio-first-page-no { position: absolute; right: calc(3mm / var(--dieji-scale, 1)); bottom: calc(2.5mm / var(--dieji-scale, 1)); font-size: calc(10pt / var(--dieji-scale, 1)); color:#000; line-height:1; }
.bio-page .bio-inner { display: flex; flex-direction: column; }
.bio-head {
  flex: 0 0 auto;
  display: flex;
  align-items: flex-start;
  gap: 18px;
  box-sizing: border-box;
  overflow: hidden;
  height: var(--bio-head-h, 176px);       /* ＝ JS bioHeadH()（含上下 padding 与下边框） */
  padding: 12px 0;
  border-bottom: 1px solid #1a1a1a;
  margin-bottom: 12px;                    /* ＝ JS BIO_HEAD_GAP */
}
.bio-info { flex: 1 1 auto; min-width: 0; }
.bio-info .preface-cat,
.bio-info .preface-title,
.bio-info .preface-meta { text-align: left; }
/* 行高与 JS bioHeadH() 的估算一致（类别 20×1.2、姓名 30×1.2、资料 13×1.5） */
.bio-info .preface-cat { line-height: 1.2; margin-bottom: 8px; }
.bio-info .preface-title { line-height: 1.2; margin-bottom: 6px; }
.bio-info .preface-meta { line-height: 1.5; margin-bottom: 0; }
.bio-portrait { flex: 0 0 auto; }
.bio-portrait img,
.bio-portrait .ph {
  display: block;
  width: 120px !important;               /* ＝ JS BIO_PORTRAIT_W */
  height: 150px !important;              /* ＝ JS BIO_PORTRAIT_H */
  object-fit: cover;
  box-sizing: border-box;
  border: 1px solid #1a1a1a;
  background: #f2ece0;
}
.bio-portrait .ph {
  display: flex; align-items: center; justify-content: center;
  color: #a3927c; font-size: 48px;
}
.bio-zh { font-size: 16px; line-height: 1.2; letter-spacing: 1px; margin-left: 6px; }
/* 传记资料区整体 +2：姓名30→32、生卒/籍贯等13→15 */
.bio-info .preface-title { font-size: 32px; line-height: 1.2; }
.bio-info .preface-meta { font-size: 15px; line-height: 1.5; }
/* 正文竖格：高度与 JS 分页容量同一公式（屏幕）；列宽固定＝页宽/每页列数、整组居中
   （与牒记式竖版一致：末页不满列时不会被拉宽成稀疏列，左右留白均衡） */
.bio-page .vgrid { flex: 0 0 auto; height: var(--bio-grid-h, 800px); justify-content: flex-start; align-items: flex-start; }
/* 传记正文顶对齐：打印时网格更高，居中会把正文整体下推，造成 PDF 顶部空白大于预览 */
.bio-long .vgrid { justify-content: flex-start; align-items: flex-start; margin-top: 0 !important; }
.bio-page .vcol { flex: 0 0 calc(100% / var(--vcol-n, 14)); min-width: 0; }
/* 续页旗标（1 / 2）：放在页脚左侧，避开右下角页码 */
.bio-page .page-flag {
  position: absolute;
  left: 40px;
  bottom: 24px;
  font-size: 14px;
  line-height: 1.4;
  color: #333;
}

@media print {
  /* 打印：竖格改用固定 mm 高（首页让出资料带，续页用完整高度），内容远少于容量，绝不溢页 */
  .bio-head {
    height: calc(var(--bio-head-h, 176px) / var(--dieji-scale, 1));
    padding: calc(12px / var(--dieji-scale, 1)) 0;
    border-bottom-width: calc(1px / var(--dieji-scale, 1));
    margin-bottom: calc(12px / var(--dieji-scale, 1));
    gap: calc(18px / var(--dieji-scale, 1));
  }
  .bio-portrait img,
  .bio-portrait .ph {
    width: calc(120px / var(--dieji-scale, 1)) !important;
    height: calc(150px / var(--dieji-scale, 1)) !important;
  }
  .bio-page .vgrid { height: calc(176mm / var(--dieji-scale, 1)); }
  .bio-page.bio-cont .vgrid { height: calc(190mm / var(--dieji-scale, 1)); }
  .bio-zh { font-size: calc(16px / var(--dieji-scale, 1)); }
  .bio-page .page-flag {
    left: calc(14mm / var(--dieji-scale, 1));
    bottom: calc(4mm / var(--dieji-scale, 1));
    font-size: calc(11px / var(--dieji-scale, 1));
  }
  .bio-page .vcol { font-size: 11px; line-height: 1.7; padding: 4px 1px; }
}
</style>

<!-- 全局（非 scoped）：A4 竖版命名页保留（历史兼容）

<!-- 全局（非 scoped）：A4 竖版命名页保留（历史兼容）；实际打印纸型由 setup 注入的默认 @page 给出。
     末尾空白页修复后不再给页块挂 page: dieji-sheet：绑定命名页会让最后一页（牌記）后多出一个
     「默认页盒」（实测那页为 A4 横版尺寸）→ PDF 末尾多 1 页空白。 -->
<style>
@page dieji-sheet {
  size: A4 portrait;
  margin: 12mm;
}

@media print {
  /* 祠宇志/茔墓志：一逻辑页=一打印页；内容超长时裁切，绝不拆成第二页 */
  .preface.sg-page { break-before: page !important; page-break-before: always !important;
    break-after: page !important; page-break-after: always !important;
    break-inside: avoid !important; page-break-inside: avoid !important; overflow: hidden !important; }
  .preface.sg-page .preface-inner { overflow: hidden !important; }
}
</style>
