# Social Protection Platform Deep Audit and Remediation Report

**Audit date:** 19 September 2026

**Repository:** `munisp/socialservices`

**Audit standard:** source and executable evidence, not design-document claims

## Executive conclusion

The repository now contains a buildable administration application, database-backed stakeholder onboarding, and a real runnable PyTorch/Ray model lifecycle. It is **not yet a production-certified national social-protection platform**. The distinction is important: this audit repaired code defects and false-success paths that could be fixed locally, but no source-code change can substitute for live Nigerian programme data, regulator-approved identity access, Mojaloop conformance, production credentials, multi-node failure tests, a privacy impact assessment, or accountable model approval.

The strongest result is the ML remediation. Fraud, credit-risk, and GraphSAGE models now have trained CPU weights, reproducible training loops, manifests, checksums, immutable registry entries, fine-tuning, a governed database export, Neo4j snapshot export, deterministic A/B assignment, persisted predictions and delayed outcomes, drift/performance evaluation, MLflow integration, and an executed three-trial Ray run. All artifacts remain marked **synthetic and not production validated**.

The platform has **12 distinct stakeholder classes**. Before remediation, the core user table exposed only `admin` and `user`, and onboarding was largely implied. The repository now has a policy catalogue for all 12, a first-class onboarding record and event trail, organization linkage, evidence gates, MFA/training/safeguarding/payment certification, controlled transitions, suspension, reactivation, and offboarding. This is a materially stronger backend workflow, but there is not yet a complete stakeholder-facing UI or live identity/organization verification for every class.

## Audit provenance and archive merge

The GitHub repository was cloned locally. The supplied `social-protection-platform-ENCRYPTION-OPENLANE(1).tar.gz` archive was extracted and imported as the substantive baseline. The later supplied `social-protection-platform-FINAL-COMPLETE.tar.gz` archive was also extracted and compared path-by-path. Its unique/changed content was evaluated against the already-audited tree; older or weaker versions were not allowed to overwrite durability, ML, or security fixes. The merge decision and archive checksum are recorded in `ARCHIVE_MERGE_NOTES.md`.

The Google Drive URL itself was not accepted as a verified source archive because it returned a Drive confirmation/interstitial rather than directly verifiable archive bytes. The subsequently attached FINAL-COMPLETE archive supplied the second archive content used for the merge.

## What was fixed

| Area | Before | Remediation completed |
|---|---|---|
| Type safety/build | Dozens of TypeScript errors, invalid Zod v4 calls, stale compiler cache, array/page contract mismatches | Strict `pnpm check` passes; beneficiary, PMT, interoperability, middleware, offline-sync, and Zod contracts repaired |
| Beneficiary UI | Detail page assumed non-existent balances, cards, transactions, and mutations | Page now renders only real router fields, programme enrollments, and real approve/reject/suspend mutations |
| Stakeholder lifecycle | Generic roles; no first-class all-party onboarding | Twelve policies, persistent onboarding and events, evidence gates, MFA/training/certification, suspension and offboarding added |
| Fraud ML | Rule-based scorer, fixed claims, no governed feedback | Real PyTorch inference, shipped weights, exact version, prediction log, delayed outcomes, deterministic experiments |
| Training | Full-batch synthetic-only script | Temporal split, minibatches, class weighting, AdamW, early stopping, clipping, calibration metric, threshold selection, hashes, parent fine-tuning |
| Distributed compute | Ray service trained an in-memory random forest and returned random batch scores | Ray now runs real PyTorch trials against mounted data, selects by measured validation F1, persists the selected artifact, and performs artifact-based batch inference |
| GNN | Model class only; no weights or training | GraphSAGE training loop, shipped weights and graph snapshot, CPU inference, Neo4j export and snapshot fine-tuning implemented |
| Credit model | No shipped credit weights | Real PyTorch credit-risk model, dataset generator, weights, preprocessing, metrics, and a prohibited-use declaration added |
| Registry | Manifest directory only | Immutable version registry, per-file SHA-256, stage pointers, approval/reason requirement, collision detection, and rollback resolution added |
| MLflow | Code claim without deployed backend | MLflow logger and compose stack with PostgreSQL metadata backend and persistent artifact volume added |
| A/B testing | Simulated counts/results | Sticky SHA-256 assignment, SQLite predictions, delayed outcomes, and computed per-variant precision/recall/F1 added |
| Monitoring | PSI helper only | Persisted prediction/outcome evaluation, feature PSI, delayed-label performance, alert list, and insufficient-label state added |
| Production data path | No DB-to-training pipeline | Governed MySQL export of resolved fraud investigations, watermark/label/privacy metadata checks, hashes, and explicit missing-feature disclosure added |
| Lakehouse durability | Kafka offsets were acknowledged before Parquet durability | Messages are retained with the batch and committed only after all partition writes succeed |
| Event replay | Generated 1,000 fake successful events | Replay requires Kafka and a bounded offset range, consumes real records, logs real payloads, commits offsets, and fails honestly |
| Audit API | Entire audit viewer returned fabricated people, IPs, counts, and random time series | Database-backed search, detail, entity trail, user activity, deterministic export/checksum, dashboard, and statistics added |
| Orphan code | Duplicate router keys, four unused mock routers, unmounted placeholder GraphQL implementation | Duplicate keys removed; mock router files and unused GraphQL surface/dependencies removed |
| Incomplete journeys | Thirty workflows registered tracing-only/no-op activities as if successful | Incomplete journeys and their activities are disabled by default and require an explicit non-production opt-in |
| Kafka | Consumer failures were swallowed; producer absence silently skipped | Retry/DLQ path wired; processing errors propagate; required production publishing fails closed |
| Integration health | Kafka “health” used HTTP against a binary broker port | KafkaJS admin connectivity and topic listing now determine health |
| Search/authorization | OpenSearch insecure defaults; Permify could silently skip | Production credentials/TLS are enforced for OpenSearch; Permify schema/relationship writes fail closed in production |
| Fluvio | Configured failures were swallowed | `FLUVIO_REQUIRED=true` makes production delivery fail closed |

