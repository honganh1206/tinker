<script>
  import { sessions, selectedId, selectSession } from '../lib/stores/sessions.js'

  function fmtDur(ms) {
    return ms < 1000 ? ms + 'ms' : (ms / 1000).toFixed(1) + 's'
  }

  function fmtTime(iso) {
    return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
</script>

<aside class="border-r flex flex-col overflow-hidden">
  <div class="px-4 py-3 text-xs uppercase tracking-widest text-gray-500 border-b flex justify-between">
    Sessions
    <span class="font-mono text-xs">{$sessions.length} runs</span>
  </div>
  <div class="overflow-y-auto flex-1">
    {#each $sessions as s (s.id)}
      <button
        class="w-full text-left px-4 py-3 border-b cursor-pointer flex flex-col gap-1"
        class:bg-gray-100={$selectedId === s.id}
        onclick={() => selectSession(s.id)}
      >
        <div class="flex items-center gap-2">
          <span class="w-1.5 h-1.5 rounded-full {s.status === 'success' ? 'bg-green-500' : 'bg-red-500'}"></span>
          <span class="text-sm truncate flex-1">{s.prompt}</span>
        </div>
        <div class="flex gap-2 pl-3.5 text-xs text-gray-400 font-mono">
          <span>{fmtTime(s.started_at)}</span>
          <span>{fmtDur(s.duration_ms)}</span>
        </div>
      </button>
    {:else}
      <div class="text-gray-400 text-sm p-10 text-center">no sessions yet</div>
    {/each}
  </div>
</aside>
