# Orch Implementation Progress

## Purpose

Bu doküman şu ana kadar yapılan işleri, tamamlanan temel mimari adımları ve sıradaki işleri unutmamak için tutulur.

Son güncelleme kapsamı:
- Sprint 1 foundation
- Sprint 2 execution contract + scope control
- plan compliance gate
- bounded retry directive groundwork
- review rubric engine
- confidence scoring v1
- confidence enforcement policy
- test failure classifier v0
- explain command v1
- stats command v1

---

## 1. Product Direction Reminder

Orch'ün hedefi artık net:

> Orch başka bir serbest agent olmayacak.
> Orch, LLM'leri planlı, doğrulanabilir, testli ve review edilen bir coding workflow'una zorlayan control plane olacak.

Ana ürün hedefleri:
- planı Orch sahiplenmeli
- validation'ı Orch sahiplenmeli
- test'i Orch sahiplenmeli
- review'ı Orch sahiplenmeli
- LLM sadece bounded worker olmalı
- model değişse de kalite standardı mümkün olduğunca korunmalı

---

## 2. Şu Ana Kadar Yapılanlar

### 2.1 Dokümantasyon ve strateji
Hazırlanan dokümanlar:
- `docs/SYSTEMATIC_CODING_ROADMAP.md`
- `docs/IMPLEMENTATION_TASK_LIST.md`
- `docs/QUALITY_SYSTEM_SPEC.md`
- `docs/EXECUTION_CONTRACT_SPEC.md`
- `docs/PLANNING_ENGINE_SPEC.md`
- `docs/SPRINT_BOARD_90_DAY_PLAN.md`

Bu dokümanlarla birlikte:
- ürün vizyonu netleşti
- 90 günlük sprint board yeni vizyona göre hizalandı
- execution backlog çıkarıldı
- contracts/planning/execution/quality tasarımı ayrıştırıldı

---

### 2.2 Structured data model foundation
`internal/models/models.go` içinde eklendi/geliştirildi:

#### Yeni yapılar
- `TaskBrief`
- genişletilmiş `Plan`
- `ExecutionContract`
- `ValidationResult`
- `RetryDirective`
- `ReviewScorecard`

#### `RunState` genişletmeleri
- `TaskBrief`
- `ExecutionContract`
- `ValidationResults`
- `RetryDirective`
- `ReviewScorecard`

Bunlar sayesinde Orch artık serbest metin akışından çıkıp structured artifact üretmeye başladı.

---

### 2.3 Task normalization ve Orch-owned planning başlangıcı
Eklenen dosya:
- `internal/planning/normalizer.go`

Yapılanlar:
- task type classification v0
  - feature
  - bugfix
  - test
  - refactor
  - docs
  - chore
- risk level derivation v0
- normalized goal üretimi
- acceptance criteria generator v0
- test requirement generator v0
- invariant / forbidden change generator v0
- deterministic file ranking v0
- structured plan compile etme altyapısı

Sonuç:
- plan sadece LLM'den gelmiyor
- Orch artık kendi task brief + plan taslağını oluşturuyor

---

### 2.4 Planner redesign başlangıcı
Güncellenen dosya:
- `internal/agents/planner.go`

Yeni yaklaşım:
- Orch taban planı oluşturuyor
- planner LLM varsa yalnızca refinement yapıyor
- final plan shape artık daha çok Orch tarafında tanımlanıyor

Bu, "LLM workflow owner olmasın" hedefinin ilk gerçek adımı.

---

### 2.5 `orch plan --json`
Güncellenen dosya:
- `cmd/plan.go`

Eklendi:
- `--json` seçeneği

Artık `orch plan`:
- human-readable çıktı verebiliyor
- machine-readable structured JSON da dönebiliyor

JSON çıktı şu ana parçaları içeriyor:
- `task`
- `task_brief`
- `plan`

---

### 2.6 Execution contract builder
Eklenen dosya:
- `internal/execution/contract_builder.go`

Builder şu alanları üretiyor:
- `allowed_files`
- `inspect_files`
- `required_edits`
- `prohibited_actions`
- `acceptance_criteria`
- `invariants`
- `patch_budget`
- `scope_expansion_policy`

