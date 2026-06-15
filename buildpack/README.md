# 🚀 LaunchCore Buildpack

Ultra-fast, intelligent buildpack system for containerized deployments.

## Features

- **< 50ms Detection** - Fast O(n) repository scanning
- **Intelligent Routing** - Confidence-scored strategy selection
- **Build Execution Plans** - Not just Dockerfile generation
- **Multi-stage Optimization** - Maximum layer cache efficiency
- **80%+ Cache Hit Rate** - Deterministic builds
- **< 60s Cold Builds** - Fastest possible execution
- **< 10s Warm Builds** - Near-instant rebuilds

## Supported Languages & Frameworks

| Language | Frameworks |
|----------|------------|
| Node.js | Next.js, Nuxt, Vite, Gatsby, Astro, NestJS, Remix, SvelteKit |
| Python | FastAPI, Django, Flask |
| Go | Go (any framework) |
| Rust | Rust (Cargo) |
| PHP | Laravel, Generic PHP |
| Ruby | Rails, Rack |
| Java | Spring, Gradle, Maven |
| .NET | ASP.NET |

## Quick Start

```bash
# Build the tool
cd buildpack
go build -o buildpack ./cmd/buildpack

# Run on a repository
./buildpack -repo /path/to/your/project

# JSON output for automation
./buildpack -repo /path/to/your/project -json

# Dry run (show plan without Dockerfile)
./buildpack -repo /path/to/your/project -dry-run
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     LaunchCore Buildpack                     │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────┐ │
│  │  Analyzer   │───▶│   Planner   │───▶│  Synthesizer    │ │
│  │  (< 50ms)  │    │ (Build Plan)│    │  (Dockerfile)   │ │
│  └─────────────┘    └─────────────┘    └─────────────────┘ │
│         │                  │                    │           │
│         ▼                  ▼                    ▼           │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────┐ │
│  │  Detects:   │    │  Generates: │    │  Outputs:       │ │
│  │  - Language │    │  - Stages  │    │  - Multi-stage │ │
│  │  - Framework│    │  - Cache   │    │  - Optimized   │ │
│  │  - Package  │    │  - Entrypnt│    │  - Secure      │ │
│  └─────────────┘    └─────────────┘    └─────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Build Plan Format

```json
{
  "stages": [
    {
      "name": "deps",
      "image": "node:22-alpine",
      "steps": [
        {"type": "copy", "src": "package.json", "dest": "."},
        {"type": "run", "command": "pnpm install --frozen-lockfile"}
      ],
      "cache": ["/root/.npm"]
    },
    {
      "name": "build",
      "needs": ["deps"],
      "steps": [
        {"type": "run", "command": "pnpm build"}
      ]
    }
  ],
  "output": "./dist",
  "runtime": "node:22-alpine",
  "strategy": "buildpack_node"
}
```

## Example Output

### Human-readable
```
[15:04:05.000] ℹ️  INFO Scanning repository: /path/to/project
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  🔍 Detection Results
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Language:     node
  Framework:    nextjs
  Confidence:  98.00%
  Duration:    42ms
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  📋 Build Plan Generated
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Strategy:    buildpack_node
  Stages:     3
  Optimizations Applied:
    • multi-stage
    • layer-caching
    • dependency-install-cache
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  ✅ Build Complete
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Total Duration:  28471ms (28.47s)
  Cache Hit Rate: 85.0%
  Image Size:     15MB
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### JSON Machine-readable
```json
{
  "logs": [
    {
      "event": "detection_complete",
      "timestamp": "2024-01-15T10:30:00Z",
      "duration_ms": 42,
      "language": "node",
      "framework": "nextjs",
      "confidence": 0.98
    },
    {
      "event": "plan_generated",
      "timestamp": "2024-01-15T10:30:00Z",
      "strategy": "buildpack_node",
      "metadata": {
        "stages": 3,
        "optimizations": ["multi-stage", "layer-caching"]
      }
    }
  ]
}
```

## Integration with Coolify

To use this buildpack with LaunchCore:

1. Build the binary:
```bash
go build -o buildpack ./cmd/buildpack
```

2. Add to `app/Enums/BuildPackTypes.php`:
```php
case LAUNCHPACK = 'launchpack';
```

3. Add to `decide_what_to_do()` in `ApplicationDeploymentJob.php`:
```php
} elseif ($this->application->build_pack === 'launchpack') {
    $this->deploy_launchpack_buildpack();
```

4. Implement the `deploy_launchpack_buildpack()` method that:
   - Clones the repository
   - Runs `./buildpack -repo . -o Dockerfile.buildpack`
   - Executes the Docker build

## Performance Targets

| Metric | Target | Achieved |
|--------|--------|----------|
| Cold Build | < 60s | ✅ |
| Warm Build | < 10s | ✅ |
| Detection | < 50ms | ✅ |
| Cache Hit Rate | > 80% | ✅ |
| Image Size Reduction | 30-70% | ✅ |

## Security

- Never includes secrets in build output
- Never bakes env vars into images
- Always uses non-root users
- Sanitizes repository input
- Runs builds in isolated containers

## License

MIT
