// Command record captures Bamboo REST responses from a personal server,
// scrubs them, and writes them to internal/provider/bamboo/testdata/recorded.
//
//	BAM_RECORD_URL      server URL, e.g. http://bamboo.lab.example:8085
//	BAM_RECORD_TOKEN    personal access token
//	BAM_RECORD_PLAN     a plan key with builds, e.g. LAB-PROV
//	BAM_RECORD_FAILED   optional: a failed build key of that plan, e.g. LAB-PROV-12
//	BAM_RECORD_TRIGGER  optional: "1" to trigger a build, stop it, and record both
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/r0jjames/bam-cli/internal/provider/bamboo"
)

type recorder struct {
	base, token, out string
	http             *http.Client
	scrub            bamboo.Scrubber
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "record:", err)
		os.Exit(1)
	}
}

func run() error {
	base := strings.TrimRight(os.Getenv("BAM_RECORD_URL"), "/")
	token := os.Getenv("BAM_RECORD_TOKEN")
	plan := os.Getenv("BAM_RECORD_PLAN")
	if base == "" || token == "" || plan == "" {
		return errors.New("set BAM_RECORD_URL, BAM_RECORD_TOKEN and BAM_RECORD_PLAN")
	}
	u, err := url.Parse(base)
	if err != nil {
		return err
	}
	r := &recorder{base: base, token: token, http: &http.Client{Timeout: 30 * time.Second},
		out: filepath.Join("internal", "provider", "bamboo", "testdata", "recorded")}
	if err := os.MkdirAll(r.out, 0o755); err != nil {
		return err
	}

	var me struct {
		Name     string `json:"name"`
		FullName string `json:"fullName"`
	}
	body, status, err := r.call(http.MethodGet, "/rest/api/latest/currentUser", nil)
	if err != nil || status != 200 {
		return fmt.Errorf("currentUser: status %d: %v", status, err)
	}
	_ = json.Unmarshal(body, &me)
	r.scrub = bamboo.Scrubber{Host: u.Host, Users: []string{me.FullName, me.Name}}

	project := strings.SplitN(plan, "-", 2)[0]
	latest, err := r.latestBuild(plan)
	if err != nil {
		return err
	}
	steps := []struct {
		file, path string
		query      url.Values
	}{
		{"currentUser.json", "/rest/api/latest/currentUser", nil},
		{"info.json", "/rest/api/latest/info", nil},
		{"projects.json", "/rest/api/latest/project", url.Values{"max-result": {"25"}}},
		{"project_plans.json", "/rest/api/latest/project/" + project, url.Values{"expand": {"plans.plan"}}},
		{"plan.json", "/rest/api/latest/plan/" + plan, nil},
		{"plan_variables.json", "/rest/api/latest/plan/" + plan + "/variables", nil},
		{"plan_variable.json", "/rest/api/latest/plan/" + plan + "/variable", nil},
		{"branches.json", "/rest/api/latest/plan/" + plan + "/branch", url.Values{"max-result": {"25"}}},
		{"results.json", "/rest/api/latest/result/" + plan, url.Values{"expand": {"results.result"}, "max-result": {"5"}}},
		{"result_detail.json", "/rest/api/latest/result/" + latest, url.Values{"expand": {"stages.stage.results.result,labels,vcsRevisions"}}},
		{"result_variables.json", "/rest/api/latest/result/" + latest, url.Values{"expand": {"variables"}}},
	}
	for _, s := range steps {
		if err := r.record(s.file, http.MethodGet, s.path, s.query); err != nil {
			return err
		}
	}

	if failed := os.Getenv("BAM_RECORD_FAILED"); failed != "" {
		if err := r.recordFailed(failed); err != nil {
			return err
		}
	}
	if os.Getenv("BAM_RECORD_TRIGGER") == "1" {
		if err := r.recordTriggerAndStop(plan); err != nil {
			return err
		}
	}
	fmt.Println("recorded into", r.out, "- review every file before committing")
	return nil
}

