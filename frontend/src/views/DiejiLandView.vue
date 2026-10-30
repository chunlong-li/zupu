<script setup>
// 横版牒记式：与竖版 /demo-dieji 共用 DiejiDemoView 组件，仅方向不同（landscape）。
// 独立成页，方便菜单入口、书签、打印纸张各自独立。
import DiejiDemoView from './DiejiDemoView.vue'
</script>

<template>
  <DiejiDemoView orient="landscape" />
</template>
