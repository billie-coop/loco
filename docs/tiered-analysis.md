# Tiered Analysis System

## Overview

Loco uses a progressive 4-tier analysis system where each tier builds upon and refines the previous one. Each tier uses progressively larger models and is encouraged to be skeptical of the tier below it.

## Tier Structure

### Tier 1: Quick Analysis (⚡ 2-3 seconds)
- **Model**: Small (e.g., 7B)
- **Input**: File list only (no content reading)
- **Output**: `knowledge/quick/`
- **Purpose**: Instant project overview

### Tier 2: Detailed Analysis (📊 30-60 seconds)
- **Model**: Small → Medium (7B → 14B)
- **Process**:
  1. Small models analyze each file's content in parallel
  2. Medium models synthesize into 4 knowledge documents
- **Output**: `knowledge/detailed/`
- **Purpose**: Comprehensive file-level understanding

### Tier 3: Deep Analysis (💎 2-5 minutes)
- **Model**: Large (e.g., 32B+)
- **Input**: 
  - Tier 2 knowledge files
  - Raw file analysis from Tier 2
  - Instruction to be skeptical and refine
- **Output**: `knowledge/deep/`
- **Purpose**: Nuanced, high-quality documentation

### Tier 4: Full Analysis (🚀 Future)
- **Model**: XXL local or API (70B+, Claude, GPT-4)
- **Input**: Everything from previous tiers
- **Output**: `knowledge/full/`
- **Purpose**: Professional-grade documentation

## Knowledge Files

Each tier generates the same 4 files with progressively better quality:

1. **structure.md** - Code organization and architecture
2. **patterns.md** - Development patterns and conventions
3. **context.md** - Project purpose and business logic
4. **overview.md** - High-level summary and quick start

## Folder Structure

```
.loco/
├── knowledge/
│   ├── quick/          # Tier 1: Basic understanding
│   │   ├── structure.md
│   │   ├── patterns.md
│   │   ├── context.md
│   │   └── overview.md
│   ├── detailed/       # Tier 2: Comprehensive analysis
│   │   ├── structure.md
│   │   ├── patterns.md
│   │   ├── context.md
│   │   └── overview.md
│   ├── deep/          # Tier 3: Refined insights
│   │   ├── structure.md
│   │   ├── patterns.md
│   │   ├── context.md
│   │   └── overview.md
│   └── full/          # Tier 4: Professional docs
│       └── ...
├── file_analysis.json  # Raw Tier 2 data
└── analysis_cache.json # Incremental update cache
```

## Skeptical Refinement

Each tier is instructed to:
1. Review the analysis from the tier below
2. Question assumptions and generalizations
3. Add nuance and correct misunderstandings
4. Provide deeper insights based on greater capability

Example prompt structure:
```
"Here's what a less capable model analyzed. As a more powerful model, 
critically review this analysis and provide a more accurate, nuanced 
understanding..."
```

## Use Cases

- **Quick Response**: Use `knowledge/quick/` for instant context
- **Standard Development**: Use `knowledge/detailed/` for most queries
- **Critical Decisions**: Use `knowledge/deep/` for architecture choices
- **External Sharing**: Use `knowledge/full/` for documentation

## Model Requirements

Recommended model sizes by tier:
- **Tier 1**: 7B models (Qwen2.5-Coder-7B, DeepSeek-Coder-6.7B)
- **Tier 2**: 7B + 14B models (small + medium teams)
- **Tier 3**: 32B+ models (CodeQwen-32B, DeepSeek-33B)
- **Tier 4**: 70B+ or API models (Qwen-72B, Claude, GPT-4)

## Incremental Updates

The system supports incremental updates:
- Only changed files are re-analyzed
- Knowledge synthesis can be partial
- Each tier can be updated independently
- Git hash-based change detection ensures accuracy