FROM golang:1.22-bookworm AS build

WORKDIR /usr/src/app/
COPY go.mod go.sum  /usr/src/app/
RUN go mod download
COPY *.go           /usr/src/app/
ENV CGO_ENABLED=0
RUN go build -o /usr/src/app/mkredirector ./...

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build --chown=nonroot:nonroot /usr/src/app/mkredirector /

EXPOSE 8080/tcp

CMD ["/mkredirector"]

