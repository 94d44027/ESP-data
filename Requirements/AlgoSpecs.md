# Algorithm Requirements Specification (ALGO)
## ESP PoC — TTA/TTB Path Calculation and related things

**Version:** 1.2  
**Date:** March 4, 2026  
**Prepared by:** Konstantin Smirnov with the kind assistance of Perplexity AI  
**Project:** ESP PoC for Nebula Graph  
**Reference:** Derived from SRS, UIS, SCHEMA
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

| Document                             | Version | Relationship                                                                                                                  |
|--------------------------------------|---------|-------------------------------------------------------------------------------------------------------------------------------|
| Requirements.md (SRS)                | v1.12   | Parent document. Stubs REQ-029–032 reference this spec. API summary in Appendix C links here.                                 |
| UI-Requirements.md  (UIR)            | v1.12   | UI-REQ-207 consumes path calculation results; UI-REQ-208/332 visualise them on the graph canvas.                              |
| ESP01_NebulaGraph_Schema.md (SCHEMA) | v1.7    | Defines Asset.TTB property (TA001), connects_to edges (ED005), applied_to edges (ED001) and other elements of database schema |

### 1.4 Requirement ID Convention

All requirements in this document use the prefix `ALG-REQ-` followed by a three-digit number. Sections use `##` for chapters and `###` for individual requirements (as headers), matching the style of UI-Requirements.md.

---

## 2. Definitions

| Term               | Definition                                                                                                                                                                                                                     |
|--------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **TTA**            | Time To Attack — the cumulative time from initial access to the beginning of actions on objective, computed as the sum of TTB along a path                                                                                     |
| **TTB**            | Time To Bypass — the time interval to traverse (bypass) a single host; stored as `Asset.TTB` (int32, default 10)                                                                                                               |
| **TTT**            | Time to execuTe a Technique - time interval required to execute a technioue/subtechniqe on a particular node, considering mitigations applied                                                                                  |
| **Path**           | An ordered sequence of Asset nodes connected by directed `connects_to` edges, from an entry point to a target, with no repeated nodes                                                                                          |
| **Hop**            | A single `connects_to` edge traversal between two adjacent nodes in a path                                                                                                                                                     |
| **Entry Point**    | An Asset where `is_entrance == true`; represents the attacker's starting position                                                                                                                                              |
| **Target**         | An Asset where `is_target == true`; represents the objective the attacker aims to reach                                                                                                                                        |
| **Path ID**        | Ephemeral sequential identifier (e.g. P00001) assigned to each calculated path within a single session; not persisted in the database                                                                                          |
| **Mitigation**     | A MITRE ATT&CK mitigation (`tMitreMitigation`) linked to an Asset via `applied_to` edge, potentially modifying the effective TTB                                                                                               |
| **Tactic Chain**   | An ordered list of MITRE ATT&CK tactic IDs that an attacker executes on a node, determined by the node's position in the attack path. Defined in `chains.json` (ALG-REQ-050).                                                  |
| **Chain Position** | The role of an asset within a specific attack path: **Entrance** (first node, N₀), **Intermediate** (middle nodes, N₁..Nₖ₋₁), or **Target** (last node, Nₖ). A single asset may hold different positions in different paths.   |


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
- `Cₑ` = `Entrance_chain` from `chains.json` (ALG-REQ-050)
- `Cᵣ` = `Regular_chain` from `chains.json` (ALG-REQ-050)
- `Cₜ` = `Target_chain` from `chains.json` (ALG-REQ-050)
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
OPTIONAL MATCH (a)-[:runs_on]->(os:OS_Type)
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.TTB AS current_ttb,
  a.Asset.hash AS stored_hash,
  hash(concat_ws("##",
    reduce(s = "", x IN conn_parts | s + x + ";"),
    reduce(s = "", x IN mit_parts | s + x + ";"),
    toString(a.Asset.has_vulnerability),
    COALESCE(os.OS_Type.OS_Name, "none"),
    COALESCE(t.Asset_Type.Type_Name, "none")
  )) AS computed_hash;