## Stakeholders: count and onboarding robustness

The defensible count is **12 stakeholder classes**, not two. The `admin`/`user` authentication roles remain coarse platform roles; the onboarding policy and scope record now carries the business stakeholder identity.

| # | Stakeholder | Implemented onboarding gates | Source-level robustness | Remaining live proof |
|---:|---|---|---:|---|
| 1 | Beneficiary/recipient | Identity evidence, consent/legal basis, region/programme scope, activation, suspension/offboarding | 4/5 | NIN/provider conformance, deduplication, assisted/offline UI, accessibility |
| 2 | Caregiver/household representative | Identity, delegated scope, training, MFA, safeguarding | 3.5/5 | Relationship proof, beneficiary consent, authority expiry/revocation UI |
| 3 | Case/social worker | Organization, identity, consent, caseload scope, training, MFA, safeguarding | 4/5 | HR/agency roster synchronization, supervisor approval, device binding |
| 4 | Programme administrator | Organization, programme/region scope, training, MFA | 4/5 | Maker-checker provisioning and periodic access recertification |
| 5 | Government ministry/agency | Legal organization, scope, training, MFA | 3.5/5 | Data-sharing agreement registry and sovereign tenant boundary tests |
| 6 | NGO/CBO/implementing partner | Organization, programme scope, training, MFA, safeguarding | 3.5/5 | Contract/grant expiry, roster bulk import, partner offboarding rehearsal |
| 7 | DFSP/payment provider | Organization, scope, training, MFA, payment certification | 3.5/5 | Mojaloop participant certificates, settlement limits, conformance evidence |
| 8 | Bank/mobile-money agent | Organization, identity, training, MFA, safeguarding, payment certification | 3.5/5 | Agent KYC, device/location binding, float/liquidity controls |
| 9 | Merchant | Organization, MCC/programme scope, training, MFA, payment certification | 3.5/5 | Merchant settlement account verification and dispute/chargeback operations |
| 10 | Auditor/oversight body | Organization, read scope, training, MFA, lifecycle trail | 3.5/5 | Legal hold, segregated export, purpose limitation and expiry tests |
| 11 | Grievance/appeal officer | Organization, scope, training, MFA, safeguarding | 4/5 | Conflict-of-interest assignment and independent appeal tier |
| 12 | Platform/data/ML governance | Organization, scope, training, MFA, model evidence gates | 4/5 | Break-glass integration, dual approval, recurring access certification |

**Overall onboarding verdict:** backend policy/state robustness is now approximately **3.7/5**. All 12 classes have concrete requirements and auditable states. It is not 5/5 because live verification providers, complete self-service/assisted UIs, notification delivery, periodic recertification, and end-to-end organization offboarding have not been exercised against deployed services.

