package k8s

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"higress-portal-backend/internal/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	wasmPluginGVR = schema.GroupVersionResource{
		Group:    "extensions.higress.io",
		Version:  "v1alpha1",
		Resource: "wasmplugins",
	}
	mcpBridgeGVR = schema.GroupVersionResource{
		Group:    "networking.higress.io",
		Version:  "v1",
		Resource: "mcpbridges",
	}
	ingressGVR = schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingresses",
	}
)

const (
	defaultNamespace  = "aigateway-system"
	defaultMcpBridge  = "default"
	defaultCurrency   = "CNY"
	aiProxyPluginName = "ai-proxy"
)

type ProviderCapabilities struct {
	Modalities []string
	Features   []string
}

type ProviderPricing struct {
	Currency                                                    string
	InputCostPerMillionTokens                                  float64
	OutputCostPerMillionTokens                                 float64
	InputCostPerRequest                                        float64
	CacheCreationInputTokenCostPerMillionTokens                float64
	CacheCreationInputTokenCostAbove1hrPerMillionTokens        float64
	CacheReadInputTokenCostPerMillionTokens                    float64
	InputCostPerMillionTokensAbove200kTokens                   float64
	OutputCostPerMillionTokensAbove200kTokens                  float64
	CacheCreationInputTokenCostPerMillionTokensAbove200kTokens float64
	CacheReadInputTokenCostPerMillionTokensAbove200kTokens     float64
	OutputCostPerImage                                         float64
	OutputImageTokenCostPerMillionTokens                       float64
	InputCostPerImage                                          float64
	InputImageTokenCostPerMillionTokens                        float64
	SupportsPromptCaching                                      bool
}

type ProviderLimits struct {
	RPM           int64
	TPM           int64
	ContextWindow int64
}

type ProviderModelMeta struct {
	Intro        string
	Tags         []string
	Capabilities ProviderCapabilities
	Pricing      ProviderPricing
	Limits       ProviderLimits
}

type ProviderModel struct {
	ID               string
	Type             string
	Protocol         string
	Endpoint         string
	InternalEndpoint string
	InternalRouteURL string
	RouteModel       string
	Meta             ProviderModelMeta
}

type GatewayIngressRoute struct {
	Host   string
	Path   string
	HasTLS bool
}

type providerRouteBinding struct {
	RouteName    string
	Path         string
	InternalPath string
	ExactModel   string
}

type Client struct {
	namespace     string
	dynamicClient dynamic.Interface
	initErr       error
}

func New(cfg config.Config) *Client {
	c := &Client{
		namespace: strings.TrimSpace(cfg.K8sNamespace),
	}
	if c.namespace == "" {
		c.namespace = defaultNamespace
	}

	restCfg, err := buildRestConfig(strings.TrimSpace(cfg.KubeConfigPath))
	if err != nil {
		c.initErr = gerror.Wrap(err, "build kubernetes config failed")
		return c
	}
	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		c.initErr = gerror.Wrap(err, "initialize kubernetes dynamic client failed")
		return c
	}
	c.dynamicClient = dyn
	return c
}

func (c *Client) InitError() error {
	return c.initErr
}

