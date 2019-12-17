package nicehttp

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

var (
	matchSlashGroup = regexp.MustCompile(`\\(\d+)`)
)

// IsProdAppEngine returns whether environment variables match the production App Engine environment.
func IsProdAppEngine() bool {
	return os.Getenv("GAE_DEPLOYMENT_ID") != ""
}

// handlerRule describes a rule in the handlers: section of the app.yaml for local development only.
type handlerRule struct {
	URL         string            `yaml:"url"`
	StaticFiles string            `yaml:"static_files"`
	StaticDir   string            `yaml:"static_dir"`
	MimeType    string            `yaml:"mime_type"`
	HTTPHeaders map[string]string `yaml:"http_headers"`
}

func (hr *handlerRule) Valid() bool {
	return hr.URL != "" && (hr.StaticFiles != "" || hr.StaticDir != "")
}

func parseRules(yamlPath string) ([]handlerRule, error) {
	var parsed struct {
		Handlers []handlerRule `yaml:"handlers"`
	}
	raw, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(raw, &parsed)

	// drop rules that don't look right
	valid := make([]handlerRule, 0, len(parsed.Handlers))
	for _, rule := range parsed.Handlers {
		if rule.StaticFiles != "" && rule.StaticDir != "" {
			return nil, errors.New("can't specify both static_files and static_dir")
		}
		if rule.Valid() {
			valid = append(valid, rule)
		}
	}
	return valid, err
}

func buildHandler(rules []handlerRule, fallback http.Handler) (http.Handler, error) {
	// compile all regexp
	matchers := make([]*regexp.Regexp, 0, len(rules))
	for _, rule := range rules {

		format := "^%s"
		if rule.StaticFiles != "" {
			format += "$" // files match whole thing
		} else if !strings.HasSuffix(rule.StaticDir, "/") {
			format += "/" // static_dir must end with "/"
		}

		if !strings.HasPrefix(rule.URL, "/") {
			return nil, errors.New("expected static URL to begin with /")
		}

		safe := fmt.Sprintf(format, rule.URL)
		re, err := regexp.Compile(safe)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, re)
	}

	f := func(w http.ResponseWriter, r *http.Request) {
		for i, re := range matchers {
			m := re.FindStringSubmatch(r.URL.Path)
			if m == nil {
				continue
			}
			rule := rules[i]

			if rule.MimeType != "" {
				w.Header().Set("Content-Type", rule.MimeType)
			}
			for key, value := range rule.HTTPHeaders {
				w.Header().Set(key, value)
			}

			if rule.StaticDir != "" {
				trailer := r.URL.Path[len(m[0]):]
				r.URL.Path = trailer

				handler := http.FileServer(http.Dir(rule.StaticDir))
				handler.ServeHTTP(w, r)
				return
			}

			// otherwise, rewrite staticFiles \1 \2 etc
			p := matchSlashGroup.ReplaceAllStringFunc(rule.StaticFiles, func(match string) string {
				index, _ := strconv.Atoi(match[1:])
				if index <= 0 || index >= len(m) {
					// in prod, this is a mismatch
					return ""
				}
				return m[index]
			})
			http.ServeFile(w, r, p) // nb. http.ServeFile refuses to serve paths with ".."
			return
		}

		fallback.ServeHTTP(w, r)
	}
	return http.HandlerFunc(f), nil
}

// Static builds a *ServeMux that enacts the given handler rules, or falls back to the passed http.Handler.
func Static(yamlPath string, fallback http.Handler) (http.Handler, error) {
	if fallback == nil {
		fallback = http.DefaultServeMux
	}
	rules, err := parseRules(yamlPath)
	if err != nil {
		return nil, err
	}
	return buildHandler(rules, fallback)
}

// Serve hosts this App Engine application.
// In dev, hosts on port 8080 and reads the passed YAML file for static handlers. This is needed for runtimes like go112, which don't serve handlers locally.
// Don't use this for go111 or earlier, which still use dev_appserver.
func Serve(yamlPath string, handler http.Handler) {
	if handler == nil {
		handler = http.DefaultServeMux
	}

	if !IsProdAppEngine() && yamlPath != "" {
		var err error
		handler, err = Static(yamlPath, handler)
		if err != nil {
			panic(err)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Serving on :%s...", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), handler))
}
