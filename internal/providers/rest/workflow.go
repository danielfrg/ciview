package rest

import (
	"net/url"
	"path"
	"strings"
	"unicode"
)

// WorkflowURL builds a provider's workflow history URL from repository web
// metadata and a verified workflow file. Unknown identities remain unlinked.
func WorkflowURL(repositoryURL, provider, workflowFile string) string {
	filename := workflowFile
	switch provider {
	case "github":
		if !strings.HasPrefix(workflowFile, ".github/workflows/") {
			return ""
		}
		filename = strings.TrimPrefix(workflowFile, ".github/workflows/")
	case "forgejo":
	default:
		return ""
	}
	if filename == "" || strings.ContainsAny(filename, "/\\") || strings.IndexFunc(filename, unicode.IsControl) >= 0 || (!strings.HasSuffix(filename, ".yml") && !strings.HasSuffix(filename, ".yaml")) {
		return ""
	}

	u, err := url.Parse(repositoryURL)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	repositoryPath := strings.TrimSuffix(u.Path, "/")
	escapedPath := strings.ToLower(u.EscapedPath())
	if repositoryPath == "" || strings.Contains(u.Path, "\\") || strings.IndexFunc(u.Path, unicode.IsControl) >= 0 || path.Clean(repositoryPath) != repositoryPath || strings.Contains(escapedPath, "%2f") || strings.Contains(escapedPath, "%5c") {
		return ""
	}
	u.RawPath, u.RawQuery, u.Fragment, u.RawFragment = "", "", "", ""
	u.ForceQuery = false
	if provider == "github" {
		u.Path = repositoryPath + "/actions/workflows/" + filename
	} else {
		u.Path = repositoryPath + "/actions"
		u.RawQuery = url.Values{"workflow": {filename}}.Encode()
	}
	return u.String()
}
