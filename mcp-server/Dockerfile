FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go test ./... && CGO_ENABLED=0 go build -buildvcs=false -trimpath -o /mcp-server .
FROM scratch
COPY --from=build /mcp-server /mcp-server
USER 10001:10001
ENTRYPOINT ["/mcp-server"]
