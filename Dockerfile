FROM golang:1.25-alpine as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server cmd/server/main.go

FROM scratch

COPY --from=builder /app/server /server

EXPOSE 8080

CMD ["./server", "--port=:8080"]
