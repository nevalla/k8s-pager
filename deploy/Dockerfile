FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /k8s-pager .

FROM gcr.io/distroless/static-debian12
COPY --from=builder /k8s-pager /k8s-pager
USER nonroot:nonroot
ENTRYPOINT ["/k8s-pager"]
