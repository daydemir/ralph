# Progress JSON Schema Design

## Overview

This document defines the `progress.json` format that replaces `progress.txt` in the Ralph CLI PRD-based architecture. The new format provides structured, queryable data that supports the analyzer's decision-making while maintaining the append-only spirit of the original progress tracking.

## Design Principles

1. **Append-Only Semantics**: New entries are added, not replaced (mirrors progress.txt behavior)
2. **Strictly Typed**: All fields have defined types and validation
3. **Versioned**: Schema version allows backward-compatible evolution
4. **PRD-Linked**: Every entry traces back to the PRD that produced it
5. **Analyzer-Friendly**: Structure supports pattern extraction and decision queries

---

## JSON Schema

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://ralph.dev/schemas/progress.json",
  "title": "Ralph Progress",
  "description": "Structured progress tracking for Ralph CLI execution history",
  "type": "object",
  "required": ["version", "created_at", "entries"],
  "additionalProperties": false,
  "properties": {
    "version": {
      "type": "string",
      "description": "Schema version for forward compatibility",
      "pattern": "^\\d+\\.\\d+$",
      "examples": ["1.0"]
    },
    "created_at": {
      "type": "string",
      "format": "date-time",
      "description": "When this progress file was created"
    },
    "project_name": {
      "type": "string",
      "description": "Name of the project being tracked"
    },
    "entries": {
      "type": "array",
      "description": "Append-only list of progress entries",
      "items": {
        "$ref": "#/$defs/ProgressEntry"
      }
    },
    "learnings": {
      "type": "array",
      "description": "Persistent learnings extracted from entries",
      "items": {
        "$ref": "#/$defs/Learning"
      }
    },
    "patterns": {
      "type": "array",
      "description": "Codebase patterns discovered during execution",
      "items": {
        "$ref": "#/$defs/Pattern"
      }
    }
  },
  "$defs": {
    "ProgressEntry": {
      "type": "object",
      "description": "A single iteration's progress record",
      "required": ["id", "timestamp", "prd_id", "iteration", "status", "observations"],
      "additionalProperties": false,
      "properties": {
        "id": {
          "type": "string",
          "description": "Unique entry ID (format: prd_id-iteration)",
          "pattern": "^[a-z0-9-]+-\\d+$"
        },
        "timestamp": {
          "type": "string",
          "format": "date-time"
        },
        "prd_id": {
          "type": "string",
          "description": "The PRD feature ID this entry relates to"
        },
        "iteration": {
          "type": "integer",
          "minimum": 1,
          "description": "Which iteration of this PRD (1, 2, 3...)"
        },
        "status": {
          "type": "string",
          "enum": ["completed", "failed", "blocked", "partial"],
          "description": "Outcome of this iteration"
        },
        "duration_seconds": {
          "type": "integer",
          "minimum": 0,
          "description": "How long the iteration took"
        },
        "summary": {
          "type": "string",
          "description": "One-liner describing what happened"
        },
        "observations": {
          "type": "array",
          "items": {
            "$ref": "#/$defs/Observation"
          }
        },
        "files_modified": {
          "type": "array",
          "items": {"type": "string"},
          "description": "Files changed during this iteration"
        },
        "git_commits": {
          "type": "array",
          "items": {"type": "string"},
          "description": "Git commit SHAs from this iteration"
        },
        "context": {
          "$ref": "#/$defs/IterationContext"
        }
      }
    },
    "Observation": {
      "type": "object",
      "description": "A finding captured during execution (simplified 3-type system)",
      "required": ["type", "title"],
      "additionalProperties": false,
      "properties": {
        "type": {
          "type": "string",
          "enum": ["blocker", "finding", "completion"],
          "description": "blocker=can't continue, finding=noticed something, completion=already done"
        },
        "title": {
          "type": "string",
          "description": "Short descriptive title"
        },
        "description": {
          "type": "string",
          "description": "What was noticed and agent's thoughts"
        },
        "file": {
          "type": "string",
          "description": "Relevant file path (optional)"
        },
        "category": {
          "type": "string",
          "enum": [
            "bug", "stub", "dependency", "scope-creep", "api-issue",
            "test-failure", "tooling-friction", "architecture",
            "documentation", "performance", "security"
          ],
          "description": "Inferred category for pattern analysis"
        },
        "severity": {
          "type": "string",
          "enum": ["critical", "high", "medium", "low", "info"],
          "description": "Severity inferred by analyzer"
        },
        "action_taken": {
          "type": "string",
          "enum": ["fixed", "deferred", "escalated", "documented", "none"],
          "description": "What action was taken on this observation"
        },
        "related_learning_id": {
          "type": "string",
          "description": "If this observation produced a learning, its ID"
        }
      }
    },
    "IterationContext": {
      "type": "object",
      "description": "Context for analyzer decision-making",
      "additionalProperties": false,
      "properties": {
        "retry_count": {
          "type": "integer",
          "minimum": 0,
          "description": "How many times this PRD has been retried"
        },
        "previous_failure_reason": {
          "type": "string",
          "description": "Why the previous iteration failed (if applicable)"
        },
        "recovery_action": {
          "type": "string",
          "enum": ["retry", "fix-state", "break-chunks", "skip", "manual"],
          "description": "Recovery action taken to get here"
        },
        "recovery_guidance": {
          "type": "string",
          "description": "Specific guidance that led to this iteration"
        },
        "dependencies_completed": {
          "type": "array",
          "items": {"type": "string"},
          "description": "PRD IDs that were completed before this"
        },
        "blocker_verified": {
          "type": "boolean",
          "description": "Was the blocker claim verified by analyzer?"
        },
        "blocker_valid": {
          "type": "boolean",
          "description": "Was the blocker determined to be legitimate?"
        }
      }
    },
    "Learning": {
      "type": "object",
      "description": "A persistent learning that applies across iterations",
      "required": ["id", "type", "content", "source_prd_id", "created_at"],
      "additionalProperties": false,
      "properties": {
        "id": {
          "type": "string",
          "pattern": "^learning-\\d{4}$",
          "description": "Unique learning ID"
        },
        "type": {
          "type": "string",
          "enum": [
            "codebase-pattern", "build-command", "test-pattern",
            "api-convention", "error-workaround", "tool-usage",
            "architecture-constraint", "dependency-quirk"
          ]
        },
        "content": {
          "type": "string",
          "description": "The learning itself"
        },
        "context": {
          "type": "string",
          "description": "When/where this applies"
        },
        "source_prd_id": {
          "type": "string",
          "description": "Which PRD produced this learning"
        },
        "source_entry_id": {
          "type": "string",
          "description": "Which progress entry produced this"
        },
        "created_at": {
          "type": "string",
          "format": "date-time"
        },
        "times_referenced": {
          "type": "integer",
          "minimum": 0,
          "description": "How often this learning has been used"
        },
        "still_valid": {
          "type": "boolean",
          "default": true,
          "description": "Whether this learning is still applicable"
        }
      }
    },
    "Pattern": {
      "type": "object",
      "description": "A codebase pattern discovered during execution",
      "required": ["id", "name", "type", "discovered_at"],
      "additionalProperties": false,
      "properties": {
        "id": {
          "type": "string",
          "pattern": "^pattern-\\d{4}$"
        },
        "name": {
          "type": "string",
          "description": "Short name for the pattern"
        },
        "type": {
          "type": "string",
          "enum": [
            "file-structure", "naming-convention", "api-pattern",
            "test-pattern", "error-handling", "state-management",
            "build-pattern", "deployment-pattern"
          ]
        },
        "description": {
          "type": "string",
          "description": "What the pattern is and how to follow it"
        },
        "examples": {
          "type": "array",
          "items": {"type": "string"},
          "description": "File paths or code snippets exemplifying the pattern"
        },
        "discovered_at": {
          "type": "string",
          "format": "date-time"
        },
        "source_prd_id": {
          "type": "string"
        },
        "confidence": {
          "type": "string",
          "enum": ["high", "medium", "low"],
          "description": "How confident we are this is a real pattern"
        }
      }
    }
  }
}
```

---

## Example Content

```json
{
  "version": "1.0",
  "created_at": "2025-01-15T10:00:00Z",
  "project_name": "my-awesome-app",
  "entries": [
    {
      "id": "auth-login-1",
      "timestamp": "2025-01-15T10:30:00Z",
      "prd_id": "auth-login",
      "iteration": 1,
      "status": "completed",
      "duration_seconds": 1847,
      "summary": "Implemented JWT-based login with refresh token rotation",
      "observations": [
        {
          "type": "finding",
          "title": "Existing auth middleware pattern",
          "description": "Found existing auth middleware in middleware/auth.go that uses different token format. Adapted new implementation to match.",
          "file": "internal/middleware/auth.go",
          "category": "architecture",
          "severity": "medium",
          "action_taken": "documented",
          "related_learning_id": "learning-0001"
        },
        {
          "type": "completion",
          "title": "Token validation already implemented",
          "description": "Token validation helper was already present in utils/jwt.go. Reused instead of reimplementing.",
          "file": "internal/utils/jwt.go",
          "category": "architecture",
          "severity": "info",
          "action_taken": "none"
        }
      ],
      "files_modified": [
        "internal/handlers/auth.go",
        "internal/services/auth_service.go",
        "internal/models/user.go"
      ],
      "git_commits": ["a1b2c3d"],
      "context": {
        "retry_count": 0,
        "dependencies_completed": ["user-model"]
      }
    },
    {
      "id": "auth-oauth-1",
      "timestamp": "2025-01-15T11:15:00Z",
      "prd_id": "auth-oauth",
      "iteration": 1,
      "status": "blocked",
      "duration_seconds": 623,
      "summary": "OAuth integration blocked - needs Google API credentials",
      "observations": [
        {
          "type": "blocker",
          "title": "Missing OAuth credentials",
          "description": "Cannot test OAuth flow without GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables. Implementation is complete but untestable.",
          "category": "dependency",
          "severity": "critical",
          "action_taken": "escalated"
        }
      ],
      "files_modified": [
        "internal/handlers/oauth.go",
        "internal/services/oauth_service.go"
      ],
      "git_commits": ["d4e5f6g"],
      "context": {
        "retry_count": 0,
        "dependencies_completed": ["auth-login"],
        "blocker_verified": true,
        "blocker_valid": true
      }
    },
    {
      "id": "auth-oauth-2",
      "timestamp": "2025-01-15T14:00:00Z",
      "prd_id": "auth-oauth",
      "iteration": 2,
      "status": "completed",
      "duration_seconds": 412,
      "summary": "Completed OAuth integration after credentials were provided",
      "observations": [
        {
          "type": "finding",
          "title": "OAuth callback URL must be exact match",
          "description": "Google OAuth requires exact callback URL match including trailing slash. Added note to deployment docs.",
          "category": "api-issue",
          "severity": "medium",
          "action_taken": "documented",
          "related_learning_id": "learning-0002"
        }
      ],
      "files_modified": [
        "internal/handlers/oauth.go",
        "docs/deployment.md"
      ],
      "git_commits": ["h7i8j9k"],
      "context": {
        "retry_count": 1,
        "previous_failure_reason": "Missing OAuth credentials",
        "recovery_action": "manual",
        "recovery_guidance": "User provided GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET",
        "dependencies_completed": ["auth-login"]
      }
    },
    {
      "id": "user-profile-1",
      "timestamp": "2025-01-15T15:30:00Z",
      "prd_id": "user-profile",
      "iteration": 1,
      "status": "failed",
      "duration_seconds": 1205,
      "summary": "Profile image upload failing - S3 bucket policy issue",
      "observations": [
        {
          "type": "finding",
          "title": "S3 CORS not configured",
          "description": "Direct browser uploads to S3 failing with CORS error. Need to configure CORS on the bucket.",
          "file": "internal/services/upload_service.go",
          "category": "api-issue",
          "severity": "high",
          "action_taken": "deferred"
        },
        {
          "type": "finding",
          "title": "Image resizing pattern discovered",
          "description": "Found existing image processing in utils/images.go using sharp. Should use same pattern for profile images.",
          "file": "internal/utils/images.go",
          "category": "architecture",
          "severity": "medium",
          "action_taken": "documented",
          "related_learning_id": "learning-0003"
        }
      ],
      "files_modified": [
        "internal/handlers/profile.go",
        "internal/services/upload_service.go"
      ],
      "git_commits": [],
      "context": {
        "retry_count": 0,
        "dependencies_completed": ["auth-login", "user-model"]
      }
    },
    {
      "id": "user-profile-2",
      "timestamp": "2025-01-15T16:45:00Z",
      "prd_id": "user-profile",
      "iteration": 2,
      "status": "completed",
      "duration_seconds": 892,
      "summary": "Profile feature complete - switched to presigned URLs to bypass CORS",
      "observations": [
        {
          "type": "finding",
          "title": "Presigned URLs simpler than CORS config",
          "description": "Using S3 presigned URLs avoids CORS entirely. Backend generates URL, client uploads directly. This is the preferred pattern.",
          "category": "api-issue",
          "severity": "info",
          "action_taken": "fixed",
          "related_learning_id": "learning-0004"
        }
      ],
      "files_modified": [
        "internal/handlers/profile.go",
        "internal/services/upload_service.go"
      ],
      "git_commits": ["l0m1n2o"],
      "context": {
        "retry_count": 1,
        "previous_failure_reason": "S3 CORS not configured",
        "recovery_action": "fix-state",
        "recovery_guidance": "Use presigned URLs instead of direct upload to avoid CORS",
        "dependencies_completed": ["auth-login", "user-model"]
      }
    }
  ],
  "learnings": [
    {
      "id": "learning-0001",
      "type": "architecture-constraint",
      "content": "All auth middleware must use the Bearer token format with RS256 signing",
      "context": "When implementing any authentication-related feature",
      "source_prd_id": "auth-login",
      "source_entry_id": "auth-login-1",
      "created_at": "2025-01-15T10:30:00Z",
      "times_referenced": 2,
      "still_valid": true
    },
    {
      "id": "learning-0002",
      "type": "api-convention",
      "content": "OAuth callback URLs must include trailing slash for Google provider",
      "context": "When configuring OAuth providers",
      "source_prd_id": "auth-oauth",
      "source_entry_id": "auth-oauth-2",
      "created_at": "2025-01-15T14:00:00Z",
      "times_referenced": 0,
      "still_valid": true
    },
    {
      "id": "learning-0003",
      "type": "codebase-pattern",
      "content": "Use utils/images.go with sharp for all image processing",
      "context": "When handling image uploads or transformations",
      "source_prd_id": "user-profile",
      "source_entry_id": "user-profile-1",
      "created_at": "2025-01-15T15:30:00Z",
      "times_referenced": 1,
      "still_valid": true
    },
    {
      "id": "learning-0004",
      "type": "error-workaround",
      "content": "Use S3 presigned URLs for file uploads instead of direct client uploads to avoid CORS configuration",
      "context": "When implementing file upload features",
      "source_prd_id": "user-profile",
      "source_entry_id": "user-profile-2",
      "created_at": "2025-01-15T16:45:00Z",
      "times_referenced": 0,
      "still_valid": true
    }
  ],
  "patterns": [
    {
      "id": "pattern-0001",
      "name": "Service-Handler Architecture",
      "type": "file-structure",
      "description": "Business logic in services/, HTTP handling in handlers/. Services are injected into handlers.",
      "examples": [
        "internal/services/auth_service.go",
        "internal/handlers/auth.go"
      ],
      "discovered_at": "2025-01-15T10:30:00Z",
      "source_prd_id": "auth-login",
      "confidence": "high"
    },
    {
      "id": "pattern-0002",
      "name": "Error Response Format",
      "type": "error-handling",
      "description": "All API errors use {\"error\": \"message\", \"code\": \"ERROR_CODE\"} format with appropriate HTTP status",
      "examples": [
        "internal/handlers/errors.go"
      ],
      "discovered_at": "2025-01-15T11:15:00Z",
      "source_prd_id": "auth-oauth",
      "confidence": "high"
    }
  ]
}
```

---

## Go Struct Definitions

```go
package types

