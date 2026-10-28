# Pinned by digest for reproducible, tamper-evident builds.
# Build stage
FROM golang:1.24@sha256:d2d2bc1c84f7e60d7d2438a3836ae7d0c847f4888464e7ec9ba3a1339a1ee804 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -o /out/tupaqnin ./cmd/tupaqnin

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/tupaqnin /tupaqnin
EXPOSE 8080
ENTRYPOINT ["/tupaqnin"]
