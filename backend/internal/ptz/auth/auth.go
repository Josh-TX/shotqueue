// Package auth implements HTTP Basic/Digest auth for cameras whose "User auth." is switched on,
// ported from poc-controller's auth.js. The camera decides the scheme via its WWW-Authenticate
// challenge (Digest or Basic), not the client.
package auth

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

type Challenge struct {
	Scheme string
	Params map[string]string
}

// StatusError is what Send must return for a 401/403, carrying what RequestWithAuth needs to
// decide whether and how to retry.
type StatusError struct {
	StatusCode int
	Header     http.Header
}

func (e *StatusError) Error() string { return fmt.Sprintf("HTTP %d", e.StatusCode) }

func hashFor(algorithm string) func([]byte) []byte {
	switch strings.ToUpper(algorithm) {
	case "", "MD5", "MD5-SESS":
		return func(b []byte) []byte { h := md5.Sum(b); return h[:] }
	case "SHA-256", "SHA-256-SESS":
		return func(b []byte) []byte { h := sha256.Sum256(b); return h[:] }
	case "SHA-512-256", "SHA-512-256-SESS":
		return func(b []byte) []byte { h := sha512.Sum512_256(b); return h[:] }
	default:
		return nil
	}
}

// ParseAuthChallenges reads a WWW-Authenticate value that may carry several challenges separated
// by the same comma that separates one challenge's own parameters, e.g.
// `Digest realm="x", nonce="y", Basic realm="x"`. A token followed by "=" is a parameter of the
// challenge in hand; a token that isn't begins a new one.
func ParseAuthChallenges(header string) []Challenge {
	var challenges []Challenge
	i := 0
	n := len(header)

	isSpaceOrComma := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',' }
	isSpace := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
	skipSpace := func() {
		for i < n && isSpaceOrComma(header[i]) {
			i++
		}
	}
	skipBlanks := func() {
		for i < n && isSpace(header[i]) {
			i++
		}
	}
	readToken := func() string {
		start := i
		for i < n && header[i] != ' ' && header[i] != '\t' && header[i] != '\n' && header[i] != '\r' && header[i] != ',' && header[i] != '=' {
			i++
		}
		return header[start:i]
	}
	readValue := func() string {
		if i >= n || header[i] != '"' {
			return readToken()
		}
		i++ // opening quote
		var out strings.Builder
		for i < n && header[i] != '"' {
			if header[i] == '\\' && i+1 < n {
				i++
			}
			out.WriteByte(header[i])
			i++
		}
		i++ // closing quote
		return out.String()
	}

	for i < n {
		skipSpace()
		token := readToken()
		if token == "" {
			break
		}

		skipSpace()
		if i < n && header[i] == '=' {
			i++
			skipBlanks() // RFC 7230 allows `nonce = "abc"`
			value := readValue()
			if len(challenges) > 0 {
				challenges[len(challenges)-1].Params[strings.ToLower(token)] = value
			}
		} else {
			challenges = append(challenges, Challenge{Scheme: strings.ToLower(token), Params: map[string]string{}})
		}
	}

	return challenges
}

