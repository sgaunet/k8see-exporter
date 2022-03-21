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



FROM scratch AS final
WORKDIR /
COPY k8see-exporter /k8see-exporter
COPY resources/* /
USER MyUser
CMD [ "/k8see-exporter" ]