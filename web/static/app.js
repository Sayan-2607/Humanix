// HUMANIX web client. No framework, no build step. All user/server text is
// inserted with textContent (never innerHTML) except server-escaped citation HTML.
const $ = (s) => document.querySelector(s);
const API = "/api/v1";
const PDFJS = "https://cdnjs.cloudflare.com/ajax/libs/pdf.js/4.10.38/pdf.min.mjs";
const PDFJS_WORKER = "https://cdnjs.cloudflare.com/ajax/libs/pdf.js/4.10.38/pdf.worker.min.mjs";

function h(tag, attrs = {}, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v == null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k === "style") el.style.cssText = v; // CSSOM: allowed under strict CSP
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? "" : v);
  }
  for (const k of kids.flat()) if (k != null && k !== false) el.append(k instanceof Node ? k : String(k));
  return el;
}

// replaceChildren that flattens arrays and drops null/false
function fill(el, ...kids) {
  el.replaceChildren(...kids.flat(Infinity).filter((k) => k != null && k !== false));
  return el;
}

const store = {
  get(k, d) { try { const v = localStorage.getItem(k); return v ? JSON.parse(v) : d; } catch { return d; } },
  set(k, v) { try { localStorage.setItem(k, JSON.stringify(v)); return true; } catch { return false; } },
};

const doc = $("#doc"), statusEl = $("#status");
const setStatus = (msg = "") => { statusEl.textContent = msg; };

async function api(path, body, opts = {}) {
  setStatus("");
  const init = body instanceof FormData ? { method: "POST", body }
    : body ? { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) } : {};
  let res;
  try { res = await fetch(API + path, init); }
  catch { throw new Error("Can't reach the HUMANIX server. Check your connection and try again."); }
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data?.error?.message || `Request failed (HTTP ${res.status}).`);
  return data;
}

async function run(btn, fn) {
  const label = btn.textContent;
  btn.disabled = true; btn.textContent = "Working…";
  try { await fn(); } catch (e) { setStatus(e.message); }
  finally { btn.disabled = false; btn.textContent = label; }
}

function needText() {
  if (!doc.value.trim()) { setStatus("Add some text to the document first."); doc.focus(); return false; }
  return true;
}

const fmt = (n, d = 1) => (typeof n === "number" ? n.toFixed(d).replace(/\.0+$/, "") : n);

// Render text with highlight spans (rune offsets). Overlapping spans are
// layered: each run of characters gets every class that covers it.
function marked(text, spans) {
  const chars = Array.from(text);
  const cls = new Array(chars.length).fill(""), tip = new Array(chars.length).fill("");
  for (const s of spans) {
    for (let i = Math.max(0, s.start); i < Math.min(chars.length, s.end); i++) {
      if (!cls[i].split(" ").includes(s.cls)) cls[i] = (cls[i] + " " + s.cls).trim();
      if (s.title && !tip[i].includes(s.title)) tip[i] = tip[i] ? tip[i] + "; " + s.title : s.title;
    }
  }
  const frag = document.createDocumentFragment();
  let i = 0;
  while (i < chars.length) {
    let j = i + 1;
    while (j < chars.length && cls[j] === cls[i] && tip[j] === tip[i]) j++;
    const piece = chars.slice(i, j).join("");
    frag.append(cls[i] ? h("mark", { class: cls[i], title: tip[i] || null }, piece) : piece);
    i = j;
  }
  return frag;
}

function showMarked(text, spans, legend) {
  const box = $("#marked");
  fill(box, marked(text, spans));
  if (legend) {
    box.prepend(h("ul", { class: "legend", "aria-label": "Highlight key" },
      legend.map(([cls, label]) => h("li", {}, h("mark", { class: cls }, label)))), "\n");
  }
  box.hidden = false;
}

function stats(pairs) {
  return h("dl", { class: "stats" }, pairs.map(([k, v]) => h("div", {}, h("dt", {}, k), h("dd", {}, v))));
}

