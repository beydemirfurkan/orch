# Orch Session-Only Full-Stack Gecis Plani

Bu dokuman, `orch` icin sifirdan **session-only** mimariye gecis planini tanimlar.

- Tek kaynak: `session store` (runstore yok, dual-write yok, import yok)
- Kapsam: `interactive` + `run pipeline` ayni runtime ustunde
- Hedef: `opencode-typescript` seviyesinde context yonetimi, compaction ve token-budget kontrolu

## 1) Hedef Mimari

### 1.1 Temel prensipler
- Tek source of truth: `sessions/messages/parts/summaries/metrics`
- Her tur kalici: user/assistant/tool/reasoning/file olaylari part bazli saklanir
- Prompt her turda DB'den yeniden derlenir (context assembler)
- Model context limiti token-budget ile yonetilir
- Overflow durumunda auto-compaction devreye girer
- Buyuk tool output'lari truncation + artifact path ile yonetilir

### 1.2 Session veri modeli
- `sessions`
  - `id`, `project_id`, `title`, `mode`, `status`, `created_at`, `updated_at`
- `messages`
  - `id`, `session_id`, `role`, `parent_id`, `provider_id`, `model_id`, `finish_reason`, `error`, `created_at`
- `parts`
  - `id`, `message_id`, `type`, `payload_json`, `compacted`, `created_at`, `updated_at`
- `session_summaries`
  - `session_id`, `summary_text`, `updated_at`
- `session_metrics`
  - `session_id`, `input_tokens`, `output_tokens`, `total_cost`, `turn_count`, `updated_at`

### 1.3 Part tipleri
- `text`
- `tool`
- `reasoning`
- `file`
- `compaction`
- `stage` (run safhalari icin)
- `error`

## 2) Uygulama Fazlari

## Faz 1 - Storage Foundation

### Is kalemleri
- Yeni tablolarin migration ile eklenmesi
- Zorunlu indexler:
  - `messages(session_id, id)`
  - `parts(message_id, id)`
  - `sessions(project_id, updated_at desc)`
- Session repository katmani (`internal/session/store`)
- Atomik yazim kurali: 1 turn = 1 transaction boundary

### Teslim kriterleri
- Migration idempotent calisir
- DB schema testleri gecer
- Concurrent write testleri gecer

---

## Faz 2 - Session Domain Service

### Is kalemleri
- `internal/session` API:
  - `Create`, `Get`, `List`, `Resume`
  - `AppendUserMessage`
  - `AppendAssistantMessage`
  - `AppendPart`
  - `StreamMessages`
- Message/part validation
- Error taxonomy:
  - `ContextOverflow`
  - `ToolExecutionInterrupted`
  - `AuthError`
  - `ProviderError`

### Teslim kriterleri
- Unit test coverage (CRUD + validation + stream)
- Race/concurrency testleri gecer

---

## Faz 3 - Prompt Assembler ve Context Engine

### Is kalemleri
- `toModelMessages` benzeri assembler
- Provider capability aware donusum:
  - media destek/degisimi
  - tool-call/result eslestirme
  - interrupted tool fallback
- Synthetic reminder kurallari (gereken yerde)

### Teslim kriterleri
- Golden tests (parts -> expected model messages)
- Provider-specific conversion testleri gecer

---

## Faz 4 - Token Budget ve Overflow Detection

### Is kalemleri
- Model bazli limit parametreleri:
  - `context_limit`
  - `max_output`
  - `reserved`
- Usable input hesabı:
  - `usable_input = context_limit - reserved`
- Turn oncesi token estimate
- Overflow tespiti ve compaction tetigi

### Teslim kriterleri
- Deterministic overflow testleri
- Uzun context senaryolarinda dogru tetik

---

## Faz 5 - Compaction Engine

### Is kalemleri
- Auto summary uretimi (assistant message + compaction part)
- Summary template:
  - Goal
  - Instructions
  - Discoveries
  - Accomplished
  - Next steps
  - Relevant files/directories
- Prune stratejisi:
  - Eski tool output temizlenir
  - Metadata ve input korunur
- Replay-safe continuation

