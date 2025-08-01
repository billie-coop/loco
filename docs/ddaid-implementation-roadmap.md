# DDAID Implementation Roadmap

This document outlines the technical implementation plan for Documentation-Driven AI Development (DDAID) in Loco.

## Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Git Changes   │    │  File Analysis  │    │ Doc Agents      │
│                 │────│                 │────│                 │
│ • File hashes   │    │ • Incremental   │    │ • API Agent     │
│ • Status check  │    │ • Smart caching │    │ • Arch Agent    │
│ • Dirty detect  │    │ • Change focus  │    │ • CLI Agent     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │ Living Docs     │
                    │                 │
                    │ • API.md        │
                    │ • ARCH.md       │
                    │ • README.md     │
                    │ • patterns.md   │
                    └─────────────────┘
```

## Phase 1: Smart Change Detection Foundation

### 1.1 Git Hash-Based File Caching

**Goal**: Only re-analyze files that have actually changed

**Implementation**:
```go
type FileCache struct {
    FilePath    string    `json:"file_path"`
    GitHash     string    `json:"git_hash"`
    LastAnalysis time.Time `json:"last_analysis"`
    Analysis    *FileAnalysis `json:"analysis,omitempty"`
}

type ProjectCache struct {
    HeadCommit   string      `json:"head_commit"`
    IsDirty      bool        `json:"is_dirty"`
    FileHashes   []FileCache `json:"file_hashes"`
    LastUpdate   time.Time   `json:"last_update"`
}
```

**Key Functions**:
- `getFileGitHash(filePath string) string` - Get current hash of file
- `getProjectStatus() (headCommit string, isDirty bool)` - Check repo status
- `findChangedFiles(cache *ProjectCache) []string` - Detect changed files
- `updateCache(cache *ProjectCache, analyses []FileAnalysis)` - Update cache

### 1.2 Project-Level Cache Busting

**Triggers for cache invalidation**:
- `git status --porcelain` returns any output (dirty repo)
- `git rev-parse HEAD` returns different commit hash
- Any file hash differs from cached version

**Cache Strategy**:
- **Clean repo + same HEAD**: Use all caches
- **Dirty repo OR new HEAD**: Bust project-level caches, incremental file analysis
- **New files**: Add to analysis queue
- **Deleted files**: Remove from cache

### 1.3 Incremental Analysis Pipeline

**Current Flow**:
```
All Files → Small Model → Analysis → Knowledge Generation
```

**New Flow**:
```
Changed Files Only → Small Model → Incremental Analysis → Smart Doc Updates
```

**Performance Impact**:
- Typical change: 1-5 files vs 50+ files
- Analysis time: 5-15 seconds vs 30-60 seconds
- Startup time: Near-instant for unchanged repos

## Phase 2: Documentation Agent System

### 2.1 Agent Architecture

**Base Agent Interface**:
```go
type DocumentationAgent interface {
    // What files does this agent care about?
    IsRelevantChange(filePath string, analysis *FileAnalysis) bool
    
    // What documents does this agent maintain?
    ManagedDocuments() []string
    
    // Update docs based on changes
    UpdateDocumentation(changes []FileChange) error
    
    // Agent metadata
    Name() string
    Description() string
}
```

**Core Agents**:

1. **API Agent** (`internal/docs/agents/api.go`)
   - Watches: `*_handler.go`, `*_routes.go`, `*_api.go`
   - Maintains: `docs/API.md`
   - Detects: New endpoints, parameter changes, response format changes

2. **Architecture Agent** (`internal/docs/agents/architecture.go`)
   - Watches: Package structure, import patterns, major refactors
   - Maintains: `docs/ARCHITECTURE.md`
   - Detects: New modules, dependency changes, pattern shifts

3. **CLI Agent** (`internal/docs/agents/cli.go`)
   - Watches: Command definitions, flag changes, help text
   - Maintains: `README.md` (usage section)
   - Detects: New commands, changed flags, usage patterns

4. **Security Agent** (`internal/docs/agents/security.go`)
   - Watches: Auth code, crypto usage, security patterns
   - Maintains: `docs/SECURITY.md`
   - Detects: New auth flows, crypto changes, security concerns

### 2.2 Document Structure Convention

**Standard docs/ layout**:
```
docs/
├── API.md                 # API Agent
├── ARCHITECTURE.md        # Architecture Agent  
├── SECURITY.md           # Security Agent
├── PERFORMANCE.md        # Performance Agent
├── DEVELOPMENT.md        # Development patterns
└── TROUBLESHOOTING.md    # Common issues
```

**Agent Responsibility Matrix**:
| Agent | Primary Doc | Secondary Docs | Change Types |
|-------|-------------|----------------|--------------|
| API | API.md | README.md | Endpoints, schemas |
| Architecture | ARCHITECTURE.md | DEVELOPMENT.md | Structure, patterns |
| CLI | README.md | - | Commands, usage |
| Security | SECURITY.md | - | Auth, crypto, policies |

### 2.3 Smart Document Updates

**Update Strategies**:

1. **Append Updates** - Add new sections without replacing existing content
2. **Section Replace** - Update specific sections while preserving structure  
3. **Merge Updates** - Combine new info with existing content intelligently
4. **Conflict Resolution** - Handle cases where multiple agents want to update same section

**Example API Agent Update**:
```markdown
## Recent Changes

