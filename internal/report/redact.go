package report

import (
	"regexp"
	"strings"

	"attestara/internal/decision"
)

const redacted = "[REDACTED]"

var quotedLocalPath = regexp.MustCompile(`(?i)(?:"(?:/[A-Za-z0-9_.~-]|~/|[a-z]:[/\\]|\\\\)[^"]*"|'(?:/[A-Za-z0-9_.~-]|~/|[a-z]:[/\\]|\\\\)[^']*')`)

var publicTextPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bfile://[^\s,;"'<>]+`),
	regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s/@]+@`),
	regexp.MustCompile(`(?i)\b(?:bearer|basic)\s+(?:"[^"]*"|'[^']*'|[^\s,;"']+)`),
	regexp.MustCompile(`(?i)\b(?:password|passwd|pwd|token|api[_-]?key|secret|authorization)\s*[:=]\s*(?:"[^"]*"|'[^']*'|[^\s,;]+)`),
	regexp.MustCompile(`(?i)(^|[\s(=:'"])[a-z]:[/\\][^\s,'")<>;]+`),
	regexp.MustCompile(`(^|[\s(=:'"])\\\\[^\s,'")<>;]+`),
	regexp.MustCompile(`(^|[\s(=:'"])(?:/[A-Za-z0-9_.~-]|~/)[^\s,'")<>;]*`),
}

func redactPublicText(value string) string {
	if strings.ContainsAny(value, "\r\n<>") {
		return redacted
	}
	value = quotedLocalPath.ReplaceAllString(value, redacted)
	for _, pattern := range publicTextPatterns {
		value = pattern.ReplaceAllStringFunc(value, func(match string) string {
			if len(match) > 0 && strings.ContainsRune(" (=: '\"\t\n", rune(match[0])) {
				return match[:1] + redacted
			}
			return redacted
		})
	}
	return value
}

func redactPassport(passport Passport) Passport {
	passport.ReleaseID = redactPublicText(passport.ReleaseID)
	passport.SourceRevision = redactPublicText(passport.SourceRevision)
	for index, warning := range passport.Warnings {
		switch warning {
		case decision.EvidenceMissing, decision.EvidenceStale, decision.PolicyFailed:
		default:
			passport.Warnings[index] = redacted
		}
	}
	return passport
}
