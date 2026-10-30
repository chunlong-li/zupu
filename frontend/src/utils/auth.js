// 角色权限工具：全局判定当前登录用户是否有编辑权限。
// 角色层级（后端一致）：族员(只读) < 理事 < 理事长 < 超级管理员
// 编辑权限 = 理事及以上（族员一律只读）。

// 内部角色名（用于权限判定，不随别名改变）
const INTERNAL_ROLES = {
  MEMBER: '族员',
  ELDER: '理事',
  BRANCH_ELDER: '房支理事',
  CHIEF: '理事长',
  SUPER_ADMIN: '超级管理员'
}

const EDIT_ROLES = [INTERNAL_ROLES.ELDER, INTERNAL_ROLES.BRANCH_ELDER, INTERNAL_ROLES.CHIEF, INTERNAL_ROLES.SUPER_ADMIN]

// 修谱助手（AI）权限 = 理事长及以上（理事长/超级管理员）
const AI_ROLES = [INTERNAL_ROLES.CHIEF, INTERNAL_ROLES.SUPER_ADMIN]

// 可回滚/审核的角色（理事长及以上）
const CHIEF_ROLES = [INTERNAL_ROLES.CHIEF, INTERNAL_ROLES.SUPER_ADMIN]

// 默认显示名（内部角色→默认显示名）
const DEFAULT_NAMES = {
  [INTERNAL_ROLES.MEMBER]: '族员',
  [INTERNAL_ROLES.ELDER]: '理事',
  [INTERNAL_ROLES.BRANCH_ELDER]: '房支理事',
  [INTERNAL_ROLES.CHIEF]: '理事长',
  [INTERNAL_ROLES.SUPER_ADMIN]: '超级管理员'
}

export function currentRole() {
  try {
    const u = JSON.parse(localStorage.getItem('user') || 'null')
    return (u && u.role) || ''
  } catch (e) {
    return ''
  }
}

// 返回角色的显示名称（优先别名）
export function displayRole(internalRole) {
  const aliases = getRoleAliases()
  if (aliases[internalRole]) return aliases[internalRole]
  return DEFAULT_NAMES[internalRole] || internalRole
}

// 加载角色别名映射
function getRoleAliases() {
  try {
    return JSON.parse(localStorage.getItem('role_aliases') || '{}')
  } catch (e) {
    return {}
  }
}

// 保存角色别名映射
export function saveRoleAliases(aliases) {
  localStorage.setItem('role_aliases', JSON.stringify(aliases))
}

// 是否有编辑权限（理事及以上）
export function canEdit() {
  return EDIT_ROLES.includes(currentRole())
}

// 是否可用修谱助手 AI（理事长及以上）
export function canUseAI() {
  return AI_ROLES.includes(currentRole())
}

// 是否可回滚版本 / 执行审核（理事长及以上）
export function canRollback() {
  return CHIEF_ROLES.includes(currentRole())
}

// 是否是超级管理员
export function isSuperAdmin() {
  return currentRole() === INTERNAL_ROLES.SUPER_ADMIN
}

// 是否可查看/编辑完整 DNA 编码（理事长及以上）
export function canViewDNA() {
  return CHIEF_ROLES.includes(currentRole())
}