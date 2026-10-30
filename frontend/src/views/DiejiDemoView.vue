<template>
  <div class="demo-root" ref="demoRoot" :class="[isLand ? 'orient-land' : 'orient-port', { 'bind-on': bind }]">
    <!-- 工具栏（打印时隐藏） -->
    <div class="toolbar no-print">
      <span class="serif title">{{ isLand ? '横版牒记式' : '竖排牒记式' }}</span>
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
      <span class="mult-label no-print">封面</span>
      <el-select v-model="coverSize" class="font-sel no-print" size="small" style="width: 72px" @change="onMultChange">
        <el-option v-for="o in MULT_OPTS" :key="o.val" :label="o.label" :value="o.val" />
      </el-select>
      <span class="mult-label no-print">姓名</span>
      <el-select v-model="nameSize" class="font-sel no-print" size="small" style="width: 72px" @change="onMultChange">
        <el-option v-for="o in MULT_OPTS" :key="o.val" :label="o.label" :value="o.val" />
      </el-select>
      <span class="mult-label no-print">正文</span>
      <el-select v-model="bodySize" class="font-sel no-print" size="small" style="width: 72px" @change="onMultChange">
        <el-option v-for="o in MULT_OPTS" :key="o.val" :label="o.label" :value="o.val" />
      </el-select>
      <el-button class="no-print" size="small" @click="pickFontFile">⬆ 上传字体</el-button>
      <el-button v-if="currentFontCustom" class="no-print" size="small" type="danger" plain @click="deleteCurrentFont">删除</el-button>
      <input ref="fontInput" type="file" accept=".ttf,.otf,.woff,.woff2"
             class="font-file-input" @change="onFontFile" />
      <span class="mult-label no-print">装订</span>
      <el-switch v-model="bind" class="no-print" size="small" />
      <el-button type="primary" @click="exportPDF">🖨️ 打印 / 导出 PDF</el-button>
      <span class="mult-label no-print">预览</span>
      <el-checkbox v-model="autoFitPreview" class="no-print" size="small">自动</el-checkbox>
      <el-slider v-model="previewScale" class="no-print" style="width: 110px" size="small"
                 :min="0.2" :max="2" :step="0.05" />
      <span class="no-print" style="font-size:12px;color:#8c5a3c;white-space:nowrap">{{ Math.round(effectivePreviewScale * 100) }}%</span>
      <span v-if="clampedByFit" class="no-print" style="font-size:12px;color:#b08840;white-space:nowrap">目前螢幕寬度只夠 {{ Math.round(effectivePreviewScale * 100) }}%</span>
      <span class="hint no-print" style="color:#b08840">版面按 A3 制作；打印弹窗可直接选 A4/A5 等，勾「适应纸张/缩放」即自动等比缩小、不变形</span>
      <span class="hint">{{ familyTitle }} · 牒记式表格 · 每人一竖格 · 自上而下、自右而左</span>
    </div>

    <!-- 谱书：封面 + 一页页网格（sheet），flex 自右而左排，每表固定列数，排满续页，空格补框线 -->
    <div class="book" ref="book">
      <!-- 封面（譜名頁）：雙邊框、豎排書名、堂號郡望、歲次重修 -->
      <div class="cover">
        <div class="cover-inner">
          <!-- 堂號與家族名（譜名）：竖版家族名居中、堂號緊挨其右（同一組）；横版堂號在上、家族名在下。
               竖版用「左侧占位 = 堂號宽 + 间距」把整组居中，数学上使家族名中心 = 内容中心；
               横版 column-reverse 把 DOM 里在后的堂號排到上方。 -->
          <div class="cover-head">
            <div class="cover-title">
              <span class="vtitle" :style="coverTitleStyle">{{ familyTitle }}</span>
            </div>
            <div class="cover-tanghao" v-if="tanghao">{{ tanghao }}</div>
          </div>
          <div class="cover-meta">
            <div class="meta-line" v-if="junwang"><span>郡望　{{ junwang }}</span></div>
            <div class="meta-line" v-if="zuji"><span>祖籍　{{ zuji }}</span></div>
            <div class="meta-line"><span>歲次{{ currentYearGanzhi }}　{{ xiuci }}</span></div>
          </div>
        </div>
      </div>

      <!-- 內封（書名頁）：封面之後、目錄之前。
           竖排居中書名 + 雙線邊框（.cover 本身即雙線）+ 右下角「X堂藏板」（無堂號則不顯示）。
           取舍（三本谱书一致）：與封面/目錄同屬「卷首」，不進目錄、不佔正文頁碼。 -->
      <div class="cover inner-cover">
        <div class="cover-inner it-host">
          <InnerTitlePage :title="familyTitle" :tanghao="tanghao" :year-label="'歲次' + currentYearGanzhi" :xiuci="xiuci"
                          :unit="isLand ? 10.6 : 8.8" :page-h="isLand ? 718 : 1058" />
        </div>
      </div>

      <!-- 目錄頁：封面之後、譜頭之前（傳統版式：封面→目錄→譜序→凡例→祖訓→族規→字輩→世系）。
           注意：**不可**用 class="sheet"——.sheet 帶 counter-increment: sheetpage，會頂掉既有書頁號；
           故用新類 .toc，並沿用同檔 .preface 的分頁規則（固定頁高 + break-after: page + 命名頁 page: dieji-sheet[-land]）。
           條目與頁碼由 tocEntries 在數據側推算（列印分頁是「每塊恰好一頁」，無需也無法在屏幕上量列印版面）。 -->
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

      <!-- 名人传记：一传一页；正文过长按「本页式实际可用容量」续页（页尾有「1 / 2」页码旗标）。
           只有第 1 页（seq=0）带头像与资料——竖版＝上部横带（左：资料 / 右：头像）+ 下部正文竖格；
           横版＝右侧竖列（上：头像 / 下：资料）+ 左侧正文竖格。
           续页（seq>0）不带头像、不带资料带，整页就是正文竖格（旗标保留，读者知道是续页）；
           因此续页容量更大（竖版用完整竖格高、横版用整宽列数），见 splitBioCols。
           横带高 / 右列宽由 JS 先算好（bioHeadH / BIO_SIDE_W）再经 CSS 变量落成真实尺寸，
           保证「算出来的每页容量」与「真实渲染高度」一致，正文绝不溢出裁字。 -->
      <div class="preface preface-long bio-long is-bio" v-for="(pg, bi) in bioPages" :key="'bio' + bi" :data-page-no="bi + 1"
           :class="{ 'page-flash': highlightPageNo === bi + 1 }">
        <div class="preface-inner bio-inner" :data-cont="pg.total > 1 ? (pg.seq + 1) + '/' + pg.total : ''" :class="{ 'bio-cont': !pg.isFirst }"
             :style="{
               '--bio-head-h': pg.headH + 'px', '--bio-grid-h': pg.gridH + 'px',
               '--bio-portrait-w': BIO_PORTRAIT_W + 'px', '--bio-portrait-h': BIO_PORTRAIT_H + 'px',
               '--bio-land-w': BIO_LAND_PORTRAIT_W + 'px', '--bio-land-h': BIO_LAND_PORTRAIT_H + 'px',
               '--bio-side-w': BIO_SIDE_W + 'px', '--bio-side-gap': BIO_SIDE_GAP + 'px',
               '--bio-head-gap': BIO_HEAD_GAP + 'px'
             }">
          <div class="bio-head" v-if="pg.isFirst">
            <div class="bio-info">
              <div class="preface-cat">名人传记</div>
              <div class="preface-title">{{ pg.bio.name }}<span v-if="pg.bio.zi || pg.bio.hao" class="bio-zh">　{{ pg.bio.zi ? '字' + pg.bio.zi : '' }}{{ pg.bio.zi && pg.bio.hao ? '　' : '' }}{{ pg.bio.hao ? '號' + pg.bio.hao : '' }}</span></div>
              <div class="preface-meta" v-if="bioMeta(pg.bio)">{{ bioMeta(pg.bio) }}</div>
            </div>
            <div class="bio-portrait">
              <img v-if="pg.bio.portrait" :src="pg.bio.portrait" :alt="pg.bio.name || ''" />
              <div v-else class="ph">{{ (pg.bio.name || '傳').slice(0, 1) }}</div>
            </div>
          </div>
          <div class="vgrid" :style="pg.maxCols ? { '--vcol-n': pg.maxCols } : {}">
            <div class="vcol" v-for="(c, ci) in pg.cols" :key="'bc' + bi + '-' + ci">{{ c.text }}</div>
          </div>
          </div>
      </div>

      <!-- 谱头页：凡例短篇→成员牒记同款表格（标题在姓名格位、正文在资料格，一条一列，排满续页）；谱序/祖训/族规长篇→整页竖排 -->

      <!-- 圖片展示（多條目）：傳記之後、譜頭/世系之前；每個條目各自佈局/圖說，按條目順序連續續頁 -->
      <div class="preface gallery-page" v-for="(gp, gi) in galleryPages" :key="'gal' + gp.entryIndex + '-' + gp.partNo"
           :data-page-no="bioPages.length + gi + 1"
           :class="{ 'page-flash': highlightPageNo === bioPages.length + gi + 1 }">
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
        <div v-if="pg.type === 'grid'" class="sheet rule-sheet" :data-page-no="bioPages.length + galleryPageCount + pi + 1"
             :class="{ 'page-flash': highlightPageNo === bioPages.length + galleryPageCount + pi + 1 }">
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
        <!-- 谱序/祖训/族规长页：直接复用 EventsPage（与大事记同版式，仅换内容/页标题） -->
        <div v-else-if="pg.type === 'long-event'" class="preface sg-page ep-page epc-page" :data-page-no="bioPages.length + galleryPageCount + pi + 1"
             :class="{ 'page-flash': highlightPageNo === bioPages.length + galleryPageCount + pi + 1 }">
          <div class="preface-inner sg-inner" :data-cont="pg.total > 1 ? pg.part + '/' + pg.total : ''"><EventsPage :items="[{ title: '', year: '', content: pg.text }]" :page-title="(pg.category ? pg.category + ' · ' : '') + pg.title" :part="pg.part" :total="pg.total" variant="dieji" :per-page="isLand ? 28 : 23" :content-h="isLand ? 667 : 974" :page-no="bioPages.length + galleryPageCount + pi + 1" /></div>
        </div>
        <!-- 长篇整页：谱序/祖训/族规（横排大标题）；凡例长条目（竖排标题右起、正文紧凑） -->
        <div v-else class="preface preface-long" :class="{ 'rule-long': pg.kind === 'rule', 'page-flash': highlightPageNo === bioPages.length + galleryPageCount + pi + 1 }" :data-page-no="bioPages.length + galleryPageCount + pi + 1">
          <div class="preface-inner" :class="{ 'with-head': pg.isFirst && pg.kind !== 'rule' }">
            <template v-if="pg.isFirst && pg.kind !== 'rule'">
              <div class="preface-cat">{{ pg.article.category }}</div>
              <div class="preface-title">{{ pg.article.title }}</div>
              <div class="preface-meta" v-if="pg.article.author || pg.article.year">{{ pg.article.author }}<template v-if="pg.article.author && pg.article.year"> · </template>{{ pg.article.year }}</div>
            </template>
            <div class="vgrid" :class="{ 'rule-vgrid': pg.kind === 'rule', 'no-flag': !(pg.total > 1) }" :style="pg.maxCols ? { '--vcol-n': pg.maxCols } : {}">
              <div v-if="pg.isFirst && pg.kind === 'rule'" class="vcol rule-vcol-title">{{ pg.article.title }}</div>
              <div class="vcol" v-for="(c, ci) in pg.cols" :key="'lc'+pi+'-'+ci">{{ c.text }}</div>
            </div>
            </div>
        </div>
      </template>

      <div class="preface genword" v-for="(g, gi) in nonEmptyGenerationWords" :key="'gw'+g.id"
           :data-page-no="bioPages.length + galleryPageCount + prefacePages.length + gi + 1"
           :class="{ 'page-flash': highlightPageNo === bioPages.length + galleryPageCount + prefacePages.length + gi + 1 }">
        <div class="preface-inner">
          <div class="preface-cat">字辈派语</div>
          <div class="genword-branch">{{ g.branch || '总谱' }}</div>
          <div class="genword-poem">{{ g.poem }}</div>
          <div class="genword-chars">
            <span v-for="(c, i) in parseWords(g.words_array)" :key="i" class="genword-char">{{ c }}<small>{{ g.start_generation + i }}</small></span>
          </div>
        </div>
      </div>

      <!-- 五服圖（服制圖）：字輩派語之後、世系之前 —— 譜頭文化之後、五世一圖之前（與垂珠體同序）。
           整頁一幅 SVG 矢量圖，佔正文頁碼（頁碼由 .preface 的 bookpage 計數自動給出），並進目錄。 -->
      <div class="preface fivefu-page" :data-page-no="prePageCount + 1"
           :class="{ 'page-flash': highlightPageNo === prePageCount + 1 }">
        <div class="preface-inner ff-inner">
          <div class="ff-head">
            <span class="ff-title">五服圖</span>
            <span class="ff-meta">{{ tanghao ? tanghao + '　' : '' }}本宗九族服制（斬衰·齊衰·大功·小功·緦麻）</span>
          </div>
          <div class="ff-body"><FiveFuChart :layout="isLand ? 'wide' : 'tall'" /></div>
        </div>
      </div>

      <!-- 家族大事记（五服圖之后、祠宇/世系之前） -->
      <div class="preface sg-page ep-page epc-page" v-for="(ep, ei) in eventPages" :key="'ev' + ei"
           :data-page-no="prePageCount + 2 + ei"
           :class="{ 'page-flash': highlightPageNo === prePageCount + 2 + ei }">
        <div class="preface-inner sg-inner" :data-cont="ep.total > 1 ? ep.part + '/' + ep.total : ''"><EventsPage :items="ep.items" :part="ep.part" :total="ep.total" variant="dieji" :per-page="isLand ? 28 : 23" :content-h="isLand ? 667 : 974" :page-no="prePageCount + 2 + ei" /></div>
      </div>

      <!-- 祠宇志（每个祠宇独立起页；有图则正文页 + 图版页） → 茔墓志 → 世系 -->
      <div class="preface sg-page sg-fs16" v-for="(sp, si) in shrinePageList" :key="'sh'+si"
           :data-page-no="prePageCount + 2 + eventPages.length + si"
           :class="{ 'page-flash': highlightPageNo === prePageCount + 2 + si, 'sg-plate-page': sp.mode === 'image' }">
        <div class="preface-inner sg-inner"><ShrinePage :shrine="sp.item" :mode="sp.mode" :part="sp.part || 'all'"
                        :inline-photo="sp.inlinePhoto || ''" :images="sp.images || []" :image-kind="sp.imageKind || 'photo'" :layout="isLand ? 'cols' : 'rows'" variant="dieji" /></div>
      </div>
      <div class="preface sg-page sg-fs16" v-for="(gp, gi) in gravePageList" :key="'gr'+gi"
           :data-page-no="prePageCount + 2 + eventPages.length + shrinePageList.length + gi"
           :class="{ 'page-flash': highlightPageNo === prePageCount + 2 + eventPages.length + shrinePageList.length + gi }">
        <div class="preface-inner sg-inner"><GravePage :grave="gp.item" variant="dieji" :stack="!isLand" /></div>
      </div>

      <div class="sheet" v-for="(sheet, si) in sheets" :key="si"
           :data-page-no="prePageCount + 2 + eventPages.length + shrinePageList.length + gravePageList.length + si"
           :class="{ 'page-flash': highlightPageNo === prePageCount + 2 + eventPages.length + shrinePageList.length + gravePageList.length + si }">
        <!-- 谱名列（最右，row-reverse 下首项居最右） -->
        <div class="col-title">{{ sheet.colTitle || familyTitle }}</div>
        <!-- 人物列：关系小格 / 姓名大字 / 资料格（竖排，黑条起首） -->
        <div class="person-col" :data-person-key="memberKey(p)" :class="{ cont: p.isCont }" v-for="(p, ci) in sheet.persons" :key="'p'+ci" :style="{ 'flex-grow': p.span || 1, '--p-font': p.font || 1 }">
          <div class="cell-rel"><span class="rel2" v-if="relParts(p.rel).f"><span class="rel2-f">{{ relParts(p.rel).f }}</span><span class="rel2-r" v-if="relParts(p.rel).r">{{ relParts(p.rel).r }}</span></span></div>
          <div class="cell-name" :class="{ 'cont-name': p.isCont }"><span class="vtext">{{ p.isCont ? '续' : p.name }}</span></div>
          <div class="cell-detail" :data-chunk-len="Array.from(p.contText || detailText(p)).length">
            <span v-if="p.elder && !p.isCont" class="gen-bar">{{ p.waiji ? '外纪' : '' }}{{ cn(p.gen) }}世</span>{{ p.contText || detailText(p) }}
          </div>
        </div>
        <!-- 空白列：补满框线 -->
        <div class="person-col blank" v-for="b in sheet.blanks" :key="'b'+b" :style="{ 'flex-grow': PAGE_SPAN }">
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
    <!-- 隱藏量測探針的外層裁剪盒：探針本身是 880px 寬的絕對定位塊、且不吃 --preview-scale 的 zoom，
         不裁剪會把整個文檔撐到 880px 寬（手機上出現可左右拖走的空白、且「適應屏幕」形同虛設）。
         overflow:hidden 只裁溢出，不參與佈局，探針自身的 offsetHeight/clientWidth 量測不受影響。 -->
    <div class="probe-clip" aria-hidden="true">
    <!-- 长篇竖行容量测量探针（隐藏，不占布局） -->
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
    <!-- 成员资料格容量探针（隐藏）：测每字竖向步长/横向列步长/单列宽，用于超长换列分页 -->
    <div class="measure-probe" aria-hidden="true">
      <div class="sheet probe-sheet" ref="probeSheet">
        <div class="col-title"></div>
        <div class="person-col">
          <div class="cell-rel"></div>
          <div class="cell-name"></div>
          <div class="cell-detail" ref="probeDetail"></div>
        </div>
        <div class="person-col blank" v-for="i in 14" :key="'pb'+i">
          <div class="cell-rel"></div>
          <div class="cell-name"></div>
          <div class="cell-detail"></div>
        </div>
      </div>
      <div class="detail-step" ref="probeDetailStep">字</div>
      <div class="detail-hstep" ref="probeHStep">字</div>
    </div>
    </div><!-- /.probe-clip -->

    <!-- 圖片展示選擇器（不參與打印） -->
    <!-- 圖片展示選擇器（多條目；共用組件，不參與打印） -->
    <GalleryDialog v-model="galleryOpen" :entries="galleryEntries" :pool="galleryPool"
                   :pool-loading="galleryPoolLoading" @update:entries="onGalleryEntries" @upload="galleryUpload" />

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
import { ArrowUp } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import api from '../api'
import { readDiejiPreviewScale, saveDiejiPreviewScale, hasUserZoomPref, DIEJI_SCALE_KEY } from '../utils/viewZoomPrefs'
import { uploadFontWithRecovery } from '../utils/fontUpload'
import { formatLifeDates } from '../utils/dateFormat'
import { lostContactText, memberKnown, NO_HEIR_TRAD } from '../utils/memberFlags'
import { relHeadOf, relHeadPartsOfText } from '../utils/lineageRel'
import { fetchAllSources, groupSourcesByMember, sourceFoot } from '../utils/memberSources'
import InnerTitlePage from '../components/book/InnerTitlePage.vue'
import { branchCtx } from '../utils/branchContext'
import ColophonPage from '../components/book/ColophonPage.vue'
import FiveFuChart from '../components/book/FiveFuChart.vue'
import ShrinePage from '../components/book/ShrinePage.vue'
import GravePage from '../components/book/GravePage.vue'
import { fetchShrineGrave } from '../utils/shrineGrave'
import EventsPage from '../components/book/EventsPage.vue'

