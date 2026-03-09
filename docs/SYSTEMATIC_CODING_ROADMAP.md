# Orch Systematic Coding Roadmap

## Status

- Owner: Core Runtime / Product
- Priority: Critical
- Horizon: 6 months
- Goal Type: Product + Architecture + Execution Quality

---

## 1. Problem Statement

Piyasadaki çoğu AI coding agent şu şekilde çalışır:

- kullanıcı bir görev verir
- LLM kendi kafasına göre plan yapar
- dosya seçimi, değişiklik kapsamı, test yaklaşımı ve review kalitesi modele göre değişir
- sonuç kalitesi kullanılan modele, prompt'a ve o anki context'e aşırı bağımlı olur

Bu yaklaşımın temel sorunları:

1. **Tutarsız kalite:** Aynı görev farklı modellerde çok farklı sonuç verir.
2. **Düşük süreç disiplini:** Planlama, validation, test ve review genelde gevşektir.
3. **Yetersiz denetlenebilirlik:** Hangi kararın neden alındığı net değildir.
4. **Yüksek hata oranı:** LLM plan dışına çıkar, gereksiz dosya değiştirir, eksik test yazar veya hiç yazmaz.
5. **Model bağımlılığı:** Ürün kalitesi, orkestrasyon kalitesinden çok model kalitesine bağlı kalır.

Orch'ün çözmesi gereken gerçek problem şudur:

> LLM'leri serbest bırakan bir agent olmak değil, LLM'leri sistematik, denetlenebilir, düşük hata oranlı bir yazılım geliştirme sürecine zorlayan bir control plane olmak.

---

## 2. Product Thesis

### Core thesis

**Orch, kodu LLM'e bırakmaz; kod yazma sürecini standardize eder.**

### Product definition

Orch şu rolü üstlenmelidir:

- görevi normalize eden sistem
- yapılandırılmış planı üreten sistem
- planı enforce eden sistem
- validation kapılarını işleten sistem
- testleri seçen ve çalıştıran sistem
- review rubric'ini uygulayan sistem
- retry/fix loop'unu yöneten sistem
- güven skorunu hesaplayan sistem

LLM ise şu role indirgenmelidir:

- verilen execution contract içinde patch üreten worker
- belirli eksikleri kapatan yardımcı bileşen
- review açıklaması üreten ama review standardını belirlemeyen parça

### Positioning

> Orch = Control Plane for Deterministic AI Coding

> LLM = Replaceable Execution Engine

---

## 3. Vision

### Long-term vision

Kullanıcı bir görev verdiğinde Orch:

1. görevi standart bir iş tanımına çevirir
2. repo bağlamını seçer
3. yapılandırılmış plan üretir
4. acceptance criteria ve invariants belirler
5. LLM'e sadece bu contract içinde kod yazdırır
6. patch'i validate eder
7. testleri çalıştırır
8. review rubric'ine göre değerlendirir
9. gerekiyorsa bounded auto-fix loop uygular
10. sonucu bir güven skoru ile kullanıcıya sunar

### Target promise

Dış vaadimiz "sıfır hata garantisi" olmamalıdır. Daha doğru vaat:

- daha tutarlı kalite
- daha düşük hata oranı
- fail-closed davranış
- modelden bağımsız süreç kalitesi
- ölçülebilir güven

### Internal quality target

İç hedefimiz yine de şudur:

- sıfıra yakın hata
- minimum scope sapması
- minimum gereksiz değişiklik
- yüksek ilk sefer test geçiş oranı

---

## 4. Product Principles

1. **Plan before code**
2. **Structured plans only**
3. **Patch before apply**
4. **Every change maps to an acceptance criterion**
5. **No test, no confidence**
6. **No review pass, no completion**
7. **Model is swappable, process is fixed**
8. **Fail closed, not open**
9. **Small diffs beat clever diffs**
10. **Everything is logged**
11. **Unrelated edits are defects**
12. **Orch owns workflow quality, LLM does not**

---

## 5. Current State Assessment

Bu repo'nun mevcut durumuna göre güçlü taraflar:

