import api from '../api'
import { ElMessageBox } from 'element-plus'
import { canUseAI } from '../utils/auth'

// 保存前 AI 校验：把人物资料文本交给本地模型检查。
// 成功返回 { ok, problems }；模型未启用(403)、无权限或调用失败时返回 null。
export async function aiValidateSave(content) {
  if (!content || !content.trim()) return null
  if (!canUseAI()) return null // 非理事长及以上，不触发 AI 校验
  try {
    const { data } = await api.post(
      '/ai/validate-save',
      { content: content.trim() },
      { timeout: 190000 } // 本地模型推理较慢，单独放宽超时
    )
    return { ok: !!data.ok, problems: data.problems || [] }
  } catch (e) {
    return null
  }
}

// —— 确定性本地校验（不依赖 AI，100% 可靠）——

function parseDate(s) {
  const m = String(s || '').match(/^(\d{4})-(\d{1,2})-(\d{1,2})/)
  if (!m) return null
  return { y: parseInt(m[1], 10), mo: parseInt(m[2], 10), d: parseInt(m[3], 10) }
}

const CURRENT_YEAR = new Date().getFullYear()

function pushDateProblems(p, birth, death) {
  const b = parseDate(birth)
  const d = parseDate(death)
  // 卒年早于生年（按完整日期比较）
  if (b && d) {
    const bv = b.y * 10000 + b.mo * 100 + b.d
    const dv = d.y * 10000 + d.mo * 100 + d.d
    if (dv < bv) p.push('逝世日期早于出生日期')
  }
  // 未来日期 / 年份异常
  if (b && (b.y > CURRENT_YEAR + 1 || b.y < 1000)) p.push('出生日期异常（未来或年份不合理）')
  if (d && (d.y > CURRENT_YEAR + 1 || d.y < 1000)) p.push('逝世日期异常（未来或年份不合理）')
}

const PLACEHOLDERS = ['未填', '待定', '未知', '暂无', '无', '不详', '待补', '待考', '未详']

function nameProblems(surname, given) {
  const s = (surname || '').trim()
  const g = (given || '').trim()
  const p = []
  if (!s || !g) {
    p.push('姓名未填写完整（姓或名为空）')
  } else if (PLACEHOLDERS.includes(g)) {
    p.push('名字疑似未填写（占位符：' + g + '）')
  }
  return p
}

export function localMemberProblems(f, allMembers = []) {
  const p = nameProblems(f.surname, f.given_name)
  pushDateProblems(p, f.birth_gregorian, f.death_gregorian)
  const selfId = f.id || 0
  const fullName = ((f.surname || '') + (f.given_name || '')).trim()

  // 重复人物预警：同名且同世代（排除自己）
  if (fullName && f.generation) {
    const dup = allMembers.find(m => m.id !== selfId && ((m.surname || '') + (m.given_name || '')).trim() === fullName && m.generation === f.generation)
    if (dup) {
      p.push('疑似重复人物：已存在同名同世代的「' + fullName + '」（ID ' + dup.id + '），请确认是否重复录入')
    }
  }

  // 父子年龄 / 世代 / 性别冲突
  const fatherId = f.father_id || 0
  if (fatherId) {
    const father = allMembers.find(m => m.id === fatherId)
    if (father) {
      const childBirth = parseDate(f.birth_gregorian)
      const fatherBirth = parseDate(father.birth_gregorian)
      if (childBirth && fatherBirth) {
        const age = childBirth.y - fatherBirth.y
        if (age < 14) {
          p.push('父子年龄差过小：父亲「' + (father.name || '') + '」生此子时约 ' + age + ' 岁')
        }
      }
      // 父亲世代必须小于子女世代
      if (father.generation && f.generation && father.generation >= f.generation) {
        p.push('世代冲突：父亲「' + (father.name || '') + '」为第' + father.generation + '世，不应高于或等于子女的第' + f.generation + '世')
      }
      // 父亲性别应为男
      if (father.gender && father.gender !== 1) {
        p.push('血缘冲突：父亲「' + (father.name || '') + '」性别不是男')
      }
    }
  }
  return p
}

export function localSpouseProblems(f) {
  const p = nameProblems(f.surname, f.given_name)
  pushDateProblems(p, f.birth_gregorian, f.death_gregorian)
  return p
}

// 合并本地确定性问题 + AI 问题，去重；无问题返回空数组
export function mergeProblems(local, ai) {
  const set = new Set()
  const out = []
  const push = arr => (arr || []).forEach(x => { if (x && !set.has(x)) { set.add(x); out.push(x) } })
  push(local)
  if (ai && !ai.ok) push(ai.problems)
  return out
}

// 二次确认弹窗：返回 true 表示「确定无误，不做修改」继续保存；false 表示「去修改」
export async function confirmSaveProblems(problems) {
  const list = (problems || []).filter(Boolean)
  if (!list.length) return true // 无具体问题，直接放行
  const html = list.map((p, i) => `<p style="margin:4px 0">${i + 1}. ${String(p).replace(/</g, '&lt;')}</p>`).join('')
  try {
    await ElMessageBox.confirm(
      html,
      'AI 校验发现以下问题，请确认',
      {
        confirmButtonText: '确定无误，不做修改',
        cancelButtonText: '去修改',
        type: 'warning',
        dangerouslyUseHTMLString: true,
        closeOnClickModal: false,
        closeOnPressEscape: false
      }
    )
    return true
  } catch (e) {
    return false
  }
}

// 把成员/子女表单拼成一段可读的族谱资料文本
export function buildMemberText(f, extra = []) {
  const lines = []
  const name = (f.surname || '') + (f.given_name || '')
  lines.push(`姓名：${name || '（未填）'}`)
  if (f.generation) lines.push(`世代：第${f.generation}世`)
  lines.push(`性别：${f.gender === 1 ? '男' : f.gender === 2 ? '女' : '未填'}`)
  if (f.generation_word) lines.push(`字辈：${f.generation_word}`)
  if (f.zi) lines.push(`字：${f.zi}`)
  if (f.hao) lines.push(`号：${f.hao}`)
  if (f.rank) lines.push(`排行：${f.rank}`)
  if (f.clan_type) lines.push(`宗法：${f.clan_type}`)
  if (f.birth_gregorian) lines.push(`出生（公历）：${f.birth_gregorian}`)
  if (f.death_gregorian) lines.push(`逝世（公历）：${f.death_gregorian}`)
  if (f.native_place) lines.push(`籍贯：${f.native_place}`)
  if (f.occupation) lines.push(`职业：${f.occupation}`)
  if (f.education) lines.push(`学历：${f.education}`)
  if (f.biography) lines.push(`生平：${f.biography}`)
  return [...lines, ...extra].join('\n')
}

// 把配偶表单拼成一段可读的族谱资料文本
export function buildSpouseText(f, extra = []) {
  const lines = []
  const name = (f.surname || '') + (f.given_name || '')
  lines.push(`配偶姓名：${name || '（未填）'}`)
  if (f.type) lines.push(`类型：${f.type}`)
  if (f.family) lines.push(`家世：${f.family}`)
  if (f.birth_gregorian) lines.push(`出生（公历）：${f.birth_gregorian}`)
  if (f.death_gregorian) lines.push(`逝世（公历）：${f.death_gregorian}`)
  if (f.occupation) lines.push(`职业：${f.occupation}`)
  if (f.education) lines.push(`学历：${f.education}`)
  if (f.remark) lines.push(`备注：${f.remark}`)
  return [...lines, ...extra].join('\n')
}
