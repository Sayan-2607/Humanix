# HUMANIX API v1

Base path: `/api/v1`. JSON in, JSON out (`Content-Type: application/json`). Unknown fields are rejected.

Errors use HTTP status codes plus `{"error": {"code": "...", "message": "..."}}`. Codes: 400 bad JSON · 404 · 405 · 413 too large · 415 · 422 validation · 429 rate limited · 502 upstream (Crossref).

Offsets (`start`, `end`) are **Unicode code-point** positions in the *normalised* text that the endpoint returns (`normalized_text`, `original_text`). In JavaScript, use `Array.from(text).slice(start, end)`.

---

### GET /health
`{"status":"ok","version":"1.0.0","uptime_seconds":12}`. Also available at `/health`.

### GET /meta
Returns audiences, citation styles, limits, the privacy statement and the endpoint list.

### POST /analyze
```json
{"text": "...", "audience": "general|health|government|youth|academic"}
```
Returns `analysis` with:
- `counts`: words, sentences, syllables, complex_words, averages, reading time
- `readability`: flesch_reading_ease, flesch_kincaid_grade, gunning_fog, smog, coleman_liau, automated_readability_index, consensus_grade, ease_band, notes
- `lexical`: type_token_ratio, mattr, mtld, mtld_reliable
- `repetition`: repeated_terms, repeated_trigram_ratio, repeated_sentence_openers
- `issues`: long_sentences, passive_voice_candidates, wordy_or_complex_phrases (each with offsets)
- `audience`: target_grade, meets_target, grade_gap
- `recommendations`: `[{priority, message, evidence}]`
- `disclaimer`

The response also includes `normalized_text`.

### POST /simplify
```json
{"text": "...", "level": "standard|conservative", "audience": "health", "preserve_terms": ["Ayushman Bharat"]}
```
Returns `original_text`, `revised_text`, `applied[]` and `suggestions[]` (each `{from, to, kind, start, end, reason}`), `before`/`after` snapshots (grade, ease, words per sentence), `remaining_issues`, `preservation` (as in /compare) and `method`.

### POST /compare
```json
{"original": "...", "revised": "..."}
```
Returns `preservation`: `lexical_cosine`, `char_trigram_cosine`, `numbers` / `modal_verbs` / `named_entities` / `citations` / `urls` as `{missing[], added[]}`, `negation_counts`, `risk` (low/medium/high), `warnings[]` and `method`.

### POST /originality/check
```json
{"text": "...", "references": [{"id": "who", "title": "WHO factsheet", "text": "..."}], "shingle_size": 5}
```
Accepts up to 25 references; `shingle_size` ranges from 3 to 12. Returns `originality`: normalized_text, total_words, matched_words, overlap_percent, unattributed_percent, `references[]` (containment, jaccard, tfidf_cosine, matched_words), `matches[]` (`{reference_id, start, end, words, text, attribution: quoted|cited|unattributed}`), interpretation and scope.

### POST /citations/extract
```json
{"text": "..."}
```
Returns `citations`: `citations[]` (`{type: doi|url|author_year|numeric, value, start, end}`), `claims_possibly_needing_citation[]` (`{sentence, start, end, reason}`), sentences_total and sentences_with_citation.

### POST /citations/format
```json
{"style": "apa", "source": {"type": "article", "authors": [{"given": "Ana", "family": "García"}],
  "title": "...", "container": "Journal", "year": "2021", "volume": "12", "issue": "3",
  "pages": "45–67", "doi": "10.1234/x", "url": "", "publisher": "", "accessed": ""}}
```
Omit `style` to get all four styles. You can send `sources: [...]` (up to 200) instead of `source`. The response is `formatted`: `[{ "apa": {text, html, warnings[]} , ... }]`. In `text`, italics are marked with `*asterisks*`; `html` is escaped and uses `<i>`.

### GET /citations/lookup?doi=10.xxxx/yyyy
Looks the DOI up through Crossref and returns `source` (with `verified: true`) and `formatted` in all styles. Returns 404 if Crossref has no record (the DOI may be registered with DataCite instead), 502 if Crossref is unreachable, and 503 if lookups are disabled.

### POST /documents/extract
`multipart/form-data` with a `file` field (`.docx`, `.txt`, `.md`). Returns `{filename, text, characters, truncated}`. PDFs are parsed in the browser by the web app.
