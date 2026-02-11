# ESP01 NebulaGraph 3.8 Schema - Complete Documentation
**Generated:** February 10, 2026 3:16 PM MSK  
**Space:** ESP01 (IT Infrastructure / MITRE ATT&CK Model)  
**Source:** Full NebulaGraph Studio console, author's comments


## Space Configuration
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

## Tags (8 Types) - Full Properties
MITRE ATT&CK tactics, techniques, subtechniques, mitigations
IT Infrastructure assets, asset types, OS types
MITRE Tactic/Technique pairs and their patterns

### Asset
#### Used for
Represents an asset in IT Infrastructure
#### Tag properties
| Field             | Type   | Null | Default | Comment                 |
|-------------------|--------|------|---------|-------------------------|
| Asset_ID          | string | NO   | _EMPTY_ | like A0001              |
| Asset_Name        | string | NO   | _EMPTY_ | like CRM-SRV-01         |
| Asset_Description | string | YES  | _EMPTY_ | like primary CRM server |
| Asset_Note        | string | YES  | _EMPTY_ | any useful information  |
| Asset_Version     | string | YES  | 1.0     | _EMPTY_                 |
| is_entrance       | bool   | NO   | false   | entry point?            |
| is_target         | bool   | NO   | false   | attack target?          |
| priority          | int16  | YES  | 4       | lower = more critical   |
| has_vulnerability | bool   | YES  | false   | critical vuln present?  |
| TTB               | int32  | YES  | 10      | Time To Bypass          |

#### Notes
Asset IDs and VIDs (here and for all tags ID for a tag is its VID as well), are in a format like "A00001". The specific index format can be later substituted for GUID, or longer string. This format is chose for simplicity and clarity.
TTB stands for time to bypass - teh calculated time the hacker needs to traverse (bypass) this very node.
Asset Version field is reserved for future use.
has vulnerability is used to indicate that there is a vulnerability on this host.

### Asset_Type
#### Used for
Type of asset like Server, Workstation, etc.
#### Tag properties
| Field             | Type    | Null | Default | Comment |
|-------------------|---------|------|---------|---------|
| Type_ID           | string  | YES  | _EMPTY_ | _EMPTY_ |
| Type_Name         | string  | YES  | _EMPTY_ | _EMPTY_ |
| Type_Description  | string  | YES  | _EMPTY_ | _EMPTY_ |
#### Note
Asset Type IDs have the format like "DT001".

### Network_Segment
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
#### Note
Segment IDs have format like "SEG00001".

### OS_Type
#### Used for
Type of OS that asset runs, like Windows, Linux, Cisco iOS, etc.
#### Tag properties
| Field        | Type    | Null | Default | Comment |
|--------------|---------|------|---------|---------|
| OS_ID        | string  | YES  | _EMPTY_ | _EMPTY_ |
| OS_Name      | string  | YES  | _EMPTY_ | _EMPTY_ |
| OS_Version   | string  | YES  | _EMPTY_ | _EMPTY_ |
| OS_Vendor    | string  | YES  | _EMPTY_ | _EMPTY_ |
#### Note
OS ID have the formats like "OPS0001"

### tMitreMitigation
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

### tMitreState
#### Used for
This tag represents a pair of tactic/technique (or tactic/subtechnique) which can pattern (transit) to another combination of tactic/technique
#### Tag properties
| Field     | Type    | Null | Default | Comment |
|-----------|---------|------|---------|---------|
| state_id  | string  | NO   | _EMPTY_ | _EMPTY_ |
#### Note
These tMitreState pairs not necessarily exist for all combinations of tactic/technique (subtechnique). The state ID has the following format "TA0001|T1133" (Tactic|Technique or subtechnique respectfully).

### tMitreTactic
#### Used for
Represents Mitre tactic
#### Tag properties
| Field                | Type   | Null | Default | Comment              |
|----------------------|--------|------|---------|----------------------|
| Tactic_ID            | string | NO   | _EMPTY_ | Tactic ID            |
| Tactic_Name          | string | NO   | _EMPTY_ | Tactic Name          |
| Mitre_Attack_Version | string | YES  | _EMPTY_ | Mitre ATTACK version |
#### Note
Tactic ID is the same as in Mitre.

### tMitreTechnique
#### Used for
Represents Mitre Technique/Subtechnique
#### Field properties
| Field                 | Type    | Null | Default | Comment                                      |
|-----------------------|---------|------|---------|----------------------------------------------|
| Technique_ID          | string  | NO   | _EMPTY_ | MITRE Technique number                       |
| Technique_Name        | string  | NO   | _EMPTY_ | MITRE Technique name                         |
| Mitre_Attack_Version  | string  | YES  | _EMPTY_ | Mitre Attack Matrix version                  |
| rcelpe                | bool    | YES  | false   | Can be applied to host with critical vuln    |
| priority              | int8    | NO   | 4       | Priority (1-4, higher=more likely used)      |
| execution_min         | float   | NO   | 0.1667  | Minimum execution time                       |
| execution_max         | float   | NO   | 120     | Maximum execution time                       |
#### Note
Technique and subtechnique are represented by teh same type of tag, subtechnique has an extra relationship to its parent.
Technique ID is the same as in MITRE.


