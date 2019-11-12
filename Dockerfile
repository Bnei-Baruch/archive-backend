ARG work_dir=/go/src/github.com/Bnei-Baruch/archive-backend

FROM golang:alpine as build

LABEL maintainer="edoshor@gmail.com"

ARG work_dir

ENV BUILD_NUMBER='' \
	GOOS=linux \
	CGO_ENABLED=0

RUN apk update && \
    apk add --no-cache \
    git

WORKDIR ${work_dir}
COPY . .
RUN go get github.com/jteeuwen/go-bindata/... && \
    go-bindata data/... && sed -i 's/package main/package bindata/' bindata.go && \
    mv bindata.go ./bindata && \
    go build -ldflags '-w -X ${work_dir}/version.PreRelease=${BUILD_NUMBER}'


FROM alpine:3.10
ARG work_dir
WORKDIR /app
COPY --from=build ${work_dir}/archive-backend .

EXPOSE 8080
CMD ["./archive-backend", "server"]