// ===== 圖片展示（多條目；localStorage 持久化；舊「單塊」數據自動遷移成 1 個條目；空條目不產頁）=====
import GalleryDialog from '../components/GalleryDialog.vue'
import { createGalleryStore, createFamilyGalleryStore, buildEntryPages, entrySummaries } from '../utils/galleryStore'
import { fetchMediaImages, fetchMemberImages, fetchBranchImages } from '../utils/galleryPool'
import { onGalleryImgError } from '../utils/thumb'

const GALLERY_KEY = 'diejiGallery'
const gallery = createFamilyGalleryStore()
const galleryEntries = gallery.entries
const galleryOpen = ref(false)
const galleryPool = ref([])             // 可選圖片（家族多媒體 + 成員附件）
const galleryPoolLoading = ref(false)
const galleryImageTotal = computed(() => galleryEntries.value.reduce((n, e) => n + (e.images ? e.images.length : 0), 0))
// 書頁：按條目順序展平（空條目自動跳過）
const galleryPages = computed(() => buildEntryPages(galleryEntries.value))
const galleryPageCount = computed(() => galleryPages.value.length)
function loadGallery() { gallery.load() }
function onGalleryEntries(list) { galleryEntries.value = list; gallery.save() }
async function loadGalleryPool(force = false) {
  if ((galleryPool.value.length || galleryPoolLoading.value) && !force) return
  galleryPoolLoading.value = true
  try {
    const media = await fetchMediaImages()          // ① 家族多媒體：1 個請求，秒出 → 先能選先用
    galleryPool.value = media
    // ② 成員附件（併發 8 路）＋ ③ 各房支支譜已選圖片（本機 localStorage）
    const [atts, brs] = await Promise.all([
      fetchMemberImages(members.value || []),
      fetchBranchImages(),
    ])
    galleryPool.value = media.concat(atts, brs)
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
    if (members.value && members.value[0]) fd.append('member_id', members.value[0].id)
    await api.post('/upload', fd)
    ElMessage.success('圖片已上傳')
    galleryPool.value = []
    await loadGalleryPool(true)
    opt.onSuccess && opt.onSuccess({})
  } catch (e) {
    ElMessage.error('上傳失敗：' + (e?.response?.data?.error || e?.message || e))
    opt.onError && opt.onError(e)
  }
}
watch(galleryOpen, (v) => { if (v) loadGalleryPool() })


// 纸张方向：竖版页传 portrait（默认），横版页传 landscape。两页共用本组件。
const props = defineProps({
  orient: { type: String, default: 'portrait' } // 'portrait' | 'landscape'
})
const isLand = computed(() => props.orient === 'landscape')

const book = ref(null)
const members = ref([])
const familyName = ref('')
const branchList = ref([])
const surname = ref('')
const tanghao = ref('')
const xiuci = ref('重修')   // 修次：封面「歲次…X」用（服务器设置）
// 封面書名自適應（2026-10-19 属主要求再加大）：≤6 字不縮（竖版 84px / 横版 120px 屏幕、132px 打印），
// >6 字按「可用空間」收縮，長書名也不溢出封面框。堂號也同步加大（见 .cover-tanghao：竖 44/48、横 44/56）。
//  · 竖版：書名豎排，占高 = 字數×(字號+字距)；內框 1024px 要同時容下「書名組 + 信息行(≈317) + 行距」，
//    故書名組高度預算取 640px（8~10 字仍可容納）。
//  · 横版：堂號在上、書名在下（上下排，不并排；属主 2026-10-19 反馈），
//    書名可用整幅内框宽 ≈803px，按字數分攤（屏幕/打印字距不同，分别算）。
const coverTitleStyle = computed(() => {
  const n = Math.max(1, Array.from(familyTitle.value || '').length)
  const ls = n <= 6 ? 22 : Math.max(10, Math.round(22 * 6 / n))
  const fs = n <= 6 ? 84 : Math.max(28, Math.floor(640 / n) - ls)
  const lsL = n <= 6 ? 24 : Math.max(10, Math.round(24 * 6 / n))
  // 横版：堂號在上、書名在下 → 書名不再与堂號分宽，可用整幅内框宽（约 803px，留边距/余量）
  const budgetL = 803
  const fsL = Math.min(120, Math.max(16, Math.floor((budgetL - (n - 1) * lsL) / n)))
  // 横版打印：打印侧字距固定 16px；短書名上限 132px（≈99pt，较上一版 107px 再 +23%）。
  const fsLP = Math.min(132, Math.max(16, Math.floor((budgetL - (n - 1) * 16) / n)))
  return {
    '--title-fs': fs + 'px', '--title-ls': ls + 'px',
    '--title-fs-land': fsL + 'px', '--title-ls-land': lsL + 'px',
    '--title-fs-land-print': fsLP + 'px'
  }
})
const junwang = ref('')
const zuji = ref('')
const diejiShowSisters = ref(true)   // 牒记式本支条目显示姐妹/女儿（家族设置，默认开）
const bind = ref(false)               // 装订：true=打印时左侧预留装订线/活页孔栏

