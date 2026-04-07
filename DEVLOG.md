## DevLog
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
