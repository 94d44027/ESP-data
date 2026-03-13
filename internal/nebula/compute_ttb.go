package nebula

import (
	"fmt"
	"log"
	"strings"

	"ESP-data/config"

	nebula "github.com/vesoft-inc/nebula-go/v3"
)

// ======================================================================================================
// TTB Calculation — ALG-REQ-070 through ALG-REQ-080
// ======================================================================================================

// TTBParams holds configurable parameters for the TTB calculation (ALG-REQ-071, 072, 075).
type TTBParams struct {
	OrientationTime   float64
	SwitchoverTime    float64
	PriorityTolerance int
}

// TTBLogEntry records one step of the tactic chain traversal (ALG-REQ-079).
type TTBLogEntry struct {
	TacticID        string  `json:"tactic_id"`
	TacticName      string  `json:"tactic_name"`
	TechniqueID     *string `json:"technique_id"`
	TechniqueName   *string `json:"technique_name"`
	TTT             float64 `json:"ttt"`
	CandidatesCount int     `json:"candidates_count"`
}

// TTBResult is the output of ComputeTTB (ALG-REQ-070).
type TTBResult struct {
	TTB float64       `json:"ttb"`
	Log []TTBLogEntry `json:"log"`
}

// techniqueCandidate holds one technique row returned by the selection queries.
type techniqueCandidate struct {
	TechniqueID    string
	TechniqueName  string
	Priority       int
	VulnApplicable bool
	TTT            float64
}

// getOrderedTactics returns the tactic VIDs for a chain, ordered by chain_includes rank.
func getOrderedTactics(session *nebula.Session, chainVid string) ([]struct{ VID, TacticID, TacticName string }, error) {
	query := fmt.Sprintf(
		`GO FROM "%s" OVER chain_includes `+
			`YIELD chain_includes._rank AS rank, id($$) AS tactic_vid `+
			`| ORDER BY $-.rank ASC;`, chainVid)

	rs, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("getOrderedTactics GO: %w", err)
	}
	if !rs.IsSucceed() {
		return nil, fmt.Errorf("getOrderedTactics GO: %s", rs.GetErrorMsg())
	}

	type tacticRef struct{ VID, TacticID, TacticName string }
	var tactics []tacticRef

	for i := 0; i < rs.GetRowSize(); i++ {
		record, _ := rs.GetRowValuesByIndex(i)
		vid := safeString(record, 1)
		if vid == "" {
			continue
		}
		tactics = append(tactics, tacticRef{VID: vid})
	}

	if len(tactics) == 0 {
		return nil, fmt.Errorf("getOrderedTactics: chain %s has no tactics", chainVid)
	}

	var vids []string
	for _, t := range tactics {
		vids = append(vids, fmt.Sprintf(`"%s"`, t.VID))
	}
	fetchQ := fmt.Sprintf(
		`FETCH PROP ON tMitreTactic %s `+
			`YIELD tMitreTactic.Tactic_ID AS tid, tMitreTactic.Tactic_Name AS tname;`,
		strings.Join(vids, ","))

	fs, err := session.Execute(fetchQ)
	if err != nil {
		return nil, fmt.Errorf("getOrderedTactics FETCH: %w", err)
	}
	if !fs.IsSucceed() {
		return nil, fmt.Errorf("getOrderedTactics FETCH: %s", fs.GetErrorMsg())
	}

	info := make(map[string]tacticRef)
	for i := 0; i < fs.GetRowSize(); i++ {
		record, _ := fs.GetRowValuesByIndex(i)
		vid := safeString(record, 0)
		info[vid] = tacticRef{
			VID:        vid,
			TacticID:   safeString(record, 1),
			TacticName: safeString(record, 2),
		}
	}

	result := make([]struct{ VID, TacticID, TacticName string }, 0, len(tactics))
	for _, t := range tactics {
		if inf, ok := info[t.VID]; ok {
			result = append(result, struct{ VID, TacticID, TacticName string }{
				VID: inf.VID, TacticID: inf.TacticID, TacticName: inf.TacticName,
			})
		} else {
			result = append(result, struct{ VID, TacticID, TacticName string }{
				VID: t.VID, TacticID: t.VID, TacticName: t.VID,
			})
		}
	}
	return result, nil
}

