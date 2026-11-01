# HUMANIX — Plain Language Desk

**Write so everyone can read it.**

HUMANIX helps NGOs, health workers, public offices, teachers and students turn hard-to-read text into information people can actually use. A clinic notice, a scheme application form, or a disaster advisory only works if its readers understand it.

It is a single Go binary, has no external dependencies, and stores nothing. It deploys to Vercel, Render, Railway, Fly.io, Google Cloud Run, Azure, AWS or any machine that runs a container.

## What it does

| Tool | What it gives you | How it works |
|---|---|---|
| **Readability** | Reading grade against the audience's target, long sentences, passive voice, wordy or hard words highlighted in place | Five published readability formulas (median taken as consensus), lexical diversity (TTR, MATTR, MTLD), repetition statistics |
| **Plain language** | A revised text with every change shown as a tracked edit, before/after grade, items left for human review | ~100 curated substitutions ("in order to" → "to", "utilize" → "use"). Quotes, citations, URLs, emails, numbers and your protected terms are never touched. Context-dependent swaps are suggestions only |
| **Check meaning** | Risk rating plus the exact anchors that changed | Diffs of numbers, negations, obligation words (must/may/shall/required), names, citations and links, plus lexical and character-trigram cosine |
| **Source overlap** | Passages of your text that repeat your sources, each labelled quoted / cited / unattributed | Word 5-gram shingling (Broder 1997), containment and Jaccard per source, TF-IDF cosine for topic similarity |
| **Citations** | Citations found, statements that may need a source, references in APA 7 / MLA 9 / IEEE / Chicago | Regex extraction (DOI, URL, author–year, numeric); DOI metadata from the Crossref REST API |
| **Drafts** | Saved versions | Browser `localStorage` only — never sent to the server |

Files: `.txt`, `.md`, and `.docx` (parsed server-side with the standard library) and `.pdf` (parsed in the browser with pdf.js, so the PDF never leaves the device).

## Run locally

Requires Go 1.22+.

```bash
go run ./cmd/server          # http://localhost:8080
go test ./...                # 8 test packages
make build && ./bin/humanix  # production binary (~7 MB)
```

Windows PowerShell:

```powershell
go run ./cmd/server
$env:PORT=3000; go run ./cmd/server
```

## Deploy

### Vercel
`vercel.json` sets `"framework": "go"`. Vercel's Go preset detects `go.mod` and `cmd/server/main.go`, builds the binary and runs it on `$PORT`. The web UI is embedded in the binary, so no separate static build is needed.

1. Push this folder to a GitHub repository.
2. In Vercel, choose **Add New → Project**, import the repo, and keep the defaults.
3. Optionally add environment variables (see below), then deploy.

Or use the CLI: `npm i -g vercel && vercel --prod`.

Notes: Vercel's Go runtime is in **Beta**. Request bodies are limited to 4.5 MB. Each instance keeps its own rate-limit counters. If the Go preset gives you trouble, Vercel's **Container** preset can build the included `Dockerfile` instead.

### Docker (any cloud)
```bash
docker build -t humanix .
docker run -p 8080:8080 humanix
```
The image is distroless, runs as non-root, and is about 15 MB.

| Platform | How |
|---|---|
| Render | New → Blueprint, select repo (`render.yaml` included) |
| Fly.io | `fly launch --copy-config && fly deploy` (`fly.toml`, region `bom`) |
| Google Cloud Run | `gcloud run deploy humanix --source . --allow-unauthenticated` |
| Railway / Koyeb / Northflank | Deploy from repo; they detect the Dockerfile |
| Heroku-style buildpacks | `Procfile` included |
| A VPS | `make build`, copy `bin/humanix`, run behind Caddy/Nginx |

