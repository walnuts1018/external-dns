FROM golang:1.21 AS builder
ENV ROOT=/build
RUN mkdir ${ROOT}
WORKDIR ${ROOT}

COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o main $ROOT/main.go && chmod +x ./main

FROM alpine:latest
WORKDIR /app

COPY --from=builder /build/main ./

CMD ["./main"]