import (
    "fmt"
    "time"
)

// Progress represents the full progress.json file
type Progress struct {
    Version     string          `json:"version"`
    CreatedAt   time.Time       `json:"created_at"`
    ProjectName string          `json:"project_name,omitempty"`
    Entries     []ProgressEntry `json:"entries"`
    Learnings   []Learning      `json:"learnings,omitempty"`
    Patterns    []Pattern       `json:"patterns,omitempty"`
}

// Validate ensures the progress file is valid
func (p *Progress) Validate() error {
    if p.Version == "" {
        return fmt.Errorf("progress.version: field is required")
    }
    if p.CreatedAt.IsZero() {
        return fmt.Errorf("progress.created_at: field is required")
    }
    for i, entry := range p.Entries {
        if err := entry.Validate(); err != nil {
            return fmt.Errorf("progress.entries[%d]: %w", i, err)
        }
    }
    for i, learning := range p.Learnings {
        if err := learning.Validate(); err != nil {
            return fmt.Errorf("progress.learnings[%d]: %w", i, err)
        }
    }
    for i, pattern := range p.Patterns {
        if err := pattern.Validate(); err != nil {
            return fmt.Errorf("progress.patterns[%d]: %w", i, err)
        }
    }
    return nil
}

// ProgressEntry represents a single iteration's progress record
type ProgressEntry struct {
    ID              string            `json:"id"`
    Timestamp       time.Time         `json:"timestamp"`
    PRDID           string            `json:"prd_id"`
    Iteration       int               `json:"iteration"`
    Status          ProgressStatus    `json:"status"`
    DurationSeconds int               `json:"duration_seconds,omitempty"`
    Summary         string            `json:"summary,omitempty"`
    Observations    []ProgressObs     `json:"observations"`
    FilesModified   []string          `json:"files_modified,omitempty"`
    GitCommits      []string          `json:"git_commits,omitempty"`
    Context         *IterationContext `json:"context,omitempty"`
}

