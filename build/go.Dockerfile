# syntax=docker/dockerfile:1
###############################################################################
#  СТАДИЯ СБОРКИ
###############################################################################
ARG GO_VERSION=1.23.4
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk add --no-cache git
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE_PATH
WORKDIR /src/${SERVICE_PATH}
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /service

###############################################################################
#  СТАДИЯ РАНТАЙМА
###############################################################################
FROM gcr.io/distroless/base-debian12
COPY --from=builder /service /service

ENTRYPOINT ["/service"]
