FROM golang:1.25-alpine AS builder

WORKDIR /app


COPY go.mod go.sum ./
RUN go mod download

COPY internal/ ./internal/
COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main ./main.go

FROM alpine:3.22 AS runtime

RUN apk add --no-cache tzdata ca-certificates

RUN adduser -D appuser
USER appuser

WORKDIR /home/appuser

COPY --from=builder /app/main .

ENTRYPOINT ["./main"]