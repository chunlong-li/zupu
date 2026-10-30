<template>
  <div class="usage-guide-page">
    <h2 class="page-title">📖 使用说明</h2>
    <p class="page-desc">各功能模块说明，帮助您快速了解系统</p>
    <el-divider />
    <UsageGuideContent :is-admin="isAdminUser" />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import UsageGuideContent from '../components/UsageGuideContent.vue'

const isAdminUser = computed(() => {
  const u = JSON.parse(localStorage.getItem('user') || '{}')
  return u.role === '理事长' || u.role === '超级管理员'
})
</script>

<style scoped>
.usage-guide-page {
  max-width: 900px;
}
.page-title {
  font-size: 18px;
  color: #3a3226;
  margin: 0;
}
.page-desc {
  color: #7a6a52;
  font-size: 13px;
  margin: 8px 0 0;
}
</style>