// Validate ensures the progress entry is valid
func (e *ProgressEntry) Validate() error {
    if e.ID == "" {
        return fmt.Errorf("entry.id: field is required")
    }
    if e.Timestamp.IsZero() {
        return fmt.Errorf("entry.timestamp: field is required")
    }
    if e.PRDID == "" {
        return fmt.Errorf("entry.prd_id: field is required")
    }
    if e.Iteration < 1 {
        return fmt.Errorf("entry.iteration: must be >= 1")
    }
    if !e.Status.IsValid() {
        return fmt.Errorf("entry.status: invalid value %q", e.Status)
    }
    for i, obs := range e.Observations {
        if err := obs.Validate(); err != nil {
            return fmt.Errorf("entry.observations[%d]: %w", i, err)
        }
    }
    return nil
}

// ProgressStatus represents the outcome of an iteration
type ProgressStatus string

const (
    ProgressCompleted ProgressStatus = "completed"
    ProgressFailed    ProgressStatus = "failed"
    ProgressBlocked   ProgressStatus = "blocked"
    ProgressPartial   ProgressStatus = "partial"
)

// IsValid checks if a progress status is valid
func (s ProgressStatus) IsValid() bool {
    switch s {
    case ProgressCompleted, ProgressFailed, ProgressBlocked, ProgressPartial:
        return true
    }
    return false
}

