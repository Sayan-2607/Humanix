# HUMANIX — Plain Language Desk

**Write so everyone can read it.**

HUMANIX helps NGOs, health workers, public offices, teachers, and students turn hard-to-read text into information people can actually use.

A clinic notice, a government scheme application, or a disaster advisory only works if its readers understand it.

HUMANIX makes important information easier to read, understand, verify, and share.

- **Privacy-first:** No server-side document storage.
- **Lightweight:** One Go binary with an embedded web interface.
- **Portable:** Deployable anywhere that supports Go or containers.
- **Transparent:** Every readability score and suggested edit is explainable.
- **Responsible:** Preserves critical meaning instead of blindly rewriting text.

---

## Features

| Tool | What it does | How it works |
|---|---|---|
| **Readability Analysis** | Measures reading difficulty against the target audience. Highlights long sentences, passive voice, difficult words, and wordy expressions. | Five published readability formulas, median consensus, TTR, MATTR, MTLD, and repetition statistics. |
| **Plain Language Rewriting** | Rewrites text into simpler language and displays tracked changes. | Approximately 100 curated substitutions, with protected terms and context-dependent suggestions. |
| **Meaning Verification** | Identifies potentially important changes between original and revised text. | Checks numbers, negations, obligation words, names, citations, URLs, lexical similarity, and character-trigram similarity. |
| **Source Overlap Analysis** | Finds passages that overlap with supplied source documents. | Word 5-gram shingling, containment, Jaccard similarity, and TF-IDF cosine similarity. |
| **Citation Management** | Extracts citations, identifies potentially unsupported statements, and formats references. | DOI, URL, author-year and numeric extraction; Crossref metadata lookup; APA 7, MLA 9, IEEE, and Chicago. |
| **Draft Management** | Saves and restores document versions. | Browser localStorage. Drafts are never sent to the server. |

### Supported documents

| Format | Processing |
|---|---|
| `.txt` | Server-side |
| `.md` | Server-side |
| `.docx` | Server-side, using Go standard libraries |
| `.pdf` | Browser-side using PDF.js |

**Privacy note:** PDF files are parsed in the browser, so the PDF itself never needs to leave the device.

---

## Technology Stack

| Component | Technology |
|---|---|
| Backend | Go 1.22+ |
| Frontend | Vanilla HTML, CSS, JavaScript |
| Architecture | Modular, stateless monolith |
| Document processing | Go standard library, PDF.js |
| Citation metadata | Crossref REST API |
| UI delivery | `go:embed` |
| Deployment | Docker and cloud platforms |
| Testing | Go testing framework |

---

## Getting Started

### Prerequisites

- Go 1.22 or later
- Git
- Docker (optional)

### 1. Clone the repository

```bash
git clone https://github.com/Sayan-2607/Humanix.git
cd Humanix
```

### 2. Run locally

```bash
go run ./cmd/server
```

Open:

```text
http://localhost:8080
```

### 3. Run tests

```bash
go test ./...
```

### 4. Build production binary

```bash
make build
```

Run the generated binary:

```bash
./bin/humanix
```

### Windows PowerShell

```powershell
go run ./cmd/server
```

To change the port:

```powershell
$env:PORT=3000
go run ./cmd/server
```

---

## Deployment

HUMANIX is designed to be portable across cloud platforms.

### Vercel

The repository includes `vercel.json`.

1. Push the project to GitHub.
2. Open Vercel.
3. Select **Add New → Project**.
4. Import the HUMANIX repository.
5. Configure optional environment variables.
6. Deploy.

CLI deployment:

```bash
npm install -g vercel
vercel --prod
```

**Important:** Vercel's Go runtime is in Beta. Request bodies are limited to 4.5 MB, and rate-limit counters are instance-specific. The included Dockerfile provides an alternative deployment route where supported.

### Docker

Build the image:

```bash
docker build -t humanix .
```

Run the container:

```bash
docker run -p 8080:8080 humanix
```

The intended production image uses a distroless base, runs as a non-root user, and targets an approximately 15 MB image size.

