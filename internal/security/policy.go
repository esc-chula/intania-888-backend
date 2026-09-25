package security

import "strings"

var allowedIndividualEmails = map[string]struct{}{
	"phanthawasjira@gmail.com": {},
	"bububiib@gmail.com":       {},
	"pear.nataya49@gmail.com":  {},
}

var blacklistedEmails = map[string]struct{}{
	"6530162621@student.chula.ac.th": {},
	"6633129621@student.chula.ac.th": {},
	"6733023821@student.chula.ac.th": {},
	"6630054621@student.chula.ac.th": {},
	"6538004621@student.chula.ac.th": {},
	"6733291621@student.chula.ac.th": {},
	"6430039021@student.chula.ac.th": {},
}

var blacklistedIDs = map[string]struct{}{
	"115982048644097094953": {},
	"101935624102444830754": {},
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func IsAllowedEmail(email string) bool {
	email = NormalizeEmail(email)
	if strings.HasSuffix(email, "@student.chula.ac.th") {
		return true
	}
	_, ok := allowedIndividualEmails[email]
	return ok
}

func IsBlacklisted(email, userID string) bool {
	if _, ok := blacklistedEmails[NormalizeEmail(email)]; ok {
		return true
	}
	_, ok := blacklistedIDs[strings.TrimSpace(userID)]
	return ok
}
