# DDAID Technical Guide: How Shared AI Context Works

## Overview

Documentation-Driven AI Development (DDAID) is a system for maintaining persistent shared context between human developers and AI collaborators. This guide explains the technical architecture and implementation details.

## Core Concepts

### 1. Shared Context vs. Documentation

Traditional documentation is written for humans to read. Shared AI context is designed as a persistence layer for AI memory:

- **Format**: Structured markdown optimized for AI parsing
- **Updates**: Automatic via git-based change detection
- **Scope**: Project-specific patterns, decisions, and knowledge
- **Access**: Available to any AI model working on the project

### 2. The Three-Layer Architecture

```
┌─────────────────────────────────────┐
│         Application Layer           │
│  (Your code, commits, changes)      │
└─────────────────────────────────────┘
                 ▼
┌─────────────────────────────────────┐
│      Context Detection Layer        │
│  (Git integration, file analysis)   │
└─────────────────────────────────────┘
                 ▼
┌─────────────────────────────────────┐
│    Context Persistence Layer        │
│  (Markdown files in .loco/)         │
└─────────────────────────────────────┘
```

## Implementation Components

### Git-Based Change Detection

DDAID uses git to intelligently detect what needs updating:

1. **File-Level Hashing**: Each file gets a git blob hash
2. **Incremental Updates**: Only changed files trigger context updates
3. **Commit Awareness**: Context updates can be triggered by commits

```bash
# Example: How DDAID detects changes
git hash-object auth.go  # abc123...
# After edit
git hash-object auth.go  # def456... (changed!)
# DDAID updates only auth-related context
```

### Specialized Context Agents

Different agents maintain different aspects of the shared context:

#### Architecture Agent
- **Watches**: Structural changes, new patterns, refactoring
- **Maintains**: `ARCHITECTURE.md` with current system design
- **Example Update**: "Added Repository pattern to data layer"

#### API Agent  
- **Watches**: Endpoint changes, request/response formats
- **Maintains**: `API.md` with current API surface
- **Example Update**: "POST /auth/login now returns JWT token"

#### Security Agent
- **Watches**: Auth flows, permission checks, crypto usage
- **Maintains**: `SECURITY.md` with security patterns
- **Example Update**: "All endpoints now require Bearer token"

#### Performance Agent
- **Watches**: Optimization patterns, caching strategies
- **Maintains**: `PERFORMANCE.md` with perf decisions
- **Example Update**: "Added Redis caching for user sessions"

### Context Storage Format

Context is stored as structured markdown in `.loco/context/`:

```markdown
# ARCHITECTURE.md
Generated: 2025-01-15T10:30:00Z
Model: qwen2.5-coder:7b

## Current Architecture

### Authentication
- Pattern: JWT with refresh tokens
- Storage: Redis for sessions
- Decision: Chose JWT over sessions for stateless API
- Added: 2025-01-10 in commit abc123

### Data Layer
- Pattern: Repository pattern with interfaces
- Database: PostgreSQL with sqlx
- Decision: Raw SQL over ORM for performance
- Added: 2025-01-12 in commit def456
```

### The Context Feedback Loop

1. **Code Change** → Developer modifies files
2. **Detection** → Git detects which files changed
3. **Analysis** → Relevant agents analyze changes
4. **Update** → Agents update their context files
5. **Next Session** → AI loads updated context automatically

## Technical Implementation Details

### Progressive Analysis Pipeline

DDAID uses a three-tier analysis system:

#### Tier 1: Quick Analysis (2-3 seconds)
- Basic project structure detection
- Language and framework identification
- Rough project type classification
- Uses small model (3B-7B parameters)

#### Tier 2: Detailed Analysis (30s-2min)
- File-by-file analysis with parallel workers
- Importance scoring and dependency mapping
- Architectural pattern detection
- Uses small model with high parallelism

#### Tier 3: Knowledge Synthesis (2-5min)
- Multi-model synthesis of findings
- Generates comprehensive context documents
- Creates cross-cutting insights
- Uses medium model (14B+ parameters)

### Caching Strategy

```json
{
  "head_commit": "abc123...",
  "is_dirty": false,
  "file_hashes": [
    {
      "file_path": "auth.go",
      "git_hash": "def456...",
      "last_analysis": "2025-01-15T10:30:00Z",
      "analysis": { /* cached analysis */ }
    }
  ]
}
```

### Model Compatibility

DDAID works with any AI model by using standard markdown format:

