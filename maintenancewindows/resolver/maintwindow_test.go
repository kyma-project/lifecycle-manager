package resolver_test

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/kyma-project/lifecycle-manager/maintenancewindows/resolver"
)

const testfile = "testdata/ruleset-1.json"

type testData struct {
	runtime  resolver.Runtime
	expected bool
}

func createRuntime(gaid string, plan string, region string,
	platformregion string,
) resolver.Runtime {
	return resolver.Runtime{
		GlobalAccountID: gaid,
		Plan:            plan,
		Region:          region,
		PlatformRegion:  platformregion,
	}
}

func resWin(begin string, end string) resolver.ResolvedWindow {
	bTime, err := time.Parse(time.RFC3339, begin)
	if err != nil {
		panic(err.Error())
	}
	eTime, err := time.Parse(time.RFC3339, end)
	if err != nil {
		panic(err.Error())
	}
	return resolver.ResolvedWindow{
		Begin: bTime,
		End:   eTime,
	}
}

func at(timestamp string) resolver.TimeStamp {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		panic(err.Error())
	}
	return resolver.TimeStamp(t)
}

type testCase struct {
	name     string
	runtime  resolver.Runtime
	options  []interface{}
	errors   bool
	expected resolver.ResolvedWindow
}

func (tc testCase) Message() string {
	opts := []string{}
	for idx, opt := range tc.options {
		switch val := opt.(type) {
		case resolver.TimeStamp:
			t := time.Time(val)
			opts = append(opts, fmt.Sprintf("At:%s(%s)", t.String(), t.Weekday()))
		case resolver.OngoingWindow:
			opts = append(opts, fmt.Sprintf("Ongoing:%v", bool(val)))
		case resolver.MinWindowSize:
			opts = append(opts, fmt.Sprintf("MinDuration:%v", time.Duration(val)))
		case resolver.FirstMatchOnly:
			opts = append(opts, fmt.Sprintf("FirstMatchOnly:%v", bool(val)))
		case resolver.FallbackDefault:
			opts = append(opts, fmt.Sprintf("FallbackDefault:%v", bool(val)))
			// forward something we can test interface error handling
		case string:
			opts = append(opts, val)
		default:
			panic(fmt.Sprintf("Unknown option at %d: %s/%+v",
				idx, reflect.TypeOf(opt), opt))
		}
	}
	return fmt.Sprintf("testCase:\n - Name(%s)\n - Opts(%s)\n - Runtime(GAID:%s Plan:%s Region:%s PlatformRegion:%s)",
		tc.name, strings.Join(opts, " "),
		tc.runtime.GlobalAccountID, tc.runtime.Plan, tc.runtime.Region,
		tc.runtime.PlatformRegion)
}

type MaintWindowSuite struct {
	suite.Suite

	plan      resolver.MaintenanceWindowPolicy
	testCases []testCase
}

func (suite *MaintWindowSuite) SetupSuite() {
	// load the testing ruleset
	rawdata, err := os.ReadFile(testfile)
	suite.Require().NoErrorf(err, "Unable to read testdata from %s", testfile)
	suite.Require().NotNil(rawdata)

	suite.plan, err = resolver.NewMaintenanceWindowPolicyFromJSON(rawdata)
	suite.Require().NoError(err)

	// specify the testcases
	suite.testCases = []testCase{
		{
			name:     "freetrials next",
			runtime:  createRuntime("", "trial", "", ""),
			options:  []interface{}{at("2024-10-03T05:05:00Z")},
			errors:   false,
			expected: resWin("2024-10-04T01:00:00Z", "2024-10-05T01:00:00Z"),
		},
		{
			name:    "ongoing",
			runtime: createRuntime("", "", "uksouth-vikings", ""),
			options: []interface{}{
				at("2024-10-10T22:05:00Z"),
				resolver.OngoingWindow(true),
			},
			errors:   false,
			expected: resWin("2024-10-10T20:00:00Z", "2024-10-11T00:00:00Z"),
		},
		{
			name:    "ongoing+minsize",
			runtime: createRuntime("", "", "uksouth-vikings", ""),
			options: []interface{}{
				at("2024-10-10T22:05:00Z"),
				resolver.OngoingWindow(true),
				resolver.MinWindowSize(5 * time.Hour),
			},
			errors:   false,
			expected: resWin("2024-12-08T20:00:00Z", "2024-12-09T00:00:00Z"),
		},
		{
			name:    "not just first match",
			runtime: createRuntime("", "", "uksouth-vikings", ""),
			options: []interface{}{
				at("2024-12-10T22:05:00Z"),
				resolver.FirstMatchOnly(false),
			},
			errors:   false,
			expected: resWin("2024-12-18T20:00:00Z", "2024-12-19T00:00:00Z"),
		},
		{
			name:    "first match fail -> default",
			runtime: createRuntime("", "", "uksouth-vikings", ""),
			options: []interface{}{
				at("2024-12-10T22:05:00Z"),
				resolver.FirstMatchOnly(true),
			},
			errors:   false,
			expected: resWin("2024-12-14T00:00:00Z", "2024-12-15T00:00:00Z"),
		},
		{
			name:    "first match fail -> nodefault",
			runtime: createRuntime("", "", "uksouth-vikings", ""),
			options: []interface{}{
				at("2024-12-10T22:05:00Z"),
				resolver.FirstMatchOnly(true), resolver.FallbackDefault(false),
			},
			errors:   true,
			expected: resWin("2024-12-14T00:00:00Z", "2024-12-15T00:00:00Z"),
		},
		{
			name:    "wrong arg",
			runtime: createRuntime("", "", "uksouth-vikings", ""),
			options: []interface{}{
				at("2024-12-10T22:05:00Z"),
				resolver.FirstMatchOnly(true), resolver.FallbackDefault(false),
				"lol",
			},
			errors:   true,
			expected: resWin("2042-12-14T00:00:00Z", "2024-12-15T00:00:00Z"),
		},
	}
}

