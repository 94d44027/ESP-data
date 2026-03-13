package nebula

import (
	"fmt"
	"log"
	"time"

	"ESP-data/config"

	nebula "github.com/vesoft-inc/nebula-go/v3"
)

// ============================================================
// Mitigations queries (REQ-033, REQ-034, REQ-035, REQ-036)
// ============================================================

// QueryMitigations fetches all MITRE mitigations for the editor dropdown (REQ-033).
// Uses pure nGQL LOOKUP per REQ-243.
func QueryMitigations(pool *nebula.ConnectionPool, cfg *config.Config) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-033 query — pure nGQL per REQ-243
	query := `LOOKUP ON tMitreMitigation
YIELD
  id(vertex) AS vid,
  tMitreMitigation.Mitigation_ID AS mitigation_id,
  tMitreMitigation.Mitigation_Name AS mitigation_name;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryMitigations executing LOOKUP query", queryStart.Format("15:04:05.000"))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryMitigations completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	mitigations := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		mitigations = append(mitigations, map[string]interface{}{
			"mitigation_id":   safeString(record, 1),
			"mitigation_name": safeString(record, 2),
		})
	}

	log.Printf("nebula: QueryMitigations returned %d mitigations", len(mitigations))
	return mitigations, nil
}

// QueryAssetMitigations fetches all mitigations applied to a specific asset (REQ-034).
// MATCH is used because traversing applied_to edge from tMitreMitigation to Asset
// with property retrieval on both the edge and the source vertex is cleaner with
// MATCH than with chained GO/FETCH statements (REQ-244 justification).
func QueryAssetMitigations(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-034 query — MATCH per REQ-244 justification
	query := fmt.Sprintf(`MATCH (m:tMitreMitigation)-[e:applied_to]->(a:Asset)
WHERE a.Asset.Asset_ID == "%s"
RETURN m.tMitreMitigation.Mitigation_ID AS mitigation_id,
  m.tMitreMitigation.Mitigation_Name AS mitigation_name,
  e.Maturity AS maturity,
  e.Active AS active;`, assetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssetMitigations executing MATCH query for asset %s",
		queryStart.Format("15:04:05.000"), assetID)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssetMitigations completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	mitigations := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		mitigations = append(mitigations, map[string]interface{}{
			"mitigation_id":   safeString(record, 0),
			"mitigation_name": safeString(record, 1),
			"maturity":        safeInt(record, 2, 100),
			"active":          safeBool(record, 3),
		})
	}

	log.Printf("nebula: QueryAssetMitigations returned %d mitigations for asset %s", len(mitigations), assetID)
	return mitigations, nil
}

// UpsertMitigation adds or updates an applied_to edge between a mitigation and an asset (REQ-035).
// Uses pure nGQL UPSERT EDGE per REQ-243.
func UpsertMitigation(pool *nebula.ConnectionPool, cfg *config.Config, mitigationID, assetID string, maturity int, active bool) error {
	session, err := openSession(pool, cfg)
	if err != nil {
		return err
	}
	defer session.Release()

	// REQ-035 query — pure nGQL per REQ-243
	// @0 rank is fixed per schema ED001
	// Version is hardcoded to "1.0" — version-aware modelling is deferred
	query := fmt.Sprintf(`UPSERT EDGE ON applied_to "%s" -> "%s" @0
SET Version = "1.0", Maturity = %d, Active = %v;`, mitigationID, assetID, maturity, active)

	queryStart := time.Now()
	log.Printf("[%s] nebula: UpsertMitigation executing for %s -> %s (maturity=%d, active=%v)",
		queryStart.Format("15:04:05.000"), mitigationID, assetID, maturity, active)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: UpsertMitigation completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return fmt.Errorf("upsert execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return fmt.Errorf("upsert failed: %s", resultSet.GetErrorMsg())
	}

	return nil
}

// DeleteMitigation removes an applied_to edge between a mitigation and an asset (REQ-036).
// Uses pure nGQL DELETE EDGE per REQ-243.
// Caution: only deletes rank 0 — correct per current design (REQ-035 note).
func DeleteMitigation(pool *nebula.ConnectionPool, cfg *config.Config, mitigationID, assetID string) error {
	session, err := openSession(pool, cfg)
	if err != nil {
		return err
	}
	defer session.Release()

	// REQ-036 query — pure nGQL per REQ-243
	query := fmt.Sprintf(`DELETE EDGE applied_to "%s" -> "%s" @0;`, mitigationID, assetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: DeleteMitigation executing for %s -> %s",
		queryStart.Format("15:04:05.000"), mitigationID, assetID)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: DeleteMitigation completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return fmt.Errorf("delete execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return fmt.Errorf("delete failed: %s", resultSet.GetErrorMsg())
	}

	return nil
}

