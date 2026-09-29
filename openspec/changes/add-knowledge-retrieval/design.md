# Design: class-scoped retrieval

## Decisions
- D1: Keep the trust boundary in Go. The browser calls same-origin `/api`; Go derives class and role from its existing cookie session on every request. Qdrant and model gateways have no host port mappings.
- D2: Preserve `knowledge_entries.body`. `knowledge_chunks` in MySQL holds processed chunk text, character offsets in the processed text, status and an ngram FULLTEXT index (token length 2). Qdrant holds vectors and IDs/class metadata only. Point ID equals chunk row ID. No source text in vector payload.
- D3: Chunking is a pure function. Auto uses 800 Unicode characters / 80 overlap and nearby paragraph, line or sentence boundaries; custom allows 100–2000 characters, 0–50% overlap, selected separators and optional preprocessing; hierarchy partitions Markdown headings then uses auto windows for long chapters. Original file/body never changes.
- D4: Upload commits the original and both iteration-1 rows first. Indexing is a separate, retryable step. Failures leave the material readable and chunks marked failed; they never return partly ready vectors. Reindex deletes old vectors before replacing chunks. A crash between MySQL and Qdrant can leave an orphan vector, but back-reference to ready MySQL chunks with class filtering prevents disclosure. Reindex repairs it.
- D5: Keyword path is MySQL FULLTEXT ngram with class and ready filters. Vector path embeds the query, filters class in Qdrant, checks cosine >=0.35, then rechecks class/status in MySQL. Hybrid independently filters each path then uses RRF k=60. A Qdrant failure leaves keyword available; vector/hybrid return 503.
- D6: `POST /api/ask` calls hybrid retrieval for latest question only, top 4; no hits returns fixed text without chat call. Chat receives numbered source passages and only this class's text. Prior dialogue is optional and only user/assistant roles are kept; client system messages are discarded. Output uses [n] markers and ordered citations.
- D7: Embedding and chat use separate OpenAI-compatible HTTP calls from Go. URL, models, key and vector dimension come from environment. Local Compose uses a clearly labeled deterministic mock gateway so a third party can run the scenarios without a course credential. Real gateway can replace it through environment; no key goes to React.

## Endpoints
| Method | Path | Access | Result |
|---|---|---|---|
| GET | `/api/search?q=&mode=keyword|vector|hybrid` | session | hits with title, chunk index, processed-text offsets, excerpt, material link; empty hits with fixed message |
| POST | `/api/ask` | session | short answer and ordered citations; empty result never calls chat |
| POST | `/api/materials/{id}/reindex` | teacher + own class | rebuild chunks/vectors with requested strategy; cross-class 404 |
| POST | `/api/materials` | teacher | existing multipart plus optional chunk strategy fields |

## Alternatives
A SQL-only keyword search cannot match paraphrases. A vector-only search has no reliable literal match and makes Qdrant availability a single point of failure. We combine the two but retain keyword service when Qdrant fails. A framework would hide the class filters and status transitions the scenarios need to inspect, so Go coordinates these calls directly. A single DB transaction cannot atomically include Qdrant and the file system; ready status plus reindex is the repair boundary.

## Configuration and deployment
MySQL 8 ngram token size 2; Qdrant cosine collection `campusclaw_chunks`; Qdrant dimension must equal `EMBEDDING_DIM`. Demo gateway is internal. Only Nginx maps a host port. For real models set `GATEWAY_BASE_URL`, `GATEWAY_API_KEY`, `EMBEDDING_MODEL`, `CHAT_MODEL`, and matching dimension, then rebuild the collection and reindex. Never expose a course key or passage content to the browser except authorized search results.
