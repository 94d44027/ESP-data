# Algorithm Requirements Specification (ALGO)
## ESP PoC — TTA/TTB Path Calculation and related things

**Version:** 1.8  
**Date:** March 14, 2026  
**Prepared by:** Konstantin Smirnov with the kind assistance of Perplexity AI  
**Project:** ESP PoC for Nebula Graph  
**Reference:** Derived from SRS, UIS, SCHEMA, TTB/TTT flows
**Document code:** ALGO 

---

## 1. Overview

### 1.1 Purpose

This document specifies the algorithmic requirements for path discovery and TTA (Time To Attack) calculation in the ESP PoC system. It covers how attack paths are found in the graph, how TTA/TTB values are computed, and how mitigations influence the results.

### 1.2 Document Scope

This specification covers:
- Path discovery algorithm and its nGQL/MATCH implementation
- TTA/TTB computation rules
- Supporting data endpoints (entry points, targets)
- Mitigation impact on path scoring (future sections, to be expanded)
- Edge cases and algorithmic constraints

This specification does **not** cover:
- Visual presentation of paths — see UI-Requirements.md (UI-REQ-206 through UI-REQ-209, UI-REQ-332)
- General API contracts and non-functional requirements — see Requirements.md (SRS)
- Data model definitions — see ESP01_NebulaGraph_Schema.md

### 1.3 Relationship to Other Documents

| Document                             | Version | Relationship                                                                                                                                                                                                                                                                                                                            |
|--------------------------------------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Requirements.md (SRS)                | v1.15   | Parent document. Stubs REQ-029–032 reference this spec. API summary in Appendix C links here.                                                                                                                                                                                                                                           |
| UI-Requirements.md  (UIR)            | v1.13   | UI-REQ-207 consumes path calculation results; UI-REQ-208/332 visualise them on the graph canvas.                                                                                                                                                                                                                                        |
| ESP01_NebulaGraph_Schema.md (SCHEMA) | v1.10   | Defines Asset.TTB property (TA001), connects_to edges (ED005), applied_to edges (ED001) and other elements of database schema, like MitrePlatform (TA011), ED014 (represents), ED003 (can_be_executed_on) being used for OS-platform filtering, defines DI-01/DI-02/DI-03 guarantee has_type, belongs_to, runs_on edges for all Assets. |
| ADR-Requirements.md (ADR)            | v0.1    | ADR-REQ-040 instruments ComputeTTB/computeBatchTTT to populate audit buffer. ADR-REQ-014 persists TTT formula inputs (ALG-REQ-060).                                                                                                                                                                                                     |

### 1.4 Requirement ID Convention

All requirements in this document use the prefix `ALG-REQ-` followed by a three-digit number. Sections use `##` for chapters and `###` for individual requirements (as headers), matching the style of UI-Requirements.md.

---

## 2. Definitions

| Term                   | Definition                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
|------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **TTA**                | Time To Attack — the cumulative time from initial access to the beginning of actions on objective, computed as the sum of TTB along a path                                                                                                                                                                                                                                                                                                                                             |
| **TTB**                | Time To Bypass — the calculated time interval (float, in hours) to traverse (bypass) a single host, computed as the sum of Orientation Time plus, for each tactic in the applicable tactic chain, the Switchover Time and the TTT of the fastest technique. Stored as `Asset.TTB` (TA001) for the intermediate chain position; computed on-the-fly for entrance/target positions (ALG-REQ-053). Supersedes the previous definition (static stored value).                              |
| **TTT**                | Time to execuTe a Technique — time interval (float, in hours) required to execute a technique/subtechnique on a particular asset, considering mitigations applied to that asset and their maturity. TTT is computed per (asset, technique) pair and is consumed by the TTB calculation (ALG-REQ-060). The value ranges from `execution_min` (no mitigations possible or none applied and active) to `execution_max` (all possible mitigations applied and active at maximum maturity). |
| **Path**               | An ordered sequence of Asset nodes connected by directed `connects_to` edges, from an entry point to a target, with no repeated nodes                                                                                                                                                                                                                                                                                                                                                  |
| **Hop**                | A single `connects_to` edge traversal between two adjacent nodes in a path                                                                                                                                                                                                                                                                                                                                                                                                             |
| **Entry Point**        | An Asset where `is_entrance == true`; represents the attacker's starting position                                                                                                                                                                                                                                                                                                                                                                                                      |
| **Target**             | An Asset where `is_target == true`; represents the objective the attacker aims to reach                                                                                                                                                                                                                                                                                                                                                                                                |
| **Path ID**            | Ephemeral sequential identifier (e.g. P00001) assigned to each calculated path within a single session; not persisted in the database                                                                                                                                                                                                                                                                                                                                                  |
| **Mitigation**         | A MITRE ATT&CK mitigation (`tMitreMitigation`) linked to an Asset via `applied_to` edge, potentially modifying the effective TTB                                                                                                                                                                                                                                                                                                                                                       |
| **Tactic Chain**       | An ordered list of MITRE ATT&CK tactic IDs that an attacker executes on a node, determined by the node's position in the attack path. Defined in `chains.json` (ALG-REQ-050).                                                                                                                                                                                                                                                                                                          |
| **Chain Position**     | The role of an asset within a specific attack path: **Entrance** (first node, N₀), **Intermediate** (middle nodes, N₁..Nₖ₋₁), or **Target** (last node, Nₖ). A single asset may hold different positions in different paths.                                                                                                                                                                                                                                                           |
| **Orientation Time**   | A configurable parameter (float, default 0.25 hours / 15 minutes) representing the time an attacker spends on initial reconnaissance of a host before executing techniques. Added once per TTB calculation. See ALG-REQ-071.                                                                                                                                                                                                                                                           |
| **Switchover Time**    | A configurable parameter (float, default 0.1667 hours / 10 minutes) representing the overhead time an attacker incurs when transitioning between tactics on the same host. Added once per tactic iteration in the TTB loop. See ALG-REQ-072.                                                                                                                                                                                                                                           |
| **Priority Tolerance** | A configurable parameter (int, default 1) defining how many priority levels below the maximum are included when filtering techniques by priority. A value of 0 means only the highest-priority techniques are selected; a value of 1 includes the top two priority levels. See ALG-REQ-075.                                                                                                                                                                                            |


---

## 3. Path Discovery

### ALG-REQ-001: Path Calculation Endpoint

The APP layer SHALL provide an API endpoint (`GET /api/paths?from={entryId}&to={targetId}&hops={maxHops}`) that calculates all loop-free directed paths from the entry point asset to the target asset, following `connects_to` edges up to `maxHops` hops (default 6, valid range 2–9). For each path the response SHALL include:

- A server-generated sequential path ID (format `P` + zero-padded 5-digit number, e.g. `"P00001"`)
- The ordered host chain as a string of Asset_IDs separated by `->`
- The TTA value computed per ALG-REQ-010 (position-aware sum of TTB values)

The response SHALL be ordered by TTA ascending. Both `from` and `to` parameters SHALL be validated per REQ-025 (SRS). The `hops` parameter SHALL be validated as an integer in range 2–9; if omitted, default to 6.

Justification for MATCH syntax (per REQ-244 in SRS): variable-length path traversal with loop detection and per-node property extraction has no practical nGQL/GO equivalent. The underlying query returns **path topology and per-node stored TTB values** (not a pre-summed TTA). Final TTA is computed by the APP layer using position-aware TTB (ALG-REQ-010, ALG-REQ-051).

```nGQL
MATCH p = (a:Asset)-[e:connects_to*..{maxHops}]->(b:Asset)
WHERE a.Asset.Asset_ID == "{entryId}" AND b.Asset.Asset_ID == "{targetId}"
  AND ALL(n IN nodes(p) WHERE single(m IN nodes(p) WHERE m == n))
WITH nodes(p) AS pathNodes
WITH [n IN pathNodes | n.Asset.Asset_ID] AS ids,
     [n IN pathNodes | COALESCE(n.Asset.TTB, 10)] AS ttbs
RETURN ids, ttbs;
```

>Note 1: The query returns an ordered list of Asset_IDs (`ids`) and their stored TTB values (`ttbs`) per path. The stored TTB represents the Regular_chain (intermediate-position) value. The APP layer substitutes position-specific TTB values for the entry point (index 0) and target (last index) before summing.

>Note 2: The previous version of this query computed `SUM(r.Asset.TTB)` and the `A -> B -> C` host chain string in the GrDB. Both have been moved to the APP layer: TTA computation now requires position-aware TTB substitution, and host chain formatting is a presentation concern that does not belong in the database query. The APP layer SHALL construct the host chain string by joining the `ids` array with `" -> "` as separator (e.g. `strings.Join(ids, " -> ")` in Go).

>Note 3: Path IDs are generated by the APP layer (Go code) sequentially per response — they are ephemeral and not persisted.

Response format:

```json
{
  "paths": [
    {
      "path_id": "P00001",
      "hosts": "A00013 -> A00014 -> A00012 -> A00011",
      "tta": 274
    },
    {
      "path_id": "P00002",
      "hosts": "A00013 -> A00014 -> A00007 -> A00011",
      "tta": 286
    }
  ],
  "entry_point": "A00013",
  "target": "A00011",
  "hops": 6,
  "total": 15
}
```

**Migrated from:** REQ-029 (SRS v1.10)  
**Amended in:** v1.2 — query returns per-node data; APP computes position-aware TTA.


### ALG-REQ-002: Entry Points List Endpoint

The APP layer SHALL provide an API endpoint (`GET /api/entry-points`) that returns all assets where `is_entrance == true`, along with their Asset_ID and Asset_Name. This populates the entry point dropdown in the Path Inspector UI (UI-REQ-207 §1). The underlying query:

```nGQL
LOOKUP ON Asset WHERE Asset.is_entrance == true
YIELD id(vertex) AS vid, Asset.Asset_ID AS asset_id, Asset.Asset_Name AS asset_name;
```

**Migrated from:** REQ-030 (SRS v1.10)

### ALG-REQ-003: Targets List Endpoint

The APP layer SHALL provide an API endpoint (`GET /api/targets`) that returns all assets where `is_target == true`, along with their Asset_ID and Asset_Name. This populates the target dropdown in the Path Inspector UI (UI-REQ-207 §2). The underlying query:

```nGQL
LOOKUP ON Asset WHERE Asset.is_target == true
YIELD id(vertex) AS vid, Asset.Asset_ID AS asset_id, Asset.Asset_Name AS asset_name;
```

>Design note on ALG-REQ-002/003: Alternatively, these could have been derived client-side from the existing `/api/assets` response (REQ-021 in SRS), which already returns `is_entrance` and `is_target` for every asset. However, dedicated endpoints are cleaner for the Path Inspector and avoid coupling to the full asset list load.

**Migrated from:** REQ-031 (SRS v1.10)

---

## 4. TTA Computation

### ALG-REQ-010: TTA Calculation Rule

TTA for a given path SHALL be computed as a position-aware sum of TTB values for all nodes in the path. Each node's TTB is computed using the tactic chain corresponding to its position (ALG-REQ-051).

**Formal definition:**

Given a path `[N₀, N₁, N₂, ..., Nₖ]` where `N₀` is the entry point and `Nₖ` is the target:

    TTA = TTB(N₀, Cₑ) + Σ TTB(Nᵢ, Cᵣ) for i = 1 to k-1 + TTB(Nₖ, Cₜ)

where:
- `Cₑ` = `CHAIN_ENTRANCE` vertex in GrDB (ALG-REQ-050)
- `Cᵣ` = `CHAIN_INTERMEDIATE` vertex in GrDB (ALG-REQ-050)
- `Cₜ` = `CHAIN_TARGET` vertex in GrDB (ALG-REQ-050)
- `TTB(N, C)` = Time To Bypass for asset N computed using tactic chain C (ALG-REQ-052)

For intermediate nodes (N₁ through Nₖ₋₁), the stored `Asset.TTB` property is used directly (it represents the Regular_chain value). For the entry point (N₀) and target (Nₖ), TTB is computed on-the-fly using their respective tactic chains (ALG-REQ-053).

