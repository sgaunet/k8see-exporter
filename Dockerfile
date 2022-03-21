# FROM golang:1.17.5-alpine AS builder
# LABEL stage=builder
# RUN apk add --no-cache git upx
# ENV GOPATH /go
# COPY ./ /go/src/
# WORKDIR /go/src/
# RUN echo $GOPATH
# RUN go get 
# RUN CGO_ENABLED=0 GOOS=linux go build . 
# RUN upx k8see-exporter



FROM alpine:3.15.0 AS final
LABEL maintainer="Sylvain Gaunet"
LABEL description=""
# RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl && \
#       chmod +x ./kubectl && \
#       mv ./kubectl /usr/local/bin/kubectl
RUN addgroup -S k8see_group -g 1000 && adduser -S k8see -G k8see_group --uid 1000 && mkdir /opt/k8see-exporter
WORKDIR /opt/k8see-exporter
COPY k8see-exporter /opt/k8see-exporter/k8see-exporter
RUN chmod +x /opt/k8see-exporter/k8see-exporter
USER k8see
CMD ["/opt/k8see-exporter/k8see-exporter"]
