# ESP01 NebulaGraph 3.8 Schema - Complete Documentation
**Version:** 1.8  
**Created:** March 04, 2026  
**Prepared by:** Konstantin Smirnov
**Space:** ESP01 (IT Infrastructure / MITRE ATT&CK Model)  
**Source:** Full NebulaGraph Studio console, author's comments
**Document code:** SCHEMA


## SP01: Space Configuration
CREATE SPACE ESP01 (
partition_num = 10,
replica_factor = 1,
charset = utf8,
collate = utf8_bin,
vid_type = FIXED_STRING(64)
);


| Property         | Value            |
|------------------|------------------|
| ID               | 6                |
| Partition Number | 10               |
| Replica Factor   | 1                |
| Charset          | utf8             |
| Collate          | utf8_bin         |
| VID Type         | FIXED_STRING(64) |
| Comment          | (empty)          |

## TA: Tags
MITRE ATT&CK tactics, techniques, subtechniques, mitigations
IT Infrastructure assets, asset types, OS types
MITRE Tactic/Technique pairs and their patterns
SystemState tag to keep Merkle has of the modelled IT Infrastructure

Every tag description follows this pattern:
* `Used for` - what this tag is used for
* `Tag properties` - what properties does this edge have, type, nullable or not, default value, optional comments.
* `Notes` (optional) - any other useful information regarding the use of a tag or the nature of its properties

### TA001: Asset
#### Used for
Represents an asset in IT Infrastructure
#### Tag properties
| Field             | Type   | Null | Default | Comment                                |
|-------------------|--------|------|---------|----------------------------------------|
| Asset_ID          | string | NO   | _EMPTY_ | like A0001                             |
| Asset_Name        | string | NO   | _EMPTY_ | like CRM-SRV-01                        |
| Asset_Description | string | YES  | _EMPTY_ | like primary CRM server                |
| Asset_Note        | string | YES  | _EMPTY_ | any useful information                 |
| Asset_Version     | string | YES  | 1.0     | _EMPTY_                                |
| is_entrance       | bool   | NO   | false   | entry point?                           |
| is_target         | bool   | NO   | false   | attack target?                         |
| priority          | int16  | YES  | 4       | lower = more critical                  |
| has_vulnerability | bool   | YES  | false   | critical vuln present?                 |
| TTB               | int32  | YES  | 10      | Time To Bypass                         |
| hash              | string | YES  |         | hash represents the state of the Asset |
| hash_valid        | bool   | YES  | false   | false if the hash is stale             |

#### Notes
Asset IDs and VIDs (here and for all tags ID for a tag is its VID as well), are in a format like "A00001". The specific index format can be later substituted for GUID, or longer string. This format is chose for simplicity and clarity.
TTB stands for time to bypass - teh calculated time the hacker needs to traverse (bypass) this very node.
Asset Version field is reserved for future use.
has vulnerability is used to indicate that there is a vulnerability on this host.

### TA002: Asset_Type
#### Used for
Type of asset like Server, Workstation, etc.
#### Tag properties
| Field             | Type    | Null | Default | Comment |
|-------------------|---------|------|---------|---------|
| Type_ID           | string  | YES  | _EMPTY_ | _EMPTY_ |
| Type_Name         | string  | YES  | _EMPTY_ | _EMPTY_ |
| Type_Description  | string  | YES  | _EMPTY_ | _EMPTY_ |
#### Notes
Asset Type IDs have the format like "DT001".

### TA003: Network_Segment
#### Used for
Network segment that asset belongs to, such as DMZ, DC Lan, etc.
#### Tag properties
| Field               | Type   | Null | Default | Comment             |
|---------------------|--------|------|---------|---------------------|
| Segment_ID          | string | YES  | _EMPTY_ | Like SEG0001        |
| Segment_Name        | string | YES  | _EMPTY_ | Like Office_Wifi    |
| Segment_Description | string | YES  | _EMPTY_ | Any meaningful info |
| Segment_Version     | string | NO   | 1.0     | Version             |
| Segment_Date        | date   | YES  | _EMPTY_ | When created        |
#### Notes
Segment IDs have format like "SEG00001".

