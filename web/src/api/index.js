import axios from 'axios'

const http = axios.create({ baseURL: '/api/v1' })

export const getModels = () => http.get('/models')

export const chatStream = async (payload, onChunk, onDone) => {
  const resp = await fetch('/api/v1/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...payload, stream: true })
  })
  if (!resp.ok) {
    const text = await resp.text().catch(() => '')
    throw new Error(text || `chat request failed: ${resp.status}`)
  }
  if (!resp.body) throw new Error('chat stream is not readable')
  const reader = resp.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop()
    for (const line of lines) {
      if (!line.startsWith('data: ')) continue
      const data = line.slice(6).trim()
      if (data === '[DONE]') { onDone?.(); return }
      try {
        const json = JSON.parse(data)
        const delta = json.choices?.[0]?.delta?.content
        if (delta) onChunk(delta)
      } catch {}
    }
  }
  onDone?.()
}
