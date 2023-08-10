FROM golang:1.21 as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN make

FROM fedora:38
RUN dnf -y update && dnf install -y binutils go file && dnf clean all
COPY --from=builder /app/check-payload /check-payload

ENTRYPOINT ["/check-payload"]