- deterministik bir state machine iskeleti var
- `run -> analyze -> plan -> code -> validate -> test -> review` hattı var
- patch-first yaklaşım var
- apply aşaması güvenlik kontrollü
- retry limitleri var
- repo lock var
- session ve SQLite persistence var
- provider abstraction başlamış durumda
- interactive + CLI command yüzeyi mevcut

Mevcut zayıf/eksik alanlar:

1. **Planner/Coder/Reviewer hâlâ sığ:** Structured, enforce edilen contract yerine çoğunlukla basit prompt tabanlı davranış var.
2. **Plan Orch tarafından compile edilmiyor:** LLM planı daha baskın.
3. **Context selection yüzeysel:** Derin sembol/ilişki/etki analizi yok.
4. **Validation katmanları sınırlı:** Patch validation var, ama semantic ve plan-compliance validation zayıf.
5. **Review rubric tabanlı değil:** Review daha çok serbest yorum gibi.
6. **Model variance ölçülmüyor:** Aynı task'ın farklı modellerle kalite farkı bilinmiyor.
7. **Acceptance criteria zorunlu değil:** Değişikliklerin task hedeflerine birebir map edilmesi enforce edilmiyor.
8. **Confidence scoring yok:** Kullanıcıya güven seviyesi net verilmiyor.

Sonuç:

> Temel orkestrasyon altyapısı var. Eksik olan şey, Orch'ün bu altyapıyı gerçek bir kalite kontrol sistemine dönüştürmesi.

---

## 6. North Star Outcomes

Orch başarıyı yalnızca "patch üretildi" diye ölçmemelidir.

### Primary KPIs

1. **Task Success Rate**
   - Görev acceptance criteria'larının tamamını karşılayıp completion alan run oranı

2. **First-Pass Validation Rate**
   - İlk kod üretiminde validation kapılarından geçen run oranı

3. **First-Pass Test Pass Rate**
   - İlk patch'te testleri geçen run oranı

4. **Review Acceptance Rate**
   - Review loop'a girmeden kabul edilen run oranı

5. **Model Variance Score**
   - Aynı benchmark task için modeller arası başarı farkı
   - Hedef: zamanla düşmesi

6. **Unplanned File Touch Rate**
   - Plan dışı değiştirilen dosya oranı
   - Hedef: sıfıra yaklaşması

7. **Retry Exhaustion Rate**
   - Retry limitine rağmen başarılamayan run oranı

8. **Confidence Accuracy**
   - Yüksek confidence verilen run'ların gerçekten başarılı olma oranı

### Secondary KPIs

- Average run duration
- Patch size median
- Patch revert rate
- Post-apply defect rate
- Human intervention rate

---

## 7. Target System Architecture

### 7.1 Orch as Control Plane

Orch şu çekirdek katmanlardan oluşmalıdır:

1. **Task Normalizer**
2. **Plan Compiler**
3. **Context Selector**
4. **Execution Contract Builder**
5. **Constrained Coder Runtime**
6. **Validation Engine**
7. **Test Orchestrator**
8. **Review Engine**
9. **Fix Loop Manager**
10. **Confidence Scorer**
11. **Benchmark & Model Variance Harness**
12. **Audit & Run Intelligence Layer**

### 7.2 Required Contracts

LLM ile serbest text alışverişi değil, yapılandırılmış sözleşmeler kullanılmalıdır.

#### A. Task Brief

```json
{
  "task_id": "...",
  "user_request": "fix race condition in auth service",
  "normalized_goal": "remove concurrent state mutation bug in auth service without behavior regression",
  "constraints": [],
  "assumptions": [],
  "risk_level": "medium",
  "success_definition": []
}
```

#### B. Structured Plan

```json
{
  "summary": "...",
  "files_to_inspect": [],
  "files_to_modify": [],
  "steps": [],
  "invariants": [],
  "forbidden_changes": [],
  "test_requirements": [],
  "acceptance_criteria": [],
  "rollback_notes": []
}
```

#### C. Execution Contract

```json
{
  "allowed_files": [],
  "required_edits": [],
  "prohibited_actions": [],
  "max_patch_files": 5,
  "max_patch_lines": 200,
  "must_keep_invariants": [],
  "must_satisfy_acceptance_criteria": []
}
```

#### D. Review Scorecard

