<template>
  <div class="usage-guide">
    <div class="guide-intro">
      <p class="guide-intro-text">欢迎使用族谱管理系统。本说明按功能模块简要介绍各项功能，帮助您快速上手。</p>
    </div>

    <div v-for="group in visibleGroups" :key="group.title" class="guide-group">
      <div class="guide-group-title">
        <span class="guide-group-icon">{{ group.icon }}</span>
        <span>{{ group.title }}</span>
      </div>
      <div class="guide-items">
        <div v-for="item in group.items" :key="item.name" class="guide-item">
          <div class="guide-item-name">{{ item.name }}</div>
          <!-- desc 内含 <b>/<code> 等内联标签（均为源码写死的静态文案），故用 v-html 渲染 -->
          <div class="guide-item-desc" v-html="item.desc"></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  isAdmin: { type: Boolean, default: false }
})

const groups = [
  {
    icon: '📊',
    title: '数据看板',
    items: [
      { name: '数据看板', desc: '系统首页，展示家族人口、支系、世代等核心统计，一屏总览全族概况。' },
      { name: '生日提醒', desc: '按公历/农历自动汇总近期族员生日，方便组织祝寿与问候。' },
      { name: '祭日提醒', desc: '汇总近期族员忌日，便于安排祭祀与缅怀。' }
    ]
  },
  {
    icon: '🌳',
    title: '世系可视化',
    items: [
      { name: '宝塔式', desc: '以宝塔形自上而下展示世系传承，适合查看整体脉络。' },
      { name: '苏式垂吊图', desc: '苏体谱常用的垂吊式世系图，自始祖向下逐代垂挂。' },
      { name: '苏式垂丝图', desc: '垂丝式世系图，以细线串联各代成员，脉络清晰。' },
      { name: '苏氏垂珠体', desc: '苏氏垂珠式世系图，珠串式排版展示世代关系。' },
      { name: '竖排牒记式', desc: '竖排书写的牒记式世系，适合传统竖排阅读习惯。' },
      { name: '横版牒记式', desc: '横排书写的牒记式世系，适合现代横排阅读。' },
      { name: '传代树', desc: '以树状结构展示家族代际传承关系。' }
    ]
  },
  {
    icon: '🗺️',
    title: '地图与地域分析',
    items: [
      { name: '地域分布热力图', desc: '在地图上按地域汇总展示族员分布，支持省份下钻、精确定位落点与光晕显示，手机端可双指缩放。' },
      { name: '在线底图', desc: '底图可选用「在线地图」，无需下载离线包：① <b>免 Key 源</b>（高德街道图·中文 / 高德卫星影像 / Esri 卫星影像）—— 保存后全家族直接可用，无每日额度；② <b>天地图</b>（中文地名/行政区最全，但需在「⚙️ 在线底图设置」填天地图「<b>浏览器端</b>」Key；也可选「服务器中转」，则用「<b>服务端</b>」Key，两者不能混用，且有日配额）。底图源与 Key 保存在服务器、全家族共用，仅理事长/超级管理员可修改。' },
      { name: '离线地图', desc: '离线底图包把瓦片存在服务器，热力图无需联网即可看真底图。把离线包文件（<code>.tar.gz</code>）放到本机，在「系统设置 → 🗺️ 离线地图」点「⬆️ 导入离线包」上传，导入后按<b>上传的文件名</b>列在列表里（像备份那样一行一个），可逐个点「删除」清理；热力图的「底图」下拉里用同样的名字选用。<br />说明：本系统<b>不提供从在线地图批量下载瓦片，也不提供离线包导出/分发</b>（前者违反地图服务条款，可能被封号）；离线数据只从导入获得。' },
    ]
  },
  {
    icon: '👥',
    title: '成员与支系',
    items: [
      { name: '成员管理', desc: '维护族员档案，包括姓名、性别、生卒、字辈、配偶、子女关系等基本信息。' },
      { name: '房支管理', desc: '按房支/支系划分家族结构，管理各支系的信息与归属。' },
      { name: '配偶资料', desc: '独立维护配偶信息，可在世系图中正确显示夫妻关系。' },
      { name: '迁徙事件', desc: '记录族员迁居、分房等重大迁徙事件，可关联房支与涉及成员，是地域分布分析的数据来源。' }
    ]
  },
  {
    icon: '📜',
    title: '谱牒文化',
    items: [
      { name: '字辈派语', desc: '录入并管理家族字辈排行，自动为成员匹配字辈。' },
      { name: '谱头文化', desc: '管理族谱前言、家训族规、谱序等谱头文化内容。' },
      { name: '祠堂', desc: '展示家族祠堂信息，支持三维/全景浏览。' }
    ]
  },
  {
    icon: '🤖',
    title: '智能与协作',
    items: [
      { name: '修谱助手', desc: 'AI 修谱助手，可辅助撰写谱文、整理资料、解答修谱问题。' },
      { name: '审核中心', desc: '集中审核族员提交的修改与新增内容，保证谱牒数据质量。' }
    ]
  },
  {
    icon: '⚙️',
    title: '系统管理',
    items: [
      { name: '家族设置', desc: '配置家族信息、始祖设置、用户权限与授权等核心参数。' },
      { name: '备份管理', desc: '数据的备份与还原，保障谱牒数据安全：每日/每月自动全量备份、手动备份、多媒体快照，以及 GEDCOM 导入导出。' },
      { name: 'GEDCOM 备份与导入', desc: '「📤 备份为 GEDCOM」把全部成员导出/备份为 .ged 文件（不含账号密码，可对外分享）；「📥 导入 GEDCOM」把 .ged 文件中的成员追加进现有族谱，与其它族谱软件互通。' },
      { name: '版本更新', desc: '在线检查并更新系统版本，及时获取新功能与修复。' },
      { name: '服务器设置', desc: '登录安全、AI 助手配置、操作日志等服务器级设置。' }
    ]
  },
  {
    icon: '🔐',
    title: '管理功能',
    adminOnly: true,
    items: [
      { name: '权限分级', desc: '家族职务分六级：游客（仅看数据看板）、族员（查看所有数据）、理事（增改成员）、房支理事（仅管理所属支系）、理事长（任命/罢免理事）、超级管理员（全部权限）。' },
      { name: '账号与初始密码', desc: '族员的登录账号默认为其姓名，初始密码按姓名生成 12 位（新增成员、批量导入后会弹窗显示，请转告本人）；成员首次登录时才自动创建账号，首次登录后需修改密码。' },
      { name: '重置密码', desc: '理事长及以上可在「用户管理」重置用户密码（系统随机生成 12 位），被重置用户下次登录需强制修改；超级管理员可重置理事长密码。' },
      { name: '带账号密码的 GEDCOM 备份', desc: '「🔐 备份为 GEDCOM（带账号密码）」会把成员账号与密码一并写入 .ged（存的是密码哈希），存入「GEDCOM（带账号密码）」分类，仅理事长/超级管理员可见与下载；每日自动备份也会同时生成一份。重新导入该文件即可恢复成员原登录密码（成员已有账号时保留原密码，不会重新生成初始密码），且不会恢复超级管理员角色。该文件含密码信息，请仅留作自己的备份、不要外发。' }
    ]
  }
]

