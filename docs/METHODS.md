# Methods

Every metric is a deterministic function of the input text. Notation: W = words, S = sentences, Sy = syllables, C = complex words (≥ 3 syllables), L = letters.

## Pre-processing
- **Normalisation:** unify line endings, strip control characters, collapse horizontal whitespace, cap blank lines at one. Wording is never changed.
- **Tokens:** `[\p{L}\p{N}]+(?:['’-][\p{L}\p{N}]+)*`, with offsets in Unicode code points so browsers can map them via `Array.from`.
- **Sentences:** split at `. ! ? …` followed by whitespace and a non-lower-case start; abbreviations (Dr., e.g., et al., months…), single-letter initials and decimals are protected; line breaks are boundaries.
- **Syllables:** vowel-group heuristic with silent-e and -es/-ed handling. Expect roughly 85–90% word-level agreement with dictionaries; errors largely average out across a text.

## Readability (published coefficients)

| Index | Formula | Source |
|---|---|---|
| Flesch Reading Ease | 206.835 − 1.015·W/S − 84.6·Sy/W | Flesch (1948) |
| Flesch–Kincaid Grade | 0.39·W/S + 11.8·Sy/W − 15.59 | Kincaid et al. (1975) |
| Gunning Fog | 0.4·(W/S + 100·C/W) | Gunning (1952) |
| SMOG | 1.0430·√(C·30/S) + 3.1291 | McLaughlin (1969) |
| Coleman–Liau | 0.0588·L₁₀₀ − 0.296·S₁₀₀ − 15.8 | Coleman & Liau (1975) |
| ARI | 4.71·L/W + 0.5·W/S − 21.43 | Smith & Senter (1967) |

The **consensus grade** is the median of the five grade-level indices. The median is used because each formula has different biases (Coleman–Liau and ARI count letters; the others count syllables), and the median is robust to one outlier. SMOG was validated on samples of 30 or more sentences, and the API flags shorter samples.

**Audience targets** (U.S. grade): general 8, health 6, public notices 8, young readers 5, academic 14. These follow widely used plain-language and health-literacy conventions; they are guidance, not standards with legal force.

## Lexical diversity
- **TTR** = types/tokens. It falls as text length grows, so compare TTR only between texts of similar length.
- **MATTR** (Covington & McFall, 2010): mean TTR over a sliding 50-token window, which makes it stable across lengths.
- **MTLD** (McCarthy & Jarvis, 2010): mean length of segments that keep TTR > 0.72, averaged over forward and backward passes. Reported as unreliable below 100 tokens.

## Issues
- **Long sentence:** more than 25 words.
- **Passive-voice candidate:** a form of *be*, an optional adverb, then a word ending in *-ed* or one of ~50 irregular participles. This is a heuristic, so adjectival uses ("is tired") produce false positives.
- **Repetition:** stemmed content words occurring ≥ 3 times and ≥ 15 per 1,000 words; the share of word trigrams (excluding all-stopword trigrams) that occur more than once; sentence openers covering ≥ 20% of sentences.

## Plain-language rewriting
Phrases are matched longest-first, case-insensitively, at word boundaries. Matches that overlap protected spans are skipped. Protected spans are: double-quoted text, back-ticked code, URLs, emails, parenthetical author–year citations, numeric citations and user terms. The original capitalisation pattern is transferred to the replacement, and a preceding *a/an* is corrected. Entries with `Safe=false` (noun/verb ambiguity, technical senses) are returned as suggestions and never applied.

## Meaning preservation
- **Lexical cosine:** cosine of term-frequency vectors over stemmed content words.
- **Character-trigram cosine:** captures inflection-level similarity.
- **Anchors (multiset differences):** numbers (including %), URLs, citations, obligation/certainty words (must, may, might, should, shall, required, optional, mandatory, prohibited, forbidden), non-initial capitalised tokens and acronyms. Negation *counts* (not, no, never, none, nobody, nothing, neither, nor, without, cannot, nowhere, *n't*) are compared.
- **Risk score:** numbers +3, negation count +3, citations removed +2, modal change +2, lexical cosine < 0.5 +2, links removed +1, entities removed +1. A score of ≥ 3 is high, 1–2 is medium, and 0 is low. These weights are hand-set priors (meaning-flipping edits weigh most) and have not been fitted to data.

## Source overlap
For shingle size k (default 5), the document's word k-grams are compared with each source's k-gram set. Consecutive matching k-grams are merged into passages.
- Containment = |D ∩ R| / |D|
- Jaccard = |D ∩ R| / |D ∪ R|
- TF-IDF cosine uses a smoothed IDF, ln((1+N)/(1+df)) + 1, over the document and its sources.

A passage is labelled **quoted** if it lies entirely inside quotation marks, **cited** if its sentence (or a short following sentence) contains an author–year, numeric or DOI marker, and **unattributed** otherwise.

## Citations
Formatting follows the core patterns of APA 7th ed., MLA 9th ed., IEEE and Chicago 17th ed. (notes-bibliography, bibliography entry) for journal articles, books and web pages. Edge cases such as editors, translators, editions and chapters are not modelled. Metadata from Crossref is marked `verified`; anything a client sends is always treated as unverified.

## References
- Broder, A. Z. (1997). On the resemblance and containment of documents. *Compression and Complexity of Sequences*, 21–29.
- Coleman, M., & Liau, T. L. (1975). A computer readability formula designed for machine scoring. *Journal of Applied Psychology, 60*(2), 283–284.
- Covington, M. A., & McFall, J. D. (2010). Cutting the Gordian knot: The moving-average type–token ratio (MATTR). *Journal of Quantitative Linguistics, 17*(2), 94–100.
- Flesch, R. (1948). A new readability yardstick. *Journal of Applied Psychology, 32*(3), 221–233.
- Gunning, R. (1952). *The technique of clear writing*. McGraw-Hill.
- Kincaid, J. P., Fishburne, R. P., Rogers, R. L., & Chissom, B. S. (1975). *Derivation of new readability formulas for Navy enlisted personnel* (Research Branch Report 8-75). Naval Technical Training Command.
- McCarthy, P. M., & Jarvis, S. (2010). MTLD, vocd-D, and HD-D: A validation study of sophisticated approaches to lexical diversity assessment. *Behavior Research Methods, 42*(2), 381–392.
- McLaughlin, G. H. (1969). SMOG grading: A new readability formula. *Journal of Reading, 12*(8), 639–646.
- Smith, E. A., & Senter, R. J. (1967). *Automated readability index* (AMRL-TR-66-220). Wright-Patterson AFB.
