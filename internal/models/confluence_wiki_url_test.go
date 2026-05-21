package models

import "testing"

func TestWikiPageURL(t *testing.T) {
	got := WikiPageURL("https://tenant.atlassian.net/wiki", "/spaces/X/pages/1")
	want := "https://tenant.atlassian.net/wiki/spaces/X/pages/1"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if WikiPageURL("", "/x") != "" {
		t.Fatal("empty wiki base")
	}
}
