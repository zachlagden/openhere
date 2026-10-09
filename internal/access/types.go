package access

import "time"

type Mode string

const (
	ModePublic     Mode = "public"
	ModePassword   Mode = "password"
	ModeRestricted Mode = "restricted"
)

func ParseMode(raw string) (Mode, bool) {
	switch mode := Mode(raw); mode {
	case ModePublic, ModePassword, ModeRestricted:
		return mode, true
	default:
		return ModePublic, false
	}
}

func (m Mode) Gated() bool {
	return m == ModePassword || m == ModeRestricted
}

type Method string

const (
	MethodPassword Method = "password"
	MethodEmail    Method = "email"
	MethodMember   Method = "member"
)

func (m Method) valid() bool {
	return m == MethodPassword || m == MethodEmail || m == MethodMember
}

func (m Method) fits(mode Mode) bool {
	switch mode {
	case ModePassword:
		return m == MethodPassword
	case ModeRestricted:
		return m == MethodEmail || m == MethodMember
	default:
		return false
	}
}

const (
	CookieLifetime  = 7 * 24 * time.Hour
	CodeLifetime    = 10 * time.Minute
	HandoffLifetime = 60 * time.Second

	CodeDigits   = 8
	CodeAttempts = 5

	MaxAllowlistEntries = 500
	MinPasswordLength   = 8
	MaxPasswordLength   = 128
	maxDomainLength     = 253
)

type State struct {
	Mode  Mode
	Epoch int64
}

type Config struct {
	Mode        Mode
	HasPassword bool
	Emails      []string
	Domains     []string
}

type PasswordChange struct {
	Set   bool
	Value *string
}

type ConfigUpdate struct {
	Mode     *string
	Password PasswordChange
	Emails   *[]string
	Domains  *[]string
}