// AllProgressStatuses returns all valid progress status values
func AllProgressStatuses() []ProgressStatus {
    return []ProgressStatus{ProgressCompleted, ProgressFailed, ProgressBlocked, ProgressPartial}
}

// ProgressObs represents a finding captured during execution
// Uses the simplified 3-type system from Ralph's observation types
type ProgressObs struct {
    Type              ObservationType `json:"type"`
    Title             string          `json:"title"`
    Description       string          `json:"description,omitempty"`
    File              string          `json:"file,omitempty"`
    Category          ObsCategory     `json:"category,omitempty"`
    Severity          ObsSeverity     `json:"severity,omitempty"`
    ActionTaken       ObsAction       `json:"action_taken,omitempty"`
    RelatedLearningID string          `json:"related_learning_id,omitempty"`
}

// Validate ensures the observation is valid
func (o *ProgressObs) Validate() error {
    if !o.Type.IsValid() {
        return fmt.Errorf("observation.type: invalid value %q", o.Type)
    }
    if o.Title == "" {
        return fmt.Errorf("observation.title: field is required")
    }
    if o.Category != "" && !o.Category.IsValid() {
        return fmt.Errorf("observation.category: invalid value %q", o.Category)
    }
    if o.Severity != "" && !o.Severity.IsValid() {
        return fmt.Errorf("observation.severity: invalid value %q", o.Severity)
    }
    if o.ActionTaken != "" && !o.ActionTaken.IsValid() {
        return fmt.Errorf("observation.action_taken: invalid value %q", o.ActionTaken)
    }
    return nil
}

// ObsCategory represents the category of an observation
type ObsCategory string

const (
    ObsCatBug             ObsCategory = "bug"
    ObsCatStub            ObsCategory = "stub"
    ObsCatDependency      ObsCategory = "dependency"
    ObsCatScopeCreep      ObsCategory = "scope-creep"
    ObsCatAPIIssue        ObsCategory = "api-issue"
    ObsCatTestFailure     ObsCategory = "test-failure"
    ObsCatToolingFriction ObsCategory = "tooling-friction"
    ObsCatArchitecture    ObsCategory = "architecture"
    ObsCatDocumentation   ObsCategory = "documentation"
    ObsCatPerformance     ObsCategory = "performance"
    ObsCatSecurity        ObsCategory = "security"
)

