package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/Digital-Shane/title-tidy/internal/provider"
	"github.com/Digital-Shane/title-tidy/internal/provider/local"
)

// TemplateRegistry manages template variables from all providers
type TemplateRegistry struct {
	mu        sync.RWMutex
	variables map[string]provider.TemplateVariable // variable name -> definition
	providers map[string]provider.Provider         // provider name -> provider
	varOwners map[string][]string                  // variable name -> providers that supply it
	resolver  *TemplateResolver
}

// NewTemplateRegistry creates a new template registry
func NewTemplateRegistry() *TemplateRegistry {
	return &TemplateRegistry{
		variables: make(map[string]provider.TemplateVariable),
		providers: make(map[string]provider.Provider),
		varOwners: make(map[string][]string),
		resolver:  NewTemplateResolver(),
	}
}

// RegisterProvider registers a provider and its variables
func (r *TemplateRegistry) RegisterProvider(p provider.Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	r.providers[name] = p

	// Register all variables from this provider
	for _, v := range p.SupportedVariables() {
		// Ensure provider name is set
		v.Provider = name

		owners := r.varOwners[v.Name]
		if !slices.Contains(owners, name) {
			r.varOwners[v.Name] = append(owners, name)
		}

		// Check for conflicts with existing variables
		if _, exists := r.variables[v.Name]; exists {
			continue
		}

		r.variables[v.Name] = v
	}

	return nil
}

// UnregisterProvider removes a provider and its variables
func (r *TemplateRegistry) UnregisterProvider(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return
	}

	// Remove provider
	delete(r.providers, name)

	// Remove variables that came from this provider
	for varName, v := range r.variables {
		if v.Provider == name {
			delete(r.variables, varName)
		}
	}

	for varName, owners := range r.varOwners {
		r.varOwners[varName] = removeProvider(owners, name)
		if len(r.varOwners[varName]) == 0 {
			delete(r.varOwners, varName)
			delete(r.variables, varName)
		}
	}
}

// GetAvailableVariables returns all available template variables
func (r *TemplateRegistry) GetAvailableVariables() []provider.TemplateVariable {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]provider.TemplateVariable, 0, len(r.variables))
	for _, v := range r.variables {
		result = append(result, v)
	}
	return result
}

// GetVariablesForMediaType returns variables available for a specific media type
func (r *TemplateRegistry) GetVariablesForMediaType(mediaType provider.MediaType) []provider.TemplateVariable {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := []provider.TemplateVariable{}
	for _, v := range r.variables {
		// If no media types specified, assume available for all
		if len(v.MediaTypes) == 0 {
			result = append(result, v)
			continue
		}

		// Check if this variable supports the media type
		for _, mt := range v.MediaTypes {
			if mt == mediaType {
				result = append(result, v)
				break
			}
		}
	}
	return result
}

// GetVariablesByProvider returns variables from a specific provider
func (r *TemplateRegistry) GetVariablesByProvider(providerName string) []provider.TemplateVariable {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := []provider.TemplateVariable{}
	for name, v := range r.variables {
		if slices.Contains(r.varOwners[name], providerName) {
			clone := v
			clone.Provider = providerName
			result = append(result, clone)
		}
	}
	return result
}

// VariableProviders returns all providers that can supply a given variable.
func (r *TemplateRegistry) VariableProviders(variableName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	owners := r.varOwners[variableName]
	result := make([]string, len(owners))
	copy(result, owners)
	return result
}

// ResolveTemplate processes a template string with metadata
func (r *TemplateRegistry) ResolveTemplate(template string, ctx *FormatContext, metadata *provider.Metadata) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.resolver.Resolve(template, ctx, metadata, r.providers)
}

// GetProvider returns a registered provider by name
func (r *TemplateRegistry) GetProvider(name string) (provider.Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.providers[name]
	return p, exists
}

// GetProviders returns all registered providers
func (r *TemplateRegistry) GetProviders() []provider.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]provider.Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

func removeProvider(list []string, name string) []string {
	result := make([]string, 0, len(list))
	for _, item := range list {
		if item != name {
			result = append(result, item)
		}
	}
	return result
}

// TemplateResolver handles template variable resolution
type TemplateResolver struct {
	variablePattern *regexp.Regexp
}

// NewTemplateResolver creates a new template resolver
func NewTemplateResolver() *TemplateResolver {
	return &TemplateResolver{
		variablePattern: regexp.MustCompile(`\{([^}]+)\}`),
	}
}

// Resolve processes a template string, replacing variables with values
func (r *TemplateResolver) Resolve(template string, ctx *FormatContext, metadata *provider.Metadata, providers map[string]provider.Provider) (string, error) {
	result := template

	// Find all variables in the template
	matches := r.variablePattern.FindAllStringSubmatch(template, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		varName := match[1]
		varPlaceholder := match[0] // includes the braces

		// Resolve the variable value
		value, err := r.resolveVariable(varName, ctx, metadata, providers)
		if err != nil {
			// Variable couldn't be resolved, leave it as is or use empty string
			value = ""
		}

		result = strings.ReplaceAll(result, varPlaceholder, value)
	}

	// Clean up the result
	result = local.CleanName(result)

	return result, nil
}

