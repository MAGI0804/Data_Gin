package geocoder

import (
	"sort"
	"strings"
)

const (
	autoConfirmMinimumScore = 85.0
	autoConfirmMinimumLead  = 15.0
)

type ScoreInput struct {
	Name         string
	Aliases      []string
	Province     string
	City         string
	District     string
	Street       string
	StreetNumber string
	Address      string
}

type CandidateScore struct {
	Index   int
	Score   float64
	Reasons []string
}

func ScoreCandidates(input ScoreInput, candidates []Candidate) []CandidateScore {
	results := make([]CandidateScore, len(candidates))
	for index, candidate := range candidates {
		results[index] = scoreCandidate(input, candidate, len(candidates))
		results[index].Index = index
	}
	return results
}

func AutoConfirmCandidate(scores []CandidateScore) (int, bool) {
	if len(scores) == 0 {
		return 0, false
	}
	ranked := append([]CandidateScore(nil), scores...)
	sort.SliceStable(ranked, func(left, right int) bool { return ranked[left].Score > ranked[right].Score })
	if ranked[0].Score < autoConfirmMinimumScore {
		return 0, false
	}
	if candidateScoreHasReason(ranked[0], "LOW_PRECISION") {
		return 0, false
	}
	if len(ranked) > 1 && ranked[0].Score-ranked[1].Score < autoConfirmMinimumLead {
		return 0, false
	}
	return ranked[0].Index, true
}

func candidateScoreHasReason(score CandidateScore, want string) bool {
	for _, reason := range score.Reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func scoreCandidate(input ScoreInput, candidate Candidate, candidateCount int) CandidateScore {
	var score float64
	reasons := make([]string, 0, 8)
	applyExactRegionScore := func(label, expected, actual string, matched, mismatched float64) {
		expected, actual = normalizeGeocodeText(expected), normalizeGeocodeText(actual)
		if expected == "" || actual == "" {
			return
		}
		if expected == actual {
			score += matched
			reasons = append(reasons, label+"_MATCH")
		} else {
			score += mismatched
			reasons = append(reasons, label+"_MISMATCH")
		}
	}

	applyExactRegionScore("PROVINCE", input.Province, candidate.Province, 15, -20)
	candidateCity := candidate.City
	if candidateCity == "" && isMunicipality(candidate.Province) {
		candidateCity = candidate.Province
	}
	applyExactRegionScore("CITY", input.City, candidateCity, 25, -40)
	applyExactRegionScore("DISTRICT", input.District, candidate.District, 15, -15)

	searchable := normalizeGeocodeText(strings.Join([]string{candidate.FormattedAddress, candidate.Street, candidate.StreetNumber}, " "))
	if containsNormalized(searchable, input.Street) {
		score += 10
		reasons = append(reasons, "STREET_MATCH")
	}
	if containsNormalized(searchable, input.StreetNumber) {
		score += 10
		reasons = append(reasons, "STREET_NUMBER_MATCH")
	}
	names := append([]string{input.Name}, input.Aliases...)
	for _, name := range names {
		if containsNormalized(normalizeGeocodeText(candidate.FormattedAddress), name) {
			score += 20
			reasons = append(reasons, "MALL_NAME_MATCH")
			break
		}
	}
	if containsNormalized(normalizeGeocodeText(candidate.FormattedAddress), input.Address) {
		score += 10
		reasons = append(reasons, "FULL_ADDRESS_MATCH")
	}

	switch normalizeGeocodeText(candidate.Level) {
	case "省", "市", "区县", "区", "县":
		score -= 20
		reasons = append(reasons, "LOW_PRECISION")
	}
	if candidateCount > 1 {
		score -= 5
		reasons = append(reasons, "MULTIPLE_CANDIDATES")
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return CandidateScore{Score: score, Reasons: reasons}
}

func containsNormalized(haystack, needle string) bool {
	needle = normalizeGeocodeText(needle)
	return needle != "" && strings.Contains(haystack, needle)
}

func normalizeGeocodeText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), ""))
}

func isMunicipality(value string) bool {
	value = normalizeGeocodeText(value)
	return value == "北京市" || value == "上海市" || value == "天津市" || value == "重庆市"
}