// IsValid checks if an observation category is valid
func (c ObsCategory) IsValid() bool {
    switch c {
    case ObsCatBug, ObsCatStub, ObsCatDependency, ObsCatScopeCreep,
        ObsCatAPIIssue, ObsCatTestFailure, ObsCatToolingFriction,
        ObsCatArchitecture, ObsCatDocumentation, ObsCatPerformance, ObsCatSecurity:
        return true
    case "": // Empty is valid (optional field)
        return true
    }
    return false
}

// ObsSeverity represents the severity of an observation
type ObsSeverity string

const (
    ObsSevCritical ObsSeverity = "critical"
    ObsSevHigh     ObsSeverity = "high"
    ObsSevMedium   ObsSeverity = "medium"
    ObsSevLow      ObsSeverity = "low"
    ObsSevInfo     ObsSeverity = "info"
)

// IsValid checks if an observation severity is valid
func (s ObsSeverity) IsValid() bool {
    switch s {
    case ObsSevCritical, ObsSevHigh, ObsSevMedium, ObsSevLow, ObsSevInfo:
        return true
    case "": // Empty is valid (optional field)
        return true
    }
    return false
}

// ObsAction represents the action taken on an observation
type ObsAction string

const (
    ObsActionFixed      ObsAction = "fixed"
    ObsActionDeferred   ObsAction = "deferred"
    ObsActionEscalated  ObsAction = "escalated"
    ObsActionDocumented ObsAction = "documented"
    ObsActionNone       ObsAction = "none"
)

// IsValid checks if an observation action is valid
func (a ObsAction) IsValid() bool {
    switch a {
    case ObsActionFixed, ObsActionDeferred, ObsActionEscalated, ObsActionDocumented, ObsActionNone:
        return true
    case "": // Empty is valid (optional field)
        return true
    }
    return false
}

// IterationContext holds context for analyzer decision-making
type IterationContext struct {
    RetryCount              int              `json:"retry_count,omitempty"`
    PreviousFailureReason   string           `json:"previous_failure_reason,omitempty"`
    RecoveryAction          RecoveryAction   `json:"recovery_action,omitempty"`
    RecoveryGuidance        string           `json:"recovery_guidance,omitempty"`
    DependenciesCompleted   []string         `json:"dependencies_completed,omitempty"`
    BlockerVerified         bool             `json:"blocker_verified,omitempty"`
    BlockerValid            bool             `json:"blocker_valid,omitempty"`
}

// RecoveryAction represents the recovery action type
type RecoveryAction string

const (
    RecoveryRetry       RecoveryAction = "retry"
    RecoveryFixState    RecoveryAction = "fix-state"
    RecoveryBreakChunks RecoveryAction = "break-chunks"
    RecoverySkip        RecoveryAction = "skip"
    RecoveryManual      RecoveryAction = "manual"
)

// Learning represents a persistent learning extracted from execution
type Learning struct {
    ID              string       `json:"id"`
    Type            LearningType `json:"type"`
    Content         string       `json:"content"`
    Context         string       `json:"context,omitempty"`
    SourcePRDID     string       `json:"source_prd_id"`
    SourceEntryID   string       `json:"source_entry_id,omitempty"`
    CreatedAt       time.Time    `json:"created_at"`
    TimesReferenced int          `json:"times_referenced,omitempty"`
    StillValid      bool         `json:"still_valid"`
}

// Validate ensures the learning is valid
func (l *Learning) Validate() error {
    if l.ID == "" {
        return fmt.Errorf("learning.id: field is required")
    }
    if !l.Type.IsValid() {
        return fmt.Errorf("learning.type: invalid value %q", l.Type)
    }
    if l.Content == "" {
        return fmt.Errorf("learning.content: field is required")
    }
    if l.SourcePRDID == "" {
        return fmt.Errorf("learning.source_prd_id: field is required")
    }
    if l.CreatedAt.IsZero() {
        return fmt.Errorf("learning.created_at: field is required")
    }
    return nil
}

// LearningType represents the type of learning
type LearningType string

const (
    LearningCodebasePattern      LearningType = "codebase-pattern"
    LearningBuildCommand         LearningType = "build-command"
    LearningTestPattern          LearningType = "test-pattern"
    LearningAPIConvention        LearningType = "api-convention"
    LearningErrorWorkaround      LearningType = "error-workaround"
    LearningToolUsage            LearningType = "tool-usage"
    LearningArchitectureConstraint LearningType = "architecture-constraint"
    LearningDependencyQuirk      LearningType = "dependency-quirk"
)

// IsValid checks if a learning type is valid
func (t LearningType) IsValid() bool {
    switch t {
    case LearningCodebasePattern, LearningBuildCommand, LearningTestPattern,
        LearningAPIConvention, LearningErrorWorkaround, LearningToolUsage,
        LearningArchitectureConstraint, LearningDependencyQuirk:
        return true
    }
    return false
}

// Pattern represents a codebase pattern discovered during execution
type Pattern struct {
    ID          string         `json:"id"`
    Name        string         `json:"name"`
    Type        PatternType    `json:"type"`
    Description string         `json:"description,omitempty"`
    Examples    []string       `json:"examples,omitempty"`
    DiscoveredAt time.Time     `json:"discovered_at"`
    SourcePRDID string         `json:"source_prd_id,omitempty"`
    Confidence  PatternConfidence `json:"confidence,omitempty"`
}

