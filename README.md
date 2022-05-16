# kube-healthcheck-operator
Kubernetes Operator to Health Check HTTP and MQ endpoints using annotations

See kube-deploy-example.yaml for example kubernetes operator deployment


Example HTTP POD annotation:

    metadata:
      annotations:
        eppo.io/healthcheck-tag: "xxxx"
        eppo.io/healtcheck-protocol: "http"
        eppo.io/healthcheck-url: "https://xxxx"
        eppo.io/healthcheck-httpresponse: '200'
        eppo.io/healthcheck-interval: '360'


Example MQTT POD annotation:

    metadata:
      annotations:
        eppo.io/healthcheck-tag: "mqtt"
        eppo.io/healthcheck-protocol: "mqtt"
        eppo.io/healthcheck-url: "ssl://xxx:8883"
        eppo.io/healthcheck-interval: '360'