```

>Design note: `hash()` is not a cryptographic hash — it is MurmurHash2 returning int64. This is acceptable for change detection purposes. The stored `hash` property on Asset is a string representation of this int64 (via `toString()` in the APP layer).

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

Until the full TTB computation algorithm is implemented (see ALG-REQ-020/021 placeholders and ALG-REQ-052), TTB SHALL be recalculated using a stub procedure that accepts a tactic chain parameter:

```
new_TTB = current_TTB + random_integer(1, 10) + len(tactic_chain)
```

where `random_integer(1, 10)` produces a uniformly distributed integer in the range [1, 10] inclusive, and `len(tactic_chain)` is the number of tactics in the provided chain.

The `len(tactic_chain)` term ensures that TTB values differ by position even while using the stub: Entrance_chain (8 tactics) produces a different offset than Regular_chain (7 tactics) or Target_chain (10 tactics).

The stub is implemented in the APP layer (Go code). The function signature SHALL accept the tactic chain:

```go
func computeTTBStub(currentTTB int, tacticChain []string) int
```

The purpose is to verify the end-to-end hash invalidation, position-aware recalculation pipeline, and TTA summation without requiring the full mitigation-aware TTB formula.

>Design note: The `len(tactic_chain)` addition is a minimal change to make the stub position-sensitive. It will be replaced entirely when the real TTB formula (ALG-REQ-052) is implemented.

**Amended in:** v1.2 — stub accepts tactic chain parameter.

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

The system SHALL load tactic chain definitions from a configuration file (`chains.json`) at APP startup. The file defines three tactic chains, each being an ordered list of MITRE ATT&CK tactic IDs:

```json
{
  "Entrance_chain": [
    "TA0001","TA0002","TA0003","TA0004",
    "TA0005","TA0006","TA0007","TA0008"
  ],
  "Regular_chain": [
    "TA0002","TA0003","TA0004","TA0005",
    "TA0006","TA0007","TA0008"
  ],
  "Target_chain": [
    "TA0002","TA0003","TA0004","TA0005",
    "TA0006","TA0007","TA0008","TA0011",
    "TA0009","TA0040"
  ]
}
```

**Semantics:**

| Chain | Position | Distinctive Tactics | Rationale |
|-------|----------|---------------------|-----------|
| `Entrance_chain` | Entry point (N₀) | Includes **TA0001** (Initial Access) | The attacker must first gain access to the entry point |
| `Regular_chain` | Intermediate (N₁..Nₖ₋₁) | TA0002–TA0008 only | The attacker traverses intermediates via Lateral Movement (TA0008); no initial access or post-exploitation needed |
| `Target_chain` | Target (Nₖ) | Adds **TA0011** (C2), **TA0009** (Collection), **TA0040** (Impact) | The attacker performs actions on objective at the target |

The file SHALL be loaded once at startup and cached in memory. If the file is missing or malformed, the APP layer SHALL log an error and fall back to hardcoded default chains matching the values above.

>Design note: `chains.json` is a configuration artifact, not a database entity. Tactic chains are static across all path calculations within a session. If they need to change, the APP is restarted with an updated file. Future versions may store chains in the database for dynamic updates.

### ALG-REQ-051: Chain Position Assignment Rule

When computing TTA for a set of discovered paths between entry point `E` and target `T`, the APP layer SHALL assign chain positions as follows:

Given a path `[N₀, N₁, N₂, ..., Nₖ]`:
- `N₀` (always equal to `E`) → **Entrance** → uses `Entrance_chain`
- `Nₖ` (always equal to `T`) → **Target** → uses `Target_chain`
- All other nodes `N₁..Nₖ₋₁` → **Intermediate** → uses `Regular_chain`

Position assignment is determined solely by the node's index in the specific path, **not** by the asset's `is_entrance` or `is_target` properties. The same physical asset may be an intermediate node in one path and an entry point in another (across different API calls with different `from`/`to` parameters).

>Design note: The `is_entrance` and `is_target` asset properties (TA001) define which assets are *eligible* to serve as entry points or targets (used to populate dropdowns in UI-REQ-207). Chain position assignment (this requirement) defines the asset's *role* within a specific calculated path. These are distinct concepts.

### ALG-REQ-052: Position-Aware TTB Query Template

When the full TTB computation replaces the stub (ALG-REQ-044), the TTB for a given `(asset, tactic_chain)` pair SHALL be computed by querying the MITRE subgraph in the database. The query identifies which techniques (under the given tactics) apply to the asset, and which of those techniques are mitigated.

The reference MATCH query template:

```nGQL
MATCH (tech:tMitreTechnique)-[:part_of]->(tac:tMitreTactic)
WHERE tac.tMitreTactic.Tactic_ID IN {tactic_chain}
OPTIONAL MATCH (mit:tMitreMitigation)-[:mitigates]->(tech)
OPTIONAL MATCH (mit)-[app:applied_to]->(a:Asset)
WHERE a.Asset.Asset_ID == "{assetId}"
RETURN
  tech.tMitreTechnique.Technique_ID AS tech_id,
  tech.tMitreTechnique.execution_min AS exec_min,
  tech.tMitreTechnique.execution_max AS exec_max,
  tech.tMitreTechnique.priority AS tech_priority,
  tech.tMitreTechnique.rcelpe AS vuln_applicable,
  mit.tMitreMitigation.Mitigation_ID AS mit_id,
  app.Maturity AS maturity,
  app.Active AS active;
