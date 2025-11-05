# Multi-Architecture Docker Build Guide

This guide explains how to build Docker images that support both AMD64 and ARM64 architectures.

## Quick Start

### Option 1: Build and Push Multi-Arch Image (Recommended)
```bash
# One command to build for both architectures and push
make docker-build-and-push IMAGE=your-registry/cloudwatch-agent:v1.0.0
```

### Option 2: Build Multi-Arch with Latest Tag
```bash
# Builds and pushes with both version tag and :latest
make docker-build-multiarch IMAGE=your-registry/cloudwatch-agent:v1.0.0
```

### Option 3: Build Locally for Testing (Single Architecture)
```bash
# For AMD64 (loads into local Docker)
make docker-build-amd64

# For ARM64 (loads into local Docker)
make docker-build-arm64
```

## Understanding Multi-Arch Builds

### What is a Multi-Arch Image?

A multi-arch (manifest list) image is a single image tag that contains multiple architecture-specific images. When you pull the image, Docker automatically selects the correct architecture for your platform.

**Example:**
```bash
# Same command works on both AMD64 and ARM64 nodes
docker pull amazon/cloudwatch-agent:latest
```

Docker will automatically pull:
- `linux/amd64` image on x86_64 machines
- `linux/arm64` image on ARM machines

### How It Works

1. **Build binaries** for both architectures
2. **Create Docker images** for each architecture
3. **Create a manifest list** that references both images
4. **Push to registry** - the manifest list becomes the image tag

## Prerequisites

### 1. Docker Buildx

Docker Buildx is required for multi-arch builds. It's included in Docker Desktop and recent Docker Engine versions.

**Check if buildx is available:**
```bash
docker buildx version
```

**If not available, install:**
```bash
# On Linux
sudo apt-get install docker-buildx-plugin

# Or update Docker to latest version
```

### 2. Setup Buildx Builder

**One-time setup:**
```bash
make docker-buildx-setup
```

This creates a builder instance named `multiarch-builder` that supports multiple platforms.

**Verify builder:**
```bash
docker buildx ls
```

You should see:
```
NAME/NODE           DRIVER/ENDPOINT             STATUS  PLATFORMS
multiarch-builder * docker-container                   
  multiarch-builder0 unix:///var/run/docker.sock running linux/amd64, linux/arm64, ...
```

### 3. Registry Access

You need push access to your container registry:

**Docker Hub:**
```bash
docker login
```

**Amazon ECR:**
```bash
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin <account-id>.dkr.ecr.us-east-1.amazonaws.com
```

**GitHub Container Registry:**
```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
```

## Build Commands

### Build for Both Architectures and Push

```bash
# Using default image name (amazon/cloudwatch-agent:VERSION)
make docker-build-and-push

# With custom image name
make docker-build-and-push IMAGE=myregistry/cloudwatch-agent:v1.2.3

# With custom registry and tag
make docker-build-and-push \
  IMAGE_REGISTRY=myregistry.io \
  IMAGE_REPO=cloudwatch-agent \
  IMAGE_TAG=v1.2.3
```

### Build with Multiple Tags

```bash
# Builds and pushes with both version tag and :latest
make docker-build-multiarch IMAGE=myregistry/cloudwatch-agent:v1.2.3
```

This creates:
- `myregistry/cloudwatch-agent:v1.2.3`
- `myregistry/cloudwatch-agent:latest`

### Build Without Pushing (Manifest Only)

```bash
# Build for both architectures but don't push
make docker-build
```

**Note:** This creates the manifest but doesn't load it into local Docker (buildx limitation). To test locally, use single-arch builds.

### Build for Single Architecture (Local Testing)

```bash
# Build for AMD64 and load into local Docker
make docker-build-amd64

# Build for ARM64 and load into local Docker
make docker-build-arm64

# Test the image
docker run --rm amazon/cloudwatch-agent:$(git describe --tag) --version
```

## Advanced Usage

### Custom Dockerfile

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f path/to/Dockerfile \
  -t myregistry/cloudwatch-agent:custom \
  --push \
  .
```

### Build for Specific Platforms

```bash
# Only AMD64
docker buildx build --platform linux/amd64 -t myimage:amd64 --push .

# Only ARM64
docker buildx build --platform linux/arm64 -t myimage:arm64 --push .

# Both
docker buildx build --platform linux/amd64,linux/arm64 -t myimage:latest --push .
```

### Build with Build Args

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --build-arg VERSION=1.2.3 \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  -t myimage:latest \
  --push \
  .
```

### Inspect Multi-Arch Image

```bash
# View manifest
docker buildx imagetools inspect amazon/cloudwatch-agent:latest

# Output shows:
# Name:      amazon/cloudwatch-agent:latest
# MediaType: application/vnd.docker.distribution.manifest.list.v2+json
# Digest:    sha256:...
#
# Manifests:
#   Name:      amazon/cloudwatch-agent:latest@sha256:...
#   MediaType: application/vnd.docker.distribution.manifest.v2+json
#   Platform:  linux/amd64
#
#   Name:      amazon/cloudwatch-agent:latest@sha256:...
#   MediaType: application/vnd.docker.distribution.manifest.v2+json
#   Platform:  linux/arm64
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build Multi-Arch Image

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2
      
      - name: Login to Registry
        uses: docker/login-action@v2
        with:
          registry: ${{ secrets.REGISTRY }}
          username: ${{ secrets.REGISTRY_USERNAME }}
          password: ${{ secrets.REGISTRY_PASSWORD }}
      
      - name: Build and Push
        run: |
          make docker-build-and-push IMAGE=${{ secrets.REGISTRY }}/cloudwatch-agent:${{ github.ref_name }}
```

