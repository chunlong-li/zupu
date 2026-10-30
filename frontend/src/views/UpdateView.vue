<template>
  <div class="update-page">
    <el-card class="section">
      <template #header><span>版本更新</span></template>
      <div class="update-box">
        <div class="update-row">
          <span class="label">当前版本</span>
          <span class="value"><b>v{{ currentVersion }}</b></span>
        </div>
        <el-divider />
        <div class="update-row">
          <span class="label">更新说明</span>
          <span class="value hint" style="flex-direction: column; align-items: flex-start; gap: 6px;">
            <span>1. 下载最新版本的更新包（.tar.gz）</span>
            <span>2. 点击下方「选择文件」上传更新包</span>
            <span>3. 确认版本号后点击「立即更新」</span>
            <span>4. 系统自动替换并重启，稍后自动刷新</span>
          </span>
        </div>

        <el-upload
          drag
          action="#"
          :auto-upload="false"
          :limit="1"
          :on-change="onFileChange"
          :on-remove="onFileRemove"
          :file-list="fileList"
          accept=".tar.gz,.tgz"
          style="margin: 8px 0 16px;"
        >
          <div class="el-upload__text">拖拽更新包到此处，或 <em>点击选择文件</em></div>
          <template #tip>
            <div class="el-upload__tip">仅支持 .tar.gz 格式，文件名需包含版本号（如 族谱系统v0.3.0826.1.source.tar.gz）</div>
          </template>
        </el-upload>

        <div class="update-row" v-if="selectedFile">
          <span class="label">所选文件</span>
          <span class="value">
            {{ selectedFile.name }}
            <el-tag v-if="selectedVersion" type="warning" size="small">版本 v{{ selectedVersion }}</el-tag>
            <el-tag v-else type="danger" size="small">未识别版本号</el-tag>
          </span>
        </div>

        <div class="update-actions">
          <el-button type="success" :loading="updating" :disabled="!selectedFile || !selectedVersion || !canUpdate" @click="doUpdate">
            立即更新（本地上传）
          </el-button>
          <el-button :disabled="!selectedFile" @click="clearFile">清除</el-button>
          <el-button type="warning" :loading="rolling" @click="doRollback">回滚上一版本</el-button>
        </div>

        <el-divider />
        <div class="update-row">
          <span class="label">在线更新</span>
          <span class="value hint" style="flex-direction: column; align-items: flex-start; gap: 6px;">
            <span>从多个更新源（latest.json）自动检测最新版本，取版本最高者，一键下载并更新</span>
            <span v-if="sourcesCount > 0" class="dir-path">已配置 {{ sourcesCount }} 个更新源</span>
            <span v-else class="dir-path">（未配置更新源，点击右侧「更新地址」添加）</span>
          </span>
        </div>
        <div class="server-update">
          <el-button type="primary" :loading="checking" @click="checkManifestUpdate">检查更新</el-button>
          <el-button v-if="shareInfo.hasUpdate" type="success" :loading="updatingManifest" @click="doUpdateFromManifest">
            立即更新
          </el-button>
          <el-button v-else-if="shareInfo.canDowngrade" type="warning" plain :loading="updatingManifest" @click="doUpdateFromManifest">
            回退到 v{{ shareInfo.latest }}
          </el-button>
          <span v-else-if="shareInfo.checked" class="check-result">
            {{ shareInfo.error ? '检测失败：' + shareInfo.error : '当前已是最新版本 v' + currentVersion }}
          </span>
          <el-button @click="openSourcesDialog">更新地址</el-button>
        </div>
        <div class="update-tip" v-if="shareInfo.hasUpdate">
          <el-alert type="success" :closable="false" show-icon
            :title="`发现新版本 v${shareInfo.latest}${shareInfo.notes ? '：' + shareInfo.notes : ''}（来自 ${shareInfo.source}）`" />
        </div>
        <div class="update-tip" v-else-if="shareInfo.canDowngrade">
          <el-alert type="warning" :closable="false" show-icon
            :title="`服务器最新版本 v${shareInfo.latest} 低于当前版本 v${currentVersion}，可回退（来自 ${shareInfo.source}）`" />
        </div>
        <div class="update-tip" v-if="shareInfo.checked && shareInfo.sources && shareInfo.sources.length">
          <div class="source-scan">
            <div class="source-scan-title">各更新源检测结果：</div>
            <div v-for="(s, i) in shareInfo.sources" :key="i" class="source-scan-item">
              <span class="src-status" :class="s.ok ? 'ok' : 'fail'">{{ s.ok ? '✓' : '✗' }}</span>
              <span class="src-url">{{ s.source }}</span>
              <span v-if="s.ok" class="src-ver">v{{ s.version }}</span>
              <span v-else class="src-err">{{ s.error }}</span>
            </div>
          </div>
        </div>
        <div class="update-tip" v-if="updatingManifest">
          <el-alert type="warning" :closable="false" show-icon
            title="正在下载并部署更新包，完成后服务会自动重启，请勿关闭页面。" />
        </div>

        <!-- 更新地址管理弹窗 -->
        <el-dialog v-model="sourcesDialogVisible" title="更新地址管理" width="620px">
          <div class="sources-dialog-tip">
            配置更新源地址（latest.json 清单）。检查更新时按顺序逐个获取，取版本号最高者作为最新版本。
          </div>
          <div class="sources-list">
            <div v-for="(s, i) in sourcesDraft" :key="i" class="sources-row">
              <span class="src-idx">{{ i + 1 }}</span>
              <el-input v-model="sourcesDraft[i]" placeholder="https://example.com/latest.json" />
              <el-button size="small" type="danger" text @click="sourcesDraft.splice(i, 1)">删除</el-button>
            </div>
            <el-button size="small" text type="primary" @click="sourcesDraft.push('')">+ 添加地址</el-button>
          </div>
          <div class="sources-dialog-tip" v-if="builtinSource">
            内置更新源（来自服务器环境变量，不可删除）：{{ builtinSource }}
          </div>
          <template #footer>
            <el-button @click="sourcesDialogVisible = false">取消</el-button>
            <el-button type="primary" :loading="savingSources" @click="saveSources">保存</el-button>
          </template>
        </el-dialog>

        <el-divider />
        <div class="update-row">
          <span class="label">测试版（beta）更新</span>
          <span class="value hint" style="flex-direction: column; align-items: flex-start; gap: 6px;">
            <span>手动检查测试版，不会自动下载或安装；测试版可能不稳定。</span>
          </span>
        </div>
        <div class="server-update">
          <el-button type="primary" plain :loading="loadingServer" @click="checkBeta">检查测试版</el-button>
          <span v-if="betaChecked" class="check-result">共 {{ serverPackages.length }} 个测试版</span>
          <el-button v-if="betaChecked" size="small" @click="checkBeta">刷新</el-button>
        </div>
        <template v-if="betaChecked">
          <div class="server-update">
            <el-select v-model="serverFile" placeholder="选择测试版更新包" :loading="loadingServer" clearable style="flex:1;">
              <el-option v-for="p in serverPackages" :key="p.name" :label="`${p.name}（v${p.version}）`" :value="p.name" />
            </el-select>
            <el-button type="warning" :loading="updatingServer" :disabled="!serverFile" @click="downloadBeta">下载</el-button>
          </div>
          <div class="update-tip" v-if="selectedBeta">
            <el-alert :closable="false" show-icon type="warning"
              :title="`测试版 v${selectedBeta.version}：${selectedBeta.notes || '无更新说明'}`" />
          </div>
          <div class="update-tip" v-if="serverPackages.length === 0 && !loadingServer">
            <el-alert type="info" :closable="false" show-icon title="当前未检测到测试版更新包。" />
          </div>
        </template>
        <div class="update-tip" v-if="!canUpdate && selectedVersion">
          <el-alert type="warning" :closable="false" show-icon
            :title="`所选版本 v${selectedVersion} 与当前版本相同`" />
        </div>
        <div class="update-tip" v-if="updating">
          <el-alert type="warning" :closable="false" show-icon
            title="更新中，请勿关闭页面。更新完成后服务会自动重启，稍后自动刷新。" />
        </div>
      </div>
    </el-card>

    <el-card class="section update-log">
      <template #header><span>版本更新日志</span></template>
      <el-collapse v-model="activeLogs">
                                      <el-collapse-item name="logStable139">
            <template #title>
              <span class="log-title">v1.3.9</span>
              <span class="log-count">2026-10-28 发布</span>
            </template>
            <div class="log-entry">
              <div class="log-entry-head">
                <span class="log-ver">v1.3.9</span>
                <span class="log-date">2026-10-28</span>
              </div>
              <div class="log-entry-body">
                <p><b>打印 / 导出 PDF 修正（重要）：</b></p>
                <p>· 修正"<b>导出 PDF 后格子变窄、正文换行、底部被裁</b>"：此前全局样式里有一条 A4 的页面规则，把各谱书自己的 A3 纸张设置覆盖掉了，导致内容按 A3 排版、却印在 A4 纸上。现已改为各谱书自行指定纸张。</p>
                <p>· <b>传记页顶部对齐</b>：PDF 里正文起点与预览对齐（相差 1.8mm）。</p>
                <p>· <b>谱头（谱序/族规/祖训）页上下边距对称</b>（6.7 / 6.7mm），不再上小下大。</p>
                <p><b>牒记式成员表：</b></p>
                <p>· <b>新增"续格"</b>：某位成员资料过长、一格放不下时，<b>自动在其右侧新增一格继续排</b>（名字位显示灰色「续」），<b>不再裁掉剩余文字</b>；正文按句断行，不会断在句子中间。</p>
                <p>· <b>容量调整</b>：一格可容纳 92 字（竖版），常见成员资料一格即可放下。</p>
                <p>· <b>资料字距 +1px</b>，避免部分字黏连。</p>
                <p>· <b>关系格</b>（某某公 长子/次子/始祖）字号加大（竖版 14px、横版 12px）。</p>
                <p>· <b>列标题</b>（外纪 / 李氏家谱）<b>居中并加大到 23px</b>。</p>
                <p>· <b>横版凡例减少一列</b>，格子均分加宽，不再拥挤。</p>
                <p><b>支谱并入宗谱：</b></p>
                <p>· 不再单独提供"支谱"入口；改为<b>顶部选择房支</b>，选择"长房"时封面自动变为「<b>长房支谱</b>」，选"宗谱（全部）"则为「<b>李氏家谱</b>」，内容也随之筛选。</p>
                <p><b>其它：</b></p>
                <p>· 修正部分环境下<b>接口 401</b>（请求未携带登录凭证）导致页面空白的问题。</p>
                <p>· 谱书内页字号统一（大事记 / 谱序族规祖训 / 凡例 / 成员资料均 18px 档）。</p>
              </div>
            </div>
          </el-collapse-item>
<el-collapse-item name="logStable138">
            <template #title>
              <span class="log-title">v1.3.8</span>
              <span class="log-count">2026-10-27 发布</span>
            </template>
            <div class="log-entry">
              <div class="log-entry-head">
                <span class="log-ver">v1.3.8</span>
                <span class="log-date">2026-10-27</span>
              </div>
              <div class="log-entry-body">
                <p><b>牒记式谱书：大事记 / 谱序 / 族规 / 祖训 改用「竖格」版式：</b></p>
                <p>· <b>一格一排字</b>：单页等宽格子（CSS Grid + 从右往左），格子宽度完全一致、<b>排满整页</b>；</p>
                <p>· <b>页标题放在最右一格、竖排居中</b>（"家族大事记" / "谱序 · 创修本支谱序"）；</p>
                <p>· <b>每列按格子实际高度动态排满</b>（竖排字步进 = 字号 18px，不是行高），首列填充 95% 以上；</p>
                <p>· <b>字在格子内左右居中</b>（竖排下用 flex 居中，修正了原来偏右的问题）；</p>
                <p>· <b>分类标题</b>：谱序/族规/祖训 页显示「<b>分类 · 文章标题</b>」（如「谱序 · 创修本支谱序」）；</p>
                <p>· <b>大事记条目头合并为一格</b>：「<b>标题 · 日期</b>」（如「创修本支谱序 · 2026-08-25」），省下的格子给正文；</p>
                <p>· <b>字号统一 18px</b>：大事记 / 谱序族规祖训 / 凡例 / 成员资料 全部统一，观感一致；</p>
                <p>· <b>支谱与牒记式竖版完全同步</b>（A4 与 A3 只差纸张，版式一致）。</p>
                <p><b>垂珠体五世圖修订：</b></p>
                <p>· 成员格<b>避开中间书芯</b>（不再压中书芯被裁）；每半幅最多 3 人；</p>
                <p>· <b>本名与诸子名统一 28px</b>；<b>去掉世代副标</b>（"行一/行二"不是排行，无意义）；</p>
                <p>· <b>第 4 世与第 5 世同页</b>（原来是各占一页，断开图）；</p>
                <p>· 字号恢复为定稿大小，右边不再超出纸张。</p>
                <p><b>祠宇志 / 茔墓志 手机端适配：</b></p>
                <p>· 抽屉全宽、表单单列、子表改卡片、图片一行 3 张、坐标按钮换行不超屏；</p>
                <p>· 地图选点对话框近全屏、底部按钮吸底；保存/取消底部吸底操作条（不被键盘遮挡）；输入框 16px 防 iOS 缩放。</p>
                <p><b>其它：</b></p>
                <p>· 谱书目錄页去掉底部说明行与分隔线（三本都改）；</p>
                <p>· 谱头文化页与大事记共用同一套容器与 CSS，不再出现"格子比页框宽"的溢出。</p>
              </div>
            </div>
          </el-collapse-item>