// ---------- tabs ----------
const tabs = [...document.querySelectorAll('[role="tab"]')];
function selectTab(btn) {
  tabs.forEach((t) => {
    const on = t === btn;
    t.setAttribute("aria-selected", on); t.tabIndex = on ? 0 : -1;
    $("#" + t.getAttribute("aria-controls")).hidden = !on;
  });
  setStatus("");
  if (btn.dataset.tab === "drafts") renderDrafts();
}
tabs.forEach((t, i) => {
  t.addEventListener("click", () => selectTab(t));
  t.addEventListener("keydown", (e) => {
    const d = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
    if (!d) return;
    const n = tabs[(i + d + tabs.length) % tabs.length]; n.focus(); selectTab(n);
  });
});

// ---------- word count ----------
const updateCount = () => {
  const n = (doc.value.match(/[\p{L}\p{N}]+(?:['’-][\p{L}\p{N}]+)*/gu) || []).length;
  $("#count").textContent = `${n} word${n === 1 ? "" : "s"}`;
};
doc.addEventListener("input", () => { updateCount(); $("#marked").hidden = true; });

// ---------- readability ----------
function ruler(r) {
  const max = 18, g = Math.min(Math.max(r.readability.consensus_grade, 0), max);
  const target = r.audience.target_grade;
  const pct = (x) => `${(x / max) * 100}%`;
  const ok = r.audience.meets_target;
  return h("div", { class: "ruler" },
    h("div", { class: "ruler-head" },
      h("div", { class: "grade-big" }, `Grade ${fmt(r.readability.consensus_grade)}`, h("small", {}, "reading level")),
      h("div", { class: "verdict " + (ok ? "ok" : "no") },
        ok ? `Within target for ${r.audience.label.toLowerCase()}`
           : `${fmt(r.audience.grade_gap)} grade${r.audience.grade_gap === 1 ? "" : "s"} above target (${target})`)),
    h("div", { class: "scale", role: "img", "aria-label": `Reading grade ${fmt(g)} on a scale of 0 to ${max}; target ${target} or lower.` },
      h("div", { class: "track" }),
      h("div", { class: "band", style: `left:0;width:${pct(target)}`, title: "Target range" }),
      h("div", { class: "pin", style: `left:${pct(g)}` }),
      h("div", { class: "ticks", "aria-hidden": "true" }, ["0", "4", "8", "12", "16+"].map((t) => h("span", {}, t)))),
  );
}

$("#run-read").addEventListener("click", (e) => run(e.currentTarget, async () => {
  if (!needText()) return;
  const { analysis: r, normalized_text } = await api("/analyze", { text: doc.value, audience: $("#audience").value });
  const c = r.counts, rd = r.readability, lx = r.lexical;
  const out = $("#out-read");
  fill(out, 
    ruler(r),
    stats([
      ["Flesch reading ease", `${fmt(rd.flesch_reading_ease)} (${rd.ease_band})`],
      ["Words per sentence", fmt(c.avg_words_per_sentence)],
      ["Long sentences (25+ words)", r.issues.long_sentences.length],
      ["Words with 3+ syllables", `${c.complex_words} (${fmt((100 * c.complex_words) / Math.max(c.words, 1), 0)}%)`],
      ["Words / sentences", `${c.words} / ${c.sentences}`],
      ["Reading time", `${fmt(c.reading_time_minutes)} min`],
    ]),
    h("h3", {}, "What to fix first"),
    r.recommendations.length
      ? h("ul", { class: "recs" }, r.recommendations.map((x) =>
          h("li", {}, h("span", { class: "pri " + x.priority }, x.priority), x.message, " ", h("span", { class: "note" }, x.evidence))))
      : h("p", {}, "No major issues found for this audience."),
    h("details", {},
      h("summary", {}, "All formulas and word-variety measures"),
      h("div", { class: "table-wrap" }, h("table", {},
        h("tr", {}, h("th", {}, "Measure"), h("th", {}, "Value")),
        [["Flesch–Kincaid grade", rd.flesch_kincaid_grade], ["Gunning fog", rd.gunning_fog], ["SMOG", rd.smog],
         ["Coleman–Liau", rd.coleman_liau], ["Automated Readability Index", rd.automated_readability_index],
         ["Consensus (median of five)", rd.consensus_grade], ["Type–token ratio", lx.type_token_ratio],
         [`MATTR (window ${lx.mattr_window})`, lx.mattr], [`MTLD${lx.mtld_reliable ? "" : " (needs 100+ words)"}`, lx.mtld],
         ["Sentence length SD", c.sentence_length_sd], ["Repeated 3-word sequences", r.repetition.repeated_trigram_ratio]]
          .map(([k, v]) => h("tr", {}, h("td", {}, k), h("td", { class: "num" }, fmt(v, 2)))))),
      rd.notes.map((n) => h("p", { class: "note" }, n)),
      r.repetition.repeated_terms.length ? h("p", { class: "note" }, "Most repeated terms: " +
        r.repetition.repeated_terms.map((t) => `${t.term} (${t.count})`).join(", ")) : null,
      h("p", { class: "note" }, r.disclaimer)),
  );
  const spans = [
    ...r.issues.wordy_or_complex_phrases.map((s) => ({ ...s, cls: "wordy", title: `Try: ${s.suggestion}` })),
    ...r.issues.passive_voice_candidates.map((s) => ({ ...s, cls: "passive", title: "Possible passive voice" })),
    ...r.issues.long_sentences.map((s) => ({ ...s, cls: "long", title: `${s.words} words` })),
  ];
  showMarked(normalized_text, spans, [["long", "long sentence"], ["passive", "passive voice?"], ["wordy", "wordy or hard word"]]);
}));

// ---------- plain language ----------
function diffView(original, applied) {
  const chars = Array.from(original), frag = document.createDocumentFragment();
  let pos = 0;
  for (const c of applied) {
    if (c.start > pos) frag.append(chars.slice(pos, c.start).join(""));
    frag.append(h("del", {}, c.from), h("ins", {}, c.to));
    pos = c.end;
  }
  frag.append(chars.slice(pos).join(""));
  return frag;
}

function preservationBlock(p) {
  return h("div", {},
    h("p", {}, "Meaning check: ", h("span", { class: "risk " + p.risk }, `${p.risk} risk`)),
    p.warnings.length ? h("ul", { class: "plain-list" }, p.warnings.map((w) => h("li", {}, w)))
      : h("p", { class: "note" }, "Numbers, negations, must/may words, names, citations and links all match."),
    h("p", { class: "note" }, `Word overlap ${fmt(p.lexical_cosine, 2)} · character overlap ${fmt(p.char_trigram_cosine, 2)}. This is a lexical check, not proof of identical meaning — read the result.`),
  );
}

$("#run-plain").addEventListener("click", (e) => run(e.currentTarget, async () => {
  if (!needText()) return;
  const keep = $("#keep").value.split(",").map((s) => s.trim()).filter(Boolean);
  const r = await api("/simplify", { text: doc.value, level: $("#level").value, audience: $("#audience").value, preserve_terms: keep });
  const copy = h("button", { class: "ghost", type: "button", onclick: async () => {
    try { await navigator.clipboard.writeText(r.revised_text); copy.textContent = "Copied"; } catch { setStatus("Copy failed; select the text and copy it manually."); }
  } }, "Copy revised text");
  fill($("#out-plain"), 
    stats([
      ["Reading grade", `${fmt(r.before.consensus_grade)} → ${fmt(r.after.consensus_grade)}`],
      ["Words per sentence", `${fmt(r.before.avg_words_per_sentence)} → ${fmt(r.after.avg_words_per_sentence)}`],
      ["Changes made", r.applied.length],
      ["Suggestions to review", r.suggestions.length],
    ]),
    r.applied.length ? h("div", { class: "result-text", "aria-label": "Changes shown inline" }, diffView(r.original_text, r.applied))
      : h("p", {}, "No automatic replacements found. See remaining issues below."),
    h("div", { class: "actions" },
      h("button", { class: "primary", type: "button", onclick: () => {
        $("#revised").value = r.revised_text;
        saveDraft("Before simplifying", r.original_text);
        doc.value = r.revised_text; updateCount(); $("#marked").hidden = true;
        setStatus(""); $("#out-plain").prepend(h("p", { class: "note" }, "Revised text placed in the editor. Your previous version was saved in Drafts."));
      } }, "Use revised text"), copy),
    preservationBlock(r.preservation),
    r.suggestions.length ? [h("h3", {}, "Review these yourself"),
      h("ul", { class: "plain-list" }, r.suggestions.map((s) => h("li", {}, `“${s.from}” → “${s.to}”`, s.reason ? h("span", { class: "note" }, ` — ${s.reason}`) : null)))] : null,
    h("h3", {}, "Still worth rewriting by hand"),
    r.remaining_issues.long_sentences.length
      ? h("ul", { class: "plain-list" }, r.remaining_issues.long_sentences.map((s) => h("li", {}, h("span", { class: "note" }, `${s.words} words: `), s.text)))
      : h("p", { class: "note" }, "No sentences over 25 words."),
    r.remaining_issues.passive_voice_candidates.length
      ? h("p", { class: "note" }, "Possible passive voice: " + r.remaining_issues.passive_voice_candidates.map((p) => `“${p.text}”`).join(", ")) : null,
    h("p", { class: "note" }, r.method),
  );
}));

// ---------- meaning ----------
$("#run-meaning").addEventListener("click", (e) => run(e.currentTarget, async () => {
  if (!needText()) return;
  if (!$("#revised").value.trim()) { setStatus("Paste the revised version to compare."); $("#revised").focus(); return; }
  const { preservation: p } = await api("/compare", { original: doc.value, revised: $("#revised").value });
  const diffRow = (k, d) => (d.missing.length || d.added.length)
    ? h("tr", {}, h("td", {}, k), h("td", {}, d.missing.join(", ") || "—"), h("td", {}, d.added.join(", ") || "—")) : null;
  const rows = [diffRow("Numbers", p.numbers), diffRow("Must / may words", p.modal_verbs), diffRow("Names", p.named_entities),
    diffRow("Citations", p.citations), diffRow("Links", p.urls)].filter(Boolean);
  fill($("#out-meaning"), 
    preservationBlock(p),
    h("p", { class: "note" }, `Negation words: ${p.negation_counts.original} in original, ${p.negation_counts.revised} in revision.`),
    rows.length ? h("div", { class: "table-wrap" }, h("table", {}, h("tr", {}, h("th", {}, "Anchor"), h("th", {}, "Missing"), h("th", {}, "Added")), rows)) : null,
  );
}));

// ---------- overlap ----------
function addRef(title = "", text = "") {
  const box = $("#refs");
  const n = box.children.length + 1;
  const el = h("div", { class: "ref" },
    h("div", { class: "ref-head" },
      h("input", { type: "text", "aria-label": `Source ${n} title`, placeholder: `Source ${n} title`, value: title }),
      h("button", { type: "button", class: "ghost", "aria-label": `Remove source ${n}`, onclick: () => el.remove() }, "Remove")),
    h("textarea", { "aria-label": `Source ${n} text`, placeholder: "Paste the source text here" }));
  el.querySelector("textarea").value = text;
  box.append(el);
}
addRef();
$("#add-ref").addEventListener("click", () => addRef());

$("#run-overlap").addEventListener("click", (e) => run(e.currentTarget, async () => {
  if (!needText()) return;
  const references = [...document.querySelectorAll("#refs .ref")].map((el, i) => ({
    id: `ref-${i + 1}`, title: el.querySelector("input").value.trim() || `Source ${i + 1}`, text: el.querySelector("textarea").value,
  })).filter((r) => r.text.trim());
  if (!references.length) { setStatus("Paste at least one source to compare against."); return; }
  const { originality: o } = await api("/originality/check", { text: doc.value, references, shingle_size: +$("#shingle").value });
  fill($("#out-overlap"), 
    stats([
      ["Words matching a source", `${fmt(o.overlap_percent)}%`],
      ["Matching without quote or citation", `${fmt(o.unattributed_percent)}%`],
      ["Matched passages", o.matches.length],
    ]),
    h("p", {}, o.interpretation),
    h("div", { class: "table-wrap" }, h("table", {},
      h("tr", {}, h("th", {}, "Source"), h("th", {}, "Shared words"), h("th", {}, "Containment"), h("th", {}, "Topic similarity")),
      o.references.map((r) => h("tr", {}, h("td", {}, r.title), h("td", { class: "num" }, r.matched_words),
        h("td", { class: "num" }, `${fmt(100 * r.containment, 1)}%`), h("td", { class: "num" }, fmt(r.tfidf_cosine, 2)))))),
    h("p", { class: "note" }, `Containment: share of your ${o.shingle_size}-word sequences found in the source. Topic similarity: TF-IDF cosine of vocabulary (0–1), which is high for texts on the same subject even without copying. ${o.scope}`),
  );
  showMarked(o.normalized_text, o.matches.map((m) => ({ ...m, cls: m.attribution, title: `${m.attribution} · ${m.reference_title}` })),
    [["unattributed", "no quote or citation"], ["quoted", "quoted"], ["cited", "cited nearby"]]);
}));

// ---------- citations ----------
$("#run-cite").addEventListener("click", (e) => run(e.currentTarget, async () => {
  if (!needText()) return;
  const { citations: c } = await api("/citations/extract", { text: doc.value });
  const labels = { doi: "DOI", url: "Link", author_year: "Author–year", numeric: "Numbered" };
  fill($("#out-cite"), 
    stats([["Citations found", c.citations.length], ["Sentences with a citation", `${c.sentences_with_citation} of ${c.sentences_total}`],
      ["Statements to check", c.claims_possibly_needing_citation.length]]),
    c.citations.length ? h("ul", { class: "plain-list" }, c.citations.map((x) => h("li", {}, h("span", { class: "note" }, labels[x.type] + ": "), x.value))) : null,
    c.claims_possibly_needing_citation.length ? [h("h3", {}, "Statements that may need a source"),
      h("ul", { class: "plain-list" }, c.claims_possibly_needing_citation.map((x) => h("li", {}, x.sentence, h("span", { class: "note" }, ` — ${x.reason}`))))] : null,
    h("p", { class: "note" }, c.note),
  );
  let normalized = doc.value;
  try { normalized = (await api("/analyze", { text: doc.value })).normalized_text; } catch {}
  showMarked(normalized, [
    ...c.citations.map((x) => ({ ...x, cls: "cited", title: labels[x.type] })),
    ...c.claims_possibly_needing_citation.map((x) => ({ ...x, cls: "claim", title: x.reason })),
  ], [["cited", "citation"], ["claim", "may need a source"]]);
}));

function showFormatted(formatted, source) {
  const names = { apa: "APA 7", mla: "MLA 9", ieee: "IEEE", chicago: "Chicago (bibliography)" };
  const first = Object.values(formatted)[0];
  fill($("#out-ref"), 
    source?.verified ? h("p", { class: "note" }, "Metadata retrieved from Crossref. Still check it against the source.") : null,
    ...Object.entries(formatted).map(([k, f]) => {
      const p = h("p", {}); p.innerHTML = f.html; // server-escaped; italics only
      const btn = h("button", { class: "ghost", type: "button", onclick: async () => {
        try { await navigator.clipboard.writeText(f.text.replace(/\*/g, "")); btn.textContent = "Copied"; } catch { setStatus("Copy failed."); }
      } }, `Copy ${names[k]}`);
      return h("div", { class: "cite-out" }, h("strong", {}, names[k]), p, btn);
    }),
    first?.warnings?.filter((w) => !(source?.verified && w.includes("not verified"))).length
      ? h("ul", { class: "plain-list note" }, first.warnings.filter((w) => !(source?.verified && w.includes("not verified"))).map((w) => h("li", {}, w))) : null,
  );
}

$("#doi-form").addEventListener("submit", (e) => {
  e.preventDefault();
  run(e.submitter || e.target.querySelector("button"), async () => {
    const doi = $("#doi").value.trim();
    if (!doi) { setStatus("Enter a DOI, for example 10.1371/journal.pmed.1000097."); return; }
    const r = await api("/citations/lookup?doi=" + encodeURIComponent(doi));
    showFormatted(r.formatted, r.source);
  });
});

$("#ref-form").addEventListener("submit", (e) => {
  e.preventDefault();
  const f = new FormData(e.target);
  const authors = String(f.get("authors") || "").split("\n").map((l) => l.trim()).filter(Boolean).map((l) => {
    if (l.includes(",")) { const [family, given] = l.split(",").map((s) => s.trim()); return { given, family }; }
    const parts = l.split(/\s+/); const family = parts.pop(); return { given: parts.join(" "), family };
  });
  const source = Object.fromEntries(["type", "title", "container", "year", "volume", "issue", "pages", "publisher", "url", "accessed"]
    .map((k) => [k, String(f.get(k) || "").trim()]));
  source.authors = authors;
  run(e.submitter || e.target.querySelector("button"), async () => {
    const r = await api("/citations/format", { source });
    showFormatted(r.formatted[0], null);
  });
});

// ---------- drafts ----------
function saveDraft(title, text) {
  const drafts = store.get("humanix.drafts", []);
  drafts.unshift({ id: Date.now(), title, text, saved: new Date().toLocaleString() });
  return store.set("humanix.drafts", drafts.slice(0, 30));
}
$("#save-draft").addEventListener("click", () => {
  if (!needText()) return;
  const first = doc.value.trim().split(/\s+/).slice(0, 8).join(" ");
  setStatus(saveDraft(first + (doc.value.trim().split(/\s+/).length > 8 ? "…" : ""), doc.value) ? "" : "This browser blocks local storage, so drafts can't be saved.");
  const b = $("#save-draft"); b.textContent = "Saved"; setTimeout(() => (b.textContent = "Save draft"), 1500);
});
function renderDrafts() {
  const list = $("#drafts"), drafts = store.get("humanix.drafts", []);
  if (!drafts.length) { fill(list, h("li", {}, "No drafts yet. Use “Save draft” above the editor.")); return; }
  fill(list, ...drafts.map((d) => h("li", {},
    h("div", {}, h("div", { class: "title" }, d.title), h("div", { class: "note" }, d.saved)),
    h("div", { class: "actions" },
      h("button", { class: "ghost", type: "button", onclick: () => { doc.value = d.text; updateCount(); $("#marked").hidden = true; doc.focus(); } }, "Open"),
      h("button", { class: "ghost", type: "button", onclick: () => {
        store.set("humanix.drafts", store.get("humanix.drafts", []).filter((x) => x.id !== d.id)); renderDrafts(); } }, "Delete")))));
}

// ---------- files ----------
async function pdfText(file) {
  const pdfjs = await import(PDFJS);
  pdfjs.GlobalWorkerOptions.workerSrc = PDFJS_WORKER;
  const pdf = await pdfjs.getDocument({ data: await file.arrayBuffer() }).promise;
  const pages = [];
  for (let i = 1; i <= Math.min(pdf.numPages, 200); i++) {
    const tc = await (await pdf.getPage(i)).getTextContent();
    pages.push(tc.items.map((it) => it.str + (it.hasEOL ? "\n" : "")).join(""));
  }
  return pages.join("\n\n");
}

$("#file").addEventListener("change", async (e) => {
  const file = e.target.files[0]; e.target.value = "";
  if (!file) return;
  const ext = file.name.toLowerCase().split(".").pop();
  setStatus("");
  try {
    let text;
    if (ext === "pdf") {
      text = await pdfText(file);
      if (!text.trim()) throw new Error("No text found in this PDF. It may be a scanned image; run it through OCR first.");
    } else if (ext === "docx") {
      const fd = new FormData(); fd.append("file", file);
      const r = await api("/documents/extract", fd);
      text = r.text; if (r.truncated) setStatus("The document was long, so only the first part was loaded.");
    } else if (["txt", "md"].includes(ext)) {
      text = await file.text();
    } else throw new Error("Open a .txt, .md, .docx or .pdf file.");
    doc.value = text; updateCount(); $("#marked").hidden = true;
  } catch (err) { setStatus(err.message || "Couldn't read that file."); }
});

// ---------- sample ----------
$("#sample").addEventListener("click", () => {
  doc.value = `Notice regarding the vaccination programme

In order to facilitate the commencement of the vaccination programme, all individuals who are eligible are required to obtain a registration number prior to their visit to the health centre. Registration can be completed by means of the online portal or at the reception desk, and individuals who are not able to utilize the online portal will be provided assistance by the staff at the centre in a timely manner.

Approximately 40% of residents have not yet registered. Studies show that early registration reduces waiting times significantly. Children under 12 must not be brought to the adult vaccination session.`;
  updateCount(); $("#marked").hidden = true;
  $("#audience").value = "health";
});

updateCount();
