package config

const (
	AgentCredentialsDir  = ".selfhost"
	AgentCredentialsFile = "credentials"
)

func AgentCredentialsPath() string {
	return "~/" + AgentCredentialsDir + "/" + AgentCredentialsFile
}