func qopList(qop string) []string {
	if qop == "" {
		return nil
	}
	parts := strings.Split(qop, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func offersQop(qop string) bool {
	if qop == "" {
		return true
	}
	for _, v := range qopList(qop) {
		if v == "auth" {
			return true
		}
	}
	return false
}

// ChooseChallenge prefers Digest over Basic wherever both are offered, and skips a challenge this
// build cannot answer (unknown hash, or a qop of auth-int only).
func ChooseChallenge(challenges []Challenge) *Challenge {
	var usable []Challenge
	for _, c := range challenges {
		if c.Scheme == "basic" || (c.Scheme == "digest" && hashFor(c.Params["algorithm"]) != nil && offersQop(c.Params["qop"])) {
			usable = append(usable, c)
		}
	}
	for _, c := range usable {
		if c.Scheme == "digest" {
			return &c
		}
	}
	for _, c := range usable {
		if c.Scheme == "basic" {
			return &c
		}
	}
	return nil
}

func quote(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(value) + `"`
}

func BuildBasicAuthorization(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

type digestOpts struct {
	username, password, method, uri, cnonce string
	nc                                      int
}

// buildDigestAuthorization implements RFC 7616. uri is the request-target as it goes on the wire
// (path and query, still percent-encoded) — this module puts the camera command in the query, so
// hashing a decoded path yields a response the camera rejects.
func buildDigestAuthorization(challenge Challenge, opts digestOpts) string {
	realm := challenge.Params["realm"]
	nonce := challenge.Params["nonce"]
	qop := challenge.Params["qop"]
	opaque, hasOpaque := challenge.Params["opaque"]
	algorithm := challenge.Params["algorithm"]

	hf := hashFor(algorithm)
	if hf == nil {
		return ""
	}
	H := func(s string) string { return hex.EncodeToString(hf([]byte(s))) }
	ncHex := fmt.Sprintf("%08x", opts.nc)

	sess := strings.HasSuffix(strings.ToLower(algorithm), "-sess")
	secret := H(opts.username + ":" + realm + ":" + opts.password)
	ha1 := secret
	if sess {
		ha1 = H(secret + ":" + nonce + ":" + opts.cnonce)
	}

	useQop := ""
	for _, v := range qopList(qop) {
		if v == "auth" {
			useQop = "auth"
		}
	}

	ha2 := H(opts.method + ":" + opts.uri)
	var response string
	if useQop != "" {
		response = H(ha1 + ":" + nonce + ":" + ncHex + ":" + opts.cnonce + ":" + useQop + ":" + ha2)
	} else {
		response = H(ha1 + ":" + nonce + ":" + ha2) // RFC 2069 form, for a camera offering no qop
	}

	parts := []string{
		"username=" + quote(opts.username),
		"realm=" + quote(realm),
		"nonce=" + quote(nonce),
		"uri=" + quote(opts.uri),
		"response=" + quote(response),
	}
	if algorithm != "" {
		parts = append(parts, "algorithm="+algorithm)
	}
	if useQop != "" {
		parts = append(parts, "qop="+useQop, "nc="+ncHex, "cnonce="+quote(opts.cnonce))
	} else if sess {
		parts = append(parts, "cnonce="+quote(opts.cnonce))
	}
	if hasOpaque {
		parts = append(parts, "opaque="+quote(opaque))
	}

	return "Digest " + strings.Join(parts, ", ")
}

// Session holds one connection's credentials and, once the camera has asked, the challenge to
// answer every later request with, so the handshake happens once rather than per request.
type Session struct {
	mu             sync.Mutex
	Username       string
	Password       string
	hasCredentials bool
	scheme         string // "unknown" | "none" | "basic" | "digest"
	ok             bool
	challenge      *Challenge
	cnonce         string
	nc             int
}

func NewSession(username, password string) *Session {
	return &Session{
		Username:       username,
		Password:       password,
		hasCredentials: username != "" || password != "",
		scheme:         "unknown",
	}
}

func randomCnonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// adoptChallenge replaces nonce, cnonce and counter together: nc counts requests against one
// nonce, and reusing a count the camera has already seen is a replay to it. Caller must hold mu.
func (s *Session) adoptChallenge(challenge Challenge) {
	s.scheme = challenge.Scheme
	s.challenge = &challenge
	s.cnonce = randomCnonce()
	s.nc = 0
}

// authHeaders builds the header for one request, taking the nc counter under lock so concurrent
// requests never sign themselves with the same nc.
func (s *Session) authHeaders(method, uri string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.hasCredentials || s.challenge == nil {
		return map[string]string{}
	}
	if s.scheme == "basic" {
		return map[string]string{"Authorization": BuildBasicAuthorization(s.Username, s.Password)}
	}

	s.nc++
	authorization := buildDigestAuthorization(*s.challenge, digestOpts{
		username: s.Username,
		password: s.Password,
		method:   method,
		uri:      uri,
		nc:       s.nc,
		cnonce:   s.cnonce,
	})
	if authorization == "" {
		return map[string]string{}
	}
	return map[string]string{"Authorization": authorization}
}

// Event reports what happened on the connection, for logging.
type Event struct {
	Type      string
	Scheme    string
	Realm     string
	Algorithm string
	Offered   string
}

func isUnauthorized(err error) bool {
	se, ok := err.(*StatusError)
	return ok && se.StatusCode == 401
}

func isForbidden(err error) bool {
	se, ok := err.(*StatusError)
	return ok && se.StatusCode == 403
}

// SendFunc performs one request with the given headers. It must return a *StatusError for a
// 401/403 so RequestWithAuth can decide whether and how to retry.
type SendFunc func(headers map[string]string) (*http.Response, error)

// RequestWithAuth sends one request, and answers a 401 by adopting the challenge and sending it
// again — once. The retry-once bound is the whole safety property: a camera that keeps saying no
// is answered by at most one extra request, never a stream of them.
func RequestWithAuth(send SendFunc, session *Session, method, uri string, report func(Event)) (*http.Response, error) {
	if report == nil {
		report = func(Event) {}
	}

	// The challenge this request signs with, taken before anything goes out. Concurrent requests
	// share one session, so asking afterwards whether the session has a challenge answers a
	// different question: another request in flight may have adopted one meanwhile.
	session.mu.Lock()
	attempted := session.challenge
	session.mu.Unlock()

	resp, err := send(session.authHeaders(method, uri))
	if err == nil {
		session.mu.Lock()
		if session.scheme == "unknown" {
			session.scheme = "none"
			report(Event{Type: "none"})
		}
		session.ok = true
		session.mu.Unlock()
		return resp, nil
	}

	if isForbidden(err) {
		session.mu.Lock()
		scheme := session.scheme
		session.mu.Unlock()
		report(Event{Type: "forbidden", Scheme: scheme})
		return nil, err
	}

	if !isUnauthorized(err) {
		return nil, err
	}

	se := err.(*StatusError)
	var offered []Challenge
	if h := se.Header.Get("WWW-Authenticate"); h != "" {
		offered = ParseAuthChallenges(h)
	}
	challenge := ChooseChallenge(offered)

	if !session.hasCredentials {
		var scheme string
		if challenge != nil {
			scheme = challenge.Scheme
		}
		report(Event{Type: "credentialsRequired", Scheme: scheme})
		return nil, err
	}

	if len(offered) > 0 && challenge == nil {
		schemes := make([]string, len(offered))
		for i, c := range offered {
			schemes[i] = c.Scheme
		}
		report(Event{Type: "unsupported", Offered: strings.Join(schemes, ", ")})
		return nil, err
	}

	// This request offered credentials and was refused anyway: either the nonce aged out, or the
	// password is wrong. Only the first is worth another request.
	stale := challenge != nil && strings.EqualFold(challenge.Params["stale"], "true")

	if attempted != nil && !stale {
		session.mu.Lock()
		scheme := session.scheme
		session.mu.Unlock()
		report(Event{Type: "rejected", Scheme: scheme})
		return nil, err
	}

	// Adopt only where this request is the one with something to replace: the challenge it
	// attempted is still the session's, or the session carries none at all. A challenge another
	// request adopted while this one was in flight is used as it stands — adopting again would
	// reset the nonce counter, and a count the camera has already seen is a replay to it.
	session.mu.Lock()
	mine := false
	if stale {
		mine = session.challenge == attempted
	} else {
		mine = session.challenge == nil
	}
	if mine {
		if challenge != nil {
			session.adoptChallenge(*challenge)
		} else {
			// Some firmware answers 401 with no challenge at all. Basic needs none, so it's worth
			// the single retry we allow ourselves.
			session.adoptChallenge(Challenge{Scheme: "basic", Params: map[string]string{}})
		}
	}
	session.mu.Unlock()

	if mine && stale {
		var realm string
		if challenge != nil {
			realm = challenge.Params["realm"]
		}
		report(Event{Type: "stale", Realm: realm})
	}

	headers := session.authHeaders(method, uri)
	if headers["Authorization"] == "" {
		var scheme string
		if challenge != nil {
			scheme = challenge.Scheme
		}
		report(Event{Type: "unsupported", Offered: scheme})
		return nil, err
	}

	retryResp, retryErr := send(headers)
	if retryErr == nil {
		session.mu.Lock()
		session.ok = true
		scheme := session.scheme
		session.mu.Unlock()

		// Reported only once the retry has come back: announcing the handshake before then names a
		// login as accepted while the camera is still free to refuse it.
		if !stale {
			var realm, algorithm, offeredScheme string
			if challenge != nil {
				realm = challenge.Params["realm"]
				algorithm = challenge.Params["algorithm"]
				offeredScheme = challenge.Scheme
			}
			report(Event{Type: "authenticated", Scheme: scheme, Realm: realm, Algorithm: algorithm, Offered: offeredScheme})
		}
		return retryResp, nil
	}

	// The one extra request is spent. A second refusal is reported here rather than left for the
	// next request to discover.
	session.mu.Lock()
	scheme := session.scheme
	session.mu.Unlock()
	if isUnauthorized(retryErr) {
		var realm string
		if challenge != nil {
			realm = challenge.Params["realm"]
		}
		report(Event{Type: "rejected", Realm: realm, Scheme: scheme})
	} else if isForbidden(retryErr) {
		var realm string
		if challenge != nil {
			realm = challenge.Params["realm"]
		}
		report(Event{Type: "forbidden", Realm: realm, Scheme: scheme})
	}
	return nil, retryErr
}
