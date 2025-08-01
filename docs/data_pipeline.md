# Loco Data Analysis Pipeline

## Overview
This document describes the data flow for the `/analyze-files` command and knowledge generation.

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
│                        4 Parallel Medium Models                     │
│                              (Qwen 2.5 7B)                          │
│                                                                     │
│    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│    │  STRUCTURE   │  │   PATTERNS   │  │   CONTEXT    │  │   OVERVIEW   │
│    │              │  │              │  │              │  │              │
│    │ • Directory  │  │ • Code Style │  │ • Recent     │  │ • What it    │
│    │   Layout     │  │ • Data Flow  │  │   Changes    │  │   Does       │
│    │ • Key Files  │  │ • Common     │  │ • Design     │  │ • Tech Stack │
│    │ • Module     │  │   Operations │  │   Decisions  │  │ • Key        │
│    │   Structure  │  │              │  │              │  │   Features   │
│    └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘
│           │                 │                 │                 │         │
│           ▼                 ▼                 ▼                 ▼         │
│     structure.md      patterns.md      context.md      overview.md      │
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
- **Processing**: 4 specialized medium models run in parallel
- **Order Dependencies**: 
  - Structure & Patterns run first
  - Their outputs feed into Context & Overview
- **Output**: 4 knowledge markdown files

### Phase 3: Summary Synthesis (Optional)
- **Input**: All 4 knowledge files
- **Processing**: Single medium/large model
- **Output**: Unified `codebase_summary.md`

## Timing Estimates
- Phase 1: ~26-30 seconds (depends on file count)
- Phase 2: ~14-20 seconds (4 models in parallel)
- Phase 3: ~10-15 seconds (optional)
- **Total**: ~40-65 seconds

## Context Size Management
- Small models: Default context (usually 2-4k)
- Medium models: Dynamic sizing (16k → 32k → 64k → 128k)
- Automatic retry with larger context on overflow

## Key Improvements Over v1
1. **Dependency Tracking**: Each file's imports and exports
2. **Specialized Analysis**: 4 focused models vs 1 general
3. **Living Knowledge**: Updates existing knowledge files
4. **Better Context**: Models can see relationships between files