package rest

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"ciview/internal/ci"
	"gopkg.in/yaml.v3"
)

// ListWorkflowCatalog enriches a verified file list with optional YAML titles.
// Definition failures never change file presence or discard workflow history.
func (t *Transport) ListWorkflowCatalog(ctx context.Context, repository ci.Repository, branch string) (ci.WorkflowCatalog, error) {
	files, directory, err := t.workflowFiles(ctx, repository, branch)
	catalog := ci.WorkflowCatalog{Files: files, Names: map[string]string{}}
	if err != nil {
		return catalog, err
	}
	if directory == "" || t.Provider != "forgejo" {
		return catalog, nil
	}
	// Optional metadata cannot consume the entire run-collection request budget.
	metadata, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	p, _ := RepoPath(repository) // Already validated by workflowFiles.
	for i, file := range files {
		if i >= 100 || metadata.Err() != nil {
			break
		}
		target, err := t.Endpoint(p+"/contents/"+directory+"/"+file, url.Values{"ref": {branch}})
		if err != nil {
			continue
		}
		var body struct{ Type, Path, Encoding, Content string }
		if _, err := t.Get(metadata, target, &body); err != nil {
			var pe *ci.ProviderError
			if errors.As(err, &pe) && (pe.Kind == "rate_limit" || pe.Kind == "authentication" || pe.Kind == "permission") {
				catalog.MetadataError = err
				break
			}
			continue
		}
		if body.Type != "file" || body.Path != directory+"/"+file || body.Encoding != "base64" || len(body.Content) > 90000 {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(body.Content)
		if err != nil || len(data) > 65536 {
			continue
		}
		if title := workflowTitle(data); title != "" {
			catalog.Names[file] = title
		}
	}
	return catalog, nil
}

func workflowTitle(data []byte) string {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if decoder.Decode(&doc) != nil || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return ""
	}
	var extra yaml.Node
	if decoder.Decode(&extra) != io.EOF {
		return ""
	}
	root := doc.Content[0]
	title := ""
	seen := map[string]bool{}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if key.Kind != yaml.ScalarNode || seen[key.Value] {
			return ""
		}
		seen[key.Value] = true
		if key.Value == "name" {
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return ""
			}
			title = strings.TrimSpace(value.Value)
		}
	}
	if len(title) > 256 || strings.ContainsAny(title, "\r\n\x00") {
		return ""
	}
	return title
}