const visibleGroups = computed(() => props.isAdmin ? groups : groups.filter(g => !g.adminOnly))
</script>

<style scoped>
.usage-guide {
  line-height: 1.6;
  color: #3a3226;
}
.guide-intro {
  padding: 12px 16px;
  background: #f7f4ef;
  border-left: 3px solid #8c5a3c;
  border-radius: 4px;
  margin-bottom: 16px;
}
.guide-intro-text {
  margin: 0;
  font-size: 13px;
  color: #6b5b45;
}
.guide-group {
  margin-bottom: 18px;
}
.guide-group-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 15px;
  font-weight: 600;
  color: #3a3226;
  margin-bottom: 8px;
}
.guide-group-icon {
  font-size: 16px;
}
.guide-items {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.guide-item {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 6px 10px;
  background: #fff;
  border: 1px solid #eee4d6;
  border-radius: 6px;
}
.guide-item-name {
  flex-shrink: 0;
  min-width: 88px;
  font-weight: 600;
  font-size: 13px;
  color: #8c5a3c;
}
.guide-item-desc {
  font-size: 13px;
  color: #6b5b45;
}
/* v-html 注入的内容不在 scoped 作用域内，需用 :deep() */
.guide-item-desc :deep(b) {
  color: #3a3226;
  font-weight: 600;
}
.guide-item-desc :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, 'Courier New', monospace;
  font-size: 12px;
  background: #f2ece2;
  padding: 1px 4px;
  border-radius: 3px;
  color: #8c5a3c;
}
@media (max-width: 600px) {
  .guide-item {
    flex-direction: column;
    gap: 2px;
  }
  .guide-item-name {
    min-width: auto;
  }
}
</style>
