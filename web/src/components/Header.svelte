<script>
  import { sessions } from '../lib/stores/sessions.js'

  let total = $derived($sessions.length)
  let success = $derived($sessions.filter(s => s.status === 'success').length)
  let failed = $derived($sessions.filter(s => s.status !== 'success').length)
</script>

<header class="h-11 border-b flex items-center px-5 gap-6">
  <div class="font-mono text-sm font-medium">tinker</div>
  <div class="flex gap-4 ml-auto text-xs text-gray-500">
    <span>{total} total</span>
    <span>{success} success</span>
    <span>{failed} error</span>
  </div>
</header>
