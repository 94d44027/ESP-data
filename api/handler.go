package api

import (
	"fmt"
	"regexp"
	"strings"
)

// validAssetID matches the Asset ID format defined in the schema (e.g. "A00012").
// Used by REQ-025 to reject malformed or injected input before it reaches nGQL.
var validAssetID = regexp.MustCompile(`^A\d{4,5}$`)

// validMitigationID matches the Mitigation ID format (e.g. "M1020").
// Used by REQ-038 to reject malformed input before it reaches nGQL.
var validMitigationID = regexp.MustCompile(`^M\d{4}$`)

// validMaturity defines the allowed maturity values per REQ-039.
var validMaturity = map[int]bool{25: true, 50: true, 80: true, 100: true}

// ============================================================
// URL path helpers
// ============================================================

// extractAssetID pulls the asset ID from the given URL path segment,
// validates it against the expected format (REQ-025), and returns
// it or a descriptive error for an HTTP 400 response.
func extractAssetID(urlPath string, segmentIndex int) (string, error) {
	parts := strings.Split(urlPath, "/")
	if len(parts) <= segmentIndex {
		return "", fmt.Errorf("missing asset ID in path")
	}

	assetID := parts[segmentIndex]
	if assetID == "" {
		return "", fmt.Errorf("asset ID cannot be empty")
	}

	if !validAssetID.MatchString(assetID) {
		return "", fmt.Errorf("invalid asset ID format: %q (expected pattern like A00012)", assetID)
	}

	return assetID, nil
}

// extractMitigationID pulls the mitigation ID from the given URL path segment,
// validates it against the expected format (REQ-038), and returns it or a
// descriptive error for an HTTP 400 response.
func extractMitigationID(urlPath string, segmentIndex int) (string, error) {
	parts := strings.Split(urlPath, "/")
	if len(parts) <= segmentIndex {
		return "", fmt.Errorf("missing mitigation ID in path")
	}

	mitigationID := parts[segmentIndex]
	if mitigationID == "" {
		return "", fmt.Errorf("mitigation ID cannot be empty")
	}

	if !validMitigationID.MatchString(mitigationID) {
		return "", fmt.Errorf("invalid mitigation ID format: %q (expected pattern like M1020)", mitigationID)
	}

	return mitigationID, nil
}