// Validate ensures the pattern is valid
func (p *Pattern) Validate() error {
    if p.ID == "" {
        return fmt.Errorf("pattern.id: field is required")
    }
    if p.Name == "" {
        return fmt.Errorf("pattern.name: field is required")
    }
    if !p.Type.IsValid() {
        return fmt.Errorf("pattern.type: invalid value %q", p.Type)
    }
    if p.DiscoveredAt.IsZero() {
        return fmt.Errorf("pattern.discovered_at: field is required")
    }
    return nil
}

// PatternType represents the type of codebase pattern
type PatternType string

const (
    PatternFileStructure     PatternType = "file-structure"
    PatternNamingConvention  PatternType = "naming-convention"
    PatternAPIPattern        PatternType = "api-pattern"
    PatternTestPattern       PatternType = "test-pattern"
    PatternErrorHandling     PatternType = "error-handling"
    PatternStateManagement   PatternType = "state-management"
    PatternBuildPattern      PatternType = "build-pattern"
    PatternDeploymentPattern PatternType = "deployment-pattern"
)

// IsValid checks if a pattern type is valid
func (t PatternType) IsValid() bool {
    switch t {
    case PatternFileStructure, PatternNamingConvention, PatternAPIPattern,
        PatternTestPattern, PatternErrorHandling, PatternStateManagement,
        PatternBuildPattern, PatternDeploymentPattern:
        return true
    }
    return false
}

// PatternConfidence represents confidence level in a pattern
type PatternConfidence string

const (
    ConfidenceHigh   PatternConfidence = "high"
    ConfidenceMedium PatternConfidence = "medium"
    ConfidenceLow    PatternConfidence = "low"
)
```

---

## State Package Functions

```go
package state

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/daydemir/ralph/internal/types"
)

// LoadProgressJSON loads progress from .ralph/progress.json
// Returns empty Progress if file doesn't exist (not an error - new project)
func LoadProgressJSON(ralphDir string) (*types.Progress, error) {
    progressPath := filepath.Join(ralphDir, "progress.json")

    file, err := os.Open(progressPath)
    if os.IsNotExist(err) {
        // New project - return empty progress
        return &types.Progress{
            Version:   "1.0",
            CreatedAt: time.Now(),
            Entries:   []types.ProgressEntry{},
            Learnings: []types.Learning{},
            Patterns:  []types.Pattern{},
        }, nil
    }
    if err != nil {
        return nil, fmt.Errorf("cannot open progress.json: %w", err)
    }
    defer file.Close()

    var progress types.Progress
    decoder := json.NewDecoder(file)
    decoder.DisallowUnknownFields()

    if err := decoder.Decode(&progress); err != nil {
        return nil, fmt.Errorf("cannot decode progress.json: %w", err)
    }

    if err := progress.Validate(); err != nil {
        return nil, fmt.Errorf("progress validation failed: %w", err)
    }

    return &progress, nil
}

// SaveProgressJSON saves progress to .ralph/progress.json atomically
func SaveProgressJSON(ralphDir string, progress *types.Progress) error {
    if err := progress.Validate(); err != nil {
        return fmt.Errorf("cannot save invalid progress: %w", err)
    }

    progressPath := filepath.Join(ralphDir, "progress.json")

    data, err := json.MarshalIndent(progress, "", "  ")
    if err != nil {
        return fmt.Errorf("cannot marshal progress: %w", err)
    }

    // Atomic write
    tempPath := progressPath + ".tmp"
    if err := os.WriteFile(tempPath, data, 0644); err != nil {
        return fmt.Errorf("cannot write temp progress file: %w", err)
    }

    if err := os.Rename(tempPath, progressPath); err != nil {
        os.Remove(tempPath)
        return fmt.Errorf("cannot rename temp progress file: %w", err)
    }

    return nil
}

// AppendProgressEntry adds a new entry to progress.json (append-only)
func AppendProgressEntry(ralphDir string, entry types.ProgressEntry) error {
    progress, err := LoadProgressJSON(ralphDir)
    if err != nil {
        return fmt.Errorf("cannot load progress: %w", err)
    }

    // Validate entry before appending
    if err := entry.Validate(); err != nil {
        return fmt.Errorf("invalid entry: %w", err)
    }

    progress.Entries = append(progress.Entries, entry)

    return SaveProgressJSON(ralphDir, progress)
}

// AppendLearning adds a new learning to progress.json
func AppendLearning(ralphDir string, learning types.Learning) error {
    progress, err := LoadProgressJSON(ralphDir)
    if err != nil {
        return fmt.Errorf("cannot load progress: %w", err)
    }

    if err := learning.Validate(); err != nil {
        return fmt.Errorf("invalid learning: %w", err)
    }

    progress.Learnings = append(progress.Learnings, learning)

    return SaveProgressJSON(ralphDir, progress)
}

// AppendPattern adds a new pattern to progress.json
func AppendPattern(ralphDir string, pattern types.Pattern) error {
    progress, err := LoadProgressJSON(ralphDir)
    if err != nil {
        return fmt.Errorf("cannot load progress: %w", err)
    }

    if err := pattern.Validate(); err != nil {
        return fmt.Errorf("invalid pattern: %w", err)
    }

    progress.Patterns = append(progress.Patterns, pattern)

    return SaveProgressJSON(ralphDir, progress)
}

