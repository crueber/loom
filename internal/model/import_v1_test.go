package model

import "testing"

// A v1 export dual-imports links: the same bookmark appears in both
// "bookmarks" and "items" (and with cosmetic URL variants). The import
// must keep the first occurrence and preserve order.
func TestImportV1DedupesLinks(t *testing.T) {
	v1 := `{"version":1,"lists":[
		{"id":1,"title":"Tech","color":"blue","position":0,"collapsed":false,
		 "bookmarks":[
		   {"id":1,"title":"First","url":"https://Example.com/Docs/","position":1},
		   {"id":2,"title":"Dup slash+case+frag","url":"https://example.com/Docs#a","position":2},
		   {"id":3,"title":"Other","url":"https://example.com/Other","position":4}],
		 "notes":[],
		 "items":[
		   {"id":4,"type":"bookmark","title":"Dup across arrays","url":"  https://example.com/Docs  ","position":0},
		   {"id":5,"type":"note","content":"keep me","position":3}]},
		{"id":2,"title":"Second list","color":"red","position":1,"collapsed":false,
		 "bookmarks":[
		   {"id":6,"title":"Same link, other list","url":"https://example.com/Docs/","position":0}],
		 "notes":[],"items":[]}
	]}`
	tree, err := ImportV1([]byte(v1))
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(tree.Columns))
	}
	first := tree.Cards[tree.Columns[0].ID]
	if len(first) != 3 {
		t.Fatalf("expected 3 cards in first column, got %d", len(first))
	}
	// Order preserved by position: link (pos 1), note (pos 3), other (pos 4).
	if first[0].Blocks[0].Type != BlockLink || first[0].Blocks[0].Title != "First" {
		t.Fatalf("first occurrence must win: %+v", first[0].Blocks[0])
	}
	if first[0].Blocks[0].Position != 0 {
		t.Fatalf("positions renumbered from 0, got %+v", first[0].Blocks)
	}
	if first[1].Blocks[0].Type != BlockNote || first[1].Blocks[0].Content != "keep me" {
		t.Fatalf("note must survive dedupe: %+v", first[1].Blocks[0])
	}
	if first[2].Blocks[0].URL != "https://example.com/Other" {
		t.Fatalf("distinct URL must survive: %+v", first[2].Blocks[0])
	}
	// Dedupe is per list: the same link in another list is kept.
	second := tree.Cards[tree.Columns[1].ID]
	if len(second) != 1 || second[0].Blocks[0].URL != "https://example.com/Docs/" {
		t.Fatalf("same link in another list must be kept: %+v", second)
	}
}

func TestNormalizeLinkURL(t *testing.T) {
	cases := map[string]string{
		"https://Example.COM/Docs/":    "https://example.com/Docs",
		"https://example.com/Docs#a":   "https://example.com/Docs",
		"  https://example.com/Docs  ": "https://example.com/Docs",
		"https://example.com/":         "https://example.com",
		"https://example.com":          "https://example.com",
		"https://example.com/x?y=1":    "https://example.com/x?y=1",
		"not a url":                    "not a url",
		"":                             "",
	}
	for in, want := range cases {
		if got := normalizeLinkURL(in); got != want {
			t.Errorf("normalizeLinkURL(%q) = %q, want %q", in, got, want)
		}
	}
}
