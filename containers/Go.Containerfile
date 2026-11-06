FROM docker.io/library/golang:1.22-alpine AS build

WORKDIR /src
COPY backend/go.mod ./
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/openbiometrics-api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 biometrics \
    && adduser -S -D -H -u 10001 -G biometrics biometrics

COPY --from=build /out/openbiometrics-api /usr/local/bin/openbiometrics-api

USER 10001:10001
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/openbiometrics-api"]
