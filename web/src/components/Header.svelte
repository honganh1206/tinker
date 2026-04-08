<script>
  import { sessions } from '../lib/stores/sessions.js'

  let total = $derived($sessions.length)
  let success = $derived($sessions.filter(s => s.status === 'success').length)
  let failed = $derived($sessions.filter(s => s.status !== 'success').length)
</script>

<header class="h-11 bg-[var(--color-bg-surface-primary)] border-b flex items-center px-5 gap-6">
  <div class="font-[var(--font-mono)] text-[13px] font-medium text-[var(--color-accent)] tracking-wider flex items-center gap-2">
    <span class="LemonBadge--dot LemonBadge--dot-accent"></span>
    tinker
  </div>
  <div class="flex gap-5 ml-auto items-center text-[var(--font-size-xxs)] text-[var(--color-text-tertiary)]">
    <div class="flex items-center gap-1.5">
      <strong class="text-[var(--color-text-primary)] font-medium font-[var(--font-mono)]">{total}</strong> total
    </div>
    <div class="w-px h-3.5 bg-[var(--color-border-primary)]"></div>
    <div class="flex items-center gap-1.5">
      <strong class="text-[var(--color-success)] font-medium font-[var(--font-mono)]">{success}</strong> success
    </div>
    <div class="w-px h-3.5 bg-[var(--color-border-primary)]"></div>
    <div class="flex items-center gap-1.5">
      <strong class="text-[var(--color-danger)] font-medium font-[var(--font-mono)]">{failed}</strong> error
    </div>
  </div>
</header>
