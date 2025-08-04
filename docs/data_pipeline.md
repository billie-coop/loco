# Loco Data Analysis Pipeline

## Overview
This document describes the data flow for the analysis command and knowledge generation.

## Three Analysis Tiers
All tiers follow the same pipeline but with different depth:
- **Quick Analysis**: Small models, file structure only (3-5 seconds)
- **Detailed Analysis**: Medium models, key file contents (30-60 seconds)  
- **Deep Analysis**: Large models, extensive analysis with skepticism (2-5 minutes)

## Pipeline Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          PHASE 1: FILE ANALYSIS                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  git ls-files ──> Filter Files ──> Parallel Workers (10)          │
│                                           │                         │
│                                           ▼                         │
│                                    Small Model (LFM2)               │
│                                    Analyzes each file:              │
│                                    • Purpose                        │
│                                    • Importance (1-10)              │
│                                    • Summary                        │
│                                    • Dependencies (NEW)             │
│                                    • Exports (NEW)                  │
│                                    • File Type (NEW)                │
│                                           │                         │
│                                           ▼                         │
│                                   file_analysis.json                │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    PHASE 2: KNOWLEDGE GENERATION                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│                      Cascading Document Pipeline                    │
│                         (Medium Models)                             │
│                                                                     │
│                        ┌──────────────┐                            │
│                        │  STRUCTURE   │                            │
│                        │              │                            │
│                        │ • Directory  │                            │
│                        │   Layout     │                            │
│                        │ • Key Files  │                            │
│                        │ • Module     │                            │
│                        │   Structure  │                            │
│                        └──────┬───────┘                            │
│                               │                                     │
│                               ▼                                     │
│              ┌──────────────┐     ┌──────────────┐                │
│              │   PATTERNS   │     │   CONTEXT    │                │
│              │              │     │              │                │
│              │ • Code Style │     │ • Project    │                │
│              │ • Data Flow  │     │   Purpose    │                │
│              │ • Common     │     │ • Business   │                │
│              │   Operations │     │   Logic      │                │
│              │ • Dev        │     │ • Design     │                │
│              │   Patterns   │     │   Decisions  │                │
│              └──────┬───────┘     └──────┬───────┘                │
│                     │                     │                         │
│                     └──────┬──────────────┘                         │
│                            │                                        │
│                            ▼                                        │
│                   ┌──────────────┐                                │
│                   │   OVERVIEW   │                                │
│                   │              │                                │
│                   │ • Summary    │                                │
│                   │ • Tech Stack │                                │
│                   │ • Key        │                                │
│                   │   Features   │                                │
│                   │ • Quick      │                                │
│                   │   Start      │                                │
│                   └──────────────┘                                │
│                                                                     │
│  Output Files:                                                     │
│    1. structure.md (runs first)                                    │
│    2. patterns.md + context.md (run in parallel)                   │
│    3. overview.md (runs last, uses all previous)                   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
                                           │
                                           ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    PHASE 3: SUMMARY SYNTHESIS                       │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│                      Medium/Large Model (Optional)                  │
│                                                                     │
│              Combines all 4 knowledge files into:                  │
│                     codebase_summary.md                             │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘

## Data Flow Details

### Phase 1: File Analysis
- **Input**: All git-tracked files
- **Filter**: Excludes binaries, images, build artifacts
- **Processing**: 10 parallel workers using small model
- **Output**: `file_analysis.json` with enhanced metadata

### Phase 2: Knowledge Generation  
- **Input**: `file_analysis.json`
- **Processing**: Cascading pipeline with medium models
- **Order Dependencies**: 
  - Structure runs first (alone)
  - Patterns & Context run in parallel (both receive structure.md)
  - Overview runs last (receives all three previous documents)
- **Output**: 4 knowledge markdown files

### Phase 3: Summary Synthesis (Optional)
- **Input**: All 4 knowledge files
- **Processing**: Single medium/large model
- **Output**: Unified `codebase_summary.md`

## Timing Estimates

### Quick Analysis (Small models)
- Phase 1: ~2-3 seconds (file summaries with small model)
- Phase 2: ~1-2 seconds (cascading docs with small model)
- **Total**: ~3-5 seconds

### Detailed Analysis (Medium models)
- Phase 1: ~20-30 seconds (deeper file analysis)
- Phase 2: ~10-15 seconds (cascading: structure → patterns+context → overview)
- Phase 3: ~10-15 seconds (optional summary)
- **Total**: ~30-60 seconds

### Deep Analysis (Large models)
- Phase 1: ~60-90 seconds (extensive file reading)
- Phase 2: ~30-60 seconds (professional-grade cascading docs)
- Phase 3: ~15-20 seconds (optional summary)
- **Total**: ~2-5 minutes

## Context Size Management
- Small models: Default context (usually 2-4k)
- Medium models: Dynamic sizing (16k → 32k → 64k → 128k)
- Automatic retry with larger context on overflow

## Key Improvements Over v1
1. **Dependency Tracking**: Each file's imports and exports
2. **Specialized Analysis**: 4 focused models vs 1 general
3. **Living Knowledge**: Updates existing knowledge files
4. **Better Context**: Models can see relationships between files