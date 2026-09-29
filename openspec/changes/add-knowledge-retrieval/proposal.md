# Add class-scoped knowledge retrieval and cited answers

## Why
Iteration 1 stores full text but offers only title search. Teachers and students need to find passages in their own class and trace an answer back to its original material.

## Scope
Add chunk indexing at upload and startup backfill; teacher-only reindex; keyword, vector and hybrid search; cited short answers; a search/ask page. Preserve iteration 1 sessions, roles, material downloads and original body.

## Modified capabilities
None. Existing auth-upload contract stays in force.

## Added capabilities
knowledge-retrieval (see `specs/knowledge-retrieval/spec.md`).

## Non-goals
Cross-class search, semantic reranker, streamed or long chat, arbitrary tools/agents, browser-held model credentials, PDF/Word extraction, and an administrator role. A deterministic mock gateway is provided only to reproduce the pipeline locally; it is not a real language model.