### Supported deployment platforms

| Platform | Deployment method |
|---|---|
| Vercel | Go runtime or supported container deployment |
| Render | Blueprint using `render.yaml` |
| Railway | Deploy using Dockerfile |
| Fly.io | `fly launch --copy-config && fly deploy` |
| Google Cloud Run | `gcloud run deploy humanix --source . --allow-unauthenticated` |
| AWS | Container deployment |
| Azure | Container deployment |
| Koyeb | Deploy using Dockerfile |
| Northflank | Deploy using Dockerfile |
| VPS | Production Go binary behind Caddy/Nginx |

---

## Configuration

All environment variables are optional.

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server listening port |
| `ALLOWED_ORIGINS` | `*` | Comma-separated CORS origins |
| `MAX_TEXT_CHARS` | `100000` | Maximum characters per text field |
| `MAX_UPLOAD_BYTES` | `4194304` | Maximum upload size |
| `RATE_LIMIT_RPM` | `120` | Requests per minute per IP, per instance |
| `CROSSREF_MAILTO` | Not set | Contact email for Crossref polite-pool requests |
| `CROSSREF_ENABLED` | `true` | Enable or disable external DOI lookups |

---

## API Documentation

All API endpoints are available under:

```text
/api/v1
```

Requests and responses use JSON.

Errors follow this structure:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error description"
  }
}
```

### Available endpoints

| Method | Endpoint | Purpose |
|---|---|---|
| GET | `/health` | Health check |
| GET | `/meta` | Application metadata |
| POST | `/api/v1/analyze` | Readability and text quality analysis |
| POST | `/api/v1/simplify` | Plain-language rewriting |
| POST | `/api/v1/compare` | Meaning and change verification |
| POST | `/api/v1/originality/check` | Source overlap analysis |
| POST | `/api/v1/citations/extract` | Citation extraction |
| POST | `/api/v1/citations/format` | Citation formatting |
| GET | `/api/v1/citations/lookup?doi=` | DOI metadata lookup |
| POST | `/api/v1/documents/extract` | Document text extraction |

### Example API request

```bash
curl -s http://localhost:8080/api/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "text": "In order to apply, individuals must submit the form prior to 30 June.",
    "audience": "general"
  }'