Every activation is expected to traverse identity, organization when required, consent/legal basis, regional/programme scope, role training, MFA, safeguarding when required, and payment certification when required. Invalid transitions are rejected. Activation reports missing evidence rather than silently enabling an account. Suspension, reactivation, terminal rejection, terminal offboarding, an actor, a reason, and evidence are persisted as lifecycle events.

## Scenario and use-case coverage

### Priority missing product use case

The highest-value incomplete scenario is **closed-loop beneficiary recourse**. A recipient should receive an accessible reason for an eligibility, KYC, fraud, or payment decision; submit evidence online or offline; obtain independent human review under an SLA; receive correction and payment repair when upheld; and feed the adjudicated outcome into model monitoring/retraining without using the appeal itself as an adverse signal. Individual components exist, but this complete loop is not proven end-to-end.

### Exhaustive scenario matrix

| Domain | Scenario | Current status after remediation | Required completion evidence |
|---|---|---|---|
| Identity/enrollment | Online adult registration with NIN | Partial | Live identity-provider match and consent evidence |
| Identity/enrollment | Offline field registration and later sync | Partial | Conflict tests, signed device queue, replay and duplicate handling |
| Identity/enrollment | No phone/shared phone household | Missing product flow | Alternate contact and privacy-safe household communication |
| Identity/enrollment | No NIN/refugee/undocumented person | Missing | Exception identity policy and accountable manual review |
| Identity/enrollment | Duplicate identity across programmes/states | Partial | Cross-programme deterministic/probabilistic dedupe and appeal |
| Identity/enrollment | Child enrolled by guardian then reaches adulthood | Missing | Authority transition, fresh consent and direct account conversion |
| Identity/enrollment | Disability, low literacy, or language accommodation | Missing/partial | WCAG testing, assisted mode, local-language notices, proxy controls |
| Household | Household create/member add | Declared in incomplete journeys; disabled | Durable household repository and tests |
| Household | Split, merge, marriage/divorce, temporary absence | Missing | Effective-dated membership and benefit recalculation |
| Household | Caregiver delegation and revocation | Policy added; operational flow partial | Relationship proof, expiry and beneficiary notification |
| Household | Death report and false death match | Declared but incomplete; disabled | Civil registry evidence, payment hold, appeal and reinstatement |
| Eligibility | PMT collection and scoring | Partial deterministic UI | Versioned policy, provenance, supervisor review and fairness testing |
| Eligibility | Rules change mid-cycle | Missing | Effective-dated rules and grandfathering/reassessment |
| Eligibility | Multi-programme eligibility and stacking | Partial | Conflict/precedence policy and overpayment recovery |
| Eligibility | State-to-state migration | Missing | Transfer of case ownership and programme continuity |
| Eligibility | Emergency/disaster top-up | Missing | Geographic targeting, expiry, surge controls and after-action review |
| Programme | Programme create/update | Implemented | Live approval-policy test and rollback |
| Programme | Pause, restart, close and beneficiary transition | Partial | Durable lifecycle and communication/reconciliation tests |
| Programme | Budget exhaustion during batch | Journey declared but disabled | Atomic reservation, partial-batch policy and compensation |
| Payment | Single controlled merchant purchase | Implemented in app path | Live ledger/processor proof |
| Payment | Monthly bulk disbursement | Partial | End-to-end TigerBeetle/Mojaloop settlement and reconciliation |
| Payment | Timeout after transfer succeeds | Partial design | Idempotent callback/requery and no double pay proof |
| Payment | Partial batch failure/retry | Partial | Bounded retry, poison-item isolation and operator repair |
| Payment | Reversal/refund/chargeback | Schema partial | Ledger reversal, beneficiary notice and settlement reconciliation |
| Payment | Lost/stolen card | Schema partial | Authentication, block/reissue, balance preservation |
| Payment | Agent cash-out and liquidity shortage | Missing | Float checks, alternative agent routing and incident handling |
| Payment | Merchant MCC misuse/collusion | ML/GNN foundation | Real labels, graph ingestion and investigator workflow |
| Payment | Currency/rounding/date boundary | Partial | Property tests and ledger parity |
| Grievance | Named complaint | Partial | Durable workflow, assignment and notification proof |
| Grievance | Anonymous/whistleblower complaint | Missing | Protected identity channel and retaliation controls |
| Grievance | Fraud false-positive appeal | Missing end-to-end | Model version/reason capture, independent adjudication, feedback label |
| Grievance | Appeal of eligibility decision | Missing end-to-end | Decision evidence bundle, SLA, second-level appeal |
| Grievance | GBV/safeguarding-sensitive case | Policy gate added | Restricted case visibility and safe referral process |
| Stakeholder | Staff invitation and activation | Backend implemented | UI and live identity/MFA integration |
| Stakeholder | Partner organization roster | Partial | Bulk roster, contract expiry and recertification |
| Stakeholder | Agent/merchant onboarding | Backend gates added | KYC, location, settlement and sanctions checks |
| Stakeholder | Access suspension/offboarding | Backend implemented | Keycloak/Permify revocation propagation and session invalidation test |
| Stakeholder | Temporary delegation | Incomplete journey disabled | Durable authorization relation and expiry worker |
| Stakeholder | Break-glass access | Incomplete journey disabled | Dual control, alarm, reason, expiry and retrospective review |
| Privacy | Consent withdrawal | Data structure partial | Processing-purpose stop, downstream propagation and legal exceptions |
| Privacy | Data access/correction request | Missing | Identity verification, export, correction trail and SLA |
| Privacy | Retention/deletion/legal hold | Missing | Policy engine, cryptographic deletion and hold precedence |
| Privacy | Cross-agency data sharing | Partial interoperability | Purpose limitation, minimization, consent/legal basis and audit |
| Security | Credential recovery/device theft | Partial | Recovery proofing, token/session revocation and device rebind |
| Security | Tenant boundary escape | Partial multi-tenancy code | Adversarial integration tests against DB, search and object storage |
| Security | Keycloak/Permify unavailable | Improved fail-closed paths | Chaos test and emergency operating procedure |
| Security | APISIX/open-appsec bypass | Manifest/config only | Route-policy and WAF attack corpus tests |
| Events | Duplicate/out-of-order Kafka event | Partial idempotency/DLQ | Broker-backed tests with reorder and retry |
| Events | Poison event | Retry/DLQ now wired | Kafka-backed DLQ persistence and replay approval |
| Events | Bounded forensic replay | Implemented in code | Live broker integration test and reducer-specific replay mode |
| Data | Late-arriving lakehouse event | Partial | Event-time correction and partition rewrite policy |
| Data | Parquet/Delta write failure | Offset durability fixed | Object-store fault injection and recovery |
| Data | Schema evolution | Partial | Compatibility registry and producer/consumer contract tests |
| Data | Database failover/restore | Not proven | RPO/RTO test with validated backups |
| Data | Redis loss/eviction | Not proven | HA/TLS/eviction configuration and cache-loss test |
| Search | OpenSearch outage/reindex | Partial/fail-closed config | Backfill checkpoint, alias swap, shard restore |
| ML | CPU fraud inference | Implemented and tested | Production latency/capacity SLO |
| ML | Distributed Ray training | Implemented and executed | Multi-node cluster and failure recovery test |
| ML | Fine-tune from parent checkpoint | Implemented and executed | Approved production dataset and reviewer |
| ML | Neo4j graph export and GNN fine-tune | Implemented and executed against snapshot | Live Neo4j snapshot with governed labels |
| ML | Sticky A/B assignment | Implemented | Live variant artifacts and experiment approval |
| ML | Delayed fraud outcomes | Implemented | Investigator integration and label-quality audit |
| ML | Drift/performance alerts | Implemented evaluator | Scheduler, alert routing and response runbook |
| ML | Bias by state/sex/disability/programme | Not claimable | Approved protected attributes, fairness metrics and mitigation |
| ML | Rollback | Registry pointer supports it | Deployment-controller integration and rehearsal |
| Operations | Disaster recovery | Not proven | Full restore and reconciliation exercise |
| Operations | Observability/SLO | Partial | Unified dashboards, paging and error-budget policy |
| Operations | Supply-chain security | Partial | SBOM, signature, provenance and image scanning gates |

