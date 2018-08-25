FROM golang:1.10 as builder

ADD https://github.com/golang/dep/releases/download/v0.5.0/dep-linux-amd64 /usr/bin/dep
RUN chmod +x /usr/bin/dep

WORKDIR $GOPATH/src/github.com/denniswinter/nginx-log-exporter
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure --vendor-only
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /nginx-log-exporter .

FROM scratch
COPY --from=builder /nginx-log-exporter ./
ENTRYPOINT ["./nginx-log-exporter"]