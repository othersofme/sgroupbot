package main

import (
	"testing"
)

func TestIdiomsSolitaire(t *testing.T) {
	idioms, err := LoadIdioms(idiomsPath)
	if err != nil {
		t.Error("load idioms failed", err)
		return
	}

	// var total = 5
	is := NewIdiomsSolitaire(idioms, 10)

	t.Run("randome idiom", func(t *testing.T) {
		word, last := is.radomIdiom()
		if len(word) == 0 || last == 0 {
			t.Error("random failed")
		} else {
			t.Log(word, last)
		}
	})

}
