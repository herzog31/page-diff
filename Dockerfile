FROM alpine:3.2

MAINTAINER Mark J. Becker <mjb@marb.ec>

RUN apk --update add openssl ca-certificates

RUN mkdir /go-root
RUN chmod -R 0777 /go-root
WORKDIR /go-root

COPY dist/linux_amd64_page-diff ./page-diff
RUN chmod +x ./page-diff
CMD ["./page-diff"]