# crudgen

A Go-based CRUD generator for generating boilerplate code for microservices following clean architecture patterns.

## Installation

```bash
# 1. Clone the repository
git clone https://github.com/KaiyrkhanSerik/code-generator.git
cd code-generator

# 2. Build the binary
go build -o crudgen

# 3. Create an alias (add to ~/.bashrc or ~/.zshrc)
echo "alias crudgen='$(pwd)/crudgen'" >> ~/.bashrc
source ~/.bashrc
```

## Usage

```bash
crudgen -name <EntityName> -project-name <ProjectName>
```

### Parameters

- `-name`: Entity name (must start with uppercase letter, e.g., "Post", "User")
- `-project-name`: Project name (must start with uppercase letter, e.g., "Blog", "Auth")

### Example

```bash
crudgen -name Post -project-name Blog
```

This will generate:
- Proto file definitions
- Domain models and services
- Repository layer (PostgreSQL)
- Use case layer
- gRPC handlers
- DTO converters

## Generated Structure

```
.
├── api/proto/<project>_v1/<entity>.proto
├── internal/
│   ├── domain/<entity>/
│   │   ├── model/
│   │   ├── repo/pg/
│   │   └── <entity>.go
│   ├── usecase/<entity>/
│   └── handler/grpc/
│       └── dto/
```

## Post-Generation Steps

After running the generator, you need to:

1. Create database migration for the generated table
2. Run `make generate-proto` to generate protobuf code
3. Register the generated code in `internal/app/app.go`
   - Import domain, repository, and usecase packages
   - Initialize services
   - Register in gRPC server and gateway

## Requirements

- Go 1.25+
- PostgreSQL
- gRPC and protobuf tools (for code generation)
