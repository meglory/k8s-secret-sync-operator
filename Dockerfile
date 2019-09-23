FROM golang:1.10.3 as builder
WORKDIR /go/src/github.com/meglory/k8s-secret-sync-operator
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o build/bin/k8s-secret-sync-operator github.com/meglory/k8s-secret-sync-operator/cmd/manager

FROM debian:9
COPY --from=builder /go/src/github.com/meglory/k8s-secret-sync-operator/build/bin/k8s-secret-sync-operator /usr/local/bin
ENTRYPOINT ["/usr/local/bin/k8s-secret-sync-operator"]
