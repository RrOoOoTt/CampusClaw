FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go mod tidy && go test ./... && CGO_ENABLED=0 go build -buildvcs=false -trimpath -o /campusclaw .
RUN mkdir /uploads && chown 10001:10001 /uploads
FROM scratch
COPY --from=build --chown=10001:10001 /campusclaw /campusclaw
COPY --from=build --chown=10001:10001 /uploads /uploads
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 10001:10001
ENTRYPOINT ["/campusclaw"]
