# syntax=docker/dockerfile:1
# Multi-stage build: ~15 MB final image, runs as non-root.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/humanix ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/humanix /humanix
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/humanix"]
