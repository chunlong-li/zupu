<template>
  <div class="chuizhu" ref="rootRef" :class="{ 'bind-on': bind }">
    <div class="toolbar">
      <span class="serif title">蘇氏垂珠體</span>
      <span class="lbl">修次</span>
      <el-select v-model="xiuci" size="small" class="no-print" style="width:130px" filterable allow-create default-first-option @change="saveXiuci">
        <el-option v-for="o in XIUCI_OPTS" :key="o" :label="o" :value="o" />
      </el-select>
      <el-input v-model="xiuci" size="small" class="no-print" style="width:110px" placeholder="自定义修次" @change="saveXiuci" />

      <span class="hint">A3橫版 · 封面 → 內封 → 目錄 → 名人傳記 → 譜頭文化 → 五服圖 → 五世一圖（版心居中、行內由右至左）→ 行傳 → 牌記</span>
      <span class="hint">裝訂</span>
      <el-switch v-model="bind" size="small" />
      <span class="hint">邊距</span>
      <el-input-number v-model="printMargin" :min="0" :max="60" :step="1" size="small" controls-position="right"
                       style="width: 120px" />
      <span class="hint">mm（0 = 滿版）</span>
      <span class="hint">縮放</span>
      <el-button size="small" @click="zoomOut">－</el-button>
      <el-button size="small" @click="zoomReset" :title="explicitManual ? '點擊回到自動適合寬度（自動值不超過記住的 ' + rememberedPct + '%）' : '目前為自動適合寬度（記住上限 ' + rememberedPct + '%）'">{{ zoomLabel }}</el-button>
      <el-button size="small" @click="zoomIn">＋</el-button>
      <span v-if="clampedByFit" class="hint" style="color:#b08840">目前螢幕寬度只夠顯示 {{ effPct }}%（已記住 {{ rememberedPct }}%）</span>
      <el-button size="small" @click="galleryOpen = true">圖片展示（{{ galleryImageTotal }}）</el-button>
      <el-button type="primary" size="small" @click="exportPDF">打印 / 導出 PDF</el-button>
      <el-button size="small" @click="loadAll" :loading="loading">刷新</el-button>
    </div>

    <div class="scroll" ref="scrollRef" v-loading="loading">
      <div class="pages">
        <!-- 離屏量測區：用真實 A3 版式量成員寬度、小傳寬度 -->
        <div class="measure" v-if="rawPages.length && !sheets.length">
          <div v-for="(pg, pi) in rawPages" :key="'m' + pi" class="sheet">
            <div class="page">
              <div class="board"><div class="bin">
                <div class="content">
                  <div v-for="g in pg.generations" :key="g.gen" class="gen-row">
                    <div class="part left"><div class="members"></div></div>
                    <div class="gap"></div>
                    <div class="part right">
                      <div class="gen-side">
                        <div class="gen-tag">第{{ g.gen }}世</div>
                      </div>
                      <div class="members">
                        <div v-for="m in g.members" :key="m.id" class="memb" :data-mid="m.id">
                          <div class="m-name">{{ m.name }}</div>
                          <div class="m-detail">
                            <div v-for="(d, di) in fields(m)" :key="di" class="d-col">{{ d }}</div>
                            <div v-if="m.children || m.no_heir || m.lost_contact" class="d-col son-block">
                              <div v-if="m.lost_contact" class="sg-label">{{ lostTxt(m, false) }}</div>
                              <template v-else-if="m.no_heir && !(m.sons && m.sons.length)">
                                <div class="sg-label">無嗣</div>
                              </template>
                              <template v-else>
                              <div class="sg-label">{{ m.children }}</div>
                              <div class="sg-names" v-if="m.sons && m.sons.length">
                                <div v-for="s in m.sons" :key="'s' + s.id" class="son-cell">
                                  <div class="son-name">{{ s.name }}</div>
                                  <div class="son-page" v-if="bioPageMap[s.id]">{{ bioPageMap[s.id] }}</div>
                                </div>
                              </div>
                              </template>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
                <div class="ear"></div>
              </div></div>
            </div>
          </div>
        </div>

        <!-- 封面（A3 橫版） -->
        <div class="sheet" v-if="sheets.length">
          <div class="page cover" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin cv-bin">
              <!-- 封面家族名/堂号：横排；堂号在上、家族名在下；两者水平中心 = 内容中心；
                   整组垂直居中略偏上（属主 2026-10-19 定稿）。正文仍竖排，只改这两个元素。 -->
              <div class="cv-head">
                <div class="cv-sub" v-if="tanghao">{{ tanghao }}</div>
                <div class="cv-title" :style="{ fontSize: coverTitleFs + 'px', letterSpacing: coverTitleLs + 'px' }">{{ coverTitle }}</div>
              </div>
              <div class="cv-foot">公元{{ yearG }}年　{{ ganZhi }}　{{ xiuci }}</div>
              <div class="cv-seal" v-if="surname">{{ surname }}</div>
            </div></div>
          </div>
        </div>

        <!-- 內封（書名頁）：封面之後、目錄之前。
             傳統叫「內封/書名頁」：一頁只有竖排書名大字 + 雙線邊框 + 右下角「X堂藏板」。
             取舍（三本谱书一致）：與封面、目錄同屬「卷首」，**不進 sheets 模型、不佔正文頁號**，
             也不列進目錄 —— 實物目錄（參考圖集 目录-01）只列到卷以下各節，內封屬書衣一部；
             這樣做同時保證正文頁碼不因加頁而整體平移。 -->
        <div class="inner-sheet" v-if="sheets.length" data-page-no="inner">
          <div class="page inner-page" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin it-host">
              <InnerTitlePage :title="coverTitle" :tanghao="tanghao" :year-label="'歲次' + ganZhi" :xiuci="xiuci"
                              :unit="16" :page-h="1123" />
            </div></div>
          </div>
        </div>

        <!-- 目錄（A3 橫版）：排在封面之後、名人傳記之前。
             刻意**不用** .sheet、也不 push 進 sheets 模型 —— 它與封面一樣在正文之外，不佔正文頁號；
             但版式與正文頁同一套：右側書耳「目錄」、中間版心「族譜全卷 + 目錄」，只把版心頁碼位留空。
             條目只排左側一列（約半幅），右側留空給日後使用。 -->
        <div class="toc-sheet" v-if="sheets.length" data-page-no="toc">
          <div class="page" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin">
              <div class="content">
                <div class="toc-doc">
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
                <!-- 版心（中）：與正文頁同款；目錄不參與編號 → 頁碼位留空 -->
                <div class="spine">
                  <div class="spine-title">{{ bookTitle }}</div>
                  <div class="spine-meta">目錄</div>
                  <div class="spine-page"></div>
                </div>
              </div>
              <!-- 書耳（右）：與正文頁同款，章節名換成「目錄」 -->
              <div class="ear">
                <div class="ear-branch">目錄</div>
              </div>
            </div></div>
          </div>
        </div>

        <!-- 正式渲染：分頁後的版面（垂珠體頁 + 行傳頁，頁碼連續） -->
        <div v-for="(sh, si) in sheets" :key="'s' + si" class="sheet" :data-page="sh.pageNo" :data-page-no="sh.pageNo"
               :class="{ 'page-flash': highlightPageNo === sh.pageNo,
                 'sg-sheet': sh.kind === 'shrine' || sh.kind === 'grave',
                 'sg-plate-sheet': sh.kind === 'shrine' && sh.mode === 'image' }">
          <!-- 垂珠體頁 -->
          <div class="page" v-if="sh.kind === 'chart'" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin">
              <div class="content">
                <div v-for="r in sh.rows" :key="r.gen" class="gen-row">
                  <!-- 左半：同一行排不下的成員（緊貼版心續排） -->
                  <div class="part left">
                    <div class="members">
                      <div v-for="m in r.left" :key="'l' + m.id" class="memb">
                        <div class="m-name">{{ m.name }}</div>
                        <div class="m-detail">
                          <div v-for="(d, di) in fields(m)" :key="di" class="d-col">{{ d }}</div>
                          <div v-if="m.children || m.no_heir || m.lost_contact" class="d-col son-block">
                              <div v-if="m.lost_contact" class="sg-label">{{ lostTxt(m, false) }}</div>
                              <template v-else-if="m.no_heir && !(m.sons && m.sons.length)">
                                <div class="sg-label">無嗣</div>
                              </template>
                              <template v-else>
                              <div class="sg-label">{{ m.children }}</div>
                              <div class="sg-names" v-if="m.sons && m.sons.length">
                                <div v-for="s in m.sons" :key="'s' + s.id" class="son-cell">
                                  <div class="son-name">{{ s.name }}</div>
                                  <div class="son-page" v-if="bioPageMap[s.id]" @click.stop="jumpToBio(bioPageMap[s.id])">{{ bioPageMap[s.id] }}</div>
                                </div>
                              </div>
                              </template>
                            </div>
                        </div>
                      </div>
                    </div>
                  </div>
                  <div class="gap"></div>
                  <!-- 右半：成員由右往左排，行右端世次框 -->
                  <div class="part right">
                    <div class="gen-side">
                      <div class="gen-tag">第{{ r.gen }}世</div>
                    </div>
                    <div class="members">
                      <div v-for="m in r.right" :key="'r' + m.id" class="memb">
                        <div class="m-name">{{ m.name }}</div>
                        <div class="m-detail">
                          <div v-for="(d, di) in fields(m)" :key="di" class="d-col">{{ d }}</div>
                          <div v-if="m.children || m.no_heir || m.lost_contact" class="d-col son-block">
                              <div v-if="m.lost_contact" class="sg-label">{{ lostTxt(m, false) }}</div>
                              <template v-else-if="m.no_heir && !(m.sons && m.sons.length)">
                                <div class="sg-label">無嗣</div>
                              </template>
                              <template v-else>
                              <div class="sg-label">{{ m.children }}</div>
                              <div class="sg-names" v-if="m.sons && m.sons.length">
                                <div v-for="s in m.sons" :key="'s' + s.id" class="son-cell">
                                  <div class="son-name">{{ s.name }}</div>
                                  <div class="son-page" v-if="bioPageMap[s.id]" @click.stop="jumpToBio(bioPageMap[s.id])">{{ bioPageMap[s.id] }}</div>
                                </div>
                              </div>
                              </template>
                            </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- 版心（中）：譜名 + 房派世次 + 頁碼 -->
                <div class="spine">
                  <div class="spine-title">{{ bookTitle }}</div>
                  <div class="spine-meta">{{ sh.headName }}公下派　第{{ sh.startGen }}世至第{{ sh.endGen }}世</div>
                  <div class="spine-page">{{ sh.pageNo }}</div>
                </div>
              </div>

              <!-- 書耳（右）：房派；有分页时标「當前第X頁」 -->
              <div class="ear">
                <div class="ear-branch">{{ sh.headName }}公下派</div>
                <div class="ear-page" v-if="sh.subNo">當前第{{ cnNum(sh.subNo) }}頁</div>
              </div>
            </div></div>
          </div>

          <!-- 行傳頁：牒記式（每人一豎格：世次 / 名 / 小傳，按世代長幼由右而左連排） -->
          <div class="page" v-else-if="sh.kind === 'bio'" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin">
              <div class="content">
                <div class="bio-sheet">
                  <!-- 左半：讀序在後 -->
                  <div class="bio-half left">
                    <div v-for="p in sh.left" :key="'bl' + p.key" class="p-col" :class="{ 'p-cont': p.cont }"
                         :data-cid="p.cid" :data-chunk="p.chunk" :data-kind="p.cont ? 'cont' : 'person'">
                      <div class="p-gen" :class="{ filled: p.gen }">{{ p.gen ? cnNum(p.gen) + '世' : '' }}</div>
                      <div class="p-rel"><span class="rel2" v-if="relParts(p.rel).f" :style="{ fontSize: relFs(p.rel) }"><span class="rel2-f">{{ relParts(p.rel).f }}</span><span class="rel2-r" v-if="relParts(p.rel).r">{{ relParts(p.rel).r }}</span></span></div>
                      <div class="p-name"><span class="p-v">{{ p.name }}</span></div>
                      <div class="p-text">{{ p.text }}<span v-if="p.src" class="p-src">{{ p.src }}</span></div>
                    </div>
                    <div v-for="b in sh.leftBlank" :key="'blb' + b" class="p-col blank">
                      <div class="p-gen"></div><div class="p-rel"></div><div class="p-name"></div><div class="p-text"></div>
                    </div>
                  </div>
                  <div class="bio-gap"></div>
                  <!-- 右半：讀序在前 -->
                  <div class="bio-half right">
                    <div v-for="p in sh.right" :key="'br' + p.key" class="p-col" :class="{ 'p-cont': p.cont }"
                         :data-cid="p.cid" :data-chunk="p.chunk" :data-kind="p.cont ? 'cont' : 'person'">
                      <div class="p-gen" :class="{ filled: p.gen }">{{ p.gen ? cnNum(p.gen) + '世' : '' }}</div>
                      <div class="p-rel"><span class="rel2" v-if="relParts(p.rel).f" :style="{ fontSize: relFs(p.rel) }"><span class="rel2-f">{{ relParts(p.rel).f }}</span><span class="rel2-r" v-if="relParts(p.rel).r">{{ relParts(p.rel).r }}</span></span></div>
                      <div class="p-name"><span class="p-v">{{ p.name }}</span></div>
                      <div class="p-text">{{ p.text }}<span v-if="p.src" class="p-src">{{ p.src }}</span></div>
                    </div>
                    <div v-for="b in sh.rightBlank" :key="'brb' + b" class="p-col blank">
                      <div class="p-gen"></div><div class="p-rel"></div><div class="p-name"></div><div class="p-text"></div>
                    </div>
                  </div>
                </div>

                <div class="spine">
                  <div class="spine-title">{{ bookTitle }}</div>
                  <div class="spine-meta">行傳</div>
                  <div class="spine-page">{{ sh.pageNo }}</div>
                </div>
              </div>
              <div class="ear">
                <div class="ear-branch">{{ sh.headName }}公下派</div>
              </div>
            </div></div>
          </div>

          <!-- 譜頭文化頁（譜序／凡例／祖訓／族規）：整列為一豎格，由右而左＝【類型列】【標題列】【正文列…】；
               列內自上而下、滿一列換下一列；列內不分格（無任何橫向分隔線），列與列之間以豎線分隔。
               整頁豎格排滿就另起一頁（書耳標 n／N、版心標「譜頭文化（續）」）。 -->
          <div class="page" v-else-if="sh.kind === 'preface'" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin">
              <div class="content">
                <div class="ph-doc">
                  <div class="ph-half left">
                    <div v-for="(c, ci) in sh.left" :key="'pl' + ci"
                         class="ph-col" :class="'is-' + c.kind">
                      <div class="ph-blank" v-if="c.kind === 'blank'"></div>
                      <div class="ph-type" v-else-if="c.kind === 'type'">{{ c.text }}</div>
                      <div class="ph-title" v-else-if="c.kind === 'title'">{{ c.text }}</div>
                      <div class="ph-text" v-else>{{ c.text }}</div>
                    </div>
                  </div>
                  <div class="ph-gap"></div>
                  <div class="ph-half right">
                    <div v-for="(c, ci) in sh.right" :key="'pr' + ci"
                         class="ph-col" :class="'is-' + c.kind">
                      <div class="ph-blank" v-if="c.kind === 'blank'"></div>
                      <div class="ph-type" v-else-if="c.kind === 'type'">{{ c.text }}</div>
                      <div class="ph-title" v-else-if="c.kind === 'title'">{{ c.text }}</div>
                      <div class="ph-text" v-else>{{ c.text }}</div>
                    </div>
                  </div>
                </div>

                <div class="spine">
                  <div class="spine-title">{{ bookTitle }}</div>
                  <div class="spine-meta">譜頭文化{{ sh.partNo > 1 ? '（續）' : '' }}</div>
                  <div class="spine-page">{{ sh.pageNo }}</div>
                </div>
              </div>
              <div class="ear">
                <div class="ear-branch">{{ (sh.cats && sh.cats.length ? sh.cats.map(c => PH_CAT_TRAD[c] || c).join('·') : '譜頭文化') }}</div>
                <div class="ear-page">{{ sh.partNo }}／{{ sh.partTotal }}</div>
              </div>
            </div></div>
          </div>

          <!-- 五服圖（服制圖）：譜頭文化之後、五世一圖之前 —— 一頁一幅矢量圖。
               位置理由：五服圖是「禮制圖」，屬譜頭文化之後的圖版（參考 图版-01），
               在實物譜裡正排在譜序/凡例之後、世系圖之前；故置於此，並進目錄、佔正文頁碼。 -->
          <div class="page" v-else-if="sh.kind === 'fivefu'" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin">
              <div class="content">
                <div class="ff-doc">
                  <div class="ff-head">
                    <div class="ff-title">五服圖</div>
                    <div class="ff-sub">{{ tanghao ? tanghao + '　' : '' }}本宗九族服制（斬衰·齊衰·大功·小功·緦麻）</div>
                  </div>
                  <div class="ff-body"><FiveFuChart layout="wide" /></div>
                </div>
                <div class="spine">
                  <div class="spine-title">{{ bookTitle }}</div>
                  <div class="spine-meta">五服圖</div>
                  <div class="spine-page">{{ sh.pageNo }}</div>
                </div>
              </div>
              <div class="ear">
                <div class="ear-branch">五服圖</div>
              </div>
            </div></div>
          </div>

          <!-- 家族大事记：五服圖之後、祠宇/五世之前 -->
          <div class="page" v-else-if="sh.kind === 'events'" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin">
              <div class="content">
                <div class="ep-doc"><EventsPage :items="sh.items" :part="sh.part" :total="sh.total" variant="chuizhu" /></div>
                <div class="spine">
                  <div class="spine-title">{{ bookTitle }}</div>
                  <div class="spine-meta">家族大事記</div>
                  <div class="spine-page">{{ sh.pageNo }}</div>
                </div>
              </div>
              <div class="ear"><div class="ear-branch">家族大事記</div></div>
            </div></div>
          </div>

          <!-- 祠宇志 / 茔墓志：五服圖之後、五世一圖/行傳之前（沿用版心/書耳/頁碼） -->
          <div class="page" v-else-if="sh.kind === 'shrine' || sh.kind === 'grave'" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin">
              <div class="content">
                <div class="sg-doc">
                  <ShrinePage v-if="sh.kind === 'shrine'" :shrine="sh.item" :mode="sh.mode" :part="sh.part || 'all'"
                             :inline-photo="sh.inlinePhoto || ''" :images="sh.images || []" :image-kind="sh.imageKind || 'photo'" layout="cols"
                             variant="chuizhu" />
                  <GravePage v-else :grave="sh.item" variant="chuizhu" />
                </div>
                <div v-if="sh.mode === 'image'" class="sg-plate-page-no">{{ sh.pageNo }}</div>
                <div class="spine">
                  <div class="spine-title">{{ bookTitle }}</div>
                  <div class="spine-meta">{{ sh.kind === 'shrine' ? '祠宇志' : '茔墓志' }}</div>
                  <div class="spine-page">{{ sh.pageNo }}</div>
                </div>
              </div>
              <div class="ear">
                <div class="ear-branch">{{ sh.kind === 'shrine' ? '祠宇志' : '茔墓志' }}</div>
              </div>
            </div></div>
          </div>

          <!-- 名人傳記頁：一篇一頁（排在圖表頁之後、行傳頁之前）；
               正文超長時自動續頁，每續頁仍是一次獨立的 pageNo，頁碼連續、後頁不漂。
               版式：只有該篇第 1 頁帶「肖像+資料」（大字「傳」與正文自其下方起排）；
               續頁（第 2 頁起）不帶肖像、不帶資料、也不重複標題 —— 純正文，兩列都自列頂起排。
               寫滿的順序不變：首列（左半）→ 右半 → 再滿才續頁 -->
          <div class="page" v-else-if="sh.kind === 'biography'" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin">
              <div class="content">
                <div class="bio-doc">
                  <!-- 左半：首頁＝肖像+資料在上、標題與正文緊接其下；續頁＝只有正文，自列頂起排 -->
                  <div class="bd-half left" :data-bid="sh.bio.id">
                    <div class="bd-side" v-if="sh.partNo === 1">
                      <div class="bd-portrait">
                        <img v-if="sh.bio.portrait" :src="sh.bio.portrait" :alt="sh.bio.name" />
                        <div v-else class="bd-ph">{{ (sh.bio.name || '傳').slice(0, 1) }}</div>
                      </div>
                      <div class="bd-name">
                        {{ sh.bio.name }}
                        <span class="bd-zihao" v-if="bioZiHao(sh.bio)">{{ bioZiHao(sh.bio) }}</span>
                      </div>
                      <div class="bd-meta" v-if="bioInfoLine(sh.bio)">{{ bioInfoLine(sh.bio) }}</div>
                      <div class="bd-summary" v-if="sh.bio.summary">{{ sh.bio.summary }}</div>
                      <div class="bd-reason" v-if="sh.bio.reason">入傳緣由：{{ sh.bio.reason }}</div>
                    </div>
                    <div class="bd-title" v-if="sh.partNo === 1">傳</div>
                    <div class="bd-text" v-if="sh.cols[0] && sh.cols[0].length"
                         :data-bid="sh.bio.id" :data-col="0" :data-part="sh.partNo" :data-cap="sh.caps[0]">
                      <div class="bd-cont" v-if="sh.partNo > 1">（續上頁）</div>
                      <p v-for="(p, pi) in sh.cols[0]" :key="'b0' + pi">{{ p }}</p>
                    </div>
                  </div>
                  <div class="bd-gap"></div>
                  <!-- 右半：滿高的一列，左半寫滿後由此接續（仍按自然段排版，續頁標「續上頁」） -->
                  <div class="bd-half right">
                    <div class="bd-text" v-if="sh.cols[1] && sh.cols[1].length"
                         :data-bid="sh.bio.id" :data-col="1" :data-part="sh.partNo" :data-cap="sh.caps[1]">
                      <p v-for="(p, pi) in sh.cols[1]" :key="'b1' + pi">{{ p }}</p>
                    </div>
                  </div>
                </div>

                <div class="spine">
                  <div class="spine-title">{{ bookTitle }}</div>
                  <div class="spine-meta">名人傳記{{ sh.partNo > 1 ? '（續）' : '' }}</div>
                  <div class="spine-page">{{ sh.pageNo }}</div>
                </div>
              </div>
              <div class="ear">
                <div class="ear-branch">名人傳記</div>
                <div class="ear-page">{{ sh.partNo }}／{{ sh.partTotal }}</div>
              </div>
            </div></div>
          </div>

          <!-- 圖片展示頁：書末板塊（行傳之後）。每頁按佈局容納 1/2/4/6/9 張，超出自動續頁。 -->
          <div class="page" v-else-if="sh.kind === 'gallery'" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin">
              <div class="content">
                <div class="gallery-doc" :class="'g' + sh.perPage">
                  <div class="gallery-grid">
                    <figure class="g-cell" v-for="(g, gi) in sh.images" :key="'gi' + sh.pageNo + '-' + gi">
                      <img :src="g.path" :alt="g.description || ''" @error="onGalleryImgError" />
                      <figcaption v-if="sh.showCaption">{{ g.description || '' }}</figcaption>
                    </figure>
                  </div>
                </div>
                <div class="spine">
                  <div class="spine-title">{{ bookTitle }}</div>
                  <div class="spine-meta">{{ sh.title }}{{ sh.partNo > 1 ? '（續）' : '' }}</div>
                  <div class="spine-page">{{ sh.pageNo }}</div>
                </div>
              </div>
              <div class="ear">
                <div class="ear-branch">{{ sh.title }}</div>
                <div class="ear-page">第 {{ sh.partNo }}／{{ sh.partTotal }} 页</div>
              </div>
            </div></div>
          </div>
        </div>

        <!-- 牌記（版權頁）：全書末頁 —— 三行小字（堂號／修次／紀年）＋「X堂藏板」。
             與封面、內封、目錄同屬「書衣之外」：不進 sheets 模型、不佔正文頁號、不列進目錄
             （實物牌記就印在末頁/內封背面，從不出現在目錄裡）。 -->
        <div class="colo-sheet" v-if="sheets.length" data-page-no="colophon">
          <div class="page colo-page" :style="{ zoom: zoomScale }">
            <div class="board"><div class="bin it-host">
              <ColophonPage :tanghao="tanghao" :xiuci="xiuci" :year-label="coloYear" :unit="26" />
            </div></div>
          </div>
        </div>
      </div>
    </div>

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
import { ref, computed, nextTick, watch, onMounted, onBeforeUnmount } from 'vue'
import { branchCtx } from '../utils/branchContext'
import api from '../api'
import { readSuzhuZoomPct, saveSuzhuZoomPct } from '../utils/viewZoomPrefs'
import { ArrowUp } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { lostContactText } from '../utils/memberFlags'
import { relHeadPartsOfText } from '../utils/lineageRel'
import { fetchAllSources, groupSourcesByMember, sourceFoot } from '../utils/memberSources'
import ShrinePage from '../components/book/ShrinePage.vue'
import GravePage from '../components/book/GravePage.vue'
import { fetchShrineGrave } from '../utils/shrineGrave'
import EventsPage from '../components/book/EventsPage.vue'

