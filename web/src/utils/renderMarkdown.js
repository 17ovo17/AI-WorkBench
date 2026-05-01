import { marked } from 'marked'
import hljs from 'highlight.js'
import { sanitizeHtml } from './sanitizeHtml'

marked.setOptions({
  gfm: true,
  breaks: true,
  highlight(code, lang) {
    return hljs.highlight(code, { language: hljs.getLanguage(lang) ? lang : 'plaintext' }).value
  }
})

const markdownKeys = ['report', 'summary_report', 'markdownReport', 'markdown', 'analysis', 'diagnosis', 'result', 'content']

const stripJsonFence = text => {
  const value = String(text || '').trim()
  return value.replace(/^```(?:json)?\s*/i, '').replace(/```$/i, '').trim()
}

const firstMarkdownField = value => {
  if (!value || typeof value !== 'object') return ''
  for (const key of markdownKeys) {
    const item = value[key]
    if (typeof item === 'string' && item.trim()) return item
    if (item && typeof item === 'object') {
      const nested = firstMarkdownField(item)
      if (nested) return nested
    }
  }
  return ''
}

export const extractMarkdownText = input => {
  if (input && typeof input === 'object') {
    return firstMarkdownField(input) || JSON.stringify(input, null, 2)
  }
  const text = String(input || '')
  const trimmed = stripJsonFence(text)
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return text
  try {
    const parsed = JSON.parse(trimmed)
    return firstMarkdownField(parsed) || text
  } catch {
    return text
  }
}

export const renderMarkdown = input => sanitizeHtml(marked.parse(extractMarkdownText(input)))
