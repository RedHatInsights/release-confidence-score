# Claude Code Guidelines

## Code Quality Principles

### Simplicity First
- Write the simplest code that solves the problem
- Avoid abstractions until they're clearly needed (rule of three)
- No helper functions for one-time operations
- No wrapper functions that just call another function
- Inline short logic instead of extracting unnecessary functions

### No Redundancy
- Remove dead code immediately
- Don't store data that can be computed or already exists
- Don't rebuild values when you have them (e.g., don't reconstruct URLs from parts when you have the original URL)
- Check if a function/variable is actually used before keeping it
- If data flows IN somewhere, don't create a method to pull it back OUT

### Avoid Over-Engineering
- No premature optimization
- No "just in case" parameters or configurations
- No backward compatibility shims unless explicitly requested
- Don't add error handling for impossible scenarios
- Trust internal code; only validate at system boundaries

### Consistency
- Use consistent naming across similar files (e.g., GitHub and GitLab implementations)
- Match existing patterns in the codebase
- Same error message format across similar functions
- Same variable naming conventions

### GitHub/GitLab Parity
The `git/github/` and `git/gitlab/` packages implement the same `GitProvider` interface. When modifying one:
- Check if the same change applies to the other platform
- Keep function signatures, error messages, and behavior consistent
- If a fix applies to one platform, it likely applies to both (e.g., URL regex fixes)
- When in doubt about whether a change should be mirrored, ask the user before proceeding

### Before Writing Code
- Read existing code first to understand patterns
- Question whether the feature/change is needed
- Ask: "Is there a simpler way?"
- Ask: "Does this data already exist somewhere?"
- Ask: "Can I reuse existing code?"

## Go-Specific Guidelines

### Naming
- Use descriptive but concise names
- Match receiver variable to type (e.g., `d` for `documentationSource`)
- Private functions/types use camelCase
- Only export what's needed

### Error Handling
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Don't log AND return errors (pick one)
- Fatal errors should return, not just log

### Interfaces
- Keep interfaces minimal (1-3 methods)
- Name interfaces after what they do, not what they are
- Only create interfaces when you need abstraction

### Testing
- Test behavior, not implementation
- Use table-driven tests for multiple cases
- Mock at boundaries (SDK clients, HTTP)

## Review Checklist

Before submitting code, verify:
- [ ] No unused imports, variables, or functions
- [ ] No duplicate logic
- [ ] No unnecessary helper functions
- [ ] Consistent with existing codebase patterns
- [ ] Error messages are clear and consistent
- [ ] No over-engineered abstractions
- [ ] Comments explain "why", not "what"