// 條首小字「某公第幾子」→ 固定兩段：{ f: 父名＋公, r: 行第 }（缓存，模板每格只解析一次）
const relPartCache = new Map()
function relParts(rel) {
  const k = String(rel == null ? '' : rel)
  let v = relPartCache.get(k)
  if (!v) { v = relHeadPartsOfText(k); relPartCache.set(k, v) }
  return v
}

// ===== 關係小字字號：按「兩段裡較長那一段的字數」分檔，取格內放得下的最大整數字號 =====
// 格內幾何（實測，一律不改）：`.p-rel` flex 74px → clientHeight 72px、列寬 57px；
// 兩段各成一豎列（line-height:1 → 每字縱向步長＝字號、每列橫向寬度＝字號，兩列間 gap 2px）。
// 屬主定稿：關係字「再小一號 + 加字距」——1~3 字 18px、4 字 15px、5 字 12px、6+ 字 9px；
// 字距 letter-spacing 2px（豎排＝字與字縱向更疏）、兩段間 gap 6px；仍明確小於名諱（26px 加粗）與資料（24px）。
// 同字數一律同字號（不按整個字串長度縮）；單段（配／始祖）也按該段字數取檔。
const REL_FS_BY_CHARS = { 1: 18, 2: 18, 3: 18, 4: 15, 5: 12 }
function relFs(rel) {
  const p = relParts(rel)
  const n = Math.max(Array.from(p.f).length, Array.from(p.r).length)
  return (REL_FS_BY_CHARS[n] || 9) + 'px'
}

// ===== 行傳「成員資料」：固定格 + 統一固定字號 + 寫滿換格（流式接寫）=====
// 格＝一列（`.p-col`）：外框 59.3 × 1084.7px（半頁 12 列 × 2 ＝ 每頁 24 格；版心／書耳／頁碼一律不動）。
// 格內四段固定：世次 34 ＋ 關係 74 ＋ 名諱 90 ＋ 正文（剩餘 886.7，扣上下 padding 12 ＝ 可用高 877）。
// 統一資料字號 24px、line-height 1.25（豎排：每字縱向步長 ≈ 1.0 字號、每列橫向節距 ＝ 1.25 字號）。
// 實測 24px 每字縱向步長 ≈ 24.1~24.3px → 每格 36 字（36×24.3＝875 ≤ 877；37 字會折到第二列 → 橫向溢出，故取 36）。
// 一人文字超過 36 字 → 後面的字直接接寫進下一格；續格不重複表頭（世次/關係/名諱留空），
// 正文照樣自表頭下同一高度起排；**不加任何「接上／續」標記**（屬主口徑）。
// 一人寫完 → 下一人另起一格；短文不滿格就留白（與參考圖「少一」那格一致）。
// 世次帶（黑條 `X世`）只出現在每人第一格（chunk 0）。
const BIO_TEXT_FS = 24        // 全頁統一資料字號（不按內容縮放；屬主定稿 24px）
const BIO_TEXT_LH = 1.25      // 豎排列節距係數
let BIO_TEXT_CAP = 36         // 每格字數（渲染後 calibrateBioCells() 以真實 DOM 實測校正）
const BIO_TEXT_ADV_RATIO = 1.012   // 縱向步長／字號（實測 1.007~1.012，取保守值）
const BIO_CLOSE_PUNCT = '。，、；：！？）〕」』】》”’'   // 換格時不讓下一格以收尾標點開頭
const BIO_ASCII_RUN = /[0-9A-Za-z\-.:]/                     // 西文/數字串（如 2026-08-02）不切斷
function sliceBioText(s, cap) {
  const chars = Array.from(String(s == null ? '' : s))
  if (!chars.length) return ['']
  const out = []
  let i = 0
  while (i < chars.length) {
    let end = Math.min(i + cap, chars.length)
    // 下一格首字若是收尾標點，就把它留在本格末尾（避免「。開頭」）
    while (end < chars.length && end > i + 1 && BIO_CLOSE_PUNCT.indexOf(chars[end]) >= 0) end--
    // 不把西文/數字串切兩半（2026-08-02 這種）：整串留到下一格（本格短幾個字，不裁字）
    while (end < chars.length && end > i + 1 && BIO_ASCII_RUN.test(chars[end]) && BIO_ASCII_RUN.test(chars[end - 1])) end--
    out.push(chars.slice(i, end).join(''))
    i = end
  }
  return out
}

