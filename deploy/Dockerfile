# Build stage
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN CGO_ENABLED=0 go build -o /out/tupaqnin ./cmd/tupaqnin

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/tupaqnin /tupaqnin
EXPOSE 8080
ENTRYPOINT ["/tupaqnin"]
