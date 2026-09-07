package auth

import "testing"

// RFC 2617 §3.5 worked example.
func TestBuildDigestAuthorization_RFC2617Example(t *testing.T) {
	challenge := Challenge{Scheme: "digest", Params: map[string]string{
		"realm": "testrealm@host.com",
		"nonce": "dcd98b7102dd2f0e8b11d0f600bfb0c093",
		"qop":   "auth",
	}}
	got := buildDigestAuthorization(challenge, digestOpts{
		username: "Mufasa",
		password: "Circle Of Life",
		method:   "GET",
		uri:      "/dir/index.html",
		nc:       1,
		cnonce:   "0a4f113b",
	})
	want := `response="6629fae49393a05397450978507c4ef1"`
	if !contains(got, want) {
		t.Fatalf("got %q, want it to contain %q", got, want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestParseAuthChallenges_MultipleSchemes(t *testing.T) {
	got := ParseAuthChallenges(`Digest realm="Control", nonce="abc", qop="auth", Basic realm="Control"`)
	if len(got) != 2 {
		t.Fatalf("got %d challenges, want 2: %+v", len(got), got)
	}
	if got[0].Scheme != "digest" || got[0].Params["realm"] != "Control" || got[0].Params["nonce"] != "abc" {
		t.Fatalf("unexpected first challenge: %+v", got[0])
	}
	if got[1].Scheme != "basic" || got[1].Params["realm"] != "Control" {
		t.Fatalf("unexpected second challenge: %+v", got[1])
	}
}

func TestChooseChallenge_PrefersDigest(t *testing.T) {
	challenges := []Challenge{
		{Scheme: "basic", Params: map[string]string{}},
		{Scheme: "digest", Params: map[string]string{}},
	}
	got := ChooseChallenge(challenges)
	if got == nil || got.Scheme != "digest" {
		t.Fatalf("got %+v, want digest", got)
	}
}
