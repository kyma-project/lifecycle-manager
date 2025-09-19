package resolver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"time"
)

const (
	timeOnlyFormat = "15:04:05Z07:00"
	time24Hours    = 24 * time.Hour
)

var (
	ErrPolicyNotExists    = errors.New("maintenance policy doesn't exist")
	ErrUnknownOption      = errors.New("unknown option")
	ErrNoWindowInPolicies = errors.New("matched policies did not provide a window")
	ErrNoWindowFound      = errors.New("matches and defaults also failed to provide a window")
	ErrJSONUnmarshal      = errors.New("error during unmarshal")
)

type ResolvedWindow struct {
	Begin time.Time
	End   time.Time
}

func (rw ResolvedWindow) String() string {
	return fmt.Sprintf("<ResolvedWindow %s - %s>", rw.Begin, rw.End)
}

type MaintenanceWindowPolicy struct {
	Rules   []MaintenancePolicyRule `json:"rules"`
	Default MaintenanceWindow       `json:"default"`
}

// options.
type resolveOptions struct {
	time            time.Time
	ongoing         bool
	minDuration     time.Duration
	firstMatchOnly  bool
	fallbackDefault bool
}

// Specify the time to calculate with.
type TimeStamp time.Time

// Take ongoing windows into account.
type OngoingWindow bool

// If taking ongoing windows into account, minimum duration.
type MinWindowSize time.Duration

// Whether stop at first matched policy's windows.
type FirstMatchOnly bool

// If matched policies had no available windows whether to fall back
// to the default, or bail out with an error.
type FallbackDefault bool

/* GetMaintenancePolicy gets the maintenance window policy based on the policy name we specify
 * non-nil error returned if meeting one of below conditions:
 * - the speficied maintenance policy doesn't exist.
 * - error during unmarshal the policy data.
 */
