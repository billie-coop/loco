# Tiered Analysis System

## Overview

Loco uses a progressive, tiered analysis system to provide immediate responsiveness while building increasingly sophisticated understanding of your codebase in the background. This is similar to how progressive JPEG loading works - you get a "blur hash" immediately, then progressively sharper images.

## The Three Tiers

### Tier 1: Quick Analysis (⚡ 2-3 seconds)
- **Purpose**: Instant context for immediate chat responsiveness
- **Model**: Small (XS/S) 
- **Input**: File list from `git ls-files`
- **Output**: Basic project overview including:
  - File count and types
  - Directory structure
  - Detected languages/frameworks
  - Project type (CLI, web app, library, etc.)
- **Storage**: `.loco/quick_analysis.json`

### Tier 2: Detailed Analysis (📊 30-60 seconds)
- **Purpose**: Comprehensive understanding for most development tasks
- **Model**: Small models for file analysis + Medium models for synthesis
- **Input**: Full file contents
- **Output**: Knowledge base files:
  - `structure.md` - Code organization
  - `patterns.md` - Development patterns
  - `context.md` - Project context
  - `overview.md` - High-level overview
- **Storage**: `.loco/file_analysis.json` + `.loco/knowledge/*.md`

### Tier 3: Deep Intelligence (💎 2-5 minutes)
- **Purpose**: Highest quality insights for complex questions
- **Model**: Large (L/XL)
- **Input**: All Tier 2 outputs + additional context
- **Output**: Enhanced analysis including:
  - Architectural insights
  - Code quality assessment
  - Security considerations
  - Refactoring opportunities
- **Storage**: `.loco/deep_analysis.json`

## How It Works

1. **On Session Start**:
   - Check for cached analyses
   - If no cache, start Tier 1 immediately
   - Chat is available within seconds

2. **Progressive Enhancement**:
   - Each tier runs in the background
   - Chat uses best available analysis
   - Quality improves transparently

3. **Status Indicators**:
   - Sidebar shows current analysis level
   - Status messages indicate progress
   - Users know what quality to expect

## Benefits

1. **Immediate Responsiveness**: No more waiting for analysis before chatting
2. **Resource Efficiency**: Small models do preparation work, large models add intelligence
3. **Progressive Quality**: Answers improve as better analysis becomes available
4. **Background Processing**: Analysis doesn't block user interaction

## Implementation Status

- ✅ Tier 2 implemented (`/analyze-files` command)
- 🚧 Tier 1 quick analysis (next to implement)
- 📋 Tier 3 deep analysis (planned)
- 📋 Automatic progressive enhancement (planned)

## Migration from Legacy System

The old `analyzer.AnalyzeProject()` system that ran on startup is being replaced because:
- It didn't specify which model to use (causing errors)
- It blocked startup until complete
- It couldn't leverage our new parallel analysis capabilities

The new system is non-blocking and progressive, providing a much better user experience.