FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /mautic-form-proxy-api .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /mautic-form-proxy-api /usr/local/bin/mautic-form-proxy-api
EXPOSE 3000
ENTRYPOINT ["mautic-form-proxy-api"]
