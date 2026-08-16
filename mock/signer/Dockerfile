FROM golang:1.24-alpine

WORKDIR /app
COPY main.go .

RUN go mod init mock-signer && go mod tidy && go build -o mock-signer .

EXPOSE 8080
EXPOSE 4222

CMD ["./mock-signer"]