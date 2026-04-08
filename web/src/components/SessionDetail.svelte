<script>
  import { selectedSession } from '../lib/stores/sessions.js'
  import StepTrace from './StepTrace.svelte'

  function fmtDur(ms) {
    return ms < 1000 ? ms + 'ms' : (ms / 1000).toFixed(1) + 's'
  }
</script>

{#if $selectedSession}
  {@const s = $selectedSession}
  <div class="px-6 pt-[18px] pb-3.5 border-b bg-[var(--color-bg-surface-primary)] flex-shrink-0">
    <div class="text-[var(--font-size-base)] font-medium text-[var(--color-text-primary)] mb-2.5 leading-[1.45]">{s.prompt}</div>
    <div class="flex gap-[7px] flex-wrap items-center">
      <span class="LemonTag {s.status === 'success' ? 'LemonTag--status-success' : 'LemonTag--status-danger'}">{s.status}</span>
      <span class="LemonTag LemonTag--status-default">{s.model}</span>
      <span class="LemonTag LemonTag--status-default">{fmtDur(s.duration_ms)}</span>
      <span class="LemonTag LemonTag--status-default">{s.tokens_used?.toLocaleString()} tokens</span>
      <span class="text-[9px] font-[var(--font-mono)] text-[var(--color-text-secondary)] ml-auto">{s.id?.slice(0, 8)}…</span>
    </div>
  </div>

  <div class="flex-1 overflow-y-auto px-6 pt-[18px] pb-8">
    <StepTrace messages={s.messages || []} sessionId={s.id} />

    {#if s.final_message}
      <div class="LemonCard mt-6">
        <div class="text-[var(--font-size-xxs)] tracking-widest uppercase text-[var(--color-text-tertiary)] font-[var(--font-mono)] mb-2">final response</div>
        <div class="text-[var(--font-size-sm)] text-[var(--color-text-primary)] leading-[1.65] whitespace-pre-wrap">{s.final_message}</div>
      </div>
    {/if}
  </div>
{/if}
