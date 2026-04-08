export async function listSessions() {
  const res = await fetch('/api/sessions')
  if (!res.ok) throw new Error(`Failed to list sessions: ${res.status}`)
  return res.json()
}

export async function getSession(id) {
  const res = await fetch(`/api/sessions/${id}`)
  if (!res.ok) throw new Error(`Failed to get session: ${res.status}`)
  return res.json()
}
