FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /mcp-frete .

# Imagem sem shell, executando como usuário não-root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /mcp-frete /mcp-frete
EXPOSE 8080
ENTRYPOINT ["/mcp-frete", "-http", ":8080"]
