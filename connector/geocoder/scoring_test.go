package geocoder

import "testing"

func TestScoreCandidatesPenalizesCrossCity(t *testing.T) {
	input := ScoreInput{Name: "示例商场", Province: "上海市", City: "上海市", District: "黄浦区", Street: "示例路", StreetNumber: "1号"}
	scores := ScoreCandidates(input, []Candidate{
		{FormattedAddress: "上海市黄浦区示例路1号示例商场", Province: "上海市", District: "黄浦区", Street: "示例路", StreetNumber: "1号", Level: "兴趣点"},
		{FormattedAddress: "江苏省南京市示例商场", Province: "江苏省", City: "南京市", District: "玄武区", Level: "兴趣点"},
	})
	if scores[0].Score <= scores[1].Score || !hasScoreReason(scores[1], "CITY_MISMATCH") {
		t.Fatalf("scores = %+v", scores)
	}
}

func TestAutoConfirmCandidateRejectsAmbiguousMultipleCandidates(t *testing.T) {
	if _, ok := AutoConfirmCandidate([]CandidateScore{{Index: 0, Score: 92}, {Index: 1, Score: 84}}); ok {
		t.Fatal("ambiguous candidates were auto-confirmed")
	}
	index, ok := AutoConfirmCandidate([]CandidateScore{{Index: 0, Score: 70}, {Index: 1, Score: 95}})
	if !ok || index != 1 {
		t.Fatalf("AutoConfirmCandidate() = %d,%v", index, ok)
	}
}

func TestScoreCandidatesRejectsLowPrecisionAutoConfirmation(t *testing.T) {
	input := ScoreInput{Name: "示例商场", Province: "上海市", City: "上海市", District: "黄浦区"}
	scores := ScoreCandidates(input, []Candidate{{FormattedAddress: "上海市黄浦区示例商场", Province: "上海市", District: "黄浦区", Level: "区县"}})
	if !hasScoreReason(scores[0], "LOW_PRECISION") {
		t.Fatalf("score = %+v", scores[0])
	}
	if _, ok := AutoConfirmCandidate(scores); ok {
		t.Fatalf("low precision candidate was auto-confirmed: %+v", scores[0])
	}
}

func TestScoreCandidatesHandlesMunicipalityEmptyCity(t *testing.T) {
	input := ScoreInput{Name: "示例商场", Province: "上海市", City: "上海市", District: "黄浦区", Address: "上海市黄浦区示例商场"}
	scores := ScoreCandidates(input, []Candidate{{FormattedAddress: "上海市黄浦区示例商场", Province: "上海市", City: "", District: "黄浦区", Level: "兴趣点"}})
	if !hasScoreReason(scores[0], "CITY_MATCH") {
		t.Fatalf("score = %+v", scores[0])
	}
	if _, ok := AutoConfirmCandidate(scores); !ok {
		t.Fatalf("high confidence municipality candidate not auto-confirmed: %+v", scores[0])
	}
}

func hasScoreReason(score CandidateScore, want string) bool {
	for _, reason := range score.Reasons {
		if reason == want {
			return true
		}
	}
	return false
}
