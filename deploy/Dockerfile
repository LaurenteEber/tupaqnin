# Pinned by digest for reproducible, tamper-evident builds.
# Build stage
FROM golang:1.27@sha256:2920bb124f1a2e12a1d39f0cab756ae0068441ee1da7655dec7f5e374944095b AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
# Empty for the public image: no development routes (B-4). The test deploy
# (H-5) builds with --build-arg TAGS=devroutes.
ARG TAGS=""
RUN CGO_ENABLED=0 go build -tags "$TAGS" -o /out/tupaqnin ./cmd/tupaqnin

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/tupaqnin /tupaqnin
EXPOSE 8080
ENTRYPOINT ["/tupaqnin"]
