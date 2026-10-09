package sites

import (
	"github.com/zachlagden/selfhost/internal/slug"
	"github.com/zachlagden/selfhost/internal/validate"
)

var reservedSlugs = map[string]struct{}{
	"www": {}, "api": {}, "app": {}, "admin": {}, "administrator": {}, "dashboard": {}, "mail": {}, "smtp": {},
	"imap": {}, "pop": {}, "pop3": {}, "ftp": {}, "sftp": {}, "ssh": {}, "static": {}, "assets": {}, "cdn": {},
	"s": {}, "docs": {}, "doc": {}, "status": {}, "health": {}, "metrics": {}, "login": {}, "logout": {},
	"signin": {}, "signup": {}, "register": {}, "setup": {}, "settings": {}, "account": {}, "auth": {},
	"oauth": {}, "invite": {}, "invites": {}, "console": {}, "help": {}, "support": {}, "billing": {},
	"root": {}, "system": {}, "localhost": {}, "ns1": {}, "ns2": {}, "webmail": {}, "dev": {}, "staging": {},
	"test": {}, "email": {}, "download": {}, "downloads": {}, "upload": {}, "uploads": {}, "workspace": {},
	"workspaces": {}, "sites": {}, "site": {}, "me": {}, "domains": {}, "analytics": {}, "llms": {},
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
	return nil
}