## Configuration (environment variables, all optional)

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | Listen port (set automatically by most platforms) |
| `ALLOWED_ORIGINS` | `*` | CORS origins for the API, comma-separated |
| `MAX_TEXT_CHARS` | `100000` | Maximum characters per text field |
| `MAX_UPLOAD_BYTES` | `4194304` | Maximum upload size |
| `RATE_LIMIT_RPM` | `120` | Requests/minute per IP, per instance |
| `CROSSREF_MAILTO` | — | Your contact email, sent to Crossref for its "polite" pool (recommended) |
| `CROSSREF_ENABLED` | `true` | Set `false` to disable outbound DOI lookups |

## API

All endpoints are under `/api/v1`, accept and return JSON, and report errors as `{"error":{"code","message"}}`. See [`docs/API.md`](docs/API.md) for full request and response examples.

```bash
curl -s localhost:8080/api/v1/analyze -H 'Content-Type: application/json' \
  -d '{"text":"In order to apply, individuals must submit the form prior to 30 June.","audience":"general"}'
```

`GET /health` · `GET /meta` · `POST /analyze` · `POST /simplify` · `POST /compare` · `POST /originality/check` · `POST /citations/extract` · `POST /citations/format` · `GET /citations/lookup?doi=` · `POST /documents/extract`

## Project layout

```
cmd/server/            entrypoint (graceful shutdown, timeouts)
internal/textutil/     normalisation, sentence splitting, tokens with rune offsets, syllables
internal/quality/      readability formulas, lexical diversity, issues, recommendations
internal/plain/        plain-language rewriter + phrase list
internal/preserve/     meaning-anchor verifier
internal/originality/  shingling overlap against supplied sources
internal/citation/     extraction, APA/MLA/IEEE/Chicago formatting, Crossref client
internal/docx/         stdlib .docx text extraction (zip-bomb guarded)
internal/server/       routing, validation, CORS, rate limiting, security headers
web/static/            UI (vanilla HTML/CSS/JS, embedded via go:embed)
```

## Scientific basis and honest limits

Formulas and methods are documented in [`docs/METHODS.md`](docs/METHODS.md). The key limits:

- **English only.** Syllable counting and the readability formulas were developed for English. Scores on Hindi, Odia or other languages are meaningless. See the roadmap.
- **Readability formulas measure surface features** (word and sentence length). They do not measure accuracy, cultural fit or tone. Test important texts with real readers.
- **The rewriter does not restructure sentences.** It makes safe word and phrase swaps and lists long sentences and passive voice for a human to fix. This is deliberate: automatic restructuring can change meaning, which matters most in health, legal and safety texts.
- **The meaning check is lexical**, not a semantic model. It reliably catches dropped "not"s, changed numbers and must→may, but it cannot prove two texts mean the same thing.
- **Source overlap only covers the sources you paste.** It is not an internet-wide plagiarism checker and its percentages are not comparable to Turnitin or iThenticate.
- **No AI-detection or detector evasion.** HUMANIX measures readability, not authorship.

### Changes from the original Python blueprint

| Blueprint | This build | Reason |
|---|---|---|
| FastAPI + Next.js | One Go binary with an embedded UI | Deploys anywhere, including Vercel, with no build pipeline |
| PyTorch / Transformer models (QualityNet, MiniLM) | Deterministic, published algorithms | No training data or GPU is needed, every result can be explained, and inference stays fast |
| PostgreSQL + pgvector, Redis, Celery, JWT accounts | Stateless; drafts kept in the browser | Privacy by design; runs on free serverless tiers |
| "Humanization" | Plain-language rewriting with meaning checks | Aimed at public benefit (comprehension) rather than disguising authorship |

## Roadmap

1. Indian-language support: Hindi and Odia readability needs language-specific formulas and validated syllable (akshara) counting. Treat this as a research task, not a quick port.
2. An optional LLM-assisted sentence splitter, gated behind the meaning-anchor check.
3. Optional accounts and shared workspaces (Postgres, e.g. Neon) for organisations.
4. Glossary mode: an organisation's preferred terms as protected or replacement lists.
5. Evaluation: compare formula grades with comprehension tests (cloze scores) on real community readers.

## License

MIT
#   H u m a n i x  
 