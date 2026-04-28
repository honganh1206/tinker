export enum RecordType {
  Prompt = 0,
  ModelResp = 1,
  ToolUse = 2,
  ToolResult = 3,
  SystemPrompt = 4,
}

export interface Context {
  id: string
  name: string
  start_time: string
}

export interface Record {
  id: number
  timestamp: string
  source: RecordType
  content: string
  live: boolean
  est_tokens: number
  context_id: string
}

export interface ContextTool {
  id: number
  context_id: string
  tool_name: string
  created_at: string
}

export interface Session {
  id: string
  name?: string
  start_time?: string
  context_count: number
  record_count: number
  context_tool_count: number
  contexts?: Context[]
  records?: Record[]
  context_tools?: ContextTool[]
}

