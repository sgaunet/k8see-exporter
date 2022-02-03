FROM golang:1.17.5-alpine AS builder
LABEL stage=builder

RUN apk add --no-cache git upx

ENV GOPATH /go

COPY ./ /go/src/
WORKDIR /go/src/

RUN echo $GOPATH
RUN go get 
RUN CGO_ENABLED=0 GOOS=linux go build . 
RUN upx k8see-exporter



FROM alpine:3.15.0 AS final
LABEL maintainer="Sylvain Gaunet"
LABEL description=""

RUN apk update && apk add --no-cache curl bash
# RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl && \
#       chmod +x ./kubectl && \
#       mv ./kubectl /usr/local/bin/kubectl

RUN addgroup -S k8see_group -g 1000 && adduser -S k8see -G k8see_group --uid 1000

WORKDIR /opt/k8see-exporter
COPY --from=builder /go/src/k8see-exporter .
COPY entrypoint.sh /opt/k8see-exporter/entrypoint.sh 
RUN chmod +x /opt/k8see-exporter/entrypoint.sh && touch /opt/k8see-exporter/conf.yaml && chmod 777 /opt/k8see-exporter/conf.yaml

USER k8see

ENTRYPOINT ["/opt/k8see-exporter/entrypoint.sh"]
CMD [ "/opt/k8see-exporter/k8see-exporter","-f","/opt/k8see-exporter/conf.yaml" ]