func (c *Client) ListEnabledModels(ctx context.Context) ([]ProviderModel, error) {
	if c.dynamicClient == nil {
		if c.initErr != nil {
			return nil, gerror.Wrap(c.initErr, "kubernetes model catalog unavailable")
		}
		return nil, gerror.New("kubernetes model catalog unavailable")
	}

	providers, err := c.listProviderConfigs(ctx)
	if err != nil {
		return nil, err
	}
	registryNames, err := c.listRegistryNames(ctx)
	if err != nil {
		return nil, err
	}
	routeBindings, err := c.listProviderRouteBindings(ctx)
	if err != nil {
		routeBindings = map[string][]providerRouteBinding{}
	}

	items := make([]ProviderModel, 0, len(providers))
	for _, item := range providers {
		baseEndpoint := item.Endpoint
		registryName := buildRegistryName(item.ID)
		if _, ok := registryNames[registryName]; !ok {
			continue
		}
		bindings := routeBindings[registryName]
		if len(bindings) == 0 {
			items = append(items, item)
			continue
		}
		for _, binding := range bindings {
			bound := item
			if binding.Path != "" {
				bound.Endpoint = buildGatewayEndpoint(binding.Path, bound.Protocol, baseEndpoint)
			}
			if binding.InternalPath != "" {
				bound.InternalEndpoint = buildInternalGatewayEndpoint(binding.InternalPath, binding.Path, bound.Protocol,
					baseEndpoint)
				bound.InternalRouteURL = binding.InternalPath
			}
			if binding.ExactModel != "" {
				bound.RouteModel = binding.ExactModel
				bound.ID = binding.ExactModel
			}
			items = append(items, bound)
		}
	}
	// Keep stable ordering for frontend rendering and detail lookup.
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (c *Client) ListGatewayIngressRoutes(ctx context.Context) ([]GatewayIngressRoute, error) {
	if c.dynamicClient == nil {
		if c.initErr != nil {
			return nil, gerror.Wrap(c.initErr, "kubernetes gateway ingress unavailable")
		}
		return nil, gerror.New("kubernetes gateway ingress unavailable")
	}

	list, err := c.dynamicClient.Resource(ingressGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, gerror.Wrap(err, "list gateway ingresses failed")
	}
	if list == nil || len(list.Items) == 0 {
		return []GatewayIngressRoute{}, nil
	}

	routes := make([]GatewayIngressRoute, 0)
	seen := make(map[string]struct{})
	for _, item := range list.Items {
		tlsHosts, wildcardTLS := ingressTLSHosts(item.Object)
		rules, found, nestedErr := unstructured.NestedSlice(item.Object, "spec", "rules")
		if nestedErr != nil || !found {
			continue
		}
		for _, ruleItem := range rules {
			ruleMap, ok := ruleItem.(map[string]any)
			if !ok {
				continue
			}
			host := strings.TrimSpace(asString(ruleMap["host"]))
			httpMap, ok := ruleMap["http"].(map[string]any)
			if !ok {
				continue
			}
			paths, ok := httpMap["paths"].([]any)
			if !ok {
				continue
			}
			for _, pathItem := range paths {
				pathMap, ok := pathItem.(map[string]any)
				if !ok {
					continue
				}
				path := normalizePath(asString(pathMap["path"]))
				if path == "" {
					continue
				}
				hasTLS := wildcardTLS
				if host != "" {
					if _, ok := tlsHosts[host]; ok {
						hasTLS = true
					}
				}
				key := host + "|" + path + "|" + strconv.FormatBool(hasTLS)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				routes = append(routes, GatewayIngressRoute{
					Host:   host,
					Path:   path,
					HasTLS: hasTLS,
				})
			}
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Host == routes[j].Host {
			if len(routes[i].Path) == len(routes[j].Path) {
				return routes[i].Path < routes[j].Path
			}
			return len(routes[i].Path) > len(routes[j].Path)
		}
		return routes[i].Host < routes[j].Host
	})
	return routes, nil
}

func (c *Client) listProviderConfigs(ctx context.Context) ([]ProviderModel, error) {
	selector := "higress.io/resource-definer=higress,higress.io/wasm-plugin-name=" + aiProxyPluginName
	list, err := c.dynamicClient.Resource(wasmPluginGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "list ai-proxy wasmplugins failed")
	}
	if list == nil || len(list.Items) == 0 {
		return []ProviderModel{}, nil
	}

	providers := make(map[string]ProviderModel)
	for _, item := range list.Items {
		spec, found, nestedErr := unstructured.NestedMap(item.Object, "spec")
		if nestedErr != nil || !found || len(spec) == 0 {
			continue
		}
		defaultConfig, found, nestedErr := unstructured.NestedMap(spec, "defaultConfig")
		if nestedErr != nil || !found || len(defaultConfig) == 0 {
			continue
		}
		providerItems, found, nestedErr := unstructured.NestedSlice(defaultConfig, "providers")
		if nestedErr != nil || !found || len(providerItems) == 0 {
			continue
		}

		for _, providerItem := range providerItems {
			providerMap, ok := providerItem.(map[string]any)
			if !ok {
				continue
			}
			id := asString(providerMap["id"])
			if id == "" {
				continue
			}
			if _, exists := providers[id]; exists {
				continue
			}

			providers[id] = ProviderModel{
				ID:       id,
				Type:     asString(providerMap["type"]),
				Protocol: normalizeProtocol(asString(providerMap["protocol"])),
				Endpoint: inferProviderEndpoint(providerMap),
				Meta:     parsePortalModelMeta(providerMap),
			}
		}
	}

	results := make([]ProviderModel, 0, len(providers))
	for _, item := range providers {
		results = append(results, item)
	}
	return results, nil
}