<el-collapse-item name="logStable137">
            <template #title>
              <span class="log-title">v1.3.7</span>
              <span class="log-count">2026-10-25 发布</span>
            </template>
            <div class="log-entry">
              <div class="log-entry-head">
                <span class="log-ver">v1.3.7</span>
                <span class="log-date">2026-10-25</span>
              </div>
              <div class="log-entry-body">
                <p><b>新增「🏯 祠宇志」：</b></p>
                <p>· 记录现实中的祠堂/祖厝：名称、类型（祖厝/祠堂/家庙）、地址、坐标、<b>始建与重修历史（可多条）</b>、建筑规制、<b>楹联匾额（可多条）</b>、神位排布、祭祀仪轨、<b>族产公田（可多条）</b>、<b>实景图（多张）+ 平面布局图</b></p>
                <p>· <b>楹联匾额分「匾额 / 对联 / 其他」</b>：选匾额给匾额位置提示、选对联给对联位置提示，位置仍可自由填</p>
                <p>· <b>🗺️ 地图选点</b>：编辑页点「地图选点」弹出在线地图（默认高德 GCJ-02），点地图或拖图钉即可回填经纬度；已有坐标自动定位；底图加载失败有提示，手填/粘贴链接两条老路保留</p>
                <p>· 支持<b>粘贴高德/百度/腾讯地图链接自动提取坐标</b>（百度腾讯 lat,lng 自动换序）</p>
                <p><b>新增「🪦 茔墓志」：</b></p>
                <p>· 每座祖茔独立条目：墓主（可关联成员）、<b>世次</b>、生卒、<b>墓碑文</b>、<b>山向分金</b>、地理位置、<b>经纬度</b>、<b>墓碑照片（多张）+ 墓域四至图</b>，列表按<b>世次 → 姓名</b>排序</p>
                <p>· 详情页「地图查看」「<b>祭扫导航</b>」一键跳高德（<b>未填坐标时置灰并提示</b>），方便后代寻根谒祖</p>
                <p><b>谱书新增两志（四本谱书都有，含支谱）：</b></p>
                <p>· 位置：名人傳記之后、五世一圖/世系之前；每祠宇 = 正文 + 实景图版 + 平面图版，每茔墓 1 页</p>
                <p>· <b>匾额横排单行加细框</b>、<b>对联竖排双行（上联在右、下联在左）</b>；图片排法按书的版向：<b>横版左右并排、竖版上下排</b>，竖版茔墓文字在上、图片在下</p>
                <p>· 目錄自动加两行（起始页 + 页数），与正文实际页 0 偏差；<b>没有数据时不生成空页</b></p>
                <p><b>名人傳記按辈分排行排序：</b></p>
                <p>· 排序改为 <b>世代 → 排行 → 生年 → id</b>（排行能解析"行一/行十二/长子/十五/廿三"）；<b>配（配偶）随夫</b>；关联不到成员的排最后；<b>原来人工填的"排序年份"不再越辈插队</b></p>
                <p>· <b>支谱新增「名人傳記」板块</b>（只显示本支成员的传记，配偶随夫判断）</p>
                <p><b>支谱：</b></p>
                <p>· 支谱成员范围<b>跟随顶部「含下级」开关</b>（开启时含全部下级支，按 url 的房支为根），世系与名人傳記一并跟随，工具栏显示「含下级支 / 仅本支」</p>
                <p><b>菜单重新归组：</b></p>
                <p>· <b>🏮 祠墓</b>：🛕 祠堂（电子祠堂/上香，原样）/ 🏯 祠宇志 / 🪦 茔墓志</p>
                <p>· <b>🏛️ 谱头文化</b>：🗓️ 家族大事记 / ✒️ 谱序 / 👤 名人传记（父子菜单、默认折叠，进子页自动展开；旧链接自动兼容）</p>
                <p><b>精简：</b></p>
                <p>· 删除重复的旧「谱头文化 → 祠墓」tab（<b>数据全部保留</b>，可在祠宇志/茔墓志顶部一键归入）；删除「欧式五世表」与旧版「牒记式」（竖排/横版牒记式保留）；旧链接重定向不白屏</p>
                <p><b>垂珠体封面：</b></p>
                <p>· 堂号移到家族名<b>上方</b>、两者水平居中、间距 20mm；家族名加大到 <b>180px</b>、堂号 64px</p>
                <p><b>修复：</b></p>
                <p>· 老库启动自动迁移（祠宇/茔墓相关表与字段），旧数据不丢；备份/恢复覆盖新表；打印新增页全部输出、目錄 0 偏差</p>
              </div>
            </div>
          </el-collapse-item>
