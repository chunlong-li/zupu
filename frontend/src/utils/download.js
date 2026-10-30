// 文件下载工具：把接口返回的 Blob 存成文件，并从 Content-Disposition 里取后端给的中文文件名。
export function filenameFromDisposition(header, fallback) {
  if (!header) return fallback
  try {
    const star = /filename\*=UTF-8''([^;]+)/i.exec(header)
    if (star && star[1]) return decodeURIComponent(star[1].replace(/^"|"$/g, ''))
    const plain = /filename="?([^";]+)"?/i.exec(header)
    if (plain && plain[1]) return plain[1]
  } catch (e) { /* ignore */ }
  return fallback
}

export function saveBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename || 'download'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  setTimeout(() => URL.revokeObjectURL(url), 2000)
}
