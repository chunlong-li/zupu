<template>
  <div class="member-detail">
    <el-button @click="$router.back()" style="margin-bottom: 16px">← 返回</el-button>

    <el-row :gutter="16">
      <!-- 左侧主体 -->
      <el-col :xs="24" :md="16">
        <el-card class="section">
          <template #header>
            <div class="card-header">
              <div class="header-left">
                <el-avatar v-if="member.portrait" :src="thumbUrl(member.portrait, 120)" :size="48" shape="square" class="avatar" />
                <span class="serif">{{ member.name || (member.surname || '') + (member.given_name || '') }}</span>
                <el-button v-if="canEdit()" size="small" text type="primary" @click="openRename">改名</el-button>
                <span class="tag" v-if="member.clan_type !== '本生'">{{ member.clan_type }}</span>
              </div>
              <el-tooltip v-if="canEdit()" placement="top" content="迁居外地失联：不列入「生日/祭日」，世系图按老谱标注「殁葬失考，子嗣失考」">
                <span class="flag-group">
                  <span class="flag-label">迁居外地失联</span>
                  <el-switch class="flag-switch" :model-value="!!member.lost_contact" @change="toggleLostContact" />
                </span>
              </el-tooltip>
              <el-button v-if="canEdit()" type="primary" size="small" @click="openEdit">编辑</el-button>
              <el-button size="small" text @click="openVersions">修改历史</el-button>
              <el-button v-if="canViewDNA()" size="small" text type="primary" @click="openDNAMatch">DNA 比对</el-button>
            </div>
          </template>

          <div class="info-grid">
            <div class="info-item"><span class="label">世代</span><span>{{ genLabel(member) }}</span></div>
            <div class="info-item"><span class="label">字辈</span><span>{{ member.generation_word || '—' }}</span></div>
            <div class="info-item" v-if="member.former_name"><span class="label">曾用名</span><span class="former-name">{{ member.former_name }}</span></div>
            <div class="info-item"><span class="label">手机</span><span>{{ member.phone || '—' }}</span></div>
            <div class="info-item" v-if="member.dna_haploid"><span class="label">单倍体</span><span>{{ member.dna_haploid }}</span></div>
            <div class="info-item" v-if="member.dna_diploid"><span class="label">二倍体</span><span>{{ member.dna_diploid }}</span></div>
            <div class="info-item" v-if="member.dna_mito"><span class="label">线粒体</span><span>{{ member.dna_mito }}</span></div>
            <div class="info-item"><span class="label">字</span><span>{{ member.zi || '—' }}</span></div>
            <div class="info-item"><span class="label">号</span><span>{{ member.hao || '—' }}</span></div>
            <div class="info-item"><span class="label">排行</span><span>{{ member.rank || '—' }}</span></div>
            <div class="info-item"><span class="label">房支</span><span>{{ branchName(member.branch_id) || member.branch || '—' }}</span></div>
            <div class="info-item"><span class="label">性别</span><span>{{ member.gender === 1 ? '男' : '女' }}</span></div>
            <div class="info-item"><span class="label">宗法类型</span><span>{{ member.clan_type }}</span></div>
            <div class="info-item"><span class="label">族谱分类</span><span>{{ member.record_type || '内纪' }}</span></div>
            <div class="info-item" v-if="member.foster_father_id"><span class="label">嗣父</span><span class="clickable" @click="goMember(member.foster_father_id)">{{ getMemberName(member.foster_father_id) }}</span></div>
            <div class="info-item"><span class="label">生父</span><span class="clickable" @click="goMember(member.father_id)">{{ getMemberName(member.father_id) }}</span></div>
            <div class="info-item"><span class="label">生母</span><span class="clickable" @click="goMother()">{{ getMotherName() }}</span></div>
            <div class="info-item"><span class="label">籍贯</span><span>{{ member.native_place || '—' }}</span></div>
            <div class="info-item"><span class="label">现居地</span><span>{{ member.current_place || '—' }}</span></div>
            <div class="info-item"><span class="label">功名/官职</span><span>{{ member.title || '—' }}</span></div>
            <div class="info-item"><span class="label">职业</span><span>{{ member.occupation || '—' }}</span></div>
            <div class="info-item"><span class="label">学历</span><span>{{ member.education || '—' }}</span></div>
            <div class="info-item"><span class="label">生卒</span><span>{{ formatLifeDates(member) }}</span></div>
            <div class="info-item" v-if="member.lifespan"><span class="label">享年</span><span>{{ member.lifespan }} 岁</span></div>
            <div class="info-item" v-if="calcAges()"><span class="label">年龄</span><span>{{ calcAges() }}</span></div>
            <div class="info-item"><span class="label">葬地</span><span>{{ member.burial_place || '—' }}</span></div>
            <div class="info-item" v-if="member.married_out"><span class="label">出嫁</span><span>{{ member.married_out }}</span></div>
          </div>

          <div class="bio">
            <div class="bio-head">
              <h4>生平</h4>
              <div class="bio-head-actions">
                <el-button size="small" type="primary" text @click="goBiography()">{{ bioCount ? `查看传记（${bioCount}）` : '为此人立传' }}</el-button>
                <el-button v-if="canEdit()" size="small" type="primary" text @click="openBioDialog()">+ 新增条目</el-button>
              </div>
            </div>
            <div class="bio-entry" v-for="e in bioEntries" :key="e.id">
              <div class="bio-entry-head">
                <span class="bio-title serif">{{ e.title }}</span>
                <span v-if="canEdit()" class="bio-entry-actions">
                  <el-button size="small" text type="primary" @click="openBioDialog(e)">编辑</el-button>
                  <el-button size="small" text type="danger" @click="deleteBioEntry(e.id)">删除</el-button>
                </span>
              </div>
              <p class="bio-content">{{ e.content }}</p>
            </div>
            <el-empty v-if="!bioEntries.length" description="暂无生平条目" :image-size="40" />
          </div>

          <!-- 资料来源（成员页内区块，可多条、按类型区分）：有来源才整块显示；空时只给可编辑者留一个添加入口。
               权限沿用成员编辑权限（member_edit → 前端 canEdit()，后端同名权限点）。 -->
          <div class="sources" v-if="sources.length">
            <div class="bio-head">
              <h4>资料来源</h4>
              <div class="bio-head-actions" v-if="canEdit()">
                <el-button size="small" type="primary" text @click="openSourceDialog()">+ 添加</el-button>
              </div>
            </div>
            <div class="source-entry" v-for="s in sources" :key="s.id">
              <div class="source-head">
                <el-tag size="small" effect="plain" class="source-kind">{{ s.kind || '其他' }}</el-tag>
                <span v-if="canEdit()" class="bio-entry-actions">
                  <el-button size="small" text type="danger" @click="deleteSource(s.id)">删除</el-button>
                </span>
              </div>
              <p class="source-text">{{ s.text }}</p>
            </div>
          </div>
          <div class="sources-add" v-else-if="canEdit()">
            <el-button size="small" text type="primary" @click="openSourceDialog()">+ 添加资料来源</el-button>
          </div>
        </el-card>

        <!-- 配偶列表 -->
        <el-card class="section">
          <template #header>
            <div class="card-header">
              <span>配偶（{{ spouses.length }}）</span>
              <el-button v-if="canEdit()" type="primary" size="small" @click="openSpouseDialog()">添加配偶</el-button>
            </div>
          </template>
          <div v-if="spouses.length">
            <div class="spouse-item" v-for="s in sortedSpouses" :key="s.id">
              <div class="spouse-main">
                <span class="spouse-name clickable" @click="router.push(`/spouse/${s.id}`)">{{ s.name || (s.surname || '') + (s.given_name || '') }}</span>
                <el-tag size="small" :type="s.type === '正配' ? 'danger' : s.type === '继配' ? 'warning' : 'info'">{{ s.type }}</el-tag>
                <el-tag v-if="s.type === '继配' && hasOriginalSpouse" size="small" type="danger" effect="plain">原配在前</el-tag>
              </div>
              <div class="spouse-extra" v-if="s.remark">{{ s.remark }}</div>
              <div class="spouse-extra" v-if="s.family">家世：{{ s.family }}</div>
              <div class="spouse-extra" v-if="s.occupation">职业：{{ s.occupation }}</div>
              <div class="spouse-extra" v-if="s.education">学历：{{ s.education }}</div>
              <div class="spouse-extra">生卒：{{ formatLifeDates(s) }}</div>
              <div class="spouse-actions" v-if="canEdit()">
                <el-button size="small" text @click="openSpouseDialog(s)">编辑</el-button>
                <el-button size="small" text type="danger" @click="deleteSpouse(s.id)">删除</el-button>
              </div>
            </div>
          </div>
          <el-empty v-else description="暂无配偶记录" :image-size="40" />
        </el-card>

        <!-- 亲属关系 -->
        <el-card class="section">
          <template #header><span>亲属关系</span></template>
          <div v-if="Object.keys(relations).length">
            <div v-for="(items, group) in relations" :key="group" class="relation-group">
              <span class="relation-label">{{ group }}：</span>
              <span v-for="r in items" :key="r.member_id || ('sp'+r.spouse_id)" class="relation-tag clickable" @click="r.spouse_id ? goSpouse(r.spouse_id) : r.member_id ? goMember(r.member_id) : null">
                {{ r.name }}<small v-if="r.relation !== group">({{ r.relation }})</small>
              </span>
            </div>
          </div>
          <el-empty v-else description="暂无亲属关系" :image-size="40" />
        </el-card>

        <!-- 多媒体 -->
        <el-card class="section">
          <template #header>
            <div class="card-header">
              <span>多媒体资料（{{ attachments.length }}）</span>
              <el-upload
                v-if="canEdit()"
                :action="`/api/upload`"
                :data="uploadData"
                :headers="uploadHeaders"
                :show-file-list="false"
                :on-success="onUploadSuccess"
                accept="image/*,video/*,audio/*"
                style="display: inline-block"
              >
                <el-button type="primary" size="small">上传多媒体</el-button>
              </el-upload>
            </div>
          </template>
          <el-radio-group v-model="mediaCategory" size="small" style="margin-bottom: 12px;">
            <el-radio-button value="全部">全部</el-radio-button>
            <el-radio-button value="照片">照片</el-radio-button>
            <el-radio-button value="视频">视频</el-radio-button>
            <el-radio-button value="音频">音频</el-radio-button>
          </el-radio-group>
          <div class="media-grid" v-if="filteredAttachments.length">
            <div class="media-item" v-for="a in filteredAttachments" :key="a.id" @click="openMediaViewer(a)" style="cursor:pointer">
              <div class="media-thumb" v-if="isVideo(a)">
                <el-image :src="thumbUrl(a.path, 400)" fit="cover" class="media-img" />
                <div class="play-overlay"><span class="play-icon">▶</span></div>
              </div>
              <el-image v-else-if="isPhoto(a)" :src="thumbUrl(a.path, 400)" fit="cover" class="media-img" :preview-src-list="[a.path]" />
              <audio v-else-if="isAudio(a)" :src="a.path" controls class="media-audio"></audio>
              <el-image v-else :src="thumbUrl(a.path, 400)" fit="cover" class="media-img" />
              <div class="media-actions" v-if="canEdit()">
                <el-button v-if="isPhoto(a)" size="small" text @click="setAsPortrait(a.path)">设头像</el-button>
                <el-button size="small" text type="danger" @click="deleteAttachment(a.id)">删除</el-button>
              </div>
            </div>
          </div>
          <el-empty v-else description="暂无多媒体" :image-size="40" />
        </el-card>
      </el-col>

      <!-- 右侧：子女 -->
      <el-col :xs="24" :md="8">
        <el-card class="section">
          <template #header>
            <div class="card-header">
              <span>子女（{{ children.length }}）</span>
              <el-tooltip v-if="canEdit()" placement="top" content="无嗣：标记后不能再添加子女，世系图标注「无嗣」">
                <span class="flag-group">
                  <span class="flag-label">无嗣</span>
                  <el-switch class="flag-switch" :model-value="!!member.no_heir" @change="toggleNoHeir" />
                </span>
              </el-tooltip>
              <el-button v-if="canEdit()" type="primary" size="small" :disabled="!!member.no_heir" @click="openAddChild">添加子女</el-button>
            </div>
          </template>
          <div class="child-item" v-for="ch in children" :key="ch.id">
            <router-link :to="`/member/${ch.id}`" class="child-link">{{ ch.name || (ch.surname || '') + (ch.given_name || '') }}</router-link>
            <span class="child-sub">{{ ch.generation_word ? `字辈${ch.generation_word}` : '' }} {{ ch.gender === 1 ? '男' : '女' }}</span>
          </div>
          <el-empty v-if="!children.length" description="暂无子女记录" :image-size="40" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 编辑成员弹窗 -->
    <el-dialog v-model="editVisible" :title="editForm.id ? '编辑成员' : '新增成员'" width="750px">
      <el-form :model="editForm" label-width="90px">
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="姓"><el-input v-model="editForm.surname" disabled /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="名"><div class="hz-inline"><el-input v-model="editForm.given_name" placeholder="输入名" /><el-button size="small" text type="primary" @click="openHanzi('given_name')">生僻字</el-button></div></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="世代">
            <template v-if="editForm.record_type === '外纪'">
              <el-input :model-value="waijiGenLabel" disabled />
            </template>
            <el-input-number v-else v-model="editForm.generation" :min="1" />
          </el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="性别"><el-select v-model="editForm.gender"><el-option label="男" :value="1" /><el-option label="女" :value="2" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="字"><div class="hz-inline"><el-input v-model="editForm.zi" /><el-button size="small" text type="primary" @click="openHanzi('zi')">生僻字</el-button></div></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="号"><div class="hz-inline"><el-input v-model="editForm.hao" /><el-button size="small" text type="primary" @click="openHanzi('hao')">生僻字</el-button></div></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="排行"><el-input v-model="editForm.rank" placeholder="行三" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="字辈"><div class="hz-inline"><el-input v-model="editForm.generation_word" /><el-button size="small" text type="primary" @click="openHanzi('generation_word')">生僻字</el-button></div></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="房支"><el-select v-model="editForm.branch_id" clearable filterable placeholder="选择房支" style="width:100%"><el-option v-for="b in branches" :key="b.id" :label="b.name" :value="b.id" /></el-select></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="8"><el-form-item label="宗法"><el-select v-model="editForm.clan_type"><el-option v-for="t in clanTypes" :key="t" :label="t" :value="t" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="类型"><el-select v-model="editForm.record_type" @change="onRecordTypeChange"><el-option label="内纪" value="内纪" /><el-option label="外纪" value="外纪" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="出生"><el-row :gutter="4"><el-col :span="8"><el-select v-model="editForm.birth_calendar" size="small"><el-option label="公历" value="solar" /><el-option label="农历" value="lunar" /></el-select></el-col><el-col :span="16"><el-input v-model="editForm.birth_date" placeholder="1900-08-01" @blur="convertDate" /></el-col></el-row></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="卒年"><el-input v-model="editForm.death_gregorian" placeholder="留空=在世" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="生父"><el-select v-model="editForm.father_id" filterable clearable placeholder="选择生父" style="width:100%" @change="onFatherChange"><el-option v-for="m in fatherCandidates" :key="m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel(m)}）`" :value="m.id" /></el-select></el-form-item>
          </el-col>
          <el-col :span="12" v-if="editForm.record_type === '外纪'">
            <el-form-item label="儿子"><el-select v-model="editForm.son_id" filterable clearable placeholder="选择儿子（关键）" style="width:100%" @change="onSonChange"><el-option v-for="m in sonCandidates" :key="m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel(m)}）`" :value="m.id" /></el-select>
              <div v-if="editForm.son_id" style="margin-top:4px;color:#909399;font-size:12px">{{ sonPositionText }}</div></el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12" v-if="editForm.record_type !== '外纪'">
          <el-col :span="12">
            <el-form-item label="生母"><el-select v-model="editForm.mother_spouse_id" filterable clearable placeholder="选择生母（配偶）" style="width:100%" @focus="loadEditMotherCandidates"><el-option v-for="m in editMotherCandidates" :key="'sp'+m.id" :label="`${m.name}（${m.type}）`" :value="m.id" /></el-select></el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="嗣父"><el-select v-model="editForm.foster_father_id" filterable clearable placeholder="选择嗣父" style="width:100%"><el-option v-for="m in fatherCandidates" :key="m.id" :label="`${m.name || (m.surname||'')+(m.given_name||'')}（${genLabel(m)}）`" :value="m.id" /></el-select></el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="籍贯"><el-input v-model="editForm.native_place" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="现居地"><el-input v-model="editForm.current_place" placeholder="如：浙江省杭州市西湖区" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="手机"><el-input v-model="editForm.phone" placeholder="手机号码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="单倍体"><el-input v-model="editForm.dna_haploid" placeholder="单倍体DNA编码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="二倍体（STR）"><el-input v-model="editForm.dna_diploid" type="textarea" :rows="3" placeholder="粘贴 STR 位点串：位点名/数1/数2，用 - 分隔（如 D8S1179/12/14-D21S11/29/31-…）" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="线粒体"><el-input v-model="editForm.dna_mito" placeholder="线粒体DNA编码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="职业"><el-input v-model="editForm.occupation" placeholder="如：教师 / 医生" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="学历"><el-input v-model="editForm.education" placeholder="如：本科 / 举人" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="功名/官职"><el-input v-model="editForm.title" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="出嫁信息"><el-input v-model="editForm.married_out" placeholder="适 XXX" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="葬地"><el-input v-model="editForm.burial_place" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" @click="saveMember">保存</el-button>
      </template>
    </el-dialog>

    <!-- 生僻字查字面板 -->
    <HanziPicker v-model="hanziVisible" @pick="onHanziPick" />

    <!-- 配偶编辑弹窗 -->
    <el-dialog v-model="spouseVisible" :title="spouseForm.id ? '编辑配偶' : '添加配偶'" width="500px">
      <el-form :model="spouseForm" label-width="80px">
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="姓"><el-input v-model="spouseForm.surname" placeholder="配偶姓氏" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="名"><el-input v-model="spouseForm.given_name" placeholder="配偶名" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="类型"><el-select v-model="spouseForm.type"><el-option label="正配" value="正配" /><el-option label="继配" value="继配" /><el-option label="侧室" value="侧室" /></el-select></el-form-item>
        <el-form-item label="家世"><el-input v-model="spouseForm.family" /></el-form-item>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="出生"><el-row :gutter="4"><el-col :span="8"><el-select v-model="spouseForm.birth_calendar" size="small"><el-option label="公历" value="solar" /><el-option label="农历" value="lunar" /></el-select></el-col><el-col :span="16"><el-input v-model="spouseForm.birth_date" placeholder="1900-08-01" @blur="spouseConvertDate" /></el-col></el-row></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="卒年"><el-input v-model="spouseForm.death_gregorian" placeholder="留空=在世" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="职业"><el-input v-model="spouseForm.occupation" placeholder="如：教师 / 医生" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="学历"><el-input v-model="spouseForm.education" placeholder="如：本科 / 举人" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="手机"><el-input v-model="spouseForm.phone" placeholder="手机号码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="单倍体"><el-input v-model="spouseForm.dna_haploid" placeholder="单倍体DNA编码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="二倍体"><el-input v-model="spouseForm.dna_diploid" placeholder="二倍体DNA编码" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="12" v-if="canViewDNA()">
          <el-col :span="24"><el-form-item label="线粒体"><el-input v-model="spouseForm.dna_mito" placeholder="线粒体DNA编码" /></el-form-item></el-col>
        </el-row>
        <el-form-item label="备注"><el-input v-model="spouseForm.remark" placeholder="例如：原配早逝，继配于某年续弦" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="spouseVisible = false">取消</el-button>
        <el-button type="primary" :loading="spouseSaving" @click="saveSpouse">保存</el-button>
      </template>
    </el-dialog>

    <!-- 改名弹窗 -->
    <el-dialog v-model="renameVisible" title="成员改名" width="480px">
      <el-alert type="warning" :closable="false" show-icon class="rename-alert">
        <template #title>改名后，该成员同名登录账号也会同步改名，请谨慎操作。</template>
      </el-alert>
      <el-form :model="renameForm" label-width="80px" class="rename-form">
        <el-form-item label="新名">
          <el-input v-model="renameForm.given_name" placeholder="请输入新名（姓氏不可改）">
            <template #prepend>{{ member.surname || '—' }}</template>
          </el-input>
        </el-form-item>
      </el-form>
      <div class="rename-tip">姓氏不可修改，仅可修改名字部分。</div>
      <div class="rename-tip">当前姓名：{{ member.name }}<template v-if="member.former_name">（曾用名：{{ member.former_name }}）</template></div>
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmRename">确认改名</el-button>
      </template>
    </el-dialog>

    <!-- 生平条目弹窗 -->
    <el-dialog v-model="bioVisible" :title="bioForm.id ? '编辑条目' : '新增条目'" width="640px">
      <el-form :model="bioForm" label-width="60px">
        <el-form-item label="标题"><el-input v-model="bioForm.title" placeholder="如：生平 / 迁居 / 功绩 / 轶事" /></el-form-item>
        <el-form-item label="内容"><el-input type="textarea" v-model="bioForm.content" :rows="6" placeholder="输入该条目的内容" /></el-form-item>

        <!-- AI 优化（病句排查 + 美化）-->
        <div class="ai-polish-box" v-if="canUseAI()">
          <div class="ai-polish-row">
            <span class="ai-polish-label">✨ AI 优化</span>
            <el-button size="small" type="warning" plain :loading="aiPolishing" @click="aiPolish">病句排查 + 美化</el-button>
          </div>
          <div class="ai-polish-row">
            <span class="ai-polish-label">📝 AI 生成小传</span>
            <el-select v-model="aiGenStyle" size="small" style="width: 104px">
              <el-option label="传记体" value="传记体" />
              <el-option label="简洁" value="简洁" />
              <el-option label="文言" value="文言" />
            </el-select>
            <el-select v-model="aiGenLength" size="small" style="width: 84px">
              <el-option label="短" value="短" />
              <el-option label="中" value="中" />
              <el-option label="长" value="长" />
            </el-select>
            <el-button size="small" type="success" plain :loading="aiGenerating" @click="aiGenerateBio">按档案生成</el-button>
            <span class="form-hint">依据该成员的档案字段（世代/生卒/籍贯/职业/学历/亲属…）生成草稿，日期由后端保护不会改动</span>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">约束等级</span>
            <el-button v-for="v in levelOptions" :key="'l'+v" size="small" :type="aiConstraint === v ? 'success' : ''" :plain="aiConstraint !== v" @click="aiConstraint = v">{{ v }}%</el-button>
          </div>
          <div class="ai-level-row">
            <span class="ai-level-label">润色程度</span>
            <el-button v-for="v in levelOptions" :key="'d'+v" size="small" :type="aiDegree === v ? 'success' : ''" :plain="aiDegree !== v" @click="aiDegree = v">{{ v }}%</el-button>
          </div>
          <template v-if="aiPolishResult">
            <el-form-item label="标题"><el-input v-model="aiPolishResult.title" placeholder="AI 优化的标题" /></el-form-item>
            <div class="ai-polish-row">
              <span class="ai-polish-label">内容</span>
              <el-button size="small" type="primary" plain @click="useAiText(true)">使用这个（含标题）</el-button>
            </div>
            <el-input type="textarea" v-model="aiPolishResult.content" :rows="6" placeholder="AI 优化后的内容输出在此" />
            <div class="ai-use-row">
              <el-button size="small" @click="useAiText(false)">使用这个（仅内容，标题用我填的）</el-button>
            </div>
          </template>
          <div v-else class="ai-polish-tip">「病句排查 + 美化」把已写好的标题和内容交给 AI 润色；「按档案生成」则依据成员档案从零生成小传草稿。结果都输出在下方，可选择是否采用。</div>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="bioVisible = false">取消</el-button>
        <el-button type="primary" :loading="bioSaving" @click="saveBioEntry">保存</el-button>
      </template>
    </el-dialog>

    <!-- 资料来源弹窗（新增一条：类型下拉 + 多行文本） -->
    <el-dialog v-model="sourceVisible" title="添加资料来源" width="560px">
      <el-form :model="sourceForm" label-width="70px">
        <el-form-item label="类型">
          <el-select v-model="sourceForm.kind" filterable allow-create default-first-option placeholder="选择或输入类型" style="width: 100%">
            <el-option v-for="k in SOURCE_KINDS" :key="k" :label="k" :value="k" />
          </el-select>
        </el-form-item>
        <el-form-item label="内容">
          <el-input type="textarea" v-model="sourceForm.text" :rows="4" maxlength="500" show-word-limit
                    placeholder="如：光緒壬辰劉氏族譜·卷三·第12頁" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="sourceVisible = false">取消</el-button>
        <el-button type="primary" :loading="sourceSaving" @click="saveSource">保存</el-button>
      </template>
    </el-dialog>

    <!-- 修改历史 / 版本回滚 -->
    <el-dialog v-model="versionsVisible" title="修改历史" width="640px">
      <el-table :data="versions" size="small" max-height="420">
        <el-table-column prop="version_no" label="版本" width="70" />
        <el-table-column prop="note" label="操作" width="90" />
        <el-table-column prop="changed_by_name" label="操作人" width="110">
          <template #default="{ row }">{{ row.changed_by_name || '—' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="170" />
        <el-table-column label="操作" width="150" align="right">
          <template #default="{ row }">
            <el-button size="small" text type="info" @click="viewVersionChanges(row)">查看修改</el-button>
            <el-button v-if="canRollback()" size="small" text type="primary" @click="rollbackVersion(row)">回滚</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!versions.length" description="暂无修改记录" :image-size="40" />
      <template #footer>
        <el-button @click="versionsVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 版本修改详情 -->
    <el-dialog v-model="changesVisible" :title="`修改详情 · 版本 ${changesVersionNo}`" width="560px">
      <el-alert v-if="changesDeleted" type="danger" :closable="false" title="该版本操作为删除，记录已被删除" />
      <el-table v-else :data="changes" size="small" max-height="420" empty-text="无字段级差异">
        <el-table-column prop="field" label="字段" width="120" />
        <el-table-column label="原值" min-width="160">
          <template #default="{ row }"><span class="diff-old">{{ row.from }}</span></template>
        </el-table-column>
        <el-table-column label="新值" min-width="160">
          <template #default="{ row }"><span class="diff-new">{{ row.to }}</span></template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="changesVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 头像裁剪 -->
    <AvatarCropper v-model:visible="cropVisible" :src="cropSrc" @crop="onCropConfirm" />
    <!-- 媒体查看器 -->
    <el-dialog v-model="mediaViewer.show" :title="mediaViewer.title || '查看媒体'" width="800px" destroy-on-close>
      <div style="text-align:center">
        <video v-if="isVideo(mediaViewer)" :src="mediaViewer.path" controls style="max-width:100%;max-height:70vh"></video>
        <img v-else :src="mediaViewer.path" style="max-width:100%;max-height:70vh" />
      </div>
    </el-dialog>

    <!-- 新增成员后自动建号：初始密码弹窗 -->
    <AccountPwDialog v-model:visible="acctPw.visible" :name="acctPw.name" :password="acctPw.password" />
    <!-- DNA 比对结果 -->
    <el-dialog v-model="dnaMatchVisible" title="DNA 比对" width="720px">
      <div v-if="dnaMatchLoading" style="text-align:center;padding:24px">比对中…</div>
      <div v-else>
        <div v-if="dnaMatchNote" style="color:#999;margin-bottom:12px">{{ dnaMatchNote }}</div>
        <div v-if="dnaMatchTarget" style="margin-bottom:12px;font-weight:600">
          比对对象：{{ dnaMatchTarget.name }}
          <span v-if="dnaMatchTarget.dna_haploid">｜单倍体 {{ dnaMatchTarget.dna_haploid }}</span>
          <span v-if="dnaMatchTarget.dna_mito">｜线粒体 {{ dnaMatchTarget.dna_mito }}</span>
          <span v-if="dnaMatchTarget.dna_diploid">｜二倍体 {{ dnaMatchTarget.dna_diploid }}</span>
        </div>
        <el-table :data="dnaMatches" v-if="dnaMatches.length" border>
          <el-table-column prop="name" label="成员" min-width="80" />
          <el-table-column prop="relation" label="关系" width="110" />
          <el-table-column prop="match_type" label="匹配方式" width="100">
            <template #default="scope">{{ scope.row.match_type === 'exact' ? '完全一致' : (scope.row.match_type === 'lineage' ? '同支系' : '前缀同支') }}</template>
          </el-table-column>
          <el-table-column prop="code" label="DNA 编码" min-width="130" />
          <el-table-column prop="generation" label="世代" width="70" />
          <el-table-column prop="score" label="评分" width="60" />
          <el-table-column label="亲子比对" width="90" fixed="right">
            <template #default="scope">
              <el-button size="small" text type="primary" @click="dnaMatchToParent(scope.row)">亲子比对</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div v-else style="color:#999;text-align:center;padding:16px">未找到 DNA 匹配的族人</div>
      </div>
    </el-dialog>

    <!-- 亲子比对 -->
    <el-dialog v-model="strCompareVisible" title="亲子比对（STR）" width="720px">
      <div style="margin-bottom:12px">
        <div style="font-weight:600">子 / 待鉴定人：{{ member.name }}</div>
        <div style="margin:12px 0 8px;color:#666">疑似父 / 母：</div>
        <el-select v-model="strCompareParentId" filterable placeholder="选择疑似父/母" style="width:100%">
          <el-option v-for="m in allMembers.filter(x => x.id !== Number(memberId))" :key="m.id" :label="`${m.name}（世代${m.generation}）`" :value="m.id" />
        </el-select>
      </div>
      <div v-if="strCompareLoading" style="text-align:center;padding:16px">比对中…</div>
      <div v-else-if="strCompareResult" style="margin-top:12px">
        <el-alert :type="strCompareResult.conclusion.includes('排除') ? 'error' : (strCompareResult.conclusion.includes('数据不足') ? 'warning' : 'success')" :title="strCompareResult.conclusion" :closable="false" style="margin-bottom:12px" />
        <div style="margin-bottom:8px">比对位点 {{ strCompareResult.loci_compared }} 个：符合 {{ strCompareResult.loci_matched }}，不符 {{ strCompareResult.loci_mismatched }}</div>
        <div v-if="strCompareResult.mismatch_loci.length" style="color:#f56c6c;margin-bottom:8px">不符位点：{{ strCompareResult.mismatch_loci.join('、') }}</div>
        <el-table :data="strCompareResult.detail" border size="small" max-height="300">
          <el-table-column prop="locus" label="位点" width="100" />
          <el-table-column label="疑似父/母" width="110"><template #default="s">{{ s.row.parent.join(' / ') }}</template></el-table-column>
          <el-table-column label="子" width="110"><template #default="s">{{ s.row.child.join(' / ') }}</template></el-table-column>
          <el-table-column label="符合" width="70"><template #default="s">{{ s.row.match ? '✓' : '✗' }}</template></el-table-column>
        </el-table>
      </div>
      <template #footer>
        <el-button @click="strCompareVisible=false">关闭</el-button>
        <el-button type="primary" :disabled="!strCompareParentId" :loading="strCompareLoading" @click="runSTRCompare">开始比对</el-button>
      </template>
    </el-dialog>

  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { aiValidateSave, confirmSaveProblems, buildMemberText, buildSpouseText, localMemberProblems, localSpouseProblems, mergeProblems } from '../utils/aiCheck'