<el-collapse-item name="logStable136">
          <template #title>
            <span class="log-title">v1.3.6</span>
            <span class="log-count">2026-10-19 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.3.6</span>
              <span class="log-date">2026-10-19</span>
            </div>
            <div class="log-entry-body">
              <p><b>首页「未来 30 天」：</b></p>
              <p>· 首页顶部一条「📅 未来 30 天」：列出 30 天内的 <b>传统节日 / 族人生日 / 忌日</b>（按日期排序，显示"今天/还有 N 天"）；生日忌日可点进成员或配偶详情，节日不跳转；窄屏横向滚动</p>
              <p>· <b>游客不可见</b>：未登录不请求不渲染，接口一律 401</p>
              <p>· <b>跟随顶部房支筛选</b>：切「宗谱／某房支」即刷新，条上标注当前范围（如"长房（含下级）"）</p>
              <p>· <b>传统节日「不固定」可临时填日期</b>：填了显示具体日期，<b>该日期过后自动回到"不固定"</b>，不跨年沿用</p>
              <p><b>登录后默认按账号房支：</b></p>
              <p>· 分配了房支的账号登录后，顶部直接显示自己的房支（原来是永远"宗谱"）；手动切换过会记住；换账号不残留</p>
              <p><b>Excel / CSV 导入导出：</b></p>
              <p>· 成员列表新增「导出」：当前房支 / 当前筛选 / 全部，xlsx 或 csv，全中文列名；「导入」三步向导（选文件→预览校验→结果），失败逐行给原因并可下载失败清单，支持整体回滚或跳过出错行</p>
              <p><b>🩺 数据体检（系统设置下）：</b></p>
              <p>· 六类检查：父子年龄倒挂 / 生卒矛盾 / 重复人物 / 缺失必填 / 配偶关系异常 / 世代断档；每条可一键跳转修正</p>
              <p><b>来源引证 + 谱书脚注：</b></p>
              <p>· 成员可录入<b>多条资料来源</b>（族谱旧谱/口述/墓碑/家书/方志等类型）；谱书<b>行传页</b>会在该人条目末尾以小字脚注注明「据：…」，无来源则不留空</p>
              <p><b>谱书版式（据真实族谱参考图集补齐）：</b></p>
              <p>· 新增 <b>内封页</b>（竖排书名 + 双线框 + 「X堂藏板」+ 岁次/修次，不占正文页码）、<b>牌记页</b>（全书末页）、<b>五服图</b>（纯矢量，25 格 + 五服服制，进目錄占页码）</p>
              <p>· 行传条首加「<b>某公第几子</b>」小字（固定两排行距、统一字号、三本谱书一致）</p>
              <p>· 垂珠体行传改为<b>统一格 + 统一字号 + 跨格续写</b>：格子一律等宽，一人写满一格直接接写下一格（无续接标记），短文占一格留白</p>
              <p>· 封面：<b>堂号与家族名</b>按要求排布（横版上下排、竖版家族名居中且堂号紧挨右侧并与家族名中线对齐），家族名字号明显加大</p>
              <p><b>世系图分页打印：</b></p>
              <p>· 宝塔式/垂吊/垂丝/世代分层四图支持「🖨️ 分页打印」：A3/A4、横竖、<b>整张一页 / 分页瓦片 / 卷轴</b>；<b>每一页按整张纸切、最后一页往回借内容</b>（保证每页都满）；比例可选自动（推荐）/ 50%/75%/100% 等；<b>索引页固定第 1 张</b>；切线只落在节点空隙、重叠 12mm、跨缝连线两页可见；屏幕预览完全不变</p>
              <p><b>隐私与权限：</b></p>
              <p>· 新增「<b>取消隐私 · 全公开</b>」总开关（默认关闭；打开后所有成员信息对所有登录用户完全公开，不放开操作权限与房支隔离），并新增「<b>恢复默认设置</b>」按钮（先回填表单、点保存才生效）</p>
              <p><b>热力图：</b></p>
              <p>· 「现居地」模式对未迁出者<b>自动落回祖籍</b>显示；新增「<b>全部</b>」模式，祖籍/现居地/未迁出/葬地<b>四色区分 + 图例</b>，图例点选可只看某类（真正过滤渲染）</p>
              <p><b>修复：</b></p>
              <p>· 谱书打印<b>末尾空白页</b>（命名页收尾 + 牌記超印区）、支谱字辈页预览过长/牌記跨页、牒记式竖版封面 cover-meta 打印超框</p>
              <p>· 垂珠体 6 字关系字原本会被裁字</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logStable135">
          <template #title>
            <span class="log-title">v1.3.5</span>
            <span class="log-count">2026-10-13 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.3.5</span>
              <span class="log-date">2026-10-13</span>
            </div>
            <div class="log-entry-body">
              <p><b>家族多媒体（新增）：</b></p>
              <p>· 左侧菜单新增「<b>🖼️ 家族多媒体</b>」：上传与浏览<b>不属于某位成员</b>的公共素材（祠堂照片、传统节日图片/视频等），支持<b>图片与视频</b>、分类（家族多媒体 / 祠堂 / 传统节日 / 族务活动 / 其他，可自定义）、标题与说明、删除、按分类筛选</p>
              <p>· <b>上传即生成缩略图</b>（120 / 240 / 400 三档，落盘缓存）→ 列表与选择弹窗秒开（实测首屏传输省 <b>99%</b>）；<b>书页与打印仍用原图</b>（保证清晰度）</p>
              <p>· <b>归属跟随顶部「宗谱／房支」选择器</b>：选总谱则素材属于总谱、选某房支则属于该房支（<b>不用手填</b>）</p>
              <p>· <b>可见范围按权限</b>（后端强制）：超级管理员不限；<b>未分配房支只看总谱素材</b>；已分配则看本房支（勾「含下级」含下级房支）；<b>没有权限的来源在界面上不显示</b>（支谱里不显示「总谱素材」）</p>
              <p>· 删除时会<b>一并清理源文件与缩略图</b>（先做引用检查，仍被引用则只删记录）；<b>重复上传不再报错</b>而是复用；另提供<b>「清理孤儿文件」</b>入口</p>
              <p><b>谱书「图片展示」板块（三个谱书）：</b></p>
              <p>· 位置固定在 <b>名人傳記之后</b>、谱头/世系之前；<b>不选图片不生成任何页</b></p>
              <p>· <b>多条目</b>：可新增/复制/删除/排序多个条目，每条目独立选图、独立布局、独立图说、可填标题；<b>总谱三本共用同一套条目</b>（支谱各自独立）</p>
              <p>· 布局：<b>单张 / 2 / 4 / 6 / 9 宫格</b>；<b>每页按图片数量固定格子大小</b>（同页格子完全等大、图片等比不裁切）；横版「2 张」左右并排</p>
              <p>· 目錄里<b>每个条目单独一行</b>（张数 / 共几页 / 起始页），<b>点击可跳转</b></p>
              <p><b>谱书封面：</b></p>
              <p>· 新增可配置<b>修次</b>（創修 / 初修 / 二修… / 重修 / 續修 / 自定义），全局生效；可勾选「<b>题名含修次</b>」→ 谱名如「<b>李氏創修族譜</b>」</p>
              <p>· <b>题签自适应</b>：书名 <b>6 字以内不缩字号</b>（框自动长高），<b>超过 6 字才缩小</b>，堂号与卷次始终完整可见；<b>堂号 / 郡望 / 祖籍有值才显示</b>（空值不留空行）</p>
              <p><b>手机端预览：</b></p>
              <p>· 放大时<b>纸张与内容一起放大</b>，可<b>左右上下滚出屏幕</b>查看；进页面默认「适应屏幕」，<b>只有刷新</b>才回到适应屏幕</p>
              <p><b>本次修掉的问题：</b></p>
              <p>· 修复「保存修次会把堂号/郡望/祖籍一起清空」（设置接口改为按字段更新）</p>
              <p>· 修复家族多媒体「删除不删源文件、重新上传报错」（路径拼写错误 + 查重改为复用）</p>
              <p>· 修复垂珠体图片页跑到全书最后（现紧跟名人傳記）</p>
              <p>· 修复竖版牒记式部分传记页被内容撑宽（书页宽度固定，长串/缺图/极端肖像都不再撑宽）</p>
              <p>· 修复横版牒记式屏幕预览纸张比例（现与 A3 横版一致；打印保持不变）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logStable134">
          <template #title>
            <span class="log-title">v1.3.4</span>
            <span class="log-count">2026-10-11 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.3.4</span>
              <span class="log-date">2026-10-11</span>
            </div>
            <div class="log-entry-body">
              <p><b>苏氏垂珠体 · 谱头文化（新增）：</b></p>
              <p>· 新增「谱头文化」页，收录 谱序 / 凡例 / 祖训 / 族规（按序）与字辈派语；排在 <b>目錄之后、名人傳記之后、五世一圖之前</b></p>
              <p>· 版式：<b>从右往左</b>依次为「<b>標題列</b> → <b>正文列</b>」；<b>每一列＝一行竖排文字</b>（列内不折行），写满一列自动换左边一列，续列标「（續）」</p>
              <p>· <b>一种分类独占起页</b>（不与其它分类同页混排）；分类名只写在<b>书耳</b>，并显示<b>该分类内</b>的 n／N（如 凡例 1／2）</p>
              <p>· 每页固定画满 <b>18 格</b>：没有内容的格子也照常画框，<b>不留白</b>；<b>绝不裁字</b>（容量实测、宁可多一页）</p>
              <p>· 目錄同步：谱头各分类<b>逐行列出</b>并标注起始页（页码与册内实际逐行一致）</p>
              <p><b>谱书预览缩放（垂珠体 / 竖版牒记式 / 横版牒记式）：</b></p>
              <p>· 规则：<b>实际缩放 = min(按屏幕宽度自动适应的值, 你调过的值)</b>；首次默认 垂珠体 <b>50%</b>、牒记式 <b>70%</b></p>
              <p>· <b>手动缩放不受屏幕限制</b>（可放大到超出屏幕）；<b>只有刷新</b>才回到「适应屏幕」；切换页面不会重置</p>
              <p>· <b>不做手机/电脑判断</b>：窄屏上自动值本来就小，行为与以往一致</p>
              <p><b>世代分层图 · 标记字扩充：</b></p>
              <p>· 新增 <b>迁</b>（在迁徙记录名单里，只标该支最顶端）／<b>祧</b>（兼祧）／<b>夭</b>（逝世时未满 14 周岁；缺月缺日按年份差且边界也算）／<b>养</b>（养子）</p>
              <p>· 原「入」只保留<b>嗣子</b>；优先级：<b>失 &gt; 夭 &gt; 入 &gt; 养 &gt; 出 &gt; 赘 &gt; 祧 &gt; 迁 &gt; 止</b>（每节点只显示一个）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logStable133">
          <template #title>
            <span class="log-title">v1.3.3</span>
            <span class="log-count">2026-10-10 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.3.3</span>
              <span class="log-date">2026-10-10</span>
            </div>
            <div class="log-entry-body">
              <p><b>世系可视化 · 世代分层图（新增）：</b></p>
              <p>· 一行一辈、整体靠左排（参照苏式垂丝图镜像：长子居左主干、父对齐其长子），世系连线两处折角、从姓名下沿接线、不压姓名</p>
              <p>· 圈内不放排行：族内男为空白（有标记时圈内显示 <b>失 / 入 / 出 / 赘 / 止</b>），族内女为「女」、配偶为「配」（实心深红、文字反白）</p>
              <p>· 支持房支筛选（含下级）、搜索定位、滚轮缩放、拖拽平移、一键导出；一行一辈的世代标签随图平移缩放</p>
              <p><b>各图视图导出统一：</b></p>
              <p>· 宝塔式 / 苏式垂吊图 / 苏式垂丝图 / 世代分层图 / 谱书封面 / 功德碑 均改为「导出」下拉：<b>PNG（高清 300DPI）/ SVG（矢量图）/ SVG（内嵌字体）/ PDF（打印用）</b>；宝塔式另有 <b>SVG（内嵌头像）</b> 与 <b>SVG（内嵌头像+字体）</b></p>
              <p>· PNG 成功提示会写明实际 DPI；受画布上限自动降倍率时明确说明</p>
              <p><b>字体：</b></p>
              <p>· 四个树图新增<b>字体选择</b>（6 种内置字体 + 服务器上传字体）与<b>上传字体</b>按钮（理事长及以上），换字体即时生效、刷新后保留、四个视图共用同一选择</p>
              <p>· 选中上传字体后，PNG / SVG / PDF 三档导出会<b>自动内嵌该字体</b>；矢量 SVG 还可内嵌头像（任何机器打开都能显示）</p>
              <p>· 字体内嵌采用<b>子集化</b>：只嵌图里用到的字（百余字约 20KB），换机器字形完全一致；woff2 / otf 无法子集内嵌时会明确提示请另传 TTF 版本</p>
              <p>· 上传字体<b>查重</b>：内容相同或同名不同内容都会拒绝并给出说明，不再产生「-2 / -3」重复副本；新增<b>删除字体</b>入口；大字体上传超时会自动复查服务器，已保存即按成功处理</p>
              <p><b>备份与还原：</b></p>
              <p>· 修复<b>备份缺数据</b>：打包前统一做数据库检查点（此前主库单文件可能比真实数据少若干行，表现为「传记少一条、账本少一条」）；完整备份现在包含<b>数据库 + 附件 + 上传字体 + 缩略图</b></p>
              <p>· 每个备份包内新增 <b>backup-manifest.json</b>：备份类型、版本、时间、<b>每张表逐表行数</b>、各目录文件数与大小、字体清单 —— 打开包即可核对内容</p>
              <p>· 一键备份新增<b>字体选择</b>：全部备份（默认）/ 选择备份（多选框）/ 不备份字体；选择会被记住</p>
              <p>· <b>还原字体去重</b>：同名同格式的字体跳过不还原（不再产生重复副本）；内容不同只提示不覆盖；不含字体的备份包还原时保留现有字体</p>
              <p>· 修复「重置初始设置」漏清 16 张表的问题；还原后自动逐表自检并回报结果</p>
              <p>· 备份页新增「各类备份包含什么」说明：系统备份仅含程序页面（不含任何族谱数据），GEDCOM 仅含家谱关系（不含传记正文 / 账本账目 / 照片 / 字体）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logStable132">
          <template #title>
            <span class="log-title">v1.3.2</span>
            <span class="log-count">2026-10-07 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.3.2</span>
              <span class="log-date">2026-10-07</span>
            </div>
            <div class="log-entry-body">
              <p><b>数据看板：</b></p>
              <p>· 新增「职业占比」：成员与配偶合并统计，范围可选「在世」（默认）/「全部」，选择会被记住；该范围内<b>所有人</b>都计入，未填职业归入「未知」（灰色扇区），表头显示「在世 N 人，其中未填职业 M 人」</p>
              <p>· 新增「年龄结构」：在世者按出生年份分六档（0-17 / 18-30 / 31-45 / 46-60 / 61-75 / 76 以上），未填出生单列，并给出平均年龄</p>
              <p><b>谱书：</b></p>
              <p>· 新增「目錄」页：竖排牒记式、横版牒记式、苏氏垂珠体三处都有，逐节列出内容与起始页；页码口径为全书页序，封面与目录不参与编号</p>
              <p>· 新增「名人传记」页：<b>一传一页</b>，正文超长时自动续到下一页（页尾有「1／2」页码旗标），<b>字多也不会被裁掉</b>；竖版为「上部横带（左资料 / 右头像）+ 下方竖排正文」，横版为「右侧竖列（上头像 / 下资料）+ 左侧正文」，垂珠体为「肖像与资料在上、标题正文自其下方起排、写满接右半」；每篇仅第 1 页带头像与资料，续页为纯正文；没有头像时用姓名首字占位</p>
              <p>· 传记页排在目录之后：竖排 / 横版牒记式为 封面 → 目錄 → 名人傳記 → 谱序 / 凡例 / 祖训 / 族规 → 字辈 → 世系；苏氏垂珠体为 封面 → 目錄 → 名人傳記 → 五世一圖 → 行傳</p>
              <p>· 垂珠体目录页采用与正文页同一套书耳与版心（书耳为「目錄」，版心有章节名、页码位留空），条目靠左排一列、右侧留白</p>
              <p><b>亲属称谓：</b></p>
              <p>· 大幅扩展：伯父 / 叔叔（分长幼）、堂兄 / 堂弟 / 堂姐 / 堂妹、姑表 · 舅表 · 姨表分列、堂伯叔 / 堂姑 / 表伯叔 / 表姑、曾祖父母 / 高祖父、曾孙 / 玄孙、侄孙 / 堂侄 / 表侄，以及姻亲（伯母婶婶 / 姑父 / 舅母 / 姨父 / 嫂子 / 弟媳 / 姐夫 / 妹夫 / 儿媳 / 女婿 / 岳父母 / 公婆）</p>
              <p><b>AI 修谱助手：</b></p>
              <p>· 新增「AI 生成小传」：在成员 / 配偶的生平条目里，按族谱档案一键生成小传草稿，可选文体（传记体 / 简洁 / 文言）与篇幅（短 / 中 / 长）</p>
              <p>· 新增「AI 生成传记」：在名人传记的新增 / 编辑页，按本页资料 + 关联成员的档案 + 其生平条目生成传记正文</p>
              <p>· 供应商改为通用可选：本地（llama.cpp / Ollama / vLLM）+ 云端（OpenAI / DeepSeek / 通义 / 火山方舟）+ 自定义 OpenAI 兼容端点；支持「测试连接」、最大生成长度与超时可配、可一键清除密钥</p>
              <p>· 两项生成都带日期保护：发送前把日期换成占位符（模型看不到年月），返回后校验并还原，越界年份会被拦截；结果只填在可编辑预览框，点「采用」才写入，保存仍走原有审核流程</p>
              <p><b>其它：</b></p>
              <p>· 新增「本机（127.0.0.1）访问免登录」开关（服务器设置页，<b>默认关闭</b>，仅理事长 / 超级管理员可改）：开启后本机访问无需登录、权限按「族员」只读；已提示反向代理环境下请勿开启</p>
              <p><b>修复：</b></p>
              <p>· 用族谱账号登录知识库时，没登录过族谱的成员会报「账号密码错误」</p>
              <p>· 谱内女性的子女查不到、同母异父漏算、配偶性别写死</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logStable131">
          <template #title>
            <span class="log-title">v1.3.1</span>
            <span class="log-count">2026-10-04 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.3.1</span>
              <span class="log-date">2026-10-04</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「DNA 比对」：成员详情页对已录入 DNA 编码的成员，按亲缘可能性评分排序，匹配结果可一键进行亲子比对</p>
              <p>· 新增「DNA 认亲比对」：成员管理页手动输入外部 DNA 编码（父系 / 母系 / 二倍体），与族内已录入 DNA 的成员全库匹配，用于外地或失散族人回来认亲</p>
              <p>· 内置完整单倍群谱系树：Y-DNA 全量（ISOGG 2019-2020，约 9.6 万节点）与 mtDNA 全量（PhyloTree Build 17，约 1.28 万节点），父系 / 母系分树识别，兼容位点命名（O-M175）与字母数字命名（O2a1b），按「精确 > 同支系（上下游归属）> 前缀」三级匹配</p>
              <p>· 二倍体字段支持直接填写 STR 位点串（位点名 / 数1 / 数2，用 - 分隔，自动解析），可对两成员做亲子比对（孟德尔排除法：0 不符 = 不排除、1 不符 = 存疑、≥2 不符 = 排除）</p>
              <p>· 新增「功德碑」：从「捐款账簿」已审核的捐款一键生成，支持门槛筛选，传统竖排右起排版，可导出碑刻稿</p>
              <p><b>修复：</b></p>
              <p>· 修复单倍群匹配的大小写与位点别名识别（如 O-M175 与 O2a1b 互通、父系与母系同名单倍群不再误判）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logStable130">
          <template #title>
            <span class="log-title">v1.3.0</span>
            <span class="log-count">2026-10-02 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.3.0</span>
              <span class="log-date">2026-10-02</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「🔒 隐私与权限」：在「设置」里按「项目 × 职位」逐项配置——每类信息（世系框架、生卒年龄、生平传记、照片头像、职业学历称号、葬地墓址、配偶信息、籍贯现居地、手机号、配偶手机号、DNA 编码）对每个职位可分别设为「完整 / 打码 / 隐藏」；各项操作（编辑、审核、导出、备份、账号、系统设置、家族知识库、操作日志等）也按职位授权；另有两个总开关「五服近亲分级」「逝世者信息公开」（默认按原有规则设定，可在矩阵内调整）</p>
              <p>· 「用户权限」「隐私与权限」改为按钮弹出，不再常驻设置页，并适配手机</p>
              <p>· 房支管理升级为多级（层数不限；命名用「序数 + 名讳」如「长房 · 廷玉公」；可新增下级、移动、排序）</p>
              <p>· 顶部新增「房支」下拉：选定房支后，成员列表、数据看板、统计、各世系图、生日 / 忌日、热力分布与迁徙、审核中心等数据都只看该房支范围；可勾选「含下级」；选择会记忆在本机</p>
              <p>· 苏氏垂珠体及各世系图均支持按房支过滤（垂珠体以本房支起世，首页关系格标「房祖」）</p>
              <p><b>修复：</b></p>
              <p>· 修复在线更新偶尔卡在「正在解压 100%」不动的问题（更新完成后会自动刷新页面）</p>
              <p>· 修复切换房支后当前页面数据不自动刷新的问题</p>
              <p>· 修复苏氏垂丝图外纪世号显示（以最早的先祖为外纪一世，与其它世系图一致）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev129">
          <template #title>
            <span class="log-title">v1.2.9</span>
            <span class="log-count">2026-09-30 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.9</span>
              <span class="log-date">2026-09-30</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 与「家族知识库」打通：族谱的人物传记、谱头文化（谱序/凡例/族规/祖训）、字辈派语、传统节日会自动同步进知识库，在知识库顶部的「家族谱牒」分类下可只读浏览、搜索；未公开的传记在知识库中标记为「机密」</p>
              <p>· 新增迁徙事件时，「迁出地」自动填入所涉及成员的现居地（未填现居地则用籍贯）；涉及成员现居地不一致时列出来供选择，仍可手工修改</p>
              <p>· 迁徙事件保存时可勾选「把涉及成员的现居地更新为迁入地」（默认勾选）</p>
              <p>· 「迁徙轨迹」改为真实地图：按迁出地/迁入地的实际位置绘制轨迹（同一省内迁徙也能看到路线），并自动缩放到轨迹范围；进入该页默认使用在线底图</p>
              <p>· 轨迹上叠加流动的方向箭头（一串带头、带尾的箭头沿路线滑动，指明迁徙方向）</p>
              <p>· 轨迹上用小人数量表示迁徙人数：人数按数量级分 5 档配色（个/十/百/千/万），小人按人数排成三角队形（第一排 1 个、第二排 2 个、第三排 3 个…）随轨迹前进</p>
              <p>· 地图缩小时小人同步变小，放大时恢复</p>
              <p><b>修复：</b></p>
              <p>· 修复热力分布切换「按籍贯/按现居地/葬地」时，精确热力不立即刷新的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev128">
          <template #title>
            <span class="log-title">v1.2.8</span>
            <span class="log-count">2026-09-29 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.8</span>
              <span class="log-date">2026-09-29</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「无嗣」标记：可标记某成员无子女，标记后不能再为他添加子女，世系图相应标注</p>
              <p>· 新增「迁居失联」标记：标记后该成员不再列入生日与祭日提醒，世系图按老谱惯例标注</p>
              <p>· 世系图统一按老谱惯例标注：已知的生、殁、葬照常显示，缺的部分补「失考」，无子女再缀「子嗣失考」</p>
              <p>· 图形版式（宝塔式、苏式垂吊、垂丝）不显示生卒，只标注失考 / 无嗣；书式版式（牒记式、垂珠体）生卒按农历年号显示</p>
              <p>· 新增「名人传记」模块：为有重大贡献 / 成就的人立传（族内成员、配偶、其他人员均可），支持设定入传标准、理事提交→审核、管理员直接发布；肖像可取自关联人或单独上传</p>
              <p>· 世系图上为有传记的人加「金色卷轴」标记，点击可直接查看传记</p>
              <p>· 传记支持「人物志」谱页样式打印（谱页排版，一人一页）</p>
              <p>· 启动时自动为历史成员补齐农历 / 年号信息（此前只录了公历的会自动补全）</p>
              <p><b>修复：</b></p>
              <p>· 修复「无嗣」「迁居失联」开关只有开关、没有文字说明的问题</p>
              <p>· 修复世系图上失考标注显示不全（被裁切）的问题</p>
              <p>· 修复垂吊图 / 垂丝图加上标注后整排连线被拉长的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev127">
          <template #title>
            <span class="log-title">v1.2.7</span>
            <span class="log-count">2026-09-27 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.7</span>
              <span class="log-date">2026-09-27</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「家族知识库」（传承服务器）：在族谱中可直接进入家族知识库</p>
              <p>· 族谱与知识库登录打通：从族谱进入知识库自动登录，免二次登录</p>
              <p>· 知识库登录支持两种账号来源：族谱账号 / 知识库账号（登录页可切换）</p>
              <p>· 苏氏垂珠体：打印「边距」支持自定义输入（单位 mm，默认 5mm，填 0 即为「满版」）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev126">
          <template #title>
            <span class="log-title">v1.2.6</span>
            <span class="log-count">2026-09-26 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.6</span>
              <span class="log-date">2026-09-26</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「收支公示」功能：家族账目公开，理事提交 → 审核 → 族员可查</p>
              <p>· 支持多账本管理（按年度 / 项目建账），可写说明、设是否「族员可见」</p>
              <p>· 账目明细：收 / 支、金额、分类、日期、摘要、经手人、凭证图片</p>
              <p>· 捐款可关联家族成员，默认实名公示，捐款人可自行转为匿名</p>
              <p>· 新增职位「财务管理」，与理事长 / 超管一同负责账目审核；管理员可把理事加入「理账名单」以开放提交权限</p>
              <p>· 账本可「封账」（永久锁定，不可取消），封账前需先审核完待审账目；未封账账本可随时删除</p>
              <p>· 新增家族资金「家族基金」系统账本：记录资金注入 / 提取，以及各账本启动资金的划拨与封账结余归还，家族资金余额即为该账本余额</p>
              <p>· 新建账本时可从家族资金划拨「启动资金」，封账时结余自动归还</p>
              <p>· 账本列表支持「置顶」；列表顺序为：置顶的排最前 → 新建的在前、旧的在后 → 已封账的排在未封账之后</p>
              <p><b>修复：</b></p>
              <p>· 修复账本列表在部分情况下不显示已建账本的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev125">
          <template #title>
            <span class="log-title">v1.2.5</span>
            <span class="log-count">2026-09-25 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.5</span>
              <span class="log-date">2026-09-25</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 谱书封面增加「横版」切换：勾选后封面按横版重新排布，打印时页面方向也随之切换为横向</p>
              <p>· 横版封面：装订线、题签框、家族名、堂号/卷次、印章、封底牌记均按横版重新排布</p>
              <p>· 封面预览增加缩放控制（缩放滑块 20%~150% + 「适应屏幕」按钮）</p>
              <p>· 封底牌记的「主修」「编纂」可直接在封面设计页填写</p>
              <p><b>修复：</b></p>
              <p>· 修复横版封面整体偏下、各款式印章与年月位置不当的问题</p>
              <p>· 修复竖版题签内家族名、堂号、卷次被挤成多列、显示错乱的问题</p>
              <p>· 修复竖版题签款印章与家族名同处一列的问题</p>
              <p>· 修复古铜函套（横版）年月与边框重叠的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev124">
          <template #title>
            <span class="log-title">v1.2.4</span>
            <span class="log-count">2026-09-24 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.4</span>
              <span class="log-date">2026-09-24</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 离线地图页精简为「导入离线包」+「删除地图包」两件事：不再提供从在线地图批量下载瓦片、智能模式、开发者服务器下载、导出离线包，也取消了「攒离线瓦片」（以上均涉及批量获取地图数据，不符合地图服务条款）</p>
              <p>· 在线底图仍然保留（含经服务器中转、免 Key 的高德/Esri 底图源）</p>
              <p><b>修复：</b></p>
              <p>· 修复天地图「服务端」Key 额度用尽时在线底图不再显示的问题（现在会自动切换为「浏览器直连」继续显示）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev123">
          <template #title>
            <span class="log-title">v1.2.3</span>
            <span class="log-count">2026-09-24 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.3</span>
              <span class="log-date">2026-09-24</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 修复通过 GEDCOM 备份文件导入的成员全部落在「第一世」、且没有父母关系的问题（现在会按备份文件中的家族关系还原成员的父母与世代）</p>
              <p>· 修复重复导入同一份 GEDCOM 备份时会产生重复成员的问题（现在同名成员会自动跳过，不会重复添加）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev122">
          <template #title>
            <span class="log-title">v1.2.2</span>
            <span class="log-count">2026-09-24 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.2</span>
              <span class="log-date">2026-09-24</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 谱书封面新增「导出图片」：封面、封底可分别导出高清图片（A4 竖版 300 DPI、满版无白边，适合打印机做无边距图片打印）和矢量 SVG（放大不失真，可交印刷厂）</p>
              <p><b>修复：</b></p>
              <p>· 修复谱书封面与封底的装订线未镜像的问题（封底装订线现在与封面左右对称）</p>
              <p>· 修复使用说明页面把格式标签（加粗、行内代码等）当纯文本直接显示的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev121">
          <template #title>
            <span class="log-title">v1.2.1</span>
            <span class="log-count">2026-09-23 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.1</span>
              <span class="log-date">2026-09-23</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 备份管理新增「带账号密码的 GEDCOM 备份」（仅理事长 / 超级管理员可见）：备份文件携带成员账号与密码，恢复时已有账号保留原密码、不会重置，也不会还原超级管理员身份；每日自动备份同时保留一份带账号的备份</p>
              <p>· 离线地图新增「导出离线包（下载文件）」与「上传导入离线包」：导出的包可以发给其他家族系统，对方在页面里直接上传即可使用</p>
              <p>· 离线地图新增「智能模式」：按族谱资料中的地名自动规划下载范围（全国 + 相关省市，祖籍地 / 相关县市做街道级），可选全国清晰度（缩放 3–9 / 3–10 / 3–11 级）、街道级范围（不要街道级 / 小范围 / 中范围 / 大范围 / 超大范围）与下载源；按天地图配额分批次后台下载，额度用尽次日自动继续，事先提示预计容量、预计耗时与数据盘可用空间</p>
              <p>· 在线底图的底图源改为国内可直接使用的免 Key 源（高德街道图、高德卫星影像、Esri 卫星影像）与天地图（需 Key）；级别统一按「缩放 x–y 级」显示；卫星底图放到最大不会再出现「Map data not yet available」</p>
              <p>· 在线底图新增「Key 走法」：浏览器直连（默认，速度最快、不占用服务器流量）或经本系统服务器中转（天地图 Key 不下发到浏览器）</p>
              <p>· 在线底图（中转）可开启「攒离线瓦片」：浏览过的区域自动存进离线地图的「浏览缓存」包，可作为离线底图使用，断网或天地图额度用尽也能看</p>
              <p>· 离线地图新增「浏览缓存」包：显示浏览在线地图时攒下的瓦片，可以作为离线底图直接选用</p>
              <p>· 天地图「服务端」Key 与「浏览器端」Key 分开配置：离线下载与服务器中转使用服务端 Key，浏览器直连使用浏览器端 Key</p>
              <p>· 使用说明补充了「地图与地域分析」「迁徙事件」「GEDCOM 备份与导入」等近期功能</p>
              <p><b>修复：</b></p>
              <p>· 修复接口会返回完整天地图 Key 的问题：页面上现在只显示脱敏值（如 eb55****0133），保存时留空表示不修改</p>
              <p>· 修复「浏览缓存」包可能造成服务卡死的问题</p>
              <p>· 修复智能模式在未开始下载时档位列表为空的问题</p>
              <p>· 修复智能模式「不要街道级」档体积比更大档还大的问题</p>
              <p>· 浏览缓存与智能模式已下载的瓦片互相认账：同一张瓦片不会重复下载，不重复消耗天地图额度</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev120">
          <template #title>
            <span class="log-title">v1.2.0</span>
            <span class="log-count">2026-09-23 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.2.0</span>
              <span class="log-date">2026-09-23</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 热力图「地域分布」新增「在线底图（天地图）」：在页面内配置天地图「浏览器端」Key（保存于服务器、全家族成员通用），无需下载离线底图包即可在线加载底图，支持矢量 / 影像 / 地形三种图层</p>
              <p>· 成员账号：新增成员时按姓名自动生成 12 位初始密码（账号为成员姓名），新增成功后弹窗显示、可复制；成员首次登录时才自动创建账号，并强制修改密码</p>
              <p>· 重置密码改为系统随机生成 12 位（不再固定为默认密码）</p>
              <p>· GEDCOM 批量导入完成后弹出成员初始密码一览（姓名 + 初始密码），支持一键复制全部</p>
              <p><b>修复：</b></p>
              <p>· 修复「在线底图设置」按钮不显示的问题（角色判断错误）</p>
              <p>· 在线底图瓦片加载失败时给出提示（可能为天地图 Key 额度用尽）</p>
              <p>· 修复未创建账号的成员首次登录可用任意密码（如 123456）自动建号登录的问题，现必须使用按姓名生成的初始密码</p>
              <p>· 修复理事长可越权重置超级管理员密码的问题</p>
              <p>· 修复 GEDCOM 批量导入失败、一直无法导入的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev116">
          <template #title>
            <span class="log-title">v1.1.6</span>
            <span class="log-count">2026-09-22 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.1.6</span>
              <span class="log-date">2026-09-22</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「离线地图」页面：可下载天地图离线底图包（全国概览 / 省市级 / 市县级 / 街道级），显示预计瓦片数与下载进度</p>
              <p>· 热力图「地域分布」新增底图选择：可使用已下载的离线底图包作为地图底图，拖动缩放流畅，并可点击省份下钻</p>
              <p>· 已授权用户可用「开发者服务器」下载源一键下载预打包好的离线底图包；未授权用户使用天地图官方下载（需自行申请天地图 Key）</p>
              <p>· 离线底图包支持导出，导出文件放到服务器后即可供其他家族系统下载分发</p>
              <p>· 天地图下载额度用尽时自动暂停，次日自动继续（已下载部分保留，无需重新下载）</p>
              <p>· 手机端热力图高度自适应，自动铺满屏幕</p>
              <p>· 热力图「地域分布」最大放大倍数提高到 30 倍</p>
              <p>· 「房支」列表增加「序号」列（按创建顺序编号）</p>
              <p>· 「迁徙事件」增加新增入口；表单改进：房支改为下拉选择，涉及成员支持搜索、筛选与全选</p>
              <p><b>修复：</b></p>
              <p>· 修复热力图地图有时空白不显示的问题</p>
              <p>· 修复热力图第一次进入不显示、切换一次页签后才出现的问题</p>
              <p>· 修复精确热力落点位置偏移（点画在地图外）及光晕未对准落点的问题</p>
              <p>· 修复手机端双指缩放手感异常（一动就过度缩放、无法放大）的问题，电脑端与手机端灵敏度分开设置</p>
              <p>· 修复手机端热力图缩放卡顿的问题</p>
              <p>· 修复离线底图拖动/缩放与地图不同步、瓦片错位或被裁剪、重影的问题</p>
              <p>· 修复从「开发者服务器」下载离线底图包失败的问题</p>
              <p>· 修复「迁徙事件」页新增按钮有时不显示的问题（权限判断错误）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev115">
          <template #title>
            <span class="log-title">v1.1.5</span>
            <span class="log-count">2026-09-21 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.1.5</span>
              <span class="log-date">2026-09-21</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「谱书封面」页面（世系可视化菜单下）：提供 6 款封面样式（靛蓝布面·经典线装 / 藏青烫金 / 朱红缎面 / 墨绿布面 / 古铜函套 / 米黄仿古宣纸）与 3 款封底（印章 / 牌记 / 纯素），可切换样式、填写卷次、打印导出 PDF</p>
              <p>· 封面支持「无边距（满版）」打印</p>
              <p>· 封面支持「镜像（装订在左）」</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev114">
          <template #title>
            <span class="log-title">v1.1.4</span>
            <span class="log-count">2026-09-21 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.1.4</span>
              <span class="log-date">2026-09-21</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「传统日程」页面（数据看板菜单下、祭日与通讯录之间）：按农历月日展示家族传统活动（祭祖、团拜等），理事及以上可增删改</p>
              <p>· 家族设置新增「衍派」字段（郡望下方，闽台常用），选填</p>
              <p>· 详情页「添加子女」改用新增成员同款弹窗（完整字段），替代原先简化版</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev113">
          <template #title>
            <span class="log-title">v1.1.3</span>
            <span class="log-count">2026-09-18 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.1.3</span>
              <span class="log-date">2026-09-18</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 成员新增「DNA 编码」：单倍体 / 二倍体 / 线粒体 三项，一个框一种；超级管理员与理事长可见完整编码，其余角色打码</p>
              <p>· 配偶新增「手机号码」与「DNA 编码」（单倍体/二倍体/线粒体），可见范围与成员一致</p>
              <p>· 角色「家主」更名为「理事长」（权限不变，历史数据自动迁移）</p>
              <p>· 新增「通讯录」页面（数据看板菜单下、祭日之后）：汇总已填写手机号的族人，支持搜索、房支筛选与分页，敏感字段按隐私规则打码</p>
              <p>· 左侧菜单补齐图标（数据看板子项、世系可视化系列、家族设置、备份管理、版本更新等）</p>
              <p>· Windows 版更新/还原后自动重启：替换或还原完成后自动以新窗口重新启动服务</p>
              <p><b>修复：</b></p>
              <p>· 理事提交的修改经审核通过后，手机号/DNA 等敏感字段未加密入库的问题</p>
              <p>· Windows 数据还原失败（文件被占用）的问题</p>
              <p>· Windows 更新/还原后不自动重启的问题</p>
              <p>· DNA 编码标签换行显示的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev112">
          <template #title>
            <span class="log-title">v1.1.2</span>
            <span class="log-count">2026-09-17 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.1.2</span>
              <span class="log-date">2026-09-17</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 备份与还原新增「上传备份还原」：直接上传备份文件即可还原数据（还原前自动留底）</p>
              <p>· 苏氏垂珠体新增「显示女儿」开关（家族设置），招赘续房的女儿仍保留在图谱中</p>
              <p>· 苏氏垂珠体新增装订线开关（活页孔预留）与预览缩放（工具栏 − / 百分比 / ＋，点百分比恢复自动适应）</p>
              <p>· 打印输出优化：牒记式打印页边距、竖版装订线显示与内容铺满 A3；垂珠体打印四周增加留白</p>
              <p><b>修复：</b></p>
              <p>· 牒记式竖版预览不显示装订线的问题</p>
              <p>· 牒记式打印留白过大、内容未铺满、开装订后页面底部多出空白的问题</p>
              <p>· 牒记式竖版打印底部被裁的问题</p>
              <p>· 苏氏垂珠体打印内容被打印机裁边的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev111">
          <template #title>
            <span class="log-title">v1.1.1</span>
            <span class="log-count">2026-09-17 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.1.1</span>
              <span class="log-date">2026-09-17</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 苏氏垂珠体按《沙美陳氏族譜》式样重做：世代横带分组、成员自右向左排列、版心居中，成员过多时自动续页</p>
              <p>· 新增行传（牒记式）：一人一竖列、自右向左阅读，四格结构（世代 / 关系 / 名 / 生卒生平），配偶列与「生子几」列依次排列</p>
              <p>· 谱书新增封面、谱序、凡例、族规、祖训、字辈派语等版面，页码全书连续</p>
              <p>· 「生子N」名下排列诸子名，点击页码可跳转到对应行传页</p>
              <p>· 生卒日期改用农历（年号 + 干支 + 月日），图形版面用简式</p>
              <p>· 设置新增字号（初号～八号）与封面 / 姓名 / 正文倍率分别可调；每页人数、每行字数随字号自适应</p>
              <p>· 在线更新增加下载进度、解压中 / 重启中提示，完成后自动刷新页面</p>
              <p>· Windows 版一键更新恢复为下载对应平台（Windows）更新包</p>
              <p><b>修复：</b></p>
              <p>· 长页（谱序 / 祖训 / 族规）正文下方大片空白、内容框过小的问题</p>
              <p>· 族规 / 祖训 / 谱系页面文字底部被裁、双数排被切一半的问题</p>
              <p>· 凡例页与成员页列宽不一致、大字号下一页只放一人的问题</p>
              <p>· 带标题页面页码消失、大字号下标题与正文大小失衡的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev110">
          <template #title>
            <span class="log-title">v1.1.0</span>
            <span class="log-count">2026-09-16 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.1.0</span>
              <span class="log-date">2026-09-16</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 支持 ARM 设备在线更新（自动区分 x86 / ARM 架构，匹配对应平台更新包）</p>
              <p><b>修复：</b></p>
              <p>· 启动横幅显示的版本号错误的问题</p>
              <p>· 设置密码超过 72 字节时修改失败且无提示的问题</p>
              <p>· Windows 版在线更新时备份旧版本失败的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev105">
          <template #title>
            <span class="log-title">v1.0.5</span>
            <span class="log-count">2026-09-16 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.0.5</span>
              <span class="log-date">2026-09-16</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 视频缩略图：上传视频时自动提取缩略图（连接 ffmpeg）</p>
              <p>· 新增「使用说明」页面（系统设置内），按功能模块说明各项功能</p>
              <p>· 管理员首次登录（需修改密码）时，自动弹出使用说明窗口</p>
              <p>· 用户资料卡（右上角头像）新增「使用帮助」入口，点击弹出功能说明（所有登录用户可用）</p>
              <p>· 使用说明按角色分级：权限分级、默认密码等管理功能说明仅管理员/家主可见</p>
              <p><b>修复：</b></p>
              <p>· 祠堂自由视角（第三视角）点击牌位弹出的成员面板，改为与第一人称视角一致正面朝向自己</p>
              <p style="color: #e6a23c;"><b>⚠️ 若从 v1.0.4 及之前版本直接升级，需重建 Docker 镜像</b>（Dockerfile 新增 ffmpeg 依赖 + 构建阶段改用 golang:alpine）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev104">
          <template #title>
            <span class="log-title">v1.0.4</span>
            <span class="log-count">2026-09-15 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.0.4</span>
              <span class="log-date">2026-09-15</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 图腾牌匾字号可调节（家族设置-图腾字号，80~300 滑块）</p>
              <p><b>修复：</b></p>
              <p>· 族员查看世系图（宝塔式/苏氏垂吊图/苏氏垂丝图）时 father_id 被隐私脱敏清零，导致所有成员平铺一排无连线</p>
              <p>· 配偶头像未经隐私脱敏（族员查看五服外成员时配偶头像仍可见）</p>
              <p>· 缩略图生成速度：用高性能 bilinear 缩放替代纯 Go 逐像素插值，提速约 10 倍</p>
              <p>· 缩略图全面化：上传时自动预生成缩略图（图片同步、视频异步）；删除媒体时同步清除缩略图；多媒体列表/头像改用缩略图显示，点击查看原图/原视频</p>
            </div>
          </div>
          </el-collapse-item>
        <el-collapse-item name="logDev103">
          <template #title>
            <span class="log-title">v1.0.3</span>
            <span class="log-count">2026-09-14 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.0.3</span>
              <span class="log-date">2026-09-14</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 祠堂视角重构：第三人称改为自由环绕/旁观飞行视角，右键切换锁定（锁定=FPS 准星+鼠标转视角+左键对准牌位牌匾打开详情；未锁定=左键拖拽转视角），第一人称同样支持右键锁定</p>
              <p>· 新增祠堂建筑（房子）：半开放室内空间，第一视角实心墙、自由视角透明，含前墙大门、屋顶、木柱房梁、灯笼</p>
              <p>· 新增图腾牌匾（堂号，无堂号用姓氏）与自定义图腾（上传图片底图 + 木纹/红/蓝/石纹预设样式 + 字号调节）</p>
              <p>· 新增祖训竖长匾（图腾两侧）与凡例竖长匾（右墙），点击弹出完整内容面板</p>
              <p>· 香炉烟雾粒子、香燃烧变短（一炷香 30 分钟）、上香持久化、香炉计数牌（实时显示总香数，多用户同步）</p>
              <p>· 手机端双指捏合缩放、桌面端滚轮缩放视野</p>
              <p><b>修复：</b></p>
              <p>· 左右移动方向相反</p>
              <p>· 滚轮缩放失效</p>
              <p>· 计数牌显示错误</p>
              <p>· 按空格误触发视角切换按钮</p>
              <p>· 锁定模式下无法关闭详情面板</p>
              <p>· 手机端第三视角（旁观）双指缩放失效</p>
              <p>· 图腾/上香数据加载失败导致场景渲染异常</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev102">
          <template #title>
            <span class="log-title">v1.0.2</span>
            <span class="log-count">2026-09-13 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.0.2</span>
              <span class="log-date">2026-09-13</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 点击牌位改为原地弹出 3D 详情面板：真 3D 木牌、有厚度，固定在祠堂场景中（旋转可见侧面/背面），点面板关闭、点其他牌位切换</p>
              <p>· 供奉牌位增加底座，更贴近真实牌位造型</p>
              <p>· 祠堂默认视角改为正面俯视（俯角 15°）</p>
              <p><b>修复：</b></p>
              <p>· 祠堂「数据加载失败」（牌位点击回调缺失）</p>
              <p>· 刷新祠堂后「渲染失败」</p>
              <p>· 左键拖动旋转场景、松开时误触发点击牌位</p>
              <p>· 手机端点击牌位闪一下</p>
              <p>· 详情面板生卒年超出面板范围（改自动换行）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev101">
          <template #title>
            <span class="log-title">v1.0.1</span>
            <span class="log-count">2026-09-13 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.0.1</span>
              <span class="log-date">2026-09-13</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「祠堂」页面：真 3D 渲染，按世代台阶式供桌陈列已故成员牌位，可旋转缩放、点击牌位查看详情</p>
              <p>· 供桌含香炉、贡品果盘、蜡烛等祭器；木牌有厚度与文字纹理，方向光阴影与景深，明暗面分明</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev100">
          <template #title>
            <span class="log-title">v1.0.0</span>
            <span class="log-count">2026-09-13 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v1.0.0</span>
              <span class="log-date">2026-09-13</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 取消免费版 100 人人数上限，改为导出水印（未激活有水印、激活后无水印）</p>
              <p>· 隐私分级：超管/家主/理事全部可见；房支理事仅本房支全部可见；族员仅五服以内可见、以外仅公开信息</p>
              <p><b>修复：</b></p>
              <p>· 角色别名路由重复注册导致启动 panic</p>
              <p>· DB 迁移缺少 phone 列，旧数据库还原后成员列表查询报错</p>
              <p>· 附件删除路径拼写错误，删附件后磁盘文件残留</p>
              <p>· 删除成员时级联清理附件物理文件，避免磁盘垃圾堆积</p>
              <p>· 树状图导出（宝塔式/苏氏垂吊/垂丝图）未加水印，已补上</p>
              <p>· PDF 导出中文水印 btoa 编码报错</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev096">
          <template #title>
            <span class="log-title">v0.9.6</span>
            <span class="log-count">2026-09-12 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.9.6</span>
              <span class="log-date">2026-09-12</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「苏式垂珠体」谱式：竖排世系，长子居右主幹、余子往左，五世一提、按辈分广度优先排布；父子竖线、兄弟横线自动对齐；横版 A3 排版；支持连续翻页与打印，左侧预留装订线</p>
              <p>· 新增「横版牒记式」谱页：每页固定 A3 高度，内容超长自动换列、不裁剪不缩放，排满自动换页</p>
              <p>· 更新页新增「测试版（beta）」更新通道：需输入测试版授权码激活后，才能查看并更新测试版</p>
              <p><b>修复：</b></p>
              <p>· 牒记式竖版/横版多处排版：正文页内框超出外框、页码被裁、封面字号随正文缩放、装订线位置、超长内容换列分页等</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev095">
          <template #title>
            <span class="log-title">v0.9.5</span>
            <span class="log-count">2026-09-12 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.9.5</span>
              <span class="log-date">2026-09-12</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「苏式垂丝图」：长子居右主幹布局，父子竖线、兄弟横线；配偶独立框显示正配/继配；成员框显示辈分；外纪/内纪分离（虚线分隔）并可分别筛选；排行按「行一/行二」识别、缺失时按出生日期排序；外纪按「外纪X世」编号；支持缩放平移、搜索定位、导出高清PNG与矢量SVG</p>
              <p>· 房支管理：新增成员自动继承父亲房支；修改成员房支级联到后代；支持按祖先一键回填未分房成员</p>
              <p>· 世系图新增房支下拉筛选（含「未分房」选项），选具体房支时若存在未归入后代，弹窗可选加入房支/不加入但显示/否</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev094">
          <template #title>
            <span class="log-title">v0.9.4</span>
            <span class="log-count">2026-09-11 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.9.4</span>
              <span class="log-date">2026-09-11</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 授权绑定家族姓氏：激活后姓氏不可更改，未设置姓氏时无法激活</p>
              <p>· 传代树支持删除成员：删除时确认（列出姓名与人数），同步到族谱时再次确认（同步/不同步/取消）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev093">
          <template #title>
            <span class="log-title">v0.9.3</span>
            <span class="log-count">2026-09-10 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.9.3</span>
              <span class="log-date">2026-09-10</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「关于我们」页面：本地部署说明、微信二维码</p>
              <p>· 新增传代树工具（世系图菜单内），数据联动族谱系统（自动生成视图、可编辑后同步回、内纪/外纪切换）</p>
              <p><b>修复：</b></p>
              <p>· 出生只填年份（如「1980」）时享年不显示</p>
              <p>· 在世成员年龄只填年份时不显示</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev092">
          <template #title>
            <span class="log-title">v0.9.2</span>
            <span class="log-count">2026-09-10 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.9.2</span>
              <span class="log-date">2026-09-10</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 增加头像裁剪功能：照片设为头像时可缩放、拖动、裁剪</p>
              <p>· 出生日期支持只填年份（如「1980」），自动显示对应年号及农历月日</p>
              <p>· 生卒日期显示优化：年号后带农历月日（如「共和四十五年七月廿八」）；「生未详」「卒未详」统一为「生年未详」「卒年未详」，两者都未详时显示「生卒未详」</p>
              <p>· 牒记式（竖版/横版）成员页的配偶、子女、生卒等信息之间增加空格分隔</p>
              <p><b>修复：</b></p>
              <p>· 配偶多媒体串栏（成员多媒体列表混入配偶附件）</p>
              <p>· 删除生卒年后享年不消除</p>
              <p>· 更改生卒年后农历、干支、年号不重算</p>
              <p>· 卒年填「已逝」等标记不显示「卒未详」</p>
              <p>· 出生只填年份不生效</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev091">
          <template #title>
            <span class="log-title">v0.9.1</span>
            <span class="log-count">2026-09-10 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.9.1</span>
              <span class="log-date">2026-09-10</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 生卒日期显示规则统一为繁体中文：生卒未詳 / 生未詳 / 卒未詳 / 生於××年 / 生於××年間 等格式</p>
              <p>· 在世者不再显示卒年（卒日期字段全空视为在世）</p>
              <p><b>修复：</b></p>
              <p>· 清空日期字段时关联字段残留的问题</p>
              <p>· 在世者误显示「卒未詳」的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev090">
          <template #title>
            <span class="log-title">v0.9.0</span>
            <span class="log-count">2026-09-09 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.9.0</span>
              <span class="log-date">2026-09-09</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增授权系统：数据库实例 ID + Ed25519 离线授权码，配套授权码生成工具（keygen），在线更新增加授权校验，备份还原自动携带实例 ID</p>
              <p>· 新增成员人数上限：免费版超过 100 人后限制新增/编辑（可删除，删回 100 以内自动恢复）</p>
              <p>· 谱序/祖训/族规竖排排版优化：行距、列距、边距、每页字数、末页居中、预览比例可调</p>
              <p>· 在线更新增加授权核对，支持降级更新（正式版出问题可回退旧版本）</p>
              <p><b>修复：</b></p>
              <p>· 导出 PDF 底部空白过大、竖排文字换行截断等问题</p>
              <p>· 谱序/祖训/族规排版细节问题（标题遮挡、列宽、换页等）</p>
              <p>· 在线更新下载更新包提示 404 的问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev082">
          <template #title>
            <span class="log-title">v0.8.8</span>
            <span class="log-count">2026-09-06 发布</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.8</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 封面/姓名/正文独立字号倍率控制（工具栏三个下拉框，各0.5x~1.5x互不影响）</p>
              <p>· 封面字辈等元素独立缩放，不同字段分开放大</p>
              <p>· 竖版横版封面统一字号 66/30/22</p>
              <p>· 打印前JS动态注入@page规则设置页面方向，绕过CSS page属性浏览器兼容问题</p>
              <p><b>修复：</b></p>
              <p>· 横版打印CSS选择器特异性问题——屏幕样式覆盖打印样式导致横版封面字号调整无效</p>
              <p>· 封面内容被裁切/一半截断的问题</p>
              <p>· 字辈派语一排超出页面不换行的问题</p>
              <p>· 字体大小不一、部分内容不放大、封面跳页等问题</p>
              <p>· 封面双框错位问题</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev081">
          <template #title>
            <span class="log-title">v0.8.7 测试版</span>
            <span class="log-count">19 条</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.19</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 封面内容被裁切/一半截断的问题（去掉overflow:hidden）</p>
              <p>· 字辈派语一排超出页面不换行的问题（恢复固定8列/16列布局，加max-width约束）</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.18</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 姓名/正文独立倍率在页面预览中不生效的问题（屏幕CSS补齐倍率变量）</p>
              <p>· 封面内容溢出到第二页、内框超出外框的问题（打印封面加overflow:hidden）</p>
              <p>· 字辈派语字号过大时不能自动换行的问题（打印grid改auto-fit自适应列数）</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.17</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 封面/姓名/正文各独立字号倍率选项（0.5x~1.5x），工具栏新增三个下拉框分别控制封面元素、成员姓名、正文内容的大小，互不影响</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.16</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 封面家族名/堂号/郡望大字号时过大：封面元素字号改为独立缩放（不受用户字号选择影响），正文成员表/字辈仍随字号选择统一缩放</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.15</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 彻底修正 A3 打印字号缩放：字号、格高恢复为随所选字号统一等比放大，部分内容没放大、大小不一的问题解决；页框保持铺满不溢出，封面大字最大号也不再跳到第二页叠加</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.14</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 放大字号时字体大小不一（部分字没跟着放大）的问题；现在封面、字辈、成员表等全部统一按字号等比放大，各元素比例一致</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.13</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 封面内框超出外框、双框错位的问题</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.12</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 放大字号后成员表框只占半页高的问题；现在放大字号时框、格、字同步放大铺满，封面等固定大字也不会爆框，各页比例统一</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.11</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 放大字号后打印出现框只占半页、格子空荡、封面分页方向错乱的问题；现在打印按标准密度整体等比放大铺满 A3，与字号选择解耦，各页比例统一</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.10</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 放大字号后部分内容（封面书名等）放大过度、部分没放大、比例失调的问题；现在打印按标准开本整体等比放大，各页比例统一</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.9</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 改用整体等比放大输出 A3，修正成员表横向溢出被裁、右侧空白、竖线不到底等排版问题，整张谱页连同字、格、间距一起铺满 A3</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.8</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 牒记式成员表打印时表格只占左上一块、右侧和下方大片空白的问题；现在表格拉满整页，竖格线通到页底</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.7</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 横版牒记成员表加宽（每格更宽、字更大，铺满横向）</p>
              <p>· 字号新增「一号 26pt」「初号 30pt」两个更大档位</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.6</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 牒记式打印版面固定按 A3 制作，取消原来的 A3/A4 开本切换按钮</p>
              <p><b>修复：</b></p>
              <p>· 打印 A3 时内容只占纸张约七成、下方留白的问题；现在 A3 每页内容铺满整页</p>
              <p>· 需打印 A4/A5 等小开本时，在打印弹窗直接选对应纸张并勾选「适应纸张/缩放」，即自动等比缩小、版面不变形</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.5</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· A3 开本选择未保存、刷新或更新后弹回 A4，导致只在打印弹窗选 A3 时内容仍是 A4 大小铺在 A3 纸上的问题；开本选择现在会记住，并提示打印弹窗纸张要与所选开本一致</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.4</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 竖排牒记 A3 打印时内层框线仍只围到内容高度、下方留白的问题，现在内外双层边框都铺满整页 A3</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.3</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 竖排牒记 A3 打印时封面、字辈、成员表所有页面下方都差一截留白的问题，A3 版面统一贴满纸张（上下左右边距一致）</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.2</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 竖排牒记 A3 打印时封面页、字辈页的边框仍只排到约七成高、下方留白的问题，现在封面/字辈/谱头页也铺满整页 A3</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7.0906.1</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>修复：</b></p>
              <p>· 竖排牒记选 A3 打印时每页内容只占上半页、下方大片空白的问题（手机打印分页未按 A3 页高断页），现在 A3 每页内容铺满纸面</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev080">
          <template #title>
            <span class="log-title">v0.8 系列</span>
            <span class="log-count">8 条</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.7</span>
              <span class="log-date">2026-09-06</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 世系图导出升级：矢量 SVG、高清 PNG（300 DPI）、矢量 PDF 三种导出，放大到 A3/A2 印刷依旧清晰</p>
              <p>· 新增「矢量 PDF 导出」与苏式垂吊图内纪/外纪分离导出</p>
              <p>· 新增牒记式横版谱页（横排版式），与竖排牒记式分开入口</p>
              <p>· 牒记式新增「纸张开本」A4 / A3 开关：选 A3 时整套版面（字号、格子、间距、边距）等比放大铺满 A3，竖版横版均生效，适合交印厂放大开本</p>
              <p><b>修复：</b></p>
              <p>· 新增、删除成员等写操作未记入操作日志的问题</p>
              <p>· 宝塔式/苏式垂吊图父子连线错位、相邻家庭姓名块横向重叠的问题</p>
              <p>· 搜索「定位」要点两下才跳转、手机端弹出键盘后定位弹回的问题</p>
              <p>· 横版牒记封面/字辈页溢出、字辈页被拆成两页、空白边框页、字辈页只显示半页等排版问题</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.6</span>
              <span class="log-date">2026-09-05</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 竖排牒记式：外纪开基祖标「外纪始祖」，本支开基祖标「始祖」</p>
              <p>· 竖排牒记式：本支开基一世条目自动备注血缘世次（系某人之子、血缘第几世）</p>
              <p>· 竖排牒记式：本支人物条目可列出同父亲的姐/妹（家族设置可关闭，默认开）</p>
              <p><b>修复：</b></p>
              <p>· 竖排牒记式始祖称谓及位置显示的问题</p>
              <p>· 竖排牒记式姐/妹长幼改按出生年份判定（不再按录入顺序），出生年份缺失者统称「姐妹」</p>
              <p>· 叔伯/姑姑等父系平辈亲属查询分类错误的问题</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.5</span>
              <span class="log-date">2026-09-05</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 亲属关系增加自动计算堂表亲称谓（堂兄弟姐妹、表兄弟姐妹）</p>
              <p>· 新增母系亲属：外祖父母、舅舅、姨</p>
              <p>· 新增晚辈亲属：侄子、侄女、外甥、外甥女</p>
              <p>· 新增外纪/内纪：世系图分段显示，虚线和间距分隔，导出可单独渲染内纪或外纪</p>
              <p>· 编辑表单：选外纪时自动计算世代（选生父即定），内纪默认无世代、可手动或选父自动</p>
              <p>· 竖排牒记式：外纪/内纪分页独立显示，gen-bar 世代标签按各自世代分组</p>
              <p><b>修复：</b></p>
              <p>· 竖排牒记式外纪二世 gen-bar 不显示及页长过短的问题</p>
              <p>· 外纪选儿子时联动回填生母、外纪世代显示从一世开始</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.4</span>
              <span class="log-date">2026-09-04</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 创建支谱改用竖排牒记式（支谱名 + 牒记式表格，每人一竖格竖排）</p>
              <p><b>修复：</b></p>
              <p>· 宝塔式/苏式垂吊图定位：搜索定位放大并居中，不再直接弹出资料卡</p>
              <p>· 宝塔式/苏式垂吊图缩放：改为以鼠标位置为中心</p>
              <p>· 手机端双指缩放后不再误触发点击弹卡</p>
              <p>· 鼠标在人物上拖动不再误弹资料卡（仅单次点击弹卡）</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.3</span>
              <span class="log-date">2026-09-04</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 竖排牒记式新增谱头页（谱序、凡例、祖训、族规、字辈派语竖排排版）</p>
              <p>· 竖排牒记式新增字体选择（宋体/楷体/仿宋/黑体/隶书）</p>
              <p>· 竖排牒记式新增「上传字体」，保存到系统、换浏览器/设备可继续使用</p>
              <p>· 竖排牒记式新增字号调整（传统中文字号制，最大二号）</p>
              <p>· 字辈派语单独改为横排</p>
              <p><b>修复：</b></p>
              <p>· 谱头页文字溢出格子、叠字、跑出框线</p>
              <p>· 手机端谱序/族规竖行底部字被裁掉一半</p>
              <p>· 上传中文字体名误报「需家主权限」</p>
              <p>· 手机/微信上传 woff2 误报「格式不支持」</p>
              <p>· 放大字号导出 PDF 超出页面</p>
              <p>· 放大字号后分页不联动（格子挤爆、凡例不减少、族规不翻页）</p>
              <p>· 二号字时凡例页溢出页底</p>
              <p>· 上传字体只能保留一个（再次上传覆盖）</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.2</span>
              <span class="log-date">2026-09-04</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 删除「功能测试」菜单及两个测试功能（竖排谱书、苏式垂珠分页）</p>
              <p>· 竖排牒记式新增封面（竖排书名、堂号、郡望/祖籍/歲次重修）</p>
              <p>· 竖排牒记式导出 PDF 自带页码</p>
              <p><b>修复：</b></p>
              <p>· 手机端导出 PDF 封面与牒记表跨页空白、页码叠入表格</p>
              <p>· 家主/理事无法执行版本更新（提示权限不足）</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.1</span>
              <span class="log-date">2026-09-03</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 多媒体增加分类（照片/视频/音频），查看时按分类筛选，上传时按文件类型自动归类</p>
              <p>· 竖排牒记式重写：每人一竖格竖排排版，世代黑条贴格缘不压字、文字从格顶起排、满列向左换列</p>
              <p>· 竖排牒记式接入真实成员数据（随谱自动生成）</p>
              <p>· 菜单调整：竖排牒记式移入「世系可视化」，竖排谱书移入「功能测试」</p>
              <p><b>修复：</b></p>
              <p>· 成员/配偶详情页上传照片时页面崩溃、多媒体卡片不显示</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.8.0</span>
              <span class="log-date">2026-09-02</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 密码安全：改用 bcrypt 加密存储，旧密码登录后自动升级</p>
              <p>· 用户权限（家族职务分配）从「服务器设置」迁至「家族设置」</p>
              <p>· 菜单「家族设置」去掉「（含职务分配）」</p>
              <p>· 备份文件列表按时间倒序，最新的在最前</p>
              <p><b>修复：</b></p>
              <p>· 每次启动会把 admin 密码重置的问题</p>
              <p>· 跨房支操作被拦截时提示时有时无</p>
              <p>· 设置支系下拉选项空白</p>
              <p>· 更新回滚备份与系统备份只增不删、占用空间无限膨胀</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev070">
          <template #title>
            <span class="log-title">v0.7 系列</span>
            <span class="log-count">1 条</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.7.0</span>
              <span class="log-date">2026-09-01</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 协同修谱：理事增删改先提交审核，家主/超管通过后生效</p>
              <p>· 修改历史与回滚：每次变更自动留档，可查看字段差异并回滚</p>
              <p>· 房支写权限隔离：理事绑定所属房支，跨房支操作被拦截</p>
              <p>· 冲突检测：同名同世代重复、父子世代冲突、父亲性别非男</p>
              <p>· 备份体系升级：每日 GEDCOM 快照、多媒体去重快照、每月全量备份</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="logDev">
          <template #title>
            <span class="log-title">v0.6 系列</span>
            <span class="log-count">2 条</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.6.1</span>
              <span class="log-date">2026-08-31</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 配偶生平条目化（多条「标题+内容」条目式管理）</p>
              <p>· 配偶新增「职业」「学历」字段</p>
              <p>· 配偶详情支持 AI 优化</p>
              <p><b>修复：</b></p>
              <p>· 世系图画布边界截断（宝塔式 + 苏式）</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.6.0</span>
              <span class="log-date">2026-08-31</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 成员资料卡新增「职业」「学历」字段</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="log0">
          <template #title>
            <span class="log-title">v0.5 系列</span>
            <span class="log-count">6 条</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.5.18</span>
              <span class="log-date">2026-08-31</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 修谱助手权限收口：仅家主/超管可用 AI 优化与修谱助手</p>
              <p>· AI 优化新增「约束等级」「润色程度」双滑条</p>
              <p>· AI 优化日期保护（日期/纪年不随润色改动）</p>
              <p>· 修谱助手新增「火山方舟」云端大模型切换</p>
              <p><b>修复：</b></p>
              <p>· 修谱助手模型名称保存后又变回默认</p>
              <p>· 火山方舟调用 401 报错</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.5.17</span>
              <span class="log-date">2026-08-30</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 成员生平条目化（多条「标题+内容」条目）</p>
              <p>· 族员全系统只读，隐藏所有编辑按钮</p>
              <p><b>修复：</b></p>
              <p>· 族员按钮隐藏未生效</p>
              <p>· 成员详情「生平」区块无条目时消失</p>
              <p>· 新增成员保存无反应</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.5.16</span>
              <span class="log-date">2026-08-30</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 家族设置新增「始祖设置」（始祖/始迁祖）</p>
              <p>· 房支管理新增「创建支谱」（一键生成房支专属谱）</p>
              <p>· 修谱助手（本地 AI）上线：校验/语病提醒/语言美化/修谱建议</p>
              <p>· 新增成员保存前 AI 校验 + 父子年龄检查</p>
              <p>· 生日/祭日页补充配偶，生日页排除已过世者</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.5.15</span>
              <span class="log-date">2026-08-29</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 房支管理上线（房支增删改 + 分配成员）</p>
              <p>· 家族设置新增「堂口」「郡望」「祖籍」字段</p>
              <p>· 设置入口重构：系统设置仅家主/超管可见</p>
              <p><b>修复：</b></p>
              <p>· 全新部署时管理员密码未哈希导致无法登录</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.5.14</span>
              <span class="log-date">2026-08-29</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 清单式在线更新：拉取 latest.json 对比版本，一键下载 + sha256 校验 + 自动部署</p>
              <p>· 更新页新增「更新地址」可配置多个更新源</p>
            </div>
          </div>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.5.13</span>
              <span class="log-date">2026-08-29</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 版本号规则升级：正式版 x.y.z、测试版 x.y.z.mmdd.n</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="log1">
          <template #title>
            <span class="log-title">v0.4 系列</span>
            <span class="log-count">1 条</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.4</span>
              <span class="log-date">2026-08-29</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 新增「功能测试」竖排牒记式示例页</p>
              <p>· 数据看板新增「生日」「祭日」子菜单</p>
              <p>· 成员/配偶资料卡新增年龄、享年</p>
              <p>· 登录防爆破（连续失败锁定）</p>
              <p>· 检查更新、服务器更新包目录切换</p>
              <p><b>修复：</b></p>
              <p>· 每日备份重复</p>
              <p>· 农历月份显示与闰月</p>
              <p>· 享年不显示</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="log2">
          <template #title>
            <span class="log-title">v0.3 系列</span>
            <span class="log-count">1 条</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.3</span>
              <span class="log-date">2026-08-26</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 行传页、竖排谱书完善</p>
              <p>· 在线更新（上传更新包、自动重启、回滚）</p>
              <p>· GEDCOM 导出/导入</p>
              <p>· 用户权限体系、成员改名、角色更名（长老→理事、族长→家主）</p>
              <p><b>修复：</b></p>
              <p>· 导出 PDF 横版与打印样式</p>
              <p>· 世系图连线交叉、名字重叠</p>
              <p>· 下载备份提示未登录</p>
              <p>· 日期年号显示</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="log3">
          <template #title>
            <span class="log-title">v0.2 系列</span>
            <span class="log-count">1 条</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.2</span>
              <span class="log-date">2026-08-24</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· PC/手机端自动适配（响应式布局）</p>
              <p>· 苏式垂吊图双人并排（配偶卡片）</p>
            </div>
          </div>
        </el-collapse-item>
        <el-collapse-item name="log4">
          <template #title>
            <span class="log-title">v0.1 系列</span>
            <span class="log-count">1 条</span>
          </template>
          <div class="log-entry">
            <div class="log-entry-head">
              <span class="log-ver">v0.1</span>
              <span class="log-date">2026-08-23</span>
            </div>
            <div class="log-entry-body">
              <p><b>更新：</b></p>
              <p>· 初始版本：基本族谱管理、备份系统、配偶详情、宝塔式世系图</p>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <!-- 更新进度弹窗 -->
    <el-dialog
      v-model="progressVisible"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
      width="400px"
      align-center
    >
      <div class="progress-body">
        <div class="progress-title">{{ progressTitle }}</div>
        <el-progress
          v-if="progressStage === 'downloading'"
          :percentage="progressPercent"
          :stroke-width="16"
          text-inside
          class="progress-bar"
        />
        <el-progress
          v-else-if="progressStage === 'extracting' || progressStage === 'restarting'"
          :percentage="100"
          :stroke-width="16"
          :indeterminate="true"
          :duration="1"
          class="progress-bar"
        />
        <div class="progress-msg" :class="{ 'is-error': progressStage === 'error', 'is-success': progressStage === 'done' }">
          {{ progressText }}
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'

