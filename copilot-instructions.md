This is a Go based repository with tools for os image creation and customization. The repo includes 3 main tools: imagecustomizer, osmodifier (aka EMU) and imagecreator. Please follow these guidelines when contributing:

## Code Standards

### Required Before Each Commit
- Run `sudo make -C ./toolkit` before committing any changes to ensure proper build with no errors.
- Run `go mod tidy` to ensure go mod files are updated.

### Development Flow
- Build: `make build`
- Test: `make test`

## Repository Structure
- `toolkit/tools/imagecustomizer`: imagecustomizer executable
- `toolkit/tools/osmodifier`: os modifier executable
- `toolkit/tools/internal/`: Shared code for all tools
- `docs/`: Documentation
- `test/`: E2E VM based tests

## Key Guidelines
1. Follow Go best practices and idiomatic patterns
2. Maintain existing code structure and organization
3. Write unit tests for new functionality. Use table-driven unit tests when possible.
4. Document public APIs and complex logic. Suggest changes to the `docs/` folder when appropriate