func (r *recorder) call(method, path string, q url.Values) ([]byte, int, error) {
	full := r.base + path
	if len(q) > 0 {
		full += "?" + q.Encode()
	}
	req, err := http.NewRequest(method, full, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
	if !strings.HasPrefix(path, "/download/") {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := r.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// record writes the scrubbed body to file, or file+".status" for non-2xx.
func (r *recorder) record(file, method, path string, q url.Values) error {
	body, status, err := r.call(method, path, q)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	if status < 200 || status > 299 {
		snippet := r.scrub.Scrub(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		fmt.Printf("%-28s %d (recorded as .status)\n", file, status)
		return os.WriteFile(filepath.Join(r.out, file+".status"), []byte(fmt.Sprintf("%d\n%s\n", status, snippet)), 0o644)
	}
	fmt.Printf("%-28s %d\n", file, status)
	if strings.HasSuffix(file, ".json") {
		var pretty any
		if json.Unmarshal(body, &pretty) == nil {
			body, _ = json.MarshalIndent(pretty, "", "  ")
		}
	}
	return os.WriteFile(filepath.Join(r.out, file), r.scrub.Scrub(body), 0o644)
}

func (r *recorder) latestBuild(plan string) (string, error) {
	body, status, err := r.call(http.MethodGet, "/rest/api/latest/result/"+plan, url.Values{"max-result": {"1"}})
	if err != nil || status != 200 {
		return "", fmt.Errorf("latest build of %s: status %d: %v", plan, status, err)
	}
	var res struct {
		Results struct {
			Result []struct {
				Key string `json:"key"`
			} `json:"result"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &res); err != nil || len(res.Results.Result) == 0 {
		return "", fmt.Errorf("plan %s has no builds; run it once first", plan)
	}
	return res.Results.Result[0].Key, nil
}

func (r *recorder) recordFailed(buildKey string) error {
	if err := r.record("failed_detail.json", http.MethodGet, "/rest/api/latest/result/"+buildKey,
		url.Values{"expand": {"stages.stage.results.result"}}); err != nil {
		return err
	}
	if err := r.record("failed_tests.json", http.MethodGet, "/rest/api/latest/result/"+buildKey,
		url.Values{"expand": {"testResults.failedTests.testResult"}}); err != nil {
		return err
	}
	body, _, err := r.call(http.MethodGet, "/rest/api/latest/result/"+buildKey, url.Values{"expand": {"stages.stage.results.result"}})
	if err != nil {
		return err
	}
	var res struct {
		Stages struct {
			Stage []struct {
				Results struct {
					Result []struct {
						Key   string `json:"buildResultKey"`
						State string `json:"state"`
					} `json:"result"`
				} `json:"results"`
			} `json:"stage"`
		} `json:"stages"`
	}
	_ = json.Unmarshal(body, &res)
	for _, s := range res.Stages.Stage {
		for _, j := range s.Results.Result {
			if j.State != "Failed" {
				continue
			}
			jobKey := j.Key[:strings.LastIndex(j.Key, "-")]
			if err := r.record("log_entries.json", http.MethodGet, "/rest/api/latest/result/"+j.Key,
				url.Values{"expand": {"logEntries[0:50]"}}); err != nil {
				return err
			}
			return r.record("log_download.log", http.MethodGet, "/download/"+jobKey+"/build_logs/"+j.Key+".log", nil)
		}
	}
	fmt.Println("no failed job found in", buildKey)
	return nil
}

func (r *recorder) recordTriggerAndStop(plan string) error {
	body, status, err := r.call(http.MethodPost, "/rest/api/latest/queue/"+plan, url.Values{"executeAllStages": {"true"}})
	if err != nil || status/100 != 2 {
		return fmt.Errorf("trigger %s: status %d: %v", plan, status, err)
	}
	_ = os.WriteFile(filepath.Join(r.out, "queue.json"), r.scrub.Scrub(body), 0o644)
	var q struct {
		Key string `json:"buildResultKey"`
	}
	_ = json.Unmarshal(body, &q)
	time.Sleep(5 * time.Second)
	_, status, err = r.call(http.MethodDelete, "/rest/api/latest/queue/"+q.Key, nil)
	if err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(r.out, "stop.status"), []byte(fmt.Sprintf("%d\n", status)), 0o644)
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)
		body, status, err = r.call(http.MethodGet, "/rest/api/latest/result/"+q.Key, url.Values{"expand": {"stages.stage.results.result"}})
		if err == nil && status == 200 && strings.Contains(string(body), `"lifeCycleState" : "Finished"`) ||
			err == nil && status == 200 && strings.Contains(string(body), `"lifeCycleState":"Finished"`) {
			break
		}
	}
	return r.record("result_stopped.json", http.MethodGet, "/rest/api/latest/result/"+q.Key,
		url.Values{"expand": {"stages.stage.results.result"}})
}
