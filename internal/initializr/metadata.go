package initializr

import (
	"context"
	"encoding/json"
	"fmt"
)

const metadataMediaType = "application/vnd.initializr.v2.3+json"

// Metadata holds the options a Spring Initializr instance offers.
type Metadata struct {
	DefaultGroupID     string
	BootVersions       []BootVersion
	DefaultBootVersion string
	JavaVersions       []string
	DefaultJavaVersion string
	Dependencies       []Dependency
}

// BootVersion is a Spring Boot version to build on. ID is what Initializr
// expects, e.g. "4.2.0-M2"; Name is how it is listed, e.g. "4.2.0 (M2)".
type BootVersion struct {
	ID   string
	Name string
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
	GroupID struct {
		Default string `json:"default"`
	} `json:"groupId"`
	BootVersion struct {
		Default string `json:"default"`
		Values  []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"values"`
	} `json:"bootVersion"`
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

// Metadata fetches the available Spring Boot versions, Java versions and
// dependencies.
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

	md := Metadata{
		DefaultGroupID:     raw.GroupID.Default,
		DefaultBootVersion: raw.BootVersion.Default,
		DefaultJavaVersion: raw.JavaVersion.Default,
	}
	for _, v := range raw.BootVersion.Values {
		md.BootVersions = append(md.BootVersions, BootVersion{ID: v.ID, Name: v.Name})
	}
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
