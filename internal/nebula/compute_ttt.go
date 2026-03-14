package nebula

import (
	"fmt"
	"log"
	"strings"

	"ESP-data/config"
	"ESP-data/internal/store"

	nebula "github.com/vesoft-inc/nebula-go/v3"
)

type TTTResult struct {
	TechniqueID   string  // Technique_ID
	TechniqueName string  // Technique_Name
	ExecMin       float64 // execution_min from TA008
	ExecMax       float64 // execution_max from TA008
	P             int     // count of possible mitigations (mitigates edges)
	A             int     // count of active-applied mitigations on this asset
	SumMaturity   float64 // sum of Maturity * 0.01 for active-applied mitigations
	TTT           float64 // computed Time To Execute Technique (hours)
}

// computeTTTWithSession is the internal implementation of the TTT calculation.
// It reuses an existing session (Strategy A: Session Reuse) and accepts an
// osPreChecked flag (Strategy B: Eliminate Redundant OS Pre-Check).
// When osPreChecked is true, the ALG-REQ-062 OS platform query is skipped
// because the caller has already guaranteed OS validity — see
// selectFirstTacticTechniques (ALG-REQ-073) and filterByOS (ALG-REQ-062).
func computeTTTWithSession(session *nebula.Session, assetVid, techniqueVid string, osPreChecked bool) (*TTTResult, error) {

	// ALG-REQ-062: OS platform pre-check.
	// Skipped when osPreChecked == true (caller already filtered by OS).
	if !osPreChecked {
		osCheck := fmt.Sprintf(
			`MATCH (a:Asset)-[:runs_on]->(os:OS_Type)-[:represents]->(p:MitrePlatform)`+
				`<-[:can_be_executed_on]-(t:tMitreTechnique) `+
				`WHERE id(a) == "%s" AND id(t) == "%s" `+
				`RETURN count(*) AS cnt;`, assetVid, techniqueVid)

		osRS, err := session.Execute(osCheck)
		if err != nil {
			return nil, fmt.Errorf("ComputeTTT os check: %w", err)
		}
		if !osRS.IsSucceed() {
			return nil, fmt.Errorf("ComputeTTT os check: %s", osRS.GetErrorMsg())
		}
		if osRS.GetRowSize() > 0 {
			rec, _ := osRS.GetRowValuesByIndex(0)
			cnt := safeInt(rec, 0, 0)
			if cnt == 0 {
				return nil, nil
			}
		}
	}

	// ALG-REQ-064: TTT query — split into two queries to avoid
	// OPTIONAL MATCH ... WHERE which is not supported in nGQL 3.x.

	// Query 1: Get technique properties and count of possible mitigations (P)
	q1 := fmt.Sprintf(
		`MATCH (t:tMitreTechnique) WHERE id(t) == "%s" `+
			`OPTIONAL MATCH (m:tMitreMitigation)-[:mitigates]->(t) `+
			`WITH t, count(m) AS P, collect(id(m)) AS mitigation_vids `+
			`RETURN t.tMitreTechnique.Technique_ID AS technique_id, `+
			`  t.tMitreTechnique.Technique_Name AS technique_name, `+
			`  t.tMitreTechnique.execution_min AS exec_min, `+
			`  t.tMitreTechnique.execution_max AS exec_max, `+
			`  P AS possible_mitigations, `+
			`  mitigation_vids AS mit_vids;`,
		techniqueVid)

	rs1, err := session.Execute(q1)
	if err != nil {
		return nil, fmt.Errorf("ComputeTTT query1: %w", err)
	}
	if !rs1.IsSucceed() {
		return nil, fmt.Errorf("ComputeTTT query1: %s", rs1.GetErrorMsg())
	}
	if rs1.GetRowSize() == 0 {
		return nil, fmt.Errorf("ComputeTTT: technique %s not found", techniqueVid)
	}

	rec1, _ := rs1.GetRowValuesByIndex(0)
	result := &TTTResult{
		TechniqueID:   safeString(rec1, 0),
		TechniqueName: safeString(rec1, 1),
		ExecMin:       safeFloat64(rec1, 2, 0.1667),
		ExecMax:       safeFloat64(rec1, 3, 120.0),
		P:             safeInt(rec1, 4, 0),
	}

	// Extract mitigation VIDs from the list column
	mitVids, _ := extractStringList(rec1, 5)

	// Query 2: Count active-applied mitigations on this asset from the P set.
	if len(mitVids) > 0 {
		quotedVids := make([]string, len(mitVids))
		for i, v := range mitVids {
			quotedVids[i] = fmt.Sprintf(`"%s"`, v)
		}
		vidListStr := strings.Join(quotedVids, ",")

		q2 := fmt.Sprintf(
			`MATCH (m2:tMitreMitigation)-[ap:applied_to]->(a:Asset) `+
				`WHERE id(a) == "%s" AND id(m2) IN [%s] AND ap.Active == true `+
				`RETURN count(m2) AS A, `+
				`  CASE WHEN count(m2) > 0 THEN sum(ap.Maturity) ELSE 0 END AS maturity_sum;`,
			assetVid, vidListStr)

		rs2, err := session.Execute(q2)
		if err != nil {
			log.Printf("nebula: ComputeTTT query2 failed: %v", err)
		} else if !rs2.IsSucceed() {
			log.Printf("nebula: ComputeTTT query2: %s", rs2.GetErrorMsg())
		} else if rs2.GetRowSize() > 0 {
			rec2, _ := rs2.GetRowValuesByIndex(0)
			result.A = safeInt(rec2, 0, 0)
			result.SumMaturity = safeFloat64(rec2, 1, 0.0)
		}
	}

	// ALG-REQ-060: TTT formula
	if result.P == 0 {
		result.TTT = result.ExecMin
	} else if result.A == result.P {
		result.TTT = result.ExecMax
	} else {
		maturityFactor := result.SumMaturity * 0.01
		pf := float64(result.P)
		result.TTT = result.ExecMin + (result.ExecMax-result.ExecMin)*(maturityFactor/pf)
	}

	return result, nil
}

