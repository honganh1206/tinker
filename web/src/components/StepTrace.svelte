<script>
  let { messages = [], sessionId = "" } = $props();
  let expanded = $state({});

  function buildSteps(msgs) {
    const steps = [];
    msgs.forEach((m, i) => {
      if (!m.content) return;
      m.content.forEach((c) => {
        if (m.role === "user" && c.type === "text" && i === 0)
          steps.push({ kind: "user", text: c.text });
        if (m.role === "user" && c.type === "tool_result")
          steps.push({ kind: "result", data: c });
        if (m.role === "assistant" && c.type === "tool_use")
          steps.push({ kind: "tool", data: c });
        if (m.role === "assistant" && c.type === "text" && i < msgs.length - 1)
          steps.push({ kind: "atext", text: c.text });
      });
    });
    return steps;
  }

  function toggleExpand(key) {
    expanded[key] = !expanded[key];
  }

  function truncate(s, n) {
    return s && s.length > n ? s.slice(0, n) + "…" : s;
  }

  let steps = $derived(buildSteps(messages));
</script>

{#if steps.length > 0}
  <div
    class="text-[var(--font-size-xxs)] tracking-widest uppercase text-[var(--color-text-tertiary)] font-[var(--font-mono)] mb-4"
  >
    execution trace · {steps.length} steps
  </div>

  {#each steps as step, idx (idx)}
    {@const key = `${sessionId?.slice(0, 8)}_${idx}`}
    {@const isLast = idx === steps.length - 1}

    <div class="grid grid-cols-[28px_1fr] gap-x-3">
      <div class="flex flex-col items-center">
        {#if step.kind === "user"}
          <div
            class="w-[22px] h-[22px] rounded text-[var(--font-size-xxs)] font-[var(--font-mono)] font-medium flex items-center justify-center bg-[var(--color-step-user-highlight)] text-[var(--color-step-user)] border border-[var(--color-step-user-border)]"
          >
            U
          </div>
        {:else if step.kind === "tool"}
          <div
            class="w-[22px] h-[22px] rounded text-[var(--font-size-xxs)] font-[var(--font-mono)] font-medium flex items-center justify-center bg-[var(--color-step-tool-highlight)] text-[var(--color-step-tool)] border border-[var(--color-step-tool-border)]"
          >
            T
          </div>
        {:else if step.kind === "result"}
          <div
            class="w-[22px] h-[22px] rounded text-[var(--font-size-xxs)] font-[var(--font-mono)] font-medium flex items-center justify-center border
            {step.data.is_error
              ? 'bg-[var(--color-danger-highlight)] text-[var(--color-danger)] border-[var(--color-danger-border)]'
              : 'bg-[var(--color-success-highlight)] text-[var(--color-success)] border-[var(--color-success-border)]'}"
          >
            {step.data.is_error ? "✕" : "✓"}
          </div>
        {:else}
          <div
            class="w-[22px] h-[22px] rounded text-[var(--font-size-xxs)] font-[var(--font-mono)] font-medium flex items-center justify-center bg-[var(--color-bg-surface-secondary)] text-[var(--color-text-tertiary)] border"
          >
            A
          </div>
        {/if}
        {#if !isLast}
          <div
            class="w-px flex-1 bg-[var(--color-border-primary)] min-h-[10px] mt-[3px]"
          ></div>
        {/if}
      </div>

      <div class="pb-3">
        {#if step.kind === "user"}
          <div class="flex items-center gap-2 mb-1.5 pt-0.5">
            <span
              class="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)] font-[var(--font-mono)]"
              >user prompt</span
            >
          </div>
          <div
            class="LemonCard !p-2 text-[var(--font-size-xs)] leading-[1.65] text-[var(--color-text-secondary)] font-[var(--font-mono)] break-all"
          >
            {truncate(step.text, 160)}
          </div>
        {:else if step.kind === "tool"}
          <div class="flex items-center gap-2 mb-1.5 pt-0.5">
            <span
              class="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)] font-[var(--font-mono)]"
              >{step.data.name}</span
            >
          </div>
          <div
            class="LemonCard !p-2 text-[var(--font-size-xs)] leading-[1.65] text-[var(--color-text-secondary)] font-[var(--font-mono)] break-all"
          >
            {#if expanded[key]}
              <pre
                class="whitespace-pre-wrap text-[var(--font-size-xs)] text-[var(--color-text-primary)] leading-[1.6]">{JSON.stringify(
                  step.data.input,
                  null,
                  2,
                )}</pre>
            {:else}
              {#each Object.entries(step.data.input || {}) as [k, v]}
                <span class="text-[var(--color-step-user)]">{k}</span>:
                <span class="text-[var(--color-step-tool)]"
                  >"{truncate(String(v), 80)}"</span
                >&nbsp;&nbsp;
              {/each}
            {/if}
          </div>
          <button
            class="text-[var(--font-size-xxs)] text-[var(--color-text-secondary)] cursor-pointer font-[var(--font-mono)] mt-[5px] hover:text-[var(--color-text-tertiary)]"
            onclick={() => toggleExpand(key)}
          >
            {expanded[key] ? "▴ collapse" : "▸ expand input"}
          </button>
        {:else if step.kind === "result"}
          <div class="flex items-center gap-2 mb-1.5 pt-0.5">
            <span
              class="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)] font-[var(--font-mono)]"
              >{step.data.tool_name} result</span
            >
            <span
              class="LemonTag LemonTag--size-small {step.data.is_error
                ? 'LemonTag--status-danger'
                : 'LemonTag--status-success'}"
            >
              {step.data.is_error ? "error" : "ok"}
            </span>
          </div>
          <div
            class="LemonCard !p-2 text-[var(--font-size-xs)] leading-[1.65] font-[var(--font-mono)] break-all
            {step.data.is_error
              ? 'text-[var(--color-danger)]'
              : 'text-[var(--color-success)]'}"
          >
            {truncate(String(step.data.content), 160)}
          </div>
        {:else if step.kind === "atext"}
          <div class="flex items-center gap-2 mb-1.5 pt-0.5">
            <span
              class="text-[var(--font-size-sm)] font-medium text-[var(--color-text-primary)] font-[var(--font-mono)]"
              >assistant (mid-run)</span
            >
          </div>
          <div
            class="LemonCard !p-2 text-[var(--font-size-xs)] leading-[1.65] text-[var(--color-text-secondary)] font-[var(--font-mono)] break-all"
          >
            {truncate(step.text, 120)}
          </div>
        {/if}
      </div>
    </div>
  {/each}
{/if}
