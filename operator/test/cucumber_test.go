package test

import (
	"fmt"
	"github.com/cucumber/godog"
	"github.com/kyma-incubator/testdrape/godog/testing"
	gotesting "testing"
)

func createKCPCluster(t *testing.T) {
	fmt.Println("Creating KCP cluster")
}

func createSKRCluster(t *testing.T) {
	fmt.Println("Creating customer cluster")
}

func installReconciler(t *testing.T) {
	fmt.Printf("Install reconciler")
}

func createKymaCR(t *testing.T, centralMods, decentralMods int64, cluster string) {
	fmt.Printf("Create Kyma CR with %d centralised and %d dencentralized modules in cluster '%s'", centralMods, decentralMods, cluster)
}

func updateKymaCR(t *testing.T, centralMods, decentralMods int64, cluster string) {
	fmt.Printf("Updating Kyma CR by adding %d centralised and %d decentralized modules in cluster '%s'", centralMods, decentralMods, cluster)
}

func deleteKymaCR(t *testing.T, cluster string) {
	fmt.Printf("Delete Kyma CR in cluster '%s'", cluster)
}

func updateKymaCRInvalid(t *testing.T, cluster string) {
	fmt.Printf("Updating Kyma CR with invalid change in cluster '%s'", cluster)
}

func assertModuleCR(t *testing.T, expected int64, cluster string) {
	fmt.Printf("Assert %d module CRs in cluster '%s'", expected, cluster)
}

func assertModuleCRNotExist(t *testing.T, cluster string) {
	assertModuleCR(t, 0, cluster)
}

func assertManifestCR(t *testing.T, expected int64, cluster string) {
	fmt.Printf("Assert %d manifest CRs in cluster '%s'", expected, cluster)
}

func assertManifestCRNotExist(t *testing.T, cluster string) {
	assertManifestCR(t, 0, cluster)
}

func assertModuleDeployed(t *testing.T, expected int64, cluster string) {
	fmt.Printf("Assert %d manifest CRs in cluster '%s'", expected, cluster)
}

func assertModuleUndeployed(t *testing.T, cluster string) {
	assertModuleDeployed(t, 0, cluster)
}

func assertKymaCRState(t *testing.T, state string, timeout int64) {
	fmt.Printf("Assert Kyma CR is in '%s' state within '%d' sec", state, timeout)
}

func assertKymaCRConditionsUpdated(t *testing.T, cluster string) {
	fmt.Printf("Assert Kyma CR conditions were updated in cluster '%s'", cluster)
}

func assertKymaCRCopied(t *testing.T, fromCluster, toCluster string) {
	fmt.Printf("Assert Kyma CR copied from '%s' to '%s' cluster", fromCluster, toCluster)
}

func assertKymaCREvent(t *testing.T, severity string) {
	fmt.Printf("Assert Kyma CR contains event with severity '%s'", severity)
}

func assertValidatingWebhookLog(t *testing.T, severity string) {
	fmt.Printf("Assert validating Webhook logs '%s'", severity)
}

func initializeScenarios(s *godog.ScenarioContext) {
	//Pre-condition steps
	testing.NewContext(s).Register(`^KCP cluster created$`, createKCPCluster)
	testing.NewContext(s).Register(`^SKR cluster created$`, createSKRCluster)
	testing.NewContext(s).Register(`^Kyma reconciler installed in KCP cluster$`, installReconciler)
	testing.NewContext(s).Register(`^Kyma CR with (\d+) centralized modules? and (\d+) decentralized modules? created in (\w+) cluster$`, createKymaCR)
	testing.NewContext(s).Register(`^Kyma CR updated by setting (\d+) centralized modules? and (\d+) decentralized modules? in (\w+) cluster$`, updateKymaCR)
	testing.NewContext(s).Register(`^Kyma CR updated with invalid change in (\w+) cluster$`, updateKymaCRInvalid)
	testing.NewContext(s).Register(`^Kyma CR deleted in (\w+) cluster$`, deleteKymaCR)

	//Assertions
	testing.NewContext(s).Register(`^(\d+) d?e?centralized module CRs? created in (\w+) cluster$`, assertModuleCR)
	testing.NewContext(s).Register(`^(\d+) manifest CRs? created in (\w+) cluster$`, assertManifestCR)
	testing.NewContext(s).Register(`^(\d+) modules? deployed in (\w+) cluster$`, assertModuleDeployed)
	testing.NewContext(s).Register(`^module CRs? deleted in (\w+) cluster$`, assertModuleCRNotExist)
	testing.NewContext(s).Register(`^manifest CRs? deleted in (\w+) cluster$`, assertManifestCRNotExist)
	testing.NewContext(s).Register(`^modules? undeployed in (\w+) cluster$`, assertModuleUndeployed)
	testing.NewContext(s).Register(`^Kyma CR in state (\w+) within (\d+)sec$`, assertKymaCRState)
	testing.NewContext(s).Register(`^Kyma CR conditions updated in (\w+) cluster$`, assertKymaCRConditionsUpdated)
	testing.NewContext(s).Register(`^Kyma CR copied from (\w+) to (\w+) cluster$`, assertKymaCRCopied)
	testing.NewContext(s).Register(`^Kyma CR contains event with (\w+)`, assertKymaCREvent)
	testing.NewContext(s).Register(`^Validating webhook logs (\w+)`, assertValidatingWebhookLog)
}

func TestFeatures(t *gotesting.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: initializeScenarios,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Errorf("non-zero status returned, failed to run feature tests")
	}
}
