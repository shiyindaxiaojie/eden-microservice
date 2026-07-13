package nacos

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shiyindaxiaojie/eden-registry/internal/configcenter"
)

const (
	nacosConfigLineSeparator = "\x01"
	nacosConfigWordSeparator = "\x02"
)

type ConfigHTTPAdapter struct {
	configs configcenter.Service
}

func NewConfigHTTPAdapter(configs configcenter.Service) *ConfigHTTPAdapter {
	return &ConfigHTTPAdapter{configs: configs}
}

func (a *ConfigHTTPAdapter) RegisterRoutes(mux *http.ServeMux, prefix string, wrap middleware) {
	route := func(path string) string {
		if prefix == "" {
			return path
		}
		return prefix + path
	}
	mux.HandleFunc(route("/v1/cs/configs"), func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			a.query(w, r)
		case http.MethodPost:
			wrap(http.HandlerFunc(a.publish)).ServeHTTP(w, r)
		case http.MethodDelete:
			wrap(http.HandlerFunc(a.delete)).ServeHTTP(w, r)
		default:
			writeNacosConfigError(w, http.StatusMethodNotAllowed, "GET, POST or DELETE required")
		}
	})
	mux.HandleFunc(route("/v1/cs/configs/listener"), a.listener)
}

func identityFromNacos(values url.Values) configcenter.Identity {
	return configcenter.Identity{
		Namespace: values.Get("tenant"),
		Group:     values.Get("group"),
		DataID:    values.Get("dataId"),
	}
}

func parseNacosForm(r *http.Request) (url.Values, error) {
	values := r.URL.Query()
	if r.Body == nil || !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/x-www-form-urlencoded") {
		return values, nil
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read form: %w", err)
	}
	bodyValues, err := url.ParseQuery(string(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid form: %w", err)
	}
	for key, items := range bodyValues {
		for _, item := range items {
			values.Add(key, item)
		}
	}
	return values, nil
}

func (a *ConfigHTTPAdapter) query(w http.ResponseWriter, r *http.Request) {
	values, err := parseNacosForm(r)
	if err != nil {
		writeNacosConfigError(w, http.StatusBadRequest, err.Error())
		return
	}
	resource, err := a.configs.Get(identityFromNacos(values))
	if err != nil {
		if errors.Is(err, configcenter.ErrNotFound) {
			writeNacosConfigError(w, http.StatusNotFound, err.Error())
			return
		}
		writeNacosDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-MD5", resource.MD5)
	w.Header().Set("Config-Type", resource.Type)
	_, _ = io.WriteString(w, resource.Content)
}

func (a *ConfigHTTPAdapter) publish(w http.ResponseWriter, r *http.Request) {
	values, err := parseNacosForm(r)
	if err != nil {
		writeNacosConfigError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = a.configs.Publish(configcenter.PublishRequest{
		Identity:    identityFromNacos(values),
		Content:     values.Get("content"),
		Type:        values.Get("type"),
		Description: values.Get("desc"),
		ExpectedMD5: values.Get("casMd5"),
		Operator:    "nacos-client",
	})
	if err != nil {
		writeNacosDomainError(w, err)
		return
	}
	writeNacosBoolean(w)
}

func (a *ConfigHTTPAdapter) delete(w http.ResponseWriter, r *http.Request) {
	values, err := parseNacosForm(r)
	if err != nil {
		writeNacosConfigError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, err = a.configs.Delete(identityFromNacos(values), "nacos-client")
	if err != nil && !errors.Is(err, configcenter.ErrNotFound) {
		writeNacosDomainError(w, err)
		return
	}
	writeNacosBoolean(w)
}

func (a *ConfigHTTPAdapter) listener(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeNacosConfigError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	values, err := parseNacosForm(r)
	if err != nil {
		writeNacosConfigError(w, http.StatusBadRequest, err.Error())
		return
	}
	raw := values.Get("Listening-Configs")
	if raw == "" {
		raw = r.Header.Get("Listening-Configs")
	}
	targets, err := parseListeningConfigs(raw)
	if err != nil {
		writeNacosConfigError(w, http.StatusBadRequest, err.Error())
		return
	}
	timeout := 30 * time.Second
	if milliseconds, parseErr := strconv.ParseInt(strings.TrimSpace(r.Header.Get("Long-Pulling-Timeout")), 10, 64); parseErr == nil && milliseconds > 0 {
		timeout = time.Duration(milliseconds) * time.Millisecond
	}
	changes, err := a.configs.Wait(targets, timeout)
	if err != nil {
		writeNacosDomainError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, change := range changes {
		_, _ = io.WriteString(w, change.DataID+nacosConfigWordSeparator+change.Group+nacosConfigWordSeparator+change.Namespace+nacosConfigLineSeparator)
	}
}

func parseListeningConfigs(raw string) ([]configcenter.WatchTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("Listening-Configs required")
	}
	items := strings.Split(raw, nacosConfigLineSeparator)
	targets := make([]configcenter.WatchTarget, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		parts := strings.Split(item, nacosConfigWordSeparator)
		if len(parts) < 3 || len(parts) > 4 {
			return nil, fmt.Errorf("invalid Listening-Configs item")
		}
		target := configcenter.WatchTarget{
			Identity: configcenter.Identity{DataID: parts[0], Group: parts[1]},
			MD5:      parts[2],
		}
		if len(parts) == 4 {
			target.Namespace = parts[3]
		}
		targets = append(targets, target)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("Listening-Configs required")
	}
	return targets, nil
}

func writeNacosBoolean(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "true")
}

func writeNacosDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, configcenter.ErrInvalidIdentity), errors.Is(err, configcenter.ErrTooManyTargets):
		writeNacosConfigError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, configcenter.ErrConflict):
		writeNacosConfigError(w, http.StatusConflict, err.Error())
	case errors.Is(err, configcenter.ErrTooManyWaiters):
		writeNacosConfigError(w, http.StatusTooManyRequests, err.Error())
	default:
		writeNacosConfigError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeNacosConfigError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, message)
}