const currentVersion = ref('')
const activeLogs = ref(["logStable139"]) // 更新日志折叠面板（默认展开最新正式版；测试版不发日志）
const selectedFile = ref(null)
const fileList = ref([])
const updating = ref(false)
const rolling = ref(false)
const serverPackages = ref([])
const serverFile = ref('')
const loadingServer = ref(false)
const updatingServer = ref(false)
const betaChecked = ref(false)
const checking = ref(false)
const updatingManifest = ref(false)
const shareInfo = ref({ checked: false, hasUpdate: false, canDowngrade: false, latest: '', notes: '', latestURL: '', error: '', source: '', sources: [] })
const updateDirHost = ref('')
const updateURL = ref('')

// 更新进度弹窗
const progressVisible = ref(false)
const progressStage = ref('downloading') // downloading / extracting / restarting / done / error
const progressPercent = ref(0)
const progressText = ref('')
let progressTimer = null

const progressTitle = computed(() => {
  switch (progressStage.value) {
    case 'downloading': return '正在下载更新包'
    case 'extracting': return '正在解压...'
    case 'restarting': return '正在重启...'
    case 'done': return '更新完毕'
    case 'error': return '更新失败'
    default: return '正在更新...'
  }
})

// 更新地址管理
const sourcesDialogVisible = ref(false)
const sourcesDraft = ref([])
const savingSources = ref(false)
const builtinSource = ref('')
const sourcesCount = computed(() => sourcesDraft.value.length + (builtinSource.value ? 1 : 0))

