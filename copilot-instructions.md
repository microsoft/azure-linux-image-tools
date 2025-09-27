This is a Go based repository with tools for os image creation and customization. The repo includes 3 main tools: imagecustomizer, osmodifier (aka EMU) and imagecreator. Please follow these guidelines when contributing:

## Code Standards

### Required Before Each Commit
- Run `make go-fmt-all` from `toolkit` to fix any formatting.
- Run `make go-mod-tidy` from `toolkit` folder to ensure go mod files are updated.
- Run `make -C toolkit/tools/imagecustomizerschemacli/` to ensure schema is probably updated if an API is changed.
- Run `make imagecustomizer-targz go-osmodifier` from `toolkit` before committing any changes to ensure proper build with no errors.

### Development Flow
- Build: `make imagecustomizer-targz go-osmodifier`
- Test: Tests are run as part of the build command

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
