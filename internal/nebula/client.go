package nebula

import (
	"fmt"
	"log"

	"ESP-data/config"

	nebula "github.com/vesoft-inc/nebula-go/v3"
)

// the properties the front-end needs for colouring, labels, and badges.
type AssetRow struct {
	SrcAssetID          string
	SrcAssetName        string
	SrcIsEntrance       bool
	SrcIsTarget         bool
	SrcPriority         int
	SrcHasVulnerability bool
	SrcAssetType        string

	DstAssetID          string
	DstAssetName        string
	DstIsEntrance       bool
	DstIsTarget         bool
	DstPriority         int
	DstHasVulnerability bool
	DstAssetType        string
}

// NewPool creates and initializes a Nebula ConnectionPool.
// The caller is responsible for calling pool.Close() when done.
// This satisfies REQ-121: use Vesoft's Go client libraries.

func NewPool(cfg *config.Config) *nebula.ConnectionPool {
	hostAddress := nebula.HostAddress{
		Host: cfg.NebulaHost,
		Port: cfg.NebulaPort,
	}

	hostList := []nebula.HostAddress{hostAddress}
	poolConfig := nebula.GetDefaultConf()
	logger := nebula.DefaultLogger{}
	pool, err := nebula.NewConnectionPool(hostList, poolConfig, logger)
	if err != nil {
		log.Fatalf("nebula: failed to create pool: %v", err)
	}

	log.Printf("nebula: pool created for %s:%d", cfg.NebulaHost, cfg.NebulaPort)
	return pool
}

// openSession is a small helper that obtains a session and switches to

func openSession(pool *nebula.ConnectionPool, cfg *config.Config) (*nebula.Session, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	res, err := session.Execute(useStmt)
	if err != nil {
		session.Release()
		return nil, fmt.Errorf("failed to USE space: %w", err)
	}
	if !res.IsSucceed() {
		session.Release()
		return nil, fmt.Errorf("USE space failed: %s", res.GetErrorMsg())
	}
	return session, nil
}


func safeString(record *nebula.Record, idx int) string {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return ""
	}
	s, err := val.AsString()
	if err != nil {
		return ""
	}
	return s
}

// safeBool extracts a bool from a ResultSet value, returning false on error.

func safeBool(record *nebula.Record, idx int) bool {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return false
	}
	b, err := val.AsBool()
	if err != nil {
		return false
	}
	return b
}

// safeInt extracts an int from a ResultSet value, returning the supplied
// default on error (e.g. 4 for priority).

func safeInt(record *nebula.Record, idx int, def int) int {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return def
	}
	n, err := val.AsInt()
	if err != nil {
		return def
	}
	return int(n)
}

// safeFloat64 extracts a float64 from a ResultSet value.
// NebulaGraph may return integer type for float columns when the value
// has no fractional part (e.g. 120 instead of 120.0), so we try AsFloat
// first, then fall back to AsInt conversion.

func safeFloat64(record *nebula.Record, idx int, def float64) float64 {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return def
	}
	f, err := val.AsFloat()
	if err == nil {
		return f
	}
	n, err := val.AsInt()
	if err == nil {
		return float64(n)
	}
	return def
}

// QueryAssets executes the enriched connectivity query specified in REQ-020.
// MATCH is used here because multi-hop property retrieval across asset
// pairs and their types is significantly cleaner than chained GO statements

// extractStringList extracts a list of strings from a ResultSet column.
func extractStringList(record *nebula.Record, idx int) ([]string, error) {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return nil, err
	}
	list, err := val.AsList()
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(list))
	for _, v := range list {
		s, err := v.AsString()
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}

// extractIntList extracts a list of ints from a ResultSet column.

func extractIntList(record *nebula.Record, idx int) ([]int, error) {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return nil, err
	}
	list, err := val.AsList()
	if err != nil {
		return nil, err
	}
	result := make([]int, 0, len(list))
	for _, v := range list {
		n, err := v.AsInt()
		if err != nil {
			return nil, err
		}
		result = append(result, int(n))
	}
	return result, nil
}


func extractFloatList(record *nebula.Record, idx int) ([]float64, error) {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return nil, err
	}
	list, err := val.AsList()
	if err != nil {
		return nil, err
	}
	result := make([]float64, 0, len(list))
	for _, v := range list {
		// NebulaGraph may return int or float depending on the stored type
		if f, err := v.AsFloat(); err == nil {
			result = append(result, f)
		} else if n, err := v.AsInt(); err == nil {
			result = append(result, float64(n))
		} else {
			return nil, fmt.Errorf("cannot convert list element to float64")
		}
	}
	return result, nil
}

// safeInt64 extracts an int64 from a ResultSet value, returning 0 on error.

func safeInt64(record *nebula.Record, idx int) int64 {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return 0
	}
	n, err := val.AsInt()
	if err != nil {
		return 0
	}
	return n
}

