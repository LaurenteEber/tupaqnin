# Pinned by digest for reproducible, tamper-evident builds.
# Build stage
FROM golang:1.27@sha256:2669ec97bbb6e54602952b4171211116e3fe90e5f9c47619920c74554b3d7d09 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -o /out/tupaqnin ./cmd/tupaqnin

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/tupaqnin /tupaqnin
EXPOSE 8080
ENTRYPOINT ["/tupaqnin"]
