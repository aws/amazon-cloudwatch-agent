# Multi-Arch Build - Quick Start

## TL;DR

```bash
# 1. One-time setup
make docker-buildx-setup

# 2. Build for both AMD64 and ARM64, then push
make docker-build-and-push IMAGE=your-registry/cloudwatch-agent:v1.0.0

# 3. Verify
docker buildx imagetools inspect your-registry/cloudwatch-agent:v1.0.0
```

## What Changed in the Makefile

### New Targets Added

**`make docker-build-and-push`** - Build multi-arch and push to registry
```bash
make docker-build-and-push IMAGE=myregistry/cloudwatch-agent:v1.0.0
```

**`make docker-build-multiarch`** - Build multi-arch with :latest tag
```bash
make docker-build-multiarch IMAGE=myregistry/cloudwatch-agent:v1.0.0
```
Creates both `v1.0.0` and `latest` tags.

**`make docker-buildx-setup`** - Setup buildx builder (one-time)
```bash
make docker-buildx-setup
```

**`make docker-buildx-cleanup`** - Remove buildx builder
```bash
make docker-buildx-cleanup
```

### Existing Targets (Unchanged)

**`make docker-build`** - Build multi-arch (no push, no local load)
```bash
make docker-build
```

**`make docker-build-amd64`** - Build AMD64 only, load locally
```bash
make docker-build-amd64
```

**`make docker-build-arm64`** - Build ARM64 only, load locally
```bash
make docker-build-arm64
```

## Common Workflows

### Development (Local Testing)

```bash
# Build for your local architecture
make docker-build-amd64  # On AMD64 machine
# OR
make docker-build-arm64  # On ARM64 machine

# Test locally
docker run --rm amazon/cloudwatch-agent:$(git describe --tag) --version
```

### Production (Multi-Arch Release)

```bash
# Setup buildx (first time only)
make docker-buildx-setup

# Build and push
make docker-build-and-push IMAGE=public.ecr.aws/cloudwatch-agent/cloudwatch-agent:1.300050.0

# Verify both architectures
docker buildx imagetools inspect public.ecr.aws/cloudwatch-agent/cloudwatch-agent:1.300050.0
```

### CI/CD Pipeline

```bash
# In your CI/CD script
make docker-buildx-setup
make docker-build-and-push IMAGE=$REGISTRY/$REPO:$VERSION
```

## Key Differences

| Command | Architectures | Loads Locally | Pushes to Registry |
|---------|--------------|---------------|-------------------|
| `make docker-build-amd64` | AMD64 only | ✅ Yes | ❌ No |
| `make docker-build-arm64` | ARM64 only | ✅ Yes | ❌ No |
| `make docker-build` | Both | ❌ No | ❌ No |
| `make docker-build-and-push` | Both | ❌ No | ✅ Yes |

**Why can't multi-arch images load locally?**
Docker buildx limitation - multi-arch manifests can't be loaded into local Docker daemon. You must push to a registry.

## Testing Multi-Arch Images

```bash
# Pull and test AMD64
docker run --rm --platform linux/amd64 myregistry/cloudwatch-agent:v1.0.0 --version

# Pull and test ARM64
docker run --rm --platform linux/arm64 myregistry/cloudwatch-agent:v1.0.0 --version

# Check architecture
docker run --rm --platform linux/amd64 myregistry/cloudwatch-agent:v1.0.0 uname -m
# Output: x86_64

docker run --rm --platform linux/arm64 myregistry/cloudwatch-agent:v1.0.0 uname -m
# Output: aarch64
```

## Troubleshooting

### "multiple platforms feature is currently not supported"
```bash
make docker-buildx-setup
```

### "failed to push"
```bash
# Login to your registry first
docker login your-registry.com
```

### Build is slow
Multi-arch builds use emulation (QEMU) for non-native platforms. This is normal. For faster builds, use native builders for each architecture.

## Full Documentation

See `MULTIARCH-BUILD-GUIDE.md` for complete documentation.
