FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bot ./cmd/bot

# Часовые пояса вшиты в бинарник (импорт time/tzdata), CA-сертификаты для api.telegram.org
# уже есть в static-образе — больше ничего не нужно.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bot /bot
ENTRYPOINT ["/bot"]