Kaynakları:
- task
- task brief
- structured plan
- context
- patch config

Sonuç:
- coder artık teorik olarak serbest değil
- bounded contract içinde çalıştırılabilir hale geldi

---

### 2.7 Scope guard
Eklenen dosya:
- `internal/execution/scope_guard.go`

Yaptığı iş:
- patch içindeki değişen dosyaları `ExecutionContract.AllowedFiles` ile karşılaştırıyor
- scope dışı dosya değişikliğini fail ediyor
- structured `ValidationResult` dönüyor

Yeni gate:
- `scope_compliance`

---

### 2.8 Plan compliance gate
Eklenen dosya:
- `internal/execution/plan_compliance.go`

Yaptığı iş:
- structured plan'de acceptance criteria var mı kontrol ediyor
- patch boş mu kontrol ediyor
- planın zorunlu modify dosyaları gerçekten değişmiş mi kontrol ediyor
- forbidden change kurallarına göre config/test benzeri ihlalleri yakalıyor

Yeni gate:
- `plan_compliance`

Bu, validation'ı gerçek quality gate sistemine dönüştürme yönünde önemli bir adım.

---

### 2.9 Bounded retry directive groundwork
Eklenen dosya:
- `internal/execution/retry_directive.go`

Yeni yapı:
- `RetryDirective`

Kaynaklara göre üretiliyor:
- validation failure
- test failure
- review revise

İçeriği:
- stage
- attempt
- reasons
- failed gates
- failed tests
- instructions
- avoid listesi

Sonuç:
- retry artık "yeniden dene" değil
- "şu gate fail oldu, şu hataları tekrarlama, şu düzeltmeleri yap" formatına geçti

---

### 2.10 Coder prompt’unun constrained hale gelmesi
Güncellenen dosya:
- `internal/agents/coder.go`

Artık coder prompt'u şunları görebiliyor:
- task
- task brief
- normalized goal
- task type
- risk level
- plan summary
- acceptance criteria
- allowed files
- inspect files
- required edits
- prohibited actions
- patch budget
- related tests
- retry directive varsa onun detayları

Bu, Orch'ün coder üzerinde kural koymaya başlaması açısından kritik.

---

### 2.11 Orchestrator entegrasyonları
Güncellenen dosya:
- `internal/orchestrator/orchestrator.go`

Eklenen/geliştirilen davranışlar:
- plan artifact compile etme
- task brief state'e yazma
- execution contract build etme
- patch parse etme ve `Patch.Files` doldurma
- validation sırasında structured gate sonuçları üretme
- retry öncesi `RetryDirective` oluşturma
- retry contract bilgilerini loglama

Validation tarafında şu gate'ler oluşmaya başladı:
- `patch_parse_valid`
- `patch_hygiene`
- `scope_compliance`
- `plan_compliance`

---

### 2.12 Persistence genişletmeleri
Güncellenen dosya:
- `internal/storage/storage.go`

SQLite tarafında eklendi:
- `task_brief_json`
- `execution_contract_json`
- `validation_results_json`
- `retry_directive_json`
- `review_scorecard_json`

Ayrıca:
- eski DB'ler için `ensureRunColumns()` ile kolon tamamlama yolu eklendi

Sonuç:
- structured artifact'lar artık persistence'a doğru taşınıyor

---

### 2.13 Review rubric engine
Eklenen dosyalar:
- `internal/review/engine.go`
- `internal/review/engine_test.go`

Yapılanlar:
- Orch-owned deterministic review rubric eklendi
- şu eksenlerde skor üretmeye başladı:
  - requirement coverage
  - scope control
  - regression risk
  - readability
  - maintainability
  - test adequacy
- review sonucu artık sadece provider yorumuna bırakılmıyor
- provider reviewer varsa yardımcı sinyal olarak kullanılıyor
- `ReviewScorecard` ve final `ReviewResult` Orch tarafından üretiliyor

Sonuç:
- review sahipliği Orch tarafına taşınmaya başladı
- review retry loop'u için structured finding üretme zemini oluştu

---

