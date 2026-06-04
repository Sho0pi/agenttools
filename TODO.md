# TODO: AI agent tool implementation roadmap

Research date: 2026-06-04

This is a ranked implementation list for a general purpose AI agent. I prioritized tools that recur across the referenced agent codebases and current agent platform docs, then ranked them by practical usefulness, safety impact, and how often later tools depend on them.

Source shorthand: Z = Zeroclaw, H = Hermes Agent, P = Picoclaw, O = OpenClaw, OA = OpenAI Agents or Responses tooling, AN = Anthropic computer use docs, MCP = Model Context Protocol.

Sources scanned:

Zeroclaw tools: https://github.com/zeroclaw-labs/zeroclaw/tree/master/crates/zeroclaw-tools/src

Hermes Agent tools: https://github.com/NousResearch/hermes-agent/tree/main/tools

Picoclaw tools: https://github.com/sipeed/picoclaw/tree/main/pkg/tools

OpenClaw tools: https://github.com/openclaw/openclaw/tree/main/src/agents/tools

OpenAI Agents SDK tools: https://openai.github.io/openai-agents-python/tools/

OpenAI Responses tools guide: https://developers.openai.com/api/docs/guides/tools

Anthropic computer use docs: https://platform.claude.com/docs/en/agents-and-tools/tool-use/computer-use-tool

MCP Python SDK: https://github.com/modelcontextprotocol/python-sdk

## 01. [ ] `web_search`

Signal: Z, H, P, OA.

Description: Search the public web and return ranked results with title, URL, snippet, date if available, and source metadata.

Useful because: Agents need current facts, news, docs, product info, and source-backed answers beyond model memory.

Expected parameters: `query: string`, `max_results?: int`, `recency_days?: int`, `domains?: string[]`, `exclude_domains?: string[]`, `locale?: string`, `safe_search?: bool`, `search_context_size?: string`.

## 02. [ ] `web_fetch`

Signal: Z, H, P, OA.

Description: Fetch a URL and extract readable text, markdown, links, metadata, and optionally raw HTML.

Useful because: Search gives candidates, but fetch gives the evidence needed for citation, summarization, and precise extraction.

Expected parameters: `url: string`, `format?: enum[text, markdown, html]`, `max_chars?: int`, `timeout_sec?: int`, `headers?: object`, `follow_redirects?: bool`, `extract_main_content?: bool`.

## 03. [ ] `fs_read`

Signal: H, P, Z.

Description: Read a file from the workspace with line ranges, byte limits, and encoding control.

Useful because: It is the safest primitive for inspecting code, configs, docs, logs, and generated artifacts before editing.

Expected parameters: `path: string`, `start_line?: int`, `end_line?: int`, `max_bytes?: int`, `encoding?: string`, `include_line_numbers?: bool`.

## 04. [ ] `fs_write`

Signal: Z, H, P.

Description: Create, overwrite, or append to a file inside an allowed workspace.

Useful because: Agents need to create source files, notes, generated assets, reports, and task outputs.

Expected parameters: `path: string`, `content: string`, `mode?: enum[create, overwrite, append]`, `create_dirs?: bool`, `encoding?: string`, `backup?: bool`.

## 05. [ ] `fs_edit`

Signal: Z, H, P.

Description: Replace an exact text range or exact old string with new content.

Useful because: Small deterministic edits are much safer than rewriting whole files, especially in codebases.

Expected parameters: `path: string`, `old_string: string`, `new_string: string`, `expected_replacements?: int`, `create_backup?: bool`, `dry_run?: bool`.

## 06. [ ] `fs_list`

Signal: H, P, Z.

Description: List files and directories under a path with recursion and filtering.

Useful because: Agents need a cheap map of the workspace before reading or editing specific files.

Expected parameters: `path?: string`, `recursive?: bool`, `include_hidden?: bool`, `max_depth?: int`, `max_entries?: int`, `sort?: enum[name, modified, size]`.

## 07. [ ] `fs_glob`

Signal: Z, P, H.

Description: Return paths matching one or more glob patterns.

Useful because: It quickly finds likely files without scanning file contents.

Expected parameters: `patterns: string[]`, `root?: string`, `include_hidden?: bool`, `max_results?: int`, `exclude?: string[]`.

## 08. [ ] `fs_grep`

Signal: Z, H, P.

Description: Search file contents with plain text or regex patterns and return matched lines with context.

Useful because: It is the main discovery tool for symbols, errors, TODOs, config keys, and usage sites.

Expected parameters: `pattern: string`, `root?: string`, `glob?: string`, `regex?: bool`, `case_sensitive?: bool`, `context_lines?: int`, `max_results?: int`.

## 09. [ ] `shell_exec`

Signal: H, P, AN, OA.

Description: Run a shell command in a sandbox or controlled workspace.