```

Where `{tactic_chain}` is substituted with the appropriate chain (e.g. `["TA0001","TA0002","TA0003","TA0004","TA0005","TA0006","TA0007","TA0008"]` for Entrance_chain) and `{assetId}` is the target asset's ID.

Justification for MATCH syntax (per REQ-244 in SRS): traversing tMitreTechnique → tMitreTactic with a filtered tactic list, then optionally joining through mitigates → applied_to to check asset-specific mitigation status, requires multi-hop OPTIONAL MATCH with property retrieval on intermediate edges — this is significantly cleaner with MATCH than with chained GO/FETCH statements.

The query returns one row per technique under the given tactics, with `NULL` values for `mit_id`/`maturity`/`active` when a technique has no mitigation applied to this specific asset. The APP layer (or a future GrDB-side `reduce()`) uses these rows to compute TTB according to the formula defined in ALG-REQ-020/021 (placeholders).

>Design note 1: This query is the foundation for the real TTB formula. The stub (ALG-REQ-044) does not use this query — it uses a simple arithmetic formula. Once ALG-REQ-020/021 are defined, this query replaces the stub.

>Design note 2: The `rcelpe` field on tMitreTechnique (TA008) indicates whether a technique can exploit a critical vulnerability. When the asset has `has_vulnerability == true`, techniques with `rcelpe == true` may receive reduced execution time in the TTB formula (future ALG-REQ-020).

>Design note 3: For the PoC dataset, the MITRE subgraph contains ~200 techniques. The `IN` filter on tactic IDs reduces this to a subset per chain. The OPTIONAL MATCH for asset-specific mitigations is further scoped. Expected execution time is well under 1 second.

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

### ALG-REQ-020: Mitigation-Aware TTA (Placeholder)

_Reserved for: How applied mitigations (via `applied_to` edges) modify the effective TTB of an asset when computing TTA._

Key questions to address:
- Does a mitigation increase TTB (making the host harder to bypass)?
- How does Maturity (25/50/80/100) scale the mitigation effect?
- How does Active/Disabled status affect the calculation?
- Are multiple mitigations on the same asset additive, multiplicative, or capped?

### ALG-REQ-021: Mitigation Maturity Weighting (Placeholder)

_Reserved for: The formula or lookup table that maps Maturity values (25, 50, 80, 100) to their effect on TTB._

### ALG-REQ-022: Recalculation Trigger (Placeholder)

_Reserved for: When and how TTA is recalculated after mitigations are added, modified, or removed._

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
- [ ] Full TTB formula implementation (ALG-REQ-020/021) using ALG-REQ-052 query results
- [ ] Dynamic tactic chain configuration via database instead of `chains.json`
- [ ] APP-layer TTB cache for entry/target positions (ALG-REQ-053 optional optimisation)


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

| ALG-REQ     | SRS Ref | API Endpoint                           |
|-------------|---------|----------------------------------------|
| ALG-REQ-040 | —       | (definition, not endpoint)             |
| ALG-REQ-041 | —       | (definition, not endpoint)             |
| ALG-REQ-042 | —       | (algorithm, not endpoint)              |
| ALG-REQ-043 | REQ-042 | (invalidation side-effect)             |
| ALG-REQ-044 | —       | (computation rule)                     |
| ALG-REQ-045 | REQ-040 | `POST /api/recalculate-ttb`            |
| ALG-REQ-046 | —       | (path-scoped, within ALG-REQ-001 flow) |
| ALG-REQ-047 | —       | (computation rule)                     |
| ALG-REQ-048 | REQ-041 | `GET /api/system-state`                |
| ALG-REQ-050 | —       | (definition, loaded from chains.json)  |
| ALG-REQ-051 | —       | (computation rule)                     |
| ALG-REQ-052 | —       | (query template, future ALG-REQ-020)   |
| ALG-REQ-053 | —       | (caching strategy)                     |

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

| ALG-REQ     | Schema Reference                         | Context                                                   |
|-------------|------------------------------------------|-----------------------------------------------------------|
| ALG-REQ-001 | ED005 (connects_to)                      | Path traversal follows connects_to edges                  |
| ALG-REQ-010 | TA001 (Asset.TTB)                        | TTB property used for TTA summation                       |
| ALG-REQ-020 | ED001 (applied_to), TA005                | Mitigation impact via applied_to edge properties          |
| ALG-REQ-040 | TA001 (Asset.hash, hash_valid)           | Hash properties on Asset tag                              |
| ALG-REQ-042 | TA001, ED001, ED006, ED011, TA004, TA002 | All hash input sources                                    |
| ALG-REQ-043 | TA001, TA009                             | Invalidation writes to Asset + SystemState                |
| ALG-REQ-047 | TA009 (SystemState.merkle_root)          | Merkle root stored in SystemState                         |
| ALG-REQ-050 | TA007 (tMitreTactic.Tactic_ID)           | Tactic IDs in chains reference tMitreTactic vertices      |
| ALG-REQ-051 | TA001 (Asset.is_entrance, is_target)     | Distinguishes eligibility (property) from position (path) |
| ALG-REQ-052 | TA008, TA007, TA005, ED010, ED009, ED001 | Full MITRE subgraph traversal for TTB computation         |
| ALG-REQ-053 | TA001 (Asset.TTB, hash, hash_valid)      | Caching uses existing hash infrastructure                 |
---

## Change Log

| Version | Date        | Author   | Changes                                                                                                                                                                                                                                                                                    |
|---------|-------------|----------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1.0     | Mar 1, 2026 | KSmirnov | Initial version. Migrated REQ-029–032 from SRS v1.10. Added structure for mitigation impact and edge cases.                                                                                                                                                                                |
| 1.1     | Mar 2, 2026 | KSmirnov | Added §4A: ALG-REQ-040–048 (asset state hashing, hash computation, invalidation, bulk/path-scoped TTB recalculation, Merkle root, SystemState endpoint). Cross-reference matrices updated (two ALG-REQ sections added to distinguish between older migrated and newly created requirements |
| 1.2     | Mar 4, 2026 | KSmirnov | Added §4B: ALG-REQ-050–053 (tactic chains, position assignment, TTB query template, caching strategy). Amended ALG-REQ-001 (per-node query), ALG-REQ-010 (position-aware TTA formula), ALG-REQ-044 (stub accepts chain), ALG-REQ-045 (Regular_chain clarification), ALG-REQ-046 (entry/target computation steps). Cross-reference matrices updated. |

---

**End of Document**
