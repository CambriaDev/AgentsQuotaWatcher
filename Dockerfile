FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /agents-quota-watcher ./cmd/server

FROM alpine:3.20

COPY --from=build /agents-quota-watcher /usr/local/bin/agents-quota-watcher

EXPOSE 48080

ENTRYPOINT ["agents-quota-watcher"]