import { canEdit, canUseAI, canRollback, canViewDNA } from '../utils/auth'
import { formatLifeDates, normalizeDeathFields } from '../utils/dateFormat'
import { SOURCE_KINDS } from '../utils/memberSources'
import AvatarCropper from '../components/AvatarCropper.vue'
import HanziPicker from '../components/HanziPicker.vue'
import AccountPwDialog from '../components/AccountPwDialog.vue'

const buildId = __BUILD_ID__ || 'dev'
const acctPw = ref({ visible: false, name: '', password: '' })
function thumbUrl(path, size = 400) {
  if (!path) return ''
  // 视频暂时返回缩略图URL（后端会按需用ffmpeg生成）
  return `/api/thumbnail?path=${encodeURIComponent(path)}&size=${size}&v=${buildId}`
}

const route = useRoute()
const router = useRouter()
const bioCount = ref(0)
function goBiography() {
  router.push({ path: '/culture', query: { tab: 'bios', member: String(memberId), new: bioCount.value ? '0' : '1' } })
}
async function loadBioCount() {
  try {
    const { data } = await api.get('/biographies', { params: { member_id: memberId } })
    bioCount.value = (data || []).length
  } catch (e) { bioCount.value = 0 }
}
const memberId = route.params.id
const dnaMatchVisible = ref(false)
const dnaMatchLoading = ref(false)
const dnaMatchTarget = ref(null)
const dnaMatches = ref([])
const dnaMatchNote = ref('')