func GetMaintenancePolicy(pool map[string]*[]byte, name string) (*MaintenanceWindowPolicy, error) {
	if name == "" {
		return nil, nil //nolint:nilnil //changing that now would break the API
	}

	extName := name + ".json"
	data, exist := pool[extName]
	if !exist {
		return nil, fmt.Errorf("%w: %s", ErrPolicyNotExists, name)
	}

	policy, err := NewMaintenanceWindowPolicyFromJSON(*data)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

/*
 * This function parse a JSON document from a byte array into a
 * MaintenanceWindowPolicy structure, and returns it. If any errors
 * are encountered, the error is returned, and the structured return data
 * is undefined.
 *
 * Once a MaintenanceWindowPolicy is returned its Resolve method can be used to find
 * A suitable maintenance window.
 */
func NewMaintenanceWindowPolicyFromJSON(raw []byte) (MaintenanceWindowPolicy, error) {
	var ruleset MaintenanceWindowPolicy

	err := json.Unmarshal(raw, &ruleset)
	if err != nil {
		return ruleset, fmt.Errorf("%w: %w", ErrJSONUnmarshal, err)
	}
	return ruleset, nil
}

/*
 * Finds the next applicatable maintenance window for a given runtime on the plan.
 *
 * The algorithm can be parameterized using the following typed varargs:
 *  - TimeStamp: A time.Time, to specify the resolving's time instead of now
 *  - OngoingWindow: A boolean, if true then already started windows are returned
 *    if long enough. Defaults to false.
 *  - MinWindowSize: A time.Duration, when OngoingWindow is true, this holds the
 *    minimum size for the windows. Defaults to 1h.
 *  - FirstMatchOnly: A boolean indicating wether or not to stop at the first
 *    matching rule in the ruleset before proceeding to the defaults.
 *    Defaults to true.
 *  - FallbackDefault: A boolean indicating whether or not fall back to the default
 *    rules if no specific matching rules are found. Defaults to true.
 *
 * If a match is found then a ResolvedWindow pointer is returned with a nil error.
 * Otherwise an error is returned and the ResolvedWindow pointer is expected to be
 * nil.
 */
func (mwp *MaintenanceWindowPolicy) Resolve(runtime *Runtime, opts ...interface{}) (*ResolvedWindow, error) {
	// first set up the internal logic parameters
	// defaults here
	options := resolveOptions{
		time:            time.Now(),
		ongoing:         false,
		minDuration:     time.Hour,
		firstMatchOnly:  true,
		fallbackDefault: true,
	}

	// overrides from typed varargs
	for idx, opt := range opts {
		switch val := opt.(type) {
		case TimeStamp:
			options.time = time.Time(val)
		case OngoingWindow:
			options.ongoing = bool(val)
		case MinWindowSize:
			options.minDuration = time.Duration(val)
		case FirstMatchOnly:
			options.firstMatchOnly = bool(val)
		case FallbackDefault:
			options.fallbackDefault = bool(val)
		default:
			return nil, fmt.Errorf("%w at %d: %s/%+v", ErrUnknownOption,
				idx, reflect.TypeOf(opt), opt)
		}
	}

	// first let's see whether any policies are having matching rules
	matched := false
	for _, policyrule := range mwp.Rules {
		if matched = policyrule.Match.Match(runtime); !matched {
			continue
		}
		// this policy is matching

		// we need to find the first window in the future
		window := policyrule.Windows.LookupAvailable(&options)
		if window != nil {
			return window, nil
		}

		// close but no cigar
		if options.firstMatchOnly {
			break
		}
	}

	// if we don't fall back to default if matches had no available
	// windows then we error out
	if matched && !options.fallbackDefault {
		return nil, ErrNoWindowInPolicies
	}

	// we do the default ruleset, if there are no matches
	if rw := mwp.Default.NextWindow(&options); rw != nil {
		return rw, nil
	}

	return nil, ErrNoWindowFound
}

type MaintenancePolicyRule struct {
	Match   MaintenancePolicyMatch `json:"match"`
	Windows MaintenanceWindows     `json:"windows"`
}
type MaintenanceWindows []MaintenanceWindow

func (mws *MaintenanceWindows) LookupAvailable(opts *resolveOptions) *ResolvedWindow {
	for _, mw := range *mws {
		if window := mw.NextWindow(opts); window != nil {
			return window
		}
	}
	return nil
}

type MaintenancePolicyMatch struct {
	GlobalAccountID Regexp `json:"globalAccountID,omitempty"` //nolint:tagliatelle,revive //changing that now would break the API
	Plan            Regexp `json:"plan,omitempty"`
	Region          Regexp `json:"region,omitempty"`
	PlatformRegion  Regexp `json:"platformRegion,omitempty"`
}

func (mpm MaintenancePolicyMatch) String() string {
	ret := "<MaintenancePolicyMatch"
	if mpm.GlobalAccountID.IsValid() {
		ret += fmt.Sprintf(" GlobalAccountID:'%s'", mpm.GlobalAccountID)
	}
	if mpm.Plan.IsValid() {
		ret += fmt.Sprintf(" Plan:'%s'", mpm.Plan)
	}
	if mpm.Region.IsValid() {
		ret += fmt.Sprintf(" Region:'%s'", mpm.Region)
	}
	if mpm.PlatformRegion.IsValid() {
		ret += fmt.Sprintf(" PlatformRegion:'%s'", mpm.PlatformRegion)
	}
	return ret + ">"
}

func (mpm MaintenancePolicyMatch) Match(runtime *Runtime) bool {
	// programmer is running with -fno-unroll-loops
	for _, field := range []string{
		"GlobalAccountID", "Plan",
		"Region", "PlatformRegion",
	} {
		rexp := reflect.Indirect(reflect.ValueOf(mpm)).FieldByName(field).Interface().(Regexp) //nolint:forcetypeassert,revive //we know it's a Regexp
		if !rexp.IsValid() {
			continue
		}
		value := reflect.Indirect(reflect.ValueOf(runtime)).FieldByName(field).String()
		if len(value) > 0 && rexp.MatchString(value) {
			return true
		}
	}
	return false
}

type Regexp struct {
	Str    string
	Regexp *regexp.Regexp
}

func NewRegexp(pattern string) Regexp {
	return Regexp{
		Str:    pattern,
		Regexp: regexp.MustCompile(pattern),
	}
}

func (r *Regexp) UnmarshalJSON(data []byte) error {
	r.Str = string(bytes.Trim(data, `"`))
	if len(r.Str) == 0 {
		r.Regexp = nil
		return nil
	}
	var err error
	r.Regexp, err = regexp.Compile(r.Str)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrJSONUnmarshal, err)
	}
	return nil
}

