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
RUN go build -ldflags '-w -X ${work_dir}/version.PreRelease=${BUILD_NUMBER}'


FROM alpine:3.10

RUN apk update && \
    apk add --no-cache \
    mailx \
    postfix

ARG work_dir
WORKDIR /app
ADD data data
COPY misc/wait-for /wait-for
COPY misc/*.sh ./
COPY --from=build ${work_dir}/archive-backend .

EXPOSE 8080
CMD ["./archive-backend", "server"]
