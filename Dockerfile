FROM golang

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /apintsync

EXPOSE 8080

CMD ["/apintsync", "ws", "start"]