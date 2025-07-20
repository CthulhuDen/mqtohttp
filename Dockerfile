# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /mqtohttp .


FROM alpine

CMD ["/mqtohttp"]
ENV MQTT_SESSION_FILE=/data/session-id.txt

VOLUME /data

COPY --from=build /mqtohttp /