const selectedVersion = computed(() => {
  if (!selectedFile.value) return ''
  const m = selectedFile.value.name.match(/v?(\d+(?:\.\d+){2,4})/)
  return m ? m[1] : ''
})

const canUpdate = computed(() => {
  if (!selectedVersion.value || !currentVersion.value) return false
  return compareVersion(selectedVersion.value, currentVersion.value) !== 0
})

function compareVersion(a, b) {
  const pa = a.split('.').map(Number)
  const pb = b.split('.').map(Number)
  const n = Math.max(pa.length, pb.length)
  for (let i = 0; i < n; i++) {
    const na = pa[i] || 0
    const nb = pb[i] || 0
    if (na > nb) return 1
    if (na < nb) return -1
  }
  return 0
}

onMounted(async () => {
  try {
    const { data } = await api.get('/settings/version')
    currentVersion.value = data.version || '1.3.9'
  } catch (e) { /* ignore */ }
  checkManifestUpdate() // 打开页面自动检查一次更新
})

// 手动检查测试版更新包列表（不自动检查/安装）
async function checkBeta() {
  loadingServer.value = true
  betaChecked.value = true
  serverFile.value = ''
  try {
    const { data } = await api.get('/update/server-packages', { params: { mode: 'beta' } })
    serverPackages.value = Array.isArray(data) ? data : (data.packages || [])
    if (serverPackages.value.length === 0) ElMessage.info('未检测到测试版更新包')
  } catch (e) {
    serverPackages.value = []
    ElMessage.error('检查测试版失败：' + (e?.response?.data?.error || e.message))
  } finally {
    loadingServer.value = false
  }
}

