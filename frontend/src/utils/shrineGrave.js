// 祠宇志 / 茔墓志：谱书取数（列表 → 详情，含子项与图片）
// 无记录 / 接口失败时返回空数组（该栏目整段不生成，不出现空页）。
import api from '../api'

export async function fetchShrineGrave() {
  const [sRes, gRes] = await Promise.all([
    api.get('/shrines').catch(() => ({ data: { items: [] } })),
    api.get('/graves').catch(() => ({ data: { items: [] } })),
  ])
  const sList = (sRes && sRes.data && sRes.data.items) || []
  const gList = (gRes && gRes.data && gRes.data.items) || []

  const [shrines, graves] = await Promise.all([
    Promise.all(sList.map(x => api.get('/shrines/' + x.id).then(r => r.data).catch(() => null))),
    Promise.all(gList.map(x => api.get('/graves/' + x.id).then(r => r.data).catch(() => null))),
  ])
  return {
    shrines: shrines.filter(Boolean),
    graves: graves.filter(Boolean),
  }
}
