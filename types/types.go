package types

type Contact struct {
	Name string
}

type GoogleAppCredentials struct {
	Installed InstalledType
}

type InstalledType struct {
	ClientId                string   `json:"client_id"`
	ProjectId               string   `json:"project_id"`
	AuthURI                 string   `json:"auth_uri"`
	TokenURI                string   `json:"token_uri"`
	AuthProviderx509CertURL string   `json:"auth_provider_x509_cert_url"`
	ClientSecret            string   `json:"client_secret"`
	RedirectUris            []string `json:"redirect_uris"`
}
