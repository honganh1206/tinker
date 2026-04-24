<script>
  let { messages = [], sessionId = '' } = $props()
  let expanded = $state({})

  function buildSteps(msgs) {
    const steps = []
    msgs.forEach((m, i) => {
      if (!m.content) return
      m.content.forEach(c => {
        if (m.role === 'user' && c.type === 'text' && i === 0)
          steps.push({ kind: 'user', text: c.text })
        if (m.role === 'user' && c.type === 'tool_result')
          steps.push({ kind: 'result', data: c })
        if (m.role === 'assistant' && c.type === 'tool_use')
          steps.push({ kind: 'tool', data: c })
        if (m.role === 'assistant' && c.type === 'text' && i < msgs.length - 1)
          steps.push({ kind: 'atext', text: c.text })
      })
    })
    return steps
  }

  function toggleExpand(key) {
    expanded[key] = !expanded[key]
  }

  function truncate(s, n) {
    return s && s.length > n ? s.slice(0, n) + '…' : s
  }

  let steps = $derived(buildSteps(messages))
</script>

{#if steps.length > 0}
  <div class="text-xs uppercase tracking-widest text-gray-400 font-mono mb-4">
    execution trace · {steps.length} steps
  </div>

  {#each steps as step, idx (idx)}
    {@const key = `${sessionId?.slice(0,8)}_${idx}`}
    {@const isLast = idx === steps.length - 1}

    <div class="grid grid-cols-[28px_1fr] gap-x-3">
      <div class="flex flex-col items-center">
        {#if step.kind === 'user'}
          <div class="w-6 h-6 rounded text-xs font-mono font-medium flex items-center justify-center bg-blue-50 text-blue-600 border border-blue-200">U</div>
        {:else if step.kind === 'tool'}
          <div class="w-6 h-6 rounded text-xs font-mono font-medium flex items-center justify-center bg-amber-50 text-amber-600 border border-amber-200">T</div>
        {:else if step.kind === 'result'}
          <div class="w-6 h-6 rounded text-xs font-mono font-medium flex items-center justify-center border
            {step.data.is_error ? 'bg-red-50 text-red-600 border-red-200' : 'bg-green-50 text-green-600 border-green-200'}">
            {step.data.is_error ? '✕' : '✓'}
          </div>
        {:else}
          <div class="w-6 h-6 rounded text-xs font-mono font-medium flex items-center justify-center bg-gray-50 text-gray-400 border">A</div>
        {/if}
        {#if !isLast}
          <div class="w-px flex-1 bg-gray-200 min-h-[10px] mt-1"></div>
        {/if}
      </div>

      <div class="pb-3">
        {#if step.kind === 'user'}
          <div class="text-sm font-medium font-mono mb-1 pt-0.5">user prompt</div>
          <div class="border rounded p-2 text-xs leading-relaxed text-gray-600 font-mono break-all">
            {truncate(step.text, 160)}
          </div>
        {:else if step.kind === 'tool'}
          <div class="text-sm font-medium font-mono mb-1 pt-0.5">{step.data.name}</div>
          <div class="border rounded p-2 text-xs leading-relaxed text-gray-600 font-mono break-all">
            {#if expanded[key]}
              <pre class="whitespace-pre-wrap text-xs leading-relaxed">{JSON.stringify(step.data.input, null, 2)}</pre>
            {:else}
              {#each Object.entries(step.data.input || {}) as [k, v]}
                <span class="text-blue-600">{k}</span>: <span class="text-amber-600">"{truncate(String(v), 80)}"</span>&nbsp;&nbsp;
              {/each}
            {/if}
          </div>
          <button class="text-xs text-gray-400 cursor-pointer font-mono mt-1 hover:text-gray-600"
            onclick={() => toggleExpand(key)}>
            {expanded[key] ? '▴ collapse' : '▸ expand input'}
          </button>
        {:else if step.kind === 'result'}
          <div class="flex items-center gap-2 mb-1 pt-0.5">
            <span class="text-sm font-medium font-mono">{step.data.tool_name} result</span>
            <span class="text-xs px-1 py-0.5 rounded border {step.data.is_error ? 'text-red-600 border-red-300' : 'text-green-600 border-green-300'}">
              {step.data.is_error ? 'error' : 'ok'}
            </span>
          </div>
          <div class="border rounded p-2 text-xs leading-relaxed font-mono break-all {step.data.is_error ? 'text-red-600' : 'text-green-600'}">
            {truncate(String(step.data.content), 160)}
          </div>
        {:else if step.kind === 'atext'}
          <div class="text-sm font-medium font-mono mb-1 pt-0.5">assistant (mid-run)</div>
          <div class="border rounded p-2 text-xs leading-relaxed text-gray-600 font-mono break-all">
            {truncate(step.text, 120)}
          </div>
        {/if}
      </div>
    </div>
  {/each}
{/if}