If any stored TTB value is `NULL` for an intermediate node, the default value of **10** (per schema TA001 default) SHALL be used.

**Special case — 2-node path** `[N₀, Nₖ]` (direct connection, 1 hop):

    TTA = TTB(N₀, Cₑ) + TTB(Nₖ, Cₜ)

No intermediate nodes contribute. Both TTB values are computed on-the-fly.

>Design note (v1.2): The entry point is now **included** in TTA. In v1.1, the entry point was excluded on the rationale that it represents the attacker's starting position. With position-aware tactic chains, the entry point incurs the cost of Initial Access (TA0001), which takes measurable time. Excluding it would undercount TTA.

**Migrated from:** REQ-032 (SRS v1.10)  
**Amended in:** v1.2 — position-aware formula; entry point included.

### ALG-REQ-011: Path ID Generation

Path IDs SHALL be generated by the APP layer sequentially per API response, starting from `P00001` and incrementing by 1 for each path in the result set. Path IDs are ephemeral — they are not stored in the database and have no meaning outside the current response context.

**Derived from:** ALG-REQ-001 note

---

## 4A. Asset State Hashing and TTB Recalculation

### ALG-REQ-040: Asset State Hash Definition

Each Asset vertex SHALL have two properties that track its computational state (per SCHEMA v1.7, TA001):

- `hash` (string, default ""): A string representation of the MurmurHash2 value computed from the asset's state inputs.
- `hash_valid` (bool, default false): Indicates whether the stored hash reflects the current state of the asset's edges and properties.

The hash represents a fingerprint of everything that affects an asset's TTB. When any hash input changes, `hash_valid` is set to `false`, marking the asset as **stale** — meaning its TTB may no longer be accurate and should be recalculated before being used in TTA computation.

On initial deployment (or application restart in the current stateless architecture), all assets have `hash_valid == false` by default, ensuring a full TTB recalculation on first use.

>Note: Why MurmurHash2? Here and everywhere in this document. Any other hash function would have done the job, yet it is a built-in function of the Nebula Graph. Since every attempt has been made to offload the data shuffling from the APP to the GRDB layer, Nebula's own built-in function is as good as any.

### ALG-REQ-041: Hash Input Definition

The hash for an asset SHALL be computed from the following inputs, chosen based on the principle "what makes this node harder to bypass":

| Input                           | Source                                                                                        | Reasoning                                                                                                     |
|---------------------------------|-----------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------------------------------|
| **Inbound `connects_to` edges** | `(src:Asset)-[c:connects_to]->(this)` — source Asset_ID, Connection_Protocol, Connection_Port | Defines how the attacker can reach this node. SSH vs. HTTPS vs. ICMP changes the available attack techniques. |
| **Applied mitigations**         | `(m:tMitreMitigation)-[e:applied_to]->(this)` — Mitigation_ID, Maturity, Active               | Directly affects difficulty of compromise.                                                                    |
| **Operating system**            | `(this)-[:runs_on]->(os:OS_Type)` — OS_Name                                                   | OS type determines available techniques and hardening posture.                                                |
| **Vulnerability flag**          | `this.Asset.has_vulnerability`                                                                | Critical vulnerability drastically reduces TTB.                                                               |
| **Asset type**                  | `(this)-[:has_type]->(t:Asset_Type)` — Type_Name                                              | Device type influences technique applicability (server vs. IoT vs. mobile).                                   |

**Excluded from hash:** Outbound `connects_to` edges. These define where the attacker goes *after* bypassing this node, not how hard the node itself is to compromise. An outbound connection change affects the *destination* asset's inbound set, not the source's TTB.

>Design note: If a new inbound `connects_to` edge is added to asset A from asset B, only A's hash is invalidated (A's inbound set changed). B's hash remains valid because B's TTB — how hard B itself is to bypass — is unaffected.

### ALG-REQ-042: Hash Computation Algorithm

The hash SHALL be computed using the NebulaGraph built-in `hash()` function (MurmurHash2, returns int64) applied to a deterministic canonical string built from the inputs defined in ALG-REQ-041.

The canonical string is constructed by:
1. Collecting inbound connection descriptors, sorted by (source Asset_ID, protocol, port)
2. Collecting applied mitigation descriptors, sorted by Mitigation_ID
3. Concatenating all parts with `"##"` as the section separator and `";"` as the item separator

The reference nGQL query for computing hashes of all stale assets:

```nGQL
MATCH (a:Asset)
WHERE a.Asset.hash_valid == false
OPTIONAL MATCH (src:Asset)-[c:connects_to]->(a)
WITH a, src, c,
  src.Asset.Asset_ID AS src_id,
  c.Connection_Protocol AS c_proto,
  c.Connection_Port AS c_port
ORDER BY src_id, c_proto, c_port
WITH a, collect(concat_ws("|", src_id, c_proto, c_port)) AS conn_parts
OPTIONAL MATCH (m:tMitreMitigation)-[e:applied_to]->(a)
WITH a, conn_parts, m, e,
  m.tMitreMitigation.Mitigation_ID AS mit_id
ORDER BY mit_id
WITH a, conn_parts,
  collect(concat_ws("|", mit_id, toString(e.Maturity), toString(e.Active))) AS mit_parts
MATCH (a)-[:runs_on]->(os:OS_Type)
MATCH (a)-[:has_type]->(t:Asset_Type)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.TTB AS current_ttb,
  a.Asset.hash AS stored_hash,
  hash(concat_ws("##",
    reduce(s = "", x IN conn_parts | s + x + ";"),
    reduce(s = "", x IN mit_parts | s + x + ";"),
    toString(a.Asset.has_vulnerability),
    os.OS_Type.OS_Name,
    t.Asset_Type.Type_Name
  )) AS computed_hash;
```

>Design note 1: `hash()` is not a cryptographic hash — it is MurmurHash2 returning int64. This is acceptable for change detection purposes. The stored `hash` property on Asset is a string representation of this int64 (via `toString()` in the APP layer).

> Design note 2: "OPTIONAL MATCH for runs_on and has_type replaced with MATCH per SRS REQ-043 / SCHEMA DI-01 and DI-03. COALESCE() fallbacks removed as these relationships are guaranteed present. (since ver 1.6 of this document)

>Design note 3: The OPTIONAL MATCH for connects_to (inbound connections) and applied_to (mitigations) SHALL remain OPTIONAL MATCH, because an asset MAY have zero inbound connections and MAY have zero applied mitigations. These are not covered by DI invariants. Since version 1.6 of this document.



>nGQL note: `ORDER BY` operates on column aliases defined in the preceding `WITH` clause, not on raw tag property paths. The pattern `WITH ... AS alias ORDER BY alias WITH ... collect(...)` ensures deterministic ordering before aggregation.

### ALG-REQ-043: Hash Invalidation on Mitigation Change

When an `applied_to` edge is created or updated (REQ-035) or deleted (REQ-036), the APP layer SHALL **additionally**:

1. Set `hash_valid = false` on the affected Asset vertex:
   ```nGQL
   UPDATE VERTEX ON Asset "{assetId}" SET hash_valid = false;
   ```
2. Increment the `stale_count` on the SystemState vertex:
   ```nGQL
   UPDATE VERTEX ON SystemState "SYS001" SET stale_count = stale_count + 1;
   ```

Both statements SHALL be executed after the primary `UPSERT EDGE` or `DELETE EDGE` succeeds. If either invalidation statement fails, the error SHALL be logged but SHALL NOT cause the mitigation operation itself to fail (best-effort invalidation).

>Design note: Future editors (connectivity, vulnerability, OS assignment) SHALL follow the same invalidation pattern — set `hash_valid = false` on the affected asset(s) and increment `stale_count`.

### ALG-REQ-044: TTB Computation Stub

Until the full TTB computation algorithm is implemented (see ALG-REQ-020/021 placeholders and ALG-REQ-052), TTB SHALL be recalculated using a stub procedure that accepts a chain VID parameter identifying the asset's position in the attack path:

``` text
new_TTB = current_TTB + random_integer(1, 10) + chain_tactic_count(chainVID)
```
where `random_integer(1, 10)` produces a uniformly distributed integer in the range inclusive, and `chain_tactic_count(chainVID)` is the number of chain_includes edges from the given TacticChain vertex (SCHEMA TA010, ED013).


The `chain_tactic_count` term ensures that TTB values differ by position even while using the stub: `CHAIN_ENTRANCE` (8 tactics) produces a different offset than `CHAIN_INTERMEDIATE` (7 tactics) or `CHAIN_TARGET` (10 tactics). The APP layer MAY hardcode these counts for the stub or query them from the GrDB.

The stub is implemented in the APP layer (Go code). The function signature SHALL accept the chain VID:

```go
func computeTTBStub(currentTTB int, chainVID string) int
```

The purpose is to verify the end-to-end hash invalidation, position-aware recalculation pipeline, and TTA summation without requiring the full mitigation-aware TTB formula.

>Design note: The chain_tactic_count addition is a minimal change to make the stub position-sensitive. It will be replaced entirely when the real TTB formula (ALG-REQ-052) is implemented. At that point the stub is superseded by the full MITRE subgraph traversal query starting from the same chain VID.

Amended in: v1.2 — stub accepts tactic chain parameter. v1.3 — parameter changed from tacticChain []string to chainVID string; count derived from GrDB chain_includes edges.

> **Superseded in v1.5:** This stub is replaced by the full TTB calculation algorithm defined in ALG-REQ-070. All references to the stub in ALG-REQ-045 and ALG-REQ-046 SHALL be updated in a subsequent coordinated release to reference ALG-REQ-070 instead. Until that update, the stub remains the active implementation in the Go code, but the algorithm defined in ALG-REQ-070 is the normative specification.

### ALG-REQ-045: Bulk TTB Recalculation

The APP layer SHALL provide an API endpoint (`POST /api/recalculate-ttb`) that performs the following steps:

1. Execute the hash computation query (ALG-REQ-042) to obtain all stale assets with their computed hashes
2. For each stale asset where `computed_hash != stored_hash` (or `stored_hash` is empty):
   a. Compute new TTB using the stub procedure (ALG-REQ-044)
>**Clarification (v1.2):** Bulk TTB recalculation computes TTB using the `Regular_chain` tactic chain for all assets. This is because `Asset.TTB` stored in the database always represents the intermediate-position TTB value. Entry-point and target-position TTB values are ephemeral and computed on-the-fly during path calculation only (ALG-REQ-053).

   b. Write back to the database:
      ```nGQL
      UPDATE VERTEX ON Asset "{assetId}"
      SET TTB = {newTTB}, hash = "{computedHash}", hash_valid = true;
      ```
3. After all assets are updated, recompute the Merkle root (ALG-REQ-047) and update SystemState:
   ```nGQL
   UPDATE VERTEX ON SystemState "SYS001"
   SET merkle_root = {newRoot},
       last_recalc_time = datetime(),
       total_assets = {totalCount},
       stale_count = 0;
   ```
4. Return a summary response like:
   ```json
   {
     "recalculated": 12,
     "unchanged": 51,
     "total": 63,
     "merkle_root": "8837429571023847561"
   }
   ```

>Performance note: Steps 2a–2b can be batched. The APP layer SHOULD construct a single multi-statement nGQL string with semicolon-separated UPDATE VERTEX commands and execute them in one session call, rather than issuing N individual queries.

### ALG-REQ-046: Path-Scoped TTB Recalculation

When computing TTA via the path calculation endpoint (ALG-REQ-001), the APP layer SHALL perform the following optimised recalculation flow:

1. **Find paths**: Execute the path discovery query (ALG-REQ-001) to obtain all loop-free directed paths. The query returns ordered node ID lists and their stored (intermediate-position) TTB values.

2. **Extract path members**: Parse the returned paths and extract the unique set of Asset_IDs that participate in any path.

3. **Check hash validity**: For the path member subset only, fetch hash validity:
   ```nGQL
   FETCH PROP ON Asset "A00013","A00014","A00012","A00011"
   YIELD Asset.Asset_ID AS asset_id,
         Asset.hash_valid AS hash_valid,
         Asset.TTB AS ttb;
   ```

