// 房支树工具：找出某房支下「尚未归入该房支」的后代成员
// （branch_id === 0 且其最近有房支的祖先恰好是该房支）
export function findUnassignedDescendants(members, branchId) {
  const byId = new Map(members.map(m => [m.id, m]))
  const children = new Map()
  for (const m of members) {
    if (m.father_id) {
      if (!children.has(m.father_id)) children.set(m.father_id, [])
      children.get(m.father_id).push(m.id)
    }
  }
  const result = []
  const visited = new Set()
  const queue = []
  for (const m of members) {
    if ((m.branch_id || 0) === branchId) queue.push(m.id)
  }
  while (queue.length) {
    const id = queue.shift()
    if (visited.has(id)) continue
    visited.add(id)
    for (const kid of (children.get(id) || [])) {
      const km = byId.get(kid)
      if (!km) continue
      const kb = km.branch_id || 0
      if (kb === 0) {
        // 未分房后代：记入结果，并继续向下（其子孙可能也未分房）
        result.push(km)
        queue.push(kid)
      } else if (kb === branchId) {
        // 已在本房支：继续向下找未分房后代
        queue.push(kid)
      }
      // 其它房支：分房保护，停止向下
    }
  }
  return result
}