### 2.14 Confidence scoring v1
Eklenen dosyalar:
- `internal/confidence/scorer.go`
- `internal/confidence/scorer_test.go`

Yapılanlar:
- validation, test, review, retry ve plan sinyallerinden confidence score üretilmeye başlandı
- confidence raporu şu alanları içeriyor:
  - `score`
  - `band`
  - `reasons`
  - `warnings`
- confidence artık run state'e yazılıyor
- CLI summary tarafında görünür hale gelmeye başladı
- SQLite persistence tarafına `confidence_json` eklendi

Sonuç:
- kullanıcı artık sadece pass/fail değil, güven seviyesi de görebilecek
- review sonrası kalite sinyalleri tek bir confidence katmanında toplanmaya başladı

---

### 2.15 Confidence enforcement policy
Eklenen dosyalar:
- `docs/CONFIDENCE_ENFORCEMENT_POLICY.md`
- `internal/confidence/policy.go`
- `internal/confidence/policy_test.go`

Yapılanlar:
- confidence artık sadece görüntülenen skor değil, completion policy sinyali haline gelmeye başladı
- review aşamasına şu gate'ler eklendi:
  - `review_scorecard_valid`
  - `review_decision_threshold_met`
- default policy:
  - `score >= 0.70` -> geçebilir
  - `0.50 <= score < 0.70` -> revise
  - `score < 0.50` -> hard fail
- config tarafına confidence policy threshold'ları eklendi
- confidence enforcement feature flag eklendi

Sonuç:
- Orch artık düşük güvenli run'ları tamamlanmış saymamak için mekanizma kazandı
- confidence, gerçek karar katmanına dönüşmeye başladı

---

### 2.16 Test failure classifier v0
Eklenen dosyalar:
- `internal/testing/classifier.go`
- `internal/testing/classifier_test.go`
- `internal/orchestrator/test_classifier_test.go`

Yapılanlar:
- test failure sınıflandırması eklendi
- test stage gate'leri structured hale geldi:
  - `required_tests_executed`
  - `required_tests_passed`
- test retry directive artık failure code'lara göre daha hedefli instruction üretiyor
- `TestFailures` artık run state ve persistence içine yazılıyor

Sonuç:
- test katmanı da deterministic quality signal üretmeye başladı
- retry loop test tarafında daha anlamlı hale geldi

---

### 2.17 Explain command v1
Eklenen dosyalar:
- `cmd/explain.go`
- `cmd/explain_test.go`
- `docs/EXPLAIN_COMMAND_SPEC.md`

Yapılanlar:
- `orch explain [run-id]` komutu eklendi
- run state içindeki structured artifact'ler okunabilir terminal özetine dönüştürüldü
- latest run fallback desteği eklendi
- interactive shell içine `/explain` eklendi

Sonuç:
- kullanıcı artık bir run'ın neden geçtiğini, neden revise olduğunu veya neden fail ettiğini görebiliyor
- Orch için explainability yüzeyi oluşmaya başladı

---

### 2.18 Stats command v1
Eklenen dosyalar:
- `cmd/stats.go`
- `cmd/stats_test.go`
- `docs/STATS_COMMAND_SPEC.md`
- `internal/runstore/store_test.go`

Yapılanlar:
- `orch stats` komutu eklendi
- `.orch/runs/*.state` dosyaları okunup sıralanabiliyor
- son N run için şu sinyaller toplanıyor:
  - status breakdown
  - review accept / revise counts
  - average confidence
  - confidence band dağılımı
  - average retry count
  - test failure code dağılımı
- interactive shell içine `/stats` eklendi

Sonuç:
- Orch artık tek run explainability yanında toplu kalite görünürlüğü de sunuyor
- control plane yaklaşımı telemetry tarafında da görünür hale gelmeye başladı

---

### 2.19 Test altyapısı güncellemeleri
Eklenen testler:
- `internal/planning/normalizer_test.go`
- `internal/models/contracts_test.go`
- `internal/execution/contract_builder_test.go`
- `internal/execution/plan_compliance_test.go`
- `internal/review/engine_test.go`
- `internal/confidence/scorer_test.go`
- `internal/orchestrator/review_engine_test.go`

