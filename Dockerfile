FROM golang:alpine as build
WORKDIR /app
COPY . .
RUN apk update && apk add make
RUN make build

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/bin/dss /app/dss
EXPOSE 80
ENTRYPOINT ["/app/dss"]
