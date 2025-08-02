# Knowledge Generation Refactor: Specialized Analysis with Weighted Inputs

## Overview
Refactor the knowledge generation system to use specialized analysis perspectives with configurable weights for code vs documentation, creating a more nuanced understanding of projects.

## Current State
- All 4 knowledge files attempt to analyze everything equally
- Documentation (markdown) is treated the same as code files
- Missing the "why" and project purpose in analyses
- Phase 1: structure.md + patterns.md (parallel)
- Phase 2: context.md + overview.md (parallel, both receive structure + patterns)

## Proposed Architecture

### New Dependency Flow
```
structure.md ──→ patterns.md ─┐
                              │
context.md ───────────────────┴──→ overview.md
```

### Specialized Roles with Weights

**1. structure.md - Code Architecture Focus**
- Weight: 80% code, 20% markdown
- Analyzes: Directory layout, module organization, dependencies
- Ignores: Most documentation claims, focuses on actual code structure

**2. patterns.md - Code Patterns & Conventions**  
- Weight: 70% code, 30% markdown
- Receives: structure.md output
- Analyzes: Coding patterns, naming conventions, common operations
- Light markdown reading for documented conventions

**3. context.md - Documentation & Purpose Focus**
- Weight: 20% code, 80% markdown  
- Prioritizes: README.md, CLAUDE.md, docs/*.md
- Extracts: Project purpose, philosophy, roadmap, decisions
- Independent of code analysis

**4. overview.md - Synthesis & Truth Reconciliation**
- Receives: structure.md, patterns.md, context.md
- Reconciles: Documentation claims vs code reality
- Highlights: Discrepancies, outdated docs, true project state

## Implementation Details

### 1. File Importance Calculation Update
```go
type FileWeight struct {
    CodeWeight     float64
    MarkdownWeight float64
}

// Per knowledge file type
var weights = map[string]FileWeight{
    "structure": {CodeWeight: 0.8, MarkdownWeight: 0.2},
    "patterns":  {CodeWeight: 0.7, MarkdownWeight: 0.3},
    "context":   {CodeWeight: 0.2, MarkdownWeight: 0.8},
}
```

### 2. Markdown Priority List
Key markdown files to prioritize (by importance):
1. README.md / README.*.md
2. CLAUDE.md (AI context)
3. docs/overview.md or similar
4. ARCHITECTURE.md
5. CONTRIBUTING.md
6. docs/*.md
7. Any .md with recent timestamps

### 3. Updated Knowledge Generator Flow
```go
// Phase 1: Structure (independent)
structureContent := generateStructure(codeFiles, markdownFiles, weights["structure"])

// Phase 2: Patterns (depends on structure)  
patternsContent := generatePatterns(codeFiles, markdownFiles, weights["patterns"], structureContent)

// Phase 3: Context (independent, markdown-focused)
contextContent := generateContext(codeFiles, markdownFiles, weights["context"])

// Phase 4: Overview (synthesis of all)
overviewContent := generateOverview(structureContent, patternsContent, contextContent)
```

### 4. Prompt Updates

**Structure prompt additions:**
- "Focus primarily on actual code structure, not documentation claims"
- "Identify entry points, core modules, and dependencies from code"

**Context prompt additions:**
- "Extract the project's purpose, mission, and target users from documentation"
- "Identify key design decisions and development philosophy"
- "Note any roadmap, todo lists, or future plans mentioned"

**Overview prompt additions:**
- "Compare what the documentation claims vs what the code actually does"
- "Identify any discrepancies or outdated information"
- "Provide the real picture of the project's current state"

## Configuration
Add to `.loco/settings.json`:
```json
{
  "knowledgeWeights": {
    "structure": {"code": 0.8, "markdown": 0.2},
    "patterns": {"code": 0.7, "markdown": 0.3},
    "context": {"code": 0.2, "markdown": 0.8}
  }
}
```

## Benefits
1. **Better Context Understanding**: Documentation-focused analysis captures the "why"
2. **Code Truth**: Pure code analysis isn't biased by potentially outdated docs
3. **Discrepancy Detection**: Overview can spot when docs and code disagree
4. **Flexible**: Weights can be tuned per project type
5. **Parallel Efficiency**: Context can run independently while structure→patterns runs

## Testing Strategy
1. Test with projects that have good docs (should capture purpose well)
2. Test with projects with outdated docs (should detect discrepancies)
3. Test with code-only projects (should still work with adjusted weights)
4. Measure quality improvement in understanding project purpose

This refactor will make the knowledge generation system more intelligent and nuanced, providing both accurate code analysis and proper context understanding.