// ===== 資料來源小字腳注：與正文同一條流，接在該人正文末尾 =====
// 腳注字號 = 正文 24px 的 0.625（15px），豎排每字縱向步長按字號比例縮小，
// 因此折算成「正文格容量單位」= 15/24 ≈ 0.625（取 0.7 保守，寧可少排不撐破格）：
// 最後一格有剩餘容量就接寫；寫滿就接寫到下一格（續格不重複表頭），絕不為了腳注加大格子 / 縮正文。
const SRC_TEXT_FS = 15
const SRC_UNIT = 0.7
function sliceBioRich(main, foot, cap) {
  const cells = sliceBioText(main, cap).map((t) => ({ text: t, src: '' }))
  const chars = Array.from(String(foot == null ? '' : foot))
  if (!chars.length) return cells
  let i = 0
  const last = cells[cells.length - 1]
  const room = Math.floor((cap - Array.from(last.text).length) / SRC_UNIT)
  if (room > 0) {
    let end = Math.min(i + room, chars.length)
    while (end < chars.length && end > i + 1 && BIO_ASCII_RUN.test(chars[end]) && BIO_ASCII_RUN.test(chars[end - 1])) end--
    last.src = chars.slice(i, end).join('')
    i = end
  }
  const perCell = Math.max(1, Math.floor(cap / SRC_UNIT))
  while (i < chars.length) {
    let end = Math.min(i + perCell, chars.length)
    while (end < chars.length && end > i + 1 && BIO_ASCII_RUN.test(chars[end]) && BIO_ASCII_RUN.test(chars[end - 1])) end--
    cells.push({ text: '', src: chars.slice(i, end).join('') })
    i = end
  }
  return cells
}
import InnerTitlePage from '../components/book/InnerTitlePage.vue'
import ColophonPage from '../components/book/ColophonPage.vue'
import FiveFuChart from '../components/book/FiveFuChart.vue'

// 迁居失联标注：生/殁/葬 缺啥补啥 + 失考，再缀子嗣失考
function lostTxt(m, showKnown = true) {
  return lostContactText(
    {
      birth: m.birth_str || '',
      death: m.death_str || '',
      burial: String(m.burial || '').replace(/^葬/, '')
    },
    !!(m.sons && m.sons.length),
    showKnown
  )
}

const A3_W = 1587 // A3 横版 420mm ≈ 1587px（96dpi）
const MEMBER_GAP = 10
const BIO_UNITS = 12   // 行傳每個半頁的列單元數（每人按小傳長短佔 1~N 個單元）
const BIO_REL_H = 74   // 行傳「世次」格高
const BIO_NAME_H = 90  // 行傳「姓名」格高

const loading = ref(false)
const rawPages = ref([]) // 接口返回的原始頁
const sourceMap = ref({})   // memberId → 資料來源腳注（'據：…；…'；無來源則無此鍵）
const sheets = ref([])   // 分頁後的版面（封面之後：垂珠體頁 + 行傳頁）
const bioPageMap = ref({}) // memberId → 該人行傳所在頁碼（圖上諸子頁碼 / 點擊跳轉）
// 祠宇志/茔墓志：每个祠宇一页正文；有图再加一页图版；每座茔墓一页。无数据 → 空数组（不生成页/目录行）。
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

// 家族大事记：按年份升序（接口已按 year,id 排序），按内容量自然分页；无数据不生成。
const EVENT_PAGE_CHARS = 1800
const eventPages = computed(() => {
  const list = events.value || []
  const pages = []
  let cur = [], len = 0
  for (const e of list) {
    const n = String(e.year || '').length + String(e.title || '').length + String(e.content || '').length + 24
    if (cur.length && len + n > EVENT_PAGE_CHARS) { pages.push(cur); cur = []; len = 0 }
    cur.push(e); len += n
  }
  if (cur.length) pages.push(cur)
  return pages.map((items, i) => ({ items, part: i + 1, total: pages.length }))
})
const biographies = ref([]) // 名人傳記（/api/biographies）
const shrines = ref([])     // 祠宇志（含子项）
const graves = ref([])      // 茔墓志
const events = ref([])      // 家族大事记
const surname = ref('')
const bind = ref(false)   // 裝訂：true = 每頁左側預留裝訂邊（留白 + 裝訂線／活頁孔欄），屏幕預覽 + 打印都生效

// 打印邊距（mm，可自訂）：加大留白避免打印機裁切；0 = 滿版（舖滿整張，最外缘可能被裁）
const printMargin = ref(5)

// 依邊距算出縮放：內容需放進 (420-2m)×(297-2m) 可用區（A3 橫版 420×297mm）
function zoomForMargin(m) {
  if (!m || !isFinite(m)) return 1
  const s = Math.min((420 - 2 * m) / 420, (297 - 2 * m) / 297)
  return Math.max(0.5, Math.floor(s * 1000) / 1000 - 0.001)
}

// @page 不支援 CSS 變數 → 用動態 <style> 注入邊距與縮放（只影響打印，屏幕預覽不變）
function applyPrintStyle() {
  const m = Number(printMargin.value) || 0
  const z = zoomForMargin(m)
  let el = document.getElementById('chuizhu-print-page')
  if (!el) {
    el = document.createElement('style')
    el.id = 'chuizhu-print-page'
    document.head.appendChild(el)
  }
  el.textContent = '@media print {' +
    '@page { size: A3 landscape; margin: ' + m + 'mm; }' +
    '.chuizhu .page { zoom: ' + z + ' !important; margin-left: auto !important; margin-right: auto !important; }' +
    '}'
}
watch(printMargin, applyPrintStyle)
// 記住的百分比（localStorage，預設 50）：作為自動（適合寬度）時的上限；使用者 ＋/－ 後寫回
const rememberedPct = ref(readSuzhuZoomPct())
// 本次工作階段是否由使用者主動調過（手動檔）：
//   · 主動調過 → 依使用者設定值顯示（同屏可超出適合寬度，屬主已確認）；
//   · 只是「沿用上次記住的值」→ 仍受適合寬度壓制（跨屏時自動縮小，例如 80% 在手機上回到 20%）。
const explicitManual = ref(false)
const fitPct = computed(() => Math.min(100, Math.max(20, Math.round(fitScale.value * 100))))
const autoPct = computed(() => Math.min(fitPct.value, rememberedPct.value))
const effPct = computed(() => (explicitManual.value ? rememberedPct.value : autoPct.value))
const zoomScale = computed(() => effPct.value / 100)
// 按鈕顯示使用者期望的百分比；實際渲染值另有提示
const zoomLabel = computed(() => effPct.value + '%')
// 手動值被「螢幕只夠這麼寬」壓回時給提示（屬主預期行為，非 bug）
const clampedByFit = ref(false)
let clampTimer = null
function notifyClamped() {
  clampedByFit.value = true
  if (clampTimer) clearTimeout(clampTimer)
  clampTimer = setTimeout(() => { clampedByFit.value = false }, 4000)
}
function zoomIn() {
  const next = Math.min(400, effPct.value + 10)
  explicitManual.value = true
  rememberedPct.value = next
  saveSuzhuZoomPct(next)
  if (next > fitPct.value) notifyClamped()   // 只做提示：同屏手動放大仍生效
}
function zoomOut() {
  const next = Math.max(20, effPct.value - 10)
  explicitManual.value = true
  rememberedPct.value = next
  saveSuzhuZoomPct(next)
}
// 回到「自動」：清掉本次手動檔，實際 = min(適合寬度, 記住的值)
function zoomReset() { explicitManual.value = false }
const tanghao = ref('')
const xiuci = ref('重修')   // 修次：封面「公元…岁次…X」用（服务器设置）
// 封面書名自適應：横排封面，4 字用 180px（属主要求 170~180 取上限），更长按 180×4/n 收缩；
// 封面内框宽约 1551px，4 字 720px、7 字约 721px（按比例缩字号后总宽基本恒定）→ 不超框。
const coverTitleFs = computed(() => {
  const n = Math.max(4, Array.from(coverTitle.value || '').length)
  return Math.max(56, Math.round(180 * 4 / n))
})
// 横排封面：不再需要竖排的 40px 字距；0 字距使元素框中心与字形中心重合（家族名严格居中）
const coverTitleLs = computed(() => 0)
const rootRef = ref(null)
const scrollRef = ref(null)
const fitScale = ref(1)
let ro = null

const familyTitle = computed(() => {
  const b = (branchCtx.branches || []).find(x => String(x.id) === String(branchCtx.current))
  if (b && b.id) return (b.name || '') + '支谱'
  if (surname.value) return surname.value + '氏家谱'
  return '族谱'
})
const bookTitle = computed(() => familyTitle.value)
const coverTitle = computed(() => familyTitle.value)

const GAN = ['甲', '乙', '丙', '丁', '戊', '己', '庚', '辛', '壬', '癸']
const ZHI = ['子', '丑', '寅', '卯', '辰', '巳', '午', '未', '申', '酉', '戌', '亥']
const yearG = new Date().getFullYear()
const ganZhi = GAN[(yearG - 4) % 10] + ZHI[(yearG - 4) % 12]

// 牌記紀年：歲次干支（內封左下另給「歲次+修次」兩竖列）
const coloYear = computed(() => '歲次' + ganZhi)

// 行右端功名框：取该行首位（最右）成员的功名
function rowRank(g) {
  const first = g.members && g.members[0]
  return first ? first.rank : ''
}

// 成員詳情：拆成若干字段，每字段一豎列
function fields(m) {
  const a = []
  if (m.zihao) a.push(m.zihao)
  if (m.birth_death_short || m.birth_death) a.push(m.birth_death_short || m.birth_death)
  if (m.spouse) a.push(m.spouse)
  if (m.burial) a.push(m.burial)
  return a
}

// 行傳成員格正文：字號。生卒。葬地。生平（子女另起一列）
function bioText(m) {
  const a = []
  if (m.zihao) a.push(m.zihao)
  // 遷居外地失聯：生殁葬「缺啥補啥 + 失考」（確知項不重複列）；其餘照常
  if (m.lost_contact) a.push(lostTxt(m))
  else if (m.birth_death) a.push(m.birth_death)
  if (m.burial) a.push(m.burial)
  if (m.bio) a.push(m.bio)
  // 無嗣：名下無子者，行傳正文末注「無嗣」
  if (m.no_heir && !(m.sons && m.sons.length)) a.push('無嗣')
  return a.join('。')
}

// 行傳「生几子」列（單獨一列，上面三格空白，最下一格）：生子N　諸子名
function childText(m) {
  const sons = m.sons || []
  if (!sons.length) return ''
  return '生子' + cnNum(sons.length) + '　' + sons.map((s) => s.name).join('　')
}

// 行傳配偶格正文：生卒。生平
function spouseText(s) {
  const a = []
  if (s.birth_death) a.push(s.birth_death)
  if (s.bio) a.push(s.bio)
  return a.join('。')
}

// 中文數字（書耳分頁 / 行傳世次）
function cnNum(n) {
  const d = ['', '一', '二', '三', '四', '五', '六', '七', '八', '九', '十']
  if (n <= 10) return d[n]
  if (n < 20) return '十' + d[n - 10]
  return d[Math.floor(n / 10)] + '十' + (n % 10 ? d[n % 10] : '')
}

// ===== 名人傳記：一篇一頁（正文超長自動續頁 —— 寧可多一頁，也絕不裁字） =====
// 只收錄「公開展示」的傳記；接口已按「世代 → 排行 → 生年 → id」排好序，保持原序（前端不再排序）
const bookBios = computed(() =>
  (biographies.value || []).filter((b) => b.public === 1 || b.public === '1' || b.public == null)
)

// ===== 目錄（不佔正文頁號）=====
// 條目與頁碼完全由 sheets 模型推算，所以「目錄上的頁碼」與「書裡實際的頁碼」必然一致。
// 每行只標該內容所在的第一頁；封面與目錄本身不參與編號（它們不在 sheets 裡）。
// ===== 譜頭文化（譜序／凡例／祖訓／族規）=====
// 版式：整列為一「格」（豎格），由右而左依次為【類型列】【標題列】【正文列…】，列內自上而下、滿列換列。
const PH_CAT_ORDER = { 谱序: 0, 凡例: 1, 祖训: 2, 族规: 3 }
const PH_CAT_TRAD = { 谱序: '譜序', 凡例: '凡例', 祖训: '祖訓', 族规: '族規' }   // 類型列用繁體，與本書一致
const PH_COL_W = 68           // 每個豎格（列）寬：實測容得下 3 條豎排字，裝得下 60 字的正文列（不裁字）
const PH_HALF_W = 620         // 半幅可用寬（實測同 .bd-half：50% − 版心半寬 42px 再扣內距）
const PH_COLS_PER_HALF = Math.max(2, Math.floor(PH_HALF_W / PH_COL_W))   // 每個半幅（版心一側）可容納的豎格數
const PH_CHARS_PER_COL = 48   // 每列字數（21px/行高30px、letter-spacing 0 時 Range 實測單行上限 48 字／列；取 48−餘量 1 = 47 字/列）
const PH_CAP_SAFETY_CHARS = 1
const articles = ref([])
const prefaceStat = ref({ pages: 0, arts: {}, catFrom: {}, catTo: {} })
const articleList = computed(() => {
  const list = (articles.value || []).filter((a) => a && (a.category in PH_CAT_ORDER))
  return list.slice().sort((a, b) => (PH_CAT_ORDER[a.category] - PH_CAT_ORDER[b.category]) || ((a.id || 0) - (b.id || 0)))
})

const TOC_FOOT = ''

