## Running the module creation

```shell
sh generate-module-template.sh
```

## FAQ

### I get an error because of my hosts file! What do I do?
In this case you will have to manually edit your hosts file and edit your entry for your registry
```
# sudo vim /etc/hosts
# ----------------
# THIS IS IMPORTANT!
127.0.0.1 operator-test-registry.localhost
# -----------------
# With this, test if your registry is working
# docker push operator-test-registry.localhost:50241/nginx:latest 
```