func (r *Regexp) MatchString(s string) bool {
	return r.Regexp.MatchString(s)
}

func (r *Regexp) IsValid() bool {
	return r.Regexp != nil
}

func (r *Regexp) String() string {
	return r.Str
}

/*
If days is empty, then begin and end are ISO8601 strings with
exact times, otherwise if days is specified it's a time-only (with timezone).
*/
type MaintenanceWindow struct {
	Days  []string   `json:"days"`
	Begin WindowTime `json:"begin"`
	End   WindowTime `json:"end"`
}

// this has two main modes: whether we have days or not.
func (mw *MaintenanceWindow) NextWindow(opts *resolveOptions) *ResolvedWindow {
	if len(mw.Days) == 0 {
		// in this case begin and end are absolute units
		if rw := windowWithin(opts, mw.Begin.T(), mw.End.T()); rw != nil {
			return rw
		}
	} else {
		// right here begin and end are simply times within the duration of a day
		// logic is, we construct today's begin and end timestamps with the supplied
		// time, and we keep on stepping it day by day until we hit one of the windows
		begin := time.Date(opts.time.Year(), opts.time.Month(), opts.time.Day(),
			mw.Begin.T().Hour(), mw.Begin.T().Minute(), mw.Begin.T().Second(),
			0, mw.Begin.T().Location())
		end := time.Date(opts.time.Year(), opts.time.Month(), opts.time.Day(),
			mw.End.T().Hour(), mw.End.T().Minute(), mw.End.T().Second(),
			0, mw.End.T().Location())

		// next day diff
		incr := time24Hours

		// if it goes through midnight
		if end.Before(begin) || end.Equal(begin) {
			end = end.Add(incr)
		}

		// now get the next suitable
		// days are weekdays, and there's a total of 7 of them, so iterating ahead
		// of that would be getting the next cycle, so we stop at a week's lookahead
		for range 8 {
			day3 := begin.Weekday().String()[0:3]
			// if this day is not available, then next
			if !slices.Contains(mw.Days, day3) {
				begin = begin.Add(incr)
				end = end.Add(incr)
				continue
			}
			if rw := windowWithin(opts, begin, end); rw != nil {
				return rw
			}
			begin = begin.Add(incr)
			end = end.Add(incr)
		}
	}
	return nil
}

// type alias for (un)marshalling.
type WindowTime time.Time

func (wt *WindowTime) UnmarshalJSON(data []byte) error {
	trimmed := string(bytes.Trim(data, `"`))

	// try the fullformat first
	tParsed, err := time.Parse(time.RFC3339, trimmed)
	if err == nil {
		*wt = WindowTime(tParsed)
		return nil
	}

	// now try the time-only format
	tParsed, err = time.Parse(timeOnlyFormat, trimmed)
	if err == nil {
		*wt = WindowTime(tParsed)
		return nil
	}

	return &json.UnsupportedValueError{
		Value: reflect.ValueOf(trimmed),
		Str: fmt.Sprintf("Unable to parse value \"%s\" as ISO8601 or timeonly-with-tz",
			trimmed),
	}
}

func (wt *WindowTime) T() time.Time {
	return time.Time(*wt)
}

// utility functions.
func windowWithin(opts *resolveOptions, begin time.Time, end time.Time) *ResolvedWindow {
	if !opts.ongoing {
		// simple, just verify whether the begin is in the future
		if opts.time.Before(begin) {
			return &ResolvedWindow{
				Begin: begin,
				End:   end,
			}
		}
	} else {
		// in this case we need to verify that the end window is in the future
		// AND is at least minDuration ahead of us
		if opts.time.Add(opts.minDuration).Before(end) {
			return &ResolvedWindow{
				Begin: begin,
				End:   end,
			}
		}
	}
	return nil
}
