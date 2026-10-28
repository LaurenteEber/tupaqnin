# Pinned by digest for reproducible, tamper-evident builds.
# Build stage
FROM golang:1.25@sha256:699337d620559a59b4a2bb298ad59611e535d2ee755a34cf2d2a98f37578dc80 AS build
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