func (suite *MaintWindowSuite) Test_Match_Plans() {
	testdata := []testData{
		{
			runtime:  createRuntime("", "free", "", ""),
			expected: true,
		},
		{
			runtime:  createRuntime("", "trial", "", ""),
			expected: true,
		},
		{
			runtime:  createRuntime("", "azure_lite", "", ""),
			expected: false,
		},
	}

	matcher := suite.plan.Rules[0].Match
	for _, subject := range testdata {
		suite.Require().Equal(subject.expected, matcher.Match(&subject.runtime))
	}
}

func (suite *MaintWindowSuite) Test_Match_Plan() {
	testdata := []testData{
		{
			runtime:  createRuntime("", "free", "", ""),
			expected: true,
		},
		{
			runtime:  createRuntime("", "trial", "", ""),
			expected: true,
		},
		{
			runtime:  createRuntime("", "azure_lite", "", ""),
			expected: false,
		},
	}

	matcher := suite.plan.Rules[0].Match
	for _, subject := range testdata {
		suite.Require().Equal(subject.expected, matcher.Match(&subject.runtime))
	}
}

func (suite *MaintWindowSuite) Test_Match_Region() {
	testdata := []testData{
		{
			runtime:  createRuntime("", "", "eu-balkan-1", ""),
			expected: true,
		},
		{
			runtime:  createRuntime("", "", "uksouth-teaparty", ""),
			expected: true,
		},
		{
			runtime:  createRuntime("", "", "us-cottoneyejoe", ""),
			expected: false,
		},
	}

	matcher := suite.plan.Rules[1].Match
	for _, subject := range testdata {
		suite.Require().Equal(subject.expected, matcher.Match(&subject.runtime))
	}
}

func (suite *MaintWindowSuite) Test_Match_GAID() {
	testdata := []testData{
		{
			runtime:  createRuntime("sup-er-ga-case", "", "", ""),
			expected: true,
		},
		{
			runtime:  createRuntime("not-matching", "", "", ""),
			expected: false,
		},
	}

	matcher := suite.plan.Rules[2].Match
	for _, subject := range testdata {
		suite.Require().Equal(subject.expected, matcher.Match(&subject.runtime))
	}
}

func (suite *MaintWindowSuite) Test_Match_PlatformRegion() {
	testdata := []testData{
		{
			runtime:  createRuntime("", "", "uksouth-teaparty", "super-mario-bros"),
			expected: true,
		},
		{
			runtime:  createRuntime("", "", "us-cottoneyejoe", "luigi"),
			expected: false,
		},
	}

	matcher := suite.plan.Rules[2].Match
	for _, subject := range testdata {
		suite.Require().Equal(subject.expected, matcher.Match(&subject.runtime))
	}
}

func (suite *MaintWindowSuite) Test_Match_TestCases() {
	/*
		runtime resolver.Runtime
		errors bool
		at time.Time
		expected resolver.ResolvedWindow
	*/
	for _, tcase := range suite.testCases {
		result, err := suite.plan.Resolve(&tcase.runtime, tcase.options...)
		if tcase.errors {
			suite.Require().Errorf(err, "test:\n%s\nresult:\n%v\n", tcase.Message(), result)
			suite.Require().Nil(result, tcase.Message())
		} else {
			suite.Require().NoError(err, tcase.Message())
			suite.Require().NotNil(result, tcase.Message())
		}
		if result != nil && err == nil {
			suite.Require().Equal(tcase.expected.String(), result.String(), tcase.Message())
		}
	}
}

func Test_RunMaintWindowSuite(t *testing.T) {
	suite.Run(t, new(MaintWindowSuite))
}

func Test_MPMString(t *testing.T) {
	gaid := "blah1"
	plan := "blah2"
	reg := "blah3"
	preg := "blah4"
	data := resolver.MaintenancePolicyMatch{
		GlobalAccountID: resolver.NewRegexp(gaid),
		Plan:            resolver.NewRegexp(plan),
		Region:          resolver.NewRegexp(reg),
		PlatformRegion:  resolver.NewRegexp(preg),
	}
	expected := fmt.Sprintf(
		"<MaintenancePolicyMatch GlobalAccountID:'%s' Plan:'%s' Region:'%s' PlatformRegion:'%s'>",
		gaid,
		plan,
		reg,
		preg,
	)
	require.Equal(t, expected, data.String())
}
