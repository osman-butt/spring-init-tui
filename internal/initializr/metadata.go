package initializr

import (
	"context"
	"encoding/json"
	"fmt"
)

const metadataMediaType = "application/vnd.initializr.v2.3+json"

// Metadata holds the options a Spring Initializr instance offers.
type Metadata struct {
	JavaVersions       []string
	DefaultJavaVersion string
	Dependencies       []Dependency
}

// Dependency is a single selectable dependency. Group is the category it is
// listed under, e.g. "Web".
type Dependency struct {
	ID          string
	Name        string
	Group       string
	Description string
}

type metadataResponse struct {
	JavaVersion struct {
		Default string `json:"default"`
		Values  []struct {
			ID string `json:"id"`
		} `json:"values"`
	} `json:"javaVersion"`
	Dependencies struct {
		Values []struct {
			Name   string `json:"name"`
			Values []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"values"`
		} `json:"values"`
	} `json:"dependencies"`
}

// Metadata fetches the available Java versions and dependencies.
func (c *Client) Metadata(ctx context.Context) (Metadata, error) {
	resp, err := c.get(ctx, c.BaseURL, metadataMediaType)
	if err != nil {
		return Metadata{}, fmt.Errorf("fetch metadata: %w", err)
	}
	defer resp.Body.Close()

	var raw metadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Metadata{}, fmt.Errorf("decode metadata: %w", err)
	}

	md := Metadata{DefaultJavaVersion: raw.JavaVersion.Default}
	for _, v := range raw.JavaVersion.Values {
		md.JavaVersions = append(md.JavaVersions, v.ID)
	}
	for _, group := range raw.Dependencies.Values {
		for _, d := range group.Values {
			md.Dependencies = append(md.Dependencies, Dependency{
				ID:          d.ID,
				Name:        d.Name,
				Group:       group.Name,
				Description: d.Description,
			})
		}
	}
	return md, nil
}