### Teslim kriterleri
- Long-session integration tests gecer
- Compaction sonrasi devam dogru ve tutarli

---

## Faz 6 - Tool Output Truncation ve Artifact Yonetimi

### Is kalemleri
- Buyuk output'ta artifact path yazimi
- Prompt icin compact preview uretimi
- Metadata:
  - `truncated`
  - `artifact_path`
  - `byte_count`
  - `hash`
- Secret redaction pipeline

### Teslim kriterleri
- Truncation testleri
- Redaction testleri

---

## Faz 7 - Provider Runtime v2

### Is kalemleri
- Unified stream event modeli:
  - `text`
  - `reasoning`
  - `tool_start`
  - `tool_result`
  - `error`
- OpenAI adapter'in bu event modeline alinmasi
- Role -> model routing ve capability guard

### Teslim kriterleri
- Provider contract testleri
- Timeout, retry, partial failure testleri

---

## Faz 8 - Interactive Full Integration

### Is kalemleri
- `cmd/interactive.go` session runtime ile calisir
- Her prompt kalici user message
- Assistant yanitlari part-part stream edilir
- `--session` resume tam transcript ustunden

### Teslim kriterleri
- E2E:
  - yeni session
  - resume
  - compaction sonrasi devam

---

## Faz 9 - Run Pipeline Full Integration

### Is kalemleri
- Orchestrator safhalari `stage` parts olarak yazilir:
  - analyzing
  - planning
  - coding
  - validating
  - testing
  - reviewing
- Planner/Coder/Reviewer ciktilari assistant messages olarak kaydedilir
- Validation/test/review structured parts olarak yazilir
- `RunState` persistence kaldirilir; projection olarak uretilir

### Teslim kriterleri
- Pipeline regression testleri
- Replay/debug tutarlilik testleri

---

## Faz 10 - CLI ve Dokumantasyon Gecisi

### Is kalemleri
- Session odakli komutlarin genisletilmesi:
  - `orch session list`
  - `orch session current`
  - `orch session runs`
  - `orch session messages <id>` (yeni)
  - `orch session compact <id>` (yeni)
- JSON ciktilarinin standardizasyonu
- README ve docs guncellemesi

### Teslim kriterleri
- CLI snapshot testleri
- Komut davranisi dokumanla birebir

---

## Faz 11 - Hardening ve Release

### Is kalemleri
- Soak tests (uzun multi-turn)
- Failure drills:
  - provider timeout
  - DB lock contention
  - compaction failure
  - tool crash/interruption
- Performans hedefleri:
  - turn latency
  - compaction frequency
  - DB growth

### Teslim kriterleri
- `go test ./...` yesil
- SLO hedefleri raporlanmis
- Release checklist tamam

## 3) Degisiklik Etki Analizi

### Kaldirilacak / degisecek davranislar
- Ayrik runstore persistence tamamen kaldirilir
- Run gecmisi, session mesaj/part zincirinden uretilir
- Interactive tek-shot modelden cok-turlu transcript modeline gecer

### Avantajlar
- Tek veri kaynagi ile tutarlilik
- Replay ve debug kolayligi
- Uzun contextlerde stabil performans (compaction + truncation)

### Riskler ve mitigasyon
- DB buyumesi:
  - retention + prune + artifact offload
- Compaction kalite kaybi:
  - quality checks + fallback
- Provider uyumsuzluklari:
  - capability matrix + adapter tests

## 4) Uygulama Sirasi (Oncelik)

1. Faz 1-2 (session foundation + domain)
2. Faz 3-4 (assembler + token/overflow)
3. Faz 5-6 (compaction + truncation)
4. Faz 7-8 (provider v2 + interactive)
5. Faz 9-11 (run entegrasyonu + hardening + release)

## 5) Done Tanimi (Definition of Done)

- Session store tek source of truth olarak canli
- Interactive ve run pipeline ayni runtime uzerinden calisiyor
- Context overflow otomatik compaction ile yonetiliyor
- Buyuk tool output promptu sisirmiyor
- Tum test paketleri yesil, docs guncel, release checklist tamam