// selectFirstTacticTechniques implements ALG-REQ-073.
func selectFirstTacticTechniques(session *nebula.Session, assetVid, tacticVid string) ([]techniqueCandidate, error) {
	query := fmt.Sprintf(
		`MATCH (a:Asset)-[:runs_on]->(os:OS_Type)-[:represents]->(p:MitrePlatform)`+
			`<-[:can_be_executed_on]-(t:tMitreTechnique)-[:part_of]->(tac:tMitreTactic) `+
			`WHERE id(a) == "%s" AND id(tac) == "%s" `+
			`WITH collect({ `+
			`  tid: t.tMitreTechnique.Technique_ID, `+
			`  tname: t.tMitreTechnique.Technique_Name, `+
			`  pri: t.tMitreTechnique.priority, `+
			`  rcelpe: t.tMitreTechnique.rcelpe `+
			`}) AS rows `+
			`UNWIND rows AS r `+
			`RETURN DISTINCT r.tid AS technique_id, `+
			`       r.tname AS technique_name, `+
			`       r.pri AS technique_priority, `+
			`       r.rcelpe AS vuln_applicable `+
			`ORDER BY technique_priority DESC, technique_id;`,
		assetVid, tacticVid)

	rs, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("selectFirstTacticTechniques: %w", err)
	}
	if !rs.IsSucceed() {
		return nil, fmt.Errorf("selectFirstTacticTechniques: %s", rs.GetErrorMsg())
	}

	return parseTechniqueCandidates(rs)
}

// selectPatternTechniques implements ALG-REQ-076.
func selectPatternTechniques(session *nebula.Session, previousTacticID, fastestTechniqueID, currentTacticID string) ([]techniqueCandidate, error) {
	stateID := previousTacticID + "|" + fastestTechniqueID
	query := fmt.Sprintf(
		`MATCH (src_state:tMitreState)-[:patterns_to]->(dst_state:tMitreState) `+
			`WHERE id(src_state) == "%s" `+
			`WITH dst_state.tMitreState.state_id AS dst_id `+
			`WITH dst_id, split(dst_id, "|") AS parts `+
			`WHERE size(parts) == 2 AND parts[0] == "%s" `+
			`WITH parts[1] AS technique_vid `+
			`MATCH (t:tMitreTechnique) `+
			`WHERE id(t) == technique_vid `+
			`RETURN t.tMitreTechnique.Technique_ID AS technique_id, `+
			`       t.tMitreTechnique.Technique_Name AS technique_name, `+
			`       t.tMitreTechnique.priority AS technique_priority, `+
			`       t.tMitreTechnique.rcelpe AS vuln_applicable `+
			`ORDER BY technique_priority DESC, technique_id;`,
		stateID, currentTacticID)

	rs, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("selectPatternTechniques: %w", err)
	}
	if !rs.IsSucceed() {
		return nil, fmt.Errorf("selectPatternTechniques: %s", rs.GetErrorMsg())
	}

	return parseTechniqueCandidates(rs)
}