const tocEntries = computed(() => {
  const rows = []
  const list = sheets.value || []
  const group = (kind) => list.filter((s) => s.kind === kind)
  const fmt = (n) => '第 ' + n + ' 页'

  // 名人傳記（一傳一頁，超長者已拆成多頁）
  const bios = group('biography')
  if (bios.length) {
    const names = bookBios.value.map((b) => b.name).filter(Boolean)
    let note = bookBios.value.length + ' 篇'
    if (bios.length > bookBios.value.length) note += '·共 ' + bios.length + ' 页'
    if (names.length && names.length <= 3) note += '·' + names.join('、')
    rows.push({ name: '名人傳記', note, page: fmt(bios[0].pageNo), pageNo: bios[0].pageNo })
  }

  // 圖片展示（多條目）：**緊接名人傳記之後、譜頭文化之前**（與書內頁序一致；每個條目一行）
  for (const gs of entrySummaries(galleryEntries.value)) {
    const first = list.find((x) => x.kind === 'gallery' && x.entryIndex === gs.index && x.partNo === 1)
    if (!first) continue
    const notes = [gs.count + ' 張']
    if (gs.pages > 1) notes.push('共 ' + gs.pages + ' 页')
    rows.push({ name: gs.title, note: notes.join(' · '), page: fmt(first.pageNo), pageNo: first.pageNo })
  }

  // 譜頭文化（譜序／凡例／祖訓／族規）：**每個分類單獨一行**（各自起頁，行內只標該分類起始頁；
  // 頁碼由 sheets 模型裡的實際 pageNo 推算，不手寫）
  const catPages = {}          // 分類 → { from, to }（書內真實頁碼）
  group('preface').forEach((sh) => {
    const cats = sh.cats && sh.cats.length ? sh.cats : []
    cats.forEach((c) => {
      if (!catPages[c]) catPages[c] = { from: sh.pageNo, to: sh.pageNo }
      else catPages[c].to = sh.pageNo
    })
  })
  Object.keys(catPages).forEach((c) => {
    const r = catPages[c]
    const cnt = r.to - r.from + 1
    rows.push({ name: PH_CAT_TRAD[c] || c, note: cnt > 1 ? '共 ' + cnt + ' 页' : '', page: fmt(r.from), pageNo: r.from })
  })

  // 五服圖（譜頭文化之後、五世一圖之前）：固定 1 頁，進目錄
  const ffPages = group('fivefu')
  if (ffPages.length) {
    rows.push({ name: '五服圖', note: '本宗九族服制', page: fmt(ffPages[0].pageNo), pageNo: ffPages[0].pageNo })
  }

  // 家族大事记（五服圖之後、祠宇/五世之前）
  const evPages = group('events')
  if (evPages.length) {
    rows.push({ name: '家族大事記', note: '共 ' + evPages.length + ' 页', page: fmt(evPages[0].pageNo), pageNo: evPages[0].pageNo })
  }

  // 祠宇志 / 茔墓志（五服圖之後、五世一圖之前）
  const shrinePages = group('shrine')
  if (shrinePages.length) {
    rows.push({ name: '祠宇志', note: '共 ' + shrinePages.length + ' 页', page: fmt(shrinePages[0].pageNo), pageNo: shrinePages[0].pageNo })
  }
  const gravePages = group('grave')
  if (gravePages.length) {
    rows.push({ name: '茔墓志', note: '共 ' + gravePages.length + ' 页', page: fmt(gravePages[0].pageNo), pageNo: gravePages[0].pageNo })
  }

  // 五世一圖：≤5 張逐張列出（名稱＝房派＋世代範圍），否則合併成一行
  const charts = group('chart')
  if (charts.length) {
    if (charts.length <= 5) {
      charts.forEach((sh) => {
        const range = sh.startGen === sh.endGen
          ? '第' + sh.startGen + '世' : '第' + sh.startGen + '世至第' + sh.endGen + '世'
        rows.push({
          name: (sh.headName || '') + '公下派　' + range,
          note: sh.subNo ? '當前第' + cnNum(sh.subNo) + '頁' : '',   // 同一組分多頁時標明第幾張
          page: fmt(sh.pageNo), pageNo: sh.pageNo,
        })
      })
    } else {
      rows.push({ name: '五世一圖', note: '共 ' + charts.length + ' 页', page: fmt(charts[0].pageNo), pageNo: charts[0].pageNo })
    }
  }

  // 行傳（牒記式：每人一豎格）
  const zc = group('bio')
  if (zc.length) rows.push({ name: '行傳', note: '共 ' + zc.length + ' 页', page: fmt(zc[0].pageNo), pageNo: zc[0].pageNo })

  // 保險：日後若新增其它頁型，也照樣列出來（不至於目錄漏項）
  const known = { biography: 1, preface: 1, fivefu: 1, chart: 1, bio: 1, gallery: 1, shrine: 1, grave: 1, events: 1 }   // gallery 已在上面逐條目出行，勿再落進「其它」
  const others = list.filter((s) => !known[s.kind])
  if (others.length) rows.push({ name: '其它', note: '共 ' + others.length + ' 页', page: fmt(others[0].pageNo), pageNo: others[0].pageNo })

  return rows
})

// 每頁容量：優先「渲染後實測」——第 1 頁直接量真實 DOM：整列高 −「肖像+資料+傳標題」實高 − 正文列上下
// 外距 = 首列真實可用高；滿高列＝整列高 − 上下外距。再換算行數：
//   列容量(行) = floor(該列實測可用高 ÷ 行高 36.1px) − BIO_CAP_SAFETY
// 摘要/入傳緣由長短自動算進去，起點與容量都不靠猜。量不到時才退回保守常量，
// 最後再由 verifyBioFit 渲染後兜底校正。
// 兩套容量：首頁 = [首列(肖像+資料+標題之下), 滿高列]；續頁 = 沒有肖像/資料/標題，兩列都是滿高。
// 註：.page 用的是整體 CSS zoom，紙張與字號同步縮放，所以每頁容量不隨 zoomScale / 打印邊距
//     變化；只有「裝訂」（.bind-on 左側多留 14mm）會真正擠窄版心，故單獨換算每行字數。
const BIO_COLS_PER_PAGE = 2      // 每頁兩列：左半（先排）→ 右半（接續）
const BIO_LINES_PER_COL = 24     // 滿高列行數上限（保守常量；量測失敗時的兜底，也是首頁滿高列的上限）
const BIO_FIRST_COL_LESS = 16    // 首頁首列比滿高列少掉的行數（保守常量；量測失敗時的兜底）
const BIO_CAP_SAFETY = 2         // 實測行數再留 2 行餘量（標點換行浮動）
const BIO_MEASURE_EPS = 1        // 實測高度一律先扣 1px：clientHeight 只到整數，同一版面在不同時刻可能 ±1px，
                                 // 若剛好卡在行數邊界（例如 1046/36.1=28.97）會多算一行 → 寧可少排一行
const BIO_MIN_COL_LINES = 3      // 一列至少要裝得下 3 行才用來放字；否則整列跳過（避免硬塞壓字）
const BIO_LINE_H = 36.1          // 正文行高 19px × 1.9（量不到 computed line-height 時的兜底）
const BIO_CHARS_PER_LINE = 33    // 單行字數（保守值；實測半幅淨寬 660px ÷ 19px ≈ 34 字）
const BIO_PARA_GAP_LINES = 0.35  // 段間距（10px）折算行數
const BIO_VERIFY_MAX = 3         // 渲染後實測校正的最多輪數

// 裝訂邊會擠窄版心 → 每行少約 2 字
function bioCharsPerLine() {
  return bind.value ? BIO_CHARS_PER_LINE - 2 : BIO_CHARS_PER_LINE
}

// 每篇傳記的兩套容量（行數）：{ first: [首列, 滿高列], cont: [列1, 列2] }
// 優先序：verifyBioFit 實測溢出後的收緊值 > 首輪渲染後的實測值 > 保守常量。
const bioMeasured = {}    // id → { first, cont }（calibrateBioCaps 實測填入）
const bioCapOverride = {} // id → { first, cont }（渲染後實測溢出再收緊，優先於量測值）
let bioVerifyRound = 0
function bioCaps(id, cont) {
  const key = cont ? 'cont' : 'first'
  const o = bioCapOverride[id]
  if (o && o[key]) return o[key]
  const m = bioMeasured[id]
  if (m && m[key]) return m[key]
  // 量測失敗時的保守兜底：首頁首列較短；續頁沒有肖像+資料+標題 → 兩列都按滿高
  const full = BIO_LINES_PER_COL
  return cont ? [full, full] : [Math.max(2, full - BIO_FIRST_COL_LESS), full]
}

// 名諱下的字號（字X　號Y）
function bioZiHao(b) {
  return [b.zi ? '字' + b.zi : '', b.hao ? '號' + b.hao : ''].filter(Boolean).join('　')
}

// 傳記小字履歷行：性別 / 生卒 / 籍貫現居 / 身份職銜 / 職業 / 榮譽（只列有值的）
function bioInfoLine(b) {
  const a = []
  if (b.gender) a.push(b.gender)
  const bt = b.birth_gregorian || b.birth_lunar || b.birth_era
  const dt = b.death_gregorian || b.death_lunar || b.death_era
  if (bt || dt) a.push((bt ? '生 ' + bt : '生年未詳') + '　' + (dt ? '殁 ' + dt : '卒年未詳'))
  if (b.native_place) a.push('籍貫 ' + b.native_place)
  if (b.current_place) a.push('現居 ' + b.current_place)
  if (b.title) a.push(b.title)
  if (b.occupation) a.push('業 ' + b.occupation)
  if (b.honors) a.push('榮譽 ' + b.honors)
  return a.join('　·　')
}

// 按句讀斷句（不用 lookbehind，兼容性更穩）
function bioSentences(s) {
  const out = []
  let buf = ''
  for (const ch of s) {
    buf += ch
    if ('。！？；'.indexOf(ch) >= 0) { out.push(buf); buf = '' }
  }
  if (buf) out.push(buf)
  return out
}

// 一篇傳記正文 → 一頁頁 × 一列列的「段落數組」（每頁 BIO_COLS_PER_PAGE 列）。
// pageCaps = [首頁各列容量, 續頁各列容量]（續頁所有頁相同；只有一組時全部頁都用它）。
// 首頁要讓出「肖像+資料+傳標題」的高度，續頁沒有那塊 → 兩列都是滿高，故容量分兩套。
// 只按自然段整段搬列、搬頁，絕不切斷自然段；只有單段（乃至單句）本身就超過一列時，
// 才先按句讀、再按字數兜底切開，保證每一塊一定放得進任一列。
function splitBioCols(content, pageCaps, cpl) {
  const capsOf = (pi) => pageCaps[Math.min(pi, pageCaps.length - 1)]
  // 可用列：容量太小（< BIO_MIN_COL_LINES，例如肖像+資料高到首列只剩一兩行）的列整列跳過，
  // 字寧可從下一列起排，也絕不硬塞（硬塞就會壓字/溢出）。容量 0 同樣視為不可用。
  const usableOf = (caps) => {
    const u = caps.map((c, i) => (c >= BIO_MIN_COL_LINES ? i : -1)).filter((i) => i >= 0)
    return u.length ? u : [0]                                     // 兜底：保證有輸出
  }
  // 單塊必須放得進「任何一頁的任何可用列」→ 取所有頁可用列容量的最小值把關
  const allUse = pageCaps.map((caps) => usableOf(caps).map((i) => caps[i]))
  const minCap = Math.max(1, Math.min.apply(null, allUse.flat()))
  const linesOf = (s) => Math.ceil((String(s).length + 2) / cpl) + BIO_PARA_GAP_LINES // +2 = 段首縮進兩字
  const raw = String(content == null ? '' : content).replace(/\r\n?/g, '\n')
    .split('\n').map((s) => s.trim()).filter(Boolean)
  const paras = raw.length ? raw : ['（正文待補充）']

  const pieces = []
  paras.forEach((p) => {
    if (linesOf(p) <= minCap) { pieces.push(p); return }
    let buf = ''
    bioSentences(p).forEach((s) => {
      if (linesOf(s) > minCap) {                 // 單句仍超長 → 按字數硬切（兜底）
        if (buf) { pieces.push(buf); buf = '' }
        const max = Math.max(1, (minCap - 1) * cpl - 2) // -2 抵消段首縮進，切出的塊必定 ≤ 一列
        for (let i = 0; i < s.length; i += max) pieces.push(s.slice(i, i + max))
        return
      }
      if (buf && linesOf(buf + s) > minCap) { pieces.push(buf); buf = '' }
      buf += s
    })
    if (buf) pieces.push(buf)
  })

  // 逐列裝箱：先填本頁首列，滿了接本頁次列；可用列都滿才換頁（頁內列流向固定：左半 → 右半）
  const pages = []
  let pi = 0                                         // 第幾頁（0 = 首頁：用首頁容量）
  let use = usableOf(capsOf(0))
  let cols = capsOf(0).map(() => [])
  let used = 0
  let ui = 0                                         // 目前在本頁的第幾個可用列
  pieces.forEach((p) => {
    const n = linesOf(p)
    const ci = use[ui]
    if (cols[ci].length && used + n > capsOf(pi)[ci]) { // 本列裝不下 → 換下一可用列；都沒有了就續頁
      used = 0                                       // 換列（或換頁）時用量歸零
      ui++
      if (ui >= use.length) {
        pages.push(cols)
        pi++
        use = usableOf(capsOf(pi))
        cols = capsOf(pi).map(() => [])
        ui = 0
      }
    }
    cols[use[ui]].push(p)
    used += n
  })
  if (cols.some((c) => c.length) || !pages.length) pages.push(cols)
  return pages.length ? pages : [[['（正文待補充）'], []]]
}

// 從 start 起，能塞進 avail 寬度的成員數（至少 1 個，避免死循環）
function take(list, start, avail) {
  let used = 0
  let k = 0
  // 字号加大后每半幅最多 4 人：再多就会把最后一个挤进书芯/被裁。
  const MAX_PER_HALF = 3
  for (let i = start; i < list.length; i++) {
    if (k >= MAX_PER_HALF) break
    const w = list[i].w + (k === 0 ? 0 : MEMBER_GAP)
    if (used + w > avail) break
    used += w
    k++
  }
  if (k === 0 && start < list.length) k = 1
  return k
}

// 行傳列（扁列）：全卷成員去重（按世代先後、同世長幼）；每人一列，其配偶（配/繼配/側室）緊接其後各一列
const bioCols = computed(() => {
  const seen = new Set()
  const gens = new Map()
  rawPages.value.forEach((pg) => {
    (pg.generations || []).forEach((g) => {
      if (!gens.has(g.gen)) gens.set(g.gen, [])
      ;(g.members || []).forEach((m) => {
        if (seen.has(m.id)) return
        seen.add(m.id)
        gens.get(g.gen).push(m)
      })
    })
  })
  const out = []
  ;[...gens.keys()].sort((a, b) => a - b).forEach((gen) => {
    gens.get(gen).forEach((m) => {
      out.push({ key: 'm' + m.id, id: m.id, gen, rel: m.rel || '', name: m.name, text: bioText(m),
                 foot: sourceFoot(sourceMap.value, m.id, '據：') })
      ;(m.spouses || []).forEach((s, i) => {
        out.push({ key: 's' + m.id + '_' + i, id: 0, gen: 0, rel: s.rel || '配', name: s.name, text: spouseText(s), foot: '' })
      })
      const ct = childText(m)
      if (ct) out.push({ key: 'c' + m.id, id: 0, gen: 0, rel: '', name: '', text: ct, foot: '' })
    })
  })
  return out
})