## Edges (12 Types) - Full Properties
Relationships for network topology, asset types, OS, how mitigation applied to assets, and relationships between tactics, techniques, subtechniques, and mitigations.

### applied_to
#### Used for
This is a relationship between tMitreMitigation and Asset tags. (tMitreMitigation --applied_to--> Asset)
#### Edge properties
| Field     | Type    | Null | Default | Comment                  |
|-----------|---------|------|---------|--------------------------|
| Version   | string  | YES  | 1.0     | Version for a project    |
| Maturity  | int16   | YES  | 100     | 0-100, higher better     |
| Active    | bool    | YES  | true    | If inactive/deprecated   |
#### Note
Version is a field for a version of an IT Infrastructure, where different versions of relationships indicate the different sets of mitigations applied to a host (IT Infrastructure asset). Later versions will have 


### belongs_to
#### Used for
This relationship is used to indicate to which network segment an asset belongs to. (Asset --belongs_to--> NetworkSegment)
#### Edge properties
| Field          | Type   | Null | Default | Comment                      |
|----------------|--------|------|---------|------------------------------|
| interface_name | string | YES  | _EMPTY_ | Physical/logical (eth0, ilo) |
| role           | string | YES  | _EMPTY_ | primary, gateway, management |
| ip_address     | string | YES  | _EMPTY_ | IP on interface              |
| vlan_id        | int16  | YES  | _EMPTY_ | VLAN ID if applicable        |
#### Note
None of the fields are used so far.

### can_be_executed_on
#### Used for
So far is not used.
#### Edge properties
No properties.

### defines_state
#### Used for 

### has_subtechnique
No properties (pure relationship edges)

### connects_to
| Field               | Type   | Null | Default | Comment                 |
|---------------------|--------|------|---------|-------------------------|
| Connection_Protocol | string | YES  | TCP     | ip, tcp, udp, icmp      |
| Connection_Port     | string | YES  | 0-65536 | Port/range: 443, 80;443 | [page:40]

### has_type
| Field         | Type     | Null | Default     | Comment |
|---------------|----------|------|-------------|---------|
| assigned_date | datetime | YES  | datetime()  | _EMPTY_ | [page:40]

### implements
| Field     | Type   | Null | Default | Comment                                                    |
|-----------|--------|------|---------|------------------------------------------------------------|
| function  | string | YES  | _EMPTY_ | Device: firewall, switch, router, wireless_ap, vpn_gateway |
| vlan_id   | int16  | YES  | _EMPTY_ | VLAN ID if applicable                                      |
| role      | string | YES  | _EMPTY_ | primary, backup, load_balanced                             |
| is_active | bool   | YES  | true    | Currently active/operational                               | [page:40]

### mitigates
| Field            | Type    | Null | Default    | Comment |
|------------------|---------|------|------------|---------|
| Use_Description  | string  | YES  | NULL       | _EMPTY_ |
| Domain           | string  | YES  | Enterprise | _EMPTY_ | [page:40]

### part_of
No properties (pure relationship edge) [page:40]

## Indexes (10 Total)
### Tag Indexes (7)
| Index Name           | On Tag             | Columns                          |
|----------------------|--------------------|----------------------------------|
| MTactic_Index        | tMitreTactic       | ["Tactic_ID"]                    |
| TecniqueIndex        | tMitreTechnique    | ["Technique_ID"]                 |
| idx_asset_any        | Asset              | ["Asset_ID", "Asset_Name"]       |
| idx_asset_type_any   | Asset_Type         | []                               |
| idx_os_type_any      | OS_Type            | []                               |
| idx_segment_any      | Network_Segment    | []                               |
| state_id_index       | tMitreState        | ["state_id"]                     | [page:40]

### Edge Indexes (3)
| Index Name      | On Edge          | Columns |
|-----------------|------------------|---------|
| ConnectsToIndex | connects_to      | []      |
| PartOfIndex     | part_of          | []      |
| SubtechIndex    | has_subtechnique | []      | [page:40]

## Design Rationale & Usage Notes
- **Model Focus:** MITRE ATT&CK integrated with cyber assets for threat simulation/vuln mgmt. Assets have risk flags (priority, vuln, TTB).
- **Performance:** Indexes on IDs/names for fast lookups; limited edge props to avoid duplication overhead.
- **Regeneration:** Re-run console script: `USE ESP01; DESCRIBE SPACE ESP01; SHOW CREATE SPACE ESP01; SHOW TAGS; SHOW EDGES; SHOW TAG INDEXES; SHOW EDGE INDEXES;` then DESCRIBE each.
- **Edit Here:** Add query examples, sample data volumes, or evolution history.

**Download:** Right-click > Save As > `ESP01_schema_complete_20260210.md` [page:40]
