import { writable } from 'svelte/store'
import { listSessions, getSession } from '../api.js'

export const sessions = writable([])
export const selectedId = writable(null)
export const selectedSession = writable(null)

export async function refreshSessions() {
  try {
    const data = await listSessions()
    sessions.set(data || [])
  } catch (e) {
    console.error('Failed to refresh sessions:', e)
  }
}

export async function selectSession(id) {
  selectedId.set(id)
  try {
    const data = await getSession(id)
    selectedSession.set(data)
  } catch (e) {
    console.error('Failed to fetch session detail:', e)
    selectedSession.set(null)
  }
}
