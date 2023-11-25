#!/bin/bash


# install Vault without vault-injector
helm install vault vault-helm -n vault --create-namespace --values vault-values.yaml

# install Vault Secrets Operator
helm install vault-secrets-operator vault-secrets-operator -n vault-secrets-operator-system --create-namespace --values vault-operator-values.yaml

# configure Vault
kubectl exec --stdin=true --tty=true vault-0 -n vault -- /bin/sh

vault auth enable -path tor-auth-mount kubernetes 

vault write auth/tor-auth-mount/config kubernetes_host="https://$KUBERNETES_PORT_443_TCP_ADDR:443"

vault secrets enable -path=kvv2 kv-v2

vault policy write development - <<EOF
path "kvv2/*" {
	capabilities = ["read"]
}
EOF

vault write auth/tor-auth-mount/role/role1 \
   bound_service_account_names=default \
   bound_service_account_namespaces=release \
   policies=development \
   audience=vault \
   ttl=24h

vault kv put kvv2/ns/number-node-chain/domain/city/server/wg client.vpn="creds-in-base64"
vault kv put kvv2/ns/number-node-chain/domain/city/server/ovpn client.vpn="creds-in-base64"

# configure K8s
## install VaultAuth recource
kubectl apply -f vault-auth-static.yaml

## install VaultStaticSecret resource
kubectl apply -f static-secret.yaml

## install Secret recource (example)
kubectl apply -f static-secret-create.yaml



