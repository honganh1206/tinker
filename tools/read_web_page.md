Fetch and read the text content of a web page at a given URL.

WHEN TO USE THIS TOOL:
- After using web_search, when you need to read the full content of a result
- When the user provides a specific URL and asks about its content
- When you need detailed information from documentation, blog posts, or articles
- When a search snippet is insufficient and you need the full page text

WHEN NOT TO USE THIS TOOL:
- For local files (use read_file instead)
- When a brief search snippet already answers the question
- For binary files, images, or non-text content

USAGE TIPS:
- Prefer reading specific URLs found via web_search rather than guessing URLs
- The tool returns plain text extracted from the HTML, not the raw HTML itself
- Very large pages are truncated to keep context manageable