const selectedBeta = computed(() => serverPackages.value.find(p => p.name === serverFile.value) || null)

// 测试版下载：三次确认（不稳定 → 可能崩溃/需手动恢复 → 输入“更新”）
async function downloadBeta() {
  const pkg = selectedBeta.value
  if (!pkg) return
  try {
    await ElMessageBox.confirm(
      `测试版 v${pkg.version} 可能不稳定，是否继续？`,
      '下载测试版',
      { type: 'warning', confirmButtonText: '继续', cancelButtonText: '取消' }
    )
  } catch (e) { return }
  try {
    await ElMessageBox.confirm(
      '⚠️ 测试版可能导致程序崩溃或无法启动，需自行手动恢复（备份数据/回滚二进制），确认继续？',
      '再次确认',
      { type: 'error', confirmButtonText: '仍然继续', cancelButtonText: '取消' }
    )
  } catch (e) { return }
  try {
    await ElMessageBox.prompt(
      '请输入“更新”两个字，确认下载并安装此测试版。',
      '最终确认',
      { confirmButtonText: '下载并更新', cancelButtonText: '取消', inputPattern: /^更新$/, inputErrorMessage: '请输入“更新”' }
    )
  } catch (e) { return }
  await doUpdateFromServer(pkg, true)
}

// 从更新服务器下载更新包更新
async function doUpdateFromServer(pkgArg = null, skipConfirm = false) {
  if (!serverFile.value) return
  const pkg = pkgArg || serverPackages.value.find(p => p.name === serverFile.value)
  const ver = pkg ? pkg.version : ''
  if (!skipConfirm) try {
    await ElMessageBox.confirm(
      `确认更新到 v${ver} 吗？系统将从更新服务器下载更新包，完成后服务会自动重启，期间系统将短暂不可用。`,
      '确认更新',
      { type: 'warning', confirmButtonText: '确认更新', cancelButtonText: '取消' }
    )
  } catch (e) {
    return
  }
  updatingServer.value = true
  openProgress()
  try {
    const payload = { file: serverFile.value }
    if (pkg && pkg.url) payload.url = pkg.url
    if (pkg && pkg.sha256) payload.sha256 = pkg.sha256
    if (pkg && pkg.channel) payload.channel = pkg.channel
    const { data } = await api.post('/update/from-server', payload, { timeout: 30000 })
    if (data && data.ok && data.task) {
      pollProgress(data.task)
    } else {
      progressVisible.value = false
      ElMessage.info(data?.message || '无需更新')
    }
  } catch (e) {
    progressVisible.value = false
    const msg = e?.response?.data?.error || e.message || '更新失败'
    ElMessage.error('更新失败：' + msg)
  } finally {
    updatingServer.value = false
  }
}

