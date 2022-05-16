FROM golang:1.17-alpine as builder
RUN apk add git

WORKDIR /app

COPY . .

RUN go get . && \
    CGO_ENABLED=0 go build -a -installsuffix cgo \
	-ldflags "-s -w" \
	-o kube-healthcheck .


# Use distroless as minimal base image to package the binary
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /app/kube-healthcheck /
USER nonroot:nonroot

CMD ["/kube-healthcheck"]

