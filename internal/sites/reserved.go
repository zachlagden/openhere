package sites

import (
	"strings"

	"github.com/zachlagden/openhere/internal/slug"
	"github.com/zachlagden/openhere/internal/validate"
)

const punycodePrefix = "xn--"

var reservedSlugs = map[string]struct{}{
	"www": {}, "api": {}, "app": {}, "admin": {}, "administrator": {}, "dashboard": {}, "mail": {}, "smtp": {},
	"imap": {}, "pop": {}, "pop3": {}, "ftp": {}, "sftp": {}, "ssh": {}, "static": {}, "assets": {}, "cdn": {},
	"s": {}, "docs": {}, "doc": {}, "status": {}, "health": {}, "metrics": {}, "login": {}, "logout": {},
	"signin": {}, "signup": {}, "register": {}, "setup": {}, "settings": {}, "account": {}, "auth": {},
	"oauth": {}, "invite": {}, "invites": {}, "console": {}, "help": {}, "support": {}, "billing": {},
	"root": {}, "system": {}, "localhost": {}, "ns1": {}, "ns2": {}, "webmail": {}, "dev": {}, "staging": {},
	"test": {}, "email": {}, "download": {}, "downloads": {}, "upload": {}, "uploads": {}, "workspace": {},
	"workspaces": {}, "sites": {}, "site": {}, "me": {}, "domains": {}, "analytics": {}, "llms": {},
	"mta-sts": {}, "autoconfig": {}, "autodiscover": {}, "ns3": {}, "vpn": {}, "git": {}, "files": {}, "media": {}, "cname": {},
}

func IsReserved(slug string) bool {
	_, reserved := reservedSlugs[slug]
	return reserved
}

func CheckSlug(candidate string) error {
	if err := slug.Check(candidate); err != nil {
		return err
	}
	if IsReserved(candidate) {
		return validate.Invalid("That slug is reserved. Choose another.")
	}
	if strings.HasPrefix(candidate, punycodePrefix) {
		return validate.Invalid("A slug must not start with xn--. Choose another.")
	}
	return nil
}