async function openDNAMatch() {
  dnaMatchVisible.value = true
  dnaMatchLoading.value = true
  dnaMatchTarget.value = null
  dnaMatches.value = []
  dnaMatchNote.value = ''
  try {
    const { data } = await api.get(`/members/${memberId}/dna-match`)
    dnaMatchTarget.value = data.target
    dnaMatches.value = data.matches || []
    dnaMatchNote.value = data.note || ''
  } catch (e) {
    ElMessage.error(e.response?.data?.error || 'DNA 比对失败')
  } finally {
    dnaMatchLoading.value = false
  }
}



// 亲子比对（STR，从「二倍体」字段解析）
const strCompareVisible = ref(false)
const strCompareParentId = ref(null)
const strCompareLoading = ref(false)
const strCompareResult = ref(null)

async function runSTRCompare() {
  strCompareLoading.value = true
  strCompareResult.value = null
  try {
    const { data } = await api.post('/members/dna-str-compare', {
      child_id: Number(memberId),
      parent_id: strCompareParentId.value,
    })
    strCompareResult.value = data
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '比对失败')
  } finally {
    strCompareLoading.value = false
  }
}

// 从 DNA 比对结果点「亲子比对」：当前成员为子，匹配成员为疑似父/母
function dnaMatchToParent(row) {
  strCompareParentId.value = row.member_id
  strCompareVisible.value = true
  runSTRCompare()
}

