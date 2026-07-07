package palette

import "testing"

func TestFuzzyScore_EmptyQueryMatchesAll(t *testing.T) {
	if _, ok := fuzzyScore("", "anything"); !ok {
		t.Error("empty query should match")
	}
}

func TestFuzzyScore_SubsequenceMatch(t *testing.T) {
	if _, ok := fuzzyScore("gnl", "general"); !ok {
		t.Error("expected subsequence 'gnl' to match 'general'")
	}
	if _, ok := fuzzyScore("xyz", "general"); ok {
		t.Error("expected 'xyz' not to match 'general'")
	}
}

func TestFuzzyScore_CaseInsensitive(t *testing.T) {
	if _, ok := fuzzyScore("GEN", "general"); !ok {
		t.Error("expected case-insensitive match")
	}
	if _, ok := fuzzyScore("gen", "GENERAL"); !ok {
		t.Error("expected case-insensitive match against upper candidate")
	}
}

func TestFuzzyScore_ConsecutiveBeatsScattered(t *testing.T) {
	consecutive, ok1 := fuzzyScore("gen", "general")
	scattered, ok2 := fuzzyScore("gen", "garden-hen")
	if !ok1 || !ok2 {
		t.Fatal("both should match")
	}
	if consecutive <= scattered {
		t.Errorf("consecutive match (%d) should outscore scattered (%d)", consecutive, scattered)
	}
}

func TestFuzzyScore_PrefixBeatsMidword(t *testing.T) {
	prefix, ok1 := fuzzyScore("gen", "general")
	midword, ok2 := fuzzyScore("gen", "urgent-notices")
	if !ok1 || !ok2 {
		t.Fatal("both should match")
	}
	if prefix <= midword {
		t.Errorf("prefix match (%d) should outscore mid-word (%d)", prefix, midword)
	}
}

func TestFuzzyScore_QueryLongerThanCandidate(t *testing.T) {
	if _, ok := fuzzyScore("longquery", "shrt"); ok {
		t.Error("query longer than candidate should not match")
	}
}