// --- Query Functions for Analyzer ---

// GetRecentObservations returns observations from the last N entries
func GetRecentObservations(progress *types.Progress, count int) []types.ProgressObs {
    var observations []types.ProgressObs

    start := len(progress.Entries) - count
    if start < 0 {
        start = 0
    }

    for _, entry := range progress.Entries[start:] {
        observations = append(observations, entry.Observations...)
    }

    return observations
}

// GetObservationsByPRD returns all observations for a specific PRD
func GetObservationsByPRD(progress *types.Progress, prdID string) []types.ProgressObs {
    var observations []types.ProgressObs

    for _, entry := range progress.Entries {
        if entry.PRDID == prdID {
            observations = append(observations, entry.Observations...)
        }
    }

    return observations
}

// GetBlockers returns all blocker observations
func GetBlockers(progress *types.Progress) []types.ProgressObs {
    var blockers []types.ProgressObs

    for _, entry := range progress.Entries {
        for _, obs := range entry.Observations {
            if obs.Type == types.ObsTypeBlocker {
                blockers = append(blockers, obs)
            }
        }
    }

    return blockers
}

// GetLearningsByType returns learnings of a specific type
func GetLearningsByType(progress *types.Progress, learningType types.LearningType) []types.Learning {
    var learnings []types.Learning

    for _, learning := range progress.Learnings {
        if learning.Type == learningType && learning.StillValid {
            learnings = append(learnings, learning)
        }
    }

    return learnings
}

// GetPatternsByType returns patterns of a specific type
func GetPatternsByType(progress *types.Progress, patternType types.PatternType) []types.Pattern {
    var patterns []types.Pattern

    for _, pattern := range progress.Patterns {
        if pattern.Type == patternType {
            patterns = append(patterns, pattern)
        }
    }

    return patterns
}

// GetRetryCount returns how many times a PRD has been attempted
func GetRetryCount(progress *types.Progress, prdID string) int {
    count := 0
    for _, entry := range progress.Entries {
        if entry.PRDID == prdID {
            count++
        }
    }
    return count
}

// GetLastEntryForPRD returns the most recent entry for a PRD
func GetLastEntryForPRD(progress *types.Progress, prdID string) *types.ProgressEntry {
    for i := len(progress.Entries) - 1; i >= 0; i-- {
        if progress.Entries[i].PRDID == prdID {
            return &progress.Entries[i]
        }
    }
    return nil
}

// GetFailurePatterns analyzes entries to find recurring failure categories
func GetFailurePatterns(progress *types.Progress) map[types.ObsCategory]int {
    patterns := make(map[types.ObsCategory]int)

    for _, entry := range progress.Entries {
        if entry.Status == types.ProgressFailed || entry.Status == types.ProgressBlocked {
            for _, obs := range entry.Observations {
                if obs.Category != "" {
                    patterns[obs.Category]++
                }
            }
        }
    }

    return patterns
}

// NextLearningID generates the next learning ID
func NextLearningID(progress *types.Progress) string {
    return fmt.Sprintf("learning-%04d", len(progress.Learnings)+1)
}