### Added Endpoints
- `POST /api/auth/login` - User authentication (added 2024-01-15)
- `GET /api/users/profile` - User profile data (added 2024-01-15)

### Modified Endpoints  
- `PUT /api/users/{id}` - Now accepts optional `avatar_url` field (updated 2024-01-15)
```

## Phase 3: Integration & Polish

### 3.1 Command Interface

**New Commands**:
- `/docs-update` - Manual trigger for doc updates
- `/docs-status` - Show which docs are current vs stale
- `/docs-agents` - List active agents and their responsibilities
- `/docs-diff` - Show what changed in docs since last commit

**Enhanced Existing Commands**:
- `/analyze-files` - Now includes incremental doc updates
- `/quick-analyze` - Also checks doc freshness
- `/knowledge` - Shows both internal knowledge and external docs

### 3.2 Conflict Resolution

**Conflict Scenarios**:
1. Multiple agents want to update same doc section
2. Human edits conflict with agent updates  
3. Agent suggests changes that contradict existing content

**Resolution Strategies**:
1. **Agent Priority** - Some agents take precedence over others
2. **Human Override** - Always preserve human edits, agents work around them
3. **Merge Proposals** - Show proposed changes for human approval
4. **Version Control** - Use git to track and resolve conflicts

### 3.3 Configuration System

**Agent Configuration** (`docs/agents.yaml`):
```yaml
agents:
  api:
    enabled: true
    model: "qwen2.5-coder:7b"
    watch_patterns: ["*_handler.go", "*_routes.go"]
    documents: ["docs/API.md"]
    
  architecture:
    enabled: true  
    model: "qwen2.5-coder:14b"
    watch_patterns: ["internal/**", "cmd/**"]
    documents: ["docs/ARCHITECTURE.md"]
    
  cli:
    enabled: true
    model: "qwen2.5-coder:7b" 
    watch_patterns: ["cmd/**", "*_command.go"]
    documents: ["README.md"]
```

## Implementation Timeline

### Week 1-2: Foundation
- [ ] Implement git hash-based file caching
- [ ] Add project status detection (dirty/clean)
- [ ] Create incremental analysis pipeline
- [ ] Update existing commands to use caching

### Week 3-4: Agent System
- [ ] Create base DocumentationAgent interface
- [ ] Implement API Agent (simplest case)
- [ ] Create document update mechanisms
- [ ] Add `/docs-update` command

### Week 5-6: Multiple Agents
- [ ] Implement Architecture Agent
- [ ] Implement CLI Agent  
- [ ] Add conflict resolution system
- [ ] Create agent configuration system

### Week 7-8: Integration & Polish
- [ ] Integrate with existing analysis pipeline
- [ ] Add comprehensive error handling
- [ ] Create agent status/monitoring commands
- [ ] Write documentation and examples

## Success Metrics

**Performance Metrics**:
- Startup time for unchanged repos: < 1 second
- Incremental analysis time: < 15 seconds for typical changes
- Doc update time: < 10 seconds per agent

**Quality Metrics**:
- Doc freshness: Always current with latest changes
- Conflict rate: < 5% of updates require human intervention
- Coverage: All major code patterns reflected in docs

**User Experience Metrics**:
- Context switch time: < 2 minutes to resume productive work
- Onboarding time: New contributors productive in < 30 minutes
- Architecture drift: Zero major inconsistencies in docs vs code

## Technical Challenges

### Challenge 1: Change Detection Accuracy
**Problem**: Distinguishing meaningful changes from trivial ones
**Solution**: Semantic analysis of changes, not just text diffs

### Challenge 2: Context Preservation
**Problem**: Agents need to understand existing doc structure/style
**Solution**: Include existing docs in agent prompts for consistency

### Challenge 3: Performance at Scale
**Problem**: Large repos with many files and frequent changes
**Solution**: Incremental processing, agent specialization, smart caching

### Challenge 4: Human-AI Collaboration
**Problem**: Balancing automation with human control
**Solution**: Clear human override mechanisms, transparency in agent actions

## Future Enhancements

**Phase 4 Ideas** (post-MVP):
- **Performance Agent**: Tracks optimization decisions and benchmarks
- **Testing Agent**: Maintains test documentation and coverage reports  
- **Deployment Agent**: Documents deployment procedures and environment configs
- **Dependency Agent**: Tracks external dependencies and update procedures
- **Cross-Project Learning**: Agents learn patterns from other projects

**Advanced Features**:
- Natural language queries: "What changed in the API since last week?"
- Proactive suggestions: "This change might affect the deployment docs"
- Integration hooks: Trigger doc updates on PR merge, release tags
- Quality metrics: Track doc accuracy, completeness, usefulness

---

*This roadmap represents the technical implementation of DDAID philosophy in Loco, focusing on practical, incremental steps toward intelligent documentation automation.*