### GitLab CI Example

```yaml
build-multiarch:
  image: docker:latest
  services:
    - docker:dind
  before_script:
    - docker login -u $CI_REGISTRY_USER -p $CI_REGISTRY_PASSWORD $CI_REGISTRY
    - docker buildx create --use
  script:
    - make docker-build-and-push IMAGE=$CI_REGISTRY_IMAGE:$CI_COMMIT_TAG
  only:
    - tags
```

## Troubleshooting

### Error: "multiple platforms feature is currently not supported for docker driver"

**Solution:** Create and use a buildx builder:
```bash
make docker-buildx-setup
```

### Error: "failed to solve: failed to push"

**Causes:**
1. Not logged into registry
2. No push permissions
3. Registry doesn't exist

**Solution:**
```bash
# Login to registry
docker login your-registry.com

# Verify credentials
docker info | grep Username

# Test push permissions
docker tag alpine:latest your-registry.com/test:latest
docker push your-registry.com/test:latest
```

### Error: "exec user process caused: exec format error"

**Cause:** Running wrong architecture image (e.g., ARM64 image on AMD64 host)

**Solution:** Ensure you're pulling the multi-arch image, not a specific architecture tag.

### Build is Very Slow

**Cause:** Building for multiple architectures requires emulation (QEMU) for non-native platforms.

**Solutions:**
1. Use native builders for each architecture (recommended for CI/CD)
2. Build on ARM64 machine for ARM64, AMD64 machine for AMD64
3. Use cloud build services (AWS CodeBuild, Google Cloud Build)

### Cannot Load Multi-Arch Image Locally

**Limitation:** `docker buildx build --load` only works for single platform.

**Workaround:**
```bash
# Build for your local architecture only
make docker-build-amd64  # On AMD64 machine
make docker-build-arm64  # On ARM64 machine
```

## Best Practices

### 1. Use Buildx Builder

Always use a dedicated buildx builder for multi-arch builds:
```bash
make docker-buildx-setup
```

### 2. Tag Properly

Use semantic versioning and include architecture in tags when needed:
```
myregistry/cloudwatch-agent:1.2.3           # Multi-arch manifest
myregistry/cloudwatch-agent:1.2.3-amd64     # Specific architecture (optional)
myregistry/cloudwatch-agent:1.2.3-arm64     # Specific architecture (optional)
myregistry/cloudwatch-agent:latest          # Latest multi-arch
```

### 3. Test Both Architectures

```bash
# Test AMD64
docker run --rm --platform linux/amd64 myimage:latest /bin/sh -c "uname -m"
# Output: x86_64

# Test ARM64
docker run --rm --platform linux/arm64 myimage:latest /bin/sh -c "uname -m"
# Output: aarch64
```

### 4. Cache Builds

Use build cache to speed up builds:
```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --cache-from type=registry,ref=myregistry/cloudwatch-agent:buildcache \
  --cache-to type=registry,ref=myregistry/cloudwatch-agent:buildcache,mode=max \
  -t myregistry/cloudwatch-agent:latest \
  --push \
  .
```

### 5. Verify Manifest

Always verify the manifest after pushing:
```bash
docker buildx imagetools inspect myregistry/cloudwatch-agent:latest
```

## Cleanup

### Remove Buildx Builder

```bash
make docker-buildx-cleanup
```

### Remove Build Cache

```bash
docker buildx prune -a
```

## Summary of Make Targets

| Target | Description | Pushes to Registry |
|--------|-------------|-------------------|
| `make docker-build` | Build multi-arch (no push) | ❌ No |
| `make docker-build-amd64` | Build AMD64 only (loads locally) | ❌ No |
| `make docker-build-arm64` | Build ARM64 only (loads locally) | ❌ No |
| `make docker-build-and-push` | Build multi-arch and push | ✅ Yes |
| `make docker-build-multiarch` | Build multi-arch with :latest tag and push | ✅ Yes |
| `make docker-buildx-setup` | Setup buildx builder | N/A |
| `make docker-buildx-cleanup` | Remove buildx builder | N/A |

## Quick Reference

```bash
# Setup (one-time)
make docker-buildx-setup

# Build and push multi-arch
make docker-build-and-push IMAGE=myregistry/cloudwatch-agent:v1.0.0

# Verify
docker buildx imagetools inspect myregistry/cloudwatch-agent:v1.0.0

# Test on different architectures
docker run --rm --platform linux/amd64 myregistry/cloudwatch-agent:v1.0.0 --version
docker run --rm --platform linux/arm64 myregistry/cloudwatch-agent:v1.0.0 --version

# Cleanup
make docker-buildx-cleanup
```

## Resources

- [Docker Buildx Documentation](https://docs.docker.com/buildx/working-with-buildx/)
- [Multi-platform Images](https://docs.docker.com/build/building/multi-platform/)
- [Docker Manifest](https://docs.docker.com/engine/reference/commandline/manifest/)