### TA004: OS_Type
#### Used for
Type of OS that asset runs, like Windows, Linux, Cisco iOS, etc.
#### Tag properties
| Field        | Type    | Null | Default | Comment |
|--------------|---------|------|---------|---------|
| OS_ID        | string  | YES  | _EMPTY_ | _EMPTY_ |
| OS_Name      | string  | YES  | _EMPTY_ | _EMPTY_ |
| OS_Version   | string  | YES  | _EMPTY_ | _EMPTY_ |
| OS_Vendor    | string  | YES  | _EMPTY_ | _EMPTY_ |
#### Notes
OS ID have the formats like "OPS0001"

### TA005: tMitreMitigation
#### Used for
This tag is used to represent MITRE mitigation applied to a host.
#### Tag properties
| Field              | Type    | Null | Default    | Comment |
|--------------------|---------|------|------------|---------|
| Mitigation_ID      | string  | YES  | NULL       | _EMPTY_ |
| Mitigation_Name    | string  | YES  | NULL       | _EMPTY_ |
| Matrix             | string  | YES  | Enterprise | _EMPTY_ |
| Description        | string  | YES  | NULL       | _EMPTY_ |
| Mitigation_Version | string  | YES  | NULL       | _EMPTY_ | 
#### Notes
Both Matrix and Mitigation version are filled from the current MITRE site. Mitigation IDs are teh same as in MITRE.

### TA006: tMitreState
#### Used for
This tag represents a pair of tactic/technique (or tactic/subtechnique) which can pattern (transit) to another combination of tactic/technique
#### Tag properties
| Field     | Type    | Null | Default | Comment |
|-----------|---------|------|---------|---------|
| state_id  | string  | NO   | _EMPTY_ | _EMPTY_ |
#### Notes
These tMitreState pairs not necessarily exist for all combinations of tactic/technique (subtechnique). The state ID has the following format "TA0001|T1133" (Tactic|Technique or subtechnique respectfully).

### TA007: tMitreTactic
#### Used for
Represents Mitre tactic
#### Tag properties
| Field                | Type   | Null | Default | Comment              |
|----------------------|--------|------|---------|----------------------|
| Tactic_ID            | string | NO   | _EMPTY_ | Tactic ID            |
| Tactic_Name          | string | NO   | _EMPTY_ | Tactic Name          |
| Mitre_Attack_Version | string | YES  | _EMPTY_ | Mitre ATTACK version |
#### Notes
Tactic ID is the same as in Mitre.

### TA008: tMitreTechnique
#### Used for
Represents Mitre Technique/Subtechnique
#### Tag properties
| Field                 | Type    | Null | Default | Comment                                   |
|-----------------------|---------|------|---------|-------------------------------------------|
| Technique_ID          | string  | NO   | _EMPTY_ | MITRE Technique number                    |
| Technique_Name        | string  | NO   | _EMPTY_ | MITRE Technique name                      |
| Mitre_Attack_Version  | string  | YES  | _EMPTY_ | MITRE Attack Matrix version               |
| rcelpe                | bool    | YES  | false   | Can be applied to host with critical vuln |
| priority              | int8    | NO   | 4       | Priority (1-4, higher=more likely used)   |
| execution_min         | float   | NO   | 0.1667  | Minimum execution time                    |
| execution_max         | float   | NO   | 120     | Maximum execution time                    |
#### Notes
Technique and subtechnique are represented by the same type of tag, subtechnique has an extra relationship to its parent.
Technique ID is the same as in MITRE.

### TA009: SystemState
#### Used for
Keeps Merkle root - "hash of hashes" to indicate that something has changed in the modelled IT infrastructure, to force TTB recalculation (individual assets have their own hashes to determine TTB staleness).
### Tag properties
| Field            | Type     | Null | Default    | Comment                                            |
|------------------|----------|------|------------|----------------------------------------------------|
| state_id         | string   | NO   | ""         | So far going to be only one like "SYS001"          |
| merkle_root      | int64    | YES  | 0          | hash of hashes                                     |
| last_recalc_time | datetime | YES  | datetime() | When was it last time calculated                   |
| total_assets     | int32    | YES  | 0          | Number of assets in the model                      |
| stale_count      | int32    | YES  | 0          | Number of changed (stale hash) assets in the model |