const member = reactive({})
const spouses = ref([])
const children = ref([])
const allMembers = ref([])
const branches = ref([])
const relations = ref({})
const attachments = ref([])
const uploadHeaders = computed(() => ({ Authorization: 'Bearer ' + (localStorage.getItem('token') || '') }))
const mediaViewer = reactive({ show: false, type: '', path: '', title: '' })
function openMediaViewer(a) {
  mediaViewer.show = true
  mediaViewer.type = a.category || '照片'
  mediaViewer.path = a.path
  mediaViewer.title = a.description || ''
}
const mediaCategory = ref('全部')
const cropVisible = ref(false)
const cropSrc = ref('')
const uploadData = computed(() => ({ member_id: memberId }))
const filteredAttachments = computed(() => {
  if (mediaCategory.value === '全部') return attachments.value
  return attachments.value.filter(a => {
    let c = (a.category || '照片').replace('配偶', '')
    if (c === '头像') c = '照片'
    return c === mediaCategory.value
  })
})
const isVideo = (a) => a.category === '视频'
const isAudio = (a) => a.category === '音频'
const isPhoto = (a) => !isVideo(a) && !isAudio(a)
const familySurname = ref('')
const editVisible = ref(false)
const editForm = reactive({})

// 生僻字查字面板
const hanziVisible = ref(false)
const hanziTarget = ref('given_name')
function openHanzi(field) {
  hanziTarget.value = field
  hanziVisible.value = true
}
function onHanziPick(ch) {
  const k = hanziTarget.value
  if (!k) return
  editForm[k] = (editForm[k] || '') + ch
}
const originalBranchId = ref(0)  // 打开编辑时的原房支，用于判断房支是否变更
const spouseVisible = ref(false)
const spouseForm = reactive({})
const spouseSaving = ref(false)
const renameVisible = ref(false)
const renameForm = reactive({ surname: '', given_name: '' })
const bioEntries = ref([])
const bioVisible = ref(false)
const bioSaving = ref(false)
const bioForm = reactive({ id: 0, title: '', content: '', sort_order: 0 })
// 资料来源（成员页区块：可多条、按类型区分；权限沿用 member_edit）
const sources = ref([])
const sourceVisible = ref(false)
const sourceSaving = ref(false)
const sourceForm = reactive({ kind: '族谱旧谱', text: '' })
const aiPolishing = ref(false)
const aiGenerating = ref(false)
const aiPolishResult = ref(null)
const aiHasRun = ref(false)
const aiGenStyle = ref('传记体')  // 生成小传的文体
const aiGenLength = ref('中')     // 生成小传的篇幅
const levelOptions = [0, 25, 50, 75, 100]
const aiConstraint = ref(50) // 约束等级（越低越严）
const aiDegree = ref(50) // 润色程度（越高改动越大）
const clanTypes = ['本生', '嗣子', '兼祧', '入赘', '养子']

