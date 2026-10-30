import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 版本号：正式版 x.y.z / 测试版 x.y.z.mmdd.n —— 必须与后端 5 处手改的版本号一致
// （界面显示以后端 settings 表为准，这里只是未登录/接口失败时的兜底）
const BASE_VERSION = '1.3.1.1005.4'
// 构建时刻（MMDD.HHmm）：仅供静态资源/缩略图的缓存打破（?v=__BUILD_ID__）使用。
// ⚠️ 不要拼进版本号：版本号末段是流水号，不是时间（拼上去会变成 6 段怪号）
function buildStamp() {
  const d = new Date()
  const p = n => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}${p(d.getDate())}.${p(d.getHours())}${p(d.getMinutes())}`
}

export default defineConfig({
  plugins: [vue()],
  define: {
    __BUILD_VERSION__: JSON.stringify(BASE_VERSION),
    __BUILD_ID__: JSON.stringify(buildStamp())
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  },
  build: {
    outDir: 'dist'
  }
})