func (c *Client) listRegistryNames(ctx context.Context) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	obj, err := c.dynamicClient.Resource(mcpBridgeGVR).Namespace(c.namespace).Get(ctx, defaultMcpBridge, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return result, nil
	}
	if err != nil {
		return nil, gerror.Wrap(err, "read default mcpbridge failed")
	}
	if obj == nil {
		return result, nil
	}

	registries, found, nestedErr := unstructured.NestedSlice(obj.Object, "spec", "registries")
	if nestedErr != nil || !found || len(registries) == 0 {
		return result, nil
	}
	for _, registryItem := range registries {
		registryMap, ok := registryItem.(map[string]any)
		if !ok {
			continue
		}
		name := asString(registryMap["name"])
		if name != "" {
			result[name] = struct{}{}
		}
	}
	return result, nil
}

func (c *Client) listProviderRouteBindings(ctx context.Context) (map[string][]providerRouteBinding, error) {
	grouped := make(map[string]map[string]providerRouteBinding)
	modelsByIngress, err := c.listRouteModelsFromModelMapper(ctx)
	if err != nil {
		modelsByIngress = map[string]string{}
	}
	list, err := c.dynamicClient.Resource(ingressGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, gerror.Wrap(err, "list gateway ingresses failed")
	}
	if list == nil || len(list.Items) == 0 {
		return map[string][]providerRouteBinding{}, nil
	}

	for _, item := range list.Items {
		destination, _, _ := unstructured.NestedString(item.Object, "metadata", "annotations", "higress.io/destination")
		registryName := normalizeDestinationRegistryName(destination)
		if registryName == "" {
			continue
		}
		ingressName, _, _ := unstructured.NestedString(item.Object, "metadata", "name")
		routePath := firstIngressPath(item.Object)
		headerModel := extractRouteModelHeader(item.Object)
		mappedModel := strings.TrimSpace(modelsByIngress[strings.TrimSpace(ingressName)])
		if routePath == "" && headerModel == "" && mappedModel == "" {
			continue
		}
		routeName := routeBindingName(ingressName)
		registryBindings := grouped[registryName]
		if registryBindings == nil {
			registryBindings = make(map[string]providerRouteBinding)
		}
		binding := registryBindings[routeName]
		if strings.TrimSpace(binding.RouteName) == "" {
			binding.RouteName = routeName
		}
		if isInternalAIRoutePath(routePath) {
			binding.InternalPath = routePath
		} else if routePath != "" {
			binding.Path = routePath
		}
		if headerModel != "" {
			binding.ExactModel = headerModel
		} else if binding.ExactModel == "" && mappedModel != "" {
			binding.ExactModel = mappedModel
		}
		registryBindings[routeName] = binding
		grouped[registryName] = registryBindings
	}

	result := make(map[string][]providerRouteBinding, len(grouped))
	for registryName, registryBindings := range grouped {
		items := make([]providerRouteBinding, 0, len(registryBindings))
		for _, binding := range registryBindings {
			items = append(items, binding)
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].RouteName < items[j].RouteName
		})
		result[registryName] = items
	}
	return result, nil
}

func (c *Client) listRouteModelsFromModelMapper(ctx context.Context) (map[string]string, error) {
	selector := "higress.io/resource-definer=higress,higress.io/wasm-plugin-name=model-mapper"
	list, err := c.dynamicClient.Resource(wasmPluginGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, gerror.Wrap(err, "list model-mapper wasmplugins failed")
	}
	if list == nil || len(list.Items) == 0 {
		return map[string]string{}, nil
	}

	result := make(map[string]string)
	for _, item := range list.Items {
		matchRules, found, nestedErr := unstructured.NestedSlice(item.Object, "spec", "matchRules")
		if nestedErr != nil || !found || len(matchRules) == 0 {
			continue
		}
		for _, ruleItem := range matchRules {
			ruleMap, ok := ruleItem.(map[string]any)
			if !ok {
				continue
			}
			ingresses := asStringSlice(ruleMap["ingress"])
			if len(ingresses) == 0 {
				continue
			}
			configMap, ok := ruleMap["config"].(map[string]any)
			if !ok {
				continue
			}
			modelMapping, ok := configMap["modelMapping"].(map[string]any)
			if !ok || len(modelMapping) == 0 {
				continue
			}
			modelID := inferMappedModel(modelMapping)
			if modelID == "" {
				continue
			}
			for _, ingressName := range ingresses {
				if _, exists := result[ingressName]; !exists {
					result[ingressName] = modelID
				}
			}
		}
	}
	return result, nil
}

