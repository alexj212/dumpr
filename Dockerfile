FROM alpine
COPY dumpr /dumpr
ENTRYPOINT ["/dumpr"]
