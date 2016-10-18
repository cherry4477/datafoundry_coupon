package statistics

import (
	"testing"
)

func _testParseStatKey(t *testing.T,
	statKey string,
	date, user string, itemKeys []string, statName string) {
	d, u, ks, n := ParseStatKey(statKey)

	if d != date {
		t.Errorf("d(%s) != date(%s)", d, date)
	}

	if u != user {
		t.Errorf("u(%s) != user(%s)", u, user)
	}

	if n != statName {
		t.Errorf("n(%s) != statName(%s)", n, statName)
	}

	if len(ks) != len(itemKeys) {
		t.Errorf("len(ks)(%#v) != len(itemKeys)(%#v)", ks, itemKeys)
	}

	for i := len(ks) - 1; i >= 0; i-- {
		if ks[i] != itemKeys[i] {
			t.Errorf("ks[%d](%s) != itemKeys[%d](%s)", i, ks[i], i, itemKeys[i])
		}
	}
}

func TestParseStatKey(t *testing.T) {
	_testParseStatKey(t,
		"repo/item/tag#subs",
		"", "", []string{"repo", "item", "tag"}, "subs",
	)
	_testParseStatKey(t,
		"repo/item#subs",
		"", "", []string{"repo", "item"}, "subs",
	)
	_testParseStatKey(t,
		"zhang$repo/item/tag#subs",
		"", "zhang", []string{"repo", "item", "tag"}, "subs",
	)
	_testParseStatKey(t,
		"zhang$repo/item#subs",
		"", "zhang", []string{"repo", "item"}, "subs",
	)
	_testParseStatKey(t,
		"zhang$#subs",
		"", "zhang", []string{}, "subs",
	)
}
