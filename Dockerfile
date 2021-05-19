FROM gcr.io/distroless/base
EXPOSE 9999
ENTRYPOINT ["/demo-rest"]
COPY demo-rest /