// 修改历史 / 版本回滚
const versionsVisible = ref(false)
const versions = ref([])
const changesVisible = ref(false)
const changes = ref([])
const changesVersionNo = ref(0)
const changesDeleted = ref(false)

const fatherCandidates = computed(() => {
  if (editForm.record_type === '外纪') {
    return allMembers.value.filter(m => m.gender === 1)
  }
  return allMembers.value.filter(m => m.gender === 1 && m.generation === editForm.generation - 1)
})

function genLabel(m) {
  if (m.record_type === '外纪') {
    const waiji = allMembers.value.filter(x => x.record_type === '外纪')
    if (waiji.length > 0) {
      const minGen = Math.min(...waiji.map(x => x.generation))
      return `外纪${m.generation - minGen + 1}世`
    }
    return `外纪${m.generation + 1}世`
  }
  return `第${m.generation}世`
}

const waijiGenLabel = computed(() => {
  if (!editForm.generation) return '—'
  const waiji = allMembers.value.filter(x => x.record_type === '外纪')
  if (waiji.length === 0) return `外纪${editForm.generation + 1}世`
  const base = Math.min(...waiji.map(x => x.generation))
  return `外纪${editForm.generation - base + 1}世`
})

const sonCandidates = computed(() => {
  return allMembers.value.filter(m => m.id !== editForm.id)
})

const sonPositionText = computed(() => {
  if (!editForm.son_id) return ''
  const son = allMembers.value.find(m => m.id === editForm.son_id)
  if (!son) return ''
  return `保存后 ${son.name || (son.surname||'')+(son.given_name||'')} 的生父将自动设为当前成员`
})

function onFatherChange(fid) {
  if (!fid) {
    if (editForm.record_type === '外纪') {
      editForm.generation = 0
    }
    return
  }
  const father = allMembers.value.find(m => m.id === fid)
  if (father) {
    editForm.generation = father.generation + 1
    // 自动继承父亲的房支：未手动选择房支时，跟随父亲
    if ((!editForm.branch_id || editForm.branch_id === 0) && father.branch_id) {
      editForm.branch_id = father.branch_id
    }
  }
  loadEditMotherCandidates()
}

function onSonChange(sid) {
  if (!sid) {
    if (editForm.record_type === '外纪') {
      editForm.generation = 0
    }
    return
  }
  const son = allMembers.value.find(m => m.id === sid)
  if (son) {
    editForm.generation = son.generation - 1
  }
}

function onRecordTypeChange(val) {
  if (val === '外纪') {
    editForm.father_id = 0
    editForm.son_id = 0
    editForm.generation = 0
  } else {
    editForm.father_id = 0
    editForm.son_id = 0
    editForm.generation = null
  }
}
const motherSpouseCandidates = ref([]) // 当前成员的配偶（生母候选人）
const fatherSpouses = ref([])           // 父亲的配偶（用于显示生母名称）
const editMotherCandidates = ref([])    // 编辑时生母下拉选项

// 继配排序：继配在原配之后
const hasOriginalSpouse = computed(() => spouses.value.some(s => s.type === '正配'))
const sortedSpouses = computed(() => {
  const order = { '正配': 1, '继配': 2, '侧室': 3 }
  return [...spouses.value].sort((a, b) => (order[a.type] || 4) - (order[b.type] || 4))
})

