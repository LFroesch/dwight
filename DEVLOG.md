## DevLog

### 2026-04-21 — Agent: bug-fixer
- Closing gap with Claude Code. Next up: multi-file @, bash execution, diff view.

### 2026-04-21: Chat composer overhaul
- Reworked the chat draft input so it auto-grows with content, keeps normal arrow-key navigation inside multiline drafts, and scrolls internally once it hits its max height
- Moved transcript scrolling to page/shift navigation so composing a longer message no longer fights with chat history scrolling
- Added a bordered composer with inline usage hints, then updated help text and README to match the new workflow
- Files touched: `main.go`, `helpers.go`, `update.go`, `views.go`, `README.md`, `WORK.md`

### 2026-04-21: Speaker labels in copied chat messages
- Copy mode clipboard output now prefixes each copied message with its speaker, using the configured username for user messages and `AI`/`System` for non-user roles
- This keeps copied excerpts self-contained when pasted outside Dwight instead of losing who said what
- Files touched: `helpers.go`, `README.md`, `WORK.md`

### 2026-04-21: Gemini provider support for demo deployments
- Added provider-aware model profiles with a new `provider` field; existing profiles normalize to `ollama`
- Added Gemini streaming chat support using Google AI Studio keys from `GEMINI_API_KEY` or `GOOGLE_API_KEY`
- Updated model management UI so profiles can target `ollama` or `gemini`, while model library/pull actions stay explicitly Ollama-only
- Polished demo UX with clearer header/footer model info, better empty-state guidance, richer `?` help text, and a more actionable missing-Gemini-key error
- Updated README with a concrete Gemini demo setup flow for remote deployments that cannot run Ollama
- Files touched: `internal/gemini/gemini.go`, `internal/storage/storage.go`, `helpers.go`, `update.go`, `views.go`, `model.go`, `main.go`, `README.md`, `WORK.md`

### 2026-04-18: Chat clear/close semantics, safer history restore, direct export
- `ctrl+c` is now chat-local: clears the current draft first, closes chat only when the composer is already empty, and interrupts generation while streaming
- Fixed conversation state leakage when opening a fresh chat after loading history or after clearing chat; old conversations no longer stay bound and get overwritten by mistake
- Added direct in-chat markdown export via `ctrl+o`
- Conversation exports now live under `~/.local/share/dwight/exports/<project>/<day>/` with cleaner timestamped filenames, and markdown exports include conversation/update timestamps plus per-message timestamps
- History/export views now show export status toasts, and conversation project labels prefer the local repo/folder name over raw remote URLs
- Copy mode now supports multi-select (`space` to mark), redraws cleanly after copy, and supports macOS clipboard via `pbcopy`
- Files touched: `update.go`, `helpers.go`, `views.go`, `internal/storage/storage.go`, `README.md`, `WORK.md`

### 2026-04-04: Interrupt, token/speed stats, copy messages
- `esc` during loading cancels in-flight HTTP request via `context.WithCancel`; `InterruptMsg` cleans state
- ollama.Chat/ChatStream now take `context.Context` — cancellation propagates through HTTP layer
- Token stats format: `2.3s · 45 tok/s · 123 tok` (completion tokens/sec, completion token count)
- `ctrl+y` enters copy mode: j/k navigates messages, `►` highlights selected, `y`/enter yanks to clipboard (tries wl-copy → clip.exe → xclip → xsel)
- Footer contextually shows interrupt/copy hints based on state

### 2026-04-04: Fix @ autocomplete ignoring gitignored files
Removed `isGitignored` check from `scanProjectFiles()` — gitignore filtering excluded files users legitimately want to reference (e.g. `.env`, `go.sum`, build artifacts). Also removed unused `loadGitignore`/`isGitignored` helpers.

### 2026-04-03: Glamour MD + UI polish + ctrl+c fix + fuzzy @ scoring
- Wired glamour for all AI response rendering (dark theme, word-wrapped to chat width); streaming preview uses fallback renderer to avoid per-chunk overhead
- ctrl+c now quits from any view (intercepted before view dispatch in Update)
- Menu redesigned: bordered rounded box, shows active model name in header
- @ autocomplete fuzzy scoring upgraded to character-sequence matching (fzf-style): consecutive basename hits scored +30/+50, depth penalty, length penalty — much better results on partial names

### 2026-04-03: @file autocomplete + accept/reject code blocks + UI overhaul
Added Claude Code-style `@filename` references: typing `@` triggers fzf-like popup, fuzzy filters project files, Tab/Enter selects. On send, `@path` references are resolved — file contents injected as fenced code blocks into the message. AI response code blocks with path hints (` ```lang:path `) trigger accept/refine/reject flow with diff preview for existing files. Enhanced chat header with context window usage bar, scroll indicator, file count. Better empty state.

### 2026-03-23: Doc suite refresh
Updated README to scout standard. Added LICENSE (AGPL-3.0). Updated WORK.md with feature ideas.

### 2026-03-20: Audit
Reviewed full codebase post-rewrite. Architecture is clean (3 internal packages: ollama, storage, styles). 2685 lines, well-structured. Main gaps: streaming not wired, no token/speed stats, no context window indicator, UI lacks Scout-style polish. Model switching works but discoverable only via Alt+,/. This is highest-effort / highest-payoff project for interviews — local AI tooling is hot.

### 2026-03-19: Full rewrite — 6083→2685 lines, internal/ packages
Complete architectural overhaul. Extracted 3 internal packages (ollama, storage, styles). Deleted ~3400 lines of dead/duplicate/stubbed code. Simplified menu from 9 items to 5. Centralized styles with Footer() builder. Consistent view pattern across all screens.

### 2026-03-19: Moved Dwight to Second Brain + Ollama env var fix
Copied from tui-hub/apps/dwight/. Removed Docker management. Added OLLAMA_HOST env var support.