// resolveVariable resolves a single variable to its value
func (r *TemplateResolver) resolveVariable(varName string, ctx *FormatContext, metadata *provider.Metadata, providers map[string]provider.Provider) (string, error) {
	// For variables that can come from both local context and metadata, prefer metadata
	switch varName {
	case "title", "show", "movie", "year":
		if metadata != nil {
			if value := r.resolveFromMetadata(varName, metadata); value != "" {
				return value, nil
			}
		}
		if value := r.resolveCoreVariable(varName, ctx); value != "" {
			return value, nil
		}
	case "season", "episode":
		// These always come from core variables
		if value := r.resolveCoreVariable(varName, ctx); value != "" {
			return value, nil
		}
	default:
		// For all other variables, try metadata only
		if metadata != nil {
			if value := r.resolveFromMetadata(varName, metadata); value != "" {
				return value, nil
			}
		}
	}

	// Variable not found
	return "", fmt.Errorf("variable %s not found", varName)
}

// resolveCoreVariable resolves core variables that don't need metadata
func (r *TemplateResolver) resolveCoreVariable(varName string, ctx *FormatContext) string {
	switch varName {
	case "show":
		return ctx.ShowName
	case "movie":
		return ctx.MovieName
	case "title":
		if ctx.MovieName != "" {
			return ctx.MovieName
		}
		return ctx.ShowName
	case "year":
		return ctx.Year
	case "season":
		if ctx.Season >= 0 {
			return fmt.Sprintf("%02d", ctx.Season)
		}
		return ""
	case "episode":
		if ctx.Episode >= 0 {
			return fmt.Sprintf("%02d", ctx.Episode)
		}
		return ""
	default:
		return ""
	}
}

// resolveFromMetadata resolves a variable from metadata
func (r *TemplateResolver) resolveFromMetadata(varName string, metadata *provider.Metadata) string {
	// Check core metadata fields
	switch varName {
	case "title":
		return metadata.Core.Title
	case "year":
		return metadata.Core.Year
	case "season", "season_num":
		if metadata.Core.SeasonNum > 0 {
			return fmt.Sprintf("%02d", metadata.Core.SeasonNum)
		}
		return ""
	case "episode_name", "episode_title":
		return metadata.Core.EpisodeName
	case "episode", "episode_num":
		if metadata.Core.EpisodeNum > 0 {
			return fmt.Sprintf("%02d", metadata.Core.EpisodeNum)
		}
		return ""
	case "overview":
		return metadata.Core.Overview
	case "rating":
		if metadata.Core.Rating > 0 {
			return fmt.Sprintf("%.1f", metadata.Core.Rating)
		}
		return ""
	case "genres":
		return strings.Join(metadata.Core.Genres, ", ")
	case "language":
		return metadata.Core.Language
	case "country":
		return metadata.Core.Country
	}

	// Check extended metadata fields
	if metadata.Extended != nil {
		if value, exists := metadata.Extended[varName]; exists {
			return fmt.Sprintf("%v", value)
		}
	}

	// Check IDs
	if metadata.IDs != nil {
		if value, exists := metadata.IDs[varName]; exists {
			return value
		}
	}

	return ""
}

// ValidateTemplate checks if a template string is valid
func (r *TemplateRegistry) ValidateTemplate(template string, mediaType provider.MediaType) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find all variables in the template
	matches := r.resolver.variablePattern.FindAllStringSubmatch(template, -1)

	availableVars := r.GetVariablesForMediaType(mediaType)
	availableMap := make(map[string]bool)
	for _, v := range availableVars {
		availableMap[v.Name] = true
	}

	// Also add core variables that are always available
	coreVars := []string{"title", "year", "show", "movie", "season", "episode"}
	for _, v := range coreVars {
		availableMap[v] = true
	}

	// Check each variable in the template
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		varName := match[1]

		if !availableMap[varName] {
			return fmt.Errorf("unknown variable: {%s}", varName)
		}
	}

	return nil
}

// GetVariableHelp returns help text for a variable
func (r *TemplateRegistry) GetVariableHelp(varName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if v, exists := r.variables[varName]; exists {
		help := v.Description
		if v.Example != "" {
			help += fmt.Sprintf(" (Example: %s)", v.Example)
		}
		return help
	}

	// Check for core variables
	coreHelp := map[string]string{
		"title":   "The title of the media (movie or show name)",
		"year":    "The release year",
		"show":    "The TV show name",
		"movie":   "The movie name",
		"season":  "The season number (padded to 2 digits)",
		"episode": "The episode number (padded to 2 digits)",
	}

	if help, exists := coreHelp[varName]; exists {
		return help
	}

	return "Unknown variable"
}
