FROM golang:1.13 as builder

ADD . /app/
WORKDIR /app/
RUN make build

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/zfaucet /app/zfaucet
COPY ./templates /app/templates
ENTRYPOINT ["/app/zfaucet"]