## Integration readiness

Scores use a strict five-point scale: **0 absent, 1 declaration only, 2 real adapter, 3 integrated application path, 4 locally exercised, 5 production/HA/conformance proven**.

| Component | Score | Evidence-based assessment | What prevents a higher score |
|---|---:|---|---|
| Application MySQL/Drizzle | 3.5/5 | Broad schema, migrations, CRUD, pagination, audit, onboarding | No live HA/failover/restore test; migration drift not exercised here |
| PostgreSQL | 1.5/5 | Added as MLflow metadata backend only | Main application is MySQL, not PostgreSQL; no `pg-core` app schema |
| TigerBeetle | 2/5 | Go clients and domain activities exist | Thirty journey activities previously bypassed it; no live ledger/idempotency/reconciliation proof |
| Redis | 2.5/5 | Node/Go clients, rate limiting, caching/idempotency references, real ping health | No cluster/TLS/eviction/failover test; some disabled journeys still contain tracing-only calls |
| Mojaloop | 3/5 | FSPIOP clients, callback routes, durable callback handler, pinned RSA key parsing and real RS256 JWS verification | No scheme conformance, certificate rotation, settlement or timeout reconciliation test |
| Kafka | 3.5/5 | KafkaJS producer/consumer, retries, broker-backed DLQ in production, real admin health, bounded replay, lakehouse durability fix | No broker available for live integration/chaos test |
| APISIX | 2.5/5 | Real Admin API route management, CORS/rate limits and Mojaloop callbacks | No live route contract test; Mojaloop JWS must be validated end-to-end |
| Keycloak | 3/5 | Token verification, login/admin helpers and HA manifests | Realm/client/role bootstrap, recovery and outage behavior not exercised live |
| open-appsec | 1.5/5 | Kubernetes deployment/config declaration | No runtime enforcement or attack-suite evidence |
| Permify | 3/5 | SDK checks, schema/relationship operations, health, production fail-closed change | No deployed policy fixture test or propagation test with Keycloak/offboarding |
| OpenSearch | 2.5/5 | Real client/index/search, production credentials/TLS enforced | No live cluster, reindex/backfill, snapshot/restore or consistency proof |
| Fluvio | 2/5 | Producer/consumer and required-delivery mode | No live cluster/topology, partition semantics or Kafka coexistence decision |
| Lakehouse/Delta/Parquet | 3/5 | Services/manifests, exporter, Parquet path, post-durability Kafka acknowledgment | No live object store/catalog compaction/restore test |
| Ray | 4/5 | Real three-trial distributed PyTorch path executed successfully | Local multi-process only; no multi-node fault tolerance test |
| MLflow | 4/5 | Tracking logger plus PostgreSQL-backed compose deployment; a local tracking server accepted the shipped model, metrics, tags and artifacts | No SSO/TLS/backup or HA validation |
| Neo4j | 2.5/5 | Compose service and credential-required graph exporter; snapshot fine-tuning works | No live graph database or production graph data |
| AI/ML/DL/GNN overall | 4/5 engineering, 1.5/5 production validation | Real models, weights, loops, registry, experiments, monitoring, Ray, CPU inference | All shipped labels are synthetic; no real fraud-case validation or fairness approval |

