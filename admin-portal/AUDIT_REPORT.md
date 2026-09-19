# Social Protection Platform Deep Audit

**Audit basis.** This report is based on the cloned repository `munisp/socialservices`, the supplied `social-protection-platform-ENCRYPTION-OPENLANE(1).tar.gz` archive, and a source-level audit of the imported tree. The GitHub repository was empty at audit time; the archive was imported into `admin-portal/` and committed as the baseline. The Google Drive URL was reachable only through Google Drive's virus-scan confirmation page, so it was not silently treated as a second archive or merged without access to its bytes.

## Executive conclusion

The platform is a substantial **admin-oriented prototype/reference implementation**, not yet a production-ready multi-stakeholder social-protection operating platform. It contains many real adapters, Kubernetes manifests, Temporal journey definitions, Kafka/Fluvio/Redis/TigerBeetle/Mojaloop/OpenSearch/Keycloak/Permify/APISIX references, and a MySQL/Drizzle schema. However, configuration presence is frequently mistaken for operational integration. The strongest source-proven gaps were simulated event replay, rule-only fraud scoring, fabricated ML metrics, constant beneficiary risk, simulated A/B results, missing production model artifacts, no production data-to-training watermark, no deployed registry, no drift/performance monitoring, limited role modeling, and a MySQL implementation conflicting with documentation that calls for PostgreSQL.

## Implemented in this audit

The repository now includes a reproducible synthetic Nigerian-like transaction generator, an actual PyTorch fraud MLP training loop, a dependency-free PyTorch GraphSAGE implementation, CPU artifact loading, model manifest and lineage output, continuous-training entrypoint, and PSI drift-monitoring utilities under `ml/`. The fraud service was replaced with fail-closed PyTorch inference; it no longer reports placeholder metrics or silently scores with hand-written rules. These artifacts are deliberately marked synthetic until a governed production export is supplied.

## Stakeholders and onboarding coverage

The platform needs at least **12 stakeholder classes**. The existing code materially supports only two generic application roles (`admin`, `user`) plus beneficiary records and simulated personas in tests. The target stakeholder inventory is:

| Stakeholder | Current source evidence | Onboarding maturity | Required workflow |
|---|---|---:|---|
| Beneficiary/recipient | beneficiary, KYC, card, enrollment tables and mobile journey | Partial | assisted/offline registration, consent, deduplication, KYC, appeal, accessibility, payment instrument |
| Household representative/caregiver | no complete first-class actor model | Missing | delegated consent, relationship proof, limits, revocation |
| Case/social worker | journey references and test persona only | Missing/partial | agency invitation, training, caseload, least privilege, supervision |
| Programme administrator | programs, schedules, approvals | Partial | organization verification, programme scope, maker-checker, SLA |
| Government ministry/agency | tenant/workflow references, no complete identity lifecycle | Partial | legal entity verification, data-sharing agreement, region scope |
| NGO/CBO/implementing partner | not first-class in core schema | Missing | organization verification, grant/program scope, worker roster |
| Payment service provider/DFSP | Mojaloop client code | Partial | participant onboarding, certificates, settlement/reconciliation, callback controls |
| Bank/mobile-money agent | payment references, no complete agent lifecycle | Missing | agent KYC, location, float, sanctions, device binding |
| Merchant | MCC rules and simulator | Partial | merchant enrollment, MCC/location verification, settlement and dispute |
| Auditor/oversight body | audit logs/reports | Partial | read-only evidence, legal hold, export, independent tenant boundary |
| Grievance/appeal officer | grievance journey code | Partial | intake, triage, conflict checks, SLA, escalation, resolution appeal |
| Platform operator/data/ML governance | middleware dashboards and ML tables | Partial | privileged break-glass, model approvals, data lineage, monitoring |

**Overall onboarding verdict: 2/12 mature enough to claim generic access; 6/12 partial; 4/12 missing.** Authentication is not onboarding. A production onboarding workflow must include organization/person identity proof, role approval, consent and legal basis, tenant/region/program scope, training/attestation, MFA/device binding, suspension/offboarding, auditability, and recovery.

## Scenarios the platform does not handle adequately

The highest-value uncovered scenarios are: beneficiary registration without connectivity; shared phone or no-phone households; caregiver delegation and revocation; duplicate identity across programmes; a beneficiary moving state; household split/merge; disability/accessibility accommodation; language/low-literacy assisted service; death/incorrect death match; child-to-adult transition; refugee/undocumented recipient; disaster emergency top-up; programme pause and restart; partial or failed bulk payment; payment reversal and reconciliation; provider outage during callback; merchant dispute/chargeback; agent cash-out abuse; cross-programme overpayment; sanctions or watchlist match; data-subject access/correction/deletion/objection; consent withdrawal; grievance anonymity and retaliation protection; whistleblower case; safeguarding/GBV-sensitive case; conflict-of-interest assignment; field-worker device theft; lost credential/MFA recovery; tenant offboarding; audit legal hold; model false positive appeal; label delay and chargeback feedback; model drift by state, gender, disability, programme or merchant; graph collusion ring; fraud model rollback; lakehouse late-arriving events; Kafka duplicate/out-of-order events; corrupted Parquet partition; database failover; Redis loss; TigerBeetle idempotency collision; Mojaloop timeout after successful transfer; Keycloak realm misconfiguration; Permify unavailable; OpenSearch lag; Fluvio/Kafka divergence; and a complete disaster recovery restore test.

