# Apply and verify

- [x] Add MySQL chunks table, ngram index and Qdrant collection with cosine/dimension check. Schema and topology are specified; live Compose verification pending.
- [x] Implement pure auto/custom/hierarchy chunking and unit tests. Unit cases written; Go execution pending.
- [x] Hook upload, teacher reindex and startup backfill. Failure path implemented; live fault scenario pending.
- [x] Implement keyword/vector/hybrid with independent score filters, class filters in both paths and SQL back-check. Both paths filter and back-check class; live scenarios pending.
- [x] Implement ask and citations with zero-hit short circuit. Zero-hit return occurs before chat call; live scenario pending.
- [x] Add UI, documentation and repeatable smoke checks. Verify front-end build and Python script syntax.
- [ ] Run Compose integration scenarios on a Docker-capable machine; capture actual output in `verification.md`, then archive this change into main specs.
