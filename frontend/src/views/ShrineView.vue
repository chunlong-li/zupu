<template>
  <div class="shrine-page">
    <!-- 顶部：标题 + 筛选 + 新增 -->
    <div class="page-head">
      <div class="title-wrap">
        <span class="serif page-title">🏯 祠宇志</span>
        <span class="sub">祖厝 · 祠堂 · 家庙 —— 实体建筑档案（与菜单「🛕 祠堂」的电子祠堂场景不是一回事）</span>
      </div>
      <div class="actions">
        <el-input
          v-model="keyword" size="small" class="kw" clearable
          placeholder="搜索名称 / 地址 / 规制" @keyup.enter="load" @clear="load"
        />
        <el-select v-model="typeFilter" size="small" class="type-sel" placeholder="全部类型" clearable @change="load">
          <el-option v-for="t in TYPES" :key="t" :label="t" :value="t" />
        </el-select>
        <el-button size="small" @click="load">查询</el-button>
        <el-button v-if="canEdit()" type="primary" size="small" @click="openEdit()">＋ 新增祠宇</el-button>
      </div>
    </div>

    <!-- 旧「祠墓」记录（kind 为空）归入入口，归入后本页可继续完善 -->
    <LegacyCemeteryPanel ref="legacyRef" kind="shrine" module-name="祠宇志" @classified="load" />

    <el-table :data="rows" v-loading="loading" border class="list" @row-click="openDetail">
      <el-table-column label="实景" width="88" align="center">
        <template #default="{ row }">
          <el-image
            v-if="row.cover" :src="thumbUrl(row.cover, THUMB_MINI)"
            :preview-src-list="[fullUrl(row.cover)]" preview-teleported
            fit="cover" class="cover"
          />
          <span v-else class="none">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="name" label="名称" min-width="150" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="serif name-link">{{ row.name }}</span>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="88" align="center">
        <template #default="{ row }"><el-tag size="small" effect="plain">{{ row.shrine_type || '祠堂' }}</el-tag></template>
      </el-table-column>
      <el-table-column prop="address" label="地址" min-width="150" show-overflow-tooltip>
        <template #default="{ row }"><span>{{ row.address || '—' }}</span></template>
      </el-table-column>
      <el-table-column label="始建/重修" width="96" align="center">
        <template #default="{ row }"><span>{{ row.history_count || 0 }} 条</span></template>
      </el-table-column>
      <el-table-column label="楹联匾额" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          <div v-if="row.couplet_preview" class="couplet-preview">{{ row.couplet_preview }}</div>
          <span v-if="row.couplet_count" class="couplet-count">{{ row.couplet_count }} 条</span>
          <span v-else class="none">—</span>
        </template>
      </el-table-column>
      <el-table-column label="族产公田" width="96" align="center">
        <template #default="{ row }"><span>{{ row.property_count || 0 }} 条</span></template>
      </el-table-column>
      <el-table-column label="坐标" width="84" align="center">
        <template #default="{ row }">
          <el-tag v-if="hasCoord(row.lng, row.lat)" size="small" type="success" effect="plain">已填</el-tag>
          <el-tag v-else size="small" type="info" effect="plain">未填</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="196" align="center" fixed="right">
        <template #default="{ row }">
          <el-button size="small" text type="primary" @click.stop="openDetail(row)">详情</el-button>
          <el-button v-if="canEdit()" size="small" text @click.stop="openEdit(row)">编辑</el-button>
          <el-button v-if="canEdit()" size="small" text type="danger" @click.stop="del(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="!loading && !rows.length" description="还没有祠宇记录 —— 点右上「＋ 新增祠宇」开始" />

    <!-- ============ 详情抽屉 ============ -->
    <el-drawer v-model="detailVisible" :size="isMobile ? '100%' : '760px'" :with-header="false" class="detail-drawer">
      <div v-if="detail" class="drawer-body">
        <div class="drawer-head">
          <div class="head-left">
            <span class="serif head-name">{{ detail.name }}</span>
            <el-tag size="small" effect="plain">{{ detail.shrine_type || '祠堂' }}</el-tag>
          </div>
          <div class="head-actions">
            <el-tooltip :content="detailCoordOk ? '' : '未填写坐标'" :disabled="detailCoordOk" placement="top">
              <span class="btn-wrap">
                <el-button size="small" :disabled="!detailCoordOk" @click="openMap(detail)">🗺️ 地图查看</el-button>
              </span>
            </el-tooltip>
            <el-button v-if="canEdit()" size="small" type="primary" @click="openEdit(detail)">编辑</el-button>
            <el-button size="small" text @click="detailVisible = false">关闭</el-button>
          </div>
        </div>

        <div class="info-grid">
          <div class="info-item"><span class="label">类型</span><span>{{ detail.shrine_type || '祠堂' }}</span></div>
          <div class="info-item"><span class="label">地址</span><span>{{ detail.address || '—' }}</span></div>
          <div class="info-item"><span class="label">坐标</span><span>{{ fmtCoord(detail.lng, detail.lat) || '—' }}</span></div>
          <div class="info-item"><span class="label">排序值</span><span>{{ detail.sort_order || 0 }}</span></div>
          <div class="info-item"><span class="label">实景图</span><span>{{ (detail.photos || []).length }} 张</span></div>
          <div class="info-item"><span class="label">平面图</span><span>{{ (detail.plan_images || []).length }} 张</span></div>
          <div class="info-item" v-if="detail.updated_at"><span class="label">更新时间</span><span>{{ detail.updated_at }}</span></div>
        </div>

        <div class="sec">
          <div class="sec-title serif">始建与重修历史</div>
          <el-timeline v-if="(detail.history || []).length" class="his-timeline">
            <el-timeline-item v-for="(h, i) in detail.history" :key="i" :timestamp="h.year || '年代待考'" placement="top">
              <div class="his-title">{{ h.event || '记事' }}</div>
              <div class="his-note" v-if="h.note">{{ h.note }}</div>
            </el-timeline-item>
          </el-timeline>
          <div v-else class="empty-line">— 未填写 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">建筑规制</div>
          <div class="free-text" v-if="detail.architecture">{{ detail.architecture }}</div>
          <div v-else class="empty-line">— 未填写 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">楹联匾额</div>
          <template v-if="(detail.couplets || []).length">
            <el-table v-if="!isMobile" :data="detail.couplets" size="small" border>
              <el-table-column label="位置（类型）" width="180">
                <template #default="{ row }">
                  <span class="couplet-pos">{{ coupletLabel(row) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="内容">
                <template #default="{ row }">
                  <span :class="['couplet-content', row.kind === '匾额' ? 'plaque' : (row.kind === '对联' ? 'distich' : '')]">{{ row.content }}</span>
                </template>
              </el-table-column>
            </el-table>
            <!-- 手机：每条一段（位置·类型 在上，内容在下，长联不挤） -->
            <div v-else class="detail-cards">
              <div class="detail-card" v-for="(c, i) in detail.couplets" :key="'dc' + i">
                <div class="detail-card-tag">{{ coupletLabel(c) }}</div>
                <div :class="['couplet-content', c.kind === '匾额' ? 'plaque' : (c.kind === '对联' ? 'distich' : '')]">{{ c.content }}</div>
              </div>
            </div>
          </template>
          <div v-else class="empty-line">— 未填写 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">神位排布</div>
          <div class="free-text" v-if="detail.spirit_layout">{{ detail.spirit_layout }}</div>
          <div v-else class="empty-line">— 未填写 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">祭祀仪轨</div>
          <div class="free-text" v-if="detail.ritual">{{ detail.ritual }}</div>
          <div v-else class="empty-line">— 未填写 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">族产公田</div>
          <template v-if="(detail.properties || []).length">
            <el-table v-if="!isMobile" :data="detail.properties" size="small" border>
              <el-table-column prop="name" label="名称" width="180" />
              <el-table-column prop="area" label="面积" width="120" />
              <el-table-column prop="note" label="说明" />
            </el-table>
            <div v-else class="detail-cards">
              <div class="detail-card" v-for="(pr, i) in detail.properties" :key="'dp' + i">
                <div class="detail-card-tag">{{ pr.name || '未命名' }}<span v-if="pr.area"> · {{ pr.area }}</span></div>
                <div class="free-text">{{ pr.note || '—' }}</div>
              </div>
            </div>
          </template>
          <div v-else class="empty-line">— 未填写 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">实景图</div>
          <div class="img-show" v-if="(detail.photos || []).length">
            <el-image
              v-for="(p, i) in detail.photos" :key="'p' + i"
              :src="thumbUrl(p, THUMB_LIST)" :preview-src-list="detail.photos.map(fullUrl)"
              :initial-index="i" preview-teleported fit="cover" class="show-img"
            />
          </div>
          <div v-else class="empty-line">— 未上传 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">平面布局图</div>
          <div class="img-show" v-if="(detail.plan_images || []).length">
            <el-image
              v-for="(p, i) in detail.plan_images" :key="'pl' + i"
              :src="thumbUrl(p, THUMB_LIST)" :preview-src-list="detail.plan_images.map(fullUrl)"
              :initial-index="i" preview-teleported fit="cover" class="show-img"
            />
          </div>
          <div v-else class="empty-line">— 未上传 —</div>
        </div>

        <div class="sec">
          <div class="sec-title serif">备注</div>
          <div class="free-text" v-if="detail.remark">{{ detail.remark }}</div>
          <div v-else class="empty-line">— 未填写 —</div>
        </div>
      </div>
    </el-drawer>

    <!-- ============ 编辑抽屉 ============ -->
    <el-drawer v-model="editVisible" :size="isMobile ? '100%' : '760px'" :with-header="false" class="edit-drawer">
      <div class="drawer-body">
        <div class="drawer-head">
          <span class="serif head-name">{{ form.id ? '编辑祠宇' : '新增祠宇' }}</span>
          <div class="head-actions" v-if="!isMobile">
            <el-button size="small" @click="editVisible = false">取消</el-button>
            <el-button v-if="canEdit()" size="small" type="primary" :loading="saving" @click="save">保存</el-button>
          </div>
        </div>

        <el-form :model="form" :label-width="isMobile ? '0px' : '96px'" :label-position="isMobile ? 'top' : 'right'" class="edit-form">
          <el-form-item label="名称" required>
            <el-input v-model="form.name" placeholder="如：李氏宗祠 / 下厝祖厝" maxlength="120" />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="form.shrine_type" class="w-full">
              <el-option v-for="t in TYPES" :key="t" :label="t" :value="t" />
            </el-select>
          </el-form-item>
          <el-form-item label="地址">
            <el-input v-model="form.address" placeholder="如：福建省泉州市某镇某村" maxlength="240" />
          </el-form-item>
          <el-form-item label="坐标">
            <div class="coord-row">
              <el-input v-model.number="form.lng" placeholder="经度 lng" class="coord-in" />
              <el-input v-model.number="form.lat" placeholder="纬度 lat" class="coord-in" />
              <el-button size="small" type="primary" plain @click="mapPickVisible = true">🗺️ 地图选点</el-button>
              <el-button size="small" @click="pasteCoords">粘贴链接提取</el-button>
              <el-button size="small" text @click="clearCoords">清空</el-button>
            </div>
            <div class="hint">可点「🗺️ 地图选点」在在线地图上单击取点（默认高德 GCJ-02）；也可填「116.397,39.909」，或粘贴高德/百度/腾讯地图链接后点「粘贴链接提取」；留空则详情页地图按钮置灰。</div>
          </el-form-item>

          <div class="sub-block">
            <div class="sub-head">
              <span class="serif">始建与重修历史</span>
              <el-button size="small" @click="form.history.push({ year: '', event: '始建', note: '' })">＋ 添加一条</el-button>
            </div>
            <el-table v-if="!isMobile" :data="form.history" size="small" border>
              <el-table-column label="年份" width="150">
                <template #default="{ row }"><el-input v-model="row.year" placeholder="乾隆三十五年" /></template>
              </el-table-column>
              <el-table-column label="事件" width="130">
                <template #default="{ row }">
                  <el-select v-model="row.event" filterable allow-create default-first-option>
                    <el-option v-for="e in EVENTS" :key="e" :label="e" :value="e" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="说明">
                <template #default="{ row }"><el-input v-model="row.note" placeholder="发起人 / 捐资 / 规模等" /></template>
              </el-table-column>
              <el-table-column width="56" align="center">
                <template #default="{ $index }">
                  <el-button size="small" text type="danger" @click="form.history.splice($index, 1)">删</el-button>
                </template>
              </el-table-column>
            </el-table>
            <!-- 手机：每条一张卡片（竖向排，不横向挤） -->
            <div v-else class="sub-cards">
              <div class="sub-card" v-for="(row, i) in form.history" :key="'h' + i">
                <div class="sub-card-head">
                  <span>第 {{ i + 1 }} 条 · 始建/重修</span>
                  <el-button size="small" type="danger" plain @click="form.history.splice(i, 1)">删除</el-button>
                </div>
                <div class="sub-field">
                  <label>年份</label>
                  <el-input v-model="row.year" placeholder="乾隆三十五年" />
                </div>
                <div class="sub-field">
                  <label>事件</label>
                  <el-select v-model="row.event" filterable allow-create default-first-option class="w-full">
                    <el-option v-for="e in EVENTS" :key="e" :label="e" :value="e" />
                  </el-select>
                </div>
                <div class="sub-field">
                  <label>说明</label>
                  <el-input v-model="row.note" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" placeholder="发起人 / 捐资 / 规模等" />
                </div>
              </div>
              <div v-if="!form.history.length" class="sub-empty">还没有记录，点上方「＋ 添加一条」</div>
            </div>
          </div>

          <el-form-item label="建筑规制" class="mt">
            <el-input
              v-model="form.architecture" type="textarea" :autosize="{ minRows: 2, maxRows: 10 }"
              placeholder="坐向、面阔/进深、结构、占地等，如：坐北朝南，三开间两进，抬梁式木构，占地约 320 平方米"
            />
          </el-form-item>

          <div class="sub-block">
            <div class="sub-head">
              <span class="serif">楹联匾额</span>
              <el-button size="small" @click="form.couplets.push({ kind: '对联', position: '', content: '' })">＋ 添加一条</el-button>
            </div>
            <div class="sub-hint">先选「类型」（匾额 / 对联 / 其他），「位置」会按类型给常见值；找不到对应分类时选「其他」自己填。</div>
            <el-table v-if="!isMobile" :data="form.couplets" size="small" border>
              <el-table-column label="类型" width="110">
                <template #default="{ row }">
                  <el-select v-model="row.kind" placeholder="未分类" @change="onCoupletKindChange(row)">
                    <el-option v-for="k in COUPLET_KINDS" :key="k" :label="k" :value="k" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="位置" width="150">
                <template #default="{ row }">
                  <el-select v-model="row.position" filterable allow-create default-first-option clearable placeholder="如 大门联">
                    <el-option v-for="p in coupletPositions(row)" :key="p" :label="p" :value="p" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="内容">
                <template #default="{ row }"><el-input v-model="row.content" type="textarea" :rows="1" placeholder="联语 / 匾额文字" /></template>
              </el-table-column>
              <el-table-column width="56" align="center">
                <template #default="{ $index }">
                  <el-button size="small" text type="danger" @click="form.couplets.splice($index, 1)">删</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div v-else class="sub-cards">
              <div class="sub-card" v-for="(row, i) in form.couplets" :key="'c' + i">
                <div class="sub-card-head">
                  <span>第 {{ i + 1 }} 条 · 楹联匾额</span>
                  <el-button size="small" type="danger" plain @click="form.couplets.splice(i, 1)">删除</el-button>
                </div>
                <div class="sub-field">
                  <label>类型</label>
                  <el-select v-model="row.kind" placeholder="未分类" class="w-full" @change="onCoupletKindChange(row)">
                    <el-option v-for="k in COUPLET_KINDS" :key="k" :label="k" :value="k" />
                  </el-select>
                </div>
                <div class="sub-field">
                  <label>位置</label>
                  <el-select v-model="row.position" filterable allow-create default-first-option clearable placeholder="如 大门联" class="w-full">
                    <el-option v-for="p in coupletPositions(row)" :key="p" :label="p" :value="p" />
                  </el-select>
                </div>
                <div class="sub-field">
                  <label>内容</label>
                  <el-input v-model="row.content" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" placeholder="联语 / 匾额文字" />
                </div>
              </div>
              <div v-if="!form.couplets.length" class="sub-empty">还没有记录，点上方「＋ 添加一条」</div>
            </div>
          </div>

          <el-form-item label="神位排布">
            <el-input v-model="form.spirit_layout" type="textarea" :autosize="{ minRows: 2, maxRows: 8 }" placeholder="如：正中始祖，左昭右穆，按世次依次排列" />
          </el-form-item>
          <el-form-item label="祭祀仪轨">
            <el-input v-model="form.ritual" type="textarea" :autosize="{ minRows: 2, maxRows: 10 }" placeholder="祭期（如清明、冬至）、仪程、供品等" />
          </el-form-item>

          <div class="sub-block">
            <div class="sub-head">
              <span class="serif">族产公田</span>
              <el-button size="small" @click="form.properties.push({ name: '', area: '', note: '' })">＋ 添加一条</el-button>
            </div>
            <el-table v-if="!isMobile" :data="form.properties" size="small" border>
              <el-table-column label="名称" width="180">
                <template #default="{ row }"><el-input v-model="row.name" placeholder="如：祠前公田" /></template>
              </el-table-column>
              <el-table-column label="面积" width="130">
                <template #default="{ row }"><el-input v-model="row.area" placeholder="如：三亩二分" /></template>
              </el-table-column>
              <el-table-column label="说明">
                <template #default="{ row }"><el-input v-model="row.note" placeholder="坐落、租佃、收益用途等" /></template>
              </el-table-column>
              <el-table-column width="56" align="center">
                <template #default="{ $index }">
                  <el-button size="small" text type="danger" @click="form.properties.splice($index, 1)">删</el-button>
                </template>
              </el-table-column>
            </el-table>
            <div v-else class="sub-cards">
              <div class="sub-card" v-for="(row, i) in form.properties" :key="'p' + i">
                <div class="sub-card-head">
                  <span>第 {{ i + 1 }} 条 · 族产公田</span>
                  <el-button size="small" type="danger" plain @click="form.properties.splice(i, 1)">删除</el-button>
                </div>
                <div class="sub-field">
                  <label>名称</label>
                  <el-input v-model="row.name" placeholder="如：祠前公田" />
                </div>
                <div class="sub-field">
                  <label>面积</label>
                  <el-input v-model="row.area" placeholder="如：三亩二分" />
                </div>
                <div class="sub-field">
                  <label>说明</label>
                  <el-input v-model="row.note" type="textarea" :autosize="{ minRows: 2, maxRows: 6 }" placeholder="坐落、租佃、收益用途等" />
                </div>
              </div>
              <div v-if="!form.properties.length" class="sub-empty">还没有记录，点上方「＋ 添加一条」</div>
            </div>
          </div>

          <el-form-item label="实景图" class="mt">
            <ImageListInput v-model="form.photos" category="祠宇" :can-upload="canEdit()" cover-badge
              tip="可多张；第一张作为列表封面。上传后自动进「家族多媒体」（分类：祠宇）。" />
          </el-form-item>
          <el-form-item label="平面布局图">
            <ImageListInput v-model="form.plan_images" category="祠宇" :can-upload="canEdit()" :limit="6"
              tip="单张或少量（平面图 / 布局图）" />
          </el-form-item>

          <el-form-item label="备注">
            <el-input v-model="form.remark" type="textarea" :autosize="{ minRows: 2, maxRows: 8 }" placeholder="其他要记的事（保存于 cemeteries.description）" />
          </el-form-item>
          <el-form-item label="排序值">
            <el-input v-model.number="form.sort_order" class="coord-in" placeholder="0" />
            <span class="hint inline">列表按「排序值 → 名称」排，越小越靠前</span>
          </el-form-item>
        </el-form>

        <!-- 手机端：底部操作条吸底（带安全区，键盘弹出也不被遮） -->
        <div v-if="isMobile" class="drawer-bottom-bar">
          <el-button size="large" @click="editVisible = false">取消</el-button>
          <el-button v-if="canEdit()" size="large" type="primary" :loading="saving" @click="save">保存</el-button>
        </div>
      </div>
    </el-drawer>

    <!-- 地图选点（复用「热力图 → 底图设置」的在线底图配置；默认高德 GCJ-02） -->
    <MapPicker
      v-model:visible="mapPickVisible"
      :lng="form.lng" :lat="form.lat" :name="form.name"
      @confirm="onPickCoord"
    />
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '../api'
import { canEdit } from '../utils/auth'
import { thumbUrl, THUMB_MINI, THUMB_LIST } from '../utils/thumb'
import { hasCoord, fmtCoord, parseCoords, amapMarkerUrl } from '../utils/geo'
import ImageListInput from '../components/ImageListInput.vue'
import LegacyCemeteryPanel from '../components/LegacyCemeteryPanel.vue'
import { useIsMobile } from '../utils/useIsMobile'
import MapPicker from '../components/MapPicker.vue'

const TYPES = ['祖厝', '祠堂', '家庙', '其它']
const EVENTS = ['始建', '重建', '重修', '扩建', '迁建', '改建', '其他']
// 楹联匾额的「类型」：匾额 / 对联 / 其他（其他＝找不到对应分类时兜底；空＝未分类，兼容老数据）
const COUPLET_KINDS = ['匾额', '对联', '其他']
const COUPLET_POSITIONS = {
  匾额: ['大门额', '正厅横匾', '堂号匾', '神龛匾', '其它'],
  对联: ['大门联', '神龛联', '厅柱联', '龛柱联', '其它'],
  其他: ['大门额', '正厅横匾', '堂号匾', '神龛匾', '大门联', '神龛联', '厅柱联', '龛柱联', '其它']
}
// 按当前类型给「位置」的常见值；类型为空/未分类时给全部
function coupletPositions(row) {
  return COUPLET_POSITIONS[row && row.kind] || COUPLET_POSITIONS['其他']
}
// 详情里显示成「大门联（对联）」这种形式
function coupletLabel(x) {
  const pos = (x && x.position) ? String(x.position).trim() : '未填位置'
  const kind = (x && x.kind) ? String(x.kind).trim() : '未分类'
  return pos + '（' + kind + '）'
}
// 切换类型时：若原位置不属于新类型的常见值，保留用户已填内容（不覆盖），只做提示性清理
function onCoupletKindChange(row) {
  const allowed = coupletPositions(row)
  if (row.position && !allowed.includes(row.position) && row.kind) {
    // 用户从常填值切类型（如 大门联→匾额）时，清掉不适配的常见值，避免张冠李戴；自定义文本保留
    const allCommon = new Set([].concat(...Object.values(COUPLET_POSITIONS)))
    if (allCommon.has(row.position)) row.position = ''
  }
}

const rows = ref([])
const loading = ref(false)
const saving = ref(false)
const keyword = ref('')
const typeFilter = ref('')
const legacyRef = ref(null)
const mapPickVisible = ref(false)
// 窄屏（<768px）：抽屉全宽、表单项上下排、子表改卡片、底部操作条吸底
const isMobile = useIsMobile()

const detailVisible = ref(false)
const detail = ref(null)
const editVisible = ref(false)
const form = reactive(emptyForm())

function emptyForm() {
  return {
    id: 0, name: '', shrine_type: '祠堂', address: '', lng: null, lat: null,
    architecture: '', spirit_layout: '', ritual: '', remark: '', photos: [], plan_images: [],
    sort_order: 0, history: [], couplets: [], properties: []
  }
}

function fullUrl(p) {
  const s = String(p || '')
  if (!s) return ''
  return s.startsWith('/') ? s : (s.startsWith('uploads/') ? '/' + s : '/uploads/' + s)
}
const detailCoordOk = computed(() => !!detail.value && hasCoord(detail.value.lng, detail.value.lat))

async function load() {
  loading.value = true
  try {
    const { data } = await api.get('/shrines', { params: { keyword: keyword.value || undefined, type: typeFilter.value || undefined } })
    rows.value = (data && data.items) || []
  } catch (e) {
    ElMessage.error('读取祠宇志失败：' + (e?.response?.data?.error || e?.message || e))
    rows.value = []
  } finally {
    loading.value = false
  }
}

async function openDetail(row) {
  try {
    const { data } = await api.get('/shrines/' + row.id)
    detail.value = data
    detailVisible.value = true
  } catch (e) {
    ElMessage.error('读取详情失败：' + (e?.response?.data?.error || e?.message || e))
  }
}

function openEdit(row) {
  const src = row || null
  Object.assign(form, emptyForm())
  if (src) {
    // 列表行没有正文，编辑时按 id 拉一次完整详情
    api.get('/shrines/' + src.id).then(({ data }) => {
      Object.assign(form, {
        id: data.id, name: data.name || '', shrine_type: data.shrine_type || '祠堂',
        address: data.address || '', lng: data.lng || null, lat: data.lat || null,
        architecture: data.architecture || '', spirit_layout: data.spirit_layout || '',
        ritual: data.ritual || '', remark: data.remark || '',
        photos: [...(data.photos || [])], plan_images: [...(data.plan_images || [])],
        sort_order: data.sort_order || 0,
        history: (data.history || []).map((x) => ({ year: x.year, event: x.event, note: x.note })),
        couplets: (data.couplets || []).map((x) => ({ kind: x.kind || '', position: x.position, content: x.content })),
        properties: (data.properties || []).map((x) => ({ name: x.name, area: x.area, note: x.note }))
      })
      editVisible.value = true
    }).catch((e) => {
      ElMessage.error('读取详情失败：' + (e?.response?.data?.error || e?.message || e))
    })
    return
  }
  editVisible.value = true
}

function clearCoords() {
  form.lng = null
  form.lat = null
}

// 粘贴链接/坐标 → 提取经纬度：优先读剪贴板（https/localhost 可用），读不到就弹输入框
async function pasteCoords() {
  let text = ''
  try {
    if (navigator.clipboard && navigator.clipboard.readText) {
      text = await navigator.clipboard.readText()
    }
  } catch (e) { text = '' }
  if (!text) {
    try {
      const r = await ElMessageBox.prompt('把高德/百度/腾讯地图链接或「经度,纬度」粘贴到这里', '粘贴坐标', {
        confirmButtonText: '提取', cancelButtonText: '取消', inputPlaceholder: 'https://uri.amap.com/marker?position=116.397,39.909'
      })
      text = r.value || ''
    } catch (e) { return }
  }
  const p = parseCoords(text)
  if (!p) {
    ElMessage.warning('没能从这段内容里认出坐标，请手动填写经纬度')
    return
  }
  form.lng = p.lng
  form.lat = p.lat
  ElMessage.success(`已提取坐标：${p.lng}, ${p.lat}`)
}

async function save() {
  if (!String(form.name || '').trim()) {
    ElMessage.warning('请填写祠宇名称')
    return
  }
  saving.value = true
  try {
    const payload = {
      ...form,
      name: String(form.name).trim(),
      lng: Number(form.lng) || 0,
      lat: Number(form.lat) || 0,
      sort_order: Number(form.sort_order) || 0
    }
    let res
    if (form.id) res = await api.put('/shrines/' + form.id, payload)
    else res = await api.post('/shrines', payload)
    ElMessage.success('已保存')
    editVisible.value = false
    await load()
    if (detailVisible.value) detail.value = res.data
    if (legacyRef.value) legacyRef.value.load()
  } catch (e) {
    ElMessage.error('保存失败：' + (e?.response?.data?.error || e?.message || e))
  } finally {
    saving.value = false
  }
}

async function del(row) {
  try {
    await ElMessageBox.confirm(`删除祠宇「${row.name}」？始建重修 / 楹联 / 族产等子项会一起删除，不可恢复。`, '确认删除', {
      type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger'
    })
  } catch (e) { return }
  try {
    await api.delete('/shrines/' + row.id)
    ElMessage.success('已删除')
    if (detail.value && detail.value.id === row.id) detailVisible.value = false
    await load()
  } catch (e) {
    ElMessage.error('删除失败：' + (e?.response?.data?.error || e?.message || e))
  }
}

function openMap(item) {
  const url = amapMarkerUrl(item.lng, item.lat, item.name)
  if (!url) { ElMessage.warning('未填写坐标'); return }
  window.open(url, '_blank')
}

// 地图选点回填（底图坐标系即存库坐标系；默认高德 = GCJ-02，与「祭扫导航」一致）
function onPickCoord(p) {
  if (!p) return
  form.lng = p.lng
  form.lat = p.lat
  ElMessage.success(`已选点：${p.lng}, ${p.lat}`)
}

onMounted(load)
</script>

<style scoped>
.shrine-page { min-height: 100%; }
.page-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.title-wrap { display: flex; align-items: baseline; gap: 10px; flex-wrap: wrap; }
.page-title { font-size: 20px; color: #3a3226; }
.sub { font-size: 12px; color: #9a8a74; }
.actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.kw { width: 200px; }
.type-sel { width: 120px; }
.list { background: #fff; }
.list :deep(.el-table__row) { cursor: pointer; }
.cover { width: 46px; height: 46px; border-radius: 4px; }
.name-link { color: #3a3226; font-size: 15px; }
.none { color: #bbb; }
.hint { font-size: 12px; color: #9a8a74; line-height: 1.5; }
.hint.inline { margin-left: 8px; }

/* 抽屉 */
.drawer-body { padding: 16px 20px 28px; }
.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  background: #fff;
  z-index: 5;
  flex-wrap: wrap;
}
.head-left { display: flex; align-items: center; gap: 10px; }
.head-name { font-size: 20px; color: #3a3226; }
.head-actions { display: flex; align-items: center; gap: 8px; }
.btn-wrap { display: inline-block; }
.info-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin: 14px 0 6px;
}
@media (max-width: 767px) { .info-grid { grid-template-columns: 1fr; } }
.info-item { display: flex; padding: 4px 0; font-size: 14px; }
.info-item .label { width: 80px; color: #8c5a3c; flex-shrink: 0; }
.sec { margin-top: 18px; padding-top: 12px; border-top: 1px dashed var(--border); }
.sec-title { font-size: 16px; color: #8c5a3c; margin-bottom: 10px; }
.free-text { font-size: 14px; line-height: 1.9; white-space: pre-wrap; color: #3a3226; }
.empty-line { font-size: 13px; color: #b0a494; }
.his-timeline { padding-left: 4px; }
.his-title { font-size: 14px; color: #3a3226; }
.his-note { font-size: 13px; color: #7a6a52; margin-top: 2px; white-space: pre-wrap; }
.couplet-preview { font-size: 13px; color: #3a3226; }
.couplet-count { font-size: 12px; color: #9a8a74; }
.couplet-pos { color: #8c5a3c; }
.couplet-content.plaque { letter-spacing: 2px; font-weight: 600; }
.couplet-content.distich { letter-spacing: 1px; }
.sub-hint { font-size: 12px; color: #9a8a74; margin: 0 0 6px; }

/* ============ 手机端（<768px）============ */
@media (max-width: 767px) {
  /* 抽屉内容：底部留出吸底操作条的高度 */
  .drawer-body { padding: 12px 12px calc(84px + env(safe-area-inset-bottom)); }
  .drawer-head { padding-bottom: 8px; }
  .head-name { font-size: 17px; }
  .head-actions { flex-wrap: wrap; gap: 6px; }
  /* 详情/编辑抽屉头部的按钮也放大到 ≥40px（手机上更好点） */
  .head-actions .el-button { min-height: 40px; }

  /* 表单项：label 上下排 */
  .edit-form :deep(.el-form-item) { margin-bottom: 14px; }
  .edit-form :deep(.el-form-item__label) {
    font-size: 13px; line-height: 1.6; color: #8c5a3c;
    padding: 0 0 2px; margin-bottom: 2px; width: 100% !important; justify-content: flex-start;
  }
  .edit-form :deep(.el-form-item__content) { line-height: 1.5; }

  /* 输入框字号 ≥16px：防 iOS 聚焦自动放大 */
  .edit-form :deep(.el-input__inner),
  .edit-form :deep(.el-textarea__inner),
  .edit-form :deep(.el-select__selected-item),
  .edit-form :deep(.el-select__placeholder) { font-size: 16px; }
  .edit-form :deep(.el-textarea__inner) { line-height: 1.7; }

  /* 坐标行：按钮换行、按钮够大、不超出屏幕 */
  .coord-row { gap: 8px; }
  .coord-in { width: calc(50% - 4px); }
  .coord-row .el-button { min-height: 40px; flex: 1 1 auto; margin-left: 0; }

  /* 子表 → 卡片 */
  .sub-head { flex-wrap: wrap; gap: 6px; }
  .sub-head .el-button { min-height: 40px; }
  .sub-cards { display: flex; flex-direction: column; gap: 10px; }
  .sub-card { border: 1px solid var(--border); border-radius: 8px; padding: 10px 10px 4px; background: #fff; }
  .sub-card-head {
    display: flex; align-items: center; justify-content: space-between;
    margin-bottom: 8px; color: #8c5a3c; font-size: 13px;
  }
  .sub-card-head .el-button { min-height: 40px; min-width: 64px; }
  .sub-field { margin-bottom: 10px; }
  .sub-field > label { display: block; font-size: 12px; color: #8c5a3c; margin-bottom: 4px; }
  .sub-empty { font-size: 13px; color: #b0a494; padding: 6px 2px; }

  /* 详情：卡片式列表 + 图片一行 3 张 */
  .detail-cards { display: flex; flex-direction: column; gap: 8px; }
  .detail-card { border: 1px solid var(--border); border-radius: 8px; padding: 8px 10px; background: #fff; }
  .detail-card-tag { font-size: 12px; color: #8c5a3c; margin-bottom: 4px; }
  .info-grid { grid-template-columns: 1fr; gap: 4px; }
  .info-item { font-size: 14px; }
  .info-item .label { width: 76px; }
  .img-show { gap: 8px; }
  .img-show .show-img { width: calc((100% - 16px) / 3); height: auto; aspect-ratio: 1 / 1; }
  .sec-title { font-size: 15px; }
  .free-text, .epitaph { font-size: 14px; }

  /* 底部操作条：吸底 + 安全区 + 大按钮 */
  .drawer-bottom-bar {
    position: fixed; left: 0; right: 0; bottom: 0; z-index: 3000;
    display: flex; gap: 10px;
    padding: 10px 12px calc(10px + env(safe-area-inset-bottom));
    background: #fff; border-top: 1px solid var(--border);
    box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.06);
  }
  .drawer-bottom-bar .el-button { flex: 1; min-height: 44px; font-size: 16px; margin-left: 0; }

  /* 页面本身不要出现横向滚动 */
  .shrine-page { overflow-x: hidden; }
}
.img-show { display: flex; flex-wrap: wrap; gap: 10px; }
.show-img { width: 124px; height: 124px; border-radius: 6px; border: 1px solid var(--border); cursor: zoom-in; }

/* 编辑表单 */
.edit-form { margin-top: 14px; }
.coord-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.coord-in { width: 130px; }
.w-full { width: 100%; }
.sub-block { margin: 12px 0 18px; }
.sub-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  color: #8c5a3c;
  font-size: 15px;
}
.mt { margin-top: 10px; }
</style>
