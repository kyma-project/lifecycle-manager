```shell
component-cli component-archive create ./example --component-name "kyma-project.io/module/example" --component-version "v0.0.1"
component-cli ca resources add ./example ./resources.yaml
```

```
# sudo vim /etc/hosts
# ----------------
# THIS IS IMPORTANT!
127.0.0.1 operator-test-registry.localhost
# -----------------
# With this, test if your registry is working
# docker push operator-test-registry.localhost:50241/nginx:latest 
```

```shell
component-cli ca remote push example --repo-ctx operator-test-registry.localhost:50241
```

```shell
openssl genpkey -algorithm RSA -out ./private-key.pem
openssl rsa -in ./private-key.pem -pubout > public-key.pem
component-cli ca signatures sign rsa operator-test-registry.localhost:50241 kyma-project.io/module/example v0.0.1 --upload-base-url operator-test-registry.localhost:50241/signed --recursive --signature-name test-signature --private-key ./private-key.pem
component-cli ca signatures verify rsa operator-test-registry.localhost:50241/signed kyma-project.io/module/example v0.0.1 --signature-name test-signature --public-key ./public-key.pem
```

```shell
component-cli ca remote get operator-test-registry.localhost:50241 kyma-project.io/module/example v0.0.1 >> remote-component-descriptor.yaml
component-cli ca remote get operator-test-registry.localhost:50241/signed kyma-project.io/module/example v0.0.1 >> remote-component-descriptor-signed.yaml
```

```shell
sh generate-module-template.sh
```