### Direct answers

1. **PostgreSQL:** not the application database. It is now configured only for MLflow metadata. The application remains MySQL/Drizzle; claims of full PostgreSQL integration would be false.
2. **TigerBeetle:** a real adapter exists, but production ledger correctness is unproven and incomplete journeys are disabled rather than allowed to fake account/transfer success.
3. **Redis:** real clients and health checks exist; HA, TLS, eviction, failover and all journey-level semantics are not proven.
4. **Mojaloop:** meaningful protocol code and real RS256 callback verification exist, but no live scheme conformance, certificate rotation, settlement or timeout-after-success proof exists.
5. **Kafka:** one of the stronger integrations after remediation, including real health, retries/DLQ, replay, and lakehouse acknowledgment ordering. A live broker test is still required.
6. **APISIX:** real configuration integration, not production proof. Routes, rate limits and callbacks exist; policy/JWS tests remain.
7. **Keycloak:** real token and administration code plus manifests; realm lifecycle and all stakeholder provisioning are not live-tested.
8. **open-appsec:** deployment declaration only; it must not be counted as an enforced WAF without runtime evidence.
9. **Permify:** real SDK integration and now fail-closed for critical production writes; live policy fixtures and revocation propagation remain.
10. **OpenSearch:** real search client and index definitions, now without insecure production defaults; operational durability and reindexing remain.
11. **Fluvio:** real API use but weak operational evidence; the architecture must decide its non-overlapping role beside Kafka.
12. **AI/ML/DL/GNN/Neo4j:** real PyTorch training and weights now exist for fraud, credit and GNN; training/fine-tuning scripts exist; Ray was executed; Neo4j export exists; inference runs on CPU; the fraud API does not use hand-written scoring rules. None of the shipped models is validated on real fraud cases.

## Model artifacts and measured results

| Artifact | Version | Measured validation | Important interpretation |
|---|---|---|---|
| Fraud | `fraud-ml-20260920T002126Z-43-50000` | F1 0.3451 on 10,000-row synthetic temporal holdout | Ray-selected of three trials; staging only |
| Credit | `credit-risk-20260920T001846Z-52-30000` | F1 0.2497 on 6,000-row synthetic holdout | Deliberately prohibited for benefit eligibility |
| GNN | `fraud-gnn-20260920T001851Z-62-1800` | F1 0.7391 on 360 synthetic graph nodes | Demonstrates graph pipeline, not real fraud performance |

