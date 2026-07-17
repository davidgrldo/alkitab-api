FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /alkitab-api ./cmd/alkitab-api

FROM gcr.io/distroless/static-debian12
COPY --from=build /alkitab-api /alkitab-api
EXPOSE 3000
ENTRYPOINT ["/alkitab-api"]