func buildRestConfig(kubeConfigPath string) (*rest.Config, error) {
	if kubeConfigPath != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err == nil {
			return cfg, nil
		}
		return nil, err
	}

	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	defaultConfigPath := filepath.Join(home, ".kube", "config")
	if _, statErr := os.Stat(defaultConfigPath); statErr != nil {
		return nil, statErr
	}
	return clientcmd.BuildConfigFromFlags("", defaultConfigPath)
}

func buildRegistryName(providerID string) string {
	return fmt.Sprintf("llm-%s.internal", providerID)
}

func normalizeProtocol(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "auto", "openai", "openai/v1", "openai/v1/chatcompletions", "openai/v1/chat/completions", "responses", "openai/v1/responses":
		return "openai/v1"
	case "anthropic", "claude", "anthropic/v1/messages", "/v1/messages":
		return "anthropic/v1/messages"
	case "original":
		return "original"
	default:
		return strings.TrimSpace(v)
	}
}

func inferProviderEndpoint(provider map[string]any) string {
	if v := asString(provider["openaiCustomUrl"]); v != "" {
		return v
	}
	if v := asString(provider["azureServiceUrl"]); v != "" {
		return v
	}
	providerType := strings.ToLower(asString(provider["type"]))
	switch providerType {
	case "openai":
		return "https://api.openai.com/v1"
	case "qwen":
		domain := asString(provider["qwenDomain"])
		if domain == "" {
			domain = "dashscope.aliyuncs.com"
		}
		if !strings.Contains(domain, "://") {
			domain = "https://" + domain
		}
		return domain
	case "claude":
		return "https://api.anthropic.com"
	case "zhipuai":
		domain := asString(provider["zhipuDomain"])
		if domain == "" {
			domain = "open.bigmodel.cn"
		}
		if !strings.Contains(domain, "://") {
			domain = "https://" + domain
		}
		return domain
	}
	if domain := asString(provider["domain"]); domain != "" {
		return domain
	}
	return ""
}

func normalizeDestinationRegistryName(destination string) string {
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return ""
	}
	if host, _, ok := strings.Cut(destination, ":"); ok {
		destination = host
	}
	destination = strings.TrimSuffix(destination, ".dns")
	return destination
}

func firstIngressPath(object map[string]any) string {
	rules, found, err := unstructured.NestedSlice(object, "spec", "rules")
	if err != nil || !found {
		return ""
	}
	for _, ruleItem := range rules {
		ruleMap, ok := ruleItem.(map[string]any)
		if !ok {
			continue
		}
		httpMap, ok := ruleMap["http"].(map[string]any)
		if !ok {
			continue
		}
		paths, ok := httpMap["paths"].([]any)
		if !ok {
			continue
		}
		for _, pathItem := range paths {
			pathMap, ok := pathItem.(map[string]any)
			if !ok {
				continue
			}
			path := strings.TrimSpace(asString(pathMap["path"]))
			if path != "" {
				return path
			}
		}
	}
	return ""
}

func extractRouteModelHeader(object map[string]any) string {
	exactModel, _, _ := unstructured.NestedString(object, "metadata", "annotations", "higress.io/exact-match-header-x-higress-llm-model")
	if model := strings.TrimSpace(exactModel); model != "" {
		return model
	}
	prefixModel, _, _ := unstructured.NestedString(object, "metadata", "annotations", "higress.io/prefix-match-header-x-higress-llm-model")
	return strings.TrimSpace(prefixModel)
}

func inferMappedModel(modelMapping map[string]any) string {
	if mapped := strings.TrimSpace(asString(modelMapping["*"])); mapped != "" {
		return mapped
	}
	if len(modelMapping) != 1 {
		return ""
	}
	for key, value := range modelMapping {
		if mapped := strings.TrimSpace(asString(value)); mapped != "" {
			return mapped
		}
		return strings.TrimSpace(key)
	}
	return ""
}

func routeBindingName(ingressName string) string {
	name := strings.TrimSpace(ingressName)
	if strings.HasSuffix(name, "-internal") {
		return strings.TrimSuffix(name, "-internal")
	}
	return name
}

