package model

import (
	"fmt"
	"regexp"

	"github.com/google/uuid"
)

// Server-owned and checked post props.
const (
	// PropProvenance records what produced an agent's message: the tools, the
	// OKF namespaces and blob, the model and the attachments it used. HALO's
	// membership hook reads it to decide who may join a channel. Only an agent
	// or bot, or an allowlisted client posting on behalf of a person, may set
	// it, and ValidateProvenance checks its shape.
	PropProvenance = "provenance"
	// PropOnBehalfOf is set by the server on a post that an allowlisted
	// OAuth2 client made on behalf of a person: {"actor_id": <the client's
	// user ID>}. The post's user_id is the person. A client cannot set it.
	PropOnBehalfOf = "on_behalf_of"
)

// Provenance limits. They bound what one post can carry, not what HALO may
// use: a message that used more than this many tools is not a message.
const (
	ProvenanceMaxIDs      = 100
	ProvenanceMaxModelLen = 200
)

// semscope: allow(NASA-P10-06) compiled once, never changed
var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// provenanceIDFields returns the provenance fields that hold UUID lists.
func provenanceIDFields() []string {
	return []string{"tool_ids", "okf_namespace_ids", "attachment_ids"}
}

// ValidateProvenance checks a props.provenance value, as decoded from JSON.
// It is an object with only these optional fields:
//
//	tool_ids, okf_namespace_ids, attachment_ids  lists of canonical UUIDs
//	okf_blob_sha256                              64 lowercase hex characters
//	model                                        1 to 200 characters
//
// Any other field, or a value of another type, is a 400.
func ValidateProvenance(v any) *AppError {
	obj, ok := v.(map[string]any)
	if !ok {
		return badProvenance("must be an object")
	}
	for key, val := range obj {
		if err := validateProvenanceField(key, val); err != nil {
			return err
		}
	}
	return nil
}

// badProvenance is ValidateProvenance's 400 for an invalid props.provenance.
func badProvenance(format string, args ...any) *AppError {
	return NewBadRequestError("Post.Provenance", "props.provenance: "+fmt.Sprintf(format, args...))
}

// validateProvenanceField checks one field of a props.provenance object.
func validateProvenanceField(key string, val any) *AppError {
	switch key {
	case "tool_ids", "okf_namespace_ids", "attachment_ids":
		return validateProvenanceIDs(key, val)
	case "okf_blob_sha256":
		s, ok := val.(string)
		if !ok || !sha256HexRe.MatchString(s) {
			return badProvenance("okf_blob_sha256 must be 64 lowercase hex characters")
		}
	case "model":
		s, ok := val.(string)
		if !ok || s == "" || len(s) > ProvenanceMaxModelLen {
			return badProvenance("model must be a string of 1 to %d characters", ProvenanceMaxModelLen)
		}
	default:
		return badProvenance("unknown field %q (allowed: %v, okf_blob_sha256, model)", key, provenanceIDFields())
	}
	return nil
}

// validateProvenanceIDs checks a provenance field that holds a list of UUIDs.
func validateProvenanceIDs(key string, val any) *AppError {
	list, ok := val.([]any)
	if !ok {
		return badProvenance("%s must be a list of UUIDs", key)
	}
	if len(list) > ProvenanceMaxIDs {
		return badProvenance("%s has %d entries, more than %d", key, len(list), ProvenanceMaxIDs)
	}
	for i, item := range list {
		s, ok := item.(string)
		if !ok || !isCanonicalUUID(s) {
			return badProvenance("%s[%d] is not a UUID", key, i)
		}
	}
	return nil
}

// isCanonicalUUID accepts only the 36-character hyphenated form, which is
// what HALO writes and what a reader can compare as a string; uuid.Parse
// would also take the urn:, braced and unhyphenated forms.
func isCanonicalUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	_, err := uuid.Parse(s)
	return err == nil
}