// 譜頭正文清洗：去掉 Markdown 引用符，行內換行轉全角空格（豎排連續排布）
function phCleanText(s) {
  return String(s == null ? '' : s).replace(/^\s*>\s?/gm, '').replace(/\s*\n\s*/g, '　').trim()
}

// 一篇譜頭 → 一串「豎格」：類型列 → 標題列 → 正文列…；正文列內自上而下，滿一列就換下一列。
function phColumnsOf(cat, title, text) {
  const out = []
  out.push({ kind: 'type', text: PH_CAT_TRAD[cat] || cat, cat })
  out.push({ kind: 'title', text: String(title || '').trim() || (PH_CAT_TRAD[cat] || cat), cat })
  const body = phCleanText(text)
  const cap = Math.max(20, PH_CHARS_PER_COL - PH_CAP_SAFETY_CHARS)   // 單行上限（格內不折行）
  const chunks = body ? String(body).match(new RegExp('.{1,' + cap + '}', 'gs')) : []
  if (!chunks || !chunks.length) out.push({ kind: 'text', text: '（本文待補）', cont: false, cat })
  else chunks.forEach((t, i) => out.push({ kind: 'text', text: t, cont: i > 0, cat }))
  return out
}

// 全部譜頭篇按「類型 → 篇序」展開成豎格序列，再按每頁豎格數切頁。
// 規則：① 絕不裁字（只按容量切列、切頁）；② 每一篇的【類型列＋標題列】必須與該篇正文同頁起排
//（不讓某篇只剩續文掉到下一頁開頭）——首頁放不下「類型+標題」就整篇另起一頁；
// ③ 一篇跨頁時，首頁一定帶「類型列 + 標題列」。
// 屬主最新口徑（方案 B）：
//   · 同一分類共用**最右一列類型列**（不再每篇各寫一次類型）；
//   · **每個分類另起一頁**（一種分類最少占一頁），該分類最後一頁沒寫滿 → 空格子照常畫出；
//   · 該分類的**每一頁**最右都保留類型列（每頁都能看出當前分類）。
function buildPrefaceSheets() {
  const pages = []            // 每頁 = [右半 9 格, 左半 9 格]
  const arts = {}
  const catFromMap = {}       // 分類 → 起始頁（供目錄逐分類列行）
  const catToMap = {}         // 分類 → 結束頁（＝該分類總頁數）
  const cats = []
  articleList.value.forEach((a) => { if (!cats.includes(a.category)) cats.push(a.category) })

  cats.forEach((cat) => {
    const startIdx = pages.length                    // 该分类第一页在 pages 里的下标
    catFromMap[cat] = startIdx + 1                   // 该分类的起始页（逐分类目錄用）
    const list = articleList.value.filter((a) => a.category === cat)
    const body = []           // 「標題列 + 正文列」序列（同一分類連續排，寫滿換頁）
    list.forEach((a, ai) => {
      const id = cat + '#' + ai
      const cols = phColumnsOf(cat, a.title, a.content)
      const rest = cols.slice(2)                     // 去掉 [類型列, 標題列] 裡的類型列
      arts[id] = { cat, title: a.title, from: null, to: null, cols: cols.length }
      body.push({ ...cols[1], cat, title: a.title, art: id })   // 標題列
      rest.forEach((c, i) => body.push({ ...c, cont: i > 0, cat, title: a.title, art: id }))
    })
    // 逐頁排：每頁 18 格，全部是「標題列 + 正文列」（類型列已改由書耳承擔）
    const usable = PH_COLS_PER_HALF * 2
    let idx = 0
    let first = true
    if (!body.length) { pages.push([[], []]); return }
    while (idx < body.length || first) {
      const take = body.slice(idx, idx + usable)
      idx += take.length
      first = false
      pages.push([take.slice(0, PH_COLS_PER_HALF), take.slice(PH_COLS_PER_HALF)])
      if (!take.length) break
    }
    Object.keys(arts).forEach((k) => { if (arts[k].cat === cat && arts[k].from == null) { arts[k].from = pages.length; arts[k].to = pages.length } })
    catToMap[cat] = pages.length - startIdx                   // 该分类总页数（书耳按「分类内 n／N」计数用）
  })

  if (!pages.length) pages.push([[], []])
  // 每頁固定畫滿 PH_COLS_PER_HALF × 2 格（空格照畫）
  const pad = (cols) => { const out = cols.slice(); while (out.length < PH_COLS_PER_HALF) out.push({ kind: 'blank', text: '', cat: '', title: '' }); return out }
  return { pages: pages.map((p2) => [pad(p2[0]), pad(p2[1])]), arts, catFromMap, catToMap }
}

// 量測 + 分頁：全書順序＝封面（模板另繪，不佔 sheets） → 名人傳記 → 譜頭文化 → 五世一圖 → 行傳。
// 圖表頁行內由右往左排，右半→左半→（排不下）換頁續同一排，上面留空
function buildSheets() {
  const root = rootRef.value
  if (!root) return
  const model = []
  let pageNo = 1

  // 名人傳記（一篇一頁）排在最前面 —— 緊跟封面之後（封面是模板裡獨立的一張，不進 model），
  // 所以直接 push 在 model 開頭即可：頁碼仍用同一個 pageNo 自增，後面的圖表頁／行傳頁自然順延。
  // 正文先填首頁左半首列、再填右半；兩列都滿 → 續頁，每續頁仍是一次獨立的 pageNo
  //（書耳標「名人傳記 n／N」、版心標「名人傳記（續）」、正文首列標「續上頁」）。
  // 容量分兩套：首頁要讓出「肖像+資料+傳標題」的高度（實測值，見 calibrateBioCaps）；
  // 續頁沒有那一塊，兩列都按滿高算 → 每頁裝得更多。
  const bioCpl = bioCharsPerLine()
  bookBios.value.forEach((b) => {
    const pageCaps = [bioCaps(b.id, false), bioCaps(b.id, true)]
    const pages = splitBioCols(b.content, pageCaps, bioCpl)
    pages.forEach((cols, i) => {
      model.push({
        kind: 'biography', pageNo: pageNo++, bio: b, cols,
        caps: pageCaps[Math.min(i, pageCaps.length - 1)],
        partNo: i + 1, partTotal: pages.length,
      })
    })
  })
  // 圖片展示（多條目）：**緊接名人傳記之後**、譜頭文化之前（與牒記式頁序一致）。空條目完全不生成頁面；條目順序即書內順序。
  for (const p of buildEntryPages(galleryEntries.value)) {
    model.push({
      kind: 'gallery', pageNo: pageNo++,
      images: p.images, perPage: p.per, showCaption: p.showCaption,
      title: p.title, entryNo: p.entryNo, entryIndex: p.entryIndex,
      partNo: p.partNo, partTotal: p.partTotal,
    })
  }
  // 譜頭文化（譜序／凡例／祖訓／族規）：整列為一豎格，按【類型列 → 標題列 → 正文列】由右而左排；
  // 放在名人傳記之後、五世一圖之前（與牒記式一致）。
  const phBuilt = buildPrefaceSheets()
  const catSeen = {}          // 分類 → 已排頁數（書耳 n／N 的 n）
  phBuilt.pages.forEach((halves, i, arr) => {
    const all = halves[0].concat(halves[1])
    // 書耳頁碼口徑（屬主確認）：**該分類內**第 n／N（不用全譜頭總序）
    const catOfPage = all.find((c) => c && c.cat) || { cat: '' }
    catSeen[catOfPage.cat] = (catSeen[catOfPage.cat] || 0) + 1
    model.push({
      kind: 'preface', pageNo: pageNo++,
      right: halves[0], left: halves[1],
      partNo: catSeen[catOfPage.cat], partTotal: phBuilt.catToMap[catOfPage.cat] || arr.length,
      cats: Array.from(new Set(all.map((c) => c.cat).filter(Boolean))),
      startCat: halves[0][0] ? halves[0][0].cat : '',
      startTitle: (all.find((c) => c.kind === 'title') || {}).text || '',
    })
  })
  prefaceStat.value = { pages: phBuilt.pages.length, arts: phBuilt.arts, catFrom: phBuilt.catFromMap, catTo: phBuilt.catToMap }

  // 五服圖（服制圖）：譜頭文化之後、五世一圖之前 —— 整頁一幅 SVG 矢量圖，不依賴成員量測，
  // 固定佔一頁（進目錄、佔正文頁碼；其後各頁頁碼自動順延）。
  model.push({ kind: 'fivefu', pageNo: pageNo++ })

  // 家族大事记（谱头文化/五服之后、祠宇/五世之前）
  for (const ep of eventPages.value) model.push({ kind: 'events', pageNo: pageNo++, items: ep.items, part: ep.part, total: ep.total })

  // 祠宇志 → 茔墓志（五服圖之後、五世一圖/行傳之前）
  for (const sp of shrinePageList.value) model.push({ ...sp, pageNo: pageNo++ })
  for (const gp of gravePageList.value) model.push({ ...gp, pageNo: pageNo++ })

  const mr = root.querySelector('.measure .part.right > .members')
  const ml = root.querySelector('.measure .part.left > .members')
  if (mr && ml) {
    // 成员字号加大后，行内容易顶到纸右缘/被裁：按实测宽度再留 12% 余量，
    // 每行少放 1~2 人，宁多翻页，不裁字、不越界。
    const CHART_FIT = 0.88
    const rightAvail = mr.clientWidth * CHART_FIT
    const leftAvail = ml.clientWidth * CHART_FIT
    const wmap = {}
    root.querySelectorAll('.measure .memb').forEach((el) => { wmap[el.dataset.mid] = el.offsetWidth })

    rawPages.value.forEach((pg) => {
      const gens = pg.generations || []
      const q = gens.map((g) => ({
        gen: g.gen,
        rank: rowRank(g),
        list: (g.members || []).map((m) => ({ m, w: wmap[m.id] || 0 })),
      }))
      // 本頁：每世先填右半，再填左半
      const rows = q.map((item) => {
        const a = take(item.list, 0, rightAvail)
        const b = take(item.list, a, leftAvail)
        const right = item.list.slice(0, a).map((x) => x.m)
        const left = item.list.slice(a, a + b).map((x) => x.m)
        item.list = item.list.slice(a + b)
        return { gen: item.gen, rank: item.rank, right, left }
      })
      model.push({ kind: 'chart', pageNo: pageNo++, headName: pg.head_name, startGen: pg.start_gen, endGen: pg.end_gen, rows })
      const gStart = model.length - 1

      // 續頁：所有溢出的世共用一頁（4/5 世必須同頁，不能一世一頁拆開）。
      // 每輪把各世剩餘成員按世序填入同一張續頁的對應行；本頁裝不下的留到下一輪再一起續。
      let guard = 0
      while (q.some((x) => x.list.length) && guard++ < 60) {
        const crows = gens.map((g) => ({ gen: g.gen, rank: rowRank(g), right: [], left: [] }))
        let filled = false
        for (const item of q) {
          if (!item.list.length) continue
          const a = take(item.list, 0, rightAvail)
          const b = take(item.list, a, leftAvail)
          if (a + b <= 0) continue
          const rr = crows.find((x) => x.gen === item.gen)
          rr.right = item.list.slice(0, a).map((x) => x.m)
          rr.left = item.list.slice(a, a + b).map((x) => x.m)
          item.list = item.list.slice(a + b)
          filled = true
        }
        if (!filled) break
        model.push({ kind: 'chart', pageNo: pageNo++, headName: pg.head_name, startGen: pg.start_gen, endGen: pg.end_gen, rows: crows })
      }

      // 這一組若分了多頁，書耳下標「當前第X頁」
      const cnt = model.length - gStart
      if (cnt > 1) {
        for (let j = gStart; j < model.length; j++) model[j].subNo = j - gStart + 1
      }
    })
  }

  // 行傳（固定格流式）：每人從一格起排，文字固定 30px；超過一格容量（28 字）就直接接寫到下一格
  //（續格不重複表頭、正文與其它格同高起排，**不加接續標記**）；一人寫完，下一人另起一格。
  // 所有格一律 1 個列單元（半頁 12 格 × 2 ＝ 每頁 24 格），始祖格也不再加寬。
  // 右半排滿 → 左半 → 換頁；不足補空格線（與原分頁口徑一致）。
  const bmap = {}
  const bioCells = []
  bioCols.value.forEach((it) => {
    const chunks = sliceBioRich(it.text, it.foot, BIO_TEXT_CAP)
    chunks.forEach((c, ci) => {
      bioCells.push({
        key: it.key + '#' + ci, cid: it.key, chunk: ci, cont: ci > 0,
        id: ci === 0 ? it.id : 0, gen: ci === 0 ? it.gen : 0,
        rel: ci === 0 ? it.rel : '', name: ci === 0 ? it.name : '',
        text: c.text, src: c.src,
      })
    })
  })
  const halves = []
  let cur = { cells: [], units: 0 }
  bioCells.forEach((p) => {
    if (cur.cells.length && cur.units + 1 > BIO_UNITS) { halves.push(cur); cur = { cells: [], units: 0 } }
    cur.cells.push(p)
    cur.units += 1
  })
  halves.push(cur)
  for (let i = 0; i < halves.length; i += 2) {
    const r = halves[i]
    const l = halves[i + 1] || { cells: [], units: 0 }
    const pno = pageNo++
    r.cells.concat(l.cells).forEach((p) => { if (p.id) bmap[p.id] = pno })
    model.push({
      kind: 'bio', pageNo: pno,
      right: r.cells, rightBlank: Math.max(BIO_UNITS - r.units, 0),
      left: l.cells, leftBlank: Math.max(BIO_UNITS - l.units, 0),
    })
  }

  // 圖片展示已於名人傳記之後排入（見上），此處不再重複。

  bioPageMap.value = bmap

  sheets.value = model
  // 渲染後：先按真實 DOM 校正容量（首頁/續頁兩套），再實測溢出兜底收緊
  nextTick(() => { calibrateBioCaps(); calibrateBioCells() })
}

