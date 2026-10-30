<template>
  <div class="about-page">
    <el-card class="section" shadow="never">
      <template #header><span>关于我们</span></template>
      <div class="about-content">
        <p>本系统为族谱数字化工具，由业余爱好者利用工作之余独立开发维护。作者并非专职程序员，日常另有正职工作，精力所限，功能迭代与问题响应或有延迟，还望海涵。</p>
        <p>本系统采用纯本地部署，数据全部保存在您自己的服务器上，不做任何云端储存，家族信息由您自主掌控、绝不外传。如需多人协作访问，请自行配置端口映射或内网穿透，将服务开放给有需要的族亲。</p>
        <p>若您在使用中遇到问题或有改进建议，欢迎扫描下方二维码添加作者微信沟通。每一份反馈，都是系统持续完善的动力。</p>
        <p>愿这套小小的工具，能帮您把家族的记忆一代代传承下去。</p>
        <div class="qr-box">
          <img :src="qrImg" alt="作者微信二维码" class="qr-img" />
          <div class="qr-tip">扫码添加作者微信</div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import qrImg from '../assets/wechat-qr.png'
</script>

<style scoped>
.about-page {
  max-width: 720px;
  margin: 0 auto;
  padding: 20px;
}
.about-content p {
  line-height: 1.9;
  color: #4a3f2f;
  font-size: 15px;
  margin: 0 0 14px;
  text-align: justify;
}
.qr-box {
  text-align: center;
  margin-top: 24px;
}
.qr-img {
  width: 200px;
  height: auto;
  border: 1px solid #eee;
  border-radius: 8px;
}
.qr-tip {
  margin-top: 10px;
  color: #8a7a5f;
  font-size: 13px;
}
</style>
