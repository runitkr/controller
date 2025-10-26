FROM scratch AS runtime

ARG user=1000
ARG group=1000

USER $user:$group
WORKDIR /app

COPY --chown=$user:$group ./main /app/main
COPY --chown=$user:$group ./public/ /app/public/

ENTRYPOINT ["/app/main"]
