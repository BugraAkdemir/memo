# Skill System Design — Memo

> **Status:** Draft  
> **Version:** v1.0  
> **Target Release:** v3.2.0  

## 1. Overview

Add opencode-style skill support to Memo's Agent and Orchestra modes. Users can install, list, activate, and deactivate skills that inject instructions into the LLM system prompt and optionally register new tools in the agent pipeline.

## 2. Skill Format (opencode-compatible)

### Directory Structure

```
data/skills/<skill-name>/
├── SKILL.md                    # YAML front matter + Markdown instructions
└── <any-references-or-files>   # Optional supporting files
```

### SKILL.md Format

```markdown
---
name: skill-name
description: "Short description of what this skill does"
version: "1.0.0"
author: "Author Name"
danger_level: safe|medium|dangerous
metadata:
  tags: ["tag1", "tag2"]
  icon: "emoji-or-icon"
tools:                           # Optional: tool definitions for the agent registry
  - name: tool_name
    description: "Tool description"
    parameters:                  # JSON Schema
      type: object
      properties:
        arg1:
          type: string
          description: "Arg description"
    danger_level: safe|medium|dangerous
---

# Skill Name

Full markdown instructions for the LLM. These are injected into the system prompt
when the skill is active.

## Usage

How to use this skill...
```

### Single-File Alternative (YAML-only)

If `SKILL.md` contains only YAML front matter with an `instructions` field, it is treated as a single-file skill:

```markdown
---
name: example
description: "Example skill"
instructions: |
  Direct instructions here without a separate file.
---
```

## 3. Architecture

### New Package: `internal/skill/`

| File | Responsibility |
|------|---------------|
| `types.go` | `SkillDefinition`, `SkillManifest`, `SkillTool` types |
| `manager.go` | `Manager`: discover, install, remove, activate/deactivate skills |
| `loader.go` | Parse `SKILL.md` files, detect format, validate |

### Key Types

```go
type DangerLevel string
const (
    Safe      DangerLevel = "safe"
    Medium    DangerLevel = "medium"
    Dangerous DangerLevel = "dangerous"
)

type SkillTool struct {
    Name        string          `yaml:"name" json:"name"`
    Description string          `yaml:"description" json:"description"`
    Parameters  json.RawMessage `yaml:"parameters" json:"parameters"`
    DangerLevel DangerLevel     `yaml:"danger_level" json:"danger_level"`
}

type SkillManifest struct {
    Name        string            `yaml:"name" json:"name"`
    Description string            `yaml:"description" json:"description"`
    Version     string            `yaml:"version" json:"version,omitempty"`
    Author      string            `yaml:"author" json:"author,omitempty"`
    DangerLevel DangerLevel       `yaml:"danger_level" json:"danger_level"`
    Metadata    map[string]any    `yaml:"metadata" json:"metadata,omitempty"`
    Tools       []SkillTool       `yaml:"tools" json:"tools,omitempty"`
    Instructions string           `yaml:"instructions" json:"instructions,omitempty"`
}

type SkillDefinition struct {
    Manifest     SkillManifest
    Instructions string    // Full instructions content
    Path         string    // Absolute path to skill directory
}

type SkillManager struct {
    mu          sync.RWMutex
    skillsDir   string              // data/skills/
    skills      map[string]*SkillDefinition
    activeSkills map[string]bool   // Currently active skill names

    registry    *agent.ToolRegistry    // To register skill tools
    bridge      *AppBridge             // To notify app of changes
}
```

### Integration Points

```
┌──────────────────────────────────────────────────┐
│                   app.go                          │
│  ┌────────────────────────────────────────────┐   │
│  │ skillManager *skill.Manager                │   │
│  │ agentExecutor *agent.Executor              │   │
│  └────────┬───────────────────────────────────┘   │
│           │                                       │
│           ▼                                       │
│  ┌──────────────────┐   ┌────────────────────┐   │
│  │ callAgentStream() │   │ callLLMStream()    │   │
│  │ • Inject active   │   │ • Orchestra: skill │   │
│  │   skill prompts   │   │   as role modifier │   │
│  │ • Register skill  │   └────────────────────┘   │
│  │   tools           │                            │
│  └──────────────────┘                             │
└──────────────────────────────────────────────────┘
```

### Integration: Agent Mode

In `callAgentStream()`:
1. Before building messages, query `skillManager.ActiveSkillsInstructions()` 
2. Append active skills' instructions to system prompt
3. Register skill tools in the agent's `ToolRegistry` (via `SkillManager.SyncTools(registry)`)
4. Skills stay active for the entire session

### Integration: Orchestra Mode

- Active skills' instructions are appended to orchestra role system prompts
- Skills can be assigned as role modifiers via orchestra config

## 4. Chat Commands

| Command | Action |
|---------|--------|
| `/skill` | List all installed skills with active/inactive status |
| `/skill install <path>` | Install skill from local path |
| `/skill install <git-url>` | Install skill from git repository |
| `/skill remove <name>` | Remove installed skill |
| `/skill:on <name>` | Activate a skill (or multiple: `/skill:on name1,name2`) |
| `/skill:off <name>` | Deactivate a skill |
| `/skill:off` | Deactivate all skills |

## 5. API Endpoints (Backend)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/skills` | List installed skills |
| `POST` | `/api/skills/install` | Install skill from path or URL |
| `DELETE` | `/api/skills/:name` | Remove skill |
| `GET` | `/api/skills/:name` | Get skill details |
| `PUT` | `/api/skills/active` | Set active skills list |
| `GET` | `/api/skills/active` | Get active skills list |

## 6. Bridge Interface Additions (FullBridge)

```go
type FullBridge interface {
    // ... existing methods
    
    // Skill management
    ListSkills() []skill.SkillDefinition
    InstallSkill(path string) error
    RemoveSkill(name string) error
    GetSkill(name string) (*skill.SkillDefinition, error)
    SetActiveSkills(names []string) error
    GetActiveSkills() []string
}
```

## 7. Data Flow

```
User: "/skill:on brainstorming"
  → Flutter sends PUT /api/skills/active {"names":["brainstorming"]}
  → Server → App.SetActiveSkills(["brainstorming"])
  → SkillManager marks brainstorming active

User: "Hey, şu feature için fikir üret"
  → Flutter sends POST /api/send/stream
  → App.SendMessageStream()
  → App.callAgentStream() or callLLMStream()
  → BuildMessages():
     • Start with base system prompt
     • Append active skill instructions
     • Append memory context
     → Send to LLM
```

## 8. Error Handling

- Skill parse errors → logged, skill marked invalid, not loaded
- Missing `SKILL.md` → directory ignored
- Duplicate skill names → last loaded wins (with warning)
- Install from invalid path/URL → clear error message to user
- Tool name conflicts → skill tools prefixed with `skill_<name>_<toolname>` to avoid collisions

## 9. Testing Strategy

| Test | Scope |
|------|-------|
| Parse SKILL.md (with/without tools, with/without instructions field) | unit |
| Discover skills in directory | unit |
| Activate/deactivate skills | unit |
| Instruction injection into system prompt | integration |
| Skill tool registration into agent registry | integration |
| `/skill` command parsing | unit |
| Install from path (copy to skills dir) | unit |
