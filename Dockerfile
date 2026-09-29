# Build the Go binary, then copy it into a small image without the compiler.
FROM golang:1.26-alpine AS build

WORKDIR /src
COPY app/go.mod app/go.sum ./
RUN go mod download
COPY app/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/cloud-seminar-demo .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/cloud-seminar-demo /cloud-seminar-demo

ENV APP_NAME=cloud-seminar-demo \
    APP_VERSION=1.0.0 \
    PORT=8080 \
    AWS_REGION=us-west-2 \
    S3_BUCKET=prasad-cloud-demo

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/cloud-seminar-demo"]
