FROM golang:1.23

WORKDIR /app

COPY . .


RUN go build ./cmd/server

EXPOSE 8888

CMD ["./server"]