```json
{
  "requirement_coverage": 0,
  "scope_control": 0,
  "regression_risk": 0,
  "readability": 0,
  "maintainability": 0,
  "test_adequacy": 0,
  "decision": "accept|revise|reject",
  "findings": []
}
```

#### E. Run Manifest

```json
{
  "task_brief": {},
  "plan": {},
  "execution_contract": {},
  "validation_results": [],
  "test_results": [],
  "review_scorecard": {},
  "confidence": 0.0
}
```

---

## 8. Strategic Workstreams

Bu vizyonu hayata geçirmek için 8 paralel iş akışı gerekir.

### WS1 - Product Contracts and Schemas
- Task, plan, execution, review, confidence şemaları
- CLI/JSON çıktılarının standartlaşması

### WS2 - Deterministic Planning
- Orch-owned plan compiler
- acceptance criteria ve invariants üretimi

### WS3 - Constrained Coding
- LLM'in scope dışına çıkmasını engelleyen contract execution
- minimal diff enforcement

### WS4 - Quality Gates
- validation, semantic checks, plan compliance, patch hygiene

### WS5 - Test Intelligence
- test seçimi, test matrisi, failure classification, auto-fix loop

### WS6 - Review Intelligence
- rubric tabanlı review, decision scoring, explanation quality

### WS7 - Benchmarking and Model Agnosticism
- golden task suite
- model variance ölçümü
- provider karşılaştırması

### WS8 - Observability and Developer UX
- confidence display
- run manifest
- logs, metrics, explainability

---

## 9. Detailed Roadmap

# Phase 0 - Product Contract and Quality System Foundation

### Duration
Weeks 1-2

### Goal
Orch'ün kaliteyi modelden bağımsızlaştıracak çekirdek sözleşmelerini ve kalite sistemini tanımlamak.

### Why this phase matters
Bu faz yapılmadan sonraki tüm geliştirmeler prompt iyileştirmesi seviyesinde kalır. Önce sistem hangi standartları enforce edeceğini bilmeli.

### Scope

1. Product manifesto ve core principles dokümantasyonu
2. Task Brief schema tanımı
3. Structured Plan schema tanımı
4. Execution Contract schema tanımı
5. Review Scorecard schema tanımı
6. Confidence model v0 tanımı
7. Run Manifest tanımı
8. Error taxonomy standardizasyonu

### Deliverables

- `docs/SYSTEMATIC_CODING_ROADMAP.md`
- `docs/QUALITY_SYSTEM_SPEC.md`
- `docs/EXECUTION_CONTRACT_SPEC.md`
- `internal/contracts/` veya `internal/models/` altında yeni typed contract yapıları
- JSON serialization testleri

### Proposed implementation areas

- `internal/models/task_brief.go`
- `internal/models/plan_contract.go`
- `internal/models/execution_contract.go`
- `internal/models/review_scorecard.go`
- `internal/models/confidence.go`
- `internal/models/run_manifest.go`

### Acceptance Criteria

- Tüm agent input/output yapıları typed contract olarak tanımlı
- Planner/Coder/Reviewer ile Orch arasındaki veri akışı serbest text yerine typed yapıya geçmeye hazır
- Her contract için schema/unit tests mevcut
- Run state içinde bu contract'ları tutacak alanlar tanımlı

### KPIs unlocked

- plan completeness
- acceptance coverage
- execution scope compliance

### Risks

- Fazla şema tasarımı yapıp implementasyonu geciktirmek
- Çözüm yerine soyut doküman üretmek

### Risk mitigation

- Her schema için minimum viable alan seti ile başla
- Önce v0, sonra iteratif genişletme

---

# Phase 1 - Orch-Owned Planning Engine

### Duration
Weeks 3-5

### Goal
Planın sahipliğini LLM'den Orch'e taşımak.

### Core idea
LLM planı belirlemesin; Orch planı compile etsin. LLM sadece planın bazı boşluklarını doldursun.

### Scope

1. Task normalization pipeline
2. Repo analysis iyileştirmesi
3. File targeting heuristic engine
4. Structured plan compiler
5. Acceptance criteria generator
6. Invariant generator
7. Forbidden change generator
8. `orch plan --json` çıktısı

### Detailed tasks

