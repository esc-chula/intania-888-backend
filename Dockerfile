FROM golang:1.22.5-bullseye AS build

WORKDIR /app

COPY . ./
RUN go mod download

RUN CGO_ENABLED=0 go build -o /bin/app ./cmd/main.go

FROM gcr.io/distroless/static-debian11

COPY --from=build /bin/app /bin

EXPOSE 8080

ENTRYPOINT [ "/bin/app" ]