func ingressTLSHosts(object map[string]any) (map[string]struct{}, bool) {
	result := make(map[string]struct{})
	tlsItems, found, err := unstructured.NestedSlice(object, "spec", "tls")
	if err != nil || !found || len(tlsItems) == 0 {
		return result, false
	}
	wildcardTLS := false
	for _, item := range tlsItems {
		tlsMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		hosts := asStringSlice(tlsMap["hosts"])
		if len(hosts) == 0 {
			wildcardTLS = true
			continue
		}
		for _, host := range hosts {
			if host != "" {
				result[host] = struct{}{}
			}
		}
	}
	return result, wildcardTLS
}

func buildGatewayEndpoint(routePath string, protocol string, fallbackEndpoint string) string {
	routePath = normalizePath(routePath)
	suffix := normalizeEndpointSuffix(fallbackEndpoint, protocol)
	if routePath == "" {
		return suffix
	}
	if suffix == "" || suffix == "/" {
		return routePath
	}
	if routePath == "/" {
		return suffix
	}
	if suffix == routePath {
		return routePath
	}
	return strings.TrimRight(routePath, "/") + "/" + strings.TrimLeft(suffix, "/")
}

func buildInternalGatewayEndpoint(internalPath string, publicPath string, protocol string, fallbackEndpoint string) string {
	internalPath = normalizePath(internalPath)
	publicPath = normalizePath(publicPath)
	suffix := normalizeEndpointSuffix(fallbackEndpoint, protocol)
	if publicPath != "" && publicPath != "/" {
		if suffix == publicPath {
			suffix = ""
		} else if strings.HasPrefix(suffix, publicPath+"/") {
			suffix = strings.TrimPrefix(suffix, publicPath)
		}
	}
	if internalPath == "" {
		return suffix
	}
	if suffix == "" || suffix == "/" {
		return internalPath
	}
	return strings.TrimRight(internalPath, "/") + "/" + strings.TrimLeft(suffix, "/")
}

func normalizeEndpointSuffix(endpoint string, protocol string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint != "" && endpoint != "-" {
		if parsed, err := url.Parse(endpoint); err == nil && parsed.Path != "" {
			if parsed.Path == "/v1/chatcompletions" {
				return "/v1/chat/completions"
			}
			return normalizePath(parsed.Path)
		}
		if strings.HasPrefix(endpoint, "/") {
			if endpoint == "/v1/chatcompletions" {
				return "/v1/chat/completions"
			}
			return normalizePath(endpoint)
		}
	}

	switch normalizeProtocol(protocol) {
	case "openai/v1":
		return "/v1/chat/completions"
	case "anthropic/v1/messages":
		return "/v1/messages"
	}
	return ""
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return path
}

func isInternalAIRoutePath(path string) bool {
	path = normalizePath(path)
	return strings.HasPrefix(path, "/internal/ai-routes/")
}