#### P1.1 Task Normalizer
- kullanıcı isteğini normalize et
- task type sınıflandır: feature, bugfix, test, refactor, docs, chore
- risk level hesapla
- belirsizlik alanlarını işaretle

#### P1.2 Repo-aware planning hints
- repo language/framework/package manager sinyallerini kullan
- test framework ve mevcut klasör yapısına göre öneri üret
- hot paths ve likely target files heuristics ekle

#### P1.3 Plan Compiler
- plan steps'i Orch oluştursun
- files_to_inspect ve files_to_modify önerilerini deterministic heuristics ile çıkar
- acceptance criteria üret
- invariants üret

#### P1.4 Planner role redesign
- LLM planner artık plan owner değil
- sadece plan refinement worker olsun
- Orch-produced plan draft'ını iyileştirsin, ama final shape Orch'e ait olsun

### Deliverables

- `internal/planning/normalizer.go`
- `internal/planning/compiler.go`
- `internal/planning/heuristics.go`
- `internal/planning/acceptance.go`
- `internal/planning/invariants.go`
- `cmd/plan.go` JSON mode

### Acceptance Criteria

- Her task için structured plan üretiliyor
- Plan şu alanları zorunlu içeriyor:
  - summary
  - files_to_inspect
  - files_to_modify
  - steps
  - acceptance_criteria
  - invariants
  - test_requirements
- Plan üretimi provider kapalı olsa bile deterministic fallback ile çalışıyor
- Plan dışı dosya değişikliğini sonradan tespit etmeye yetecek allowed file listesi oluşuyor

### Metrics

- plan completeness score
- file targeting precision
- acceptance criteria coverage

### Risks

- Heuristics'in fazla zayıf kalması
- Büyük repolarda noise üretmesi

### Risk mitigation

- önce Go/TS ağırlıklı repos için optimize et
- golden task set ile ölç

---

# Phase 2 - Constrained Coding Engine

### Duration
Weeks 6-8

### Goal
LLM'e serbest kod yazdırmak yerine execution contract içinde çalıştırmak.

### Core idea
Coder'a "çöz" demek yerine "şu planın şu kısmını, şu dosya sınırları içinde, şu acceptance criteria'lara göre uygula" denmeli.

### Scope

1. Execution Contract builder
2. Allowed file scope enforcement
3. Required change checklist
4. Minimal diff policy
5. Plan compliance tracking
6. Structured coder output
7. Patch parsing robustness

### Detailed tasks

#### P2.1 Execution Contract Builder
- plan + context + policy'den contract üret
- allowed files set'i çıkar
- prohibited actions listesi oluştur
- max patch size contract'a girsin

#### P2.2 Coder I/O redesign
- coder input: task brief + plan + contract + selected context
- coder output:
  - unified diff
  - change summary
  - criterion mapping
  - assumptions

#### P2.3 Scope Guard
- patch içindeki dosyalar allowed list dışında ise fail
- unrelated edit heuristics ekle
- büyük diff alarmı ekle

#### P2.4 Minimal Change Enforcer
- no opportunistic refactor
- no rename unless required
- no formatting-only churn unless required

### Deliverables

- `internal/execution/contract_builder.go`
- `internal/execution/scope_guard.go`
- `internal/execution/minimal_diff.go`
- `internal/agents/coder.go` redesign
- `internal/patch/parser.go` iyileştirmeleri

### Acceptance Criteria

- Coder her run'da execution contract ile çalışıyor
- Allowed file list dışındaki değişiklikler otomatik reddediliyor
- Patch summary acceptance criteria ile eşleniyor
- Patch boş ise neden boş olduğu structured şekilde dönüyor

### Metrics

- unplanned file touch rate
- patch scope violation rate
- unrelated edit rate

### Risks

- Fazla sıkı kural seti nedeniyle task completion düşebilir
- Bazı görevler gerçekten plan dışı dosya gerektirebilir

### Risk mitigation

- controlled scope expansion mekanizması ekle
- scope expansion ayrı log entry olarak kaydedilsin

---

# Phase 3 - Validation Engine Expansion

### Duration
Weeks 9-11

### Goal
Validation'ı patch doğrulamasından çıkarıp gerçek kalite kapıları sistemine dönüştürmek.

### Scope