- **Local Models**: Via LM Studio, Ollama, etc.
- **Cloud Models**: OpenAI, Anthropic, Google
- **Format**: Plain markdown, no proprietary extensions
- **Switching**: Context travels with code, not tied to model

## Integration Patterns

### CLI Integration (Current)

```bash
# Automatic context loading
$ loco
> AI loads .loco/context/* automatically

# Manual analysis triggers
$ loco /analyze-files
> Updates context based on changes

# Context inspection
$ loco /knowledge
> Shows current context state
```

### Future Integration Patterns

#### Git Hook Integration
```bash
# .git/hooks/post-commit
loco-update-context --incremental
```

#### CI/CD Integration
```yaml
# .github/workflows/ddaid.yml
- name: Update AI Context
  run: loco analyze --update-context
```

#### IDE Integration
```json
// .vscode/settings.json
{
  "ddaid.autoUpdate": true,
  "ddaid.updateOnSave": true
}
```

## Performance Characteristics

### Analysis Performance

| Operation | Files | Time | Model |
|-----------|-------|------|--------|
| Quick Analysis | All | 2-3s | 3B |
| Detailed Analysis | 100 | 30s | 7B |
| Detailed Analysis | 1000 | 2-3min | 7B |
| Knowledge Synthesis | N/A | 2-5min | 14B+ |

### Incremental Update Performance

| Changed Files | Analysis Time | Context Update |
|--------------|---------------|----------------|
| 1-10 | 5-10s | <1s |
| 10-50 | 30-60s | 2-3s |
| 50-200 | 2-3min | 5-10s |

## Best Practices

### 1. Context Granularity
- Keep context focused on architectural decisions
- Avoid implementation details that change frequently
- Focus on the "why" not the "what"

### 2. Agent Specialization
- Each agent should have a clear domain
- Avoid overlap between agent responsibilities
- Agents should be additive, not redundant

### 3. Update Frequency
- Batch small changes for efficiency
- Trigger immediate updates for architectural changes
- Use incremental updates whenever possible

### 4. Model Selection
- Small models (3B-7B) for file analysis
- Medium models (14B+) for synthesis
- Large models (70B+) for complex reasoning (optional)

## Comparison with Existing Approaches

### vs. Static Documentation Generators

| Feature | Static Generators | DDAID |
|---------|------------------|--------|
| Updates | Manual or build-time | Automatic on change |
| AI-Aware | No | Yes |
| Context Preservation | No | Yes |
| Incremental | No | Yes |

### vs. Code Comments

| Feature | Code Comments | DDAID |
|---------|--------------|--------|
| Scope | Local to code | Project-wide |
| AI Accessible | Only if in context | Always available |
| Maintenance | Manual | Automatic |
| Evolution Tracking | No | Yes |

### vs. Cloud AI Memory

| Feature | Cloud AI Memory | DDAID |
|---------|----------------|--------|
| Privacy | Cloud-stored | Local-first |
| Portability | Vendor-locked | Git-portable |
| Team Sharing | Platform-specific | Via version control |
| Offline | No | Yes |

## Security Considerations

1. **Local-First**: Context never leaves your machine unless you commit it
2. **Gitignore**: `.loco/` can be gitignored for private context
3. **Selective Sharing**: Choose which context to share with team
4. **No Secrets**: Context agents avoid capturing credentials

## Future Directions

### Enhanced Agent Intelligence
- Agents that understand semantic changes, not just textual
- Cross-agent collaboration for comprehensive updates
- Learning from developer corrections

### Standardization
- OpenAI-compatible context format
- Industry-standard context exchange protocol
- Plugin system for custom agents

### Integration Ecosystem
- Native IDE support
- Git platform integration (GitHub, GitLab)
- CI/CD pipeline integration

## Getting Started with Implementation

```bash
# Clone and build Loco (reference implementation)
git clone https://github.com/billie-coop/loco
cd loco
go build

# Run initial analysis
./loco
/analyze-files

# View generated context
ls -la .loco/context/

# Use with any AI
cat .loco/context/ARCHITECTURE.md | gpt-4
cat .loco/context/API.md | claude
```

## Contributing to DDAID

The DDAID philosophy is bigger than any single implementation. We encourage:

1. **Alternative Implementations**: Build DDAID for your favorite language/framework
2. **Agent Development**: Create specialized agents for new domains
3. **Integration Tools**: Build bridges to existing workflows
4. **Research**: Study optimal context formats and update strategies

---

*This technical guide is part of the DDAID (Documentation-Driven AI Development) project. For the philosophy and vision, see the [DDAID Manifesto](ddaid-manifesto-v2.md).*