The relatively modest fraud and credit F1 scores are reported rather than hidden. Accuracy is not used as the sole metric on imbalanced labels. The registry marks all three as staging-only synthetic candidates. No source or API describes them as production validated.

## Security, integrity, and operations findings outside the original scope

The audit also found and repaired build-breaking contracts, a beneficiary UI displaying fields the server never returned, duplicate router keys, a malformed blockchain predecessor lookup, a fully fake audit API, invalid Kafka health logic, swallowed message failures, stale mock routers, unmounted GraphQL placeholder code, insecure OpenSearch credentials/TLS defaults, and a lakehouse acknowledgment-order bug capable of losing events after a crash.

Residual high-risk items are not hidden:

- The main application database choice is MySQL while some documentation says PostgreSQL.
- Open-appsec enforcement, APISIX policy, Keycloak realm bootstrap, Permify policies, Mojaloop conformance, TigerBeetle settlement, and infrastructure HA need live environments.
- National identity and cross-sector federation now make bounded authenticated HTTP calls instead of fabricating successful responses. Incomplete journey workflows remain disabled by default so tracing-only activities cannot falsely complete in normal deployment.
- The audit table lacks standardized result, severity, IP/session, target type, review state and tamper-evident chaining.
- Event replay now replays real records into an audit log; domain-specific state rebuilding still needs idempotent reducers and a controlled write target.
- The web build passes but warns about optional Vite branding/analytics variables and a large JavaScript chunk.
- Docker was unavailable in the sandbox, so compose files were reviewed and Python services executed directly, but container startup was not proven.

## Validation evidence

| Validation | Result |
|---|---|
| Strict TypeScript `pnpm check` | Passed |
| Production `pnpm build` | Passed; only environment/template and chunk-size warnings |
| Python compile | Passed for all ML, fraud-service and Ray modules |
| ML regression suite | 8/8 passed |
| Stakeholder policy/lifecycle suite | 14/14 passed across all actor classes |
| Fraud FastAPI CPU inference | Passed with real shipped weights |
| Prediction and delayed-outcome persistence | Passed |
| Ray distributed training | Passed; three real trials, selected model registered |
| Fraud checkpoint fine-tuning | Passed with parent lineage |
| GraphSAGE snapshot fine-tuning | Passed on CPU with parent lineage |
| Model registry checksums/stage gates | Passed |
| MLflow tracking API and artifact upload | Passed against a live local server |
| Monitoring evaluator with 60 persisted labels | Passed |
| Go formatting | Passed with `gofmt`; module now declares the Dapr-required Go 1.24.6 toolchain |
| Go full build/tests | Passed across all packages with Go 1.24.6 after Dapr/TigerBeetle API migrations and explicit fail-closed missing activities |
| Docker Compose runtime validation | Not run: Docker unavailable in sandbox |

## Release gates that remain mandatory

1. Select and document one application database dialect; run migration, failover, backup and restore tests.
2. Keep synthetic models out of production. Acquire governed adjudicated fraud labels and execute temporal, geographic, programme and subgroup validation.
3. Add real device/geospatial features or formally remove them from the model contract; do not continue silent production imputation.
4. Complete the beneficiary recourse loop and test it with accessibility and local-language requirements.
5. Connect stakeholder activation/offboarding to Keycloak, Permify, organization verification, notification and session revocation.
6. Run TigerBeetle/Mojaloop idempotency, timeout-after-success, reversal and reconciliation tests in a conformance environment.
7. Add broker integration tests for duplicate, reordered and poison messages and exercise the broker-backed DLQ under failure.
8. Implement reducer-specific state replay with dry-run, approval, isolated target and reconciliation before allowing recovery writes.
9. Prove APISIX/open-appsec/Keycloak/Permify/OpenSearch configurations against deployed services and attack/failure tests.
10. Run privacy impact, threat model, accessibility, disaster recovery, chaos, SBOM/signing and operational readiness reviews.

> **Final verdict:** the repository is materially more honest and capable than the imported archives. It now contains a real ML engineering stack and a credible backend stakeholder lifecycle, while incomplete external integrations and unvalidated real-world performance are explicitly blocked or identified. Production readiness still depends on external systems, governed data and operational evidence that cannot be fabricated inside a source audit.