Useful because: Build, test, lint, package install, data processing, and repository inspection often need native commands.

Expected parameters: `command: string`, `cwd?: string`, `timeout_sec?: int`, `env?: object`, `stdin?: string`, `background?: bool`, `pty?: bool`.

## 10. [ ] `shell_session`

Signal: P, H, O.

Description: Manage long-running or interactive shell processes.

Useful because: Servers, REPLs, watchers, test runners, and interactive CLIs need polling, stdin, and termination.

Expected parameters: `action: enum[list, poll, read, write, kill, send_keys]`, `session_id?: string`, `data?: string`, `keys?: string`, `signal?: string`, `max_output?: int`.

## 11. [ ] `code_execute`

Signal: H, OA.

Description: Execute code snippets in a sandboxed interpreter, usually Python or JavaScript, with file inputs and captured outputs.

Useful because: It supports calculations, parsing, plotting, data cleanup, simulations, and quick validation without exposing raw shell power.

Expected parameters: `language: string`, `code: string`, `files?: string[]`, `timeout_sec?: int`, `packages?: string[]`, `stdin?: string`, `return_files?: bool`.

## 12. [ ] `memory`

Signal: Z, H.

Description: Store, retrieve, update, forget, purge, and export long-term memory entries.

Useful because: Persistent preferences, project facts, decisions, and reusable context make an agent useful across sessions.

Expected parameters: `action: enum[store, search, update, forget, purge, export]`, `key?: string`, `query?: string`, `content?: string`, `category?: string`, `limit?: int`, `ttl?: string`, `metadata?: object`.

## 13. [ ] `todo_update`

Signal: H, O.

Description: Create and maintain a structured task list with status, priority, and notes.

Useful because: Agents need visible state for multi-step work, recovery after interruptions, and user auditability.

Expected parameters: `action: enum[add, update, complete, delete, list, reorder]`, `id?: string`, `title?: string`, `status?: enum[pending, in_progress, blocked, done]`, `priority?: int`, `notes?: string`.

## 14. [ ] `ask_user`

Signal: Z, H.

Description: Ask a focused clarification question or request missing user input.

Useful because: It prevents wrong assumptions when requirements are ambiguous or credentials, choices, and preferences are needed.

Expected parameters: `question: string`, `context?: string`, `options?: string[]`, `default?: string`, `urgency?: enum[low, normal, high]`.

## 15. [ ] `approval_request`

Signal: H, AN, OA.

Description: Ask for explicit human approval before risky or irreversible actions.

Useful because: It creates a safety boundary for destructive commands, purchases, account changes, email sends, and external side effects.

Expected parameters: `action_summary: string`, `risk_level: enum[low, medium, high]`, `affected_resources?: string[]`, `proposed_command?: string`, `proposed_payload?: object`, `timeout_sec?: int`.

## 16. [ ] `safety_check`

Signal: H, Z, AN.

Description: Evaluate a URL, command, file path, dependency, or tool request against safety policies.

Useful because: Agents are vulnerable to prompt injection, path traversal, unsafe URLs, credential exposure, and dangerous shell commands.

Expected parameters: `target_type: enum[url, command, file_path, dependency, tool_args, text]`, `content: string`, `policy?: string`, `context?: string`, `return_reasons?: bool`.

## 17. [ ] `http_request`

Signal: Z, H, P.

Description: Call HTTP APIs with methods, headers, query params, request body, and auth profiles.

Useful because: Many agent tasks need structured API calls that are more precise than browser or web fetch tools.

Expected parameters: `method: enum[GET, POST, PUT, PATCH, DELETE]`, `url: string`, `headers?: object`, `query?: object`, `body?: object|string`, `auth_profile?: string`, `timeout_sec?: int`.

## 18. [ ] `browser_navigate`

Signal: Z, H, AN.

Description: Open or navigate a stateful browser session to a URL.

Useful because: Login-gated apps, dynamic websites, forms, and multi-page workflows cannot be handled reliably by stateless fetch alone.

Expected parameters: `url: string`, `session_id?: string`, `wait_until?: enum[load, domcontentloaded, networkidle]`, `viewport?: object`, `user_agent?: string`.

## 19. [ ] `browser_interact`

Signal: Z, H, AN.

Description: Interact with a browser page using selectors, coordinates, keyboard input, and scrolling.

Useful because: Real-world web tasks require clicking, typing, selecting options, downloading files, and navigating application state.

Expected parameters: `session_id: string`, `action: enum[click, type, select, scroll, hover, back, forward, wait]`, `selector?: string`, `text?: string`, `coordinates?: object`, `value?: string`, `timeout_sec?: int`.

## 20. [ ] `screenshot`

Signal: Z, H, AN.

Description: Capture a browser page, desktop, element, or PDF page as an image.

