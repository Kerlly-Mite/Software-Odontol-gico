FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/sigo-api .

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/sigo-api /usr/local/bin/sigo-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sigo-api"]
