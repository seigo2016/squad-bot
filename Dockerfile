FROM golang:1.17.2-alpine as builder
RUN apk update && apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /go/src/app
RUN apk update && apk add git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o binary

FROM scratch as prod

ENV ROOT=/go/src/app
WORKDIR /go/src/app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder  /go/src/app/binary  /go/src/app
COPY --from=builder /go/src/app/.env /go/src/app
CMD ["/go/src/app/binary"]