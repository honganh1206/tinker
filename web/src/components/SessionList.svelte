<script>
  import {
    sessions,
    selectedId,
    selectSession,
  } from "../lib/stores/sessions.js";

  function fmtDur(ms) {
    return ms < 1000 ? ms + "ms" : (ms / 1000).toFixed(1) + "s";
  }

  function fmtTime(iso) {
    return new Date(iso).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    });
  }
</script>

<aside
  class="bg-[var(--color-bg-surface-primary)] border-r flex flex-col overflow-hidden"
>
  <div
    class="px-4 py-3 text-[var(--font-size-xxs)] tracking-widest text-[var(--color-text-tertiary)] uppercase border-b flex justify-between items-center"
  >
    Sessions
    <span class="font-[var(--font-mono)] text-[var(--font-size-xxs)]"
      >{$sessions.length} runs</span
    >
  </div>
  <div class="overflow-y-auto flex-1">
    {#each $sessions as s (s.id)}
      <button
        class="w-full text-left px-4 py-[11px] border-b cursor-pointer flex flex-col gap-[5px] border-l-2 transition-colors duration-100
          {$selectedId === s.id
          ? 'bg-[var(--color-bg-surface-secondary)] border-l-[var(--color-accent)] pl-3.5'
          : 'border-l-transparent hover:bg-[var(--color-bg-surface-secondary)]'}"
        onclick={() => selectSession(s.id)}
      >
        <div class="flex items-center gap-2">
          <span
            class="LemonBadge--dot {s.status === 'success'
              ? 'LemonBadge--dot-success'
              : 'LemonBadge--dot-danger'}"
          ></span>
          <span
            class="text-[var(--font-size-sm)] text-[var(--color-text-primary)] truncate flex-1"
            >{s.prompt}</span
          >
        </div>
        <div class="flex gap-2.5 pl-3.5">
          <span
            class="text-[var(--font-size-xxs)] text-[var(--color-text-tertiary)] font-[var(--font-mono)]"
            >{fmtTime(s.started_at)}</span
          >
          <span
            class="text-[var(--font-size-xxs)] text-[var(--color-text-tertiary)] font-[var(--font-mono)]"
            >{fmtDur(s.duration_ms)}</span
          >
        </div>
      </button>
    {:else}
      <div
        class="text-[var(--color-text-tertiary)] text-[var(--font-size-sm)] p-10 text-center font-[var(--font-mono)]"
      >
        no sessions yet
      </div>
    {/each}
  </div>
</aside>