```

For complete request and response examples, see [`docs/API.md`](docs/API.md).

---

## Project Architecture

```text
Humanix/
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── textutil/
│   │   └── Text normalization, tokenization and syllables
│   │
│   ├── quality/
│   │   └── Readability formulas and quality analysis
│   │
│   ├── plain/
│   │   └── Plain-language rewriting engine
│   │
│   ├── preserve/
│   │   └── Meaning-anchor verification
│   │
│   ├── originality/
│   │   └── Source overlap detection
│   │
│   ├── citation/
│   │   └── Citation extraction and formatting
│   │
│   ├── docx/
│   │   └── DOCX text extraction
│   │
│   └── server/
│       └── HTTP routing, validation and security
│
├── web/
│   └── static/
│       └── HTML, CSS and JavaScript
│
├── docs/
│   ├── API.md
│   └── METHODS.md
│
├── Dockerfile
├── Makefile
├── Procfile
├── render.yaml
├── fly.toml
├── vercel.json
├── go.mod
└── README.md
```

---

## Scientific Foundation and Limitations

HUMANIX uses explainable, deterministic algorithms rather than claiming that a single AI model can reliably determine whether a document is understandable.

### Readability

Five established readability formulas provide a consensus reading-grade estimate. Additional metrics measure lexical diversity and repetition.

### Plain-language rewriting

The rewriting engine uses approximately 100 curated word and phrase substitutions.

For example:

| Original | Suggested |
|---|---|
| In order to | To |
| Utilize | Use |
| Prior to | Before |
| In the event that | If |

The system protects quotations, citations, URLs, email addresses, numbers, and user-defined protected terms.

Context-dependent replacements are suggestions rather than automatic changes.

### Meaning preservation

The meaning verification engine checks critical elements such as:

- Numbers and dates
- Negations
- Must, may, shall, and required
- Names and identifiers
- Citations and references
- URLs and links

It also uses lexical and character-trigram similarity to identify potentially significant changes.

### Source overlap

The originality checker compares submitted text against sources supplied by the user.

It uses:
- Word 5-gram shingling (Broder, 1997)
- Containment similarity
- Jaccard similarity
- TF-IDF cosine similarity

It is not an internet-wide plagiarism detector.

### Important limitations

1. **English only:** Readability formulas and syllable counting are designed for English. Results for Hindi, Odia, or other languages are not validated.
2. **Readability is not comprehension:** Sentence and word length cannot establish whether a reader understands a document.
3. **Human review matters:** The rewriting engine does not automatically restructure sentences.
4. **Meaning verification is lexical:** It can identify many critical changes but cannot prove semantic equivalence.
5. **Originality checks are source-dependent:** Only the sources supplied by the user are checked.
6. **No AI detection or detector evasion:** HUMANIX evaluates readability and text quality, not authorship.

---

## Design Philosophy

HUMANIX started with a broader AI-based humanization concept. Its implementation takes a more focused direction: making public-interest information understandable without compromising its meaning.

| Original blueprint | HUMANIX implementation | Reason |
|---|---|---|
| FastAPI + Next.js | Single Go binary with embedded UI | Simplified deployment |
| PyTorch / Transformer models | Deterministic published algorithms | Explainability and reproducibility |
| PostgreSQL + pgvector | Stateless architecture | Privacy-first operation |
| Redis + Celery | No external infrastructure required | Lightweight deployment |
| JWT-based accounts | Browser-based drafts | Minimal data collection |
| AI humanization | Plain-language rewriting | Better public comprehension |

---

## Roadmap

- [ ] **Indian-language support:** Develop language-specific readability formulas and validated akshara counting for Hindi and Odia.
- [ ] **LLM-assisted rewriting:** Introduce optional sentence restructuring with meaning-anchor verification.
- [ ] **Organisation accounts:** Add optional shared workspaces using PostgreSQL.
- [ ] **Custom glossaries:** Allow organisations to define protected and preferred terminology.
- [ ] **Comprehension evaluation:** Compare readability estimates against real-world comprehension tests and cloze scores.
- [ ] **Accessibility improvements:** Expand support for readers with different accessibility requirements.
- [ ] **Community evaluation:** Test public notices and government communications with real readers.

---

## Intended Users

HUMANIX is designed for people and organisations that need to communicate important information clearly.

- **NGOs:** Make community information accessible.
- **Healthcare workers:** Simplify patient-facing notices and instructions without changing medical meaning.
- **Government offices:** Improve the clarity of public notices and scheme information.
- **Teachers:** Make educational material easier to understand.
- **Students:** Improve the readability and presentation of academic writing.
- **Researchers:** Review citations, document quality, and source overlap.

---

## Privacy and Security

Privacy is a core architectural principle.

- No server-side document storage.
- Browser-based draft storage using localStorage.
- PDF parsing takes place locally in the browser.
- No mandatory database.
- No external AI inference dependency.
- Configurable request limits and CORS.
- Security headers and upload-size validation.

**Note:** Citation lookups may contact the Crossref REST API when enabled. Hosting providers may also maintain their own operational logs.

---

## License

This project is licensed under the **MIT License**.

See the [LICENSE](LICENSE) file for details.

---

## Author

**Sayan Ghosh**

B.Tech Computer Science and Engineering | KIIT University

- GitHub: [@Sayan-2607](https://github.com/Sayan-2607)
- LinkedIn: [Sayan Ghosh](https://linkedin.com/in/sayan-g-600ab5307)
- Portfolio: [sxynn.vercel.app](https://sxynn.vercel.app)

---

<p align="center">
  <strong>HUMANIX — Plain Language Desk</strong>
  <br />
  Making important information understandable for everyone.
  <br /><br />
  <em>Clarity is not a luxury. It is a necessity.</em>
</p>
