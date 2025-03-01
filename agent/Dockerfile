FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o agent ./cmd

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/agent .

CMD ["./agent"]
