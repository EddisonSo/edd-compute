FROM golang:1.24-alpine AS builder

ENV GOTOOLCHAIN=auto
WORKDIR /src

# Copy go-gfs dependency (provided by build context)
COPY go-gfs /go-gfs

# Copy edd-compute source
COPY edd-compute/go.mod edd-compute/go.sum /src/
COPY edd-compute /src/

# Update replace directive to match Docker build context paths
RUN sed -i 's|replace eddisonso.com/go-gfs => ../go-gfs|replace eddisonso.com/go-gfs => /go-gfs|' go.mod

RUN go mod download
RUN CGO_ENABLED=0 go build -o /out/edd-compute .

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /out/edd-compute /app/edd-compute

EXPOSE 8080
ENTRYPOINT ["/app/edd-compute"]