Güncellenen testler:
- `internal/storage/storage_test.go`
- `internal/orchestrator/orchestrator_retry_test.go`

Doğrulanan alanlar:
- task normalization v0
- plan compile v0
- JSON roundtrip for structured artifacts
- execution contract build
- scope violation fail
- plan compliance fail örnekleri
- deterministic review scorecard üretimi
- confidence scoring davranışı
- run state içinde task brief / execution contract / validation results / review scorecard / confidence varlığı

---

## 3. Şu Anda Tam Olarak Nerede Kaldık?

### Tamamlanan ana başlıklar
- [x] Contract foundation başladı
- [x] Task brief modeli var
- [x] Structured plan alanları genişletildi
- [x] `orch plan --json` var
- [x] Task normalizer v0 var
- [x] Acceptance criteria generator v0 var
- [x] Execution contract builder v0 var
- [x] Scope guard var
- [x] Plan compliance gate v0 var
- [x] Retry directive groundwork var
- [x] Review rubric engine v0 var
- [x] Review scorecard deterministic şekilde hesaplanmaya başladı
- [x] Confidence scoring v1 var
- [x] Confidence enforcement policy v1 var
- [x] Test failure classifier v0 var
- [x] Test gate sonuçları structured hale gelmeye başladı
- [x] Persistence genişletmesi başladı

### Henüz eksik / sıradaki ana başlıklar
- [ ] Review / confidence threshold tuning
- [ ] Richer plan compliance / criterion-to-change mapping
- [ ] Scope expansion mekanizması
- [ ] benchmark / model variance suite
- [ ] targeted test selector / test matrix
- [ ] session-scoped and json telemetry views

---

## 4. Kaldığımız Teknik Sınır

Bugünkü durumda Orch şunları yapabiliyor:
- structured plan üretmeye başlayabiliyor
- coder'a execution contract verebiliyor
- patch scope'unu denetleyebiliyor
- plan compliance için ilk kuralları uygulayabiliyor
- retry loop'u structured hale getirmeye başlayabiliyor
- review rubric ile deterministic scorecard üretebiliyor
- review sonrası confidence score hesaplayabiliyor
- düşük confidence sonuçlarını revise/fail politikasına sokabiliyor
- test failure'larını sınıflandırabiliyor
- `orch explain` ile tek run gerekçelerini gösterebiliyor
- `orch stats` ile toplu kalite sinyallerini özetleyebiliyor

Ama henüz şunlar eksik:
- criterion-to-change mapping hâlâ sığ
- targeted test selection / matrix henüz yok
- benchmark katmanı eksik
- confidence threshold tuning daha erken aşamada
- telemetry henüz session/json bazında zengin değil

Yani kaldığımız yer:

> Orch artık planning + validation + test classification + review + confidence enforcement + explainability + telemetry ekseninde gerçek bir control plane olmaya başladı.
> Sıradaki adım, daha akıllı test intelligence ve benchmark/telemetry derinliğini güçlendirmek.

---

## 5. Sıradaki Adım

## Richer Telemetry + Benchmark + Test Intelligence

Bir sonraki implementasyon hedefi:
- `orch stats` için session-scoped ve JSON output eklemek
- confidence trend / retry hotspot gibi daha zengin telemetry üretmek
- benchmark / model variance suite oluşturmak
- targeted test selector / matrix katmanını eklemek

### Beklenen çıktı
- kalite görünürlüğü daha ölçülebilir hale gelecek
- farklı model/provider davranışları daha kolay kıyaslanacak
- test çalıştırma katmanı daha seçici ve akıllı hale gelecek

---

## 6. Ortam Notu

Artık `go` ortamda mevcut.

Bu güncelleme sonrası doğrulandı:
1. `gofmt -w ...`
2. `go test ./...`

Mevcut durumda tüm testler geçiyor.

Yapılacak her yeni adım sonrası bu iki komutla doğrulama yapılmalı.

---

## 7. Devam Komutu

Bu noktadan sonraki doğal komut:

> test failure classifier ve confidence enforcement implement et

Bu dosya özellikle kaldığımız yeri unutmamak için bırakıldı.
