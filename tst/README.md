Here is the procedure to get a local developmeny environment based on kind.

You need :

* docker
* kind
* kubectl


# Create the cluster

```
kind create cluster --config kind-config.yaml
```

# Configure context

```
kubectl cluster-info --context kind-kind
```

# Deploy nginx ingress controller

```
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
```

```
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

# Deploy applications

```
# deploy postgresql (there is no persistence of data)
kubectl apply -f postgres

# deploy redis (there is no persistence of data)
kubectl apply -f redis

# deploy importer (will create the tables...)
kubectl apply -f k8see-importer

# deploy exporter 
kubectl apply -f k8see-exporter

# deploy the webui
kubectl apply -f portal-ui

# Create the ingress
kubectl apply -f ingress.yaml
```

You should be able to see the interface http://localhost

# When finished, destroy the cluster

```
kind delete cluster
```