Useful because: Visual state, charts, PDFs, errors, and GUI layouts often cannot be understood from DOM text alone.

Expected parameters: `target: enum[browser, desktop, url, element, pdf_page]`, `session_id?: string`, `path?: string`, `selector?: string`, `full_page?: bool`, `page_number?: int`.

## 21. [ ] `computer_use`

Signal: H, AN, OA.

Description: Control a GUI environment with screenshot, mouse, keyboard, and scroll actions.

Useful because: It unlocks desktop applications, browser automation fallback, file managers, IDEs, and legacy tools that lack APIs.

Expected parameters: `action: enum[screenshot, click, double_click, move, drag, type, key, scroll]`, `x?: int`, `y?: int`, `text?: string`, `keys?: string[]`, `display_id?: int`, `duration_ms?: int`.

## 22. [ ] `git_status`

Signal: Z, H by shell fallback.

Description: Inspect repository branch, staged changes, unstaged changes, and untracked files.

Useful because: Before editing or committing, the agent must know the working tree state and avoid overwriting user work.

Expected parameters: `repo_path?: string`, `include_untracked?: bool`, `include_ignored?: bool`, `branch?: bool`.

## 23. [ ] `git_diff`

Signal: Z, H by shell fallback.

Description: Show repository diffs for working tree, staged changes, commits, or selected files.

Useful because: It lets the agent verify its modifications and explain exactly what changed.

Expected parameters: `repo_path?: string`, `target?: string`, `staged?: bool`, `files?: string[]`, `context_lines?: int`, `stat?: bool`.

## 24. [ ] `git_apply_patch`

Signal: OA, Z, H.

Description: Apply a unified diff or generated patch to a repository.

Useful because: Patch application is more auditable and reversible than ad hoc file rewrites.

Expected parameters: `repo_path?: string`, `patch: string`, `dry_run?: bool`, `check_only?: bool`, `strip?: int`, `reverse?: bool`.

## 25. [ ] `git_commit`

Signal: Z, H by shell fallback.

Description: Create branches, stage files, commit, tag, and optionally push.

Useful because: It turns agent edits into clean version control units that can be reviewed or rolled back.

Expected parameters: `repo_path?: string`, `message: string`, `files?: string[]`, `branch?: string`, `create_branch?: bool`, `tag?: string`, `push?: bool`.

## 26. [ ] `project_index`

Signal: Z, H.

Description: Build or refresh a high-level map of project files, languages, entry points, package manifests, tests, and important symbols.

Useful because: It helps agents reason over a codebase before searching deeply or making edits.

Expected parameters: `root?: string`, `languages?: string[]`, `include?: string[]`, `exclude?: string[]`, `refresh?: bool`, `max_files?: int`.

## 27. [ ] `knowledge_search`

Signal: Z, H, OA.

Description: Search indexed project knowledge, uploaded documents, notes, or vector stores.

Useful because: It gives agents retrieval over private context and long documents without loading everything into the prompt.

Expected parameters: `query: string`, `sources?: string[]`, `limit?: int`, `search_mode?: enum[keyword, embedding, hybrid]`, `filters?: object`, `include_snippets?: bool`.

## 28. [ ] `knowledge_ingest`

Signal: Z, H, OA, MCP.

Description: Add files, URLs, notes, or directories into a searchable knowledge index.

Useful because: Search quality depends on reliable ingestion, chunking, metadata, deduplication, and refresh behavior.

Expected parameters: `inputs: string[]`, `collection: string`, `chunk_size?: int`, `chunk_overlap?: int`, `metadata?: object`, `refresh?: bool`, `delete_missing?: bool`.

## 29. [ ] `pdf_read`

Signal: Z, O.

Description: Extract text, pages, tables, and rendered page images from PDFs.

Useful because: PDFs are common in research, invoices, contracts, reports, and manuals, and they often mix text with visual layout.

Expected parameters: `path_or_url: string`, `pages?: string`, `extract_tables?: bool`, `render_pages?: bool`, `password?: string`, `max_chars?: int`.

## 30. [ ] `document_extract`

Signal: H, O, OA.

Description: Parse common document formats such as DOCX, PPTX, HTML, Markdown, JSON, XML, logs, and plain text.

Useful because: Agents need a consistent extraction layer across user files before summarizing, searching, or transforming them.

Expected parameters: `path_or_url: string`, `file_type?: string`, `extraction_mode?: enum[text, structured, metadata]`, `pages_or_sections?: string`, `max_chars?: int`, `include_metadata?: bool`.

## 31. [ ] `data_table`

Signal: Z data management pattern, OA code interpreter pattern.

Description: Inspect, query, transform, and write tabular files such as CSV, TSV, XLSX, Parquet, and JSONL.

Useful because: Many agent tasks involve cleaning data, analyzing tables, converting formats, and producing spreadsheets.

