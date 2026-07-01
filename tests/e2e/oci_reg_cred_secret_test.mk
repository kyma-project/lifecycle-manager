.DEFAULT_GOAL := test
.PHONY: test $(MAKECMDGOALS)

include $(dir $(abspath $(lastword $(MAKEFILE_LIST))))e2e.common.mk

.PHONY: klm-patch
klm-patch:
	@echo "::group::KLM patch - OCI registry credential secret"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(LIFECYCLE_MANAGER_DIR)/config/watcher_local_test > /dev/null
	printf '%s\n' \
		'- op: add' \
		'  path: /spec/template/spec/containers/0/args/-' \
		'  value: --oci-registry-cred-secret=private-oci-reg-creds' \
		>> oci_registry_cred_secret.yaml
	kustomize edit add patch --path oci_registry_cred_secret.yaml --kind Deployment
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: module-setup
module-setup:
	@echo "::group::Setup private OCI registry"
	@$(SCRIPTS_DIR)/setup_private_registry.sh
	@echo "::endgroup::"
	@echo "::group::Build and push module to private registry"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(TEMPLATE_OPERATOR_DIR) > /dev/null
	cp $(SCRIPTS_DIR)/ocm-config-private-registry.yaml .
	modulectl create --config-file ./module-config.yaml \
		--disable-ocm-registry-push \
		--output-constructor-file ./component-constructor.yaml
	ocm --config ./ocm-config-private-registry.yaml add componentversions \
		--create --file ./component-ctf --skip-digest-generation ./component-constructor.yaml
	ocm --config ./ocm-config-private-registry.yaml transfer ctf \
		--overwrite --no-update ./component-ctf http://k3d-private-oci-reg.localhost:5001
	kubectl apply -f <(yq eval '.metadata.namespace = "kcp-system"' ./template.yaml)
	$(SCRIPTS_DIR)/deploy_modulereleasemeta.sh $(MODULE_NAME) regular:$(MODULE_DEPLOYABLE_VERSION)
	@popd > /dev/null
	@echo "::endgroup::"

.PHONY: test-run
test-run: log-tool-versions
	@echo "::group::Setting kubeconfig variables"
	@export KCP_KUBECONFIG=$(shell k3d kubeconfig write kcp)
	@export SKR_KUBECONFIG=$(shell k3d kubeconfig write skr)
	@echo "::endgroup::"

	@echo "::group::E2E test: OCI Registry Credentials Secret"
	@export PATH=$(LOCALBIN):$$PATH
	@pushd $(E2E_TESTS_DIR) > /dev/null
	set +e; $(GO) test -timeout 20m -ginkgo.v -ginkgo.focus "OCI Registry Credentials Secret"; status=$$?; set -e
	@popd > /dev/null
	@echo "::endgroup::"
	exit $${status}

.PHONY: test
test: create-clusters klm-patch deploy-klm module-setup test-run
