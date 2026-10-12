FROM golang:1.26-alpine AS adapter
WORKDIR /src
COPY go.mod ./
COPY adapter/ ./adapter/
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/railway-adapter ./adapter

FROM ghcr.io/yetone/magpie:0.1.1203
COPY --from=adapter /out/railway-adapter /railway-adapter
ENV PORT=8080 MAGPIE_ADDR=0.0.0.0:3425
EXPOSE 8080
ENTRYPOINT ["/railway-adapter"]
CMD []