The most consequential product use case currently missing is **closed-loop beneficiary recourse**: a recipient should be able to see a decision, understand the reason in an accessible language, submit evidence, obtain human review within an SLA, receive a payment correction if upheld, and have the outcome fed into governed model retraining. The existing grievance journeys do not prove this end-to-end loop.

## Integration evidence matrix

| Component | Evidence in tree | Honest assessment | Blocking gap |
|---|---|---|---|
| MySQL/Drizzle | `server/db.ts`, `drizzle/schema.ts` | Integrated for app CRUD when `DATABASE_URL` exists | Docs say PostgreSQL; driver/schema are MySQL. No HA/failover proof. |
| PostgreSQL | no `pg` driver or pg-core schema | Not integrated | Choose one dialect or build a tested dual-dialect boundary. |
| TigerBeetle | Go middleware/client and workflows | Real client calls exist | Go toolchain/tests unavailable here; production settlement/retry/idempotency proof absent. |
| Redis | Go client and monitoring references | Partial | No demonstrated HA, TLS, eviction policy, or failure semantics. |
| Mojaloop | FSPIOP client/callback code | Partial protocol client | No live conformance or end-to-end settlement evidence. |
| Kafka | TypeScript middleware, Go clients, lakehouse consumer | Partial-to-strong code presence | Lakehouse marks idempotency/offset before durable write; no production broker proof. |
| APISIX | route/config middleware | Configuration integration | Default/open deployment rules and no live route contract test. |
| Keycloak | token/admin helpers and manifests | Partial | Realm/client/role provisioning and recovery are not proven. |
| openappsec | Kubernetes manifest | Deployment declaration only | No enforcement evidence or policy tests. |
| Permify | middleware/client | Partial | Fail-open/disabled behavior needs explicit production refusal and policy fixtures. |
| OpenSearch | client/index setup | Partial; explicitly mock mode when unset | Search silently degrades; no reindex/backfill or shard/backup proof. |
| Fluvio | middleware/manifest | Partial | No verified producer/consumer topology or migration semantics with Kafka. |
| AI/ML/DL/GNN/Neo4j | prior rule/LLM path; no shipped weights/Neo4j integration | Previously aspirational; now training/inference foundations added | Need governed real labels, Neo4j graph export, registry, rollout, monitoring, and approval. |

## ML audit and remediation status

The old implementation used `invokeLLM` for “training” and prediction, a rule-only FastAPI scorer, fixed metrics, a constant beneficiary risk of `0.25`, and simulated A/B counts/results. Those are removed from the fraud service path. New code trains a real `FraudMLP` with `BCEWithLogitsLoss`, AdamW, validation metrics, saved weights, preprocessing statistics, and a manifest; it provides a GraphSAGE model for future graph feature integration; it supports CPU inference with `map_location='cpu'`; it provides a continuous-training entrypoint that requires an explicit exported dataset and writes lineage; and it implements PSI drift scoring.

The new synthetic generator is useful for development and pipeline tests but is **not evidence of Nigerian production performance**. Before a production decision, add: label governance and review queues; temporal and geographic leakage controls; subgroup metrics and calibration; signed registry artifacts; model approval and rollback; shadow/A-B traffic assignment with real prediction logs; delayed-label evaluation; drift/performance alerts; Neo4j graph snapshots; and a real lakehouse export watermark.

## Security/data-integrity findings

The source contains unsafe production defaults or degraded modes: wildcard CORS in the old fraud service; default webhook/data-protection secrets; generated encryption keys that change on restart; OpenSearch mock mode; broad `0.0.0.0/0` infrastructure rules; generic admin/user authorization; background `setTimeout` training; and simulated replay data. These must be denied or explicitly blocked in production, not merely logged. The audit also found documentation/status files that label simulated ETL and mock activities as complete; those labels should be corrected to distinguish implementation, test double, and production evidence.

## Recommended release gates

1. Establish a single database dialect and pass HA/failover/migration tests.
2. Ship the model artifact only through a signed registry with a reproducible manifest.
3. Connect production event export to training using immutable watermarks and label governance.
4. Implement all 12 stakeholder onboarding/offboarding workflows with tenant/region/program scope.
5. Prove payment idempotency and reconciliation across TigerBeetle and Mojaloop.
6. Replace simulated replay with a real Kafka consumer or disable the feature in production.
7. Add integration tests that run against real Postgres/MySQL choice, Redis, Kafka, Keycloak, Permify, OpenSearch, and Mojaloop test doubles/conformance environments.
8. Run threat modeling, privacy impact assessment, accessibility, DR restore, chaos, and subgroup ML validation before production launch.

**Bottom line:** after this change the repository contains a real, runnable ML foundation and a more honest platform boundary. It still cannot truthfully claim production readiness for all integrations or all stakeholder journeys until the release gates above are executed against real environments and governed data.
