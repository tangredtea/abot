# Lightweight sandbox image for abot agent exec commands.
# Designed for use with gVisor (runsc) runtime for kernel-level isolation.
#
# Build:
#   docker build -f sandbox.Dockerfile -t abot/sandbox:latest .
#
# Usage with gVisor:
#   docker run --rm --runtime=runsc --network=none --read-only \
#     --memory=512m --cpus=0.5 --pids-limit=256 \
#     --tmpfs=/tmp:size=100m,exec --tmpfs=/home/sandbox:size=100m \
#     -v /path/to/workspace:/workspace -w /workspace --user 1000:1000 \
#     abot/sandbox:latest sh -c "npm install express"

FROM node:20-alpine

RUN apk add --no-cache \
    bash \
    curl \
    git \
    jq \
    python3 \
    py3-pip \
    && rm -rf /var/cache/apk/*

RUN adduser -D -u 1000 sandbox

WORKDIR /workspace

USER 1000:1000

ENV HOME=/home/sandbox \
    NPM_CONFIG_CACHE=/tmp/.npm \
    PIP_CACHE_DIR=/tmp/.pip \
    YARN_CACHE_FOLDER=/tmp/.yarn

CMD ["sh"]
