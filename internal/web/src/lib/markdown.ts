import { marked } from 'marked'

marked.use({
  gfm: true,
  breaks: true,
})

/**
 * Render a markdown string to HTML for use with Svelte's {@html ...}.
 *
 * Note: marked does not sanitize HTML. Tinker's chat content originates from
 * the agent's own records, so we accept that trade-off here. If untrusted
 * sources are ever introduced, wrap with DOMPurify.
 */
export function renderMarkdown(src: string): string {
  if (!src) return ''
  return marked.parse(src, { async: false }) as string
}