1. Patch hygiene gates
2. Plan compliance validation
3. Syntax / parse validation
4. Compile/build validation
5. Static analysis validation
6. Sensitive file / secrets validation
7. Dependency mutation validation
8. Risk-aware validation profiles

### Detailed tasks

#### P3.1 Patch Hygiene
- binary/sensitive file blocking geliştirilir
- patch size budget stricter hale getirilir
- generated files policy tanımlanır

#### P3.2 Plan Compliance Validator
- acceptance criteria mapping kontrol edilir
- required files gerçekten değişmiş mi bakılır
- forbidden changes ihlali denetlenir

#### P3.3 Language Validators
- Go için: parse/build/test paket bazlı gates
- TS/JS için: typecheck/lint/build gates
- dil bazlı adapter sistemi hazırlanır

#### P3.4 Static Safety Gates
- lint
- type errors
- import cycles
- dead reference / unresolved symbol tespiti

### Deliverables

- `internal/quality/validator.go`
- `internal/quality/gates/patch_hygiene.go`
- `internal/quality/gates/plan_compliance.go`
- `internal/quality/gates/build.go`
- `internal/quality/gates/static_analysis.go`
- `internal/quality/profile.go`

### Acceptance Criteria

- Validation sonucu tek string değil, structured gate listesi olarak dönüyor
- Hangi gate fail etti net görülebiliyor
- Retry loop, gate sonuçlarını kullanarak targeted fix prompt üretebiliyor
- Dil bazlı en az 2 profile (Go ve JS/TS) çalışıyor

### Metrics

- first-pass validation rate
- validation gate failure taxonomy
- false-positive validation rate

### Risks

- Validation süresinin uzaması
- Cross-language destek karmaşıklığı

### Risk mitigation

- targeted validation + full validation ayrımı
- timeout ve output truncation standardı

---

# Phase 4 - Test Intelligence and Auto-Fix Loop

### Duration
Weeks 12-14

### Goal
Testleri sadece komut koşturan katman olmaktan çıkarıp planla bağlantılı bir güven kapısına dönüştürmek.

### Scope

1. Test requirement planner integration
2. Targeted test selection
3. Full suite escalation rules
4. Failure classification
5. Auto-fix retry contracts
6. Retry memory and anti-loop safeguards

### Detailed tasks

#### P4.1 Test Matrix
- plan içinde required test matrix alanı tanımla
- unit/integration/e2e/smoke sınıflandırması yap

#### P4.2 Test Selector
- değişen dosyalara göre ilgili testleri seç
- yüksek riskte full suite escalate et

#### P4.3 Failure Classifier
- syntax failure
- compile failure
- assertion failure
- timeout
- flaky suspicion
- missing test coverage

#### P4.4 Bounded Fix Loop
- retry prompt'u serbest olmayacak
- sadece failed gates + failed tests + violated criteria ile dönecek
- önceki başarısız denemelerin özeti verilecek

### Deliverables

- `internal/testing/matrix.go`
- `internal/testing/selector.go`
- `internal/testing/classifier.go`
- `internal/orchestrator/fix_loop.go`
- retry state genişletmesi

### Acceptance Criteria

- Test selection structured şekilde kaydediliyor
- Test failure nedeni kategorize ediliyor
- Retry loop aynı hatayı sonsuz tekrar etmiyor
- Retry prompt'ları gate bazlı ve deterministic

### Metrics

- first-pass test pass rate
- retry exhaustion rate
- average retries per successful run

### Risks

- Flaky testler yanlış sinyal verebilir
- Büyük reposlarda test cost artabilir

### Risk mitigation

- flaky suspicion state'i ekle
- targeted + full fallback modları ekle

---

# Phase 5 - Review Engine and Confidence Scoring

### Duration
Weeks 15-17

### Goal
Review'ı serbest yorum olmaktan çıkarıp rubric tabanlı karar motoruna dönüştürmek.

### Scope

1. Review rubric design
2. Structured review scorecard
3. Independent second-pass review option
4. Human-readable findings
5. Confidence scoring v1
6. Completion thresholds

### Detailed tasks

#### P5.1 Review Rubric
Aşağıdaki eksenlerde skor üret:
- requirement coverage
- scope control
- regression risk
- readability
- maintainability
- test adequacy

