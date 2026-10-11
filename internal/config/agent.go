package config

const (
	AgentCredentialsDir  = ".openhere"
	AgentCredentialsFile = "credentials"
)

func AgentCredentialsPath() string {
	return "~/" + AgentCredentialsDir + "/" + AgentCredentialsFile
}
