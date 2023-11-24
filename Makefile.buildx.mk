ifeq (${TAG}, )
	export TAG=latest
endif

ifeq (${IMAGE_ORG}, )
	export IMAGE_ORG="reg.docker.alibaba-inc.com/dbplatform"
endif

# default list of platforms for which multiarch image is built
ifeq (${PLATFORMS}, )
	export PLATFORMS="linux/amd64,linux/arm64"
endif

# if IMG_RESULT is unspecified, by default the image will be pushed to registry
ifeq (${IMG_RESULT}, load)
	export PUSH_ARG="--load"
    # if load is specified, image will be built only for the build machine architecture.
    export PLATFORMS="local"
else ifeq (${IMG_RESULT}, cache)
	# if cache is specified, image will only be available in the build cache, it won't be pushed or loaded
	# therefore no PUSH_ARG will be specified
else
	export PUSH_ARG="--push"
endif

ifeq (${CUSTOM_BUILDKIT_CFG}, on)
	export BUILDKIT_CFG_ARGS="--config=$(PWD)/hack/docker/agent/buildkit.toml"
endif

# Name of the multiarch image for disk-agent and csi driver
DOCKERX_IMAGE_AGENT:=${IMAGE_ORG}/node-disk-controller:${TAG}

.PHONY: docker.buildx
docker.buildx:
	export DOCKER_CLI_EXPERIMENTAL=enabled
	@if ! docker buildx ls | grep -q container-builder; then\
		docker buildx create --platform ${PLATFORMS} --name container-builder ${BUILDKIT_CFG_ARGS} --use;\
	fi
	@docker buildx build --platform ${PLATFORMS} \
		-t "$(DOCKERX_IMAGE_NAME)" ${DBUILD_ARGS} -f $(PWD)/hack/docker/${COMPONENT}/Dockerfile.buildx \
		. ${PUSH_ARG}
	@echo "--> Build docker image: $(DOCKERX_IMAGE_NAME)"
	@echo

.PHONY: docker.buildx.agent
docker.buildx.agent: DOCKERX_IMAGE_NAME=$(DOCKERX_IMAGE_AGENT)
docker.buildx.agent: COMPONENT=agent
docker.buildx.agent: docker.buildx