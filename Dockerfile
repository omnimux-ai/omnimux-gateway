FROM oven/bun:1.4.0@sha256:5ff609364c049b54eb0ff560ec96319729a972078ef2c755d758f0c6ef89c2d6 AS builder

WORKDIR /build/web
COPY web/package.json web/bun.lock ./
RUN BUN_FEATURE_FLAG_DISABLE_STREAMING_INSTALL=1 bun install --frozen-lockfile
COPY ./web ./
COPY ./VERSION /build/VERSION
RUN DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat /build/VERSION) bun run build

FROM golang:1.26.1-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS builder2
ENV GO111MODULE=on CGO_ENABLED=0 GOWORK=off

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

ADD go.mod go.sum ./
# relaykit and tokenkit are local submodules referenced via replace; their
# go.mod files must be present for go mod download to resolve the main module graph.
ADD relaykit/go.mod ./relaykit/go.mod
ADD tokenkit/go.mod ./tokenkit/go.mod
RUN go mod download

COPY . .
COPY --from=builder /build/web/dist ./web/dist
# fork: the compiler cache is a BuildKit mount, never part of the final image.
RUN --mount=type=cache,id=omnimux-go-build-linux-amd64-v1,target=/root/.cache/go-build,sharing=locked \
    printf 'OMNIMUX_GO_CACHE_BEFORE files=%s kib=%s\n' "$(find /root/.cache/go-build -type f | grep -E '/[0-9a-f]{2}/[0-9a-f]{64}-[ad]$' | wc -l)" "$(du -sk /root/.cache/go-build | cut -f1)" \
    && if go build -x -pgo="$(go list -m -f '{{.Dir}}' github.com/Calcium-Ion/moejs)/default.pgo" -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api 2>/tmp/omnimux-go-build.trace; then \
        awk 'index($0, "/pkg/tool/linux_amd64/compile ") {compile++} index($0, "/pkg/tool/linux_amd64/link ") {link++} END {printf "OMNIMUX_GO_TOOL_ACTIONS compile=%d link=%d\n", compile, link}' /tmp/omnimux-go-build.trace; \
    else build_status=$?; tail -n 80 /tmp/omnimux-go-build.trace; exit "$build_status"; fi \
    && rm /tmp/omnimux-go-build.trace \
    && printf 'OMNIMUX_GO_CACHE_AFTER files=%s kib=%s\n' "$(find /root/.cache/go-build -type f | grep -E '/[0-9a-f]{2}/[0-9a-f]{64}-[ad]$' | wc -l)" "$(du -sk /root/.cache/go-build | cut -f1)"

FROM debian:bookworm-slim@sha256:f06537653ac770703bc45b4b113475bd402f451e85223f0f2837acbf89ab020a

# fork: shared native generation queue and funding snapshots, not a build argument.
LABEL org.omnimux.generation-queue-protocol="1"
# fork: immutable support for frozen image inputs, original-task recovery and funding stages.
LABEL org.omnimux.image-task-capability="image_task_v1"

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata libasan8 wget ffmpeg \
    && (ffmpeg -hide_banner -encoders 2>/dev/null | grep -Eq "[[:space:]]libwebp[[:space:]]") \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/new-api /
COPY LICENSE NOTICE THIRD-PARTY-LICENSES.md /licenses/
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/new-api"]
