FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mistral-sticky ./cmd/mistral-sticky

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/mistral-sticky /mistral-sticky
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/mistral-sticky"]
