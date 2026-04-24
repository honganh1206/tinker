<script>
  import { selectedSession } from '../lib/stores/sessions.js'
  import StepTrace from './StepTrace.svelte'

  function fmtDur(ms) {
    return ms < 1000 ? ms + 'ms' : (ms / 1000).toFixed(1) + 's'
  }
</script>

{#if $selectedSession}
  {@const s = $selectedSession}
  <div class="px-6 pt-5 pb-3 border-b">
    <div class="text-base font-medium mb-2">{s.prompt}</div>
    <div class="flex gap-2 flex-wrap text-xs">
      <span class="px-1.5 py-0.5 rounded border {s.status === 'success' ? 'text-green-600 border-green-300' : 'text-red-600 border-red-300'}">{s.status}</span>
      <span class="px-1.5 py-0.5 rounded border text-gray-500">{s.model}</span>
      <span class="px-1.5 py-0.5 rounded border text-gray-500">{fmtDur(s.duration_ms)}</span>
      <span class="px-1.5 py-0.5 rounded border text-gray-500">{s.tokens_used?.toLocaleString()} tokens</span>
    </div>
  </div>

  <div class="flex-1 overflow-y-auto px-6 pt-5 pb-8">
    <StepTrace messages={s.messages || []} sessionId={s.id} />

    {#if s.final_message}
      <div class="mt-6 border rounded-lg p-3">
        <div class="text-xs uppercase tracking-widest text-gray-400 mb-2">final response</div>
        <div class="text-sm leading-relaxed whitespace-pre-wrap">{s.final_message}</div>
      </div>
    {/if}
  </div>
{/if}