#### P5.2 Decision Policy
- `accept`
- `revise`
- `reject`
- confidence threshold'a bağlı completion gating

#### P5.3 Confidence Scorer
Skor girdileri:
- validation gate pass ratio
- test pass quality
- review rubric score
- scope compliance
- retry count
- patch size/risk level

#### P5.4 Explainability
Kullanıcıya şu soruların cevabı net verilmeli:
- neden accept?
- neden revise?
- confidence neden şu kadar?
- risk nerede?

### Deliverables

- `internal/review/rubric.go`
- `internal/review/engine.go`
- `internal/review/decision_policy.go`
- `internal/confidence/scorer.go`
- CLI summary güncellemeleri

### Acceptance Criteria

- Review sonucu scorecard dönüyor
- Revise kararının gerekçesi madde madde veriliyor
- Confidence 0-1 veya 0-100 arası numerik gösteriliyor
- Düşük confidence run'lar completed olmadan önce uyarılıyor veya revise ediliyor

### Metrics

- review acceptance rate
- confidence calibration accuracy
- human override rate

### Risks

- Confidence yanlış kalibre olabilir
- Review rubric aşırı katı olabilir

### Risk mitigation

- önce shadow scoring ile ölç
- manuel benchmark ile kalibre et

---

# Phase 6 - Model-Agnostic Quality Layer

### Duration
Weeks 18-21

### Goal
Orch'ün kaliteyi gerçekten modelden bağımsızlaştırdığını ölçmek ve iyileştirmek.

### Scope

1. Golden benchmark task suite
2. Multi-model replay harness
3. Model variance dashboard/report
4. Provider comparison runner
5. Fallback routing policy
6. Regression benchmark CI

### Detailed tasks

#### P6.1 Golden Task Suite
Görev kategorileri:
- küçük bug fix
- orta seviye feature
- test yazma
- refactor
- config change
- concurrency fix
- API contract change

Her görev için:
- expected scope
- expected files
- required tests
- acceptance criteria
- scoring rubric

#### P6.2 Replay Harness
Aynı task'ı farklı modellerle çalıştır:
- provider A
- provider B
- same provider different model

#### P6.3 Variance Reporting
Ölçümler:
- success rate difference
- patch size difference
- retry count difference
- review score difference
- confidence stability

#### P6.4 Adaptive Routing
- basit task -> daha ucuz model
- kompleks task -> daha güçlü model
- ama aynı process contract korunur

### Deliverables

- `bench/tasks/*.json`
- `internal/bench/replay.go`
- `internal/bench/scoring.go`
- `cmd/stats.go` veya `cmd/bench.go`
- CI benchmark workflow

### Acceptance Criteria

- En az 20 golden task ile benchmark suite var
- Aynı task farklı modellerde replay edilebiliyor
- Model variance raporu üretilebiliyor
- Orch improvements sonrası variance trendi düşüyor

### Metrics

- model variance score
- benchmark success rate
- cost-to-success ratio

### Risks

- Benchmark set gerçek dünyayı temsil etmeyebilir
- Model API maliyeti artabilir

### Risk mitigation

- küçük ama temsili benchmark set ile başla
- nightly yerine controlled benchmark cadence kullan

---

# Phase 7 - Developer UX, Auditability, and Release Hardening

### Duration
Weeks 22-24

### Goal
Ürünü sadece teknik olarak doğru değil, geliştirici için güven verici ve açıklanabilir hale getirmek.

### Scope

1. Run manifest görünürlüğü
2. CLI summary redesign
3. `orch stats`
4. `orch explain <run-id>`
5. session-level insights
6. release checklist ve regression suite

### Detailed tasks

#### P7.1 Run Summary UX
Gösterilecekler:
- task brief summary
- selected files
- changed files
- validation gate results
- executed tests
- review scorecard
- confidence
- unresolved risks

#### P7.2 Explain Command
Örnek:
- neden şu dosya seçildi?
- neden revise verildi?
- neden confidence düşük?

#### P7.3 Release Hardening
- smoke suite
- benchmark gate
- docs completeness
- regression suite

### Deliverables

- `cmd/stats.go`
- `cmd/explain.go`
- enhanced interactive summaries
- release checklist doc

