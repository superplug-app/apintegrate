FROM golang

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /apintegrate

EXPOSE 8080

CMD ["/apintegrate", "ws", "start"]