# RAG (Retrieval-Augmented Generation) Setup

Loco includes a RAG system for semantic code search. It can use different embedding backends depending on your needs.

## Quick Start

### Option 1: LM Studio (Default, Easy)
1. Open LM Studio
2. Download an embedding model like `nomic-embed-text-v1.5.gguf`
3. Load the model in LM Studio
4. Start Loco - it will automatically use LM Studio for embeddings
5. Use `/rag <query>` to search

**Pros:** Easy setup, works with existing LM Studio
**Cons:** Slower (~200ms per query), requires LM Studio running

### Option 2: ONNX (Fast, Pure Go) - Coming Soon
```bash
# Set environment variable
export LOCO_EMBEDDER=onnx

# Start Loco - it will download the model automatically
./loco
```

**Pros:** Very fast (~10ms per query), no external dependencies
**Cons:** Requires adding hugot dependency (not yet added)

### Option 3: Mock (Testing Only)
```bash
export LOCO_EMBEDDER=mock
./loco
```

**Pros:** No dependencies, instant
**Cons:** Not real embeddings, just for testing

## Performance Comparison

| Backend | Speed | Setup | Quality | Dependencies |
|---------|-------|-------|---------|--------------|
| ONNX | ~10ms | Automatic | Good | Pure Go (hugot) |
| LM Studio | ~200ms | Manual | Better | LM Studio server |
| Mock | ~1ms | None | None | None |

## How It Works

1. **Indexing**: On startup, Loco indexes all code files in your project
2. **Embeddings**: Each code chunk is converted to a vector (list of numbers)
3. **Search**: Your query is converted to a vector and compared with all chunks
4. **Results**: Most similar code chunks are returned

## Commands

- `/rag <query>` - Search for semantically similar code
- `/scan` - Re-run the startup scan

## Examples

```
/rag error handling
/rag database connection
/rag parse JSON
/rag test examples
```

## Adding ONNX Support

To enable the fast ONNX embedder, add this dependency:

```bash
go get github.com/knights-analytics/hugot
```

Then rebuild Loco. The ONNX backend will download models automatically (~30MB).

## Troubleshooting

### "No embedding model loaded"
- Make sure you've loaded an embedding model in LM Studio
- Models with "embed" in the name are embedding models

### Slow search
- Switch to ONNX backend for 20x faster searches
- Or reduce the number of indexed files

### No results
- The indexing might still be running (check console)
- Try a different query
- Make sure you have code files in the directory