4. **Recalculate stale intermediates**: If any path member (excluding entry and target) has `hash_valid == false`, run the hash computation query (ALG-REQ-042) scoped to those assets only, compute new TTB using the stub with `Regular_chain` (ALG-REQ-044), and write back the updated values:
   ```nGQL
   UPDATE VERTEX ON Asset "{assetId}"
   SET TTB = {newTTB}, hash = "{computedHash}", hash_valid = true;
   ```

5. **Compute entry-point TTB**: Compute `TTB(N₀, Entrance_chain)` for the entry point asset using the stub (ALG-REQ-044) with the `Entrance_chain` tactic chain. This value is held **in-memory only** — it is NOT written to `Asset.TTB`.

6. **Compute target TTB**: Compute `TTB(Nₖ, Target_chain)` for the target asset using the stub (ALG-REQ-044) with the `Target_chain` tactic chain. This value is held **in-memory only** — it is NOT written to `Asset.TTB`.

7. **Compute TTA per path**: For each discovered path `[N₀, N₁, ..., Nₖ]`:
   ```
   TTA = ttb_entry + Σ stored_TTB(Nᵢ) for i=1..k-1 + ttb_target
   ```
   where `stored_TTB(Nᵢ)` is the now-fresh `Asset.TTB` from step 4 (or unchanged if hash was valid), and `ttb_entry`/`ttb_target` are the in-memory values from steps 5-6.

8. **Return results**: The response format matches ALG-REQ-001, with an additional field indicating whether recalculation occurred:
   ```json
   {
     "paths": [...],
     "entry_point": "A00013",
     "target": "A00011",
     "hops": 6,
     "total": 15,
     "recalculated_assets": ["A00014", "A00007"]
   }
   ```

>Design note 1: Steps 5-6 are always executed (regardless of hash validity) because the stored `Asset.TTB` represents the Regular_chain value, not the entry/target value. The cost is exactly 2 TTB computations per path-set — negligible even at scale.

>Design note 2: The entry and target assets are the same across all paths in a single API call (the `from` and `to` parameters). Therefore steps 5-6 run once per call, not once per path.

>Design note 3: On large datasets, the intermediate recalculation (step 4) is scoped to stale path members only — an O(P×H) operation instead of O(N). Steps 5-6 add O(1) constant overhead.

>Design note 4: No hash invalidation is needed when the Path Inspector is closed. `Asset.TTB` in the database always stores the Regular_chain value; entry/target TTBs are never persisted. The database state is never "contaminated" by position-specific calculations.

>Design note 5: For the future — a locking mechanism should be put in place for the multi-user version. The "project" (IT Infrastructure model and its mitigations) can be locked (checked in) for a particular user while kept read-only for other authorised users.

**Amended in:** v1.2 — steps 5-7 added for position-aware TTB; step 4 scoped to intermediates only; design note 4 added regarding hash invariance.


### ALG-REQ-047: Merkle Root Computation

The Merkle root (hash-of-hashes) SHALL be computed as follows:

1. Query all asset hashes in deterministic order:
   ```nGQL
   LOOKUP ON Asset
   YIELD Asset.Asset_ID AS asset_id, Asset.hash AS hash
   | ORDER BY $-.asset_id;
   ```
2. Concatenate all hashes in Asset_ID order with `";"` as separator
3. Apply `hash()` to the concatenated string to produce the Merkle root (int64)

The Merkle root is stored in `SystemState.merkle_root` (TA009). It is recomputed:
- After bulk TTB recalculation (ALG-REQ-045, step 3)
- Optionally, on demand via `GET /api/system-state` if a `?refresh=true` parameter is provided (future enhancement)

The Merkle root provides a single-value check for "has anything changed in the model since last full recalculation." If the current computed root differs from the stored root, at least one asset has a stale hash.

### ALG-REQ-048: SystemState Endpoint

The APP layer SHALL provide an API endpoint (`GET /api/system-state`) that returns the current SystemState, fetched from the `"SYS001"` vertex:

```nGQL
FETCH PROP ON SystemState "SYS001"
YIELD SystemState.state_id AS state_id,
      SystemState.merkle_root AS merkle_root,
      SystemState.last_recalc_time AS last_recalc_time,
      SystemState.total_assets AS total_assets,
      SystemState.stale_count AS stale_count;
```

Response format:
```json
{
  "state_id": "SYS001",
  "merkle_root": "8837429571023847561",
  "last_recalc_time": "2026-03-02T01:05:00",
  "total_assets": 63,
  "stale_count": 3
}
```

This endpoint is consumed by the UI to display the stale-asset badge on the Recalculate TTBs button (UI-REQ-112).

## 4B. Position-Aware TTB Computation

### ALG-REQ-050: Tactic Chain Definition

The system SHALL define tactic chains as graph objects in the GrDB, using `TacticChain` vertices (SCHEMA TA010) connected to `tMitreTactic` vertices via `chain_includes` edges (SCHEMA ED013). Each chain represents an ordered sequence of MITRE ATT&CK tactics that an attacker executes on a node depending on its position in the attack path.

**Chain vertices:**

| VID                    | chain_name     | Description                                                      |
|------------------------|----------------|------------------------------------------------------------------|
| `"CHAIN_ENTRANCE"`     | Entrance Chain | Tactics executed on the entry point (includes Initial Access)    |
| `"CHAIN_INTERMEDIATE"` | Regular Chain  | Tactics executed on intermediate nodes                           |
| `"CHAIN_TARGET"`       | Target Chain   | Tactics executed on the target (includes C2, Collection, Impact) |

**Tactic ordering** is encoded in the `chain_includes` edge rank (`@rank`). Rank 0 = first tactic in the chain, rank 1 = second, etc.

**Semantics:**

| Chain                | Position                | Distinctive Tactics                                                | Rationale                                                                                                         |
|----------------------|-------------------------|--------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------|
| `CHAIN_ENTRANCE`     | Entry point (N₀)        | Includes **TA0001** (Initial Access)                               | The attacker must first gain access to the entry point                                                            |
| `CHAIN_INTERMEDIATE` | Intermediate (N₁..Nₖ₋₁) | TA0002–TA0008 only                                                 | The attacker traverses intermediates via Lateral Movement (TA0008); no initial access or post-exploitation needed |
| `CHAIN_TARGET`       | Target (Nₖ)             | Adds **TA0011** (C2), **TA0009** (Collection), **TA0040** (Impact) | The attacker performs actions on objective at the target                                                          |

The chain data is loaded into the GrDB during initial deployment (see SCHEMA data load section). The APP layer reads chain definitions by traversing `chain_includes` edges — no external configuration file is required.

>Design note 1: `chains.json` is superseded by this requirement. It may be retained as a seed data / documentation artifact but is no longer loaded at APP runtime.

>Design note 2: Storing chains in the GrDB enables pure graph-traversal TTB computation (ALG-REQ-052) — the query starts from a chain vertex and follows edges through the MITRE subgraph without the APP layer needing to pass tactic lists as parameters. This moves computation closer to the data.

>Design note 3: Adding a new chain type (e.g. for a future "pivot point" position) requires only inserting a new TacticChain vertex and its chain_includes edges — no code changes. Chain definitions are queryable and editable via nGQL.

**Amended in:** v1.3 — chains moved from JSON config to GrDB graph objects.

>Note: The chances that this will be changing often are low.


### ALG-REQ-051: Chain Position Assignment Rule

When computing TTA for a set of discovered paths between entry point `E` and target `T`, the APP layer SHALL assign chain positions as follows:

Given a path `[N₀, N₁, N₂, ..., Nₖ]`:
- `N₀` (always equal to `E`) → **Entrance** → chain VID `"CHAIN_ENTRANCE"`
- `Nₖ` (always equal to `T`) → **Target** → chain VID `"CHAIN_TARGET"`
- All other nodes `N₁..Nₖ₋₁` → **Intermediate** → chain VID `"CHAIN_INTERMEDIATE"`

Position assignment is determined solely by the node's index in the specific path, **not** by the asset's `is_entrance` or `is_target` properties. The same physical asset may be an intermediate node in one path and an entry point in another (across different API calls with different `from`/`to` parameters).

The APP layer maps positions to chain VIDs using a simple lookup:

```go
func chainVIDForPosition(index, pathLength int) string {
    switch {
    case index == 0:
        return "CHAIN_ENTRANCE"
    case index == pathLength-1:
        return "CHAIN_TARGET"
    default:
        return "CHAIN_INTERMEDIATE"
    }
}
```

>Design note: The `is_entrance` and `is_target` asset properties (TA001) define which assets are *eligible* to serve as entry points or targets (used to populate dropdowns in UI-REQ-207). Chain position assignment (this requirement) defines the asset's *role* within a specific calculated path. These are distinct concepts.

**Amended in:** v1.3 — position now maps to chain VIDs in GrDB.


### ALG-REQ-052: Position-Aware TTB Query Template

When the full TTB computation replaces the stub (ALG-REQ-044), the TTB for a given `(asset, chain_position)` pair SHALL be computed by traversing from the appropriate `TacticChain` vertex through the MITRE subgraph in the database. The query follows the path: chain → tactics (via `chain_includes`) → techniques (via `part_of`) → mitigations (via `mitigates`) → asset-specific application (via `applied_to`).

The reference MATCH query:

```nGQL
MATCH (chain:TacticChain)-[s:chain_includes]->(tac:tMitreTactic)
  <-[:part_of]-(tech:tMitreTechnique)
WHERE id(chain) == "{chainVid}"
OPTIONAL MATCH (mit:tMitreMitigation)-[:mitigates]->(tech)
OPTIONAL MATCH (mit)-[app:applied_to]->(a:Asset)
WITH tac, s, tech, mit, app, a, rank(s) AS tactic_order
WHERE a IS NULL OR a.Asset.Asset_ID == "{assetId}"
RETURN
  tac.tMitreTactic.Tactic_ID AS tactic_id,
  tactic_order,
  tech.tMitreTechnique.Technique_ID AS tech_id,
  tech.tMitreTechnique.execution_min AS exec_min,
  tech.tMitreTechnique.execution_max AS exec_max,
  tech.tMitreTechnique.priority AS tech_priority,
  tech.tMitreTechnique.rcelpe AS vuln_applicable,
  mit.tMitreMitigation.Mitigation_ID AS mit_id,
  app.Maturity AS maturity,
  app.Active AS active
ORDER BY tactic_order, tech_id;
```

Where `{chainVid}` is one of `"CHAIN_ENTRANCE"`, `"CHAIN_INTERMEDIATE"`, or `"CHAIN_TARGET"` (per ALG-REQ-051), and `{assetId}` is the asset being evaluated.

Justification for MATCH syntax (per REQ-244 in SRS): traversing TacticChain → tMitreTactic → tMitreTechnique with tactic ordering via edge rank, then optionally joining through mitigates → applied_to to check asset-specific mitigation status, requires multi-hop OPTIONAL MATCH with property retrieval on intermediate edges. This has no practical nGQL/GO equivalent.

**nGQL notes:**

1. `WHERE` cannot follow `OPTIONAL MATCH` in NebulaGraph. The asset filter is applied after collecting all variables through `WITH`, using the pattern `WHERE a IS NULL OR a.Asset.Asset_ID == "{assetId}"`. This preserves OPTIONAL MATCH semantics: unmitigated techniques (where `a` is NULL) are kept; mitigations applied to *other* assets are filtered out.

2. `rank(s)` cannot be used directly in `ORDER BY`. It is piped through `WITH ... rank(s) AS tactic_order` and then referenced by alias.

3. The query returns one row per (technique, mitigation) pair under the given chain's tactics. Rows where `mit_id` / `maturity` / `active` are NULL indicate unmitigated techniques. The APP layer uses these rows to compute TTB according to the formula defined in ALG-REQ-020/021 (placeholders).

>Design note 1: This query is the foundation for the real TTB formula. The stub (ALG-REQ-044) does not use this query — it uses a simple arithmetic formula with the chain VID as parameter. Once ALG-REQ-020/021 are defined, this query replaces the stub.

