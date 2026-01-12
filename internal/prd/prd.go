package prd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Feature represents a single PRD item
type Feature struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Steps       []string   `json:"steps"`
	Passes      bool       `json:"passes"`
	MayDependOn []string   `json:"may_depend_on,omitempty"`
	Notes       string     `json:"notes,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	GitCommits  []string   `json:"git_commit_sha,omitempty"`
}

// File represents the prd.json structure
type File struct {
	Features []Feature `json:"features"`
}

// Load reads and parses a prd.json file
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	return &file, nil
}

// Save writes the PRD file to disk
func (f *File) Save(path string) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PRD file: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

// Pending returns features where passes is false
func (f *File) Pending() []Feature {
	var pending []Feature
	for _, feat := range f.Features {
		if !feat.Passes {
			pending = append(pending, feat)
		}
	}
	return pending
}

// Completed returns features where passes is true
func (f *File) Completed() []Feature {
	var completed []Feature
	for _, feat := range f.Features {
		if feat.Passes {
			completed = append(completed, feat)
		}
	}
	return completed
}

// FindByID returns a feature by its ID
func (f *File) FindByID(id string) *Feature {
	for i := range f.Features {
		if f.Features[i].ID == id {
			return &f.Features[i]
		}
	}
	return nil
}

// PrintJSON outputs filtered PRDs as JSON
func PrintJSON(f *File, pendingOnly, completedOnly bool) error {
	var features []Feature

	if pendingOnly {
		features = f.Pending()
	} else if completedOnly {
		features = f.Completed()
	} else {
		features = f.Features
	}

	output := File{Features: features}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
