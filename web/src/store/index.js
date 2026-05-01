import { createStore } from 'vuex'

export default createStore({
  state: {
    models: [],
    currentModel: localStorage.getItem('selectedModel') || '',
    sessions: [],
    activeSessionId: null
  },
  mutations: {
    setModels(state, models) { state.models = models },
    setCurrentModel(state, model) { state.currentModel = model; localStorage.setItem('selectedModel', model) },
    setSessions(state, sessions) { state.sessions = sessions },
    addSession(state, session) { state.sessions.unshift(session) },
    setActiveSession(state, id) { state.activeSessionId = id },
    appendMessage(state, { sessionId, message }) {
      const s = state.sessions.find(s => s.id === sessionId)
      if (s) s.messages.push(message)
    },
    updateLastMessage(state, { sessionId, content }) {
      const s = state.sessions.find(s => s.id === sessionId)
      if (s && s.messages.length) s.messages[s.messages.length - 1].content = content
    }
  },
  getters: {
    activeSession: state => state.sessions.find(s => s.id === state.activeSessionId),
    contextRounds: (state, getters) => {
      const s = getters.activeSession
      return s ? Math.floor(s.messages.length / 2) : 0
    }
  }
})