// ===== 行傳固定格：渲染後以真實 DOM 校正「每格字數」 =====
// 量實際渲染的 `.p-text`：可用高 = clientHeight − 上下 padding；每字縱向步長 = 字號 × 1.012（保守）。
// 只在首輪量一次（bioCellCalibrated），之後每次重排只做溢出兜底；資料重載 / 裝訂切換時重置。
let bioCellCalibrated = false
let bioCellFixRound = 0
async function calibrateBioCells() {
  const root = rootRef.value
  if (!root) return
  if (!bioCellCalibrated) {
    const el = root.querySelector('.sheet[data-page] .p-col:not(.blank) .p-text')
    if (!el) return
    const cs = getComputedStyle(el)
    const fs = parseFloat(cs.fontSize) || BIO_TEXT_FS
    const padT = parseFloat(cs.paddingTop) || 0
    const padB = parseFloat(cs.paddingBottom) || 0
    const usable = el.clientHeight - padT - padB
    const cap = Math.max(6, Math.floor(usable / (fs * BIO_TEXT_ADV_RATIO)))
    if (typeof window !== 'undefined') window.__bioCellCal = { fs, usable, cap, ratio: BIO_TEXT_ADV_RATIO, fixRound: bioCellFixRound }
    bioCellCalibrated = true
    if (cap !== BIO_TEXT_CAP) {
      BIO_TEXT_CAP = cap
      sheets.value = []
      await nextTick()
      buildSheets()
      return
    }
  }
  verifyBioCells()
}

// 渲染後逐格複核：正文 scrollHeight/Width 超出 client ＝ 有裁字風險 → 每格字數減 1 重排（最多 3 輪）。
async function verifyBioCells() {
  const root = rootRef.value
  if (!root) return
  let over = 0
  const bad = []
  root.querySelectorAll('.sheet[data-page] .p-col:not(.blank) .p-text').forEach((el) => {
    const dy = el.scrollHeight - el.clientHeight, dx = el.scrollWidth - el.clientWidth
    if (dy > 1 || dx > 1) {
      over++
      if (bad.length < 6) bad.push({ page: (el.closest('.sheet') || {}).dataset && el.closest('.sheet').dataset.page, cid: (el.closest('.p-col') || {}).dataset && el.closest('.p-col').dataset.cid, chars: [...el.textContent].length, dy, dx, text: el.textContent.slice(0, 20) })
    }
  })
  if (!over) return
  if (typeof window !== 'undefined' && window.__bioCellCal) window.__bioCellCal.overflow = over
  if (typeof window !== 'undefined' && window.__bioCellCal) window.__bioCellCal.bad = bad
  if (bioCellFixRound >= 3 || BIO_TEXT_CAP <= 8) return
  bioCellFixRound++
  BIO_TEXT_CAP -= 1
  sheets.value = []
  await nextTick()
  buildSheets()
}

// 首輪渲染後，按「真實 DOM」實測每篇傳記第 1 頁的幾何 → 算出兩套容量（行數）：
//   整列高      = .bd-half.left 的 clientHeight（＝版心內容高，與內容多少無關）
//   滿高列可用高 = 整列高 − 正文列上下外距（首頁首列以外的所有列，含續頁兩列）
//   首頁首列可用高 = 整列高 − .bd-side 實高 − .bd-title 實高（含外距）− 正文列上下外距
//   → 列容量(行) = floor(可用高 ÷ 行高) − BIO_CAP_SAFETY
// 這樣「肖像+資料+標題」佔多高（摘要/入傳緣由長短）都直接算進首列容量裡，不必猜。
// 只做一次（bioMeasured 填好後即跳過）；量不到就繼續用保守常量。
async function calibrateBioCaps() {
  const root = rootRef.value
  if (!root) return
  let got = false
  root.querySelectorAll('.sheet[data-page] .bd-half.left[data-bid]').forEach((half) => {
    const id = half.dataset.bid
    if (bioMeasured[id]) return
    const textEl = half.querySelector('.bd-text')          // 第 1 頁首列正文（首輪必有：默認容量 ≥ 1 行）
    if (!textEl) return
    const cs = getComputedStyle(textEl)
    const lineH = parseFloat(cs.lineHeight) || BIO_LINE_H
    const mt = parseFloat(cs.marginTop) || 0
    const mb = parseFloat(cs.marginBottom) || 0
    const halfH = half.clientHeight - BIO_MEASURE_EPS     // 整列高（扣掉整數取整誤差；首頁/續頁相同）
    const fullH = Math.max(0, halfH - mt - mb)             // 滿高列可用高（續頁兩列都用它）
    const side = half.querySelector('.bd-side')            // 續頁沒有這塊；首頁必有
    const title = half.querySelector('.bd-title')
    let blockH = 0
    if (side) blockH += side.offsetHeight
    if (title) {
      const ts = getComputedStyle(title)
      blockH += title.offsetHeight + (parseFloat(ts.marginTop) || 0) + (parseFloat(ts.marginBottom) || 0)
    }
    const firstH = Math.max(0, fullH - blockH)             // 首頁首列（肖像+資料+標題之下）可用高
    const lines = (h) => Math.max(0, Math.floor(h / lineH) - BIO_CAP_SAFETY)
    // 首列排不下一整行（含安全餘量）→ 容量 0：該列整列不放字，正文改由右半起排（不硬塞）
    const first0 = firstH >= (1 + BIO_CAP_SAFETY) * lineH ? Math.max(1, lines(firstH)) : 0
    // 續頁沒有肖像+資料+標題 → 兩列都是滿高（同一容量）
    const cont = Math.max(BIO_MIN_COL_LINES, lines(fullH))
    bioMeasured[id] = {
      first: [first0, Math.min(BIO_LINES_PER_COL, Math.max(BIO_MIN_COL_LINES, lines(fullH)))],
      cont: [cont, cont],
    }
    got = true
  })
  if (got) {                       // 容量變了 → 重排一次（重排後會再進來，此時 bioMeasured 已填好 → 直接驗溢出）
    sheets.value = []
    await nextTick()
    buildSheets()
    return
  }
  verifyBioFit()
}

// 渲染後實測：某篇傳記某一列仍超出容量 → 收緊該篇該套（首頁/續頁）該列的行數上限並重排一次。
// 每一列只會越收越緊（單調），故必定收斂；配合上面的保守常量，正常情況第一輪即通過。
async function verifyBioFit() {
  const root = rootRef.value
  if (!root || bioVerifyRound >= BIO_VERIFY_MAX) return
  const els = root.querySelectorAll('.bd-text[data-bid]')
  if (!els.length) return
  let changed = false
  els.forEach((el) => {
    const ch = el.clientHeight
    const sh = el.scrollHeight
    if (sh <= ch + 2) return                                 // 沒溢出（空列 sh=0 也走這裡）
    const id = el.dataset.bid
    const ci = Number(el.dataset.col) || 0
    const cont = Number(el.dataset.part) > 1                  // 續頁用「續頁容量」那一套
    const key = cont ? 'cont' : 'first'
    const caps = bioCaps(id, cont).slice()
    const cap = Number(el.dataset.cap) || caps[ci]
    const next = Math.max(1, Math.floor(cap * (ch / sh)) - 1) // 按該列的溢出比例收緊
    if (next < caps[ci]) {
      caps[ci] = next
      const o = bioCapOverride[id] || {}
      o[key] = caps
      bioCapOverride[id] = o
      changed = true
    }
  })
  if (!changed) return
  bioVerifyRound++
  sheets.value = []
  await nextTick()
  buildSheets()
}

