const allowedTags = new Set([
  'A', 'B', 'BLOCKQUOTE', 'BR', 'CODE', 'DD', 'DIV', 'DL', 'DT', 'EM', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6',
  'HR', 'I', 'IMG', 'LI', 'OL', 'P', 'PRE', 'S', 'SPAN', 'STRONG', 'TABLE', 'TBODY', 'TD', 'TH', 'THEAD', 'TR', 'UL'
])

const allowedAttrs = new Set(['alt', 'class', 'colspan', 'href', 'rel', 'rowspan', 'src', 'target', 'title'])

const safeUrl = value => {
  const text = String(value || '').trim().toLowerCase()
  return text === '' || text.startsWith('#') || text.startsWith('/') || text.startsWith('http://') || text.startsWith('https://') || text.startsWith('mailto:')
}

export const sanitizeHtml = html => {
  const template = document.createElement('template')
  template.innerHTML = html || ''
  const walk = node => {
    for (const child of [...node.children]) {
      if (!allowedTags.has(child.tagName)) {
        child.replaceWith(document.createTextNode(child.textContent || ''))
        continue
      }
      for (const attr of [...child.attributes]) {
        const name = attr.name.toLowerCase()
        if (name.startsWith('on') || !allowedAttrs.has(name)) {
          child.removeAttribute(attr.name)
          continue
        }
        if ((name === 'href' || name === 'src') && !safeUrl(attr.value)) {
          child.removeAttribute(attr.name)
        }
      }
      if (child.tagName === 'A') {
        child.setAttribute('rel', 'noopener noreferrer')
      }
      walk(child)
    }
  }
  walk(template.content)
  return template.innerHTML
}