// 轮询健康检查，服务恢复后自动刷新（更新与回滚共用）
function waitAndReload() {
  setTimeout(() => {
    let waited = 0
    const timer = setInterval(async () => {
      waited += 2000
      try {
        await api.get('/health', { timeout: 3000 })
        clearInterval(timer)
        location.reload()
      } catch (e) {
        if (waited > 60000) {
          clearInterval(timer)
          ElMessage.error('服务重启超时，请手动刷新')
        }
      }
    }, 2000)
  }, 3000)
}

// 检查在线更新（清单式，多源取最高版本）
async function checkManifestUpdate() {
  checking.value = true
  shareInfo.value = { checked: false, hasUpdate: false, canDowngrade: false, latest: '', notes: '', latestURL: '', error: '', source: '', sources: [] }
  try {
    const { data } = await api.get('/update/check', { timeout: 60000 })
    if (data && data.ok) {
      shareInfo.value = {
        checked: true,
        hasUpdate: !!data.hasUpdate,
        canDowngrade: !!data.canDowngrade,
        latest: data.latest || '',
        notes: data.notes || '',
        latestURL: data.latestURL || '',
        source: data.source || '',
        sources: data.sources || [],
        error: ''
      }
      if (data.updateURL) updateURL.value = data.updateURL
      if (data.hasUpdate) {
        ElMessage.success(`发现新版本 v${data.latest}`)
      } else if (data.canDowngrade) {
        ElMessage.info(`服务器最新版本 v${data.latest} 低于当前版本，可回退`)
      } else {
        ElMessage.success('当前已是最新版本')
      }
    } else {
      shareInfo.value = { ...shareInfo.value, checked: true, sources: data?.sources || [], error: data?.error || '检测失败' }
    }
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '检测失败'
    shareInfo.value = { ...shareInfo.value, checked: true, error: msg }
  } finally {
    checking.value = false
  }
}

