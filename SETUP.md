# Loco Development Setup

## Quick Start

```bash
# 1. Install Go (if needed)
brew install go  # macOS
# or visit https://go.dev/dl/

# 2. Install all dev tools
make install-tools

# 3. Run the linters to check everything
make lint

# 4. Run tests
make test

# 5. Start developing!
make watch  # TDD mode
```

## Editor Setup

### Zed
- Go support is built-in via gopls
- Format on save should work automatically
- Settings: `cmd+,` → Go → Enable all features

### VSCode
- Install "Go" extension by Google
- It will prompt to install/update tools
- Enable "Format on Save" in settings

### Vim/Neovim
- Use vim-go or nvim-lspconfig with gopls

## Linting Setup

We use MAXIMUM strictness:
- **golangci-lint** - 50+ linters enabled!
- **pre-commit** - Runs on every commit
- **staticcheck** - Additional static analysis

## Pre-commit Hooks

After running `make install-tools`, every commit will:
1. Format your code
2. Fix imports
3. Run all linters
4. Run tests
5. Check security issues

If anything fails, the commit is blocked!

## Manual Commands

```bash
make lint       # Run all linters
make lint-fix   # Auto-fix what can be fixed
make security   # Security scan
make pre-commit # Run pre-commit manually
```

## First Commit

```bash
git init
git add .
git commit -m "Initial commit: Loco project setup"
# Pre-commit will run and check EVERYTHING
```

## If Pre-commit Fails

1. Read the error message
2. Run `make lint-fix` to auto-fix
3. Fix any remaining issues manually
4. Try committing again

The strictness is intentional - we want perfect code from day one! 🚀