// filterByOS applies ALG-REQ-062 OS platform filter to pattern-derived candidates.
func filterByOS(session *nebula.Session, candidates []techniqueCandidate, assetVid string) ([]techniqueCandidate, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	query := fmt.Sprintf(
		`MATCH (a:Asset)-[:runs_on]->(os:OS_Type)-[:represents]->(p:MitrePlatform)`+
			`<-[:can_be_executed_on]-(t:tMitreTechnique) `+
			`WHERE id(a) == "%s" `+
			`RETURN DISTINCT t.tMitreTechnique.Technique_ID AS technique_id;`, assetVid)

	rs, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("filterByOS: %w", err)
	}
	if !rs.IsSucceed() {
		return nil, fmt.Errorf("filterByOS: %s", rs.GetErrorMsg())
	}

	allowed := make(map[string]bool)
	for i := 0; i < rs.GetRowSize(); i++ {
		record, _ := rs.GetRowValuesByIndex(i)
		allowed[safeString(record, 0)] = true
	}

	var filtered []techniqueCandidate
	for _, c := range candidates {
		if allowed[c.TechniqueID] {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

// filterByVulnerability implements ALG-REQ-074.
func filterByVulnerability(candidates []techniqueCandidate, hasVulnerability bool) []techniqueCandidate {
	if !hasVulnerability {
		return candidates
	}
	var vulnCandidates []techniqueCandidate
	for _, c := range candidates {
		if c.VulnApplicable {
			vulnCandidates = append(vulnCandidates, c)
		}
	}
	if len(vulnCandidates) > 0 {
		return vulnCandidates
	}
	return candidates
}

// filterByPriority implements ALG-REQ-075.
func filterByPriority(candidates []techniqueCandidate, tolerance int) []techniqueCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	maxPri := 0
	for _, c := range candidates {
		if c.Priority > maxPri {
			maxPri = c.Priority
		}
	}
	threshold := maxPri - tolerance
	if threshold < 1 {
		threshold = 1
	}
	var filtered []techniqueCandidate
	for _, c := range candidates {
		if c.Priority >= threshold {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// selectFastest implements ALG-REQ-077 (argmin TTT with deterministic tie-breaking).
func selectFastest(candidates []techniqueCandidate) *techniqueCandidate {
	if len(candidates) == 0 {
		return nil
	}
	best := &candidates[0]
	for i := 1; i < len(candidates); i++ {
		c := &candidates[i]
		if c.TTT < best.TTT {
			best = c
		} else if c.TTT == best.TTT {
			if c.Priority > best.Priority {
				best = c
			} else if c.Priority == best.Priority && c.TechniqueID < best.TechniqueID {
				best = c
			}
		}
	}
	return best
}

// parseTechniqueCandidates converts a ResultSet into techniqueCandidate slices.
func parseTechniqueCandidates(rs *nebula.ResultSet) ([]techniqueCandidate, error) {
	var candidates []techniqueCandidate
	for i := 0; i < rs.GetRowSize(); i++ {
		record, _ := rs.GetRowValuesByIndex(i)
		tid := safeString(record, 0)
		if tid == "" {
			continue
		}
		candidates = append(candidates, techniqueCandidate{
			TechniqueID:    tid,
			TechniqueName:  safeString(record, 1),
			Priority:       safeInt(record, 2, 4),
			VulnApplicable: safeBool(record, 3),
		})
	}
	return candidates, nil
}

// queryAssetHasVulnerability fetches the has_vulnerability flag for an asset.
func queryAssetHasVulnerability(session *nebula.Session, assetVid string) (bool, error) {
	query := fmt.Sprintf(`FETCH PROP ON Asset "%s" YIELD Asset.has_vulnerability AS hv;`, assetVid)
	rs, err := session.Execute(query)
	if err != nil {
		return false, fmt.Errorf("queryAssetHasVulnerability: %w", err)
	}
	if !rs.IsSucceed() {
		return false, fmt.Errorf("queryAssetHasVulnerability: %s", rs.GetErrorMsg())
	}
	if rs.GetRowSize() == 0 {
		return false, nil
	}
	record, _ := rs.GetRowValuesByIndex(0)
	return safeBool(record, 1), nil
}

// ComputeTTB implements the full TTB calculation algorithm (ALG-REQ-070).

func ComputeTTB(pool *nebula.ConnectionPool, cfg *config.Config, assetVid, chainVid string, params TTBParams) (*TTBResult, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	tactics, err := getOrderedTactics(session, chainVid)
	if err != nil {
		return nil, fmt.Errorf("ComputeTTB: %w", err)
	}

	hasVuln, err := queryAssetHasVulnerability(session, assetVid)
	if err != nil {
		log.Printf("nebula: ComputeTTB warning — could not fetch has_vulnerability for %s: %v", assetVid, err)
	}

	ttb := params.OrientationTime
	var ttbLog []TTBLogEntry
	var previousTacticID string
	var fastestTechID *string
	techniqueCount := 0

	for i, tactic := range tactics {
		var candidates []techniqueCandidate
		usedFallback := false

		if i == 0 || fastestTechID == nil {
			candidates, err = selectFirstTacticTechniques(session, assetVid, tactic.VID)
			if err != nil {
				log.Printf("nebula: ComputeTTB selectFirstTacticTechniques failed for tactic %s: %v", tactic.TacticID, err)
				candidates = nil
			}
			if i > 0 {
				usedFallback = true
			}
		} else {
			candidates, err = selectPatternTechniques(session, previousTacticID, *fastestTechID, tactic.TacticID)
			if err != nil {
				log.Printf("nebula: ComputeTTB selectPatternTechniques failed for %s|%s -> %s: %v",
					previousTacticID, *fastestTechID, tactic.TacticID, err)
				candidates = nil
			}

			if len(candidates) > 0 {
				candidates, err = filterByOS(session, candidates, assetVid)
				if err != nil {
					log.Printf("nebula: ComputeTTB filterByOS failed: %v", err)
				}
			}

			if len(candidates) == 0 && !usedFallback {
				candidates, err = selectFirstTacticTechniques(session, assetVid, tactic.VID)
				if err != nil {
					log.Printf("nebula: ComputeTTB fallback selectFirstTacticTechniques failed for tactic %s: %v", tactic.TacticID, err)
					candidates = nil
				}
				usedFallback = true
			}
		}

		candidates = filterByVulnerability(candidates, hasVuln)
		candidates = filterByPriority(candidates, params.PriorityTolerance)
		candidatesCount := len(candidates)

		if candidatesCount == 0 {
			log.Printf("nebula: ComputeTTB — empty technique set for tactic %s (%s) on asset %s",
				tactic.TacticID, tactic.TacticName, assetVid)
			ttbLog = append(ttbLog, TTBLogEntry{
				TacticID:        tactic.TacticID,
				TacticName:      tactic.TacticName,
				TechniqueID:     nil,
				TechniqueName:   nil,
				TTT:             0.0,
				CandidatesCount: 0,
			})
			previousTacticID = tactic.TacticID
			fastestTechID = nil
			continue
		}

		// Strategy C: Batch ComputeTTT — 2 queries per tactic instead of 2×N
		if err := computeBatchTTT(session, assetVid, candidates); err != nil {
			log.Printf("nebula: ComputeTTB computeBatchTTT failed for tactic %s: %v", tactic.TacticID, err)
			for j := range candidates {
				candidates[j].TTT = 999999.0
			}
		}

		fastest := selectFastest(candidates)
		if fastest == nil {
			ttbLog = append(ttbLog, TTBLogEntry{
				TacticID:        tactic.TacticID,
				TacticName:      tactic.TacticName,
				TechniqueID:     nil,
				TechniqueName:   nil,
				TTT:             0.0,
				CandidatesCount: candidatesCount,
			})
			previousTacticID = tactic.TacticID
			fastestTechID = nil
			continue
		}

		if techniqueCount > 0 {
			ttb += params.SwitchoverTime
		}
		ttb += fastest.TTT
		techniqueCount++

		tid := fastest.TechniqueID
		tname := fastest.TechniqueName
		ttbLog = append(ttbLog, TTBLogEntry{
			TacticID:        tactic.TacticID,
			TacticName:      tactic.TacticName,
			TechniqueID:     &tid,
			TechniqueName:   &tname,
			TTT:             fastest.TTT,
			CandidatesCount: candidatesCount,
		})

		previousTacticID = tactic.TacticID
		fastestTechID = &tid
	}

	return &TTBResult{TTB: ttb, Log: ttbLog}, nil
}