// 打开更新地址管理弹窗
async function openSourcesDialog() {
  sourcesDialogVisible.value = true
  try {
    const { data } = await api.get('/update/sources', { timeout: 15000 })
    sourcesDraft.value = data?.sources?.slice() || []
    builtinSource.value = data?.builtin || ''
  } catch (e) {
    ElMessage.error('获取更新地址失败：' + (e?.response?.data?.error || e.message))
  }
}

// 保存更新地址
async function saveSources() {
  const cleaned = sourcesDraft.value.map(s => (s || '').trim()).filter(Boolean)
  savingSources.value = true
  try {
    const { data } = await api.post('/update/sources', { sources: cleaned }, { timeout: 15000 })
    sourcesDraft.value = data?.sources?.slice() || []
    ElMessage.success('更新地址已保存')
    sourcesDialogVisible.value = false
    // 清空上次检测结果，提示重新检测
    shareInfo.value = { checked: false, hasUpdate: false, canDowngrade: false, latest: '', notes: '', latestURL: '', error: '', source: '', sources: [] }
  } catch (e) {
    ElMessage.error('保存失败：' + (e?.response?.data?.error || e.message))
  } finally {
    savingSources.value = false
  }
}

// 打开更新进度弹窗
function openProgress() {
  progressVisible.value = true
  progressStage.value = 'downloading'
  progressPercent.value = 0
  progressText.value = '正在下载...'
}

// 等待服务重启恢复后回调（轮询 /health）
function waitServiceUp(onUp) {
  let waited = 0
  const timer = setInterval(async () => {
    waited += 2000
    try {
      await api.get('/health', { timeout: 3000 })
      clearInterval(timer)
      if (onUp) onUp()
    } catch (e) {
      if (waited > 120000) {
        clearInterval(timer)
        progressStage.value = 'error'
        progressText.value = '服务重启超时，请手动刷新页面'
      }
    }
  }, 2000)
}

// 轮询更新进度（下载 % → 解压中 → 重启中 → 更新完毕）
function pollProgress(task) {
  clearInterval(progressTimer)
  progressTimer = setInterval(async () => {
    try {
      const { data } = await api.get('/update/progress', { params: { task }, timeout: 5000 })
      progressStage.value = data.stage || 'downloading'
      if (data.stage === 'downloading') {
        progressPercent.value = data.percent || 0
        progressText.value = data.message || '正在下载...'
      } else if (data.stage === 'extracting') {
        progressPercent.value = 100
        progressText.value = data.message || '正在解压...'
      } else if (data.stage === 'restarting') {
        progressPercent.value = 100
        progressText.value = '正在重启...'
        clearInterval(progressTimer)
        waitServiceUp(() => {
          progressStage.value = 'done'
          progressText.value = '更新完毕'
          setTimeout(() => {
            progressVisible.value = false
            location.reload()
          }, 1500)
        })
      } else if (data.stage === 'done') {
        clearInterval(progressTimer)
        progressStage.value = 'done'
        progressText.value = data.message || '更新完毕'
        setTimeout(() => {
          progressVisible.value = false
          location.reload()
        }, 1500)
      } else if (data.stage === 'error') {
        clearInterval(progressTimer)
        progressStage.value = 'error'
        progressText.value = data.error || '更新失败'
        setTimeout(() => { progressVisible.value = false }, 2500)
      }
    } catch (e) {
      // 请求失败（服务已在部署/重启中退出，或重启后任务已被清空）→ 切到健康检查等待恢复
      if (progressStage.value === 'extracting' || progressStage.value === 'restarting') {
        clearInterval(progressTimer)
        if (progressStage.value !== 'restarting') {
          progressStage.value = 'restarting'
          progressPercent.value = 100
          progressText.value = '正在部署并重启...'
        }
        waitServiceUp(() => {
          progressStage.value = 'done'
          progressText.value = '更新完毕'
          setTimeout(() => {
            progressVisible.value = false
            location.reload()
          }, 1500)
        })
      }
    }
  }, 600)
}

// 一键在线更新（从清单 URL 下载并部署）
async function doUpdateFromManifest() {
  try {
    const act = shareInfo.value.hasUpdate ? '更新' : '回退'
    await ElMessageBox.confirm(
      `确认${act}到 v${shareInfo.value.latest} 吗？系统将下载更新包并自动重启，期间短暂不可用。`,
      `确认${act}`,
      { type: 'warning', confirmButtonText: `确认${act}`, cancelButtonText: '取消' }
    )
  } catch (e) {
    return
  }
  updatingManifest.value = true
  openProgress()
  try {
    const { data } = await api.post('/update/from-manifest', {}, { timeout: 30000 })
    if (data && data.ok && data.task) {
      pollProgress(data.task)
    } else {
      progressVisible.value = false
      ElMessage.info(data?.message || '无需更新')
    }
  } catch (e) {
    progressVisible.value = false
    const msg = e?.response?.data?.error || e.message || '更新失败'
    ElMessage.error('更新失败：' + msg)
  } finally {
    updatingManifest.value = false
  }
}

function onFileChange(file) {
  selectedFile.value = file.raw || file
  fileList.value = [file]
}

function onFileRemove() {
  selectedFile.value = null
  fileList.value = []
}

function clearFile() {
  selectedFile.value = null
  fileList.value = []
}

async function doUpdate() {
  if (!selectedFile.value) return
  try {
    await ElMessageBox.confirm(
      `确认更新到 v${selectedVersion.value} 吗？更新完成后服务会自动重启，期间系统将短暂不可用。`,
      '确认更新',
      { type: 'warning', confirmButtonText: '确认更新', cancelButtonText: '取消' }
    )
  } catch (e) {
    return
  }

  updating.value = true
  try {
    const formData = new FormData()
    formData.append('file', selectedFile.value)
    const { data } = await api.post('/update/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 300000
    })
    if (data && data.ok) {
      ElMessage.success(data.message || '更新完成，服务即将重启')
      waitAndReload()
    } else {
      ElMessage.info(data?.message || '无需更新')
    }
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '更新失败'
    ElMessage.error('更新失败：' + msg)
  } finally {
    updating.value = false
  }
}

async function doRollback() {
  try {
    await ElMessageBox.confirm(
      '确认回滚到上一个版本吗？回滚后服务会自动重启。',
      '确认回滚',
      { type: 'warning', confirmButtonText: '确认回滚', cancelButtonText: '取消' }
    )
  } catch (e) {
    return
  }
  rolling.value = true
  try {
    const { data } = await api.post('/update/rollback', {}, { timeout: 60000 })
    if (data && data.ok) {
      ElMessage.success(data.message || '回滚完成，服务即将重启')
      // 轮询健康检查，服务恢复后再刷新（避免撞上重启窗口导致“无法访问”）
      waitAndReload()
    } else {
      ElMessage.info(data?.message || '无法回滚')
    }
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '回滚失败'
    ElMessage.error('回滚失败：' + msg)
  } finally {
    rolling.value = false
  }
}
</script>

<style scoped>
.update-page { max-width: 700px; }
@media (max-width: 767px) { .update-page { max-width: 100%; } }
.update-page h4 { margin: 0 0 6px; color: #3a3226; }
.update-page p { margin: 0; color: #666; font-size: 14px; }

.update-box { padding: 4px 0; }
.update-row { display: flex; margin-bottom: 14px; align-items: center; }
.update-row .label { min-width: 90px; color: #666; flex-shrink: 0; white-space: nowrap; }
.update-row .value { color: #3a3226; display: flex; align-items: center; gap: 8px; }
.update-row .value.hint { color: #999; font-size: 13px; }
.update-actions { margin: 8px 0; display: flex; gap: 8px; flex-wrap: wrap; }
.server-update { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.update-tip { margin-top: 12px; }
.check-result { color: #999; font-size: 13px; }
.dir-path { color: #0bbd87; font-family: monospace; font-size: 13px; word-break: break-all; }

.source-scan { background: #fafafa; border: 1px solid #eee; border-radius: 6px; padding: 10px 12px; font-size: 13px; }
.source-scan-title { color: #666; margin-bottom: 6px; }
.source-scan-item { display: flex; gap: 8px; align-items: baseline; padding: 3px 0; flex-wrap: wrap; }
.src-status.ok { color: #0bbd87; font-weight: bold; }
.src-status.fail { color: #e6a23c; font-weight: bold; }
.src-url { color: #606266; font-family: monospace; word-break: break-all; }
.src-ver { color: #0bbd87; flex-shrink: 0; }
.src-err { color: #e6a23c; font-size: 12px; }

.sources-dialog-tip { color: #999; font-size: 13px; margin-bottom: 12px; }
.sources-list { display: flex; flex-direction: column; gap: 8px; }
.sources-row { display: flex; gap: 8px; align-items: center; }
.src-idx { color: #999; width: 18px; text-align: right; flex-shrink: 0; }

.update-log .log-title { font-weight: 600; color: #3a3226; }
.update-log .log-count { margin-left: 8px; color: #999; font-size: 12px; }
.update-log .log-entry { padding: 6px 4px 10px; border-bottom: 1px dashed #eee; }
.update-log .log-entry:last-child { border-bottom: none; }
.update-log .log-entry-head { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.update-log .log-ver { font-weight: 700; color: #0bbd87; }
.update-log .log-date { color: #aaa; font-size: 12px; }
.update-log .log-entry-body { color: #666; font-size: 13px; line-height: 1.7; }
.update-log .log-entry-body p { margin: 0 0 4px; }
.update-log .log-entry-body b { color: #b88230; }

.progress-body { text-align: center; padding: 8px 4px; }
.progress-title { font-size: 16px; font-weight: 600; color: #3a3226; margin-bottom: 16px; }
.progress-bar { margin-bottom: 12px; }
.progress-msg { font-size: 13px; color: #999; min-height: 18px; }
.progress-msg.is-error { color: #f56c6c; }
.progress-msg.is-success { color: #0bbd87; }
</style>