<script lang="ts">
  import {
    ChevronRight,
    ChevronDown,
    Wrench,
    CornerDownRight,
  } from "lucide-svelte";
  import { RecordType } from "../lib/types";
  import type { Record } from "../lib/types";
  import { renderMarkdown } from "../lib/markdown";

  let { records = [] }: { records: Record[] } = $props();

  let expanded: { [key: number]: boolean } = $state({});

  function toggleExpand(id: number): void {
    expanded[id] = !expanded[id];
  }

  function truncate(s: string, n: number): string {
    return s && s.length > n ? s.slice(0, n) + "…" : s;
  }
</script>

{#if records.length === 0}
  <div class="trace-row__empty">No records yet</div>
{:else}
  <div class="flex flex-col gap-5 mx-auto w-full">
    {#each records as r (r.id)}
      {#if r.source === RecordType.Prompt}
        <!-- user: avatar on right, bubble on right -->
        <div class="msg-row is-user">
          <div class="msg-avatar is-user" aria-hidden="true">YO</div>
          <div class="msg-bubble is-user md">
            {@html renderMarkdown(r.content)}
          </div>
        </div>
      {:else if r.source === RecordType.ModelResp}
        <!-- agent: avatar on left, bubble on left -->
        <div class="msg-row">
          <div class="msg-avatar is-agent" aria-hidden="true">
            <img src="/icon-192.png" alt="" width="32" height="32" />
          </div>
          <div class="msg-bubble md">
            {@html renderMarkdown(r.content)}
          </div>
        </div>
      {:else if r.source === RecordType.ToolUse}
        <!-- tool use: secondary text + thin left rule + wrench icon -->
        <div class="trace-row trace-row--tool">
          <Wrench size={13} class="trace-row__icon" />
          <div class="trace-row__body">
            {expanded[r.id] ? r.content : truncate(r.content, 220)}
            {#if r.content.length > 220}
              <button
                class="trace-row__toggle"
                onclick={() => toggleExpand(r.id)}
              >
                {expanded[r.id] ? "collapse" : "expand"}
              </button>
            {/if}
          </div>
        </div>
      {:else if r.source === RecordType.ToolResult}
        <!-- tool result: accent green + corner-down-right icon -->
        <div class="trace-row trace-row--result">
          <CornerDownRight size={13} class="trace-row__icon" />
          <div class="trace-row__body">
            {expanded[r.id] ? r.content : truncate(r.content, 220)}
            {#if r.content.length > 220}
              <button
                class="trace-row__toggle"
                onclick={() => toggleExpand(r.id)}
              >
                {expanded[r.id] ? "collapse" : "expand"}
              </button>
            {/if}
          </div>
        </div>
      {:else if r.source === RecordType.SystemPrompt}
        <!-- system prompt: tertiary, collapsed by default -->
        <div class="trace-row--system">
          <button onclick={() => toggleExpand(r.id)}>
            {#if expanded[r.id]}
              <ChevronDown size={12} />
            {:else}
              <ChevronRight size={12} />
            {/if}
            system prompt · {r.est_tokens} tokens
          </button>
          {#if expanded[r.id]}
            <pre>{r.content}</pre>
          {/if}
        </div>
      {:else}
        <div class="trace-row--fallback">{r.content}</div>
      {/if}
    {/each}
  </div>
{/if}
