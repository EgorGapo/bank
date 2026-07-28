FROM golang:1.26-alpine AS builder
ARG TARGET=bank                    # ← какой бинарь собирать (значение по умолчанию)
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/app ./cmd/$TARGET

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/app /app/app
CMD ["/app/app"]
