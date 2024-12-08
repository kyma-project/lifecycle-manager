# Overview of All Scripts

## Version Checker: `version.sh`
Checks if the Command Line Tools for `kubectl`, `docker`, `GoLang`, `k3d` and `istioctl` have the correct versions.
It ensures that the versions are in valid format using [Semantic Versioning](https://semver.org/).
In case of outdated versions, it simply gives a warning and exits with success. Have a look below to know more about the return codes.

#### Current Versions:
| CLI Tool  | Version |
| --------- | ------- |
| `kubectl` | v1.31.3 |
| `go`      | v1.23.3 |
| `k3d`     | v5.6.0  |
| `docker`  | v27.3.1 |
| `istioctl`| v1.24.1 |

#### Returns:
`0` -> Success<br>
`1` -> If the aforementioned tools aren't installed<br>
`2` -> Invalid version found, i.e., not conforming to Semantic Versioning schema<br>