Expected parameters: `action: enum[inspect, query, transform, write, convert]`, `path: string`, `sheet?: string`, `query?: string`, `output_path?: string`, `sample_rows?: int`.

## 32. [ ] `database_query`

Signal: Common agent integration pattern, adjacent to HTTP and data management tools.

Description: Execute parameterized read-only queries against configured SQL or NoSQL databases.

Useful because: Business agents often need operational data from databases without giving the model broad write access.

Expected parameters: `connection_id: string`, `query: string`, `parameters?: object`, `read_only?: bool`, `timeout_sec?: int`, `max_rows?: int`.

## 33. [ ] `cron_schedule`

Signal: H, P, O.

Description: Create, list, update, pause, resume, and delete delayed or recurring agent jobs.

Useful because: Agents often need reminders, repeated checks, follow-ups, monitoring, and long-running automation.

Expected parameters: `action: enum[create, list, update, pause, resume, delete]`, `schedule?: string`, `timezone?: string`, `task?: string`, `payload?: object`, `job_id?: string`, `max_runs?: int`.

## 34. [ ] `vision_analyze`

Signal: H, O, AN.

Description: Analyze images, screenshots, diagrams, charts, and UI states with an optional task prompt.

Useful because: Many files and interfaces are visual, and the agent needs structured observations before acting.

Expected parameters: `image_path_or_url: string`, `task?: string`, `prompt?: string`, `regions?: object[]`, `max_tokens?: int`, `include_ocr?: bool`.

## 35. [ ] `image_generate`

Signal: Z, H, O, OA.

Description: Generate or edit images from prompts and optional reference images.

Useful because: It supports design, thumbnails, diagrams, visual brainstorming, and user-facing creative outputs.

Expected parameters: `prompt: string`, `size?: string`, `style?: string`, `count?: int`, `transparent_background?: bool`, `reference_images?: string[]`, `output_path?: string`.

## 36. [ ] `message_send`

Signal: Z, H, P, O.

Description: Send messages or notifications through channels such as Slack, Discord, push notifications, or internal chat.

Useful because: Agents need to notify humans, report completion, request attention, and coordinate with teams.

Expected parameters: `channel: string`, `recipient?: string`, `message: string`, `attachments?: string[]`, `thread_id?: string`, `urgent?: bool`, `dry_run?: bool`.

## 37. [ ] `workspace_connector`

Signal: Z, H.

Description: A scoped connector for email, calendar, docs, drive, tasks, and enterprise workspace APIs.

Useful because: Real assistants need user and company context from Gmail, Outlook, Google Drive, Microsoft Graph, Notion, Jira, or similar systems.

Expected parameters: `provider: string`, `action: string`, `account_id?: string`, `query?: string`, `resource_id?: string`, `payload?: object`, `time_range?: object`, `dry_run?: bool`.

## 38. [ ] `mcp_tool`

Signal: Z, H, P, OA, MCP.

Description: Connect to MCP servers, list their tools, and call selected MCP tools with typed arguments.

Useful because: MCP gives agents a standard way to access external tools, resources, and prompts without hardcoding every integration.

Expected parameters: `action: enum[connect, list_tools, call_tool, disconnect]`, `server: string`, `transport?: enum[stdio, sse, streamable_http]`, `tool_name?: string`, `arguments?: object`, `timeout_sec?: int`.

## 39. [ ] `tool_search`

Signal: Z, H, OA.

Description: Search, rank, and load deferred tools only when relevant to the current task.

Useful because: Large tool surfaces waste context and confuse models, while tool search keeps the active schema small.

Expected parameters: `query: string`, `namespace?: string`, `max_results?: int`, `include_deferred?: bool`, `required_capabilities?: string[]`, `load?: bool`.

## 40. [ ] `agent_delegate`

Signal: H, P, O, OA.

Description: Delegate a bounded subtask to a specialized subagent or agent-as-tool and return its result.

Useful because: Complex work benefits from role specialization, parallel exploration, verification agents, and isolated context windows.

Expected parameters: `task: string`, `agent_role?: string`, `context?: string`, `files?: string[]`, `tools_allowed?: string[]`, `budget?: object`, `return_format?: string`.

## Implementation notes

Use typed JSON schemas for every tool. Return a consistent result envelope with `success`, `data`, `error`, `warnings`, `artifacts`, and `provenance`.

Mutating tools should support `dry_run` or require `approval_request` when risk is medium or high.

All filesystem, shell, browser, and computer tools should run inside a workspace sandbox with path guards, output caps, timeouts, audit logs, and secret redaction.

Implement tools in this order unless your product is highly domain-specific. The first 16 create the basic safe agent loop. Tools 17 through 33 unlock real workflows. Tools 34 through 40 add multimodal, integration, extensibility, and multi-agent capabilities.