>Design note 2: The `rcelpe` field on tMitreTechnique (TA008) indicates whether a technique can exploit a critical vulnerability. When the asset has `has_vulnerability == true`, techniques with `rcelpe == true` may receive reduced execution time in the TTB formula (future ALG-REQ-020).

>Design note 3: The `tactic_order` column (from edge rank) preserves the sequential order of tactics in the chain. While the current TTB stub does not use ordering, the future state-machine traversal through `tMitreState` / `patterns_to` will rely on it.

>Design note 4: For the PoC dataset (~200 techniques, ~43 mitigations), the chain filter narrows the technique set to the relevant subset per position. Expected execution time is well under 1 second.

**Amended in:** v1.3 — query traverses from chain vertex in GrDB; corrected nGQL syntax (WHERE/OPTIONAL MATCH, rank() aliasing).


### ALG-REQ-053: TTB Caching Strategy

The system SHALL use a split caching strategy for TTB values based on chain position:

**Intermediate-position TTB (persisted):**
- Stored in `Asset.TTB` (TA001) in the database
- Computed using `Regular_chain` (ALG-REQ-050)
- Protected by the hash/staleness system (ALG-REQ-040–043)
- Recalculated only when `hash_valid == false` (i.e. when the asset's intrinsic defenses change)
- Written back to the database after recalculation

**Entry-position and Target-position TTB (ephemeral):**
- Computed on-the-fly during path calculation (ALG-REQ-046, steps 5-6)
- Computed using `Entrance_chain` or `Target_chain` respectively (ALG-REQ-050)
- Held in APP-layer memory only — **never written to `Asset.TTB`**
- Discarded after the API response is returned
- Always recomputed on each path calculation request (no hash check)

**Rationale:**
- Intermediate is the most common position (most nodes in most paths) — caching yields the greatest benefit
- Entry and target are exactly 2 nodes per path-set — the cost of always recomputing them is negligible
- Not persisting entry/target TTB avoids contaminating `Asset.TTB` with position-specific values
- No hash invalidation is needed when the Path Inspector is closed or when the user switches to a different entry/target pair

**Optional future optimisation:** The APP layer MAY implement an in-memory LRU cache keyed by `(asset_id, chain_position, hash)`. If the asset's stored hash has not changed since the last computation, the cached entry/target TTB can be reused without re-querying. This is an implementation-level optimisation, not a requirement.


## 5. Mitigation Impact on TTA

> **Status:** This section is a placeholder for future algorithmic requirements. The requirements below are drafts to be refined as the mitigation-aware path calculation is developed.

### ALG-REQ-020: Mitigation-Aware TTA (Superseded)

_This placeholder is superseded by ALG-REQ-070 (TTB Calculation Algorithm) and its sub-requirements ALG-REQ-071–080 in Section 5B. Mitigations affect TTA through the TTT sub-calculation (ALG-REQ-060) invoked within the TTB loop._

### ALG-REQ-021: Mitigation Maturity Weighting (Superseded)

_This placeholder is superseded by ALG-REQ-060 (TTT Definition and Formula) in Section 5A, which defines how Maturity values (25, 50, 80, 100) are normalised and applied within the TTT interpolation formula._

### ALG-REQ-022: Recalculation Trigger (Superseded)

_This placeholder is superseded by the existing hash invalidation system (ALG-REQ-040–043) and the bulk/path-scoped recalculation flows (ALG-REQ-045/046). The TTB calculation algorithm (ALG-REQ-070) defines the computation itself; recalculation triggers remain as previously defined._

---

## Section 5A. TTT Calculation — Time to Execute a Technique

### ALG-REQ-060: TTT Definition and Formula

TTT (Time to Execute a Technique) SHALL be computed for a given `(asset, technique)` pair as a function of:
- The technique's intrinsic execution time boundaries (`execution_min`, `execution_max` — SCHEMA TA008)
- The set of mitigations that **can** mitigate this technique (regardless of whether they are applied to the asset)
- The subset of those mitigations that **are** applied to the asset (via `applied_to` edges — SCHEMA ED001)
- The **Maturity** and **Active** properties of each `applied_to` edge

**Formal definition:**

Given:
- `exec_min` = `tMitreTechnique.execution_min` (float, SCHEMA TA008, default 0.1667)
- `exec_max` = `tMitreTechnique.execution_max` (float, SCHEMA TA008, default 120)
- `P` = count of all mitigations that mitigate this technique (via `mitigates` edges, SCHEMA ED009) — the **possible mitigations** count
- `A` = count of mitigations from `P` that are applied to this asset (via `applied_to` edges, SCHEMA ED001) **and** have `Active == true` — the **active applied mitigations** count
- `M_i` = `Maturity` property (int, values: 25, 50, 80, 100) of the `applied_to` edge for the `i`-th active applied mitigation

The TTT formula has three cases:

**Case 1 — No possible mitigations** (`P == 0`):

    TTT = exec_min

No mitigations exist for this technique in the MITRE knowledge base. The attacker faces no defensive obstacles; execution proceeds at minimum time.

**Case 2 — All possible mitigations are active-applied** (`A == P` and `P > 0`):

    TTT = exec_max

Every known mitigation for this technique is deployed and active on this asset. The attacker faces maximum resistance; execution takes maximum time.

**Case 3 — Partial or no active-applied mitigations** (`0 ≤ A < P` and `P > 0`):

    TTT = exec_min + (Σ 0.01 × M_i for i = 1..A) × (exec_max − exec_min) / P

Where the summation is over all active applied mitigations. If `A == 0` (mitigations exist but none are applied or active), the summation is zero and `TTT = exec_min`.

**Units:** TTT is expressed in the same time unit as `execution_min` / `execution_max` (hours in the current dataset).

>Design note: to be clarified in the subsequent release. Actual units are hours, i.e. 0.1667 h is 10 minutes.

**Return type:** float (matching the type of `execution_min` and `execution_max` in SCHEMA TA008).

>Design note 1: The formula implements a **linear interpolation** between `exec_min` and `exec_max`. The interpolation weight is the ratio of accumulated maturity contribution to the maximum possible maturity contribution. With all mitigations at Maturity=100, the weight reaches 1.0 and TTT = `exec_max`. With no mitigations, the weight is 0.0 and TTT = `exec_min`.

>Design note 2: Case 2 (`A == P`) is a mathematical consequence of Case 3 only when all mitigations have `Maturity == 100`. The explicit `exec_max` assignment for Case 2 is a **design decision**: if all possible mitigations are applied and active (regardless of individual maturity levels), the technique is considered fully defended and assigned maximum execution time. This provides a ceiling guarantee.

>Design note 3: The `0.01` multiplier normalises the Maturity integer (25, 50, 80, 100) into a [0.0, 1.0] range fraction. For example, Maturity=80 contributes 0.8 to the maturity factor.


### ALG-REQ-061: TTT Boundary Conditions and Edge Cases

The TTT calculation SHALL handle the following boundary conditions:

| Condition                                                                     | `P` | `A` | Result                    | Rationale                                                                 |
|-------------------------------------------------------------------------------|-----|-----|---------------------------|---------------------------------------------------------------------------|
| No possible mitigations                                                       | 0   | 0   | `exec_min`                | No defenses exist in the MITRE knowledge base for this technique          |
| Possible mitigations exist, none applied                                      | >0  | 0   | `exec_min`                | Defenses exist but the organisation has not deployed them                 |
| Possible mitigations exist, some applied but all inactive (`Active == false`) | >0  | 0   | `exec_min`                | Applied mitigations with `Active == false` are excluded from count `A`    |
| All possible mitigations applied and active                                   | >0  | =P  | `exec_max`                | Full mitigation coverage; Case 2 ceiling                                  |
| Single mitigation, applied, active, Maturity=100                              | 1   | 1   | `exec_max`                | Case 2 applies (A == P)                                                   |
| Single mitigation, applied, active, Maturity=50                               | 1   | 1   | `exec_max`                | Case 2 applies (A == P), regardless of maturity                           |
| Multiple mitigations, all applied but mixed maturity                          | >1  | =P  | `exec_max`                | Case 2 applies (A == P), individual maturity irrelevant                   |
| exec_min == exec_max (degenerate technique)                                   | any | any | `exec_min` (= `exec_max`) | The range is zero; formula returns the constant regardless of mitigations |

**Constraint:** TTT SHALL always satisfy `exec_min ≤ TTT ≤ exec_max`.

>Design note: The boundary table above serves as a test specification. Each row maps directly to a unit test case for TTT computation.


### ALG-REQ-062: OS Platform Filtering for TTT

When computing TTT for a given `(asset, technique)` pair, the system SHALL first verify that the technique is **applicable** to the asset's operating system platform. Applicability is established via the graph traversal:

    Asset —[runs_on]→ OS_Type —[represents]→ MitrePlatform ←[can_be_executed_on]— tMitreTechnique

(SCHEMA: ED011, ED014, TA011, ED003, TA008)

If the technique cannot be executed on any platform represented by the asset's OS, the `(asset, technique)` pair is **invalid** — TTT is undefined and the technique SHALL be excluded from further processing.

>Design note 1: This filter is a **precondition** for TTT calculation. It is also used independently by the TTB calculation (see TTB flow, "FilterOS" step). The TTT reference query (ALG-REQ-064) incorporates this filter as the initial MATCH clause.

>Design note 2: There is no explicit "ANY_OS" platform vertex in the schema. A technique applicable to all platforms has individual `can_be_executed_on` edges to every `MitrePlatform` vertex, consistent with MITRE ATT&CK modelling. The traversal handles this naturally.

>Design note 3: The `represents` edge (ED014) bridges the CMDB-oriented OS_Type vocabulary (e.g., "Windows 11 Pro") to the MITRE-oriented MitrePlatform vocabulary (e.g., "Windows"). This mapping was introduced in SCHEMA v1.9.


### ALG-REQ-063: Mitigation Active/Inactive Handling in TTT

When counting applied mitigations for the TTT formula (ALG-REQ-060), the system SHALL inspect the `Active` property on the `applied_to` edge (SCHEMA ED001):

- If `Active == true`: the mitigation IS counted as an active applied mitigation. Its `Maturity` value contributes to the maturity factor summation.
- If `Active == false`: the mitigation is **not** counted as applied. Its `Maturity` value is **disregarded**. The `applied_to` edge relationship may exist in the graph (for audit/history purposes), but it has no effect on TTT.

The practical effect: a mitigation with `Active == false` is treated identically to a mitigation that has no `applied_to` edge to the asset at all, from the TTT formula's perspective.

>Design note 1: The `Active` flag enables "soft disable" of mitigations — e.g., an organisation may temporarily disable a mitigation for maintenance without deleting the relationship. The TTT formula reflects the actual operational state.
>Design note 2: The enabling/disabling of the mitigation is already implemented in mitigation editor (UI-REQ-250).


### ALG-REQ-064: TTT Reference nGQL Query

The TTT for a given `(asset, technique)` pair SHALL be computed using the following nGQL MATCH query, which consolidates all three cases from ALG-REQ-060 into a single query with proper division-by-zero handling:

```nGQL
MATCH (a:Asset)-[:runs_on]->(os:OS_Type)-[:represents]->(p:MitrePlatform)
      <-[:can_be_executed_on]-(t:tMitreTechnique)
WHERE id(a) == "{assetId}" AND id(t) == "{techniqueId}"
WITH a, t,
     t.tMitreTechnique.execution_min AS exec_min,
     t.tMitreTechnique.execution_max AS exec_max,
     t.tMitreTechnique.Technique_ID AS technique_id,
     t.tMitreTechnique.Technique_Name AS technique_name
OPTIONAL MATCH (t)<-[:mitigates]-(m_all:tMitreMitigation)
WITH a, t, exec_min, exec_max, technique_id, technique_name,
     count(m_all) AS possible_count
OPTIONAL MATCH (t)<-[:mitigates]-(m_applied:tMitreMitigation)-[ap:applied_to]->(a)
WITH technique_id,
     technique_name,
     exec_min,
     exec_max,
     possible_count,
     collect({maturity: ap.Maturity, active: ap.Active}) AS applied_raw
WITH technique_id,
     technique_name,
     exec_min,
     exec_max,
     possible_count,
     [x IN applied_raw WHERE x.active == true] AS active_mitigations
WITH technique_id,
     technique_name,
     exec_min,
     exec_max,
     possible_count,
     size(active_mitigations) AS applied_count,
     CASE WHEN possible_count == 0 THEN 0.0
          ELSE reduce(s = 0.0, x IN active_mitigations | s + 0.01 * x.maturity)
     END AS maturity_factor
RETURN technique_id,
       technique_name,
       applied_count,
       possible_count,
       exec_min,
       exec_max,
       CASE
         WHEN possible_count == 0 THEN exec_min
         WHEN applied_count == possible_count THEN exec_max
         ELSE exec_min + (maturity_factor * (exec_max - exec_min))
              / toFloat(possible_count)
       END AS TTT;
```

**Parameters:**
- `{assetId}` — VID of the Asset vertex (e.g., `"A00014"`)
- `{techniqueId}` — VID of the tMitreTechnique vertex (e.g., `"T1071.001"`)

**Query structure explanation:**

| Step | MATCH / WITH clause                                                           | Purpose                                                                                                                 |
|------|-------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------|
| 1    | `MATCH ... Asset → OS_Type → MitrePlatform ← tMitreTechnique`                 | OS platform filter (ALG-REQ-062). If no path exists, the query returns empty — TTT is undefined for this pair.          |
| 2    | `WITH a, t, exec_min, exec_max, ...`                                          | Extract technique's execution time boundaries.                                                                          |
| 3    | `OPTIONAL MATCH (t)<-[:mitigates]-(m_all)` → `count(m_all) AS possible_count` | Count **P** — all mitigations that can mitigate this technique (regardless of asset).                                   |
| 4    | `OPTIONAL MATCH (t)<-[:mitigates]-(m_applied)-[ap:applied_to]->(a)`           | Find mitigations that both mitigate the technique **and** are applied to this asset. Collect their edge properties.     |
| 5    | `[x IN applied_raw WHERE x.active == true]`                                   | Filter to active-only mitigations (ALG-REQ-063).                                                                        |
| 6    | `size(active_mitigations) AS applied_count`                                   | Count **A**.                                                                                                            |
| 7    | `reduce(s = 0.0, ...)`                                                        | Compute maturity factor **Σ(0.01 × M_i)**. Guarded by `CASE WHEN possible_count == 0` to avoid meaningless computation. |
| 8    | Final `CASE` in `RETURN`                                                      | Apply the three-case formula from ALG-REQ-060.                                                                          |

**nGQL syntax notes:**

1. `WHERE` cannot follow `OPTIONAL MATCH` in NebulaGraph 3.8. The asset-filtering for applied mitigations is handled by the MATCH pattern itself (`-[ap:applied_to]->(a)`) where `a` is already bound from the initial MATCH clause. This is semantically equivalent to a WHERE filter but does not violate the nGQL grammar constraint.

2. `toFloat(possible_count)` ensures floating-point division in the Case 3 formula. Without it, integer division could truncate the result.

3. The `reduce()` list comprehension iterates over the `active_mitigations` list (already filtered to `active == true`) and sums the normalised maturity values. If the list is empty, `reduce` returns the seed value `0.0`.

4. Both `OPTIONAL MATCH` clauses are necessary: the first counts all mitigations for the technique globally; the second counts only those applied to the specific asset. These are distinct sets — a technique may have 5 possible mitigations, of which only 2 are applied to the asset in question.
5. The `OPTIONAL MATCH` for `mitigates` relationships is retained because a technique MAY have zero mitigations. The `MATCH` clause for OS platform filtering (`runs_on → represents → can_be_executed_on`) is already a plain MATCH and is correct — it serves as a precondition filter per ALG-REQ-062.


>Design note 1: The query is self-contained — it performs OS platform validation, mitigation counting, and TTT computation in a single database round-trip. The APP layer receives the final TTT value directly.

>Design note 2: If the query returns an empty result set (no rows), this means the technique is not applicable to the asset's platform. The APP layer SHALL treat this as "TTT undefined for this (asset, technique) pair" and exclude the technique from TTB processing.

>Design note 3: This query is designed to be called from within the TTB calculation loop (future ALG-REQ-020 replacement). In that context, the `{techniqueId}` is the technique selected by the TTB algorithm's priority/pattern logic, and `{assetId}` is the asset being evaluated.

**Justification for MATCH syntax** (per REQ-244 in SRS): The query requires multi-hop traversal through the OS → Platform → Technique path for platform filtering, OPTIONAL MATCH for mitigation counting with asset-specific filtering, list comprehension for Active-flag filtering, and `reduce()` for maturity accumulation. This combination has no practical nGQL/GO equivalent.


### ALG-REQ-065: TTT Output Contract

The TTT calculation (whether performed via ALG-REQ-064 query or equivalent APP-layer logic) SHALL return the following data for a given `(asset, technique)` pair:

| Field            | Type   | Description                                           |
|------------------|--------|-------------------------------------------------------|
| `technique_id`   | string | MITRE Technique/Subtechnique ID (e.g., `"T1071.001"`) |
| `technique_name` | string | Human-readable technique name                         |
| `applied_count`  | int    | Number of active applied mitigations (`A`)            |
| `possible_count` | int    | Number of possible mitigations (`P`)                  |
| `exec_min`       | float  | Technique's minimum execution time                    |
| `exec_max`       | float  | Technique's maximum execution time                    |
| `TTT`            | float  | Computed Time to Execute a Technique                  |

This output is consumed by the TTB calculation algorithm (future ALG-REQ-020), specifically at the step where TTT is calculated for each technique in the SelectedTechniques set.

The output is **not** exposed as a standalone API endpoint in the current design. It is an internal computation result passed within the TTB calculation pipeline. A diagnostic/debug endpoint MAY be added in a future version.

>Design note: The `applied_count` and `possible_count` fields are included for transparency and debugging. They allow the operator to understand *why* a particular TTT value was computed — e.g., "3 out of 5 possible mitigations are active on this asset for this technique."


### ALG-REQ-066: TTT and ALG-REQ-052 Relationship

ALG-REQ-064 (TTT reference query) SHALL be considered the **implementation** of the TTT sub-component within the broader TTB query template defined in ALG-REQ-052.

The relationship between the two requirements is:

- ALG-REQ-052 defines the overall TTB data retrieval query — it traverses `TacticChain → tMitreTactic → tMitreTechnique` and collects technique/mitigation data for all tactics in a chain.
- ALG-REQ-064 defines the **per-technique TTT formula** — given one technique and one asset, it computes the execution time considering mitigations.

When the full TTB formula (ALG-REQ-020 replacement) is implemented:
1. The TTB algorithm iterates through tactics in the chain (per ALG-REQ-052 query results or equivalent logic).
2. For each tactic, it selects the relevant techniques (filtered by OS, priority, vulnerability — per TTB flow).
3. For each selected technique, it computes TTT using the formula from ALG-REQ-060 / query from ALG-REQ-064.
4. The technique with the **minimum TTT** is chosen (the "fastest technique" — the attacker's optimal choice).
5. That minimum TTT is added to the running TTB sum.

The APP layer MAY choose to:
- **(a)** Execute ALG-REQ-064 as a standalone query per technique (simple, clear, multiple round-trips), or
- **(b)** Embed the TTT formula logic directly into the ALG-REQ-052 query (complex, single round-trip, better performance)

For the PoC, option **(a)** is recommended for clarity and debuggability. Option **(b)** is a performance optimisation for future versions.

>Design note: Once TTB flow requirements are defined (next update to ALGO), the exact integration point between ALG-REQ-064 and the TTB loop will be formalised. At that stage, ALG-REQ-052 may be amended to incorporate inline TTT computation.

---


## Section 5B. TTB Calculation — Time To Bypass

### ALG-REQ-070: TTB Calculation Algorithm

TTB (Time To Bypass) for a given `(asset, chain)` pair SHALL be computed by iterating through the tactics in the specified tactic chain and, for each tactic, selecting the fastest applicable technique. The algorithm is a sequential loop that models the attacker's progression through MITRE ATT&CK tactics on a single host.

**Inputs:**
- `assetId` — VID of the Asset vertex being evaluated
- `chainVid` — VID of the TacticChain vertex (`"CHAIN_ENTRANCE"`, `"CHAIN_INTERMEDIATE"`, or `"CHAIN_TARGET"` — per ALG-REQ-050/051)
- `orientationTime` — Orientation Time parameter (ALG-REQ-071)
- `switchoverTime` — Switchover Time parameter (ALG-REQ-072)
- `priorityTolerance` — Priority Tolerance parameter (ALG-REQ-075)

**Output:**
- `TTB` — float, total Time To Bypass in the same units as TTT (hours in the current dataset)
- `log` — ordered list of `(tactic_id, technique_id, TTT)` tuples representing the attacker's chosen path through the tactic chain (ALG-REQ-079)

**Algorithm (pseudocode):**

```text
function computeTTB(assetId, chainVid, orientationTime, switchoverTime, priorityTolerance):
    tactics = getOrderedTactics(chainVid)          // ALG-REQ-050, ordered by chain_includes rank
    TTB = orientationTime                           // ALG-REQ-071
    log = []
    previousTactic = NULL
    fastestTechnique = NULL

    for i = 0 to len(tactics) - 1:
        currentTactic = tactics[i]

        // Step 1: Select candidate techniques
        if i == 0:
            // First tactic: select all techniques for this tactic applicable to asset's OS
            candidates = selectFirstTacticTechniques(assetId, currentTactic)    // ALG-REQ-073
        else:
            // Subsequent tactics: select via pattern transition from previous state
            candidates = selectPatternTechniques(previousTactic, fastestTechnique, currentTactic)  // ALG-REQ-076
            // Apply OS filter to pattern-derived candidates
            candidates = filterByOS(candidates, assetId)                        // ALG-REQ-062

        // Step 2: Vulnerability filter
        if asset.has_vulnerability == true:
            vulnCandidates = filter(candidates, WHERE rcelpe == true)           // ALG-REQ-074
            if len(vulnCandidates) > 0:
                candidates = vulnCandidates
            // If no rcelpe techniques exist, fall through with unfiltered candidates
        
        // Step 3: Priority filter
        candidates = filterByPriority(candidates, priorityTolerance)            // ALG-REQ-075

        // Step 4: Empty set guard
        if len(candidates) == 0:
            // ALG-REQ-080: no techniques available for this tactic
            // No TTT and no switchover added — attacker skips this tactic entirely
            log.append((currentTactic, NULL, 0))
            previousTactic = currentTactic
            fastestTechnique = NULL
            continue                                // Skip to next tactic

        // Step 5: Compute TTT for each candidate
        for each tech in candidates:
            tech.TTT = computeTTT(assetId, tech.id)                             // ALG-REQ-060/064

        // Step 6: Select fastest technique (minimum TTT)
        fastestTechnique = selectFastest(candidates)                            // ALG-REQ-077

        // Step 7: Accumulate TTB
        // Switchover is added BEFORE each technique except the first one (ALG-REQ-072/078)
        if len(log) > 0:                                                        // at least one technique already logged
            TTB = TTB + switchoverTime
        TTB = TTB + fastestTechnique.TTT                                        // ALG-REQ-078

        // Step 8: Log the result
        log.append((currentTactic, fastestTechnique.id, fastestTechnique.TTT))  // ALG-REQ-079

        previousTactic = currentTactic

    return TTB, log
```

**Go function signature:**

```go
func computeTTB(assetId string, chainVid string, params TTBParams) (float64, []TTBLogEntry, error)
```

where `TTBParams` is a struct containing `OrientationTime`, `SwitchoverTime`, and `PriorityTolerance`.

>Design note 1: The algorithm is implemented in the APP layer (Go code), not as a single GrDB query. The loop structure with conditional branching (vulnerability filter, pattern transitions, empty-set handling) is not expressible in nGQL. Individual steps (technique selection, TTT computation) use nGQL queries.

>Design note 2: The algorithm supersedes the stub defined in ALG-REQ-044. The stub's function signature `computeTTBStub(currentTTB int, chainVID string) int` is replaced by the richer signature above. The return type changes from `int` to `float64` because TTT values are floats (execution_min/execution_max in TA008 are float).

>Design note 3: The `log` output is critical for transparency. It records exactly which technique the attacker would use at each tactic step, enabling the operator to understand the TTB composition and identify which tactics/techniques are the weakest links.

>Design note 4: The algorithm does NOT modify the asset's `has_vulnerability` flag or any graph data. It is a pure read-only computation that produces a TTB value and an audit log.


### ALG-REQ-071: Orientation Time Parameter

The TTB calculation SHALL begin by initialising the TTB accumulator with the **Orientation Time** — a configurable parameter representing the time an attacker spends on initial reconnaissance of a host before executing any techniques.

**Default value:** 0.25 hours (15 minutes)  
**Type:** float  
**Valid range:** 0.0 – 24.0 hours (0 to 1440 minutes)  
**Storage:** APP-layer parameter, passed to `computeTTB()`. In the PoC, this SHALL be an editable field in the Path Inspector UI panel.

The Orientation Time is added **once** per TTB calculation (not once per tactic). It represents a fixed overhead regardless of the number of tactics in the chain.

>Design note 1: The value should be stateful — persisted in `static/state.js` or equivalent client-side storage for the PoC. A future version may store it in a dedicated configuration table in the database or a separate relational store.

>Design note 2: This parameter affects ALL TTB calculations equally (entrance, intermediate, target). A future enhancement could differentiate orientation time by chain position (e.g., the attacker may spend more time orienting on initial access vs. lateral movement). For the PoC, a single global value is sufficient.

>Design note 3: Setting Orientation Time to 0 effectively removes the reconnaissance overhead — useful for modelling scenarios where the attacker has prior knowledge of the target environment.

>Design note 4: Further storage decisions may be influenced by architectural changes - i.e. if for various reasons (including making the stateful application) parameters like Orientation Time will be stored in the auxiliary relational database like MariaDB.


### ALG-REQ-072: Switchover Time Parameter

>Note For each iteration between techniques in the TTB loop, the system SHALL add a **Switchover Time** — a configurable parameter representing the overhead time an attacker incurs when transitioning between techniques across tactic steps. (The parameter has no relationship to the tactic itself - from that parameter's standpoint it exists only to designate the switchover between techniques/subtechniques, whether they belong to the same tactic or not. We simply add this after each TTT calculation, except this is the last technique). So,this parameter is added only for transitions and not after the last technique. I.e. if we have two tactics TA1 and TA2 with the respectful techniques T001, T002, T003 (for TA1) and T004 and T005 (for TA2),  and  T002 and T005 are the fastest for their respectful tactics the TTB calculation should follow this logic (see ALG-REQ-078 for more details):
 ``` example
 TTB = orientation  + TTT(T002) + switchover + TTT(T005)
```
>This (switchover between techniques rather than tactics) is done so (theoretically) we will be able to use two techniques per tactic if the need arises. Not now, at any rate.

**Default value:** 0.1667 hours (10 minutes)  
**Type:** float  
**Valid range:** 0.0 – 24.0 hours (0 to 1440 minutes)  
**Storage:** APP-layer parameter, passed to `computeTTB()`. In the PoC, this SHALL be an editable field in the Path Inspector UI panel.

The Switchover Time is added **once per technique** in the chain, except for the last one. For a chain with total `K` techniques/subtechniques, the total switchover contribution is `(K-1) × switchoverTime`.

>Design note 1: Same statefulness requirements as Orientation Time (ALG-REQ-071).

>Design note 2: The Switchover Time models the attacker's overhead when pivoting from one technique to the next, whether they (techniques/subtechniques) have the same parent tactic or not.

>Design note 3: The Switchover Time is NOT added when the empty-set guard (ALG-REQ-080) triggers — since no technique is executed, there is no "switchover" between techniques. Empty-set tactics contribute nothing to TTB.


### ALG-REQ-073: First-Tactic Technique Selection

For the **first tactic** in the chain (loop iteration `i == 0`), the candidate technique set SHALL be obtained by selecting all techniques/subtechniques that:

1. Belong to the current tactic (via `part_of` edge, SCHEMA ED010)
2. Are applicable to the asset's operating system platform (via `Asset → runs_on → OS_Type → represents → MitrePlatform ← can_be_executed_on ← tMitreTechnique`, per ALG-REQ-062)

This is the "initial population" of techniques — no pattern-based filtering applies because there is no previous tactic/technique state.

**Reference nGQL query:**

```nGQL
MATCH (a:Asset)-[:runs_on]->(os:OS_Type)-[:represents]->(p:MitrePlatform)
      <-[:can_be_executed_on]-(t:tMitreTechnique)-[:part_of]->(tac:tMitreTactic)
WHERE id(a) == "{assetId}" AND id(tac) == "{tacticId}"
WITH collect({
  tid: t.tMitreTechnique.Technique_ID,
  tname: t.tMitreTechnique.Technique_Name,
  pri: t.tMitreTechnique.priority,
  rcelpe: t.tMitreTechnique.rcelpe,
  plat: p.MitrePlatform.platform_name
}) AS rows
UNWIND rows AS r
RETURN DISTINCT r.tid AS technique_id,
       r.tname AS technique_name,
       r.pri AS technique_priority,
       r.rcelpe AS vuln_applicable
ORDER BY technique_priority DESC, technique_id;
```

**Parameters:**
- `{assetId}` — VID of the Asset vertex (e.g., `"A00014"`)
- `{tacticId}` — VID of the tMitreTactic vertex (e.g., `"TA0002"`)

**nGQL notes:**

1. The query combines the OS platform filter (ALG-REQ-062) with the tactic membership check (`part_of`) in a single traversal. The `collect() → UNWIND → DISTINCT` pattern deduplicates techniques that match multiple platforms for the same asset (e.g., a technique applicable to both "Windows" and "Linux" when the asset's OS maps to one of them).

2. The query returns `rcelpe` for subsequent vulnerability filtering (ALG-REQ-074) and `priority` for priority filtering (ALG-REQ-075). These filters are applied in the APP layer, not in the query, to keep the query simple and the filter logic explicit.

>Design note 1: This query is functionally similar to Query A from the TTB flow PDF, but simplified: priority filtering is deferred to ALG-REQ-075 rather than embedded in the query. This separation of concerns makes each step independently testable.
>Design note 2: The filtering mentioned in step 2 may be later turned into a database (GrDB) query. 


### ALG-REQ-074: Vulnerability Filtering

After technique selection (ALG-REQ-073 or ALG-REQ-076) and before priority filtering (ALG-REQ-075), the system SHALL apply a **vulnerability filter** when the asset has a known critical vulnerability:

**Rule:**
- If `Asset.has_vulnerability == true` (SCHEMA TA001):
   - Filter the candidate set to include **only** techniques where `tMitreTechnique.rcelpe == true` (SCHEMA TA008)
   - If the filtered set is **non-empty**, it becomes the new candidate set
   - If the filtered set is **empty** (no vulnerability-exploiting techniques for this tactic), retain the **unfiltered** candidate set — the attacker falls back to non-vulnerability-specific techniques
- If `Asset.has_vulnerability == false`:
   - No filtering is applied; the full candidate set proceeds to priority filtering

**Rationale:** An attacker with knowledge of a critical vulnerability will preferentially use techniques that exploit it (`rcelpe == true`), as these are expected to have lower execution times. The fallback to the unfiltered set ensures that the presence of a vulnerability never *removes* attack options — it only *narrows* the preferred set.

>Design note 1: The `rcelpe` field name stands for "recipe" (a technique that has a known "recipe" for exploiting a vulnerability on the host). The field was introduced in TA008 and referenced in ALG-REQ-052 design note 2.

>Design note 2: The vulnerability filter is applied **before** priority filtering. This ordering matters: a high-priority non-vulnerability technique might be filtered out in favour of a lower-priority vulnerability-specific technique. This models the attacker's preference for exploiting known weaknesses even if other, ostensibly more common, techniques exist.

>Design note 3: The has_vulnerability flag is a binary signal — either a critical vulnerability exists or it doesn't. A future enhancement could introduce vulnerability severity levels or specific CVE mappings, but this is out of scope for the PoC.


### ALG-REQ-075: Priority Selection and Tolerance

After vulnerability filtering (ALG-REQ-074), the candidate technique set SHALL be filtered by **priority** using the following rule:

1. Determine the **maximum priority** value (`max_pri`) in the current candidate set. Priority values range from 1 (lowest) to 4 (highest) — per SCHEMA TA008.
2. Retain only techniques where `priority >= max_pri - priorityTolerance`

**Priority Tolerance parameter:**
- **Default value:** 1
- **Type:** int
- **Valid range:** 0 – 3
- **Storage:** APP-layer parameter, passed to `computeTTB()`. In the PoC, this MAY be an editable field in the Path Inspector UI panel or hardcoded to the default value.

**Effect of tolerance values:**

| Tolerance | Effect                           | Example (if max_pri=4)               |
|-----------|----------------------------------|--------------------------------------|
| 0         | Only highest-priority techniques | priority >= 4 (only priority 4)      |
| 1         | Top two priority levels          | priority >= 3 (priority 3 and 4)     |
| 2         | Top three priority levels        | priority >= 2 (priority 2, 3, and 4) |
| 3         | All techniques (no filtering)    | priority >= 1 (all)                  |

**Rationale:** An attacker generally prefers the most commonly used (highest-priority) techniques, but may also consider less common techniques if they offer faster execution. The tolerance parameter controls the trade-off between realism (attacker sticks to known methods) and optimality (attacker considers all options).

>Design note 1: The PDF's Query A uses `WHERE r.pri >= max_pri - 1 AND r.pri >= 1`, which corresponds to tolerance=1 with a floor at priority 1. The `AND r.pri >= 1` guard is implicit in the valid range (priority is int8 with minimum 1 in TA008), so it need not be added explicitly.

>Design note 2: For the PoC, hardcoding tolerance=1 is acceptable. Exposing it as a UI parameter adds flexibility but is not critical for the initial implementation.


### ALG-REQ-076: Pattern-Based Technique Selection

For **subsequent tactics** in the chain (loop iteration `i > 0`), the candidate technique set SHALL be obtained via the **pattern transition** mechanism using `tMitreState` vertices and `patterns_to` edges:

**Algorithm:**

1. Construct the state ID from the previous tactic and the fastest technique: `stateId = "{previousTacticId}|{fastestTechniqueId}"` (e.g., `"TA0002|T1059"`)
2. Find the `tMitreState` vertex with VID matching `stateId` (SCHEMA TA006)
3. Traverse `patterns_to` edges from `tMitreState` to find destination states (SCHEMA ED012)
4. From the destination states, extract only those whose tactic component matches `currentTacticId`
5. Extract the technique IDs from the matching destination states

**Reference nGQL query:**

```nGQL
MATCH (src_state:tMitreState)-[:patterns_to]->(dst_state:tMitreState)
WHERE id(src_state) == "{previousTacticId}|{fastestTechniqueId}"
WITH dst_state.tMitreState.state_id AS dst_id
WITH dst_id,
     split(dst_id, "|") AS parts
WHERE size(parts) == 2 AND parts[0] == "{currentTacticId}"
WITH parts[1] AS technique_vid
MATCH (t:tMitreTechnique)
WHERE id(t) == technique_vid
RETURN t.tMitreTechnique.Technique_ID AS technique_id,
       t.tMitreTechnique.Technique_Name AS technique_name,
       t.tMitreTechnique.priority AS technique_priority,
       t.tMitreTechnique.rcelpe AS vuln_applicable
ORDER BY technique_priority DESC, technique_id;
```

**Parameters:**
- `{previousTacticId}` — Tactic_ID of the previous tactic (e.g., `"TA0002"`)
- `{fastestTechniqueId}` — Technique_ID of the fastest technique from the previous tactic (e.g., `"T1059"`)
- `{currentTacticId}` — Tactic_ID of the current tactic being evaluated (e.g., `"TA0003"`)

**Semantics:** The `tMitreState` graph encodes observed attack patterns — which (tactic, technique) combinations transition to which other combinations. By following `patterns_to` edges, the algorithm respects realistic attack sequencing rather than assuming any technique can follow any other.

After obtaining the pattern-derived candidates, the OS filter (ALG-REQ-062) is applied because pattern transitions do not guarantee OS compatibility.

>Design note 1: The `patterns_to` edges are populated externally (SCHEMA ED012 notes). The quality of the TTB calculation is directly dependent on the completeness and accuracy of these pattern edges. Missing patterns may result in empty candidate sets (handled by ALG-REQ-080).

>Design note 2: The `split()` function in NebulaGraph 3.8 returns a list of strings. The `size(parts) == 2` guard protects against malformed state IDs.

>Design note 3: If `fastestTechnique` from the previous iteration is NULL (because ALG-REQ-080 triggered — no techniques were available), the pattern transition cannot be computed. In this case, the system SHALL fall back to the first-tactic selection method (ALG-REQ-073) for the current tactic. This ensures the loop can continue even after an empty-set tactic.

>Design note 4: The `patterns_to` edge has a `probability` property (SCHEMA ED012) that is not currently used. A future enhancement could weight technique selection by transition probability rather than purely by TTT speed.


### ALG-REQ-077: Fastest Technique Selection

After all filtering steps (OS, vulnerability, priority) and TTT computation for each remaining candidate, the system SHALL select the **fastest technique** — the candidate with the **minimum TTT** value.

**Rule:** `fastestTechnique = argmin(TTT)` over the filtered candidate set.

**Tie-breaking:** If multiple techniques share the same minimum TTT value, the system SHALL select the technique with the **highest priority** value. If still tied, the technique with the **lexicographically smallest Technique_ID** SHALL be chosen. This ensures deterministic results.

**The selected fastest technique serves two purposes:**
1. Its TTT is added to the TTB accumulator (ALG-REQ-078)
2. Its Technique_ID is used to construct the pattern state for the next tactic iteration (ALG-REQ-076)

>Design note: The "fastest technique" models the attacker's optimal behaviour — a rational attacker will choose the technique that achieves the tactic objective in the least time. This is a worst-case (for the defender) assumption.


### ALG-REQ-078: TTB Accumulation Formula

The TTB value SHALL be accumulated across tactic iterations using the following formula. Per ALG-REQ-072, Switchover Time is added **between** techniques — i.e., before each technique except the first one. There is no switchover after the last technique.

**Initialisation:**

    TTB = orientationTime

**Per-tactic iteration (for each tactic `k` in the chain where a technique was selected):**

    if k is the first tactic producing a technique:
        TTB = TTB + TTT_k
    else:
        TTB = TTB + switchoverTime + TTT_k

Where:
- `switchoverTime` = Switchover Time parameter (ALG-REQ-072)
- `TTT_k` = TTT of the fastest technique for tactic `k` (ALG-REQ-077)

**Expanded formula for `K` tactics that produced techniques (i.e., excluding empty-set tactics):**

    TTB = orientationTime + Σ TTT_k for k = 0..K-1 + (K-1) × switchoverTime

Equivalently, matching the example from ALG-REQ-072:

    TTB = orientationTime + TTT_0 + switchoverTime + TTT_1 + ... + switchoverTime + TTT_{K-1}

For a chain with 2 selected techniques (T002 and T005):

    TTB = orientation + TTT(T002) + switchover + TTT(T005)

**For tactics where the empty-set guard triggered (ALG-REQ-080):**

Neither Switchover Time nor TTT is added — the attacker skips the tactic entirely. Empty-set tactics do NOT count toward `K` in the formula above.

**Return type:** float (same units as TTT — hours in the current dataset).

>Design note 1: The old `Asset.TTB` was int32 (SCHEMA TA001, default 10). The new TTB is float. This requires a schema change to `Asset.TTB` type — **however**, this is deferred to a future SCHEMA update to minimise simultaneous changes. In the interim, the APP layer SHALL round the computed float TTB to int32 when storing to `Asset.TTB`. The full-precision float is used in memory for TTA computation.

>Design note 2: For a typical intermediate chain (7 tactics, all producing techniques), the minimum possible TTB is: `0.25 + 7 × 0.1667 + 6 × 0.1667 ≈ 2.42 hours`. The maximum possible TTB depends on mitigation coverage and execution_max values. This is a significant increase from the previous default TTB of 10 (which had no defined units).

>Design note 3: The "first technique producing a technique" check in the pseudocode (ALG-REQ-070, Step 7) is implemented by testing whether the log already contains a successful technique entry. This naturally handles the case where the first tactic(s) in the chain hit the empty-set guard — the first non-empty tactic will still be treated as "first technique" with no preceding switchover.


### ALG-REQ-079: TTB Calculation Log

The TTB calculation SHALL produce an ordered log (audit trail) recording the attacker's simulated progression through the tactic chain. Each entry in the log SHALL contain:

| Field              | Type           | Description                                                                        |
|--------------------|----------------|------------------------------------------------------------------------------------|
| `tactic_id`        | string         | Tactic_ID of the chain step (e.g., `"TA0002"`)                                     |
| `tactic_name`      | string         | Human-readable tactic name                                                         |
| `technique_id`     | string or null | Technique_ID of the fastest technique selected, or null if empty-set (ALG-REQ-080) |
| `technique_name`   | string or null | Human-readable technique name, or null                                             |
| `TTT`              | float          | Computed TTT for the selected technique, or 0.0 if empty-set                       |
| `candidates_count` | int            | Number of candidate techniques after all filtering, before TTT computation         |

The log is ordered by tactic chain sequence (matching the chain_includes edge ranks).

**Purpose:** The log enables:
- Debugging TTB calculations during development
- Understanding which techniques drive the TTB value (weakest links)
- Future UI visualisation of the attacker's path through tactics on a single host
- Validation that pattern transitions are working correctly

**Storage:** The log is returned as part of the TTB computation result. For stored (intermediate) TTB values, the log is **not persisted** in the database — it is ephemeral. A future enhancement MAY persist logs for reporting purposes.

>Design note: The `candidates_count` field helps identify tactics where filtering was too aggressive (count = 1, single technique dominates) vs. too permissive (count = 50, many options). This informs tuning of the priority tolerance parameter.


### ALG-REQ-080: Empty Technique Set Handling

If at any point in the TTB loop the candidate technique set becomes empty (after OS filtering, vulnerability filtering, and priority filtering), the system SHALL handle the situation as follows:

**For pattern-based selection (ALG-REQ-076):**
1. If the pattern transition yields no candidates for the current tactic, **fall back** to first-tactic selection (ALG-REQ-073) — i.e., select all techniques for the current tactic that match the asset's OS, ignoring pattern constraints.
2. Apply vulnerability filtering (ALG-REQ-074) and priority filtering (ALG-REQ-075) to the fallback set.

**If the set is STILL empty after fallback:**
1. Log the empty tactic step with `technique_id = null` and `TTT = 0.0` (ALG-REQ-079).
2. Add **nothing** to TTB — neither Switchover Time nor TTT. The attacker skips this tactic entirely (per ALG-REQ-078).
3. Set `fastestTechnique = NULL` for this iteration.
4. Proceed to the next tactic. If the next tactic uses pattern-based selection (ALG-REQ-076), the NULL fastestTechnique triggers a fallback to first-tactic selection for that tactic as well (ALG-REQ-076 design note 3).

**Rationale:** An empty technique set means the MITRE data does not contain applicable techniques for this tactic on this asset's platform, or the pattern graph has a gap. The attacker effectively "skips" this tactic. This is not an error — it reflects a limitation in the MITRE data coverage or pattern completeness. The algorithm continues rather than aborting.

>Design note 1: In practice, empty technique sets are most likely to occur due to incomplete `patterns_to` edges (the pattern data is externally generated and may not cover all transitions). The fallback mechanism ensures robustness.

>Design note 2: The empty-set event SHOULD be logged at warning level in the APP layer, as it may indicate data quality issues that should be addressed (if they can).

>Design note 3: -it is that not all the techniques are observed so frequently that they can be reliably represented as patterns. 

---


## 6. Edge Cases and Constraints

### ALG-REQ-030: No Path Exists

When no loop-free directed path exists between the selected entry point and target within the specified hop limit, the API SHALL return an empty `paths` array with `total: 0`. The APP layer SHALL NOT treat this as an error condition.

```json
{
  "paths": [],
  "entry_point": "A00013",
  "target": "A00011",
  "hops": 6,
  "total": 0
}
```

### ALG-REQ-031: Loop Prevention

The path discovery query SHALL enforce loop-free paths by ensuring no node appears more than once in any single path. This is implemented via the `ALL(n IN nodes(p) WHERE single(m IN nodes(p) WHERE m == n))` predicate in the MATCH query (ALG-REQ-001).

### ALG-REQ-032: Performance Bound

Path calculation queries SHALL complete within 5 seconds (per CNST003 in SRS). If the graph topology or hop limit produces excessive combinatorial paths, the APP layer SHOULD log a warning. No timeout-based truncation is required for v1.0.

### ALG-REQ-033: Hop Limit Validation

The `hops` parameter SHALL be validated as an integer in the range 2–9. Values outside this range SHALL result in an HTTP 400 response. If the parameter is omitted, the default value of 6 SHALL be used.

---

## 7. Future Extensions

The following capabilities are anticipated but out of scope for v1.0:

- [x] ~~Mitigation-aware TTA calculation (ALG-REQ-020 through ALG-REQ-022)~~ — partially addressed: tactic chain framework in place (ALG-REQ-050–053); TTB formula itself still pending (ALG-REQ-020/021)
- [ ] Path probability scoring (likelihood-weighted TTA)
- [ ] Risk-weighted paths (incorporating asset priority)
- [ ] Multi-target analysis (single entry point, multiple targets)
- [ ] Mitigation impact simulation ("what-if" recalculation)
- [ ] Path comparison (before/after mitigation changes)
- [x] ~~TTB recalculation based on vulnerability presence (`has_vulnerability`)~~ — addressed in ALG-REQ-052 via `rcelpe` technique filter
- [x] ~~Full TTT formula implementation (ALG-REQ-060–066) defining per-technique execution time considering mitigations~~ — addressed in v1.4
- [x] ~~Full TTB formula implementation (ALG-REQ-020/021) integrating TTT results with tactic chain traversal, pattern transitions, priority selection, and orientation/switchover time parameters~~ — addressed in v1.5 (ALG-REQ-070–080)
- [ ] Embedded TTT computation within ALG-REQ-052 query for single-round-trip TTB calculation (ALG-REQ-066 option b)
- [ ] Dynamic tactic chain configuration via database instead of `chains.json`
- [ ] APP-layer TTB cache for entry/target positions (ALG-REQ-053 optional optimisation)
- [ ] `Asset.TTB` schema type change from int32 to float (required for full-precision TTB storage; ALG-REQ-078 design note 1)
- [ ] Update ALG-REQ-045/046 to reference ALG-REQ-070 instead of ALG-REQ-044 stub (coordinated SRS/UIS update)
- [ ] Persist TTB calculation logs for reporting (ALG-REQ-079 future enhancement) — MariaDB schema ready (ADR-REQ-012, ADR-REQ-013), instrumentation pending
- [ ] Weighted technique selection using `patterns_to.probability` (ALG-REQ-076 design note 4)
- [ ] Position-differentiated Orientation Time (ALG-REQ-071 design note 2)
- [x] ~~UI controls for Orientation Time, Switchover Time, and Priority Tolerance in Path Inspector~~ — addressed in UI-REQ-2091, implemented in v1.14 sprint
- [x] ~~Adding small relational database (like MariaDB) to keep configuration and calculation results in table format~~ — stub implemented: `internal/store/` package with schema migrations, connection pool, FlushBatch, and cache invalidation. See ADR-Requirements.md v0.1.
- [ ] **Batch ComputeTTT** — consolidate per-technique TTT queries into a single batch query per tactic, reducing ~90 DB round-trips per ComputeTTB call to ~10 (observed 10.2s for entry+target TTB on 25-asset graph; ALG-REQ-064 design note)
- [ ] **Re-unify ComputeTTT query** — the current two-query split (technique+P in q1, applied mitigations in q2) was necessitated by nGQL 3.8's prohibition of WHERE after OPTIONAL MATCH. Investigate WITH...WHERE pattern to restore single-query execution (would halve TTT query count)
- [x] ~~UI controls for Orientation Time, Switchover Time, and Priority Tolerance in Path Inspector~~ — addressed in UI-REQ-2091, implemented in v1.14 sprint


---

## 8. Cross-Reference Matrix

>Note: two sections - 8.1a - older migrated requirements, 8.1b - newly created ones. To be merged in the next version.

### 8.1a ALG-REQ to SRS (Requirements.md) - migrations

| ALG-REQ     | Migrated From | SRS Stub                  | API Endpoint                          |
|-------------|---------------|---------------------------|---------------------------------------|
| ALG-REQ-001 | REQ-029       | REQ-029 → see AlgoSpec.md | `GET /api/paths?from=&to=&hops=`      |
| ALG-REQ-002 | REQ-030       | REQ-030 → see AlgoSpec.md | `GET /api/entry-points`               |
| ALG-REQ-003 | REQ-031       | REQ-031 → see AlgoSpec.md | `GET /api/targets`                    |
| ALG-REQ-010 | REQ-032       | REQ-032 → see AlgoSpec.md | (computation rule, not an endpoint)   |

### 8.1b ALG-REQ to SRS (Requirements.md)

| ALG-REQ     | SRS Ref | API Endpoint                                |
|-------------|---------|---------------------------------------------|
| ALG-REQ-040 | —       | (definition, not endpoint)                  |
| ALG-REQ-041 | —       | (definition, not endpoint)                  |
| ALG-REQ-042 | —       | (algorithm, not endpoint)                   |
| ALG-REQ-043 | REQ-042 | (invalidation side-effect)                  |
| ALG-REQ-044 | —       | (computation rule)                          |
| ALG-REQ-045 | REQ-040 | `POST /api/recalculate-ttb`                 |
| ALG-REQ-046 | —       | (path-scoped, within ALG-REQ-001 flow)      |
| ALG-REQ-047 | —       | (computation rule)                          |
| ALG-REQ-048 | REQ-041 | `GET /api/system-state`                     |
| ALG-REQ-050 | —       | (definition, stored in GrDB as TacticChain) |
| ALG-REQ-051 | —       | (computation rule)                          |
| ALG-REQ-052 | —       | (query template, future ALG-REQ-020)        |
| ALG-REQ-053 | —       | (caching strategy)                          |
| ALG-REQ-060 | —       | (computation rule, internal to TTB)         |
| ALG-REQ-061 | —       | (boundary conditions / test spec)           |
| ALG-REQ-062 | —       | (precondition, OS filtering)                |
| ALG-REQ-063 | —       | (rule, Active flag handling)                |
| ALG-REQ-064 | —       | (reference query, internal to TTB)          |
| ALG-REQ-065 | —       | (output contract, internal to TTB)          |
| ALG-REQ-066 | —       | (relationship to ALG-REQ-052)               |
| ALG-REQ-070 | —       | (algorithm, replaces ALG-REQ-020/044)       |
| ALG-REQ-071 | —       | (parameter definition)                      |
| ALG-REQ-072 | —       | (parameter definition)                      |
| ALG-REQ-073 | —       | (query, internal to TTB)                    |
| ALG-REQ-074 | —       | (rule, vulnerability filtering)             |
| ALG-REQ-075 | —       | (rule + parameter, priority filtering)      |
| ALG-REQ-076 | —       | (query, pattern transitions)                |
| ALG-REQ-077 | —       | (rule, fastest technique selection)         |
| ALG-REQ-078 | —       | (computation rule, accumulation)            |
| ALG-REQ-079 | —       | (output contract, audit log)                |
| ALG-REQ-080 | —       | (edge case, empty set handling)             |


### 8.2 ALG-REQ to UI-Requirements

| ALG-REQ     | Referenced by UI-REQ | Context                                             |
|-------------|----------------------|-----------------------------------------------------|
| ALG-REQ-001 | UI-REQ-207 §4–5      | Run button triggers path calculation; results table |
| ALG-REQ-002 | UI-REQ-207 §1        | Entry point dropdown population                     |
| ALG-REQ-003 | UI-REQ-207 §2        | Target dropdown population                          |
| ALG-REQ-010 | UI-REQ-207 §5        | TTA column value in results table                   |
| ALG-REQ-045 | UI-REQ-112           | Recalculate button triggers bulk recalculation      |
| ALG-REQ-046 | UI-REQ-113           | Stale path warning in Path Inspector                |
| ALG-REQ-048 | UI-REQ-112           | System state endpoint for badge count               |

### 8.3 ALG-REQ to Schema

| ALG-REQ     | Schema Reference                                                                                               | Context                                                   |
|-------------|----------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------|
| ALG-REQ-001 | ED005 (connects_to)                                                                                            | Path traversal follows connects_to edges                  |
| ALG-REQ-010 | TA001 (Asset.TTB)                                                                                              | TTB property used for TTA summation                       |
| ALG-REQ-020 | ED001 (applied_to), TA005                                                                                      | Mitigation impact via applied_to edge properties          |
| ALG-REQ-040 | TA001 (Asset.hash, hash_valid)                                                                                 | Hash properties on Asset tag                              |
| ALG-REQ-042 | TA001, ED001, ED006, ED011, TA004, TA002                                                                       | All hash input sources                                    |
| ALG-REQ-043 | TA001, TA009                                                                                                   | Invalidation writes to Asset + SystemState                |
| ALG-REQ-047 | TA009 (SystemState.merkle_root)                                                                                | Merkle root stored in SystemState                         |
| ALG-REQ-050 | **TA010** (TacticChain), **ED013** (chain_includes), TA007                                                     | Chain vertices + edges reference tMitreTactic vertices    |
| ALG-REQ-051 | TA001 (Asset.is_entrance, is_target)                                                                           | Distinguishes eligibility (property) from position (path) |
| ALG-REQ-052 | **TA010**, **ED013**, TA008, TA007, TA005, ED010, ED009, ED001                                                 | Full chain → MITRE subgraph traversal for TTB             |
| ALG-REQ-053 | TA001 (Asset.TTB, hash, hash_valid)                                                                            | Caching uses existing hash infrastructure                 |
| ALG-REQ-060 | TA008 (execution_min, execution_max), ED001 (Maturity, Active), ED009                                          | TTT formula inputs                                        |
| ALG-REQ-061 | TA008, ED001, ED009                                                                                            | Boundary conditions reference same schema elements        |
| ALG-REQ-062 | ED011 (runs_on), ED014 (represents), TA011 (MitrePlatform), ED003 (can_be_executed_on)                         | OS platform filtering traversal path                      |
| ALG-REQ-063 | ED001 (applied_to.Active)                                                                                      | Active flag on applied_to edge                            |
| ALG-REQ-064 | TA001, TA004, TA005, TA008, TA011, ED001, ED003, ED009, ED011, ED014                                           | Full TTT query touches all MITRE subgraph elements        |
| ALG-REQ-065 | TA008 (output fields sourced from technique properties)                                                        | Output contract reflects technique tag properties         |
| ALG-REQ-066 | TA010, ED013 (via ALG-REQ-052 relationship)                                                                    | Links TTT into the TacticChain-based TTB framework        |
| ALG-REQ-070 | TA010 (TacticChain), ED013 (chain_includes), TA001 (Asset), TA008 (tMitreTechnique)                            | Master algorithm traverses chain and computes TTB         |
| ALG-REQ-071 | —                                                                                                              | Parameter, no schema dependency                           |
| ALG-REQ-072 | —                                                                                                              | Parameter, no schema dependency                           |
| ALG-REQ-073 | ED010 (part_of), ED011 (runs_on), ED014 (represents), TA011 (MitrePlatform), ED003 (can_be_executed_on), TA008 | First-tactic technique selection query                    |
| ALG-REQ-074 | TA001 (Asset.has_vulnerability), TA008 (tMitreTechnique.rcelpe)                                                | Vulnerability filter inputs                               |
| ALG-REQ-075 | TA008 (tMitreTechnique.priority)                                                                               | Priority values from technique tag                        |
| ALG-REQ-076 | TA006 (tMitreState), ED012 (patterns_to)                                                                       | Pattern transition traversal                              |
| ALG-REQ-077 | —                                                                                                              | Selection rule, no direct schema dependency               |
| ALG-REQ-078 | TA001 (Asset.TTB)                                                                                              | TTB value written back to Asset                           |
| ALG-REQ-079 | —                                                                                                              | Log structure, no schema dependency                       |
| ALG-REQ-080 | —                                                                                                              | Edge case handling, no schema dependency                  |

---

## Change Log

| Version | Date        | Author   | Changes                                                                                                                                                                                                                                                                                    |
|---------|-------------|----------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1.0     | Mar 1, 2026 | KSmirnov | Initial version. Migrated REQ-029–032 from SRS v1.10. Added structure for mitigation impact and edge cases.                                                                                                                                                                                |
| 1.1     | Mar 2, 2026 | KSmirnov | Added §4A: ALG-REQ-040–048 (asset state hashing, hash computation, invalidation, bulk/path-scoped TTB recalculation, Merkle root, SystemState endpoint). Cross-reference matrices updated (two ALG-REQ sections added to distinguish between older migrated and newly created requirements |
| 1.2     | Mar 4, 2026 | KSmirnov | Added §4B: ALG-REQ-050–053 (tactic chains, position assignment, TTB query template, caching strategy). Amended ALG-REQ-001 (per-node query), ALG-REQ-010 (position-aware TTA formula), ALG-REQ-044 (stub accepts chain), ALG-REQ-045 (Regular_chain clarification), ALG-REQ-046 (entry/target computation steps). Cross-reference matrices updated. |
| 1.3     | Mar 4, 2026 | KSmirnov | Moved tactic chains from chains.json to GrDB graph objects (TA010 TacticChain, ED013 chain_includes). Rewrote ALG-REQ-050 (chain definition), ALG-REQ-052 (TTB query — corrected nGQL syntax). Amended ALG-REQ-051 (VID mapping), ALG-REQ-044 (chain VID parameter). Cross-references updated for new schema elements. |
| 1.4     | Mar 10, 2026 | KSmirnov | Added §5A: ALG-REQ-060–066 (TTT calculation formula, boundary conditions, OS platform filtering, Active/Inactive mitigation handling, reference nGQL query, output contract, relationship to ALG-REQ-052). TTT definition in §2 expanded. Schema cross-reference updated for SCHEMA v1.9 elements (TA011 MitrePlatform, ED014 represents, ED003 can_be_executed_on). Future extensions updated. |
| 1.5 | Mar 10, 2026 | KSmirnov | Added §5B: ALG-REQ-070–080 (TTB calculation algorithm, orientation/switchover time parameters, first-tactic technique selection, vulnerability filtering, priority selection with tolerance, pattern-based technique transitions, fastest technique selection, TTB accumulation formula, calculation log, empty-set handling). TTB definition in §2 expanded; Orientation Time, Switchover Time, Priority Tolerance definitions added. ALG-REQ-020/021/022 marked as superseded. ALG-REQ-044 supersession notice added. Cross-reference matrices updated. Future extensions updated. |
| 1.6  | Mar 11, 2026 | KSmirnov | ALG-REQ-042: replaced OPTIONAL MATCH with MATCH for runs_on and has_type per SCHEMA DI-01/DI-03; removed COALESCE fallbacks. Added note to ALG-REQ-064. Updated SCHEMA reference to v1.10. |
| 1.7  | Mar 11, 2026 | KSmirnov | §7: Added batch ComputeTTT and re-unify ComputeTTT query to future extensions. Marked UI controls for TTB params as completed (UI-REQ-2091). |
| 1.8 | Mar 13, 2026 | KSmirnov | §1.3 updated (ADR companion doc reference). §7 updated: MariaDB stub marked as complete; TTB log persistence noted as schema-ready. No algorithm changes. |
---

**End of Document**
