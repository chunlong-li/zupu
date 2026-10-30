// 窄屏判定（默认 <768px）：祠宇志 / 茔墓志 的新增·编辑抽屉、地图选点需要按屏宽切换布局
import { onMounted, onUnmounted, ref } from 'vue'

export function useIsMobile(breakpoint = 768) {
  const isMobile = ref(false)
  const update = () => {
    isMobile.value = typeof window !== 'undefined' && window.innerWidth < breakpoint
  }
  onMounted(() => {
    update()
    window.addEventListener('resize', update)
  })
  onUnmounted(() => window.removeEventListener('resize', update))
  return isMobile
}
