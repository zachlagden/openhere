package settings

const (
	KeyTLSDNSProvider     = "tls.dnsProvider"
	KeyCloudflareAPIToken = "tls.cloudflareApiToken"

	DNSProviderNone       = "none"
	DNSProviderCloudflare = "cloudflare"
)

type TLS struct {
	DNSProvider        string
	CloudflareAPIToken string
}

func tlsDefinitions() []definition {
	return []definition{
		enumDefinition(KeyTLSDNSProvider, DNSProviderNone, []string{DNSProviderNone, DNSProviderCloudflare},
			func(s *Settings, v string) { s.TLS.DNSProvider = v },
			func(s Settings) string { return s.TLS.DNSProvider }),
		secretDefinition(KeyCloudflareAPIToken,
			func(s *Settings, v string) { s.TLS.CloudflareAPIToken = v },
			func(s Settings) string { return s.TLS.CloudflareAPIToken }),
	}
}
