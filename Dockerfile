ARG work_dir=/go/src/github.com/Bnei-Baruch/archive-backend
ARG build_number=dev

FROM golang:1.16-alpine3.14 as build

LABEL maintainer="edoshor@gmail.com"

ARG work_dir
ARG build_number

ENV GOOS=linux \
	CGO_ENABLED=0

RUN apk update && \
    apk add --no-cache \
    git

WORKDIR ${work_dir}
COPY . .
RUN go build -ldflags "-w -X github.com/Bnei-Baruch/archive-backend/version.PreRelease=${build_number}"

FROM alpine:3.14

RUN apk update && \
    apk add --no-cache \
    mailx \
    postfix

RUN echo "mydomain = kabbalahmedia.info" >> /etc/postfix/main.cf
RUN echo "myhostname = localhost" >> /etc/postfix/main.cf
RUN echo "myorigin = \$mydomain" >> /etc/postfix/main.cf
RUN echo "relayhost = [smtp.local]" >> /etc/postfix/main.cf

ARG work_dir
WORKDIR /app
ADD data data
COPY misc/wait-for /wait-for
COPY misc/*.sh ./
COPY misc/docker/docker-entrypoint.sh /usr/local/bin
COPY --from=build ${work_dir}/archive-backend .

ENTRYPOINT ["docker-entrypoint.sh"]

CMD ["./archive-backend", "server"]
