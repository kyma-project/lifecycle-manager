#!/usr/bin/env bash

COMMIT_BASE="a180040ef2e6b8742a61ccc44eaaabae1dded8f5"

FILE=".github/actions/deploy-template-operator/action.yaml"
FILE=".github/workflows/test-e2e.yaml"
FILE="api-version-compatibility-config.yaml"
FILE="api/shared/channel.go"
FILE="api/v1beta2/kyma_types.go"
FILE="api/v1beta2/moduletemplate_types.go"
FILE="api/v1beta2/moduletemplate_types_test.go"
FILE="api/v1beta2/zz_generated.deepcopy.go"
FILE="config/crd/bases/operator.kyma-project.io_moduletemplates.yaml"
FILE="internal/controller/kyma/controller.go"
FILE="internal/descriptor/cache/key.go"
FILE="internal/descriptor/provider/provider_test.go"
#TODO-ONLY_NEW!A	internal/manifest/deployment_ready_check.go
#TODO-ONLY_NEW!A	internal/manifest/deployment_ready_check_test.go
#TODO-ONLY_NEW!A	internal/manifest/statefulset_ready_check.go
#TODO-ONLY_NEW!A	internal/manifest/statefulset_ready_check_test.go
FILE="internal/pkg/metrics/kyma.go"
FILE="internal/remote/skr_context.go"
FILE="internal/remote/skr_context_test.go"
#TODO-ONLY_OLD!M	pkg/module/parse/template_to_module.go
#BOTH_DELETED!FILE="pkg/module/sync/errors.go"
FILE="pkg/templatelookup/availableModules.go"
FILE="pkg/templatelookup/availableModules_test.go"
#TODO!FILE="pkg/templatelookup/regular.go"
FILE="pkg/templatelookup/regular_test.go"
FILE="pkg/testutils/builder/moduletemplate.go"
FILE="pkg/testutils/moduletemplate.go"
FILE="pkg/testutils/utils.go"
FILE="tests/e2e/Makefile"
FILE="tests/e2e/module_install_by_version_test.go"
FILE="tests/e2e/module_status_decoupling_test.go"
FILE="tests/integration/apiwebhook/moduletemplate_crd_validation_test.go"
FILE="tests/integration/apiwebhook/ocm_test.go"
FILE="tests/integration/controller/kcp/helper_test.go"
FILE="tests/integration/controller/kcp/remote_sync_test.go"
FILE="tests/integration/controller/kyma/helper_test.go"
FILE="tests/integration/controller/kyma/kyma_module_channel_test.go"
FILE="tests/integration/controller/kyma/kyma_module_enable_test.go"
FILE="tests/integration/controller/kyma/kyma_module_version_test.go"
FILE="tests/integration/controller/kyma/kyma_test.go"
#TODO!FILE="tests/integration/controller/kyma/manifest_test.go"
FILE="tests/integration/controller/kyma/moduletemplate_install_test.go"
FILE="tests/integration/controller/kyma/moduletemplate_test.go"
FILE="tests/integration/controller/mandatorymodule/deletion/controller_test.go"
FILE="tests/integration/controller/mandatorymodule/installation/controller_test.go"
FILE="tests/integration/controller/moduletemplate/moduletemplate_test.go"
FILE="tests/integration/controller/moduletemplate/suite_test.go"
FILE="tests/moduletemplates/moduletemplate_template_operator_v2_direct_version.yaml"
#TODO!FILE="unit-test-coverage.yaml"

#2nd round:
FILE="api/v1beta2/kyma_types.go"
FILE="internal/controller/kyma/controller.go"
git diff $COMMIT_BASE  feat/module-catalogue-improvements           $FILE > old.diff
git diff main                                                       $FILE > new.diff

#NOTES
# internal/remote/skr_context_test.go: in commit 98811dee24447a000f82a0e1ed401db62e004b8d the "testCase" variable is modified -> possible bug
#
#
#
#
