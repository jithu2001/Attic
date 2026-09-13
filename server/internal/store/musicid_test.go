package store

import (
	"strings"
	"testing"
)

func TestArtistIDRoundTrips(t *testing.T) {
	names := []string{
		"Miles Davis",
		"Sigur Rós",
		"AC/DC",
		"?",
		"坂本龍一",
		"A name with / slashes and ?query=marks & ampersands",
		strings.Repeat("long ", 50),
	}

	for _, name := range names {
		id := EncodeArtistID(name)
		// Ids travel in URL paths, so they must survive one unescaped.
		if strings.ContainsAny(id, "/?&#% ") {
			t.Errorf("EncodeArtistID(%q) = %q, which is not URL-path safe", name, id)
		}
		got, err := DecodeArtistID(id)
		if err != nil {
			t.Fatalf("DecodeArtistID(%q) error = %v", id, err)
		}
		if got != name {
			t.Errorf("round trip of %q gave %q", name, got)
		}
	}
}

func TestAlbumIDRoundTrips(t *testing.T) {
	tests := [][2]string{
		{"Miles Davis", "Kind of Blue"},
		{"Various Artists", "Now That's What I Call Music! 42"},
		{"A", ""},
		{"Sigur Rós", "( )"},
	}

	for _, tc := range tests {
		id := EncodeAlbumID(tc[0], tc[1])
		if strings.ContainsAny(id, "/?&#% ") {
			t.Errorf("EncodeAlbumID(%q, %q) = %q, which is not URL-path safe", tc[0], tc[1], id)
		}
		artist, album, err := DecodeAlbumID(id)
		if err != nil {
			t.Fatalf("DecodeAlbumID(%q) error = %v", id, err)
		}
		if artist != tc[0] || album != tc[1] {
			t.Errorf("round trip of (%q, %q) gave (%q, %q)", tc[0], tc[1], artist, album)
		}
	}
}

func TestIDsAreStable(t *testing.T) {
	// Clients cache these; a rescan that rewrites every track row must not
	// change them.
	if EncodeArtistID("Miles Davis") != EncodeArtistID("Miles Davis") {
		t.Error("EncodeArtistID is not deterministic")
	}
	if EncodeAlbumID("a", "b") == EncodeAlbumID("b", "a") {
		t.Error("artist and album are not distinguished in the id")
	}
}

func TestMalformedIDsAreRejected(t *testing.T) {
	for _, bad := range []string{"", "!!!", "%%%", "a b"} {
		if _, err := DecodeArtistID(bad); err == nil {
			t.Errorf("DecodeArtistID(%q) succeeded", bad)
		}
	}
	for _, bad := range []string{"", "!!!", EncodeArtistID("no separator here")} {
		if _, _, err := DecodeAlbumID(bad); err == nil {
			t.Errorf("DecodeAlbumID(%q) succeeded", bad)
		}
	}
}

func TestEscapeLikeNeutralisesWildcards(t *testing.T) {
	// A directory called "100%" must not match every path in the library.
	if got := escapeLike(`/music/100%/_b_/c\d/`); got != `/music/100\%/\_b\_/c\\d/` {
		t.Errorf("escapeLike = %q", got)
	}
}
