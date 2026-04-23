Read a file from the file system. If the file doesn't exist, an error is returned.

- When the user provides a specific file path, read it directly without searching first.
- By default, this tool returns the first 500 lines. To read more, call it again with start_line/end_line.
- The response includes the total line count so you know if more content remains.
- **Always** specify start_line and end_line when you know the relevant line range. Never read an entire file when you only need a portion.
- Use the grep_search tool to find specific content and line numbers in large files, then read only the relevant range.
- The contents are returned with each line prefixed by its line number.
- When possible, call this tool in parallel for all files you will want to read.
- Avoid tiny repeated slices. If you need more context, read a larger range.
