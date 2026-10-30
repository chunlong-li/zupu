import { ElMessage } from 'element-plus'

// 处理后端写操作返回：
//  - 若返回 draft:true（理事提交进入审核队列），提示“已提交审核”，返回 true
//  - 否则提示 successMsg，返回 false
export function afterSave(data, successMsg = '已保存') {
  if (data && data.draft) {
    ElMessage.success(data.message || '已提交审核，待审核通过后生效')
    return true
  }
  ElMessage.success(successMsg)
  return false
}
