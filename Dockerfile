# syntax=docker/dockerfile:1
FROM golang:1.17

WORKDIR /build
COPY ./ /build

RUN go mod download
RUN make dumpr

RUN mkdir /app

RUN bash -c 'mkdir -p {/app,/conf};cp /build/bin/dumpr /app;cp /build/responses.yaml /conf'

EXPOSE 8080
EXPOSE 8081

CMD [ "/app/dumpr", "--responses=/conf/responses.yaml" ]
