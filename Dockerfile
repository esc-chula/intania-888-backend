FROM golang:1.27.1-bookworm AS build

WORKDIR /app

COPY . ./
RUN go mod download

RUN CGO_ENABLED=0 go build -o /bin/app ./cmd/main.go
RUN CGO_ENABLED=0 go build -o /bin/migrate ./cmd/migrate
RUN CGO_ENABLED=0 go build -o /bin/seed ./cmd/seed

FROM gcr.io/distroless/static-debian11

COPY --from=build /bin/app /bin
COPY --from=build /bin/migrate /bin
COPY --from=build /bin/seed /bin
COPY --from=build /app/config/auth.production.yaml /config/auth.production.yaml
COPY --from=build /app/migrations /migrations

WORKDIR /
USER 65532:65532

EXPOSE 8080

ENTRYPOINT [ "/bin/app" ]