// NextPatternID generates the next pattern ID
func NextPatternID(progress *types.Progress) string {
    return fmt.Sprintf("pattern-%04d", len(progress.Patterns)+1)
}
```

---

## How the Analyzer Uses progress.json

### 1. Pre-Execution Analysis

Before executing a PRD, the analyzer queries progress.json to inform its approach:

```go
func (a *Analyzer) PrepareExecution(prdID string) *ExecutionContext {
    progress, _ := state.LoadProgressJSON(a.ralphDir)

    ctx := &ExecutionContext{
        PRDID:     prdID,
        Iteration: state.GetRetryCount(progress, prdID) + 1,
    }

    // Check for previous failures on this PRD
    lastEntry := state.GetLastEntryForPRD(progress, prdID)
    if lastEntry != nil && lastEntry.Status != types.ProgressCompleted {
        ctx.PreviousFailureReason = lastEntry.Summary
        ctx.RetryCount = lastEntry.Context.RetryCount + 1
    }

    // Get relevant learnings
    ctx.Learnings = state.GetLearningsByType(progress, types.LearningCodebasePattern)
    ctx.Learnings = append(ctx.Learnings,
        state.GetLearningsByType(progress, types.LearningErrorWorkaround)...)

    // Get relevant patterns
    ctx.Patterns = progress.Patterns

    // Check for recurring failures
    failurePatterns := state.GetFailurePatterns(progress)
    if failurePatterns[types.ObsCatToolingFriction] > 3 {
        ctx.Warnings = append(ctx.Warnings,
            "High tooling friction detected - consider fixing infrastructure first")
    }

    return ctx
}
```

### 2. Post-Execution Recording

After execution completes, record the entry:

```go
func (a *Analyzer) RecordExecution(result *ExecutionResult) error {
    entry := types.ProgressEntry{
        ID:              fmt.Sprintf("%s-%d", result.PRDID, result.Iteration),
        Timestamp:       time.Now(),
        PRDID:           result.PRDID,
        Iteration:       result.Iteration,
        Status:          result.Status,
        DurationSeconds: int(result.Duration.Seconds()),
        Summary:         result.Summary,
        Observations:    result.Observations,
        FilesModified:   result.FilesModified,
        GitCommits:      result.GitCommits,
        Context: &types.IterationContext{
            RetryCount:            result.RetryCount,
            PreviousFailureReason: result.PreviousFailure,
            RecoveryAction:        result.RecoveryAction,
            RecoveryGuidance:      result.RecoveryGuidance,
            DependenciesCompleted: result.DepsCompleted,
        },
    }

    return state.AppendProgressEntry(a.ralphDir, entry)
}
```

### 3. Learning Extraction

The analyzer extracts learnings from significant observations:

```go
func (a *Analyzer) ExtractLearnings(entry *types.ProgressEntry) []types.Learning {
    var learnings []types.Learning
    progress, _ := state.LoadProgressJSON(a.ralphDir)

    for _, obs := range entry.Observations {
        // Only extract learnings from certain observation types
        if obs.ActionTaken == types.ObsActionDocumented ||
           obs.Category == types.ObsCatArchitecture {

            learning := types.Learning{
                ID:            state.NextLearningID(progress),
                Type:          inferLearningType(obs),
                Content:       obs.Description,
                Context:       fmt.Sprintf("Discovered while working on %s", entry.PRDID),
                SourcePRDID:   entry.PRDID,
                SourceEntryID: entry.ID,
                CreatedAt:     time.Now(),
                StillValid:    true,
            }
            learnings = append(learnings, learning)
        }
    }

    return learnings
}
```

### 4. Decision Making

The analyzer uses progress data to make recovery decisions:

```go
func (a *Analyzer) DecideRecovery(failure *FailureSignal, prdID string) RecoveryAction {
    progress, _ := state.LoadProgressJSON(a.ralphDir)

    // Check retry count
    retryCount := state.GetRetryCount(progress, prdID)
    if retryCount >= 3 {
        return RecoveryAction{
            Action:   "manual",
            Guidance: fmt.Sprintf("PRD %s has failed %d times - needs human review", prdID, retryCount),
        }
    }

    // Check for similar blockers that were resolved
    blockers := state.GetBlockers(progress)
    for _, blocker := range blockers {
        if similar(blocker.Title, failure.Detail) {
            // Find if this blocker was later resolved
            if workaround := findWorkaround(progress, blocker); workaround != "" {
                return RecoveryAction{
                    Action:   "retry",
                    Guidance: workaround,
                }
            }
        }
    }

    // Check for learnings that might help
    learnings := state.GetLearningsByType(progress, types.LearningErrorWorkaround)
    for _, learning := range learnings {
        if relevant(learning, failure) {
            return RecoveryAction{
                Action:   "retry",
                Guidance: learning.Content,
            }
        }
    }

    // Default: escalate to manual
    return RecoveryAction{
        Action:   "manual",
        Guidance: "No automated recovery found",
    }
}
```

---

## Migration from progress.txt

When upgrading from progress.txt to progress.json:

1. Parse existing progress.txt entries (best-effort)
2. Create initial progress.json with empty entries
3. Archive progress.txt to progress.txt.backup
4. New entries go to progress.json

```go
func MigrateProgressTxt(ralphDir string) error {
    txtPath := filepath.Join(ralphDir, "progress.txt")
    jsonPath := filepath.Join(ralphDir, "progress.json")

    // Skip if already migrated
    if _, err := os.Stat(jsonPath); err == nil {
        return nil
    }

    // Create new progress.json
    progress := &types.Progress{
        Version:   "1.0",
        CreatedAt: time.Now(),
        Entries:   []types.ProgressEntry{},
        Learnings: []types.Learning{},
        Patterns:  []types.Pattern{},
    }

    // Best-effort parse of progress.txt for historical context
    if content, err := os.ReadFile(txtPath); err == nil {
        // Add as a "legacy" learning
        progress.Learnings = append(progress.Learnings, types.Learning{
            ID:          "learning-0000",
            Type:        types.LearningCodebasePattern,
            Content:     string(content),
            Context:     "Migrated from progress.txt",
            SourcePRDID: "migration",
            CreatedAt:   time.Now(),
            StillValid:  true,
        })
    }

    // Save new format
    if err := state.SaveProgressJSON(ralphDir, progress); err != nil {
        return err
    }

    // Archive old file
    backupPath := txtPath + ".backup"
    return os.Rename(txtPath, backupPath)
}
```

---

## Versioning Strategy

The schema uses semantic versioning:

- **1.0**: Initial release
- **1.1**: Add new optional fields (backward compatible)
- **2.0**: Breaking changes (require migration)

Version handling in load:

```go
func LoadProgressJSON(ralphDir string) (*types.Progress, error) {
    // ... load file ...

    switch progress.Version {
    case "1.0":
        return &progress, nil
    case "1.1":
        // Handle 1.1-specific fields
        return &progress, nil
    default:
        // Unknown version - try best-effort
        if progress.Version > "2.0" {
            return nil, fmt.Errorf("progress.json version %s is too new", progress.Version)
        }
        return &progress, nil
    }
}
```

---

## Summary

This `progress.json` design:

1. **Replaces progress.txt** with a structured, queryable format
2. **Captures observations** using Ralph's simplified 3-type system (blocker, finding, completion)
3. **Tracks learnings** that persist across iterations with source tracing
4. **Records codebase patterns** discovered during execution
5. **Maintains per-PRD execution history** with iteration tracking
6. **Provides analyzer context** through query functions and iteration context
7. **Supports append-only semantics** while being fully structured
8. **Is versioned** for future evolution without breaking existing data
9. **Follows Ralph's existing patterns** for validation, atomic writes, and strict typing
