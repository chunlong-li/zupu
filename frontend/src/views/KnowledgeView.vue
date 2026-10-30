<template>
  <div class="kb-host">
    <iframe v-if="src" :src="src" class="kb-frame" allow="fullscreen; clipboard-write"></iframe>
    <el-empty v-else description="家族知识库未启用或离线，请联系管理员" />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'

const src = ref('')

onMounted(() => {
  const t = localStorage.getItem('token') || ''
  // 经族谱后端反代 /knowledge/* 到传承服务器；带上 token 做 SSO（首次进入会落 Cookie 供子请求续用）
  src.value = '/knowledge/?token=' + encodeURIComponent(t)
})
</script>

<style scoped>
.kb-host { height: 100%; width: 100%; background: #f5f3ee; }
.kb-frame { width: 100%; height: 100%; border: 0; display: block; }
</style>
