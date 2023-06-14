FROM golang:1.20 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /check-payload

FROM fedora:38
RUN dnf install -y binutils go file
COPY --from=builder /check-payload /check-payload

ENTRYPOINT ["/check-payload"]
