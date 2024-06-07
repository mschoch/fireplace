package main

type UploadRequestResponse struct {
	UploadURL string `json:"uploadURL"`
	Key       string `json:"Key"`
}

type MetaResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

type MetaRequest struct {
	CID     string   `json:"cid"`
	Data    string   `json:"data"`
	Parents []string `json:"parents,omitempty"`
}

type MetaItems struct {
	Items []*MetaRequest `json:"items"`
}

type UserProfile struct {
	LoginName     string `json:"loginName"`   // "alice@smith.com"; for display purposes only (provider is not listed)
	DisplayName   string `json:"displayName"` // "Alice Smith"
	ProfilePicURL string `json:"profilePicURL"`
	Node          string `json:"node"`
}

type Database struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Application struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`

	BindAddr  string `json:"bindAddr"`
	AutoStart bool   `json:"autoStart"`
	LocalPath string `json:"localPath"`

	// these are determined at runtime and not persisted
	URL string `json:"url"`
}
