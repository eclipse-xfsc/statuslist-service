FROM golang as build

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /status-list-service

FROM alpine as runner

WORKDIR /app

COPY --from=build /status-list-service /status-list-service

ENTRYPOINT ["/status-list-service"]