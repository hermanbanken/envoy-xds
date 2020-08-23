############################
# STEP 1 build executable binary
############################
FROM golang:alpine AS builder
# Install git. Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git
WORKDIR $GOPATH/src/hermanbanken/envoy-xds/xds/
# Fetch dependencies. Using go get.
COPY go.sum go.mod ./
RUN go mod download
# Build the binary.
COPY . .
RUN go build -o /go/bin/main

############################
# STEP 2 build a small image
############################
FROM alpine
# Copy our static executable.
COPY --from=builder /go/bin/main /bin/main
# Run the hello binary.
ENTRYPOINT ["/bin/main"]