### TA010: TacticChain

#### Used for
Represents an ordered tactic chain that defines which MITRE ATT&CK tactics are executed on an asset depending on its position in an attack path (entry point, intermediate, or target). Each chain vertex is connected to its constituent tMitreTactic vertices via `chain_includes` edges (ED013), with edge rank encoding execution order.

#### Tag properties
| Field       | Type   | Null | Default | Comment                                      |
|-------------|--------|------|---------|----------------------------------------------|
| chain_id    | string | NO   | _EMPTY_ | Same as VID, e.g. "CHAIN_ENTRANCE"           |
| chain_name  | string | NO   | _EMPTY_ | Human-readable name, e.g. "Entrance Chain"   |
| description | string | YES  | _EMPTY_ | Purpose of this chain                        |

#### Notes
Three TacticChain vertices exist in the initial dataset. The chain_id matches the VID for consistency. Future versions may add additional chain types (e.g. "pivot point" for assets that serve as stepping stones between network segments).

#### CREATE TAG statement
```nGQL
CREATE TAG IF NOT EXISTS TacticChain(
  chain_id string NOT NULL DEFAULT "",
  chain_name string NOT NULL DEFAULT "",
  description string DEFAULT ""
);
```

## ED: Edges
Relationships for network topology, asset types, OS, how mitigation applied to assets, and relationships between tactics, techniques, subtechniques, and mitigations.

Every edge description follows this pattern:
* `Used for` - what this edge is used for
* `Edge properties` - what properties does this edge have, type, nullable or not, default value, optional comments.
* `Notes` (optional) - any other useful information regarding the use of an edge or the nature of its properties

### ED001: applied_to
#### Used for
This is a relationship between tMitreMitigation and Asset tags. (tMitreMitigation --applied_to--> Asset)
#### Edge properties
| Field     | Type    | Null | Default | Comment                  |
|-----------|---------|------|---------|--------------------------|
| Version   | string  | YES  | 1.0     | Version for a project    |
| Maturity  | int16   | YES  | 100     | 0-100, higher better     |
| Active    | bool    | YES  | true    | If inactive/deprecated   |
#### Note
Version is a field for a version of an IT Infrastructure, where different versions of relationships indicate the different sets of mitigations applied to a host (IT Infrastructure asset). Later versions will have versions of IT Infrastructure differentiated by versions (i.e. same component but with the newer version). **To Be Verified Later**


### ED002: belongs_to
#### Used for
This relationship is used to indicate to which network segment an asset belongs to. (Asset --belongs_to--> NetworkSegment)
#### Edge properties
| Field          | Type   | Null | Default | Comment                      |
|----------------|--------|------|---------|------------------------------|
| interface_name | string | YES  | _EMPTY_ | Physical/logical (eth0, ilo) |
| role           | string | YES  | _EMPTY_ | primary, gateway, management |
| ip_address     | string | YES  | _EMPTY_ | IP on interface              |
| vlan_id        | int16  | YES  | _EMPTY_ | VLAN ID if applicable        |
#### Notes
None of the fields are used so far.

### ED003: can_be_executed_on
#### Used for
So far is not used.
#### Edge properties
No properties.

### ED004: defines_state
#### Used for 
This relationship is used to link MITRE tactic (tMitreTactic) and MITRE technique/subtechnique (tMitreTechnique) to a state (pattern) that they both form for automating the transitions between one Tactic/Technique to another Tactic/Technique.
#### Edge properties
No properties.


### ED005: has_subtechnique
#### Used for
This relationship between a technique and a subtechnique indicates that this technique has a subtechnique (tMitreTechnique --has_subtechnique--> tMitreTechnique). This is done, because MITRE treats techniques and subtechniques as equal, i.e. the hierarchy Tactic - Technique/Subtechnique has one level, and this type of edges provides the way to group otherwise same-level items under its parent technique. In other words, subtechniques have two relations to their parents, one to tactic and one to its parent technique. 
#### Edge properties
No properties (pure relationship edge)