func parsePortalModelMeta(provider map[string]any) ProviderModelMeta {
	meta := ProviderModelMeta{
		Pricing: ProviderPricing{Currency: defaultCurrency},
	}
	metaMap, ok := provider["portalModelMeta"].(map[string]any)
	if !ok || len(metaMap) == 0 {
		return meta
	}

	meta.Intro = asString(metaMap["intro"])
	meta.Tags = asStringSlice(metaMap["tags"])

	if capMap, ok := metaMap["capabilities"].(map[string]any); ok {
		meta.Capabilities.Modalities = asStringSlice(capMap["modalities"])
		meta.Capabilities.Features = asStringSlice(capMap["features"])
	}
	if pricingMap, ok := metaMap["pricing"].(map[string]any); ok {
		if currency := asString(pricingMap["currency"]); currency != "" {
			meta.Pricing.Currency = currency
		}
		meta.Pricing.InputCostPerMillionTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"inputCostPerMillionTokens", "inputPer1K", "input_cost_per_token")
		meta.Pricing.OutputCostPerMillionTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"outputCostPerMillionTokens", "outputPer1K", "output_cost_per_token")
		if v, ok := asFloat64(pricingMap["input_cost_per_request"]); ok {
			meta.Pricing.InputCostPerRequest = v
		}
		meta.Pricing.CacheCreationInputTokenCostPerMillionTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"cacheCreationInputTokenCostPerMillionTokens", "cache_creation_input_token_cost", "cacheCreationInputTokenCost")
		meta.Pricing.CacheCreationInputTokenCostAbove1hrPerMillionTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"cacheCreationInputTokenCostAbove1hrPerMillionTokens", "cache_creation_input_token_cost_above_1hr", "cacheCreationInputTokenCostAbove1hr")
		meta.Pricing.CacheReadInputTokenCostPerMillionTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"cacheReadInputTokenCostPerMillionTokens", "cache_read_input_token_cost", "cacheReadInputTokenCost")
		meta.Pricing.InputCostPerMillionTokensAbove200kTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"inputCostPerMillionTokensAbove200kTokens", "input_cost_per_token_above_200k_tokens", "inputCostPerTokenAbove200kTokens")
		meta.Pricing.OutputCostPerMillionTokensAbove200kTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"outputCostPerMillionTokensAbove200kTokens", "output_cost_per_token_above_200k_tokens", "outputCostPerTokenAbove200kTokens")
		meta.Pricing.CacheCreationInputTokenCostPerMillionTokensAbove200kTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"cacheCreationInputTokenCostPerMillionTokensAbove200kTokens", "cache_creation_input_token_cost_above_200k_tokens", "cacheCreationInputTokenCostAbove200kTokens")
		meta.Pricing.CacheReadInputTokenCostPerMillionTokensAbove200kTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"cacheReadInputTokenCostPerMillionTokensAbove200kTokens", "cache_read_input_token_cost_above_200k_tokens", "cacheReadInputTokenCostAbove200kTokens")
		if v, ok := asFloat64(pricingMap["output_cost_per_image"]); ok {
			meta.Pricing.OutputCostPerImage = v
		}
		meta.Pricing.OutputImageTokenCostPerMillionTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"outputImageTokenCostPerMillionTokens", "output_cost_per_image_token", "outputCostPerImageToken")
		if v, ok := asFloat64(pricingMap["input_cost_per_image"]); ok {
			meta.Pricing.InputCostPerImage = v
		}
		meta.Pricing.InputImageTokenCostPerMillionTokens = readPricingWithLegacyTokenFallback(pricingMap,
			"inputImageTokenCostPerMillionTokens", "input_cost_per_image_token", "inputCostPerImageToken")
		if v, ok := pricingMap["supports_prompt_caching"].(bool); ok {
			meta.Pricing.SupportsPromptCaching = v
		}
	}
	if limitMap, ok := metaMap["limits"].(map[string]any); ok {
		if v, ok := asInt64(limitMap["rpm"]); ok {
			meta.Limits.RPM = v
		}
		if v, ok := asInt64(limitMap["tpm"]); ok {
			meta.Limits.TPM = v
		}
		if v, ok := asInt64(limitMap["contextWindow"]); ok {
			meta.Limits.ContextWindow = v
		}
	}

	return meta
}

func readPricingWithLegacyTokenFallback(pricingMap map[string]any, modernKey string, legacyKeys ...string) float64 {
	if value, ok := asFloat64(pricingMap[modernKey]); ok {
		return value
	}
	for _, key := range legacyKeys {
		if value, ok := asFloat64(pricingMap[key]); ok {
			switch key {
			case "inputPer1K", "outputPer1K":
				return value * 1000
			default:
				return value * 1_000_000
			}
		}
	}
	return 0
}

func asString(v any) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case fmt.Stringer:
		return strings.TrimSpace(val.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", val))
	}
}

func asStringSlice(v any) []string {
	result := make([]string, 0)
	switch val := v.(type) {
	case []string:
		for _, item := range val {
			if normalized := strings.TrimSpace(item); normalized != "" {
				result = append(result, normalized)
			}
		}
	case []any:
		for _, item := range val {
			if normalized := asString(item); normalized != "" {
				result = append(result, normalized)
			}
		}
	case string:
		for _, item := range strings.Split(val, ",") {
			if normalized := strings.TrimSpace(item); normalized != "" {
				result = append(result, normalized)
			}
		}
	}
	return result
}

func asFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func asInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int32:
		return int64(val), true
	case int64:
		return val, true
	case uint:
		return int64(val), true
	case uint32:
		return int64(val), true
	case uint64:
		return int64(val), true
	case float64:
		return int64(val), true
	case float32:
		return int64(val), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		if err == nil {
			return parsed, true
		}
		floatParsed, floatErr := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if floatErr != nil {
			return 0, false
		}
		return int64(floatParsed), true
	default:
		return 0, false
	}
}
