package bamboo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/r0jjames/bam-cli/internal/errs"
	"github.com/r0jjames/bam-cli/internal/provider"
)

// MaskedValue is what Bamboo returns in place of a secret variable's value.
const MaskedValue = "********"

type varDTO struct {
	Name  string `json:"name"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (v varDTO) name() string {
	if v.Name != "" {
		return v.Name
	}
	return v.Key
}

// decodeVariables accepts a bare list, or an object holding the list under
// "variables" or "variable", bare or inside a collection envelope.
func decodeVariables(body []byte) ([]varDTO, bool) {
	if items, ok := decodeVarList(body); ok {
		return items, true
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(body, &obj) != nil {
		return nil, false
	}
	for _, k := range []string{"variables", "variable"} {
		if raw, ok := obj[k]; ok {
			if items, ok := decodeVarList(raw); ok {
				return items, true
			}
		}
	}
	return nil, false
}

func decodeVarList(raw json.RawMessage) ([]varDTO, bool) {
	var items []varDTO
	if len(raw) > 0 && raw[0] == '[' && json.Unmarshal(raw, &items) == nil {
		return items, true
	}
	var env envelope
	if len(raw) > 0 && raw[0] == '{' && json.Unmarshal(raw, &env) == nil && env.Items != nil {
		if json.Unmarshal(env.Items, &items) == nil {
			return items, true
		}
	}
	return nil, false
}

// ListVariables returns a plan's declared variables. Bamboo versions differ in
// the path; the working one is learned and cached.
func (c *Client) ListVariables(ctx context.Context, key string) ([]provider.Variable, error) {
	caps := c.fresh(ctx)
	unsupported := errs.Bamboof("plan variables cannot be listed on %s", c.base.Host).Wrap(errs.ErrUnsupported)
	var suffixes []string
	switch caps.PlanVars {
	case "none":
		return nil, unsupported
	case "variables", "variable":
		suffixes = []string{caps.PlanVars}
	default:
		suffixes = []string{"variables", "variable"}
	}
	for _, s := range suffixes {
		body, err := c.do(ctx, request{method: http.MethodGet, path: api + "/plan/" + url.PathEscape(key) + "/" + s})
		if err != nil {
			if isUnsupportedStatus(err) {
				continue
			}
			return nil, err
		}
		items, ok := decodeVariables(body)
		if !ok {
			continue
		}
		suffix := s
		c.learn(ctx, func(cp *Capabilities) { cp.PlanVars = suffix })
		out := make([]provider.Variable, 0, len(items))
		for _, v := range items {
			out = append(out, provider.Variable{Name: v.name(), Value: v.Value, Masked: v.Value == MaskedValue})
		}
		return out, nil
	}
	// Both paths failed: tell a missing plan apart from a missing feature.
	if _, err := c.GetPlan(ctx, key); err != nil {
		return nil, err
	}
	c.learn(ctx, func(cp *Capabilities) { cp.PlanVars = "none" })
	return nil, unsupported
}

// BuildVariables returns the variables a finished build ran with.
func (c *Client) BuildVariables(ctx context.Context, key string) (map[string]string, error) {
	caps := c.fresh(ctx)
	unsupported := errs.Bamboof("build variables cannot be read back on %s", c.base.Host).Wrap(errs.ErrUnsupported)
	if caps.BuildVars == "no" {
		return nil, unsupported
	}
	var obj map[string]json.RawMessage
	if err := c.getJSON(ctx, api+"/result/"+url.PathEscape(key), url.Values{"expand": {"variables"}}, &obj); err != nil {
		return nil, c.notFoundAs(err, "build", key, "")
	}
	raw, present := obj["variables"]
	items, ok := decodeVarList(raw)
	if !present || !ok {
		c.learn(ctx, func(cp *Capabilities) { cp.BuildVars = "no" })
		return nil, unsupported
	}
	if caps.BuildVars == "" {
		c.learn(ctx, func(cp *Capabilities) { cp.BuildVars = "yes" })
	}
	out := make(map[string]string, len(items))
	for _, v := range items {
		out[v.name()] = v.Value
	}
	return out, nil
}

type testDTO struct {
	TestCaseName string `json:"testCaseName"`
	ClassName    string `json:"className"`
	MethodName   string `json:"methodName"`
}

// failedTests returns failed test names of a build, or nil when unavailable.
func (c *Client) failedTests(ctx context.Context, key string) []string {
	caps := c.fresh(ctx)
	if caps.FailedTests == "no" {
		return nil
	}
	var r struct {
		TestResults *struct {
			FailedTests *struct {
				TestResult []testDTO `json:"testResult"`
			} `json:"failedTests"`
		} `json:"testResults"`
	}
	if err := c.getJSON(ctx, api+"/result/"+url.PathEscape(key), url.Values{"expand": {"testResults.failedTests.testResult"}}, &r); err != nil {
		return nil
	}
	if r.TestResults == nil {
		c.learn(ctx, func(cp *Capabilities) { cp.FailedTests = "no" })
		return nil
	}
	if caps.FailedTests == "" {
		c.learn(ctx, func(cp *Capabilities) { cp.FailedTests = "yes" })
	}
	if r.TestResults.FailedTests == nil {
		return nil
	}
	var names []string
	for _, t := range r.TestResults.FailedTests.TestResult {
		if t.ClassName != "" && t.MethodName != "" {
			names = append(names, t.ClassName+"."+t.MethodName)
		} else {
			names = append(names, t.TestCaseName)
		}
	}
	return names
}