// 无嗣：标记后禁用「添加子女」，世系图标注「无嗣」
async function toggleNoHeir(val) {
  const v = val ? 1 : 0
  try {
    await api.put(`/members/${memberId}/flags`, { no_heir: v })
    member.no_heir = v
    ElMessage.success(v ? '已标记为无嗣' : '已取消无嗣标记')
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

// 迁居外地失联：不进生日/祭日，世系图标注「存殁未详，子女未详」
async function toggleLostContact(val) {
  const v = val ? 1 : 0
  try {
    await api.put(`/members/${memberId}/flags`, { lost_contact: v })
    member.lost_contact = v
    ElMessage.success(v ? '已标记为迁居外地失联' : '已取消失联标记')
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

async function load() {  try {
    const { data: fsData } = await api.get('/settings/family-surname')
    familySurname.value = fsData.family_surname || ''
  } catch (e) { /* ignore */ }
  loadBioCount()
  const { data } = await api.get(`/members/${memberId}/detail`)
  Object.assign(member, data)
  spouses.value = data.spouses || []
  const ch = await api.get(`/members/${memberId}/children`)
  children.value = ch.data
  const all = await api.get('/members')
  allMembers.value = all.data
  try {
    const { data: bData } = await api.get('/branches')
    branches.value = bData || []
  } catch (e) { /* ignore */ }
  const rel = await api.get(`/members/${memberId}/relations`)
  relations.value = rel.data
  const att = await api.get(`/members/${memberId}/attachments`)
  attachments.value = att.data
  // 加载生平条目
  try {
    const { data: bioData } = await api.get(`/members/${memberId}/bio-entries`)
    bioEntries.value = bioData || []
  } catch (e) { /* ignore */ }
  // 加载资料来源（成员页区块）
  loadSources()
  // 加载当前成员的配偶作生母候选人（用于编辑成员和添加子女时选择）
  try {
    const { data: spData } = await api.get(`/members/${memberId}/spouses-for-mother`)
    motherSpouseCandidates.value = spData
  } catch (e) { /* ignore */ }
  // 加载父亲的配偶（用于显示生母名称）
  try {
    if (member.father_id) {
      const { data: fsData } = await api.get(`/members/${member.father_id}/spouses`)
      fatherSpouses.value = fsData || []
    } else {
      fatherSpouses.value = []
    }
  } catch (e) { /* ignore */ }
}

function getMemberName(id) {
  if (!id) return '—'
  const p = allMembers.value.find(m => m.id === id)
  return p ? (p.name || (p.surname || '') + (p.given_name || '')) : `ID:${id}`
}

function branchName(id) {
  const b = branches.value.find(x => x.id === id)
  return b ? b.name : ''
}

function getMotherName() {
  if (member.mother_spouse_id) {
    const s = fatherSpouses.value.find(m => m.id === member.mother_spouse_id)
    if (s) return s.name || (s.surname || '') + (s.given_name || '')
  }
  return getMemberName(member.mother_id)
}
function goMember(id) { if (id) router.push(`/member/${id}`) }
function goSpouse(id) { if (id) router.push(`/spouse/${id}`) }
function goMother() {
  if (member.mother_spouse_id) {
    router.push(`/spouse/${member.mother_spouse_id}`)
  } else if (member.mother_id) {
    router.push(`/member/${member.mother_id}`)
  }
}

// 打开修改历史
async function openVersions() {
  versions.value = []
  try {
    const { data } = await api.get(`/members/${memberId}/versions`)
    versions.value = data || []
  } catch (e) {
    ElMessage.error('加载修改历史失败')
  }
  versionsVisible.value = true
}

// 查看某版本的字段级修改
async function viewVersionChanges(row) {
  changesVersionNo.value = row.version_no
  changes.value = []
  changesDeleted.value = false
  changesVisible.value = true
  try {
    const { data } = await api.get(`/entity-versions/${row.id}/changes`)
    changes.value = data.changes || []
    changesDeleted.value = !!data.deleted
  } catch (e) {
    ElMessage.error('加载修改详情失败')
  }
}

// 回滚到某版本
async function rollbackVersion(row) {
  try {
    await ElMessageBox.confirm(`确定回滚到版本 ${row.version_no}（${row.note}）吗？回滚前当前状态会另存一份，不会丢数据。`, '确认回滚', {
      confirmButtonText: '回滚', cancelButtonText: '取消', type: 'warning'
    })
  } catch (e) { return }
  try {
    await api.post(`/entity-versions/${row.id}/rollback`, { note: `回滚到版本 ${row.version_no}` })
    ElMessage.success('已回滚')
    versionsVisible.value = false
    load()
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '回滚失败'
    ElMessage.error('回滚失败：' + msg)
  }
}

// 计算年龄（周岁/虚岁），返回显示文本；无出生日期返回空
function calcAges() {
  const b = (member.birth_gregorian || '').trim()
  if (!b) return ''
  let by, bm = 1, bd = 1
  const mFull = b.match(/^(\d{4})-(\d{1,2})-(\d{1,2})/)
  const mYear = b.match(/^(\d{4})$/)
  if (mFull) { by = parseInt(mFull[1]); bm = parseInt(mFull[2]); bd = parseInt(mFull[3]) }
  else if (mYear) { by = parseInt(mYear[1]) }
  else return ''
  // 基准日期：已故用逝世日期，在世用今天
  let cy, cm, cd
  const d = (member.death_gregorian || '').trim()
  const dmFull = d.match(/^(\d{4})-(\d{1,2})-(\d{1,2})/)
  const dmYear = d.match(/^(\d{4})$/)
  if (dmFull) {
    cy = parseInt(dmFull[1]); cm = parseInt(dmFull[2]); cd = parseInt(dmFull[3])
  } else if (dmYear) {
    cy = parseInt(dmYear[1]); cm = 12; cd = 31
  } else {
    const now = new Date()
    cy = now.getFullYear(); cm = now.getMonth() + 1; cd = now.getDate()
  }
  // 周岁：完整年数，未过生日减 1
  let zhousui = cy - by
  if (cm < bm || (cm === bm && cd < bd)) zhousui--
  // 虚岁：出生即 1 岁，每过一个春节（公历 1 月 1 日近似）加 1 岁
  const xusui = cy - by + 1
  if (zhousui < 0) return ''
  return `周岁 ${zhousui} · 虚岁 ${xusui}`
}

function openEdit() {
  const copy = JSON.parse(JSON.stringify(member))
  Object.assign(editForm, copy)
  originalBranchId.value = member.branch_id || 0
  if (!editForm.surname) editForm.surname = familySurname.value
  // 初始化出生日期字段
  editForm.birth_calendar = editForm.birth_calendar || 'solar'
  editForm.birth_date = editForm.birth_date || editForm.birth_gregorian || ''
  editVisible.value = true
  // 预加载编辑时的生母候选人
  loadEditMotherCandidates()
}

async function loadEditMotherCandidates() {
  if (!editForm.father_id) { editMotherCandidates.value = []; return }
  try {
    const { data } = await api.get(`/members/${editForm.father_id}/spouses`)
    editMotherCandidates.value = data || []
  } catch (e) { editMotherCandidates.value = [] }
}

// 统计某成员的后代数量（沿 father_id 遍历）
function countDescendants(memberId) {
  let count = 0
  const queue = [memberId]
  const visited = new Set()
  while (queue.length) {
    const pid = queue.shift()
    for (const m of allMembers.value) {
      if (m.father_id === pid && !visited.has(m.id)) {
        visited.add(m.id)
        count++
        queue.push(m.id)
      }
    }
  }
  return count
}

async function saveMember() {
  await convertDate()  // 确保转换已执行
  const payload = { ...editForm }
  payload.name = (payload.surname || '') + (payload.given_name || '')
  // 房支规范化：未选择时清空 branch_id 与 branch 文本（避免旧文本被后端重新匹配）
  payload.branch_id = payload.branch_id || 0
  if (!payload.branch_id) payload.branch = ''
  // 卒年规范化：留空=在世；"已逝"等标记转存 death_era
  normalizeDeathFields(payload)
  // birth_gregorian 已由 convertDate 设置，不再从 birth_date 覆盖
  delete payload.birth_date
  delete payload.birth_calendar
  // 编辑已有成员且房支变更：若有后代，确认是否一起归入新分支
  if (editForm.id && (payload.branch_id || 0) !== (originalBranchId.value || 0)) {
    const desc = countDescendants(editForm.id)
    if (desc > 0) {
      const bname = branches.value.find(b => b.id === payload.branch_id)?.name || '未分房'
      try {
        await ElMessageBox.confirm(
          `该成员有 ${desc} 个后代，将一起归入「${bname}」，是否继续？`,
          '房支变更',
          { type: 'warning', confirmButtonText: '继续', cancelButtonText: '取消' }
        )
      } catch { return }
    }
  }
  let r
  try {
    if (editForm.id) { r = await api.put(`/members/${editForm.id}`, payload) }
    else { r = await api.post('/members', payload) }
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '保存失败'
    ElMessage.error('保存失败：' + msg)
    return
  }
  if (r.data && r.data.draft) { editVisible.value = false; ElMessage.success(r.data.message || '已提交审核'); return }
  editVisible.value = false
  if (r?.data?.account_password) {
    acctPw.value = { visible: true, name: r.data.name || editForm.name || '', password: r.data.account_password }
  } else {
    ElMessage.success('已保存')
  }
  load()
}

async function convertDate() {
  const d = (editForm.birth_date || '').trim()
  if (!d) {
    // 用户清空了出生日期 → 清空所有出生相关字段
    editForm.birth_gregorian = ''
    editForm.birth_lunar = ''
    editForm.birth_ganzhi = ''
    editForm.birth_era = ''
    return
  }
  // 只填年份（YYYY）→ 直接存年份，不转换月日
  const mYear = d.match(/^(\d{4})$/)
  if (mYear) {
    editForm.birth_gregorian = mYear[1]
    editForm.birth_lunar = ''
    editForm.birth_ganzhi = ''
    editForm.birth_era = ''
    return
  }
  const m = d.match(/^(\d{4})-(\d{2})-(\d{2})$/); if (!m) return
  if (editForm.birth_calendar === 'lunar') {
    // 农历 → 公历
    const { data } = await api.get(`/lunar/to-solar?y=${m[1]}&m=${m[2]}&d=${m[3]}`)
    editForm.birth_gregorian = data.gregorian
    editForm.birth_lunar = data.lunar
    editForm.birth_ganzhi = data.ganzhi
    editForm.birth_era = data.era + data.eraYear
    ElMessage.success(`公历：${data.gregorian}，${data.lunar}`)
  } else {
    // 公历 → 农历
    const { data } = await api.get(`/lunar/convert?y=${m[1]}&m=${m[2]}&d=${m[3]}`)
    editForm.birth_gregorian = d
    editForm.birth_lunar = data.lunar
    editForm.birth_ganzhi = data.ganzhi
    editForm.birth_era = data.era + data.eraYear
    ElMessage.success(`农历：${data.lunar}`)
  }
}

function openAddChild() {
  // 使用与编辑/新增成员相同的弹窗，预填父亲为当前成员
  const autoMotherId = motherSpouseCandidates.value.length > 0 ? motherSpouseCandidates.value[0].id : 0
  Object.assign(editForm, {
    id: 0,
    surname: familySurname.value,
    given_name: '',
    gender: 1,
    rank: '',
    birth_calendar: 'solar',
    birth_date: '',
    birth_gregorian: '',
    birth_lunar: '',
    birth_ganzhi: '',
    birth_era: '',
    death_gregorian: '',
    father_id: parseInt(memberId),
    mother_id: 0,
    mother_spouse_id: autoMotherId,
    foster_father_id: 0,
    generation: member.generation + 1,
    generation_word: '',
    clan_type: '本生',
    record_type: '内纪',
    branch_id: member.branch_id || 0,
    branch: member.branch || '',
    native_place: '',
    current_place: '',
    phone: '',
    occupation: '',
    education: '',
    title: '',
    married_out: '',
    burial_place: '',
    zi: '',
    hao: '',
    dna_haploid: '',
    dna_diploid: '',
    dna_mito: '',
    son_id: 0,
  })
  originalBranchId.value = 0
  editVisible.value = true
  loadEditMotherCandidates()
}

function openSpouseDialog(s = null) {
  if (s) {
    const copy = JSON.parse(JSON.stringify(s))
    copy.birth_calendar = copy.birth_calendar || 'solar'
    copy.birth_date = copy.birth_date || copy.birth_gregorian || ''
    Object.assign(spouseForm, copy)
  } else {
    Object.assign(spouseForm, {
      id: 0, member_id: parseInt(memberId),
      surname: '', given_name: '',
      name: '', type: '正配', family: '',
      birth_calendar: 'solar', birth_date: '',
      birth_gregorian: '', death_gregorian: '',
      occupation: '', education: '', remark: '',
      phone: '', dna_haploid: '', dna_diploid: '', dna_mito: ''
    })
  }
  spouseVisible.value = true
}

async function saveSpouse() {
  if (spouseSaving.value) return
  await spouseConvertDate()  // 确保转换已执行
  const payload = { ...spouseForm }
  payload.name = (payload.surname || '') + (payload.given_name || '')
  delete payload.birth_date

  // 新增配偶：保存前校验（编辑不校验）
  if (!spouseForm.id) {
    spouseSaving.value = true
    try {
      const local = localSpouseProblems(spouseForm)
      const ai = await aiValidateSave(buildSpouseText(spouseForm))
      const problems = mergeProblems(local, ai)
      if (problems.length) {
        const goAhead = await confirmSaveProblems(problems)
        if (!goAhead) return // 去修改
      }
    } finally {
      spouseSaving.value = false
    }
  }

  spouseSaving.value = true
  try {
    let r
    if (spouseForm.id) { r = await api.put(`/spouses/${spouseForm.id}`, payload) }
    else { r = await api.post('/spouses', payload) }
    if (r.data && r.data.draft) {
      spouseVisible.value = false
      ElMessage.success(r.data.message || '已提交审核')
      return
    }
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '保存失败'
    ElMessage.error('保存失败：' + msg)
    return
  } finally {
    spouseSaving.value = false
  }
  spouseVisible.value = false; ElMessage.success('已保存'); load()
}

async function spouseConvertDate() {
  const d = (spouseForm.birth_date || '').trim(); if (!d) return
  // 只填年份（YYYY）→ 直接存年份，不转换月日
  const mYear = d.match(/^(\d{4})$/)
  if (mYear) {
    spouseForm.birth_gregorian = mYear[1]
    spouseForm.birth_lunar = ''
    spouseForm.birth_ganzhi = ''
    spouseForm.birth_era = ''
    return
  }
  const m = d.match(/^(\d{4})-(\d{2})-(\d{2})$/); if (!m) return
  if (spouseForm.birth_calendar === 'lunar') {
    const { data } = await api.get(`/lunar/to-solar?y=${m[1]}&m=${m[2]}&d=${m[3]}`)
    spouseForm.birth_gregorian = data.gregorian
    spouseForm.birth_lunar = data.lunar; spouseForm.birth_ganzhi = data.ganzhi; spouseForm.birth_era = data.era + data.eraYear
    ElMessage.success(`公历：${data.gregorian}`)
  } else {
    const { data } = await api.get(`/lunar/convert?y=${m[1]}&m=${m[2]}&d=${m[3]}`)
    spouseForm.birth_gregorian = d
    spouseForm.birth_lunar = data.lunar; spouseForm.birth_ganzhi = data.ganzhi; spouseForm.birth_era = data.era + data.eraYear
    ElMessage.success(`农历：${data.lunar}`)
  }
}

async function deleteSpouse(id) {
  await ElMessageBox.confirm('确定删除该配偶记录？', '确认', { type: 'warning' })
  let r
  try {
    r = await api.delete(`/spouses/${id}`)
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '删除失败'
    ElMessage.error('删除失败：' + msg)
    return
  }
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已删除'); load()
}

function openRename() {
  renameForm.surname = member.surname || familySurname.value || ''
  renameForm.given_name = member.given_name || ''
  renameVisible.value = true
}

async function confirmRename() {
  const given = (renameForm.given_name || '').trim()
  if (!given) { ElMessage.warning('名不能为空'); return }
  const surname = member.surname || familySurname.value || ''
  const newName = surname + given
  try {
    await ElMessageBox.confirm(
      `确认将「${member.name}」改名为「${newName}」？\n同名登录账号将一并改名，旧名会记入曾用名。`,
      '改名确认',
      { type: 'warning', confirmButtonText: '确认改名', cancelButtonText: '取消' }
    )
  } catch (e) {
    return // 用户取消
  }
  let data
  try {
    const res = await api.put(`/members/${memberId}/rename`, { surname, given_name: given })
    data = res.data
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '改名失败'
    ElMessage.error('改名失败：' + msg)
    return
  }
  renameVisible.value = false
  if (data && data.draft) { ElMessage.success(data.message || '已提交审核'); return }
  if (data.user_renamed) {
    ElMessage.success('改名成功，同名账号已同步更新')
  } else {
    ElMessage.success('改名成功')
  }
  load()
}

async function onUploadSuccess(resp) { if (resp.id) { ElMessage.success('上传成功'); load() } }
function setAsPortrait(path) {
  cropSrc.value = path
  cropVisible.value = true
}
async function onCropConfirm(blob) {
  const fd = new FormData()
  fd.append('file', blob, 'avatar.png')
  fd.append('member_id', String(memberId))
  fd.append('category', '头像')
  try {
    await api.post('/upload', fd)
    ElMessage.success('已设为头像')
    cropVisible.value = false
    load()
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '设置头像失败'
    ElMessage.error('设置头像失败：' + msg)
  }
}
async function deleteAttachment(id) { await ElMessageBox.confirm('确定删除该附件？', '确认', { type: 'warning' }); await api.delete(`/attachments/${id}`); ElMessage.success('已删除'); load() }

// —— 生平条目 ——
function openBioDialog(e = null) {
  aiPolishResult.value = null
  aiHasRun.value = false
  if (e) {
    Object.assign(bioForm, { id: e.id, title: e.title, content: e.content, sort_order: e.sort_order || 0 })
  } else {
    Object.assign(bioForm, { id: 0, title: '', content: '', sort_order: bioEntries.value.length })
  }
  bioVisible.value = true
}

// AI 优化：把标题+内容发本地模型，排查病句并美化
async function aiPolish() {
  if (!bioForm.content || !bioForm.content.trim()) { ElMessage.warning('请先填写内容'); return }
  aiPolishing.value = true
  aiPolishResult.value = null
  try {
    const { data } = await api.post('/ai/polish-bio', {
      title: bioForm.title || '',
      content: bioForm.content.trim(),
      level: aiConstraint.value,
      degree: aiDegree.value
    }, { timeout: 190000 })
    aiPolishResult.value = { title: data.title || bioForm.title || '', content: data.content || bioForm.content }
    aiHasRun.value = true
    if (data.protected) {
      ElMessage.warning('已拦截 AI 对日期/数字的改动，保留原文（小模型纪年换算不可靠，已自动保护）')
    }
  } catch (e) {
    aiHasRun.value = true
    ElMessage.error(e?.response?.data?.error || 'AI 优化失败，请确认修谱助手已启用并安装模型')
  } finally {
    aiPolishing.value = false
  }
}

// AI 生成小传：把成员档案（结构化字段）交给后端拼提示词生成草稿；结果只填进表单，保存仍走原有审核流程
async function aiGenerateBio() {
  if (bioForm.content && bioForm.content.trim()) {
    try {
      await ElMessageBox.confirm('将根据该成员档案生成小传草稿，并覆盖下方「内容」里已写的文字（可手动修改，或用历史版本恢复）。继续？', '生成小传', { type: 'warning' })
    } catch (e) { return }
  }
  aiGenerating.value = true
  aiPolishResult.value = null
  try {
    const { data } = await api.post('/ai/generate-bio', {
      member_id: Number(memberId),
      style: aiGenStyle.value,
      length: aiGenLength.value,
      level: aiConstraint.value,
      degree: aiDegree.value
    }, { timeout: 190000 })
    aiPolishResult.value = { title: data.title || '生平', content: data.content || '' }
    aiHasRun.value = true
    if (data.protected) {
      ElMessage.warning('AI 对日期/数字的改动已被拦截，保留原文纪年')
    } else {
      ElMessage.success('已生成小传草稿，确认后点「使用这个」填入')
    }
  } catch (e) {
    aiHasRun.value = true
    ElMessage.error(e?.response?.data?.error || 'AI 生成失败，请确认修谱助手已启用并配置好模型')
  } finally {
    aiGenerating.value = false
  }
}

// 采用 AI 结果：withTitle=true 时连标题一起采用；false 时只采用内容，标题保留用户手填的
function useAiText(withTitle) {
  if (!aiPolishResult.value) return
  if (withTitle) {
    if (aiPolishResult.value.title) bioForm.title = aiPolishResult.value.title
  }
  bioForm.content = aiPolishResult.value.content || ''
  ElMessage.success(withTitle ? '已采用 AI 标题与内容' : '已采用 AI 内容（标题保留手动填写）')
}

async function saveBioEntry() {
  if (!bioForm.title || !bioForm.title.trim()) { ElMessage.warning('请输入标题'); return }
  if (bioSaving.value) return
  bioSaving.value = true
  try {
    let r
    if (bioForm.id) {
      r = await api.put(`/bio-entries/${bioForm.id}`, bioForm)
    } else {
      r = await api.post(`/members/${memberId}/bio-entries`, bioForm)
    }
    if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); bioVisible.value = false; return }
    ElMessage.success('已保存')
    bioVisible.value = false
    // 重新加载条目
    const { data } = await api.get(`/members/${memberId}/bio-entries`)
    bioEntries.value = data || []
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '保存失败')
  } finally {
    bioSaving.value = false
  }
}

async function deleteBioEntry(id) {
  try {
    await ElMessageBox.confirm('确定删除该条目？', '确认', { type: 'warning' })
  } catch { return }
  let r
  try {
    r = await api.delete(`/bio-entries/${id}`)
  } catch (e) {
    const msg = e?.response?.data?.error || e.message || '删除失败'
    ElMessage.error('删除失败：' + msg)
    return
  }
  if (r.data && r.data.draft) { ElMessage.success(r.data.message || '已提交审核'); return }
  ElMessage.success('已删除')
  const { data } = await api.get(`/members/${memberId}/bio-entries`)
  bioEntries.value = data || []
}

// —— 资料来源（成员页区块）——
async function loadSources() {
  try {
    const { data } = await api.get(`/members/${memberId}/sources`)
    sources.value = Array.isArray(data) ? data : []
  } catch (e) {
    sources.value = []
  }
}
function openSourceDialog() {
  sourceForm.kind = '族谱旧谱'
  sourceForm.text = ''
  sourceVisible.value = true
}
async function saveSource() {
  const text = (sourceForm.text || '').trim()
  if (!text) { ElMessage.warning('请填写来源内容'); return }
  if (sourceSaving.value) return
  sourceSaving.value = true
  try {
    await api.post(`/members/${memberId}/sources`, { text, kind: (sourceForm.kind || '').trim() || '其他' })
    sourceVisible.value = false
    ElMessage.success('已添加')
    await loadSources()
  } catch (e) {
    ElMessage.error('添加失败：' + (e?.response?.data?.error || e.message || e))
  } finally {
    sourceSaving.value = false
  }
}
async function deleteSource(id) {
  try {
    await ElMessageBox.confirm('确定删除该条资料来源？', '确认', { type: 'warning' })
  } catch { return }
  try {
    await api.delete(`/sources/${id}`)
    ElMessage.success('已删除')
    await loadSources()
  } catch (e) {
    ElMessage.error('删除失败：' + (e?.response?.data?.error || e.message || e))
  }
}

onMounted(load)
</script>

<style scoped>
.section { margin-bottom: 16px; }
.card-header { display: flex; align-items: center; justify-content: space-between; }
.header-left { display: flex; align-items: center; gap: 12px; }
.card-header .serif { font-size: 20px; font-family: "Songti SC", "SimSun", serif; }
.flag-group { display: inline-flex; align-items: center; gap: 6px; margin-right: 8px; cursor: default; }
.flag-label { font-size: 13px; color: #8c5a3c; white-space: nowrap; }
.tag { background: #e6a23c; color: #fff; padding: 2px 8px; border-radius: 4px; font-size: 12px; }
.avatar { border: 2px solid #d9cbb6; }
.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
@media (max-width: 767px) { .info-grid { grid-template-columns: 1fr; } }
.info-item { display: flex; padding: 4px 0; }
.info-item .label { width: 80px; color: #8c5a3c; flex-shrink: 0; }
.info-item em { color: #8c5a3c; font-style: normal; margin-left: 6px; font-size: 12px; }
.clickable { color: #3a3226; cursor: pointer; text-decoration: underline; text-decoration-style: dotted; }
.clickable:hover { color: #8c5a3c; }
.bio { margin-top: 16px; padding-top: 16px; border-top: 1px solid #d9cbb6; }
.bio-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.bio-head-actions { display: flex; align-items: center; gap: 8px; }
.bio h4 { margin-bottom: 8px; color: #8c5a3c; }
.bio-entry { padding: 10px 0; border-bottom: 1px solid #f0ebe0; }
.bio-entry-head { display: flex; align-items: center; justify-content: space-between; }
.bio-title { font-weight: bold; font-size: 15px; color: #3a3226; }
.bio-entry-actions { display: inline-flex; gap: 4px; }
.bio-content { margin-top: 4px; line-height: 1.8; white-space: pre-wrap; }
/* 资料来源（成员页区块）：与「生平」同款分隔线；列表只在有来源时显示 */
.sources { margin-top: 16px; padding-top: 16px; border-top: 1px solid #d9cbb6; }
.sources h4 { margin-bottom: 8px; color: #8c5a3c; }
.sources-add { margin-top: 12px; }
.source-entry { padding: 8px 0; border-bottom: 1px solid #f0ebe0; }
.source-head { display: flex; align-items: center; justify-content: space-between; }
.source-kind { flex-shrink: 0; }
.source-text { margin-top: 4px; line-height: 1.8; white-space: pre-wrap; color: #3a3226; }
.ai-polish-box {
  margin-top: 8px;
  padding: 12px;
  border: 1px dashed #e6a23c;
  border-radius: 8px;
  background: #fdf6ec;
}
.ai-polish-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}
.ai-polish-label { font-weight: 600; color: #8c5a3c; }
.ai-level-row { display: flex; align-items: center; gap: 6px; margin: 6px 0; flex-wrap: wrap; }
.ai-level-label { font-size: 12px; color: #8c5a3c; font-weight: 500; min-width: 56px; }
.ai-use-row { display: flex; justify-content: flex-end; margin-top: 8px; }
.ai-polish-tip { font-size: 12px; color: #8c5a3c; }
.relation-group { padding: 6px 0; border-bottom: 1px solid #f0ebe0; display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
.relation-label { color: #8c5a3c; font-weight: 500; white-space: nowrap; }
.relation-tag { background: #f7f0e8; padding: 2px 10px; border-radius: 12px; font-size: 13px; cursor: pointer; display: inline-flex; align-items: center; gap: 4px; }
.relation-tag small { color: #8c5a3c; font-size: 11px; }
.relation-tag:hover { background: #e8d5b8; }
.media-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 12px; }
.media-item { border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.media-img { width: 100%; height: 140px; }
.media-audio { width: 100%; margin: 8px 0; }
.media-actions { display: flex; justify-content: space-around; padding: 4px; background: #f7f3ec; }
.media-thumb { position: relative; display: inline-block; width: 100%; }
.play-overlay { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.3); display: flex; align-items: center; justify-content: center; border-radius: 8px; }
.play-icon { color: #fff; font-size: 36px; text-shadow: 0 2px 4px rgba(0,0,0,0.5); }
.spouse-item { padding: 10px 0; border-bottom: 1px solid #f0ebe0; }
.spouse-main { display: flex; align-items: center; gap: 8px; }
.spouse-name { font-weight: bold; font-size: 15px; }
.spouse-extra { font-size: 12px; color: #8c5a3c; margin-top: 4px; }
.spouse-actions { margin-top: 4px; text-align: right; }
.child-item { padding: 6px 0; border-bottom: 1px solid #f0ebe0; display: flex; justify-content: space-between; align-items: center; }
.child-link { color: #3a3226; text-decoration: none; font-weight: 500; }
.child-link:hover { color: #8c5a3c; }
.child-sub { font-size: 12px; color: #8c5a3c; }
.former-name { color: #8c5a3c; }
.rename-alert { margin-bottom: 14px; }
.rename-form { margin-top: 6px; }
.rename-tip { color: #8c5a3c; font-size: 13px; margin-top: 4px; }
.diff-old { color: #c45656; }
.diff-new { color: #67c23a; }
.hz-inline { display: flex; align-items: center; gap: 4px; width: 100%; }
</style>