### ED006: connects_to
#### Used for
This edge indicates that one host can connect to another - through which combination of ports and protocols.
#### Edge properties
| Field               | Type   | Null | Default | Comment                 |
|---------------------|--------|------|---------|-------------------------|
| Connection_Protocol | string | YES  | TCP     | ip, tcp, udp, icmp      |
| Connection_Port     | string | YES  | 0-65536 | Port/range: 443, 80;443 |
#### Notes
> This is future changes candidate Number 1 (i.e. which model describes the connectivity best).
>
#### Edge Uniqueness and Rank
> In NebulaGraph, an edge is uniquely identified by the four-tuple: `(source_vid, edge_type, rank, destination_vid)`. If two `connects_to` edges share the same source, destination, **and rank**, the second INSERT **overwrites** the first.
>
> To store multiple connections between the same pair of assets (e.g., TCP/443 and UDP/1194 from the same source to the same target), each edge MUST have a **unique rank value** assigned via the `@rank` syntax:
>
> ```sql
> -- First connection (rank 0 — default)
> INSERT EDGE IF NOT EXISTS connects_to(Connection_Protocol, Connection_Port)
>   VALUES "A00025"->"A00002"@0:("TCP", "443");
>
> -- Second connection (rank 1)
> INSERT EDGE IF NOT EXISTS connects_to(Connection_Protocol, Connection_Port)
>   VALUES "A00025"->"A00002"@1:("UDP", "1194");
> ```
>
> **Rank assignment convention:** For each unique `(source, target)` pair, ranks start at 0 and increment by 1. When bulk-loading from a spreadsheet, sort by `(source, target)` and compute rank as: if the current row's `(source, target)` matches the previous row, `rank = previous_rank + 1`; otherwise `rank = 0`.
>
> **Note:** This rank requirement applies to **all edge types** in NebulaGraph where multiple edges of the same type may connect the same vertex pair. Currently, only `connects_to` requires this in practice.


### ED007: has_type
#### Used for
This field is used to indicate device type (Asset_Type) to IT infrastructure asset (Asset) - (Asset --has_type--> AssetType).
#### Edge properties
| Field         | Type     | Null | Default     | Comment |
|---------------|----------|------|-------------|---------|
| assigned_date | datetime | YES  | datetime()  | _EMPTY_ | 
#### Notes
Assigned date is the indirect way to indicate when teh asset was added to an asset database. More properties will be added at later stage (like IT Infrastructure version - for future use).

### ED008: implements
#### Used for
This relationship is to indicate which IT Infrastructure asset (Asset) implements particular network segment - (Asset --implements--> Network_Segment).
#### Edge properties
| Field     | Type   | Null | Default | Comment                                                    |
|-----------|--------|------|---------|------------------------------------------------------------|
| function  | string | YES  | _EMPTY_ | Device: firewall, switch, router, wireless_ap, vpn_gateway |
| vlan_id   | int16  | YES  | _EMPTY_ | VLAN ID if applicable                                      |
| role      | string | YES  | _EMPTY_ | primary, backup, load_balanced                             |
| is_active | bool   | YES  | true    | Currently active/operational                               |
#### Notes
So far, none of the properties are used, except is_active. Subject to future improvements.

### ED009: mitigates
#### Used for
This is a relationship between the mitigation and the Technique/Subtechniqu - (tMitreMitigation --mitigates--> tMitreTechnique).
#### Edge properties
| Field            | Type    | Null | Default    | Comment |
|------------------|---------|------|------------|---------|
| Use_Description  | string  | YES  | NULL       | _EMPTY_ |
| Domain           | string  | YES  | Enterprise | _EMPTY_ |
#### Notes
The data on connectivity is collected from MITRE ATT&CK Enterprise matrix by an external tool (going to be a part of the project later on). So far treated as static relationship. Use_Description field is not used at the moment.

