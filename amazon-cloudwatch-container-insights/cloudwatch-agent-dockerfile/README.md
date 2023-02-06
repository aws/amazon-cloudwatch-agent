# CloudWatch Agent Dockerfiles

- [Dockerfile](Dockerfile) builds from the [latest release published on s3](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/install-CloudWatch-Agent-commandline-fleet.html)
- [localdeb](localdeb/Dockerfile) builds from a local deb file
- [source](source/Dockerfile) builds from source code, you can execute `make dockerized-build` at project root.

## Multi arch image

### Build multi arch image on mac

- Make sure you are using the edge version instead of stable (btw: they [just got merged into one installer](https://docs.docker.com/docker-for-mac/faqs/#where-can-i-find-information-about-stable-and-edge-releases))

```bash
# NOTE: you need to create a builder, the name does not matter, you have a default one out of box, but that does not work multi-arch
docker buildx create --name multi-builder
docker buildx use multi-builder
# Add proper tag and --push if you want to publish it
docker buildx build --platform linux/amd64,linux/arm64 .
# To build multi arch image from source code, run the following at project root
docker buildx build --platform linux/amd64,linux/arm64 -f amazon-cloudwatch-container-insights/cloudwatch-agent-dockerfile/Dockerfile .
```

### Build multi arch image manifest from single arch images

If you choose to build x86 and arm images on different machines, and create a multi arch image later.
You need to be aware of the following:

- Single arch images should already exists on registry first because the multi arch image is reference to existing images on the registry.
  - `docker buildx` is an exception because it pushes blob to registry without creating a new tag for the single arch images.
- Both [docker manifest](https://docs.docker.com/engine/reference/commandline/manifest/) command and [manifest-tool](https://github.com/estesp/manifest-tool) should work, `manifest-tool` does not requires a docker daemon.

Example using `docker manifest`

```bash
# NOTE: manifest is a experimental command, docker versions released after mid 2018 should have it 
# enable experimental in your ~/.docker/config.json with:
# {
#   "experimental": "enabled"
# }
docker manifest create cloudwatch-agent:foo --amend cloudwatch-agent:foo-arm64 --amend cloudwatch-agent:foo-amd64
docker manifest push cloudwatch-agent:foo
```

Example using `manifest-tool` and ECR, make sure to replace `{{account_id}}` and `{{aws_region}}` with your AWS account id and region.

```bash
# NOTE: the released version of manifest-tool is a bit outdated, you need to build it from source
manifest-tool push from-spec multi-arch-agent.yaml
```

```yaml
# multi-arch-agent.yaml
image: {{account_id}}.dkr.ecr.{{aws_region}}.amazonaws.com/cloudwatch-agent:foo
manifests:
  - image: {{account_id}}.dkr.ecr.{{aws_region}}.amazonaws.com/cloudwatch-agent:foo-amd64
    platform:
      architecture: amd64
      os: linux
  - image: {{account_id}}.dkr.ecr.{{aws_region}}.amazonaws.com/cloudwatch-agent:foo-arm64
    platform:
      architecture: arm64
      os: linux
```

## References

- [docker buildx](https://github.com/docker/buildx/#building-multi-platform-images)
- [Multi-arch build and images, the simple way](https://www.docker.com/blog/multi-arch-build-and-images-the-simple-way/)