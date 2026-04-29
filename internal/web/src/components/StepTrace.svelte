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
  <div
    class="text-[var(--text-muted)] font-[var(--mono)] text-[0.78rem] text-center pt-8"
  >
    No records yet
  </div>
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
            <svg
              viewBox="0 0 40 40"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
            >
              <circle cx="20" cy="20" r="13" fill="#FCF3ED" />
              <circle cx="26" cy="17" r="3.5" fill="#1A4D55" />
            </svg>
          </div>
          <div class="msg-bubble is-agent md">
            {@html renderMarkdown(r.content)}
          </div>
        </div>
      {:else if r.source === RecordType.ToolUse}
        <!-- tool use: inline line, terra, wrench icon -->
        <div
          class="flex items-start gap-2 font-[var(--mono)] text-[0.75rem] text-[var(--terra)] pl-1"
        >
          <Wrench size={13} class="mt-0.5 flex-shrink-0" />
          <div class="flex-1 break-all">
            {expanded[r.id] ? r.content : truncate(r.content, 220)}
            {#if r.content.length > 220}
              <button
                class="ml-2 text-[var(--text-muted)] hover:text-[var(--terra)] cursor-pointer text-[0.7rem] underline-offset-2 hover:underline"
                onclick={() => toggleExpand(r.id)}
              >
                {expanded[r.id] ? "collapse" : "expand"}
              </button>
            {/if}
          </div>
        </div>
      {:else if r.source === RecordType.ToolResult}
        <!-- tool result: inline line, green, corner-down-right icon -->
        <div
          class="flex items-start gap-2 font-[var(--mono)] text-[0.75rem] text-[var(--green)] pl-1"
        >
          <CornerDownRight size={13} class="mt-0.5 flex-shrink-0" />
          <div class="flex-1 break-all">
            {expanded[r.id] ? r.content : truncate(r.content, 220)}
            {#if r.content.length > 220}
              <button
                class="ml-2 text-[var(--text-muted)] hover:text-[var(--green)] cursor-pointer text-[0.7rem] underline-offset-2 hover:underline"
                onclick={() => toggleExpand(r.id)}
              >
                {expanded[r.id] ? "collapse" : "expand"}
              </button>
            {/if}
          </div>
        </div>
      {:else if r.source === RecordType.SystemPrompt}
        <!-- system prompt: muted, collapsed by default -->
        <div
          class="font-[var(--mono)] text-[0.7rem] text-[var(--text-muted)] pl-1"
        >
          <button
            class="cursor-pointer hover:text-[var(--text-mid)] uppercase tracking-[0.05em] inline-flex items-center gap-1"
            onclick={() => toggleExpand(r.id)}
          >
            {#if expanded[r.id]}
              <ChevronDown size={12} />
            {:else}
              <ChevronRight size={12} />
            {/if}
            system prompt · {r.est_tokens} tokens
          </button>
          {#if expanded[r.id]}
            <pre
              class="mt-1.5 whitespace-pre-wrap break-all text-[var(--text-muted)] leading-[1.55]">{r.content}</pre>
          {/if}
        </div>
      {:else}
        <div
          class="font-[var(--mono)] text-[0.75rem] text-[var(--text-muted)] pl-1 break-all"
        >
          {r.content}
        </div>
      {/if}
    {/each}
  </div>
{/if}