// ComputeTTT computes the Time To Execute a Technique for a given (asset, technique) pair.
// Implements ALG-REQ-060 through ALG-REQ-066.
// Returns (nil, nil) when the technique is not executable on the asset's OS platform (ALG-REQ-062).
// This public wrapper opens its own session and performs the full OS pre-check,
// preserving backward compatibility for standalone callers.
func ComputeTTT(pool *nebula.ConnectionPool, cfg *config.Config, assetVid, techniqueVid string) (*TTTResult, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()
	return computeTTTWithSession(session, assetVid, techniqueVid, false)
}

// computeBatchTTT computes TTT for ALL technique candidates in a single batch
// using exactly 2 nGQL queries (regardless of candidate count).
// Strategy C: Batch ComputeTTT per Tactic.
//
// Query 1 fetches technique properties (exec_min, exec_max) and possible
// mitigation VIDs for every candidate technique in one round-trip.
// Query 2 fetches active-applied mitigations on the asset for ALL unique
// mitigation VIDs collected from Q1.
//
// The APP layer then maps Q2 results back to each technique and applies
// the ALG-REQ-060 formula in-process.
//
// Precondition: all candidates have already passed OS filtering (ALG-REQ-062)
// so no OS pre-check is needed here.
func computeBatchTTT(session *nebula.Session, assetVid string, candidates []techniqueCandidate, audit *store.AuditBuffer) error {
	if len(candidates) == 0 {
		return nil
	}

	// Build the IN list of technique VIDs for Q1
	techVids := make([]string, len(candidates))
	for i, c := range candidates {
		techVids[i] = fmt.Sprintf(`"%s"`, c.TechniqueID)
	}

	q1 := fmt.Sprintf(
		`MATCH (t:tMitreTechnique) `+
			`WHERE id(t) IN [%s] `+
			`OPTIONAL MATCH (t)<-[:mitigates]-(m_all:tMitreMitigation) `+
			`WITH t, count(m_all) AS P, collect(id(m_all)) AS mit_vids `+
			`RETURN id(t) AS technique_vid, `+
			`  t.tMitreTechnique.execution_min AS exec_min, `+
			`  t.tMitreTechnique.execution_max AS exec_max, `+
			`  P AS possible_count, `+
			`  mit_vids AS mitigation_vids;`,
		strings.Join(techVids, ", "))

	rs1, err := session.Execute(q1)
	if err != nil {
		return fmt.Errorf("computeBatchTTT Q1: %w", err)
	}
	if !rs1.IsSucceed() {
		return fmt.Errorf("computeBatchTTT Q1: %s", rs1.GetErrorMsg())
	}

	// Parse Q1 results into a map keyed by technique VID
	type techInfo struct {
		ExecMin float64
		ExecMax float64
		P       int
		MitVids []string
	}
	techMap := make(map[string]*techInfo)
	allMitVidsSet := make(map[string]bool)

	for i := 0; i < rs1.GetRowSize(); i++ {
		rec, _ := rs1.GetRowValuesByIndex(i)
		vid := safeString(rec, 0)
		if vid == "" {
			continue
		}
		info := &techInfo{
			ExecMin: safeFloat64(rec, 1, 0.1667),
			ExecMax: safeFloat64(rec, 2, 120.0),
			P:       safeInt(rec, 3, 0),
		}
		mitVids, _ := extractStringList(rec, 4)
		info.MitVids = mitVids
		techMap[vid] = info

		for _, mv := range mitVids {
			allMitVidsSet[mv] = true
		}
	}

	// Build set of active-applied mitigations from Q2
	// Key: mitigation VID -> maturity (int)
	activeMitMap := make(map[string]int)

	if len(allMitVidsSet) > 0 {
		quotedMitVids := make([]string, 0, len(allMitVidsSet))
		for mv := range allMitVidsSet {
			quotedMitVids = append(quotedMitVids, fmt.Sprintf(`"%s"`, mv))
		}

		q2 := fmt.Sprintf(
			`MATCH (m:tMitreMitigation)-[ap:applied_to]->(a:Asset) `+
				`WHERE id(a) == "%s" AND id(m) IN [%s] AND ap.Active == true `+
				`RETURN id(m) AS mit_vid, ap.Maturity AS maturity;`,
			assetVid, strings.Join(quotedMitVids, ", "))

		rs2, err := session.Execute(q2)
		if err != nil {
			return fmt.Errorf("computeBatchTTT Q2: %w", err)
		}
		if !rs2.IsSucceed() {
			return fmt.Errorf("computeBatchTTT Q2: %s", rs2.GetErrorMsg())
		}

		for i := 0; i < rs2.GetRowSize(); i++ {
			rec, _ := rs2.GetRowValuesByIndex(i)
			mitVid := safeString(rec, 0)
			maturity := safeInt(rec, 1, 0)
			if mitVid != "" {
				activeMitMap[mitVid] = maturity
			}
		}
	}

	// Apply ALG-REQ-060 formula to each candidate in-place
	for j := range candidates {
		info, ok := techMap[candidates[j].TechniqueID]
		if !ok {
			candidates[j].TTT = 999999.0
			continue
		}

		P := info.P
		execMin := info.ExecMin
		execMax := info.ExecMax

		if P == 0 {
			candidates[j].TTT = execMin
			if audit != nil {
				audit.TTTDetails = append(audit.TTTDetails, store.TTTDetailRecord{
					TechniqueVid:  candidates[j].TechniqueID,
					ExecMin:       execMin,
					ExecMax:       execMax,
					PossibleCount: P,
					AppliedCount:  0,
					MaturityFactor: 0,
					FormulaCase:  "no_mitigations",
					TTTHours:     execMin,
				})
			}
			continue
		}

		// Count A and sum maturity_factor for this technique's mitigations
		A := 0
		maturityFactor := 0.0
		for _, mv := range info.MitVids {
			if mat, found := activeMitMap[mv]; found {
				A++
				maturityFactor += 0.01 * float64(mat)
			}
		}

		var formulaCase string
		var tttVal float64
		if A == P {
			tttVal = execMax
			formulaCase = "full_coverage"
		} else {
			tttVal = execMin + (maturityFactor*(execMax-execMin))/float64(P)
			formulaCase = "partial"
		}
		candidates[j].TTT = tttVal

		if audit != nil {
			audit.TTTDetails = append(audit.TTTDetails, store.TTTDetailRecord{
				TechniqueVid:   candidates[j].TechniqueID,
				ExecMin:        execMin,
				ExecMax:        execMax,
				PossibleCount:  P,
				AppliedCount:   A,
				MaturityFactor: maturityFactor,
				FormulaCase:    formulaCase,
				TTTHours:       tttVal,
			})
		}
	}

	return nil
}