// ===== 打印纸型：统一由「默认 @page」给出（不再给元素挂命名页 page: dieji-sheet*）=====
// 原因（末尾空白页修复）：命名页与默认页混用时，最后一页（牌記）结束后浏览器会再补一个
// 「默认页盒」——实测末尾多出的那一页 MediaBox = A4（841.92×594.96pt，横），与命名页 A3 不同，
// 即它是命名页组收尾产生的空默认页。全文档只用默认 @page（纸型/边距与原来逐字一致）后，
// 页数 == 逻辑页数（末页无空白）。装订时左侧 17mm 仍按原口径。
function applyDefaultPrintPage() {
  let el = document.getElementById('__dieji-print-page')
  if (!el) {
    el = document.createElement('style')
    el.id = '__dieji-print-page'
    document.head.appendChild(el)
  }
  const left = bind.value ? '17mm' : '5mm'
  el.textContent = isLand.value
    ? `@page { size: A3 landscape; margin: 5mm 5mm 5mm ${left}; }`
    : `@page { size: A3 portrait; margin: 5mm 5mm 5mm ${left}; }`
}
watch([isLand, bind], applyDefaultPrintPage, { immediate: true })
onBeforeUnmount(() => {
  const el = document.getElementById('__dieji-print-page')
  if (el) el.remove()
})
const articles = ref([])
const biographies = ref([])   // 名人传记（谱书里「一传一页」，过长自动续页）
const generationWords = ref([])
const shrines = ref([])       // 祠宇志
const graves = ref([])        // 茔墓志
const events = ref([])        // 家族大事记
// 版面固定按 A3 制作（不再提供 A3/A4 开关）；打印到 A4/A5 等由 PDF/打印对话框「适应纸张」自动等比缩小。
// 供非空白字辈/谱头页过滤等使用
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
  { pt: 30, label: '初号（30pt · 特大）' },
  { pt: 26, label: '一号（26pt · 超大）' },
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
// 独立字号倍率：封面 / 姓名 / 正文 各独立可调
const MULT_OPTS = [
  { val: 0.5, label: '0.5x' }, { val: 0.6, label: '0.6x' }, { val: 0.7, label: '0.7x' },
  { val: 0.8, label: '0.8x' }, { val: 0.9, label: '0.9x' }, { val: 1.0, label: '1.0x' },
  { val: 1.1, label: '1.1x' }, { val: 1.2, label: '1.2x' }, { val: 1.3, label: '1.3x' },
  { val: 1.5, label: '1.5x' },
]
const coverSize = ref(parseFloat(localStorage.getItem('diejiCoverSize') || '1.0'))
const nameSize = ref(parseFloat(localStorage.getItem('diejiNameSize') || '1.0'))
const bodySize = ref(parseFloat(localStorage.getItem('diejiBodySize') || '1.0'))
function applyMults() {
  document.documentElement.style.setProperty('--cover-size', String(coverSize.value))
  document.documentElement.style.setProperty('--name-size', String(nameSize.value))
  document.documentElement.style.setProperty('--body-size', String(bodySize.value))
  localStorage.setItem('diejiCoverSize', String(coverSize.value))
  localStorage.setItem('diejiNameSize', String(nameSize.value))
  localStorage.setItem('diejiBodySize', String(bodySize.value))
}
function onMultChange() { applyMults() }
watch([coverSize, nameSize, bodySize], applyMults, { immediate: true })
// 旧版存的是缩放因子(0.92~1.16)，不匹配 pt 列表则回退标准四号
const storedPt = parseFloat(localStorage.getItem('diejiFontSize') || '14')
const fontSize = ref(PTVALS.includes(storedPt) ? storedPt : 14)
const fontInput = ref(null)
// 当前缩放倍数（四号=1）；字号放大 s 倍时：每页列数 ÷s、每列字数 ÷s、标题格高 ×s
const scaleVal = computed(() => {  const pt = PTVALS.includes(fontSize.value) ? fontSize.value : BASE_PT
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

// ===== 预览缩放：自动按屏幕调整比率 + 手动调整 =====
const demoRoot = ref(null)
// 记住的预览比率（localStorage，默认 0.70）：既作为「自动」时的上限，也作为恢复手动态时的初始值
// 记住的预览比率：默认 0.70（作为自动时的上限）；
// 仅当用户以前手动调过该值，才初始进入「手动档」，否则保持「自动」（= min(适合宽度, 记住的值)）
const storedPreviewScale = ref(readDiejiPreviewScale())
const autoFitPreview = ref(!hasUserZoomPref(DIEJI_SCALE_KEY))
const previewScale = ref(autoFitPreview.value ? 1 : storedPreviewScale.value)
// 本次工作階段是否由使用者手動調過：主動調過 → 依使用者值顯示（可超出螢幕，屬主確認的預期行為）；
// 僅「沿用上次記住的值」→ 仍受適合寬度壓制（跨屏自動縮小）。重新整理即回到自動（min(適合寬度, 記住值)）。
const explicitManual = ref(false)
// 手動調整（滑桿／手動檔）時標記為「本次主動調整」→ 不再被適合寬度壓回
// 手动值被「螢幕只夠這麼寬」壓回時給提示（屬主預期行為，非 bug）
const clampedByFit = ref(false)
let clampTimer = null
function notifyClamped() {
  clampedByFit.value = true
  if (clampTimer) clearTimeout(clampTimer)
  clampTimer = setTimeout(() => { clampedByFit.value = false }, 4000)
}
// 手动调整（滑块 / 手动档）时写回记忆
watch(previewScale, v => {
  storedPreviewScale.value = v
  saveDiejiPreviewScale(v)
  explicitManual.value = true          // 使用者拖動滑桿/手動檔 → 本次主動調整
  autoFitPreview.value = false         // 拖過滑桿即切到「手動檔」：滑桿不再被禁用、勾選框狀態與實際一致
})
const previewViewW = ref(0)         // 预览容器可视宽度（响应式，resize 更新）
// 纸张「設計寬」（＝除以 --preview-scale 之前的本地 px）：由 DOM 實測，隨橫豎版/字號自適應。
// 舊寫法用固定基準（豎 880 / 橫 1240）算「適應螢幕」，但橫版實際紙寬只有 880 →
// 被多縮了 880/1240 = 0.71 倍（手機上紙只 226px，沒有鋪滿螢幕）。
const paperW = ref(880)

function measurePaperW() {
  const bk = book.value
  if (!bk) return
  // .book 上掛著 zoom；紙張本地寬 = 設備寬 / zoom（本地寬與 zoom 無關，故一次量測即收斂，不會抖動）
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

// 按容器宽度自动适应的比率（基准宽 = 实测纸张宽，夹在 0.2~3）
const fitPreviewScale = computed(() => {
  const baseW = paperW.value || 880
  const fit = (previewViewW.value - 48) / baseW
  return Math.max(0.2, Math.min(fit, 3))
})
// 实际预览缩放：自动 = min(适合宽度, 记住的值)；手动 = 记住的值（previewScale）
const effectivePreviewScale = computed(() => {
  if (explicitManual.value) return previewScale.value          // 手動調整：不壓回適合寬度
  if (!autoFitPreview.value) return Math.min(previewScale.value, fitPreviewScale.value)
  return Math.min(fitPreviewScale.value, storedPreviewScale.value)
})

watch(effectivePreviewScale, v => {
  document.documentElement.style.setProperty('--preview-scale', String(v))
}, { immediate: true })

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
  // 大字体（几 MB~30MB）走外网反代常 >30s：超时放宽到 300s；真超时/断网时再问一次服务端，
  // 若文件其实已保存（后端可能避重名存成 xxx-2.woff2）就按成功处理，不再给用户看英文 network error
  let info
  let recovered = false
  try {
    const r = await uploadFontWithRecovery(file)
    info = r.info
    recovered = r.recovered
  } catch (err) {
    console.error(err)
    // 服务端去重拒绝（409）：把服务端文案原样弹出；「内容重复」时顺手选中已有的那个
    ElMessage.error(err.message || '字体上传到服务器失败：' + file.name)
    if (err && err.kind === 'duplicate' && err.existing) {
      const hit = FONTS.value.find(f => f.name === err.existing)
      if (hit) { fontKey.value = hit.key; ElMessage.success('已为你选中已有的：' + hit.label) }
    }
    return
  }
  if (recovered) ElMessage.success('上传超时，但已从服务器确认字体已保存：' + (info.name || file.name))
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
// 凡例表每页条数 = 成员表每页人数：两张表列宽完全一致（否则凡例格比成员格宽/窄）
const GRID_COLS = computed(() => Math.max(1, TARGET_PERSONS.value))
// 横版凡例少一列，格子均分加宽；竖版不动
const RULE_COLS = computed(() => (isLand.value ? Math.max(1, GRID_COLS.value - 1) : GRID_COLS.value))
const LONG_COLS = computed(() => Math.max(7, Math.round(14 / scaleVal.value)))  // 长篇(凡例)每页竖行数，随字号收缩
// 长篇每竖行字数：按实际渲染测量（竖行撑满框内高度），避免固定字数导致竖行下半截空白却提前翻页
const longCap = ref({ first: 24, cont: 30 })   // 首行(有标题) / 续行 容量；挂载后实测覆盖（保守值防裁字）
const longCols = ref(14)                      // 长篇每页竖行数（按实测宽度算，防列窄于一行→字被横切）
// 名人传记每页竖行数：竖版正文竖格仍占整宽（头像/资料在正文上方的横带里）→ 同 longCols；
// 横版右侧要让出「头像+资料」竖列，只有剩余宽度 → 单独实测
const bioCols = ref(14)
const bioColsLand = ref(10)
const sourceMap = ref({})   // memberId → 資料來源腳注（'据：…；…'；無來源則無此鍵）
const charsPerBase = ref(60)   // 每基础列能装多少字（含竖排换列），挂载后实测覆盖
const probeInner = ref(null)
const probeTitle = ref(null)
const probeCol = ref(null)
const probeDetail = ref(null)
const probeDetailStep = ref(null)
const probeHStep = ref(null)
const probeSheet = ref(null)
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
  // 竖版：探针页高已按 scale 缩小（280mm/scale），容量比不再被抵消 → 不再除 scale；
  // 横版维持原逻辑（探针按 190mm/scale 的 min-height 量，除 scale 更保险）
  const sz = isLand.value ? (scaleVal.value || 1) : 1
  const first = Math.max(8, Math.floor((contentH - titleH) / step * SAFETY / sz))
  const cont  = Math.max(10, Math.floor((contentH - 24) / step * SAFETY / sz))
  longCap.value = { first, cont, step }
  // 每竖行横向占宽 = .vcol 的 line-height（竖排下即行距），列宽不够一行时字会被横切
  const innerW = inner.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight)
  const adv = parseFloat(csCol.lineHeight) || parseFloat(csCol.fontSize) * 1.2
  const colPadH = parseFloat(csCol.paddingLeft) + parseFloat(csCol.paddingRight)
  if (innerW > 0 && adv > 0) {
    longCols.value = Math.max(3, Math.min(40, Math.floor(innerW / (adv + colPadH))))
    // 名人传记：竖版竖格占整宽；横版先扣掉右侧「头像+资料」竖列（宽 BIO_SIDE_W + 间距）再算列数
    bioCols.value = longCols.value
    bioColsLand.value = Math.max(3, Math.min(40,
      Math.floor(Math.max(adv + colPadH, innerW - BIO_SIDE_W - BIO_SIDE_GAP) / (adv + colPadH))))
  }
}
// 成员资料格容量：测每字竖向步长/横向列步长/单列宽，算出每基础列能装多少字（含换列）
function measureDetailCap() {
  if (!probeDetail.value || !probeDetailStep.value || !probeHStep.value) return
  // 竖向步长（每字）：独立竖行，高度随内容
  const stepEl = probeDetailStep.value
  stepEl.textContent = '字'.repeat(30)
  const scs = getComputedStyle(stepEl)
  const stepPadV = parseFloat(scs.paddingTop) + parseFloat(scs.paddingBottom)
  const vStep = (stepEl.offsetHeight - stepPadV) / 30
  stepEl.textContent = ''
  if (!(vStep > 0)) return
  // 资料格实际可用高度与单列内容宽（探针 sheet 里该格正好 1 基础列宽）
  const cell = probeDetail.value
  const cs = getComputedStyle(cell)
  const contentH = cell.clientHeight - parseFloat(cs.paddingTop) - parseFloat(cs.paddingBottom)
  const padH = parseFloat(cs.paddingLeft) + parseFloat(cs.paddingRight)
  const baseCW = cell.clientWidth - padH
  if (!(contentH > 0) || !(baseCW > 0)) return
  // 每竖行字数（用于横向步长测量：填 1 竖行 / 2 竖行）
  const cap = Math.max(4, Math.floor(contentH / vStep))
  // 横向列步长：定高强制换列，量 1 竖行 vs 2 竖行 的宽度差
  const hEl = probeHStep.value
  hEl.style.height = contentH + 'px'
  hEl.textContent = '字'.repeat(cap)
  const w1 = hEl.offsetWidth
  hEl.textContent = '字'.repeat(cap * 2)
  const w2 = hEl.offsetWidth
  hEl.textContent = ''
  hEl.style.height = ''
  const hStep = w2 - w1
  if (!(hStep > 0)) return
  // 每基础列总容量（含换列）= 格面积 ÷ 单字面积，留 8% 余量防裁字
  // 探針表固定 16 欄（1 譜名 + 15 單元），實際表是 1 譜名 + PERSON_COLS 單元；
  // 兩者單元寬不同：不修正的話大字形下跨度偏大（每頁人少、末頁留幾列空框）
  const perReal = Math.max(1, PERSON_COLS.value || 1)
  const unitFix = Math.min(1.5, Math.max(0.7, 15 / perReal))
  charsPerBase.value = Math.max(8, Math.floor(contentH * baseCW * unitFix / (vStep * hStep) * 0.92))
}
// 渲染后实测“真实长篇页竖格高度”：竖格是 flex 撑满（高度与字数无关）→ 量一次即稳定，不会反复抖动
function measureRealGrid() {}   // 已废弃：竖格高改为公式算（见 longGridCss），不再依赖渲染后测量

// 长篇页竖格 CSS 高度（px，与 CSS 里 .vgrid 的 height 公式严格一致）：
//   base = 页高 − 上下留白 − 页码位 − 余量（竖 900 / 横 620）；
//   无页码 +20（让出页码的位置：有页码小一点、没页码大一点）；
//   带标题 −98×body-size（标题随号数除 scale、保持物理恒定，故与正文同除 scale）
function longGridCss(isFirst, hasFlag) {
  const s = scaleVal.value || 1
  const base = (isLand.value ? 620 : 900) + (hasFlag ? 0 : 20)
  const head = isFirst ? 98 * (bodySize.value || 1) : 0
  return (base - head) / s
}
function scheduleMeasure() {
  clearTimeout(measureTimer)
  measureTimer = setTimeout(() => nextTick(() => { measureLongCap(); measureDetailCap() }), 120)
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
onMounted(() => { scheduleMeasure();  window.addEventListener('beforeprint', measureDetailCap); updatePreviewViewW(); window.addEventListener('resize', () => { scheduleMeasure(); updatePreviewViewW() }); loadServerFonts(); loadGallery()
  // 回到顶部按钮：监听本视图的滚动容器 .book
  if (book.value) book.value.addEventListener('scroll', onScrollTopBtn, { passive: true })
})
onBeforeUnmount(() => { if (book.value) book.value.removeEventListener('scroll', onScrollTopBtn) })

// ===== 名人传记版式常量（px，皆为「除以 --dieji-scale 前的基准值」，与 CSS 里同一批常量对应）=====
// 首页（seq=0）：竖版上部横带（左资料 / 右头像）压在正文上方 → 横带越高，正文竖格越矮；
//       带高由 bioHeadH() 按内容估出（含头像 120×150、只多不少），CSS 用 --bio-head-h 落成同一高度。
//       横版右侧竖列（上头像 150×190 / 下资料）只吃宽度 → 正文竖格高度不变，列数改由 bioColsLand 扣宽后算。
// 续页（seq>0）：不带横带、不带右列 → 竖版用完整竖格高（900 档）、横版用整宽列数（bioCols），容量更大。
const BIO_PORTRAIT_W = 120        // 竖版头像宽
const BIO_PORTRAIT_H = 150        // 竖版头像高
const BIO_LAND_PORTRAIT_W = 150   // 横版头像宽
const BIO_LAND_PORTRAIT_H = 190   // 横版头像高
const BIO_SIDE_W = 210            // 横版首页右侧「头像+资料」竖列宽（含头像 150 + 内边距）
const BIO_SIDE_GAP = 18           // 横版首页右侧竖列与正文竖格的间距
const BIO_HEAD_GAP = 12           // 竖版上部横带与正文竖格的间距
const BIO_HEAD_PAD = 12           // 竖版横带上下内边距
const BIO_HEAD_BORDER = 1         // 竖版横带下边框（1px）
const BIO_HEAD_INFO_W = 380       // 竖版左栏可用宽度（保守值：实际更宽 → 估算折行数偏多 → 带高偏大 → 容量更保守）
const BIO_GRID_BASE_PORT = 974    // 竖版正文竖格基准高：与 .preface-long 的 900px 一致（已让出「1 / 2」旗标位）；首页再扣横带，续页直接用
const BIO_GRID_BASE_LAND = 667    // 横版首页正文竖格基准高：比 .preface-long 的 620px 再留 20px 余量（右列 + 旗标都排得下）
const BIO_GRID_BASE_LAND_CONT = 667  // 横版续页基准高：无右列，用 .preface-long 的完整 620 档
const BIO_GRID_MIN = 240          // 竖格高下限（夸张字号下也不至于为 0）

// 名人传记分页：一传一页；正文过长续页（页尾带「1 / 2」旗标）。
// **两套容量**：第 1 页带头像/资料带（竖版扣带高、横版扣右列宽），第 2 页起不带 → 用完整容量。
const bioPages = computed(() => {
  const pages = []
  for (const b of biographies.value) {
    const text = cleanText(b.content || '')
    if (!text) continue                     // 正文为空的传记不占页
    const pgList = splitBioCols(text, b)
    const total = Math.max(1, pgList.length)
    const headH = bioHeadH(b)                // 竖版首页横带高（横版为 0：头像/资料在右侧竖列，不占正文高度）
    pgList.forEach((p, pi) => {
      const isFirst = pi === 0
      pages.push({
        type: 'bio', bio: b, cols: p.cols, maxCols: p.maxCols,
        isFirst, seq: pi, total,
        headH: isFirst ? headH : 0,          // 续页没有资料/头像带
        gridH: p.gridH                       // 首页=扣带后的竖格高；续页=完整竖格高
      })
    })
  }
  return pages
})

// 传记名字下方的信息行（只拼有值的，用 · 分隔）
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

const nonEmptyGenerationWords = computed(() =>
  (generationWords.value || []).filter(g => (parseWords(g && g.words_array) || []).length > 0)
)

// 长篇竖排「定列分页」：按实测行容量切分，超出容量的内容自动续到下一页（绝不裁字）。
// 谱序/祖训/族规走这一套；名人传记版式不同（上部有资料/头像带），走下面的 splitBioCols。
function splitLongCols(text) {
  let rest = text
  const pagesCols = []
  const maxCols = longCols.value
  // 每竖行容量 = 该页竖格高 ÷ 每字步长 × 0.92（带标题/页码的页内容框更小）
  const stepMeas = Math.max(8, longCap.value.step || 16)
  const capOf = (isFirst, hasFlag) => Math.max(6, Math.floor(longGridCss(isFirst, hasFlag) / stepMeas * 0.92))
  // 判断整篇能否塞进「单页（无页码）」：能就不用让出页码位，框更大
  const onePageChars = Math.max(1, Math.round(capOf(true, false) * maxCols))
  const hasFlag = text.length > onePageChars
  while (rest.length) {
    const pageIsFirst = pagesCols.length === 0
    const cap = Math.max(1, Math.round(capOf(pageIsFirst, hasFlag)))
    const cols = []
    while (cols.length < maxCols && rest.length) {
      const piece = rest.slice(0, cap)
      if (piece) cols.push({ text: piece })
      rest = rest.slice(cap)
    }
    if (cols.length) pagesCols.push(cols)
  }
  return { pagesCols, maxCols, hasFlag }
}

// 估算折行长度的「字重」：全角算 1、半角算 0.55（仅用于估横带里资料行要占几行）
function textWeight(s) {
  let n = 0
  for (const ch of String(s || '')) n += /[\x00-\xff]/.test(ch) ? 0.55 : 1
  return n
}

// 竖版上部「资料 + 头像」横带高度（基准 px，CSS 用 --bio-head-h 落成同一高度；横版返回 0）。
// 高度 = max(头像高 150, 左栏「类别 + 姓名 + 资料行」估算高) + 上下内边距 + 下边框；
// 估算只多不少（宁可带子高一点、正文少装几个字，也不让资料被横带裁掉或文字压出框外）。
function bioHeadH(b) {
  if (isLand.value) return 0                       // 横版：头像/资料在右侧竖列，不占正文高度
  const bs = bodySize.value || 1
  const perLine = Math.max(6, Math.floor(BIO_HEAD_INFO_W / (13 * bs)))
  const lines = Math.max(1, Math.min(9, Math.ceil(textWeight(bioMeta(b)) / perLine)))
  const infoH = Math.ceil(24 * bs) + 8             // 「名人传记」类别行 + 下间距
    + Math.ceil(36 * bs) + 6                       // 姓名行（30px × 1.2 行高）+ 下间距
    + lines * Math.ceil(13 * bs * 1.5)             // 资料行（13px × 1.5 行高）
  const content = Math.max(BIO_PORTRAIT_H, infoH)
  return Math.max(BIO_PORTRAIT_H + 2 * BIO_HEAD_PAD + 2 * BIO_HEAD_BORDER,
    Math.min(420, content + 2 * BIO_HEAD_PAD + 2 * BIO_HEAD_BORDER + 12))
}

// 名人传记正文竖格高度（基准 px，与 CSS 里 .bio-long .vgrid 的高度公式严格一致）。
// 首页：竖版 = 900（.preface-long 基准，已让出旗标位）− 上部横带 − 带下间距；横版 = 600（右列 + 旗标）。
// 续页：不带横带/右列 → 竖版 900、横版 620（.preface-long 的完整档）。
// 两种页面都预留在页面内（竖格高 + 旗标 ≈ 920 ≤ 内框 982；横版 640 ≤ 内框可用高度），列印不裁。
function bioGridFirst(b) {
  const base = isLand.value ? BIO_GRID_BASE_LAND : (BIO_GRID_BASE_PORT - bioHeadH(b) - BIO_HEAD_GAP)
  return Math.max(BIO_GRID_MIN, base)
}
function bioGridCont() {
  const base = isLand.value ? BIO_GRID_BASE_LAND_CONT : BIO_GRID_BASE_PORT
  return Math.max(BIO_GRID_MIN, base)
}
// 每页竖行数：竖版首页/续页都占整宽 → bioCols；横版首页要让出右侧竖列 → bioColsLand，续页用整宽 → bioCols
function bioColsFirst() { return Math.max(3, isLand.value ? bioColsLand.value : bioCols.value) }
function bioColsCont() { return Math.max(3, bioCols.value) }

// 传记正文分页：一传多页，**按页装箱**（第 1 页用首页容量，之后每页用续页容量）。
// 每竖行字数 = 该页竖格高 ÷ 实测每字步长 × 0.92（安全余量：字体渲染比测量偏大时不裁字）。
// 按码点切分（Array.from），避免把代理对（生僻字）切成半个字。
function splitBioCols(text, b) {
  const chars = Array.from(text)
  // 竖排字步进是 font-size 18px；longCap.step 是行高32，会把容量算成一半。
  const step = 18
  const s = scaleVal.value || 1
  const pages = []
  let i = 0
  while (i < chars.length && pages.length < 500) {
    const isFirst = pages.length === 0
    const gridH = isFirst ? bioGridFirst(b) : bioGridCont()
    const maxCols = isFirst ? bioColsFirst() : bioColsCont()
    const cap = Math.max(6, Math.floor(gridH / s / step * 0.98))
    const cols = []
    while (cols.length < maxCols && i < chars.length) {
      cols.push({ text: chars.slice(i, i + cap).join('') })
      i += cap
    }
    if (cols.length) pages.push({ cols, maxCols, gridH })
  }
  return pages
}

const PREFACE_GRID_COLS = () => (isLand.value ? 28 : 23)
const prefaceGridH = () => (isLand.value ? 667 : 974)
const prefaceGridChars = () => Math.floor((prefaceGridH() - 8) / 18) + 1

const prefacePages = computed(() => {
  const pages = []
  let shortItems = []          // 凡例 [{title,text}]
  const flushGrid = () => {
    if (shortItems.length) {
      const maxTitle = shortItems.reduce((m, it) => Math.max(m, (it.title || '').length), 4)
      const cols = RULE_COLS.value
      // cats：本页含哪些谱头类别（仅供目錄页推算起始页；纯新增字段，不影响既有排版/分页）
      pages.push({ type: 'grid', items: shortItems, blanks: cols - shortItems.length,
                   nameH: Math.ceil(maxTitle * RULE_NAME_CHAR_H),
                   cats: [...new Set(shortItems.map(it => it.cat))] })
      shortItems = []
    }
  }
  for (const a of prefaceArticles.value) {
    const text = cleanText(a.content)
    if (!text) continue            // 空正文文章不生成页（否则产生空白双边框页）
    const isRule = a.category === '凡例'
    // 字号放大时单格容量变小：阈值 ÷缩放倍数，超出的条目走长篇竖排页（定列分页、可续页，绝不挤格/溢出）
    // 凡例不管多长都不换页：始终留在短篇表格（每条一列、格内竖排换行），不走整页竖排
    const shortMax = Math.round(SHORT_MAX / Math.pow(scaleVal.value, 1.3))
    if (text.length <= shortMax || isRule) {
      shortItems.push({ title: a.title || '', text, cat: a.category })
      if (shortItems.length >= RULE_COLS.value) flushGrid()
    } else {
      flushGrid()
      // 逐列填充：每竖行塞满该页可用高度（首页有标题→first 容量，续页→cont 容量）
      // 仅谱序/祖训/族规(book)走此分支；凡例已在上方短路进表格
      const charsPer = prefaceGridChars()
      const perBody = Math.max(1, PREFACE_GRID_COLS() - 1)
      const chunkSize = charsPer * perBody
      const totalPages = Math.max(1, Math.ceil(text.length / chunkSize))
      for (let pi = 0; pi < totalPages; pi++) {
        pages.push({
          type: 'long-event', article: a, text: text.slice(pi * chunkSize, (pi + 1) * chunkSize),
          title: (a.title || ''), category: a.category || '', part: pi + 1, total: totalPages,
        })
      }
    }
  }
  flushGrid()
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

// 提取出生公历年：优先公历，其次农历；支持民国纪年（民国N年 = 1911+N）；无法解析返回 null
function birthYear(m) {
  const pick = (s) => {
    if (!s) return null
    // 民国纪年：民国X年 / 民國X年
    let mg = String(s).match(/(?:民国|民國)\s*(\d{1,3})\s*年/)
    if (mg) return 1911 + parseInt(mg[1], 10)
    // 公元四位数年份（19xx / 20xx / 1xxx）
    let g = String(s).match(/(\d{4})\s*年?/)
    if (g) {
      const y = parseInt(g[1], 10)
      if (y >= 1000 && y <= 2100) return y
    }
    return null
  }
  return pick(m.birth_gregorian) ?? pick(m.birth_lunar)
}

// 條首小字「某公第幾子」→ 固定兩段：{ f: 父名＋公, r: 行第 }（缓存，模板每格只解析一次）
//   版面：兩段各自成豎列（右＝父名，左＝行第），格內不換行；字號由 CSS 按格高固定給
//   （屏幕 14px / 打印 12px、橫版打印 10px），不再按字數縮字號。
const relPartCache = new Map()
function relParts(rel) {
  const k = String(rel == null ? '' : rel)
  let v = relPartCache.get(k)
  if (!v) { v = relHeadPartsOfText(k); relPartCache.set(k, v) }
  return v
}

// ===== 谱书标题：姓氏 + 氏家谱 =====
const familyTitle = computed(() => {
  const b = branchList.value.find(x => String(x.id) === String(branchCtx.current))
  if (b && b.id) return (b.name || '') + '支谱'
  if (surname.value) return surname.value + '氏家谱'
  if (familyName.value) return familyName.value
  return '族谱'
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

      // 姐妹/女儿（本支条目显示，默认开）：同父的女性节点，跨外纪也算；按【出生年份】区分姐/妹
      let sistersText = ''
      if (diejiShowSisters.value && node.gender !== 2 && node.father_id) {
        const sis = list
          .filter(m => (m.father_id || 0) === node.father_id && m.id !== node.id && m.gender === 2)
        if (sis.length) {
          const myYear = birthYear(node)
          const older = [], younger = [], unknown = []
          sis.forEach(s => {
            const y = birthYear(s)
            if (myYear == null || y == null) unknown.push(fullName(s))
            else if (y < myYear) older.push(fullName(s))
            else younger.push(fullName(s))
          })
          const parts = []
          if (older.length) parts.push('姐' + older.join('、'))
          if (younger.length) parts.push('妹' + younger.join('、'))
          if (unknown.length) parts.push('姐妹' + unknown.join('、'))
          sistersText = parts.join('；')
        }
      }

      out.push({
        id: node.id,
        father_id: node.father_id,
        gen: node.generation,
        record_type: node.record_type || '内纪',
        kaijiNote: '',   // 本支开基祖小字（后处理填）
        sisters: sistersText,
        rel: (() => {
          const myType = node.record_type === '外纪' ? '外纪' : '内纪'
          // 父亲在同一支（同 record_type）才标父子关系；跨边界或无父者为该支开基祖
          if (node.father_id && idMap[node.father_id]) {
            const dad = idMap[node.father_id]
            const dadType = dad.record_type === '外纪' ? '外纪' : '内纪'
            if (dadType === myType) {
              // 條首小字「某公第幾子」：行第按同父同胞（同性别）排序位置取（长先幼后）
              const nm = dad.given_name || dad.name || ''
              const sib = node.gender === 2 ? dad.daughters : dad.sons
              const rankIdx = sib.findIndex((x) => x.id === node.id) + 1
              return relHeadOf(nm, rankIdx, node.gender) || (nm + (node.gender === 2 ? '之女' : '之子'))
            }
          }
          // 本支开基：外纪开基祖标“外纪始祖”，内纪（本宗）开基祖标“始祖”
          return myType === '外纪' ? '外纪始祖' : '始祖'
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
  // 外纪在前，内纪在后，各自保持 DFS 代序
  const waijiOut = out.filter(p => p.record_type === '外纪')
  const neijiOut = out.filter(p => p.record_type === '内纪')
  // 外纪世代编号：外纪一世、外纪二世...（避免显示零世/〇世）
  if (waijiOut.length) {
    const waijiMinGen = Math.min(...waijiOut.map(p => p.gen))
    waijiOut.forEach(p => {
      p.gen = p.gen - waijiMinGen + 1
      p.waiji = true
    })
  }
  // 重新计算 elder 标记：按各自 record_type 内的世代分组，每组第一个为 elder
  function recalcElder(list) {
    const gm = {}
    list.forEach(p => {
      if (!gm[p.gen]) gm[p.gen] = []
      gm[p.gen].push(p)
    })
    Object.values(gm).forEach(group => {
      group.forEach((p, i) => { p.elder = i === 0 })
    })
  }
  recalcElder(waijiOut)
  recalcElder(neijiOut)
  // 本支开基祖小字：内纪中父亲在外纪（或无父且为内纪最低世）的人物，
  // 备注本支开基一世、系外纪父之子、血缘世次，避免混淆开基世次与血缘世次
  const waijiGenById = new Map()
  waijiOut.forEach(p => waijiGenById.set(p.id, p.gen))
  neijiOut.forEach(p => {
    if (!p.father_id) return
    const dadWaijiGen = waijiGenById.get(p.father_id)  // 父亲在外纪重编号后的世代
    if (dadWaijiGen) {
      const dad = idMap[p.father_id]
      const dadName = (dad.given_name || dad.name || '')
      const bloodGen = cn(dadWaijiGen + 1)
      p.kaijiNote = `本支开基一世，系${dadName}之子，血缘${bloodGen}世`
    }
  })
  return [...waijiOut, ...neijiOut]
})

// 资料文字：竖排连贯而下（与原 demo 一致：配偶X子女Y、適、出生籍、終享年、葬、小传）
function detailText(p) {
  // 迁居外地失联：生殁葬、子嗣缺啥补啥 + 失考
  if (p.lostContact) {
    const ls = []
    if (p.kaijiNote) ls.push(p.kaijiNote + '。')
    const hasKids = (p.spouses || []).some(s => s.children && s.children.length)
    ls.push(lostContactText(p.known || {}, hasKids))
    if (p.detail) ls.push(p.detail)
    return ls.join(' ')
  }
  const parts = []
  if (p.kaijiNote) parts.push(p.kaijiNote + '。')
  if (p.sisters) parts.push(p.sisters + '。')
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

// ===== 分页：每表 13 列（谱名 1 + 人物 12），自右而左；外纪/内纪分页，排满续页，空格补框线 =====
const BASE_COLS = 13        // 竖版 A4 标准字号：谱名 1 + 人物 12
const BASE_COLS_LAND = 20    // 横版（A3 大版面）：谱名 1 + 人物 19；列宽收窄约20%，避免每格过宽留白过大
const PERSON_COLS = computed(() => {
  const base = isLand.value ? BASE_COLS_LAND : BASE_COLS
  // 每列能装的字数已随字号 ÷s²（探针实测），列数不能再 ÷s²：否则一页容量跌到 1/s⁴，
  // 大字号会变成「一个成员一页、大片空白」。只有比标准小字号时才放宽列数做细排。
  const s = Math.min(1, scaleVal.value || 1)
  return Math.max(4, Math.round((base - 1) / Math.pow(s, 2)))
})
// ===== 每页人数：随字号自动调整（大字号少、小字号多），上限 = 每页列数 =====
// 参考点：初号 30pt（scale 2.143）竖版 5 人 / 横版 6 人刚好；字号缩 f 倍 → 每页容量 ×f²（按字符面积算）
const REF_S = 30 / 14
const TARGET_PERSONS = computed(() => {
  const s = scaleVal.value || 1
  const ref = isLand.value ? 6 : 5
  const n = Math.round((ref * REF_S * REF_S) / (s * s))
  return Math.max(1, Math.min(PERSON_COLS.value, n - 1))
})
const PAGE_SPAN = computed(() => PERSON_COLS.value / TARGET_PERSONS.value)
function personFont(p) {
  // 脚注（資料來源小字）按 0.72 折算成资料格字数：小字更省地方，但仍计入，宁多勿少
  const need = (detailText(p) || '').length + 2 + Math.round((p.src || '').length * 0.72)
  const cap = PAGE_SPAN.value * charsPerBase.value
  if (!(cap > 0) || need <= cap) return 1
  // 字号缩 f 倍 → 每列容量 ×1/f²；解 f ≤ √(cap/need)，下限 0.45：长资料宁小不裁
  return 1   // 字号固定18，不再自动缩小
}
const MEMBER_FIRST_PORT = 92
const MEMBER_CONT_PORT = 92
const MEMBER_FIRST_LAND = 68
const MEMBER_CONT_LAND = 64
function memberKey(p) { return String(p.id ?? '') + '|' + String(p.name ?? '') + '|' + String(p.generation ?? '') }
function splitByCap(text, cap) {
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
  if (pieces.reduce((n,c)=>n+Array.from(c).length,0) !== chars.length) throw new Error('member split lost')
  if (pieces.some(c => Array.from(c).length > cap)) throw new Error('member split > cap')
  return pieces
}
function splitMember(p) {
  const contCap = isLand.value ? MEMBER_CONT_LAND : MEMBER_CONT_PORT
  const firstCap = p.elder ? (isLand.value ? MEMBER_FIRST_LAND : MEMBER_FIRST_PORT) : contCap
  const text = detailText(p) + (p.src ? '　' + p.src : '')
  const chars = Array.from(text)
  if (chars.length <= firstCap) return [{ ...p, contText: text }]
  const first = chars.slice(0, firstCap).join('')
  const rest = chars.slice(firstCap).join('')
  const parts = [first, ...splitByCap(rest, contCap)]
  if (parts.reduce((n,c)=>n+Array.from(c).length,0) !== chars.length) throw new Error('member split lost')
  return parts.map((t, i) => i === 0 ? { ...p, contText: t } : { ...p, name: '续', rel: '', elder: false, src: '', waiji: false, isCont: true, contText: t })
}
const sheets = computed(() => {
  const out = []
  const span = PAGE_SPAN.value
  const target = TARGET_PERSONS.value
  const pack = (list, section, colTitle) => {
    const expanded = []
    for (const p of list) expanded.push(...splitMember(p))
    for (let i = 0; i < expanded.length; i += target) {
      const chunk = expanded.slice(i, i + target)
      out.push({
        persons: chunk.map(p => ({ ...p, span, font: personFont(p) })),
        blanks: target - chunk.length,
        section,
        colTitle
      })
    }
  }
  pack(people.value.filter(p => p.record_type === '外纪'), '外纪', '外纪')
  pack(people.value.filter(p => p.record_type === '内纪'), '内纪', familyTitle.value)
  return out
})
const prePageCount = computed(() =>
  bioPages.value.length + galleryPageCount.value + prefacePages.value.length +
  (nonEmptyGenerationWords.value || []).length)
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

// 牒记式大事记：按“列数”分页（不是字数），每页 2×PH_COLS_PER_HALF 列
const EVENT_CHARS_PER_COL = 48
const EVENT_COLS_PER_HALF = computed(() => isLand.value ? 7 : 6)
const eventPages = computed(() => {
  const list = events.value || []
  const perPage = 2 * EVENT_COLS_PER_HALF.value
  const pages = []; let cur = [], cols = 0
  for (const e of list) {
    const n = Math.max(1, Math.ceil(String(e.content || '').replace(/\*\*/g, '').length / EVENT_CHARS_PER_COL))
    if (cur.length && cols + n > perPage) { pages.push(cur); cur = []; cols = 0 }
    cur.push(e); cols += n
  }
  if (cur.length) pages.push(cur)
  return pages.map((items, i) => ({ items, part: i + 1, total: pages.length }))
})

// ===== 目錄頁：頁碼推導（純數據側；列印分頁由 CSS 固定頁高 + break-after 驅動，每塊恰好一頁，故不必也不能在屏幕上量列印版面） =====
// 兩種頁碼口徑（目錄裏同時標註，避免混淆）：
//  ① 書頁號 = 頁面右下角 CSS counter(sheetpage)。只數 .sheet（凡例短篇表 + 世系牒記表），從 1 起；
//     封面、譜序、祖訓、族規、字輩派語、目錄都不是 .sheet → 不佔號（與頁腳數字嚴格一致，且不受字輩頁溢出影響）。
//  ② 全書頁序（物理頁序）= 封面(1) → 目錄(1) → 譜頭各塊（凡例短篇表 1 頁 / 長篇按實際塊數）→ 字輩（每支 1 頁）→ 世系（每 .sheet 1 頁）。
//     只給「沒有書頁號」的頁用（標 ※）；字輩頁 height:auto，內容極長時可能溢出成 2 頁，會令其後的全書頁序順延，故世系一律用書頁號。
// 依賴 prefacePages / sheets / 字輩（皆隨 isLand 與 --dieji-scale=scaleVal 重算），故本 computed 自動響應。
// 实际打印页号：.book 内所有参与 bookpage 计数的页（.preface 非目录 + .sheet）按 DOM 顺序编号。
// 目錄用这份映射覆盖数据侧推算，保证「目錄起始页 = 实际页脚页码」逐条 0 偏差。
const pageNoMap = ref({})   // data-page-no → 实际打印页号（CSS counter(bookpage) 1-based）

function measurePageNos() {
  const root = document.querySelector('.book')
  if (!root) return
  const map = {}
  const list = root.querySelectorAll('.preface:not(.toc), .sheet')
  let n = 0
  list.forEach((el) => { if (el.dataset.pageNo) map[el.dataset.pageNo] = ++n })
  pageNoMap.value = map
}

const TOC_FOOT = ''
const tocEntries = computed(() => {
  let leaf = 1          // 下一页的页码（封面、目录不参与编号 → 第一个谱头页 = 第 1 页）
  let bookNo = 0        // 已用掉的書頁號（counter(sheetpage)）
  // ① 封面、② 目錄（本頁）都不参与页码编号（属主要求）

  const catOrder = Object.keys(CAT_ORDER)
  const catStat = {}
  catOrder.forEach(c => { catStat[c] = { start: null, blocks: 0, bookStart: null, bookEnd: null, firstIsGrid: false } })

  // ③ 名人傳記（緊跟目錄之後，故最先計數）：每頁一頁（一傳一頁，過長者已由 bioPages 拆成多頁）
  const bioList = bioPages.value
  const bioStart = bioList.length ? leaf : null
  const bioPageCount = bioList.length
  leaf += bioList.length

  // ③′ 圖片展示（名人傳記之後、譜頭/世系之前）：**每個條目一行**，頁碼按條目順序連續；空條目不計頁不出行
  const galSums = entrySummaries(galleryEntries.value)
  const galList = galleryPages.value
  let galCursor = 0
  for (const gs of galSums) { gs.start = leaf + galCursor; galCursor += gs.pages }
  leaf += galCursor

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
  const genList = nonEmptyGenerationWords.value
  const genStart = genList.length ? leaf : null
  leaf += genList.length

  // ⑤′ 五服圖（字輩之後、世系之前）：固定 1 頁
  const fiveFuStart = leaf
  leaf += 1

  // ⑤″ 家族大事记（五服圖之後、祠宇/世系之前）
  const eventStart = eventPages.value.length ? leaf : null
  leaf += eventPages.value.length

  // ⑤‴ 祠宇志 → 茔墓志（五服圖之後、世系之前）
  const shrineStart = shrinePageList.value.length ? leaf : null
  leaf += shrinePageList.value.length
  const graveStart = gravePageList.value.length ? leaf : null
  leaf += gravePageList.value.length

  // ⑥ 世系牒記表：每個 .sheet 一頁（外紀在前、內紀在後，見 sheets computed）；按 section 分別給起止
  const secStat = []
  for (const sh of sheets.value) {
    const sec = sh.section || '内纪'
    let s = secStat.find(x => x.sec === sec)
    if (!s) {
      s = { sec, pages: 0, start: leaf, bookStart: bookNo + 1, bookEnd: bookNo + 1, genMin: null, genMax: null }
      secStat.push(s)
    }
    if (s.pages === 0) { s.start = leaf; s.bookStart = bookNo + 1 }
    s.pages += 1
    s.bookEnd = bookNo + 1
    for (const p of (sh.persons || [])) {
      const g = Number(p.gen)
      if (!Number.isFinite(g)) continue
      if (s.genMin == null || g < s.genMin) s.genMin = g
      if (s.genMax == null || g > s.genMax) s.genMax = g
    }
    bookNo += 1
    leaf += 1
  }

  const rows = []
  // 名人传记（紧跟目录之后 → 排在最前）：一篇一行（只标起始页；多页时标注总页数）
  if (bioList.length) {
    const names = bioList.filter(p => p.isFirst).map(p => p.bio.name).filter(Boolean)
    const notes = [`${names.length} 篇`]
    if (bioPageCount > names.length) notes.push(`共 ${bioPageCount} 页`)
    if (names.length <= 3 && names.length) notes.push(names.join('、'))
    rows.push({
      name: '名人传记',
      note: notes.join(' · '),
      page: `第 ${bioStart} 页`, pageNo: bioStart
    })
  const bioRows = rows.length
  }

  // 圖片展示（緊跟名人傳記之後）：**每個條目一行**（用條目標題；未填則「图片展示 1/2/3…」）
  for (const gs of galSums) {
    const notes = [`${gs.count} 張`]
    if (gs.pages > 1) notes.push(`共 ${gs.pages} 页`)
    rows.push({ name: gs.title, note: notes.join(' · '), page: `第 ${gs.start} 页`, pageNo: gs.start })
  }

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
  // 家族大事记（五服圖之後、祠宇/世系之前）
  if (eventPages.value.length) {
    rows.push({ name: '家族大事记', note: `共 ${eventPages.value.length} 页`, page: `第 ${eventStart} 页`, pageNo: eventStart })
  }
  // 祠宇志 / 茔墓志（五服圖之後、世系之前）
  if (shrinePageList.value.length) {
    rows.push({ name: '祠宇志', note: `共 ${shrinePageList.value.length} 页`, page: `第 ${shrineStart} 页`, pageNo: shrineStart })
  }
  if (gravePageList.value.length) {
    rows.push({ name: '茔墓志', note: `共 ${gravePageList.value.length} 页`, page: `第 ${graveStart} 页`, pageNo: graveStart })
  }
  // 世系正文：内纪/外纪各一行（外纪无则不显示），带世代范围 + 页数（只标起始页）
  for (const s of secStat) {
    const notes = []
    if (s.genMin != null) {
      notes.push(s.genMin === s.genMax ? `${cn(s.genMin)}世` : `${cn(s.genMin)}世–${cn(s.genMax)}世`)
    }
    notes.push(`共 ${s.pages} 页`)
    rows.push({
      name: `${s.sec} 世系`,
      note: notes.join(' · '),
      page: `第 ${s.start} 页`, pageNo: s.start
    })
  }
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

async function exportPDF() {
  // 打印前确保所有字体（含用户上传的自定义字体）已加载，避免 PDF 里字体回退
  try { await document.fonts.ready } catch (e) {}
  // 默认 @page 已由 applyDefaultPrintPage() 常驻注入（见 bind 附近）；这里只需刷新并打印。
  applyDefaultPrintPage()
  window.print()
}

async function load() {
  const [mv, srcList] = await Promise.all([
    api.get('/members/with-spouses'),
    fetchAllSources(),   // 資料來源（行傳頁「据：…」小字腳注；失败返回空数组，不影响谱书主体）
  ])
  const data = mv.data
  members.value = data
  sourceMap.value = groupSourcesByMember(srcList)
  try {
    const { data: fs } = await api.get('/settings/family-surname')
    surname.value = fs.family_surname || ''
    familyName.value = fs.family_name || ''
    try { const { data: bl } = await api.get('/branches'); branchList.value = bl || [] } catch (e) { /* ignore */ }
    tanghao.value = fs.tanghao || ''
    junwang.value = fs.junwang || ''
    zuji.value = fs.zuji || ''
    xiuci.value = fs.xiuci || '重修'                          // 修次（服务器设置，默认「重修」）
    diejiShowSisters.value = fs.dieji_show_sisters !== false  // 默认开
  } catch (e) { /* ignore */ }
  // 谱头文化：谱序/凡例/祖训/族规 + 字辈派语
  try {
    const { data: arts } = await api.get('/articles')
    // 名人传记：只收录「公开展示」的（接口已按「世代 → 排行 → 生年 → id」排好序，前端保持原序）
    try {
      const { data: bios } = await api.get('/biographies')
      biographies.value = (bios || []).filter(b => b.public === 1 || b.public === '1' || b.public == null)
    } catch (e) { biographies.value = [] }
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
.demo-root { display: flex; flex-direction: column; height: 100%;
  /* 書頁「紙張」寬高（除以 --dieji-scale 前的基準 px）：**單一來源**，
     所有頁型（封面/目錄/譜頭/傳記/圖片/世系/行傳）一律用同一個 --paper-w，
     內容再也不會把紙撐寬（第九輪②：豎版牒記式傳記頁曾被 width:max-content 撐寬）。
     豎版＝880px；橫版由 .orient-land 覆寫為 A3 橫版比例（第九輪③）。 */
  --paper-w: calc(880px / var(--dieji-scale, 1));
}

.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.toolbar .title { font-size: 18px; margin-right: 8px; }
.hint { margin-left: auto; color: #8c5a3c; font-size: 12px; }
.mult-label { font-size: 11px; color: #8c5a3c; margin-left: 4px; }

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
  /* 缩放：zoom = 字号缩放 × 预览缩放（预览缩放自动按屏幕 fit，可手动覆盖）；打印时预览缩放不生效 */
  zoom: calc(var(--dieji-scale, 1) * var(--preview-scale, 1));
}

/* 装订预留栏（屏幕预览）：开启装订后每一页（封面/谱头/成员页）左侧留装订线/活页孔栏，虚线分隔（right:100% 跟随页面，避免页面居中时装订线飘到 .book 左侧） */
.demo-root.bind-on .cover,
.demo-root.bind-on .preface,
.demo-root.bind-on .sheet {
  position: relative;
}
.demo-root.bind-on .cover::before,
.demo-root.bind-on .preface::before,
.demo-root.bind-on .sheet::before {
  content: '装订线 · 活页孔预留';
  position: absolute;
  right: calc(100% + 3mm / var(--dieji-scale, 1));
  top: 0;
  bottom: 0;
  width: calc(5mm / var(--dieji-scale, 1));
  writing-mode: vertical-rl;
  text-orientation: mixed;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: calc(11px / var(--dieji-scale, 1));
  letter-spacing: calc(3px / var(--dieji-scale, 1));
  color: #c0392b;
  border-right: 1px dashed #c0392b;
  pointer-events: none;
}

/* 竖版页面自带 overflow:hidden（防内容溢出），会连画在页面外侧的装订线一起裁掉 →
   改用 overflow: clip + overflow-clip-margin:10mm：页面外侧 10mm 内的装订线可见（观感与横版一致，画在页面外的留白里），
   超出 10mm 的内容仍被裁（保留防溢出兜底）；不支持 overflow:clip 的浏览器退回 hidden，仅装订线不显示 */
@media screen {
  .demo-root.bind-on.orient-port .cover,
  .demo-root.bind-on.orient-port .preface,
  .demo-root.bind-on.orient-port .sheet {
    overflow: clip;
    overflow-clip-margin: 10mm;
  }
}

/* ===================== 封面（譜名頁） ===================== */
.cover {
  /* 書頁寬固定（＝基準 880px ÷ 字號縮放）：**不再用 max-content**——否則內容固有寬度
     （如傳記履歷行、長串、寬圖）會把整張紙撐寬，令同書不同頁寬窄不一（見第九輪②）。 */
  width: var(--paper-w, calc(880px / var(--dieji-scale, 1)));
  min-width: 0;
  max-width: none;
  margin: 0 auto calc(28px / var(--dieji-scale, 1));
  background: #fffdf5;
  border: 3px double #1a1a1a;   /* 雙邊框 */
  padding: calc(14px / var(--dieji-scale, 1));
}
.cover-inner {
  border: 1px solid #1a1a1a;
  height: calc(280mm / var(--dieji-scale, 1));
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
  padding: 60px 30px;
  overflow: hidden;
}
/* 竖版封面版式（属主 2026-10-19）：家族名居中；堂號“顺排”到右侧、偏右上（不挤在家族名左边）。
/* 竖版：家族名居中、堂號緊挨右側（同一組），且堂號垂直中線與家族名中線對齊。
   做法：flex 行 [占位条][家族名][堂號]，占位条宽度 = 堂號宽 + 间距 → 家族名中心 = 内容中心；
   堂號用固定列宽 1.3×字号，占位条用同一公式，故左右严格抵消（不再贴内容右缘）；
   align-items:center → 堂號在行高中垂直居中，行高 = max(家族名高, 堂號高)，
   故堂號中線 y 恒等于家族名中線 y。 */
.cover-head {
  display: flex;
  justify-content: center;
  align-items: center;
  width: 100%;
  box-sizing: border-box;
}
.cover-head .cover-title { margin: 0; }
.cover-head .cover-tanghao { margin: 0 0 0 var(--cover-th-gap, 0px); }
/* 竖版专用尺寸：堂號字号 44 屏 / 48 打印；与家族名间距 5mm（打印物理约 7.3mm） */
.demo-root.orient-port {
  --cover-th-fs: calc(44px * var(--cover-size, 1) / var(--dieji-scale, 1));
  --cover-th-gap: calc(5mm / var(--dieji-scale, 1));
}
.demo-root.orient-port .cover-head::before {
  content: '';
  flex: 0 0 calc(1.3 * var(--cover-th-fs) + var(--cover-th-gap));
}
/* 堂號：竖排；列宽固定 1.3×字号（与占位条同宽，保证家族名严格居中） */
.cover-tanghao {
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: var(--cover-th-fs, calc(44px * var(--cover-size, 1) / var(--dieji-scale, 1)));
  font-weight: bold;
  letter-spacing: calc(16px * var(--cover-size, 1) / var(--dieji-scale, 1));
  color: #3a2a18;
  width: calc(1.3 * var(--cover-th-fs, calc(44px * var(--cover-size, 1) / var(--dieji-scale, 1))));
  flex: 0 0 auto;
  margin-bottom: 0;
  padding-top: 0;
}
/* 書名：豎排大字居中 */
.cover-title {
  display: flex;
  justify-content: center;
}
.cover-title .vtitle {
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: calc(var(--title-fs, 60px) * var(--cover-size, 1) / var(--dieji-scale, 1));
  font-weight: bold;
  letter-spacing: calc(var(--title-ls, 22px) * var(--cover-size, 1) / var(--dieji-scale, 1));
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
  font-size: calc(27px * var(--cover-size, 1) / var(--dieji-scale, 1));
  letter-spacing: calc(9px * var(--cover-size, 1) / var(--dieji-scale, 1));
  color: #3a2a18;
}

/* ===== 內封（書名頁）宿主：讓共用組件填滿既有封面版框（雙線邊框由 .cover 提供） ===== */
.it-host { padding: 0 !important; align-items: stretch !important; justify-content: stretch !important; }
.it-host > * { flex: 1 1 auto; min-height: 0; }

/* ===== 五服圖頁（服制圖）：整頁一幅 SVG 矢量圖 ===== */
.fivefu-page .ff-inner { display: flex; flex-direction: column; gap: calc(10px / var(--dieji-scale, 1)); }
.ff-head { flex: 0 0 auto; display: flex; align-items: baseline; justify-content: center;
           gap: calc(16px / var(--dieji-scale, 1)); flex-wrap: wrap; }
.ff-title { font-size: calc(26px * var(--body-size, 1) / var(--dieji-scale, 1)); font-weight: 700;
            letter-spacing: 8px; color: #111; }
.ff-meta { font-size: calc(13px * var(--body-size, 1) / var(--dieji-scale, 1)); color: #6b5a44; letter-spacing: 1px; }
.ff-body { flex: 1 1 auto; min-height: 0; display: flex; align-items: center; justify-content: center; }

/* ===== 牌記頁（全書末頁）：不叫 .preface/.cover → 不參與 bookpage 計數 ===== */
.colo-page {
  width: var(--paper-w, calc(880px / var(--dieji-scale, 1)));
  margin: 0 auto calc(28px / var(--dieji-scale, 1));
  background: #fffdf5;
  border: 3px double #1a1a1a;
  padding: calc(14px / var(--dieji-scale, 1));
  box-sizing: border-box;
  height: calc(280mm / var(--dieji-scale, 1));
  overflow: hidden;
}
.colo-inner { border: 1px solid #1a1a1a; height: 100%; }
.demo-root.orient-land .colo-page { width: var(--paper-w); height: var(--paper-h); }

/* ===== 横版封面：堂号/谱名/郡望全部横排，适配 A4 横版矮宽纸 ===== */
.demo-root.orient-land .cover-inner {
  height: calc(190mm / var(--dieji-scale, 1));
  flex-direction: column;
  justify-content: center;
  gap: 12mm;
  padding: 18mm 24mm;
  overflow: hidden;
}
/* 堂号：横排，在家族名之上（上下排，不并排）；字号 44（与家族名同比例放大）。
   DOM 顺序是「家族名→堂號」，column-reverse 把堂號排到上方。 */
.demo-root.orient-land .cover-head {
  display: flex;
  flex-direction: column-reverse;
  align-items: center;
  justify-content: center;
  gap: calc(10mm / var(--dieji-scale, 1));
  width: 100%;
}
.demo-root.orient-land .cover-head::before { content: none; }
.demo-root.orient-land .cover-tanghao {
  writing-mode: horizontal-tb;
  text-orientation: mixed;
  font-size: calc(44px * var(--cover-size, 1) / var(--dieji-scale, 1));
  letter-spacing: calc(16px * var(--cover-size, 1) / var(--dieji-scale, 1));
  width: auto;
  margin: 0;
  padding-top: 0;
  align-self: center;
}
/* 谱名：横排超大字居中（李氏家谱一字排开） */
.demo-root.orient-land .cover-title .vtitle {
  writing-mode: horizontal-tb;
  text-orientation: mixed;
  font-size: calc(var(--title-fs-land, 80px) * var(--cover-size, 1) / var(--dieji-scale, 1));
  letter-spacing: calc(var(--title-ls-land, 24px) * var(--cover-size, 1) / var(--dieji-scale, 1));
  line-height: 1.1;
}
/* 底部信息：郡望/祖籍/歲次横排一行（或各占一行横排），间距拉开 */
.demo-root.orient-land .cover-meta {
  margin-top: 0;
  flex-direction: column;
  gap: 4mm;
  align-items: center;
}
.demo-root.orient-land .cover-meta .meta-line span {
  writing-mode: horizontal-tb;
  text-orientation: mixed;
  font-size: calc(27px * var(--cover-size, 1) / var(--dieji-scale, 1));
  letter-spacing: calc(9px * var(--cover-size, 1) / var(--dieji-scale, 1));
}

/* 谱头页：与封面同风格，双边框 */
.preface {
  /* 同 .cover：固定紙寬，不用 max-content（內容不得撐寬書頁） */
  width: var(--paper-w, calc(880px / var(--dieji-scale, 1)));
  min-width: 0;
  max-width: none;
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
.preface.sg-plate-page .preface-inner { padding: 10px 14px; }
.sg-page .sg-inner { min-height: 0; height: 100%; padding: calc(24px / var(--dieji-scale, 1)) calc(30px / var(--dieji-scale, 1)); --sg-fs: 14px; }
.demo-root.orient-land .sg-page .sg-inner { --sg-fs: 14px; }
.sg-page.sg-fs16 .sg-inner { --sg-fs: 16px; }   /* 祠宇志/茔墓志：14→16，仅牒记两本 */
.epc-page .sg-inner { padding-left: 0; padding-right: 0; }   /* 大事记整页竖格：左右不留白，格线贴版心 */
/* 竖版屏幕预览：页面统一强制 A3 竖版页高（封面/谱头/成员页长短一致），内容框填满页高 */
.demo-root.orient-port .cover,
.demo-root.orient-port .preface,
.demo-root.orient-port .sheet {
  height: calc(280mm / var(--dieji-scale, 1));
  box-sizing: border-box;
  overflow: hidden;
}
.demo-root.orient-port .cover-inner {
  min-height: 0;
  height: 100%;
}
.demo-root.orient-port .preface-inner {
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
}
.demo-root.orient-port .preface-inner > .vgrid {
  flex: 1 1 auto;
  min-height: 0;
  height: auto;
  margin-top: calc(4mm / var(--dieji-scale, 1));
}
/* 横版屏幕预览：同样固定页高 + 内框纵排 + 竖格撑满剩余高度
   （否则竖格高度随字数变化 → 实测容量不稳定、页底留大片空白） */
.demo-root.orient-land .preface-inner {
  height: calc(190mm / var(--dieji-scale, 1));
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
}
.demo-root.orient-land .preface-inner > .vgrid {
  flex: 1 1 auto;
  min-height: 0;
  height: auto;
}
/* 圖片展示頁：
   ① .preface 必須是 flex 縱向容器、內框改成 flex:1（不能再 height:100%）：
      否則內框獨佔整頁高，標籤與頁碼被擠出頁外、再被 .preface 的 overflow:hidden 裁掉（實測溢出 53px）。
   ② 紙張寬要寫死成固定紙寬（--paper-w）：圖片的固有寬度會參與 width:max-content 的計算，
      不寫死時整頁被圖片撐到 2 倍寬（實測 616→1565px）。第九輪②後 --paper-w 已是固定值
      （豎版 880px／橫版 A3 橫版比例），這裡沿用同一個變量，各頁型紙寬才完全一致。 */
.demo-root.orient-port .preface.gallery-page,
.demo-root.orient-land .preface.gallery-page {
  display: flex;
  flex-direction: column;
  width: var(--paper-w, calc(880px / var(--dieji-scale, 1)));
  min-width: 0;
  max-width: none;
}
/* 橫版圖片頁：頁高固定＝其他橫版書頁（--paper-h ＝ 版心 190mm ＋ .preface 內邊距/雙邊框 34px）。
   否則頁高會被圖片內容撐高（實測 2 張 667px、4 張 1266px）→ 格子大小不固定。
   格子仍按版心均分（grid 用 1fr，頁寬/頁高一改就自動均分，公式不變）。 */
.demo-root.orient-land .preface.gallery-page { height: var(--paper-h, calc(190mm / var(--dieji-scale, 1) + 34px)); }
.demo-root.orient-port .preface.gallery-page > .preface-inner,
.demo-root.orient-land .preface.gallery-page > .preface-inner {
  flex: 1 1 auto;
  height: auto;
  min-height: 0;
}
.demo-root.orient-land .measure-probe .preface-inner {
  display: block;   /* 探针不能用 flex 纵排（同竖版：每字步长会被量成 1px） */
  height: calc(190mm / var(--dieji-scale, 1));
}
/* 长篇探针页高必须与真实页面一致（否则每列字数算多 → 预览/打印被裁）；横版探针已按 190mm/scale 量 */
.demo-root.orient-port .measure-probe .preface {
  height: calc(280mm / var(--dieji-scale, 1));
  box-sizing: border-box;
  overflow: hidden;
}
.demo-root.orient-port .measure-probe .preface-inner {
  min-height: 0;
  height: 100%;
  box-sizing: border-box;
  display: block;   /* 探针不能用 flex 纵排：.vcol 的 flex-basis 会变成高度 → 每字步长测成 ~1px，容量虚高 → 长篇被裁 */
}
.preface-cat {
  text-align: center;
  font-size: calc(20px * var(--body-size, 1) / var(--dieji-scale, 1));
  letter-spacing: calc(6px / var(--dieji-scale, 1));
  color: #8c5a3c;
  margin-bottom: calc(8px / var(--dieji-scale, 1));
}
.preface-title {
  text-align: center;
  font-size: calc(30px * var(--body-size, 1) / var(--dieji-scale, 1));
  font-weight: bold;
  letter-spacing: calc(8px / var(--dieji-scale, 1));
  color: #1a1a1a;
  margin-bottom: calc(6px / var(--dieji-scale, 1));
}
.preface-meta {
  text-align: center;
  font-size: calc(13px * var(--body-size, 1) / var(--dieji-scale, 1));
  color: #8c5a3c;
  margin-bottom: calc(8px / var(--dieji-scale, 1));
}
/* 页码（谱序/祖训等长页尾部）：同样随字号缩小，不占正文地盘 */
.page-flag { display: flex; justify-content: space-between; writing-mode: horizontal-tb; font-size: calc(14px / var(--dieji-scale, 1)); line-height: 1.4; }
/* 续页：旧右下角 bookpage 页码隐藏，只保留页首续页标右侧的新页码 */

/* 名人传记页：姓名后的「字X 號Y」小字（版式其余沿用 .preface-long） */
.bio-zh { font-size: calc(16px * var(--body-size, 1) / var(--dieji-scale, 1)); line-height: 1.2; letter-spacing: 1px; margin-left: 6px; }

/* ===== 目錄頁：沿用 .preface 的頁框與分頁（height 280mm/190mm + break-after: page + page: dieji-sheet[-land]），
   只新增條目樣式；標題風格與本檔譜頭頁（.preface-title）一致（橫排、居中、大字距）。
   寬度全部用 flex 自適應（不寫死），隨 isLand 與 --dieji-scale 自動縮放。 ===== */
.toc-title {
  text-align: center;
  font-size: calc(30px * var(--body-size, 1) / var(--dieji-scale, 1));
  font-weight: bold;
  letter-spacing: calc(8px / var(--dieji-scale, 1));
  color: #1a1a1a;
  margin-bottom: calc(18px / var(--dieji-scale, 1));
}
.toc-list {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  padding: 0 calc(6px / var(--dieji-scale, 1));
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
/* 橫版「2 張」＝左右並排（2 列 × 1 行）：橫版頁寬（880~1240）遠大於頁高，上下排會把兩張圖壓成細條；
   豎版維持上下（2 行 × 1 列）—— 屬主口徑「只改橫版」。 */
.demo-root.orient-land .gallery-doc.g2 .gallery-grid { grid-template-columns: repeat(2, 1fr); grid-template-rows: 1fr; }
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
  gap: calc(6px / var(--dieji-scale, 1));
  font-size: calc(17px * var(--body-size, 1) / var(--dieji-scale, 1));
  line-height: 2.2;
  color: #1a1a1a;
}
.toc-name { flex: 0 0 auto; letter-spacing: calc(2px / var(--dieji-scale, 1)); }
.toc-note { flex: 0 0 auto; font-size: calc(12px * var(--body-size, 1) / var(--dieji-scale, 1)); color: #8c5a3c; }
.toc-dots {
  flex: 1 1 auto;
  align-self: baseline;      /* baseline 對齊時空盒基線在底邊 → 點線正好落在字腳 */
  min-width: calc(20px / var(--dieji-scale, 1));
  border-bottom: 1px dotted #b08840;
}
.toc-page { flex: 0 0 auto; white-space: nowrap; letter-spacing: calc(1px / var(--dieji-scale, 1)); }
.toc-foot {
  flex: 0 0 auto;
  margin-top: calc(12px / var(--dieji-scale, 1));
  padding-top: calc(6px / var(--dieji-scale, 1));
  
  font-size: calc(12px * var(--body-size, 1) / var(--dieji-scale, 1));
  line-height: 1.6;
  color: #8c5a3c;
}
/* 长页（谱序/祖训/族规）屏幕预览：上下边距对齐打印版（60px→20px），凡例保持原样 */
.preface-long:not(.rule-long) .preface-inner {
  padding: calc(20px / var(--dieji-scale, 1)) calc(40px / var(--dieji-scale, 1));
}
/* 长篇正文“内容框”显式定高：竖格高 = 页高 − 上下留白 − 页码位 − 余量（竖 900 / 横 620）
   带标题的页内容框缩小一点（让出标题高 98px）；没页码的页大一点（+20px），有页码的让出页码位
   标题随号数除 scale（物理恒定），故让位值与正文同除 scale：98px × 大字倍率 / scale */
.demo-root.orient-port .preface-long:not(.rule-long) .vgrid {
  flex: 0 0 auto;
  height: calc(900px / var(--dieji-scale, 1));
}
.demo-root.orient-port .preface-long:not(.rule-long) .vgrid.no-flag {
  height: calc(920px / var(--dieji-scale, 1));
}
.demo-root.orient-port .preface-long:not(.rule-long) .preface-inner.with-head .vgrid {
  height: calc((900px - 98px * var(--body-size, 1)) / var(--dieji-scale, 1));
}
.demo-root.orient-port .preface-long:not(.rule-long) .preface-inner.with-head .vgrid.no-flag {
  height: calc((920px - 98px * var(--body-size, 1)) / var(--dieji-scale, 1));
}
.demo-root.orient-land .preface-long:not(.rule-long) .vgrid {
  flex: 0 0 auto;
  height: calc(620px / var(--dieji-scale, 1));
}
.demo-root.orient-land .preface-long:not(.rule-long) .vgrid.no-flag {
  height: calc(640px / var(--dieji-scale, 1));
}
.demo-root.orient-land .preface-long:not(.rule-long) .preface-inner.with-head .vgrid {
  height: calc((620px - 98px * var(--body-size, 1)) / var(--dieji-scale, 1));
}
.demo-root.orient-land .preface-long:not(.rule-long) .preface-inner.with-head .vgrid.no-flag {
  height: calc((640px - 98px * var(--body-size, 1)) / var(--dieji-scale, 1));
}

/* ===== 凡例短篇：与成员牒记同一张表（每条占一整列：标题→姓名格，正文→资料格格内竖排换行） ===== */
/* 列/格全部继承 .person-col / .cell-rel / .cell-name / .cell-detail，保证高度、框线、页高与成员表完全一致 */
/* 凡例标题格文字顶部对齐（居中会显得标题长短不一时参差不齐） */
.rule-sheet .cell-name { align-items: flex-start; justify-content: center; padding-top: 10px; overflow: hidden; }
/* 凡例不换页后，超长正文在格内竖排换行，超出格高部分裁掉（不溢出框线） */
.rule-sheet .cell-detail { overflow: hidden; }
/* 凡例短表：正文/标题统一 18px（只改字号，不改表格结构） */
.rule-sheet .cell-detail { font-size: 18px; line-height: 1.5; }
.rule-sheet .cell-name .rule-name { font-size: 18px; line-height: 1.3; }
.rule-name { white-space: normal; }
/* 凡例列已取消最上面的关系格；标题格高度由每页最长标题算出（--rule-name-h），整页等高 */
.rule-sheet .rule-col .cell-name,
.rule-sheet .blank .cell-name {
  flex: 0 0 var(--rule-name-h, 80px);
}
.cell-name .rule-name {
  /* 凡例标题比人名长（如「十三、本支源流及修谱缘起」），姓名格内竖排多竖行 */
  font-size: calc(17px * var(--body-size, 1));
  line-height: 1.3;
  letter-spacing: 1px;
}

/* ===== 长篇竖行容量测量探针（隐藏） ===== */
/* 探针外层裁剪盒：探针宽 880px（不随 --preview-scale 缩放）且绝对定位在文档原点，
   不裁剪会把文档 scrollWidth 撑到 881px（手机 390 宽时多出约 490px 可左右拖走的空白）。
   height:0 + overflow:hidden 只裁溢出，不参与布局，探针自身量测值不变。 */
.probe-clip {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  height: 0;
  overflow: hidden;
  /* 探針一律按「基準紙寬 880px」量（與歷輪 JS 容量口徑一致）：橫版紙張放寬到 A3 比例後，
     探針若跟著變寬會改變每列字數/欄數口徑、連帶改動分頁；故在此鎖回 880（第九輪③）。 */
  --paper-w: calc(880px / var(--dieji-scale, 1));
}
.measure-probe {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  width: var(--paper-w, calc(880px / var(--dieji-scale, 1)));
  min-width: 0;
  max-width: none;
  visibility: hidden;
  pointer-events: none;
  z-index: -1;
}
.measure-probe .vcol { width: 64px; }
/* 成员资料格容量探针：独立竖行（测每字步长，高度随内容）；sheet 去掉边距便于测量 */
.detail-step {
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: calc(13px * var(--body-size, 1) * min(1, 1.6 / var(--dieji-scale, 1)));
  line-height: 1.584;
  padding: 6px 4px;
  width: auto;
  height: auto;
}
.detail-hstep {
  display: inline-block;
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: calc(13px * var(--body-size, 1) * min(1, 1.6 / var(--dieji-scale, 1)));
  line-height: 1.5;
  padding: 6px 4px;
  height: auto;
  width: auto;
}
.probe-sheet { margin: 0; border: none; background: transparent; }

/* ===== 长篇整页（谱序/祖训/族规）：竖排竖行、定列不溢出 ===== */
.vgrid {
  display: flex;
  flex-direction: row-reverse;
  align-items: stretch;
  justify-content: center;   /* 竖排末页不满列时居中，左右均衡留白 */
}
.bio-long .vcol { font-size: 18px; line-height: 2.0; }
/* 传记正文顶对齐：打印时网格更高，居中会把正文下推 */
.bio-long .vgrid { justify-content: flex-start; align-items: flex-start; margin-top: 0 !important; }
.vcol {
  writing-mode: vertical-rl;
  text-orientation: upright;
  flex: 0 0 calc(100% / var(--vcol-n, 14));   /* 固定列宽=页宽/列数，末页不满列靠右不留大空 */
  min-width: 0;
  padding: 6px 2px;
  /* 正规字号不缩；初号等超大字号略缩（最大字号内容小一点，避免正文大于标题、一页装不下） */
  font-size: calc(16px * var(--body-size, 1) * min(1, 1.6 / var(--dieji-scale, 1)));
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
  font-size: calc(16px * var(--body-size, 1));
  line-height: 1.9;
  overflow: hidden;
}
.rule-vcol-title {
  flex: 1 1 0 !important;
  max-width: none !important;
  font-size: calc(19px * var(--body-size, 1)) !important;
  font-weight: bold;
  letter-spacing: 2px;
  padding: 6px 10px 6px 14px !important;
  color: #1a1a1a;
  line-height: 1.6 !important;
  border-left: 1px solid #1a1a1a;    /* 标题列与正文之间竖界（右缘起首） */
}

/* 字辈派语页 —— 横排布局（仅此页横排，其余谱页仍竖排） */
.genword {
  /* 字辈内容横排后更扁，去掉谱头长页的固定超高框，按内容自适应 */
}
.genword .preface-inner {
  min-height: auto;
  display: flex;
  flex-direction: column;
  justify-content: center;
}
.genword-branch {
  text-align: center;
  font-size: calc(20px * var(--body-size, 1));
  font-weight: bold;
  color: #3a2a18;
  margin-bottom: 14px;
}
.genword-poem {
  text-align: center;
  font-size: calc(20px * var(--body-size, 1));
  letter-spacing: 6px;
  color: #1a1a1a;
  line-height: 1.8;
  margin: 0 auto 26px;
  max-width: 100%;
}
.genword-chars {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
  justify-content: center;
  max-width: 400px;
  margin: 0 auto;
}
.genword-char {
  border: 1px solid #1a1a1a;
  aspect-ratio: 1 / 1.05;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  font-size: calc(22px * var(--body-size, 1));
  color: #1a1a1a;
}
.genword-char small { font-size: calc(11px * var(--body-size, 1)); color: #8c5a3c; margin-top: 2px; }

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
  /* 固定紙寬（不用 max-content）：世系/凡例表的欄寬由 flex 均分，不隨內容撐寬 */
  width: var(--paper-w, calc(880px / var(--dieji-scale, 1)));
  min-width: 0;
  max-width: none;
  margin: 0 auto calc(28px / var(--dieji-scale, 1));
  background: #fffdf5;
  border: 1.5px solid #1a1a1a;
  height: calc(280mm / var(--dieji-scale, 1));
}
.sheet:last-child { margin-bottom: 0; }

/* 横版屏幕预览：~~~ A3 橫版比例（420:297 = 1.414） ~~~
   第九輪③：原先紙寬＝880px（由 min-width/max-content 決定）、紙高＝版心 190mm ＋ 外框 34px
   ≈ 752px → 616×526（比例 1.171），看起來「偏方」。
   現在**所有橫版頁型共用同一個固定紙框**：
     高 = 752px（＝版心 190mm 718px ＋ .preface 內邊距/雙邊框 34px，與列印同一套幾何）
     寬 = 752px × 1.4142 ≈ 1063.5px（A3 橫版比例；≈281mm，也與列印版心 280mm 對齊）
   → 比例 1.4142，逐頁一致；紙寬不再受內容影響。
   注意：**不再**給 .book 設 min-width/橫向滾動——那會把「預覽縮放」的紙張撐開邏輯頂掉
   （zoom 放大時 .book 被 min-width 釘住 → 變成文檔級橫滾而不是 .book 內部橫滾）。
   窄屏適配改由 --preview-scale 的「適應屏幕」負責（它按實測紙寬算，故紙寬改了會自動跟隨）；
   放大後紙張靠 .book 的 overflow:auto 橫滾。 */
.orient-land .book {
  overflow-x: auto;
}
/* --paper-w：橫版紙寬（A3 橫版比例）＝ --paper-h × 1.4142，除以字號縮放 */
.demo-root.orient-land {
  --paper-w: calc(1063.5px / var(--dieji-scale, 1));
  --paper-h: calc(752px / var(--dieji-scale, 1));
}
.demo-root.orient-land .cover,
.demo-root.orient-land .preface,
.demo-root.orient-land .sheet {
  width: var(--paper-w);
  min-width: 0;
  max-width: none;
  height: var(--paper-h);      /* 統一紙高：封面/目錄/譜頭/傳記/圖片/世系/行傳 一致 */
  box-sizing: border-box;
  overflow: hidden;
}
/* .sheet 紙高由 190mm（718px）抬到與其它頁型相同的 752px：只增不減 → 世系格只會更寬裕，不會裁字。
   （.sheet 邊框 1.5px：內容區 749px ≥ 原 718px） */
.orient-land .cover-inner { min-height: calc(190mm / var(--dieji-scale, 1)); }
/* 横版谱头长篇（谱序/祖训/族规）页高对齐 A3 横版，不再用竖版 1180px 高度（否则呈方形） */
.orient-land .preface-inner { min-height: calc(190mm / var(--dieji-scale, 1)); }
/* 字辈页屏幕也满页框、内容居中，不显示半页 */
.orient-land .preface.genword { min-height: calc(190mm / var(--dieji-scale, 1)); }
.orient-land .preface.genword .preface-inner {
  min-height: calc(190mm / var(--dieji-scale, 1));
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 5mm;
}
/* 横版字辈派语：纸宽，16 字一行排完（竖版 8 列两行）；方格强制等大正方形 */
.orient-land .genword-chars { grid-template-columns: repeat(8, 1fr); gap: 6px; max-width: none; justify-content: center; }
.orient-land .genword-char { font-size: calc(15px * var(--body-size, 1)); width: 38px; height: 38px; box-sizing: border-box; }

/* 谱名列（最右）：竖排 */
.col-title {
  flex: 0 0 42px;
  writing-mode: vertical-rl;
  text-orientation: upright;
  font-size: 23px;
  font-weight: bold;
  letter-spacing: 4px;
  text-align: center;
  display: grid;
  place-items: center;
  padding: 0;
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
  font-size: calc(12px * var(--body-size, 1));
  color: #333;
  border-bottom: 1px solid #1a1a1a;
  overflow: hidden;
}
/* 條首小字＝固定兩段（屬主口徑）：右＝父名＋公，左＝行第。兩段各自成豎列、格內不換行；
   字號固定（不隨字數縮、不隨正文字號放大，保證 5 字長父名也不裁字）：
   屏幕格高 80px → 14px；打印豎版 62px → 12px、橫版 52px → 10px（見打印塊）。 */
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
.demo-root.orient-port .cell-rel .rel2 > span { font-size: min(calc(16px * var(--body-size, 1)), 16px); }
.demo-root.orient-land .cell-rel .rel2 > span { font-size: min(calc(16px * var(--body-size, 1)), 16px); }
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
  font-size: calc(19px * var(--name-size, 1));
  font-weight: bold;
  letter-spacing: 2px;
  border-bottom: 1px solid #1a1a1a;
}
/* 仅横版：名字格高度略减；竖版/支谱不动 */
.demo-root.orient-land .cell-name { flex: 0 0 92px; }
.demo-root.orient-land .gen-bar {
  font-size: calc(14px * var(--body-size, 1));
  letter-spacing: 1px; line-height: 1; padding: 1px 2px; margin: 0;
  display: inline; padding: 0 2px; white-space: nowrap; vertical-align: top;
}
/* 横版成员页：行距略收；标签不预留行间空白 */
.demo-root.orient-land .cell-name .vtext {
  font-size: calc(19px * var(--name-size, 1));  /* 17 → 19 */
  line-height: 1.25;
}
.demo-root.orient-land .cell-detail {
  font-size: calc(16px * var(--body-size, 1) * var(--p-font, 1) * min(1, 1.6 / var(--dieji-scale, 1))); /* 18 → 16 */
  line-height: 1.35;
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
.demo-root.orient-port .cell-detail { letter-spacing: 1px; }
.cell-detail {
  writing-mode: vertical-rl;
  text-orientation: upright;
  flex: 1 1 0;
  min-height: 0;
  overflow: hidden;
  color: #222;
  line-height: 1.5;
  padding: 6px 4px;
  font-size: calc(18px * var(--body-size, 1) * var(--p-font, 1) * min(1, 1.6 / var(--dieji-scale, 1)));
}

/* 資料來源小字腳注：接在资料格末尾（同一條竖列 inline 续写），em 与资料格字号同比缩放，
   格高/格宽/列数一律不动；无来源时不渲染（模板 v-if），不留空行。 */
.src-note { font-size: 0.72em; color: #4a4a4a; letter-spacing: 0; }

/* 世代黑条：竖排流首个行内块（贴右缘、格顶起），资料文字接在它下方 */
.gen-bar {
  display: inline;
  background: #111;
  color: #fff;
  font-size: calc(18px * var(--body-size, 1));
  font-weight: bold;
  letter-spacing: 2px;
  padding: 0 2px;
  margin: 0 2px;
  white-space: nowrap;
}

/* 空白列：补满框线 */
.person-col.blank { background: transparent; }

/* ============ 移动端 ============ */
@media (max-width: 767px) {
  .book { padding: 10px 6px; }
  .sheet { max-width: none; }
  .col-title { font-size: calc(12px * var(--name-size, 1)); letter-spacing: 2px; flex-basis: 34px; }
  .cell-rel { font-size: calc(10px * var(--body-size, 1)); }
  .cell-name { font-size: calc(15px * var(--name-size, 1)); letter-spacing: 1px; }
  .cell-detail { font-size: calc(18px * var(--body-size, 1) * min(1, 1.6 / var(--dieji-scale, 1))); line-height: 1.5; min-height: 120px; }
  .cell-name .rule-name { font-size: calc(18px * var(--body-size, 1)); }
  .gen-bar { font-size: calc(18px * var(--body-size, 1)); }
  /* 长篇竖行手机端字号略收，锁定不被浏览器自动放大，配合 0.88 安全余量不裁字 */
  .vcol { font-size: calc(14px * var(--body-size, 1) * min(1, 1.6 / var(--dieji-scale, 1))); line-height: 1.9; -webkit-text-size-adjust: 100%; }
  /* 字辈派语手机端：横排紧凑，格子缩小保证一整页放得下、不裁底部格子 */
  .genword .preface-inner { padding: 28px 18px; overflow: visible; }
  .genword-branch { font-size: calc(17px * var(--body-size, 1)); margin-bottom: 10px; }
  .genword-poem { font-size: calc(16px * var(--body-size, 1)); letter-spacing: 3px; margin-bottom: 18px; line-height: 1.7; }
  .genword-chars { grid-template-columns: repeat(4, 1fr); gap: 8px; max-width: 360px; }
  .genword-char { font-size: calc(18px * var(--body-size, 1)); }
  .genword-char small { font-size: calc(10px * var(--body-size, 1)); }
}

/* ============ 打印 / 导出 PDF（scoped 部分） ============ */
@media print {
  .demo-root { height: auto; display: block; }
  .book {
    overflow: visible !important;
    overflow-x: hidden !important;
    padding: 0;
    background: #fff;
    border: none;
    border-radius: 0;
    display: block;
    counter-reset: sheetpage bookpage;
    width: 100% !important;
    max-width: 100% !important;
    zoom: var(--dieji-scale, 1);
  }
  /* 統一：A4 直印內容區固定 265mm（頁高297-上下邊距32），手機/桌面一致不拆頁 */
  /* 封面獨佔一頁；固定頁高 + 塊級流，避免手機 WebView 把封面拆成兩頁 */
  .cover {
    max-width: 100% !important;
    width: 100%;
    margin: 0;
    height: calc(280mm / var(--dieji-scale, 1));
    box-sizing: border-box;
    overflow: hidden !important;
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
  /* 主封面（非內封）：內框縱向居中——「堂號＋家族名」一組在上、信息行在下，行距用 gap；
     整組上下留余量，故 cover-meta 不會再被擠出封面框（修前實測超出下框 3.7px 局部＝打印 ~5.4px）。 */
  .cover:not(.inner-cover) > .cover-inner {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    min-height: 0;
    padding: 0 calc(30px / var(--dieji-scale, 1));
    gap: calc(12mm / var(--dieji-scale, 1));
  }
  .cover:not(.inner-cover) > .cover-inner .cover-head,
  .cover:not(.inner-cover) > .cover-inner .cover-title,
  .cover:not(.inner-cover) > .cover-inner .cover-meta { margin: 0; }
  /* 注意：不要把 .cover-tanghao 加进上面这条 margin:0 —— 竖版要保留 margin-left＝堂號间距 */
  /* 竖版堂號打印字号 48（与占位条/列宽共用 --cover-th-fs，故家族名仍严格居中） */
  .demo-root.orient-port { --cover-th-fs: calc(48px * var(--cover-size, 1) / var(--dieji-scale, 1)); }
  .cover-tanghao {
    display: inline-block;
    margin: calc(22mm / var(--dieji-scale, 1)) 0 0;
    font-size: calc(48px * var(--cover-size, 1) / var(--dieji-scale, 1));
    letter-spacing: calc(16px * var(--cover-size, 1) / var(--dieji-scale, 1));
  }
  .cover-title {
    display: block;
    margin-top: calc(16mm / var(--dieji-scale, 1));
  }
  .cover-title .vtitle {
    font-size: calc(var(--title-fs, 84px) * var(--cover-size, 1) / var(--dieji-scale, 1));
    letter-spacing: calc(var(--title-ls, 30px) * var(--cover-size, 1) / var(--dieji-scale, 1));
  }
  .cover-meta {
    margin-top: calc(24mm / var(--dieji-scale, 1));
    gap: calc(34px * var(--cover-size, 1) / var(--dieji-scale, 1));
    justify-content: center;
  }
  .cover-meta .meta-line span {
    font-size: calc(30px * var(--cover-size, 1) / var(--dieji-scale, 1));
    letter-spacing: calc(9px * var(--cover-size, 1) / var(--dieji-scale, 1));
  }
  /* 谱头页：每个排版块（网格页或长篇页）独占一页 */
  .preface {
    max-width: none;
    width: 100%;
    margin: 0;
    height: calc(280mm / var(--dieji-scale, 1));
    box-sizing: border-box;
    page-break-after: always;
    break-after: page;
    page-break-inside: avoid;
    break-inside: avoid;
  }
  /* 牌記（全書末頁）：另起一頁、頁後不再分頁（末頁） */
  .colo-page {
    max-width: none;
    width: 100%;
    margin: 0;
    height: calc(280mm / var(--dieji-scale, 1));
    box-sizing: border-box;
    page-break-before: always;
    break-before: page;
    page-break-after: auto;
    break-after: auto;
    page-break-inside: avoid;
    break-inside: avoid;
  }
  .colo-inner { height: 100%; }
  .preface-inner {
    display: block;
    min-height: 0;
    height: 100%;
    padding: calc(16mm / var(--dieji-scale, 1)) calc(14mm / var(--dieji-scale, 1));
    box-sizing: border-box;
  }
  /* 谱头标题（谱序/祖训/族规）：字号/字距/边距除以 scale，配合 zoom 保持物理恒定（大字号下标题不被放大占页，也不比正文小） */
  .preface-cat {
    font-size: calc(20px * var(--body-size, 1) / var(--dieji-scale, 1));
    letter-spacing: calc(6px / var(--dieji-scale, 1));
    margin-bottom: calc(8px / var(--dieji-scale, 1));
  }
  .preface-title {
    font-size: calc(30px * var(--body-size, 1) / var(--dieji-scale, 1));
    letter-spacing: calc(8px / var(--dieji-scale, 1));
    margin-bottom: calc(6px / var(--dieji-scale, 1));
  }
  .preface-meta {
    font-size: calc(13px * var(--body-size, 1) / var(--dieji-scale, 1));
    margin-bottom: calc(8px / var(--dieji-scale, 1));
  }
  /* 长篇页：flex 纵向布局——标题固定、正文竖格自适应剩余高度（有标题不再把固定高度内容挤下去裁切） */
  .preface-long .preface-inner {
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
  }
  /* 正文(谱序/祖训/族规)长页：上下边距累计各缩（16mm→12.8→10.24→8.192mm），省出空间给正文竖格增高；凡例(rule-long)保持原样 */
  .preface-long:not(.rule-long) .preface-inner {
    padding: calc(20px / var(--dieji-scale, 1)) calc(40px / var(--dieji-scale, 1));
  }
  /* 标题不收缩：避免 flex 压缩标题导致标题被正文顶压/遮挡 */
  .preface-long .preface-cat,
  .preface-long .preface-title,
  .preface-long .preface-meta {
    flex: 0 0 auto;
  }
  .preface-long .vgrid {
    flex: 1 1 auto;
    height: auto;
    min-height: 0;
    margin-top: calc(4mm / var(--dieji-scale, 1));
  }
  /* 正文(book)标题与正文距离累计减（4mm→3.2→2.56mm）；凡例保持 4mm */
  .preface-long:not(.rule-long) .vgrid {
    margin-top: calc(2.56mm / var(--dieji-scale, 1));
  }
  /* 凡例表完全复用成员表打印样式（330mm 页高 / rel 62px / name 78px / detail flex:1 裁切） */
  /* 长篇竖行：占满框内高度，定列裁切，绝不溢出框线 */
  .vgrid {
    height: calc(190mm / var(--dieji-scale, 1));
    box-sizing: border-box;
  }
  .vcol { overflow: hidden; line-height: 2.0; padding: 6px 2px; word-break: keep-all; }
  .rule-sheet .cell-name { padding-top: 4px; }
  .genword-poem { max-height: none; }
  /* 字辈派语：横排后内容少，放开谱头长页的固定 268mm 高度与 overflow，
     按内容自适应高度居中排布，确保全部格子（含最后一行）完整打印、不被裁切、不跨页 */
  .preface.genword {
    height: auto;
    min-height: calc(280mm / var(--dieji-scale, 1));
    padding: 0;
    overflow: visible;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .preface.genword .preface-inner {
    height: auto;
    min-height: 0;
    overflow: visible;
    padding: calc(20mm / var(--dieji-scale, 1)) calc(16mm / var(--dieji-scale, 1));
    width: 100%;
  }
  .genword-chars { grid-template-columns: repeat(4, 1fr); gap: 6px; max-width: calc(120mm / var(--dieji-scale, 1)); }
  .genword-char { font-size: calc(18px * var(--body-size, 1)); }
  .genword-poem { font-size: calc(17px * var(--body-size, 1)); letter-spacing: calc(4px * var(--body-size, 1)); margin-bottom: calc(14mm / var(--dieji-scale, 1)); }
  .genword-branch { font-size: calc(18px * var(--body-size, 1)); margin-bottom: calc(8mm / var(--dieji-scale, 1)); }
  /* 牒記表：每個 sheet 固定一頁高（留底部空白放頁碼），強制每表一頁 */
  .sheet {
    max-width: none;
    width: 100%;
    height: calc(280mm / var(--dieji-scale, 1));
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
    font-size: calc(18px * var(--body-size, 1) * min(1, 1.6 / var(--dieji-scale, 1)));
    line-height: 1.5;
    padding: 6px 4px;
  }
  /* 列的關係/姓名格打印時略壓縮，給小注騰空間 */
  .cell-rel { flex-basis: calc(62px * var(--name-size, 1)); }
  .cell-name { flex-basis: calc(78px * var(--name-size, 1)); }
  .demo-root.orient-land .cell-name { flex-basis: 74px; }
  /* 條首小字兩段：打印豎版格高 62px → 字號 12px（最長一段 5 字＝60px，不裁字）；
     姓名格倍數調小時按格高等比收字號（62×倍數÷5.2，留 4% 餘量），永不超出格高。 */
  .demo-root.orient-port .cell-rel .rel2 > span { font-size: 14px !important; }
  .cell-rel .rel2 > span { font-size: min(12px, calc(62px * var(--name-size, 1) / 5.2)); }
  /* 横版：纸高 210mm、上下 @page 边距各 12mm → 可用约 186mm；
     封面/谱页外壳固定 192mm（含双边框 padding，border-box）留余量防溢出 */
  .demo-root.orient-land .sheet { height: calc(190mm / var(--dieji-scale, 1)); }
  .demo-root.orient-land .cover { height: calc(192mm / var(--dieji-scale, 1)); }
  /* 牌記页与横版其它页同高 190mm：屏幕专用规则把 .colo-page 钉在 --paper-h(752px≈199mm)，
     乘打印 zoom 1.4643 后 291mm 超出可印区 287mm → 底部被挤到第二页（实测多 1 页）。 */
  .demo-root.orient-land .colo-page { height: calc(190mm / var(--dieji-scale, 1)); }
  /* 封面内框填满外壳（不另固定高度，避免与 cover padding 叠加溢出），横排居中 */
  .demo-root.orient-land .cover-inner {
    height: 100%;
    min-height: 0;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: calc(12mm / var(--dieji-scale, 1));
    padding: calc(6mm / var(--dieji-scale, 1)) calc(24mm / var(--dieji-scale, 1));
  }
  .demo-root.orient-land .cover-tanghao {
    writing-mode: horizontal-tb;
    text-orientation: mixed;
    font-size: calc(42pt * var(--cover-size, 1) / var(--dieji-scale, 1));
    letter-spacing: calc(14px * var(--cover-size, 1) / var(--dieji-scale, 1));
    margin: 0;
    padding-top: 0;
  }
  /* 横版：堂號在上、家族名在下（上下排，不并排）；家族名 132px（≈99pt，较 107px 再 +23%），堂號 56px（≈42pt，较 40px +40%） */
  .demo-root.orient-land .cover-head { flex-direction: column-reverse; gap: calc(8mm / var(--dieji-scale, 1)); }
  .demo-root.orient-land .cover-title { margin: 0; }
  .demo-root.orient-land .cover-title .vtitle {
    writing-mode: horizontal-tb;
    text-orientation: mixed;
    font-size: calc(var(--title-fs-land-print, 107px) * var(--cover-size, 1) / var(--dieji-scale, 1));
    letter-spacing: calc(16px * var(--cover-size, 1) / var(--dieji-scale, 1));
  }
  .demo-root.orient-land .cover-meta {
    margin: 0;
    flex-direction: column;
    gap: calc(4mm / var(--dieji-scale, 1));
    align-items: center;
  }
  .demo-root.orient-land .cover-meta .meta-line span {
    writing-mode: horizontal-tb;
    text-orientation: mixed;
    font-size: calc(22pt * var(--cover-size, 1) / var(--dieji-scale, 1));
    letter-spacing: calc(4px * var(--cover-size, 1) / var(--dieji-scale, 1));
  }
  .demo-root.orient-land .book { min-width: 0 !important; overflow: visible !important; overflow-x: hidden !important; }
  /* 横版封面/谱头：强制限制最大宽度，禁止横向溢出 */
  .demo-root.orient-land .cover,
  .demo-root.orient-land .preface,
  .demo-root.orient-land .sheet {
    width: 100% !important;
    max-width: 100% !important;
    overflow: hidden !important;
  }
  /* 横版封面内框：确保文字不溢出；谱头内框 min-height 归零（覆盖屏幕预览 min-height）、height:100% 填满外壳，避免内细框超出外粗框 */
  .demo-root.orient-land .cover-inner,
  .demo-root.orient-land .preface-inner {
    width: 100% !important;
    max-width: 100% !important;
    overflow: hidden !important;
  }
  .demo-root.orient-land .preface-inner {
    min-height: 0 !important;
    height: 100% !important;
  }
  /* 横版封面/谱头竖排文字 */
  .demo-root.orient-land .preface-title,
  .demo-root.orient-land .preface-cat,
  .demo-root.orient-land .preface-meta,
  .demo-root.orient-land .cover-tanghao,
  .demo-root.orient-land .cover-title,
  .demo-root.orient-land .cover-meta {
    max-width: 100% !important;
    overflow: hidden !important;
  }
  /* 横版谱头/字辈长篇页用 192mm（cover 已在上固定；preface 同理） */
  .demo-root.orient-land .preface { height: calc(190mm / var(--dieji-scale, 1)); overflow: hidden; }
  /* 字辈页：满页框高，内容垂直居中（不是顶部小截、也不是半页） */
  .demo-root.orient-land .preface.genword { height: calc(190mm / var(--dieji-scale, 1)); min-height: 0; overflow: hidden; }
  .demo-root.orient-land .preface.genword .preface-inner {
    height: 100%;
    min-height: 0;
    overflow: hidden;
    box-sizing: border-box;
    padding: calc(10mm / var(--dieji-scale, 1));
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: calc(5mm / var(--dieji-scale, 1));
  }
  /* 横版纸宽：16 个字辈一行排完（竖版 8 列两行），高度减半不拆页 */
  .demo-root.orient-land .genword-chars {
    grid-template-columns: repeat(8, 1fr);
    gap: 4px;
    width: 100%;
    max-width: calc(240mm / var(--dieji-scale, 1));
    margin: 0 auto;
  }
  .demo-root.orient-land .genword-char { font-size: calc(14px * var(--body-size, 1)); aspect-ratio: 1 / 1; padding: 2px 0; box-sizing: border-box; }
  .demo-root.orient-land .genword-poem { font-size: calc(14px * var(--body-size, 1)); letter-spacing: calc(2px * var(--body-size, 1)); margin: 6mm 0 4mm; white-space: nowrap; }
  .demo-root.orient-land .genword-branch { font-size: calc(15px * var(--body-size, 1)); margin-bottom: 3mm; }
  .demo-root.orient-land .preface-cat { font-size: calc(16px * var(--body-size, 1) / var(--dieji-scale, 1)); letter-spacing: calc(6px / var(--dieji-scale, 1)); margin-bottom: 2mm; }
  .demo-root.orient-land .preface-title { font-size: calc(30px * var(--body-size, 1) / var(--dieji-scale, 1)); letter-spacing: calc(8px / var(--dieji-scale, 1)); margin-bottom: calc(6px / var(--dieji-scale, 1)); }
  .demo-root.orient-land .preface-meta { font-size: calc(13px * var(--body-size, 1) / var(--dieji-scale, 1)); margin-bottom: calc(8px / var(--dieji-scale, 1)); }
  /* 字辈页上方空白吸收：标题/诗/格子紧凑，余下空白不撑破 */
  .demo-root.orient-land .preface.genword .preface-inner > * { flex: 0 0 auto; }
  .demo-root.orient-land .vgrid { height: calc(142mm / var(--dieji-scale, 1)); overflow: hidden !important; }
  /* 横版长篇：标题固定、正文竖格自适应剩余高度（有标题不挤下去、无标题占满） */
  .demo-root.orient-land .preface-long .preface-inner {
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
  }
  /* 横版正文长页：上下边距累计各缩（→8.192mm），省出空间给正文竖格增高；凡例保持原样 */
  .demo-root.orient-land .preface-long:not(.rule-long) .preface-inner {
    padding-top: calc(8.192mm / var(--dieji-scale, 1));
    padding-bottom: calc(8.192mm / var(--dieji-scale, 1));
  }
  .demo-root.orient-land .preface-long .preface-cat,
  .demo-root.orient-land .preface-long .preface-title,
  .demo-root.orient-land .preface-long .preface-meta {
    flex: 0 0 auto;
  }
  .demo-root.orient-land .preface-long .vgrid {
    flex: 1 1 auto;
    height: auto;
    min-height: 0;
    margin-top: calc(4mm / var(--dieji-scale, 1));
    overflow: hidden !important;
  }
  .demo-root.orient-land .preface-long:not(.rule-long) .vgrid {
    margin-top: calc(2.56mm / var(--dieji-scale, 1));
  }
  /* 横版竖排文字竖行：keep-all 防止 vertical-rl 下 break-word 无效 */
  .demo-root.orient-land .vcol {
    word-break: keep-all !important;
    overflow: hidden !important;
  }
  /* 成员页：姓名格放宽到能装 3~4 字竖排名（60 太窄会撑高成参差不齐），
     内容固定裁切，所有列等高对齐 */
  .demo-root.orient-land .cell-rel { flex-basis: calc(52px * var(--name-size, 1)); font-size: calc(11px * var(--body-size, 1)); overflow: hidden; word-break: keep-all; }
  /* 橫版打印格高只有 52px → 字號 10px（5 字＝50px，不裁字）；倍數調小時按格高等比收。 */
  .demo-root.orient-land .cell-rel .rel2 > span { font-size: 12px !important; }
  .demo-root.orient-land .cell-name {
    flex-basis: calc(80px * var(--name-size, 1));
    font-size: calc(17px * var(--name-size, 1));
    overflow: hidden;
    overflow-wrap: break-word;
    align-items: center;
    justify-content: center;
  }
  /* 横版资料格：keep-all 防止竖排长文本溢出 */
  .demo-root.orient-land .cell-detail {
    overflow: hidden !important;
  }
}

/* ===== 名人传记页新版式 ==========================================================
   竖版：页面下部＝传记正文竖格；上部＝「资料带」（左：资料 / 右：头像）。
   横版：左侧＝传记正文竖格；右侧＝「头像+资料」竖列（上：头像 / 下：资料）。
   本段放在本 <style> 最末（打印媒体块之后）：与既有 .preface-long 规则特异度相同时后来者胜，
   屏幕与打印一并生效；只有「正文竖格高度」在屏幕下显式给定高（与 JS splitBioCols 的容量同一公式），
   打印侧不设固定高度、沿用 .preface-long 的 flex 撑满剩余高度（可用高度只会更宽裕，
   「1 / 2」旗标仍排在页内，绝不因固定高度被挤出页外）。
   横带高 --bio-head-h 与右列宽都由 JS 侧常量算出（bioHeadH / BIO_SIDE_W / BIO_SIDE_GAP），
   保证「算出来的每页容量」与「真实渲染高度、宽度」严格一致。 ============================== */
.bio-head {
  flex: 0 0 auto;
  display: flex;
  align-items: flex-start;
  gap: calc(18px / var(--dieji-scale, 1));
  box-sizing: border-box;
  overflow: hidden;
}
.bio-info { flex: 1 1 auto; min-width: 0; }
.bio-info .preface-cat,
.bio-info .preface-title,
.bio-info .preface-meta { text-align: left; }
/* 行高与 JS bioHeadH() 的估算一致（类别 20px×1.2、姓名 30px×1.2、资料 13px×1.5） */
.bio-info .preface-cat { line-height: 1.2; margin-bottom: calc(8px / var(--dieji-scale, 1)); }
.bio-info .preface-title { line-height: 1.2; margin-bottom: calc(6px / var(--dieji-scale, 1)); }
.bio-info .preface-meta { line-height: 1.5; margin-bottom: 0; }
/* 传记资料区整体 +2：姓名30→32、生卒/籍贯等13→15；字/号已16 */
.bio-info .preface-title { font-size: calc(32px * var(--body-size, 1) / var(--dieji-scale, 1)); line-height: 1.2; }
.bio-info .preface-meta { font-size: calc(15px * var(--body-size, 1) / var(--dieji-scale, 1)); line-height: 1.5; }
/* 头像：有图用图（同源相对路径 /uploads/… 可直接加载），无图用姓名首字占位方块 */
.bio-portrait { flex: 0 0 auto; }
.bio-portrait img,
.bio-portrait .ph {
  display: block;
  /* 尺寸由 JS 常量（BIO_PORTRAIT_W/H）经 --bio-portrait-* 给出：容量算法与真实渲染同一套数；
     打印全局有 img{height:auto!important} → 这里必须 !important 压住 */
  width: calc(var(--bio-portrait-w, 120px) / var(--dieji-scale, 1)) !important;
  height: calc(var(--bio-portrait-h, 150px) / var(--dieji-scale, 1)) !important;
  object-fit: cover;
  box-sizing: border-box;
  border: 1px solid #1a1a1a;
  background: #f2ece0;
}
.bio-portrait .ph {
  display: flex;
  align-items: center;
  justify-content: center;
  color: #a3927c;
  font-size: calc(48px * var(--body-size, 1) / var(--dieji-scale, 1));
}
/* 竖版：上部横带定高（高度＝JS bioHeadH()，只多不少 → 资料/头像绝不被裁） */
.demo-root.orient-port .bio-long .bio-head {
  flex-direction: row;                                        /* 左：资料　右：头像 */
  height: calc(var(--bio-head-h, 176px) / var(--dieji-scale, 1));
  padding: calc(12px / var(--dieji-scale, 1)) 0;               /* 上下内边距 ＝ JS BIO_HEAD_PAD */
  border-bottom: calc(1px / var(--dieji-scale, 1)) solid #1a1a1a;
  margin-bottom: calc(var(--bio-head-gap, 12px) / var(--dieji-scale, 1));   /* ＝ JS BIO_HEAD_GAP */
}
.demo-root.orient-port .bio-long .vgrid { margin-top: 0; }
/* 横版：右列「上：头像 / 下：资料」，正文竖格在左 */
.demo-root.orient-land .preface-long.bio-long .preface-inner {
  display: grid;
  grid-template-columns: 1fr calc(var(--bio-side-w, 210px) / var(--dieji-scale, 1));   /* 右列宽 ＝ JS BIO_SIDE_W */
  grid-template-rows: 1fr auto;
  column-gap: calc(var(--bio-side-gap, 18px) / var(--dieji-scale, 1));                 /* ＝ JS BIO_SIDE_GAP */
  align-items: stretch;
}
.demo-root.orient-land .bio-long .bio-head {
  grid-column: 2;
  grid-row: 1;
  flex-direction: column;      /* 头像在上、资料在下（DOM 里资料在前，用 order:-1 把头像提到最前） */
  align-items: center;
  align-self: start;
  gap: calc(12px / var(--dieji-scale, 1));
  width: calc(var(--bio-side-w, 210px) / var(--dieji-scale, 1));
  height: auto;
  padding: 0;
  border-bottom: 0;
  margin-bottom: 0;
}
.demo-root.orient-land .bio-long .bio-portrait { order: -1; }
.demo-root.orient-land .bio-long .bio-info { width: 100%; }
.demo-root.orient-land .bio-long .bio-info .preface-cat,
.demo-root.orient-land .bio-long .bio-info .preface-title,
.demo-root.orient-land .bio-long .bio-info .preface-meta { text-align: center; }
.demo-root.orient-land .bio-long .bio-portrait img,
.demo-root.orient-land .bio-long .bio-portrait .ph {
  width: calc(var(--bio-land-w, 150px) / var(--dieji-scale, 1)) !important;
  height: calc(var(--bio-land-h, 190px) / var(--dieji-scale, 1)) !important;
}
.demo-root.orient-land .bio-long .vgrid {
  grid-column: 1;
  grid-row: 1;
  margin-top: 0;
}
.demo-root.orient-land .bio-long .page-flag { grid-column: 1 / -1; grid-row: 2; }
/* 续页（第 2 页起）：不带头像/资料带 → 横版不再让出右列，整宽就是正文竖格（列数也由 JS 按整宽实测给）。
   竖版本来就没有侧列，续页只是「竖格更高」，无需额外规则。 */
.demo-root.orient-land .preface-long.bio-long .preface-inner.bio-cont {
  display: flex;
  flex-direction: column;
}
.demo-root.orient-land .preface-long.bio-long .preface-inner.bio-cont .vgrid,
.demo-root.orient-land .preface-long.bio-long .preface-inner.bio-cont .page-flag {
  grid-column: auto;
  grid-row: auto;
  margin-top: 0;
}
/* 正文竖格高度：屏幕预览与列印都用同一个公式定高（与 JS splitBioCols 的容量同一公式）。
   注意必须同时作用于列印：本档前面那条「.demo-root.orient-port .preface-long:not(.rule-long) .vgrid
   { height: 900px }」并未包在 @media print 里，列印时同样生效（特异度与下面相同、靠后来者覆盖）；
   若这里只写 @media screen，「横带 176 + 竖格 900 + 旗标」会超出内框被裁掉 → 旗标「1 / 2」印不出来。
   续页无横带，竖版竖格直接是 900（竖格 900 + 旗标 ≈ 920 ≤ 内框 982），列印同样不裁。 */
.demo-root.orient-port .preface-long.bio-long .vgrid {
  flex: 0 0 auto;
  height: calc(var(--bio-grid-h, 700px) / var(--dieji-scale, 1));
}
.demo-root.orient-land .preface-long.bio-long .vgrid {
  flex: 0 0 auto;
  height: calc(var(--bio-grid-h, 600px) / var(--dieji-scale, 1));
}

/* ===== 第九輪②：紙寬固定後，內容層自行約束——任何內容都不得撐寬／溢出書頁 =====
   紙張寬已由 --paper-w 寫死（不再 max-content），這裡再給「可能超寬的內容」加護欄：
   ① 長串（長英文／URL／長數字／表格式文本）：overflow-wrap:anywhere + word-break:break-word
      → 在紙寬內折行，而不是橫向撐出去（舊行為：整頁被 width:max-content 撐寬）。
   ② 圖片：max-width:100%（頭像另有固定寬高 !important；圖片頁格子內 img 已是 100%）。
   ③ 文字塊：min-width:0，讓 flex/grid 子項可被壓縮（不會把父框頂寬）。
   只作用於螢幕（列印已有 width:100% !important + overflow:hidden + 全域 img{max-width:100%}，
   不需要也不應改動列印口徑）。 */
@media screen {
  .demo-root .cover-inner,
  .demo-root .preface-inner,
  .demo-root .bio-head,
  .demo-root .bio-info { min-width: 0; }
  .demo-root .preface-title,
  .demo-root .preface-cat,
  .demo-root .preface-meta,
  .demo-root .bio-info,
  .demo-root .page-flag,
  .demo-root .toc-row,
  .demo-root .g-cell figcaption {
    min-width: 0;
    overflow-wrap: anywhere;
    word-break: break-word;
  }
  .demo-root .cover-inner img,
  .demo-root .preface-inner img,
  .demo-root .sheet img { max-width: 100%; }
}




/* 牒记式大事记：照搬族规/祖训整页竖排多列 (.vgrid/.vcol)，字号/行距/列宽完全同族规 */
.ep-inner { --sg-fs: 14px; padding: calc(24px / var(--dieji-scale, 1)) calc(30px / var(--dieji-scale, 1)); }
.ep-page-title { flex: 0 0 auto; text-align: center; font-size: 20px; font-weight: 700; letter-spacing: 6px; margin: 0 0 8px; color: #1a1a1a; }
.ep-vgrid { height: calc(100% - 36px); column-gap: 0; }
.ep-vgrid .vcol { font-size: calc(16px * var(--body-size, 1)); line-height: 2.0; }
.ep-vgrid .vcol.ep-col-title { color: #8a3b2a; font-weight: 700; }
.ep-vgrid .ep-year { font-size: 1.1em; letter-spacing: 1px; }
</style>

<!-- 全局（非 scoped）：牒记版面固定按 A3 制作。打印到 A4/A5 等由 PDF/打印对话框「适应纸张」自动等比缩小，不变形。 -->
<style>
@page dieji-sheet {
  size: A3 portrait;
  margin: 5mm 5mm 5mm 5mm;
}
@page dieji-sheet-land {
  size: A3 landscape;
  margin: 5mm 5mm 5mm 5mm;
}
@media print {
  /* ===== 全局盒模型 + 禁止横向溢出 ===== */
  * { box-sizing: border-box !important; }

  html, body {
    width: 100%;
    height: 100%;
    overflow-x: hidden !important;
  }

  /* ===== 竖排文字核心修复：writing-mode vertical-rl =====
    大字号下，文字不能靠 word-break，必须限制容器宽度 */
  .vcol, .cell-name .vtext {
    max-width: 100% !important;
    overflow: hidden !important;
    word-break: keep-all !important;
    writing-mode: vertical-rl !important;
  }
  .cell-detail, .vtext {
    max-width: 100% !important;
    overflow: hidden !important;
    writing-mode: vertical-rl !important;
  }

  /* 封面标题/字辈派语：横排（竖版封面靠自身 scoped 竖排，横版封面/字辈派语始终横排），只限宽不强制竖排 */
  .cover-title .vtitle, .cover-meta .meta-line span,
  .genword-poem, .genword-chars {
    max-width: 100% !important;
    overflow: hidden !important;
    word-break: keep-all !important;
  }

  /* ===== 根容器：全部限制最大宽度，禁止横向撑开 ===== */
  /* min-width:0 必要：屏幕样式里 .cover/.preface/.sheet 有 min-width: calc(880px/scale)（手機「適應屏幕」用），
     min-width 會壓過 width/max-width:100%，不清掉的話列印時紙張被撐到 880px（232mm）> A3 內容區，右緣溢出。 */
  .demo-root, .book, .cover, .preface, .sheet,
  .cover-inner, .preface-inner, .vgrid, .vcol,
  .person-col, .cell-detail, .cell-name, .cell-rel {
    min-width: 0 !important;
    max-width: 100% !important;
    width: 100% !important;
    overflow: hidden !important;
    box-sizing: border-box !important;
  }

  /* ===== 蝶记布局：flex列，禁止大字号撑开列 ===== */
  .sheet {
    width: 100% !important;
    /* 防内容溢出兜底：用 clip + 10mm 余量（与装订线那处同款写法），
       超出 10mm 的内容仍被裁；不支持 overflow:clip 的浏览器退回 hidden。 */
    overflow: clip !important;
    overflow-clip-margin: 10mm;
  }
  .person-col {
    flex: 1 1 0% !important;
    min-width: 0 !important;
    max-width: 100% !important;
    overflow: hidden !important;
  }

  /* ===== 横版A3专属：内容按 A3 横版内容区宽 396mm ÷ √2 ≈ 280mm 排版，zoom×√2≈396mm 精确填满 A3 内容区（否则按 A4 宽 273mm zoom 后只 386mm，右侧留 10mm 空白） ===== */
  .demo-root.orient-land .book {
    width: calc(280mm / var(--dieji-scale, 1)) !important;
    max-width: calc(280mm / var(--dieji-scale, 1)) !important;
    min-width: 0 !important;
    overflow-x: hidden !important;
  }
  /* 竖版同理：A3 直版内容区宽 273mm ÷ √2 ≈ 193mm */
  .demo-root.orient-port .book {
    width: calc(193mm / var(--dieji-scale, 1)) !important;
    max-width: calc(193mm / var(--dieji-scale, 1)) !important;
    min-width: 0 !important;
    overflow-x: hidden !important;
  }
  /* 装订开启：左侧 @page 多预留 5mm 装订线，内容区窄 5mm，内框宽度同步缩 5mm/√2≈3.5mm（横 280→276.5、竖 193→189.5），否则右缘溢出页面 */
  /* 装订开启：@page 左侧多预留 17mm 但只缩宽不改高（用户：只缩小宽度、底部别多空白）——
     缩放仍用 1.4643 填满页高，把内容区宽度缩到 17mm 边距装得下即可：
     竖 275/1.4643=187.8 → 取 187.5mm（× 1.4643 = 274.6 ≤ 275）；横 398/1.4643=271.8 → 取 271.5mm（× 1.4643 = 397.6 ≤ 398） */
  .demo-root.bind-on.orient-land .book {
    width: calc(271.5mm / var(--dieji-scale, 1)) !important;
    max-width: calc(271.5mm / var(--dieji-scale, 1)) !important;
  }
  .demo-root.bind-on.orient-port .book {
    width: calc(187.5mm / var(--dieji-scale, 1)) !important;
    max-width: calc(187.5mm / var(--dieji-scale, 1)) !important;
  }

  .demo-root.orient-land .cover,
  .demo-root.orient-land .preface,
  .demo-root.orient-land .sheet {
    width: 100% !important;
    max-width: 100% !important;
    overflow: hidden !important;
  }

  .demo-root.orient-land .cover-inner,
  .demo-root.orient-land .preface-inner {
    width: 100% !important;
    max-width: 100% !important;
    overflow: hidden !important;
  }

  /* 大字号标题：强制限制宽度 */
  .demo-root.orient-land .preface-title,
  .demo-root.orient-land .preface-cat,
  .demo-root.orient-land .preface-meta,
  .demo-root.orient-land .cover-tanghao,
  .demo-root.orient-land .cover-title,
  .demo-root.orient-land .cover-meta {
    max-width: 100% !important;
    overflow: hidden !important;
  }

  .demo-root.orient-land .vgrid {
    width: 100% !important;
    overflow: hidden !important;
  }

  /* 传记内容上提10px：打印顶部空白比预览大 */
  .bio-inner { padding-top: 8px !important; }
  /* 谱头/大事记：只去掉 sg-inner 上 padding（24px≈6.35mm），保留 .preface 的纸张内边距 14px */
  .demo-root .epc-page .sg-inner,
  .demo-root .epc-page.sg-page .sg-inner { padding: 0 !important; }
  .epc-page .epc, .epc-page .epc-grid { margin-top: 0 !important; }
  .preface { box-sizing: border-box !important; }
  .preface-inner { height: 100% !important; min-height: 0 !important; box-sizing: border-box !important; }

  /* 图片媒体 */
  img, svg {
    max-width: 100% !important;
    height: auto !important;
  }

  /* A3 zoom 缩放：内容按 A4 尺寸排版，打印时 zoom 把内容等比放大到 A3 内容区（字号/框线/间距一起放大）。
     四周留白 5mm 后内容区变大为：竖 287×410mm、横 410×287mm（开装订左边 17mm：竖 275×410、横 398×287）。
     zoom 取「几何能填满」的最大值：
       竖版 280mm 页高 × z = 410 → z = 410/280 = 1.4643（宽 193×1.4643=282.6 ≤ 287，右侧余 4.4mm）
       横版 280mm 页宽 × z = 410 → z = 410/280 = 1.4643（高 190×1.4643=278.2 ≤ 287，下侧余 8.8mm）
       开装订：内容区变窄 → 竖 z = 275/193 = 1.4249、横 z = 398/280 = 1.4214（否则右缘会被裁） */
  /* A：布局级缩放（zoom 参与布局，不是 transform），内框随整页一起缩放 */
  .book { zoom: 1.4422 !important; transform: none !important; margin-left: auto !important; margin-right: auto !important; }
  /* 竖版页高 280×1.4643 = 410mm 正好等于 5mm 边距的内容区高（零余量）→ 打印机实际非打印区略大时底部被裁（用户 22:00）。
     竖版单独缩到 280×1.4536 = 407mm（比原来矮 3mm；先 1.4571=408mm 仍差 1mm，用户 22:11 再加 1mm）；宽度 193×1.4536=280.5 ≤ 287 仍安全；横版不动。 */
  /* 内容比内容区略窄时居中摆放（否则余量全堆在右边） */
  .book { margin-left: auto !important; margin-right: auto !important; }
  /* 横版页高 190mm × 1.4643 = 278.2mm，比内容区高 287mm 矮 8.8mm → 每页下移一半（4.3mm物理 = 2.94mm÷scale）使上下边距均匀 */
  .demo-root.orient-land .cover,
  .demo-root.orient-land .preface,
  .demo-root.orient-land .sheet,
  .demo-root.orient-land .colo-page {
    margin-top: calc(2.94mm / var(--dieji-scale, 1)) !important;
  }
  /* ===== 末尾空白页修复：不再给页块挂命名页 page: dieji-sheet* =====
     命名页（@page dieji-sheet / -land）与默认 @page 混用时，最后一个命名页块（牌記）结束后，
     浏览器会补一个「默认页盒」收尾：实测该末尾空白页的 MediaBox = A4（841.92×594.96pt，横），
     而其余页均为 A3（竖 841.92×1191.12 / 横 1191.12×841.92）→ 正是命名页组收尾产生的空默认页。
     纸型/边距改由 setup 里 applyDefaultPrintPage() 注入的默认 @page 统一给出（A3 + 5mm，装订左 17mm），
     与原来的命名页取值逐项相同；全文档只有一种页上下文 → PDF 页数 == 逻辑页数。 */
  .cover, .preface, .sheet { box-sizing: border-box; }
  .cover-inner, .preface-inner { box-sizing: border-box; width: 100%; max-width: 100%; }
  /* 隐藏工具栏 */
  .no-print { display: none !important; }
}

@media print {
  /* 祠宇志/茔墓志：一逻辑页=一打印页；内容超长时裁切，绝不拆成第二页 */
  .preface.sg-page { break-before: page !important; page-break-before: always !important;
    break-after: page !important; page-break-after: always !important;
    break-inside: avoid !important; page-break-inside: avoid !important; overflow: hidden !important; }
  .preface.sg-page .preface-inner { overflow: hidden !important; }
}
</style>
