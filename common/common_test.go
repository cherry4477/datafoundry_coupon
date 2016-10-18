package common

import (
	"testing"
)

func TestValidateUrlWord(t *testing.T) {
	_testValidateUrlWord(t, "  aflaf98aAD  ", "aflaf98aAD", true)
	_testValidateUrlWord(t, "  ad093_-doe  ", "ad093_-doe", true)
	_testValidateUrlWord(t, "aflaf98afd33_", "aflaf98afd33_", true)
	_testValidateUrlWord(t, "aaaa vbbbb", "aaaa vbbbb", false)
	_testValidateUrlWord(t, "@ddd32423", "@ddd32423", false)
	_testValidateUrlWord(t, "aaaaa/bbbbbbb", "aaaaa/bbbbbbb", false)
}

func _testValidateUrlWord(t *testing.T, word string, expectedWord string, expectedOk bool) {
	validated_word, ok := ValidateUrlWord(word)
	if ok != expectedOk || validated_word != expectedWord {
		t.Errorf("ValidateUrlWord (%s) => (%s, %t) != (%s, %t)\n", word, validated_word, ok, expectedWord, expectedOk)
	}
}

func TestValidateEmail(t *testing.T) {
	_testValidateEmail(t, "  a@b.com  ", "a@b.com", true)
	_testValidateEmail(t, "a-b_c.d@e.com", "a-b_c.d@e.com", true)
	_testValidateEmail(t, "  @a@b.com  ", "@a@b.com", false)
	_testValidateEmail(t, "a.b.d", "a.b.d", false)
	_testValidateEmail(t, "@addd", "@addd", false)
	_testValidateEmail(t, "dd@add@d", "dd@add@d", false)
	_testValidateEmail(t, "cdfa.-_afadf@", "cdfa.-_afadf@", false)
}

func _testValidateEmail(t *testing.T, word string, expectedWord string, expectedOk bool) {
	validated_word, ok := ValidateEmail(word)
	if ok != expectedOk || validated_word != expectedWord {
		t.Errorf("ValidateUrlWord (%s) => (%s, %t) != (%s, %t)\n", word, validated_word, ok, expectedWord, expectedOk)
	}
}