### ED010: part_of
#### Used for
This is to show the relationship between technique/subtechnique and its parent tactic. (tMitreTechnique --part_of--> tMitreTactic).
#### Edge properties
No properties (pure relationship edge)
#### Notes
The data on connectivity is collected from MITRE ATT&CK Enterprise matrix by an external tool (going to be a part of the project later on). So far treated as static relationship.

### ED011: runs_on
#### Used for
This is to show the relationship between Asset and the type od operating system it runs. (Asset --runs_on--> OS_type).
#### Edge properties
| Field             | Type     | Null | Default | Comment |
|-------------------|----------|------|---------|---------|
| installation_date | datetime | YES  | NULL    | _EMPTY_ |
| license_key       | string   | YES  | NULL    | _EMPTY_ |
#### Notes
The data on connectivity is defined externally, at the data loading or later updated using the built-in ESP PoC system interface. None of the fields are used so far.

### ED012: patterns_to
#### Used for
This is to show the relationship between Tactic/Tecqnique (or subtechnique) and the MITRE state (combination of tactic and technique). (tMitreTactic --patterns_to--> tMitreState, tMitreTechnique --patterns_to--> tMitreState).
#### Edge properties
| Field          | Type   | Null | Default | Comment |
|----------------|--------|------|---------|---------|
| probability    | float  | YES  | NULL    | _EMPTY_ |
| observed_count | int64  | YES  | NULL    | _EMPTY_ |
| comment        | string | YES  | NULL    | _EMPTY_ |
#### Notes
Edges are defined externally by a separate application. None of teh fields are used at the moment.

### ED013: chain_includes

#### Used for
This edge links a TacticChain to its constituent tMitreTactic vertices, representing which tactics belong to the chain and in what order. (TacticChain --chain_includes--> tMitreTactic)

#### Edge properties
No properties (pure structural edge). Execution order is encoded in the edge rank.

#### Edge Uniqueness and Rank
The edge rank (`@rank`) encodes the sequential position of the tactic within the chain: rank 0 = first tactic, rank 1 = second, etc. This follows the same rank convention as `connects_to` (ED006). Since a given chain will not include the same tactic twice, each `(chain_vid, tactic_vid)` pair has a unique rank.

#### CREATE EDGE statement
```nGQL
CREATE EDGE IF NOT EXISTS chain_includes();
```

#### Notes
No edge index is required for the initial dataset (3 chains × ~8 tactics = 25 edges). If additional chain types are added in the future, an index may be considered.




## IN: Indexes (10 Total)
### Tag Indexes (7)
| Index Name         | On Tag          | Columns                    |
|--------------------|-----------------|----------------------------|
| MTactic_Index      | tMitreTactic    | ["Tactic_ID"]              |
| TecniqueIndex      | tMitreTechnique | ["Technique_ID"]           |
| idx_asset_any      | Asset           | ["Asset_ID", "Asset_Name"] |
| idx_asset_type_any | Asset_Type      | []                         |
| idx_os_type_any    | OS_Type         | []                         |
| idx_segment_any    | Network_Segment | []                         |
| state_id_index     | tMitreState     | ["state_id"]               |

### Edge Indexes (3)
| Index Name      | On Edge          | Columns |
|-----------------|------------------|---------|
| ConnectsToIndex | connects_to      | []      |
| PartOfIndex     | part_of          | []      |
| SubtechIndex    | has_subtechnique | []      |



| Version | Date         | Changes                                                                                                            | Author             |
|---------|--------------|--------------------------------------------------------------------------------------------------------------------|--------------------|
| 1.0     | Feb 16, 2026 | Initial version                                                                                                    | Konstantin Smirnov |
| 1.6     | Feb 27, 2026 | Added runs_on, patterns_to edge types                                                                              | Konstantin Smirnov |
| 1.7     | Mar 01, 2026 | TA001: added hash (string) and hash_valid (bool) properties. TA009 added (SystemState tag). SYS001 vertex created. | AI + K.Smirnov     |
| 1.8     | Mar 4, 2026  | TA010 added (TacticChain tag). ED013 added (chain_includes edge). 3 vertices + 25 edges loaded.                    | AI + K.Smirnov     |