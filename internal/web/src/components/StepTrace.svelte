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

<style>
  /* ============================================================
     Chat messages — avatar + bubble
     ============================================================ */
  .msg-row {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    width: 100%;

    &.is-user {
      flex-direction: row-reverse;
    }
  }

  .msg-avatar {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 0.74rem;
    font-weight: 600;
    letter-spacing: 0.02em;
    border: 1px solid var(--color-border-subtle);
    background: var(--color-surface-raised);
    color: var(--color-text-primary);
    overflow: hidden;

    &.is-agent {
      background: transparent;
      border-color: transparent;
    }

    &.is-user {
      background: var(--color-action-primary);
      color: var(--black);
      border-color: var(--color-action-primary);
    }

    & img {
      width: 100%;
      height: 100%;
      display: block;
      object-fit: cover;
    }
  }

  .msg-bubble {
    max-width: calc(100% - 44px);
    padding: var(--space-4) var(--space-5);
    border-radius: var(--radius-lg);
    font-family: var(--font-family-ui);
    font-size: 0.88rem;
    line-height: 1.6;
    /* inherits --color-text-body; .md headings re-assert white globally */
    word-break: break-word;
    border: 1px solid var(--color-border-subtle);
    background: var(--ground);

    &.is-user {
      background: var(--color-accent-tint-soft);
      border-color: var(--green-30);
    }

    /* `{@html ...}` content — escape Svelte's scope hash so the rules
       hit the rendered markdown nodes. */
    & :global(:first-child) { margin-top: 0; }
    & :global(:last-child) { margin-bottom: 0; }
  }

  /* ============================================================
     Trace rows — tool use, tool result, system prompt, fallback
     ============================================================ */
  .trace-row {
    display: flex;
    align-items: flex-start;
    gap: var(--space-2);
    padding-left: var(--space-1);
    font-family: var(--font-family-code);
    font-size: 0.75rem;
    line-height: 1.55;
  }

  /* Lucide renders the icon inside its own <svg>; the class travels through
     as a prop, so target it globally to bypass Svelte's scope hash. */
  :global(.trace-row__icon) {
    margin-top: 2px;
    flex-shrink: 0;
  }

  .trace-row__body {
    flex: 1;
    word-break: break-all;
  }

  .trace-row__toggle {
    margin-left: var(--space-2);
    background: transparent;
    border: none;
    cursor: pointer;
    font-size: 0.7rem;
    color: var(--color-text-tertiary);
    font-family: inherit;
    transition: color var(--motion-duration-fast) var(--motion-ease);

    &:hover {
      color: var(--color-action-primary);
      text-decoration: underline;
      text-underline-offset: 2px;
    }
  }

  /* Tool invocation — secondary text + thin left rule */
  .trace-row--tool {
    color: var(--color-text-secondary);
    border-left: 2px solid var(--color-border-subtle);
    padding-left: 10px;
    margin-left: 0;
  }

  /* Tool result — accent green */
  .trace-row--result {
    color: var(--color-action-primary);

    & .trace-row__toggle:hover {
      color: var(--color-action-primary);
    }
  }

  /* System prompt — collapsed, tertiary */
  .trace-row--system {
    font-size: 0.7rem;
    color: var(--color-text-tertiary);
    padding-left: var(--space-1);
    display: block;

    & button {
      cursor: pointer;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      background: transparent;
      border: none;
      color: inherit;
      font-family: inherit;
      font-size: inherit;
      transition: color var(--motion-duration-fast) var(--motion-ease);

      &:hover {
        color: var(--color-text-secondary);
      }
    }

    & pre {
      margin-top: 6px;
      white-space: pre-wrap;
      word-break: break-all;
      color: var(--color-text-tertiary);
      line-height: 1.55;
      font-family: var(--font-family-code);
    }
  }

  .trace-row--fallback {
    font-family: var(--font-family-code);
    font-size: 0.75rem;
    color: var(--color-text-tertiary);
    padding-left: var(--space-1);
    word-break: break-all;
  }

  .trace-row__empty {
    color: var(--color-text-tertiary);
    font-family: var(--font-family-code);
    font-size: 0.78rem;
    text-align: center;
    padding-top: var(--space-8);
  }
</style>
