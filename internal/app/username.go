package app

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"

	"github.com/infrashift/chit/internal/assert"
)

// Usernames for Kratos identities that have no username trait.
//
// Chit's own identity schema requires traits.username, and that value is used
// as it is. Another schema, such as HALO's, may carry only an email. Chit then
// derives the username from the email's local part and the Kratos ID:
//
//	sam.smith@example.com, Kratos ID 6f1f2a64-... -> sam.smith-6f1f2a64
//
// The local part is lowercased; each run of characters outside [a-z0-9._-]
// becomes one "-"; leading and trailing ".", "_" and "-" are removed; and it is
// cut so the whole name has at most maxUsernameLen characters. With no email,
// or a local part with nothing left, the name is "user-" and 12 hex digits.
//
// The suffix is the start of the Kratos ID's hex digits, so the name is the
// same each time and differs between identities. When the name is already
// taken, provisioning tries the next candidate, whose suffix is longer, up to
// all 32 digits of the ID.

const maxUsernameLen = 64

// usernameSuffixLens returns the suffix lengths, in hex digits, of the
// candidates for an identity with an email; an identity without one starts at 12.
func usernameSuffixLens() []int { return []int{8, 12, 16, 32} }

// usernameCandidates returns the usernames to try, in order, for a new user
// of the Kratos identity kratosID. An explicit trait username is the only
// candidate.
func usernameCandidates(traitUsername, email, kratosID string) []string {
	if traitUsername != "" {
		return []string{strings.ToLower(traitUsername)}
	}
	digits := kratosHexDigits(kratosID)
	assert.That(len(digits) == 32, "usernameCandidates: kratos hex digits are not 32 long")
	local := sanitizeLocalPart(email)
	var out []string
	for _, n := range usernameSuffixLens() {
		suffix := digits[:n]
		if local == "" {
			if n < 12 {
				continue
			}
			out = append(out, "user-"+suffix)
			continue
		}
		prefix := local
		if room := maxUsernameLen - len(suffix) - 1; len(prefix) > room {
			prefix = strings.TrimRight(prefix[:room], "._-")
		}
		assert.That(len(prefix)+1+len(suffix) <= maxUsernameLen, "usernameCandidates: candidate is longer than maxUsernameLen")
		out = append(out, prefix+"-"+suffix)
	}
	assert.That(len(out) > 0, "usernameCandidates: no candidates")
	return out
}

// sanitizeLocalPart returns the part of email before its last "@", reduced to
// the characters a username may hold. It may be empty.
func sanitizeLocalPart(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return ""
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(email[:at]) {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		switch {
		case ok:
			b.WriteRune(r)
			dash = false
		case !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "._-")
	assert.That(len(out) <= len(email), "sanitizeLocalPart: result longer than the email")
	assert.That(!strings.Contains(out, "@"), "sanitizeLocalPart: result holds an @")
	return out
}

// kratosHexDigits returns 32 lowercase hex digits for kratosID: its own when
// it is a UUID (Kratos IDs are), otherwise a SHA-256 of it.
func kratosHexDigits(kratosID string) string {
	if id, err := uuid.Parse(kratosID); err == nil {
		out := strings.ReplaceAll(id.String(), "-", "")
		assert.That(len(out) == 32, "kratosHexDigits: uuid hex is not 32 digits")
		return out
	}
	sum := sha256.Sum256([]byte(kratosID))
	out := hex.EncodeToString(sum[:])[:32]
	assert.That(len(out) == 32, "kratosHexDigits: hash hex is not 32 digits")
	return out
}
