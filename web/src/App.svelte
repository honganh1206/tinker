<script>
  import Header from './components/Header.svelte'
  import SessionList from './components/SessionList.svelte'
  import SessionDetail from './components/SessionDetail.svelte'
  import { refreshSessions, selectedId, selectedSession } from './lib/stores/sessions.js'

  $effect(() => {
    refreshSessions()
    const interval = setInterval(refreshSessions, 3000)
    return () => clearInterval(interval)
  })
</script>

<div class="h-screen overflow-hidden grid grid-rows-[44px_1fr] grid-cols-[300px_1fr]">
  <div class="col-span-2">
    <Header />
  </div>
  <SessionList />
  <main class="flex flex-col overflow-hidden">
    {#if $selectedSession}
      <SessionDetail />
    {:else}
      <div class="p-16 text-center text-sm text-gray-400">
        ← select a session to inspect
      </div>
    {/if}
  </main>
</div>