### Acceptance Criteria

- Kullanıcı bir run'ın neden başarılı/başarısız olduğunu kolayca anlayabiliyor
- Confidence ve risk görünür
- Release öncesi benchmark + smoke gates koşuyor

### Metrics

- developer trust score (survey/manual)
- explanation usefulness rate
- support/debug time per failed run

---

## 10. Immediate 30-Day Implementation Plan

Bu vizyona göre ilk 30 günde yapılması gerekenler:

### Week 1
- Bu roadmap'i ürün kararı olarak kabul et
- Quality System Spec çıkar
- Contract yapıları tasarla
- Run state genişletme planı oluştur

### Week 2
- Task Brief ve Structured Plan schema implement et
- `orch plan --json` ekle
- acceptance criteria ve invariant generator v0 çıkar

### Week 3
- Execution Contract builder yaz
- coder input/output redesign başlat
- allowed file enforcement v0 ekle

### Week 4
- plan compliance validator v0
- structured validation result formatı
- retry/fix loop için failure contract oluştur

### 30-day definition of done

- Orch structured plan üretiyor
- acceptance criteria tanımlıyor
- execution contract oluşturuyor
- coder scope dışı değişiklikte fail oluyor
- validation sonucu structured gate listesi dönüyor

Bu noktadan sonra ürün gerçekten "systematic coding engine" yoluna girmiş olur.

---

## 11. Proposed Repository Impact

### New packages/modules

- `internal/planning/`
- `internal/execution/`
- `internal/quality/`
- `internal/testing/`
- `internal/review/`
- `internal/confidence/`
- `internal/bench/`

### Existing modules to evolve

- `internal/orchestrator/`
- `internal/agents/`
- `internal/repo/`
- `internal/patch/`
- `internal/tools/`
- `internal/models/`
- `cmd/plan.go`
- `cmd/run.go`
- `cmd/logs.go`
- `cmd/interactive.go`

### Storage changes

SQLite ve JSON run state içinde şu ek alanlar düşünülmeli:

- task_brief_json
- execution_contract_json
- validation_results_json
- review_scorecard_json
- confidence_score
- benchmark_tag
- model_metadata_json

---

## 12. Testing Strategy for the Roadmap

### Unit tests
- schema validation
- planner compiler heuristics
- scope guard
- confidence scorer
- review rubric scoring

### Integration tests
- plan -> code -> validate -> test -> review tam akış
- scope violation fail case
- retry loop determinism
- session-aware run integrity

### Benchmark tests
- golden task suite replay
- model variance comparison
- regression trend tracking

### Safety tests
- sensitive file blocking
- destructive apply enforcement
- plan mode read-only enforcement
- stale lock recovery

---

## 13. Rollout Strategy

### Step 1 - Shadow mode
Yeni quality gates önce sadece log üretir, karar vermez.

### Step 2 - Soft enforcement
Bazı gates warning üretir, bazıları fail etmez.

### Step 3 - Hard enforcement
Plan compliance, scope control, sensitive file policies fail-closed olur.

### Step 4 - Benchmark gate
Release öncesi benchmark success threshold zorunlu hale gelir.

---

## 14. Definition of Success

Bu roadmap başarılı sayılacaktır eğer:

1. Orch planı gerçekten sahiplenirse
2. LLM execution contract içinde tutulursa
3. validation/test/review structured kalite kapıları haline gelirse
4. aynı task farklı modellerde daha az kalite sapması gösterirse
5. kullanıcı run sonucuna güven seviyesini anlayarak karar verebilirse

### Final success statement

> Orch başarılı olduğunda, kullanıcı yalnızca "AI kod yazdı" demez.
> Kullanıcı şunu der:
> "Orch bu görevi kontrollü şekilde planladı, doğruladı, test etti, review etti ve güvenilir bir patch üretti."

---

## 15. Final Product Statement

Orch'ün nihai ürünü şu olmalıdır:

> Rastgele davranan coding agent'ları disipline eden, modeli değil süreci optimize eden, planlı-testli-validate edilen, düşük hata oranlı AI coding runtime.

Kısa versiyon:

> **Orch is not another coding agent. Orch is the system that makes coding agents reliable.**
