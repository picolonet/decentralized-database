FROM golang:1.12.5

WORKDIR $GOPATH/src/github.com/picolonet/decentralized-database

COPY ./discovery.go
COPY ./database.go

ENV GO111MODULE=on
RUN go build -o ./db .

ARG PORT_NO
ENV PORT_NO=${PORT_NO}

CMD ./db -p ${PORT_NO} -m connect