// 點圖上諸子的頁碼 → 滾動到行傳對應頁
function jumpToBio(pageNo) {
  const root = rootRef.value
  if (!root) return
  const el = root.querySelector('.sheet[data-page="' + pageNo + '"]')
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function computeScale() {
  const el = scrollRef.value
  if (!el) return
  const w = el.clientWidth - 48
  if (w <= 0) return
  fitScale.value = Math.min(1, w / A3_W)
}

async function loadAll() {
  loading.value = true
  try {
    const [a, b, c, d, e, f, g] = await Promise.all([
      api.get('/members/chuizhu'),
      api.get('/settings/family-surname').catch(() => ({ data: {} })),
      api.get('/biographies').catch(() => ({ data: [] })),   // 名人傳記（失敗不影響譜書主體）
      api.get('/articles').catch(() => ({ data: [] })),      // 譜頭文化（譜序/凡例/祖訓/族規；失敗不影響主體）
      fetchAllSources(),                                     // 資料來源（行傳頁「據：…」小字腳注）
      fetchShrineGrave(),                                    // 祠宇志 / 茔墓志（失败返回空数组）
      api.get('/events').catch(() => ({ data: [] })),        // 家族大事记
    ])
    rawPages.value = a.data.pages || []
    surname.value = (b.data && b.data.family_surname) || ''
    tanghao.value = (b.data && b.data.tanghao) || ''
    xiuci.value = (b.data && b.data.xiuci) || '重修'   // 修次（服务器设置，默认「重修」）
    biographies.value = Array.isArray(c.data) ? c.data : []
    articles.value = Array.isArray(d && d.data) ? d.data : []
    sourceMap.value = groupSourcesByMember(e)
    shrines.value = (f && f.shrines) || []
    graves.value = (f && f.graves) || []
    events.value = Array.isArray(g && g.data) ? g.data : []
    bioVerifyRound = 0
    bioCellCalibrated = false
    bioCellFixRound = 0
    BIO_TEXT_CAP = 36
    Object.keys(bioCapOverride).forEach((k) => { delete bioCapOverride[k] })
    Object.keys(bioMeasured).forEach((k) => { delete bioMeasured[k] })
    sheets.value = []
    await nextTick()
    buildSheets()
  } finally {
    loading.value = false
  }
}

// 裝訂邊會擠窄版心 → 重新量測分頁（先清空讓量測區重出，再分頁；並重跑量測與溢出校正）
watch(bind, async () => {
  if (!rawPages.value.length && !bookBios.value.length) return
  bioVerifyRound = 0
  bioCellCalibrated = false
  bioCellFixRound = 0
  BIO_TEXT_CAP = 36
  Object.keys(bioCapOverride).forEach((k) => { delete bioCapOverride[k] })
  Object.keys(bioMeasured).forEach((k) => { delete bioMeasured[k] })
  sheets.value = []
  await nextTick()
  buildSheets()
})

// ===== 自測探針（只讀，不影響版面）=====
// 譜頭「格內行數」斷言：豎排下每一「行」= 一條豎線。
// ⚠️ 必須用 Range.getClientRects() 取文本行盒再按 x 聚類（容差 4px）；
//    用 el.getClientRects() 對塊級元素只會返回自身一個矩形 → 假通過。
function prefaceLineProbe() {
  const lineCountOf = (el) => {
    const r = document.createRange()
    r.selectNodeContents(el)
    const xs = []
    ;[...r.getClientRects()].forEach((b) => { if (!xs.some((u) => Math.abs(u - b.x) <= 4)) xs.push(b.x) })
    return xs.length
  }
  const perPage = []
  const dist = {}
  let cols = 0, multi = 0
  rootRef.value && rootRef.value.querySelectorAll('.sheet .ph-col').forEach((col) => {
    const el = col.firstElementChild
    if (!el) return
    const n = lineCountOf(el)
    cols++
    dist[n] = (dist[n] || 0) + 1
    if (n >= 2) {
      multi++
      const ear = col.closest('.page')?.parentElement?.querySelector('.ear-branch')
      perPage.push({ ear: ear ? ear.textContent.trim() : '', lines: n, chars: el.textContent.length, text: el.textContent.slice(0, 16) })
    }
  })
  return { cols, multiLineCols: multi, lineDist: dist, maxLines: Math.max(0, ...Object.keys(dist).map(Number)), offenders: perPage.slice(0, 8),
           pages: (sheets.value || []).filter((x) => x.kind === 'preface').map((x) => ({ pageNo: x.pageNo, partNo: x.partNo, partTotal: x.partTotal, cols: x.right.length + x.left.length, content: x.right.concat(x.left).filter((c) => c.kind !== 'blank').length })) }
}


// ===== 圖片展示（多條目；localStorage 持久化；舊「單塊」數據自動遷移成 1 個條目；空條目不產頁）=====
import GalleryDialog from '../components/GalleryDialog.vue'
import { createGalleryStore, createFamilyGalleryStore, buildEntryPages, entrySummaries } from '../utils/galleryStore'
import { fetchMediaImages, fetchMemberImages, fetchBranchImages } from '../utils/galleryPool'
import { onGalleryImgError } from '../utils/thumb'

const GALLERY_KEY = 'suzhuGallery'
const gallery = createFamilyGalleryStore()
const galleryEntries = gallery.entries
const galleryOpen = ref(false)
const galleryPool = ref([])        // 可選圖片（家族多媒體 + 成員附件）
const galleryPoolLoading = ref(false)
const galleryImageTotal = computed(() => galleryEntries.value.reduce((n, e) => n + (e.images ? e.images.length : 0), 0))
function loadGallery() { gallery.load() }
function saveGallery() { gallery.save() }
function onGalleryEntries(list) { galleryEntries.value = list; gallery.save(); buildSheets() }
async function loadGalleryPool(force = false) {
  if ((galleryPool.value.length || galleryPoolLoading.value) && !force) return
  galleryPoolLoading.value = true
  try {
    const media = await fetchMediaImages()          // ① 家族多媒體：1 個請求，秒出
    galleryPool.value = media
    const list = (rawPages.value || []).map((p) => p && p.member).filter(Boolean)
    const [atts, brs] = await Promise.all([
      fetchMemberImages(list),                      // ② 成員附件：併發 8 路
      fetchBranchImages(),                          // ③ 各房支支譜已選圖片（本機 localStorage）
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
    if (rawPages.value && rawPages.value[0] && rawPages.value[0].member) fd.append('member_id', rawPages.value[0].member.id)
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
// 本視圖的滾動容器是 .scroll（overflow:auto），不是 window → 對它做 scrollTo / 監聽
const showTopBtn = ref(false)
const TOP_SHOW_AT = 200
function onScrollTopBtn() {
  const el = scrollRef.value
  showTopBtn.value = !!el && el.scrollTop > TOP_SHOW_AT
}
function backToTop() {
  const el = scrollRef.value
  if (el) { try { el.scrollTo({ top: 0, behavior: 'smooth' }) } catch (e) { el.scrollTop = 0 } }
  else { try { window.scrollTo({ top: 0, behavior: 'smooth' }) } catch (e) { window.scrollTo(0, 0) } }
}

// ===== 目錄點擊跳轉 =====
// 锚点口径：每个分页块带 data-page-no（= 书内页码），目录条目按自己的页码定位（不靠 DOM 序号猜）
const highlightPageNo = ref(null)
let highlightTimer = null
function jumpToPage(pageNo) {
  if (pageNo == null) return
  const root = rootRef.value || document
  const el = root.querySelector(`.sheet[data-page-no="${pageNo}"]`) || root.querySelector(`[data-page-no="${pageNo}"]`)
  if (!el) return
  try { el.scrollIntoView({ behavior: 'smooth', block: 'start' }) } catch (e) { el.scrollIntoView() }
  // 短暂高亮目标页，便于确认跳对了（打印样式里已排除）
  highlightPageNo.value = pageNo
  if (highlightTimer) clearTimeout(highlightTimer)
  // 平滑滚动可能耗时 1~2s：高亮保持 2.6s，确保滚动落位后仍能看到描边，再自动消失
  highlightTimer = setTimeout(() => { highlightPageNo.value = null }, 2600)
}
function exportPDF() {
  window.print()
}

onMounted(() => {
  window.__suzhuProbe = prefaceLineProbe
  loadGallery()
  if (scrollRef.value) scrollRef.value.addEventListener('scroll', onScrollTopBtn, { passive: true })
  loadAll()
  computeScale()
  applyPrintStyle()
  if (scrollRef.value) {
    ro = new ResizeObserver(() => { computeScale() })
    ro.observe(scrollRef.value)
  }
})

onBeforeUnmount(() => {
  delete window.__suzhuProbe
  if (scrollRef.value) scrollRef.value.removeEventListener('scroll', onScrollTopBtn)
  if (ro) ro.disconnect()
  const el = document.getElementById('chuizhu-print-page')
  if (el) el.remove()
})
</script>

<style scoped>
.chuizhu { display: flex; flex-direction: column; height: 100%; }
.toolbar { display: flex; gap: 14px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.title { font-size: 20px; font-family: "Songti SC", "SimSun", "STSong", serif; color: #3a2a18; }
.hint { color: #8c5a3c; font-size: 12px; }

.scroll { flex: 1; overflow: auto; background: #d8d2c2; border: 1px solid #c8b896; border-radius: 4px; padding: 24px; }
.pages { display: flex; flex-direction: column; align-items: center; gap: 28px; }
/* 裝訂線：在頁面左側預留一條真正的空白裝訂邊（14mm 留白），線畫在留白内、離紙邊 8mm（不會被打印裁切），屏幕預覽與打印一致 */
.chuizhu.bind-on .page { position: relative; padding-left: calc(9px + 14mm); }
.chuizhu.bind-on .page::before {
  content: '裝訂線 · 活頁孔預留';
  position: absolute;
  left: 8mm;
  top: 8px;
  bottom: 8px;
  width: 5mm;
  writing-mode: vertical-rl;
  text-orientation: mixed;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  letter-spacing: 4px;
  color: #c0392b;
  border-left: 1px dashed #c0392b;
  pointer-events: none;
}
.sheet { display: flex; justify-content: center; width: max-content; min-width: 100%; }
/* 目錄：與封面同級的「卷首塊」，刻意不叫 .sheet —— 它不進 sheets 模型、不佔正文頁號 */
.toc-sheet { display: flex; justify-content: center; width: 100%; }
/* 內封（書名頁）與牌記（版權頁）：同屬「卷首/卷末」外圍塊，也不叫 .sheet（不佔正文頁號） */
.inner-sheet, .colo-sheet { display: flex; justify-content: center; width: 100%; }
/* 內封/牌記的內框：直接讓共用組件填滿版框（.bin 本是 flex） */
.bin.it-host { display: block; }

/* ===== 五服圖（服制圖）頁：整頁一幅矢量圖 ===== */
.ff-doc { flex: 1; display: flex; flex-direction: column; min-height: 0; padding: 6px 12px 4px 14px; }
/* 標題只排左半（版心居中貫通全頁，標題不可跨版心）；右半由圖內的服制表佔位 */
.ff-head { flex: 0 0 auto; align-self: flex-start; display: flex; align-items: baseline; gap: 18px;
           width: calc(50% - 42px); justify-content: center; padding: 2px 0 8px; }
.ff-title { font-size: 34px; font-weight: 700; letter-spacing: 14px; text-indent: 14px; color: #111; }
.ff-sub { font-size: 15px; color: #6b5a44; letter-spacing: 1px; }
.ff-body { flex: 1 1 auto; min-height: 0; display: flex; align-items: center; justify-content: center; }

/* 離屏量測，不參與正常排版 */
.measure { position: fixed; left: -99999px; top: 0; pointer-events: none; }
.measure .sheet { display: block; }

/* ===== A3 横版 ===== */
.page { width: 1587px; height: 1123px; background: #fffdf6; display: flex; padding: 9px;
        color: #1a1a1a; font-family: "Songti SC", "SimSun", "STSong", "宋体", serif; }
.board { flex: 1; border: 1px solid #1a1a1a; padding: 5px; }
.bin   { border: 3px solid #1a1a1a; height: 100%; display: flex; }

/* ===== 封面（横版） ===== */
.cv-bin { align-items: center; justify-content: center; gap: 70px; position: relative; }
/* 封面家族名/堂号整组：堂号在上、家族名在下；水平居中；整组垂直居中略偏上。
   上下间距 10→20mm（属主 2026-10-19 要求加大一倍）。为保持家族名 y 原地不动，
   整组再上移 5mm（= 间距增量的一半）：间距 +10mm 本会让居中整组的家族名下移 5mm，
   故 translateY 由 -24px 再减 5mm；结果家族名不动、堂号上移 10mm。 */
.cv-head { display: flex; flex-direction: column; align-items: center; gap: 20mm;
           transform: translateY(calc(-24px - 5mm)); }
/* 封面家族名：横排、居中（属主 2026-10-19 要求；垂珠体正文仍竖排，只这两个元素横排）
   字号 150→180（inline 由 coverTitleFs 给，长名按 180×4/n 收缩）；字距 0 使元素框中心=字形中心 */
.cv-title { writing-mode: horizontal-tb; text-orientation: mixed;
            font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
            font-size: 180px; font-weight: 700; letter-spacing: 0; color: #111; line-height: 1; white-space: nowrap; }
/* 堂号：横排、在家族名上方；字号 56→64（家族名 180 > 堂号 64，主次不变） */
.cv-sub { writing-mode: horizontal-tb; text-orientation: mixed;
          font-family: "Songti SC", "SimSun", "STSong", "宋体", serif;
          font-size: 64px; font-weight: 400; letter-spacing: 8px; color: #333; line-height: 1; white-space: nowrap; }
.cv-foot { position: absolute; bottom: 56px; left: 0; right: 0; text-align: center;
           font-size: 26px; letter-spacing: 8px; color: #333; }
.cv-seal { position: absolute; right: 80px; bottom: 150px; width: 100px; height: 100px; border: 5px solid #b22;
           color: #b22; display: flex; align-items: center; justify-content: center;
           font-size: 56px; font-weight: 700; border-radius: 6px; opacity: .85; }

/* ===== 目錄（A3 橫版：與正文頁同款書耳+版心；條目只排左側一列，右側留空） =====
   左列寬 = 半幅 − 版心半寬(42px) − 與版心的間距(40px)，所以條目與頁碼都在版心左側、不會被版心壓住；
   條目少時右側與下半頁自然留白，日後要放別的内容直接往右半排即可。 */
.toc-doc { flex: 1 1 auto; min-height: 0; align-self: flex-start;   /* 不拉伸：右側刻意留空 */
           width: calc(50% - 42px - 40px);
           display: flex; flex-direction: column; padding: 64px 0 40px 52px; }
.toc-title { flex: 0 0 auto; font-size: 60px; font-weight: 700; letter-spacing: 28px; text-indent: 28px;
             color: #111; text-align: center; padding-bottom: 18px; border-bottom: 4px double #1a1a1a; }
.toc-list { flex: 1 1 auto; min-height: 0; display: flex; flex-direction: column; gap: 22px; margin-top: 52px; }
.toc-row { display: flex; flex-wrap: wrap; align-items: baseline; font-size: 26px; color: #1a1a1a; }
/* 可点击的目錄條目：手型 + hover 高亮 */
/* 圖片展示：格線佈局（版面內容區內，不得溢出） */
.gallery-doc { flex: 1 1 auto; display: flex; align-items: center; justify-content: center; padding: 10px 6px; min-width: 0;
  min-height: 0; }   /* min-height:0 必须：否则 auto 最小尺寸＝图片的 min-content 高，把图爿页撑出纸张（实测 2 张溢出 3359px、4/9 张约 1970px） */
.gallery-grid { display: grid; gap: 14px; width: 100%; height: 100%; align-content: stretch; justify-items: stretch; align-items: stretch; }
.gallery-doc.g1 .gallery-grid { grid-template-columns: 1fr; grid-template-rows: 1fr; }
/* 垂珠體整頁固定 A3 橫版（1587×1123）→ 「2 張」一律左右並排（2 列 × 1 行），與橫版牒記式同口徑 */
.gallery-doc.g2 .gallery-grid { grid-template-columns: repeat(2, 1fr); grid-template-rows: 1fr; }
.gallery-doc.g4 .gallery-grid { grid-template-columns: repeat(2, 1fr); grid-template-rows: repeat(2, 1fr); }
.gallery-doc.g6 .gallery-grid { grid-template-columns: repeat(2, 1fr); grid-template-rows: repeat(3, 1fr); }
.gallery-doc.g9 .gallery-grid { grid-template-columns: repeat(3, 1fr); grid-template-rows: repeat(3, 1fr); }
.g-cell { margin: 0; min-width: 0; min-height: 0; display: flex; flex-direction: column; align-items: center; justify-content: center;
  box-sizing: border-box; padding: 3px; background: #fbf8f1; }   /* 淡底＝讓「固定格子」看得見 */
.g-cell img { width: 100%; height: 100%; min-height: 0; flex: 0 1 auto; object-fit: contain;
  border: 1px solid #b9ad91; background: #fff; display: block; }   /* 格內等比縮放：不裁切、不拉伸（比例不同就留白） */
.g-cell figcaption { flex: 0 0 auto; width: 100%; height: 17px; line-height: 17px; margin-top: 3px;
  font-size: 14px; color: #333; text-align: center;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }   /* 格內預留圖說高度（固定 17px）：圖說再長也不壓到下一格 */
.g-cell.img-missing { background: repeating-linear-gradient(45deg,#f7f3ea,#f7f3ea 8px,#efe8da 8px,#efe8da 16px); }
.g-cell.img-missing img { display: none; }
.g-cell.img-missing::before { content: '图片不可用'; color: #b08840; font-size: 13px; line-height: 1.4; }
/* 選擇器 */
/* 回到頂部：右下角固定，圓形；打印隱藏 */
.back-top { position: fixed; right: 24px; bottom: 24px; z-index: 2000; box-shadow: 0 4px 14px rgba(0,0,0,.22); }
@media (min-width: 768px) { .back-top { right: 44px !important; bottom: 44px !important; } }
.ph-fade-enter-active, .ph-fade-leave-active { transition: opacity .2s ease, transform .2s ease; }
.ph-fade-enter-from, .ph-fade-leave-to { opacity: 0; transform: translateY(8px); }
@media print { .back-top { display: none !important; } }
.toc-row.toc-clickable { cursor: pointer; border-radius: 4px; transition: background-color .15s ease; }
.toc-row.toc-clickable:hover { background: #f3e7cf; }
/* 跳轉後短暫高亮目標頁（僅屏幕；打印不顯示） */
.sheet.page-flash .page { outline: 3px solid #b08840; outline-offset: 4px; }
@media print { .sheet.page-flash .page { outline: none !important; } }
.toc-name { flex: 0 0 auto; letter-spacing: 3px; }
.toc-note { flex: 0 1 auto; min-width: 0; font-size: 16px; color: #8c5a3c; letter-spacing: 1px; }
.toc-dots { flex: 1 1 auto; min-width: 28px; margin: 0 12px; border-bottom: 2px dotted #8b7c66;
            transform: translateY(-7px); }
.toc-page { flex: 0 0 auto; white-space: nowrap; font-weight: 700; letter-spacing: 2px; }
.toc-foot { flex: 0 0 auto; margin-top: 30px; padding-top: 16px; 
            font-size: 15px; line-height: 1.8; color: #8b7c66; }

/* ===== 內容：整頁寬的一列行 ===== */
.content { flex: 1; display: flex; flex-direction: column; min-width: 0; position: relative; }
/* 祠宇志 / 茔墓志：两栏文章流；中间 84px 空槽正好落在版心（.spine）上 */
.sg-doc { flex: 1 1 auto; min-height: 0; width: 100%; height: 100%; --sg-fs: 17px; overflow: hidden;
  padding: 18px 20px; box-sizing: border-box; }
/* 图版页：隐藏中央版心，让单张/双张图满版心；页码贴右下 */
.sheet.sg-plate-sheet .spine { display: none; }
.sheet.sg-plate-sheet .sg-doc { padding: 10px 14px; }

.sg-plate-page-no { position: absolute; right: 16px; bottom: 10px; font-size: 14px; color: #333; }
.gen-row { flex: 1; display: flex; align-items: stretch; min-height: 0; }
.part { display: flex; flex-direction: row-reverse; align-items: stretch; min-width: 0; overflow: hidden; }
.part.left  { flex: 1; padding: 10px 12px 8px 8px; }
.part.right { flex: 1; padding: 10px 8px 8px 12px; transform: translateX(4px); }
.gen-row:not(:last-child) .part { border-bottom: 1px solid #b9ad91; }
.gap { flex: 0 0 84px; }

.gen-side { flex: 0 0 auto; display: flex; flex-direction: row-reverse; align-items: center; gap: 7px; }
.gen-tag  { writing-mode: vertical-rl; text-orientation: upright; border: 2px solid #1a1a1a;
            border-radius: 8px; padding: 10px 6px; font-size: 22px; font-weight: 700; letter-spacing: 3px; }
.gen-rank { writing-mode: vertical-rl; text-orientation: upright; border: 1px solid #1a1a1a;
            padding: 7px 5px; font-size: 19px; letter-spacing: 1px; }

.members { flex: 1; display: flex; flex-direction: row-reverse; align-items: stretch;
           gap: 10px; min-width: 0; }
.memb { display: flex; flex-direction: row-reverse; align-items: stretch; gap: 4px; }

.m-name { writing-mode: vertical-rl; text-orientation: upright;
          font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif; font-size: 28px; font-weight: 700;
          line-height: 1; letter-spacing: 2px; padding: 0 3px; }

/* 詳情：每字段一豎列，列由右往左排 */
.m-detail { display: flex; flex-direction: row-reverse; align-items: stretch; gap: 2px;
            font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
            font-size: 24px; color: #1a1a1a; }
.d-col { writing-mode: vertical-rl; text-orientation: upright; line-height: 1.25; letter-spacing: 2px; }
/* 諸子：仿古譜 ——「生子N」在上，諸子名列於其下一排（由右往左），名下有頁碼（可點，跳行傳） */
.d-col.son-block { writing-mode: horizontal-tb; display: flex; flex-direction: column;
                   align-items: flex-end; gap: 2px; }
.sg-label { writing-mode: vertical-rl; text-orientation: upright; line-height: 1.25; letter-spacing: 2px; }
.sg-names { display: flex; flex-direction: row-reverse; align-items: flex-start; gap: 2px; }
.son-cell { display: flex; flex-direction: column; align-items: center; }
.son-name { writing-mode: vertical-rl; text-orientation: upright;
            font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
            font-size: 28px; font-weight: 700; line-height: 1; letter-spacing: 1px; }
.son-page { font-size: 11px; line-height: 1.2; color: #a33; margin-top: 2px; cursor: pointer; }
.son-page:hover { text-decoration: underline; }

/* ===== 行傳：牒記式（每人一豎格，四格：世代 / 關係 / 名 / 生卒生平；按世代長幼，自右而左連排） ===== */
.bio-sheet { flex: 1; display: flex; min-height: 0; }
.bio-half { flex: 1; min-width: 0; display: flex; flex-direction: row-reverse; align-items: stretch; }
.bio-gap { flex: 0 0 84px; }
.p-col { flex: 1 1 0; min-width: 0; display: flex; flex-direction: column; border-left: 1px solid #1a1a1a; }
.p-col:first-child { border-right: 1px solid #1a1a1a; }
/* 四格：世代（有世次才上黑條）/ 關係 / 名 / 生卒生平 */
.p-gen { flex: 0 0 34px; display: flex; align-items: center; justify-content: center;
         font-size: 17px; letter-spacing: 2px;
         border-bottom: 1px solid #1a1a1a; overflow: hidden; }
.p-gen.filled { background: #2b2b2b; color: #fffdf6; }
.p-rel { flex: 0 0 74px; display: flex; align-items: center; justify-content: center;
         font-size: 15px; color: #333; border-bottom: 1px solid #1a1a1a; overflow: hidden; }
/* 條首小字「某公第幾子」＝固定兩段（屬主口徑）：右＝父名＋公，左＝行第（長子／次子／第三子／第十五子）。
   兩段各自成豎列、格內不換行（nowrap），字號由 relFs() 按「兩段裡較長一段的字數」分檔給（見 script 註釋），
   原則是「在 74px 關係格 / 57px 列寬內取放得下的最大整數字號」——格高、版心、分頁一律不動，
   也不再按字數被動折行（舊版 5 字會折成 4+1，第二列只剩一個字）。 */
.p-rel .rel2 { display: flex; flex-direction: row-reverse; align-items: center; justify-content: center;
               gap: 6px; height: 100%; max-width: 100%; overflow: hidden; }
.p-rel .rel2 > span { writing-mode: vertical-rl; text-orientation: upright;
                      font-size: inherit; line-height: 1; padding: 0;
                      letter-spacing: 2px;   /* 豎排＝縱向字距；1~6 字各檔實測仍 ≤72px 格高，不裁字 */
                      white-space: nowrap; }
.p-name { flex: 0 0 90px; display: flex; align-items: center; justify-content: center;
          font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
          font-size: 26px; font-weight: 700; letter-spacing: 2px;
          border-bottom: 1px solid #1a1a1a; overflow: hidden; }
.p-v { writing-mode: vertical-rl; text-orientation: upright; line-height: 1.35; padding: 2px; }
/* 行傳「成員資料」：全頁統一 24px / line-height 1.25（不再按字數縮字號；超出一格就接寫到下一格）。
   格內可用：寬 49px（只能排 1 條豎列）、高 877px → 每格 36 字（見 script 的 BIO_TEXT_*）。
   世次 34 / 關係 74 / 名諱 90 / 正文剩餘 886.7，padding 6px 4px 一律不動。 */
.p-text { flex: 1 1 0; min-height: 0; overflow: hidden;
          /* mixed：漢字仍直立，西文/數字按豎排慣例旋轉 90°（upright 下每個數字佔 ~36px 縱向，
             混排資料（如 "2026-08-02"）容易把一格撐破；mixed 下數字只佔 ~12px，36 字必放得下） */
          writing-mode: vertical-rl; text-orientation: mixed;
          /* nowrap 關鍵：每格文字已由 JS 按「28 字/格」切好，格內必須是「一條豎列、不折行」。
             否則 "2026-08-02" 之類的連字號會成為換行點 → 折出第 2 列 → 橫向溢出被裁。 */
          white-space: nowrap;
          font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
          font-size: 24px; line-height: 1.25; color: #222; padding: 6px 4px; }
/* 資料來源小字腳注：接在該人正文末尾、同一條豎列（inline 續寫），字號 15px（正文 24px 的 0.625）。
   格高/格寬/版心一律不動；切片時已按 15/24 折算成正文格容量，不會撐破格子。 */
.p-text .p-src { font-size: 15px; line-height: 1.25; color: #4a4a4a; letter-spacing: 0; }
.p-detail { flex: 1 1 0; min-height: 0; overflow: hidden;
            writing-mode: vertical-rl; text-orientation: upright;
            font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
            font-size: 13px; line-height: 1.44; color: #222; padding: 6px 4px; }

/* ===== 名人傳記：一篇一頁（只有第 1 頁帶肖像+資料；「傳」標題與正文自其下方起排，
   寫滿自右半接續；續頁不帶肖像/資料/標題 —— 純正文，兩列都自列頂起排） =====
   不重疊由「布局」保證：左半是同一條 column 順序流 .bd-side → .bd-title → .bd-text，
   三者都在正常文檔流裡（無 position:absolute、無負 margin）；.bd-side/.bd-title 一律
   flex:0 0 auto（不可壓縮、不可被正文擠動），只有 .bd-text 從剩餘空間長 → 幾何上不可能互相覆蓋。
   容量（每列可容行數）由渲染後按真實 DOM 實測剩餘高度換算（首頁/續頁兩套），見 calibrateBioCaps。 */
/* ===== 譜頭文化頁：整列一豎格，列內不分格（無橫線），列間豎線分隔 ===== */
.ph-doc { flex: 1; display: flex; min-height: 0; }
/* 左右兩半等寬（與行傳同口徑：半幅 − 版心半寬），中間留出版心寬度 */
.ph-half { flex: 0 0 calc(50% - 42px); min-width: 0; min-height: 0; display: flex; flex-direction: row-reverse;
           padding: 14px 12px 14px 14px; overflow: hidden; }
.ph-gap { flex: 0 0 84px; }   /* 與版心同寬（.spine 84px） */
/* 一個豎格＝一列：列內就是一整條豎排文字通道，內部不再分格 */
.ph-col { flex: 0 0 68px; width: 68px; min-width: 0; height: 100%; overflow: hidden;
          border-left: 1px solid #b9ad91; }
.ph-col:last-child { border-right: 1px solid #b9ad91; }
/* 類型列：整列就寫類型名（凡例／祖訓／譜序／族規），居中 */
.ph-type { writing-mode: vertical-rl; text-orientation: upright; height: 100%;
           display: flex; align-items: center; justify-content: center;
           font-size: 34px; font-weight: 700; letter-spacing: 12px; }
/* 標題列：整列就寫該篇標題，居中 */
.ph-title { writing-mode: vertical-rl; text-orientation: upright; height: 100%; width: 100%;
            display: flex; align-items: center; justify-content: center;
            font-size: 30px; font-weight: 700; letter-spacing: 8px; }
/* 正文列：自上而下寫滿一列就換下一列（單行豎排，格內不折行） */
.ph-text { writing-mode: vertical-rl; text-orientation: upright; height: 100%; width: 100%;
           /* 竖排 flex 实测矩阵（短列 9 字 / 长列 47 字，列高 530px）：
              · justify-content flex-start → 上缝 0（**文字从列顶起排**）✅；center → 短列上缝 215.3（被推到中段 ❌）
              · align-items center → 左右缝 12/10 ✅；flex-start → 21/1 ❌
              → 采用 justify-content:flex-start + align-items:center（列顶起排 + 左右居中） */
           display: flex; align-items: center; justify-content: flex-start;
           /* letter-spacing 由 1px 收到 0：实测量得单行上限由 46 字升到 48 字（腾出 1 字空间给 47 字/列） */
           font-size: 21px; line-height: 30px; letter-spacing: 0; overflow: hidden; }
.ph-cont { font-size: 16px; color: #8c5a3c; letter-spacing: 2px; }
/* 沒內容的格子也照常畫出格線（整頁畫滿，不留白） */
.ph-blank { height: 100%; }

.bio-doc { flex: 1; display: flex; min-height: 0; }
/* 左右兩半等寬：寬度寫死為「半幅 - 版心半寬」，保證 .bd-gap 正好落在 .spine（居中 84px）上，正文不壓版心 */
.bd-half { flex: 0 0 calc(50% - 42px); min-width: 0; min-height: 0; display: flex; flex-direction: column; }
/* 左半上段：肖像 + 名諱字號 + 小字履歷（位置與樣式一律不變，仍在上方那一塊；flex:0 0 auto = 不被壓縮） */
.bd-side { flex: 0 0 auto; display: flex; flex-direction: column; align-items: center;
           gap: 16px; padding: 44px 40px 28px 44px; }
.bd-gap { flex: 0 0 84px; }   /* 與版心同寬（.spine 84px），左右兩半正好貼著版心兩緣 */
/* 標題與正文：緊接「肖像+資料」下方起排；左右兩半內距相同，故字寬與舊版完全一致。
   標題底 0、正文頂 16px（天頭）→ 標題線到正文首行固定 16px；續頁沒有標題，正文同樣從列頂 16px 起排 */
.bd-title { flex: 0 0 auto; margin: 0 24px 0 28px; padding-bottom: 10px; text-align: center;
            font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
            font-size: 40px; font-weight: 700; letter-spacing: 16px;
            border-bottom: 1px solid #1a1a1a; }
.bd-portrait img, .bd-portrait .bd-ph { width: 210px; height: 270px; object-fit: cover;
           border: 1px solid #1a1a1a; background: #f2ece0; }
.bd-portrait .bd-ph { display: flex; align-items: center; justify-content: center;
           font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
           font-size: 92px; color: #a3927c; }
.bd-name { font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
           font-size: 44px; font-weight: 700; letter-spacing: 6px; color: #111;
           text-align: center; line-height: 1.2; }
.bd-zihao { display: block; margin-top: 10px; font-size: 20px; font-weight: 400;
            letter-spacing: 2px; color: #4a4137; }
.bd-meta { align-self: stretch; text-align: center; font-size: 16px; line-height: 1.9; color: #4a4137; }
.bd-summary { align-self: stretch; text-align: left; font-size: 17px; line-height: 1.9; color: #3d3225; }
.bd-reason { align-self: stretch; text-align: left; font-size: 15px; line-height: 1.8; color: #8b7c66; }
/* 正文用楷體；刻意不設 overflow:hidden —— 放不下時由 splitBioCols 續頁（verifyBioFit 兜底校正），
   寧可多出一頁也絕不把字裁掉。
   天頭 16px：首頁在「傳」標題之下；續頁沒有標題，兩列都自列頂 16px 起排（左右對齊） */
.bd-text { flex: 1 1 0; min-height: 0; overflow: visible; margin: 16px 24px 20px 28px;
           font-family: "KaiTi", "STKaiti", "Kaiti SC", "楷体", serif;
           font-size: 19px; line-height: 1.9; color: #222; }
.bd-text p { margin: 0 0 10px; text-indent: 2em; text-align: justify; }
.bd-cont { text-align: right; font-size: 15px; color: #8b7c66; margin-bottom: 6px; }

/* ===== 版心（中）：譜名 + 房派世次 + 頁碼 ===== */
.spine { position: absolute; top: 0; bottom: 0; left: 50%; transform: translateX(-50%); width: 84px;
         background: #fffdf6; border-left: 1px solid #1a1a1a; border-right: 1px solid #1a1a1a;
         display: flex; flex-direction: column; align-items: center; padding: 16px 0 10px; z-index: 2; }
.spine-title { writing-mode: vertical-rl; text-orientation: upright; font-size: 46px; font-weight: 700;
               letter-spacing: 12px; color: #111; }
.spine-meta  { writing-mode: vertical-rl; text-orientation: upright; font-size: 15px; letter-spacing: 6px;
               color: #333; margin-top: 18px; }
.spine-page  { margin-top: auto; writing-mode: vertical-rl; text-orientation: upright; font-size: 22px;
               font-weight: 700; letter-spacing: 2px; color: #111; }

/* ===== 書耳（右）：房派（有格）；分頁時加「當前第X頁」 ===== */
.ear { flex: 0 0 44px; align-self: stretch; border-left: 1px solid #1a1a1a; border-right: 1px solid #1a1a1a;
       display: flex; flex-direction: column; align-items: center; padding-top: 44px; }
.ear-branch { writing-mode: vertical-rl; text-orientation: upright; font-size: 26px; font-weight: 700; letter-spacing: 9px; }
.ear-page { writing-mode: vertical-rl; text-orientation: upright; font-size: 15px; letter-spacing: 4px;
            color: #333; margin-top: 12px; }
</style>

<style>
/* 打印：每页 A3 横版、一张一分页 */
@media print {
  .chuizhu .toolbar { display: none !important; }
  .chuizhu .measure { display: none !important; }
  .chuizhu .scroll { overflow: visible !important; padding: 0 !important; border: none !important; background: none !important; }
  .chuizhu .pages { display: block !important; gap: 0 !important; }
  .chuizhu .sheet { display: block !important; width: auto !important; page-break-after: always; }
  /* 祠宇志/茔墓志：一逻辑页=一打印页；内容超长时裁切，绝不拆成第二页 */
  .chuizhu .sheet.sg-sheet { break-before: page !important; page-break-before: always !important;
    break-after: page !important; page-break-after: always !important;
    break-inside: avoid !important; page-break-inside: avoid !important; }
  .chuizhu .sheet.sg-sheet .page { break-inside: avoid !important; page-break-inside: avoid !important; overflow: hidden !important; }
  /* 目錄塊：與封面一樣各佔一頁、各自分頁（不進 sheets 模型 → 不佔正文頁號） */
  .chuizhu .toc-sheet { display: block !important; width: auto !important; page-break-after: always; }
  /* 內封（封面之後）與牌記（全書末頁）同樣各佔一頁 */
  .chuizhu .inner-sheet { display: block !important; width: auto !important; page-break-after: always; }
  .chuizhu .colo-sheet { display: block !important; width: auto !important; page-break-before: always; }
  /* 邊距與縮放由動態 <style id="chuizhu-print-page"> 注入（見 applyPrintStyle），不再寫死 */
}
</style>
