# syntax=docker/dockerfile:1
#
# Infrix playground (ADOPTION-02). Pure-Go thin client; web/ + fixtures/ are
# go:embed'd, so the final image is just the static binary on distroless.
# Configure at runtime via env (see README "Deploy"); CLI flags override.

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" \
    -o /out/infrix-playground ./cmd/infrix-playground

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/infrix-playground /usr/local/bin/infrix-playground
# Bind all interfaces inside the container by default. Point at the real node
# with INFRIX_PLAYGROUND_NODE_ENDPOINT; opt into testnet with INFRIX_PLAYGROUND_KERMIT=1.
ENV INFRIX_PLAYGROUND_ADDR=0.0.0.0:8086 \
    INFRIX_PLAYGROUND_NODE_ENDPOINT=http://127.0.0.1:8080
EXPOSE 8086
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/infrix-playground"]
