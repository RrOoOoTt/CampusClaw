# Knowledge retrieval delta spec

## R1 Chunk indexing
- On successful teacher upload, the original file and `knowledge_entries.body` stay byte-for-byte unchanged while chunks are created and indexed; an embedding failure preserves material and marks chunks failed rather than producing partially ready vectors.
- Default auto chunks are at most 800 Unicode characters with 80-character overlap, preferring blank lines, newlines and sentence ends. Custom accepts max 100–2000, overlap 0–50%, separator newline/blankline/period and optional removal of URLs/emails or whitespace folding. Hierarchy retains Markdown #–### headings in their chapter and windows long chapters. Offsets refer to the processed chunk source.
- Teacher may reindex a material in their class with new parameters; student upload/reindex is 403, cross-class reindex is 404. Existing materials with no chunks receive auto indexing on startup.

## R2 Class-scoped retrieval
- Both teacher and student may search. `class_id` in query, JSON or headers never overrides the authenticated class.
- Keyword queries only MySQL ngram FULLTEXT with `class_id` and `index_status='ready'`; no embedding/Qdrant call. Vector queries embed the query, Qdrant filters payload class, score <0.35 is discarded, and MySQL rechecks class and ready status before returning text. Hybrid independently filters both paths, ranks with RRF k=60, and only includes candidates in at least one path.
- Each hit has material ID/title, chunk ID/index, processed-text character start/end and excerpt from MySQL, plus a link to authorized material detail. B-only text searched from A returns 200 and no hits; never 403/404 from the search endpoint.
- Blank queries return 400. Unrelated queries with no sufficiently relevant candidates return 200, empty hits and `资料中未找到相关内容`. Qdrant unavailable: keyword still works; vector/hybrid return 503.

## R3 Cited answer
- Ask retrieves current user class's hybrid top four using only the latest question. With zero hits, return fixed `资料中未找到相关内容`, empty citations, and do not invoke chat gateway.
- With hits, send numbered titles/chunk indices/passages to chat gateway and return a short answer with [1] etc. Citation array order matches the numbered sources and includes material link, title, index, offsets and excerpt.
- Optional prior turns accept only user/assistant roles; any client system turn or class field is ignored. No stream or unrestricted model answer is part of this change.

## R4 Security and deployment
- Browser has no direct Qdrant/model URL or key. Compose exposes only web's port. Qdrant point payload contains IDs and class metadata, no chunk text. API performs a second SQL class check even if Qdrant returns another class's point.
- Model credentials are server environment only. Demo gateway is explicitly labeled simulated; configured real OpenAI-compatible gateways use separate embedding/chat calls. Failure of a required dependency returns 503 without cross-class content.
